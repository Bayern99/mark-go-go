package portrait_test

var nativeLiteralAllowlist = map[string]map[string]int{
	"../nativeeditor/preview.go": {
		"Height(12)": 1,
	},
	"../nativefeynman/alpha.go": {
		"MinHeight(0)": 1,
	},
	"../nativefeynman/chat.go": {
		"MinHeight(0)": 1,
	},
	"../nativefeynman/files.go": {
		"MinHeight(0)": 1,
	},
	"../nativegenui/controls.go": {
		"Height(0)":      1,
		"Height(16)":     1,
		"MinHeight(160)": 1,
		"Radius(12)":     1,
		"Radius(14)":     1,
		"Radius(999)":    1,
	},
	"../nativegenui/media.go": {
		"Height(1)":     1,
		"Height(8)":     2,
		"MinHeight(96)": 1,
		"Radius(12)":    1,
		"Radius(2)":     2,
	},
	"../nativegenui/panel.go": {
		"MinHeight(0)": 1,
	},
	"../nativegenui/reading.go": {
		"Height(14)":                       2,
		"Height(7)":                        1,
		"Height(96)":                       2,
		"MinHeight(0)":                     2,
		"MinHeight(54)":                    1,
		"MinHeight(max(64, state.height))": 1,
		"Radius(12)":                       1,
		"Radius(4)":                        1,
		"Radius(6)":                        1,
	},
	"../nativegenui/statements.go": {
		"Height(1)":            2,
		"Height(24)":           2,
		"Height(5)":            1,
		"Height(8)":            1,
		"Height(9)":            1,
		"MinHeight(26)":        1,
		"Radius(0, 12, 12, 0)": 1,
		"Radius(12)":           2,
		"Radius(3)":            1,
		"Radius(5)":            1,
		"Radius(999)":          1,
	},
	"../nativeui/document.go": {
		"MinHeight(0)": 1,
	},
	"../nativeui/shell.go": {
		"MinHeight(0)": 2,
	},
	"../nativeui/window.go": {
		"MinHeight(0)": 1,
	},
}
