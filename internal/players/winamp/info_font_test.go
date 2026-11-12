package winamp

import (
	"fmt"
	"testing"
	"time"

	"github.com/codemodify/media-player-music/internal/players"
	"goframework.io/desktop-ui-skins"
	"goframework.io/desktop-ui/widget"
	"goframework.io/graphics-paint2d"
)

// Bevel and Graphing draw their main display font in its own ink and their
// playlist readouts from tinted masks: the footer readouts must take the
// pack's player.info ink, while the main font keeps its drawn colour.
func TestBevelPlaylistReadoutsUseThePacksInk(t *testing.T) {
	testPlaylistReadoutInk(t, players.Windows98Skin)
}

func TestGraphingPlaylistReadoutsUseThePacksInk(t *testing.T) {
	testPlaylistReadoutInk(t, players.TI82Skin)
}

func testPlaylistReadoutInk(t *testing.T, skin string) {
	t.Helper()
	for _, scale := range []float32{1, 1.75} {
		t.Run(fmt.Sprintf("%gx", scale), func(t *testing.T) {
			a, p := open(t, skin, scale)
			p.Pose(97 * time.Second)
			a.PumpOnce()
			sk, ok := skins.LoadSkin(skin)
			if !ok || sk.Sprites["font.31"] == nil || sk.Sprites["font.31"].Tint || sk.Sprites["info.font.31"] == nil || !sk.Sprites["info.font.31"].Tint {
				t.Fatal("main font or optional readout alpha masks are missing")
			}
			role := sk.Text["player.info"]
			if role == nil {
				t.Fatal("the pack has no player.info ink")
			}
			ink := role.Color.Clamp().NRGBA()
			want := [3]uint8{ink.R, ink.G, ink.B}
			image := p.List().Capture()
			origin := widget.DeviceOrigin(p.listPane).Sub(p.List().WindowRect().Min)
			for _, slot := range []string{"info", "clock"} {
				r := slotRect(p.listPane.Look(), layoutPlaylist, slot, p.listPane.LocalBounds()).Translate(origin)
				count := 0
				for y := max(0, int(r.Min.Y)); y < min(image.Height, int(r.Max.Y)); y++ {
					for x := max(0, int(r.Min.X)); x < min(image.Width, int(r.Max.X)); x++ {
						r, g, b, a := image.PremulAt(x, y)
						if r == want[0] && g == want[1] && b == want[2] && a == 255 {
							count++
						}
					}
				}
				if count < 8 {
					t.Fatalf("%s %s readout did not use the pack's ink #%02X%02X%02X: %d pixels", skin, slot, want[0], want[1], want[2], count)
				}
			}
		})
	}
}

func TestPlaylistInfoFontFallsBackWithoutChangingClassicArtwork(t *testing.T) {
	for _, scale := range []float32{1, 1.75} {
		t.Run(fmt.Sprintf("%gx", scale), func(t *testing.T) {
			_, p := open(t, Skin, scale)
			lk := p.List().Look()
			const text = "3:47/37:14"
			width := pixWidth(lk, text)
			if got := pixInfoWidth(lk, text); got != width {
				t.Fatalf("fallback width=%g want %g", got, width)
			}
			paint := func(info bool) *paint2d.Image {
				im := paint2d.NewImage(200, 30)
				ctx := paint2d.NewContext(im)
				if info {
					pixInfoText(lk, ctx, .25, .5, text, rgb(0x23, 0xdd, 0x91))
				} else {
					pixText(lk, ctx, .25, .5, text, rgb(0x23, 0xdd, 0x91))
				}
				return im
			}
			before, after := paint(false), paint(true)
			for y := 0; y < before.Height; y++ {
				for x := 0; x < before.Width; x++ {
					r, g, b, a := before.PremulAt(x, y)
					r2, g2, b2, a2 := after.PremulAt(x, y)
					if r != r2 || g != g2 || b != b2 || a != a2 {
						t.Fatalf("fallback changed original glyph pixel (%d,%d)", x, y)
					}
				}
			}
		})
	}
}
