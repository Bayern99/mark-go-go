package pandoc

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"strings"

	"github.com/Bayern99/mark-go-go/internal/export"
	"github.com/Bayern99/mark-go-go/internal/markdown/svgfigure"
	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/egoist/mygo/ui"
	mermaid "github.com/zkrebbekx/go-mermaid"
)

var renderMermaidSVG = svgfigure.RenderMermaidSVG
var parseSVG = ui.ParseSVG

func opaqueHex(fg, bg ui.Color) string {
	alpha := float64(fg.A) / 255
	blend := func(a, b uint8) uint8 {
		return uint8(math.Round(float64(a)*alpha + float64(b)*(1-alpha)))
	}
	return fmt.Sprintf("#%02x%02x%02x", blend(fg.R, bg.R), blend(fg.G, bg.G), blend(fg.B, bg.B))
}

func rasterizeSVGTo2xPNG(svg []byte) ([]byte, int, int, error) {
	parsed, err := parseSVG(svg)
	if err != nil {
		return nil, 0, 0, err
	}
	logicalWidth, logicalHeight := parsed.Size()
	width, height := int(math.Ceil(float64(logicalWidth))), int(math.Ceil(float64(logicalHeight)))
	if width <= 0 || height <= 0 || width > 4096 || height > 4096 {
		return nil, 0, 0, fmt.Errorf("invalid SVG dimensions %gx%g", logicalWidth, logicalHeight)
	}
	raster := parsed.Image(width*2, height*2, nil)
	opaque := image.NewRGBA(raster.Bounds())
	draw.Draw(opaque, opaque.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
	draw.Draw(opaque, opaque.Bounds(), raster, raster.Bounds().Min, draw.Over)
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, opaque); err != nil {
		return nil, 0, 0, err
	}
	return encoded.Bytes(), width, height, nil
}

func renderMermaidPortraitSVG(body string, options export.Options) ([]byte, error) {
	space, ok := portrait.SpaceNamed(options.SpaceName)
	if !ok {
		space = portrait.DefaultSpace
	}
	colors := portrait.Snapshot(space, false).Colors
	paper := colors.Paper
	palette := mermaid.Palette{
		Background: opaqueHex(paper, paper),
		NodeFill:   opaqueHex(colors.Fill, paper),
		NodeStroke: opaqueHex(colors.Accent, paper),
		Text:       opaqueHex(colors.Ink, paper),
		Edge:       opaqueHex(colors.Ink2, paper),
	}
	return renderMermaidSVG(body, mermaid.WithCustomTheme("portrait", palette))
}

func mermaidImage(body, formatID string, options export.Options) (string, error) {
	svg, err := renderMermaidPortraitSVG(body, options)
	if err != nil {
		return "", err
	}
	if formatID == "markdown-epub" {
		return "![Mermaid diagram](data:image/svg+xml;base64," + base64.StdEncoding.EncodeToString(svg) + ")", nil
	}
	pngBytes, width, height, err := rasterizeSVGTo2xPNG(svg)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("![Mermaid diagram](data:image/png;base64,%s){width=%dpx height=%dpx}", base64.StdEncoding.EncodeToString(pngBytes), width, height), nil
}

func diagramSummary(body string) string {
	summary := strings.Join(strings.Fields(body), " ")
	if len(summary) > 48 {
		summary = summary[:48] + "…"
	}
	if summary == "" {
		return "empty diagram"
	}
	return summary
}
