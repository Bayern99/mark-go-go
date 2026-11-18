package main

import (
	"os"
	"path/filepath"
)

func resolveToolPath(dir, loc string) string {
	p := loc
	if !filepath.IsAbs(p) {
		p = filepath.Join(dir, p)
	}
	p = filepath.Clean(p)
	rest := ""
	for {
		resolved, err := filepath.EvalSymlinks(p)
		if err == nil {
			p = resolved
			break
		}
		parent := filepath.Dir(p)
		if parent == p {
			break
		}
		rest = filepath.Join(filepath.Base(p), rest)
		p = parent
	}
	p = filepath.Join(p, rest)
	if fi, err := os.Lstat(p); err == nil && fi.Mode()&os.ModeSymlink != 0 {
		if resolved, err := filepath.EvalSymlinks(p); err == nil {
			p = resolved
		}
	}
	return p
}

func agentToolAllowed(dir, kind string, locations []string) bool {
	switch kind {
	case "think":
		return true
	case "read", "search":
		if len(locations) == 0 {
			return false
		}
		for _, loc := range locations {
			switch resolveToolPath(dir, loc) {
			case dir, filepath.Join(dir, "source.md"), filepath.Join(dir, "view.json"), filepath.Join(dir, "out.json"):
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
			if resolveToolPath(dir, loc) != filepath.Join(dir, "out.json") {
				return false
			}
		}
		return true
	default:
		return false
	}
}
