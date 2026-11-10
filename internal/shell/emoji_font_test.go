//go:build font_emoji

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

// Emoji must be visible, retain their own colours over each skin's row ink,
// and keep those colours when a later emoji grows the toolkit's glyph sheet.
func TestColorEmojiPlaylistMetadataStaysVisibleAcrossAppearancesAndCacheGrowth(t *testing.T) {
	t.Setenv(style.SystemFontsEnv, "1")
	if !style.FontInstalled("Noto Color Emoji") {
		t.Skip("Noto Color Emoji is not installed")
	}
	for _, skin := range players.AppearanceIDs() {
		for _, scale := range []float32{1, 1.75} {
			t.Run(fmt.Sprintf("%s/%gx", skin, scale), func(t *testing.T) {
				a, s := openAtScale(t, skin, scale)
				s.Pulse().Stop()
				m := s.Host.Model
				m.Transport.List = players.NewPlaylist(players.Track{Title: "Song", Length: 3 * time.Minute})
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
						t.Fatal("emoji metadata has no visible native row")
					}
					im, r := w.CaptureSurface(), row.Bounds
					x0, y0 := max(0, int(math.Floor(float64(r.Min.X)))), max(0, int(math.Floor(float64(r.Min.Y))))
					x1, y1 := min(im.Width, int(math.Ceil(float64(r.Max.X)))), min(im.Height, int(math.Ceil(float64(r.Max.Y))))
					var pixels []byte
					for y := y0; y < y1; y++ {
						at := (y*im.Width + x0) * 4
						pixels = append(pixels, im.Pix[at:at+(x1-x0)*4]...)
					}
					return pixels
				}
				plain, first := capture("Song"), capture("Song 😀")
				yellow := 0
				for i := 0; i+3 < len(first) && i+3 < len(plain); i += 4 {
					if !bytes.Equal(first[i:i+4], plain[i:i+4]) && first[i] > 160 && first[i+1] > 90 && int(first[i])-int(first[i+2]) > 90 && first[i+3] > 128 {
						yellow++
					}
				}
				if yellow == 0 {
					t.Fatal("colour emoji is clipped, absent or tinted to the skin's text ink")
				}
				if bytes.Equal(first, capture("Song 😎")) || !bytes.Equal(first, capture("Song 😀")) {
					t.Fatal("later emoji cache growth changed the previously rendered metadata")
				}
				if m.Transport.State != players.Paused || m.Transport.Pos != 13*time.Second || m.Transport.List.Index() != 0 {
					t.Fatal("emoji metadata refresh changed playback")
				}
			})
		}
	}
}
