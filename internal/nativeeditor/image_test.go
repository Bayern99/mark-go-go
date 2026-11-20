package nativeeditor

import (
	"image/color"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func TestPreviewLoadsHTTPSImagesFromLocalTestServer(t *testing.T) {
	const svg = `<svg xmlns="http://www.w3.org/2000/svg" width="120" height="80" viewBox="0 0 120 80"><rect width="120" height="80" fill="#2563eb"/></svg>`
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/svg+xml")
		_, _ = w.Write([]byte(svg))
	}))
	defer server.Close()
	previousTransport := remoteImageClient.Transport
	remoteImageClient.Transport = server.Client().Transport
	defer func() { remoteImageClient.Transport = previousTransport }()

	d := New("![Secure figure](" + server.URL + "/image.svg)")
	d.SetImageDirectory(t.TempDir())
	tt := ui.NewTester(func(c *ui.Context) { d.View(c, false).Fill() }, 820, 420)
	tt.Frame()
	for range 300 {
		tt.Frame()
		if r, ok := tt.Find("Secure figure"); ok && r.W > 0 && r.H > 0 {
			pixel := tt.Image().RGBAAt(int(r.X+r.W/2), int(r.Y+r.H/2))
			if imageIsBlue(pixel) {
				return
			}
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("HTTPS image from the local test server did not render")
}

func TestAbsoluteLocalImageLoadsWithoutDocumentFolder(t *testing.T) {
	dir := t.TempDir()
	const svg = `<svg xmlns="http://www.w3.org/2000/svg" width="32" height="32" viewBox="0 0 32 32"><rect width="32" height="32" fill="#16a34a"/></svg>`
	path := filepath.Join(dir, "figure.svg")
	if err := os.WriteFile(path, []byte(svg), 0600); err != nil {
		t.Fatal(err)
	}
	d := New("![Local figure](" + filepath.ToSlash(path) + ")")
	tt := ui.NewTester(func(c *ui.Context) { d.View(c, false).Fill() }, 820, 420)
	tt.Frame()
	for range 300 {
		tt.Frame()
		if r, ok := tt.Find("Local figure"); ok && r.W > 0 && r.H > 0 {
			pixel := tt.Image().RGBAAt(int(r.X+r.W/2), int(r.Y+r.H/2))
			if pixel.G > 150 && pixel.R < 80 {
				return
			}
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("absolute local image without a document folder did not render")
}

func imageIsBlue(px color.RGBA) bool { return px.B > 200 && px.R < 100 }
