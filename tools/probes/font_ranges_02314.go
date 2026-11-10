//go:build ignore

// Public mixed supplementary-text and BMP-presentation diagnostic.
// tools/testenv.sh go run -p=2 -tags font_emoji tools/probes/font_ranges_02314.go
package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/go-text/typesetting/di"
	gofont "github.com/go-text/typesetting/font"
	"github.com/go-text/typesetting/language"
	"github.com/go-text/typesetting/shaping"
	"goframework.io/desktop-ui/style"
	"goframework.io/graphics-paint2d"
	"golang.org/x/image/math/fixed"
)

func image(f *style.Font, text string, tint paint2d.Color) *paint2d.Image {
	im := paint2d.NewImage(500, 140)
	f.Draw(paint2d.NewContext(im), text, paint2d.Pt(20, 25), tint)
	return im
}

func source(family string) (*gofont.Face, error) {
	b, err := exec.Command("fc-match", "-f", "%{file}\n%{index}", family).Output()
	if err != nil {
		return nil, err
	}
	fields := strings.Split(strings.TrimSpace(string(b)), "\n")
	if len(fields) != 2 {
		return nil, fmt.Errorf("unexpected fontconfig source %q", b)
	}
	index, err := strconv.Atoi(fields[1])
	if err != nil {
		return nil, err
	}
	b, err = os.ReadFile(fields[0])
	if err != nil {
		return nil, err
	}
	faces, err := gofont.ParseTTC(bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	if index < 0 || index >= len(faces) {
		return nil, fmt.Errorf("font index %d outside %d", index, len(faces))
	}
	return faces[index], nil
}

func advance(face *gofont.Face, text string, size float32) (float32, uint32, error) {
	runes := []rune(text)
	for _, r := range runes {
		if gid, ok := face.NominalGlyph(r); !ok || gid == 0 {
			return 0, 0, fmt.Errorf("source does not cover U+%04X", r)
		}
	}
	var sh shaping.HarfbuzzShaper
	out := sh.Shape(shaping.Input{Text: runes, RunStart: 0, RunEnd: len(runes), Face: face, Size: fixed.Int26_6(size * 64), Direction: di.DirectionLTR, Script: language.LookupScript(runes[0])})
	if len(out.Glyphs) != 1 {
		return 0, 0, fmt.Errorf("source produced %d glyphs", len(out.Glyphs))
	}
	return float32(out.Advance) / 64, uint32(out.Glyphs[0].GlyphID), nil
}

func main() {
	style.SetFallbackFamilies()
	defer style.SetFallbackFamilies()
	f := style.BakeFont(24, paint2d.White)
	for _, c := range []struct{ family, text string }{
		{"Noto Sans CJK SC", "𠮷"},
		{"Noto Sans Symbols 2", "\U0001fb00"},
		{"Noto Sans Symbols", "\U0001f701"},
		{"Noto Sans Symbols 2", "\U0001fa00"},
		{"Noto Sans CJK SC", "\U0001f130"},
	} {
		if !style.FontInstalled(c.family) {
			fmt.Printf("SKIP family%q unavailable\n", c.family)
			continue
		}
		face, err := source(c.family)
		if err != nil {
			panic(err)
		}
		width, gid, err := advance(face, c.text, 24)
		if err != nil {
			fmt.Printf("SKIP %v\n", err)
			continue
		}
		lender := style.BakeFamily(c.family, style.WeightRegular, 24, paint2d.White)
		for _, before := range []bool{true, false} {
			text := "🎬" + c.text
			if !before {
				text = c.text + "🎬"
			}
			actual := image(f, text, paint2d.White)
			separate := paint2d.NewImage(500, 140)
			ctx := paint2d.NewContext(separate)
			explicit := paint2d.NewImage(500, 140)
			sourceCtx := paint2d.NewContext(explicit)
			if before {
				f.Draw(ctx, "🎬", paint2d.Pt(20, 25), paint2d.White)
				f.Draw(ctx, c.text, paint2d.Pt(20+f.Advance("🎬"), 25), paint2d.White)
				f.Draw(sourceCtx, "🎬", paint2d.Pt(20, 25), paint2d.White)
				lender.Draw(sourceCtx, c.text, paint2d.Pt(20+f.Advance("🎬"), 25+f.Ascent-lender.Ascent), paint2d.White)
			} else {
				f.Draw(ctx, c.text, paint2d.Pt(20, 25), paint2d.White)
				f.Draw(ctx, "🎬", paint2d.Pt(20+width, 25), paint2d.White)
				lender.Draw(sourceCtx, c.text, paint2d.Pt(20, 25+f.Ascent-lender.Ascent), paint2d.White)
				f.Draw(sourceCtx, "🎬", paint2d.Pt(20+width, 25), paint2d.White)
			}
			fmt.Printf("MIX text=%+q family=%q sourceGID=%d sourceTextWidth=%g explicitSourceWidth=%g standalone=%g actual=%g separate=%g pixelsEqualSeparate=%t pixelsEqualExplicitSource=%t\n", text, c.family, gid, width, lender.Advance(c.text), f.Advance(c.text), f.Advance(text), f.Advance("🎬")+width, bytes.Equal(actual.Pix, separate.Pix), bytes.Equal(actual.Pix, explicit.Pix))
		}
	}
	emoji, err := source("Noto Color Emoji")
	if err != nil {
		panic(err)
	}
	emoji.SetPpem(24, 24)
	for _, text := range []string{"⌚", "⚡", "⭐"} {
		width, gid, err := advance(emoji, text, 24)
		if err != nil {
			panic(err)
		}
		bitmap, bitmapOK := emoji.GlyphDataBitmap(gofont.GID(gid))
		plain, requested := image(f, text, paint2d.White), image(f, text+"\ufe0f", paint2d.White)
		fmt.Printf("BMP text=%q sourceGID%d sourceWidth%g sourceBitmap%t bitmapBytes%d bareWidth=%g requestedWidth=%g bareUntinted=%t requestedUntinted=%t samePixels=%t\n", text, gid, width, bitmapOK, len(bitmap.Data), f.Advance(text), f.Advance(text+"\ufe0f"), bytes.Equal(plain.Pix, image(f, text, paint2d.RGB(1, 0, 0)).Pix), bytes.Equal(requested.Pix, image(f, text+"\ufe0f", paint2d.RGB(1, 0, 0)).Pix), bytes.Equal(plain.Pix, requested.Pix))
	}
}
