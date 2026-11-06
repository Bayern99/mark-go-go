package main

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// GenerateFailure is why a generation ended without a new view, sent
// to the page as the last step of the run, so that the page tells the
// reader what happened and what to do.
type GenerateFailure struct {
	// Kind is one of the failure kinds below.
	Kind string `json:"kind"`
	// Message says what happened, in a sentence or two.
	Message string `json:"message"`
	// Hint says what the reader can do about it.
	Hint string `json:"hint,omitempty"`
	// Detail is the agent's own words, verbatim.
	Detail string `json:"detail,omitempty"`
	// Models are the models the agent offers, for model-invalid.
	Models []string `json:"models,omitempty"`
}

// The kinds of GenerateFailure.
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

// The hints of the kinds that always give the same advice.
const (
	hintBusy          = "Wait for it to finish, or cancel it."
	hintAgentNotFound = "Install the Antigravity CLI, which provides agy, or set the agent command to its full path in Agent settings."
	hintModelInvalid  = "Choose one of the models it offers in Agent settings."
	hintAgentMessage  = "Read the agent's message; retry once it is resolved."
	hintInvalidOutput = "Generate again; rephrasing the request may help."
	hintAgentError    = "Retry; if it keeps failing, run the agent command in Terminal to see what it says."
	hintTimeout       = "Retry, perhaps with a shorter request or a faster model."
)

// maxDetailRunes caps the agent's words a failure carries.
const maxDetailRunes = 4000

// genError is a generation failure as an error. Its text is the
// message and the hint, for callers that only show a string.
type genError struct {
	GenerateFailure
}

func (e *genError) Error() string {
	if e.Hint == "" {
		return e.Message
	}
	return e.Message + " " + e.Hint
}

// newFailure makes a failure of kind with its message and hint.
func newFailure(kind, message, hint string) *genError {
	return &genError{GenerateFailure{Kind: kind, Message: message, Hint: hint}}
}

// agentError is the failure of an error the reader can do nothing
// specific about: retry, or look at the agent itself.
func agentError(message string) *genError {
	return newFailure(failAgentError, message, hintAgentError)
}

// agentErrorf is agentError with a format.
func agentErrorf(format string, args ...any) *genError {
	return agentError(fmt.Sprintf(format, args...))
}

// authRequired is the failure of an agent that is not signed in;
// command is the agent command as the settings have it.
func authRequired(command, detail string) *genError {
	e := newFailure(failAuthRequired, "The agent is not signed in.",
		fmt.Sprintf("Run `%s acp --login` in Terminal, then retry.", command))
	e.Detail = cutRunes(detail, maxDetailRunes)
	return e
}

// isAuthError says whether a JSON-RPC error of the agent, or its error
// output, means it is not signed in: ACP's -32000, or agy's words for it.
func isAuthError(code int, message string) bool {
	return code == -32000 || strings.Contains(strings.ToLower(message), "authentication required")
}

// timedOut is the failure of a run that took longer than limit.
func timedOut(limit time.Duration) *genError {
	return newFailure(failTimeout, fmt.Sprintf("Generation timed out after %s.", humanDuration(limit)), hintTimeout)
}

// humanDuration writes whole minutes as words, and anything else as Go
// does.
func humanDuration(d time.Duration) string {
	if d >= time.Minute && d%time.Minute == 0 {
		if n := int(d / time.Minute); n != 1 {
			return fmt.Sprintf("%d minutes", n)
		}
		return "1 minute"
	}
	return d.String()
}

// failureOf returns the failure an error of a run stands for; an error
// that is not a genError is an agent-error with its text.
func failureOf(err error) GenerateFailure {
	var ge *genError
	if errors.As(err, &ge) {
		return ge.GenerateFailure
	}
	return agentError(err.Error()).GenerateFailure
}

// reportFailure sends the page the failure err stands for as the last
// step of the run, and returns it as the run's error. The page may be
// gone already, so the step may not arrive.
func reportFailure(err error, progress func(GenerateProgress) error) error {
	f := failureOf(err)
	_ = progress(GenerateProgress{Phase: "failed", Text: f.Message, Failure: &f})
	return &genError{f}
}
