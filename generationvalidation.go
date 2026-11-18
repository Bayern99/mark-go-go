package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/egoist/mygo"
)

const validatorBridge = "window.__mgoValidateGeneratedView"

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

func validationUnavailable(detail string) error {
	detail = strings.TrimSuffix(strings.TrimSpace(detail), ".")
	return newFailure(failInvalidOutput, "The generated view could not be checked, so it was not saved: "+detail+".", hintInvalidOutput)
}
