package nativeeditor

import (
	"fmt"

	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

func renderFindBar(c *ui.Context, bar ui.Element, find *editor.FindBar) {
	bar.Padding(portrait.Form.PopoverPad).Gap(portrait.Form.Gap).Radius(portrait.Radii.L).Clip()
	portrait.Materials.Pop.Apply(bar)
	bar.Children(func() {
		ui.Column(c).Gap(portrait.Form.Gap).Children(func() {
			ui.Row(c).Gap(portrait.Form.Gap).AlignItems(ui.Center).Children(func() {
				field := portrait.TextField(c, find.Query(), portrait.FieldSpec{Label: "Find", Placeholder: "Find", Leading: portrait.IconMagnifyingGlass, Invalid: find.Err() != nil, Size: portrait.Form.FieldCompact})
				field.Frame().MinWidth(portrait.Form.MenuMin).MaxWidth(portrait.Form.SelectMax).Grow(1)
				find.QueryInput(field.Input())
				portrait.Type.Meta.Apply(ui.Text(c, find.Status())).TextColor(portrait.Colors.Ink3).FontFeatures("tnum").Role(ui.RoleStatus).MaxLines(1)
				if portrait.QuietIconButton(c, portrait.IconCaretUp, "Previous match").KeepFocus().Clicked() {
					find.Previous()
				}
				if portrait.QuietIconButton(c, portrait.IconCaretDown, "Next match").KeepFocus().Clicked() {
					find.Next()
				}
				if find.CanReplace() {
					replace := portrait.ButtonWith(c, "Replace", "Show replacement", portrait.ButtonSpec{Look: portrait.Quiet, Size: portrait.Sizes.Compact, Selected: find.Replacing()})
					if replace.Clicked() {
						find.SetReplacing(!find.Replacing())
					}
				}
				if portrait.QuietIconButton(c, portrait.IconX, "Close find").KeepFocus().Clicked() {
					find.Close()
				}
			})
			ui.Row(c).Gap(portrait.Form.PopoverGap).Wrap().Children(func() {
				for _, option := range []struct {
					value        *bool
					title, label string
				}{{find.CaseSensitive(), "Aa", "Match case"}, {find.WholeWord(), "ab", "Match whole word"}, {find.Regexp(), ".*", "Use regular expression"}, {find.InSelection(), "[ ]", "Find in selection"}} {
					option := option
					button := portrait.ButtonWith(c, option.title, option.label, portrait.ButtonSpec{Look: portrait.Quiet, Size: portrait.Sizes.Compact, Selected: *option.value}).Checked(*option.value)
					if button.Clicked() {
						*option.value = !*option.value
					}
				}
			})
			if find.Replacing() {
				ui.Row(c).Gap(portrait.Form.Gap).AlignItems(ui.Center).Children(func() {
					field := portrait.TextField(c, find.Replacement(), portrait.FieldSpec{Label: "Replace", Placeholder: "Replace", Size: portrait.Form.FieldCompact})
					field.Frame().MinWidth(portrait.Form.MenuMin).Grow(1)
					find.ReplacementInput(field.Input())
					if portrait.ButtonWith(c, "Replace", "Replace current match", portrait.ButtonSpec{Look: portrait.Raised, Size: portrait.Sizes.Compact}).Clicked() {
						find.Replace()
					}
					if portrait.ButtonWith(c, "Replace all", "Replace all matches", portrait.ButtonSpec{Look: portrait.Raised, Size: portrait.Sizes.Compact}).Clicked() {
						find.ReplaceAll()
					}
				})
			}
		})
	})
}

func renderGoToLine(c *ui.Context, bar ui.Element, goTo *editor.GoToLineBar) {
	bar.Row().Gap(portrait.Form.Gap).AlignItems(ui.Center).Padding(portrait.Form.PopoverPad).Radius(portrait.Radii.L)
	portrait.Materials.Pop.Apply(bar)
	bar.Children(func() {
		field := portrait.TextField(c, goTo.Text(), portrait.FieldSpec{Label: "Go to line", Placeholder: fmt.Sprintf("Line 1–%d", goTo.LineCount()), Size: portrait.Form.FieldCompact})
		goTo.Input(field.Input())
		cursor := goTo.Cursor()
		portrait.Caption(c, fmt.Sprintf("%d:%d of %d", cursor.Line+1, cursor.Col+1, goTo.LineCount())).FontFeatures("tnum")
	})
}

func renderHover(c *ui.Context, panel ui.Element, hover *editor.HoverCard) {
	editor.DefaultHover(c, panel, hover)
	panel.Padding(portrait.Form.PopoverPad).Gap(portrait.Form.Gap).Radius(portrait.Radii.L).MaxWidth(portrait.Form.MenuMax).Selectable()
	portrait.Materials.GlassOverlay.Apply(panel)
}
