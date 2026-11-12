package panel

import (
	"fmt"
	"testing"
	"time"

	"goframework.io/desktop-ui-skins"
	"goframework.io/desktop-ui/platform"
)

// Dossier's text roles (its selection and its analyser ramp) must affect the
// actual application-painted rows and analyser, as well as its artwork.
func TestDossierUsesItsPlaylistAndVisualizerColors(t *testing.T) {
	for _, scale := range []float32{1, 1.75} {
		t.Run(fmt.Sprintf("%gx", scale), func(t *testing.T) {
			a, p := open(t, SkinClassified, scale)
			p.Pose(97 * time.Second)
			p.List().RequestFocus(p.listPane.list)
			for range 2 {
				p.List().Inject(platform.Event{Kind: platform.EventKeyDown, Key: platform.KeyDown})
			}
			a.PumpOnce()
			if p.listPane.list.Selected != 2 || p.Transport.List.Index() != 0 {
				t.Fatal("browsing the imported skin changed playback")
			}
			sk, ok := skins.LoadSkin(SkinClassified)
			if !ok || sk.Text["player.selected"] == nil || sk.Text["player.peak"] == nil || sk.Text["player.ramp.0"] == nil || sk.Text["player.ramp.15"] == nil {
				t.Fatal("the pack lacks its playlist and analyser colours")
			}
			sel := sk.Text["player.selected"].Color.Clamp().NRGBA()
			row := p.listPane.list.AccessibleItems()[2].Bounds
			origin := p.List().WindowRect().Min
			img := p.List().Capture()
			selectedPixels := 0
			for y := max(int(row.Min.Y-origin.Y), 0); y < min(int(row.Max.Y-origin.Y), img.Height); y++ {
				for x := max(int(row.Min.X-origin.X), 0); x < min(int(row.Max.X-origin.X), img.Width); x++ {
					r, g, b, alpha := img.PremulAt(x, y)
					if r == sel.R && g == sel.G && b == sel.B && alpha == 255 {
						selectedPixels++
					}
				}
			}
			if selectedPixels < 20 {
				t.Fatalf("Dossier's selection %v did not paint its highlighted row: %d pixels", sel, selectedPixels)
			}
			analyser := p.strip.analyser
			if len(analyser.Ramp) != 16 {
				t.Fatalf("analyser has %d imported spectrum colours, want 16", len(analyser.Ramp))
			}
			if analyser.PeakTint != sk.Text["player.peak"].Color || analyser.Ramp[0] != sk.Text["player.ramp.0"].Color || analyser.Ramp[15] != sk.Text["player.ramp.15"].Color {
				t.Fatalf("analyser did not consume Dossier's ramp: peak=%v floor=%v top=%v", analyser.PeakTint, analyser.Ramp[0], analyser.Ramp[15])
			}
		})
	}
}
