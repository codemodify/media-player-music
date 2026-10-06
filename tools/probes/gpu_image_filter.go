//go:build ignore

// Exercise image-filter changes on one cached GPU texture without any window.
// tools/testenv.sh go run ./tools/probes/gpu_image_filter.go
package main

import (
	"fmt"

	"github.com/codemodify/paintengine2d"
)

func main() {
	for _, first := range []paintengine2d.FilterMode{paintengine2d.FilterBilinear, paintengine2d.FilterNearest} {
		dev, err := paintengine2d.NewGPUDevice(40, 20)
		if err != nil {
			fmt.Printf("GPU unavailable: %v\n", err)
			return
		}
		src := paintengine2d.NewImage(4, 4)
		for y := range 4 {
			for x := range 4 {
				c := paintengine2d.Black
				if (x+y)%2 == 0 {
					c = paintengine2d.White
				}
				src.SetColor(x, y, c)
			}
		}
		ctx := paintengine2d.NewContextDevice(dev)
		ctx.Clear(paintengine2d.Black)
		ctx.DrawImageRectPaint(src, paintengine2d.XYWH(0, 0, 4, 4), paintengine2d.XYWH(0, 0, 16, 16), paintengine2d.Paint{Filter: first})
		ctx.DrawImageRectPaint(src, paintengine2d.XYWH(0, 0, 4, 4), paintengine2d.XYWH(20, 0, 16, 16), paintengine2d.Paint{Filter: paintengine2d.FilterNearest})
		im := dev.Snapshot()
		blended := 0
		for y := 2; y < 14; y++ {
			for x := 22; x < 34; x++ {
				r, _, _, a := im.PremulAt(x, y)
				if a != 255 || r != 0 && r != 255 {
					blended++
				}
			}
		}
		fmt.Printf("first_filter=%v second_filter=nearest second_interior_blended_pixels=%d\n", first, blended)
		_ = dev.Close()
	}
}
