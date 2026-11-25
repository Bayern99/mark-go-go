package mcp

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Bayern99/mark-go-go/internal/ipc"
)

func TestWholeDocumentProposalCanBeEmptyAndEditModesCannotOverlap(t *testing.T) {
	h, root := newSocketHarness(t)
	srv, reviews := startExternalWithReview(t, h)
	client, reply := connectExternal(t, srv, root)
	if !reply.OK {
		t.Fatal(reply.Error)
	}
	accepted := client.tool(toolProposeEdit, map[string]any{"path": "report.md", "proposedText": ""})
	if accepted.IsError {
		t.Errorf("empty whole document proposal failed: %+v", accepted)
	}
	proposals := reviews.ListProposals(root)
	if len(proposals) != 1 || proposals[0].ProposedText != "" || len(proposals[0].Hunks) == 0 {
		t.Errorf("queued proposals = %+v, want a deletion in review", proposals)
	}
	if text, err := os.ReadFile(filepath.Join(root, "report.md")); err != nil || string(text) != h.docs["doc-report"].Text {
		t.Fatalf("original changed before approval: %q, %v", text, err)
	}
	for _, args := range []map[string]any{
		{"proposedText": "replacement", "newText": ""},
		{"proposedText": "", "startLine": 0},
		{"oldText": "Report", "newText": "replacement", "endLine": 0},
		{"oldText": "Report"},
		{"proposedText": "replacement", "oldText": nil},
	} {
		args["path"] = "report.md"
		res := client.tool(toolProposeEdit, args)
		if !res.IsError {
			t.Errorf("accepted invalid arguments: %+v", args)
		} else if code := structured[ipc.AppError](t, res).Code; code != ipc.CodeInvalidArgs {
			t.Errorf("arguments %+v gave %q", args, code)
		}
	}
}
