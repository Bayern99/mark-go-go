package pandoc

import (
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Bayern99/mark-go-go/internal/export"
	"github.com/Bayern99/mark-go-go/internal/export/assets"
	"github.com/Bayern99/mark-go-go/internal/portrait"
	mermaid "github.com/zkrebbekx/go-mermaid"
)

func TestMain(m *testing.M) {
	if err := portrait.RegisterFonts(); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}

func TestPreprocessReaderNotes(t *testing.T) {
	source := "# Guide\n\nVisible paragraph.\n<!-- note: private reader note\n:::\nafter colons -->\n\n<pre>`\n<!-- note: pre-tick private note -->\n`</pre>\n\n<pre>\n<!-- note: residual private note -->\n</pre>\n\n    <!-- note: indented code sample -->\n\n`<!-- note: inline code sample -->`\n\n```text\n<!-- note: fenced code sample -->\n```\n"
	without, _, err := Preprocess(source, "markdown-docx", export.Options{})
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"private reader note", "after colons", "pre-tick private note", "residual private note"} {
		if strings.Contains(without, secret) {
			t.Fatalf("disabled Reader Notes left %q in:\n%s", secret, without)
		}
	}
	for _, sample := range []string{"indented code sample", "inline code sample", "fenced code sample"} {
		if !strings.Contains(without, sample) {
			t.Errorf("code sample %q was changed:\n%s", sample, without)
		}
	}
	included, _, err := Preprocess(source, "markdown-docx", export.Options{IncludeNotes: true})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(included, "::: {.reader-note custom-style=\"Reader Note\"}\nprivate reader note\n\\:::\nafter colons\n:::") {
		t.Fatalf("enabled Reader Note did not escape inner colons:\n%s", included)
	}
	if strings.Contains(included, "residual private note") || strings.Contains(included, "pre-tick private note") || strings.Contains(included, "<!-- note: private reader note") {
		t.Fatalf("enabled Reader Notes left a source comment behind:\n%s", included)
	}
	afterPre, _, err := Preprocess("# Guide\n\n<pre>code</pre>\n<!-- note: note after HTML -->\n", "markdown-docx", export.Options{IncludeNotes: true})
	if err != nil || !strings.Contains(afterPre, "custom-style=\"Reader Note\"}\nnote after HTML\n:::") {
		t.Fatalf("Reader Note immediately after HTML was not preserved: %s, %v", afterPre, err)
	}
}

func TestPreprocessImagesInlineWithinRootAndPlaceholderFailures(t *testing.T) {
	dir := t.TempDir()
	png := testPNG(t)
	if err := os.WriteFile(filepath.Join(dir, "pixel.png"), png, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "large.png"), make([]byte, assets.MaxImageBytes+1), 0o600); err != nil {
		t.Fatal(err)
	}
	svg := `<svg xmlns="http://www.w3.org/2000/svg" width="40" height="20"><rect width="40" height="20" fill="#2e5d8c"/></svg>`
	if err := os.WriteFile(filepath.Join(dir, "badge.svg"), []byte(svg), 0o600); err != nil {
		t.Fatal(err)
	}
	source := "![Local](pixel.png \"Pixel\")\n\n![Ref][good]\n\n![RefMiss][miss]\n\n![Badge](badge.svg)\n\n![Missing](missing.png)\n\n![Outside](../outside.png)\n\n![Large](large.png)\n\n![Remote](https://example.com/remote.png)\n\n> ```markdown\n> ![QuotedFence](pixel.png)\n> ```\n\n`![Code](pixel.png)`\n\n```markdown\n![Fence](pixel.png)\n```\n\n[good]: pixel.png\n[miss]: ref-missing.png\n"
	processed, warnings, err := Preprocess(source, "markdown-pptx", export.Options{BaseDir: dir})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(processed, "![Local](data:image/png;base64,"+base64.StdEncoding.EncodeToString(png)+" \"Pixel\")") {
		t.Fatalf("local image was not inlined with its title:\n%s", processed)
	}
	if !strings.Contains(processed, "![Ref](data:image/png;base64,"+base64.StdEncoding.EncodeToString(png)+")") {
		t.Fatalf("reference local image was not inlined:\n%s", processed)
	}
	if strings.Contains(processed, "image/svg+xml") || !strings.Contains(processed, "![Badge](data:image/png;base64,") {
		t.Fatalf("local SVG was not rasterized for non-EPUB:\n%s", processed)
	}
	for _, name := range []string{"ref-missing.png", "missing.png", "outside.png", "large.png", "remote.png"} {
		if !strings.Contains(processed, "*[Image unavailable: "+name+"]*") {
			t.Errorf("missing placeholder for %s:\n%s", name, processed)
		}
	}
	if !strings.Contains(processed, "`![Code](pixel.png)`") || !strings.Contains(processed, "```markdown\n![Fence](pixel.png)\n```") || !strings.Contains(processed, "> ![QuotedFence](pixel.png)") {
		t.Fatalf("image rewriting entered code:\n%s", processed)
	}
	if len(warnings) != 5 {
		t.Fatalf("warnings = %v, want five image failures", warnings)
	}
}

func TestPreprocessMermaidTargetFormats(t *testing.T) {
	source := "# Diagram\n\n```mermaid\ngraph TD; A[开始]-->B[完成]\n```\n"
	for _, formatID := range []string{"markdown-docx", "markdown-pptx", "markdown-epub", "markdown-rtf", "markdown-latex"} {
		t.Run(formatID, func(t *testing.T) {
			processed, warnings, err := Preprocess(source, formatID, export.Options{SpaceName: "Water"})
			if err != nil || len(warnings) != 0 {
				t.Fatalf("Preprocess warnings = %v, err = %v", warnings, err)
			}
			if strings.Contains(processed, "```mermaid") {
				t.Fatalf("Mermaid block was not replaced:\n%s", processed)
			}
			if formatID == "markdown-epub" {
				if !strings.Contains(processed, "data:image/svg+xml;base64,") || strings.Contains(processed, "data:image/png;base64,") {
					t.Fatalf("EPUB did not use SVG:\n%s", processed)
				}
				encoded := strings.SplitN(strings.SplitN(processed, "data:image/svg+xml;base64,", 2)[1], ")", 2)[0]
				svg, err := base64.StdEncoding.DecodeString(encoded)
				if err != nil || strings.Contains(string(svg), "var(--") || strings.Contains(string(svg), "currentColor") || strings.Contains(string(svg), "rgba(") {
					t.Fatalf("EPUB SVG has non-concrete colors or invalid base64: %v\n%s", err, svg)
				}
				return
			}
			if formatID == "markdown-latex" {
				if !strings.Contains(processed, "](media/image-1.png){width=") || !strings.Contains(processed, "px height=") || strings.Contains(processed, "image/svg+xml") {
					t.Fatalf("LaTeX did not materialize sized PNG attachment:\n%s", processed)
				}
				return
			}
			if !strings.Contains(processed, "data:image/png;base64,") || !strings.Contains(processed, "{width=") || !strings.Contains(processed, "px height=") {
				t.Fatalf("%s did not use a sized PNG:\n%s", formatID, processed)
			}
			if formatID == "markdown-rtf" {
				if strings.Contains(processed, "image/svg+xml") {
					t.Fatalf("%s retained SVG data:\n%s", formatID, processed)
				}
			}
		})
	}
}

func TestPreprocessMermaidFailureKeepsCodeAndNamesDiagram(t *testing.T) {
	previous := renderMermaidSVG
	t.Cleanup(func() { renderMermaidSVG = previous })
	renderMermaidSVG = func(string, ...mermaid.Option) ([]byte, error) {
		return nil, errors.New("fake renderer failure")
	}
	processed, warnings, err := Preprocess("```mermaid\ngraph TD; A-->B\n```\n", "markdown-docx", export.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(processed, "```mermaid\ngraph TD; A-->B\n```") || len(warnings) != 1 || !strings.Contains(warnings[0], "Mermaid diagram 1") || !strings.Contains(warnings[0], "graph TD") || !strings.Contains(warnings[0], "fake renderer failure") {
		t.Fatalf("failed Mermaid fallback = %s, warnings = %v", processed, warnings)
	}
}

func testPNG(t *testing.T) []byte {
	t.Helper()
	data, err := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=")
	if err != nil {
		t.Fatal(err)
	}
	return data
}
