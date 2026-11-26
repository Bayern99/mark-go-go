package nativegenui

import (
	"context"
	"errors"
	"testing"

	"github.com/Bayern99/mark-go-go/internal/genui"
	"github.com/Bayern99/mark-go-go/internal/portrait"
)

func TestCancelledGenerationFinishesWithoutFailureRecoveryUI(t *testing.T) {
	for _, err := range []error{context.Canceled, genui.NewFailure(genui.FailCancelled, "cancelled", "")} {
		v := &documentView{cancel: func() {}, reading: Reading{Replacing: []string{"card"}}}
		v.finished(err)
		if v.failure != nil || v.options || v.failureOpen {
			t.Errorf("cancellation %v produced failure=%v options-open=%t details-open=%t", err, v.failure, v.options, v.failureOpen)
		}
		if v.cancel != nil || v.message != "" || v.reading.Replacing != nil {
			t.Errorf("cancellation %v left generation state behind: %+v", err, v)
		}
	}
}

func TestGenerationOutcomeToast(t *testing.T) {
	if got := generationOutcomeToast("doc1", nil); got.ID != "generation:doc1" || got.Tone != portrait.Tones.Success || got.Title != "Generated" {
		t.Fatalf("success toast = %+v", got)
	}
	cancelled := genui.NewFailure(genui.FailCancelled, "cancelled", "")
	if got := generationOutcomeToast("doc1", cancelled); got.ID != "generation:doc1" || got.Tone != portrait.Tones.Neutral || got.Title != "Generation canceled" {
		t.Fatalf("FailCancelled toast = %+v", got)
	}
	if got := generationOutcomeToast("doc1", context.Canceled); got.ID != "generation:doc1" || got.Tone != portrait.Tones.Neutral || got.Title != "Generation canceled" {
		t.Fatalf("context.Canceled toast = %+v", got)
	}
	if got := generationOutcomeToast("doc1", errors.New("boom")); got.ID != "generation:doc1" || got.Tone != portrait.Tones.Danger || got.Title != "Generation failed" {
		t.Fatalf("failure toast = %+v", got)
	}
}
