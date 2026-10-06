//go:build ignore

// Public skin API sampling probe. Run from the project root with:
// tools/testenv.sh go run ./tools/probes/skin_sampling.go
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"testing/fstest"

	"github.com/codemodify/paintengine2d"
	"github.com/codemodify/uitoolkit/style"
)

func main() {
	for _, pixelated := range []bool{true, false} {
		id := fmt.Sprintf("sampling-public-%t", pixelated)
		asset := func(scale int) []byte {
			im := paintengine2d.NewImage(8*scale, 8*scale)
			for y := range im.Height {
				for x := range im.Width {
					c := paintengine2d.Black
					if (x/scale+y/scale)%2 == 0 {
						c = paintengine2d.RGB(0, 1, 0)
						if scale == 2 {
							c = paintengine2d.RGB(0, 0, 1)
						}
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
		doc := fmt.Sprintf(`{"skin":1,"label":"Sampling probe","base":"dark","design":{"pixelated":%t},"sheets":{"chrome":{"1x":"one.png","2x":"two.png"}},"sprites":{"face":{"at":[0,0,8,8]}}}`, pixelated)
		fsys := fstest.MapFS{"skin.json": {Data: []byte(doc)}, "one.png": {Data: asset(1)}, "two.png": {Data: asset(2)}}
		if err := style.RegisterSkinFS(fsys, id); err != nil {
			panic(err)
		}
		p, _ := style.LoadTheme(id)
		for _, scale := range []float32{1, 1.75, 2} {
			im := paintengine2d.NewImage(20, 20)
			style.DrawSkinSprite(style.WithScale(p.Look(), scale), paintengine2d.NewContext(im), paintengine2d.XYWH(.25, .25, 8*scale, 8*scale), "face", paintengine2d.Color{})
			green, blue, blended := 0, 0, 0
			for y := 1; y < int(8*scale)-1; y++ {
				for x := 1; x < int(8*scale)-1; x++ {
					r, g, b, a := im.PremulAt(x, y)
					switch {
					case r == 0 && g == 255 && b == 0 && a == 255:
						green++
					case r == 0 && g == 0 && b == 255 && a == 255:
						blue++
					case r != 0 || (g != 0 && g != 255) || (b != 0 && b != 255) || a != 255:
						blended++
					}
				}
			}
			fmt.Printf("public pixelated=%t scale=%g 1x_green_pixels=%d 2x_blue_pixels=%d interior_blended_pixels=%d\n", pixelated, scale, green, blue, blended)
		}
	}
	for _, id := range []string{"winamp", "winamp-silver", "winamp-classified", "dosamp", "windows-98", "winamp-amber", "calculator", "sony-cdx-mp3", "ti-82", "radar-amp", "quake-3-arena"} {
		inspectPack(id)
	}
}

func inspectPack(id string) {
	dir := filepath.Join("internal", "skins", "packs", id)
	sk, err := style.LoadSkinFS(os.DirFS(dir), "sampling-"+id)
	if err != nil {
		panic(err)
	}
	sp := sk.Sprites["key.play"]
	if sp == nil {
		panic(id + " has no key.play")
	}
	cut := func(scale float32) image.Image {
		f, err := os.Open(filepath.Join(dir, sp.Sheet.Files[scale]))
		if err != nil {
			panic(err)
		}
		defer f.Close()
		im, err := png.Decode(f)
		if err != nil {
			panic(err)
		}
		r := image.Rect(int(sp.X*scale), int(sp.Y*scale), int((sp.X+sp.W)*scale), int((sp.Y+sp.H)*scale))
		return im.(interface {
			SubImage(image.Rectangle) image.Image
		}).SubImage(r)
	}
	one, two := cut(1), cut(2)
	different := 0
	for y := range two.Bounds().Dy() {
		for x := range two.Bounds().Dx() {
			a := one.At(one.Bounds().Min.X+x/2, one.Bounds().Min.Y+y/2)
			b := two.At(two.Bounds().Min.X+x, two.Bounds().Min.Y+y)
			ar, ag, ab, aa := a.RGBA()
			br, bg, bb, ba := b.RGBA()
			if ar != br || ag != bg || ab != bb || aa != ba {
				different++
			}
		}
	}
	if err := style.RegisterSkinFS(os.DirFS(dir), "sampling-"+id); err != nil {
		panic(err)
	}
	p, _ := style.LoadTheme("sampling-" + id)
	const scale float32 = 1.75
	lk := style.WithScale(p.Look(), scale)
	w, h := sp.W*scale, sp.H*scale
	im := paintengine2d.NewImage(int(math.Ceil(float64(w))), int(math.Ceil(float64(h))))
	style.DrawSkinSprite(lk, paintengine2d.NewContext(im), paintengine2d.XYWH(0, 0, w, h), "key.play", paintengine2d.Color{})
	colors := map[[4]uint32]bool{}
	for y := one.Bounds().Min.Y; y < one.Bounds().Max.Y; y++ {
		for x := one.Bounds().Min.X; x < one.Bounds().Max.X; x++ {
			r, g, b, a := one.At(x, y).RGBA()
			colors[[4]uint32{r >> 8, g >> 8, b >> 8, a >> 8}] = true
		}
	}
	newColors := 0
	for y := range im.Height {
		for x := range im.Width {
			r, g, b, a := im.PremulAt(x, y)
			if a == 255 && !colors[[4]uint32{uint32(r), uint32(g), uint32(b), uint32(a)}] {
				newColors++
			}
		}
	}
	caption := sk.Text["caption.main"]
	font := style.WeightFontOf(lk, style.WeightBold)
	if caption != nil && caption.Size > 0 {
		font = style.BakeFamily(font.Family, style.WeightBold, caption.Size*scale/sk.Design.Scale, paintengine2d.White)
	}
	out, _ := json.Marshal(map[string]any{"pack": id, "play_sheet_pixelated": sp.Sheet.Pixelated, "2x_pixels_different_from_exact_1x_double": different, "opaque_1_75_pixels_outside_1x_source_palette": newColors, "caption_font_family": font.Family, "caption_font_size": font.Size, "caption_font_uses_outlines": font.Outline})
	fmt.Println(string(out))
}
