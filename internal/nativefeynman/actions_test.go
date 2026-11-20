package nativefeynman

import (
	"errors"
	"testing"

	"github.com/Bayern99/mark-go-go/internal/agent/feynman"
)

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

func TestFinishSessionSwitchOnlyMarksReadyWhenStillConnecting(t *testing.T) {
	cases := []struct {
		name        string
		initial     connectionStatus
		err         error
		wantStatus  connectionStatus
		wantSession string
		wantTitle   string
		wantRowsLen int
		wantOpen    bool
		wantErr     string
	}{
		{
			name:       "error after offline keeps offline so reconnect remains available",
			initial:    statusOffline,
			err:        errors.New("switch failed"),
			wantStatus: statusOffline,
			wantOpen:   true,
			wantErr:    "switch failed",
		},
		{
			name:       "error while connecting returns to ready",
			initial:    statusConnecting,
			err:        errors.New("session locked"),
			wantStatus: statusReady,
			wantOpen:   true,
			wantErr:    "session locked",
		},
		{
			name:        "late success after offline stays offline",
			initial:     statusOffline,
			wantStatus:  statusOffline,
			wantSession: "next-session",
			wantTitle:   "Next",
			wantRowsLen: 1,
			wantOpen:    false,
		},
		{
			name:        "late success while working keeps working",
			initial:     statusWorking,
			wantStatus:  statusWorking,
			wantSession: "next-session",
			wantTitle:   "Next",
			wantRowsLen: 1,
			wantOpen:    false,
		},
		{
			name:        "success while connecting marks ready and updates session view",
			initial:     statusConnecting,
			wantStatus:  statusReady,
			wantSession: "next-session",
			wantTitle:   "Next",
			wantRowsLen: 1,
			wantOpen:    false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			state := newPaneState("/workspace")
			state.status = tc.initial
			state.sessionsOpen = true
			p := &panel{state: state}

			nextState := feynman.ResearchSessionState{SessionID: "next-session", SessionName: "Next"}
			nextRows := []feynman.ResearchConversation{{ID: "next-session", SessionFile: "/workspace/next.jsonl", Name: "Next"}}
			if tc.err != nil {
				p.finishSessionSwitch(feynman.ResearchSessionState{}, nil, tc.err)
			} else {
				p.finishSessionSwitch(nextState, nextRows, nil)
			}

			if state.status != tc.wantStatus {
				t.Fatalf("status = %q, want %q", state.status, tc.wantStatus)
			}
			if state.error != tc.wantErr {
				t.Fatalf("error = %q, want %q", state.error, tc.wantErr)
			}
			if state.sessionsOpen != tc.wantOpen {
				t.Fatalf("sessionsOpen = %v, want %v", state.sessionsOpen, tc.wantOpen)
			}
			if state.session.SessionID != tc.wantSession || state.title != tc.wantTitle || len(state.sessionRows) != tc.wantRowsLen {
				t.Fatalf("session=%#v title=%q rows=%d", state.session, state.title, len(state.sessionRows))
			}
		})
	}
}
