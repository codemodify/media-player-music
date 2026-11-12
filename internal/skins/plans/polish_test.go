package plans

import (
	"image/color"
	"slices"
	"testing"

	"github.com/codemodify/media-player-music/internal/skins/panel"
	"goframework.io/desktop-ui-skingen"
)

// The 2x transport artwork may refine edges, but must not invent colors or
// alter the original 1x drawing. Test the public renderer's selected callback.
func TestPolishedTransportUsesOnlyOriginalColors(t *testing.T) {
	for _, plan := range Plans() {
		if plan.Name != "slate" || !slices.ContainsFunc(plan.Layouts, func(l LayoutSpec) bool { return l.Name == panel.LayoutPlaylist }) {
			continue
		}
		t.Run(plan.Name, func(t *testing.T) {
			var cell Cell
			var sheet *Sheet
			for _, sh := range plan.Sheets {
				for _, c := range sh.Cells {
					if c.Name == "key.play" {
						cell, sheet = c, sh
					}
				}
			}
			if sheet == nil || sheet.Pixelated {
				t.Fatal("transport must select independently authored 2x artwork")
			}
			cell.X, cell.Y = 0, 0
			local := &Sheet{W: cell.W, H: cell.H, Cells: []Cell{cell}}
			one, two := skingen.Render(local, 1), skingen.Render(local, 2)
			palette := make(map[color.NRGBA]bool)
			for y := 0; y < one.Height; y++ {
				for x := 0; x < one.Width; x++ {
					palette[one.NRGBAAt(x, y)] = true
				}
			}
			changed := 0
			for y := 0; y < two.Height; y++ {
				for x := 0; x < two.Width; x++ {
					got := two.NRGBAAt(x, y)
					if !palette[got] {
						t.Fatalf("2x transport invented color %v at %d,%d", got, x, y)
					}
					if got != one.NRGBAAt(x/2, y/2) {
						changed++
					}
				}
			}
			if changed == 0 {
				t.Fatal("2x transport is still only a duplicate of its 1x pixels")
			}
		})
	}
}
