package mcp

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"

	"github.com/Bayern99/mark-go-go/internal/ipc"
	"github.com/Bayern99/mark-go-go/internal/workspace"
)

// appError is the stable, machine-readable failure a tool returns in
// structuredContent when isError is true.
type appError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Hint    string `json:"hint,omitempty"`
}

func (e *appError) Error() string {
	if e.Hint == "" {
		return e.Message
	}
	return e.Message + " " + e.Hint
}

var hints = map[string]string{
	ipc.CodeAppNotRunning:    "Start the Mark Go Go app, then call the tool again.",
	ipc.CodeWorkspaceNotOpen: "Open the folder in Mark Go Go, or call get_workspace for the workspaces that are open.",
	ipc.CodeVersionConflict:  "Read the document again with read_document and retry with its current version.",
	ipc.CodeNotFound:         "Call get_workspace to list the documents and paths in this workspace.",
	ipc.CodePathDenied:       "Pass a workspace-relative path to a Markdown file that is not in an excluded or hidden folder.",
	ipc.CodeInvalidArgs:      "Check the tool's inputSchema and pass exactly one editing mode.",
}

func newAppError(code, message string) *appError {
	return &appError{Code: code, Message: message, Hint: hints[code]}
}

// classifyFailure turns a failure from the workspace or review service into a
// stable app error.
func classifyFailure(err error) *appError {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, workspace.ErrWorkspaceNotOpen):
		return newAppError(ipc.CodeWorkspaceNotOpen, "No workspace with that root is open in Mark Go Go.")
	case errors.Is(err, workspace.ErrPathDenied):
		return newAppError(ipc.CodePathDenied, err.Error())
	case errors.Is(err, fs.ErrNotExist):
		return newAppError(ipc.CodeNotFound, "No document matches that id or path.")
	default:
		return newAppError(ipc.CodeInvalidArgs, err.Error())
	}
}

func toolFailure(err *appError) mcpCallToolResult {
	return mcpCallToolResult{
		Content:           []mcpTextContent{{Type: "text", Text: err.Error()}},
		StructuredContent: err,
		IsError:           true,
	}
}

func toolJSON(payload any) (mcpCallToolResult, *rpcError) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return mcpCallToolResult{}, &rpcError{Code: errCodeInternalError, Message: err.Error()}
	}
	return mcpCallToolResult{
		Content:           []mcpTextContent{{Type: "text", Text: string(raw)}},
		StructuredContent: payload,
	}, nil
}

func decodeArgs(raw json.RawMessage, into any) *appError {
	if len(raw) == 0 {
		raw = json.RawMessage("{}")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(into); err != nil {
		return newAppError(ipc.CodeInvalidArgs, fmt.Sprintf("Invalid arguments: %v.", err))
	}
	return nil
}
