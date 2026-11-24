package nativefeynman

import (
	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/egoist/mygo/ui"
)

func compactResearchLayout(c *ui.Context) bool {
	_, height := c.Size()
	return height < 6*portrait.Form.AreaMin
}
