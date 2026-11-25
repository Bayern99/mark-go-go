// Command markgogo is the thin Mark Go Go client for external agents. It talks
// to the running app over its local socket and reads files itself when the app
// is not running; it does not link the desktop UI.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Bayern99/mark-go-go/internal/appversion"
	"github.com/Bayern99/mark-go-go/internal/genuikit"
	"github.com/Bayern99/mark-go-go/internal/ipc"
	"github.com/Bayern99/mark-go-go/internal/markdown"
)

// Exit codes. Each stable app error code has its own, so a script can branch on
// the status alone.
const (
	exitOK               = 0
	exitFailure          = 1
	exitUsage            = 2
	exitAppNotRunning    = 3
	exitWorkspaceNotOpen = 4
	exitVersionConflict  = 5
	exitNotFound         = 6
	exitPathDenied       = 7
)

const usage = `markgogo reads and edits the Markdown files of a Mark Go Go workspace.

Usage:
  markgogo status [--json]
  markgogo read <document> [--sections | --notes] [--section <id> | --heading <text>]
                            [--lines <start>:<end>] [--json]
  markgogo propose <document> --old <text> --new <text> [--summary <text>] [--json]
  markgogo propose <document> --lines <start>:<end> --new <text> --base-version <n>
  markgogo propose <document> --from <file|-> [--summary <text>]
  markgogo new <path> (--from <file|-> | --content <text>) [--open] [--json]
  markgogo open <document> [--section <id> | --heading <text>] [--json]
  markgogo genui format
  markgogo genui write <document> --candidate <file|-> [--json]
  markgogo checkpoint <document> [--label <text>] [--json]
  markgogo agent-prompt
  markgogo mcp
  markgogo --version

<document> is an open document id or a workspace-relative path. Every command
that needs the app returns APP_NOT_RUNNING while Mark Go Go is closed, except
read and genui format, which read from disk. The app's local channel is off
until it is enabled in Mark Go Go.`

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

type app struct {
	stdout io.Writer
	stderr io.Writer
	stdin  io.Reader
	client ipc.Client
}

// run is the whole CLI, so tests can drive it without a process.
func run(argv []string, stdout, stderr io.Writer) int {
	return runWith(os.Stdin, argv, stdout, stderr)
}

func runWith(stdin io.Reader, argv []string, stdout, stderr io.Writer) int {
	cli := &app{stdout: stdout, stderr: stderr, stdin: stdin}
	if len(argv) == 0 {
		cli.fail(fmt.Errorf("%w: pass a command", ipc.Errorf(ipc.CodeInvalidArgs, "")))
		fmt.Fprintln(stderr, usage)
		return exitUsage
	}
	dir, err := ipc.UserDataDir()
	if err != nil {
		cli.fail(ipc.Errorf(ipc.CodeAppNotRunning, "Cannot find the user data folder: %v.", err))
		return exitAppNotRunning
	}
	cwd, err := os.Getwd()
	if err != nil {
		cwd = ""
	}
	cli.client = ipc.Client{
		SocketPath: ipc.SocketPath(dir),
		CWD:        cwd,
		Name:       "markgogo-cli",
		Version:    appversion.Version,
		Timeout:    10 * time.Second,
	}

	command, rest := argv[0], argv[1:]
	switch command {
	case "help", "-h", "--help":
		fmt.Fprintln(stdout, usage)
		return exitOK
	case "-v", "--version", "version":
		return cli.version(rest)
	case "status":
		return cli.runStatus(rest)
	case "read":
		return cli.runRead(rest)
	case "propose":
		return cli.runPropose(rest)
	case "new":
		return cli.runNew(rest)
	case "open":
		return cli.runOpen(rest)
	case "genui":
		return cli.runGenUI(rest)
	case "checkpoint":
		return cli.runCheckpoint(rest)
	case "agent-prompt":
		return cli.runAgentPrompt(rest)
	case "mcp":
		return cli.runMCP(rest)
	default:
		fmt.Fprintf(stderr, "markgogo: unknown command %q\n\n%s\n", command, usage)
		return exitUsage
	}
}

// fail reports an error the way a script can read it, and never on stdout.
func (a *app) fail(err error) {
	var appErr *ipc.AppError
	if !errors.As(err, &appErr) {
		var rpcErr *ipc.RPCError
		if errors.As(err, &rpcErr) {
			appErr = &ipc.AppError{Code: ipc.CodeInvalidArgs, Message: rpcErr.Message}
		} else {
			appErr = &ipc.AppError{Code: ipc.CodeInvalidArgs, Message: err.Error()}
		}
	}
	_ = json.NewEncoder(a.stderr).Encode(map[string]any{"error": appErr})
}

func exitCodeFor(err error) int {
	switch ipc.CodeOf(err) {
	case "":
		return exitFailure
	case ipc.CodeAppNotRunning:
		return exitAppNotRunning
	case ipc.CodeWorkspaceNotOpen:
		return exitWorkspaceNotOpen
	case ipc.CodeVersionConflict:
		return exitVersionConflict
	case ipc.CodeNotFound:
		return exitNotFound
	case ipc.CodePathDenied:
		return exitPathDenied
	default:
		return exitUsage
	}
}

func (a *app) write(payload any) int {
	encoder := json.NewEncoder(a.stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(payload); err != nil {
		a.fail(err)
		return exitFailure
	}
	return exitOK
}

// deliver writes a tool's structured result and maps its error code to an exit
// status.
func (a *app) deliver(result json.RawMessage, err error) int {
	if err != nil {
		a.fail(err)
		return exitCodeFor(err)
	}
	return a.deliverPayload(result)
}

func (a *app) deliverPayload(result json.RawMessage) int {
	var payload any
	if err := json.Unmarshal(result, &payload); err != nil {
		a.fail(err)
		return exitFailure
	}
	return a.write(payload)
}

// flags builds a flag set that stops at the first positional argument, so
// `read doc.md --sections` reads the way it is written in the help.
func (a *app) flags(name string) (*flag.FlagSet, *bool) {
	set := flag.NewFlagSet(name, flag.ContinueOnError)
	set.SetOutput(io.Discard)
	jsonOut := set.Bool("json", true, "print JSON (the default)")
	return set, jsonOut
}

// parse splits flags from positionals and stops at "--".
func parse(set *flag.FlagSet, args []string) ([]string, error) {
	var positional []string
	rest := args
	for {
		if err := set.Parse(rest); err != nil {
			return nil, err
		}
		if set.NArg() == 0 {
			return positional, nil
		}
		positional = append(positional, set.Arg(0))
		rest = set.Args()[1:]
	}
}

func (a *app) version(args []string) int {
	set, _ := a.flags("version")
	if _, err := parse(set, args); err != nil {
		a.fail(err)
		return exitUsage
	}
	return a.write(map[string]any{"name": "markgogo", "version": appversion.Version})
}

type discovery struct {
	Running           bool     `json:"running"`
	Socket            string   `json:"socket"`
	UserDataDir       string   `json:"userDataDir"`
	Root              string   `json:"root,omitempty"`
	AppVersion        string   `json:"appVersion,omitempty"`
	SupportedVersions []string `json:"supportedVersions,omitempty"`
	Instructions      string   `json:"instructions,omitempty"`
}

func (a *app) runStatus(args []string) int {
	set, _ := a.flags("status")
	if _, err := parse(set, args); err != nil {
		a.fail(err)
		return exitUsage
	}
	info := discovery{
		Socket:      a.client.SocketPath,
		UserDataDir: filepath.Dir(a.client.SocketPath),
	}
	result, hello, err := a.client.CallWithHello("server/discover", nil)
	if err != nil {
		info.Running = false
		if code := ipc.CodeOf(err); code != ipc.CodeAppNotRunning && code != ipc.CodeWorkspaceNotOpen {
			a.fail(err)
			return exitCodeFor(err)
		}
		if hello != nil && hello.Root != "" {
			info.Root = hello.Root
		}
		return a.write(info)
	}
	info.Running = true
	if hello != nil {
		info.Root = hello.Root
		info.AppVersion = hello.AppVersion
		info.SupportedVersions = hello.ProtocolVersions
	}
	var found struct {
		Instructions string `json:"instructions"`
	}
	_ = json.Unmarshal(result, &found)
	info.Instructions = found.Instructions
	return a.write(info)
}

func (a *app) runRead(args []string) int {
	set, _ := a.flags("read")
	sections := set.Bool("sections", false, "print the heading outline instead of the text")
	notes := set.Bool("notes", false, "print the reader notes instead of the text")
	section := set.String("section", "", "read the section with this id")
	heading := set.String("heading", "", "read the section with this heading")
	lines := set.String("lines", "", "read this 1-based line range, as start:end")
	positional, err := parse(set, args)
	if err != nil {
		a.fail(err)
		return exitUsage
	}
	if len(positional) != 1 {
		a.fail(ipc.Errorf(ipc.CodeInvalidArgs, "read takes one document id or path."))
		return exitUsage
	}
	document := positional[0]
	startLine, endLine, err := parseLines(*lines)
	if err != nil {
		a.fail(err)
		return exitUsage
	}
	mode := 0
	for _, on := range []bool{*sections, *notes, *section != "" || *heading != "" || *lines != ""} {
		if on {
			mode++
		}
	}
	if mode > 1 {
		a.fail(ipc.Errorf(ipc.CodeInvalidArgs, "read takes one of --sections, --notes, or a section or line selection."))
		return exitUsage
	}

	if *sections || *notes {
		kind := "sections"
		if *notes {
			kind = "notes"
		}
		result, err := a.client.Call("resources/read", map[string]any{"uri": "markgogo://document/" + escapeDocument(document) + "/" + kind})
		if err == nil {
			return a.deliverResource(result)
		}
		if ipc.CodeOf(err) != ipc.CodeAppNotRunning {
			a.fail(err)
			return exitCodeFor(err)
		}
		return a.readFromDisk(document, kind, "", 0, 0)
	}

	toolArgs := map[string]any{"documentId": document}
	if isPath(document) {
		toolArgs = map[string]any{"path": document}
	}
	if *section != "" {
		toolArgs["sectionId"] = *section
	}
	if *heading != "" {
		toolArgs["heading"] = *heading
	}
	if *lines != "" {
		toolArgs["startLine"] = startLine
		toolArgs["endLine"] = endLine
	}
	result, err := a.client.ToolCall("read_document", toolArgs)
	if err == nil {
		return a.deliver(result, nil)
	}
	if ipc.CodeOf(err) != ipc.CodeAppNotRunning {
		a.fail(err)
		return exitCodeFor(err)
	}
	return a.readFromDisk(document, "", *heading, startLine, endLine)
}

// readFromDisk reads a document without the app: text, outline or notes.
func (a *app) readFromDisk(document, kind, heading string, startLine, endLine int) int {
	path := document
	if !filepath.IsAbs(path) {
		abs, err := filepath.Abs(path)
		if err != nil {
			a.fail(ipc.Errorf(ipc.CodePathDenied, "Cannot resolve %s: %v.", document, err))
			return exitPathDenied
		}
		path = abs
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			a.fail(ipc.Errorf(ipc.CodeNotFound, "No file at %s.", path))
			return exitNotFound
		}
		a.fail(ipc.Errorf(ipc.CodePathDenied, "Cannot read %s: %v.", path, err))
		return exitPathDenied
	}
	text := string(data)
	switch kind {
	case "sections":
		return a.write(map[string]any{"path": path, "source": "disk", "sections": markdown.ExtractSections(text)})
	case "notes":
		return a.write(map[string]any{"path": path, "source": "disk", "notes": markdown.ExtractNotes(text)})
	}
	out := map[string]any{"path": path, "source": "disk", "text": text, "truncated": false}
	lines := strings.Split(text, "\n")
	if n := len(lines); n > 0 && lines[n-1] == "" {
		lines = lines[:n-1]
	}
	out["startLine"], out["endLine"] = 1, len(lines)
	switch {
	case heading != "":
		section, ok := findSection(markdown.ExtractSections(text), heading)
		if !ok {
			a.fail(ipc.Errorf(ipc.CodeNotFound, "No section headed %q in %s.", heading, path))
			return exitNotFound
		}
		out["text"] = strings.Join(lines[section.StartLine-1:section.EndLine], "\n")
		out["startLine"], out["endLine"] = section.StartLine, section.EndLine
		out["heading"], out["sectionId"], out["hash"] = section.Heading, section.ID, section.Hash
	case startLine != 0:
		if startLine < 1 || endLine < startLine || endLine > len(lines) {
			a.fail(ipc.Errorf(ipc.CodeInvalidArgs, "The line range %d:%d is outside the document's %d lines.", startLine, endLine, len(lines)))
			return exitUsage
		}
		out["text"] = strings.Join(lines[startLine-1:endLine], "\n")
		out["startLine"], out["endLine"] = startLine, endLine
	}
	return a.write(out)
}

func findSection(sections []markdown.Section, heading string) (markdown.Section, bool) {
	key := markdown.NormalizeHeadingText(heading)
	for _, section := range sections {
		if markdown.NormalizeHeadingText(section.Heading) == key {
			return section, true
		}
	}
	return markdown.Section{}, false
}

// deliverResource unwraps an MCP resource read into readable JSON.
func (a *app) deliverResource(result json.RawMessage) int {
	var payload struct {
		Contents []struct {
			URI  string `json:"uri"`
			Text string `json:"text"`
		} `json:"contents"`
	}
	if err := json.Unmarshal(result, &payload); err != nil || len(payload.Contents) != 1 {
		a.fail(ipc.Errorf(ipc.CodeInvalidArgs, "The app answered with an unexpected resource."))
		return exitFailure
	}
	var out any
	if err := json.Unmarshal([]byte(payload.Contents[0].Text), &out); err != nil {
		a.fail(err)
		return exitFailure
	}
	return a.write(out)
}

func (a *app) runPropose(args []string) int {
	set, _ := a.flags("propose")
	oldText := set.String("old", "", "the exact text to replace")
	newText := set.String("new", "", "the replacement text")
	from := set.String("from", "", "read the complete proposed text from a file, or - for stdin")
	summary := set.String("summary", "", "one sentence about the change")
	lines := set.String("lines", "", "replace this 1-based line range, as start:end")
	baseVersion := set.Int64("base-version", 0, "the document version read_document returned")
	positional, err := parse(set, args)
	if err != nil {
		a.fail(err)
		return exitUsage
	}
	if len(positional) != 1 {
		a.fail(ipc.Errorf(ipc.CodeInvalidArgs, "propose takes one document id or path."))
		return exitUsage
	}
	toolArgs := map[string]any{}
	if isPath(positional[0]) {
		toolArgs["path"] = positional[0]
	} else {
		toolArgs["documentId"] = positional[0]
	}
	if *summary != "" {
		toolArgs["summary"] = *summary
	}
	switch {
	case *from != "":
		text, err := readText(*from)
		if err != nil {
			a.fail(err)
			return exitUsage
		}
		toolArgs["proposedText"] = text
	case *lines != "":
		startLine, endLine, err := parseLines(*lines)
		if err != nil {
			a.fail(err)
			return exitUsage
		}
		toolArgs["startLine"], toolArgs["endLine"] = startLine, endLine
		toolArgs["newText"] = *newText
		if *baseVersion != 0 {
			toolArgs["baseVersion"] = *baseVersion
		}
	case *oldText != "":
		toolArgs["oldText"], toolArgs["newText"] = *oldText, *newText
	default:
		a.fail(ipc.Errorf(ipc.CodeInvalidArgs, "Propose an edit with --old and --new, --lines and --new, or --from."))
		return exitUsage
	}
	return a.deliver(a.client.ToolCall("propose_edit", toolArgs))
}

func (a *app) runNew(args []string) int {
	set, _ := a.flags("new")
	content := set.String("content", "", "the Markdown to write")
	from := set.String("from", "", "read the Markdown from a file, or - for stdin")
	openIt := set.Bool("open", false, "open the document in the app")
	positional, err := parse(set, args)
	if err != nil {
		a.fail(err)
		return exitUsage
	}
	if len(positional) != 1 {
		a.fail(ipc.Errorf(ipc.CodeInvalidArgs, "new takes one workspace-relative path."))
		return exitUsage
	}
	text := *content
	if *from != "" {
		text, err = readText(*from)
		if err != nil {
			a.fail(err)
			return exitUsage
		}
	}
	return a.deliver(a.client.ToolCall("create_document", map[string]any{
		"path": positional[0], "content": text, "open": *openIt,
	}))
}

func (a *app) runOpen(args []string) int {
	set, _ := a.flags("open")
	section := set.String("section", "", "scroll to the section with this id")
	heading := set.String("heading", "", "scroll to the section with this heading")
	positional, err := parse(set, args)
	if err != nil {
		a.fail(err)
		return exitUsage
	}
	if len(positional) != 1 {
		a.fail(ipc.Errorf(ipc.CodeInvalidArgs, "open takes one document id or path."))
		return exitUsage
	}
	toolArgs := map[string]any{"documentId": positional[0]}
	if isPath(positional[0]) {
		toolArgs = map[string]any{"path": positional[0]}
	}
	if *section != "" {
		toolArgs["sectionId"] = *section
	}
	if *heading != "" {
		toolArgs["heading"] = *heading
	}
	return a.deliver(a.client.ToolCall("open_document", toolArgs))
}

func (a *app) runGenUI(args []string) int {
	if len(args) == 0 {
		a.fail(ipc.Errorf(ipc.CodeInvalidArgs, "genui takes format or write."))
		return exitUsage
	}
	switch args[0] {
	case "format":
		fmt.Fprintln(a.stdout, genuikit.Format())
		return exitOK
	case "write":
		set, _ := a.flags("genui write")
		candidate := set.String("candidate", "", "the JSON candidate to write, or - for stdin")
		positional, err := parse(set, args[1:])
		if err != nil {
			a.fail(err)
			return exitUsage
		}
		if len(positional) != 1 || *candidate == "" {
			a.fail(ipc.Errorf(ipc.CodeInvalidArgs, "genui write takes a document and --candidate."))
			return exitUsage
		}
		raw, err := readText(*candidate)
		if err != nil {
			a.fail(err)
			return exitUsage
		}
		var payload struct {
			A2UI    json.RawMessage `json:"a2ui"`
			Sources json.RawMessage `json:"sources"`
			Request string          `json:"request"`
			Model   string          `json:"model"`
		}
		if err := json.Unmarshal([]byte(raw), &payload); err != nil || len(payload.A2UI) == 0 {
			a.fail(ipc.Errorf(ipc.CodeInvalidArgs, "The candidate needs a JSON object with an a2ui array."))
			return exitUsage
		}
		toolArgs := map[string]any{"a2ui": payload.A2UI}
		if isPath(positional[0]) {
			toolArgs["path"] = positional[0]
		} else {
			toolArgs["documentId"] = positional[0]
		}
		if len(payload.Sources) > 0 {
			toolArgs["sources"] = payload.Sources
		}
		if payload.Request != "" {
			toolArgs["request"] = payload.Request
		}
		if payload.Model != "" {
			toolArgs["model"] = payload.Model
		}
		return a.deliver(a.client.ToolCall("write_genui_view", toolArgs))
	default:
		a.fail(ipc.Errorf(ipc.CodeInvalidArgs, "genui takes format or write, not %q.", args[0]))
		return exitUsage
	}
}

func (a *app) runCheckpoint(args []string) int {
	set, _ := a.flags("checkpoint")
	label := set.String("label", "", "the checkpoint label")
	positional, err := parse(set, args)
	if err != nil {
		a.fail(err)
		return exitUsage
	}
	if len(positional) != 1 {
		a.fail(ipc.Errorf(ipc.CodeInvalidArgs, "checkpoint takes one document id or path."))
		return exitUsage
	}
	toolArgs := map[string]any{"documentId": positional[0], "label": *label}
	if isPath(positional[0]) {
		toolArgs = map[string]any{"path": positional[0], "label": *label}
	}
	return a.deliver(a.client.ToolCall("create_checkpoint", toolArgs))
}

func (a *app) runAgentPrompt(args []string) int {
	set, _ := a.flags("agent-prompt")
	if _, err := parse(set, args); err != nil {
		a.fail(err)
		return exitUsage
	}
	fmt.Fprint(a.stdout, agentPrompt(a.client.SocketPath))
	return exitOK
}

func (a *app) runMCP(args []string) int {
	set, _ := a.flags("mcp")
	if _, err := parse(set, args); err != nil {
		a.fail(err)
		return exitUsage
	}
	if err := ipc.RunShim(ipc.ShimOptions{
		SocketPath:  a.client.SocketPath,
		CWD:         a.client.CWD,
		Name:        "markgogo-mcp",
		Version:     appversion.Version,
		StdIn:       a.stdin,
		StdOut:      a.stdout,
		StdErr:      a.stderr,
		DialTimeout: ipc.DialTimeout,
	}); err != nil {
		fmt.Fprintf(a.stderr, "markgogo mcp: %v\n", err)
		return exitFailure
	}
	return exitOK
}

func agentPrompt(socketPath string) string {
	return fmt.Sprintf(`You are reading and editing the Markdown workspace that Mark Go Go has open.

The app listens at %s. Use the markgogo CLI, or `+"`markgogo mcp`"+` as an MCP stdio server when your client only speaks MCP. Both bind to the workspace that contains your working directory.

Read before you write:
- get_workspace lists the root, the open documents and a bounded list of Markdown files, with a truncated flag.
- read_document (or `+"`markgogo read <document>`"+`) takes a documentId or a workspace-relative path, and may return one heading section or a line range. It always returns the document version.

Every change to existing text goes through propose_edit (or `+"`markgogo propose`"+`): pass exactly one of proposedText, oldText with newText, or startLine with endLine and newText plus the baseVersion you read. The user reviews the proposal in the app and applies it; the tool only returns its id, revision, hunk count and a one-line summary.

create_document (or `+"`markgogo new`"+`) creates a new file and refuses to overwrite. write_genui_view validates a GenUI candidate against the catalog before writing the document's .genui.json and returns the version it wrote (2 for A2UI, 3 for openui-lang; get the format with `+"`markgogo genui format`"+`). create_checkpoint snapshots a saved document.

Errors carry a stable code: APP_NOT_RUNNING, WORKSPACE_NOT_OPEN, VERSION_CONFLICT, NOT_FOUND, PATH_DENIED or INVALID_ARGS, with a hint. When APP_NOT_RUNNING says the connection closed mid-request, the write may or may not have happened: read the document again instead of retrying blindly.
`, socketPath)
}

func isPath(document string) bool {
	return strings.ContainsAny(document, "/\\") || strings.HasSuffix(strings.ToLower(document), ".md") ||
		strings.HasSuffix(strings.ToLower(document), ".markdown")
}

func parseLines(raw string) (int, int, error) {
	if raw == "" {
		return 0, 0, nil
	}
	start, end, ok := strings.Cut(raw, ":")
	if !ok {
		return 0, 0, ipc.Errorf(ipc.CodeInvalidArgs, "Write a line range as start:end, not %q.", raw)
	}
	var first, last int
	if _, err := fmt.Sscanf(start, "%d", &first); err != nil {
		return 0, 0, ipc.Errorf(ipc.CodeInvalidArgs, "The line range %q has no start line.", raw)
	}
	if _, err := fmt.Sscanf(end, "%d", &last); err != nil {
		return 0, 0, ipc.Errorf(ipc.CodeInvalidArgs, "The line range %q has no end line.", raw)
	}
	if first < 1 || last < first {
		return 0, 0, ipc.Errorf(ipc.CodeInvalidArgs, "The line range %q must start at 1 and not end before it starts.", raw)
	}
	return first, last, nil
}

func readText(file string) (string, error) {
	if file == "-" {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return "", err
		}
		return string(data), nil
	}
	data, err := os.ReadFile(file)
	if err != nil {
		return "", ipc.Errorf(ipc.CodeNotFound, "Cannot read %s: %v.", file, err)
	}
	return string(data), nil
}

func escapeDocument(document string) string { return url.PathEscape(document) }
