//go:build ignore

// Public Font copies: cached drawing/measurement while their atlas grows.
// Run this file explicitly with -race, with no font tag, font_fallback,
// font_complex or font_emoji. No application imports or private hooks.
package fontcacheaudit

import (
	"bytes"
	"sync"
	"testing"

	"goframework.io/desktop-ui/style"
	"goframework.io/graphics-paint2d"
)

func TestConcurrentCachedMeasureDrawAndAtlasGrowth(t *testing.T) {
	font := style.BakeFont(19, paint2d.White)
	// A value copy is explicitly a supported cheap Font handle. Both copies
	// share the font's shape cache and glyph sheet, without a shared image.
	measure := *font
	const anchor = "Shared font cache: AV office 2026"
	reference := paint2d.NewImage(360, 72)
	font.Draw(paint2d.NewContext(reference), anchor, paint2d.Pt(8, 8), paint2d.White)
	wantWidth := font.Advance(anchor)
	old := font.GlyphAtlas()
	oldPixels := append([]byte(nil), old.Image.Pix...)

	start := make(chan struct{})
	var painters, growers sync.WaitGroup
	for i := 0; i < 3; i++ {
		paint := *font
		painters.Add(1)
		go func() {
			defer painters.Done()
			<-start
			image := paint2d.NewImage(360, 72)
			context := paint2d.NewContext(image)
			for frame := 0; frame < 300; frame++ {
				context.Clear(paint2d.Color{})
				paint.Draw(context, anchor, paint2d.Pt(8, 8), paint2d.White)
				if got := paint.Advance(anchor); got != wantWidth {
					t.Errorf("cached width changed from %g to %g", wantWidth, got)
					return
				}
				if !bytes.Equal(image.Pix, reference.Pix) {
					t.Errorf("cached text pixels changed at frame %d while adding unrelated glyphs", frame)
					return
				}
			}
		}()
	}
	for _, first := range []rune{0x100, 0x200} {
		growers.Add(1)
		go func(first rune) {
			defer growers.Done()
			<-start
			for r := first; r < first+112; r++ {
				// Public measurement adds a new glyph/sheet, then reuses the
				// same cached anchor that the painters may still be reading.
				measure.Advance(string(r))
				if got := measure.Measure(anchor).X; got != wantWidth {
					t.Errorf("rebound cached width changed from %g to %g", wantWidth, got)
					return
				}
			}
		}(first)
	}
	close(start)
	growers.Wait()
	painters.Wait()
	if current := font.GlyphAtlas(); current == old {
		t.Fatal("control did not publish a new primary atlas")
	}
	if !bytes.Equal(old.Image.Pix, oldPixels) {
		t.Fatal("previously published atlas snapshot was mutated")
	}
	t.Logf("cached width %g remained stable; atlas grew from %d to %d cells", wantWidth, len(old.Cells), len(font.GlyphAtlas().Cells))
}
