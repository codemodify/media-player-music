//go:build ignore

// Raw stock font/list rendering of synthetic mixed-script song titles.
// tools/testenv.sh go run tools/probes/unicode_song_titles.go
// tools/testenv.sh go run -tags font_fallback tools/probes/unicode_song_titles.go
// tools/testenv.sh env GOFRAMEWORK_DESKTOP_UI_SYSTEM_FONTS=0 go run -tags font_fallback tools/probes/unicode_song_titles.go
// Equal pictures for distinct titles demonstrate missing glyph coverage,
// independently of the app's own bitmap fonts or playlist adapters.
//
// Baseline: from a separate temporary directory, write go.mod containing
// "module unicode-baseline", "go 1.22.2" and
// "require github.com/codemodify/uitoolkit v0.23.5". Then run the repository's
// testenv.sh with: go run -mod=mod /absolute/repo/tools/probes/unicode_song_titles.go.
// This pins the older toolkit and its rendering dependency without changing
// this repository's go.mod. Both versions must be run under the private bus.
package main

import (
	"bytes"
	"crypto/sha256"
	"fmt"

	"goframework.io/desktop-ui-widgets"
	"goframework.io/desktop-ui/style"
	"goframework.io/graphics-paint2d"
)

func row(lk style.LookAndFeel, title string, scale float32) (*paint2d.Image, string) {
	w, h := int(400*scale), int(40*scale)
	im := paint2d.NewImage(w, h)
	l := widgets.NewListView(1, func(int) string { return title }, nil)
	l.SetLook(lk)
	l.Arrange(paint2d.XYWH(0, 0, float32(w), float32(h)))
	l.Paint(paint2d.NewContext(im))
	return im, l.AccessibleItems()[0].Name
}

func fontImage(f *style.Font, title string) *paint2d.Image {
	im := paint2d.NewImage(400, 40)
	f.Draw(paint2d.NewContext(im), title, paint2d.Pt(4, 25), paint2d.Color{})
	return im
}

func main() {
	first, second := "Song 山川", "Song 海河"
	for _, scale := range []float32{1, 1.75} {
		lk := style.WithScale(style.DarkLook(), scale)
		a, nameA := row(lk, first, scale)
		b, nameB := row(lk, second, scale)
		latinA, _ := row(lk, "Song River", scale)
		latinB, _ := row(lk, "Song Ocean", scale)
		ha, hb := sha256.Sum256(a.Pix), sha256.Sum256(b.Pix)
		fmt.Printf("scale=%g family=%q titleA=%q titleB=%q advanceA=%g advanceB=%g viewWidth=%g equalPictures=%t accessibleNamesDiffer=%t latinControlDiffers=%t hashA=%x hashB=%x\n", scale, lk.Font().Family, first, second, lk.Font().Advance(first), lk.Font().Advance(second), 400*scale, bytes.Equal(a.Pix, b.Pix), nameA != nameB, !bytes.Equal(latinA.Pix, latinB.Pix), ha[:8], hb[:8])
	}
	found := false
	for _, family := range []string{"Noto Sans CJK SC", "Noto Sans CJK JP", "WenQuanYi Micro Hei", "Microsoft YaHei", "PingFang SC"} {
		if style.FontInstalled(family) {
			f := style.BakeFamily(family, style.WeightRegular, 16, paint2d.RGB(1, 1, 1))
			fmt.Printf("explicitFontControl family=%q equalPictures=%t\n", f.Family, bytes.Equal(fontImage(f, first).Pix, fontImage(f, second).Pix))
			found = true
			break
		}
	}
	if !found {
		fmt.Println("explicitFontControl skipped: no known CJK family installed")
	}
}
