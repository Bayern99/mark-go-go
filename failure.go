package main

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
	failUnsaved       = "unsaved"
	failBusy          = "busy"
	failAgentNotFound = "agent-not-found"
	failAuthRequired  = "auth-required"
	failModelInvalid  = "model-invalid"
	failAgentMessage  = "agent-message"
	failInvalidOutput = "invalid-output"
	failAgentError    = "agent-error"
	failTimeout       = "timeout"
	failCancelled     = "cancelled"
)

const (
	hintBusy          = "Wait for it to finish, or cancel it."
	hintAgentNotFound = "Install the Antigravity CLI, which provides agy, or set the agent command to its full path in Agent settings."
	hintModelInvalid  = "Choose one of the models it offers in Agent settings."
	hintAgentMessage  = "Read the agent's message; retry once it is resolved."
	hintInvalidOutput = "Generate again; rephrasing the request may help."
	hintAgentError    = "Retry; if it keeps failing, run the agent command in Terminal to see what it says."
	hintTimeout       = "Retry, perhaps with a shorter request or a faster model."
)

const maxDetailRunes = 4000

type genError struct {
	GenerateFailure
}

func (e *genError) Error() string {
	if e.Hint == "" {
		return e.Message
	}
	return e.Message + " " + e.Hint
}

func newFailure(kind, message, hint string) *genError {
	return &genError{GenerateFailure{Kind: kind, Message: message, Hint: hint}}
}

func agentError(message string) *genError {
	return newFailure(failAgentError, message, hintAgentError)
}

func agentErrorf(format string, args ...any) *genError {
	return agentError(fmt.Sprintf(format, args...))
}

func authRequired(command, detail string) *genError {
	e := newFailure(failAuthRequired, "The agent is not signed in.",
		fmt.Sprintf("Run `%s acp --login` in Terminal, then retry.", command))
	e.Detail = cutRunes(detail, maxDetailRunes)
	return e
}

func isAuthError(code int, message string) bool {
	return code == -32000 || strings.Contains(strings.ToLower(message), "authentication required")
}

func timedOut(limit time.Duration) *genError {
	return newFailure(failTimeout, fmt.Sprintf("Generation timed out after %s.", humanDuration(limit)), hintTimeout)
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

func failureOf(err error) GenerateFailure {
	var ge *genError
	if errors.As(err, &ge) {
		return ge.GenerateFailure
	}
	return agentError(err.Error()).GenerateFailure
}

func reportFailure(err error, progress func(GenerateProgress) error) error {
	f := failureOf(err)
	_ = progress(GenerateProgress{Phase: "failed", Text: f.Message, Failure: &f})
	return &genError{f}
}
