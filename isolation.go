package main

import (
	"os"
	"path/filepath"
)

// resolveToolPath canonicalizes one tool location against dir, the
// canonical working directory of the agent: a relative path is joined
// with dir, every path is cleaned, the symlinks of the longest
// existing prefix are resolved with the missing rest appended, and a
// final symlink is resolved. The result is an absolute path without
// symlinks, so that comparing it with paths of dir sees through the
// /private/var and /var aliases of macOS and through links.
func resolveToolPath(dir, loc string) string {
	p := loc
	if !filepath.IsAbs(p) {
		p = filepath.Join(dir, p)
	}
	p = filepath.Clean(p)
	// Walk up from p to the longest prefix that exists, resolving its
	// symlinks, then put the missing rest back.
	rest := ""
	for {
		resolved, err := filepath.EvalSymlinks(p)
		if err == nil {
			p = resolved
			break
		}
		parent := filepath.Dir(p)
		if parent == p {
			break // the root does not exist either; leave p as it is
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

// agentToolAllowed says whether a tool call of kind with the location
// paths locations stays inside dir, the agent's canonical working
// directory. Thinking may touch anything; reading and searching only
// the working directory itself, source.md, view.json (the current view,
// there for an update) and out.json; editing only out.json. Everything
// else, and any path that leaves dir, is denied: other kinds, a missing
// kind, no locations for read, search or edit, and paths elsewhere.
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
