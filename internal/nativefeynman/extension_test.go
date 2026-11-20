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
		state:           state,
		settingsRoot:    "/workspace/old",
		sessionView:     "fork",
		forkMessages:    []feynman.ResearchForkMessage{{EntryID: "old-entry", Text: "old message"}},
		researchTree:    &feynman.ResearchTree{LeafID: "old-leaf"},
		decodedImages:   map[string]decodedDataImage{"old": {}},
		questionText:    "pending answer",
		questionPick:    "option-1",
		questionFind:    "search",
		permissionID:    "req-old",
		questionPending: true,
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
	if p.questionPending || p.permissionID != "" || p.questionText != "" || p.questionPick != "" || p.questionFind != "" {
		t.Fatalf("question interaction state leaked across root switch: pending=%v id=%q text=%q pick=%q find=%q",
			p.questionPending, p.permissionID, p.questionText, p.questionPick, p.questionFind)
	}
}

func TestResetUpdateClearsForkTreeAndQuestionStateWhileKeepingDraft(t *testing.T) {
	state := newPaneState("/workspace")
	state.draft = "keep this draft"
	state.sources = []string{"/workspace/revoked.md"}
	state.messages = []message{{kind: "user", messageID: "u1", text: "hello"}}
	p := &panel{
		state:           state,
		connected:       state.root,
		sessionView:     "tree",
		forkMessages:    []feynman.ResearchForkMessage{{EntryID: "e1", Text: "branch"}},
		researchTree:    &feynman.ResearchTree{LeafID: "leaf-1"},
		decodedImages:   map[string]decodedDataImage{"img": {}},
		questionText:    "draft answer",
		questionPick:    "choice-a",
		questionFind:    "filter",
		permissionID:    "perm-1",
		questionPending: true,
	}

	p.researchUpdated(feynman.ResearchUpdate{Root: "/workspace", Kind: "reset"})

	if p.state.draft != "keep this draft" {
		t.Fatalf("draft = %q, want preserved on reset", p.state.draft)
	}
	if len(p.state.sources) != 0 || len(p.state.messages) != 0 {
		t.Fatalf("sources=%#v messages=%#v, want cleared on reset", p.state.sources, p.state.messages)
	}
	if p.sessionView != "" || p.forkMessages != nil || p.researchTree != nil || p.decodedImages != nil {
		t.Fatalf("session views not cleared on reset: view=%q fork=%#v tree=%#v imgs=%#v",
			p.sessionView, p.forkMessages, p.researchTree, p.decodedImages)
	}
	if p.questionPending || p.permissionID != "" || p.questionText != "" || p.questionPick != "" || p.questionFind != "" {
		t.Fatalf("question state not cleared on reset: pending=%v id=%q text=%q pick=%q find=%q",
			p.questionPending, p.permissionID, p.questionText, p.questionPick, p.questionFind)
	}
}

func TestEmptyRootDisconnectSwitchesToEmptyRoot(t *testing.T) {
	oldState := newPaneState("/workspace/old")
	oldState.status = statusReady
	oldState.messages = []message{{kind: "assistant", messageID: "m1", text: "stale"}}
	p := &panel{
		state:           oldState,
		connected:       oldState.root,
		sessionView:     "fork",
		forkMessages:    []feynman.ResearchForkMessage{{EntryID: "e1"}},
		researchTree:    &feynman.ResearchTree{LeafID: "leaf-1"},
		questionPending: true,
	}

	// Disabling plugin while root remains non-empty disconnects without clearing state.root.
	p.disconnect(false)
	if p.connected != "" || p.state.status != statusOffline || p.state.root != oldState.root {
		t.Fatalf("disconnect(false): connected=%q status=%q root=%q", p.connected, p.state.status, p.state.root)
	}

	// Empty workspace root clears state via switchRoot(""), even when already disconnected.
	p.disconnect(true)
	if p.state.root != "" || p.state.status != statusOffline {
		t.Fatalf("disconnect(true): root=%q status=%q", p.state.root, p.state.status)
	}
	if len(p.state.messages) != 0 || p.sessionView != "" || p.forkMessages != nil || p.researchTree != nil || p.questionPending {
		t.Fatalf("disconnect(true) retained previous workspace state: msgs=%#v view=%q pending=%v",
			p.state.messages, p.sessionView, p.questionPending)
	}
}
