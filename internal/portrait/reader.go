package portrait

import "github.com/egoist/mygo/ui"

type ReaderTokens struct {
	Tick, Caption, Label, Field, Body, Flip, Clock float32
	Success, Purple, Pink                          ui.Color
	TrackShade, RaisedShade                        ui.Color
	Charts                                         [6]ui.Color
}

var Reader = ReaderTokens{
	Tick: 12, Caption: 12, Label: 13, Field: 14, Body: 15, Flip: 21, Clock: 32,
	Success: ui.Hex("#1f8a5b"), Purple: ui.Hex("#6d28d9"), Pink: ui.Hex("#a61e5c"),
	TrackShade: black(0.05), RaisedShade: black(0.10),
	Charts: [6]ui.Color{ui.Hex("#0285ff"), ui.Hex("#00a67d"), ui.Hex("#8b5cf6"), ui.Hex("#f4a340"), ui.Hex("#e46d91"), ui.Hex("#4b9ca6")},
}

func ViewColor(hex string) ui.Color { return ui.Hex(hex) }
