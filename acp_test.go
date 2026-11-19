package main

import (
	"bufio"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestACPStopsWhenAnOutputRecordExceedsTheReadLimit(t *testing.T) {
	if os.Getenv("MGO_LARGE_ACP_RESULT") == "1" {
		in := bufio.NewScanner(os.Stdin)
		if in.Scan() {
			_, _ = os.Stdout.WriteString(`{"jsonrpc":"2.0","id":1,"result":{"text":"`)
			chunk := strings.Repeat("x", 64<<10)
			for range 256 {
				_, _ = os.Stdout.WriteString(chunk)
			}
			_, _ = os.Stdout.WriteString("\"}}\n")
		}
		for in.Scan() {
		}
		os.Exit(0)
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestACPStopsWhenAnOutputRecordExceedsTheReadLimit$")
	cmd.Env = append(os.Environ(), "MGO_LARGE_ACP_RESULT=1")
	agent, err := newACPAgent(cmd, newLimitBuffer(1024))
	if err != nil {
		t.Fatal(err)
	}
	agent.start()
	t.Cleanup(agent.kill)
	finished := make(chan error, 1)
	go func() { finished <- agent.call("session/prompt", nil, nil) }()
	select {
	case err := <-finished:
		if err == nil || !strings.Contains(err.Error(), "16 MiB") {
			t.Fatalf("missing read limit in %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("pending ACP call did not finish after its result exceeded the read limit")
	}
}
