package genui

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"time"
	"unicode/utf16"
)

const (
	DefaultGenerationTimeout = 5 * time.Minute
	StatementPreviewInterval = 25 * time.Millisecond
)

// Orchestrator is the protocol-independent half of a generation. It gives every
// turn its own directory with source.md and, for an update, view.json, builds
// the prompt, polls out.json every 25 ms for a streaming preview, then merges,
// validates and commits the candidate. A Runner only ever speaks its agent's
// protocol.
type Orchestrator struct {
	Runner       Runner
	Req          GenerateRequest
	ViewPath     string
	Model        string
	ThoughtLevel string
	Now          func() time.Time
	Timeout      time.Duration
	Validate     func(context.Context, []byte) error
	BeforeWrite  func() error

	// Prompt replaces the default GenerationPrompt and may return a function
	// that assembles the agent's out.json into the candidate.
	Prompt func(context.Context, GenerateRequest, string) (string, func([]byte) ([]byte, error), error)

	// Called in order: OnDir once the turn's directory exists, Artifact for
	// every file worth recording, Observe for validation events.
	OnDir    func(dir string)
	Artifact func(name, path, content string)
	Observe  func(event string, payload any)

	// PreviewInterval overrides the out.json poll. Tests set it; production
	// keeps 25 ms.
	PreviewInterval time.Duration
}

func (o *Orchestrator) Run(ctx context.Context, progress func(GenerateProgress) error) error {
	if progress == nil {
		progress = func(GenerateProgress) error { return nil }
	}
	if o.Runner == nil {
		return AgentError("No agent backend is configured.")
	}
	if o.Now == nil {
		o.Now = time.Now
	}
	if o.Timeout <= 0 {
		o.Timeout = DefaultGenerationTimeout
	}
	var base *BaseView
	switch o.Req.Mode {
	case "full":
	case "update":
		b, err := ReadBaseView(o.ViewPath, o.Req.ReplaceIDs)
		if err != nil {
			return err
		}
		base = b
	default:
		return AgentErrorf("Unknown generation mode %q: only \"full\" and \"update\" are supported.", o.Req.Mode)
	}

	ctx, stop := context.WithTimeoutCause(ctx, o.Timeout, ErrTimedOut)
	defer stop()
	interrupted := func() error {
		if context.Cause(ctx) == ErrTimedOut {
			return TimedOut(o.Timeout)
		}
		return NewFailure(FailCancelled, "Generation was cancelled.", "")
	}

	dir, err := os.MkdirTemp("", "markgogo-genui-")
	if err != nil {
		return AgentErrorf("Could not make the agent's folder: %v.", err)
	}
	defer os.RemoveAll(dir)
	if dir, err = filepath.EvalSymlinks(dir); err != nil {
		return AgentErrorf("Could not make the agent's folder: %v.", err)
	}
	if o.OnDir != nil {
		o.OnDir(dir)
	}
	if err := os.WriteFile(filepath.Join(dir, "source.md"), []byte(o.Req.Text), 0o644); err != nil {
		return AgentErrorf("Could not write the agent's source file: %v.", err)
	}
	o.artifact("source.md", filepath.Join(dir, "source.md"), o.Req.Text)
	if base != nil {
		if err := os.WriteFile(filepath.Join(dir, "view.json"), base.Raw(), 0o644); err != nil {
			return AgentErrorf("Could not write the agent's copy of the view: %v.", err)
		}
	}

	send := &progressSender{send: progress, artifact: o.artifact}
	if err := send.step("starting", "Starting the agent"); err != nil {
		return interrupted()
	}
	var promptText string
	var assembleOutput func([]byte) ([]byte, error)
	if o.Prompt != nil {
		var err error
		if promptText, assembleOutput, err = o.Prompt(ctx, o.Req, dir); err != nil {
			if ctx.Err() != nil {
				return interrupted()
			}
			return AgentErrorf("Could not prepare the controlled reading view: %v.", err)
		}
	}
	if promptText == "" {
		promptText = GenerationPrompt(o.Req, base, dir)
	}
	if assembleOutput == nil {
		assembleOutput = func(data []byte) ([]byte, error) { return data, nil }
	}

	stopPreview := o.startPreview(ctx, dir, base, send)
	result, err := o.Runner.ExecuteTurn(ctx, Turn{
		Dir:          dir,
		Prompt:       promptText,
		Model:        o.Model,
		ThoughtLevel: o.ThoughtLevel,
		Progress:     send.report,
	})
	stopPreview()
	if err != nil {
		if ctx.Err() != nil {
			return interrupted()
		}
		return err
	}
	if err := send.step("checking", "Checking the view"); err != nil {
		return interrupted()
	}
	run := &orchestratorRun{
		orchestrator:   o,
		base:           base,
		reply:          result.Reply,
		model:          result.Model,
		assembleOutput: assembleOutput,
		validate:       o.Validate,
	}
	err = run.collect(ctx, dir)
	if errors.Is(err, ErrCancelled) || ctx.Err() != nil {
		return interrupted()
	}
	return err
}

func (o *Orchestrator) artifact(name, path, content string) {
	if o.Artifact != nil {
		o.Artifact(name, path, content)
	}
}

func (o *Orchestrator) observe(event string, payload any) {
	if o.Observe != nil {
		o.Observe(event, payload)
	}
}

// startPreview streams what the agent has written to out.json so far. It stops
// when the returned function is called, and does nothing for an update, whose
// candidate is a fragment the reader cannot render alone.
func (o *Orchestrator) startPreview(ctx context.Context, dir string, base *BaseView, send *progressSender) func() {
	if base != nil || o.Req.Language != StatementLanguage {
		return func() {}
	}
	interval := o.PreviewInterval
	if interval <= 0 {
		interval = StatementPreviewInterval
	}
	stop, done := make(chan struct{}), make(chan struct{})
	var data []byte
	poll := func() bool {
		path := filepath.Join(dir, "out.json")
		info, err := os.Lstat(path)
		if err != nil || !info.Mode().IsRegular() || info.Size() > MaxViewBytes {
			return true
		}
		chunk, err := os.ReadFile(path)
		if err != nil || bytes.Equal(chunk, data) {
			return true
		}
		data = append(data[:0], chunk...)
		return send.preview(chunk) == nil
	}
	go func() {
		defer close(done)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			if !poll() {
				return
			}
			select {
			case <-ctx.Done():
				return
			case <-stop:
				return
			case <-ticker.C:
			}
		}
	}()
	var once sync.Once
	return func() {
		once.Do(func() {
			close(stop)
			<-done
			// The agent may have written the rest after the last tick: read
			// once more so the reader sees the finished preview rather than
			// whichever tick happened to land last.
			poll()
		})
	}
}

type orchestratorRun struct {
	orchestrator   *Orchestrator
	base           *BaseView
	reply          string
	model          string
	assembleOutput func([]byte) ([]byte, error)
	validate       func(context.Context, []byte) error
}

func (r *orchestratorRun) collect(ctx context.Context, dir string) error {
	outPath := filepath.Join(dir, "out.json")
	fi, err := os.Lstat(outPath)
	if errors.Is(err, fs.ErrNotExist) {
		if reply := strings.TrimSpace(r.reply); reply != "" {
			finished := NewFailure(FailAgentMessage, "The agent finished without writing a view.", HintAgentMessage)
			finished.Detail = CutRunes(reply, MaxDetailRunes)
			return finished
		}
		return r.outputError("The agent did not write out.json.")
	}
	if err != nil {
		return r.outputError("The agent's out.json could not be read.")
	}
	if fi.Mode()&os.ModeSymlink != 0 {
		return r.outputError("The agent's out.json is a symbolic link, not a file.")
	}
	if !fi.Mode().IsRegular() {
		return r.outputError("The agent's out.json is not a regular file.")
	}
	if fi.Size() > MaxViewBytes {
		return r.outputError(fmt.Sprintf("The agent's out.json is %d bytes, more than the %d bytes a view may have.", fi.Size(), MaxViewBytes))
	}
	data, err := os.ReadFile(outPath)
	if err != nil {
		return r.outputError("The agent's out.json could not be read.")
	}
	r.orchestrator.artifact("out.json", outPath, string(data))
	if r.assembleOutput != nil {
		data, err = r.assembleOutput(data)
		if err != nil {
			return r.outputError(fmt.Sprintf("The content slots could not be assembled: %v.", err))
		}
		r.orchestrator.artifact("assembled.json", "", string(data))
	}

	var top map[string]json.RawMessage
	if err := json.Unmarshal(data, &top); err != nil || top == nil {
		return r.outputError("The agent's out.json is not valid JSON.")
	}
	a2ui, ok := top["a2ui"]
	if !ok {
		return r.outputError(`The agent's out.json has no "a2ui".`)
	}
	var components []json.RawMessage
	if err := json.Unmarshal(a2ui, &components); err != nil || len(components) == 0 {
		return r.outputError(`The agent's out.json does not have a non-empty "a2ui" array.`)
	}
	sources, err := ParseSources(top["sources"])
	if err != nil {
		return r.outputError(err.Error())
	}
	if r.base != nil {
		merged, messages, err := r.base.Merge(a2ui, sources)
		if err != nil {
			return r.outputError(err.Error())
		}
		return r.writeView(ctx, merged, messages)
	}
	return r.writeView(ctx, sources, a2ui)
}

func (r *orchestratorRun) viewModel() string {
	if r.base != nil {
		return r.base.Model()
	}
	return r.model
}

func (r *orchestratorRun) writeView(ctx context.Context, sources map[string]AgentSource, a2ui json.RawMessage) error {
	kept := r.base.KeptHashes(a2ui)
	content, _, err := AssembleView(ViewOptions{
		A2UI:        a2ui,
		Sources:     sources,
		Model:       r.viewModel(),
		Request:     r.orchestrator.Req.Request,
		GeneratedAt: r.orchestrator.Now(),
		Hash: func(key string, src AgentSource) string {
			if hash, keep := kept[key]; keep {
				return hash
			}
			hash, _ := r.sectionHash(src)
			return hash
		},
	})
	if err != nil {
		return r.outputError("The agent's out.json could not be turned into a view.")
	}
	if ctx.Err() != nil {
		return ErrCancelled
	}
	r.orchestrator.artifact("candidate", "", string(content))
	r.orchestrator.observe("validation.start", nil)
	var report CandidateReport
	var validationErr error
	if r.validate == nil {
		report, validationErr = CheckCandidate(ctx, content)
	} else {
		validationErr = r.validate(ctx, content)
	}
	if ctx.Err() != nil {
		return ErrCancelled
	}
	if validationErr != nil {
		r.orchestrator.observe("validation.result", map[string]any{"ok": false, "error": validationErr.Error()})
		var ge *GenError
		if errors.As(validationErr, &ge) {
			return r.outputError(ge.Message)
		}
		return r.outputError(validationErr.Error())
	}
	accepted := map[string]any{"ok": true}
	if len(report.Degraded) > 0 {
		accepted["degraded"] = report.Degraded
	}
	r.orchestrator.observe("validation.result", accepted)
	if ctx.Err() != nil {
		return ErrCancelled
	}
	var expect []byte
	if r.base != nil {
		expect = r.base.Raw()
	}
	if r.orchestrator.BeforeWrite != nil {
		if err := r.orchestrator.BeforeWrite(); err != nil {
			return err
		}
	}
	if err := ReplaceView(r.orchestrator.ViewPath, content, expect); err != nil {
		return err
	}
	r.orchestrator.artifact("view", r.orchestrator.ViewPath, string(content))
	return nil
}

func (r *orchestratorRun) sectionHash(src AgentSource) (hash string, ok bool) {
	key := NormalizeKey(src.Heading)
	for _, sec := range r.orchestrator.Req.Sections {
		if sec.Occurrence != src.Occurrence || NormalizeKey(sec.Heading) != key {
			continue
		}
		if h, valid := SectionHashOf(r.orchestrator.Req.Text, sec); valid {
			return h, true
		}
	}
	return "", false
}

func (r *orchestratorRun) outputError(what string) error {
	e := NewFailure(FailInvalidOutput, what, HintInvalidOutput)
	e.Detail = CutRunes(strings.TrimSpace(r.reply), MaxDetailRunes)
	return e
}

// SectionHashOf hashes the text a SectionRef names, in UTF-16 offsets, without
// the ranges the section excludes.
func SectionHashOf(text string, sec SectionRef) (string, bool) {
	var hashed strings.Builder
	at := sec.Start
	for _, cut := range sec.Exclude {
		if cut.Start < at || cut.End < cut.Start || cut.End > sec.End {
			return "", false
		}
		part, ok := sliceUTF16(text, at, cut.Start)
		if !ok {
			return "", false
		}
		hashed.WriteString(part)
		at = cut.End
	}
	part, ok := sliceUTF16(text, at, sec.End)
	if !ok {
		return "", false
	}
	hashed.WriteString(part)
	return SectionHash(hashed.String(), text, sec.End == UTF16Len(text)), true
}

func UTF16Len(s string) int {
	n := 0
	for _, r := range s {
		l := utf16.RuneLen(r)
		if l < 0 {
			l = 1
		}
		n += l
	}
	return n
}

func sliceUTF16(text string, start, end int) (string, bool) {
	if start < 0 || end < start {
		return "", false
	}
	byteStart, byteEnd := -1, -1
	if start == 0 {
		byteStart = 0
	}
	units := 0
	for i, ch := range text {
		if units == start {
			byteStart = i
		}
		if units == end {
			byteEnd = i
			break
		}
		l := utf16.RuneLen(ch)
		if l < 0 {
			l = 1
		}
		units += l
	}
	if byteEnd < 0 {
		if units != end {
			return "", false
		}
		byteEnd = len(text)
	}
	if byteStart < 0 && units == start {
		byteStart = len(text)
	}
	if byteStart < 0 || byteStart > byteEnd {
		return "", false
	}
	return text[byteStart:byteEnd], true
}

type progressSender struct {
	mu          sync.Mutex
	send        func(GenerateProgress) error
	artifact    func(name, path, content string)
	phase       string
	text        string
	lastPreview StatementPreview
	hasPreview  bool
}

func (ps *progressSender) preview(data []byte) error {
	preview := ReadStatementPreview(data)
	if len(preview.Statements) == 0 {
		return nil
	}
	ps.mu.Lock()
	defer ps.mu.Unlock()
	if ps.hasPreview && reflect.DeepEqual(preview, ps.lastPreview) {
		return nil
	}
	ps.lastPreview, ps.hasPreview = preview, true
	if ps.phase != "writing" || ps.text != "Writing the view" {
		ps.phase, ps.text = "writing", "Writing the view"
	}
	if err := ps.send(GenerateProgress{Phase: "writing", Text: "Writing the view", Preview: &preview}); err != nil {
		return ErrCancelled
	}
	return nil
}

func (ps *progressSender) step(phase, text string) error {
	return ps.report(GenerateProgress{Phase: phase, Text: text})
}

func (ps *progressSender) report(p GenerateProgress) error {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	if p.Preview == nil && p.Phase == ps.phase && p.Text == ps.text {
		return nil
	}
	ps.phase, ps.text = p.Phase, p.Text
	if err := ps.send(p); err != nil {
		return ErrCancelled
	}
	return nil
}
