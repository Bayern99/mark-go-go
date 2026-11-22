package portrait_test

import (
	"testing"

	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/egoist/mygo/ui"
)

func TestThemeBridgesTheWebUIMyGoVariables(t *testing.T) {
	th := portrait.Theme()
	for _, tc := range []struct {
		name      string
		got, want ui.Color
	}{
		{"background", th.Background, portrait.Colors.Paper},
		{"surface", th.Surface, portrait.Colors.Paper},
		{"surface hover", th.SurfaceHover, portrait.Colors.Code},
		{"surface pressed", th.SurfacePressed, portrait.Colors.Chrome},
		{"text", th.Text, portrait.Colors.Ink},
		{"muted", th.TextMuted, portrait.Colors.Ink3},
		{"border", th.Border, portrait.Colors.LineStrong},
		{"accent", th.Accent, portrait.Colors.Accent},
		{"accent text", th.AccentText, portrait.Colors.OnAccent},
		{"accent hover", th.AccentHover, portrait.Materials.AccentRaisedHover.From},
		{"accent pressed", th.AccentPressed, portrait.Materials.AccentRaisedPress.From},
		{"danger", th.Danger, portrait.Colors.Danger},
		{"warning", th.Warning, portrait.Colors.Warn},
		{"focus", th.Focus, portrait.Colors.Accent},
	} {
		if tc.got != tc.want {
			t.Errorf("%s = %+v, want %+v", tc.name, tc.got, tc.want)
		}
	}
	if th.Font != portrait.FontSans || th.FontSize != portrait.Type.UI.Size {
		t.Errorf("theme font %q %v, want %q %v", th.Font, th.FontSize, portrait.FontSans, portrait.Type.UI.Size)
	}
}

func TestWorkspaceNameStyleMatchesTheWebUI(t *testing.T) {
	s := portrait.Shell.WorkspaceName
	if s.Size != 14 || s.Leading != 18 || !near(s.Tracking, -0.07) || s.Family != portrait.FontSans {
		t.Errorf("workspace name %+v", s)
	}
}

func TestClearIsTransparentButNotTheZeroColor(t *testing.T) {
	if portrait.Clear == (ui.Color{}) || portrait.Clear.A != 0 {
		t.Errorf("clear %+v", portrait.Clear)
	}
}
