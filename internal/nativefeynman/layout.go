package nativefeynman

import "github.com/egoist/mygo/ui"

func compactResearchLayout(c *ui.Context) bool {
	width, height := c.Size()
	return width < 641 || height < 430
}
