package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/egoist/mygo"
)

// validatorBridge is the page function that checks a generated view with
// the app's own catalog and message processor. The frontend installs it on
// its window before generation can run.
const validatorBridge = "window.__mgoValidateGeneratedView"

// generatedViewValidator checks the exact complete view file a run is about
// to write. In the app the page's Catalog and MessageProcessor are the
// authority: the agent's JSON never runs as code, only as one string
// argument to the page's validator. A run whose page cannot answer keeps
// the view file untouched.
func validateGeneratedCandidate(ctx context.Context, candidate []byte) error {
	if ctx.Err() != nil {
		return errCancelled
	}
	win := mygo.CallerWindow(ctx)
	if win == nil {
		return validationUnavailable("no window is showing the document")
	}
	page := win.Page()
	if page == nil {
		return validationUnavailable("the window shows no page")
	}
	// encoding/json quotes the candidate, including the two line separators
	// JavaScript would treat as line breaks, so no model text can become
	// code. The candidate reaches the page as one string argument.
	quoted, err := json.Marshal(string(candidate))
	if err != nil {
		return validationUnavailable("the candidate could not be passed to the page")
	}

	script := fmt.Sprintf(`const validate = %s
if (typeof validate !== "function") throw new Error("the view validator is not installed")
return await validate(%s)`, validatorBridge, quoted)
	result, err := page.EvalContext(ctx, script)
	if err != nil {
		if ctx.Err() != nil {
			return errCancelled
		}
		return validationUnavailable(err.Error())
	}
	verdict, ok := result.(map[string]any)
	if !ok {
		return validationUnavailable("the page returned no verdict")
	}
	if accepted, _ := verdict["ok"].(bool); accepted {
		return nil
	}
	reason, _ := verdict["error"].(string)
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = "the page rejected it without a reason"
	}
	return newFailure(failInvalidOutput, "The generated view is not valid: "+reason, hintInvalidOutput)
}

// validationUnavailable is the failure of a run whose view could not be
// checked at all. It fails closed: an unchecked view is never written.
func validationUnavailable(detail string) error {
	detail = strings.TrimSuffix(strings.TrimSpace(detail), ".")
	return newFailure(failInvalidOutput, "The generated view could not be checked, so it was not saved: "+detail+".", hintInvalidOutput)
}
