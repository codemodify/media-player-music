//go:build font_fallback || font_complex || font_emoji

package shell

import (
	"bytes"
	"fmt"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/codemodify/media-player-music/internal/players"
	"github.com/codemodify/media-player-music/internal/players/playertest"
	"goframework.io/desktop-ui/a11y"
)

// Compare the same row after metadata replacement: comparing two different
// rows could pass merely because their row numbers or highlights differ.
// Source bitmap readouts retain their own alphabet; these are native-font rows.
func TestUnicodePlaylistTitlesRenderDistinctlyAcrossAppearances(t *testing.T) {
	playertest.RequireCJKFallback(t)
	for _, id := range players.AppearanceIDs() {
		for _, scale := range []float32{1, 1.75} {
			t.Run(fmt.Sprintf("%s/%gx", id, scale), func(t *testing.T) {
				a, s := openAtScale(t, id, scale)
				s.Pulse().Stop()
				m := s.Host.Model
				m.Transport.List = players.NewPlaylist(players.Track{Title: "Song 山川", Length: 3 * time.Minute})
				m.Transport.State, m.Transport.Pos = players.Paused, 13*time.Second
				capture := func(title string) ([]byte, string) {
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
						t.Fatalf("title %q has no visible playlist row", title)
					}
					im := w.CaptureSurface()
					r := row.Bounds
					x0, y0 := max(0, int(math.Floor(float64(r.Min.X)))), max(0, int(math.Floor(float64(r.Min.Y))))
					x1, y1 := min(im.Bounds().Dx(), int(math.Ceil(float64(r.Max.X)))), min(im.Bounds().Dy(), int(math.Ceil(float64(r.Max.Y))))
					var pixels []byte
					for y := y0; y < y1; y++ {
						at := (y*im.Width + x0) * 4
						pixels = append(pixels, im.Pix[at:at+(x1-x0)*4]...)
					}
					return pixels, row.Name
				}
				first, nameA := capture("Song 山川")
				second, nameB := capture("Song 海河")
				if len(first) == 0 || nameA == nameB || bytes.Equal(first, second) {
					t.Fatal("distinct CJK metadata still paints an identical playlist row")
				}
				if m.Transport.State != players.Paused || m.Transport.Pos != 13*time.Second || m.Transport.List.Index() != 0 {
					t.Fatal("font/metadata refresh changed playback")
				}
			})
		}
	}
}
