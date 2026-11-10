package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"strings"
	"testing"
	"time"
)

// rejectedValidation is the external verdict of a page that refuses a
// candidate: what the real validator says when the SDK finds the view
// invalid.
func rejectedValidation(context.Context, []byte) error {
	return newFailure(failInvalidOutput, "The generated view is not valid: UNALLOWED_CHILD: Container 'card' (Card) cannot contain child 'tabs' (Tabs).", hintInvalidOutput)
}

// TestGenerateValidationRejectsFullCandidateAndKeepsViewAndUndo drives a
// whole-view run whose candidate the external validator refuses. Neither
// the current view nor the view kept for Undo may change.
func TestGenerateValidationRejectsFullCandidateAndKeepsViewAndUndo(t *testing.T) {
	viewPath, sentinel := writeSentinelView(t)
	earlier := []byte("the earlier view")
	if err := os.WriteFile(prevViewPath(viewPath), earlier, 0o644); err != nil {
		t.Fatal(err)
	}

	err, _, _ := driveGeneration(t, fakeSpec{Out: minimalOut}, nil, func(job *generationJob) {
		job.ViewPath = viewPath
		job.Validate = rejectedValidation
	})
	if f := failureFrom(t, err); f.Kind != failInvalidOutput || !strings.Contains(f.Message, "UNALLOWED_CHILD") {
		t.Errorf("failure = %+v, want the validator's invalid-output verdict", f)
	}
	sentinelKept(t, viewPath, sentinel)
	if kept, err := os.ReadFile(prevViewPath(viewPath)); err != nil || !bytes.Equal(kept, earlier) {
		t.Errorf("kept view = %q, %v; want the earlier view untouched", kept, err)
	}
}

func TestGenerateValidationMissingValidatorKeepsViewAndUndo(t *testing.T) {
	const earlier = "the earlier view"
	viewPath := writeBase(t, earlier)
	err, _, _ := driveGeneration(t, fakeSpec{Out: minimalOut}, nil, func(job *generationJob) {
		job.ViewPath = viewPath
		job.Validate = nil
	})
	if f := failureFrom(t, err); f.Kind != failInvalidOutput {
		t.Errorf("failure = %+v, want invalid-output", f)
	}
	sentinelKept(t, viewPath, []byte(baseViewText))
	if kept, err := os.ReadFile(prevViewPath(viewPath)); err != nil || string(kept) != earlier {
		t.Errorf("kept view = %q, %v; want the earlier view untouched", kept, err)
	}
}

// TestGenerateValidationRejectsUpdatedCandidateAndKeepsViewAndUndo
// does the same for an updated view: the full merged candidate is what
// the validator sees, and a refusal must not overwrite either the view
// or its Undo backup.
func TestGenerateValidationRejectsUpdatedCandidateAndKeepsViewAndUndo(t *testing.T) {
	earlier := "the earlier view"
	viewPath := writeBase(t, earlier)

	err, _, _ := driveGeneration(t, fakeSpec{Out: updateOut}, nil, func(job *generationJob) {
		updateJob(viewPath, "b")(job)
		job.Validate = rejectedValidation
	})
	if f := failureFrom(t, err); f.Kind != failInvalidOutput || !strings.Contains(f.Message, "UNALLOWED_CHILD") {
		t.Errorf("failure = %+v, want the validator's invalid-output verdict", f)
	}
	sentinelKept(t, viewPath, []byte(baseViewText))
	if kept, err := os.ReadFile(prevViewPath(viewPath)); err != nil || string(kept) != earlier {
		t.Errorf("kept view = %q, %v; want the earlier view untouched", kept, err)
	}
}

// TestGenerateValidationCommitsExactMergedCandidate checks the seam's
// success path on an update: the validator sees the whole stamped view
// file, including the base messages the update is merged into, and that
// exact candidate is what is written and kept for Undo.
func TestGenerateValidationCommitsExactMergedCandidate(t *testing.T) {
	viewPath := writeBase(t, "")
	var candidate []byte
	err, _, _ := driveGeneration(t, fakeSpec{Out: updateOut}, nil, func(job *generationJob) {
		updateJob(viewPath, "b")(job)
		job.Validate = func(_ context.Context, full []byte) error {
			candidate = append([]byte(nil), full...)
			return nil
		}
	})
	if err != nil {
		t.Fatalf("runGeneration: %v", err)
	}
	if len(candidate) == 0 {
		t.Fatal("the validator never saw the candidate")
	}

	written, err := os.ReadFile(viewPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(candidate, written) {
		t.Errorf("validated candidate differs from the written view:\nvalidated: %s\nwritten: %s", candidate, written)
	}
	var view struct {
		Version     int               `json:"version"`
		GeneratedAt string            `json:"generatedAt"`
		A2UI        []json.RawMessage `json:"a2ui"`
	}
	if err := json.Unmarshal(candidate, &view); err != nil {
		t.Fatalf("parse candidate: %v", err)
	}
	if view.Version != 2 || view.GeneratedAt != fakeNow.UTC().Format(time.RFC3339) || len(view.A2UI) != 2 {
		t.Errorf("candidate = %s, want the stamped view with both messages", candidate)
	}
	if !strings.Contains(string(candidate), `"Old A"`) || !strings.Contains(string(candidate), `"updateComponents"`) {
		t.Errorf("candidate = %s, want the base view merged with the update", candidate)
	}
	if kept, err := os.ReadFile(prevViewPath(viewPath)); err != nil || string(kept) != baseViewText {
		t.Errorf("kept view = %q, %v; want the view before the update", kept, err)
	}
}

// TestGenerateValidationCancellationWritesNothing cancels the run while
// the validator is still deciding: the view file must not be written,
// no Undo backup may appear, and the run ends as cancelled.
func TestGenerateValidationCancellationWritesNothing(t *testing.T) {
	viewPath, sentinel := writeSentinelView(t)
	prepare, _ := fakeAgentJob(t, fakeSpec{Out: minimalOut})
	ctx, cancel := context.WithCancel(context.Background())
	entered := make(chan struct{})
	job := generationJob{
		Req: GenerateRequest{
			Text:    "# A\n",
			Mode:    "full",
			Request: "A view.",
			Format:  "{}",
		},
		ViewPath: viewPath,
		Model:    "gemini-3.8-flash",
	}
	prepare(&job)
	// The validator observes the run's context, as the page call does.
	job.Validate = func(ctx context.Context, _ []byte) error {
		close(entered)
		<-ctx.Done()
		return errCancelled
	}

	done := make(chan error, 1)
	go func() { done <- runGeneration(ctx, job, func(GenerateProgress) error { return nil }) }()
	select {
	case <-entered:
	case <-time.After(10 * time.Second):
		t.Fatal("validation never started")
	}
	cancel()
	select {
	case err := <-done:
		if f := failureFrom(t, err); f.Kind != failCancelled {
			t.Errorf("failure = %+v, want cancellation", f)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("runGeneration never returned after the cancel")
	}
	sentinelKept(t, viewPath, sentinel)
	if _, err := os.Stat(prevViewPath(viewPath)); err == nil || !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("kept view exists after a cancelled validation: %v", err)
	}
}
