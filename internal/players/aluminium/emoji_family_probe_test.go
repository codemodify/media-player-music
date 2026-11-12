//go:build font_emoji && music_player_emoji_family_probe

package aluminium

import (
	"fmt"
	"math"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/codemodify/media-player-music/internal/players"
	"goframework.io/desktop-ui/style"
	"goframework.io/graphics-paint2d"
)

// This explicit diagnostic tag keeps a regression in an optional toolkit
// feature visible without adding a workaround or weakening its expected
// composition. Run with theme_engine_all,font_emoji,music_player_emoji_family_probe.
func TestEmojiFamilyCompositionInSourceSearch(t *testing.T) {
	checkSourceEmojiComposition(t, "👩‍👩‍👧‍👦")
}

func TestEmojiKeycapCompositionInSourceSearch(t *testing.T) {
	checkSourceEmojiComposition(t, "1️⃣")
}

func TestEmojiRainbowCompositionInSourceSearch(t *testing.T) {
	checkSourceEmojiComposition(t, "🏳️‍🌈")
}

func TestEmojiUnqualifiedKeycapCompositionInSourceSearch(t *testing.T) {
	checkSourceEmojiComposition(t, "1⃣")
}

func TestEmojiCoupleCompositionInSourceSearch(t *testing.T) {
	checkSourceEmojiComposition(t, "👩‍❤️‍👩")
}

func TestTextKeycapPresentationInSourceSearch(t *testing.T) {
	t.Setenv(style.SystemFontsEnv, "1")
	if !style.FontInstalled("Noto Color Emoji") || !style.FontInstalled("Noto Sans Symbols") {
		t.Skip("Noto Color Emoji and Noto Sans Symbols are not installed")
	}
	for _, scale := range []float32{1, 1.75} {
		t.Run(fmt.Sprintf("%gx", scale), func(t *testing.T) {
			a, p := open(t, scale)
			p.Transport.List = players.NewPlaylist(players.Track{Title: "A\u20e3"}, players.Track{Title: "1\ufe0e\u20e3"})
			p.Transport.State, p.Transport.Pos = players.Paused, 13*time.Second
			p.Refresh()
			a.PumpOnce()
			field := p.body.search
			p.Main.RequestFocus(field)
			calls, input := 0, field.OnInput
			field.OnInput = func(s string) { calls++; input(s) }
			for i, text := range []string{"A\u20e3", "1\ufe0e\u20e3"} {
				field.SelectAll()
				field.IMECommit(text)
				a.PumpOnce()
				if field.Text != text || field.Caret() != utf8.RuneCountInString(text) || calls != i+1 || p.body.list.Count != 1 || p.body.list.queueIndex(0) != i {
					t.Fatal("text-keycap IME search changed its sequence, callback or queue mapping")
				}
				f := field.Look().Font()
				draw := func(ink paint2d.Color) *paint2d.Image {
					im := paint2d.NewImage(512, 128)
					f.Draw(paint2d.NewContext(im), text, paint2d.Pt(16, 16), ink)
					return im
				}
				white, red := draw(paint2d.White), draw(paint2d.RGB(1, 0, 0))
				visible := 0
				for y := 0; y < white.Height; y++ {
					for x := 0; x < white.Width; x++ {
						if original := white.NRGBAAt(x, y); original.A > 40 {
							visible++
							if tinted := red.NRGBAAt(x, y); tinted.G > 2 || tinted.B > 2 || math.Abs(float64(int(tinted.A)-int(original.A))) > 1 {
								t.Fatalf("source editor sends text form %q to untinted emoji artwork", text)
							}
						}
					}
				}
				if visible == 0 {
					t.Fatal("source editor did not draw the text-keycap fixture")
				}
				if p.Transport.State != players.Paused || p.Transport.Pos != 13*time.Second || p.Transport.List.Index() != 0 {
					t.Fatal("text-keycap search changed playback")
				}
			}
		})
	}
}

// Count actual visible glyph cells from the editor's face. A zero-advance
// outline overlay can leave the total width unchanged while drawing twice.
type emojiCellDevice struct {
	paint2d.Device
	cells int
}

func (d *emojiCellDevice) Blit(src *paint2d.Image, from, to paint2d.Rect, m paint2d.Matrix, p paint2d.Paint, clip paint2d.Clip) {
	d.cells++
	d.Device.Blit(src, from, to, m, p, clip)
}

func checkSourceEmojiComposition(t *testing.T, sequence string) {
	t.Helper()
	t.Setenv(style.SystemFontsEnv, "1")
	if !style.FontInstalled("Noto Color Emoji") {
		t.Skip("Noto Color Emoji is not installed")
	}
	for _, scale := range []float32{1, 1.75} {
		t.Run(fmt.Sprintf("%gx", scale), func(t *testing.T) {
			a, p := open(t, scale)
			p.Transport.List = players.NewPlaylist(players.Track{Title: "Emoji " + sequence})
			p.Transport.State, p.Transport.Pos = players.Paused, 13*time.Second
			p.Refresh()
			a.PumpOnce()
			field := p.body.search
			p.Main.RequestFocus(field)
			calls, input := 0, field.OnInput
			field.OnInput = func(s string) { calls++; input(s) }
			field.IMECommit(sequence)
			a.PumpOnce()
			if field.Text != sequence || field.Caret() != utf8.RuneCountInString(sequence) || calls != 1 || p.body.list.Count != 1 || p.Transport.State != players.Paused || p.Transport.Pos != 13*time.Second {
				t.Fatal("emoji query changed its sequence, queue mapping, callbacks or playback")
			}
			f := field.Look().Font()
			one, joined := f.Advance("👩"), f.Advance(sequence)
			if one <= 0 || math.Abs(float64(joined-one)) > .2 {
				t.Fatalf("source editor split a one-cell sequence %q: member%g sequence%g", sequence, one, joined)
			}
			dev := &emojiCellDevice{Device: paint2d.NewCPUDevice(paint2d.NewImage(512, 128))}
			f.Draw(paint2d.NewContextDevice(dev), sequence, paint2d.Pt(16, 16), paint2d.White)
			if dev.cells != 1 {
				t.Fatalf("source editor draws %q as %d visible glyph cells, want one", sequence, dev.cells)
			}
		})
	}
}
