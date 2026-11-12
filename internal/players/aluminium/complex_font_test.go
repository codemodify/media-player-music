//go:build font_complex || font_emoji

package aluminium

import (
	"bytes"
	"fmt"
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/codemodify/media-player-music/internal/players"
	"goframework.io/desktop-ui-skins"
	"goframework.io/desktop-ui/a11y"
	"goframework.io/desktop-ui/platform"
	"goframework.io/desktop-ui/style"
	"goframework.io/desktop-ui/widget"
	"goframework.io/graphics-paint2d"
)

func TestComplexFullMiniMetadataAndSourceSearchCaretEditing(t *testing.T) {
	t.Setenv(style.SystemFontsEnv, "1")
	if !style.FontInstalled("Noto Sans Arabic") || !style.FontInstalled("Noto Sans Hebrew") {
		t.Skip("default Arabic/Hebrew test fonts are not installed")
	}
	for _, scale := range []float32{1, 1.75} {
		t.Run(fmt.Sprintf("%gx", scale), func(t *testing.T) {
			a, p := open(t, scale)
			p.Transport.List = players.NewPlaylist(
				players.Track{Title: "Song שלום", Artist: "Artist سلام", Length: 3 * time.Minute},
				players.Track{Title: "Other ספרי", Artist: "كتاب"},
				players.Track{Title: "Song سلام", Artist: "אמן"},
			)
			p.Transport.State, p.Transport.Pos = players.Paused, 13*time.Second
			p.Refresh()
			a.PumpOnce()
			crop := func(r paint2d.Rect) []byte {
				im := p.Main.CaptureSurface()
				x0, y0 := max(0, int(math.Floor(float64(r.Min.X)))), max(0, int(math.Floor(float64(r.Min.Y))))
				x1, y1 := min(im.Width, int(math.Ceil(float64(r.Max.X)))), min(im.Height, int(math.Ceil(float64(r.Max.Y))))
				var out []byte
				for y := y0; y < y1; y++ {
					at := (y*im.Width + x0) * 4
					out = append(out, im.Pix[at:at+(x1-x0)*4]...)
				}
				return out
			}
			main := p.Main
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
				if !ok || r.Empty() || p.Main != main {
					t.Fatal("mode lost its native window or metadata display")
				}
				capture := func(title string) []byte {
					p.Transport.List.Tracks[0].Title = title
					p.Refresh()
					a.PumpOnce()
					return crop(r)
				}
				for _, pair := range [][2]string{{"Song سلام", "Song كتاب"}, {"Song שלום", "Song ספרי"}} {
					if bytes.Equal(capture(pair[0]), capture(pair[1])) {
						t.Fatalf("mini%v distinct script metadata paints identical pixels", mini)
					}
				}
			}
			p.Transport.List.Tracks[0].Title = "Song שלום"
			p.SetMiniPlayer(false)
			a.PumpOnce()
			p.Advance(0)
			a.PumpOnce()
			field := p.body.search
			field.SetCaretBlink(false)
			p.Main.RequestFocus(field)
			calls := 0
			input := field.OnInput
			field.OnInput = func(s string) { calls++; input(s) }
			for _, r := range "שלום" {
				p.Main.Inject(platform.Event{Kind: platform.EventText, Rune: r})
			}
			a.PumpOnce()
			if field.Text != "שלום" || calls != 4 || !reflect.DeepEqual(p.body.list.indices, []int{0}) {
				t.Fatal("routed RTL typing lost a query, input callback or queue mapping")
			}
			// Pure Hebrew reads right to left: its initial caret must be at
			// the word's right edge, with the final caret at its left edge.
			field.SetSelection(0, 0)
			right := field.IMECaretRect().Min.X
			field.SetSelection(4, 4)
			left := field.IMECaretRect().Min.X
			width := field.Look().Font().Advance(field.Text)
			if right <= left || math.Abs(float64(right-left-width)) > .2 {
				t.Fatalf("source editor RTL endpoints right%g left%g width%g", right, left, width)
			}
			for i := 0; i <= 4; i++ {
				field.SetSelection(i, i)
				r := field.IMECaretRect()
				at := widget.DeviceOrigin(field).Add(paint2d.Pt(r.Min.X+.1, field.LocalBounds().Dy()/2))
				click(p.Main, platform.ButtonLeft, at)
				a.PumpOnce()
				if field.Caret() != i || calls != 4 {
					t.Fatalf("RTL pointer/caret disagreement at%d: caret%d calls%d", i, field.Caret(), calls)
				}
			}
			if !p.Main.AccessibleSetText(field.ID(), "Song שלום") || calls != 5 {
				t.Fatal("mixed-script accessible search did not edit exactly once")
			}
			var positions []float32
			for _, i := range []int{6, 7, 8} {
				field.SetSelection(i, i)
				positions = append(positions, field.IMECaretRect().Min.X)
			}
			if !(positions[0] > positions[1] && positions[1] > positions[2]) {
				t.Fatalf("RTL segment in mixed search is not visually reversed: %v", positions)
			}
			field.SetSelection(5, 9)
			if field.SelectedText() != "שלום" {
				t.Fatal("mixed search selection lost its logical rune range")
			}
			look := field.Look()
			for i := 0; i < 3; i++ {
				p.Refresh()
				a.PumpOnce()
			}
			if field.Look() != look || field.SelectedText() != "שלום" || calls != 5 || p.Main.Focus() != field {
				t.Fatal("refresh replaced the source editor's mixed-text selection or focus")
			}
			field.IMECommit("سلام")
			a.PumpOnce()
			if field.Text != "Song سلام" || field.Caret() != 9 || calls != 6 || !reflect.DeepEqual(p.body.list.indices, []int{2}) {
				t.Fatal("IME replacement did not update the Arabic query and filtered track exactly once")
			}
			if !p.Main.AccessibleAction(widget.ItemID(field, 0), a11y.ActionDefault) || field.Text != "" || calls != 7 || p.body.list.Count != 3 {
				t.Fatal("complex-text clear failed to restore the full queue exactly once")
			}
			if p.Transport.State != players.Paused || p.Transport.Pos != 13*time.Second || p.Transport.List.Index() != 0 {
				t.Fatal("complex metadata/search changed playback")
			}
		})
	}
}
