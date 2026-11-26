package pandoc

import (
	"encoding/base64"
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/Bayern99/mark-go-go/internal/export"
	"github.com/Bayern99/mark-go-go/internal/export/assets"
	markdownexport "github.com/Bayern99/mark-go-go/internal/export/markdown"
	md "github.com/Bayern99/mark-go-go/internal/markdown"
	"github.com/Bayern99/mark-go-go/internal/markdown/svgfigure"
	"github.com/egoist/mygo/ui/markdown/parser"
)

type spanEdit struct {
	start       int
	end         int
	replacement string
}

type latexCollector struct {
	assetDir    string
	attachments []export.Attachment
}

func (c *latexCollector) add(ext string, data []byte) string {
	name := path.Join(c.assetDir, fmt.Sprintf("image-%d%s", len(c.attachments)+1, ext))
	c.attachments = append(c.attachments, export.Attachment{Name: name, Data: data})
	return name
}

func Preprocess(source, formatID string, options export.Options) (string, []string, error) {
	processed, _, warnings, err := PreprocessWithAttachments(source, formatID, options)
	return processed, warnings, err
}

func PreprocessWithAttachments(source, formatID string, options export.Options) (string, []export.Attachment, []string, error) {
	source = preprocessNotes(source, options.IncludeNotes)
	resolver := assets.NewWithOptions(options.BaseDir, true)
	defer resolver.Close()
	doc := parser.New(parser.Default | parser.ExtDollarMath)
	doc.Set(source)

	var collector *latexCollector
	if formatID == "markdown-latex" {
		assetDir := strings.TrimSpace(options.AssetDir)
		if assetDir == "" {
			assetDir = "media"
		}
		collector = &latexCollector{assetDir: assetDir}
	}

	var edits []spanEdit
	var extraWarnings []string
	diagram := 0
	var walk func(parent int)
	walk = func(parent int) {
		for i := range doc.Children(parent) {
			block := doc.Block(i)
			if block.Kind == parser.CodeBlock {
				if parent == 0 && block.Fenced && strings.EqualFold(block.Language(), "mermaid") {
					diagram++
					body := blockBody(doc, i)
					replacement, err := mermaidImageForTarget(body, formatID, options, collector)
					if err != nil {
						extraWarnings = append(extraWarnings, fmt.Sprintf("Mermaid diagram %d (%s): %v", diagram, diagramSummary(body), err))
					} else {
						edits = append(edits, spanEdit{start: block.Start, end: block.End, replacement: "\n" + replacement + "\n\n"})
					}
				}
				continue
			}
			if block.Kind != parser.HTMLBlock {
				inlines := doc.Inlines(i)
				for idx := 0; idx < len(inlines); idx++ {
					inline := inlines[idx]
					if inline.Kind != parser.Image {
						continue
					}
					if inline.Start < 0 || inline.End <= inline.Start || inline.End > len(source) {
						continue
					}
					alt := inlineAltText(inlines[idx+1 : inline.Next])
					replacement, warn := rewritePandocImage(alt, inline.Dest, inline.Title, formatID, resolver, collector)
					if warn != "" {
						extraWarnings = append(extraWarnings, warn)
					}
					edits = append(edits, spanEdit{start: inline.Start, end: inline.End, replacement: replacement})
					idx = inline.Next - 1
				}
			}
			walk(i)
		}
	}
	walk(0)

	sort.Slice(edits, func(i, j int) bool {
		return edits[i].start > edits[j].start
	})
	lastStart := len(source) + 1
	for _, edit := range edits {
		if edit.end > lastStart || edit.start < 0 || edit.end > len(source) {
			continue
		}
		source = source[:edit.start] + edit.replacement + source[edit.end:]
		lastStart = edit.start
	}
	warnings := append(resolver.Warnings(), extraWarnings...)
	var attachments []export.Attachment
	if collector != nil {
		attachments = collector.attachments
	}
	return source, attachments, warnings, nil
}

func mermaidImageForTarget(body, formatID string, options export.Options, collector *latexCollector) (string, error) {
	if collector == nil {
		return mermaidImage(body, formatID, options)
	}
	svg, err := renderMermaidPortraitSVG(body, options)
	if err != nil {
		return "", err
	}
	pngBytes, width, height, err := rasterizeSVGTo2xPNG(svg)
	if err != nil {
		return "", err
	}
	name := collector.add(".png", pngBytes)
	return fmt.Sprintf("![Mermaid diagram](%s){width=%dpx height=%dpx}", name, width, height), nil
}

func blockBody(doc *parser.Document, blockIdx int) string {
	var b strings.Builder
	count := doc.LineCount(blockIdx)
	for k := 0; k < count; k++ {
		line, _ := doc.Line(blockIdx, k)
		b.WriteString(line)
	}
	return b.String()
}

func inlineAltText(inlines []parser.Inline) string {
	var b strings.Builder
	for _, inline := range inlines {
		switch inline.Kind {
		case parser.Text, parser.Code, parser.Math, parser.Mention, parser.Reference:
			b.WriteString(inline.Text)
		case parser.SoftBreak, parser.LineBreak:
			b.WriteByte(' ')
		}
	}
	return b.String()
}

func rewritePandocImage(alt, dest, title, formatID string, resolver *assets.Resolver, collector *latexCollector) (string, string) {
	resolution := resolver.Image(dest)
	if resolution.Missing {
		name := resolution.Name
		if name == "" || name == "." {
			name = alt
		}
		if name == "" {
			name = "image"
		}
		return fmt.Sprintf("*[Image unavailable: %s]*", escapePlaceholder(name)), ""
	}
	escapedAlt := escapeAlt(alt)
	titlePart := ""
	if title != "" {
		titlePart = ` "` + strings.ReplaceAll(strings.ReplaceAll(title, `\`, `\\`), `"`, `\"`) + `"`
	}
	if strings.HasPrefix(resolution.URL, "data:image/svg+xml;base64,") && formatID != "markdown-epub" {
		raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(resolution.URL, "data:image/svg+xml;base64,"))
		if err == nil {
			raw, err = svgfigure.SanitizeSVG(raw)
		}
		if err == nil {
			pngBytes, width, height, rasterErr := rasterizeSVGTo2xPNG(raw)
			if rasterErr == nil {
				if collector != nil {
					name := collector.add(".png", pngBytes)
					return fmt.Sprintf("![%s](%s%s){width=%dpx height=%dpx}", escapedAlt, name, titlePart, width, height), ""
				}
				return fmt.Sprintf("![%s](data:image/png;base64,%s%s){width=%dpx height=%dpx}", escapedAlt, base64.StdEncoding.EncodeToString(pngBytes), titlePart, width, height), ""
			}
			err = rasterErr
		}
		name := dest
		if name == "" {
			name = "image.svg"
		}
		return fmt.Sprintf("*[Image unavailable: %s]*", escapePlaceholder(name)), fmt.Sprintf("SVG image %q could not be rasterized: %v", dest, err)
	}
	if collector != nil {
		if header, encoded, ok := strings.Cut(resolution.URL, ";base64,"); ok {
			ext := ""
			switch strings.ToLower(strings.TrimPrefix(header, "data:")) {
			case "image/png":
				ext = ".png"
			case "image/jpeg", "image/jpg":
				ext = ".jpg"
			}
			if ext != "" {
				if data, err := base64.StdEncoding.DecodeString(encoded); err == nil {
					name := collector.add(ext, data)
					return fmt.Sprintf("![%s](%s%s)", escapedAlt, name, titlePart), ""
				}
			}
		}
	}
	return fmt.Sprintf("![%s](%s%s)", escapedAlt, resolution.URL, titlePart), ""
}

func escapeAlt(value string) string {
	return strings.NewReplacer(`\`, `\\`, `[`, `\[`, `]`, `\]`).Replace(value)
}

func preprocessNotes(source string, include bool) string {
	notes := md.ExtractNotes(source)
	if !include {
		return markdownexport.WithoutNotes(source, notes, true)
	}
	doc := parser.New(parser.Default | parser.ExtDollarMath)
	doc.Set(source)
	var htmlSpans []struct{ start, end int }
	var walk func(parent int)
	walk = func(parent int) {
		for i := range doc.Children(parent) {
			block := doc.Block(i)
			if block.Kind == parser.HTMLBlock {
				htmlSpans = append(htmlSpans, struct{ start, end int }{block.Start, block.End})
				continue
			}
			walk(i)
		}
	}
	walk(0)
	for i := len(notes) - 1; i >= 0; i-- {
		start, end := notes[i].ByteRange()
		if start < 0 || end < start || end > len(source) || insideHTMLBlock(htmlSpans, start, end) {
			continue
		}
		replacement := "\n\n::: {.reader-note custom-style=\"Reader Note\"}\n" + escapeFencedDivBody(notes[i].Text) + "\n:::\n\n"
		source = source[:start] + replacement + source[end:]
	}
	return markdownexport.StripResidualNotes(source)
}

func insideHTMLBlock(spans []struct{ start, end int }, start, end int) bool {
	for _, span := range spans {
		if start >= span.start && end <= span.end && (start > span.start || end < span.end) {
			return true
		}
	}
	return false
}

func escapeFencedDivBody(text string) string {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	for i, line := range lines {
		trimmed := strings.TrimLeft(line, " \t")
		if strings.HasPrefix(trimmed, ":::") {
			prefixLen := len(line) - len(trimmed)
			lines[i] = line[:prefixLen] + `\` + trimmed
		}
	}
	return strings.Join(lines, "\n")
}

func escapePlaceholder(value string) string {
	value = strings.Join(strings.Fields(value), " ")
	replacer := strings.NewReplacer("\\", "\\\\", "*", "\\*", "[", "\\[", "]", "\\]")
	return replacer.Replace(value)
}
