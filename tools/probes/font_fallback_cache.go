//go:build ignore

// Public runtime fallback-provider change probe. Requires an installed CJK face.
// tools/testenv.sh go run -tags font_fallback tools/probes/font_fallback_cache.go
package main

import (
	"bytes"
	"fmt"

	"goframework.io/desktop-ui/style"
	"goframework.io/graphics-paint2d"
)

func picture(f *style.Font, s string) *paint2d.Image {
	im := paint2d.NewImage(300, 70)
	f.Draw(paint2d.NewContext(im), s, paint2d.Pt(5, 5), paint2d.White)
	return im
}

func differs(f *style.Font, a, b string) bool {
	return !bytes.Equal(picture(f, a).Pix, picture(f, b).Pix)
}

func main() {
	defer style.SetFallbackFamilies()
	provider := ""
	for _, name := range style.FallbackFamilies() {
		if style.FontInstalled(name) {
			f := style.BakeFamily(name, style.WeightRegular, 16, paint2d.White)
			if differs(f, "峠", "語") {
				provider = name
				break
			}
		}
	}
	if provider == "" {
		fmt.Println("SKIP: no installed covering CJK provider")
		return
	}
	style.SetFallbackFamilies("raw-provider-does-not-exist")
	f := style.BakeFamily(style.FamilyUI, style.WeightRegular, 16, paint2d.White)
	before := differs(f, "Song 峠", "Song 語")
	style.SetFallbackFamilies(provider)
	after := differs(f, "Song 峠", "Song 語")
	// A new text still needs the already-missed runes; it avoids merely hitting
	// the original whole-string shape-cache entry.
	newText := differs(f, "New 峠", "New 語")
	freshSize := style.BakeFamily(style.FamilyUI, style.WeightRegular, 17, paint2d.White)
	fresh := differs(freshSize, "New 峠", "New 語")
	fmt.Printf("missing->covering provider=%q listNow=%q beforeDistinct=%t sameTextDistinct=%t newTextDistinct=%t freshSizeDistinct=%t; want false/true/true/true\n", provider, style.FallbackFamilies(), before, after, newText, fresh)

	style.SetFallbackFamilies(provider)
	g := style.BakeFamily(style.FamilyUI, style.WeightRegular, 21, paint2d.White)
	old := picture(g, "Song 池湖")
	beforeWidth := g.Advance("Song 池湖")
	style.SetFallbackFamilies("raw-provider-does-not-exist")
	same := bytes.Equal(old.Pix, picture(g, "Song 池湖").Pix)
	afterWidth := g.Advance("Song 池湖")
	next := style.BakeFamily(style.FamilyUI, style.WeightRegular, 22, paint2d.White)
	fmt.Printf("covering->missing listNow=%q samePixels=%t width=%g->%g reusedRunesDistinct=%t freshSizeDistinct=%t; want changed pixels/measurement and false/false\n", style.FallbackFamilies(), same, beforeWidth, afterWidth, differs(g, "New 池", "New 湖"), differs(next, "New 池", "New 湖"))
}
