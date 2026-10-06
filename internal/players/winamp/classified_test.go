package winamp

import (
	"fmt"
	"testing"
	"time"

	"github.com/codemodify/uitoolkit/platform"
)

// Classified's imported pledit.txt and viscolor.txt must affect the actual
// application-painted rows and analyser, as well as its bitmap artwork.
func TestClassifiedUsesItsOriginalPlaylistAndVisualizerColors(t *testing.T) {
	for _, scale := range []float32{1, 1.75} {
		t.Run(fmt.Sprintf("%gx", scale), func(t *testing.T) {
			a, p := open(t, SkinClassified, scale)
			p.Pose(97 * time.Second)
			p.List.RequestFocus(p.listPane.list)
			for range 2 {
				p.List.Inject(platform.Event{Kind: platform.EventKeyDown, Key: platform.KeyDown})
			}
			a.PumpOnce()
			if p.listPane.list.Selected != 2 || p.Transport.List.Index() != 0 {
				t.Fatal("browsing the imported skin changed playback")
			}
			row := p.listPane.list.AccessibleItems()[2].Bounds
			origin := p.List.WindowRect().Min
			img := p.List.Capture()
			selectedPixels := 0
			for y := max(int(row.Min.Y-origin.Y), 0); y < min(int(row.Max.Y-origin.Y), img.Height); y++ {
				for x := max(int(row.Min.X-origin.X), 0); x < min(int(row.Max.X-origin.X), img.Width); x++ {
					r, g, b, alpha := img.PremulAt(x, y)
					if r == 0x6b && g == 0x6b && b == 0x6f && alpha == 255 {
						selectedPixels++
					}
				}
			}
			if selectedPixels < 20 {
				t.Fatalf("Classified's original #6B6B6F selection did not paint its highlighted row: %d pixels", selectedPixels)
			}
			analyser := p.strip.analyser
			if len(analyser.Ramp) != 16 {
				t.Fatalf("analyser has %d imported spectrum colours, want 16", len(analyser.Ramp))
			}
			if analyser.PeakTint != rgb(150, 150, 150) || analyser.Ramp[0] != rgb(24, 132, 8) || analyser.Ramp[15] != rgb(239, 49, 16) {
				t.Fatalf("analyser did not consume Classified's original viscolor ramp: peak=%v floor=%v top=%v", analyser.PeakTint, analyser.Ramp[0], analyser.Ramp[15])
			}
		})
	}
}
