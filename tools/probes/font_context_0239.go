//go:build ignore

// Public emoji-presentation occurrence and shaped-prefix diagnostics.
// tools/testenv.sh go run -tags font_emoji tools/probes/font_context_0239.go
package main

import (
	"bytes"
	"fmt"
	"goframework.io/desktop-ui/style"
	"goframework.io/graphics-paint2d"
)

func picture(f *style.Font, s string, col paint2d.Color) *paint2d.Image {
	im := paint2d.NewImage(500, 110)
	f.Draw(paint2d.NewContext(im), s, paint2d.Pt(12, 30), col)
	return im
}
func colours(im *paint2d.Image, minx, maxx int) int {
	n := 0
	for y := 0; y < im.Height; y++ {
		for x := minx; x < maxx && x < im.Width; x++ {
			c := im.NRGBAAt(x, y)
			if c.A > 20 && (c.R != c.G || c.G != c.B) {
				n++
			}
		}
	}
	return n
}
func main() {
	style.SetFallbackFamilies("Noto Sans Symbols 2")
	defer style.SetFallbackFamilies()
	f := style.BakeFont(24, paint2d.White)
	fmt.Printf("outlineProviderInstalled=%t colourProviderInstalled=%t\n", style.FontInstalled("Noto Sans Symbols 2"), style.FontInstalled("Noto Color Emoji"))
	for _, s := range []string{"❤", "❤️", "❤︎", "❤ ❤️", "❤️ ❤", "❤︎ ❤️", "👍", "👍︎", "👍🏽"} {
		white, red := picture(f, s, paint2d.White), picture(f, s, paint2d.RGB(1, 0, 0))
		fmt.Printf("text=%q width=%g colouredPixels=%d first24pxColoured=%d tinted=%t\n", s, f.Advance(s), colours(white, 0, 500), colours(white, 12, 36), !bytes.Equal(white.Pix, red.Pix))
	}
	fmt.Printf("heartMixed actual=%g separateTextThenEmoji=%g separateEmojiThenText=%g\n", f.Advance("❤ ❤️"), f.Advance("❤")+f.Advance(" ")+f.Advance("❤️"), f.Advance("❤️")+f.Advance(" ")+f.Advance("❤"))
	for _, c := range []struct{ family, text string }{{"Noto Sans Arabic", "لا"}, {"Noto Sans Devanagari", "क्षि"}, {"Noto Sans Devanagari", "नमस्ते"}, {"Noto Sans", "fi"}, {"Noto Sans", "ffi"}} {
		style.SetFallbackFamilies(c.family)
		f := style.BakeFont(24, paint2d.White)
		if c.family == "Noto Sans" {
			f = style.BakeFamily(c.family, style.WeightRegular, 24, paint2d.White)
		}
		w := f.Advance(c.text)
		for limit := w / 40; limit < w; limit += w / 40 {
			p := f.Prefix(c.text, limit)
			if got := f.Advance(p); got > limit+.032 {
				fmt.Printf("prefixOverrun provider=%q text=%q fullWidth=%g limit=%g prefix=%q prefixWidth=%g\n", c.family, c.text, w, limit, p, got)
				break
			}
		}
		for limit := f.Advance("…") + 1; limit < w; limit += w / 40 {
			p := f.Fit(c.text, limit)
			if got := f.Advance(p); got > limit+.032 {
				fmt.Printf("fitOverrun provider=%q text=%q limit=%g fit=%q fitWidth=%g\n", c.family, c.text, limit, p, got)
				break
			}
		}
	}
}
