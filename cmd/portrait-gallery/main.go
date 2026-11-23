// Command portrait-gallery opens the live Portrait component matrix.
// It is deliberately separate from the Mark Go Go application binary.
package main

import (
	"log"

	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/Bayern99/mark-go-go/internal/portrait/portraitgallery"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

func main() {
	if err := portrait.RegisterFonts(); err != nil {
		log.Fatal(err)
	}
	mygo.NewWindow(mygo.WindowOptions{
		Title: "Portrait Visual Gallery", Width: 1360, Height: 900,
		Content: ui.View(func(c *ui.Context) {
			ui.Scroll(c).Fill().Children(func() {
				ui.Column(c).Padding(16).Gap(16).AlignItems(ui.Start).Children(func() {
					for _, surface := range portraitgallery.Surfaces {
						for _, component := range portraitgallery.Components() {
							portraitgallery.Sheet(c, component.Name, portraitgallery.Default, surface)
						}
					}
				})
			})
		}),
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
