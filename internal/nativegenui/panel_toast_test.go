package nativegenui

import (
	"context"
	"errors"
	"testing"

	"github.com/Bayern99/mark-go-go/internal/genui"
	"github.com/Bayern99/mark-go-go/internal/portrait"
)

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
