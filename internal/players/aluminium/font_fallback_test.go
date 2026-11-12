//go:build font_fallback || font_complex || font_emoji

package aluminium

import (
	"bytes"
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/codemodify/media-player-music/internal/players"
	"github.com/codemodify/media-player-music/internal/players/playertest"
	"goframework.io/desktop-ui-skins"
	"goframework.io/desktop-ui/a11y"
	"goframework.io/desktop-ui/widget"
	"goframework.io/graphics-paint2d"
)

func TestUnicodeFullMiniTitlesAndSourceSearchKeepRenderingAndEditing(t *testing.T) {
	playertest.RequireCJKFallback(t)
	for _, scale := range []float32{1, 1.75} {
		t.Run(fmt.Sprintf("%gx", scale), func(t *testing.T) {
			a, p := open(t, scale)
			p.Transport.List = players.NewPlaylist(players.Track{Title: "Song 山川", Length: 3 * time.Minute})
			p.Transport.State, p.Transport.Pos = players.Paused, 13*time.Second
			crop := func(r paint2d.Rect) []byte {
				im := p.Main.CaptureSurface()
				x0, y0 := max(0, int(math.Floor(float64(r.Min.X)))), max(0, int(math.Floor(float64(r.Min.Y))))
				x1, y1 := min(im.Width, int(math.Ceil(float64(r.Max.X)))), min(im.Height, int(math.Ceil(float64(r.Max.Y))))
				var pixels []byte
				for y := y0; y < y1; y++ {
					at := (y*im.Width + x0) * 4
					pixels = append(pixels, im.Pix[at:at+(x1-x0)*4]...)
				}
				return pixels
			}
			for _, mini := range []bool{false, true} {
				p.SetMiniPlayer(mini)
				a.PumpOnce()
				p.Advance(0)
				a.PumpOnce()
				role := LayoutMain
				if mini {
					role = LayoutMini
				}
				r, ok := skins.SkinSlotRect(p.Main.Look(), role, "display", p.Main.Content().LocalBounds())
				if !ok || r.Empty() {
					t.Fatal("mode has no title display")
				}
				capture := func(title string) []byte {
					p.Transport.List.Tracks[0].Title = title
					p.Refresh()
					a.PumpOnce()
					return crop(r)
				}
				first, second := capture("Song 山川"), capture("Song 海河")
				if len(first) == 0 || bytes.Equal(first, second) {
					t.Fatalf("mini%v title fallback still paints identical missing-glyph boxes", mini)
				}
			}
			p.SetMiniPlayer(false)
			a.PumpOnce()
			p.Advance(0)
			a.PumpOnce()
			field := p.body.search
			field.SetCaretBlink(false)
			p.Main.RequestFocus(p.body.play)
			calls := 0
			input := field.OnInput
			field.OnInput = func(s string) { calls++; input(s) }
			capture := func(title string) []byte {
				p.Transport.List.Tracks[0].Title = title
				p.Refresh()
				if !p.Main.AccessibleSetText(field.ID(), title) {
					t.Fatal("Unicode editable-text action rejected")
				}
				a.PumpOnce()
				if field.Text != title || p.body.list.Count != 1 || p.Transport.List.Index() != 0 {
					t.Fatal("Unicode search lost typed text or its mapped queue row")
				}
				return crop(widget.DeviceBounds(field))
			}
			first, second := capture("Song 山川"), capture("Song 海河")
			if bytes.Equal(first, second) || calls != 2 {
				t.Fatal("source search well did not render/edit the distinct Unicode queries exactly once")
			}
			look := field.Look()
			p.Refresh()
			a.PumpOnce()
			if field.Look() != look || calls != 2 {
				t.Fatal("font fallback refresh replaced editor state")
			}
			if !p.Main.AccessibleAction(widget.ItemID(field, 0), a11y.ActionDefault) || field.Text != "" || calls != 3 || p.body.list.Count != 1 {
				t.Fatal("Unicode search lost its accessible clear action")
			}
			if p.Transport.State != players.Paused || p.Transport.Pos != 13*time.Second {
				t.Fatal("Unicode title/search updates changed playback")
			}
		})
	}
}
