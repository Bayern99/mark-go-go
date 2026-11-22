package acp

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Bayern99/mark-go-go/internal/genui"
	"github.com/Bayern99/mark-go-go/internal/openuilang"
)

var rehearsalPlan = []string{
	`root = Col([goal_src, phase2_src])`,
	`goal_src = Source(["report"], [Card([Title("Cluster & Upgrade", "md"), Text("PostgreSQL 14.11 to 17.2", "sm")])])`,
	`phase2_src = Source(["detail"], [phase2_card])`,
	`phase2_card = Card([phase2_head, phase2_steps, Divider(), phase2_exit])`,
	`phase2_head = Col([Title("Phase 2: Rehearsal", "md"), Caption("Runs from December 7 to December 9.")], 1)`,
	`phase2_steps = List([ListItem("Build a new 17.2 replica set.", "Expected initial sync: 9 hours."), ListItem("Rehearse the cutover twice on staging.")], "number")`,
	`phase2_exit = Text("Both rehearsals meet their time limits.", "sm")`,
}

var rehearsalUpdate = []string{
	`phase2_src = Source(["detail"], [phase2_card_updated])`,
	`phase2_card_updated = Card([phase2_head, phase2_steps_updated, Divider(), phase2_exit])`,
	`phase2_steps_updated = List([ListItem("Build a new 17.2 replica set.", "Expected initial sync: 11 hours."), ListItem("Rehearse the cutover twice on staging.")], "number")`,
}

func statementsFull(statements ...string) string {
	return `{"sources":{"report":{"heading":"Report","occurrence":1},"detail":{"heading":"Detail","occurrence":1}},"a2ui":[{"version":"1.0","createSurface":{"surfaceId":"view","catalogId":"markgogo.dev:genui/v3","components":` + statementList(statements) + `}}]}`
}

func generatedStatementView(t *testing.T, statements ...string) string {
	t.Helper()
	viewPath := filepath.Join(t.TempDir(), ".notes.md.genui.json")
	if err, _, _ := driveGeneration(t, fakeSpec{Out: statementsFull(statements...)}, nil, validated(func(job *GenerationJob) { job.ViewPath = viewPath })); err != nil {
		t.Fatalf("full generation: %v", err)
	}
	return viewPath
}

func validated(mutate func(*GenerationJob)) func(*GenerationJob) {
	return func(job *GenerationJob) {
		mutate(job)
		job.Req.Language = genui.StatementLanguage
		job.Validate = nil
	}
}

func writtenMessages(t *testing.T, viewPath string) []json.RawMessage {
	t.Helper()
	raw, err := os.ReadFile(viewPath)
	if err != nil {
		t.Fatal(err)
	}
	var view struct {
		A2UI []json.RawMessage `json:"a2ui"`
	}
	if err := json.Unmarshal(raw, &view); err != nil {
		t.Fatalf("parse view: %v", err)
	}
	return view.A2UI
}

func updateStatements(t *testing.T, message json.RawMessage) []string {
	t.Helper()
	var update struct {
		UpdateComponents struct {
			Components []string `json:"components"`
		} `json:"updateComponents"`
	}
	if err := json.Unmarshal(message, &update); err != nil {
		t.Fatalf("parse update message: %v", err)
	}
	return update.UpdateComponents.Components
}

func reopenedStatements(t *testing.T, viewPath string) []string {
	t.Helper()
	raw, err := os.ReadFile(viewPath)
	if err != nil {
		t.Fatal(err)
	}
	view, err := genui.ReadStatementView(raw)
	if err != nil {
		t.Fatalf("reopen view: %v", err)
	}
	text := ""
	for _, batch := range view.Statements {
		text = openuilang.Overlay(text, strings.Join(batch, "\n"))
	}
	return openuilang.Names(text)
}

func TestAnUpdateDeletesTheStatementsItLeavesUnreferenced(t *testing.T) {
	viewPath := generatedStatementView(t, rehearsalPlan...)
	created := writtenMessages(t, viewPath)[0]

	if err, _, _ := driveGeneration(t, fakeSpec{Out: statementsUpdate(rehearsalUpdate...)}, nil, validated(updateJob(viewPath, "phase2_src"))); err != nil {
		t.Fatalf("update: %v", err)
	}

	messages := writtenMessages(t, viewPath)
	if len(messages) != 2 {
		t.Fatalf("the view holds %d messages, want the first and the update", len(messages))
	}
	if !bytes.Equal(messages[0], created) {
		t.Errorf("the createSurface message changed:\n got %s\nwant %s", messages[0], created)
	}
	wantUpdate := append(slices.Clone(rehearsalUpdate), `phase2_card = null`, `phase2_steps = null`)
	if got := updateStatements(t, messages[1]); !slices.Equal(got, wantUpdate) {
		t.Errorf("the update message holds\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(wantUpdate, "\n"))
	}
	wantReopened := []string{"root", "goal_src", "phase2_src", "phase2_head", "phase2_exit", "phase2_card_updated", "phase2_steps_updated"}
	if got := reopenedStatements(t, viewPath); !slices.Equal(got, wantReopened) {
		t.Errorf("the reopened view has the statements %v, want %v", got, wantReopened)
	}
}

func TestAnUpdateThatLeavesNothingUnreferencedIsWrittenAsTheAgentWroteIt(t *testing.T) {
	viewPath := generatedStatementView(t, rehearsalPlan...)
	out := statementsUpdate(`phase2_src = Source(["detail"], [phase2_card, phase2_note])`, `phase2_note = Caption("Schema changes are frozen until cutover.")`)
	if err, _, _ := driveGeneration(t, fakeSpec{Out: out}, nil, validated(updateJob(viewPath, "phase2_src"))); err != nil {
		t.Fatalf("update: %v", err)
	}
	var agent struct {
		A2UI []json.RawMessage `json:"a2ui"`
	}
	_ = json.Unmarshal([]byte(out), &agent)
	if got, want := compactJSON(t, string(writtenMessages(t, viewPath)[1])), compactJSON(t, string(agent.A2UI[0])); got != want {
		t.Errorf("the update message is %s, want the agent's %s", got, want)
	}
}

func TestAnUpdateKeepsTheVariablesItNoLongerUses(t *testing.T) {
	viewPath := generatedStatementView(t,
		`root = Col([phase1_src])`,
		`$p1_backup = true`,
		`$p1_replay = false`,
		`phase1_src = Source(["report"], [phase1_tasks])`,
		`phase1_tasks = Col([Checkbox($p1_backup, "Restore last night's backup"), Checkbox($p1_replay, "Replay 24 hours of traffic")], 2)`,
	)
	out := statementsUpdate(`phase1_src = Source(["report"], [Checkbox($p1_backup, "Restore last night's backup")])`)
	if err, _, _ := driveGeneration(t, fakeSpec{Out: out}, nil, validated(updateJob(viewPath, "phase1_src"))); err != nil {
		t.Fatalf("update: %v", err)
	}
	wantUpdate := []string{`phase1_src = Source(["report"], [Checkbox($p1_backup, "Restore last night's backup")])`, `phase1_tasks = null`}
	if got := updateStatements(t, writtenMessages(t, viewPath)[1]); !slices.Equal(got, wantUpdate) {
		t.Errorf("the update message holds %q, want %q", got, wantUpdate)
	}
	wantReopened := []string{"root", "$p1_backup", "$p1_replay", "phase1_src"}
	if got := reopenedStatements(t, viewPath); !slices.Equal(got, wantReopened) {
		t.Errorf("the reopened view has the statements %v, want %v", got, wantReopened)
	}
}

func TestALaterUpdateMayReuseTheNameOfADeletedStatementAndLeavesEarlierMessagesAlone(t *testing.T) {
	viewPath := generatedStatementView(t, rehearsalPlan...)
	if err, _, _ := driveGeneration(t, fakeSpec{Out: statementsUpdate(rehearsalUpdate...)}, nil, validated(updateJob(viewPath, "phase2_src"))); err != nil {
		t.Fatalf("first update: %v", err)
	}
	earlier := writtenMessages(t, viewPath)

	second := []string{
		`phase2_src = Source(["detail"], [phase2_card])`,
		`phase2_card = Card([phase2_head, Text("Both rehearsals are done.", "sm")])`,
	}
	if err, _, _ := driveGeneration(t, fakeSpec{Out: statementsUpdate(second...)}, nil, validated(updateJob(viewPath, "phase2_src"))); err != nil {
		t.Fatalf("second update: %v", err)
	}

	messages := writtenMessages(t, viewPath)
	if len(messages) != 3 {
		t.Fatalf("the view holds %d messages, want three", len(messages))
	}
	for i, message := range earlier {
		if !bytes.Equal(messages[i], message) {
			t.Errorf("message %d changed:\n got %s\nwant %s", i, messages[i], message)
		}
	}
	wantUpdate := append(slices.Clone(second), `phase2_exit = null`, `phase2_card_updated = null`, `phase2_steps_updated = null`)
	if got := updateStatements(t, messages[2]); !slices.Equal(got, wantUpdate) {
		t.Errorf("the second update message holds %q, want %q", got, wantUpdate)
	}
	wantReopened := []string{"root", "goal_src", "phase2_src", "phase2_head", "phase2_card"}
	if got := reopenedStatements(t, viewPath); !slices.Equal(got, wantReopened) {
		t.Errorf("the reopened view has the statements %v, want %v", got, wantReopened)
	}
}

func TestAnUpdateDeletesAStatementAnEarlierMessageLeftUnreferenced(t *testing.T) {
	viewPath := generatedStatementView(t,
		`root = Col([goal_src, phase2_src])`,
		`goal_src = Source(["report"], [Text("Move the orders cluster to 17.2.")])`,
		`phase2_src = Source(["detail"], [Text("Expected initial sync: 9 hours.")])`,
		`phase2_steps = List(["Rehearse the cutover twice on staging."], "number")`,
	)
	out := statementsUpdate(`goal_src = Source(["report"], [Text("Move the orders cluster to 17.4.")])`)
	if err, _, _ := driveGeneration(t, fakeSpec{Out: out}, nil, validated(updateJob(viewPath, "goal_src"))); err != nil {
		t.Fatalf("update: %v", err)
	}
	wantUpdate := []string{`goal_src = Source(["report"], [Text("Move the orders cluster to 17.4.")])`, `phase2_steps = null`}
	if got := updateStatements(t, writtenMessages(t, viewPath)[1]); !slices.Equal(got, wantUpdate) {
		t.Errorf("the update message holds %q, want %q", got, wantUpdate)
	}
	wantReopened := []string{"root", "goal_src", "phase2_src"}
	if got := reopenedStatements(t, viewPath); !slices.Equal(got, wantReopened) {
		t.Errorf("the reopened view has the statements %v, want %v", got, wantReopened)
	}
}
