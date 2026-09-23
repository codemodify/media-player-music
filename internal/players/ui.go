package players

import (
	"time"

	"github.com/codemodify/paintengine2d"
	"github.com/codemodify/uitoolkit/a11y"
	"github.com/codemodify/uitoolkit/app"
	"github.com/codemodify/uitoolkit/layout"
	"github.com/codemodify/uitoolkit/platform"
	"github.com/codemodify/uitoolkit/style"
	"github.com/codemodify/uitoolkit/widget"
)

// The pieces of interface all three players share.
//
// There are only four, and the line between what is here and what is in each
// app is worth stating: **anything a player of any era would have drawn the
// same way** is here — the transport marks, the analyser, the clock that
// drives them, the keyboard. Everything about *how a particular player
// looked* is in that player's own package, because that is the whole subject
// of the demo and sharing it would be sharing the thing being demonstrated.
//
// All of it is ordinary toolkit code. A glyph button is a component with a
// name, a keyboard route, an accessibility node and a focus ring; what the
// skin changes is the face painted under the glyph, and nothing else.

// ---- the marks a transport row is made of ------------------------------------

// Glyph is one of the marks. They are drawn from paths rather than taken
// from an icon set, because none of the toolkit's seven icon sets has a
// transport row in it and because these are the shapes a tape machine's
// keys had before any of this was software.
type Glyph uint8

// Every glyph the three players use.
const (
	GlyphNone Glyph = iota
	GlyphPlay
	GlyphPause
	GlyphStop
	GlyphPrev
	GlyphNext
	GlyphEject
	GlyphShuffle
	GlyphRepeat
	GlyphRepeatOne
	GlyphVolume
	GlyphMute
	GlyphList
	GlyphSliders
	GlyphShrink
	GlyphGrow
	GlyphSkin
)

// DrawGlyph paints g centred in b, in col. weight is the stroke width for
// the glyphs that are drawn as lines; the solid ones ignore it.
//
// Everything is worked out from one side length rather than from b's own
// width and height, so a glyph in a box that is not square is centred and
// round rather than stretched into an oval.
func DrawGlyph(ctx *paintengine2d.Context, b paintengine2d.Rect, g Glyph, col paintengine2d.Color, weight float32) {
	s := min(b.Dx(), b.Dy())
	if s <= 0 || col.A <= 0 {
		return
	}
	cx, cy := (b.Min.X+b.Max.X)/2, (b.Min.Y+b.Max.Y)/2
	// u is the glyph's unit: half of the side it is drawn inside, which is
	// a little smaller than half the box so the mark has air round it.
	u := s * 0.30
	if weight <= 0 {
		weight = max(s*0.1, 1)
	}
	fill := paintengine2d.Fill(col)
	stroke := paintengine2d.StrokePaint(col, weight)
	stroke.Stroke.Cap = paintengine2d.CapRound
	stroke.Stroke.Join = paintengine2d.JoinRound

	tri := func(x, y, h float32, left bool) {
		p := paintengine2d.NewPath()
		w := h * 0.86
		if left {
			p.MoveTo(x+w/2, y-h/2)
			p.LineTo(x-w/2, y)
			p.LineTo(x+w/2, y+h/2)
		} else {
			p.MoveTo(x-w/2, y-h/2)
			p.LineTo(x+w/2, y)
			p.LineTo(x-w/2, y+h/2)
		}
		p.Close()
		ctx.DrawPath(p, fill)
	}
	bar := func(x, y, w, h float32) {
		ctx.DrawRect(paintengine2d.XYWH(x-w/2, y-h/2, w, h), fill)
	}

	switch g {
	case GlyphPlay:
		tri(cx+u*0.12, cy, u*2, false)
	case GlyphPause:
		bar(cx-u*0.5, cy, u*0.52, u*1.9)
		bar(cx+u*0.5, cy, u*0.52, u*1.9)
	case GlyphStop:
		ctx.DrawRect(paintengine2d.XYWH(cx-u*0.85, cy-u*0.85, u*1.7, u*1.7), fill)
	case GlyphPrev:
		tri(cx+u*0.35, cy, u*1.7, true)
		bar(cx-u*0.82, cy, u*0.4, u*1.7)
	case GlyphNext:
		tri(cx-u*0.35, cy, u*1.7, false)
		bar(cx+u*0.82, cy, u*0.4, u*1.7)
	case GlyphEject:
		p := paintengine2d.NewPath()
		p.MoveTo(cx-u, cy+u*0.15)
		p.LineTo(cx, cy-u)
		p.LineTo(cx+u, cy+u*0.15)
		p.Close()
		ctx.DrawPath(p, fill)
		bar(cx, cy+u*0.75, u*2, u*0.4)
	case GlyphShuffle:
		// Two lines that cross, with an arrow head on each: the mark every
		// player settled on once shuffling was a button rather than a menu.
		p := paintengine2d.NewPath()
		p.MoveTo(cx-u, cy-u*0.6)
		p.LineTo(cx+u*0.35, cy+u*0.6)
		p.MoveTo(cx-u, cy+u*0.6)
		p.LineTo(cx+u*0.35, cy-u*0.6)
		ctx.DrawPath(p, stroke)
		tri(cx+u*0.72, cy-u*0.6, u*0.8, false)
		tri(cx+u*0.72, cy+u*0.6, u*0.8, false)
	case GlyphRepeat, GlyphRepeatOne:
		// A circle that does not quite close, with one arrow head at the
		// break. The rectangular two-headed loop every icon set draws is
		// four strokes and two triangles, and at nineteen pixels — which is
		// exactly what a compact player's toggles are — all six land on the
		// same three rows and read as a box with a dent in it.
		r := u * 0.86
		p := paintengine2d.NewPath()
		p.AddArc(paintengine2d.Pt(cx, cy), r, r, -1.15, 5.5)
		ctx.DrawPath(p, stroke)
		tri(cx+r*0.42, cy-r*0.92, u*0.95, false)
		if g == GlyphRepeatOne {
			ctx.DrawRect(paintengine2d.XYWH(cx-weight/2, cy-u*0.4, weight, u*0.8), fill)
		}
	case GlyphVolume, GlyphMute:
		p := paintengine2d.NewPath()
		p.MoveTo(cx-u*0.9, cy-u*0.32)
		p.LineTo(cx-u*0.42, cy-u*0.32)
		p.LineTo(cx+u*0.1, cy-u*0.95)
		p.LineTo(cx+u*0.1, cy+u*0.95)
		p.LineTo(cx-u*0.42, cy+u*0.32)
		p.LineTo(cx-u*0.9, cy+u*0.32)
		p.Close()
		ctx.DrawPath(p, fill)
		if g == GlyphMute {
			x := paintengine2d.NewPath()
			x.MoveTo(cx+u*0.42, cy-u*0.45)
			x.LineTo(cx+u*1.0, cy+u*0.45)
			x.MoveTo(cx+u*1.0, cy-u*0.45)
			x.LineTo(cx+u*0.42, cy+u*0.45)
			ctx.DrawPath(x, stroke)
			break
		}
		wave := paintengine2d.NewPath()
		wave.MoveTo(cx+u*0.45, cy-u*0.45)
		wave.QuadTo(cx+u*0.8, cy, cx+u*0.45, cy+u*0.45)
		ctx.DrawPath(wave, stroke)
	case GlyphList:
		for i := float32(-1); i <= 1; i++ {
			y := cy + i*u*0.7
			ctx.DrawRect(paintengine2d.XYWH(cx-u, y-weight/2, weight, weight), fill)
			ctx.DrawRect(paintengine2d.XYWH(cx-u*0.5, y-weight/2, u*1.5, weight), fill)
		}
	case GlyphSliders:
		for i := float32(-1); i <= 1; i++ {
			x := cx + i*u*0.72
			ctx.DrawRect(paintengine2d.XYWH(x-weight/2, cy-u, weight, u*2), fill)
			// The knob sits at a different height on each, so the mark
			// reads as a set of faders rather than as three lines.
			ky := cy + u*0.55*[]float32{-1, 0.35, -0.2}[int(i)+1]
			ctx.DrawRect(paintengine2d.XYWH(x-u*0.42, ky-weight, u*0.84, weight*2), fill)
		}
	case GlyphShrink, GlyphGrow:
		box := paintengine2d.XYWH(cx-u, cy-u*0.8, u*2, u*1.6)
		ctx.DrawRect(box, stroke)
		if g == GlyphShrink {
			ctx.DrawRect(paintengine2d.XYWH(cx-u*0.5, cy-weight/2, u, weight), fill)
			break
		}
		ctx.DrawRect(paintengine2d.XYWH(cx-u*0.5, cy-weight/2, u, weight), fill)
		ctx.DrawRect(paintengine2d.XYWH(cx-weight/2, cy-u*0.5, weight, u), fill)
	case GlyphSkin:
		// Two plates, one over the other: a picture laid on a control, which
		// is all a skin is. The one underneath is an outline and the one on
		// top is solid, so the mark says which is which at any size.
		side := u * 1.25
		r := u * 0.28
		ctx.DrawRoundRect(paintengine2d.XYWH(cx-u, cy-u, side, side), r, r, stroke)
		ctx.DrawRoundRect(paintengine2d.XYWH(cx+u-side, cy+u-side, side, side), r, r, fill)
	}
}

// ---- a transport button ------------------------------------------------------

// GlyphButton is a transport control: one of the look's faces with a glyph
// painted on it.
//
// It is an ordinary component and that is the whole point of it. It takes
// the focus, Return and Space work it, it names itself in the accessibility
// tree, and ShapeRole hands the look the face it paints — so a skin whose
// art for that face is a disc decides where the button really is, and a
// press in the corner of its box falls through to whatever is behind.
type GlyphButton struct {
	widget.Base
	Glyph Glyph
	// Label is what the button is called: its accessible name and its
	// tooltip. A glyph has no text to fall back on, so this is not
	// optional and a11y.Check says so when it is missing.
	Label string
	// Role is the face it paints. RoleTool is the transport row's — a mark
	// on the shell that grows a face under the pointer; RoleButton is a
	// push button with a face of its own at rest.
	Role style.Role
	// Checked draws the button in its on state (shuffle, repeat, the
	// playlist toggle) and says so in the accessibility tree.
	Checked bool
	// Toggle makes it a toggle for a screen reader's benefit rather than a
	// button that happens to look pressed.
	Toggle  bool
	OnClick func()
	// Painter, when set, paints the button in place of the look's face and
	// the glyph, and reports whether it did. It is how a player whose skin
	// draws each key as a picture of its own puts that picture on an
	// ordinary button: the button is still the component, the art is only
	// what it looks like, and a look the painter has nothing for falls back
	// to the face and the mark.
	Painter func(ctx *paintengine2d.Context, b paintengine2d.Rect, st style.ControlState) bool
	// Shaper, when set, is the silhouette of what Painter paints, for a
	// box of size — a round key painted as a picture takes the pointer on
	// the picture (widget.ArtShape). Nil, or a nil answer, leaves the
	// button to its face's shape.
	Shaper func(lk style.LookAndFeel, size paintengine2d.Point) *style.Silhouette

	hovered, pressed, outside bool
}

// NewGlyphButton is a transport button on the tool face.
func NewGlyphButton(g Glyph, label string, on func()) *GlyphButton {
	b := &GlyphButton{Glyph: g, Label: label, Role: style.RoleTool, OnClick: on}
	b.Init(b)
	b.SetWantsFocus(true)
	b.SetFocusVisibleOnly(true)
	b.SetAccessibleName(label)
	return b
}

// Tooltip is the button's label, since its face has no words on it.
func (b *GlyphButton) Tooltip() string { return b.Label }

// ShapeRole is the face the button paints, so a skin that gives that face a
// silhouette decides where the pointer has to be.
func (b *GlyphButton) ShapeRole() style.Role { return b.Role }

// ArtShape is the silhouette of the picture Painter paints, when the button
// is painted as one (Shaper).
func (b *GlyphButton) ArtShape(lk style.LookAndFeel, size paintengine2d.Point) *style.Silhouette {
	if b.Shaper == nil {
		return nil
	}
	return b.Shaper(lk, size)
}

// SetGlyph swaps the mark — play becomes pause — and repaints.
func (b *GlyphButton) SetGlyph(g Glyph, label string) {
	if b.Glyph == g && b.Label == label {
		return
	}
	b.Glyph, b.Label = g, label
	b.SetAccessibleName(label)
	b.Invalidate()
}

// SetChecked turns the button on or off.
func (b *GlyphButton) SetChecked(on bool) {
	if b.Checked == on {
		return
	}
	b.Checked = on
	b.Invalidate()
}

// PaintState is the state the face is painted in.
func (b *GlyphButton) PaintState() style.ControlState {
	st := b.State()
	if b.hovered && !b.outside {
		st |= style.StateHovered
	}
	if b.pressed && !b.outside {
		st |= style.StatePressed
	}
	if b.Checked {
		st |= style.StateChecked
	}
	return st
}

func (b *GlyphButton) Measure(c layout.Constraints) paintengine2d.Point {
	h := b.Look().Metrics().ControlH
	return c.Constrain(paintengine2d.Pt(h, h))
}

func (b *GlyphButton) Arrange(r paintengine2d.Rect) { b.SetBounds(r) }

func (b *GlyphButton) Paint(ctx *paintengine2d.Context) {
	lk, r := b.Look(), b.LocalBounds()
	st := b.PaintState()
	if b.Painter != nil && b.Painter(ctx, r, st) {
		// A picture of a key is still a key, and a key the keyboard is on
		// says so: the painter never gets to leave the ring out.
		if st.Focused() {
			lk.DrawFocusRing(ctx, r)
		}
		return
	}
	if b.Role == style.RoleButton {
		lk.DrawButton(ctx, r, st, "")
	} else {
		lk.DrawToolButton(ctx, r, st, "", style.IconNone)
	}
	DrawGlyph(ctx, r, b.Glyph, GlyphInk(lk, st), style.Dip(lk, 1.6))
}

// GlyphInk is the colour a mark is drawn in for a control state.
//
// It deliberately does not try to read the face underneath. A skin's faces
// are pictures and the engine does not publish the colour of one, so a
// glyph that guessed would be unreadable on whichever skin guessed wrong;
// what it does instead is stay the label colour and let the *accent* carry
// "this is switched on", which every skin here draws a dark face for.
func GlyphInk(lk style.LookAndFeel, st style.ControlState) paintengine2d.Color {
	pal := lk.Palette()
	switch {
	case st.Disabled():
		return pal.TextMuted
	case st.Checked():
		return pal.Accent
	case st.Backdrop():
		return pal.TextMuted
	}
	return pal.Text
}

func (b *GlyphButton) MouseEnter() { b.hovered = true; b.Base.MouseEnter() }

func (b *GlyphButton) MouseExit() {
	b.hovered, b.pressed = false, false
	b.Base.MouseExit()
}

// MousePress takes the primary button only: a right-click on a key is a
// right-click on the face it sits in, and bubbles there, rather than a
// press of the key.
func (b *GlyphButton) MousePress(e widget.MouseEvent) bool {
	if !b.Enabled() || e.Button != platform.ButtonLeft {
		return false
	}
	b.MarkPointerFocus()
	b.RequestFocus()
	b.pressed = true
	b.Invalidate()
	return true
}

// MouseMove tracks a press dragged off the button and back on, exactly as
// widgets.Button does: it pops up, and releasing outside does not click.
func (b *GlyphButton) MouseMove(e widget.MouseEvent) bool {
	if !b.pressed {
		return false
	}
	if out := !b.LocalBounds().Contains(e.Pos); out != b.outside {
		b.outside = out
		b.Invalidate()
	}
	return true
}

func (b *GlyphButton) MouseRelease(e widget.MouseEvent) bool {
	was := b.pressed
	b.pressed, b.outside = false, false
	b.Invalidate()
	if was && b.LocalBounds().Contains(e.Pos) && b.Enabled() {
		b.fire()
	}
	return true
}

func (b *GlyphButton) KeyPress(e widget.KeyEvent) bool {
	if !b.Enabled() {
		return false
	}
	if e.Key == platform.KeyReturn || e.Key == platform.KeySpace {
		b.MarkKeyboardFocus()
		b.fire()
		return true
	}
	return false
}

func (b *GlyphButton) fire() {
	if b.OnClick != nil {
		b.OnClick()
	}
}

// Describe puts the button in the accessibility tree as what it is: a
// button, or a toggle when it is one, with the label its glyph does not
// have.
func (b *GlyphButton) Describe(n *a11y.Node) {
	n.Role = a11y.RoleButton
	if b.Toggle {
		n.Role = a11y.RoleToggleButton
	}
	if n.Name == "" {
		n.Name = b.Label
	}
	if b.Checked {
		n.State |= a11y.StateChecked | a11y.StatePressed
	}
	n.Actions = n.Actions.With(a11y.ActionDefault)
}

// AccessibleAction runs the button from the accessibility tree, which is
// what a screen reader's "press" does.
func (b *GlyphButton) AccessibleAction(a a11y.Action) bool {
	if a != a11y.ActionDefault || !b.Enabled() {
		return false
	}
	b.fire()
	return true
}

// ---- text a display needs and a look does not offer --------------------------

// ScaledFace is one of a look's faces at another size: the pack's own family
// and weight, baked bigger or smaller.
//
// A pack states one body size and one mono size, and the clock on a player's
// display is neither — it is the one number the thing is read by from across
// a room, and at the body size it is a caption. The family and the weight
// still come from the pack, so an era pack's display is set in that era's
// face; only the size is the app's.
func ScaledFace(f *style.Font, size float32) *style.Font {
	if f == nil || size <= 0 {
		return f
	}
	if size >= f.Size-0.01 && size <= f.Size+0.01 {
		return f
	}
	return style.BakeFamily(f.Family, f.Weight, size, f.Color)
}

// DrawTextIn paints one line in a box with a face of the caller's choosing,
// aligned across the box and centred down it.
//
// LookAndFeel.DrawLabel does this with the look's body face and no other, so
// anything set in the mono face or at a display size has to place its own
// baseline — and Font.Draw's origin is the *top of the em box*, not the
// baseline, which is the thing that is easy to get wrong and looks like a
// layout bug when it is.
func DrawTextIn(ctx *paintengine2d.Context, f *style.Font, b paintengine2d.Rect, text string, col paintengine2d.Color, align style.Align) {
	if f == nil || text == "" {
		return
	}
	w := f.Advance(text)
	if w > b.Dx() {
		text = f.Fit(text, b.Dx())
		w = f.Advance(text)
	}
	x := b.Min.X
	switch align {
	case style.AlignCenter:
		x += (b.Dx() - w) / 2
	case style.AlignEnd:
		x += b.Dx() - w
	}
	f.Draw(ctx, text, paintengine2d.Pt(x, b.Min.Y+(b.Dy()-f.Height())/2), col)
}

// DrawScrollingText paints one line that slides when it is wider than its
// box, and paints it plainly when it fits.
//
// It slides *back and forth with a rest at each end* rather than looping
// round. A loop is what a player of this era did and it is one line of
// arithmetic, but two thirds of every still frame of one is a title cut in
// half with the start of the next copy beside it — and a demo whose whole
// job is to be looked at should be readable in a still. The rest at each end
// also gives the eye the beginning of the title back once a cycle, which is
// the part anyone is actually trying to read.
//
// The phase comes from the transport's own position rather than from a wall
// clock, so the same moment is always the same picture: a screenshot is
// comparable with the one before it, and the scroll does not speed up when
// the window happens to repaint more often.
func DrawScrollingText(ctx *paintengine2d.Context, lk style.LookAndFeel, f *style.Font, b paintengine2d.Rect, text string, col paintengine2d.Color, pos time.Duration) {
	if f == nil || text == "" || b.Dx() <= 0 {
		return
	}
	w := f.Advance(text)
	if w <= b.Dx() {
		DrawTextIn(ctx, f, b, text, col, style.AlignStart)
		return
	}
	over := w - b.Dx()
	k := ScrollK(lk, over, pos)
	y := b.Min.Y + (b.Dy()-f.Height())/2
	ctx.Save()
	ctx.ClipRect(b)
	f.Draw(ctx, text, paintengine2d.Pt(b.Min.X-over*float32(k), y), col)
	ctx.Restore()
}

// ScrollK is how far through its slide a line over pixels too wide for its
// box is at pos: 0 with its start showing, 1 with its end. It is
// DrawScrollingText's phase on its own, for a display that sets its line in
// something other than a font.
func ScrollK(lk style.LookAndFeel, over float32, pos time.Duration) float64 {
	if over <= 0 {
		return 0
	}
	// Seconds: a rest, a slide at about twenty-six design pixels a second,
	// a rest, and the slide back.
	const rest = 2.2
	slide := float64(over) / float64(style.Dip(lk, 26))
	if slide < 0.4 {
		slide = 0.4
	}
	cycle := 2 * (rest + slide)
	t := pos.Seconds()
	t -= cycle * float64(int(t/cycle))
	switch {
	case t < rest:
		return 0
	case t < rest+slide:
		return (t - rest) / slide
	case t < 2*rest+slide:
		return 1
	}
	return 1 - (t-2*rest-slide)/slide
}

// ---- a fader -----------------------------------------------------------------

// Fader is a slider stood on end.
//
// The toolkit's widgets.Slider is horizontal — it measures its height from
// the look's slider metric and draws along its width — and an equaliser is
// the one control that has to be vertical, because ten of them side by side
// *is* the control and a row of horizontal ones is a form. So this is a
// component of its own rather than a flag on that one, and it paints through
// the same three parts a slider does (slider.track, slider.fill,
// slider.thumb) by asking the look to draw a slider into a box it has
// rotated, which is what keeps a skinned fader skinned.
type Fader struct {
	widget.Base
	Min, Max, Value float32
	// Step is one press of an arrow key, and Page one of Page Up or Down.
	Step, Page float32
	// Label is the fader's accessible name: "62 Hz", "Preamp".
	Label    string
	OnChange func(float32)
	// Format turns the value into what a screen reader reads and what a
	// tooltip shows ("+4.5 dB"). Nil reads the bare number.
	Format func(float32) string
	// Horizontal lays the fader on its side: a seek bar, a volume or a
	// balance slider that a skin draws as a picture of its own. The
	// toolkit's slider is horizontal already; this is for the one whose
	// art is not the look's slider parts.
	Horizontal bool
	// Painter, when set, paints the fader in place of the look's slider
	// and reports whether it did; t is the value as a fraction of the
	// range. It is what lets a skin draw an orange volume bar beside a
	// green balance bar, which one set of slider parts cannot say. The
	// focus ring is drawn either way.
	Painter func(ctx *paintengine2d.Context, b paintengine2d.Rect, t float32, st style.ControlState) bool
	// Travel is how far in from each end the thumb's centre stops, in
	// design pixels: half the thumb a painter draws, so the pointer and
	// the picture agree about where the ends are. Zero keeps the default.
	Travel float32

	hovered, drag bool
}

// NewFader is a fader over a range.
func NewFader(min, max, value float32, label string, on func(float32)) *Fader {
	if max <= min {
		max = min + 1
	}
	f := &Fader{Min: min, Max: max, Value: value, Label: label, OnChange: on}
	f.Step = (max - min) / 24
	f.Page = (max - min) / 4
	f.Init(f)
	f.SetWantsFocus(true)
	f.SetFocusVisibleOnly(true)
	f.SetAccessibleName(label)
	return f
}

// SetValue moves the fader, clamped, and tells its owner.
func (f *Fader) SetValue(v float32) {
	if v < f.Min {
		v = f.Min
	}
	if v > f.Max {
		v = f.Max
	}
	if v == f.Value {
		return
	}
	f.Value = v
	f.Invalidate()
	if f.OnChange != nil {
		f.OnChange(v)
	}
}

// Set moves the fader without telling its owner — for a preset that moves
// ten of them at once and reports the change itself.
func (f *Fader) Set(v float32) {
	on := f.OnChange
	f.OnChange = nil
	f.SetValue(v)
	f.OnChange = on
}

// Tooltip is the label and the reading.
func (f *Fader) Tooltip() string {
	if f.Format == nil {
		return f.Label
	}
	return f.Label + "  " + f.Format(f.Value)
}

// Dragging reports whether the pointer is moving the fader, so a model that
// also moves it — a clock under a seek bar — can keep its hands off.
func (f *Fader) Dragging() bool { return f.drag }

// Fraction is the value as a fraction of the range, 0 at Min.
func (f *Fader) Fraction() float32 { return f.t() }

func (f *Fader) t() float32 {
	if f.Max <= f.Min {
		return 0
	}
	return (f.Value - f.Min) / (f.Max - f.Min)
}

func (f *Fader) Measure(c layout.Constraints) paintengine2d.Point {
	lk := f.Look()
	if f.Horizontal {
		return c.Constrain(paintengine2d.Pt(style.Dip(lk, 64), lk.Metrics().SliderH+style.Dip(lk, 8)))
	}
	return c.Constrain(paintengine2d.Pt(lk.Metrics().SliderH+style.Dip(lk, 8), style.Dip(lk, 64)))
}

func (f *Fader) Arrange(r paintengine2d.Rect) { f.SetBounds(r) }

// Paint asks the look for a horizontal slider inside a rotated frame, so a
// fader is painted from the same sprites a slider is and needs no art of its
// own. The value is turned over with the frame: up is more.
func (f *Fader) Paint(ctx *paintengine2d.Context) {
	lk, b := f.Look(), f.LocalBounds()
	if b.Dx() <= 0 || b.Dy() <= 0 {
		return
	}
	st := f.State()
	if f.hovered {
		st |= style.StateHovered
	}
	if f.drag {
		st |= style.StatePressed
	}
	if f.Painter != nil && f.Painter(ctx, b, f.t(), st) {
		if st.Focused() {
			lk.DrawFocusRing(ctx, b)
		}
		return
	}
	if f.Horizontal {
		lk.DrawSlider(ctx, b, st, f.t())
		if f.Focused() {
			lk.DrawFocusRing(ctx, b)
		}
		return
	}
	ctx.Save()
	// Rotate about the box's centre, then draw the slider in the box with
	// its width and height swapped.
	cx, cy := (b.Min.X+b.Max.X)/2, (b.Min.Y+b.Max.Y)/2
	ctx.Translate(cx, cy)
	ctx.Rotate(-90 * 3.14159265 / 180)
	ctx.Translate(-cy, -cx)
	turned := paintengine2d.XYWH(b.Min.Y, b.Min.X, b.Dy(), b.Dx())
	lk.DrawSlider(ctx, turned, st, f.t())
	ctx.Restore()
	if f.Focused() {
		lk.DrawFocusRing(ctx, b)
	}
}

func (f *Fader) MouseEnter() { f.hovered = true; f.Base.MouseEnter() }

func (f *Fader) MouseExit() {
	f.hovered, f.drag = false, false
	f.Base.MouseExit()
}

func (f *Fader) MousePress(e widget.MouseEvent) bool {
	if !f.Enabled() || e.Button != platform.ButtonLeft {
		return false
	}
	f.MarkPointerFocus()
	f.RequestFocus()
	f.drag = true
	f.setFrom(e.Pos)
	return true
}

func (f *Fader) MouseMove(e widget.MouseEvent) bool {
	if !f.drag {
		return false
	}
	f.setFrom(e.Pos)
	return true
}

func (f *Fader) MouseRelease(widget.MouseEvent) bool {
	f.drag = false
	f.Invalidate()
	return true
}

// setFrom puts the value where the pointer is, with the track's ends
// trimmed by half a thumb so the two extremes are reachable.
func (f *Fader) setFrom(p paintengine2d.Point) {
	b := f.LocalBounds()
	long, at := b.Dy(), p.Y
	if f.Horizontal {
		long, at = b.Dx(), p.X
	}
	travel := f.Travel
	if travel <= 0 {
		travel = 8
	}
	pad := min(style.Dip(f.Look(), travel), long/4)
	span := long - 2*pad
	if span <= 0 {
		return
	}
	t := clamp01((at - pad) / span)
	if !f.Horizontal {
		t = 1 - t
	}
	f.SetValue(f.Min + t*(f.Max-f.Min))
}

func (f *Fader) KeyPress(e widget.KeyEvent) bool {
	if !f.Enabled() {
		return false
	}
	step, page := f.Step, f.Page
	if step <= 0 {
		step = (f.Max - f.Min) / 24
	}
	if page <= 0 {
		page = (f.Max - f.Min) / 4
	}
	switch e.Key {
	case platform.KeyUp, platform.KeyRight:
		if e.Key == platform.KeyRight && !f.Horizontal {
			return false
		}
		f.MarkKeyboardFocus()
		f.SetValue(f.Value + step)
	case platform.KeyDown, platform.KeyLeft:
		if e.Key == platform.KeyLeft && !f.Horizontal {
			return false
		}
		f.MarkKeyboardFocus()
		f.SetValue(f.Value - step)
	case platform.KeyPageUp:
		f.MarkKeyboardFocus()
		f.SetValue(f.Value + page)
	case platform.KeyPageDown:
		f.MarkKeyboardFocus()
		f.SetValue(f.Value - page)
	case platform.KeyHome, platform.KeyEnd:
		// Home is the top of a fader and the start of a bar on its side.
		f.MarkKeyboardFocus()
		if (e.Key == platform.KeyHome) != f.Horizontal {
			f.SetValue(f.Max)
		} else {
			f.SetValue(f.Min)
		}
	default:
		return false
	}
	return true
}

// Describe puts the fader in the tree as the vertical slider it is, with a
// reading a screen reader can say out loud.
func (f *Fader) Describe(n *a11y.Node) {
	n.Role = a11y.RoleSlider
	if !f.Horizontal {
		n.State |= a11y.StateVertical
	}
	if n.Name == "" {
		n.Name = f.Label
	}
	n.HasRange = true
	n.Min, n.Max, n.Now = float64(f.Min), float64(f.Max), float64(f.Value)
	n.Step = float64(f.Step)
	if f.Format != nil {
		n.Value = f.Format(f.Value)
	}
	n.Actions = n.Actions.With(a11y.ActionIncrement).With(a11y.ActionDecrement)
}

// AccessibleAction moves the fader from the accessibility tree.
func (f *Fader) AccessibleAction(a a11y.Action) bool {
	switch a {
	case a11y.ActionIncrement:
		f.SetValue(f.Value + f.Step)
	case a11y.ActionDecrement:
		f.SetValue(f.Value - f.Step)
	default:
		return false
	}
	return true
}

// ---- the shell between the controls --------------------------------------------

// DragsWindow is a component whose own presses move the window it is in.
//
// The face of a player between its controls is a drag handle — that is what
// the whole front of a machine this size has always been. A press bubbles
// now, so a container could take one its children ignored and move the
// window from there; this stays because it says which pieces *are* face
// rather than making every picture's silence mean "drag me", and because
// CaptionAt has to be answered by the same pieces anyway.
//
// It is embedded in place of widget.Base by the pieces that are pictures:
// the readouts, the footers, the display wells.
type DragsWindow struct {
	widget.Base
	// Win is the window a press moves. A nil one is inert, which is what a
	// component wants before it has been given its window.
	Win *app.Window
}

// MousePress starts a window move and takes the press. Only the primary
// button moves a window; any other bubbles to whatever holds the face, which
// is where a player puts its context menu.
func (d *DragsWindow) MousePress(e widget.MouseEvent) bool {
	if e.Button != platform.ButtonLeft {
		return false
	}
	if d.Win != nil {
		d.Win.StartMove()
	}
	return true
}

// CaptionAt says this component is part of the window's own face, so a
// frame that asks — the toolkit's, on a window it draws — treats a press
// here as one on the caption.
func (d *DragsWindow) CaptionAt(paintengine2d.Point) bool { return true }

// ---- the analyser ------------------------------------------------------------

// AnalyserView paints a Spectrum: a row of bars with a marker riding the
// top of each.
//
// It is not interactive and it is not in the tab order — a bar chart of
// nothing is not a control — but it *is* in the accessibility tree, as an
// image with a name, because a screen reader that met an unnamed blank
// rectangle would have nothing to say about it at all.
type AnalyserView struct {
	widget.Base
	Spectrum *Spectrum
	// Gap is the space between bars in design pixels, and MinBarW the
	// narrowest a bar may be before bands are dropped to make room.
	Gap, MinBarW float32
	// Peaks draws the markers. Off for the narrow analysers that have no
	// room for them.
	Peaks bool
	// Pixelated squares the bars off and puts every edge on a whole
	// device pixel, for the skin whose art is drawn that way.
	Pixelated bool
	// Tint overrides the accent the bars are drawn in.
	Tint paintengine2d.Color
	// Ramp, when set, colours a bar by height instead: the analyser's
	// height is cut into len(Ramp) equal rows, bottom first, and a bar is
	// each row's colour where it reaches it. It is the green-to-red climb
	// of an analyser of this era, which a single tint cannot draw.
	Ramp []paintengine2d.Color
	// PeakTint overrides the markers' colour.
	PeakTint paintengine2d.Color
	// Drag is the window a press on the analyser moves: an analyser sunk
	// into a player's display is part of its face, and says so itself
	// rather than leaving it to whatever is underneath.
	Drag *app.Window
}

// MousePress moves the window when the analyser is part of a face.
func (a *AnalyserView) MousePress(e widget.MouseEvent) bool {
	if a.Drag == nil || e.Button != platform.ButtonLeft {
		return false
	}
	a.Drag.StartMove()
	return true
}

// CaptionAt: an analyser in a player's display is face, not content.
func (a *AnalyserView) CaptionAt(paintengine2d.Point) bool { return a.Drag != nil }

// NewAnalyserView shows s.
func NewAnalyserView(s *Spectrum) *AnalyserView {
	a := &AnalyserView{Spectrum: s, Gap: 1, MinBarW: 2, Peaks: true}
	a.Init(a)
	a.SetAccessibleName("Spectrum analyser")
	return a
}

func (a *AnalyserView) Measure(c layout.Constraints) paintengine2d.Point {
	return c.Constrain(paintengine2d.Pt(style.Dip(a.Look(), 76), style.Dip(a.Look(), 32)))
}

func (a *AnalyserView) Arrange(r paintengine2d.Rect) { a.SetBounds(r) }

func (a *AnalyserView) Paint(ctx *paintengine2d.Context) {
	if a.Spectrum == nil || a.Spectrum.Bands() == 0 {
		return
	}
	lk, b := a.Look(), a.LocalBounds()
	if b.Dx() <= 0 || b.Dy() <= 0 {
		return
	}
	col := a.Tint
	if col.A <= 0 {
		col = lk.Palette().Accent
	}
	gap := max(style.Dip(lk, a.Gap), 1)
	minW := max(style.Dip(lk, a.MinBarW), 1)

	// How many bands fit. The analyser is as wide as it is rather than as
	// wide as the model, so a narrow one shows fewer bands rather than a
	// row of slivers — and it averages the ones it drops in, so the
	// picture is the same shape at every width.
	n := a.Spectrum.Bands()
	for n > 1 && (b.Dx()-float32(n-1)*gap)/float32(n) < minW {
		n--
	}
	barW := (b.Dx() - float32(n-1)*gap) / float32(n)
	if barW <= 0 {
		return
	}
	per := float32(a.Spectrum.Bands()) / float32(n)
	fill := paintengine2d.Fill(col)
	peak := paintengine2d.Fill(style.Mix(col, lk.Palette().Text, 0.55))
	if a.PeakTint.A > 0 {
		peak = paintengine2d.Fill(a.PeakTint)
	}
	radius := float32(0)
	if !a.Pixelated {
		radius = min(barW/3, style.Dip(lk, 2))
	}
	for i := 0; i < n; i++ {
		lo, hi := int(float32(i)*per), int(float32(i+1)*per)
		if hi <= lo {
			hi = lo + 1
		}
		var v, p float32
		for j := lo; j < hi && j < a.Spectrum.Bands(); j++ {
			v += a.Spectrum.Bars[j]
			p += a.Spectrum.Peaks[j]
		}
		k := float32(hi - lo)
		v, p = v/k, p/k
		x := b.Min.X + float32(i)*(barW+gap)
		h := v * b.Dy()
		if a.Pixelated {
			x = snap(x)
			h = snap(h)
		}
		if h >= 1 && len(a.Ramp) > 0 {
			a.paintRamp(ctx, b, x, barW, h)
		} else if h >= 1 {
			r := paintengine2d.XYWH(x, b.Max.Y-h, barW, h)
			if radius > 0 {
				ctx.DrawRoundRectCorners(r, radius, radius, 0, 0, fill)
			} else {
				ctx.DrawRect(r, fill)
			}
		}
		if !a.Peaks {
			continue
		}
		py := b.Max.Y - p*b.Dy()
		pt := max(style.Dip(lk, 1), 1)
		if a.Pixelated {
			py, pt = snap(py), snap(pt)
		}
		if py >= b.Min.Y && py+pt <= b.Max.Y {
			ctx.DrawRect(paintengine2d.XYWH(x, py, barW, pt), peak)
		}
	}
}

// paintRamp draws one bar of height h as rows of the ramp's colours.
func (a *AnalyserView) paintRamp(ctx *paintengine2d.Context, b paintengine2d.Rect, x, w, h float32) {
	n := len(a.Ramp)
	row := b.Dy() / float32(n)
	top := b.Max.Y - h
	for k := 0; k < n; k++ {
		y1 := b.Max.Y - float32(k)*row
		y0 := y1 - row
		if a.Pixelated {
			y0, y1 = snap(y0), snap(y1)
		}
		if y1 <= top {
			break
		}
		y0 = max(y0, top)
		ctx.DrawRect(paintengine2d.XYWH(x, y0, w, y1-y0), paintengine2d.Fill(a.Ramp[k]))
	}
}

// Describe puts the analyser in the tree as a named image. It has no value
// to read out: "the bars are at 0.42, 0.61, 0.33" is not information, and a
// reader that tried to announce them every frame would be unusable.
func (a *AnalyserView) Describe(n *a11y.Node) {
	n.Role = a11y.RoleImage
	if n.Name == "" {
		n.Name = "Spectrum analyser"
	}
	n.Description = Disclaimer
}

func snap(v float32) float32 {
	f := float32(int(v))
	if v-f >= 0.5 {
		return f + 1
	}
	return f
}

// ---- opening a window whose size is a design -----------------------------------

// WindowSize turns a size in *design* pixels — the units a skin's art and a
// player's proportions are drawn in — into the numbers a window-geometry
// call wants.
//
// There is nothing left to turn: the toolkit states window geometry in
// logical pixels on every backend, which is the same thing a design pixel
// is, so this is the identity and only stays for the two callers that read
// better with it. It used to be the work-around for backends that did not
// agree — an X11 window's size was device pixels and a Wayland toplevel's
// logical, so the same strip had to ask for 481 on one and 275 on the
// other — and platform converts at its own boundary now.
func WindowSize(a *app.Application, w, h int) (int, int) {
	return max(w, 1), max(h, 1)
}

// OpenSized opens a window whose width and height are stated in design
// pixels (see [WindowSize]).
//
// fixed says the window's proportions *are* the design rather than a
// starting point: the desktop is told it may not be resized at all, so
// there is no resize band anywhere on its edge.
func OpenSized(a *app.Application, opts platform.WindowOptions, w, h int, fixed bool) (*app.Window, error) {
	opts.Width, opts.Height = WindowSize(a, w, h)
	if fixed {
		opts.Sizing = platform.SizingFixed
	}
	return a.NewWindow(opts)
}

// ---- the clock ---------------------------------------------------------------

// Pulse is the timer that moves a player: it advances the transport and the
// analyser by however much time really passed and repaints the windows that
// asked to be repainted.
//
// One timer for the whole app, not one per window. Three windows each
// waking on their own would advance the same transport three times in a
// frame, and the analyser's peak markers — which are the one part with
// memory — would fall three times as fast.
type Pulse struct {
	period time.Duration
	last   time.Time
	stop   func()
	step   func(dt time.Duration)
	// now is the clock, swappable so a test can drive a pulse without
	// waiting for one.
	now func() time.Time
}

// NewPulse makes a stopped pulse that calls step with the elapsed time.
func NewPulse(period time.Duration, step func(dt time.Duration)) *Pulse {
	if period <= 0 {
		period = 80 * time.Millisecond
	}
	return &Pulse{period: period, step: step, now: time.Now}
}

// SetClock replaces the clock a pulse measures with.
func (p *Pulse) SetClock(now func() time.Time) {
	if now != nil {
		p.now = now
	}
}

// Start begins waking on win's timer. Calling it twice is a no-op, so a
// player that starts the pulse whenever it starts playing need not track
// whether it is already running.
func (p *Pulse) Start(win *app.Window) {
	if p == nil || p.stop != nil || win == nil {
		return
	}
	p.last = p.now()
	var arm func()
	arm = func() {
		p.stop = win.AfterFunc(p.period, func() {
			p.stop = nil
			p.Tick()
			arm()
		})
	}
	arm()
}

// Stop ends the pulse. The elapsed time is not banked: a player that was
// paused for an hour does not jump an hour when it starts again, because
// Start resets the mark.
func (p *Pulse) Stop() {
	if p == nil || p.stop == nil {
		return
	}
	p.stop()
	p.stop = nil
}

// Running reports whether the pulse is armed.
func (p *Pulse) Running() bool { return p != nil && p.stop != nil }

// Tick advances by the time since the last tick and calls step. A test
// calls it directly.
func (p *Pulse) Tick() {
	if p == nil || p.step == nil {
		return
	}
	now := p.now()
	dt := now.Sub(p.last)
	p.last = now
	if dt < 0 {
		dt = 0
	}
	// A wake that was delayed past a few periods — the machine was asleep,
	// or another app had the GPU — is clamped. The transport is allowed to
	// know about the lost time (Tick handles any dt at all); the analyser's
	// peak markers are not, and a marker that fell for four seconds in one
	// frame is a row of bars with no markers on it.
	if limit := 8 * p.period; dt > limit {
		dt = limit
	}
	p.step(dt)
}

// ---- the keyboard ------------------------------------------------------------

// CommandFor maps a key press onto a transport command, or CmdNone.
//
// One table for all three players, because a person who learns the keys in
// one should not have to learn them again in the next. They are the keys
// these applications have always used: space plays and pauses, the arrows
// seek and set the volume, Z and B step (the two keys either side of the
// transport row on a keyboard of the era), and the single letters do the
// toggles.
//
// It reads e.Key for the keys that navigate and e.Rune for the letters,
// which is the split [widget.KeyEvent] states: Rune is the character the
// key stands for, with no modifier folded in, so the letters here work
// whichever the layout puts where. The e.Key arm for the same letters
// stays because it costs a line and a test may send either.
//
// Nothing here takes a modifier, which is deliberate: a player's keys have
// to work while the focus is anywhere in its window. That is only safe
// because no player ever hands one of these keys to a text field — see
// [Typing], which each of the three asks before dispatching.
func CommandFor(e widget.KeyEvent) Command {
	if e.Mods.Ctrl() || e.Mods.Alt() {
		return CmdNone
	}
	switch e.Key {
	case platform.KeySpace:
		return CmdPlayPause
	case platform.KeyLeft:
		return CmdSeekBack
	case platform.KeyRight:
		return CmdSeekForward
	case platform.KeyUp:
		return CmdVolumeUp
	case platform.KeyDown:
		return CmdVolumeDown
	case platform.KeyX, platform.KeyC:
		return CmdPlayPause
	case platform.KeyV:
		return CmdStop
	case platform.KeyB:
		return CmdNext
	case platform.KeyZ:
		return CmdPrev
	case platform.KeyM:
		return CmdMute
	case platform.KeyS:
		return CmdShuffle
	case platform.KeyR:
		return CmdRepeat
	}
	switch e.Rune {
	case 'x', 'X', 'c', 'C':
		return CmdPlayPause
	case 'v', 'V':
		return CmdStop
	case 'b', 'B':
		return CmdNext
	case 'z', 'Z':
		return CmdPrev
	case 'm', 'M':
		return CmdMute
	case 's', 'S':
		return CmdShuffle
	case 'r', 'R':
		return CmdRepeat
	}
	return CmdNone
}

// Typing reports whether c is a control that takes text, so a player's
// bare-letter keys can stay out of it.
//
// A key that nobody takes bubbles up the tree, and a text field does not
// take a plain letter — the character arrives separately as text input — so
// a window whose root turns B into "next track" would change track while
// somebody typed "Beacon" into a filter. Asking the focused component what
// it *is* rather than what it is made of is the reliable test: the role a
// component gives the accessibility tree is the same answer a screen reader
// gets, and a control that is a text field to one is a text field to both.
func Typing(c widget.Component) bool {
	d, ok := c.(interface{ Describe(*a11y.Node) })
	if !ok {
		return false
	}
	var n a11y.Node
	d.Describe(&n)
	switch n.Role {
	case a11y.RoleTextField, a11y.RoleTextArea, a11y.RolePasswordField,
		a11y.RoleSpinButton, a11y.RoleComboBox:
		return true
	}
	return false
}

// KeyHelp is the line every player shows for its keys, so the three agree
// and a screenshot of one explains the others.
const KeyHelp = "Space play/pause · Z/B track · V stop · ←/→ seek · ↑/↓ volume · M mute · S shuffle · R repeat"
