package markdown

import (
	"html"
	"strings"

	"github.com/Bayern99/mark-go-go/internal/export/theme"
	"github.com/egoist/mygo/ui/editor"
)

func CodeHTML(source, language string) string {
	var b strings.Builder
	lang := editor.LanguageNamed(language)
	lines := strings.Split(strings.TrimSuffix(source, "\n"), "\n")
	if lang == nil || lang.Tokenizer == nil {
		for i, line := range lines {
			if i > 0 {
				b.WriteByte('\n')
			}
			b.WriteString(`<span class="tok-text">` + html.EscapeString(line) + `</span>`)
		}
		return b.String()
	}
	var state uint64
	for i, line := range lines {
		if i > 0 {
			b.WriteByte('\n')
		}
		tokens, next := lang.Tokenizer.Tokenize(line, state, nil)
		state = next
		at := 0
		for _, token := range tokens {
			end := max(at, min(token.End, len(line)))
			if end == at {
				continue
			}
			b.WriteString(`<span class="` + theme.TokenClass(token.Kind) + `">`)
			b.WriteString(html.EscapeString(line[at:end]))
			b.WriteString(`</span>`)
			at = end
		}
		if at < len(line) {
			b.WriteString(`<span class="tok-text">` + html.EscapeString(line[at:]) + `</span>`)
		}
	}
	return b.String()
}

func CodeBlockHTML(source, language string) string {
	var b strings.Builder
	b.WriteString(`<pre><code`)
	if language != "" {
		b.WriteString(` class="language-` + html.EscapeString(language) + `"`)
	}
	b.WriteString(">")
	if language != "" {
		b.WriteString(`<span class="code-language">` + html.EscapeString(language) + `</span>`)
	}
	b.WriteString(CodeHTML(source, language))
	b.WriteString("</code></pre>\n")
	return b.String()
}
