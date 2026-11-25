package feynman

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Bayern99/mark-go-go/internal/fsutil"
	"github.com/Bayern99/mark-go-go/internal/pirpc"
	"github.com/Bayern99/mark-go-go/internal/review"
	"github.com/egoist/mygo"
)

func TestMain(m *testing.M) {
	if os.Getenv("MGO_FIXTURE_FEYNMAN_RPC") == "1" {
		runFeynmanRPCFixture()
		os.Exit(0)
	}
	if os.Getenv("MGO_FIXTURE_FEYNMAN_CLI") == "1" {
		runFeynmanCLIFixture()
		os.Exit(0)
	}
	home, err := os.MkdirTemp("", "feynman-test-home-")
	if err != nil {
		panic(err)
	}
	if err := os.Setenv("HOME", home); err != nil {
		panic(err)
	}
	if err := os.Setenv("USERPROFILE", home); err != nil {
		panic(err)
	}
	code := m.Run()
	_ = os.RemoveAll(home)
	os.Exit(code)
}

func TestDirectFeynmanCommandAndFraming(t *testing.T) {
	home := t.TempDir()
	root := t.TempDir()
	data := t.TempDir()
	bin := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("FEYNMAN_HOME", home)
	t.Setenv("PI_CODING_AGENT_SESSION_DIR", "")
	t.Setenv("PATH", bin)

	manager := NewResearchManager(fakeWorkspaceLookup{dataDir: data})
	defer manager.Close()
	work := review.WorkDir(data, root)
	if _, err := manager.researchCommand(root, work); err == nil || !strings.Contains(err.Error(), "Install Feynman CLI") {
		t.Fatalf("expected missing feynman error without pi-acp/node mentions, got: %v", err)
	}

	fakeFeynman := filepath.Join(bin, "feynman")
	if err := os.WriteFile(fakeFeynman, []byte("#!/bin/sh\nprintf '%s|%s' \"$FEYNMAN_HOME\" \"$HOME\"\n"), 0700); err != nil {
		t.Fatal(err)
	}
	profile := filepath.Join(fsutil.CanonicalPath(data), "feynman")
	globalTrust := filepath.Join(home, ".feynman", "agent", "trust.json")
	if err := os.MkdirAll(filepath.Dir(globalTrust), 0700); err != nil {
		t.Fatal(err)
	}
	const denied = `{"/unrelated":false}`
	if err := os.WriteFile(globalTrust, []byte(denied), 0600); err != nil {
		t.Fatal(err)
	}

	cmd, err := manager.researchCommand(root, work)
	if err != nil {
		t.Fatalf("researchCommand failed: %v", err)
	}
	if filepath.Base(cmd.Path) == "sandbox-exec" || slices.Contains(cmd.Args, "/usr/bin/sandbox-exec") {
		t.Fatalf("command must not use sandbox-exec: %v", cmd.Args)
	}
	for _, arg := range cmd.Args {
		if strings.Contains(arg, "pi-acp") || strings.Contains(arg, "feynman-rpc.mjs") {
			t.Fatalf("command must not reference pi-acp or MITM script: %v", cmd.Args)
		}
	}
	for _, env := range cmd.Env {
		if strings.HasPrefix(env, "PI_ACP_") || strings.HasPrefix(env, "MGO_FEYNMAN_UI_") {
			t.Fatalf("command env must not contain PI_ACP_* or MGO_FEYNMAN_UI_*: %s", env)
		}
	}
	if !fsutil.SameRoot(cmd.Dir, work) {
		t.Fatalf("cmd.Dir = %q, want %q", cmd.Dir, work)
	}
	wantSessions := filepath.Join(profile, ".feynman", "sessions")
	if !containsArgPair(cmd.Args, "--mode", "rpc") {
		t.Fatalf("missing --mode rpc in args: %v", cmd.Args)
	}
	if !containsArgPair(cmd.Args, "--session-dir", wantSessions) {
		t.Fatalf("missing --session-dir %q in args: %v", wantSessions, cmd.Args)
	}
	if !containsArgPair(cmd.Args, "--cwd", fsutil.CanonicalPath(work)) {
		t.Fatalf("missing --cwd %q in args: %v", fsutil.CanonicalPath(work), cmd.Args)
	}
	if !slices.Contains(cmd.Environ(), "FEYNMAN_HOME="+profile) {
		t.Error("Research must launch with the app-private Feynman home")
	}
	trust, err := os.ReadFile(filepath.Join(profile, ".feynman", "agent", "trust.json"))
	var decisions map[string]bool
	if err != nil || json.Unmarshal(trust, &decisions) != nil || !decisions[fsutil.CanonicalPath(work)] {
		t.Errorf("wanted the app-managed work directory trusted, got %s (%v)", trust, err)
	}
	for dir, trusted := range decisions {
		if trusted && dir != fsutil.CanonicalPath(work) {
			t.Errorf("unexpected trusted directory %s", dir)
		}
	}
	if got, err := os.ReadFile(globalTrust); err != nil || string(got) != denied {
		t.Errorf("global trust changed: %v", err)
	}
	if doctor, err := manager.CLIDoctor(context.Background(), root); err != nil || doctor.RawOutput != profile+"|"+home {
		t.Errorf("Doctor used a different profile: %q (%v)", doctor.RawOutput, err)
	}

	if _, err := manager.researchCommand(work, work); err == nil {
		t.Fatal("expected overlap between workspace root and work dir to fail")
	}

	h := NewResearchManager(fakeWorkspaceLookup{dataDir: data})
	defer h.Close()
	h.command = fixtureCommand(home)

	events := make(chan ResearchUpdate, 64)
	if err := h.connect(WorkspaceRef{Root: root}, func(e ResearchUpdate) { events <- e }); err != nil {
		t.Fatalf("connect failed: %v", err)
	}
	cmds := waitUpdate(t, events, "commands").Commands
	if len(cmds) == 0 {
		t.Fatal("expected dynamic commands after banner + CRLF framing")
	}

	ctx := context.Background()
	done := make(chan error, 1)
	go func() {
		_, err := h.Prompt(ctx, root, "large payload test", "", "")
		done <- err
	}()
	// The prompt's result and the large update can arrive in either order, so
	// each is received exactly once; the guard only turns a hang into a failure.
	var gotLarge, prompted bool
	guard := time.After(time.Minute)
	for !gotLarge || !prompted {
		select {
		case e := <-events:
			if e.Kind == "assistant" && len(e.Text) > 1<<20 {
				gotLarge = true
			}
		case err := <-done:
			if err != nil {
				t.Fatalf("large payload prompt failed: %v", err)
			}
			prompted = true
		case <-guard:
			t.Fatalf("large payload prompt: update received %v, prompt returned %v", gotLarge, prompted)
		}
	}
}

func TestResearchNativeRPCStreamingAndMidTurnControls(t *testing.T) {
	home := t.TempDir()
	root := t.TempDir()
	data := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("FEYNMAN_HOME", home)

	h := NewResearchManager(fakeWorkspaceLookup{dataDir: data})
	defer h.Close()
	h.command = fixtureCommand(home)

	ctx := context.Background()
	for _, command := range []string{"/new", "/quit", "/exit"} {
		if _, err := h.Prompt(ctx, root, command, "", ""); err == nil || (!strings.Contains(err.Error(), "Conversation history") && !strings.Contains(err.Error(), "Plugins")) {
			t.Fatalf("%s should point to its workspace control, got %v", command, err)
		}
	}

	events := make(chan ResearchUpdate, 128)
	if err := h.connect(WorkspaceRef{Root: root}, func(e ResearchUpdate) { events <- e }); err != nil {
		t.Fatal(err)
	}

	cmdUpdate := waitUpdate(t, events, "commands")
	if len(cmdUpdate.Commands) != 3 {
		t.Fatalf("expected 3 commands (filtering new/quit/exit), got %+v", cmdUpdate.Commands)
	}
	byName := map[string]ResearchCommand{}
	for _, c := range cmdUpdate.Commands {
		byName[c.Name] = c
	}
	lit := byName["lit"]
	if lit.Source != "prompt" || lit.SourceInfo == nil || lit.SourceInfo.Path != "/opt/feynman/prompts/lit.md" || lit.SourceInfo.Scope != "user" || lit.SourceInfo.Origin != "top-level" {
		t.Fatalf("lit command lost source or sourceInfo: %+v", lit)
	}
	skill := byName["skill:paper-search"]
	if skill.Source != "skill" || skill.SourceInfo == nil || skill.SourceInfo.BaseDir != "/opt/feynman/skills/paper-search" || skill.SourceInfo.Origin != "package" {
		t.Fatalf("skill command lost source or sourceInfo: %+v", skill)
	}
	sourceJSON, err := json.Marshal(skill.SourceInfo)
	if err != nil || !bytes.Contains(sourceJSON, []byte(`"changelogPath":"/opt/feynman/CHANGELOG.md"`)) {
		t.Fatalf("skill command lost changelog path: %s (%v)", sourceJSON, err)
	}
	ext := byName["tools"]
	if ext.Source != "extension" || ext.SourceInfo == nil || ext.SourceInfo.Source != "local" {
		t.Fatalf("extension command lost source or sourceInfo: %+v", ext)
	}

	directCmds, err := h.Host().Commands(ctx, root)
	if err != nil || len(directCmds) != 3 || directCmds[0].SourceInfo == nil {
		t.Fatalf("Host().Commands failed: %+v %v", directCmds, err)
	}

	if _, err := h.Host().SetModel(ctx, root, "unknown", "missing"); err == nil {
		t.Fatal("expected unknown model to fail")
	}
	model, err := h.Host().SetModel(ctx, root, "openai", "gpt-5.4")
	if err != nil || model.Provider != "openai" || model.ID != "gpt-5.4" {
		t.Fatalf("SetModel failed: %+v %v", model, err)
	}
	if err := h.Host().SetThinkingLevel(ctx, root, "high"); err != nil {
		t.Fatalf("SetThinkingLevel failed: %v", err)
	}
	state, err := h.Host().State(ctx, root)
	if err != nil || state.Model == nil || state.Model.ID != "gpt-5.4" || state.ThinkingLevel != "high" {
		t.Fatalf("State mismatch after SetModel/SetThinkingLevel: %+v %v", state, err)
	}
	stats, err := h.Host().SessionStats(ctx, root)
	if err != nil || stats.Tokens.Total != 1500 || stats.ContextUsage == nil || stats.ContextUsage.ContextWindow != 200000 {
		t.Fatalf("SessionStats mismatch: %+v %v", stats, err)
	}
	compacted, err := h.Host().Compact(ctx, root, "Keep equations")
	if err != nil || compacted.Summary == "" || compacted.TokensBefore != 50000 || compacted.EstimatedTokensAfter != 12000 {
		t.Fatalf("Compact mismatch: %+v %v", compacted, err)
	}

	drainEvents(events)
	done := make(chan error, 1)
	go func() {
		_, err := h.Host().Prompt(ctx, root, "stream full lifecycle", "", "")
		done <- err
	}()
	var (
		sawThought      string
		sawAssistant    string
		sawToolStart    bool
		sawToolUpdate   bool
		sawToolEnd      bool
		sawToolImage    bool
		sawToolDiff     bool
		sawRetryStart   bool
		sawRetryEnd     bool
		sawCompactStart bool
		sawCompactEnd   bool
		sawTurnEnd      bool
		sawAgentEnd     bool
	)
	promptDone := (<-chan error)(done)
	promptFinished := false
	streamSettled := false
	hangGuard := researchTestHangGuard(t)
	for !promptFinished || !streamSettled {
		select {
		case e := <-events:
			switch e.Kind {
			case "thought":
				sawThought += e.Text
			case "assistant":
				sawAssistant += e.Text
			case "tool":
				if e.Tool != nil {
					switch e.EventType {
					case "tool_execution_start":
						sawToolStart = e.Tool.Status == "in_progress" && len(e.Tool.Locations) == 1 && e.Tool.Locations[0].Line == 4
					case "tool_execution_update":
						sawToolUpdate = strings.Contains(e.Tool.Output, "partial notes")
					case "tool_execution_end":
						sawToolEnd = e.Tool.Status == "completed" && strings.Contains(e.Tool.Output, "final notes")
						sawToolImage = len(e.Tool.Images) == 1 && strings.HasPrefix(e.Tool.Images[0], "data:image/png;base64,")
						sawToolDiff = len(e.Tool.Diffs) == 1 && e.Tool.Diffs[0].NewText == "Second"
					}
				}
			case "retryStart":
				sawRetryStart = e.RetryAttempt == 1 && e.RetryMaxAttempts == 3
			case "retryEnd":
				sawRetryEnd = e.RetrySuccess && e.RetryAttempt == 2
			case "compactionStart":
				sawCompactStart = e.Reason == "threshold"
			case "compactionEnd":
				sawCompactEnd = e.Compaction != nil && e.Compaction.TokensBefore == 80000
			case "turnEnd":
				sawTurnEnd = true
			case "agentEnd":
				sawAgentEnd = true
			case "agentSettled":
				streamSettled = true
			}
		case err := <-promptDone:
			if err != nil {
				t.Fatalf("stream full lifecycle prompt failed: %v", err)
			}
			promptFinished = true
			promptDone = nil
		case <-hangGuard:
			t.Fatal("hung waiting for stream lifecycle events and prompt completion")
		}
	}
	if sawThought != "Analyzing primary sources." {
		t.Fatalf("thought delta = %q", sawThought)
	}
	if sawAssistant != "Synthesized findings." {
		t.Fatalf("assistant delta = %q", sawAssistant)
	}
	if !sawToolStart || !sawToolUpdate || !sawToolEnd || !sawToolImage || !sawToolDiff {
		t.Fatalf("tool events missing: start=%v update=%v end=%v image=%v diff=%v", sawToolStart, sawToolUpdate, sawToolEnd, sawToolImage, sawToolDiff)
	}
	if !sawRetryStart || !sawRetryEnd || !sawCompactStart || !sawCompactEnd || !sawTurnEnd || !sawAgentEnd {
		t.Fatalf("lifecycle events missing: retryStart=%v retryEnd=%v compactStart=%v compactEnd=%v turnEnd=%v agentEnd=%v",
			sawRetryStart, sawRetryEnd, sawCompactStart, sawCompactEnd, sawTurnEnd, sawAgentEnd)
	}

	drainEvents(events)
	go func() {
		_, err := h.Host().Prompt(ctx, root, "wait for steer and follow_up", "", "")
		done <- err
	}()
	waitUpdate(t, events, "progress")
	if err := h.Host().Steer(ctx, root, "Focus on acoustic latency", nil); err != nil {
		t.Fatalf("Steer failed: %v", err)
	}
	if err := h.Host().FollowUp(ctx, root, "Also summarize limitations", nil); err != nil {
		t.Fatalf("FollowUp failed: %v", err)
	}
	if err := h.Host().FollowUp(ctx, root, "To be cleared", nil); err != nil {
		t.Fatalf("second FollowUp failed: %v", err)
	}
	cleared, err := h.Host().ClearQueue(ctx, root)
	if err != nil || len(cleared.Steering) != 1 || cleared.Steering[0] != "Focus on acoustic latency" || len(cleared.FollowUp) != 2 {
		t.Fatalf("ClearQueue returned %+v, err=%v", cleared, err)
	}
	if err := h.Host().Steer(ctx, root, "Focus on acoustic latency", nil); err != nil {
		t.Fatalf("re-Steer failed: %v", err)
	}
	if err := h.Host().FollowUp(ctx, root, "Also summarize limitations", nil); err != nil {
		t.Fatalf("re-FollowUp failed: %v", err)
	}
	if err := h.Host().Steer(ctx, root, "release turn", nil); err != nil {
		t.Fatalf("release Steer failed: %v", err)
	}
	if err := <-done; err != nil {
		t.Fatalf("steered prompt failed: %v", err)
	}
	var combined string
	for len(events) > 0 {
		e := <-events
		if e.Kind == "assistant" {
			combined += e.Text
		}
	}
	if !strings.Contains(combined, "Steered: Focus on acoustic latency") || !strings.Contains(combined, "FollowUp: Also summarize limitations") {
		t.Fatalf("missing steered/follow-up output in %q", combined)
	}

	drainEvents(events)
	go func() {
		_, err := h.Host().Prompt(ctx, root, "cancel this", "", "")
		done <- err
	}()
	waitUpdate(t, events, "select")
	if err := h.Host().Abort(ctx, root); err != nil {
		t.Fatalf("Abort failed: %v", err)
	}
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("aborted prompt should return error")
		}
	case <-researchTestHangGuard(t):
		t.Fatal("aborted prompt hung")
	}
}

func TestResearchPublishesArtifactWrittenBeforeAgentSettled(t *testing.T) {
	home, root, data := t.TempDir(), t.TempDir(), t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("FEYNMAN_HOME", home)
	h := NewResearchManager(fakeWorkspaceLookup{dataDir: data})
	defer h.Close()
	h.command = fixtureCommand(home)
	events := make(chan ResearchUpdate, 128)
	if err := h.connect(WorkspaceRef{Root: root}, func(e ResearchUpdate) { events <- e }); err != nil {
		t.Fatal(err)
	}
	waitUpdate(t, events, "commands")
	if _, err := h.Host().Prompt(context.Background(), root, "write after agent_end", "", ""); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(filepath.Join(root, "late.md"))
	if err != nil || string(content) != "# Late artifact\n" {
		t.Fatalf("settled artifact was not published in the same turn: %q, %v", content, err)
	}
}

func TestResearchAllNineExtensionUIRequests(t *testing.T) {
	home := t.TempDir()
	root := t.TempDir()
	data := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("FEYNMAN_HOME", home)

	h := NewResearchManager(fakeWorkspaceLookup{dataDir: data})
	defer h.Close()
	h.command = fixtureCommand(home)

	events := make(chan ResearchUpdate, 128)
	if err := h.connect(WorkspaceRef{Root: root}, func(e ResearchUpdate) { events <- e }); err != nil {
		t.Fatal(err)
	}
	waitUpdate(t, events, "commands")
	drainEvents(events)

	ctx := context.Background()
	done := make(chan error, 1)
	go func() {
		_, err := h.Host().Prompt(ctx, root, "exercise all 9 extension ui", "", "")
		done <- err
	}()

	sel := waitUpdate(t, events, "select")
	if sel.UIMethod != "select" || sel.Title != "Choose environment" || len(sel.Options) != 2 {
		t.Fatalf("unexpected select request: %+v", sel)
	}
	if err := h.Host().AnswerText(ctx, root, sel.RequestID, "Local", false); err == nil {
		t.Fatal("AnswerText should fail for select request")
	}
	if err := h.Host().Answer(ctx, root, sel.RequestID, "nonexistent"); err == nil {
		t.Fatal("unknown option should fail for select request")
	}
	if err := h.Host().Answer(ctx, root, sel.RequestID, "Docker"); err != nil {
		t.Fatalf("Answer(select) failed: %v", err)
	}
	waitUpdate(t, events, "permissionDone")

	conf := waitUpdate(t, events, "confirm")
	if conf.UIMethod != "confirm" || conf.Title != "Run benchmark?" || !strings.Contains(conf.Message, "Takes 2 minutes") {
		t.Fatalf("unexpected confirm request: %+v", conf)
	}
	if err := h.Host().AnswerConfirm(ctx, root, conf.RequestID, true, false); err != nil {
		t.Fatalf("AnswerConfirm failed: %v", err)
	}
	waitUpdate(t, events, "permissionDone")

	inp := waitUpdate(t, events, "input")
	if inp.UIMethod != "input" || inp.Title != "Dataset name" || inp.Placeholder != "e.g. LibriSpeech" {
		t.Fatalf("unexpected input request: %+v", inp)
	}
	if err := h.Host().Answer(ctx, root, inp.RequestID, "Docker"); err == nil {
		t.Fatal("Answer should fail for input request")
	}
	if err := h.Host().AnswerText(ctx, root, inp.RequestID, "AudioSet", false); err != nil {
		t.Fatalf("AnswerText(input) failed: %v", err)
	}
	waitUpdate(t, events, "permissionDone")

	ed := waitUpdate(t, events, "editor")
	if ed.UIMethod != "editor" || ed.Title != "Edit abstract" || ed.Prefill != "Draft abstract" {
		t.Fatalf("unexpected editor request: %+v", ed)
	}
	if err := h.Host().AnswerText(ctx, root, ed.RequestID, "Revised abstract", false); err != nil {
		t.Fatalf("AnswerText(editor) failed: %v", err)
	}
	waitUpdate(t, events, "permissionDone")

	not := waitUpdate(t, events, "notify")
	if not.UIMethod != "notify" || not.NotifyType != "warning" || not.Text != "Rate limit approaching" {
		t.Fatalf("unexpected notify request: %+v", not)
	}

	st := waitUpdate(t, events, "setStatus")
	if st.UIMethod != "setStatus" || st.StatusKey != "indexer" || st.Key != "status:indexer" || st.Text != "Indexing 42 papers" {
		t.Fatalf("unexpected setStatus request: %+v", st)
	}

	wg := waitUpdate(t, events, "setWidget")
	if wg.UIMethod != "setWidget" || wg.WidgetKey != "progress" || wg.Key != "widget:progress" || wg.WidgetPlacement != "aboveEditor" || len(wg.WidgetLines) != 2 || wg.Text != "Step 1\nStep 2" {
		t.Fatalf("unexpected setWidget request: %+v", wg)
	}

	ti := waitUpdate(t, events, "setTitle")
	if ti.UIMethod != "setTitle" || ti.Title != "Feynman · Acoustic Study" || ti.Text != "Feynman · Acoustic Study" {
		t.Fatalf("unexpected setTitle request: %+v", ti)
	}

	et := waitUpdate(t, events, "editorText")
	if et.UIMethod != "set_editor_text" || et.Text != "/lit room acoustics" {
		t.Fatalf("unexpected set_editor_text request: %+v", et)
	}

	reply := waitUpdate(t, events, "assistant")
	if reply.Text != "UI answers: select=Docker confirm=true input=AudioSet editor=Revised abstract" {
		t.Fatalf("unexpected UI summary reply: %q", reply.Text)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("prompt failed: %v", err)
		}
	case <-researchTestHangGuard(t):
		t.Fatal("hung waiting for extension UI prompt completion")
	}
}

func TestResearchRefusesWorkspaceContainingAppDataBeforeScaffolding(t *testing.T) {
	root := t.TempDir()
	data := filepath.Join(root, "app-data")
	h := NewResearchManager(fakeWorkspaceLookup{dataDir: data})
	defer h.Close()
	h.command = func(_, _ string) (*exec.Cmd, error) {
		t.Error("command creation started for a workspace containing application data")
		return nil, errors.New("unexpected command creation")
	}
	err := h.connect(WorkspaceRef{Root: root}, func(ResearchUpdate) {})
	if err == nil || !strings.Contains(err.Error(), "application data folder") {
		t.Errorf("connect error = %v, want workspace isolation error", err)
	}
	if _, err := os.Stat(data); !os.IsNotExist(err) {
		t.Errorf("application data scaffold was created: %v", err)
	}
}

func TestResearchConnectConcurrencyAndCrashReconnect(t *testing.T) {
	home := t.TempDir()
	rootA := t.TempDir()
	rootB := t.TempDir()
	data := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("FEYNMAN_HOME", home)

	h := NewResearchManager(fakeWorkspaceLookup{dataDir: data})
	defer h.Close()

	gateA := make(chan struct{})
	releaseA := make(chan struct{})
	baseCmd := fixtureCommand(home)
	h.command = func(root, work string) (*exec.Cmd, error) {
		if fsutil.SameRoot(root, rootA) {
			close(gateA)
			<-releaseA
		}
		return baseCmd(root, work)
	}

	errA := make(chan error, 1)
	go func() {
		errA <- h.connect(WorkspaceRef{Root: rootA}, func(ResearchUpdate) {})
	}()
	<-gateA

	errB := make(chan error, 1)
	go func() {
		errB <- h.connect(WorkspaceRef{Root: rootB}, func(ResearchUpdate) {})
	}()

	select {
	case err := <-errB:
		if err != nil {
			close(releaseA)
			t.Fatalf("second workspace connect failed while first was starting: %v", err)
		}
	case <-researchTestHangGuard(t):
		close(releaseA)
		t.Fatal("second workspace connect blocked on global research lock while first workspace ran command startup")
	}

	close(releaseA)
	if err := <-errA; err != nil {
		t.Fatalf("first workspace connect failed: %v", err)
	}

	events := make(chan ResearchUpdate, 64)
	_ = h.Disconnect(context.Background(), rootA)
	h.command = baseCmd
	if err := h.connect(WorkspaceRef{Root: rootA}, func(e ResearchUpdate) { events <- e }); err != nil {
		t.Fatalf("reconnect failed: %v", err)
	}
	waitUpdate(t, events, "commands")
	crashCleaned := make(chan struct{})
	var crashCleanup sync.Once
	h.SetAfterStop(func(root, _ string) {
		if fsutil.SameRoot(root, rootA) {
			crashCleanup.Do(func() { close(crashCleaned) })
		}
	})
	if _, err := h.Prompt(context.Background(), rootA, "crash mid-turn", "", ""); err == nil {
		t.Fatal("expected crashed turn to return error")
	}
	select {
	case <-crashCleaned:
	case <-researchTestHangGuard(t):
		t.Fatal("hung waiting for crashed Feynman cleanup")
	}
	errEvt := waitUpdate(t, events, "error")
	if !strings.Contains(errEvt.Text, "disconnected") {
		t.Fatalf("expected disconnect error event, got %+v", errEvt)
	}
	if err := h.connect(WorkspaceRef{Root: rootA}, func(e ResearchUpdate) { events <- e }); err != nil {
		t.Fatalf("reconnect after crash failed: %v", err)
	}
	waitUpdate(t, events, "commands")
}

func TestResearchConnectWaitsForCrashCleanup(t *testing.T) {
	home := t.TempDir()
	root := t.TempDir()
	h := NewResearchManager(fakeWorkspaceLookup{dataDir: t.TempDir()})
	defer h.Close()
	h.command = fixtureCommand(home)

	cleanupStarted := make(chan struct{})
	releaseCleanup := make(chan struct{})
	release := sync.OnceFunc(func() { close(releaseCleanup) })
	defer release()
	var firstCleanup sync.Once
	h.SetAfterStop(func(_, _ string) {
		firstCleanup.Do(func() {
			close(cleanupStarted)
			<-releaseCleanup
		})
	})

	if err := h.connect(WorkspaceRef{Root: root}, func(ResearchUpdate) {}); err != nil {
		t.Fatalf("connect failed: %v", err)
	}
	if _, err := h.Prompt(context.Background(), root, "crash mid-turn", "", ""); err == nil {
		t.Fatal("expected crashed turn to return error")
	}
	<-cleanupStarted

	reconnected := make(chan error, 1)
	go func() {
		reconnected <- h.connect(WorkspaceRef{Root: root}, func(ResearchUpdate) {})
	}()
	select {
	case err := <-reconnected:
		release()
		t.Fatalf("reconnect returned before crash cleanup completed: %v", err)
	case <-time.After(50 * time.Millisecond):
	}

	release()
	select {
	case err := <-reconnected:
		if err != nil {
			t.Fatalf("reconnect after crash cleanup failed: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("reconnect did not finish after crash cleanup completed")
	}
}

func TestResearchListsSessionsWhileConnecting(t *testing.T) {
	home := t.TempDir()
	root := t.TempDir()
	h := NewResearchManager(fakeWorkspaceLookup{dataDir: t.TempDir()})
	defer h.Close()

	commandReady := make(chan struct{})
	start := make(chan struct{})
	h.command = func(root, work string) (*exec.Cmd, error) {
		cmd, err := fixtureCommand(home)(root, work)
		close(commandReady)
		<-start
		return cmd, err
	}
	connected := make(chan error, 1)
	go func() {
		connected <- h.connect(WorkspaceRef{Root: root}, func(ResearchUpdate) {})
	}()
	<-commandReady
	close(start)
	for range 32 {
		if _, err := h.Sessions(context.Background(), root); err != nil {
			t.Fatal(err)
		}
	}
	if err := <-connected; err != nil {
		t.Fatal(err)
	}
}

func TestResearchStopWhileConnectingReapsChildAndUnblocks(t *testing.T) {
	home := t.TempDir()
	root := t.TempDir()
	h := NewResearchManager(fakeWorkspaceLookup{dataDir: t.TempDir()})
	defer h.Close()

	gate := make(chan struct{})
	release := make(chan struct{})
	spawned := make(chan *exec.Cmd, 1)
	baseCmd := fixtureCommand(home)
	h.command = func(r, work string) (*exec.Cmd, error) {
		close(gate)
		<-release
		cmd, err := baseCmd(r, work)
		if err != nil {
			return nil, err
		}
		spawned <- cmd
		return cmd, nil
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- h.connect(WorkspaceRef{Root: root}, func(ResearchUpdate) {})
	}()

	<-gate
	closed := make(chan struct{})
	go func() {
		h.Close()
		close(closed)
	}()
	time.Sleep(30 * time.Millisecond)
	close(release)

	select {
	case <-closed:
	case <-time.After(3 * time.Second):
		t.Fatal("ResearchManager.Close hung while connect was starting")
	}
	select {
	case err := <-errCh:
		if err == nil {
			t.Fatal("connect should fail when host closed mid-startup")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("connect did not return after close")
	}
	select {
	case cmd := <-spawned:
		if cmd.Process != nil && cmd.ProcessState == nil {
			t.Fatal("spawned child process was not reaped after connect aborted")
		}
	default:
	}
}

func TestResearchWindowCloseWhileStarting(t *testing.T) {
	home := t.TempDir()
	root := t.TempDir()
	h := NewResearchManager(fakeWorkspaceLookup{dataDir: t.TempDir()})
	defer h.Close()
	watch := make(chan func(), 1)
	h.onWindowClosed = func(_ *mygo.Window, closed func()) func() {
		watch <- closed
		return func() {}
	}
	commandReady := make(chan *exec.Cmd, 1)
	release := make(chan struct{})
	releaseCommand := sync.OnceFunc(func() { close(release) })
	defer releaseCommand()
	h.command = func(root, work string) (*exec.Cmd, error) {
		cmd, err := fixtureCommand(home)(root, work)
		commandReady <- cmd
		<-release
		return cmd, err
	}
	released := make(chan string, 1)
	h.SetAfterStop(func(root, _ string) { released <- root })
	done := make(chan error, 1)
	go func() {
		done <- h.connect(WorkspaceRef{Root: root, Window: new(mygo.Window)}, func(ResearchUpdate) {})
	}()
	cmd := <-commandReady
	var closeWindow func()
	select {
	case closeWindow = <-watch:
	default:
		t.Fatal("window close is not observed during command startup")
	}
	closeWindow()
	releaseCommand()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	select {
	case <-ctx.Done():
		t.Fatal("connect did not return after window close")
	case err := <-done:
		if err == nil {
			t.Fatal("connect succeeded after window close")
		}
	}
	select {
	case <-ctx.Done():
		t.Fatal("window teardown did not complete during startup")
	case got := <-released:
		if got != fsutil.CanonicalPath(root) {
			t.Fatalf("cleaned up workspace %q, want %q", got, root)
		}
	}
	if cmd.Process != nil {
		t.Fatal("started the Feynman process after window close")
	}
}

func fixtureCommand(home string) func(root, work string) (*exec.Cmd, error) {
	return func(_, work string) (*exec.Cmd, error) {
		sessionsDir := filepath.Join(home, ".feynman", "sessions")
		_ = os.MkdirAll(sessionsDir, 0700)
		cmd := exec.Command(os.Args[0], "--mode", "rpc", "--session-dir", sessionsDir, "--cwd", work)
		cmd.Dir = work
		cmd.Env = append(os.Environ(),
			"MGO_FIXTURE_FEYNMAN_RPC=1",
			"HOME="+home,
			"FEYNMAN_HOME="+home,
		)
		pirpc.Configure(cmd)
		return cmd, nil
	}
}

func containsArgPair(args []string, flag, val string) bool {
	for i := 0; i+1 < len(args); i++ {
		if args[i] == flag && args[i+1] == val {
			return true
		}
	}
	return false
}

var unclaimed sync.Map

// researchTestHangGuard only turns a broken wait into a failure. Tests pass
// when their event or condition arrives, never because a duration elapsed.
func researchTestHangGuard(t *testing.T) <-chan time.Time {
	t.Helper()
	timer := time.NewTimer(time.Minute)
	t.Cleanup(func() { timer.Stop() })
	return timer.C
}

func waitUpdate(t *testing.T, events <-chan ResearchUpdate, kind string) ResearchUpdate {
	t.Helper()
	held, _ := unclaimed.Load(events)
	earlier, _ := held.([]ResearchUpdate)
	for i, e := range earlier {
		if e.Kind == kind {
			unclaimed.Store(events, append(earlier[:i:i], earlier[i+1:]...))
			return e
		}
	}
	hangGuard := researchTestHangGuard(t)
	for {
		select {
		case e := <-events:
			if e.Kind == kind {
				unclaimed.Store(events, earlier)
				return e
			}
			earlier = append(earlier, e)
		case <-hangGuard:
			unclaimed.Store(events, earlier)
			t.Fatalf("hung waiting for ResearchUpdate kind %q", kind)
		}
	}
}

func drainEvents(events <-chan ResearchUpdate) {
	unclaimed.Delete(events)
	for {
		select {
		case <-events:
		default:
			return
		}
	}
}

type fakeWorkspaceLookup struct {
	dataDir string
}

func (f fakeWorkspaceLookup) WindowWorkspace(_ context.Context, root string) (WorkspaceRef, error) {
	return WorkspaceRef{Root: root}, nil
}

func (f fakeWorkspaceLookup) EditableDocument(context.Context, string, string) error {
	return nil
}

func (f fakeWorkspaceLookup) UserDataDir() string {
	return f.dataDir
}

func runFeynmanRPCFixture() {
	cwd, _ := os.Getwd()
	sessionDir := os.Getenv("PI_CODING_AGENT_SESSION_DIR")
	for i := 0; i+1 < len(os.Args); i++ {
		switch os.Args[i] {
		case "--session-dir":
			sessionDir = os.Args[i+1]
		case "--cwd":
			if !fsutil.SameRoot(os.Args[i+1], cwd) {
				os.Exit(3)
			}
		}
	}
	if sessionDir == "" {
		home := os.Getenv("FEYNMAN_HOME")
		if home == "" {
			home, _ = os.UserHomeDir()
		}
		sessionDir = filepath.Join(home, ".feynman", "sessions")
	}
	_ = os.MkdirAll(sessionDir, 0700)

	_, _ = os.Stdout.WriteString("  ██████  Feynman 0.5.71 — The open source AI research agent.\r\n")
	_, _ = os.Stdout.WriteString("non-json banner line before protocol records\r\n")

	var outMu sync.Mutex
	sendRecord := func(v any) {
		data, err := json.Marshal(v)
		if err != nil {
			return
		}
		outMu.Lock()
		_, _ = os.Stdout.Write(append(data, '\r', '\n'))
		outMu.Unlock()
	}
	respond := func(id, command string, data any) {
		msg := map[string]any{
			"type":    "response",
			"command": command,
			"success": true,
		}
		if id != "" {
			msg["id"] = id
		}
		if data != nil {
			msg["data"] = data
		}
		sendRecord(msg)
	}
	respondErr := func(id, command, errMsg string) {
		msg := map[string]any{
			"type":    "response",
			"command": command,
			"success": false,
			"error":   errMsg,
		}
		if id != "" {
			msg["id"] = id
		}
		sendRecord(msg)
	}

	var stateMu sync.Mutex
	sessionSeq := 1
	sessionID := "first"
	sessionName := ""
	sessionFile := filepath.Join(sessionDir, "first.jsonl")
	currentModel := ResearchModel{
		ID:            "claude-sonnet-4-5",
		Name:          "Claude Sonnet 4.5",
		API:           "anthropic-messages",
		Provider:      "anthropic",
		Reasoning:     true,
		Input:         []string{"text", "image"},
		ContextWindow: 200000,
		MaxTokens:     16384,
	}
	availableModels := []ResearchModel{
		currentModel,
		{
			ID:            "gpt-5.4",
			Name:          "GPT-5.4",
			API:           "openai-responses",
			Provider:      "openai",
			Reasoning:     true,
			Input:         []string{"text", "image"},
			ContextWindow: 200000,
			MaxTokens:     16384,
		},
		{
			ID:            "hold",
			Name:          "Slow Model",
			Provider:      "local",
			Reasoning:     false,
			Input:         []string{"text"},
			ContextWindow: 32000,
		},
	}
	thinkingLevel := "medium"
	steeringMode := "one-at-a-time"
	followUpMode := "one-at-a-time"
	autoCompaction := true
	isStreaming := false
	var steeringQueue []string
	var followUpQueue []string
	var messages []map[string]any

	writeSessionHeader := func(file, id, parent string) {
		if _, err := os.Stat(file); err == nil {
			return
		}
		hdr := map[string]any{
			"type":      "session",
			"version":   3,
			"id":        id,
			"timestamp": time.Now().UTC().Format(time.RFC3339Nano),
			"cwd":       cwd,
		}
		if parent != "" {
			hdr["parentSession"] = parent
		}
		b, _ := json.Marshal(hdr)
		_ = os.WriteFile(file, append(b, '\n'), 0600)
	}
	appendSessionEntry := func(file string, entry map[string]any) {
		b, _ := json.Marshal(entry)
		f, err := os.OpenFile(file, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
		if err != nil {
			return
		}
		_, _ = f.Write(append(b, '\n'))
		_ = f.Close()
	}

	writeSessionHeader(sessionFile, sessionID, "")
	messages = []map[string]any{
		{
			"role":      "assistant",
			"content":   []map[string]any{{"type": "text", "text": "Created first"}},
			"timestamp": time.Now().UnixMilli(),
		},
	}
	appendSessionEntry(sessionFile, map[string]any{
		"type":      "message",
		"id":        "entry-init",
		"parentId":  nil,
		"timestamp": time.Now().UTC().Format(time.RFC3339Nano),
		"message": map[string]any{
			"role":      "user",
			"content":   "First research prompt",
			"timestamp": time.Now().UnixMilli(),
		},
	})
	appendSessionEntry(sessionFile, map[string]any{
		"type":      "message",
		"id":        "entry-reply",
		"parentId":  "entry-init",
		"timestamp": time.Now().UTC().Format(time.RFC3339Nano),
		"message":   messages[0],
	})

	uiResponses := make(chan map[string]any, 16)
	abortCh := make(chan struct{}, 4)
	steerNotify := make(chan string, 16)

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 0, 64<<10), 32<<20)
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		var raw map[string]any
		if json.Unmarshal(line, &raw) != nil {
			continue
		}
		msgType, _ := raw["type"].(string)
		if msgType == "extension_ui_response" {
			uiResponses <- raw
			continue
		}
		reqID, _ := raw["id"].(string)

		switch msgType {
		case "get_state":
			stateMu.Lock()
			st := map[string]any{
				"model":                 currentModel,
				"thinkingLevel":         thinkingLevel,
				"isStreaming":           isStreaming,
				"isCompacting":          false,
				"steeringMode":          steeringMode,
				"followUpMode":          followUpMode,
				"sessionFile":           sessionFile,
				"sessionId":             sessionID,
				"sessionName":           sessionName,
				"autoCompactionEnabled": autoCompaction,
				"messageCount":          len(messages),
				"pendingMessageCount":   len(steeringQueue) + len(followUpQueue),
			}
			stateMu.Unlock()
			respond(reqID, "get_state", st)

		case "get_commands":
			respond(reqID, "get_commands", map[string]any{
				"commands": []map[string]any{
					{
						"name":        "lit",
						"description": "Literature review",
						"source":      "prompt",
						"input":       map[string]string{"hint": "a topic"},
						"sourceInfo": map[string]string{
							"path":   "/opt/feynman/prompts/lit.md",
							"source": "auto",
							"scope":  "user",
							"origin": "top-level",
						},
					},
					{
						"name":        "skill:paper-search",
						"description": "Search academic papers",
						"source":      "skill",
						"sourceInfo": map[string]string{
							"path":          "/opt/feynman/skills/paper-search/SKILL.md",
							"source":        "package",
							"scope":         "user",
							"origin":        "package",
							"baseDir":       "/opt/feynman/skills/paper-search",
							"changelogPath": "/opt/feynman/CHANGELOG.md",
						},
					},
					{
						"name":        "tools",
						"description": "Inspect tool parameters",
						"source":      "extension",
						"sourceInfo": map[string]string{
							"path":   "/opt/feynman/extensions/research-tools.ts",
							"source": "local",
							"scope":  "user",
							"origin": "top-level",
						},
					},
					{"name": "new", "source": "extension"},
					{"name": "quit", "source": "extension"},
					{"name": "exit", "source": "extension"},
				},
			})

		case "get_available_models":
			respond(reqID, "get_available_models", map[string]any{"models": availableModels})

		case "get_available_thinking_levels":
			respond(reqID, "get_available_thinking_levels", map[string]any{
				"levels": []string{"off", "minimal", "low", "medium", "high", "xhigh", "max"},
			})

		case "set_model":
			provider, _ := raw["provider"].(string)
			modelID, _ := raw["modelId"].(string)
			if modelID == "hold" {
				sendRecord(map[string]any{
					"type": "message_end",
					"message": map[string]any{
						"role":    "assistant",
						"content": []map[string]any{{"type": "text", "text": "Changing model"}},
					},
				})
				continue
			}
			var found *ResearchModel
			for i := range availableModels {
				if availableModels[i].Provider == provider && availableModels[i].ID == modelID {
					found = &availableModels[i]
					break
				}
			}
			if found == nil {
				respondErr(reqID, "set_model", fmt.Sprintf("Model not found: %s/%s", provider, modelID))
				continue
			}
			stateMu.Lock()
			currentModel = *found
			stateMu.Unlock()
			respond(reqID, "set_model", currentModel)

		case "set_thinking_level":
			lvl, _ := raw["level"].(string)
			stateMu.Lock()
			thinkingLevel = lvl
			stateMu.Unlock()
			sendRecord(map[string]any{"type": "thinking_level_changed", "level": lvl})
			respond(reqID, "set_thinking_level", nil)

		case "set_steering_mode":
			mode, _ := raw["mode"].(string)
			stateMu.Lock()
			steeringMode = mode
			stateMu.Unlock()
			respond(reqID, "set_steering_mode", nil)

		case "set_follow_up_mode":
			mode, _ := raw["mode"].(string)
			stateMu.Lock()
			followUpMode = mode
			stateMu.Unlock()
			respond(reqID, "set_follow_up_mode", nil)

		case "set_auto_compaction":
			en, _ := raw["enabled"].(bool)
			stateMu.Lock()
			autoCompaction = en
			stateMu.Unlock()
			respond(reqID, "set_auto_compaction", nil)

		case "get_session_stats":
			stateMu.Lock()
			sid := sessionID
			sfile := sessionFile
			stateMu.Unlock()
			respond(reqID, "get_session_stats", map[string]any{
				"sessionFile":       sfile,
				"sessionId":         sid,
				"userMessages":      2,
				"assistantMessages": 2,
				"toolCalls":         1,
				"toolResults":       1,
				"totalMessages":     6,
				"tokens": map[string]any{
					"input":      1000,
					"output":     300,
					"cacheRead":  200,
					"cacheWrite": 0,
					"total":      1500,
				},
				"cost": 0.0125,
				"contextUsage": map[string]any{
					"tokens":        1500,
					"contextWindow": 200000,
					"percent":       0.75,
				},
			})

		case "compact":
			instr, _ := raw["customInstructions"].(string)
			respond(reqID, "compact", map[string]any{
				"summary":              "Compacted conversation (" + instr + ")",
				"firstKeptEntryId":     "entry-init",
				"tokensBefore":         50000,
				"estimatedTokensAfter": 12000,
			})

		case "get_messages":
			stateMu.Lock()
			msgs := slices.Clone(messages)
			stateMu.Unlock()
			respond(reqID, "get_messages", map[string]any{"messages": msgs})

		case "new_session":
			parent, _ := raw["parentSession"].(string)
			stateMu.Lock()
			sessionSeq++
			if sessionSeq == 2 {
				sessionID = "next"
			} else {
				sessionID = fmt.Sprintf("session-%d", sessionSeq)
			}
			sessionName = ""
			sessionFile = filepath.Join(sessionDir, sessionID+".jsonl")
			writeSessionHeader(sessionFile, sessionID, parent)
			messages = []map[string]any{
				{
					"role":      "assistant",
					"content":   []map[string]any{{"type": "text", "text": "Created " + sessionID}},
					"timestamp": time.Now().UnixMilli(),
				},
			}
			stateMu.Unlock()
			respond(reqID, "new_session", map[string]any{"cancelled": false})

		case "switch_session":
			path, _ := raw["sessionPath"].(string)
			if strings.Contains(path, "broken") {
				if _, err := os.Stat(filepath.Join(cwd, "pause-load")); err == nil {
					_ = os.WriteFile(filepath.Join(cwd, "loading"), nil, 0600)
					continue
				}
				respondErr(reqID, "switch_session", "Session file is corrupted or unavailable")
				continue
			}
			hdr, name, loadedMsgs, err := readFixtureSessionFile(path)
			if err != nil {
				respondErr(reqID, "switch_session", err.Error())
				continue
			}
			stateMu.Lock()
			sessionID = hdr.ID
			sessionName = name
			sessionFile = path
			if len(loadedMsgs) > 0 {
				messages = loadedMsgs
			} else {
				messages = []map[string]any{
					{
						"role":      "assistant",
						"content":   []map[string]any{{"type": "text", "text": "Restored " + hdr.ID}},
						"timestamp": time.Now().UnixMilli(),
					},
				}
			}
			stateMu.Unlock()
			respond(reqID, "switch_session", map[string]any{"cancelled": false})

		case "set_session_name":
			name, _ := raw["name"].(string)
			name = strings.TrimSpace(name)
			if name == "" {
				respondErr(reqID, "set_session_name", "Session name cannot be empty")
				continue
			}
			stateMu.Lock()
			sessionName = name
			sfile := sessionFile
			stateMu.Unlock()
			appendSessionEntry(sfile, map[string]any{
				"type":      "session_info",
				"id":        newRandomID()[:8],
				"parentId":  "entry-reply",
				"timestamp": time.Now().UTC().Format(time.RFC3339Nano),
				"name":      name,
			})
			sendRecord(map[string]any{"type": "session_info_changed", "name": name})
			respond(reqID, "set_session_name", nil)

		case "get_fork_messages":
			respond(reqID, "get_fork_messages", map[string]any{
				"messages": []map[string]string{
					{"entryId": "entry-init", "text": "First research prompt"},
					{"entryId": "entry-2", "text": "Second research prompt"},
				},
			})

		case "fork":
			entryID, _ := raw["entryId"].(string)
			if entryID == "" {
				respondErr(reqID, "fork", "Missing entryId")
				continue
			}
			stateMu.Lock()
			parentFile := sessionFile
			sessionSeq++
			sessionID = fmt.Sprintf("forked-%d", sessionSeq)
			sessionName = "Fork of " + entryID
			sessionFile = filepath.Join(sessionDir, sessionID+".jsonl")
			writeSessionHeader(sessionFile, sessionID, parentFile)
			appendSessionEntry(sessionFile, map[string]any{
				"type":      "session_info",
				"id":        "fork-info",
				"parentId":  nil,
				"timestamp": time.Now().UTC().Format(time.RFC3339Nano),
				"name":      sessionName,
			})
			messages = []map[string]any{
				{
					"role":      "user",
					"content":   "First research prompt",
					"timestamp": time.Now().UnixMilli(),
				},
			}
			stateMu.Unlock()
			respond(reqID, "fork", map[string]any{
				"text":      "First research prompt",
				"cancelled": false,
			})

		case "clone":
			stateMu.Lock()
			parentFile := sessionFile
			sessionSeq++
			sessionID = fmt.Sprintf("cloned-%d", sessionSeq)
			sessionFile = filepath.Join(sessionDir, sessionID+".jsonl")
			writeSessionHeader(sessionFile, sessionID, parentFile)
			stateMu.Unlock()
			respond(reqID, "clone", map[string]any{"cancelled": false})

		case "get_tree":
			respond(reqID, "get_tree", map[string]any{
				"tree": []map[string]any{
					{
						"entry": map[string]any{
							"type":      "message",
							"id":        "entry-init",
							"timestamp": "2026-11-19T09:00:00Z",
							"message": map[string]any{
								"role":    "user",
								"content": "First research prompt",
							},
						},
						"label": "initial-question",
						"children": []map[string]any{
							{
								"entry": map[string]any{
									"type":      "message",
									"id":        "entry-reply",
									"parentId":  "entry-init",
									"timestamp": "2026-11-19T09:00:05Z",
									"message": map[string]any{
										"role":    "assistant",
										"content": []map[string]any{{"type": "text", "text": "Created first"}},
									},
								},
								"children": []any{},
							},
						},
					},
				},
				"leafId": "entry-reply",
			})

		case "steer":
			msg, _ := raw["message"].(string)
			stateMu.Lock()
			steeringQueue = append(steeringQueue, msg)
			q := ResearchQueueState{Steering: slices.Clone(steeringQueue), FollowUp: slices.Clone(followUpQueue)}
			stateMu.Unlock()
			sendRecord(map[string]any{"type": "queue_update", "steering": q.Steering, "followUp": q.FollowUp})
			respond(reqID, "steer", map[string]any{"disposition": "queued"})
			select {
			case steerNotify <- msg:
			default:
			}

		case "follow_up":
			msg, _ := raw["message"].(string)
			stateMu.Lock()
			followUpQueue = append(followUpQueue, msg)
			q := ResearchQueueState{Steering: slices.Clone(steeringQueue), FollowUp: slices.Clone(followUpQueue)}
			stateMu.Unlock()
			sendRecord(map[string]any{"type": "queue_update", "steering": q.Steering, "followUp": q.FollowUp})
			respond(reqID, "follow_up", map[string]any{"disposition": "queued"})

		case "clear_queue":
			stateMu.Lock()
			cleared := map[string]any{
				"steering": slices.Clone(steeringQueue),
				"followUp": slices.Clone(followUpQueue),
			}
			steeringQueue = nil
			followUpQueue = nil
			stateMu.Unlock()
			sendRecord(map[string]any{"type": "queue_update", "steering": []string{}, "followUp": []string{}})
			respond(reqID, "clear_queue", cleared)

		case "abort":
			select {
			case abortCh <- struct{}{}:
			default:
			}
			respond(reqID, "abort", nil)

		case "prompt":
			promptMsg, _ := raw["message"].(string)
			if strings.Contains(promptMsg, "crash mid-turn") {
				respond(reqID, "prompt", map[string]any{"disposition": "started"})
				sendRecord(map[string]any{"type": "agent_start"})
				os.Exit(2)
			}
			respond(reqID, "prompt", map[string]any{"disposition": "started"})
			stateMu.Lock()
			isStreaming = true
			stateMu.Unlock()

			go func(msg string) {
				defer func() {
					stateMu.Lock()
					isStreaming = false
					stateMu.Unlock()
				}()

				sendRecord(map[string]any{"type": "agent_start"})
				sendRecord(map[string]any{"type": "turn_start"})

				switch {
				case strings.Contains(msg, "large payload test"):
					bigText := strings.Repeat("A", (1<<20)+512*1024)
					sendRecord(map[string]any{
						"type": "message_start",
						"message": map[string]any{
							"role":       "assistant",
							"responseId": "msg-large",
							"stopReason": "pending",
						},
					})
					sendRecord(map[string]any{
						"type": "message_update",
						"assistantMessageEvent": map[string]any{
							"type":         "text_delta",
							"contentIndex": 0,
							"delta":        bigText,
						},
					})
					sendRecord(map[string]any{
						"type": "message_end",
						"message": map[string]any{
							"role":       "assistant",
							"responseId": "msg-large",
							"content":    []map[string]any{{"type": "text", "text": bigText}},
							"stopReason": "stop",
						},
					})

				case strings.Contains(msg, "stream full lifecycle"):
					sendRecord(map[string]any{
						"type": "message_start",
						"message": map[string]any{
							"role":       "assistant",
							"responseId": "msg-stream-1",
							"stopReason": "pending",
						},
					})
					sendRecord(map[string]any{
						"type": "message_update",
						"assistantMessageEvent": map[string]any{
							"type":         "thinking_delta",
							"contentIndex": 0,
							"delta":        "Analyzing ",
						},
					})
					sendRecord(map[string]any{
						"type": "message_update",
						"assistantMessageEvent": map[string]any{
							"type":         "thinking_delta",
							"contentIndex": 0,
							"delta":        "primary sources.",
						},
					})
					sendRecord(map[string]any{
						"type":       "tool_execution_start",
						"toolCallId": "call-read-1",
						"toolName":   "read",
						"args":       map[string]any{"path": "notes.md", "line": 4},
					})
					sendRecord(map[string]any{
						"type":       "tool_execution_update",
						"toolCallId": "call-read-1",
						"toolName":   "read",
						"args":       map[string]any{"path": "notes.md", "line": 4},
						"partialResult": map[string]any{
							"content": []map[string]any{{"type": "text", "text": "partial notes"}},
						},
					})
					const pixel = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAIAAACQd1PeAAAADElEQVR4nGNgSPsPAAHPAWYvlgrbAAAAAElFTkSuQmCC"
					sendRecord(map[string]any{
						"type":       "tool_execution_end",
						"toolCallId": "call-read-1",
						"toolName":   "read",
						"isError":    false,
						"durationMs": 18,
						"result": map[string]any{
							"content": []map[string]any{
								{"type": "text", "text": "final notes"},
								{"type": "image", "mimeType": "image/png", "data": pixel},
							},
							"details": map[string]any{
								"diffs": []map[string]string{
									{"path": "notes.md", "oldText": "First", "newText": "Second"},
								},
							},
						},
					})
					sendRecord(map[string]any{
						"type":         "auto_retry_start",
						"attempt":      1,
						"maxAttempts":  3,
						"delayMs":      100,
						"errorMessage": "529 overloaded",
					})
					sendRecord(map[string]any{
						"type":    "auto_retry_end",
						"success": true,
						"attempt": 2,
					})
					sendRecord(map[string]any{
						"type":   "auto_compaction_start",
						"reason": "threshold",
					})
					sendRecord(map[string]any{
						"type":   "auto_compaction_end",
						"reason": "threshold",
						"result": map[string]any{
							"summary":              "Compacted older context",
							"firstKeptEntryId":     "entry-init",
							"tokensBefore":         80000,
							"estimatedTokensAfter": 20000,
						},
						"aborted":   false,
						"willRetry": false,
					})
					sendRecord(map[string]any{
						"type": "message_update",
						"assistantMessageEvent": map[string]any{
							"type":         "text_delta",
							"contentIndex": 1,
							"delta":        "Synthesized ",
						},
					})
					sendRecord(map[string]any{
						"type": "message_update",
						"assistantMessageEvent": map[string]any{
							"type":         "text_delta",
							"contentIndex": 1,
							"delta":        "findings.",
						},
					})
					sendRecord(map[string]any{
						"type": "message_end",
						"message": map[string]any{
							"role":       "assistant",
							"responseId": "msg-stream-1",
							"content": []map[string]any{
								{"type": "thinking", "thinking": "Analyzing primary sources."},
								{"type": "text", "text": "Synthesized findings."},
							},
							"stopReason": "stop",
						},
					})

				case strings.Contains(msg, "wait for steer and follow_up"):
					for {
						s := <-steerNotify
						if s == "release turn" {
							break
						}
					}
					stateMu.Lock()
					steered := strings.Join(steeringQueue, "; ")
					followed := strings.Join(followUpQueue, "; ")
					steeringQueue = nil
					followUpQueue = nil
					stateMu.Unlock()
					replyText := fmt.Sprintf("Steered: %s | FollowUp: %s", steered, followed)
					sendRecord(map[string]any{
						"type": "message_end",
						"message": map[string]any{
							"role":       "assistant",
							"responseId": "msg-steered",
							"content":    []map[string]any{{"type": "text", "text": replyText}},
							"stopReason": "stop",
						},
					})

				case strings.Contains(msg, "exercise all 9 extension ui"):
					sendRecord(map[string]any{
						"type":    "extension_ui_request",
						"id":      "ui-select-1",
						"method":  "select",
						"title":   "Choose environment",
						"options": []string{"Local", "Docker"},
					})
					r1 := <-uiResponses
					val1, _ := r1["value"].(string)

					sendRecord(map[string]any{
						"type":    "extension_ui_request",
						"id":      "ui-confirm-2",
						"method":  "confirm",
						"title":   "Run benchmark?",
						"message": "Takes 2 minutes.",
					})
					r2 := <-uiResponses
					conf2, _ := r2["confirmed"].(bool)

					sendRecord(map[string]any{
						"type":        "extension_ui_request",
						"id":          "ui-input-3",
						"method":      "input",
						"title":       "Dataset name",
						"placeholder": "e.g. LibriSpeech",
					})
					r3 := <-uiResponses
					val3, _ := r3["value"].(string)

					sendRecord(map[string]any{
						"type":    "extension_ui_request",
						"id":      "ui-editor-4",
						"method":  "editor",
						"title":   "Edit abstract",
						"prefill": "Draft abstract",
					})
					r4 := <-uiResponses
					val4, _ := r4["value"].(string)

					sendRecord(map[string]any{
						"type":       "extension_ui_request",
						"id":         "ui-notify-5",
						"method":     "notify",
						"message":    "Rate limit approaching",
						"notifyType": "warning",
					})

					sendRecord(map[string]any{
						"type":       "extension_ui_request",
						"id":         "ui-status-6",
						"method":     "setStatus",
						"statusKey":  "indexer",
						"statusText": "Indexing 42 papers",
					})

					sendRecord(map[string]any{
						"type":            "extension_ui_request",
						"id":              "ui-widget-7",
						"method":          "setWidget",
						"widgetKey":       "progress",
						"widgetLines":     []string{"Step 1", "Step 2"},
						"widgetPlacement": "aboveEditor",
					})

					sendRecord(map[string]any{
						"type":   "extension_ui_request",
						"id":     "ui-title-8",
						"method": "setTitle",
						"title":  "Feynman · Acoustic Study",
					})

					sendRecord(map[string]any{
						"type":   "extension_ui_request",
						"id":     "ui-editor-text-9",
						"method": "set_editor_text",
						"text":   "/lit room acoustics",
					})

					summary := fmt.Sprintf("UI answers: select=%s confirm=%v input=%s editor=%s", val1, conf2, val3, val4)
					sendRecord(map[string]any{
						"type": "message_end",
						"message": map[string]any{
							"role":       "assistant",
							"responseId": "msg-ui-summary",
							"content":    []map[string]any{{"type": "text", "text": summary}},
							"stopReason": "stop",
						},
					})

				case strings.Contains(msg, "cancel this"):
					sendRecord(map[string]any{
						"type":    "extension_ui_request",
						"id":      "ui-cancel-1",
						"method":  "select",
						"title":   "Save the fixture?",
						"options": []string{"Accept", "Reject"},
					})
					select {
					case <-abortCh:
						sendRecord(map[string]any{"type": "turn_end"})
						sendRecord(map[string]any{"type": "agent_end", "messages": []any{}, "willRetry": false})
						sendRecord(map[string]any{"type": "agent_settled", "aborted": true})
						return
					case r := <-uiResponses:
						if cancelled, _ := r["cancelled"].(bool); cancelled {
							select {
							case <-abortCh:
								sendRecord(map[string]any{"type": "turn_end"})
								sendRecord(map[string]any{"type": "agent_end", "messages": []any{}, "willRetry": false})
								sendRecord(map[string]any{"type": "agent_settled", "aborted": true})
								return
							case <-time.After(100 * time.Millisecond):
							}
						}
					}

				case strings.Contains(msg, "mcp-propose-edit-check"):
					sendRecord(map[string]any{
						"type": "message_end",
						"message": map[string]any{
							"role":       "assistant",
							"responseId": "msg-mcp-check",
							"content":    []map[string]any{{"type": "text", "text": msg}},
							"stopReason": "stop",
						},
					})
				}

				sendRecord(map[string]any{"type": "turn_end", "toolResults": []any{}})
				sendRecord(map[string]any{"type": "agent_end", "messages": []any{}, "willRetry": false})
				if msg == "write after agent_end" {
					time.Sleep(100 * time.Millisecond)
					if err := os.WriteFile("late.md", []byte("# Late artifact\n"), 0600); err != nil {
						os.Exit(2)
					}
				}
				sendRecord(map[string]any{"type": "agent_settled", "aborted": false})
			}(promptMsg)
		}
	}
}

type fixtureSessionHeader struct {
	Type          string `json:"type"`
	ID            string `json:"id"`
	Cwd           string `json:"cwd"`
	Timestamp     string `json:"timestamp"`
	ParentSession string `json:"parentSession"`
}

func readFixtureSessionFile(path string) (fixtureSessionHeader, string, []map[string]any, error) {
	f, err := os.Open(path)
	if err != nil {
		return fixtureSessionHeader{}, "", nil, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64<<10), 4<<20)
	var hdr fixtureSessionHeader
	var gotHeader bool
	var name string
	var msgs []map[string]any
	for sc.Scan() {
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 {
			continue
		}
		var entry struct {
			Type          string         `json:"type"`
			ID            string         `json:"id"`
			Cwd           string         `json:"cwd"`
			Timestamp     string         `json:"timestamp"`
			ParentSession string         `json:"parentSession"`
			Name          string         `json:"name"`
			Message       map[string]any `json:"message"`
		}
		if json.Unmarshal(line, &entry) != nil {
			continue
		}
		if !gotHeader {
			if entry.Type != "session" || entry.ID == "" {
				return fixtureSessionHeader{}, "", nil, fmt.Errorf("invalid session header in %s", path)
			}
			hdr = fixtureSessionHeader{
				Type:          entry.Type,
				ID:            entry.ID,
				Cwd:           entry.Cwd,
				Timestamp:     entry.Timestamp,
				ParentSession: entry.ParentSession,
			}
			gotHeader = true
			continue
		}
		switch entry.Type {
		case "session_info":
			name = strings.TrimSpace(entry.Name)
		case "message":
			if entry.Message != nil {
				msgs = append(msgs, entry.Message)
			}
		}
	}
	if !gotHeader {
		return fixtureSessionHeader{}, "", nil, fmt.Errorf("empty session file %s", path)
	}
	return hdr, name, msgs, nil
}

func TestResearchManagerBeforeStartAndAfterStopHooks(t *testing.T) {
	home := t.TempDir()
	root := t.TempDir()
	data := t.TempDir()

	var events []string
	var seenRoot, seenWork string
	h := NewResearchManager(fakeWorkspaceLookup{dataDir: data})
	defer h.Close()
	h.command = fixtureCommand(home)
	h.SetBeforeStart(func(r, w string) error {
		seenRoot = r
		seenWork = w
		events = append(events, "beforeStart")
		return nil
	})
	h.SetAfterStop(func(r, w string) {
		if r != seenRoot || w != seenWork {
			t.Errorf("afterStop got (%q, %q), want (%q, %q)", r, w, seenRoot, seenWork)
		}
		events = append(events, "afterStop")
	})

	if err := h.connect(WorkspaceRef{Root: root}, func(ResearchUpdate) {}); err != nil {
		t.Fatalf("connect failed: %v", err)
	}
	if !slices.Equal(events, []string{"beforeStart"}) {
		t.Fatalf("events after connect = %v, want [beforeStart]", events)
	}
	if err := h.Disconnect(context.Background(), root); err != nil {
		t.Fatalf("Disconnect failed: %v", err)
	}
	if !slices.Equal(events, []string{"beforeStart", "afterStop"}) {
		t.Fatalf("events after Disconnect = %v, want [beforeStart afterStop]", events)
	}
}

func TestEmitStatsUpdateConcurrentWithSetModelRace(t *testing.T) {
	home := t.TempDir()
	root := t.TempDir()
	data := t.TempDir()

	h := NewResearchManager(fakeWorkspaceLookup{dataDir: data})
	defer h.Close()
	h.command = fixtureCommand(home)
	if err := h.connect(WorkspaceRef{Root: root}, func(ResearchUpdate) {}); err != nil {
		t.Fatalf("connect failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	for i := range 4 {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for j := range 15 {
				if (worker+j)%2 == 0 {
					_, _ = h.Host().SessionStats(ctx, root)
				} else {
					_, _ = h.Host().SetModel(ctx, root, "openai", "gpt-5.4")
				}
			}
		}(i)
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		h.mu.Lock()
		s := h.sessions[fsutil.CanonicalPath(root)]
		h.mu.Unlock()
		stats := ResearchSessionStats{Tokens: ResearchTokenStats{Total: 42}}
		for range 100 {
			h.emitStatsUpdate(s, stats)
		}
	}()
	wg.Wait()
}

func TestSteerAndFollowUpForwardNonImageSourceURIs(t *testing.T) {
	home := t.TempDir()
	root := t.TempDir()
	data := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("FEYNMAN_HOME", home)

	h := NewResearchManager(fakeWorkspaceLookup{dataDir: data})
	defer h.Close()
	h.command = fixtureCommand(home)
	events := make(chan ResearchUpdate, 128)
	if err := h.connect(WorkspaceRef{Root: root}, func(e ResearchUpdate) { events <- e }); err != nil {
		t.Fatalf("connect failed: %v", err)
	}
	drainEvents(events)

	paths := []string{filepath.Join(root, "notes.md"), filepath.Join(root, "paper.pdf")}
	for i, path := range paths {
		if err := os.WriteFile(path, []byte(fmt.Sprintf("source %d", i)), 0600); err != nil {
			t.Fatal(err)
		}
	}
	approved, err := selectedResearchFiles(paths)
	if err != nil {
		t.Fatal(err)
	}
	h.mu.Lock()
	h.sessions[fsutil.CanonicalPath(root)].sources = approved
	h.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := h.Host().Prompt(ctx, root, "wait for steer and follow_up", "", "")
		done <- err
	}()
	waitUpdate(t, events, "progress")
	if err := h.Host().Steer(ctx, root, "Review these sources", paths); err != nil {
		t.Fatalf("Steer failed: %v", err)
	}
	steered := waitUpdate(t, events, "queue").Queue
	if steered == nil || len(steered.Steering) != 1 {
		t.Fatalf("steering queue = %+v", steered)
	}
	if err := h.Host().FollowUp(ctx, root, "Read the same sources", paths); err != nil {
		t.Fatalf("FollowUp failed: %v", err)
	}
	followed := waitUpdate(t, events, "queue").Queue
	if followed == nil || len(followed.FollowUp) != 1 {
		t.Fatalf("follow-up queue = %+v", followed)
	}
	for _, message := range []string{steered.Steering[0], followed.FollowUp[0]} {
		if !strings.Contains(message, "Attached source files:\n") {
			t.Errorf("RPC message has no attachment section: %q", message)
		}
		for _, path := range paths {
			uri := (&url.URL{Scheme: "file", Path: filepath.ToSlash(path)}).String()
			if !strings.Contains(message, uri) {
				t.Errorf("RPC message %q does not include %q", message, uri)
			}
		}
	}
	if err := h.Host().Steer(ctx, root, "release turn", nil); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatalf("Prompt failed: %v", err)
	}
}

func TestPromptWithSourcesUsesMCPProposeEditInsteadOfStaging(t *testing.T) {
	home := t.TempDir()
	root := t.TempDir()
	data := t.TempDir()

	events := make(chan ResearchUpdate, 64)
	h := NewResearchManager(fakeWorkspaceLookup{dataDir: data})
	defer h.Close()
	h.command = fixtureCommand(home)
	if err := h.connect(WorkspaceRef{Root: root}, func(e ResearchUpdate) { events <- e }); err != nil {
		t.Fatalf("connect failed: %v", err)
	}
	drainEvents(events)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	buf := "# Intro\n\nOriginal paragraph.\n"
	got, err := h.Host().PromptWithSources(ctx, root, "mcp-propose-edit-check: tighten intro", "doc-42", buf, nil)
	if err != nil {
		t.Fatalf("PromptWithSources failed: %v", err)
	}
	if got != buf {
		t.Fatalf("PromptWithSources returned %q, want unchanged buffer %q", got, buf)
	}

	var sawPrompt string
	deadline := time.After(3 * time.Second)
	for sawPrompt == "" {
		select {
		case ev := <-events:
			if ev.Kind == "assistant" && strings.Contains(ev.Text, "mcp-propose-edit-check") {
				sawPrompt = ev.Text
			}
		case <-deadline:
			t.Fatal("timed out waiting for assistant echo of prompt")
		}
	}
	if !strings.Contains(sawPrompt, "propose_edit") || !strings.Contains(sawPrompt, "doc-42") {
		t.Fatalf("prompt did not direct agent to MCP propose_edit for doc-42: %q", sawPrompt)
	}
	if strings.Contains(sawPrompt, "current-document.md") {
		t.Fatalf("prompt still referenced legacy current-document.md staging: %q", sawPrompt)
	}
	stagedPath := filepath.Join(review.WorkDir(data, root), review.ResearchDocumentCopy)
	if _, err := os.Stat(stagedPath); !os.IsNotExist(err) {
		t.Fatalf("legacy staged file %s should not exist, err=%v", stagedPath, err)
	}
}
