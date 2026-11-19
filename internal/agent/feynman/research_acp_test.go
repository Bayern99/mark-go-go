package feynman

import (
	"encoding/json"
	"testing"
)

func TestResearchToolAndCommandUpdatesKeepTheirDetails(t *testing.T) {
	var events []ResearchUpdate
	s := &researchSession{id: "active", stopped: make(chan struct{}), emit: func(e ResearchUpdate) { events = append(events, e) }}
	h := &ResearchManager{}
	notify := func(id, update string) {
		h.update(s, "session/update", json.RawMessage(`{"sessionId":"`+id+`","update":`+update+`}`))
	}
	notify("other", `{"sessionUpdate":"tool_call","toolCallId":"hidden","title":"Other workspace"}`)
	notify("active", `{"sessionUpdate":"tool_call","toolCallId":"read-1","title":"Read notes","kind":"read","status":"in_progress","rawInput":{"path":"notes.md"},"locations":[{"path":"notes.md","line":4}]}`)
	notify("active", `{"sessionUpdate":"tool_call_update","toolCallId":"read-1","status":"completed","content":[{"type":"content","content":{"type":"text","text":"Paper notes."}},{"type":"diff","path":"notes.md","oldText":"First","newText":"Second"}]}`)
	notify("active", `{"sessionUpdate":"tool_call_update","toolCallId":"term-1","_meta":{"terminal_output":{"data":"done\n"},"terminal_exit":{"exit_code":0}}}`)
	notify("active", `{"sessionUpdate":"available_commands_update","availableCommands":[{"name":"lit","description":"Literature review","input":{"hint":"a topic"}}]}`)
	if len(events) != 4 {
		t.Fatalf("got %d events", len(events))
	}
	start, end, terminal := events[0].Tool, events[1].Tool, events[2].Tool
	if events[0].Kind != "tool" || start == nil || start.ToolCallID != "read-1" || start.Input != `{"path":"notes.md"}` || start.Locations[0].Line != 4 {
		t.Fatal(start)
	}
	if end == nil || end.Output != "Paper notes." || len(end.Diffs) != 1 || end.Diffs[0].NewText != "Second" || end.ToolCallID != start.ToolCallID {
		t.Fatal(end)
	}
	if terminal == nil || terminal.TerminalDelta != "done\n" || terminal.ExitCode == nil || *terminal.ExitCode != 0 {
		t.Fatal(terminal)
	}
	if cmd := events[3].Commands[0]; cmd.Name != "lit" || cmd.Description != "Literature review" || cmd.Input.Hint != "a topic" {
		t.Fatal(cmd)
	}
}

func TestResearchRichContentPreservesContextWithoutFetchingExternalMedia(t *testing.T) {
	read := func(update string) ResearchUpdate { return decodeResearchUpdate(json.RawMessage(update)) }
	thought := read(`{"sessionUpdate":"agent_thought_chunk","messageId":"m1","content":{"type":"text","text":"Inspecting sources."}}`)
	if thought.Kind != "thought" || thought.Text != "Inspecting sources." || thought.MessageID != "m1" {
		t.Fatal(thought)
	}
	plan := read(`{"sessionUpdate":"plan","entries":[{"content":"Review papers","priority":"high","status":"in_progress"}]}`)
	if plan.Kind != "plan" || plan.Plan[0].Status != "in_progress" {
		t.Fatal(plan)
	}
	usage := read(`{"sessionUpdate":"usage_update","used":54,"size":1024}`)
	if usage.Kind != "usage" || usage.Used != 54 || usage.Size != 1024 {
		t.Fatal(usage)
	}
	image := read(`{"sessionUpdate":"agent_message_chunk","content":{"type":"image","mimeType":"image/svg+xml","data":"PHN2Zz4=","uri":"https://external.invalid/pixel"}}`)
	if image.Kind != "assistant" || image.Image != "" || image.Text != "Image preview unavailable (image/svg+xml)" {
		t.Fatal(image)
	}
	resource := read(`{"sessionUpdate":"user_message_chunk","content":{"type":"resource","resource":{"uri":"file:///draft.md","text":"Research draft"}}}`)
	if resource.Text != "file:///draft.md\nResearch draft" {
		t.Fatal(resource)
	}
	const pixel = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAIAAACQd1PeAAAADElEQVR4nGNgSPsPAAHPAWYvlgrbAAAAAElFTkSuQmCC"
	image = read(`{"sessionUpdate":"agent_message_chunk","content":{"type":"image","mimeType":"image/png","data":"` + pixel + `"}}`)
	if image.Image != "data:image/png;base64,"+pixel {
		t.Fatal(image)
	}
	toolImage := read(`{"sessionUpdate":"tool_call_update","toolCallId":"read-image","content":[{"type":"content","content":{"type":"text","text":"raw image result as JSON"}}],"rawOutput":{"content":[{"type":"image","mimeType":"image/png","data":"` + pixel + `"}]}}`)
	if toolImage.Tool.Output != "Image (image/png)" || len(toolImage.Tool.Images) != 1 || toolImage.Tool.Images[0] != image.Image {
		t.Fatal("lost native tool image or exposed its base64 dump as prose")
	}

}
