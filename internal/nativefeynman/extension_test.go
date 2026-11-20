package nativefeynman

import (
	"testing"

	"github.com/Bayern99/mark-go-go/internal/agent/feynman"
)

func TestSwitchingWorkspaceClearsPreviousRootViews(t *testing.T) {
	state := newPaneState("/workspace/old")
	state.messages = []message{{kind: "assistant", messageID: "old", text: "old conversation"}}
	state.draft = "old draft"
	p := &panel{
		state:         state,
		settingsRoot:  "/workspace/old",
		sessionView:   "fork",
		forkMessages:  []feynman.ResearchForkMessage{{EntryID: "old-entry", Text: "old message"}},
		researchTree:  &feynman.ResearchTree{LeafID: "old-leaf"},
		decodedImages: map[string]decodedDataImage{"old": {}},
	}

	p.switchRoot("/workspace/new")

	want := newPaneState("/workspace/new")
	if p.state.root != want.root {
		t.Fatalf("state root = %q, want %q", p.state.root, want.root)
	}
	if len(p.state.messages) != 0 || p.state.draft != "" {
		t.Fatalf("previous conversation leaked into the new root: messages=%#v draft=%q", p.state.messages, p.state.draft)
	}
	if p.settingsRoot != "" {
		t.Fatalf("settingsRoot = %q, want reset", p.settingsRoot)
	}
	if p.sessionView != "" {
		t.Fatalf("sessionView = %q, want reset", p.sessionView)
	}
	if p.forkMessages != nil {
		t.Fatalf("forkMessages = %#v, want cleared", p.forkMessages)
	}
	if p.researchTree != nil {
		t.Fatalf("researchTree = %#v, want cleared", p.researchTree)
	}
	if p.decodedImages != nil {
		t.Fatal("decoded images from the previous root were retained")
	}
}
