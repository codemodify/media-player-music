//go:build font_complex || font_emoji

package shell

import (
	"bytes"
	"fmt"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/codemodify/media-player-music/internal/players"
	"goframework.io/desktop-ui/a11y"
	"goframework.io/desktop-ui/style"
)

// Complex-script lending is opt-in and uses the toolkit's default families.
// No app configuration supplies the lenders. Fallback-only deliberately
// excludes these scripts, so that build is excluded.
func TestScriptLendingTitlesInActualPlayerRows(t *testing.T) {
	t.Setenv(style.SystemFontsEnv, "1")
	for _, name := range []string{"Noto Sans Arabic", "Noto Sans Hebrew", "Noto Sans Devanagari", "Noto Sans Thai"} {
		if !style.FontInstalled(name) {
			t.Skipf("default-script coverage test font %s is not installed", name)
		}
	}
	for _, skin := range players.AppearanceIDs() {
		for _, scale := range []float32{1, 1.75} {
			t.Run(fmt.Sprintf("%s/%gx", skin, scale), func(t *testing.T) {
				a, s := openAtScale(t, skin, scale)
				s.Pulse().Stop()
				m := s.Host.Model
				m.Transport.List = players.NewPlaylist(players.Track{Title: "Song سلام", Length: 3 * time.Minute})
				m.Transport.State, m.Transport.Pos = players.Paused, 13*time.Second
				capture := func(title string) []byte {
					m.Transport.List.Tracks[0].Title = title
					s.refresh()
					a.PumpOnce()
					w := nativePlaylistWindow(s)
					var row *a11y.Node
					w.AccessibleTree().Walk(func(n *a11y.Node) bool {
						if n.Role == a11y.RoleListItem && strings.Contains(n.Name, title) {
							row = n
							return false
						}
						return true
					})
					if row == nil || row.Bounds.Empty() {
						t.Fatal("native title row missing")
					}
					im := w.CaptureSurface()
					r := row.Bounds
					x0, y0 := max(0, int(math.Floor(float64(r.Min.X)))), max(0, int(math.Floor(float64(r.Min.Y))))
					x1, y1 := min(im.Width, int(math.Ceil(float64(r.Max.X)))), min(im.Height, int(math.Ceil(float64(r.Max.Y))))
					var pixels []byte
					for y := y0; y < y1; y++ {
						at := (y*im.Width + x0) * 4
						pixels = append(pixels, im.Pix[at:at+(x1-x0)*4]...)
					}
					return pixels
				}
				for _, pair := range [][2]string{{"Song سلام", "Song كتاب"}, {"Song שלום", "Song ספרי"}, {"Song नमस्ते", "Song पुस्तक"}, {"Song ภาษา", "Song เพลง"}} {
					if bytes.Equal(capture(pair[0]), capture(pair[1])) {
						t.Fatalf("distinct script titles still paint identical boxes: %q / %q", pair[0], pair[1])
					}
				}
				if m.Transport.State != players.Paused || m.Transport.Pos != 13*time.Second {
					t.Fatal("script title refresh changed playback")
				}
			})
		}
	}
}
