package svgfigure

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"regexp"
	"strings"
	"sync"

	mathtex "github.com/go-tex/math"
	mermaid "github.com/zkrebbekx/go-mermaid"
)

var typesetter = sync.OnceValues(func() (*mathtex.Renderer, error) { return mathtex.New(mathtex.DefaultFont()) })
var mermaidDirective = regexp.MustCompile("(?s)%%\\{.*?\\}%%")
var diagramHeader = regexp.MustCompile("(?s)^\\s*---\\n.*?\\n---\\s*\\n")
var externalDiagram = regexp.MustCompile(`(?i)<img\b|!\[[^\]]*\]\(|@\{[^}]*\b(?:img|icon)\s*:|\b(?:href|src)\s*=|url\(\s*['"]?[^#'"]`)
var mermaidClickDirective = regexp.MustCompile(`(?im)(?:^|[;\n])[ \t]*click[ \t]+\S+`)

var allowedSVGElements = map[string]bool{
	"svg": true, "g": true, "defs": true, "symbol": true, "use": true,
	"title": true, "desc": true, "path": true, "rect": true, "circle": true,
	"ellipse": true, "line": true, "polyline": true, "polygon": true,
	"text": true, "tspan": true, "marker": true, "clippath": true,
	"mask": true, "lineargradient": true, "radialgradient": true,
	"stop": true, "pattern": true, "filter": true, "fegaussianblur": true,
	"feoffset": true, "feblend": true, "fedropshadow": true,
	"fecolormatrix": true, "femerge": true, "femergenode": true, "style": true, "a": true,
}

func RenderMathSVG(source string, display bool, sizePx int) (string, error) {
	renderer, err := typesetter()
	if err != nil {
		return "", err
	}
	if display {
		svg, _, err := renderer.RenderDisplaySVGMetrics(source, sizePx)
		return svg, err
	}
	svg, _, err := renderer.RenderSVGMetrics(source, sizePx)
	return svg, err
}

func RenderMermaidSVG(source string, opts ...mermaid.Option) ([]byte, error) {
	input := strings.TrimSpace(diagramHeader.ReplaceAllString(mermaidDirective.ReplaceAllString(source, ""), ""))
	switch {
	case externalDiagram.MatchString(input) || mermaidClickDirective.MatchString(input):
		return nil, fmt.Errorf("diagram references an external resource")
	case input == "":
		return nil, fmt.Errorf("empty diagram")
	default:
		svg, err := mermaid.Render(input, opts...)
		if err != nil {
			return nil, err
		}
		return SanitizeSVG(svg)
	}
}

func SanitizeSVG(svg []byte) ([]byte, error) {
	decoder := xml.NewDecoder(bytes.NewReader(svg))
	var out bytes.Buffer
	encoder := xml.NewEncoder(&out)
	skipped := 0
	inStyle := false
	var styleStart xml.StartElement
	var styleBuf strings.Builder
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if skipped > 0 {
			switch token.(type) {
			case xml.StartElement:
				skipped++
			case xml.EndElement:
				skipped--
			}
			continue
		}
		switch token := token.(type) {
		case xml.StartElement:
			tag := strings.ToLower(token.Name.Local)
			if !allowedSVGElements[tag] {
				skipped = 1
				continue
			}
			attrs := token.Attr[:0]
			for _, attr := range token.Attr {
				name := strings.ToLower(attr.Name.Local)
				if strings.HasPrefix(name, "on") || name == "srcdoc" {
					continue
				}
				if name == "href" || name == "src" {
					if !strings.HasPrefix(strings.TrimSpace(attr.Value), "#") {
						continue
					}
				}
				if externalStyle(attr.Value) {
					continue
				}
				attrs = append(attrs, attr)
			}
			token.Attr = attrs
			if tag == "style" {
				inStyle = true
				styleStart = token.Copy()
				styleBuf.Reset()
				continue
			}
			if err := encoder.EncodeToken(token); err != nil {
				return nil, err
			}
		case xml.EndElement:
			if inStyle && strings.ToLower(token.Name.Local) == "style" {
				inStyle = false
				css := styleBuf.String()
				styleBuf.Reset()
				if !externalStyle(css) {
					if err := encoder.EncodeToken(styleStart); err != nil {
						return nil, err
					}
					if css != "" {
						if err := encoder.EncodeToken(xml.CharData(css)); err != nil {
							return nil, err
						}
					}
					if err := encoder.EncodeToken(token); err != nil {
						return nil, err
					}
				}
				continue
			}
			if err := encoder.EncodeToken(token); err != nil {
				return nil, err
			}
		case xml.CharData:
			if inStyle {
				styleBuf.Write(token)
				continue
			}
			if err := encoder.EncodeToken(token); err != nil {
				return nil, err
			}
		case xml.Comment:
			if inStyle {
				continue
			}
			if err := encoder.EncodeToken(token); err != nil {
				return nil, err
			}
		}
	}
	if err := encoder.Flush(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func externalStyle(style string) bool {
	style = strings.ToLower(style)
	if strings.Contains(style, "expression") || strings.Contains(style, "@import") || strings.Contains(style, "javascript:") {
		return true
	}
	for {
		_, rest, found := strings.Cut(style, "url(")
		if !found {
			return false
		}
		rest = strings.TrimLeft(rest, " \t\r\n\"'")
		if !strings.HasPrefix(rest, "#") {
			return true
		}
		style = rest
	}
}
