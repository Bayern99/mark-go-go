package markdown

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Bayern99/mark-go-go/internal/export"
)

func TestStandaloneMarkdown(t *testing.T) {
	dir := t.TempDir()
	png, _ := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=")
	if err := os.WriteFile(filepath.Join(dir, "pixel.png"), png, 0o600); err != nil {
		t.Fatal(err)
	}
	source := `# Guide

Paragraph **bold** *em* ~~gone~~ ` + "`code`" + ` and $x^2$.[^a]

<!-- note:
paragraph secret
-->

- [x] done
- [ ] next

> [!TIP]
> Useful.

| Name | Count |
|:-----|------:|
| One  | 2     |

![local](pixel.png)
![missing](missing.png)
![remote](https://example.com/pixel.png)

[unsafe](javascript:alert(1))

<script>alert(1)</script>

$$
a+b=c
$$

` + "```mermaid\ngraph TD; A-->B\n```" + `

` + "```go\npackage main\n```" + `

[^a]: Footnote text.
`
	result, err := Render(source, export.Options{SpaceName: "Fire", Dark: true, PrintLight: true, BaseDir: dir, Title: "Guide"})
	if err != nil {
		t.Fatal(err)
	}
	html := string(result.Data)
	for _, want := range []string{
		`<h1 id="guide">`, "<strong>bold</strong>", "<em>em</em>", "<del>gone</del>",
		`type="checkbox" disabled checked`, "markdown-alert-tip", `align="right"`,
		"data:image/png;base64,", "Image unavailable: missing.png", `src="https://example.com/pixel.png"`,
		"class=\"math\"", "<svg", "class=\"mermaid\"", "tok-keyword", `class="footnotes"`,
		"@media print", "color-scheme:dark",
	} {
		if !strings.Contains(html, want) {
			t.Errorf("output is missing %q", want)
		}
	}
	for _, unwanted := range []string{"paragraph secret", "<!-- note", "<script", "prefers-color-scheme", `href="javascript:`} {
		if strings.Contains(html, unwanted) {
			t.Errorf("output contains %q", unwanted)
		}
	}

	included, err := Render(source, export.Options{BaseDir: dir, IncludeNotes: true})
	if err != nil {
		t.Fatal(err)
	}
	text := string(included.Data)
	if !strings.Contains(text, "paragraph secret") || !strings.Contains(text, `class="reader-note"`) {
		t.Fatalf("included note is missing: %s", text)
	}
	if !strings.Contains(text, "</p>\n<aside class=\"reader-note\"") {
		t.Error("paragraph note is not inline after its paragraph")
	}
}

func TestMermaidExternalReferenceFallsBack(t *testing.T) {
	for _, click := range []string{`click A href "https://example.com"`, `click A "https://example.com"`, `click A https://example.com`} {
		source := "```mermaid\ngraph TD; A-->B\n" + click + "\n```"
		result, err := Render(source, export.Options{})
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(result.Data), `class="language-mermaid"`) {
			t.Fatalf("%q was not preserved as code", click)
		}
		if len(result.Warnings) == 0 || !strings.Contains(result.Warnings[0], "external resource") {
			t.Fatalf("warnings = %v", result.Warnings)
		}
	}
}

func TestNotesAfterPreNeverLeakAsHTML(t *testing.T) {
	for name, note := range map[string]string{
		"attributes": `<!-- note kind="reader": pre attribute secret -->`,
		"multiline":  "<!-- note:\npre multiline secret\n-->",
		"crlf":       "<!-- note:\r\npre crlf secret\r\n-->",
	} {
		t.Run(name, func(t *testing.T) {
			source := "# Guide\n\n<pre>code</pre>\n" + note + "\n"
			hidden, err := Render(source, export.Options{})
			if err != nil {
				t.Fatal(err)
			}
			if text := string(hidden.Data); strings.Contains(text, "secret") || strings.Contains(text, "<!-- note") {
				t.Fatalf("disabled note leaked: %s", text)
			}
			shown, err := Render(source, export.Options{IncludeNotes: true})
			if err != nil {
				t.Fatal(err)
			}
			text := string(shown.Data)
			if !strings.Contains(text, "secret") || !strings.Contains(text, `class="reader-note"`) || strings.Contains(text, "&lt;!-- note") {
				t.Fatalf("included note was not rendered as a card: %s", text)
			}
		})
	}
}

func TestInlineHTMLNoteIsNeverEscapedIntoExport(t *testing.T) {
	source := "Before <!-- note: inline secret --> after\n"
	hidden, err := Render(source, export.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if text := string(hidden.Data); strings.Contains(text, "secret") || strings.Contains(text, "&lt;!-- note") {
		t.Fatalf("inline note leaked: %s", text)
	}
}

func TestConsecutiveNotesKeepTheirParagraphPosition(t *testing.T) {
	source := "Paragraph\n<!-- note: first consecutive -->\n<!-- note: second consecutive -->\n\nNext paragraph\n"
	result, err := Render(source, export.Options{IncludeNotes: true})
	if err != nil {
		t.Fatal(err)
	}
	text := string(result.Data)
	first := strings.Index(text, "first consecutive")
	second := strings.Index(text, "second consecutive")
	next := strings.Index(text, "Next paragraph")
	if first < 0 || second <= first || next <= second {
		t.Fatalf("note positions first=%d second=%d next=%d: %s", first, second, next, text)
	}
}

func TestFrontmatterDoesNotShiftSectionNotePlacement(t *testing.T) {
	source := "---\ntitle: Example\n---\n\n# Actual\n\nParagraph\n<!-- note: frontmatter section note -->\n\n# Next\n\nFollowing\n"
	result, err := Render(source, export.Options{IncludeNotes: true})
	if err != nil {
		t.Fatal(err)
	}
	text := string(result.Data)
	note := strings.Index(text, "frontmatter section note")
	next := strings.Index(text, `<h1 id="next"`)
	if note < 0 || next < 0 || note > next || !strings.Contains(text, `<h1 id="actual"`) {
		t.Fatalf("frontmatter note positions note=%d next=%d: %s", note, next, text)
	}
}

func TestMermaidSequenceNotesUseThemeColors(t *testing.T) {
	source := "```mermaid\nsequenceDiagram\nA->>B: Request\nNote over B: Remember\n```"
	result, err := Render(source, export.Options{Dark: true, PrintLight: true})
	if err != nil {
		t.Fatal(err)
	}
	text := string(result.Data)
	if !strings.Contains(text, `fill="var(--note-soft)"`) || strings.Contains(text, "#FFF5AD") || !strings.Contains(text, `fill="currentColor"`) {
		t.Fatalf("Mermaid note is not themed: %s", text)
	}
}

func TestSectionOnlyNoteIsRenderedAtSectionEnd(t *testing.T) {
	source := "# One\n\n```go\npackage one\n```\n<!-- note: first section note -->\n\n# Two\n\nText.\n"
	result, err := Render(source, export.Options{IncludeNotes: true})
	if err != nil {
		t.Fatal(err)
	}
	html := string(result.Data)
	note := strings.Index(html, "first section note")
	two := strings.Index(html, `<h1 id="two"`)
	if note < 0 || two < 0 || note > two {
		t.Fatalf("section note is not before the next heading: note=%d heading=%d", note, two)
	}
}

func TestDisabledNotesNeverLeakAdjacentToPreOrInsideRawHTML(t *testing.T) {
	cases := map[string]string{
		"same-line-pre":      "# Guide\n\n<pre>code</pre><!-- note: sameline secret -->\n",
		"same-line-pre-crlf": "# Guide\r\n\r\n<pre>code</pre><!-- note k=\"v\":\r\ncrlf secret\r\n-->\r\n",
		"raw-html-container": "# Guide\n\n<div><!-- note: div secret --></div>\n",
		"unmatched-backtick": "# Guide\n\nA stray ` tick\n\n<div><!-- note: tick secret --></div>\n",
		"backtick-into-html": "# Guide\n\nA stray ` tick\n<div><!-- note: cross secret -->`</div>\n",
	}
	for name, source := range cases {
		t.Run(name, func(t *testing.T) {
			result, err := Render(source, export.Options{})
			if err != nil {
				t.Fatal(err)
			}
			text := string(result.Data)
			if strings.Contains(text, "secret") || strings.Contains(text, "<!-- note") || strings.Contains(text, "&lt;!-- note") {
				t.Fatalf("disabled note leaked: %s", text)
			}
		})
	}
	codeExample := "Keep `<!-- note: literal span -->` and:\n\n    <!-- note: literal indented -->\n \t<!-- note: mixed indented -->\n\n```md\n<!-- note: literal fence -->\n```\n"
	result, err := Render(codeExample, export.Options{})
	if err != nil {
		t.Fatal(err)
	}
	text := string(result.Data)
	if !strings.Contains(text, "literal span") || !strings.Contains(text, "literal indented") || !strings.Contains(text, "mixed indented") || !strings.Contains(text, "literal fence") {
		t.Fatalf("code examples were stripped: %s", text)
	}
}

func TestOfflineImagesModeOmitsRemoteImagesAndTightensCSP(t *testing.T) {
	result, err := Render("![Remote](https://example.com/photo.png)\n", export.Options{OfflineImages: true})
	if err != nil {
		t.Fatal(err)
	}
	text := string(result.Data)
	if strings.Contains(text, "https://example.com/photo.png") || !strings.Contains(text, "image-missing") || !strings.Contains(text, "img-src data:;") {
		t.Fatalf("offline PDF HTML did not block remote images: %s", text)
	}
	if len(result.Warnings) == 0 {
		t.Fatal("expected warning for omitted remote image")
	}
}

func TestDuplicateIdenticalSectionsKeepDistinctSectionEndNotes(t *testing.T) {
	source := "# Repeat\n\n```go\npackage same\n```\n<!-- note: first duplicate note -->\n\n# Repeat\n\n```go\npackage same\n```\n<!-- note: second duplicate note -->\n"
	result, err := Render(source, export.Options{IncludeNotes: true})
	if err != nil {
		t.Fatal(err)
	}
	text := string(result.Data)
	firstHeading := strings.Index(text, `<h1 id="repeat"`)
	secondHeading := strings.Index(text, `<h1 id="repeat-2"`)
	firstNote := strings.Index(text, "first duplicate note")
	secondNote := strings.Index(text, "second duplicate note")
	if firstHeading < 0 || secondHeading < 0 || firstNote < firstHeading || firstNote > secondHeading || secondNote < secondHeading {
		t.Fatalf("duplicate section notes misaligned: h1=%d n1=%d h2=%d n2=%d", firstHeading, firstNote, secondHeading, secondNote)
	}
}
