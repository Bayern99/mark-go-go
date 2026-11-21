package nativeui

import (
	"embed"
	"fmt"
	"io/fs"
	"strings"
	"sync"

	"github.com/egoist/mygo/ui"
)

//go:embed fonts
var typefaces embed.FS

var RegisterFonts = sync.OnceValue(func() error {
	files, err := fs.Glob(typefaces, "fonts/*.ttf")
	if err != nil {
		return err
	}
	for _, path := range files {
		contents, err := typefaces.ReadFile(path)
		if err != nil {
			return err
		}
		var name string
		switch {
		case strings.Contains(path, "IBMPlexSansSC"):
			name = "IBM Plex Sans SC"
		case strings.Contains(path, "ibm-plex-sans"):
			name = "IBM Plex Sans"
		case strings.Contains(path, "ibm-plex-mono"):
			name = "IBM Plex Mono"
		case strings.Contains(path, "barlow-semi-condensed"):
			name = "Barlow Semi Condensed"
		case strings.Contains(path, "Phosphor"):
			name = "Phosphor"
		}
		if err := ui.RegisterFont(contents, name); err != nil {
			return fmt.Errorf("load %s: %w", path, err)
		}
	}
	return nil
})
