package mcp

import (
	"testing"

	"github.com/Bayern99/mark-go-go/internal/ipc"
)

func TestModernResultsAreDiscriminatedAndRequireRequestCapabilities(t *testing.T) {
	h, root := newSocketHarness(t)
	client, reply := connectExternal(t, startExternal(t, h), root)
	if !reply.OK {
		t.Fatal(reply.Error)
	}
	for _, kind := range []string{"server/discover", "tools/list", "tools/call", "resources/list", "ping"} {
		resp := client.call(kind, map[string]any{"name": toolReadDocument, "arguments": map[string]any{"path": "closed.md"}})
		result, _ := resp.Result.(map[string]any)
		if kind == "tools/list" || kind == "resources/list" {
			if result["ttlMs"] != float64(0) || result["cacheScope"] != "private" {
				t.Errorf("%s did not declare its cache policy", kind)
			}
		}
		if resp.Error != nil || result["resultType"] != "complete" {
			t.Errorf("%s resultType = %v, error = %+v", kind, result["resultType"], resp.Error)
		}
	}
	for _, meta := range []map[string]any{
		{ipc.MetaProtocolVersion: modernProtocolVersion},
		{ipc.MetaProtocolVersion: modernProtocolVersion, ipc.MetaClientCapabilities: nil},
		{ipc.MetaProtocolVersion: modernProtocolVersion, ipc.MetaClientCapabilities: []string{}},
		{ipc.MetaClientCapabilities: map[string]any{}},
	} {
		resp := client.raw(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/list", "params": map[string]any{"_meta": meta}})
		if resp.Error == nil || resp.Error.Code != errCodeInvalidParams {
			t.Errorf("request metadata %+v accepted: %+v", meta, resp)
		}
	}
}
