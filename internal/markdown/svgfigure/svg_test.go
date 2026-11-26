package svgfigure

import (
	"bytes"
	"encoding/xml"
	"io"
	"strings"
	"testing"
)

func TestSanitizeSVGRemovesActiveContentAndExternalReferences(t *testing.T) {
	input := []byte(`<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" onload="alert(1)"><script>alert(1)</script><style>@im<!-- split -->port url(https://example.com/evil.css);</style><rect id="safe" onclick="alert(1)" href="https://example.com" xlink:href="javascript:alert(1)" src="https://example.com/x.png" fill="url(https://example.com/p)" style="fill:url(https://example.com)"/><circle style="fill:url(#safe);stroke:url(https://example.com)"/><a href="#safe"><text>local</text></a></svg>`)
	sanitized, err := SanitizeSVG(input)
	if err != nil {
		t.Fatal(err)
	}
	decoder := xml.NewDecoder(bytes.NewReader(sanitized))
	elements := 0
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		start, ok := token.(xml.StartElement)
		if !ok {
			continue
		}
		elements++
		if strings.EqualFold(start.Name.Local, "script") {
			t.Fatal("script survived sanitization")
		}
		for _, attr := range start.Attr {
			name := strings.ToLower(attr.Name.Local)
			if strings.HasPrefix(name, "on") {
				t.Fatalf("event attribute survived: %+v", attr)
			}
			if (name == "href" || name == "src") && !strings.HasPrefix(attr.Value, "#") {
				t.Fatalf("external reference survived: %+v", attr)
			}
			if strings.Contains(attr.Value, "https://") {
				t.Fatalf("external URL survived in attr %s: %+v", name, attr)
			}
		}
	}
	if strings.Contains(string(sanitized), "@import") {
		t.Fatalf("external style element content survived: %s", sanitized)
	}
	if elements == 0 || !strings.Contains(string(sanitized), `href="#safe"`) {
		t.Fatalf("safe local link was not preserved: %s", sanitized)
	}
}

func TestMermaidRejectsClickLinksWithoutHrefKeyword(t *testing.T) {
	for _, source := range []string{
		"graph TD; A-->B\nclick A \"https://example.com\"",
		"graph TD; A-->B\nclick A https://example.com",
	} {
		if _, err := RenderMermaidSVG(source); err == nil || !strings.Contains(err.Error(), "external resource") {
			t.Errorf("RenderMermaidSVG(%q) error = %v", source, err)
		}
	}
	if svg, err := RenderMermaidSVG("graph TD; A[Click here for https://example.com]-->B"); err != nil || !strings.Contains(string(svg), "<svg") {
		t.Fatalf("ordinary node label with Click text was rejected: err=%v svg=%s", err, svg)
	}
}
