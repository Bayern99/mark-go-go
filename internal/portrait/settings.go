package portrait

// SettingsTokens describe the Settings Center sheet, rail and content column.
type SettingsTokens struct {
	Width, Height, Margin, Radius             float32
	Rail, Header, Column                      float32
	ColumnPadX, ColumnPadTop, ColumnPadBottom float32
	GroupGap, HeadingGap                      float32
	NavRow, NavSectionGap, Collapse           float32
	Row, RowCompact, RowPadX, RowPadY, RowGap float32
	DescriptionMax                            float32
	SpacePreview                              float32
}

var Settings = SettingsTokens{
	Width: 1040, Height: 720, Margin: 32, Radius: Radii.XL,
	Rail: 224, Header: 44, Column: 720,
	ColumnPadX: 32, ColumnPadTop: 28, ColumnPadBottom: 40,
	GroupGap: 28, HeadingGap: 10,
	NavRow: Sizes.Large, NavSectionGap: 14, Collapse: 720,
	Row: 56, RowCompact: 44, RowPadX: 16, RowPadY: 12, RowGap: 16,
	DescriptionMax: 440, SpacePreview: 32,
}

// IconScale is the only glyph sizing scale used by native surfaces.
type IconScale struct{ XS, S, M, L float32 }

var IconSizes = IconScale{XS: 12, S: 14, M: 16, L: 20}
