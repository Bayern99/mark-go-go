package feynman

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Bayern99/mark-go-go/internal/fsutil"
)

func cliFixtureCommand(home, mode string) func(ctx context.Context, work string, args ...string) (*exec.Cmd, error) {
	return func(ctx context.Context, work string, args ...string) (*exec.Cmd, error) {
		cmd := exec.CommandContext(ctx, os.Args[0], args...)
		if work != "" {
			cmd.Dir = work
		}
		cmd.Env = append(os.Environ(),
			"MGO_FIXTURE_FEYNMAN_CLI=1",
			"MGO_FIXTURE_CLI_MODE="+mode,
			"HOME="+home,
			"FEYNMAN_HOME="+home,
		)
		configureResearchProcess(cmd)
		cmd.Cancel = func() error {
			stopResearchProcess(cmd)
			return nil
		}
		return cmd, nil
	}
}

func TestFeynmanClearsOnlyTheSelectedWebSearchCredential(t *testing.T) {
	const before = `{
		"provider": "auto", "searchProvider": "brave", "route": "gemini",
		"exaApiKey": "fake-exa", "braveApiKey": "fake-brave",
		"workflow": "summary-review", "allowBrowserCookies": true,
		"futurePreference": {"quota": 9223372036854775807}
	}`
	for _, tc := range []struct {
		name            string
		prefix          string
		linked          bool
		wrongConfigPath bool
	}{
		{name: "private profile"},
		{name: "UTF-8 BOM", prefix: "\ufeff"},
		{name: "linked profile", linked: true},
		{name: "CLI reports a different config path", wrongConfigPath: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.linked && runtime.GOOS == "windows" {
				t.Skip("symbolic links require Developer Mode or privileges on Windows")
			}
			ctx := context.Background()
			home, root := fsutil.CanonicalPath(t.TempDir()), t.TempDir()
			t.Setenv("HOME", home)
			t.Setenv("USERPROFILE", home)
			h := NewResearchManager(fakeWorkspaceLookup{dataDir: t.TempDir()})
			defer h.Close()
			h.cliCommand = cliFixtureCommand(home, "search-key")
			appPath := filepath.Join(h.profileHome(), ".feynman", "agent", "web-search.json")
			path := appPath
			if tc.linked {
				path = filepath.Join(home, ".feynman", "agent", "web-search.json")
			}
			if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
				t.Fatal(err)
			}
			input := tc.prefix + before
			if err := os.WriteFile(path, []byte(input), 0640); err != nil {
				t.Fatal(err)
			}
			info, err := os.Stat(path)
			if err != nil {
				t.Fatal(err)
			}
			if tc.wrongConfigPath {
				t.Setenv("MGO_FIXTURE_SEARCH_STATUS_PATH", filepath.Join(t.TempDir(), "web-search.json"))
			}
			result, clearErr := h.CLISearchAction(ctx, root, "clear-key", "exa", "")
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if tc.wrongConfigPath {
				if clearErr == nil || string(data) != input {
					t.Fatalf("expected rejection without changes when CLI reports another file, got %v", clearErr)
				}
				return
			}
			if clearErr != nil || result.ExitCode != 0 {
				t.Fatalf("clear Exa: exit %d (%v)", result.ExitCode, clearErr)
			}
			after, err := h.CLISearchStatus(ctx, root)
			if err != nil || after.SavedAPIKeys != "brave" || after.RequestRoute != "auto" || after.SearchRoute != "brave" ||
				after.SearchWorkflow != "summary-review" || !strings.Contains(after.RawOutput, "Gemini browser fallback: enabled") {
				t.Fatalf("clearing Exa should preserve other Web Search settings: %+v (%v)", after, err)
			}
			var config map[string]json.RawMessage
			if !bytes.HasPrefix(data, []byte(tc.prefix)) {
				t.Fatal("clearing a key removed the UTF-8 BOM")
			}
			if err := json.Unmarshal(data[len(tc.prefix):], &config); err != nil {
				t.Fatal(err)
			}
			if config["exaApiKey"] != nil || string(config["braveApiKey"]) != `"fake-brave"` || string(config["route"]) != `"gemini"` ||
				!strings.Contains(string(config["futurePreference"]), "9223372036854775807") {
				t.Fatal("clearing a key changed an unrelated Web Search config field")
			}
			if tc.linked {
				if link, err := os.Readlink(appPath); err != nil || link != path {
					t.Fatalf("app profile link was replaced: %q (%v)", link, err)
				}
			}
			if current, err := os.Stat(path); err != nil || current.Mode().Perm() != info.Mode().Perm() {
				t.Fatalf("file permissions changed: %v (%v)", current, err)
			}
		})
	}
}

func TestFeynmanInstalledCLIOutputFormats(t *testing.T) {
	t.Run("Pi MCP state", func(t *testing.T) {
		result := parseMCPListOutput(`{"servers":[{"name":"papers","scope":"project","state":"disabled"}]}`, 0)
		if !result.Parsed || len(result.Servers) != 1 || result.Servers[0].Status != "disabled" || result.Servers[0].Scope != "project" {
			t.Fatalf("Pi state was lost: %+v", result)
		}
	})
	t.Run("grouped alpha search", func(t *testing.T) {
		for mode, raw := range map[string]string{
			"both": `{"semantic":[{"title":"Semantic paper","paperId":"2501.01234","link":"https://www.alphaxiv.org/abs/2501.01234"}],"keyword":{"papers":[]}}`,
			"all":  `{"semantic":[],"keyword":[],"agentic":{"results":[{"id":"2502.01234","title":"Agentic paper"}]}}`,
		} {
			result := parseAlphaSearchOutput(raw, 0, "attention", mode)
			if !result.Parsed || len(result.Groups) < 2 {
				t.Errorf("%s output not parsed: %+v", mode, result)
				continue
			}
			if mode == "both" {
				semantic, keyword := result.Groups[0], result.Groups[1]
				if semantic.Mode != "semantic" || len(semantic.Papers) != 1 || semantic.Papers[0].ID != "2501.01234" ||
					semantic.Papers[0].URL != "https://www.alphaxiv.org/abs/2501.01234" || keyword.Mode != "keyword" || len(keyword.Papers) != 0 {
					t.Errorf("Both groups: %+v", result.Groups)
				}
			} else if len(result.Groups) != 3 || result.Groups[2].Mode != "agentic" || len(result.Groups[2].Papers) != 1 {
				t.Errorf("All groups: %+v", result.Groups)
			}
		}
	})
	t.Run("multiline annotations", func(t *testing.T) {
		home := t.TempDir()
		h := NewResearchManager(fakeWorkspaceLookup{dataDir: t.TempDir()})
		defer h.Close()
		h.cliCommand = cliFixtureCommand(home, "multiline")
		result, err := h.CLIAlphaAnnotate(context.Background(), t.TempDir(), "list", "", "")
		if err != nil || !result.Parsed || len(result.Annotations) != 2 ||
			result.Annotations[0].PaperID != "2501.01234" || result.Annotations[0].Note != "First line\nSecond line\n\nThird line: detail" ||
			result.Annotations[1].Note != "Another annotation" {
			t.Fatalf("multiline annotations were lost: %+v, %v", result, err)
		}
	})
}

func TestFeynmanCLIStructuredCommandsAndMutations(t *testing.T) {
	home := t.TempDir()
	root := t.TempDir()
	data := t.TempDir()

	h := NewResearchManager(fakeWorkspaceLookup{dataDir: data})
	defer h.Close()
	h.cliCommand = cliFixtureCommand(home, "structured")

	ctx := context.Background()

	status, err := h.CLIStatus(ctx, root)
	if err != nil {
		t.Fatalf("CLIStatus failed: %v", err)
	}
	if !status.Parsed || status.Model != "openai-codex/gpt-6.1-terra" || status.AlphaStatus != "logged in (researcher@example.edu)" {
		t.Fatalf("unexpected CLIStatus: %+v", status)
	}
	if len(status.Sections) < 3 {
		t.Fatalf("expected 3 status sections, got %+v", status.Sections)
	}

	doctor, err := h.CLIDoctor(ctx, root)
	if err != nil {
		t.Fatalf("CLIDoctor failed: %v", err)
	}
	if !doctor.Parsed || !doctor.Healthy || len(doctor.Entries) < 4 {
		t.Fatalf("unexpected CLIDoctor: %+v", doctor)
	}

	preview, err := h.CLISetupPreview(ctx, root)
	if err != nil {
		t.Fatalf("CLISetupPreview failed: %v", err)
	}
	if !preview.Parsed || !strings.Contains(preview.RawOutput, "Preview ready") {
		t.Fatalf("unexpected CLISetupPreview: %+v", preview)
	}

	models, err := h.CLIModelList(ctx, root)
	if err != nil {
		t.Fatalf("CLIModelList failed: %v", err)
	}
	if !models.Parsed || len(models.Models) != 3 || models.CurrentModel != "openai-codex/gpt-6.1-terra" || models.Tier != "priority" {
		t.Fatalf("unexpected CLIModelList: %+v", models)
	}

	for _, tc := range []struct {
		action string
		value  string
		want   string
	}{
		{"set", "anthropic/claude-sonnet-4", "Default model set to anthropic/claude-sonnet-4"},
		{"tier", "flex", "Service tier set to flex"},
		{"logout", "openai-codex", "Cleared auth for openai-codex"},
	} {
		res, err := h.CLIModelAction(ctx, root, tc.action, tc.value)
		if err != nil {
			t.Fatalf("CLIModelAction(%s) failed: %v", tc.action, err)
		}
		if !strings.Contains(res.RawOutput, tc.want) {
			t.Fatalf("CLIModelAction(%s) output = %q, want %q", tc.action, res.RawOutput, tc.want)
		}
	}

	searchSt, err := h.CLISearchStatus(ctx, root)
	if err != nil {
		t.Fatalf("CLISearchStatus failed: %v", err)
	}
	if !searchSt.Parsed || !searchSt.Configured || searchSt.SearchRoute != "Exa" {
		t.Fatalf("unexpected CLISearchStatus: %+v", searchSt)
	}

	searchSet, err := h.CLISearchAction(ctx, root, "set", "exa", "exa-secret-123")
	if err != nil || !strings.Contains(searchSet.RawOutput, "exa") {
		t.Fatalf("CLISearchAction(set) failed: %+v, err=%v", searchSet, err)
	}
	searchClear, err := h.CLISearchAction(ctx, root, "clear", "", "")
	if err != nil || !strings.Contains(searchClear.RawOutput, "auto") {
		t.Fatalf("CLISearchAction(clear) failed: %+v, err=%v", searchClear, err)
	}

	mcpList, err := h.CLIMCPList(ctx, root)
	if err != nil {
		t.Fatalf("CLIMCPList failed: %v", err)
	}
	if !mcpList.Parsed || len(mcpList.Servers) != 1 || mcpList.Servers[0].Name != "zotero" {
		t.Fatalf("unexpected CLIMCPList: %+v", mcpList)
	}

	mcpAdd, err := h.CLIMCPAdd(ctx, root, CLIMCPAddInput{
		Name:              "arxiv-mcp",
		Transport:         "http",
		URL:               "http://127.0.0.1:8123/mcp",
		Headers:           []string{"X-Api-Key=abc"},
		BearerTokenEnvVar: "ARXIV_TOKEN",
		Exposure:          "direct",
		Description:       "Paper index",
	})
	if err != nil || !strings.Contains(mcpAdd.RawOutput, "Added arxiv-mcp") {
		t.Fatalf("CLIMCPAdd failed: %+v, err=%v", mcpAdd, err)
	}
	mcpRemove, err := h.CLIMCPRemove(ctx, root, "arxiv-mcp", false)
	if err != nil || !strings.Contains(mcpRemove.RawOutput, "Removed arxiv-mcp") {
		t.Fatalf("CLIMCPRemove failed: %+v, err=%v", mcpRemove, err)
	}

	pkgs, err := h.CLIPackagesList(ctx, root)
	if err != nil {
		t.Fatalf("CLIPackagesList failed: %v", err)
	}
	if !pkgs.Parsed || len(pkgs.Packages) != 4 {
		t.Fatalf("unexpected CLIPackagesList: %+v", pkgs)
	}
	for _, tc := range []struct {
		action string
		target string
		want   string
	}{
		{"install", "memory", "Installed memory"},
		{"remove", "memory", "Removed memory"},
		{"update", "", "Updated optional packages"},
	} {
		res, err := h.CLIPackageAction(ctx, root, tc.action, tc.target)
		if err != nil || !strings.Contains(res.RawOutput, tc.want) {
			t.Fatalf("CLIPackageAction(%s) failed: %+v, err=%v", tc.action, res, err)
		}
	}

	alphaSt, err := h.CLIAlphaStatus(ctx, root)
	if err != nil || !alphaSt.Parsed || !alphaSt.LoggedIn {
		t.Fatalf("CLIAlphaStatus failed: %+v, err=%v", alphaSt, err)
	}
	alphaSearch, err := h.CLIAlphaSearch(ctx, root, "sparse attention", "semantic")
	if err != nil || !alphaSearch.Parsed || len(alphaSearch.Papers) != 2 || alphaSearch.Papers[0].ID != "2501.01234" {
		t.Fatalf("CLIAlphaSearch failed: %+v, err=%v", alphaSearch, err)
	}
	alphaGet, err := h.CLIAlphaGet(ctx, root, "2501.01234", true)
	if err != nil || !alphaGet.Parsed || !strings.Contains(alphaGet.Content, "Sparse Attention") {
		t.Fatalf("CLIAlphaGet failed: %+v, err=%v", alphaGet, err)
	}
	alphaAsk, err := h.CLIAlphaAsk(ctx, root, "2501.01234", "What is the complexity?")
	if err != nil || !strings.Contains(alphaAsk.RawOutput, "O(n log n)") {
		t.Fatalf("CLIAlphaAsk failed: %+v, err=%v", alphaAsk, err)
	}
	alphaCode, err := h.CLIAlphaCode(ctx, root, "https://github.com/example/sparse-attn", "src/attn.py")
	if err != nil || !strings.Contains(alphaCode.RawOutput, "def sparse_attention") {
		t.Fatalf("CLIAlphaCode failed: %+v, err=%v", alphaCode, err)
	}
	annWrite, err := h.CLIAlphaAnnotate(ctx, root, "set", "2501.01234", "Verify theorem 2")
	if err != nil || !annWrite.Parsed || annWrite.Note != "Verify theorem 2" {
		t.Fatalf("CLIAlphaAnnotate(set) failed: %+v, err=%v", annWrite, err)
	}
	annList, err := h.CLIAlphaAnnotate(ctx, root, "list", "", "")
	if err != nil || !annList.Parsed || len(annList.Annotations) != 1 {
		t.Fatalf("CLIAlphaAnnotate(list) failed: %+v, err=%v", annList, err)
	}
	annClear, err := h.CLIAlphaAnnotate(ctx, root, "clear", "2501.01234", "")
	if err != nil || !strings.Contains(annClear.RawOutput, "Cleared") {
		t.Fatalf("CLIAlphaAnnotate(clear) failed: %+v, err=%v", annClear, err)
	}
	alphaOut, err := h.CLIAlphaLogout(ctx, root)
	if err != nil || !strings.Contains(alphaOut.RawOutput, "Logged out") {
		t.Fatalf("CLIAlphaLogout failed: %+v, err=%v", alphaOut, err)
	}
}

func TestFeynmanCLIMalformedOutputGracefulFallback(t *testing.T) {
	home := t.TempDir()
	root := t.TempDir()
	data := t.TempDir()

	h := NewResearchManager(fakeWorkspaceLookup{dataDir: data})
	defer h.Close()
	h.cliCommand = cliFixtureCommand(home, "malformed")

	ctx := context.Background()

	status, err := h.CLIStatus(ctx, root)
	if err != nil {
		t.Fatalf("CLIStatus should degrade gracefully, got error: %v", err)
	}
	if status.Parsed || !strings.Contains(status.RawOutput, "UNEXPECTED STATUS FORMAT") {
		t.Fatalf("expected Parsed=false with raw output, got %+v", status)
	}

	doctor, err := h.CLIDoctor(ctx, root)
	if err != nil {
		t.Fatalf("CLIDoctor should degrade gracefully, got error: %v", err)
	}
	if doctor.Parsed || !strings.Contains(doctor.RawOutput, "UNEXPECTED DOCTOR BLOB") {
		t.Fatalf("expected Parsed=false with raw output, got %+v", doctor)
	}

	models, err := h.CLIModelList(ctx, root)
	if err != nil {
		t.Fatalf("CLIModelList should degrade gracefully, got error: %v", err)
	}
	if models.Parsed || !strings.Contains(models.RawOutput, "UNEXPECTED MODEL TABLE") {
		t.Fatalf("expected Parsed=false with raw output, got %+v", models)
	}

	mcp, err := h.CLIMCPList(ctx, root)
	if err != nil {
		t.Fatalf("CLIMCPList should degrade gracefully on invalid JSON, got error: %v", err)
	}
	if mcp.Parsed || !strings.Contains(mcp.RawOutput, "{not-valid-json") {
		t.Fatalf("expected Parsed=false with raw output, got %+v", mcp)
	}

	search, err := h.CLIAlphaSearch(ctx, root, "test", "semantic")
	if err != nil {
		t.Fatalf("CLIAlphaSearch should degrade gracefully, got error: %v", err)
	}
	if search.Parsed || !strings.Contains(search.RawOutput, "weird upstream search output") {
		t.Fatalf("expected Parsed=false with raw output, got %+v", search)
	}
}

func TestFeynmanCLIDoesNotBlockOrDisconnectLiveChatRPC(t *testing.T) {
	home := t.TempDir()
	root := t.TempDir()
	data := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("FEYNMAN_HOME", home)

	h := NewResearchManager(fakeWorkspaceLookup{dataDir: data})
	defer h.Close()
	h.command = fixtureCommand(home)
	h.cliCommand = cliFixtureCommand(home, "structured")

	events := make(chan ResearchUpdate, 128)
	if err := h.connect(WorkspaceRef{Root: root}, func(e ResearchUpdate) { events <- e }); err != nil {
		t.Fatal(err)
	}
	waitUpdate(t, events, "commands")
	drainEvents(events)

	ctx := context.Background()
	promptDone := make(chan error, 1)
	go func() {
		_, err := h.Prompt(ctx, root, "exercise all 9 extension ui", "", "")
		promptDone <- err
	}()
	selReq := waitUpdate(t, events, "select")

	cliDone := make(chan error, 1)
	go func() {
		st, err := h.CLIStatus(ctx, root)
		if err != nil {
			cliDone <- err
			return
		}
		if !st.Parsed {
			cliDone <- fmt.Errorf("expected parsed status")
			return
		}
		_, err = h.CLIMCPList(ctx, root)
		cliDone <- err
	}()

	select {
	case err := <-cliDone:
		if err != nil {
			t.Fatalf("CLI command failed during active Chat turn: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("CLI command blocked on active Chat RPC turn")
	}

	if err := h.Answer(ctx, root, selReq.RequestID, "Docker"); err != nil {
		t.Fatalf("Answer(select) failed after CLI calls: %v", err)
	}
	confReq := waitUpdate(t, events, "confirm")
	if err := h.AnswerConfirm(ctx, root, confReq.RequestID, true, false); err != nil {
		t.Fatalf("AnswerConfirm failed: %v", err)
	}
	inputReq := waitUpdate(t, events, "input")
	if err := h.AnswerText(ctx, root, inputReq.RequestID, "AudioSet", false); err != nil {
		t.Fatalf("AnswerText(input) failed: %v", err)
	}
	editorReq := waitUpdate(t, events, "editor")
	if err := h.AnswerText(ctx, root, editorReq.RequestID, "Revised abstract", false); err != nil {
		t.Fatalf("AnswerText(editor) failed: %v", err)
	}
	select {
	case err := <-promptDone:
		if err != nil {
			t.Fatalf("Chat prompt failed after concurrent CLI calls: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Chat prompt timed out")
	}
}

func TestTerminalWizardOutlivesLaunchContext(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("uses the Linux TERMINAL handoff")
	}
	bin := t.TempDir()
	h := NewResearchManager(fakeWorkspaceLookup{dataDir: t.TempDir()})
	defer h.Close()
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("TERMINAL", filepath.Join(bin, "terminal"))
	output := filepath.Join(bin, "launched")
	t.Setenv("WIZARD_TEST_OUTPUT", output)
	if err := os.WriteFile(filepath.Join(bin, "terminal"), []byte("#!/bin/sh\nsleep 0.05\nshift\nexec \"$@\"\n"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bin, "feynman"), []byte("#!/bin/sh\nprintf '%s|%s|%s\\n' \"$*\" \"$FEYNMAN_HOME\" \"$HOME\" >> \"$WIZARD_TEST_OUTPUT\"\n"), 0700); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	for _, wizard := range []string{"model_login", "alpha_login", "setup"} {
		if _, err := h.LaunchTerminalWizard(context.Background(), root, wizard, ""); err != nil {
			t.Fatal(err)
		}
	}
	for deadline := time.Now().Add(2 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		data, _ := os.ReadFile(output)
		got := string(data)
		profiles := "|" + h.profileHome() + "|" + os.Getenv("HOME") + "\n"
		if strings.Contains(got, "model login"+profiles) &&
			strings.Contains(got, "alpha login"+profiles) &&
			strings.Contains(got, "setup"+profiles) {
			return
		}
	}
	data, _ := os.ReadFile(output)
	t.Fatalf("wizard commands did not survive handoff: %q", data)
}

func TestFeynmanTerminalHandoffAndRefreshModels(t *testing.T) {
	home := t.TempDir()
	root := t.TempDir()
	data := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("FEYNMAN_HOME", home)

	var mu sync.Mutex
	var launched []string

	h := NewResearchManager(fakeWorkspaceLookup{dataDir: data})
	defer h.Close()
	h.command = fixtureCommand(home)
	h.openTerminal = func(_ context.Context, command, profile string) error {
		if want := filepath.Join(fsutil.CanonicalPath(data), "feynman"); profile != want {
			t.Errorf("terminal uses profile %q, want %q", profile, want)
		}
		mu.Lock()
		launched = append(launched, command)
		mu.Unlock()
		return nil
	}

	ctx := context.Background()
	for _, tc := range []struct {
		wizard string
		target string
		want   string
	}{
		{"model_login", "", "feynman model login"},
		{"model_login", "anthropic", "feynman model login anthropic"},
		{"alpha_login", "", "feynman alpha login"},
		{"setup", "", "feynman setup"},
	} {
		res, err := h.LaunchTerminalWizard(ctx, root, tc.wizard, tc.target)
		if err != nil {
			t.Fatalf("LaunchTerminalWizard(%s) failed: %v", tc.wizard, err)
		}
		if !res.Launched || res.Command != tc.want {
			t.Fatalf("LaunchTerminalWizard(%s) = %+v, want command %q", tc.wizard, res, tc.want)
		}
	}

	if _, err := h.LaunchTerminalWizard(ctx, root, "unknown", ""); err == nil {
		t.Fatal("expected error for unknown terminal wizard")
	}

	mu.Lock()
	gotCount := len(launched)
	mu.Unlock()
	if gotCount != 4 {
		t.Fatalf("expected 4 terminal launches, got %d (%v)", gotCount, launched)
	}

	events := make(chan ResearchUpdate, 64)
	if err := h.connect(WorkspaceRef{Root: root}, func(e ResearchUpdate) { events <- e }); err != nil {
		t.Fatal(err)
	}
	waitUpdate(t, events, "config")
	drainEvents(events)

	models, err := h.RefreshModels(ctx, root)
	if err != nil {
		t.Fatalf("RefreshModels failed: %v", err)
	}
	if len(models) == 0 {
		t.Fatal("expected non-empty models from RefreshModels")
	}
	upd := waitUpdate(t, events, "config")
	if len(upd.Models) != len(models) {
		t.Fatalf("expected config update with %d models, got %d", len(models), len(upd.Models))
	}
}

func runFeynmanCLIFixture() {
	mode := os.Getenv("MGO_FIXTURE_CLI_MODE")
	args := os.Args[1:]
	if len(args) == 0 {
		os.Exit(2)
	}
	if mode == "search-key" && args[0] == "search" {
		runSearchKeyFixture(args[1:])
		return
	}
	if mode == "malformed" {
		switch args[0] {
		case "status":
			fmt.Println("UNEXPECTED STATUS FORMAT WITHOUT SECTIONS")
		case "doctor":
			fmt.Println("UNEXPECTED DOCTOR BLOB")
		case "model":
			fmt.Println("UNEXPECTED MODEL TABLE")
		case "mcp":
			fmt.Println("{not-valid-json")
		case "alpha":
			fmt.Println("weird upstream search output")
		default:
			fmt.Println("unrecognized output")
		}
		return
	}

	switch args[0] {
	case "status":
		fmt.Print(`
┌───────────────────────────────────────────────────────┐
│ Feynman Status                                        │
└───────────────────────────────────────────────────────┘

◆ Core
  Model: openai-codex/gpt-6.1-terra
  Model valid: yes
  Authenticated models: 10
  Authenticated providers: 1
  Recommended model: openai-codex/gpt-6.1-terra
  alphaXiv: logged in (researcher@example.edu)
  Web access: pi-web-access (Exa)
  Service tier: priority
  Preview: configured

◆ Paths
  Sessions: ` + filepath.Join(os.Getenv("HOME"), ".feynman", "sessions") + `

◆ Runtime
  Pi runtime: ready
`)
	case "doctor":
		fmt.Print(`
alphaXiv auth: ok
default model: openai-codex/gpt-6.1-terra
default model valid: yes
pi runtime: ok
`)
	case "setup":
		if len(args) > 1 && args[1] == "preview" {
			fmt.Println("Preview ready: pandoc and Chrome verified.")
		}
	case "model":
		if len(args) < 2 {
			os.Exit(2)
		}
		switch args[1] {
		case "list":
			fmt.Print(`
◆ openai-codex
  openai-codex/gpt-6.1-terra (current, recommended)
  openai-codex/gpt-5.6-terra

◆ anthropic
  anthropic/claude-sonnet-4
`)
		case "tier":
			if len(args) == 2 {
				fmt.Println("priority")
			} else {
				fmt.Printf("Service tier set to %s\n", args[2])
			}
		case "set":
			fmt.Printf("Default model set to %s\n", args[2])
		case "logout":
			fmt.Printf("Cleared auth for %s\n", args[2])
		}
	case "search":
		switch args[1] {
		case "status":
			fmt.Print(`
  Managed by: pi-web-access
  Search route: Exa
  Request route: exa
  Search workflow: active
  Saved API keys: exa
  Config path: /tmp/web-search.json
`)
		case "set":
			fmt.Printf("Configured web search provider %s\n", args[2])
		case "clear":
			fmt.Println("Reset web search provider to auto")
		}
	case "mcp":
		switch args[1] {
		case "list":
			_ = json.NewEncoder(os.Stdout).Encode(map[string]any{
				"servers": []map[string]any{
					{
						"name":      "zotero",
						"transport": "stdio",
						"command":   "zotero-mcp",
						"exposure":  "direct",
						"status":    "connected",
						"tools":     []string{"search_library", "get_item"},
					},
				},
				"errors": []string{},
			})
		case "add":
			fmt.Printf("Added %s\n", args[2])
		case "remove":
			fmt.Printf("Removed %s\n", args[2])
		}
	case "packages":
		switch args[1] {
		case "list":
			fmt.Print(`
◆ Core
  pi-subagents
  pi-web-access

◆ Optional
  memory  Research-session preference and correction memory. (installed)
  hindsight  Hindsight-backed research continuity memory.
`)
		case "install":
			fmt.Printf("Installed %s\n", args[2])
		case "remove":
			fmt.Printf("Removed %s\n", args[2])
		}
	case "update":
		fmt.Println("Updated optional packages.")
	case "alpha":
		switch args[1] {
		case "status":
			fmt.Println("Logged in as researcher@example.edu")
		case "logout":
			fmt.Println("Logged out of alphaXiv")
		case "search":
			_ = json.NewEncoder(os.Stdout).Encode(map[string]any{
				"papers": []map[string]any{
					{
						"id":        "2501.01234",
						"title":     "Sparse Attention at Scale",
						"authors":   "A. Researcher, B. Scientist",
						"published": "2025-01-05",
						"url":       "https://alphaxiv.org/abs/2501.01234",
						"summary":   "Sub-quadratic attention routing.",
					},
					{
						"id":        "2502.05678",
						"title":     "Linear Memory Transformers",
						"authors":   "C. Author",
						"published": "2025-02-01",
						"url":       "https://alphaxiv.org/abs/2502.05678",
					},
				},
			})
		case "get":
			_ = json.NewEncoder(os.Stdout).Encode(map[string]any{
				"id":         "2501.01234",
				"title":      "Sparse Attention at Scale",
				"content":    "# Sparse Attention at Scale\nFull text section.",
				"annotation": "Verify theorem 2",
			})
		case "ask":
			fmt.Println("The asymptotic complexity is O(n log n) with block routing.")
		case "code":
			fmt.Println("def sparse_attention(q, k, v):\n    return q @ k.T")
		case "annotate":
			if len(args) >= 3 && args[2] == "--list" {
				if mode == "multiline" {
					fmt.Print("2501.01234 (2025-01-07T12:00:00Z)\n  First line\nSecond line\n\nThird line: detail\n2502.05678 (2025-02-08T13:00:00Z)\n  Another annotation\n")
					return
				}
				fmt.Println("2501.01234: Verify theorem 2")
			} else if len(args) >= 4 && args[2] == "--clear" {
				fmt.Printf("Cleared annotation for %s\n", args[3])
			} else if len(args) >= 4 {
				fmt.Printf("Saved annotation for %s: %s\n", args[2], args[3])
			}
		}
	}
}

func runSearchKeyFixture(args []string) {
	path := filepath.Join(os.Getenv("PI_CODING_AGENT_DIR"), "web-search.json")
	config := map[string]any{}
	if data, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(bytes.TrimPrefix(data, []byte("\ufeff")), &config); err != nil {
			panic(err)
		}
	}
	switch args[0] {
	case "status":
		var saved []string
		for _, provider := range []string{"brave", "exa"} {
			if key, _ := config[provider+"ApiKey"].(string); key != "" {
				saved = append(saved, provider)
			}
		}
		search, _ := config["searchProvider"].(string)
		if search == "" {
			search = "auto"
		}
		request, _ := config["provider"].(string)
		if request == "" {
			request = search
		}
		fallback := "disabled"
		if config["allowBrowserCookies"] == true {
			fallback = "enabled"
		}
		if reported := os.Getenv("MGO_FIXTURE_SEARCH_STATUS_PATH"); reported != "" {
			path = reported
		}
		fmt.Printf("Managed by: pi-web-access\nSearch route: %s\nRequest route: %s\nSearch workflow: %s\nSaved API keys: %s\nGemini browser fallback: %s\nConfig path: %s\n",
			search, request, config["workflow"], strings.Join(saved, ", "), fallback, path)
		return
	case "clear":
		delete(config, "provider")
		delete(config, "searchProvider")
	case "set":
		config["provider"] = args[1]
		config["searchProvider"] = args[1]
		if len(args) == 3 {
			config[args[1]+"ApiKey"] = args[2]
		}
	}
	delete(config, "route")
	config["workflow"] = "none"
	config["allowBrowserCookies"] = false
	data, err := json.Marshal(config)
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		panic(err)
	}
	fmt.Println("Updated Web Search")
}
