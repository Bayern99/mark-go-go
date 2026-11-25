package agentguide

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEveryHarnessIsToldWhereTheAppListens(t *testing.T) {
	socket := "/Users/reader/Library/Application Support/Mark Go Go/ipc.sock"
	for _, harness := range Harnesses {
		prompt := Prompt(harness.ID, socket)
		for _, want := range []string{socket, "read_document", "propose_edit", "VERSION_CONFLICT", "markgogo mcp"} {
			if !strings.Contains(prompt, want) {
				t.Errorf("%s prompt lacks %q", harness.Name, want)
			}
		}
		if !strings.HasPrefix(prompt, harness.Prompt) {
			t.Errorf("%s prompt opens with %q, want what its host needs first", harness.Name, strings.SplitN(prompt, "\n", 2)[0])
		}
	}
}

func TestAHarnessPromptSaysWhatThatHostHasToDo(t *testing.T) {
	cases := map[string]string{
		HarnessClaudeCode: "shell",
		HarnessCodex:      "sandbox",
	}
	for id, want := range cases {
		if prompt := Prompt(id, "/tmp/ipc.sock"); !strings.Contains(prompt, want) {
			t.Errorf("%s prompt does not mention %q: %q", id, want, prompt)
		}
	}
	if Prompt(HarnessOther, "/tmp/ipc.sock") == Prompt(HarnessCodex, "/tmp/ipc.sock") {
		t.Fatal("Other repeats what Codex is told")
	}
	if got := HarnessByID("no-such-harness"); got.ID != HarnessOther {
		t.Fatalf("unknown harness = %q, want Other", got.ID)
	}
}

func TestSkillIsAShortStaticDocumentOfTheCommands(t *testing.T) {
	skill := Skill()
	lines := strings.Count(skill, "\n")
	if lines < 30 || lines > 55 {
		t.Fatalf("the skill is %d lines; about 40 is what a reader skims", lines)
	}
	if !strings.HasPrefix(skill, "---\nname: markgogo\n") {
		t.Fatalf("the skill does not open with its front matter: %q", strings.SplitN(skill, "\n", 2)[0])
	}
	for _, want := range []string{"markgogo status", "markgogo read", "markgogo propose", "APP_NOT_RUNNING"} {
		if !strings.Contains(skill, want) {
			t.Errorf("the skill lacks %q", want)
		}
	}
	for _, unwanted := range []string{"http://", "localhost", "token"} {
		if strings.Contains(strings.ToLower(skill), unwanted) {
			t.Errorf("the skill mentions %q; the channel has no port and no token", unwanted)
		}
	}
}

func TestMCPServerConfigurationIsJustTheCommand(t *testing.T) {
	var config struct {
		Command string   `json:"command"`
		Args    []string `json:"args"`
	}
	if err := json.Unmarshal([]byte(MCPConfig), &config); err != nil {
		t.Fatalf("the configuration is not JSON: %v", err)
	}
	if config.Command != "markgogo" || len(config.Args) != 1 || config.Args[0] != "mcp" {
		t.Fatalf("configuration = %+v, want the markgogo mcp shim", config)
	}
	if strings.Contains(MCPConfig, "port") || strings.Contains(MCPConfig, "token") {
		t.Fatalf("the configuration carries more than a command: %s", MCPConfig)
	}
}

func TestInstallingTheSkillWritesIntoTheAppsOwnFolder(t *testing.T) {
	userData := t.TempDir()

	path, err := InstallSkill(userData)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(userData, "agent", "SKILL.md"); path != want {
		t.Fatalf("skill path = %q, want %q", path, want)
	}
	written, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(written) != Skill() {
		t.Fatal("the installed skill is not the text the page copies")
	}
	if info, err := os.Stat(path); err != nil || info.Mode().Perm() != 0o644 {
		t.Fatalf("skill file = %v, %v; want a readable file", info, err)
	}
	if _, err := InstallSkill(" "); err == nil {
		t.Fatal("installing the skill without a user data folder succeeded")
	}
}

func TestAgentGuidesIdentifyWhichOperationsTakeEffectImmediately(t *testing.T) {
	for _, guide := range []string{GeneralPrompt("/tmp/ipc.sock"), Skill()} {
		for _, phrase := range []string{"review", "immediately creates", "refuses to overwrite", "immediately replaces", "immediately snapshots"} {
			if !strings.Contains(guide, phrase) {
				t.Errorf("agent guide lacks %q", phrase)
			}
		}
	}
}
