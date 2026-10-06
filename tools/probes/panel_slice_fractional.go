//go:build ignore

// This public-API probe records the fractional slice regression in UIToolKit.
// Run with tools/testenv.sh go run ./tools/probes/panel_slice_fractional.go.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"testing/fstest"

	"github.com/codemodify/paintengine2d"
	"github.com/codemodify/uitoolkit/style"
)

func main() {
	for _, pixelated := range []bool{true, false} {
		id := fmt.Sprintf("panel-slice-public-%t", pixelated)
		sheet := func(scale int) []byte {
			im := paintengine2d.NewImage(273*scale, 211*scale)
			for y := 0; y < im.Height; y++ {
				for x := 0; x < im.Width; x++ {
					c := paintengine2d.RGB(0, 0, 1)
					if y >= (211-38)*scale {
						c = paintengine2d.RGB(1, 0, 0)
					}
					im.SetColor(x, y, c)
				}
			}
			var b bytes.Buffer
			if err := im.WritePNG(&b); err != nil {
				panic(err)
			}
			return b.Bytes()
		}
		doc := fmt.Sprintf(`{"skin":1,"label":"Public panel slice probe","base":"dark","design":{"pixelated":%t},"sheets":{"chrome":{"1x":"one.png","2x":"two.png"}},"sprites":{"face":{"at":[0,0,273,211],"slice":[0,148,38,124]}},"layouts":{"panel":{"size":[273,211],"art":"face","slots":{"footer":{"at":[0,0,0,38],"stretchX":true,"fromBottom":true}}}}}`, pixelated)
		fsys := fstest.MapFS{"skin.json": {Data: []byte(doc)}, "one.png": {Data: sheet(1)}, "two.png": {Data: sheet(2)}}
		if err := style.RegisterSkinFS(fsys, id); err != nil {
			panic(err)
		}
		p, ok := style.LoadTheme(id)
		if !ok {
			panic("registered skin did not load")
		}
		for _, scale := range []float32{1, 1.25, 1.5, 1.75, 2} {
			for _, size := range [][2]float32{{273, 211}, {420, 320}} {
				w, h := size[0]*scale, size[1]*scale
				im := paintengine2d.NewImage(int(math.Ceil(float64(w))), int(math.Ceil(float64(h))))
				lk := style.WithScale(p.Look(), scale)
				if !style.DrawSkinLayout(lk, paintengine2d.NewContext(im), paintengine2d.XYWH(0, 0, w, h), "panel") {
					panic("layout did not draw")
				}
				x := int(w / 2)
				red, first, last := 0, -1, -1
				for y := 0; y < im.Height; y++ {
					r, g, b, a := im.PremulAt(x, y)
					if r > 240 && g < 10 && b < 10 && a > 240 {
						red++
						if first < 0 {
							first = y
						}
						last = y
					}
				}
				r, ok := style.SkinSlotRect(lk, "panel", "footer", paintengine2d.XYWH(0, 0, w, h))
				if !ok {
					panic("footer slot did not resolve")
				}
				b, _ := json.Marshal(map[string]any{"pixelated": pixelated, "scale": scale, "logical_size": size, "footer_red_pixels": red, "footer_first_y": first, "footer_last_y": last, "design_footer_scaled": 38 * scale, "slot_footer_y": r.Min.Y, "slot_footer_height": r.Dy()})
				fmt.Println(string(b))
			}
		}
	}
}
