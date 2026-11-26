package assets

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolverLocalRemoteAndUnsafeImages(t *testing.T) {
	dir := t.TempDir()
	png, _ := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=")
	if err := os.WriteFile(filepath.Join(dir, "pixel.png"), png, 0o600); err != nil {
		t.Fatal(err)
	}
	resolver := New(dir)
	if got := resolver.Image("pixel.png"); !strings.HasPrefix(got.URL, "data:image/png;base64,") {
		t.Fatalf("local image = %+v", got)
	}
	remote := "https://example.com/image.png"
	if got := resolver.Image(remote); got.URL != remote || got.Missing {
		t.Fatalf("remote image = %+v", got)
	}
	if got := resolver.Image("missing.png"); !got.Missing || got.Name != "missing.png" {
		t.Fatalf("missing image = %+v", got)
	}
	if got := resolver.Image("javascript:alert(1)"); !got.Missing {
		t.Fatal("unsafe image was not blocked")
	}
	if got := resolver.Image("../outside.png"); !got.Missing {
		t.Fatal("relative path traversal was not blocked")
	}
	if len(resolver.Warnings()) < 3 {
		t.Fatalf("warnings = %v", resolver.Warnings())
	}
}

func TestResolverRejectsAbsoluteAndEscapingPaths(t *testing.T) {
	dir := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside.png")
	png, _ := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=")
	if err := os.WriteFile(outside, png, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(dir, "escape.png")); err != nil {
		t.Skip(err)
	}
	resolver := New(dir)
	for _, name := range []string{outside, "escape.png"} {
		if got := resolver.Image(name); got.URL != "" || !got.Missing {
			t.Errorf("Image(%q) = %+v", name, got)
		}
	}
	if len(resolver.Warnings()) != 2 {
		t.Fatalf("warnings = %v", resolver.Warnings())
	}
}

func TestResolverEnforcesCumulativeBudget(t *testing.T) {
	dir := t.TempDir()
	png, _ := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=")
	data := make([]byte, MaxImageBytes)
	copy(data, png)
	resolver := New(dir)
	for i := 0; i <= MaxTotalBytes/MaxImageBytes; i++ {
		name := filepath.Join(dir, string(rune('a'+i))+".png")
		if err := os.WriteFile(name, data, 0o600); err != nil {
			t.Fatal(err)
		}
		got := resolver.Image(filepath.Base(name))
		if i < MaxTotalBytes/MaxImageBytes && !strings.HasPrefix(got.URL, "data:image/png;base64,") {
			t.Fatalf("image %d = %+v", i, got)
		}
		if i == MaxTotalBytes/MaxImageBytes && (got.URL != "" || !got.Missing) {
			t.Fatalf("over-budget image = %+v", got)
		}
	}
	if resolver.used != MaxTotalBytes {
		t.Fatalf("used = %d, want %d", resolver.used, MaxTotalBytes)
	}
}

func TestResolverLimitsAndSafeLinks(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "large.png")
	if err := os.WriteFile(path, make([]byte, MaxImageBytes+1), 0o600); err != nil {
		t.Fatal(err)
	}
	resolver := New(dir)
	if got := resolver.Image("large.png"); got.URL != "" || !got.Missing {
		t.Fatalf("large image = %+v", got)
	}
	if len(resolver.Warnings()) != 1 {
		t.Fatalf("warnings = %v", resolver.Warnings())
	}
	if _, ok := SafeLink("javascript:alert(1)"); ok {
		t.Fatal("javascript link was accepted")
	}
	for _, link := range []string{"#section", "../guide.md", "https://example.com", "mailto:a@example.com"} {
		if _, ok := SafeLink(link); !ok {
			t.Errorf("safe link %q was rejected", link)
		}
	}
}

func TestResolverPinsDocumentDirectoryAcrossRename(t *testing.T) {
	parent := t.TempDir()
	docDir := filepath.Join(parent, "doc")
	movedDir := filepath.Join(parent, "moved")
	if err := os.MkdirAll(docDir, 0o755); err != nil {
		t.Fatal(err)
	}
	png, _ := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=")
	if err := os.WriteFile(filepath.Join(docDir, "pixel.png"), png, 0o600); err != nil {
		t.Fatal(err)
	}
	resolver := New(docDir)
	defer resolver.Close()
	if err := os.Rename(docDir, movedDir); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(docDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if got := resolver.Image("pixel.png"); !strings.HasPrefix(got.URL, "data:image/png;base64,") {
		t.Fatalf("pinned resolver failed after directory rename: %+v", got)
	}
}
