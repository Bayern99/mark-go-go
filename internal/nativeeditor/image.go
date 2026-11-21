package nativeeditor

import (
	"bytes"
	"encoding/base64"
	"errors"
	"image"
	"io"
	"mime"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo/ui"
	uimarkdown "github.com/egoist/mygo/ui/markdown"
)

const (
	maxRemoteImageBytes  = 4 << 20
	maxRemoteImagePixels = 16 << 20
	maxRemoteImageSide   = 16384
	maxRemoteBitmapSide  = 2048
	maxImageURLBytes     = 6 << 20
)

var remoteImageClient = &http.Client{
	Timeout: 8 * time.Second,
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 3 || !supportedImageURL(req.URL) || req.URL.User != nil {
			return errors.New("too many or unsafe image redirects")
		}
		if len(via) > 0 && via[len(via)-1].URL.Scheme == "https" && req.URL.Scheme != "https" {
			return errors.New("image redirect downgraded from HTTPS")
		}
		return nil
	},
}

type inlineImages struct{ entries sync.Map }

type inlineImageEntry struct {
	mu   sync.Mutex
	src  ui.ImageSource
	done bool
}

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
	if url == "" || len(url) > maxImageURLBytes || d.imageDir == "" && !loadableWithoutImageDirectory(url) {
		return nil
	}
	if d.images == nil {
		d.images = &inlineImages{}
	}
	images := d.images
	value, loaded := images.entries.LoadOrStore(url, &inlineImageEntry{})
	entry := value.(*inlineImageEntry)
	if !loaded {
		directory := d.imageDir
		services := c.Services()
		go func() {
			src, _ := loadPreviewImage(directory, url)
			entry.mu.Lock()
			entry.src = src
			entry.done = true
			entry.mu.Unlock()
			services.Invalidate()
		}()
	}
	entry.mu.Lock()
	defer entry.mu.Unlock()
	return entry.src
}

func (d *Document) pruneInlineImages(used map[string]struct{}) {
	if d.images == nil {
		return
	}
	d.images.entries.Range(func(key, _ any) bool {
		url, ok := key.(string)
		if !ok {
			return true
		}
		if _, ok := used[url]; !ok {
			d.images.entries.Delete(url)
		}
		return true
	})
}

func (d *Document) retryFailedInlineImages() {
	if d.images == nil {
		return
	}
	d.images.entries.Range(func(key, value any) bool {
		url, ok := key.(string)
		entry, validEntry := value.(*inlineImageEntry)
		if !ok || !validEntry {
			return true
		}
		entry.mu.Lock()
		failed := entry.done && entry.src == nil
		entry.mu.Unlock()
		if failed {
			d.images.entries.CompareAndDelete(url, entry)
		}
		return true
	})
}

func loadPreviewImage(directory, rawURL string) (ui.ImageSource, error) {
	if strings.HasPrefix(strings.ToLower(rawURL), "data:") {
		return loadDataImage(rawURL)
	}
	parsed, err := url.Parse(rawURL)
	if err == nil && supportedImageURL(parsed) {
		return loadRemoteImage(parsed)
	}
	return uimarkdown.FileImages(directory)(rawURL)
}

func loadableWithoutImageDirectory(rawURL string) bool {
	if strings.HasPrefix(strings.ToLower(rawURL), "data:") {
		return true
	}
	parsed, err := url.Parse(rawURL)
	if err == nil && supportedImageURL(parsed) {
		return true
	}
	path := strings.TrimPrefix(rawURL, "file:")
	return filepath.IsAbs(filepath.FromSlash(path))
}

func loadDataImage(raw string) (ui.ImageSource, error) {
	metadata, payload, ok := strings.Cut(raw[len("data:"):], ",")
	if !ok || len(payload) > base64.StdEncoding.EncodedLen(maxRemoteImageBytes)+128 {
		return nil, errors.New("invalid or oversized image data URL")
	}
	parts := strings.Split(metadata, ";")
	mediaType := strings.ToLower(parts[0])
	base64Encoded := len(parts) == 2 && strings.EqualFold(parts[1], "base64")
	if len(parts) > 2 || len(parts) == 2 && !base64Encoded || !supportedImageType(mediaType) {
		return nil, errors.New("unsupported image data URL")
	}
	var data []byte
	var err error
	if base64Encoded {
		data, err = base64.StdEncoding.DecodeString(payload)
	} else {
		var decoded string
		decoded, err = url.PathUnescape(payload)
		data = []byte(decoded)
	}
	if err != nil || len(data) > maxRemoteImageBytes {
		return nil, errors.New("invalid or oversized image data URL")
	}
	return decodeBoundedImage(mediaType, data)
}

func loadRemoteImage(target *url.URL) (ui.ImageSource, error) {
	if !supportedImageURL(target) || target.User != nil {
		return nil, errors.New("unsupported remote image URL")
	}
	req, err := http.NewRequest(http.MethodGet, target.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "image/png, image/jpeg, image/gif, image/webp, image/bmp, image/svg+xml")
	resp, err := remoteImageClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices || resp.ContentLength > maxRemoteImageBytes {
		return nil, errors.New("remote image response was unavailable or too large")
	}
	mediaType, _, err := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if err != nil || !supportedImageType(strings.ToLower(mediaType)) {
		return nil, errors.New("unsupported remote image type")
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxRemoteImageBytes+1))
	if err != nil || len(data) > maxRemoteImageBytes {
		return nil, errors.New("remote image was too large")
	}
	return decodeBoundedImage(strings.ToLower(mediaType), data)
}

func supportedImageURL(target *url.URL) bool {
	if target == nil || target.Host == "" {
		return false
	}
	scheme := strings.ToLower(target.Scheme)
	return scheme == "http" || scheme == "https"
}

func supportedImageType(mediaType string) bool {
	switch mediaType {
	case "image/png", "image/jpeg", "image/jpg", "image/gif", "image/webp", "image/bmp", "image/svg+xml":
		return true
	default:
		return false
	}
}

func decodeBoundedImage(mediaType string, data []byte) (ui.ImageSource, error) {
	if len(data) == 0 || len(data) > maxRemoteImageBytes {
		return nil, errors.New("invalid image size")
	}
	if mediaType == "image/svg+xml" {
		src, err := ui.ParseSVG(data)
		if err != nil {
			return nil, err
		}
		w, h := src.Size()
		if w <= 0 || h <= 0 || w > maxRemoteImageSide || h > maxRemoteImageSide {
			return nil, errors.New("SVG dimensions are outside the supported range")
		}
		return src, nil
	}
	config, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || config.Width <= 0 || config.Height <= 0 || config.Width > maxRemoteImageSide || config.Height > maxRemoteImageSide || int64(config.Width)*int64(config.Height) > maxRemoteImagePixels {
		return nil, errors.New("raster image dimensions are outside the supported range")
	}
	wantFormat := map[string]string{
		"image/png":  "png",
		"image/jpeg": "jpeg",
		"image/jpg":  "jpeg",
		"image/gif":  "gif",
		"image/webp": "webp",
		"image/bmp":  "bmp",
	}[mediaType]
	if format != wantFormat {
		return nil, errors.New("image data does not match its media type")
	}
	return ui.DecodeBitmapScaled(data, maxRemoteBitmapSide)
}
