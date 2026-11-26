package export

import (
	"runtime"
	"testing"
)

func TestDefaultRegistry(t *testing.T) {
	registry := NewRegistry()
	if formats := registry.Formats(ScopeMarkdown); len(formats) != 7 {
		t.Fatalf("Markdown formats = %+v", formats)
	}
	if formats := registry.Formats(ScopeGenUI); len(formats) != 2 {
		t.Fatalf("GenUI formats = %+v", formats)
	}
	for _, scope := range []Scope{ScopeMarkdown, ScopeGenUI} {
		prefix := string(scope) + "-"
		html, err := registry.Lookup(prefix + "html")
		if err != nil || html.Label != "HTML" || html.Ext != ".html" || html.Engine != EngineBuiltin || !html.Available {
			t.Fatalf("HTML format for %s = %+v, %v", scope, html, err)
		}
		pdf, err := registry.Lookup(prefix + "pdf")
		if err != nil || pdf.Label != "PDF" || pdf.Ext != ".pdf" || pdf.Engine != EngineBuiltin || pdf.Available != (runtime.GOOS == "darwin") || pdf.UnavailableReason != "Built-in PDF export is currently available on macOS." {
			t.Fatalf("PDF format for %s = %+v, %v", scope, pdf, err)
		}
	}
	for _, tc := range []struct{ id, label, ext string }{
		{"markdown-docx", "Word (.docx)", ".docx"},
		{"markdown-pptx", "PowerPoint (.pptx)", ".pptx"},
		{"markdown-epub", "EPUB", ".epub"},
		{"markdown-rtf", "RTF", ".rtf"},
		{"markdown-latex", "LaTeX", ".tex"},
	} {
		format, err := registry.Lookup(tc.id)
		if err != nil || format.Label != tc.label || format.Ext != tc.ext || format.Scope != ScopeMarkdown || format.Engine != EnginePandoc || !format.Available || format.UnavailableReason != "" {
			t.Errorf("Pandoc format %s = %+v, %v", tc.id, format, err)
		}
	}
	if _, err := registry.Lookup("missing"); err != ErrUnknownFormat {
		t.Fatalf("unknown format error = %v", err)
	}
}
