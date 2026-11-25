package pirpc

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	if os.Getenv("MGO_FIXTURE_PIRPC") == "1" {
		runPeerFixture()
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func TestSplitJSONLLinesAcceptsCRLFAndAFinalRecordWithoutANewline(t *testing.T) {
	cases := []struct {
		name  string
		data  string
		atEOF bool
		want  []string
	}{
		{name: "one record", data: "{\"a\":1}\n", want: []string{"{\"a\":1}"}},
		{name: "CRLF", data: "{\"a\":1}\r\n", want: []string{"{\"a\":1}"}},
		{name: "two records", data: "{\"a\":1}\n{\"b\":2}\n", want: []string{"{\"a\":1}", "{\"b\":2}"}},
		{name: "final record without a newline", data: "{\"a\":1}", atEOF: true, want: []string{"{\"a\":1}"}},
		{name: "empty at EOF", data: "", atEOF: true, want: nil},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			var got []string
			rest := []byte(tt.data)
			for {
				advance, token, err := SplitJSONLLines(rest, tt.atEOF)
				if err != nil {
					t.Fatalf("split: %v", err)
				}
				if advance == 0 {
					break
				}
				rest = rest[advance:]
				if len(token) > 0 {
					got = append(got, string(token))
				}
			}
			if strings.Join(got, "|") != strings.Join(tt.want, "|") {
				t.Fatalf("records = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCallCorrelatesResponsesAndDeliversEventsAndUIRequests(t *testing.T) {
	client, events, ui := startPeer(t)
	defer client.Kill()

	var echoed struct {
		Word string `json:"word"`
	}
	if err := client.Call("echo", map[string]any{"word": "hello"}, &echoed); err != nil {
		t.Fatalf("echo: %v", err)
	}
	if echoed.Word != "hello" {
		t.Fatalf("echo data = %+v, want the word", echoed)
	}
	if err := client.Call("fail", nil, nil); err == nil || err.Error() != "boom" {
		t.Fatalf("failed command returned %v, want the peer's message", err)
	}
	if err := client.Call("fail_silently", nil, nil); err == nil || err.Error() != "ThePeer fail_silently command failed." {
		t.Fatalf("failed command without a message returned %v", err)
	}
	if err := client.Call("nudge", nil, nil); err != nil {
		t.Fatalf("nudge: %v", err)
	}
	select {
	case event := <-events:
		if event.form != `{"type":"session_event","detail":"moved"}` {
			t.Fatalf("event = %s", event.form)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no session event arrived")
	}
	select {
	case request := <-ui:
		if request != `{"type":"extension_ui_request","id":"ui-1","method":"notify","message":"hi"}` {
			t.Fatalf("ui request = %s", request)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no extension UI request arrived")
	}
	if err := client.SendUIResponse("ui-1", map[string]any{"cancelled": true}); err != nil {
		t.Fatalf("respond: %v", err)
	}
}

func TestCallReportsTheProgramsOwnExitAndErrorOutput(t *testing.T) {
	client, _, _ := startPeer(t)
	defer client.Kill()

	err := client.Call("die", nil, nil)
	if err == nil {
		t.Fatal("a command the program never answered must fail")
	}
	if !strings.Contains(err.Error(), "ThePeer exited before answering die.") {
		t.Errorf("error = %q, want the program's name and the command", err)
	}
	if !strings.Contains(err.Error(), "exploded") {
		t.Errorf("error = %q, want the program's error output", err)
	}
}

func TestShutdownEndsTheProgramWithoutKillingIt(t *testing.T) {
	client, _, _ := startPeer(t)
	client.Shutdown(5 * time.Second)
	select {
	case <-client.Exited():
	case <-time.After(5 * time.Second):
		t.Fatal("the program did not exit")
	}
}

func TestKillStopsTheProgramAndClosesExited(t *testing.T) {
	client, _, _ := startPeer(t)
	var echoed struct {
		Word string `json:"word"`
	}
	if err := client.Call("echo", map[string]any{"word": "up"}, &echoed); err != nil {
		t.Fatalf("echo: %v", err)
	}
	go client.Kill()
	select {
	case <-client.Exited():
	case <-time.After(5 * time.Second):
		t.Fatal("Exited stayed open after Kill")
	}
	if err := client.Call("echo", map[string]any{"word": "late"}, nil); err == nil {
		t.Fatal("a command sent to a stopped program must fail")
	}
}

type capturedRecord struct{ form string }

func startPeer(t *testing.T) (*Client, chan capturedRecord, chan string) {
	t.Helper()
	events := make(chan capturedRecord, 8)
	ui := make(chan string, 8)
	cmd := exec.Command(os.Args[0])
	cmd.Env = append(os.Environ(), "MGO_FIXTURE_PIRPC=1")
	client, err := New(Config{Command: cmd, Name: "ThePeer"})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	client.SetHandlers(
		func(eventType string, raw json.RawMessage) {
			events <- capturedRecord{form: string(raw)}
		},
		func(raw json.RawMessage) { ui <- string(raw) },
	)
	client.Start()
	return client, events, ui
}

// runPeerFixture is the program under test: a peer that answers the commands
// the tests send and nothing else.
func runPeerFixture() {
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 0, 64<<10), MaxRecordBytes)
	send := func(record map[string]any) {
		data, _ := json.Marshal(record)
		_, _ = os.Stdout.Write(append(data, '\r', '\n'))
	}
	_, _ = os.Stdout.WriteString("a banner that is not a record\r\n")
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		var record map[string]any
		if json.Unmarshal(line, &record) != nil {
			continue
		}
		id, _ := record["id"].(string)
		response := func(data any) {
			answer := map[string]any{"id": id, "type": "response", "command": record["type"], "success": true}
			if data != nil {
				answer["data"] = data
			}
			send(answer)
		}
		switch record["type"] {
		case "echo":
			response(map[string]any{"word": record["word"]})
		case "fail":
			send(map[string]any{"id": id, "type": "response", "command": "fail", "success": false, "error": "boom"})
		case "fail_silently":
			send(map[string]any{"id": id, "type": "response", "command": "fail_silently", "success": false})
		case "nudge":
			_, _ = os.Stdout.WriteString("{\"type\":\"session_event\",\"detail\":\"moved\"}\n")
			_, _ = os.Stdout.WriteString("{\"type\":\"extension_ui_request\",\"id\":\"ui-1\",\"method\":\"notify\",\"message\":\"hi\"}\n")
			response(nil)
		case "die":
			_, _ = os.Stderr.WriteString("exploded")
			os.Exit(1)
		}
	}
}
