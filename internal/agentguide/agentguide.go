// Package agentguide is the one copy of what an external agent is told about
// Mark Go Go: the prompt `markgogo agent-prompt` prints and Settings / AI
// Integration copies, the static SKILL.md, and the MCP configuration a client
// pastes.
package agentguide

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Bayern99/mark-go-go/internal/fsutil"
)

// Harness IDs. Each one names an external agent the page writes a prompt for.
const (
	HarnessClaudeCode = "claude-code"
	HarnessCodex      = "codex"
	HarnessPi         = "pi"
	HarnessOther      = "other"
)

// Harness is one external agent: what it is like to connect, and the line the
// reader runs to hand it the same tools.
type Harness struct {
	ID, Name string
	// Summary says how this harness reaches the workspace.
	Summary string
	// Connect is the command or step that gives the harness the tools. It is
	// empty when there is nothing to run beyond the prompt itself.
	Connect string
	// Prompt opens the shared instructions with what this harness needs to
	// know.
	Prompt string
}

// Harnesses are the agents Settings / AI Integration offers, in the order it
// shows them.
var Harnesses = []Harness{
	{
		ID: HarnessClaudeCode, Name: "Claude Code",
		Summary: "A shell tool: it runs the markgogo command itself, or adds the stdio shim as an MCP server.",
		Connect: "claude mcp add markgogo -- markgogo mcp",
		Prompt:  "You have a shell. Run the markgogo command; if you would rather only speak MCP, the stdio server above reaches the same tools.",
	},
	{
		ID: HarnessCodex, Name: "Codex",
		Summary: "A sandboxed tool: the shim reaches the same socket, but the sandbox has to allow it.",
		Connect: "codex mcp add markgogo -- markgogo mcp",
		Prompt:  "You run in a sandbox. Register the stdio server above, and if the sandbox denies the app's socket, say so instead of retrying.",
	},
	{
		ID: HarnessPi, Name: "Pi",
		Summary: "A shell tool: it runs the markgogo command, or reads the SKILL.md below.",
		Connect: "Add the prompt or the SKILL.md to what Pi reads, such as AGENTS.md.",
		Prompt:  "You have a shell and read the project's instructions. Run the markgogo command, or follow the SKILL.md the user installed.",
	},
	{
		ID: HarnessOther, Name: "Other",
		Summary: "Any agent that can run a command or speak MCP over stdio.",
		Connect: MCPConfig,
		Prompt:  "Connect to the app the way your own tools allow: the markgogo command, or the stdio server above.",
	},
}

// HarnessByID finds a harness; an unknown ID is Other, so a saved or pasted
// value never leaves the page without a prompt.
func HarnessByID(id string) Harness {
	for _, harness := range Harnesses {
		if harness.ID == id {
			return harness
		}
	}
	return Harnesses[len(Harnesses)-1]
}

// MCPConfig is the JSON object an MCP client is given: a command and its
// arguments, no port and no token.
const MCPConfig = `{"command":"markgogo","args":["mcp"]}`

// GeneralPrompt is the instructions with no harness in mind: what
// `markgogo agent-prompt` prints.
func GeneralPrompt(socketPath string) string { return fmt.Sprintf(sharedPrompt, socketPath) }

// Prompt is the instructions to paste into a harness. The socket path is named
// because a reader may want to check that the app listens where the agent
// looks.
func Prompt(harnessID, socketPath string) string {
	harness := HarnessByID(harnessID)
	var out strings.Builder
	if line := strings.TrimSpace(harness.Prompt); line != "" {
		fmt.Fprintf(&out, "%s\n\n", line)
	}
	out.WriteString(GeneralPrompt(socketPath))
	return out.String()
}

// sharedPrompt is what every harness is told. It names the tools by the names
// the channel and the CLI both use.
const sharedPrompt = `You are reading and editing the Markdown workspace that Mark Go Go has open.

The app listens at %s. Use the markgogo CLI, or ` + "`markgogo mcp`" + ` as an MCP stdio server when your client only speaks MCP. Both bind to the workspace that contains your working directory.

Read before you write:
- get_workspace lists the root, the open documents and a bounded list of Markdown files, with a truncated flag.
- read_document (or ` + "`markgogo read <document>`" + `) takes a documentId or a workspace-relative path, and may return one heading section or a line range. It always returns the document version.

Every change to existing text goes through propose_edit (or ` + "`markgogo propose`" + `): pass exactly one of proposedText, oldText with newText, or startLine with endLine and newText plus the baseVersion you read. The user reviews the proposal in the app and applies it; the tool only returns its id, revision, hunk count and a one-line summary.

create_document (or ` + "`markgogo new`" + `) creates a new file and refuses to overwrite. write_genui_view validates a GenUI candidate against the catalog before writing the document's .genui.json and returns the version it wrote (2 for A2UI, 3 for openui-lang; get the format with ` + "`markgogo genui format`" + `). create_checkpoint snapshots a saved document.

Errors carry a stable code: APP_NOT_RUNNING, WORKSPACE_NOT_OPEN, VERSION_CONFLICT, NOT_FOUND, PATH_DENIED or INVALID_ARGS, with a hint. When APP_NOT_RUNNING says the connection closed mid-request, the write may or may not have happened: read the document again instead of retrying blindly.
`

// Skill is the static SKILL.md: the same instructions in the shape a harness
// that reads skills expects.
func Skill() string { return skill }

const skill = `---
name: markgogo
description: Read and edit the Markdown workspace that Mark Go Go has open.
---

# markgogo

` + "`markgogo`" + ` is Mark Go Go's command for an external agent. It reaches the running
app over the local socket the reader opens in Settings / AI Integration. Run
` + "`markgogo status`" + ` first: it names the workspace your working directory resolves
to, or says the app is not running.

## Read before you write

- ` + "`markgogo read <document>`" + ` takes an open document id or a workspace-relative
  path, and always returns the version.
- ` + "`markgogo read <document> --sections`" + ` lists headings; add ` + "`--heading <text>`" + ` or
  ` + "`--section <id>`" + ` to read one, and ` + "`--notes`" + ` for the reader's notes.
- ` + "`markgogo read <document> --lines <start>:<end>`" + ` reads a range and returns the
  version a line-range edit needs.
- ` + "`markgogo open <document> --heading <text>`" + ` brings the reader to a place in a
  document.

## Propose instead of writing

` + "`markgogo propose`" + ` stages a change the reader accepts in Mark Go Go's review
panel. Pass exactly one of:

- ` + "`--old <text> --new <text>`" + ` to replace one occurrence,
- ` + "`--lines <start>:<end> --new <text> --base-version <n>`" + ` to replace a range
  you read,
- ` + "`--from <file|->`" + ` for the whole document.

## Make and annotate documents

- ` + "`markgogo new <path> --from <file|->`" + ` creates a file and refuses to
  overwrite.
- ` + "`markgogo genui format`" + ` prints the GenUI view format;
  ` + "`markgogo genui write <document> --candidate <file|->`" + ` validates and saves a
  reading view.
- ` + "`markgogo checkpoint <document> --label <text>`" + ` snapshots a saved document.

## What failures mean

` + "`APP_NOT_RUNNING`" + `, ` + "`WORKSPACE_NOT_OPEN`" + `, ` + "`VERSION_CONFLICT`" + `, ` + "`NOT_FOUND`" + `,
` + "`PATH_DENIED`" + ` and ` + "`INVALID_ARGS`" + ` come back as JSON with a hint. After a lost
connection the write may or may not have happened: read the document again
rather than retrying blindly.
`

// SkillPath is where the app keeps the SKILL.md it writes for the reader. It
// stays in the app's own folder: each harness reads skills from its own place,
// and Mark Go Go does not write into another program's configuration.
func SkillPath(userDataDir string) string {
	return filepath.Join(userDataDir, "agent", "SKILL.md")
}

// InstallSkill writes the static SKILL.md into the app's folder and reports
// where it went.
func InstallSkill(userDataDir string) (string, error) {
	if strings.TrimSpace(userDataDir) == "" {
		return "", fmt.Errorf("Mark Go Go has no user data folder for the skill")
	}
	path := SkillPath(userDataDir)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return "", err
	}
	if err := fsutil.WriteFileAtomic(path, []byte(skill)); err != nil {
		return "", err
	}
	return path, nil
}
