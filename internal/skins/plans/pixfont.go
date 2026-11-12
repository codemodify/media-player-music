package plans

import (
	"fmt"
	"sort"

	"goframework.io/graphics-paint2d"
)

// The two panel skins (Slate, Silver) print their displays in an alphabet of their
// own: a small capital face six pixels tall, and a set of segmented digits
// for the clock.
//
// They are drawn here rather than borrowed because a player's display
// alphabet is exactly the kind of bitmap the licensing rule is about. Every
// glyph below was set by hand on this grid; none is traced
// from, or sized to match, any other player's text or number sheet. What
// they share with the era is only the idea — capitals on a five-by-six grid
// and a seven-segment clock were what every display of the kind used,
// because at that size there is little else a letter can be.
//
// The glyphs reach the app as ordinary sprites named "font.<hex>" (the rune
// in lower-case hex: "font.41" is A) and "led.<digit>". The app measures a
// line with style.SkinSpriteSize and sets it glyph by glyph; a letter's
// advance is its sprite's width plus one pixel of space.

// pixGlyphs is the capital face. Each glyph is six rows of '#' and '.', as
// wide as the letter needs: three pixels for the narrow ones, five for M, W
// and the like, four for everything else.
var pixGlyphs = map[rune][]string{
	'A': {".##.", "#..#", "#..#", "####", "#..#", "#..#"},
	'B': {"###.", "#..#", "###.", "#..#", "#..#", "###."},
	'C': {".##.", "#..#", "#...", "#...", "#..#", ".##."},
	'D': {"###.", "#..#", "#..#", "#..#", "#..#", "###."},
	'E': {"####", "#...", "###.", "#...", "#...", "####"},
	'F': {"####", "#...", "###.", "#...", "#...", "#..."},
	'G': {".##.", "#...", "#.##", "#..#", "#..#", ".###"},
	'H': {"#..#", "#..#", "####", "#..#", "#..#", "#..#"},
	'I': {"###", ".#.", ".#.", ".#.", ".#.", "###"},
	'J': {"..##", "...#", "...#", "...#", "#..#", ".##."},
	'K': {"#..#", "#.#.", "##..", "#.#.", "#..#", "#..#"},
	'L': {"#...", "#...", "#...", "#...", "#...", "####"},
	'M': {"#...#", "##.##", "#.#.#", "#...#", "#...#", "#...#"},
	'N': {"#..#", "##.#", "#.##", "#..#", "#..#", "#..#"},
	'O': {".##.", "#..#", "#..#", "#..#", "#..#", ".##."},
	'P': {"###.", "#..#", "#..#", "###.", "#...", "#..."},
	'Q': {".##.", "#..#", "#..#", "#..#", "#.#.", ".#.#"},
	'R': {"###.", "#..#", "#..#", "###.", "#.#.", "#..#"},
	'S': {".###", "#...", ".##.", "...#", "...#", "###."},
	'T': {"#####", "..#..", "..#..", "..#..", "..#..", "..#.."},
	'U': {"#..#", "#..#", "#..#", "#..#", "#..#", ".##."},
	'V': {"#...#", "#...#", "#...#", ".#.#.", ".#.#.", "..#.."},
	'W': {"#...#", "#...#", "#...#", "#.#.#", "##.##", "#...#"},
	'X': {"#..#", "#..#", ".##.", ".##.", "#..#", "#..#"},
	'Y': {"#...#", "#...#", ".#.#.", "..#..", "..#..", "..#.."},
	'Z': {"####", "...#", "..#.", ".#..", "#...", "####"},

	'0': {".##.", "#..#", "#.##", "##.#", "#..#", ".##."},
	'1': {".#.", "##.", ".#.", ".#.", ".#.", "###"},
	'2': {".##.", "#..#", "..#.", ".#..", "#...", "####"},
	'3': {"###.", "...#", ".##.", "...#", "...#", "###."},
	'4': {"#..#", "#..#", "####", "...#", "...#", "...#"},
	'5': {"####", "#...", "###.", "...#", "...#", "###."},
	'6': {".##.", "#...", "###.", "#..#", "#..#", ".##."},
	'7': {"####", "...#", "..#.", ".#..", ".#..", ".#.."},
	'8': {".##.", "#..#", ".##.", "#..#", "#..#", ".##."},
	'9': {".##.", "#..#", "#..#", ".###", "...#", ".##."},

	' ':  {"...", "...", "...", "...", "...", "..."},
	'.':  {".", ".", ".", ".", ".", "#"},
	',':  {"..", "..", "..", "..", ".#", "#."},
	':':  {".", "#", ".", ".", "#", "."},
	';':  {"..", ".#", "..", "..", ".#", "#."},
	'-':  {"...", "...", "###", "...", "...", "..."},
	'(':  {".#", "#.", "#.", "#.", "#.", ".#"},
	')':  {"#.", ".#", ".#", ".#", ".#", "#."},
	'[':  {"##", "#.", "#.", "#.", "#.", "##"},
	']':  {"##", ".#", ".#", ".#", ".#", "##"},
	'/':  {"..#", "..#", ".#.", ".#.", "#..", "#.."},
	'\'': {"#", "#", ".", ".", ".", "."},
	'"':  {"#.#", "#.#", "...", "...", "...", "..."},
	'&':  {".##..", "#..#.", ".##..", "#.#.#", "#..#.", ".##.#"},
	'!':  {"#", "#", "#", "#", ".", "#"},
	'?':  {".##.", "#..#", "..#.", ".#..", "....", ".#.."},
	'+':  {"...", ".#.", "###", ".#.", "...", "..."},
	'=':  {"...", "###", "...", "###", "...", "..."},
	'_':  {"....", "....", "....", "....", "....", "####"},
	'*':  {"...", "#.#", ".#.", "#.#", "...", "..."},
	'#':  {".#.#.", "#####", ".#.#.", ".#.#.", "#####", ".#.#."},
	'%':  {"#..#", "...#", "..#.", ".#..", "#...", "#..#"},
	'<':  {"..#", ".#.", "#..", ".#.", "..#", "..."},
	'>':  {"#..", ".#.", "..#", ".#.", "#..", "..."},
	'·':  {".", ".", "#", ".", ".", "."},
	'—':  {".....", ".....", "#####", ".....", ".....", "....."},
}

// PixFontRunes is every rune the capital face draws, in order. A line the
// app sets is folded to upper case first, and a rune missing here is drawn
// as a question mark rather than as nothing, so a gap in the face shows.
func PixFontRunes() []rune {
	out := make([]rune, 0, len(pixGlyphs))
	for r := range pixGlyphs {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// PixFontSprite is the sprite name a glyph is published under.
func PixFontSprite(r rune) string { return fmt.Sprintf("font.%x", r) }

// panel places a panel sprite: a piece of a window's picture that the app
// draws at its own size — a face, a key, a digit, a lamp.
//
// It is unsliced. A panel's pieces are drawn where the layout puts them and
// at the size they were drawn, and the toolkit magnifies a pixel sheet's
// pieces onto the device grid at every scale (style.DrawSkinSprite). They
// used to carry an empty one-pixel margin, sliced there, so that a pixel
// piece stretched nearest instead of being drawn at 1× in the middle of its
// box; the toolkit does that itself now, on a grid every piece shares, which
// the margin could not give.
func (l *lay) panel(name string, w, h int, tint bool, draw func(ctx *paint2d.Context, w, h float32)) {
	l.cell(name, w, h, [4]int{}, "", tint, draw)
}

// pixText lays the whole capital face out as tinted panel sprites, one row
// of cells, each glyph drawn as whole pixels by dot.
func pixText(l *lay, dot func(ctx *paint2d.Context, x, y float32)) {
	l.row(6)
	for _, r := range PixFontRunes() {
		rows := pixGlyphs[r]
		w := len(rows[0])
		l.panel(PixFontSprite(r), w, 6, true, func(ctx *paint2d.Context, _, _ float32) {
			for y, line := range rows {
				for x, c := range line {
					if c == '#' {
						dot(ctx, float32(x), float32(y))
					}
				}
			}
		})
	}
}

// ---- the segmented clock ------------------------------------------------------

// segDigits says which of the seven segments each digit lights, in the usual
// order: a top, b upper right, c lower right, d bottom, e lower left, f upper
// left, g middle.
var segDigits = map[rune]string{
	'0': "abcdef", '1': "bc", '2': "abdeg", '3': "abcdg", '4': "bcfg",
	'5': "acdfg", '6': "acdefg", '7': "abc", '8': "abcdefg", '9': "abcdfg",
	'-': "g",
}

// segDigit draws one seven-segment digit w by h, with strokes t wide and a
// pixel's gap where two segments meet — the gap is what makes it read as a
// display rather than as a font.
func segDigit(ctx *paint2d.Context, w, h, t float32, segs string, col paint2d.Color) {
	mid := float32(int((h - t) / 2))
	for _, s := range segs {
		switch s {
		case 'a':
			px(ctx, t, 0, w-2*t, t, col)
		case 'b':
			px(ctx, w-t, t, t, mid-t, col)
		case 'c':
			px(ctx, w-t, mid+t, t, h-mid-2*t, col)
		case 'd':
			px(ctx, t, h-t, w-2*t, t, col)
		case 'e':
			px(ctx, 0, mid+t, t, h-mid-2*t, col)
		case 'f':
			px(ctx, 0, t, t, mid-t, col)
		case 'g':
			px(ctx, t, mid, w-2*t, t, col)
		}
	}
}

// ---- the dot-matrix clock -------------------------------------------------------

// dotDigits is a five-by-seven face for a dot-matrix clock: fat enough to
// read at a glance from across a desk, which is what the big clock on a
// display of that kind was for.
var dotDigits = map[rune][]string{
	'0': {".###.", "#...#", "#..##", "#.#.#", "##..#", "#...#", ".###."},
	'1': {"..#..", ".##..", "..#..", "..#..", "..#..", "..#..", ".###."},
	'2': {".###.", "#...#", "....#", "...#.", "..#..", ".#...", "#####"},
	'3': {"####.", "....#", "....#", ".###.", "....#", "....#", "####."},
	'4': {"...#.", "..##.", ".#.#.", "#..#.", "#####", "...#.", "...#."},
	'5': {"#####", "#....", "####.", "....#", "....#", "#...#", ".###."},
	'6': {".###.", "#....", "#....", "####.", "#...#", "#...#", ".###."},
	'7': {"#####", "....#", "...#.", "..#..", ".#...", ".#...", ".#..."},
	'8': {".###.", "#...#", "#...#", ".###.", "#...#", "#...#", ".###."},
	'9': {".###.", "#...#", "#...#", ".####", "....#", "....#", ".###."},
	'-': {".....", ".....", ".....", "#####", ".....", ".....", "....."},
}
