package minim

import (
	"math"
	"strings"

	"github.com/codemodify/paintengine2d"
	"github.com/codemodify/media-player-music/internal/players"
	"github.com/codemodify/media-player-music/internal/players/minim/panel"
	"github.com/codemodify/uitoolkit/style"
	"github.com/codemodify/uitoolkit/widget"
)

// The faces.
//
// Minim wears three skins, and two of them are not dressings for its widgets
// but *panels*: pictures of the whole window with holes where the keys go.
// Each states where its holes are as a fixed layout — named slots at design
// coordinates (docs/skins.md, "Fixed layouts") — and the player binds its
// controls to the slot names and lets the skin place them. It carries no
// rect of its own for either panel: a key is where the skin it is wearing
// says, so a panel someone else draws for Minim puts its keys wherever its
// own art has them.
//
// The widget tree is the same one in every face: switching skins moves and
// repaints the controls, hides the few one face has and another does not,
// and rebuilds nothing, so the focus, the tab order and the accessibility
// tree survive a switch the way they survive Lantern's.
//
// A pack with no Minim layout — the "minim" skin itself, and every other one
// of the hundred and twenty-odd — gets the widget face: the ordinary
// layout, painted by whatever the look paints.

// face is which of the layouts a look gets.
type face uint8

const (
	faceWidgets face = iota // any pack without Minim's layouts
	faceClassic             // "minim-classic"
	faceSilver              // "minim-silver"
)

// The layouts a panel skin states for the three windows, and the frame role
// each window is given so a skin can dress it differently (the silver
// equaliser's tab). These names are the player's vocabulary; the rects are
// the skin's.
const (
	layoutStrip     = panel.LayoutStrip
	layoutEqualiser = panel.LayoutEqualiser
	layoutPlaylist  = panel.LayoutPlaylist
)

// faceOf is the face a look gets. A look is a panel when its skin states the
// strip's layout; which panel it is — and so which ink its rows and its
// analyser are set in — is the pack's name, and a panel skin the player does
// not know by name is set in the classic ink.
func faceOf(lk style.LookAndFeel) face {
	if _, ok := style.SkinLayoutOf(lk, layoutStrip); !ok {
		return faceWidgets
	}
	if style.LookAppearance(lk).Name == SkinSilver {
		return faceSilver
	}
	return faceClassic
}

// panelled reports whether the face is a panel.
func (f face) panelled() bool { return f != faceWidgets }

// ---- the ink a face sets text in -------------------------------------------------

// ink is the colours a panel face draws in where the app sets text or draws a
// line itself rather than painting a sprite: the playlist's rows, the
// equaliser's curve, the analyser's bars. They belong to the face — the art
// the rows sit on is the face's — and so they live beside its layout rather
// than in the look's palette, which a menu the player opens also reads.
type ink struct {
	text, row, current, selected, selectedText, info paintengine2d.Color
	curve, peak                                      paintengine2d.Color
	ramp                                             []paintengine2d.Color
	// rowSize is the size the playlist's rows are set at, in design
	// pixels, and rowBold their weight.
	rowSize float32
	rowBold bool
}

func (f face) ink() *ink {
	switch f {
	case faceClassic:
		return &classicInk
	case faceSilver:
		return &silverInk
	}
	return nil
}

var classicInk = ink{
	text:         rgb(0x00, 0xe0, 0x00),
	row:          rgb(0x00, 0xd0, 0x00),
	current:      rgb(0xff, 0xff, 0xff),
	selected:     rgb(0x00, 0x00, 0xb8),
	selectedText: rgb(0xff, 0xff, 0xff),
	info:         rgb(0x00, 0xe0, 0x00),
	curve:        rgb(0xe4, 0xd1, 0x2f),
	peak:         rgb(0x9a, 0x9a, 0xa6),
	rowSize:      11,
	rowBold:      true,
	// Green at the floor, then yellow, then orange at the very top: the
	// climb an analyser of the era drew, one colour per row.
	ramp: []paintengine2d.Color{
		rgb(0x10, 0x9c, 0x10), rgb(0x14, 0xa8, 0x14), rgb(0x18, 0xb4, 0x18), rgb(0x1c, 0xc0, 0x1c),
		rgb(0x20, 0xcc, 0x20), rgb(0x28, 0xd4, 0x20), rgb(0x40, 0xdc, 0x20), rgb(0x60, 0xe0, 0x20),
		rgb(0x88, 0xe0, 0x20), rgb(0xb0, 0xdc, 0x20), rgb(0xd0, 0xd4, 0x20), rgb(0xe0, 0xc0, 0x20),
		rgb(0xe8, 0xa8, 0x20), rgb(0xec, 0x90, 0x20), rgb(0xee, 0x78, 0x1c), rgb(0xf0, 0x60, 0x18),
	},
}

var silverInk = ink{
	text:         rgb(0xe6, 0xee, 0xff),
	row:          rgb(0x9c, 0xb4, 0xe6),
	current:      rgb(0xff, 0xff, 0xff),
	selected:     rgb(0x66, 0x88, 0xcc),
	selectedText: rgb(0xff, 0xff, 0xff),
	info:         rgb(0xe6, 0xee, 0xff),
	curve:        rgb(0x6c, 0x8c, 0xc8),
	peak:         rgb(0xe6, 0xee, 0xff),
	rowSize:      11,
	// A dot-matrix bar: lit rows with a dark row between each, so a bar
	// reads as a stack of dots rather than as a solid block.
	ramp: func() []paintengine2d.Color {
		out := make([]paintengine2d.Color, 0, 16)
		for i := 0; i < 8; i++ {
			out = append(out, rgb(0xd8, 0xe4, 0xfa), rgb(0x2a, 0x48, 0x88))
		}
		return out
	}(),
}

func rgb(r, g, b uint8) paintengine2d.Color {
	return paintengine2d.RGB(float32(r)/255, float32(g)/255, float32(b)/255)
}

// ---- painting a panel --------------------------------------------------------------

// slotRect is where a slot of a layout is in a component's own coordinates,
// the component's box being the whole layout.
func slotRect(lk style.LookAndFeel, layout, slot string, box paintengine2d.Rect) paintengine2d.Rect {
	r, _ := style.SkinSlotRect(lk, layout, slot, box)
	return r
}

// art paints a panel sprite into a rect of the component's own, at the
// sprite's own size. It reports false when the look has no such sprite, so
// the caller can fall back.
func art(lk style.LookAndFeel, ctx *paintengine2d.Context, r paintengine2d.Rect, name string) bool {
	return style.DrawSkinSprite(lk, ctx, r, name, paintengine2d.Color{})
}

// paintAsKey makes a glyph button paint as the art its slot in a panel's
// layout gives it — at rest, under the pointer, held down, switched on —
// and take the pointer only on that art; in the widget face it paints as
// itself.
func paintAsKey(b *players.GlyphButton, layout, slot string) {
	b.Painter = func(ctx *paintengine2d.Context, r paintengine2d.Rect, st style.ControlState) bool {
		lk := b.Look()
		if !faceOf(lk).panelled() {
			return false
		}
		return style.DrawSkinSlot(lk, ctx, widget.SlotArtRect(b, layout, slot), layout, slot, st)
	}
	b.Shaper = func(lk style.LookAndFeel, size paintengine2d.Point) *style.Silhouette {
		if !faceOf(lk).panelled() {
			return nil
		}
		return style.SkinSlotShape(lk, size, widget.SlotArtRect(b, layout, slot), layout, slot)
	}
}

// paintAsThumb makes a fader paint as a thumb sprite riding a groove the
// face's background already has printed on it. The thumb is the sprite's own
// size, and the fader's Travel is set to half of it along its length, so the
// pointer and the picture agree about where the ends are.
func paintAsThumb(f *players.Fader, name, layout, slot string) {
	f.Painter = func(ctx *paintengine2d.Context, _ paintengine2d.Rect, t float32, st style.ControlState) bool {
		lk := f.Look()
		w, h, ok := style.SkinSpriteSize(lk, name)
		if !faceOf(lk).panelled() || !ok {
			// Travel is read by the pointer, which always comes after a
			// paint, so the paint is where the face's answer is kept.
			f.Travel = 0
			return false
		}
		// The groove's own rect, on the face's grid rather than the
		// fader's whole-pixel box.
		r := widget.SlotArtRect(f, layout, slot)
		tw, th := style.Dip(lk, w), style.Dip(lk, h)
		// The thumb moves in whole design pixels, so it stays on the grid
		// the face under it is drawn on.
		d := style.Dip(lk, 1)
		var thumb paintengine2d.Rect
		if f.Horizontal {
			f.Travel = w / 2
			x := snapDesign(t*(r.Dx()-tw), d)
			thumb = paintengine2d.XYWH(r.Min.X+x, r.Min.Y+snapDesign((r.Dy()-th)/2, d), tw, th)
		} else {
			f.Travel = h / 2
			y := snapDesign((1-t)*(r.Dy()-th), d)
			thumb = paintengine2d.XYWH(r.Min.X+snapDesign((r.Dx()-tw)/2, d), r.Min.Y+y, tw, th)
		}
		sprite := name
		if st.Pressed() {
			if _, _, ok := style.SkinSpriteSize(lk, name+".down"); ok {
				sprite += ".down"
			}
		}
		return art(lk, ctx, thumb, sprite)
	}
}

// snapDesign rounds v to a whole number of design pixels d device pixels
// wide: a piece of a panel moved by a fraction of one lands on the grid the
// rest of the panel is drawn on.
func snapDesign(v, d float32) float32 {
	if d <= 0 {
		return v
	}
	return float32(math.Round(float64(v/d))) * d
}

// snapDev rounds to a whole device pixel, so a thumb that has moved by a
// fraction lands on the grid instead of between two pixels.
func snapDev(v float32) float32 {
	if v < 0 {
		return -snapDev(-v)
	}
	return float32(int(v + 0.5))
}

// ---- the display's own alphabet ------------------------------------------------------

// pixText sets a line in the skin's bitmap capitals ("font.<hex>" sprites)
// with its top left at (x, y), in ink, and returns how wide it was. A rune
// the face does not draw is set as a question mark, so a gap shows.
//
// The glyphs are the skin's and the line is the app's: this is the display
// printing the track, which a skin cannot know and an app cannot draw.
func pixText(lk style.LookAndFeel, ctx *paintengine2d.Context, x, y float32, s string, col paintengine2d.Color) float32 {
	d := func(v float32) float32 { return style.Dip(lk, v) }
	x0 := x
	for _, r := range strings.ToUpper(s) {
		name := glyphName(r)
		w, h, ok := style.SkinSpriteSize(lk, name)
		if !ok {
			name = glyphName('?')
			if w, h, ok = style.SkinSpriteSize(lk, name); !ok {
				continue
			}
		}
		if ctx != nil {
			style.DrawSkinSprite(lk, ctx, paintengine2d.XYWH(x, y, d(w), d(h)), name, col)
		}
		x += d(w + 1)
	}
	return max(x-x0-d(1), 0)
}

// pixWidth is how wide pixText would set s.
func pixWidth(lk style.LookAndFeel, s string) float32 {
	return pixText(lk, nil, 0, 0, s, paintengine2d.Color{})
}

func glyphName(r rune) string {
	const digits = "0123456789abcdef"
	var b []byte
	for v := uint32(r); ; v >>= 4 {
		b = append([]byte{digits[v&15]}, b...)
		if v < 16 {
			break
		}
	}
	return "font." + string(b)
}

// pixScroll sets a line that is wider than its box sliding back and forth
// inside it, on the same rests and pace as the widget face's title
// (players.ScrollK), clipped to the box.
func pixScroll(lk style.LookAndFeel, ctx *paintengine2d.Context, b paintengine2d.Rect, s string, col paintengine2d.Color, p *Player) {
	w := pixWidth(lk, s)
	over := w - b.Dx()
	x := b.Min.X
	if over > 0 {
		// Slid in whole design pixels, so the letters stay on the panel's
		// grid as they go.
		x -= snapDesign(over*float32(players.ScrollK(lk, over, p.Transport.Pos)), style.Dip(lk, 1))
	}
	ctx.Save()
	ctx.ClipRect(b)
	pixText(lk, ctx, x, b.Min.Y, s, col)
	ctx.Restore()
}

// ledClock paints the clock in the skin's segmented digits, in the strip's
// digit and colon slots inside box.
func ledClock(lk style.LookAndFeel, ctx *paintengine2d.Context, box paintengine2d.Rect, clock string) {
	// "m:ss" or "mm:ss" to four digits: the era's clock showed minutes and
	// seconds in two digits each, with a leading zero.
	digits := strings.ReplaceAll(clock, ":", "")
	for len(digits) < 4 {
		digits = "0" + digits
	}
	digits = digits[len(digits)-4:]
	for i, r := range digits {
		art(lk, ctx, slotRect(lk, layoutStrip, "digit."+string(rune('0'+i)), box), "led."+string(r))
	}
	art(lk, ctx, slotRect(lk, layoutStrip, "colon", box), "led.colon")
}
