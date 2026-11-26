package theme

import (
	"fmt"
	"html"
	"regexp"
	"strings"

	"github.com/Bayern99/mark-go-go/internal/export"
	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

var tokenNames = [...]string{
	"text", "keyword", "type", "function", "variable", "property", "constant",
	"number", "string", "escape", "regexp", "comment", "operator", "punctuation",
	"tag", "attribute", "meta", "heading", "emphasis", "strong", "link", "code",
	"inserted", "deleted", "invalid",
}

var customColor = regexp.MustCompile(`^#(?:[0-9a-fA-F]{3}|[0-9a-fA-F]{6}|[0-9a-fA-F]{8})$`)

func Color(c ui.Color) string {
	if c.A == 255 {
		return fmt.Sprintf("#%02x%02x%02x", c.R, c.G, c.B)
	}
	return fmt.Sprintf("rgba(%d, %d, %d, %.3f)", c.R, c.G, c.B, float32(c.A)/255)
}

func NamedColor(name string) string {
	name = strings.TrimSpace(name)
	if customColor.MatchString(name) {
		return name
	}
	names := map[string]string{
		"ink": "--ink", "muted": "--ink-2", "faint": "--ink-3", "info": "--accent",
		"canvas": "--paper", "surface": "--fill", "surface-strong": "--fill-strong",
		"border": "--line", "success": "--success", "warning": "--warn", "danger": "--danger",
		"purple": "--purple", "pink": "--pink", "primary": "--accent", "accent": "--accent",
	}
	for i := 1; i <= 6; i++ {
		names[fmt.Sprintf("chart%d", i)] = fmt.Sprintf("--chart-%d", i)
	}
	if variable, ok := names[name]; ok {
		return "var(" + variable + ")"
	}
	return ""
}

func snapshot(options export.Options, dark bool) portrait.ExportSnapshot {
	space, ok := portrait.SpaceNamed(options.SpaceName)
	if !ok {
		space = portrait.DefaultSpace
	}
	return portrait.Snapshot(space, dark)
}

func variables(s portrait.ExportSnapshot) string {
	c := s.Colors
	values := []struct {
		name  string
		color ui.Color
	}{
		{"paper", c.Paper}, {"code", c.Code}, {"ink", c.Ink}, {"ink-2", c.Ink2}, {"ink-3", c.Ink3}, {"ink-4", c.Ink4},
		{"line", c.Line}, {"line-strong", c.LineStrong}, {"fill", c.Fill}, {"fill-strong", c.FillStrong},
		{"accent", c.Accent}, {"on-accent", c.OnAccent}, {"accent-soft", c.AccentSoft},
		{"success", c.Success}, {"warn", c.Warn}, {"warn-soft", c.WarnSoft}, {"danger", c.Danger}, {"danger-soft", c.DangerSoft},
		{"note", c.Note}, {"note-soft", c.NoteSoft}, {"tip", c.Tip}, {"tip-soft", c.TipSoft},
		{"important", c.Important}, {"important-soft", c.ImportantSoft}, {"caution", c.Caution}, {"caution-soft", c.CautionSoft},
		{"purple", s.Reader.Purple}, {"pink", s.Reader.Pink},
	}
	for i, color := range s.Reader.Charts {
		values = append(values, struct {
			name  string
			color ui.Color
		}{fmt.Sprintf("chart-%d", i+1), color})
	}
	for i, name := range tokenNames {
		if i >= len(s.Code.Tokens) {
			break
		}
		values = append(values, struct {
			name  string
			color ui.Color
		}{"tok-" + name, s.Code.Tokens[i].Color})
	}
	var b strings.Builder
	for _, value := range values {
		fmt.Fprintf(&b, "--%s:%s;", value.name, Color(value.color))
	}
	return b.String()
}

func TokenClass(kind editor.TokenKind) string {
	if int(kind) < len(tokenNames) {
		return "tok-" + tokenNames[kind]
	}
	return "tok-text"
}

func tokenStyles(s portrait.ExportSnapshot) string {
	var b strings.Builder
	for i, name := range tokenNames {
		if i >= len(s.Code.Tokens) {
			break
		}
		style := s.Code.Tokens[i]
		fmt.Fprintf(&b, ".tok-%s{color:var(--tok-%s);", name, name)
		if style.Bold {
			b.WriteString("font-weight:600;")
		}
		if style.Italic {
			b.WriteString("font-style:italic;")
		}
		b.WriteString("}")
	}
	return b.String()
}

func CSS(options export.Options) string {
	current := snapshot(options, options.Dark)
	var b strings.Builder
	b.WriteString(":root{color-scheme:")
	if options.Dark {
		b.WriteString("dark")
	} else {
		b.WriteString("light")
	}
	b.WriteString(";")
	b.WriteString(variables(current))
	b.WriteString("--font-sans:Inter,'IBM Plex Sans SC',-apple-system,BlinkMacSystemFont,'Segoe UI',sans-serif;")
	b.WriteString("--font-display:'Inter Display',Inter,'IBM Plex Sans SC',sans-serif;")
	b.WriteString("--font-mono:'IBM Plex Mono',ui-monospace,SFMono-Regular,Consolas,monospace;")
	b.WriteString("--font-icons:Phosphor;}")
	b.WriteString(baseCSS)
	b.WriteString(tokenStyles(current))
	b.WriteString("@media print{")
	if options.PrintLight {
		b.WriteString(":root{color-scheme:light;")
		b.WriteString(variables(snapshot(options, false)))
		b.WriteString("}")
	}
	b.WriteString(printCSS)
	b.WriteString("}")
	return b.String()
}

func Document(options export.Options, body, additionalCSS string) []byte {
	title := strings.TrimSpace(options.Title)
	if title == "" {
		title = "Mark Go Go Export"
	}
	var b strings.Builder
	b.WriteString("<!doctype html>\n<html lang=\"en\"><head><meta charset=\"utf-8\">")
	b.WriteString("<meta name=\"viewport\" content=\"width=device-width,initial-scale=1\">")
	imgSrc := "data: http: https:"
	if options.OfflineImages {
		imgSrc = "data:"
	}
	b.WriteString("<meta http-equiv=\"Content-Security-Policy\" content=\"default-src 'none'; img-src " + imgSrc + "; style-src 'unsafe-inline'; font-src data:; base-uri 'none'; form-action 'none'\">")
	b.WriteString("<title>" + html.EscapeString(title) + "</title><style>")
	b.WriteString(CSS(options))
	b.WriteString(additionalCSS)
	b.WriteString("</style></head><body><main class=\"export-document\">")
	b.WriteString(body)
	b.WriteString("</main></body></html>\n")
	return []byte(b.String())
}

const baseCSS = `
*{box-sizing:border-box}html{background:var(--paper)}body{margin:0;background:var(--paper);color:var(--ink);font-family:var(--font-sans);font-size:16px;line-height:1.75;-webkit-print-color-adjust:exact;print-color-adjust:exact}.export-document{width:min(100% - 48px,840px);margin:0 auto;padding:48px 0 72px}h1,h2,h3,h4,h5,h6{font-family:var(--font-display);line-height:1.25;letter-spacing:-.015em;margin:1.65em 0 .65em;break-after:avoid}h1{font-size:2rem}h2{font-size:1.5rem}h3{font-size:1.2rem}h4{font-size:1.05rem}h5{font-size:.95rem}h6{font-size:.875rem}p{margin:.75em 0}a{color:var(--accent);text-decoration-thickness:.08em;text-underline-offset:.16em}img,svg{max-width:100%;height:auto}img{border-radius:8px}hr{border:0;border-top:1px solid var(--line);margin:2em 0}blockquote,.markdown-alert,.reader-note{margin:1em 0;padding:12px 16px;border-left:3px solid var(--accent);border-radius:0 12px 12px 0;background:var(--fill);break-inside:avoid}blockquote>:first-child,.reader-note>:first-child{margin-top:0}blockquote>:last-child,.reader-note>:last-child{margin-bottom:0}.markdown-alert-title,.reader-note-title{font-size:.8rem;font-weight:600;letter-spacing:.04em;text-transform:uppercase;color:var(--accent);margin:0 0 4px}.markdown-alert-tip{border-color:var(--tip);background:var(--tip-soft)}.markdown-alert-tip .markdown-alert-title{color:var(--tip)}.markdown-alert-important{border-color:var(--important);background:var(--important-soft)}.markdown-alert-important .markdown-alert-title{color:var(--important)}.markdown-alert-warning{border-color:var(--warn);background:var(--warn-soft)}.markdown-alert-warning .markdown-alert-title{color:var(--warn)}.markdown-alert-caution{border-color:var(--caution);background:var(--caution-soft)}.markdown-alert-caution .markdown-alert-title{color:var(--caution)}.reader-note{border-color:var(--note);background:var(--note-soft);color:var(--ink-2);font-size:.9rem}pre,code{font-family:var(--font-mono)}code{font-size:.9em;background:var(--fill);border-radius:4px;padding:.12em .32em}pre{margin:1em 0;padding:16px;overflow:auto;background:var(--code);border:1px solid var(--line);border-radius:12px;line-height:1.65;break-inside:avoid}pre code{padding:0;background:none;font-size:.85rem}.code-language{display:block;color:var(--ink-3);font:600 .72rem var(--font-sans);letter-spacing:.04em;text-transform:uppercase;margin-bottom:8px}.math{display:inline-block;vertical-align:middle}.math-display{text-align:center;margin:1em 0;overflow:auto;break-inside:avoid}.mermaid{margin:1em 0;text-align:center;break-inside:avoid}.mermaid svg{max-width:100%}.table-wrap{overflow-x:auto;margin:1em 0;break-inside:avoid}table{width:100%;border-collapse:collapse;font-size:.92rem}th,td{padding:9px 12px;border:1px solid var(--line);text-align:left;vertical-align:top}th{background:var(--fill);font-weight:600}td[align=right],th[align=right]{text-align:right}td[align=center],th[align=center]{text-align:center}ul,ol{padding-left:1.7em}li.task{list-style:none;margin-left:-1.35em}li.task input{margin-right:.55em;accent-color:var(--accent)}.footnotes{margin-top:2.5em;border-top:1px solid var(--line);font-size:.9rem;color:var(--ink-2)}.footnote-ref{font-size:.75em;text-decoration:none}.image-missing{display:flex;align-items:center;justify-content:center;min-height:88px;margin:1em 0;padding:16px;border:1px dashed var(--line-strong);border-radius:12px;background:var(--fill);color:var(--ink-3);font-size:.85rem}.g-col{display:flex;flex-direction:column;min-width:0}.g-row{display:flex;flex-direction:row;align-items:center;min-width:0}.g-wrap{flex-wrap:wrap}.g-grid{display:grid;grid-template-columns:repeat(var(--columns,2),minmax(0,1fr))}.g-card{padding:16px;border:1px solid var(--line);border-radius:12px;background:var(--paper);box-shadow:0 1px 2px var(--line);break-inside:avoid}.g-soft{background:var(--code)}.g-title{margin:0;font-weight:600}.g-text{margin:0}.g-caption{color:var(--ink-2);font-size:.78rem}.g-label{font-size:.84rem;font-weight:500}.g-badge{display:inline-flex;align-items:center;gap:6px;align-self:flex-start;padding:3px 10px;border-radius:999px;background:var(--fill);font-size:.82rem;font-weight:500}.g-link{font-size:inherit}.g-list{display:flex;flex-direction:column;gap:10px;padding:0;list-style:none}.g-list-number{counter-reset:g-list}.g-list-number>.g-list-item{counter-increment:g-list}.g-list-number>.g-list-item:before{content:counter(g-list);display:inline-grid;place-items:center;width:24px;height:24px;margin-right:10px;border-radius:50%;background:var(--ink);color:var(--paper);font-size:.78rem}.g-list-item{display:flex;align-items:flex-start}.g-list-body{flex:1;min-width:0}.g-list-title{font-weight:500}.g-list-description{color:var(--ink-2);font-size:.9rem}.g-quote{display:flex;margin:1em 0;background:var(--fill);border-radius:0 12px 12px 0;overflow:hidden;break-inside:avoid}.g-quote:before{content:"";width:3px;background:var(--ink);flex:none}.g-quote-body{padding:14px 16px}.g-quote-cite{color:var(--ink-2);font-size:.85rem}.g-source{position:relative}.g-sources{display:flex;flex-wrap:wrap;gap:6px;margin-top:10px}.g-source-link{display:inline-block;padding:2px 8px;border:1px solid var(--line);border-radius:999px;color:var(--ink-2);font-size:.72rem;text-decoration:none}.g-chart{margin:12px 0;break-inside:avoid}.g-chart-legend{display:flex;justify-content:center;flex-wrap:wrap;gap:8px 16px;color:var(--ink-2);font-size:.75rem}.g-legend-swatch{display:inline-block;width:8px;height:8px;border-radius:2px;margin-right:6px}.g-icon{font-family:var(--font-icons);font-style:normal;line-height:1}.g-control{display:flex;flex-direction:column;gap:6px;margin:8px 0;break-inside:avoid}.g-field{padding:9px 12px;border:1px solid var(--line-strong);border-radius:8px;background:var(--paper);color:var(--ink);min-height:36px}.g-readonly{cursor:default}.g-button{display:inline-flex;align-items:center;justify-content:center;gap:7px;align-self:flex-start;padding:7px 14px;border:1px solid var(--accent);border-radius:8px;background:var(--accent);color:var(--on-accent);font-weight:500}.g-button-outline,.g-button-soft,.g-button-ghost{background:var(--paper);color:var(--accent)}.g-button-danger{background:var(--danger);border-color:var(--danger)}.g-button-success{background:var(--success);border-color:var(--success)}.g-disabled{opacity:.5}.g-block{width:100%}.g-choice{display:flex;align-items:flex-start;gap:8px}.g-choice-marker{color:var(--accent);font-weight:600}.g-segments{display:flex;align-self:flex-start;padding:3px;border-radius:10px;background:var(--fill-strong)}.g-segment{padding:4px 10px;border-radius:8px}.g-selected{background:var(--paper);box-shadow:0 0 0 1px var(--line);color:var(--accent)}.g-slider-track{height:5px;border-radius:999px;background:var(--fill-strong);position:relative;margin:10px 5px}.g-slider-fill{height:100%;border-radius:999px;background:var(--accent)}.g-slider-thumb{position:absolute;top:50%;width:15px;height:15px;border-radius:50%;background:var(--paper);border:2px solid var(--accent);transform:translate(-50%,-50%)}.g-flashcard{display:grid;grid-template-columns:repeat(auto-fit,minmax(220px,1fr));gap:12px}.g-flash-side{padding:20px;border:1px solid var(--line);border-radius:12px;background:var(--code);break-inside:avoid}.g-flash-label{font-size:.72rem;text-transform:uppercase;letter-spacing:.06em;color:var(--ink-3);margin-bottom:8px}.g-popover-note{margin:8px 0 0 12px;padding:12px 14px;border-left:2px solid var(--line-strong);background:var(--fill);border-radius:0 8px 8px 0}.g-carousel{display:grid;grid-template-columns:repeat(auto-fit,minmax(220px,1fr));gap:12px}.g-notes{margin-top:48px;padding-top:24px;border-top:1px solid var(--line)}.g-note-group{margin:20px 0;break-inside:avoid}.g-note-back{font-size:.8rem}.g-svg{color:var(--ink);max-width:100%;height:auto}.g-spacer{flex:1}.g-pressable[aria-pressed=true]{outline:2px solid var(--accent)}.g-timer{font-size:2rem;font-weight:600;font-variant-numeric:tabular-nums}.g-hidden{display:none}
`

const printCSS = `
@page{margin:16mm}html,body{background:var(--paper)!important}.export-document{width:100%;max-width:none;padding:0}h1,h2,h3,h4,h5,h6{break-after:avoid}p{orphans:3;widows:3}pre,.g-card,.g-chart,.g-flash-side,.table-wrap,blockquote,.markdown-alert,.reader-note,tr,.mermaid,.math-display{break-inside:avoid}pre{white-space:pre-wrap;overflow-wrap:anywhere;border:1px solid var(--line)}a{color:inherit;text-decoration:none}.g-source-link:after{content:""}img,svg{max-width:100%!important}.g-carousel{display:block}.g-carousel>*{margin-bottom:12px}
`

var _ [len(tokenNames) - int(editor.TokenInvalid) - 1]struct{}
var _ [int(editor.TokenInvalid) + 1 - len(tokenNames)]struct{}
