package nativefeynman

import (
	"testing"

	"github.com/Bayern99/mark-go-go/internal/agent/feynman"
)

func TestAssistantUpdatesStreamIntoOneMessageAndSettleTheTurn(t *testing.T) {
	state := newPaneState("/workspace")
	state.receive(feynman.ResearchUpdate{Root: "/workspace", Kind: "progress", EventType: "agent_start", Text: "Working…"})
	state.receive(feynman.ResearchUpdate{Root: "/workspace", Kind: "assistant", MessageID: "response-1", Text: "A stream "})
	state.receive(feynman.ResearchUpdate{Root: "/workspace", Kind: "assistant", MessageID: "response-1", Text: "continues."})
	state.receive(feynman.ResearchUpdate{Root: "/workspace", Kind: "agentSettled"})

	if state.status != statusReady {
		t.Fatalf("status = %q, want ready", state.status)
	}
	if len(state.messages) != 1 || state.messages[0].text != "A stream continues." {
		t.Fatalf("messages = %#v", state.messages)
	}
}

func TestPaneStateIgnoresUpdatesForAnotherWorkspace(t *testing.T) {
	state := newPaneState("/workspace/one")
	state.receive(feynman.ResearchUpdate{Root: "/workspace/two", Kind: "assistant", MessageID: "other", Text: "private"})
	if len(state.messages) != 0 {
		t.Fatalf("other workspace update reached the pane: %#v", state.messages)
	}
}

func TestFailedPromptRestoresOldPromptOnlyWhenNothingNewWasTyped(t *testing.T) {
	state := newPaneState("/workspace")
	state.draft = "a newer question"
	state.sources = []string{"/workspace/new.md"}

	state.restoreDraft("the failed prompt", []string{"/workspace/old.md"})

	if state.draft != "a newer question" {
		t.Fatalf("draft = %q, want the newer typed input preserved", state.draft)
	}
	if !containsSource(state.sources, "/workspace/old.md") || !containsSource(state.sources, "/workspace/new.md") {
		t.Fatalf("sources = %#v, want failed and newly attached sources merged", state.sources)
	}

	state.draft = ""
	state.sources = nil
	state.restoreDraft("the failed prompt", []string{"/workspace/old.md"})
	if state.draft != "the failed prompt" {
		t.Fatalf("draft = %q, want the failed prompt restored into an empty composer", state.draft)
	}
	if !containsSource(state.sources, "/workspace/old.md") {
		t.Fatalf("sources = %#v, want failed attachments restored", state.sources)
	}
}

func containsSource(sources []string, target string) bool {
	for _, source := range sources {
		if source == target {
			return true
		}
	}
	return false
}

func TestResetConversationPreservesDraftAndClearsSources(t *testing.T) {
	state := newPaneState("/workspace")
	state.messages = []message{{kind: "assistant", messageID: "m1", text: "prior"}}
	state.sources = []string{"/workspace/attached.md"}
	state.draft = "unsent prompt across reconnect"
	state.permission = &feynman.ResearchUpdate{Root: "/workspace", RequestID: "req-1"}

	state.receive(feynman.ResearchUpdate{Root: "/workspace", Kind: "reset"})

	if state.draft != "unsent prompt across reconnect" {
		t.Fatalf("draft = %q, want unsent draft preserved across reset", state.draft)
	}
	if len(state.sources) != 0 {
		t.Fatalf("sources = %#v, want cleared after reset", state.sources)
	}
	if len(state.messages) != 0 || state.permission != nil {
		t.Fatalf("messages=%#v permission=%#v, want cleared after reset", state.messages, state.permission)
	}
}
