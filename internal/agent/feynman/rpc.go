package feynman

import (
	"os/exec"

	"github.com/Bayern99/mark-go-go/internal/pirpc"
)

// rpcClient is Feynman's own records over the shared JSONL transport. The
// transport names Feynman in the messages it reports, so a failure reads the
// same through the shared client as it did when the client lived here.
type rpcClient = pirpc.Client

var splitJSONLLines = pirpc.SplitJSONLLines

func newRPCClient(cmd *exec.Cmd) (*rpcClient, error) {
	return pirpc.New(pirpc.Config{Command: cmd, Name: "Feynman"})
}
