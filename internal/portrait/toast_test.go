package portrait_test

import (
	"slices"
	"testing"
	"time"

	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/egoist/mygo/ui"
)

type toastHarness struct {
	pending []portrait.ToastSpec
}

func newToastTester() (*ui.Tester, *toastHarness) {
	h := &toastHarness{}
	tt := ui.NewTester(func(c *ui.Context) {
		for _, spec := range h.pending {
			portrait.Notify(c, spec)
		}
		h.pending = nil
		portrait.Toasts(c)
	}, 480, 320)
	return tt, h
}

func (h *toastHarness) notify(spec portrait.ToastSpec) {
	h.pending = append(h.pending, spec)
}

func TestToastWithSameIDUpdatesInPlace(t *testing.T) {
	tt, h := newToastTester()
	h.notify(portrait.ToastSpec{ID: "generation:a", Tone: portrait.Tones.Info, Title: "Generating…", Timeout: -1})
	tt.Frame()
	tt.Advance(portrait.Motion.Enter)
	if !tt.HasText("Generating…") {
		t.Fatal("initial toast is not visible")
	}
	h.notify(portrait.ToastSpec{ID: "generation:a", Tone: portrait.Tones.Success, Title: "Generated"})
	tt.Frame()
	tt.Advance(portrait.Motion.Enter)
	if tt.HasText("Generating…") || !tt.HasText("Generated") {
		t.Fatalf("toast did not update in place: %q", tt.Texts())
	}
	statuses := 0
	for _, info := range tt.AccessAll() {
		if info.Role == ui.RoleStatus {
			statuses++
		}
	}
	if statuses != 1 {
		t.Fatalf("found %d toast statuses after update, want one", statuses)
	}
}

func TestToastViewportShowsOnlyNewestThree(t *testing.T) {
	tt, h := newToastTester()
	for _, title := range []string{"One", "Two", "Three", "Four"} {
		h.notify(portrait.ToastSpec{ID: title, Title: title, Timeout: -1})
	}
	tt.Frame()
	tt.Advance(portrait.Motion.Enter)
	if tt.HasText("One") {
		t.Fatal("oldest toast is visible beyond the three-toast limit")
	}
	for _, title := range []string{"Two", "Three", "Four"} {
		if !tt.HasText(title) {
			t.Errorf("%s is not visible: %q", title, tt.Texts())
		}
	}
	statuses := 0
	for _, info := range tt.AccessAll() {
		if info.Role == ui.RoleStatus {
			statuses++
		}
	}
	if statuses != 3 {
		t.Fatalf("found %d visible toasts, want three", statuses)
	}
}

func TestToastEscapeClosesFocusedToastWithoutRunningAction(t *testing.T) {
	tt, h := newToastTester()
	called := false
	h.notify(portrait.ToastSpec{
		ID: "deleted", Title: "Note deleted", Action: "Undo", Timeout: -1,
		OnAction: func() { called = true },
	})
	tt.Frame()
	tt.Advance(portrait.Motion.Enter)
	for range 4 {
		if tt.Focused("Undo") {
			break
		}
		tt.Key(0, ui.KeyTab)
	}
	if !tt.Focused("Undo") {
		t.Fatal("Tab did not reach the toast action")
	}
	tt.Key(0, ui.KeyEscape)
	tt.Advance(portrait.Motion.Enter + 20*time.Millisecond)
	if tt.HasText("Note deleted") {
		t.Fatal("Escape did not close the focused toast")
	}
	if called {
		t.Fatal("Escape ran the toast action")
	}
}

func TestToastActionRunsOnceAndCloses(t *testing.T) {
	tt, h := newToastTester()
	var calls []string
	h.notify(portrait.ToastSpec{
		ID: "deleted", Title: "Note deleted", Action: "Undo", Timeout: -1,
		OnAction: func() { calls = append(calls, "deleted") },
	})
	tt.Frame()
	tt.Advance(portrait.Motion.Enter)
	if err := tt.Click("Undo"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(portrait.Motion.Enter + 20*time.Millisecond)
	if !slices.Equal(calls, []string{"deleted"}) {
		t.Fatalf("action calls = %v", calls)
	}
	if tt.HasText("Note deleted") {
		t.Fatal("action did not close its toast")
	}
}
