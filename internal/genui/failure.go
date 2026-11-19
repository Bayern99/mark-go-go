package genui

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

type GenerateFailure struct {
	Kind    string   `json:"kind"`
	Message string   `json:"message"`
	Hint    string   `json:"hint,omitempty"`
	Detail  string   `json:"detail,omitempty"`
	Models  []string `json:"models,omitempty"`
}

const (
	FailUnsaved       = "unsaved"
	FailBusy          = "busy"
	FailAgentNotFound = "agent-not-found"
	FailAuthRequired  = "auth-required"
	FailModelInvalid  = "model-invalid"
	FailAgentMessage  = "agent-message"
	FailInvalidOutput = "invalid-output"
	FailAgentError    = "agent-error"
	FailTimeout       = "timeout"
	FailCancelled     = "cancelled"
)

const (
	HintBusy          = "Wait for it to finish, or cancel it."
	HintAgentNotFound = "Install the Antigravity CLI, which provides agy, or set the agent command to its full path in Agent settings."
	HintModelInvalid  = "Choose one of the models it offers in Agent settings."
	HintAgentMessage  = "Read the agent's message; retry once it is resolved."
	HintInvalidOutput = "Generate again; rephrasing the request may help."
	HintAgentError    = "Retry; if it keeps failing, run the agent command in Terminal to see what it says."
	HintTimeout       = "Retry, perhaps with a shorter request or a faster model."
)

const MaxDetailRunes = 4000

type GenError struct {
	GenerateFailure
}

func (e *GenError) Error() string {
	if e.Hint == "" {
		return e.Message
	}
	return e.Message + " " + e.Hint
}

func NewFailure(kind, message, hint string) *GenError {
	return &GenError{GenerateFailure{Kind: kind, Message: message, Hint: hint}}
}

func AgentError(message string) *GenError {
	return NewFailure(FailAgentError, message, HintAgentError)
}

func AgentErrorf(format string, args ...any) *GenError {
	return AgentError(fmt.Sprintf(format, args...))
}

func AuthRequired(command, detail string) *GenError {
	e := NewFailure(FailAuthRequired, "The agent is not signed in.",
		fmt.Sprintf("Run `%s acp --login` in Terminal, then retry.", command))
	e.Detail = CutRunes(detail, MaxDetailRunes)
	return e
}

func IsAuthError(code int, message string) bool {
	return code == -32000 || strings.Contains(strings.ToLower(message), "authentication required")
}

func TimedOut(limit time.Duration) *GenError {
	return NewFailure(FailTimeout, fmt.Sprintf("Generation timed out after %s.", humanDuration(limit)), HintTimeout)
}

func humanDuration(d time.Duration) string {
	if d >= time.Minute && d%time.Minute == 0 {
		if n := int(d / time.Minute); n != 1 {
			return fmt.Sprintf("%d minutes", n)
		}
		return "1 minute"
	}
	return d.String()
}

func FailureOf(err error) GenerateFailure {
	var ge *GenError
	if errors.As(err, &ge) {
		return ge.GenerateFailure
	}
	return AgentError(err.Error()).GenerateFailure
}

func ReportFailure(err error, progress func(GenerateProgress) error) error {
	f := FailureOf(err)
	_ = progress(GenerateProgress{Phase: "failed", Text: f.Message, Failure: &f})
	return &GenError{f}
}
