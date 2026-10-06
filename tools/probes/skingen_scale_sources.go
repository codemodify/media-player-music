//go:build ignore

// Public authoring probe for distinct drawing at each asset scale.
// tools/testenv.sh go run ./tools/probes/skingen_scale_sources.go
package main

import (
	"bytes"
	"fmt"

	"github.com/codemodify/paintengine2d"
	"github.com/codemodify/uitoolkit/skingen"
)

func main() {
	var calls []float32
	draw := func(ctx *paintengine2d.Context, w, h float32) {
		scale := ctx.Matrix().ApproxScale()
		calls = append(calls, scale)
		color := paintengine2d.RGB(0, 1, 0)
		if scale > 1 {
			color = paintengine2d.RGB(0, 0, 1)
		}
		ctx.DrawRect(paintengine2d.XYWH(0, 0, w, h), paintengine2d.Fill(color))
	}
	original := &skingen.Sheet{W: 4, H: 4, Pixelated: true, Cells: []skingen.Cell{{Name: "face", W: 4, H: 4, Draw: draw}}}
	oneOriginal := skingen.Render(original, 1)
	twoOriginal := skingen.Render(original, 2)
	newSheet := *original
	newSheet.Pixelated = false
	oneNew := skingen.Render(&newSheet, 1)
	twoNew := skingen.Render(&newSheet, 2)
	_, oldGreen, oldBlue, _ := twoOriginal.PremulAt(4, 4)
	_, newGreen, newBlue, _ := twoNew.PremulAt(4, 4)
	fmt.Printf("draw_callback_scales=%v\n", calls)
	fmt.Printf("unchanged_1x=%t original_2x_green=%t independent_2x_blue=%t\n", bytes.Equal(oneOriginal.Pix, oneNew.Pix), oldGreen == 255 && oldBlue == 0, newGreen == 0 && newBlue == 255)
}
