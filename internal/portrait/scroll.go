package portrait

import "github.com/egoist/mygo/ui"

// ScrollEdgeMaskX returns the paper fade that covers content scrolling under
// one horizontal edge, and whether that edge has more content to reveal. The
// caller paints it above the viewport with input and accessibility disabled.
func ScrollEdgeMaskX(scroll *ui.ScrollState, left bool) (ui.LinearGradient, bool) {
	if scroll == nil {
		return ui.LinearGradient{}, false
	}
	visible := scroll.MaxX-scroll.X > Editor.Hairline
	if left {
		visible = scroll.X > Editor.Hairline
	}
	if !visible {
		return ui.LinearGradient{}, false
	}
	mask := ui.LinearGradient{Angle: 90, From: ui.Transparent, To: Colors.Paper}
	if left {
		mask.From, mask.To = mask.To, mask.From
	}
	return mask, true
}
