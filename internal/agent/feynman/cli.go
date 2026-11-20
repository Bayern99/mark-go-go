package feynman

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"sync"
)

var searchConfigMu sync.Mutex

func (h *ResearchManager) CLIStatus(ctx context.Context, root string) (CLIStatusResult, error) {
	raw, exitCode, err := h.runCLI(ctx, root, defaultCLITimeout, "status")
	if err != nil {
		return CLIStatusResult{}, err
	}
	res := parseStatusOutput(raw, exitCode)
	res.Command = formatCLICommand("status")
	return res, nil
}

func (h *ResearchManager) CLIDoctor(ctx context.Context, root string) (CLIDoctorResult, error) {
	raw, exitCode, err := h.runCLI(ctx, root, defaultCLITimeout, "doctor")
	if err != nil {
		return CLIDoctorResult{}, err
	}
	res := parseDoctorOutput(raw, exitCode)
	res.Command = formatCLICommand("doctor")
	return res, nil
}

func (h *ResearchManager) CLISetupPreview(ctx context.Context, root string) (CLICommandResult, error) {
	return h.runCLICommand(ctx, root, longCLITimeout, "setup", "preview")
}

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
		args = []string{"model", "tier"}
		if value != "" {
			args = append(args, value)
		}
	case "logout":
		if value == "" {
			return CLICommandResult{}, errors.New("Provider ID is required.")
		}
		args = []string{"model", "logout", value}
	default:
		return CLICommandResult{}, fmt.Errorf("Unsupported model action %q.", action)
	}
	return h.runCLICommand(ctx, root, defaultCLITimeout, args...)
}

func (h *ResearchManager) CLISearchStatus(ctx context.Context, root string) (CLISearchStatusResult, error) {
	raw, exitCode, err := h.runCLI(ctx, root, defaultCLITimeout, "search", "status")
	if err != nil {
		return CLISearchStatusResult{}, err
	}
	res := parseSearchStatusOutput(raw, exitCode)
	res.Command = formatCLICommand("search", "status")
	return res, nil
}

func (h *ResearchManager) CLISearchAction(ctx context.Context, root, action, provider, apiKey string) (CLICommandResult, error) {
	action = strings.TrimSpace(action)
	provider = strings.TrimSpace(provider)
	apiKey = strings.TrimSpace(apiKey)
	if strings.HasPrefix(provider, "-") {
		return CLICommandResult{}, errors.New("Invalid web search provider.")
	}
	searchConfigMu.Lock()
	defer searchConfigMu.Unlock()
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
	case "clear-key":
		return h.clearSearchKey(ctx, root, provider)
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

func (h *ResearchManager) clearSearchKey(ctx context.Context, root, provider string) (CLICommandResult, error) {
	status, err := h.CLISearchStatus(ctx, root)
	if err != nil {
		return CLICommandResult{}, err
	}
	path := filepath.Join(h.profileHome(), ".feynman", "agent", "web-search.json")
	if !status.Parsed || !strings.EqualFold(status.ManagedBy, "pi-web-access") || filepath.Clean(status.ConfigPath) != path {
		return CLICommandResult{}, errors.New("Could not verify the app's Web Search settings file.")
	}
	keys := strings.Split(status.SavedAPIKeys, ",")
	if provider == "" || provider == "none" || !slices.ContainsFunc(keys, func(key string) bool {
		return strings.TrimSpace(key) == provider
	}) {
		return CLICommandResult{}, errors.New("No saved API key for that Web Search provider.")
	}
	if ctx != nil && ctx.Err() != nil {
		return CLICommandResult{}, ctx.Err()
	}
	if err := removeSearchAPIKey(path, provider); err != nil {
		return CLICommandResult{}, err
	}
	return CLICommandResult{Parsed: true, Summary: fmt.Sprintf("Cleared the saved %s API key.", provider)}, nil
}

func (h *ResearchManager) CLIMCPList(ctx context.Context, root string) (CLIMCPListResult, error) {
	raw, exitCode, err := h.runCLI(ctx, root, defaultCLITimeout, "mcp", "list", "--json")
	if err != nil {
		return CLIMCPListResult{}, err
	}
	res := parseMCPListOutput(raw, exitCode)
	res.Command = formatCLICommand("mcp", "list", "--json")
	return res, nil
}

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
	return h.runCLICommand(ctx, root, defaultCLITimeout, args...)
}

func (h *ResearchManager) CLIMCPRemove(ctx context.Context, root, name string, local bool) (CLICommandResult, error) {
	name = strings.TrimSpace(name)
	if name == "" || strings.HasPrefix(name, "-") {
		return CLICommandResult{}, errors.New("MCP server name is required.")
	}
	args := []string{"mcp", "remove", name}
	if local {
		args = append(args, "-l")
	}
	return h.runCLICommand(ctx, root, defaultCLITimeout, args...)
}

func (h *ResearchManager) CLIPackagesList(ctx context.Context, root string) (CLIPackageListResult, error) {
	raw, exitCode, err := h.runCLI(ctx, root, defaultCLITimeout, "packages", "list")
	if err != nil {
		return CLIPackageListResult{}, err
	}
	res := parsePackagesListOutput(raw, exitCode)
	res.Command = formatCLICommand("packages", "list")
	return res, nil
}

func (h *ResearchManager) CLIPackageAction(ctx context.Context, root, action, target string) (CLICommandResult, error) {
	action = strings.TrimSpace(action)
	target = strings.TrimSpace(target)
	if strings.HasPrefix(target, "-") {
		return CLICommandResult{}, errors.New("Invalid package name.")
	}
	var args []string
	switch action {
	case "install", "remove":
		if target == "" {
			return CLICommandResult{}, errors.New("Package preset name is required.")
		}
		args = []string{"packages", action, target}
	case "update":
		args = []string{"update"}
		if target != "" {
			args = append(args, target)
		}
	default:
		return CLICommandResult{}, fmt.Errorf("Unsupported package action %q.", action)
	}
	return h.runCLICommand(ctx, root, longCLITimeout, args...)
}

func (h *ResearchManager) CLIAlphaStatus(ctx context.Context, root string) (CLIAlphaStatusResult, error) {
	raw, exitCode, err := h.runCLI(ctx, root, defaultCLITimeout, "alpha", "status")
	if err != nil {
		return CLIAlphaStatusResult{}, err
	}
	res := parseAlphaStatusOutput(raw, exitCode)
	res.Command = formatCLICommand("alpha", "status")
	return res, nil
}

func (h *ResearchManager) CLIAlphaLogout(ctx context.Context, root string) (CLICommandResult, error) {
	return h.runCLICommand(ctx, root, defaultCLITimeout, "alpha", "logout")
}

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
	args := []string{"alpha", "search", "--json", "--mode", mode, "--", query}
	raw, exitCode, err := h.runCLI(ctx, root, longCLITimeout, args...)
	if err != nil {
		return CLIAlphaSearchResult{}, err
	}
	res := parseAlphaSearchOutput(raw, exitCode, query, mode)
	res.Command = formatCLICommand(args...)
	return res, nil
}

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
	return h.runCLICommand(ctx, root, longCLITimeout, args...)
}

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
	return h.runCLICommand(ctx, root, longCLITimeout, args...)
}

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
		res.Annotations = parseAlphaAnnotationList(raw)
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
