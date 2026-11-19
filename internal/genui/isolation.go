package genui

import (
	"path/filepath"

	"github.com/Bayern99/mark-go-go/internal/fsutil"
)

func AgentToolAllowed(dir, kind string, locations []string) bool {
	base := fsutil.ResolveToolPath(dir, dir)
	switch kind {
	case "think":
		return true
	case "read", "search":
		if len(locations) == 0 {
			return false
		}
		for _, loc := range locations {
			switch fsutil.ResolveToolPath(base, loc) {
			case base, filepath.Join(base, "source.md"), filepath.Join(base, "view.json"), filepath.Join(base, "out.json"):
			default:
				return false
			}
		}
		return true
	case "edit":
		if len(locations) == 0 {
			return false
		}
		for _, loc := range locations {
			if fsutil.ResolveToolPath(base, loc) != filepath.Join(base, "out.json") {
				return false
			}
		}
		return true
	default:
		return false
	}
}
