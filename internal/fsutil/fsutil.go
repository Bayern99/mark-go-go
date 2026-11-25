package fsutil

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

func WriteFileAtomic(path string, data []byte) error {
	mode := os.FileMode(0o644)
	if info, err := os.Stat(path); err == nil {
		mode = info.Mode().Perm()
	}
	return WriteFileAtomicMode(path, data, mode)
}

func WriteFileAtomicMode(path string, data []byte, mode os.FileMode) error {
	f, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	if err := f.Chmod(mode); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}

func ResolveToolPath(dir, loc string) string {
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

func CanonicalPath(p string) string {
	if p == "" {
		return ""
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		return filepath.Clean(p)
	}
	return ResolveToolPath(abs, abs)
}

func SameFile(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	if ca, cb := CanonicalPath(a), CanonicalPath(b); ca != "" && ca == cb {
		return true
	}
	ai, err := os.Stat(a)
	if err != nil {
		return false
	}
	bi, err := os.Stat(b)
	if err != nil {
		return false
	}
	return os.SameFile(ai, bi)
}

func SameRoot(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	return CanonicalPath(a) == CanonicalPath(b)
}

func FolderContains(root, file string) bool {
	if root == "" || file == "" {
		return false
	}
	root = CanonicalPath(root)
	file = CanonicalPath(file)
	rel, err := filepath.Rel(root, file)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func DeepestRoot(file string, roots []string) string {
	var best, bestCanon string
	for _, root := range roots {
		if !FolderContains(root, file) {
			continue
		}
		canon := CanonicalPath(root)
		if best == "" || len(canon) > len(bestCanon) {
			best = root
			bestCanon = canon
		}
	}
	return best
}

func DefaultUserBinDirs() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = ""
	}
	return StableUserBinDirsFor(home, "/opt/homebrew/bin", "/usr/local/bin")
}

// StableUserBinDirsFor lists where a user's CLI tools live when the app was
// started from Finder: its PATH holds no shell initialization, so the Node
// version manager directories have to be named here. fnm's default alias is a
// stable path; the fnm_multishells directories are per-shell and never used.
func StableUserBinDirsFor(home string, system ...string) []string {
	var dirs []string
	if home != "" {
		dirs = append(dirs,
			filepath.Join(home, ".local", "bin"),
			filepath.Join(home, ".local", "share", "fnm", "aliases", "default", "bin"),
		)
		versions, _ := filepath.Glob(filepath.Join(home, ".nvm", "versions", "node", "*", "bin"))
		sort.Sort(sort.Reverse(sort.StringSlice(versions)))
		dirs = append(dirs, versions...)
		dirs = append(dirs,
			filepath.Join(home, ".volta", "bin"),
			filepath.Join(home, ".bun", "bin"),
		)
	}
	return append(dirs, system...)
}

func ResolveExecutable(command string, env []string, extraDirs []string) (string, bool) {
	if command == "" {
		return "", false
	}
	if strings.ContainsRune(command, '/') || strings.ContainsRune(command, os.PathSeparator) {
		return command, true
	}
	if env == nil {
		if path, err := exec.LookPath(command); err == nil {
			return path, true
		}
	} else {
		pathValue := ""
		for _, entry := range env {
			if strings.HasPrefix(entry, "PATH=") {
				pathValue = strings.TrimPrefix(entry, "PATH=")
				break
			}
		}
		if found, ok := findExecutableInDirs(filepath.SplitList(pathValue), command); ok {
			return found, true
		}
	}
	return findExecutableInDirs(extraDirs, command)
}

func findExecutableInDirs(dirs []string, name string) (string, bool) {
	candidates := []string{name}
	if runtime.GOOS == "windows" && filepath.Ext(name) == "" {
		candidates = append(candidates, name+".exe", name+".cmd", name+".bat")
	}
	for _, dir := range dirs {
		if dir == "" {
			continue
		}
		for _, candidateName := range candidates {
			candidate := filepath.Join(dir, candidateName)
			info, err := os.Stat(candidate)
			if err != nil || !info.Mode().IsRegular() {
				continue
			}
			if runtime.GOOS == "windows" || info.Mode().Perm()&0o111 != 0 {
				return candidate, true
			}
		}
	}
	return "", false
}

func MarkdownName(name string) bool {
	lower := strings.ToLower(name)
	return strings.HasSuffix(lower, ".md") || strings.HasSuffix(lower, ".markdown")
}

func SkipDirName(name string) bool {
	switch name {
	case ".git", "node_modules", ".hg", ".svn", "__pycache__", ".next", "dist", "build":
		return true
	default:
		return strings.HasPrefix(name, ".") && name != "."
	}
}
