//go:build ignore

// Public, CPU-only reproduction of an explicit proportional field font being
// ignored for the empty placeholder. Run: go run tools/probes/field_placeholder_font.go
package main

import (
	"fmt"
	"github.com/codemodify/paintengine2d"
	"github.com/codemodify/uitoolkit/style"
)

func draw(look style.LookAndFeel, face *style.Font, placeholder bool) *paintengine2d.Image {
	im := paintengine2d.NewImage(350, 50)
	ctx := paintengine2d.NewContext(im)
	ctx.Clear(paintengine2d.Transparent)
	text, hint := "Search Library", ""
	if placeholder {
		text, hint = "", text
	}
	look.DrawTextField(ctx, paintengine2d.XYWH(4, 4, 342, 42), style.StateFrameless, text, hint, 0, 0, 0, false, 0, face)
	return im
}
func equalAlpha(a, b *paintengine2d.Image) bool {
	for at := 3; at < len(a.Pix); at += 4 {
		if a.Pix[at] != b.Pix[at] {
			return false
		}
	}
	return true
}
func inkBox(im *paintengine2d.Image) (int, int) {
	minX, minY, maxX, maxY := im.Width, im.Height, -1, -1
	for y := 0; y < im.Height; y++ {
		for x := 0; x < im.Width; x++ {
			if im.Pix[(y*im.Width+x)*4+3] > 0 {
				minX = min(minX, x)
				minY = min(minY, y)
				maxX = max(maxX, x)
				maxY = max(maxY, y)
			}
		}
	}
	return maxX - minX + 1, maxY - minY + 1
}
func main() {
	look := style.DarkLook()
	f11 := style.BakeFamily(look.Font().Family, style.WeightRegular, 11, paintengine2d.White)
	f16 := style.BakeFamily(look.Font().Family, style.WeightRegular, 16, paintengine2d.White)
	typed11, typed16 := draw(look, f11, false), draw(look, f16, false)
	hint11, hint16 := draw(look, f11, true), draw(look, f16, true)
	if equalAlpha(typed11, typed16) {
		panic("fixture did not resolve two distinct proportional font sizes")
	}
	if !equalAlpha(hint11, hint16) {
		panic("proportional placeholder font size is honored; issue not reproduced")
	}
	width11, height11 := inkBox(typed11)
	width16, height16 := inkBox(typed16)
	hintWidth, hintHeight := inkBox(hint11)
	mono11 := style.BakeFamily(look.MonoFont().Family, style.WeightRegular, 11, paintengine2d.White)
	mono16 := style.BakeFamily(look.MonoFont().Family, style.WeightRegular, 16, paintengine2d.White)
	if equalAlpha(draw(look, mono11, true), draw(look, mono16, true)) {
		panic("mono control case also ignored the explicit size")
	}
	fmt.Printf("lookDefault=%g proportionalFamily=%q typed11=%dx%d typed16=%dx%d placeholder11=%dx%d placeholder11Equals16=true typed11Equals16=false monoPlaceholder11Equals16=false\n", look.Font().Size, f11.Family, width11, height11, width16, height16, hintWidth, hintHeight)
}
