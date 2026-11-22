package nativegenui

import (
	"testing"

	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/egoist/mygo/ui/markdown"
)

func TestMarkdownCodeUsesPortraitMonoAtTwelvePoints(t *testing.T) {
	for _, o := range []struct {
		name string
		opts *markdown.Options
	}{
		{"Text", &readOnlyMarkdown},
		{"caption Text", &captionMarkdown},
	} {
		th := o.opts.Theme
		if th == nil {
			t.Errorf("%s markdown has no theme, want CodeFont %q at 12pt or larger", o.name, portrait.FontMono)
			continue
		}
		if th.CodeFont != portrait.FontMono {
			t.Errorf("%s markdown CodeFont = %q, want %q", o.name, th.CodeFont, portrait.FontMono)
		}
		size := th.FontSize
		if size == 0 {
			size = 14
		}
		if got := th.CodeSize * size; got < portrait.Type.Meta.Size {
			t.Errorf("%s inline code renders at %gpt, want at least %g", o.name, got, portrait.Type.Meta.Size)
		}
	}
}
