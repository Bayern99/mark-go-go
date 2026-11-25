package portrait

import "github.com/egoist/mygo/ui"

type ReaderTokens struct {
	Tick, Caption, Label, Field, Body, Flip, Clock float32
	Success, Purple, Pink                          ui.Color
	TrackShade, RaisedShade                        ui.Color
	Charts                                         [6]ui.Color
}

var Reader ReaderTokens

var charts = [6]ui.Color{ui.Hex("#0285ff"), ui.Hex("#00a67d"), ui.Hex("#8b5cf6"), ui.Hex("#f4a340"), ui.Hex("#e46d91"), ui.Hex("#4b9ca6")}

func readerFor(p Palette, dark bool) ReaderTokens {
	r := ReaderTokens{
		Tick: 12, Caption: 12, Label: 13, Field: 14, Body: 15, Flip: 21, Clock: 32,
		Success: p.Success, Purple: ui.Hex("#2E317C"), Pink: ui.Hex("#9E2F2F"),
		TrackShade: black(0.05), RaisedShade: black(0.10),
		Charts: charts,
	}
	if dark {
		r.Purple, r.Pink = ui.Hex("#B8A5D2"), ui.Hex("#F0ADA0")
		r.TrackShade, r.RaisedShade = black(0.30), black(0.40)
	}
	return r
}

func ViewColor(hex string) ui.Color { return ui.Hex(hex) }
