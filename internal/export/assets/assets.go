package assets

import (
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

const (
	MaxImageBytes = 4 << 20
	MaxTotalBytes = 24 << 20
)

type Resolution struct {
	URL     string
	Missing bool
	Name    string
}

type Resolver struct {
	baseDir  string
	offline  bool
	root     *os.Root
	rootErr  error
	used     int64
	warnings []string
}

func New(baseDir string) *Resolver {
	return NewWithOptions(baseDir, false)
}

func NewWithOptions(baseDir string, offline bool) *Resolver {
	r := &Resolver{offline: offline}
	if baseDir != "" {
		baseDir, _ = filepath.Abs(baseDir)
		if resolved, err := filepath.EvalSymlinks(baseDir); err == nil {
			baseDir = resolved
		}
		r.baseDir = baseDir
		r.root, r.rootErr = os.OpenRoot(baseDir)
	}
	return r
}

func (r *Resolver) Close() error {
	if r == nil || r.root == nil {
		return nil
	}
	err := r.root.Close()
	r.root = nil
	return err
}

func SafeLink(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.ContainsAny(raw, "\x00\r\n") {
		return "", false
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", false
	}
	switch strings.ToLower(parsed.Scheme) {
	case "", "http", "https", "mailto":
		return raw, true
	default:
		return "", false
	}
}

func (r *Resolver) Image(raw string) Resolution {
	src := strings.TrimSpace(raw)
	if src == "" {
		return Resolution{Missing: true, Name: "image"}
	}
	parsed, err := url.Parse(src)
	if err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") {
		if r.offline {
			name := filepath.Base(parsed.Path)
			if name == "" || name == "." || name == "/" {
				name = parsed.Host
			}
			if name == "" {
				name = "remote image"
			}
			r.warn("Remote image %q is omitted in offline export", src)
			return Resolution{Missing: true, Name: name}
		}
		return Resolution{URL: src}
	}
	path := filepath.FromSlash(src)
	if filepath.IsAbs(path) || err != nil || parsed.Scheme != "" || strings.HasPrefix(src, "//") {
		r.warn("Blocked unsafe image URL %q", src)
		return Resolution{Missing: true, Name: filepath.Base(src)}
	}
	if r.baseDir == "" {
		r.warn("Image %q cannot be resolved without a document directory", src)
		return Resolution{Missing: true, Name: filepath.Base(path)}
	}
	if r.root == nil || r.rootErr != nil {
		r.warn("Image %q cannot open the document directory", src)
		return Resolution{Missing: true, Name: filepath.Base(path)}
	}
	file, err := openRegularCandidate(r.root, path)
	if err != nil {
		r.warn("Image %q is outside the document directory or missing", src)
		return Resolution{Missing: true, Name: filepath.Base(path)}
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		r.warn("Image %q is not a regular file", src)
		return Resolution{Missing: true, Name: filepath.Base(path)}
	}
	limit := min(int64(MaxImageBytes), int64(MaxTotalBytes)-r.used)
	if limit <= 0 || info.Size() > limit {
		r.warn("Image %q exceeds the inline image budget", src)
		return Resolution{Missing: true, Name: filepath.Base(path)}
	}
	data, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		r.warn("Image %q could not be read", src)
		return Resolution{Missing: true, Name: filepath.Base(path)}
	}
	if int64(len(data)) > limit {
		r.warn("Image %q exceeds the inline image budget", src)
		return Resolution{Missing: true, Name: filepath.Base(path)}
	}
	mediaType := mime.TypeByExtension(strings.ToLower(filepath.Ext(path)))
	if mediaType == "" || !strings.HasPrefix(mediaType, "image/") {
		mediaType = http.DetectContentType(data)
	}
	if strings.HasPrefix(mediaType, "text/") && strings.EqualFold(filepath.Ext(path), ".svg") {
		mediaType = "image/svg+xml"
	}
	if i := strings.IndexByte(mediaType, ';'); i >= 0 {
		mediaType = mediaType[:i]
	}
	if !strings.HasPrefix(mediaType, "image/") {
		r.warn("Image %q has an unsupported file type", src)
		return Resolution{Missing: true, Name: filepath.Base(path)}
	}
	r.used += int64(len(data))
	return Resolution{URL: "data:" + mediaType + ";base64," + base64.StdEncoding.EncodeToString(data)}
}

func (r *Resolver) Warnings() []string { return append([]string(nil), r.warnings...) }

func (r *Resolver) warn(format string, args ...any) {
	r.warnings = append(r.warnings, fmt.Sprintf(format, args...))
}
