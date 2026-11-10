//go:build ignore

// Independent public fallback tests; no app imports or private toolkit hooks.
// tools/testenv.sh go test -race ./tools/probes/font_fallback_regressions_test.go
// tools/testenv.sh go test -race -tags font_fallback ./tools/probes/font_fallback_regressions_test.go -args -expect-fallback
// tools/testenv.sh env GOFRAMEWORK_DESKTOP_UI_SYSTEM_FONTS=0 go test -race -tags font_fallback ./tools/probes/font_fallback_regressions_test.go -args -expect-fallback
// font_complex and font_emoji imply font_fallback in v0.23.8; use the same
// -expect-fallback argument with either tag to check these CJK/Latin contracts.
package fallbackaudit

import (
	"bytes"
	"flag"
	"fmt"
	"math"
	"os"
	"strings"
	"testing"
	"unicode/utf8"

	"goframework.io/desktop-ui-widgets"
	"goframework.io/desktop-ui/style"
	"goframework.io/graphics-paint2d"
)

var expectFallback = flag.Bool("expect-fallback", false, "the test binary was built with font_fallback")

func renderFont(f *style.Font, text string, origin paint2d.Point) *paint2d.Image {
	im := paint2d.NewImage(900, 160)
	f.Draw(paint2d.NewContext(im), text, origin, paint2d.White)
	return im
}

func coveringFamily() string {
	for _, family := range style.FallbackFamilies() {
		if style.FontInstalled(family) {
			f := style.BakeFamily(family, style.WeightRegular, 16, paint2d.White)
			if !bytes.Equal(renderFont(f, "山", paint2d.Pt(20, 20)).Pix, renderFont(f, "海", paint2d.Pt(20, 20)).Pix) {
				return family
			}
		}
	}
	return ""
}

func inkBox(im *paint2d.Image) paint2d.Rect {
	x0, y0, x1, y1 := im.Width, im.Height, -1, -1
	for y := 0; y < im.Height; y++ {
		for x := 0; x < im.Width; x++ {
			if im.NRGBAAt(x, y).A > 40 {
				x0, y0, x1, y1 = min(x0, x), min(y0, y), max(x1, x), max(y1, y)
			}
		}
	}
	if x1 < x0 {
		return paint2d.Rect{}
	}
	return paint2d.XYWH(float32(x0), float32(y0), float32(x1-x0+1), float32(y1-y0+1))
}

func TestStockTitlesHonorBuildChoiceAndFontAvailability(t *testing.T) {
	family := coveringFamily()
	wantDifferent := *expectFallback && os.Getenv(style.SystemFontsEnv) != "0" && family != ""
	if *expectFallback && os.Getenv(style.SystemFontsEnv) != "0" && family == "" {
		t.Skip("no installed covering CJK font; positive fallback cannot be verified")
	}
	for _, scale := range []float32{1, 1.25, 1.75, 2} {
		lk := style.WithScale(style.DarkLook(), scale)
		row := func(title string) (*paint2d.Image, string) {
			l := widgets.NewListView(1, func(int) string { return title }, nil)
			l.SetLook(lk)
			l.Arrange(paint2d.XYWH(0, 0, 400*scale, 40*scale))
			im := paint2d.NewImage(int(400*scale), int(40*scale))
			l.Paint(paint2d.NewContext(im))
			return im, l.AccessibleItems()[0].Name
		}
		a, nameA := row("Song 山川")
		b, nameB := row("Song 海河")
		different := !bytes.Equal(a.Pix, b.Pix)
		if different != wantDifferent || nameA == nameB {
			t.Fatalf("scale=%g different=%t want=%t accessible names distinct=%t", scale, different, wantDifferent, nameA != nameB)
		}
		t.Logf("scale=%g different=%t provider=%q system_fonts=%q", scale, different, family, os.Getenv(style.SystemFontsEnv))
	}
}

func TestMixedTextMetricsFitClippingAndWarmAtlas(t *testing.T) {
	family := coveringFamily()
	if !*expectFallback || family == "" {
		t.Skip("requires fallback build and installed CJK font")
	}
	style.SetFallbackFamilies(family)
	t.Cleanup(func() { style.SetFallbackFamilies() })
	for _, scale := range []float32{1, 1.25, 1.75, 2} {
		for _, weight := range []style.Weight{style.WeightRegular, style.WeightBold} {
			f := style.BakeFamily(style.FamilyUI, weight, 16*scale, paint2d.White)
			provider := style.BakeFamily(family, weight, 16*scale, paint2d.White)
			const text = "AV山川 music 河 — 2026"
			width := f.Advance(text)
			if got := f.Measure(text); got.X != width || got.Y != f.Height() {
				t.Fatal("Measure/Advance/Height disagree", got, width, f.Height())
			}
			want := f.Advance("A") + provider.Advance("山") + f.Advance("V")
			if got := f.Advance("A山V"); math.Abs(float64(got-want)) > .001 {
				t.Fatalf("mixed advance=%g want=%g", got, want)
			}
			last := float32(0)
			for i := 0; i <= utf8.RuneCountInString(text); i++ {
				x := f.CaretX(text, i)
				if x < last || x > width+.001 || f.IndexAt(text, x) != i {
					t.Fatalf("caret/hit disagreement at rune%d: x=%g last=%g index=%d", i, x, last, f.IndexAt(text, x))
				}
				last = x
			}
			if f.Fit(text, width) != text || f.Fit(text, width-.0001) != text {
				t.Fatal("Fit elided text at its measured width")
			}
			for _, limit := range []float32{30 * scale, 55 * scale, 90 * scale, width * .75} {
				fit := f.Fit(text, limit)
				if !utf8.ValidString(fit) || f.Advance(fit) > limit+1.0/32+.001 || !(strings.HasPrefix(text, strings.TrimSuffix(fit, "…"))) {
					t.Fatalf("invalid/overlong fit %q width%g limit%g", fit, f.Advance(fit), limit)
				}
			}
			im := renderFont(f, text, paint2d.Pt(20, 20))
			bounds := inkBox(im)
			if bounds.Empty() || bounds.Max.X > 20+f.InkWidth(text)+1 {
				t.Fatalf("draw exceeded measured ink: %v InkWidth=%g", bounds, f.InkWidth(text))
			}
			// Growing either glyph sheet must not change previously shaped text.
			renderFont(f, "ABCDEFGHIJKLMNOPQRSTUVWXYZ abcdefghijklmnopqrstuvwxyz 夢島語峠池湖森空音楽街", paint2d.Pt(20, 20))
			if !bytes.Equal(im.Pix, renderFont(f, text, paint2d.Pt(20, 20)).Pix) || f.Advance(text) != width {
				t.Fatal("warm/grown atlas changed existing text")
			}
			clip := paint2d.XYWH(25, 25, 65*scale, 22*scale)
			clipped := paint2d.NewImage(900, 160)
			ctx := paint2d.NewContext(clipped)
			ctx.ClipRect(clip)
			f.Draw(ctx, text, paint2d.Pt(20, 20), paint2d.White)
			for y := 0; y < clipped.Height; y++ {
				for x := 0; x < clipped.Width; x++ {
					// Fractional clip edges may cover part of a boundary pixel.
					outside := float32(x+1) <= clip.Min.X || float32(x) >= clip.Max.X || float32(y+1) <= clip.Min.Y || float32(y) >= clip.Max.Y
					if clipped.NRGBAAt(x, y).A != 0 && outside {
						t.Fatal("borrowed run escaped clipping", x, y, clip)
					}
				}
			}
			t.Logf("scale=%g weight=%v advance=%g ink=%v", scale, weight, width, bounds)
		}
	}
}

func TestBorrowedGlyphUsesPrimaryBaseline(t *testing.T) {
	family := coveringFamily()
	if !*expectFallback || family == "" {
		t.Skip("requires fallback build and installed CJK font")
	}
	style.SetFallbackFamilies(family)
	t.Cleanup(func() { style.SetFallbackFamilies() })
	for _, size := range []float32{16, 20, 28, 32} {
		f := style.BakeFamily(style.FamilyUI, style.WeightRegular, size, paint2d.White)
		provider := style.BakeFamily(family, style.WeightRegular, size, paint2d.White)
		origin := paint2d.Pt(20, 30)
		actual := inkBox(renderFont(f, "山", origin))
		aligned := origin.Add(paint2d.Pt(0, f.Ascent-provider.Ascent))
		expected := inkBox(renderFont(provider, "山", aligned))
		if actual.Empty() || expected.Empty() || math.Abs(float64(actual.Min.Y-expected.Min.Y)) > 1 || math.Abs(float64(actual.Max.Y-expected.Max.Y)) > 1 {
			t.Fatalf("size%g baseline mismatch actual=%v expected=%v", size, actual, expected)
		}
		t.Log(fmt.Sprintf("size=%g primaryAscent=%g providerAscent=%g actual=%v expected=%v", size, f.Ascent, provider.Ascent, actual, expected))
	}
}

func TestProviderListCopiesAndBundledFallback(t *testing.T) {
	style.SetFallbackFamilies(" raw-family-not-installed ", " ", style.FamilyMono)
	t.Cleanup(func() { style.SetFallbackFamilies() })
	got := style.FallbackFamilies()
	if len(got) != 2 || got[0] != "raw-family-not-installed" || got[1] != style.FamilyMono {
		t.Fatal("provider list normalization/order", got)
	}
	got[0] = "changed by caller"
	if style.FallbackFamilies()[0] != "raw-family-not-installed" {
		t.Fatal("provider list exposed mutable state")
	}
	// A bundled provider remains usable even when installed fonts are disabled.
	// This case is separate from CJK, which the bundled faces do not cover.
	f := style.BakeFamily(style.FamilyUI, style.WeightRegular, 49, paint2d.White)
	different := !bytes.Equal(renderFont(f, "Song Δ", paint2d.Pt(20, 20)).Pix, renderFont(f, "Song Σ", paint2d.Pt(20, 20)).Pix)
	if different != *expectFallback {
		t.Fatalf("bundled provider different=%t want=%t", different, *expectFallback)
	}
	t.Logf("bundled provider different=%t system_fonts=%q", different, os.Getenv(style.SystemFontsEnv))
}

func TestRuntimeFallbackProviderReplacement(t *testing.T) {
	family := coveringFamily()
	if !*expectFallback || family == "" {
		t.Skip("requires fallback build and installed CJK provider")
	}
	t.Cleanup(func() { style.SetFallbackFamilies() })
	f := style.BakeFont(37, paint2d.White)
	picture := func(s string) *paint2d.Image { return renderFont(f, s, paint2d.Pt(20, 20)) }
	distinct := func(a, b string) bool { return !bytes.Equal(picture(a).Pix, picture(b).Pix) }
	style.SetFallbackFamilies("audit-provider-unavailable")
	if distinct("Song 峠", "Song 語") {
		t.Fatal("unavailable provider supplied CJK unexpectedly")
	}
	style.SetFallbackFamilies(family)
	if !distinct("Song 峠", "Song 語") || !distinct("New 峠", "New 語") {
		t.Fatal("provider replacement retained cached miss/run")
	}
	old := picture("Song 池湖")
	width := f.Advance("Song 池湖")
	style.SetFallbackFamilies("audit-provider-unavailable")
	if bytes.Equal(old.Pix, picture("Song 池湖").Pix) || width == f.Advance("Song 池湖") || distinct("New 池", "New 湖") {
		t.Fatal("provider replacement retained cached hit/run/measurement")
	}
	t.Logf("provider=%q cached misses/hits/shaped strings and measurement update in existing Font", family)
}
