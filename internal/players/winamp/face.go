package winamp

import (
	"math"
	"strconv"
	"strings"

	"github.com/codemodify/media-player-music/internal/players"
	"github.com/codemodify/media-player-music/internal/skins/panel"
	"github.com/codemodify/paintengine2d"
	"github.com/codemodify/uitoolkit/style"
	"github.com/codemodify/uitoolkit/widget"
)

// The panel layouts belong to the application's packs. Controls bind to
// named slots and the toolkit resolves their positions at each size and
// display scale. External packs can provide the same slot vocabulary with
// different artwork and layout without changing the player controls.

// face is which of the layouts a look gets.
type face uint8

const (
	faceWidgets face = iota // generic paint fallback for a helper without a panel
	faceClassic             // "winamp"
	faceSilver              // "winamp-silver"
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
// strip's layout. Silver keeps its legacy fallback ink; other panel skins
// start with classic ink. A pack's named text roles override those defaults.
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
// equaliser's curve, the analyser's bars. These defaults preserve older
// packs; named player.* text roles in the skin supply its original colours.
type ink struct {
	text, row, current, selected, selectedText, info paintengine2d.Color
	curve, peak                                      paintengine2d.Color
	ramp                                             []paintengine2d.Color
	// rowSize is the size the playlist's rows are set at, in design
	// pixels, and rowBold their weight.
	rowSize float32
	rowBold bool
}

func (f face) ink(lk style.LookAndFeel) *ink {
	var in ink
	switch f {
	case faceClassic:
		in = classicInk
	case faceSilver:
		in = silverInk
	default:
		return nil
	}
	// App-painted text and analyser colours use the pack's named text roles.
	// Older and external packs without these roles retain their baseline ink.
	sk, ok := style.LoadSkin(style.LookAppearance(lk).Name)
	if !ok {
		return &in
	}
	color := func(name string, dst *paintengine2d.Color) {
		if role := sk.Text["player."+name]; role != nil {
			*dst = role.Color
		}
	}
	color("text", &in.text)
	color("row", &in.row)
	color("current", &in.current)
	color("selected", &in.selected)
	color("selectedText", &in.selectedText)
	color("info", &in.info)
	color("curve", &in.curve)
	color("peak", &in.peak)
	if role := sk.Text["player.row"]; role != nil {
		if role.Size > 0 {
			in.rowSize = role.Size
		}
		in.rowBold = role.Bold
	}
	var ramp []paintengine2d.Color
	for i := 0; ; i++ {
		role := sk.Text["player.ramp."+strconv.Itoa(i)]
		if role == nil {
			break
		}
		ramp = append(ramp, role.Color)
	}
	if len(ramp) > 0 {
		in.ramp = ramp
	}
	return &in
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
// and take the pointer only on that art. A helper without panel artwork
// falls back to the toolkit's normal control painting.
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
// pointer and the picture agree about where the ends are. An optional track
// frame sequence follows the same value before the thumb is painted.
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
		frames, frameT := "", t
		switch {
		case slot == "volume":
			frames = "volume"
		case slot == "balance":
			frames = "balance"
			// The balance track indicates distance from centre. Its thumb
			// still gives the direction on the signed slider range.
			frameT = float32(math.Abs(float64(2*t - 1)))
		case layout == layoutEqualiser:
			frames = "eq"
		}
		paintTrackFrame(lk, ctx, r, frames, frameT)
		tw, th := style.Dip(lk, w), style.Dip(lk, h)
		// The thumb moves in whole design pixels, so it stays on the grid
		// the face under it is drawn on.
		d := style.Dip(lk, 1)
		var thumb paintengine2d.Rect
		if !f.Vertical {
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

// paintTrackFrame accepts any contiguous optional prefix.frame.N sequence.
// Its first and last frames describe the range endpoints; existing packs
// without the sequence keep their groove on the panel background.
func paintTrackFrame(lk style.LookAndFeel, ctx *paintengine2d.Context, r paintengine2d.Rect, prefix string, t float32) bool {
	if prefix == "" {
		return false
	}
	count := 0
	for {
		if _, _, ok := style.SkinSpriteSize(lk, prefix+".frame."+strconv.Itoa(count)); !ok {
			break
		}
		count++
	}
	if count == 0 {
		return false
	}
	i := int(math.Round(float64(max(float32(0), min(float32(1), t)) * float32(count-1))))
	return art(lk, ctx, r, prefix+".frame."+strconv.Itoa(i))
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
	return pixTextWithPrefix(lk, ctx, x, y, s, col, "")
}

// Playlist readouts may use the original font's shapes as alpha masks with
// their own ink. Packs without those optional glyphs keep the ordinary font.
func pixInfoText(lk style.LookAndFeel, ctx *paintengine2d.Context, x, y float32, s string, col paintengine2d.Color) float32 {
	return pixTextWithPrefix(lk, ctx, x, y, s, col, "info.")
}

func pixTextWithPrefix(lk style.LookAndFeel, ctx *paintengine2d.Context, x, y float32, s string, col paintengine2d.Color, prefix string) float32 {
	d := func(v float32) float32 { return style.Dip(lk, v) }
	x0 := x
	for _, r := range strings.ToUpper(s) {
		name, w, h, ok := textGlyph(lk, r, prefix)
		if !ok {
			if name, w, h, ok = textGlyph(lk, '?', prefix); !ok {
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

func textGlyph(lk style.LookAndFeel, r rune, prefix string) (name string, w, h float32, ok bool) {
	name = prefix + glyphName(r)
	w, h, ok = style.SkinSpriteSize(lk, name)
	if !ok && prefix != "" {
		name = glyphName(r)
		w, h, ok = style.SkinSpriteSize(lk, name)
	}
	return
}

// pixWidth is how wide pixText would set s.
func pixWidth(lk style.LookAndFeel, s string) float32 {
	return pixText(lk, nil, 0, 0, s, paintengine2d.Color{})
}

func pixInfoWidth(lk style.LookAndFeel, s string) float32 {
	return pixInfoText(lk, nil, 0, 0, s, paintengine2d.Color{})
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
// inside it, on the same rests and pace as the shared scrolling title
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
