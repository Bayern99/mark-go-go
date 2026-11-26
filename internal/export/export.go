package export

import (
	"errors"
	"runtime"
	"slices"
	"sync"
)

type Scope string

const (
	ScopeMarkdown Scope = "markdown"
	ScopeGenUI    Scope = "genui"
)

type Engine string

const (
	EngineBuiltin Engine = "builtin"
	EnginePandoc  Engine = "pandoc"
)

type Format struct {
	ID                string
	Label             string
	Ext               string
	Scope             Scope
	Engine            Engine
	Available         bool
	UnavailableReason string
}

type Options struct {
	SpaceName     string
	Dark          bool
	PrintLight    bool
	IncludeNotes  bool
	OfflineImages bool
	BaseDir       string
	AssetDir      string
	Title         string
}

type Attachment struct {
	Name string
	Data []byte
}

type Result struct {
	Data        []byte
	Warnings    []string
	Attachments []Attachment
}

var ErrUnknownFormat = errors.New("unknown export format")

type Registry struct {
	mu      sync.RWMutex
	formats map[string]Format
}

func NewRegistry() *Registry {
	r := &Registry{formats: map[string]Format{}}
	r.Register(Format{ID: "markdown-html", Label: "HTML", Ext: ".html", Scope: ScopeMarkdown, Engine: EngineBuiltin, Available: true})
	r.Register(Format{ID: "genui-html", Label: "HTML", Ext: ".html", Scope: ScopeGenUI, Engine: EngineBuiltin, Available: true})
	for _, id := range []string{"markdown-pdf", "genui-pdf"} {
		scope := ScopeMarkdown
		if id == "genui-pdf" {
			scope = ScopeGenUI
		}
		r.Register(Format{ID: id, Label: "PDF", Ext: ".pdf", Scope: scope, Engine: EngineBuiltin, Available: runtime.GOOS == "darwin", UnavailableReason: "Built-in PDF export is currently available on macOS."})
	}
	for _, format := range []Format{
		{ID: "markdown-docx", Label: "Word (.docx)", Ext: ".docx"},
		{ID: "markdown-pptx", Label: "PowerPoint (.pptx)", Ext: ".pptx"},
		{ID: "markdown-epub", Label: "EPUB", Ext: ".epub"},
		{ID: "markdown-rtf", Label: "RTF", Ext: ".rtf"},
		{ID: "markdown-latex", Label: "LaTeX", Ext: ".tex"},
	} {
		format.Scope = ScopeMarkdown
		format.Engine = EnginePandoc
		format.Available = true
		r.Register(format)
	}
	return r
}

func (r *Registry) Register(format Format) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.formats == nil {
		r.formats = map[string]Format{}
	}
	r.formats[format.ID] = format
}

func (r *Registry) Lookup(id string) (Format, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	format, ok := r.formats[id]
	if !ok {
		return Format{}, ErrUnknownFormat
	}
	return format, nil
}

func (r *Registry) Formats(scope Scope) []Format {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var formats []Format
	for _, format := range r.formats {
		if format.Scope == scope {
			formats = append(formats, format)
		}
	}
	slices.SortFunc(formats, func(a, b Format) int {
		if a.ID < b.ID {
			return -1
		}
		if a.ID > b.ID {
			return 1
		}
		return 0
	})
	return formats
}

var DefaultRegistry = NewRegistry()
