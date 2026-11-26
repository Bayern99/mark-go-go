package pandoc

import (
	"archive/zip"
	"bytes"
	"context"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Bayern99/mark-go-go/internal/export"
)

func TestRenderArgumentMatrixAndWarnings(t *testing.T) {
	installFakeCommand(t)
	for _, tc := range []struct {
		formatID, writer string
		required         []string
	}{
		{"markdown-docx", "docx", []string{"--sandbox"}},
		{"markdown-pptx", "pptx", []string{"--sandbox", "--slide-level=2"}},
		{"markdown-epub", "epub", []string{"--sandbox", "-M", "title=Integration Guide", "-H"}},
		{"markdown-rtf", "rtf", []string{"--sandbox"}},
		{"markdown-latex", "latex", []string{"-s", "--sandbox", "-M", "title=Integration Guide"}},
	} {
		t.Run(tc.formatID, func(t *testing.T) {
			record := filepath.Join(t.TempDir(), "args.txt")
			input := filepath.Join(t.TempDir(), "input.md")
			header := filepath.Join(t.TempDir(), "header.html")
			t.Setenv("MGG_PANDOC_RECORD", record)
			t.Setenv("MGG_PANDOC_INPUT", input)
			t.Setenv("MGG_PANDOC_HEADER", header)
			t.Setenv("MGG_PANDOC_FAIL", "")
			result, err := Render(context.Background(), "# Hello\n", tc.formatID, export.Options{Title: "Integration Guide"}, Status{Available: true, Path: "/fake/pandoc", Version: "3.13"})
			if err != nil || string(result.Data) != "fake Pandoc output" {
				t.Fatalf("Render = %q, %v", result.Data, err)
			}
			argsData, err := os.ReadFile(record)
			if err != nil {
				t.Fatal(err)
			}
			args := strings.Split(strings.TrimSpace(string(argsData)), "\n")
			if !slices.Contains(args, "-t") || !slices.Contains(args, tc.writer) {
				t.Fatalf("args = %v, want writer %s", args, tc.writer)
			}
			for _, required := range tc.required {
				if !slices.Contains(args, required) && !strings.Contains(strings.Join(args, "\n"), required) {
					t.Errorf("args = %v, missing %q", args, required)
				}
			}
			if slices.Contains(args, "--css") {
				t.Errorf("args contain forbidden --css: %v", args)
			}
			if !slices.Contains(result.Warnings, "[WARNING] collected warning") {
				t.Errorf("warnings = %v", result.Warnings)
			}
			inputData, err := os.ReadFile(input)
			if err != nil || !strings.Contains(string(inputData), "# Hello") {
				t.Fatalf("preprocessed input = %s, %v", inputData, err)
			}
			if tc.formatID == "markdown-epub" {
				headerData, err := os.ReadFile(header)
				if err != nil || !strings.Contains(string(headerData), ".reader-note") {
					t.Fatalf("EPUB header = %s, %v", headerData, err)
				}
			}
		})
	}
}

func TestRenderExit99IncludesStderr(t *testing.T) {
	installFakeCommand(t)
	t.Setenv("MGG_PANDOC_FAIL", "1")
	result, err := Render(context.Background(), "# Hello\n", "markdown-pptx", export.Options{}, Status{Available: true, Path: "/fake/pandoc"})
	if err == nil || !strings.Contains(err.Error(), "exit status 99") || !strings.Contains(err.Error(), "fake exit 99") {
		t.Fatalf("Render exit 99 error = %v", err)
	}
	if !slices.Contains(result.Warnings, "[WARNING] fake warning") {
		t.Fatalf("warnings = %v", result.Warnings)
	}
}

func TestPandocRealIntegration(t *testing.T) {
	status := Probe(context.Background(), "")
	if !status.Available {
		t.Skipf("Pandoc unavailable: %s", status.Error)
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "pixel.png"), testPNG(t), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "photo.jpg"), testJPEG(t), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "badge.svg"), []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="32" height="16"><rect width="32" height="16" fill="#2e5d8c"/></svg>`), 0o600); err != nil {
		t.Fatal(err)
	}
	source := "# 中文导出\n\n正文包含公式 $E=mc^2$ 与本地图片。\n\n![Pixel](pixel.png)\n\n![Photo](photo.jpg)\n\n![Badge](badge.svg)\n\n![MissingRef][missingref]\n\n> ```markdown\n> ![Literal](data:image/png;base64,not-valid-base64)\n> ```\n\n<pre>`\n<!-- note: pre-tick-secret -->\n`</pre>\n\n```mermaid\ngraph TD; A[开始]-->B[完成]\n```\n\n<!-- note: phase-three-private-note\n:::\nsecond-half-private-note -->\n\n[missingref]: nonexistent.png\n"
	for _, formatID := range []string{"markdown-docx", "markdown-pptx", "markdown-epub", "markdown-rtf", "markdown-latex"} {
		t.Run(formatID, func(t *testing.T) {
			result, err := Render(context.Background(), source, formatID, export.Options{BaseDir: dir, AssetDir: "guide-assets", Title: "中文导出", IncludeNotes: true, SpaceName: "Water"}, status)
			if err != nil {
				t.Fatalf("real Pandoc Render failed: %v; warnings = %v", err, result.Warnings)
			}
			if len(result.Data) == 0 {
				t.Fatal("real Pandoc output is empty")
			}
			t.Logf("%s: %d bytes, warnings = %v", formatID, len(result.Data), result.Warnings)
			without, err := Render(context.Background(), source, formatID, export.Options{BaseDir: dir, AssetDir: "guide-assets", Title: "中文导出", SpaceName: "Water"}, status)
			if err != nil {
				t.Fatalf("real Pandoc Render (notes off) failed: %v", err)
			}
			switch formatID {
			case "markdown-docx":
				files := unzipFiles(t, result.Data)
				styles := string(files["word/styles.xml"])
				document := string(files["word/document.xml"])
				if !strings.Contains(styles, "ReaderNote") || !strings.Contains(document, "phase-three-private-note") || !strings.Contains(document, "second-half-private-note") || !strings.Contains(document, "中文导出") {
					t.Fatalf("DOCX styles/document missing Reader Note or Chinese content")
				}
				if !zipHasSuffix(files, ".png") || zipHasSuffix(files, ".svg") {
					t.Fatalf("DOCX media = %v", slices.Sorted(mapsKeys(files)))
				}
				withoutFiles := unzipFiles(t, without.Data)
				for _, secret := range []string{"phase-three-private-note", "second-half-private-note", "pre-tick-secret"} {
					if zipContains(withoutFiles, secret) {
						t.Fatalf("DOCX with Reader Notes disabled contains %q", secret)
					}
				}
			case "markdown-pptx":
				files := unzipFiles(t, result.Data)
				if !zipHasSuffix(files, ".png") || zipHasSuffix(files, ".svg") || !zipContains(files, "phase-three-private-note") {
					t.Fatalf("PPTX structure/media is invalid")
				}
				withoutFiles := unzipFiles(t, without.Data)
				for _, secret := range []string{"phase-three-private-note", "pre-tick-secret"} {
					if zipContains(withoutFiles, secret) {
						t.Fatalf("PPTX with Reader Notes disabled contains %q", secret)
					}
				}
			case "markdown-epub":
				files := unzipFiles(t, result.Data)
				if !zipHasSuffix(files, ".svg") || !zipContains(files, "phase-three-private-note") || !zipContains(files, "中文导出") {
					t.Fatalf("EPUB structure is invalid: %v", slices.Sorted(mapsKeys(files)))
				}
				withoutFiles := unzipFiles(t, without.Data)
				for _, secret := range []string{"phase-three-private-note", "pre-tick-secret"} {
					if zipContains(withoutFiles, secret) {
						t.Fatalf("EPUB with Reader Notes disabled contains %q", secret)
					}
				}
			case "markdown-rtf":
				text := string(result.Data)
				if !strings.Contains(text, "\\pngblip") || strings.Contains(text, "image/svg+xml") || !strings.Contains(text, "phase-three-private-note") {
					t.Fatalf("RTF does not contain PNG and Reader Note content")
				}
				for _, secret := range []string{"phase-three-private-note", "pre-tick-secret"} {
					if strings.Contains(string(without.Data), secret) {
						t.Fatalf("RTF with Reader Notes disabled contains %q", secret)
					}
				}
			case "markdown-latex":
				text := string(result.Data)
				if !strings.Contains(text, "\\includegraphics") || !strings.Contains(text, "guide-assets/") || !strings.Contains(text, ".png") || !strings.Contains(text, ".jpg") || strings.Contains(text, ".svg") || strings.Contains(text, "image/svg+xml") || !strings.Contains(text, "phase-three-private-note") || !strings.Contains(text, "中文导出") {
					t.Fatalf("LaTeX output does not meet PNG/JPG/Notes/Chinese requirements:\n%s", text)
				}
				for _, secret := range []string{"phase-three-private-note", "pre-tick-secret"} {
					if strings.Contains(string(without.Data), secret) {
						t.Fatalf("LaTeX with Reader Notes disabled contains %q", secret)
					}
				}
				if len(result.Attachments) != 4 {
					t.Fatalf("LaTeX attachments = %d, want PNG+JPEG+SVG-PNG+Mermaid-PNG", len(result.Attachments))
				}
				hasJPG := false
				for _, attachment := range result.Attachments {
					if !strings.HasPrefix(attachment.Name, "guide-assets/") {
						t.Errorf("attachment outside asset dir: %+v", attachment.Name)
					}
					if strings.HasSuffix(attachment.Name, ".jpg") {
						hasJPG = true
					}
				}
				if !hasJPG {
					t.Fatal("LaTeX did not materialize local JPEG attachment")
				}
			}
		})
	}
}

func testJPEG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{R: 46, G: 93, B: 140, A: 255})
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func unzipFiles(t *testing.T, data []byte) map[string][]byte {
	t.Helper()
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("output is not a ZIP container: %v", err)
	}
	files := map[string][]byte{}
	for _, file := range reader.File {
		if file.FileInfo().IsDir() {
			continue
		}
		rc, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		content, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			t.Fatal(err)
		}
		files[file.Name] = content
	}
	return files
}

func zipHasSuffix(files map[string][]byte, suffix string) bool {
	for name := range files {
		if strings.HasSuffix(name, suffix) {
			return true
		}
	}
	return false
}

func zipContains(files map[string][]byte, text string) bool {
	for _, data := range files {
		if bytes.Contains(data, []byte(text)) {
			return true
		}
	}
	return false
}

func mapsKeys[V any](values map[string]V) func(func(string) bool) {
	return func(yield func(string) bool) {
		for key := range values {
			if !yield(key) {
				return
			}
		}
	}
}
