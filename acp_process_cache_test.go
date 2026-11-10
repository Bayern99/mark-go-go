package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

// cacheHarness is one document generating through a fake agent and a
// fresh process cache. Its cleanup closes the cache and stops any fake
// process the test left behind.
type cacheHarness struct {
	host    *documentHost
	dir     string
	docPath string
	logPath string
}

func newCacheHarness(t *testing.T, spec fakeSpec) *cacheHarness {
	t.Helper()
	dir := t.TempDir()
	h := &documentHost{settings: &settingsStore{dir: t.TempDir()}}
	setStored(t, h.settings, "gemini-3.8-flash", "agy")
	docPath := filepath.Join(dir, "a.md")
	hostSession(t, h, "a", docPath)
	h.agentCache = &acpProcessCache{}
	prepare, logPath := fakeAgentJob(t, spec)
	h.prepareJob = prepare
	t.Cleanup(func() {
		h.agentCache.close()
		killFakeProcesses(t, logPath)
	})
	return &cacheHarness{host: h, dir: dir, docPath: docPath, logPath: logPath}
}

func (c *cacheHarness) generate(t *testing.T, request string) error {
	t.Helper()
	return c.host.generate(context.Background(), "a", GenerateRequest{
		Text:    "# Source\n",
		Mode:    "full",
		Request: request,
		Format:  "{}",
	}, func(GenerateProgress) error { return nil })
}

// logPids returns the fake agent process IDs the log names, in order.
func logPids(t *testing.T, path string) []int {
	t.Helper()
	var pids []int
	for _, e := range logEntries(readFakeLog(t, path), "pid") {
		pids = append(pids, int(e["pid"].(float64)))
	}
	return pids
}

func processAlive(pid int) bool { return syscall.Kill(pid, 0) == nil }

// waitForProcessExit waits until pid no longer exists.
func waitForProcessExit(t *testing.T, pid int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if !processAlive(pid) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("process %d is still running", pid)
}

// waitForPromptCount waits until the fake has received count prompts.
func waitForPromptCount(t *testing.T, path string, count int) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if len(logRecv(readFakeLog(t, path), "session/prompt")) >= count {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("the fake never received %d session/prompt messages", count)
}

func TestAgentCacheExpiresIdleProcessAndRestartsOnDemand(t *testing.T) {
	c := newCacheHarness(t, fakeSpec{Out: minimalOut, SessionClose: true})
	c.host.agentCache.idleTimeout = 200 * time.Millisecond

	if err := c.generate(t, "one"); err != nil {
		t.Fatalf("first generation: %v", err)
	}
	pids := logPids(t, c.logPath)
	if len(pids) != 1 {
		t.Fatalf("started %d processes, want one", len(pids))
	}
	if !processAlive(pids[0]) {
		t.Fatalf("the idle process died before its timeout")
	}
	waitForProcessExit(t, pids[0])

	// The expired process is gone from the cache; the next run starts a
	// fresh one on demand.
	if err := c.generate(t, "two"); err != nil {
		t.Fatalf("second generation: %v", err)
	}
	pids = logPids(t, c.logPath)
	if len(pids) != 2 || pids[1] == pids[0] {
		t.Fatalf("processes = %v, want a fresh process after expiry", pids)
	}
	if !processAlive(pids[1]) {
		t.Errorf("the new idle process %d is not alive", pids[1])
	}
}

func TestAgentCacheDiscardsIdleProcessOnEnvironmentChange(t *testing.T) {
	c := newCacheHarness(t, fakeSpec{Out: minimalOut, SessionClose: true})
	base := c.host.prepareJob
	runs := 0
	c.host.prepareJob = func(job *generationJob) {
		base(job)
		runs++
		if runs == 2 {
			job.Env = append(job.Env, "MARKGOGO_FAKE_PROFILE=changed")
		}
	}

	if err := c.generate(t, "one"); err != nil {
		t.Fatalf("first generation: %v", err)
	}
	if err := c.generate(t, "two"); err != nil {
		t.Fatalf("second generation: %v", err)
	}
	pids := logPids(t, c.logPath)
	if len(pids) != 2 {
		t.Fatalf("started %d processes, want a cold process for the changed environment", len(pids))
	}
	if processAlive(pids[0]) {
		t.Errorf("the process of the old environment %d is still running", pids[0])
	}
	if !processAlive(pids[1]) {
		t.Errorf("the process of the current environment %d is not alive", pids[1])
	}
}

func TestAgentCacheDiscardsIdleProcessOnExecutableChange(t *testing.T) {
	c := newCacheHarness(t, fakeSpec{Out: minimalOut, SessionClose: true})
	copyCommand := filepath.Join(t.TempDir(), "agent-copy")
	if err := os.Symlink(os.Args[0], copyCommand); err != nil {
		t.Fatal(err)
	}
	base := c.host.prepareJob
	runs := 0
	c.host.prepareJob = func(job *generationJob) {
		base(job)
		runs++
		if runs == 2 {
			job.AgentCommand = copyCommand
		}
	}

	if err := c.generate(t, "one"); err != nil {
		t.Fatalf("first generation: %v", err)
	}
	if err := c.generate(t, "two"); err != nil {
		t.Fatalf("second generation: %v", err)
	}
	pids := logPids(t, c.logPath)
	if len(pids) != 2 {
		t.Fatalf("started %d processes, want a cold process for the changed executable", len(pids))
	}
	if processAlive(pids[0]) {
		t.Errorf("the process of the old executable %d is still running", pids[0])
	}
	if !processAlive(pids[1]) {
		t.Errorf("the process of the current executable %d is not alive", pids[1])
	}
}

func TestAgentCacheConcurrentRunsRetainOneIdleProcess(t *testing.T) {
	c := newCacheHarness(t, fakeSpec{Out: minimalOut, SessionClose: true, UniqueSessions: true})
	hostSession(t, c.host, "b", filepath.Join(c.dir, "b.md"))
	base := c.host.prepareJob
	started := make(chan struct{}, 2)
	release := make(chan struct{})
	c.host.prepareJob = func(job *generationJob) {
		base(job)
		inner := job.BeforeWrite
		job.BeforeWrite = func() error {
			started <- struct{}{}
			<-release
			if inner != nil {
				return inner()
			}
			return nil
		}
	}

	run := func(id, request string) <-chan error {
		ch := make(chan error, 1)
		go func() {
			ch <- c.host.generate(context.Background(), id, GenerateRequest{Text: "# Source\n", Mode: "full", Request: request, Format: "{}"}, func(GenerateProgress) error { return nil })
		}()
		return ch
	}
	aCh := run("a", "alpha")
	bCh := run("b", "beta")
	for range 2 {
		select {
		case <-started:
		case <-time.After(10 * time.Second):
			t.Fatal("both concurrent runs never reached the write gate")
		}
	}
	close(release)
	if err := <-aCh; err != nil {
		t.Fatalf("A: %v", err)
	}
	if err := <-bCh; err != nil {
		t.Fatalf("B: %v", err)
	}

	pids := logPids(t, c.logPath)
	if len(pids) != 2 {
		t.Fatalf("started %d processes, want two independent cold processes", len(pids))
	}
	if alive := countAlive(pids); alive != 1 {
		t.Fatalf("alive processes after concurrent runs = %d, want exactly one idle", alive)
	}
	if err := c.generate(t, "three"); err != nil {
		t.Fatalf("third generation: %v", err)
	}
	if got := logPids(t, c.logPath); len(got) != 2 {
		t.Errorf("started %d processes; the third run must reuse the idle one", len(got))
	}
	if alive := countAlive(pids); alive != 1 {
		t.Errorf("alive processes after reuse = %d, want one", alive)
	}
}

func countAlive(pids []int) int {
	alive := 0
	for _, pid := range pids {
		if processAlive(pid) {
			alive++
		}
	}
	return alive
}

func TestAgentCacheCancelAcknowledgedClosesSessionAndReusesProcess(t *testing.T) {
	c := newCacheHarness(t, fakeSpec{
		Hang:               true,
		AnswerCancel:       true,
		CancelReplyDelayMS: 10,
		SessionClose:       true,
		UniqueSessions:     true,
	})
	for run := 1; run <= 2; run++ {
		ctx, cancel := context.WithCancel(context.Background())
		errCh := make(chan error, 1)
		go func() {
			errCh <- c.host.generate(ctx, "a", GenerateRequest{Text: "# Source\n", Mode: "full", Request: "stop", Format: "{}"}, func(GenerateProgress) error { return nil })
		}()
		waitForPromptCount(t, c.logPath, run)
		started := time.Now()
		cancel()
		var err error
		select {
		case err = <-errCh:
		case <-time.After(10 * time.Second):
			t.Fatal("the cancelled generation never returned")
		}
		if elapsed := time.Since(started); elapsed > 1500*time.Millisecond {
			t.Errorf("cancellation %d took %s, want a bounded acknowledgement", run, elapsed)
		}
		if f := failureFrom(t, err); f.Kind != failCancelled {
			t.Fatalf("failure = %+v, want cancelled", f)
		}
	}

	log := readFakeLog(t, c.logPath)
	if got := len(logRecv(log, "session/prompt")); got != 2 {
		t.Errorf("session/prompt = %d, want two cancelled turns", got)
	}
	closes := logRecv(log, "session/close")
	if len(closes) != 2 || closes[0]["sessionId"] != "fake-session-1" || closes[1]["sessionId"] != "fake-session-2" {
		t.Errorf("session/close = %v, want both cancelled sessions closed", closes)
	}
	pids := logPids(t, c.logPath)
	if len(pids) != 1 {
		t.Fatalf("processes = %v, want one reused process", pids)
	}
	if !processAlive(pids[0]) {
		t.Errorf("the process was not returned to the cache after the acknowledged cancels")
	}
}

func TestAgentCacheCloseFailureDiscardsProcessAfterEndTurn(t *testing.T) {
	c := newCacheHarness(t, fakeSpec{
		Out:          minimalOut,
		SessionClose: true,
		Errors:       map[string]fakeRPCError{"session/close": {Code: -32603, Message: "cannot close the session"}},
	})
	if err := c.generate(t, "keep the view"); err != nil {
		t.Fatalf("generation: %v", err)
	}
	view, err := os.ReadFile(viewPath(c.docPath))
	if err != nil || !bytes.Contains(view, []byte(`"request": "keep the view"`)) {
		t.Errorf("view = %s, %v; want the valid end-turn candidate written", view, err)
	}
	pids := logPids(t, c.logPath)
	if len(pids) != 1 {
		t.Fatalf("processes = %v, want one", pids)
	}
	if processAlive(pids[0]) {
		t.Errorf("a process whose session could not be closed must be discarded")
	}
}

func TestAgentCacheEmptySessionIDDiscardsProcess(t *testing.T) {
	c := newCacheHarness(t, fakeSpec{Out: minimalOut, EmptySessionID: true})
	err := c.generate(t, "never")
	if err == nil {
		t.Fatal("the run accepted an empty session ID")
	}
	if f := failureFrom(t, err); f.Kind != failAgentError || !strings.Contains(f.Message, "session ID") {
		t.Errorf("failure = %+v, want an agent-error about the missing session ID", f)
	}
	log := readFakeLog(t, c.logPath)
	if prompts := logRecv(log, "session/prompt"); len(prompts) != 0 {
		t.Errorf("session/prompt = %v; the empty session must not run", prompts)
	}
	pids := logPids(t, c.logPath)
	if len(pids) != 1 || processAlive(pids[0]) {
		t.Errorf("processes = %v, want the discarded process", pids)
	}
}

func TestAgentCacheCancelDuringHandshakeDiscardsProcess(t *testing.T) {
	for _, method := range []string{"initialize", "session/new"} {
		t.Run(method, func(t *testing.T) {
			c := newCacheHarness(t, fakeSpec{Out: minimalOut, Silent: method})
			ctx, cancel := context.WithCancel(context.Background())
			errCh := make(chan error, 1)
			go func() {
				errCh <- c.host.generate(ctx, "a", GenerateRequest{Text: "# Source\n", Mode: "full", Format: "{}"}, func(GenerateProgress) error { return nil })
			}()
			waitForLog(t, c.logPath, "silent")
			started := time.Now()
			cancel()
			var err error
			select {
			case err = <-errCh:
			case <-time.After(10 * time.Second):
				t.Fatal("the cancelled generation never returned")
			}
			if elapsed := time.Since(started); elapsed > 2*time.Second {
				t.Errorf("handshake cancellation took %s", elapsed)
			}
			if f := failureFrom(t, err); f.Kind != failCancelled {
				t.Errorf("failure = %+v, want cancelled", f)
			}
			pids := logPids(t, c.logPath)
			if len(pids) != 1 || processAlive(pids[0]) {
				t.Errorf("processes = %v, want the unquiesced process discarded", pids)
			}
		})
	}
}

func TestHostCloseAgentCacheStopsIdleProcessAndFolder(t *testing.T) {
	c := newCacheHarness(t, fakeSpec{Out: minimalOut, SessionClose: true})
	if err := c.generate(t, "one"); err != nil {
		t.Fatalf("generation: %v", err)
	}
	pids := logPids(t, c.logPath)
	if len(pids) != 1 || !processAlive(pids[0]) {
		t.Fatalf("processes = %v, want one live idle process", pids)
	}
	processCwd, _ := logEntries(readFakeLog(t, c.logPath), "cwd")[0]["cwd"].(string)

	c.host.closeAgentCache()
	waitForProcessExit(t, pids[0])
	if _, err := os.Stat(processCwd); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("the private process folder %q still exists: %v", processCwd, err)
	}
}

func TestHostGenerateIgnoresForeignLateSessionEvents(t *testing.T) {
	c := newCacheHarness(t, fakeSpec{
		Out:                  minimalOut,
		SessionClose:         true,
		UniqueSessions:       true,
		ForeignSessionEvents: true,
	})
	type recorded struct {
		event   string
		payload any
	}
	var mu sync.Mutex
	var traces [][]recorded
	base := c.host.prepareJob
	c.host.prepareJob = func(job *generationJob) {
		base(job)
		mu.Lock()
		traces = append(traces, nil)
		index := len(traces) - 1
		mu.Unlock()
		job.Observe = func(event string, payload any) {
			mu.Lock()
			traces[index] = append(traces[index], recorded{event, payload})
			mu.Unlock()
		}
	}

	if err := c.generate(t, "one"); err != nil {
		t.Fatalf("first generation: %v", err)
	}
	if err := c.generate(t, "two"); err != nil {
		t.Fatalf("second generation: %v", err)
	}
	waitForLog(t, c.logPath, "foreign-permission-answer")

	log := readFakeLog(t, c.logPath)
	answers := logEntries(log, "foreign-permission-answer")
	if len(answers) != 1 || answers[0]["outcome"] != "cancelled" {
		t.Errorf("foreign permission answers = %v, want one cancelled", answers)
	}
	if got := len(logRecv(log, "session/prompt")); got != 2 {
		t.Errorf("session/prompt = %d, want both runs completed", got)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(traces) != 2 {
		t.Fatalf("traces = %d, want one per run", len(traces))
	}
	for _, e := range traces[1] {
		payload, ok := e.payload.(*traceACPPayload)
		if !ok || payload.Direction != acpReceived {
			continue
		}
		var msg struct {
			Method string `json:"method"`
			Params struct {
				SessionID string `json:"sessionId"`
			} `json:"params"`
		}
		if json.Unmarshal(payload.Line, &msg) != nil {
			continue
		}
		if msg.Params.SessionID == "fake-session-1" {
			t.Errorf("the reused run's trace kept a late old-session line: %s", payload.Line)
		}
	}
}
