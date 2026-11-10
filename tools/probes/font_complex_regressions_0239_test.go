//go:build ignore

// Raw public .9 font contracts, using installed source fonts as the oracle.
// tools/testenv.sh go test -race -tags font_emoji ./tools/probes/font_complex_regressions_0239_test.go -v
// For all tests use font_emoji. Complex-only script/RTL checks can use
// -tags font_complex -run 'TestScript|TestRTL|TestMixed'. No app imports.
package fontaudit

import (
	"bytes"
	"fmt"
	"image/png"
	"math"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"unicode/utf8"

	"github.com/go-text/typesetting/di"
	gofont "github.com/go-text/typesetting/font"
	"github.com/go-text/typesetting/language"
	"github.com/go-text/typesetting/shaping"
	"goframework.io/desktop-ui/style"
	"goframework.io/graphics-paint2d"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
)

func picture(f *style.Font, s string, col paint2d.Color) *paint2d.Image {
	im := paint2d.NewImage(900, 200)
	f.Draw(paint2d.NewContext(im), s, paint2d.Pt(20, 30), col)
	return im
}

// Context.DrawGlyphs emits one Blit for each nonempty visible glyph cell.
// Count those public device calls without inspecting private shaped runs or
// mistaking a zero-advance overlay for a correctly composed one-glyph image.
type glyphCountingDevice struct {
	paint2d.Device
	blits int
}

func (d *glyphCountingDevice) Blit(src *paint2d.Image, srcRect, dstRect paint2d.Rect, xform paint2d.Matrix, paint paint2d.Paint, clip paint2d.Clip) {
	d.blits++
	d.Device.Blit(src, srcRect, dstRect, xform, paint, clip)
}

func paintedGlyphCount(f *style.Font, text string) int {
	image := paint2d.NewImage(900, 200)
	device := &glyphCountingDevice{Device: paint2d.NewCPUDevice(image)}
	f.Draw(paint2d.NewContextDevice(device), text, paint2d.Pt(20, 30), paint2d.White)
	return device.blits
}

type glyphCellCaptureDevice struct {
	paint2d.Device
	cells []*paint2d.Image
}

func (d *glyphCellCaptureDevice) Blit(src *paint2d.Image, srcRect, dstRect paint2d.Rect, xform paint2d.Matrix, paint paint2d.Paint, clip paint2d.Clip) {
	cell := paint2d.NewImage(int(srcRect.Dx()), int(srcRect.Dy()))
	cell.CopyFrom(src, srcRect, 0, 0)
	d.cells = append(d.cells, cell)
	d.Device.Blit(src, srcRect, dstRect, xform, paint, clip)
}

func ink(im *paint2d.Image) paint2d.Rect {
	x0, y0, x1, y1 := im.Width, im.Height, -1, -1
	for y := 0; y < im.Height; y++ {
		for x := 0; x < im.Width; x++ {
			if im.NRGBAAt(x, y).A > 40 {
				x0, y0, x1, y1 = min(x0, x), min(y0, y), max(x1, x), max(y1, y)
			}
		}
	}
	if x1 < 0 {
		return paint2d.Rect{}
	}
	return paint2d.XYWH(float32(x0), float32(y0), float32(x1-x0+1), float32(y1-y0+1))
}
func fontBytes(t *testing.T, family string) []byte {
	t.Helper()
	if !style.FontInstalled(family) {
		t.Skipf("installed %s unavailable", family)
	}
	b, e := exec.Command("fc-match", "-f", "%{file}", family).Output()
	if e != nil {
		t.Skip("fontconfig oracle unavailable")
	}
	src, e := os.ReadFile(strings.TrimSpace(string(b)))
	if e != nil {
		t.Fatal(e)
	}
	return src
}
func sourceShape(t *testing.T, src []byte, text string, size float32) shaping.Output {
	t.Helper()
	face, e := gofont.ParseTTF(bytes.NewReader(src))
	if e != nil {
		t.Fatal(e)
	}
	rs := []rune(text)
	script := language.LookupScript(rs[0])
	dir := di.DirectionLTR
	if script == language.Arabic || script == language.Hebrew {
		dir = di.DirectionRTL
	}
	for _, r := range rs {
		if r == '\u200d' || r == '\ufe0e' || r == '\ufe0f' {
			continue
		}
		if gid, ok := face.NominalGlyph(r); !ok || gid == 0 {
			t.Fatalf("source does not cover %q", r)
		}
	}
	in := shaping.Input{Text: rs, RunStart: 0, RunEnd: len(rs), Face: face, Size: fixed.Int26_6(size * 64), Direction: dir, Script: script}
	var sh shaping.HarfbuzzShaper
	return sh.Shape(in)
}
func sourceInk(t *testing.T, src []byte, out shaping.Output, size, baseline float32) *paint2d.Image {
	t.Helper()
	face, e := sfnt.Parse(src)
	if e != nil {
		t.Fatal(e)
	}
	im := paint2d.NewImage(900, 200)
	ctx := paint2d.NewContext(im)
	var pen float32
	fx := func(v fixed.Int26_6) float32 { return float32(v) / 64 }
	for _, g := range out.Glyphs {
		segs, e := face.LoadGlyph(nil, sfnt.GlyphIndex(g.GlyphID), fixed.Int26_6(size*64), nil)
		if e != nil {
			t.Fatal(e)
		}
		p := paint2d.NewPath()
		open := false
		for _, s := range segs {
			switch s.Op {
			case sfnt.SegmentOpMoveTo:
				if open {
					p.Close()
				}
				p.MoveTo(fx(s.Args[0].X), fx(s.Args[0].Y))
				open = true
			case sfnt.SegmentOpLineTo:
				p.LineTo(fx(s.Args[0].X), fx(s.Args[0].Y))
			case sfnt.SegmentOpQuadTo:
				p.QuadTo(fx(s.Args[0].X), fx(s.Args[0].Y), fx(s.Args[1].X), fx(s.Args[1].Y))
			case sfnt.SegmentOpCubeTo:
				p.CubicTo(fx(s.Args[0].X), fx(s.Args[0].Y), fx(s.Args[1].X), fx(s.Args[1].Y), fx(s.Args[2].X), fx(s.Args[2].Y))
			}
		}
		if open {
			p.Close()
		}
		ctx.Save()
		ctx.Translate(20+pen+fx(g.XOffset), 30+baseline-fx(g.YOffset))
		ctx.DrawPath(p, paint2d.Fill(paint2d.White))
		ctx.Restore()
		pen += fx(g.Advance)
	}
	return im
}
func TestScriptBorrowingUsesSourceShapingAndPrimaryBaseline(t *testing.T) {
	if style.DirectionRuns("abc שלום") == nil {
		t.Skip("requires font_complex or font_emoji")
	}
	for _, c := range []struct {
		family string
		texts  []string
	}{
		{"Noto Sans Arabic", []string{"سلام", "لا", "سَلَام"}},
		{"Noto Sans Hebrew", []string{"שלום", "שָׁלוֹם"}},
		{"Noto Sans Devanagari", []string{"कि", "क्षि", "नमस्ते", "क्‍ष"}},
	} {
		t.Run(c.family, func(t *testing.T) {
			src := fontBytes(t, c.family)
			style.SetFallbackFamilies(c.family)
			t.Cleanup(func() { style.SetFallbackFamilies() })
			for _, size := range []float32{16, 24, 28, 42} {
				for _, text := range c.texts {
					f := style.BakeFont(size, paint2d.White)
					out := sourceShape(t, src, text, size)
					want := float32(out.Advance) / 64
					if got := f.Advance(text); math.Abs(float64(got-want)) > .02 {
						t.Fatalf("size%g text%q advance=%g source=%g", size, text, got, want)
					}
					actual, expected := ink(picture(f, text, paint2d.White)), ink(sourceInk(t, src, out, size, f.Ascent))
					if actual.Empty() || expected.Empty() || math.Abs(float64(actual.Min.Y-expected.Min.Y)) > 1 || math.Abs(float64(actual.Max.Y-expected.Max.Y)) > 1 || math.Abs(float64(actual.Min.X-expected.Min.X)) > 1 || math.Abs(float64(actual.Max.X-expected.Max.X)) > 1 {
						t.Fatalf("size%g text%q primarybaseline%g actual=%v source=%v", size, text, f.Ascent, actual, expected)
					}
					t.Logf("size%g text%q advance=%g baseline%g ink=%v source=%v", size, text, want, f.Ascent, actual, expected)
				}
			}
		})
	}
}

func TestConcurrentEmojiFontsUseIndependentSizes(t *testing.T) {
	fontBytes(t, "Noto Color Emoji")
	// Font values document that their shared immutable sheets are safe for
	// concurrent callers. Each painter has an image of its own; providers
	// remain unchanged throughout the test.
	fonts := []*style.Font{style.BakeFont(19, paint2d.White), style.BakeFont(33, paint2d.White)}
	start := make(chan struct{})
	var wg sync.WaitGroup
	for _, f := range fonts {
		wg.Add(1)
		go func(f *style.Font) {
			defer wg.Done()
			<-start
			for i := 0; i < 80; i++ {
				picture(f, fmt.Sprintf("😀 %d", i), paint2d.White)
			}
		}(f)
	}
	close(start)
	wg.Wait()
}

// A separate control for the colour-face state fixed in .10. The original
// mixed emoji/digit test above also exercises the outline scratch buffer and
// remains unchanged, including its race failure when that path is unsafe.
func TestConcurrentColourOnlyFontsUseIndependentSizes(t *testing.T) {
	fontBytes(t, "Noto Color Emoji")
	fonts := []*style.Font{style.BakeFont(19, paint2d.White), style.BakeFont(33, paint2d.White)}
	start := make(chan struct{})
	var wg sync.WaitGroup
	for _, f := range fonts {
		wg.Add(1)
		go func(f *style.Font) {
			defer wg.Done()
			<-start
			for i := 1; i <= 80; i++ {
				picture(f, strings.Repeat("👀", i), paint2d.White)
			}
		}(f)
	}
	close(start)
	wg.Wait()
}
func TestRTLAndClustersRespectCaretAndWidthContracts(t *testing.T) {
	if style.DirectionRuns("abc שלום") == nil {
		t.Skip("requires shaping")
	}
	for _, c := range []struct {
		family, text string
		rtl          bool
	}{
		{"Noto Sans Arabic", "سلام", true}, {"Noto Sans Arabic", "لا", true}, {"Noto Sans Devanagari", "क्षि", false}, {"Noto Sans Devanagari", "नमस्ते", false}, {"Liberation Sans", "שלום", true},
	} {
		t.Run(c.family+"/"+c.text, func(t *testing.T) {
			fontBytes(t, c.family)
			style.SetFallbackFamilies(c.family)
			t.Cleanup(func() { style.SetFallbackFamilies() })
			f := style.BakeFont(24, paint2d.White)
			if c.family == "Liberation Sans" {
				f = style.BakeFamily(c.family, style.WeightRegular, 24, paint2d.White)
			}
			width, n := f.Advance(c.text), utf8.RuneCountInString(c.text)
			if c.rtl && (f.CaretX(c.text, 0) != width || f.CaretX(c.text, n) != 0) {
				t.Fatalf("RTL endpoints=%g/%g want%g/0", f.CaretX(c.text, 0), f.CaretX(c.text, n), width)
			}
			for i := 0; i <= n; i++ {
				x := f.CaretX(c.text, i)
				got := f.IndexAt(c.text, x)
				if math.Abs(float64(f.CaretX(c.text, got)-x)) > .001 {
					t.Fatalf("caret%d x%g click%d x%g", i, x, got, f.CaretX(c.text, got))
				}
			}
			sp := f.SelectionSpans(c.text, 0, n)
			if len(sp) != 1 || math.Abs(float64(sp[0][0])) > .001 || math.Abs(float64(sp[0][1]-width)) > .001 {
				t.Fatalf("whole selection=%v width=%g", sp, width)
			}
			for limit := width / 40; limit < width; limit += width / 40 {
				prefix := f.Prefix(c.text, limit)
				if !strings.HasPrefix(c.text, prefix) || f.Advance(prefix) > limit+.032 {
					t.Fatalf("Prefix(%q,%g)=%q width%g", c.text, limit, prefix, f.Advance(prefix))
				}
				if limit >= f.Advance("…")+1 {
					fit := f.Fit(c.text, limit)
					if f.Advance(fit) > limit+.032 {
						t.Fatalf("Fit(%q,%g)=%q width%g", c.text, limit, fit, f.Advance(fit))
					}
				}
			}
			para := strings.Repeat(c.text+" ", 4)
			limit := width + 5
			for _, line := range f.Wrap(para, limit) {
				if f.Advance(line) > limit+.032 {
					t.Fatalf("Wrap(%g) line%q width%g", limit, line, f.Advance(line))
				}
			}
		})
	}
}
func TestMixedDirectionSelectionAndHitTesting(t *testing.T) {
	if style.DirectionRuns("abc שלום") == nil {
		t.Skip("requires shaping")
	}
	fontBytes(t, "Noto Sans Hebrew")
	style.SetFallbackFamilies("Noto Sans Hebrew")
	t.Cleanup(func() { style.SetFallbackFamilies() })
	f := style.BakeFont(24, paint2d.White)
	const text = "abc שלום abc"
	if sp := f.SelectionSpans(text, 6, 12); len(sp) < 2 {
		t.Fatalf("partial direction-crossing selection %v", sp)
	}
	for i := 0; i <= utf8.RuneCountInString(text); i++ {
		x := f.CaretX(text, i)
		got := f.IndexAt(text, x)
		if math.Abs(float64(f.CaretX(text, got)-x)) > .001 {
			t.Errorf("caret%d at%g got%d at%g", i, x, got, f.CaretX(text, got))
		}
	}
	t.Log(fmt.Sprint(style.DirectionRuns(text)), f.SelectionSpans(text, 6, 12))
}

func TestDefaultInstalledScriptProvidersMatchSourceFonts(t *testing.T) {
	if style.DirectionRuns("abc שלום") == nil {
		t.Skip("requires shaping")
	}
	style.SetFallbackFamilies()
	t.Cleanup(func() { style.SetFallbackFamilies() })
	for _, c := range []struct{ family, a, b string }{
		{"Noto Sans Arabic", "سلام", "كتاب"},
		{"Noto Sans Hebrew", "שלום", "חיים"},
		{"Noto Sans Devanagari", "कि", "का"},
		{"Noto Sans Thai", "เพลง", "ภาษา"},
	} {
		t.Run(c.family, func(t *testing.T) {
			src := fontBytes(t, c.family)
			f := style.BakeFont(24, paint2d.White)
			if bytes.Equal(picture(f, c.a, paint2d.White).Pix, picture(f, c.b, paint2d.White).Pix) {
				t.Fatalf("default list makes %q/%q identical", c.a, c.b)
			}
			out := sourceShape(t, src, c.a, 24)
			want := float32(out.Advance) / 64
			if got := f.Advance(c.a); math.Abs(float64(got-want)) > .02 {
				t.Fatalf("default width=%g expected installedsource%g for%q", got, want, c.a)
			}
			t.Logf("default list provider%q text%q width%g matches sourcefont without setter override", c.family, c.a, want)
		})
	}
}
func TestEmojiFitsLineAndRetainsColourThroughGrowth(t *testing.T) {
	fontBytes(t, "Noto Color Emoji")
	for _, size := range []float32{16, 24, 28, 42} {
		f := style.BakeFont(size, paint2d.White)
		before := picture(f, "👀", paint2d.White)
		b := ink(before)
		if b.Empty() || b.Min.Y < 29 || b.Max.Y > 30+f.Height()+2 {
			t.Fatalf("size%g emoji above/outside line ink=%v height=%g", size, b, f.Height())
		}
		if !bytes.Equal(before.Pix, picture(f, "👀", paint2d.RGB(1, 0, 0)).Pix) {
			t.Fatalf("size%g initial emoji tinted (requires font_emoji)", size)
		}
		picture(f, "😀🇯🇵👍🏽👩‍👩‍👧‍👦", paint2d.White)
		if !bytes.Equal(before.Pix, picture(f, "👀", paint2d.White).Pix) || !bytes.Equal(before.Pix, picture(f, "👀", paint2d.RGB(1, 0, 0)).Pix) {
			t.Fatalf("size%g cached emoji changed/tinted after atlas growth", size)
		}
		hand := f.Advance("👍")
		for _, s := range []string{"👍🏽", "🇯🇵", "👩‍👩‍👧‍👦", "❤️"} {
			if w := f.Advance(s); w <= 0 || w > hand*1.5 {
				t.Errorf("size%g composed%q width%g hand%g", size, s, w, hand)
			}
			if !bytes.Equal(picture(f, s, paint2d.White).Pix, picture(f, s, paint2d.RGB(1, 0, 0)).Pix) {
				t.Errorf("size%g composed%q is tinted", size, s)
			}
		}
		t.Logf("size%g primaryHeight%g emojiInk=%v handWidth%g", size, f.Height(), b, hand)
	}
}

// No application wrapper chooses these glyphs. The preserved installed font
// is shaped separately so a font lacking a sequence cannot look like a toolkit
// composition failure. All these source sequences produce one glyph.
func TestEmojiSequencesMatchInstalledSourceShaping(t *testing.T) {
	src := fontBytes(t, "Noto Color Emoji")
	face, err := gofont.ParseTTF(bytes.NewReader(src))
	if err != nil {
		t.Fatal(err)
	}
	face.SetPpem(24, 24)
	if gid, covered := face.NominalGlyph('\u20e3'); covered {
		bm, bitmap := face.GlyphDataBitmap(gid)
		t.Logf("source keycap mark U+20E3 cmapGID%d standaloneBitmap=%t bitmapBytes=%d", gid, bitmap, len(bm.Data))
	} else {
		t.Fatal("installed source keycap mark is absent from cmap")
	}
	style.SetFallbackFamilies()
	t.Cleanup(func() { style.SetFallbackFamilies() })
	for _, size := range []float32{16, 24, 28, 42} {
		f := style.BakeFont(size, paint2d.White)
		for _, text := range []string{"👩‍👩‍👧‍👦", "🏳️‍🌈", "🏳‍🌈", "👩‍💻", "1️⃣", "1⃣", "#️⃣", "*️⃣", "👍🏽", "🇯🇵"} {
			out := sourceShape(t, src, text, size)
			if len(out.Glyphs) != 1 || out.Glyphs[0].GlyphID == 0 {
				t.Fatalf("installed source does not compose %q at %g: %v", text, size, out.Glyphs)
			}
			want := float32(out.Advance) / 64
			if got := f.Advance(text); math.Abs(float64(got-want)) > .02 {
				t.Errorf("size%g composed%q width=%g source one-glyph width=%g", size, text, got, want)
			}
			white, red := picture(f, text, paint2d.White), picture(f, text, paint2d.RGB(1, 0, 0))
			if ink(white).Empty() || !bytes.Equal(white.Pix, red.Pix) {
				t.Errorf("size%g composed%q empty or tinted", size, text)
			}
			if got := paintedGlyphCount(f, text); got != len(out.Glyphs) {
				t.Errorf("size%g composed%q painted glyph cells=%d source=%d", size, text, got, len(out.Glyphs))
			}
			t.Logf("size%g composed%q sourceGID%d width=%g", size, text, out.Glyphs[0].GlyphID, want)
		}
	}
}

// Bare Emoji_Presentation characters use the installed colour source, while
// an explicit text selector must still reach a covering outline face. Testing
// the source bitmap, width, cell count and tint prevents a missing-glyph box
// from satisfying either side of the presentation contract.
func TestBMPDefaultEmojiPresentationMatchesInstalledSource(t *testing.T) {
	src := fontBytes(t, "Noto Color Emoji")
	face, err := gofont.ParseTTF(bytes.NewReader(src))
	if err != nil {
		t.Fatal(err)
	}
	outline, err := gofont.ParseTTF(bytes.NewReader(fontBytes(t, "Noto Sans Symbols 2")))
	if err != nil {
		t.Fatal(err)
	}
	style.SetFallbackFamilies()
	t.Cleanup(func() { style.SetFallbackFamilies() })
	for _, size := range []float32{16, 24, 28, 42} {
		face.SetPpem(uint16(size), uint16(size))
		f := style.BakeFont(size, paint2d.White)
		for _, text := range []string{"⌚", "⚡", "⭐"} {
			for _, r := range text {
				if gid, ok := outline.NominalGlyph(r); !ok || gid == 0 {
					t.Fatalf("installed outline source does not cover U+%04X", r)
				}
			}
			out := sourceShape(t, src, text, size)
			qualified := sourceShape(t, src, text+"\ufe0f", size)
			if len(out.Glyphs) != 1 || len(qualified.Glyphs) != 1 || out.Glyphs[0].GlyphID != qualified.Glyphs[0].GlyphID {
				t.Fatalf("source bare/VS16 do not share one glyph: %q", text)
			}
			bitmap, ok := face.GlyphDataBitmap(gofont.GID(out.Glyphs[0].GlyphID))
			if !ok || len(bitmap.Data) == 0 {
				t.Fatalf("source has no supported colour bitmap for %q", text)
			}
			want := float32(out.Advance) / 64
			if got := f.Advance(text); math.Abs(float64(got-want)) > .02 {
				t.Errorf("size%g bare%q width%g source%g", size, text, got, want)
			}
			white := picture(f, text, paint2d.White)
			if ink(white).Empty() || !bytes.Equal(white.Pix, picture(f, text, paint2d.RGB(1, 0, 0)).Pix) {
				t.Errorf("size%g bare%q empty or tinted", size, text)
			}
			if got := paintedGlyphCount(f, text); got != 1 {
				t.Errorf("size%g bare%q paintedCells%d source1", size, text, got)
			}
			if !bytes.Equal(white.Pix, picture(f, text+"\ufe0f", paint2d.White).Pix) {
				t.Errorf("size%g bare%q differs from source-equivalent VS16", size, text)
			}
			plain := picture(f, text+"\ufe0e", paint2d.White)
			red := picture(f, text+"\ufe0e", paint2d.RGB(1, 0, 0))
			untinted := 0
			for y := 0; y < plain.Height; y++ {
				for x := 0; x < plain.Width; x++ {
					if pixel := plain.NRGBAAt(x, y); pixel.A > 40 && pixel == red.NRGBAAt(x, y) {
						untinted++
					}
				}
			}
			if ink(plain).Empty() || untinted != 0 || bytes.Equal(plain.Pix, white.Pix) {
				t.Errorf("size%g text%q missing/tinted incorrectly: untintedPixels%d", size, text, untinted)
			}
			t.Logf("size%g BMP%q sourceGID%d bitmapBytes%d bareWidth%g textWidth%g textUntintedPixels%d", size, text, out.Glyphs[0].GlyphID, len(bitmap.Data), f.Advance(text), f.Advance(text+"\ufe0e"), untinted)
		}
	}
}

func TestBMPCoupleAndGenderSequencesMatchInstalledSource(t *testing.T) {
	src := fontBytes(t, "Noto Color Emoji")
	style.SetFallbackFamilies()
	t.Cleanup(func() { style.SetFallbackFamilies() })
	for _, size := range []float32{16, 24, 28, 42} {
		f := style.BakeFont(size, paint2d.White)
		for _, text := range []string{"👩‍❤️‍👩", "👩‍❤️‍👨", "👩‍❤️‍💋‍👨", "🏃‍♀️", "🏃‍♂️", "🧑‍⚕️", "🧑‍⚖️"} {
			out := sourceShape(t, src, text, size)
			if len(out.Glyphs) != 1 || out.Glyphs[0].GlyphID == 0 {
				t.Fatalf("installed source does not compose %q at %g: %v", text, size, out.Glyphs)
			}
			want := float32(out.Advance) / 64
			if got := f.Advance(text); math.Abs(float64(got-want)) > .02 {
				t.Errorf("size%g composed%q width=%g source one-glyph width=%g", size, text, got, want)
			}
			if got := paintedGlyphCount(f, text); got != 1 {
				t.Errorf("size%g composed%q painted glyph cells=%d source=1", size, text, got)
			}
			white, red := picture(f, text, paint2d.White), picture(f, text, paint2d.RGB(1, 0, 0))
			if ink(white).Empty() || !bytes.Equal(white.Pix, red.Pix) {
				t.Errorf("size%g composed%q empty or tinted", size, text)
			}
			t.Logf("size%g composed%q sourceGID%d width=%g sourceGlyphs=1", size, text, out.Glyphs[0].GlyphID, want)
		}
	}
}

func TestEmojiTagFlagsRetainSourceComposition(t *testing.T) {
	src := fontBytes(t, "Noto Color Emoji")
	face, err := gofont.ParseTTF(bytes.NewReader(src))
	if err != nil {
		t.Fatal(err)
	}
	face.SetPpem(24, 24)
	base := sourceShape(t, src, "🏴", 24)
	baseBitmap, ok := face.GlyphDataBitmap(gofont.GID(base.Glyphs[0].GlyphID))
	if !ok {
		t.Fatal("installed source has no black-flag bitmap")
	}
	style.SetFallbackFamilies()
	t.Cleanup(func() { style.SetFallbackFamilies() })
	f := style.BakeFont(24, paint2d.White)
	black := picture(f, "🏴", paint2d.White)
	for _, text := range []string{
		"\U0001f3f4\U000e0067\U000e0062\U000e0065\U000e006e\U000e0067\U000e007f",
		"\U0001f3f4\U000e0067\U000e0062\U000e0073\U000e0063\U000e0074\U000e007f",
		"\U0001f3f4\U000e0067\U000e0062\U000e0077\U000e006c\U000e0073\U000e007f",
	} {
		out := sourceShape(t, src, text, 24)
		if len(out.Glyphs) != 1 || out.Glyphs[0].GlyphID == base.Glyphs[0].GlyphID {
			t.Fatalf("source does not compose tag flag %+q: %v", text, out.Glyphs)
		}
		bitmap, ok := face.GlyphDataBitmap(gofont.GID(out.Glyphs[0].GlyphID))
		if !ok || bytes.Equal(bitmap.Data, baseBitmap.Data) {
			t.Fatalf("source tag flag %+q has no distinct supported bitmap", text)
		}
		white := picture(f, text, paint2d.White)
		if bytes.Equal(white.Pix, black.Pix) {
			t.Errorf("tag flag %+q rendered the black flag instead of source GID%d", text, out.Glyphs[0].GlyphID)
		}
		if got, want := f.Advance(text), float32(out.Advance)/64; math.Abs(float64(got-want)) > .02 {
			t.Errorf("tag flag %+q width%g source%g", text, got, want)
		}
		if !bytes.Equal(white.Pix, picture(f, text, paint2d.RGB(1, 0, 0)).Pix) {
			t.Errorf("tag flag %+q is tinted", text)
		}
		t.Logf("tag flag %+q sourceGID%d paintedCells%d equalsBlackFlag%t", text, out.Glyphs[0].GlyphID, paintedGlyphCount(f, text), bytes.Equal(white.Pix, black.Pix))
	}
}

// Width, tint, cell count and merely differing from the black flag cannot
// identify a country. Inspect public glyph-cell pixels and compare them with
// the independently parsed source GID's PNG, scaled through the public renderer
// to the emitted cell dimensions. No toolkit run internals are accessed.
func TestTagFlagsPaintTheirOwnSourceBitmaps(t *testing.T) {
	src := fontBytes(t, "Noto Color Emoji")
	face, err := gofont.ParseTTF(bytes.NewReader(src))
	if err != nil {
		t.Fatal(err)
	}
	face.SetPpem(24, 24)
	style.SetFallbackFamilies()
	t.Cleanup(func() { style.SetFallbackFamilies() })
	f := style.BakeFont(24, paint2d.White)
	var pictures []*paint2d.Image
	for _, text := range []string{
		"\U0001f3f4\U000e0067\U000e0062\U000e0065\U000e006e\U000e0067\U000e007f",
		"\U0001f3f4\U000e0067\U000e0062\U000e0073\U000e0063\U000e0074\U000e007f",
		"\U0001f3f4\U000e0067\U000e0062\U000e0077\U000e006c\U000e0073\U000e007f",
	} {
		out := sourceShape(t, src, text, 24)
		if len(out.Glyphs) != 1 {
			t.Fatalf("source does not compose tag flag %+q", text)
		}
		bitmap, ok := face.GlyphDataBitmap(gofont.GID(out.Glyphs[0].GlyphID))
		if !ok || bitmap.Format != gofont.PNG {
			t.Fatalf("source tag flag %+q has no supported PNG", text)
		}
		decoded, err := png.Decode(bytes.NewReader(bitmap.Data))
		if err != nil {
			t.Fatal(err)
		}
		bounds := decoded.Bounds()
		original := paint2d.NewImage(bounds.Dx(), bounds.Dy())
		for y := 0; y < original.Height; y++ {
			for x := 0; x < original.Width; x++ {
				r, g, b, a := decoded.At(bounds.Min.X+x, bounds.Min.Y+y).RGBA()
				i := y*original.Stride + x*4
				original.Pix[i], original.Pix[i+1], original.Pix[i+2], original.Pix[i+3] = byte(r>>8), byte(g>>8), byte(b>>8), byte(a>>8)
			}
		}
		im := paint2d.NewImage(900, 200)
		device := &glyphCellCaptureDevice{Device: paint2d.NewCPUDevice(im)}
		f.Draw(paint2d.NewContextDevice(device), text, paint2d.Pt(20, 30), paint2d.White)
		if len(device.cells) != 1 {
			t.Fatalf("tag flag %+q painted %d cells instead of source one", text, len(device.cells))
		}
		cell := device.cells[0]
		want := paint2d.NewImage(cell.Width, cell.Height)
		paint2d.NewContext(want).DrawImageRect(original,
			paint2d.XYWH(0, 0, float32(original.Width), float32(original.Height)),
			paint2d.XYWH(0, 0, float32(want.Width), float32(want.Height)))
		matches := bytes.Equal(cell.Pix, want.Pix)
		if !matches {
			t.Errorf("tag flag %+q cell differs from its own source GID%d PNG", text, out.Glyphs[0].GlyphID)
		}
		for i, previous := range pictures {
			if bytes.Equal(im.Pix, previous.Pix) {
				t.Errorf("tag flag %+q equals previously rendered country %d", text, i)
			}
		}
		pictures = append(pictures, im)
		t.Logf("tag flag %+q sourceGID%d emittedCell%dx%d matchesOwnSourcePNG%t", text, out.Glyphs[0].GlyphID, cell.Width, cell.Height, matches)
	}
}

// A terminator groups a tag specification only behind a picture. It must not
// turn plain text, stray controls or an explicit text presentation into an
// emoji, and an unfinished sequence still has the visible black-flag base.
func TestTagMarkingPreservesMalformedAndTextPresentation(t *testing.T) {
	fontBytes(t, "Noto Color Emoji")
	outline, err := gofont.ParseTTF(bytes.NewReader(fontBytes(t, "Noto Sans Symbols 2")))
	if err != nil {
		t.Fatal(err)
	}
	if gid, ok := outline.NominalGlyph('\u231a'); !ok || gid == 0 {
		t.Skip("installed outline source does not cover the text-presentation watch")
	}
	style.SetFallbackFamilies()
	t.Cleanup(func() { style.SetFallbackFamilies() })
	f := style.BakeFont(24, paint2d.White)
	const tags = "\U000e0067\U000e0062\U000e0065\U000e006e\U000e0067"
	const term = "\U000e007f"
	for _, c := range []struct {
		name, text, visible string
		plain               bool
	}{
		{"lone terminator", term, "", true},
		{"lone tag", "\U000e0067", "", true},
		{"tags without base", tags + term, "", true},
		{"terminator after text", "A" + term, "A", true},
		{"tags after text", "A" + tags + term, "A", true},
		{"unfinished flag", "🏴" + tags, "🏴", false},
		{"terminator without tags", "🏴" + term, "🏴", false},
		{"explicit text picture", "⌚\ufe0e" + tags + term, "⌚\ufe0e", true},
	} {
		actual, want := picture(f, c.text, paint2d.White), picture(f, c.visible, paint2d.White)
		samePixels := bytes.Equal(actual.Pix, want.Pix)
		if !samePixels {
			t.Errorf("%s %+q differs from visible base %+q", c.name, c.text, c.visible)
		}
		if got, want := f.Advance(c.text), f.Advance(c.visible); math.Abs(float64(got-want)) > .02 {
			t.Errorf("%s %+q width%g visible-base width%g", c.name, c.text, got, want)
		}
		if c.plain {
			red := picture(f, c.text, paint2d.RGB(1, 0, 0))
			for y := 0; y < actual.Height; y++ {
				for x := 0; x < actual.Width; x++ {
					if pixel := actual.NRGBAAt(x, y); pixel.A > 40 && pixel == red.NRGBAAt(x, y) {
						t.Fatalf("%s %+q acquired an untinted colour pixel", c.name, c.text)
					}
				}
			}
		}
		t.Logf("%s %+q width%g equalsVisibleBase%t", c.name, c.text, f.Advance(c.text), samePixels)
	}
}

// UTS #51 keycaps have digit/#/* bases. Other combining-keycap text and
// explicitly requested text presentation must retain the covering outline
// route; the successful qualified/unqualified emoji assertions stay above.
func TestKeycapMarkPreservesNonEmojiAndTextSelectorPresentation(t *testing.T) {
	fontBytes(t, "Noto Color Emoji")
	covered := false
	for _, family := range []string{"Noto Sans Symbols", "Noto Sans Symbols 2"} {
		if !style.FontInstalled(family) {
			continue
		}
		face, err := gofont.ParseTTF(bytes.NewReader(fontBytes(t, family)))
		if err != nil {
			t.Fatal(err)
		}
		if gid, ok := face.NominalGlyph('\u20e3'); ok && gid != 0 {
			covered = true
			break
		}
	}
	if !covered {
		t.Skip("no installed covering outline keycap-mark face")
	}
	style.SetFallbackFamilies()
	t.Cleanup(func() { style.SetFallbackFamilies() })
	f := style.BakeFont(24, paint2d.White)
	for _, text := range []string{"A", "A\u20e3", "1\ufe0e", "1\ufe0e\u20e3"} {
		white, red := picture(f, text, paint2d.White), picture(f, text, paint2d.RGB(1, 0, 0))
		unchanged := 0
		for y := 0; y < white.Height; y++ {
			for x := 0; x < white.Width; x++ {
				p := white.NRGBAAt(x, y)
				if p.A > 40 && p == red.NRGBAAt(x, y) {
					unchanged++
				}
			}
		}
		if ink(white).Empty() || unchanged != 0 {
			t.Errorf("text%q width%g has %d untinted visible pixels with a covering outline face", text, f.Advance(text), unchanged)
		}
		t.Logf("text%q width%g untintedPixels%d", text, f.Advance(text), unchanged)
	}
}

func TestPrefixReturnsLongestIndependentlyFittingPrefix(t *testing.T) {
	if style.DirectionRuns("abc שלום") == nil {
		t.Skip("requires shaping")
	}
	fontBytes(t, "Noto Sans Arabic")
	fontBytes(t, "Noto Sans Devanagari")
	style.SetFallbackFamilies()
	t.Cleanup(func() { style.SetFallbackFamilies() })
	f := style.BakeFont(24, paint2d.White)
	for _, c := range []struct {
		text   string
		budget float32
	}{{"مرحبا", 12.5}, {"क्षि", 17.4}} {
		p := f.Prefix(c.text, c.budget)
		var longest string
		runes := []rune(c.text)
		for n := 1; n <= len(runes); n++ {
			candidate := string(runes[:n])
			if f.Advance(candidate) <= c.budget {
				longest = candidate
			}
		}
		if p != longest || f.Advance(p) > c.budget {
			t.Errorf("Prefix(%q,%g)=%q width%g want longest%q width%g", c.text, c.budget, p, f.Advance(p), longest, f.Advance(longest))
		}
		t.Logf("text%q budget%g longest%q width%g", c.text, c.budget, p, f.Advance(p))
	}
}
