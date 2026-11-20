package feynman

import (
	"cmp"
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"
)

var (
	alphaSearchNumberedPattern = regexp.MustCompile(`^\d+\.\s+(.+)$`)
	alphaSearchArxivIDPattern  = regexp.MustCompile(`\b(\d{4}\.\d{4,5}(?:v\d+)?)\b`)
	alphaAnnotationHeader      = regexp.MustCompile(`^(\S+) \(([^()]+)\)$`)
)

func firstLine(s string) string {
	for _, line := range strings.Split(s, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
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

func parseMCPListOutput(raw string, exitCode int) CLIMCPListResult {
	res := CLIMCPListResult{
		RawOutput: raw,
		ExitCode:  exitCode,
		Error:     firstNonZeroError(exitCode, raw),
	}
	var payload struct {
		Servers []struct {
			Name      string   `json:"name"`
			Scope     string   `json:"scope"`
			Transport string   `json:"transport"`
			Type      string   `json:"type"`
			URL       string   `json:"url"`
			Command   string   `json:"command"`
			Args      []string `json:"args"`
			Exposure  string   `json:"exposure"`
			Status    string   `json:"status"`
			State     string   `json:"state"`
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
			Scope:     srv.Scope,
			Transport: transport,
			URL:       srv.URL,
			Command:   cmd,
			Exposure:  srv.Exposure,
			Status:    cmp.Or(srv.State, srv.Status),
			Error:     srv.Error,
			Tools:     tools,
		})
	}
	return res
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
	if mode == "both" || mode == "all" {
		var groups map[string]json.RawMessage
		if json.Unmarshal([]byte(raw), &groups) == nil {
			for _, name := range []string{"semantic", "keyword", "agentic"} {
				data, ok := groups[name]
				if !ok {
					continue
				}
				papers := parseAlphaSearchPapers(data)
				if papers == nil {
					return res
				}
				res.Groups = append(res.Groups, CLIAlphaSearchGroup{Mode: name, Papers: papers})
			}
			if len(res.Groups) > 0 {
				res.Parsed = true
				return res
			}
		}
	}
	if papers := parseAlphaSearchPapers([]byte(raw)); papers != nil {
		res.Parsed = true
		res.Papers = papers
		return res
	}
	low := strings.ToLower(raw)
	if strings.Contains(low, "no results") || strings.Contains(low, "no papers found") {
		res.Parsed = true
		return res
	}
	var cur *CLIAlphaPaper
	for _, line := range strings.Split(raw, "\n") {
		trimmed := strings.TrimSpace(line)
		if m := alphaSearchNumberedPattern.FindStringSubmatch(trimmed); m != nil {
			res.Papers = append(res.Papers, CLIAlphaPaper{Title: strings.TrimSpace(m[1])})
			cur = &res.Papers[len(res.Papers)-1]
			if idm := alphaSearchArxivIDPattern.FindStringSubmatch(m[1]); idm != nil {
				cur.ID = idm[1]
			}
			continue
		}
		if cur != nil && trimmed != "" {
			if idm := alphaSearchArxivIDPattern.FindStringSubmatch(trimmed); idm != nil && cur.ID == "" {
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

func parseAlphaSearchPapers(raw []byte) []CLIAlphaPaper {
	type paper struct {
		CLIAlphaPaper
		PaperID string `json:"paperId"`
		Link    string `json:"link"`
	}
	var rows []paper
	if json.Unmarshal(raw, &rows) != nil {
		var obj struct {
			Papers  []paper `json:"papers"`
			Results []paper `json:"results"`
		}
		if json.Unmarshal(raw, &obj) != nil {
			return nil
		}
		rows = obj.Papers
		if rows == nil {
			rows = obj.Results
		}
	}
	if rows == nil {
		return nil
	}
	papers := make([]CLIAlphaPaper, 0, len(rows))
	for _, row := range rows {
		p := row.CLIAlphaPaper
		p.ID = cmp.Or(p.ID, row.PaperID)
		p.URL = cmp.Or(p.URL, row.Link)
		p.Summary = cmp.Or(p.Summary, p.Snippet)
		if p.Title == "" {
			return nil
		}
		papers = append(papers, p)
	}
	return papers
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

func parseAlphaAnnotationList(raw string) []CLIAlphaAnnotation {
	var annotations []CLIAlphaAnnotation
	var note []string
	for _, line := range strings.Split(raw, "\n") {
		match := alphaAnnotationHeader.FindStringSubmatch(line)
		if match != nil {
			if _, err := time.Parse(time.RFC3339Nano, match[2]); err == nil {
				if len(annotations) > 0 {
					annotations[len(annotations)-1].Note = strings.TrimRight(strings.Join(note, "\n"), "\n")
				}
				annotations = append(annotations, CLIAlphaAnnotation{PaperID: match[1], UpdatedAt: match[2]})
				note = nil
				continue
			}
		}
		if len(annotations) > 0 {
			if len(note) == 0 {
				line = strings.TrimPrefix(line, "  ")
			}
			note = append(note, line)
		}
	}
	if len(annotations) > 0 {
		annotations[len(annotations)-1].Note = strings.TrimRight(strings.Join(note, "\n"), "\n")
		return annotations
	}
	for _, line := range strings.Split(raw, "\n") {
		if k, v, ok := splitKeyValue(strings.TrimSpace(line)); ok && alphaSearchArxivIDPattern.MatchString(k) {
			annotations = append(annotations, CLIAlphaAnnotation{PaperID: k, Note: v})
		}
	}
	return annotations
}
