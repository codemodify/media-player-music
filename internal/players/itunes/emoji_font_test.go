//go:build font_emoji

package itunes

import (
	"bytes"
	"fmt"
	"math"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/codemodify/media-player-music/internal/players"
	"goframework.io/desktop-ui-skins"
	"goframework.io/desktop-ui/style"
	"goframework.io/graphics-paint2d"
)

func TestColorEmojiFullMiniReadoutsAndSourceSearchKeepWholeSequences(t *testing.T) {
	t.Setenv(style.SystemFontsEnv, "1")
	if !style.FontInstalled("Noto Color Emoji") {
		t.Skip("Noto Color Emoji is not installed")
	}
	for _, scale := range []float32{1, 1.75} {
		t.Run(fmt.Sprintf("%gx", scale), func(t *testing.T) {
			a, p := open(t, scale)
			p.Transport.List = players.NewPlaylist(players.Track{Title: "Song", Length: 3 * time.Minute})
			p.Transport.State, p.Transport.Pos = players.Paused, 13*time.Second
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
					t.Fatal("mode has no metadata readout")
				}
				capture := func(title string) []byte {
					p.Transport.List.Tracks[0].Title = title
					p.Refresh()
					a.PumpOnce()
					return crop(r)
				}
				plain, first := capture("Song"), capture("Song 😀")
				yellow := 0
				for i := 0; i+3 < len(first) && i+3 < len(plain); i += 4 {
					if !bytes.Equal(first[i:i+4], plain[i:i+4]) && first[i] > 160 && first[i+1] > 90 && int(first[i])-int(first[i+2]) > 90 && first[i+3] > 128 {
						yellow++
					}
				}
				if yellow == 0 || bytes.Equal(first, capture("Song 😎")) || !bytes.Equal(first, capture("Song 😀")) {
					t.Fatalf("mini%v readout lost emoji colour, baseline or cached source pixels", mini)
				}
				// A selector changes only its own occurrence. The second
				// heart stays text even when the first explicitly asks for
				// emoji; FE0E must produce the same text presentation.
				textHeart := capture("❤︎")
				mixed := capture("❤️ ❤︎")
				if !bytes.Equal(textHeart, capture("❤")) || !bytes.Equal(mixed, capture("❤️ ❤")) || bytes.Equal(mixed, capture("❤️ ❤️")) {
					t.Fatalf("mini%v readout applies emoji preference to the whole string or ignores FE0E", mini)
				}
			}
			p.SetMiniPlayer(false)
			a.PumpOnce()
			p.Advance(0)
			a.PumpOnce()
			field := p.body.search
			p.Main.RequestFocus(field)
			calls := 0
			input := field.OnInput
			field.OnInput = func(s string) { calls++; input(s) }
			first := "Song 👍🏽"
			p.Transport.List.Tracks[0].Title = first
			p.Refresh()
			field.IMECommit(first)
			a.PumpOnce()
			f := field.Look().Font()
			mixedWidth := f.Advance("L 👀 R")
			separateWidth := f.Advance("L ") + f.Advance("👀") + f.Advance(" R")
			if math.Abs(float64(mixedWidth-separateWidth)) > .2 {
				t.Fatalf("source editor borrows emoji whitespace: mixed%g separate%g", mixedWidth, separateWidth)
			}
			base, toned := f.Advance("👍"), f.Advance("👍🏽")
			if base <= 0 || math.Abs(float64(base-toned)) > .2 {
				t.Fatalf("source editor splits the skin-tone sequence: base%g toned%g", base, toned)
			}
			if field.Text != first || field.Caret() != utf8.RuneCountInString(first) || calls != 1 || p.body.list.Count != 1 {
				t.Fatal("IME emoji search lost its sequence, caret or input callback")
			}
			second := "Song ❤️"
			p.Transport.List.Tracks[0].Title = second
			p.Refresh()
			field.SelectAll()
			field.IMECommit(second)
			a.PumpOnce()
			if field.Text != second || field.Caret() != utf8.RuneCountInString(second) || calls != 2 || p.body.list.Count != 1 {
				t.Fatal("variation-selector search replacement changed the sequence or filter")
			}
			if p.Transport.State != players.Paused || p.Transport.Pos != 13*time.Second || p.Transport.List.Index() != 0 {
				t.Fatal("emoji display/search changed playback")
			}
		})
	}
}
