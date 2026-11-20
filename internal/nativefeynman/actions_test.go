package nativefeynman

import "testing"

func TestSelectSessionWhileBusyKeepsReadyStatusAndShowsError(t *testing.T) {
	state := newPaneState("/workspace")
	state.status = statusReady
	state.cliBusy = true
	p := &panel{state: state}

	p.selectSession("other-conversation")

	if state.status != statusReady {
		t.Fatalf("status = %q, want ready; a rejected session switch must not leave the pane connecting", state.status)
	}
	if state.error == "" {
		t.Fatal("rejected session switch produced no visible error")
	}
	if !state.cliBusy {
		t.Fatal("rejected session switch cleared the in-flight action")
	}
}
