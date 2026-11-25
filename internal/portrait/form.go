package portrait

// FormTokens defines the shared metrics for fields, menus, popovers and
// dialogs. Control heights continue to come from the closed Sizes ladder.
type FormTokens struct {
	Field, FieldCompact                   float32
	Radius, PadX, Gap                     float32
	AreaMin, AreaCompact, AreaPadY        float32
	LabelGap, HelpGap                     float32
	SelectMin, SelectMax                  float32
	MenuRow, MenuRadius, MenuPad          float32
	MenuMin, MenuMax                      float32
	PopoverPad, PopoverGap, PopoverHeader float32
	PopoverWide                           float32
	DialogWidth, DialogPad, DialogGap     float32
	ListMax                               float32
	BubbleRatio, BubbleMax                float32
}

var Form = FormTokens{
	Field: Sizes.Large, FieldCompact: Sizes.Regular,
	Radius: Radii.M, PadX: 10, Gap: 6,
	AreaMin: 72, AreaCompact: 42, AreaPadY: 8,
	LabelGap: 6, HelpGap: 4,
	SelectMin: 128, SelectMax: 260,
	MenuRow: Sizes.Regular, MenuRadius: Radii.S, MenuPad: 4, MenuMin: 180, MenuMax: 320,
	PopoverPad: 6, PopoverGap: 2, PopoverHeader: Sizes.Large, PopoverWide: 520,
	DialogWidth: 420, DialogPad: 20, DialogGap: 12,
	ListMax:     280,
	BubbleRatio: 0.82, BubbleMax: 360,
}
