//go:build ignore

// Raw public complex-text and emoji audit; no application guards.
// tools/testenv.sh go run -tags font_complex tools/probes/font_complex_0238.go
// tools/testenv.sh go run -tags font_emoji tools/probes/font_complex_0238.go
// tools/testenv.sh env GOFRAMEWORK_DESKTOP_UI_SYSTEM_FONTS=0 go run -tags font_emoji tools/probes/font_complex_0238.go
package main

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"unicode/utf8"

	"github.com/go-text/typesetting/di"
	gofont "github.com/go-text/typesetting/font"
	"github.com/go-text/typesetting/language"
	"github.com/go-text/typesetting/shaping"
	"golang.org/x/image/math/fixed"

	"goframework.io/desktop-ui/style"
	"goframework.io/graphics-paint2d"
)

func imageOf(f *style.Font, s string, tint paint2d.Color) *paint2d.Image {
	im := paint2d.NewImage(700, 110)
	f.Draw(paint2d.NewContext(im), s, paint2d.Pt(12, 12), tint)
	return im
}

func hash(im *paint2d.Image) string {
	h := sha256.Sum256(im.Pix)
	return fmt.Sprintf("%x", h[:8])
}

func coloured(im *paint2d.Image) int {
	n := 0
	for y := 0; y < im.Height; y++ {
		for x := 0; x < im.Width; x++ {
			c := im.NRGBAAt(x, y)
			if c.A > 20 && (c.R != c.G || c.G != c.B) {
				n++
			}
		}
	}
	return n
}

func visible(im *paint2d.Image) int {
	n := 0
	for y := 0; y < im.Height; y++ {
		for x := 0; x < im.Width; x++ {
			if im.NRGBAAt(x, y).A > 20 {
				n++
			}
		}
	}
	return n
}

// Independently verifies the installed source font covers the text and that
// its own OpenType tables produce joining/reordering. No toolkit internals.
func sourceCoverage(family, text string) {
	if !style.FontInstalled(family) {
		return
	}
	out, err := exec.Command("fc-match", "-f", "%{file}", family).Output()
	if err != nil {
		fmt.Println("sourceCoverage skipped: fc-match unavailable")
		return
	}
	src, err := os.ReadFile(strings.TrimSpace(string(out)))
	if err != nil {
		panic(err)
	}
	face, err := gofont.ParseTTF(bytes.NewReader(src))
	if err != nil {
		fmt.Printf("sourceCoverage %q skipped: %v\n", family, err)
		return
	}
	var nominal []gofont.GID
	for _, r := range text {
		gid, _ := face.NominalGlyph(r)
		nominal = append(nominal, gid)
	}
	direction := di.DirectionLTR
	if strings.Contains(family, "Arabic") || strings.Contains(family, "Hebrew") {
		direction = di.DirectionRTL
	}
	in := shaping.Input{Text: []rune(text), RunStart: 0, RunEnd: utf8.RuneCountInString(text), Face: face, Size: fixed.I(24), Direction: direction, Script: language.LookupScript([]rune(text)[0])}
	var sh shaping.HarfbuzzShaper
	shaped := sh.Shape(in)
	var gids []gofont.GID
	for _, g := range shaped.Glyphs {
		gids = append(gids, g.GlyphID)
	}
	fmt.Printf("sourceCoverage family=%q text=%q nominal=%v shaped=%v advance=%g file=%q\n", family, text, nominal, gids, float32(shaped.Advance)/64, strings.TrimSpace(string(out)))
}

func main() {
	f := style.BakeFont(24, paint2d.White)
	fmt.Printf("system_fonts=%q primary=%q\n", os.Getenv(style.SystemFontsEnv), f.Family)
	for _, c := range []struct{ name, family, a, b string }{
		{"Arabic", "Noto Sans Arabic", "سلام", "كتاب"},
		{"Hebrew", "Noto Sans Hebrew", "שלום", "חיים"},
		{"Devanagari", "Noto Sans Devanagari", "कि", "का"},
		{"CJK", "Noto Sans CJK SC", "山川", "海河"},
	} {
		style.SetFallbackFamilies(c.family)
		sourceCoverage(c.family, c.a)
		box := imageOf(f, "\ue000", paint2d.White)
		boxes := 0
		for _, r := range c.a {
			if bytes.Equal(box.Pix, imageOf(f, string(r), paint2d.White).Pix) {
				boxes++
			}
		}
		a, b := imageOf(f, c.a, paint2d.White), imageOf(f, c.b, paint2d.White)
		fmt.Printf("%s provider=%q installed=%t distinct=%t boxRunes=%d/%d widthA=%g widthB=%g hashes=%s/%s carets=", c.name, c.family, style.FontInstalled(c.family), !bytes.Equal(a.Pix, b.Pix), boxes, utf8.RuneCountInString(c.a), f.Advance(c.a), f.Advance(c.b), hash(a), hash(b))
		for i := 0; i <= utf8.RuneCountInString(c.a); i++ {
			fmt.Printf("%g,", f.CaretX(c.a, i))
		}
		fmt.Println()
		primary := style.BakeFamily(c.family, style.WeightRegular, 24, paint2d.White)
		fmt.Printf("explicit requested=%q actual=%q distinct=%t\n", c.family, primary.Family, !bytes.Equal(imageOf(primary, c.a, paint2d.White).Pix, imageOf(primary, c.b, paint2d.White).Pix))
	}
	style.SetFallbackFamilies()
	defer style.SetFallbackFamilies()
	// A new glyph republishes an emoji sheet; old cached runs still refer
	// to the preceding snapshot and must keep its colour-sheet identity.
	warmFont := style.BakeFont(31, paint2d.White)
	warmBefore := imageOf(warmFont, "👀", paint2d.White)
	warmBeforeRed := imageOf(warmFont, "👀", paint2d.RGB(1, 0, 0))
	imageOf(warmFont, "😀🇯🇵", paint2d.White)
	warmWhite, warmRed := imageOf(warmFont, "👀", paint2d.White), imageOf(warmFont, "👀", paint2d.RGB(1, 0, 0))
	fmt.Printf("emojiAtlasGrowth beforeHash=%s afterHash=%s beforeUntinted=%t stable=%t afterUntinted=%t; want true/true/true under font_emoji\n", hash(warmBefore), hash(warmWhite), bytes.Equal(warmBefore.Pix, warmBeforeRed.Pix), bytes.Equal(warmBefore.Pix, warmWhite.Pix), bytes.Equal(warmWhite.Pix, warmRed.Pix))
	for _, family := range []string{"Liberation Sans", "Liberation Serif", "Noto Sans"} {
		primary := style.BakeFamily(family, style.WeightRegular, 24, paint2d.White)
		const rtl = "שלום"
		box := imageOf(primary, "\ue000", paint2d.White)
		if bytes.Equal(box.Pix, imageOf(primary, "ש", paint2d.White).Pix) {
			continue
		}
		fmt.Printf("primaryRTL family=%q text=%q width=%g caret0=%g caretEnd=%g indexLeft=%d indexRight=%d spans=%v\n", primary.Family, rtl, primary.Advance(rtl), primary.CaretX(rtl, 0), primary.CaretX(rtl, utf8.RuneCountInString(rtl)), primary.IndexAt(rtl, 0), primary.IndexAt(rtl, primary.Advance(rtl)), primary.SelectionSpans(rtl, 0, 1))
		limit := primary.Advance(rtl) * .85
		fit := primary.Fit(rtl, limit)
		paragraph := "שלום שלום שלום שלום"
		lines := primary.Wrap(paragraph, 60)
		fmt.Printf("primaryRTLfit family=%q limit=%g prefix=%q fit=%q fitWidth=%g wrapWidth60=%q lineWidths=", primary.Family, limit, primary.Prefix(rtl, limit), fit, primary.Advance(fit), lines)
		for _, line := range lines {
			fmt.Printf("%g,", primary.Advance(line))
		}
		fmt.Println()
	}
	sourceCoverage("Noto Color Emoji", "👍🏽")
	for _, s := range []string{"👀", "😀", "👩‍👩‍👧‍👦", "👍🏽", "🇯🇵", "A👀B"} {
		white, red := imageOf(f, s, paint2d.White), imageOf(f, s, paint2d.RGB(1, 0, 0))
		fmt.Printf("emoji=%q visiblePixels=%d colourPixels=%d sameWhenTinted=%t width=%g hash=%s\n", s, visible(white), coloured(white), bytes.Equal(white.Pix, red.Pix), f.Advance(s), hash(white))
	}
	// An outline provider for one rune of a sequence must not split the
	// composed emoji across faces. Compare the default to emoji-only lending.
	style.SetFallbackFamilies("audit-outline-provider-unavailable")
	for _, s := range []string{"👩‍👩‍👧‍👦", "👍🏽"} {
		im := imageOf(f, s, paint2d.White)
		fmt.Printf("emojiOnly=%q visiblePixels=%d colourPixels=%d width=%g hash=%s\n", s, visible(im), coloured(im), f.Advance(s), hash(im))
	}
	style.SetFallbackFamilies()
	eyesWhite, eyesRed := imageOf(f, "👀", paint2d.White), imageOf(f, "👀", paint2d.RGB(1, 0, 0))
	fmt.Printf("emojiWarmAtlas eyesColourPixels=%d sameWhenTinted=%t whiteHash=%s redHash=%s; want true under font_emoji\n", coloured(eyesWhite), bytes.Equal(eyesWhite.Pix, eyesRed.Pix), hash(eyesWhite), hash(eyesRed))
	for _, s := range []string{"abc שלום abc", "سلام 2026 Music", "AV山川 music 河 — 2026"} {
		fmt.Printf("mixed=%q width=%g spans=%v directionRuns=%v\n", s, f.Advance(s), f.SelectionSpans(s, 0, utf8.RuneCountInString(s)), style.DirectionRuns(s))
	}
}
