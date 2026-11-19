package genui

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/egoist/mygo"
)

const validatorBridge = "window.__mgoValidateGeneratedView"

func ValidateCandidateWebView(ctx context.Context, candidate []byte) error {
	if ctx.Err() != nil {
		return ErrCancelled
	}
	win := mygo.CallerWindow(ctx)
	if win == nil {
		return ValidationUnavailable("no window is showing the document")
	}
	page := win.Page()
	if page == nil {
		return ValidationUnavailable("the window shows no page")
	}
	quoted, err := json.Marshal(string(candidate))
	if err != nil {
		return ValidationUnavailable("the candidate could not be passed to the page")
	}

	script := fmt.Sprintf(`const validate = %s
if (typeof validate !== "function") throw new Error("the view validator is not installed")
return await validate(%s)`, validatorBridge, quoted)
	result, err := page.EvalContext(ctx, script)
	if err != nil {
		if ctx.Err() != nil {
			return ErrCancelled
		}
		return ValidationUnavailable(err.Error())
	}
	verdict, ok := result.(map[string]any)
	if !ok {
		return ValidationUnavailable("the page returned no verdict")
	}
	if accepted, _ := verdict["ok"].(bool); accepted {
		return nil
	}
	reason, _ := verdict["error"].(string)
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = "the page rejected it without a reason"
	}
	return NewFailure(FailInvalidOutput, "The generated view is not valid: "+reason, HintInvalidOutput)
}

func ValidationUnavailable(detail string) error {
	detail = strings.TrimSuffix(strings.TrimSpace(detail), ".")
	return NewFailure(FailInvalidOutput, "The generated view could not be checked, so it was not saved: "+detail+".", HintInvalidOutput)
}
