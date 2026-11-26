package pandoc

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Bayern99/mark-go-go/internal/export"
)

const readerNoteHeader = `<style>.reader-note{border-left:3px solid #6e6e73;padding:.6em .9em;background:#f4f4f4;color:#48484d}</style>`

func Render(ctx context.Context, source, formatID string, options export.Options, probed ...Status) (export.Result, error) {
	format, err := export.DefaultRegistry.Lookup(formatID)
	if err != nil || format.Engine != export.EnginePandoc || format.Scope != export.ScopeMarkdown {
		return export.Result{}, fmt.Errorf("unsupported Pandoc export format %q", formatID)
	}
	status := Status{}
	if len(probed) > 0 {
		status = probed[0]
	} else {
		status = Probe(ctx, "")
	}
	if !status.Available {
		reason := status.Error
		if reason == "" {
			reason = "Pandoc 2.15.0 or later is required."
		}
		return export.Result{}, fmt.Errorf("%s Install with %s.", reason, InstallCommand)
	}
	markdown, attachments, warnings, err := PreprocessWithAttachments(source, formatID, options)
	if err != nil {
		return export.Result{Warnings: warnings}, err
	}
	tempDir, err := os.MkdirTemp("", "mgg-pandoc-*")
	if err != nil {
		return export.Result{Warnings: warnings}, err
	}
	defer os.RemoveAll(tempDir)
	for _, attachment := range attachments {
		outputPath := filepath.Join(tempDir, filepath.FromSlash(attachment.Name))
		if err := os.MkdirAll(filepath.Dir(outputPath), 0o700); err != nil {
			return export.Result{Warnings: warnings}, err
		}
		if err := os.WriteFile(outputPath, attachment.Data, 0o600); err != nil {
			return export.Result{Warnings: warnings}, err
		}
	}
	inputPath := filepath.Join(tempDir, "doc.md")
	if err := os.WriteFile(inputPath, []byte(markdown), 0o600); err != nil {
		return export.Result{Warnings: warnings}, err
	}
	outputPath := filepath.Join(tempDir, "document"+format.Ext)
	args := pandocArguments(formatID, options.Title, inputPath, outputPath, tempDir)
	if formatID == "markdown-epub" {
		headerPath := filepath.Join(tempDir, "reader-note.html")
		if err := os.WriteFile(headerPath, []byte(readerNoteHeader), 0o600); err != nil {
			return export.Result{Warnings: warnings}, err
		}
	}
	renderCtx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	command := commandContext(renderCtx, status.Path, args...)
	command.Dir = tempDir
	var stderr bytes.Buffer
	command.Stderr = &stderr
	runErr := command.Run()
	warnings = append(warnings, pandocWarnings(stderr.String())...)
	if runErr != nil {
		detail := stderrDetail(stderr.String())
		if detail != "" {
			return export.Result{Warnings: warnings}, fmt.Errorf("pandoc %s failed: %w: %s", formatID, runErr, detail)
		}
		return export.Result{Warnings: warnings}, fmt.Errorf("pandoc %s failed: %w", formatID, runErr)
	}
	data, err := os.ReadFile(outputPath)
	if err != nil {
		return export.Result{Warnings: warnings}, fmt.Errorf("read Pandoc output: %w", err)
	}
	return export.Result{Data: data, Warnings: warnings, Attachments: attachments}, nil
}

func pandocArguments(formatID, title, inputPath, outputPath, tempDir string) []string {
	args := []string{"-f", "markdown"}
	switch formatID {
	case "markdown-docx":
		args = append(args, "-t", "docx", "--sandbox")
	case "markdown-pptx":
		args = append(args, "-t", "pptx", "--sandbox", "--slide-level=2")
	case "markdown-epub":
		args = append(args, "-t", "epub", "--sandbox", "-M", "title="+exportTitle(title), "-H", filepath.Join(tempDir, "reader-note.html"))
	case "markdown-rtf":
		args = append(args, "-t", "rtf", "--sandbox")
	case "markdown-latex":
		args = append(args, "-t", "latex", "-s", "--sandbox", "-M", "title="+exportTitle(title))
	}
	return append(args, "-o", outputPath, inputPath)
}

func exportTitle(title string) string {
	if strings.TrimSpace(title) == "" {
		return "Mark Go Go Export"
	}
	return title
}

func pandocWarnings(stderr string) []string {
	var warnings []string
	for _, line := range strings.Split(strings.ReplaceAll(stderr, "\r\n", "\n"), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "[WARNING]") {
			warnings = append(warnings, line)
		}
	}
	return warnings
}

func stderrDetail(stderr string) string {
	var lines []string
	for _, line := range strings.Split(strings.ReplaceAll(stderr, "\r\n", "\n"), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			lines = append(lines, line)
		}
	}
	if len(lines) > 8 {
		lines = lines[len(lines)-8:]
	}
	detail := strings.Join(lines, "\n")
	if len(detail) > 4000 {
		detail = detail[len(detail)-4000:]
	}
	return detail
}
