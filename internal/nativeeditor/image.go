package nativeeditor

import (
	"path/filepath"
	"sync"

	"github.com/egoist/mygo/ui"
	uimarkdown "github.com/egoist/mygo/ui/markdown"
)

type inlineImages struct{ entries sync.Map }

type inlineImageEntry struct {
	mu  sync.Mutex
	src ui.ImageSource
}

// SetImageDirectory sets the folder that relative Preview images are read
// from. MyGo's file image loader accepts local files and SVGs, and rejects
// network and data URLs.
func (d *Document) SetImageDirectory(dir string) {
	if dir != "" {
		dir = filepath.Clean(dir)
	}
	if d.imageDir == dir {
		return
	}
	d.imageDir = dir
	d.images = &inlineImages{}
	d.preview.dirty = true
	d.preview.applied = false
}

func (d *Document) loadInlineImage(c *ui.Context, url string) ui.ImageSource {
	if d.imageDir == "" || url == "" {
		return nil
	}
	if d.images == nil {
		d.images = &inlineImages{}
	}
	images := d.images
	value, loaded := images.entries.LoadOrStore(url, &inlineImageEntry{})
	entry := value.(*inlineImageEntry)
	if !loaded {
		loader := uimarkdown.FileImages(d.imageDir)
		services := c.Services()
		go func() {
			src, _ := loader(url)
			entry.mu.Lock()
			entry.src = src
			entry.mu.Unlock()
			services.Invalidate()
		}()
	}
	entry.mu.Lock()
	defer entry.mu.Unlock()
	return entry.src
}
