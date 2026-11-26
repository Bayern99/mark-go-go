package nativegenui

import (
	"context"
	"errors"
	"path/filepath"
	"strings"

	"github.com/Bayern99/mark-go-go/internal/export"
	genuiexport "github.com/Bayern99/mark-go-go/internal/export/genui"
	"github.com/Bayern99/mark-go-go/internal/nativeui"
	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/Bayern99/mark-go-go/internal/workspace"
	"github.com/egoist/mygo"
)

var genUINotify = func(win *nativeui.Window, spec portrait.ToastSpec) { win.Notify(spec) }
var genUIChoosePath = func(win *nativeui.Window, defaultPath, ext string) (string, error) {
	return win.ChooseExportPath(defaultPath, ext)
}
var genUISave = func(win *nativeui.Window, path string, result export.Result) { win.WriteExport(path, result) }
var genUISavePDF = func(win *nativeui.Window, path string, result export.Result) { win.WriteExport(path, result) }

func (p *panel) exportPDF(v *documentView, d workspace.DocumentInfo) {
	reader := v.reader()
	surface := reader.surface
	if surface == nil || surface.Session == nil {
		genUINotify(p.win, portrait.ToastSpec{ID: "genui-export", Tone: portrait.Tones.Warning, Title: "Export currently supports GenUI v3 only. Regenerate the view to export."})
		return
	}
	format, err := export.DefaultRegistry.Lookup("genui-pdf")
	if err != nil || !format.Available {
		title := "Export format is unavailable"
		if err == nil {
			title = format.UnavailableReason
		}
		genUINotify(p.win, portrait.ToastSpec{ID: "genui-export", Tone: portrait.Tones.Warning, Title: title})
		return
	}
	root, problems := reader.exportSnapshot()
	sources := exportSectionSources(surface, v.sections)
	options, pdfOpts := p.win.ExportPDFOptions(d.Name)
	path := d.Path
	if path != "" {
		path = strings.TrimSuffix(path, filepath.Ext(path)) + ".pdf"
	} else {
		path = strings.TrimSuffix(d.Name, filepath.Ext(d.Name)) + ".pdf"
	}
	notes := append([]workspace.Note(nil), v.notes...)
	ctx := p.win.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	go func() {
		target, err := genUIChoosePath(p.win, path, ".pdf")
		if err != nil || target == "" {
			if err != nil && !errors.Is(err, mygo.ErrCanceled) {
				p.win.NotifyAsync(portrait.ToastSpec{ID: "genui-export", Tone: portrait.Tones.Danger, Title: "Export failed: " + err.Error()})
			}
			return
		}
		if ctx.Err() != nil {
			return
		}
		result, err := genuiexport.RenderSnapshot(root, problems, sources, notes, options)
		if err != nil {
			p.win.NotifyAsync(portrait.ToastSpec{ID: "genui-export", Tone: portrait.Tones.Danger, Title: "Export failed: " + err.Error()})
			return
		}
		result.Data, err = nativeui.PrintPDF(ctx, result.Data, pdfOpts)
		if err != nil {
			if !errors.Is(err, context.Canceled) {
				p.win.NotifyAsync(portrait.ToastSpec{ID: "genui-export", Tone: portrait.Tones.Danger, Title: "Export failed: " + err.Error()})
			}
			return
		}
		if ctx.Err() != nil {
			return
		}
		genUISavePDF(p.win, target, result)
	}()
}

func (p *panel) exportHTML(v *documentView, d workspace.DocumentInfo) {
	surface := v.reading.surface
	if surface == nil || surface.Session == nil {
		genUINotify(p.win, portrait.ToastSpec{ID: "genui-export", Tone: portrait.Tones.Warning, Title: "Export currently supports GenUI v3 only. Regenerate the view to export."})
		return
	}
	root, problems := v.reading.exportSnapshot()
	sources := exportSectionSources(surface, v.sections)
	options := p.win.ExportOptions(d.Name)
	path := d.Path
	if path != "" {
		path = strings.TrimSuffix(path, filepath.Ext(path)) + ".html"
	} else {
		path = strings.TrimSuffix(d.Name, filepath.Ext(d.Name)) + ".html"
	}
	notes := append([]workspace.Note(nil), v.notes...)
	ctx := p.win.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	go func() {
		target, err := genUIChoosePath(p.win, path, ".html")
		if err != nil || target == "" {
			if err != nil && !errors.Is(err, mygo.ErrCanceled) {
				p.win.NotifyAsync(portrait.ToastSpec{ID: "genui-export", Tone: portrait.Tones.Danger, Title: "Export failed: " + err.Error()})
			}
			return
		}
		if ctx.Err() != nil {
			return
		}
		result, err := genuiexport.RenderSnapshot(root, problems, sources, notes, options)
		if err != nil {
			p.win.NotifyAsync(portrait.ToastSpec{ID: "genui-export", Tone: portrait.Tones.Danger, Title: "Export failed: " + err.Error()})
			return
		}
		if ctx.Err() != nil {
			return
		}
		genUISave(p.win, target, result)
	}()
}
