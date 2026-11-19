package feynman

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/Bayern99/mark-go-go/internal/fsutil"
	"github.com/Bayern99/mark-go-go/internal/review"
	"github.com/egoist/mygo"
)

var researchFocus = mygo.NewEvent[ResearchFocusEvent]("researchFocus")

var ansiEscapePattern = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]|\x1b\][^\x07\x1b]*(?:\x07|\x1b\\)`)

const (
	defaultCLITimeout     = 60 * time.Second
	longCLITimeout        = 120 * time.Second
	terminalLaunchTimeout = 15 * time.Second
)

func feynmanCLICommand(ctx context.Context, work string, args ...string) (*exec.Cmd, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	dirs := append(filepath.SplitList(os.Getenv("PATH")), fsutil.DefaultUserBinDirs()...)
	dirs = append(dirs, filepath.Join(home, ".local", "share", "fnm", "aliases", "default", "bin"), "/usr/bin", "/bin")
	var paths []string
	for _, dir := range dirs {
		if filepath.IsAbs(dir) {
			paths = append(paths, dir)
		}
	}
	pathEnv := "PATH=" + strings.Join(paths, string(os.PathListSeparator))
	feynmanBin := findExecutable([]string{pathEnv}, "feynman")
	if feynmanBin == "" {
		return nil, errors.New("Install Feynman CLI, then run feynman setup in Terminal.")
	}
	cmd := exec.CommandContext(ctx, feynmanBin, args...)
	if work != "" {
		if err := os.MkdirAll(work, 0700); err == nil {
			cmd.Dir = fsutil.CanonicalPath(work)
		}
	}
	cmd.Env = append(os.Environ(),
		pathEnv,
		"NO_COLOR=1",
		"TERM=dumb",
	)
	configureResearchProcess(cmd)
	cmd.Cancel = func() error {
		stopResearchProcess(cmd)
		return nil
	}
	return cmd, nil
}

func cleanCLIOutput(raw []byte) string {
	s := ansiEscapePattern.ReplaceAllString(string(raw), "")
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return strings.TrimSpace(s)
}

func formatCLICommand(args ...string) string {
	if len(args) == 0 {
		return "feynman"
	}
	return "feynman " + strings.Join(args, " ")
}

func (h *ResearchManager) cliWorkDir(root string) string {
	if h.dataDir == "" || root == "" || !filepath.IsAbs(root) {
		return ""
	}
	work := review.WorkDir(h.dataDir, fsutil.CanonicalPath(root))
	if err := os.MkdirAll(work, 0700); err != nil {
		return ""
	}
	return work
}

func (h *ResearchManager) runCLI(ctx context.Context, root string, timeout time.Duration, args ...string) (raw string, exitCode int, err error) {
	if _, wsErr := h.workspace(ctx, root); wsErr != nil {
		return "", 0, wsErr
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	h.mu.Lock()
	factory := h.cliCommand
	h.mu.Unlock()
	if factory == nil {
		factory = feynmanCLICommand
	}
	cmd, err := factory(ctx, h.cliWorkDir(root), args...)
	if err != nil {
		return "", 0, err
	}
	out, runErr := cmd.CombinedOutput()
	raw = cleanCLIOutput(out)
	if runErr != nil {
		var exitErr *exec.ExitError
		if errors.As(runErr, &exitErr) {
			return raw, exitErr.ExitCode(), nil
		}
		return raw, -1, runErr
	}
	return raw, 0, nil
}

// WatchFocus attaches a window focus listener for the workspace window so focus regain triggers a researchFocus event.
func (h *ResearchManager) WatchFocus(ctx context.Context, root string) error {
	w, err := h.workspace(ctx, root)
	if err != nil {
		return err
	}
	h.ensureFocusWatch(w)
	return nil
}

func (h *ResearchManager) ensureFocusWatch(w WorkspaceRef) {
	if w.Window == nil {
		return
	}
	root := fsutil.CanonicalPath(w.Root)
	h.mu.Lock()
	if h.closed {
		h.mu.Unlock()
		return
	}
	if h.focusWatchers == nil {
		h.focusWatchers = make(map[*mygo.Window]*focusWatcher)
	}
	if existing := h.focusWatchers[w.Window]; existing != nil {
		existing.root = root
		h.mu.Unlock()
		return
	}
	fw := &focusWatcher{root: root}
	h.focusWatchers[w.Window] = fw
	h.mu.Unlock()

	win := w.Window
	offFocus := win.OnFocus(func() {
		h.mu.Lock()
		closed := h.closed
		current := h.focusWatchers[win]
		r := ""
		if current != nil {
			r = current.root
		}
		h.mu.Unlock()
		if closed || r == "" {
			return
		}
		go func() {
			_ = researchFocus.Emit(win, ResearchFocusEvent{Root: r})
		}()
	})
	offClosed := win.OnClosed(func() {
		h.mu.Lock()
		delete(h.focusWatchers, win)
		h.mu.Unlock()
	})
	offAll := func() {
		offFocus()
		offClosed()
	}

	h.mu.Lock()
	if h.closed || h.focusWatchers[win] != fw {
		h.mu.Unlock()
		offAll()
		return
	}
	fw.off = offAll
	h.mu.Unlock()
}

// RefreshModels re-queries available models from an active Chat RPC session without interrupting turns.
func (h *ResearchManager) RefreshModels(ctx context.Context, root string) ([]ResearchModel, error) {
	h.mu.Lock()
	s, err := h.activeLocked(ctx, root)
	if err != nil {
		h.mu.Unlock()
		return nil, err
	}
	select {
	case <-s.ready:
		if s.startErr != nil {
			h.mu.Unlock()
			return nil, s.startErr
		}
	default:
		h.mu.Unlock()
		return nil, errors.New("Feynman is connecting.")
	}
	if s.busy || s.configuring || s.switching {
		modelsCopy := slices.Clone(s.models)
		h.mu.Unlock()
		return modelsCopy, nil
	}
	h.mu.Unlock()

	var modelsResp struct {
		Models []ResearchModel `json:"models"`
	}
	if err := s.rpc.Call("get_available_models", nil, &modelsResp); err != nil {
		return nil, err
	}
	var state ResearchSessionState
	hasState := s.rpc.Call("get_state", nil, &state) == nil
	var levelsResp struct {
		Levels []string `json:"levels"`
	}
	_ = s.rpc.Call("get_available_thinking_levels", nil, &levelsResp)

	h.mu.Lock()
	s.models = modelsResp.Models
	if hasState {
		s.state = state
		if state.ThinkingLevel != "" {
			s.thinkingLevel = state.ThinkingLevel
		}
	} else {
		state = s.state
	}
	if len(levelsResp.Levels) > 0 {
		s.thinkingLevels = levelsResp.Levels
	}
	s.imagePrompts = modelSupportsImages(state.Model, s.models)
	cfg := buildResearchConfig(state, s.models, s.thinkingLevels)
	s.config = cfg
	modelsCopy := slices.Clone(s.models)
	levelsCopy := slices.Clone(s.thinkingLevels)
	h.mu.Unlock()

	s.emit(ResearchUpdate{
		Kind:           "config",
		Config:         cfg,
		Models:         modelsCopy,
		ThinkingLevels: levelsCopy,
		ThinkingLevel:  state.ThinkingLevel,
		State:          &state,
	})
	return modelsCopy, nil
}

// LaunchTerminalWizard hands off interactive TTY/OAuth setup or login wizards to the system Terminal.
func (h *ResearchManager) LaunchTerminalWizard(ctx context.Context, root, wizard, target string) (CLITerminalLaunchResult, error) {
	w, err := h.workspace(ctx, root)
	if err != nil {
		return CLITerminalLaunchResult{}, err
	}
	h.ensureFocusWatch(w)

	wizard = strings.TrimSpace(wizard)
	target = strings.TrimSpace(target)
	if strings.HasPrefix(target, "-") {
		return CLITerminalLaunchResult{}, errors.New("Invalid provider or target identifier.")
	}

	var args []string
	switch wizard {
	case "model_login":
		args = []string{"model", "login"}
		if target != "" {
			args = append(args, target)
		}
	case "alpha_login":
		args = []string{"alpha", "login"}
	case "setup":
		args = []string{"setup"}
	default:
		return CLITerminalLaunchResult{}, fmt.Errorf("Unsupported interactive wizard %q.", wizard)
	}

	command := shellJoinCommand("feynman", args...)
	h.mu.Lock()
	launcher := h.openTerminal
	h.mu.Unlock()
	if launcher == nil {
		launcher = openSystemTerminal
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, terminalLaunchTimeout)
		defer cancel()
	}
	if err := launcher(ctx, command); err != nil {
		return CLITerminalLaunchResult{Wizard: wizard, Command: command, Launched: false}, err
	}
	return CLITerminalLaunchResult{Wizard: wizard, Command: command, Launched: true}, nil
}

func shellQuoteArg(s string) string {
	if s == "" {
		return "''"
	}
	safe := true
	for _, r := range s {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' || r == '/' || r == ':') {
			safe = false
			break
		}
	}
	if safe {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}

func shellJoinCommand(bin string, args ...string) string {
	parts := make([]string, 0, 1+len(args))
	parts = append(parts, shellQuoteArg(bin))
	for _, a := range args {
		parts = append(parts, shellQuoteArg(a))
	}
	return strings.Join(parts, " ")
}

func appleScriptString(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return `"` + s + `"`
}

func openSystemTerminal(ctx context.Context, command string) error {
	launchCmd := command
	if bin := findExecutable(nil, "feynman"); bin != "" {
		binDir := filepath.Dir(bin)
		if runtime.GOOS != "windows" && strings.HasPrefix(command, "feynman") {
			launchCmd = fmt.Sprintf("PATH=%s:\"$PATH\" %s", shellQuoteArg(binDir), command)
		}
	}
	switch runtime.GOOS {
	case "darwin":
		cmd := exec.CommandContext(ctx, "osascript",
			"-e", `tell application "Terminal" to activate`,
			"-e", fmt.Sprintf(`tell application "Terminal" to do script %s`, appleScriptString(launchCmd)),
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			msg := strings.TrimSpace(string(out))
			if msg != "" {
				return fmt.Errorf("open Terminal: %s", msg)
			}
			return err
		}
		return nil
	case "windows":
		cmd := exec.CommandContext(ctx, "cmd.exe", "/c", "start", "", "cmd.exe", "/k", command)
		return cmd.Start()
	default:
		if term := strings.TrimSpace(os.Getenv("TERMINAL")); term != "" {
			if err := exec.CommandContext(ctx, term, "-e", "sh", "-lc", launchCmd).Start(); err == nil {
				return nil
			}
		}
		for _, candidate := range []struct {
			bin  string
			args []string
		}{
			{"x-terminal-emulator", []string{"-e", "sh", "-lc", launchCmd}},
			{"gnome-terminal", []string{"--", "sh", "-lc", launchCmd}},
			{"konsole", []string{"-e", "sh", "-lc", launchCmd}},
			{"xterm", []string{"-e", "sh", "-lc", launchCmd}},
		} {
			if path := findExecutable(nil, candidate.bin); path != "" {
				return exec.CommandContext(ctx, path, candidate.args...).Start()
			}
		}
		return errors.New("No supported terminal emulator found. Run the command in your terminal: " + command)
	}
}

func firstNonZeroError(exitCode int, raw string) string {
	if exitCode == 0 {
		return ""
	}
	if raw != "" {
		return raw
	}
	return fmt.Sprintf("Command exited with status %d", exitCode)
}

func commandResult(command, raw string, exitCode int) CLICommandResult {
	return CLICommandResult{
		Command:   command,
		Parsed:    exitCode == 0 && raw != "",
		RawOutput: raw,
		ExitCode:  exitCode,
		Error:     firstNonZeroError(exitCode, raw),
		Summary:   firstLine(raw),
	}
}

func firstLine(s string) string {
	for _, line := range strings.Split(s, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}

// CLIStatus executes `feynman status` asynchronously and parses structured status sections.
func (h *ResearchManager) CLIStatus(ctx context.Context, root string) (CLIStatusResult, error) {
	raw, exitCode, err := h.runCLI(ctx, root, defaultCLITimeout, "status")
	if err != nil {
		return CLIStatusResult{}, err
	}
	res := parseStatusOutput(raw, exitCode)
	res.Command = formatCLICommand("status")
	return res, nil
}

func parseStatusOutput(raw string, exitCode int) CLIStatusResult {
	res := CLIStatusResult{
		RawOutput: raw,
		ExitCode:  exitCode,
		Error:     firstNonZeroError(exitCode, raw),
	}
	var current *CLISection
	for _, line := range strings.Split(raw, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "┌") || strings.HasPrefix(trimmed, "├") || strings.HasPrefix(trimmed, "└") || strings.HasPrefix(trimmed, "│") {
			continue
		}
		if strings.HasPrefix(trimmed, "◆ ") {
			title := strings.TrimSpace(strings.TrimPrefix(trimmed, "◆ "))
			res.Sections = append(res.Sections, CLISection{Title: title})
			current = &res.Sections[len(res.Sections)-1]
			continue
		}
		if k, v, ok := splitKeyValue(trimmed); ok {
			if current == nil {
				res.Sections = append(res.Sections, CLISection{Title: "Summary"})
				current = &res.Sections[len(res.Sections)-1]
			}
			current.Entries = append(current.Entries, CLIKeyValue{Key: k, Value: v})
			switch strings.ToLower(k) {
			case "model":
				res.Model = v
			case "model valid":
				res.ModelValid = v
			case "alphaxiv":
				res.AlphaStatus = v
			case "web access":
				res.WebAccess = v
			case "service tier":
				res.ServiceTier = v
			case "preview":
				res.PreviewStatus = v
			case "sessions":
				res.SessionsDir = v
			case "pi runtime":
				res.PiRuntime = v
			}
		}
	}
	entryCount := 0
	for _, sec := range res.Sections {
		entryCount += len(sec.Entries)
	}
	res.Parsed = exitCode == 0 && len(res.Sections) > 0 && entryCount >= 2 && (res.Model != "" || res.PiRuntime != "")
	return res
}

func splitKeyValue(line string) (string, string, bool) {
	idx := strings.Index(line, ":")
	if idx <= 0 {
		return "", "", false
	}
	key := strings.TrimSpace(line[:idx])
	val := strings.TrimSpace(line[idx+1:])
	if key == "" || val == "" || strings.ContainsAny(key, "{}\"") {
		return "", "", false
	}
	return key, val, true
}

// CLIDoctor executes `feynman doctor` asynchronously and parses diagnostics.
func (h *ResearchManager) CLIDoctor(ctx context.Context, root string) (CLIDoctorResult, error) {
	raw, exitCode, err := h.runCLI(ctx, root, defaultCLITimeout, "doctor")
	if err != nil {
		return CLIDoctorResult{}, err
	}
	res := parseDoctorOutput(raw, exitCode)
	res.Command = formatCLICommand("doctor")
	return res, nil
}

func parseDoctorOutput(raw string, exitCode int) CLIDoctorResult {
	res := CLIDoctorResult{
		RawOutput: raw,
		ExitCode:  exitCode,
		Error:     firstNonZeroError(exitCode, raw),
		Healthy:   exitCode == 0,
	}
	for _, line := range strings.Split(raw, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "┌") || strings.HasPrefix(trimmed, "├") || strings.HasPrefix(trimmed, "└") || strings.HasPrefix(trimmed, "│") {
			continue
		}
		if k, v, ok := splitKeyValue(trimmed); ok {
			res.Entries = append(res.Entries, CLIKeyValue{Key: k, Value: v})
			lowKey := strings.ToLower(k)
			lowVal := strings.ToLower(v)
			if lowVal == "missing" || lowVal == "no" || strings.Contains(lowVal, "error") || strings.Contains(lowVal, "fail") || lowKey == "error" {
				res.Issues = append(res.Issues, fmt.Sprintf("%s: %s", k, v))
			}
		}
	}
	if len(res.Issues) > 0 {
		res.Healthy = false
	}
	res.Parsed = len(res.Entries) >= 2
	return res
}

// CLISetupPreview executes `feynman setup preview` asynchronously.
func (h *ResearchManager) CLISetupPreview(ctx context.Context, root string) (CLICommandResult, error) {
	raw, exitCode, err := h.runCLI(ctx, root, longCLITimeout, "setup", "preview")
	if err != nil {
		return CLICommandResult{}, err
	}
	return commandResult(formatCLICommand("setup", "preview"), raw, exitCode), nil
}

// CLIModelList executes `feynman model list` and `feynman model tier` asynchronously.
func (h *ResearchManager) CLIModelList(ctx context.Context, root string) (CLIModelListResult, error) {
	raw, exitCode, err := h.runCLI(ctx, root, defaultCLITimeout, "model", "list")
	if err != nil {
		return CLIModelListResult{}, err
	}
	tierRaw, tierCode, tierErr := h.runCLI(ctx, root, defaultCLITimeout, "model", "tier")
	res := parseModelListOutput(raw, exitCode)
	res.Command = formatCLICommand("model", "list")
	if tierErr == nil && tierCode == 0 && tierRaw != "" {
		res.Tier = strings.TrimSpace(firstLine(tierRaw))
	}
	return res, nil
}

func parseModelListOutput(raw string, exitCode int) CLIModelListResult {
	res := CLIModelListResult{
		RawOutput: raw,
		ExitCode:  exitCode,
		Error:     firstNonZeroError(exitCode, raw),
	}
	var currentProvider string
	for _, line := range strings.Split(raw, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "◆ ") {
			currentProvider = strings.TrimSpace(strings.TrimPrefix(trimmed, "◆ "))
			continue
		}
		if currentProvider == "" && !strings.Contains(trimmed, "/") {
			continue
		}
		modelPart := trimmed
		var tags []string
		if idx := strings.Index(trimmed, " ("); idx > 0 && strings.HasSuffix(trimmed, ")") {
			modelPart = strings.TrimSpace(trimmed[:idx])
			tagStr := strings.TrimSuffix(trimmed[idx+2:], ")")
			for _, t := range strings.Split(tagStr, ",") {
				tags = append(tags, strings.TrimSpace(strings.ToLower(t)))
			}
		}
		if strings.ContainsAny(modelPart, " \t") {
			continue
		}
		provider := currentProvider
		id := modelPart
		if slash := strings.Index(modelPart, "/"); slash > 0 {
			provider = modelPart[:slash]
			id = modelPart[slash+1:]
		}
		full := modelPart
		if !strings.Contains(full, "/") && provider != "" {
			full = provider + "/" + id
		}
		entry := CLIModelEntry{
			ID:          id,
			Provider:    provider,
			Name:        full,
			Current:     slices.Contains(tags, "current"),
			Recommended: slices.Contains(tags, "recommended"),
		}
		if entry.Current {
			res.CurrentModel = full
		}
		if entry.Recommended {
			res.RecommendedModel = full
		}
		res.Models = append(res.Models, entry)
	}
	low := strings.ToLower(raw)
	if exitCode == 0 && (len(res.Models) > 0 || strings.Contains(low, "no models") || strings.Contains(low, "no authenticated")) {
		res.Parsed = true
	}
	return res
}

// CLIModelAction executes `feynman model set|tier|logout`.
func (h *ResearchManager) CLIModelAction(ctx context.Context, root, action, value string) (CLICommandResult, error) {
	action = strings.TrimSpace(action)
	value = strings.TrimSpace(value)
	if strings.HasPrefix(value, "-") {
		return CLICommandResult{}, errors.New("Invalid model or provider value.")
	}
	var args []string
	switch action {
	case "set":
		if value == "" {
			return CLICommandResult{}, errors.New("Model ID is required.")
		}
		args = []string{"model", "set", value}
	case "tier":
		if value == "" {
			args = []string{"model", "tier"}
		} else {
			args = []string{"model", "tier", value}
		}
	case "logout":
		if value == "" {
			return CLICommandResult{}, errors.New("Provider ID is required.")
		}
		args = []string{"model", "logout", value}
	default:
		return CLICommandResult{}, fmt.Errorf("Unsupported model action %q.", action)
	}
	raw, exitCode, err := h.runCLI(ctx, root, defaultCLITimeout, args...)
	if err != nil {
		return CLICommandResult{}, err
	}
	return commandResult(formatCLICommand(args...), raw, exitCode), nil
}

// CLISearchStatus executes `feynman search status` asynchronously.
func (h *ResearchManager) CLISearchStatus(ctx context.Context, root string) (CLISearchStatusResult, error) {
	raw, exitCode, err := h.runCLI(ctx, root, defaultCLITimeout, "search", "status")
	if err != nil {
		return CLISearchStatusResult{}, err
	}
	res := parseSearchStatusOutput(raw, exitCode)
	res.Command = formatCLICommand("search", "status")
	return res, nil
}

func parseSearchStatusOutput(raw string, exitCode int) CLISearchStatusResult {
	res := CLISearchStatusResult{
		RawOutput: raw,
		ExitCode:  exitCode,
		Error:     firstNonZeroError(exitCode, raw),
	}
	for _, line := range strings.Split(raw, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if k, v, ok := splitKeyValue(trimmed); ok {
			res.Entries = append(res.Entries, CLIKeyValue{Key: k, Value: v})
			switch strings.ToLower(k) {
			case "managed by":
				res.ManagedBy = v
			case "search route":
				res.SearchRoute = v
			case "request route":
				res.RequestRoute = v
			case "search workflow":
				res.SearchWorkflow = v
			case "saved api keys":
				res.SavedAPIKeys = v
			case "config path":
				res.ConfigPath = v
			}
		}
	}
	low := strings.ToLower(raw)
	res.Configured = !strings.Contains(low, "not configured yet") &&
		(strings.ToLower(res.SavedAPIKeys) != "none" && res.SavedAPIKeys != "" ||
			(res.SearchRoute != "" && strings.ToLower(res.SearchRoute) != "auto"))
	res.Parsed = exitCode == 0 && len(res.Entries) >= 2 && (res.ManagedBy != "" || res.SearchRoute != "")
	return res
}

// CLISearchAction executes `feynman search set|clear`.
func (h *ResearchManager) CLISearchAction(ctx context.Context, root, action, provider, apiKey string) (CLICommandResult, error) {
	action = strings.TrimSpace(action)
	provider = strings.TrimSpace(provider)
	apiKey = strings.TrimSpace(apiKey)
	if strings.HasPrefix(provider, "-") {
		return CLICommandResult{}, errors.New("Invalid web search provider.")
	}
	var args []string
	switch action {
	case "set":
		if provider == "" {
			return CLICommandResult{}, errors.New("Web search provider is required.")
		}
		args = []string{"search", "set", provider}
		if apiKey != "" {
			args = append(args, apiKey)
		}
	case "clear":
		args = []string{"search", "clear"}
	default:
		return CLICommandResult{}, fmt.Errorf("Unsupported search action %q.", action)
	}
	raw, exitCode, err := h.runCLI(ctx, root, defaultCLITimeout, args...)
	if err != nil {
		return CLICommandResult{}, err
	}
	displayArgs := slices.Clone(args)
	if action == "set" && apiKey != "" {
		displayArgs[len(displayArgs)-1] = "•••"
	}
	return commandResult(formatCLICommand(displayArgs...), raw, exitCode), nil
}

// CLIMCPList executes `feynman mcp list --json` and falls back to raw output if JSON parsing fails.
func (h *ResearchManager) CLIMCPList(ctx context.Context, root string) (CLIMCPListResult, error) {
	raw, exitCode, err := h.runCLI(ctx, root, defaultCLITimeout, "mcp", "list", "--json")
	if err != nil {
		return CLIMCPListResult{}, err
	}
	res := parseMCPListOutput(raw, exitCode)
	res.Command = formatCLICommand("mcp", "list", "--json")
	return res, nil
}

func parseMCPListOutput(raw string, exitCode int) CLIMCPListResult {
	res := CLIMCPListResult{
		RawOutput: raw,
		ExitCode:  exitCode,
		Error:     firstNonZeroError(exitCode, raw),
	}
	var payload struct {
		Servers []struct {
			Name      string   `json:"name"`
			Transport string   `json:"transport"`
			Type      string   `json:"type"`
			URL       string   `json:"url"`
			Command   string   `json:"command"`
			Args      []string `json:"args"`
			Exposure  string   `json:"exposure"`
			Status    string   `json:"status"`
			Error     string   `json:"error"`
			Tools     []any    `json:"tools"`
		} `json:"servers"`
		Errors []string `json:"errors"`
	}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil || (payload.Servers == nil && payload.Errors == nil) {
		return res
	}
	res.Parsed = true
	res.Errors = payload.Errors
	for _, srv := range payload.Servers {
		transport := srv.Transport
		if transport == "" {
			transport = srv.Type
		}
		if transport == "" {
			if srv.URL != "" {
				transport = "http"
			} else if srv.Command != "" {
				transport = "stdio"
			}
		}
		cmd := srv.Command
		if cmd != "" && len(srv.Args) > 0 {
			cmd = cmd + " " + strings.Join(srv.Args, " ")
		}
		var tools []string
		for _, tool := range srv.Tools {
			switch v := tool.(type) {
			case string:
				tools = append(tools, v)
			case map[string]any:
				if name, ok := v["name"].(string); ok && name != "" {
					tools = append(tools, name)
				}
			}
		}
		res.Servers = append(res.Servers, CLIMCPServer{
			Name:      srv.Name,
			Transport: transport,
			URL:       srv.URL,
			Command:   cmd,
			Exposure:  srv.Exposure,
			Status:    srv.Status,
			Error:     srv.Error,
			Tools:     tools,
		})
	}
	return res
}

// CLIMCPAdd executes `feynman mcp add` for HTTP or stdio MCP servers.
func (h *ResearchManager) CLIMCPAdd(ctx context.Context, root string, input CLIMCPAddInput) (CLICommandResult, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" || strings.HasPrefix(name, "-") {
		return CLICommandResult{}, errors.New("MCP server name is required.")
	}
	args := []string{"mcp", "add", name}
	if input.Local {
		args = append(args, "-l")
	}
	if exp := strings.TrimSpace(input.Exposure); exp != "" {
		args = append(args, "--exposure", exp)
	}
	if desc := strings.TrimSpace(input.Description); desc != "" {
		args = append(args, "--description", desc)
	}
	transport := strings.ToLower(strings.TrimSpace(input.Transport))
	if transport == "" {
		if strings.TrimSpace(input.URL) != "" {
			transport = "http"
		} else {
			transport = "stdio"
		}
	}
	switch transport {
	case "http":
		urlStr := strings.TrimSpace(input.URL)
		if urlStr == "" {
			return CLICommandResult{}, errors.New("Server URL is required for HTTP MCP servers.")
		}
		args = append(args, "--url", urlStr)
		for _, hdr := range input.Headers {
			if trimmed := strings.TrimSpace(hdr); trimmed != "" {
				args = append(args, "--header", trimmed)
			}
		}
		if envVar := strings.TrimSpace(input.BearerTokenEnvVar); envVar != "" {
			args = append(args, "--bearer-token-env-var", envVar)
		}
	case "stdio":
		cmdName := strings.TrimSpace(input.Command)
		if cmdName == "" {
			return CLICommandResult{}, errors.New("Command is required for stdio MCP servers.")
		}
		if cwd := strings.TrimSpace(input.Cwd); cwd != "" {
			args = append(args, "--cwd", cwd)
		}
		for _, envEntry := range input.Env {
			if trimmed := strings.TrimSpace(envEntry); trimmed != "" {
				args = append(args, "--env", trimmed)
			}
		}
		args = append(args, "--", cmdName)
		for _, arg := range input.Args {
			if arg != "" {
				args = append(args, arg)
			}
		}
	default:
		return CLICommandResult{}, fmt.Errorf("Unsupported MCP transport %q.", transport)
	}
	raw, exitCode, err := h.runCLI(ctx, root, defaultCLITimeout, args...)
	if err != nil {
		return CLICommandResult{}, err
	}
	return commandResult(formatCLICommand(args...), raw, exitCode), nil
}

// CLIMCPRemove executes `feynman mcp remove`.
func (h *ResearchManager) CLIMCPRemove(ctx context.Context, root, name string, local bool) (CLICommandResult, error) {
	name = strings.TrimSpace(name)
	if name == "" || strings.HasPrefix(name, "-") {
		return CLICommandResult{}, errors.New("MCP server name is required.")
	}
	args := []string{"mcp", "remove", name}
	if local {
		args = append(args, "-l")
	}
	raw, exitCode, err := h.runCLI(ctx, root, defaultCLITimeout, args...)
	if err != nil {
		return CLICommandResult{}, err
	}
	return commandResult(formatCLICommand(args...), raw, exitCode), nil
}

// CLIPackagesList executes `feynman packages list` asynchronously.
func (h *ResearchManager) CLIPackagesList(ctx context.Context, root string) (CLIPackageListResult, error) {
	raw, exitCode, err := h.runCLI(ctx, root, defaultCLITimeout, "packages", "list")
	if err != nil {
		return CLIPackageListResult{}, err
	}
	res := parsePackagesListOutput(raw, exitCode)
	res.Command = formatCLICommand("packages", "list")
	return res, nil
}

func parsePackagesListOutput(raw string, exitCode int) CLIPackageListResult {
	res := CLIPackageListResult{
		RawOutput: raw,
		ExitCode:  exitCode,
		Error:     firstNonZeroError(exitCode, raw),
	}
	section := ""
	for _, line := range strings.Split(raw, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "┌") || strings.HasPrefix(trimmed, "├") || strings.HasPrefix(trimmed, "└") || strings.HasPrefix(trimmed, "│") {
			continue
		}
		if strings.HasPrefix(trimmed, "◆ ") {
			sec := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(trimmed, "◆ ")))
			if strings.Contains(sec, "core") {
				section = "core"
			} else if strings.Contains(sec, "optional") {
				section = "optional"
			}
			continue
		}
		if section == "" || strings.HasPrefix(strings.ToLower(trimmed), "install with:") {
			continue
		}
		fields := strings.Fields(trimmed)
		if len(fields) == 0 {
			continue
		}
		name := fields[0]
		desc := strings.TrimSpace(strings.TrimPrefix(trimmed, name))
		installed := section == "core" || strings.Contains(strings.ToLower(desc), "installed")
		res.Packages = append(res.Packages, CLIPackageEntry{
			Name:        name,
			Description: desc,
			Kind:        section,
			Installed:   installed,
		})
	}
	res.Parsed = exitCode == 0 && len(res.Packages) > 0
	return res
}

// CLIPackageAction executes `feynman packages install|remove` or `feynman update`.
func (h *ResearchManager) CLIPackageAction(ctx context.Context, root, action, target string) (CLICommandResult, error) {
	action = strings.TrimSpace(action)
	target = strings.TrimSpace(target)
	if strings.HasPrefix(target, "-") {
		return CLICommandResult{}, errors.New("Invalid package name.")
	}
	var args []string
	switch action {
	case "install":
		if target == "" {
			return CLICommandResult{}, errors.New("Package preset name is required.")
		}
		args = []string{"packages", "install", target}
	case "remove":
		if target == "" {
			return CLICommandResult{}, errors.New("Package preset name is required.")
		}
		args = []string{"packages", "remove", target}
	case "update":
		args = []string{"update"}
		if target != "" {
			args = append(args, target)
		}
	default:
		return CLICommandResult{}, fmt.Errorf("Unsupported package action %q.", action)
	}
	raw, exitCode, err := h.runCLI(ctx, root, longCLITimeout, args...)
	if err != nil {
		return CLICommandResult{}, err
	}
	return commandResult(formatCLICommand(args...), raw, exitCode), nil
}

// CLIAlphaStatus executes `feynman alpha status`.
func (h *ResearchManager) CLIAlphaStatus(ctx context.Context, root string) (CLIAlphaStatusResult, error) {
	raw, exitCode, err := h.runCLI(ctx, root, defaultCLITimeout, "alpha", "status")
	if err != nil {
		return CLIAlphaStatusResult{}, err
	}
	res := parseAlphaStatusOutput(raw, exitCode)
	res.Command = formatCLICommand("alpha", "status")
	return res, nil
}

func parseAlphaStatusOutput(raw string, exitCode int) CLIAlphaStatusResult {
	res := CLIAlphaStatusResult{
		RawOutput:  raw,
		ExitCode:   exitCode,
		Error:      firstNonZeroError(exitCode, raw),
		StatusText: firstLine(raw),
	}
	var jsonPayload struct {
		LoggedIn bool   `json:"loggedIn"`
		Account  string `json:"account"`
		Email    string `json:"email"`
	}
	if json.Unmarshal([]byte(raw), &jsonPayload) == nil && (strings.HasPrefix(raw, "{") || jsonPayload.Account != "" || jsonPayload.Email != "") {
		res.Parsed = true
		res.LoggedIn = jsonPayload.LoggedIn
		res.Account = jsonPayload.Account
		if res.Account == "" {
			res.Account = jsonPayload.Email
		}
		return res
	}
	low := strings.ToLower(raw)
	switch {
	case strings.Contains(low, "not logged in"):
		res.Parsed = true
		res.LoggedIn = false
	case strings.Contains(low, "logged in") || strings.Contains(low, "authenticated"):
		res.Parsed = true
		res.LoggedIn = true
		if idx := strings.Index(low, "logged in as "); idx >= 0 {
			res.Account = strings.TrimSpace(firstLine(raw[idx+len("logged in as "):]))
		}
	}
	return res
}

// CLIAlphaLogout executes `feynman alpha logout`.
func (h *ResearchManager) CLIAlphaLogout(ctx context.Context, root string) (CLICommandResult, error) {
	raw, exitCode, err := h.runCLI(ctx, root, defaultCLITimeout, "alpha", "logout")
	if err != nil {
		return CLICommandResult{}, err
	}
	return commandResult(formatCLICommand("alpha", "logout"), raw, exitCode), nil
}

// CLIAlphaSearch executes `feynman alpha search`.
func (h *ResearchManager) CLIAlphaSearch(ctx context.Context, root, query, mode string) (CLIAlphaSearchResult, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return CLIAlphaSearchResult{}, errors.New("Search query is required.")
	}
	mode = strings.TrimSpace(strings.ToLower(mode))
	if mode == "" {
		mode = "semantic"
	}
	switch mode {
	case "semantic", "keyword", "both", "agentic", "all":
	default:
		return CLIAlphaSearchResult{}, fmt.Errorf("Unsupported alphaXiv search mode %q.", mode)
	}
	args := []string{"alpha", "search", "--mode", mode, "--", query}
	raw, exitCode, err := h.runCLI(ctx, root, longCLITimeout, args...)
	if err != nil {
		return CLIAlphaSearchResult{}, err
	}
	res := parseAlphaSearchOutput(raw, exitCode, query, mode)
	res.Command = formatCLICommand("alpha", "search", "--mode", mode, query)
	return res, nil
}

func parseAlphaSearchOutput(raw string, exitCode int, query, mode string) CLIAlphaSearchResult {
	res := CLIAlphaSearchResult{
		RawOutput: raw,
		ExitCode:  exitCode,
		Error:     firstNonZeroError(exitCode, raw),
		Query:     query,
		Mode:      mode,
	}
	if exitCode != 0 {
		return res
	}
	var obj struct {
		Papers  []CLIAlphaPaper `json:"papers"`
		Results []CLIAlphaPaper `json:"results"`
	}
	if json.Unmarshal([]byte(raw), &obj) == nil && (obj.Papers != nil || obj.Results != nil) {
		res.Parsed = true
		if obj.Papers != nil {
			res.Papers = obj.Papers
		} else {
			res.Papers = obj.Results
		}
		return res
	}
	var list []CLIAlphaPaper
	if json.Unmarshal([]byte(raw), &list) == nil && list != nil {
		res.Parsed = true
		res.Papers = list
		return res
	}
	low := strings.ToLower(raw)
	if strings.Contains(low, "no results") || strings.Contains(low, "no papers found") {
		res.Parsed = true
		return res
	}
	// Parse structured numbered text output fallback if not JSON.
	numbered := regexp.MustCompile(`^\d+\.\s+(.+)$`)
	arxivIDRe := regexp.MustCompile(`\b(\d{4}\.\d{4,5}(?:v\d+)?)\b`)
	var cur *CLIAlphaPaper
	for _, line := range strings.Split(raw, "\n") {
		trimmed := strings.TrimSpace(line)
		if m := numbered.FindStringSubmatch(trimmed); m != nil {
			res.Papers = append(res.Papers, CLIAlphaPaper{Title: strings.TrimSpace(m[1])})
			cur = &res.Papers[len(res.Papers)-1]
			if idm := arxivIDRe.FindStringSubmatch(m[1]); idm != nil {
				cur.ID = idm[1]
			}
			continue
		}
		if cur != nil && trimmed != "" {
			if idm := arxivIDRe.FindStringSubmatch(trimmed); idm != nil && cur.ID == "" {
				cur.ID = idm[1]
			}
			if strings.HasPrefix(trimmed, "http://") || strings.HasPrefix(trimmed, "https://") {
				cur.URL = trimmed
			} else if cur.Summary == "" {
				cur.Summary = trimmed
			}
		}
	}
	res.Parsed = len(res.Papers) > 0
	return res
}

// CLIAlphaGet executes `feynman alpha get`.
func (h *ResearchManager) CLIAlphaGet(ctx context.Context, root, paperID string, fullText bool) (CLIAlphaPaperResult, error) {
	paperID = strings.TrimSpace(paperID)
	if paperID == "" || strings.HasPrefix(paperID, "-") {
		return CLIAlphaPaperResult{}, errors.New("Paper ID or URL is required.")
	}
	args := []string{"alpha", "get"}
	if fullText {
		args = append(args, "--full-text")
	}
	args = append(args, paperID)
	raw, exitCode, err := h.runCLI(ctx, root, longCLITimeout, args...)
	if err != nil {
		return CLIAlphaPaperResult{}, err
	}
	res := parseAlphaGetOutput(raw, exitCode, paperID, fullText)
	res.Command = formatCLICommand(args...)
	return res, nil
}

func parseAlphaGetOutput(raw string, exitCode int, paperID string, fullText bool) CLIAlphaPaperResult {
	res := CLIAlphaPaperResult{
		RawOutput: raw,
		ExitCode:  exitCode,
		Error:     firstNonZeroError(exitCode, raw),
		PaperID:   paperID,
		FullText:  fullText,
	}
	if exitCode != 0 {
		return res
	}
	var payload struct {
		ID         string `json:"id"`
		Title      string `json:"title"`
		Content    string `json:"content"`
		Text       string `json:"text"`
		Report     string `json:"report"`
		Annotation string `json:"annotation"`
	}
	if json.Unmarshal([]byte(raw), &payload) == nil && (payload.Title != "" || payload.Content != "" || payload.Text != "" || payload.Report != "") {
		res.Parsed = true
		if payload.ID != "" {
			res.PaperID = payload.ID
		}
		res.Title = payload.Title
		res.Annotation = payload.Annotation
		switch {
		case payload.Content != "":
			res.Content = payload.Content
		case payload.Text != "":
			res.Content = payload.Text
		default:
			res.Content = payload.Report
		}
		return res
	}
	if raw != "" {
		res.Parsed = true
		res.Title = strings.TrimSpace(strings.TrimPrefix(firstLine(raw), "#"))
		res.Content = raw
	}
	return res
}

// CLIAlphaAsk executes `feynman alpha ask`.
func (h *ResearchManager) CLIAlphaAsk(ctx context.Context, root, paperID, question string) (CLICommandResult, error) {
	paperID = strings.TrimSpace(paperID)
	question = strings.TrimSpace(question)
	if paperID == "" || strings.HasPrefix(paperID, "-") {
		return CLICommandResult{}, errors.New("Paper ID or URL is required.")
	}
	if question == "" {
		return CLICommandResult{}, errors.New("Question is required.")
	}
	args := []string{"alpha", "ask", paperID, question}
	raw, exitCode, err := h.runCLI(ctx, root, longCLITimeout, args...)
	if err != nil {
		return CLICommandResult{}, err
	}
	return commandResult(formatCLICommand(args...), raw, exitCode), nil
}

// CLIAlphaCode executes `feynman alpha code`.
func (h *ResearchManager) CLIAlphaCode(ctx context.Context, root, repoURL, filePath string) (CLICommandResult, error) {
	repoURL = strings.TrimSpace(repoURL)
	filePath = strings.TrimSpace(filePath)
	if repoURL == "" || strings.HasPrefix(repoURL, "-") {
		return CLICommandResult{}, errors.New("GitHub repository URL is required.")
	}
	if strings.HasPrefix(filePath, "-") {
		return CLICommandResult{}, errors.New("Invalid repository file path.")
	}
	args := []string{"alpha", "code", repoURL}
	if filePath != "" {
		args = append(args, filePath)
	}
	raw, exitCode, err := h.runCLI(ctx, root, longCLITimeout, args...)
	if err != nil {
		return CLICommandResult{}, err
	}
	return commandResult(formatCLICommand(args...), raw, exitCode), nil
}

// CLIAlphaAnnotate executes `feynman alpha annotate` (list, get, set, clear).
func (h *ResearchManager) CLIAlphaAnnotate(ctx context.Context, root, action, paperID, note string) (CLIAlphaAnnotateResult, error) {
	action = strings.TrimSpace(action)
	paperID = strings.TrimSpace(paperID)
	note = strings.TrimSpace(note)
	if strings.HasPrefix(paperID, "-") {
		return CLIAlphaAnnotateResult{}, errors.New("Invalid paper ID.")
	}
	var args []string
	switch action {
	case "list":
		args = []string{"alpha", "annotate", "--list"}
	case "get":
		if paperID == "" {
			return CLIAlphaAnnotateResult{}, errors.New("Paper ID is required.")
		}
		args = []string{"alpha", "annotate", paperID}
	case "set":
		if paperID == "" || note == "" {
			return CLIAlphaAnnotateResult{}, errors.New("Paper ID and annotation note are required.")
		}
		args = []string{"alpha", "annotate", paperID, note}
	case "clear":
		if paperID == "" {
			return CLIAlphaAnnotateResult{}, errors.New("Paper ID is required.")
		}
		args = []string{"alpha", "annotate", "--clear", paperID}
	default:
		return CLIAlphaAnnotateResult{}, fmt.Errorf("Unsupported annotate action %q.", action)
	}
	raw, exitCode, err := h.runCLI(ctx, root, defaultCLITimeout, args...)
	if err != nil {
		return CLIAlphaAnnotateResult{}, err
	}
	res := CLIAlphaAnnotateResult{
		Command:   formatCLICommand(args...),
		RawOutput: raw,
		ExitCode:  exitCode,
		Error:     firstNonZeroError(exitCode, raw),
		PaperID:   paperID,
		Note:      note,
	}
	if exitCode != 0 {
		return res, nil
	}
	switch action {
	case "list":
		if strings.EqualFold(raw, "No annotations.") {
			res.Parsed = true
			return res, nil
		}
		for _, line := range strings.Split(raw, "\n") {
			if k, v, ok := splitKeyValue(strings.TrimSpace(line)); ok {
				res.Annotations = append(res.Annotations, CLIAlphaAnnotation{PaperID: k, Note: v})
			}
		}
		res.Parsed = len(res.Annotations) > 0
	case "get":
		res.Parsed = true
		if !strings.Contains(strings.ToLower(raw), "no annotation") {
			res.Note = raw
		} else {
			res.Note = ""
		}
	case "set", "clear":
		res.Parsed = true
	}
	return res, nil
}

// Ensure strconv is referenced if needed for numeric parsing in future extensions.
var _ = strconv.Itoa
