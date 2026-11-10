//go:build ignore

// Public positive/negative keycap presentation controls, with a real installed
// outline lender. Run with font_emoji against .12 and .13 dependency graphs.
package main

import (
	"bytes"
	"fmt"

	"goframework.io/desktop-ui/style"
	"goframework.io/graphics-paint2d"
)

type countedDevice struct {
	paint2d.Device
	blits int
}

func (d *countedDevice) Blit(src *paint2d.Image, srcRect, dstRect paint2d.Rect, xform paint2d.Matrix, paint paint2d.Paint, clip paint2d.Clip) {
	d.blits++
	d.Device.Blit(src, srcRect, dstRect, xform, paint, clip)
}

func render(f *style.Font, text string, colour paint2d.Color) (*paint2d.Image, int) {
	image := paint2d.NewImage(240, 100)
	device := &countedDevice{Device: paint2d.NewCPUDevice(image)}
	f.Draw(paint2d.NewContextDevice(device), text, paint2d.Pt(10, 10), colour)
	return image, device.blits
}

func main() {
	style.SetFallbackFamilies()
	defer style.SetFallbackFamilies()
	f := style.BakeFont(24, paint2d.White)
	fmt.Printf("primary=%q symbolsInstalled=%t emojiInstalled=%t\n", f.Family, style.FontInstalled("Noto Sans Symbols"), style.FontInstalled("Noto Color Emoji"))
	for _, text := range []string{"A", "A\u20e3", "1", "1\ufe0e", "1\ufe0e\u20e3", "1\u20e3", "1\ufe0f\u20e3", "❤", "❤\ufe0e", "❤\ufe0f"} {
		white, cells := render(f, text, paint2d.White)
		red, _ := render(f, text, paint2d.RGB(1, 0, 0))
		unchanged, ink := 0, 0
		for y := 0; y < white.Height; y++ {
			for x := 0; x < white.Width; x++ {
				p := white.NRGBAAt(x, y)
				if p.A > 40 {
					ink++
					if p == red.NRGBAAt(x, y) {
						unchanged++
					}
				}
			}
		}
		fmt.Printf("text=%q width=%g cells=%d inkPixels=%d untintedPixels=%d imagesEqual=%t\n", text, f.Advance(text), cells, ink, unchanged, bytes.Equal(white.Pix, red.Pix))
	}
}
