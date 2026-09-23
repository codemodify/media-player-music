// Package marquee is the big player: a cabinet with a display, a queue and
// a curved transport shelf, which folds down into a small shaped window and
// back.
//
// It is a **visual demo**. Nothing is decoded and nothing is played; the
// display shows an analyser drawn from the transport's own clock, and the
// shelf's controls move that clock and nothing else.
//
// What it is a demo *of* is the compact mode, and the compact mode is the
// only place in these three players where an app sets a silhouette of its
// own. A skin declares one window shape, which is the cabinet's; the flat
// stadium the player folds into is the *app's*, set with SetShapeFunc, and
// the toolkit's rule is that the app's wins where it set one. So the switch
// changes three things at once and all three are ordinary calls: the window
// resizes, the layout changes, and the outline changes from the look's to
// the app's and back again.
package marquee

import (
	"fmt"
	"time"

	"github.com/codemodify/paintengine2d"
	"github.com/codemodify/uitoolkit/a11y"
	"github.com/codemodify/uitoolkit/app"
	"github.com/codemodify/media-player-music/internal/players"
	"github.com/codemodify/uitoolkit/layout"
	"github.com/codemodify/uitoolkit/platform"
	"github.com/codemodify/uitoolkit/style"
	"github.com/codemodify/uitoolkit/widget"
	"github.com/codemodify/uitoolkit/widgets"
)

// The two sizes, in logical pixels: the cabinet, and what it folds into.
const (
	FullW    = 780
	FullH    = 540
	CompactW = 468
	CompactH = 104
)

// Skin is the pack this player wears.
const Skin = "marquee"

// Player is the window, the model under it, and which of its two shapes it
// is in.
type Player struct {
	App    *app.Application
	Window *app.Window

	Transport *players.Transport
	Spectrum  *players.Spectrum

	body    *body
	pulse   *players.Pulse
	compact bool
}

// Options is what the command line passes in.
type Options struct {
	Headless bool
	Scale    float32
	// Compact opens folded down rather than as the cabinet.
	Compact bool
}

// New opens the player.
func New(a *app.Application, opts Options) (*Player, error) {
	p := &Player{
		App:       a,
		Transport: players.NewTransport(players.NewLibrary()),
		Spectrum:  players.NewSpectrum(48),
	}
	w, h := FullW, FullH
	if opts.Compact {
		w, h = CompactW, CompactH
	}
	win, err := players.OpenSized(a, platform.WindowOptions{
		Title:       "Marquee",
		MinWidth:    CompactW,
		MinHeight:   CompactH,
		Headless:    opts.Headless,
		Decorations: platform.DecorationsClient,
	}, w, h, false)
	if err != nil {
		return nil, err
	}
	p.Window = win
	p.body = newBody(p)
	win.SetContent(p.body)

	p.Transport.Changed = p.refresh
	p.pulse = players.NewPulse(60*time.Millisecond, p.advance)
	p.SetCompact(opts.Compact)
	p.refresh()
	// Something has the keyboard from the first frame. A player's keys are
	// bare letters that bubble up from whatever is focused, and a window
	// with no focus at all has nothing for them to bubble from.
	p.Window.RequestFocus(p.body.play)
	return p, nil
}

// Compact reports which of the two the player is in.
func (p *Player) Compact() bool { return p.compact }

// SetCompact folds the player down, or opens it back up.
//
// Three things change and none of them is special: the window's size, the
// layout, and the silhouette. The last is the interesting one — the cabinet
// takes the *look's* outline, which the skin declares and the app never
// mentions, and the folded player takes one of its own, because the toolkit
// gives the app's shape precedence over the look's exactly so that a window
// which knows it is a different shape can say so.
func (p *Player) SetCompact(on bool) {
	p.compact = on
	p.body.compact = on
	if on {
		p.Window.SetSize(players.WindowSize(p.App, CompactW, CompactH))
		// The stadium: rounded by half its own height, redrawn at every
		// size and every scale rather than stretched, because it is a
		// callback and not a fixed shape.
		p.Window.SetShapeFunc(func(size paintengine2d.Point, _ float32) *platform.Shape {
			path := paintengine2d.NewPath()
			path.AddRoundRect(paintengine2d.XYWH(0, 0, size.X, size.Y), size.Y/2, size.Y/2)
			return platform.NewShape(path)
		})
	} else {
		p.Window.SetSize(players.WindowSize(p.App, FullW, FullH))
		// nil hands the window back to its look, which for this one is the
		// skin's brow and dome.
		p.Window.SetShapeFunc(nil)
		p.Window.SetShape(nil)
	}
	p.Window.RequestLayout()
	p.refresh()
}

// Start begins the clock.
func (p *Player) Start() {
	p.Transport.Play()
	p.pulse.Start(p.Window)
}

// Pulse is the clock, for a test that drives it by hand.
func (p *Player) Pulse() *players.Pulse { return p.pulse }

// Pose puts the player at a stated position with the analyser advanced to
// match and no clock involved, so a screenshot is the same picture every
// time it is taken.
func (p *Player) Pose(pos time.Duration) {
	p.Transport.State = players.Playing
	p.Transport.Pos = pos
	p.Spectrum.Reset()
	p.Spectrum.Advance(pos, true, 250*time.Millisecond)
	p.refresh()
}

func (p *Player) advance(dt time.Duration) {
	p.Transport.Tick(dt)
	p.Spectrum.Advance(p.Transport.Pos, p.Transport.State == players.Playing, dt)
	p.refresh()
}

func (p *Player) refresh() {
	if p.body != nil {
		p.body.sync()
	}
}

// Command runs a shared transport command and keeps the clock going.
func (p *Player) Command(c players.Command) {
	p.Transport.Do(c)
	if p.Transport.State == players.Playing {
		p.pulse.Start(p.Window)
	}
}

// Keys is the window's keyboard: the shared transport table, Escape to
// quit, and one of its own — Ctrl+M folds the player down and back.
func (p *Player) Keys(w *app.Window, e widget.KeyEvent) bool {
	if e.Key == platform.KeyEscape {
		p.App.Quit()
		return true
	}
	if e.Mods.Ctrl() && e.Key == platform.KeyM {
		p.SetCompact(!p.compact)
		return true
	}
	if players.Typing(w.Focus()) {
		// The focus is in something that takes text: its letters are its
		// own. Nothing the transport answers to is a modifier chord, so
		// there is nothing left to try.
		return false
	}
	if c := players.CommandFor(e); c != players.CmdNone {
		p.Command(c)
		return true
	}
	return false
}

// ---- the content -------------------------------------------------------------

// body is the whole of the player's interface, in both of its shapes.
//
// One component and one widget tree for both, rather than two swapped at the
// switch. That is not a saving, it is the behaviour: the focus stays where
// it was, the accessibility tree keeps the same node ids, and a control the
// keyboard was on is still the control the keyboard is on after the window
// has changed size and shape. Folding hides what does not fit and lays the
// rest out differently.
type body struct {
	widget.Base
	p       *Player
	compact bool

	display  *display
	analyser *players.AnalyserView
	queue    *widgets.ListView
	queueTag *widgets.Label

	seek   *widgets.Slider
	volume *widgets.Slider

	prev, play, stop, next *players.GlyphButton
	mute                   *players.GlyphButton
	shuffle, repeat        *players.GlyphButton
	fold                   *players.GlyphButton

	// shelf is the curved bar the transport sits on in the cabinet, and
	// readLine the one line of text the folded player has instead.
	shelf, readLine paintengine2d.Rect
}

func newBody(p *Player) *body {
	b := &body{p: p}
	b.Init(b)
	b.SetManagesChildren(true)

	b.display = newDisplay(p)
	b.analyser = players.NewAnalyserView(p.Spectrum)
	b.analyser.Drag = p.Window
	b.analyser.Gap = 2
	b.analyser.MinBarW = 3

	pl := p.Transport.List
	b.queueTag = widgets.NewLabel("Queue")
	b.queue = widgets.NewListView(pl.Len(), func(i int) string {
		t := pl.At(i)
		return fmt.Sprintf("%s   %s", players.Clock(t.Length), t.Label())
	}, func(i int) {
		p.Transport.SelectTrack(i)
		if p.Transport.State != players.Playing {
			p.Command(players.CmdPlayPause)
		}
	})
	b.queue.SetAccessibleName("Queue")

	b.seek = widgets.NewSlider(0, 1, 0, func(v float32) {
		if b.scrubbing() {
			p.Transport.Seek(v)
		}
	})
	b.seek.SetAccessibleName("Seek")
	b.volume = widgets.NewSlider(0, 100, p.Transport.Volume*100, func(v float32) {
		p.Transport.SetVolume(v / 100)
	})
	b.volume.SetAccessibleName("Volume")

	b.prev = players.NewGlyphButton(players.GlyphPrev, "Previous track", func() { p.Command(players.CmdPrev) })
	b.play = players.NewGlyphButton(players.GlyphPlay, "Play", func() { p.Command(players.CmdPlayPause) })
	b.play.Role = style.RoleButton
	b.stop = players.NewGlyphButton(players.GlyphStop, "Stop", func() { p.Command(players.CmdStop) })
	b.next = players.NewGlyphButton(players.GlyphNext, "Next track", func() { p.Command(players.CmdNext) })
	b.mute = players.NewGlyphButton(players.GlyphVolume, "Mute", func() { p.Command(players.CmdMute) })
	b.mute.Toggle = true
	b.shuffle = players.NewGlyphButton(players.GlyphShuffle, "Shuffle", func() { p.Command(players.CmdShuffle) })
	b.shuffle.Toggle = true
	b.repeat = players.NewGlyphButton(players.GlyphRepeat, "Repeat", func() { p.Command(players.CmdRepeat) })
	b.repeat.Toggle = true
	b.fold = players.NewGlyphButton(players.GlyphShrink, "Compact mode", func() { p.SetCompact(!p.compact) })
	b.fold.Toggle = true

	for _, c := range []widget.Component{
		b.display, b.analyser, b.queueTag, b.queue, b.seek, b.volume,
		b.prev, b.play, b.stop, b.next, b.mute, b.shuffle, b.repeat, b.fold,
	} {
		b.Add(c)
	}
	return b
}

// scrubbing reports whether the seek bar is the thing moving the transport
// rather than the other way round.
func (b *body) scrubbing() bool { return b.seek.Focused() || b.seek.Hovered() }

func (b *body) sync() {
	t := b.p.Transport
	if t.State == players.Playing {
		b.play.SetGlyph(players.GlyphPause, "Pause")
	} else {
		b.play.SetGlyph(players.GlyphPlay, "Play")
	}
	if t.Muted() {
		b.mute.SetGlyph(players.GlyphMute, "Unmute")
	} else {
		b.mute.SetGlyph(players.GlyphVolume, "Mute")
	}
	b.mute.SetChecked(t.Muted())
	b.shuffle.SetChecked(t.Random)
	b.repeat.SetChecked(t.Repeat != players.RepeatOff)
	if t.Repeat == players.RepeatOne {
		b.repeat.SetGlyph(players.GlyphRepeatOne, "Repeat one")
	} else {
		b.repeat.SetGlyph(players.GlyphRepeat, t.Repeat.String())
	}
	b.fold.SetChecked(b.compact)
	if b.compact {
		b.fold.SetGlyph(players.GlyphGrow, "Full mode")
	} else {
		b.fold.SetGlyph(players.GlyphShrink, "Compact mode")
	}
	if !b.scrubbing() {
		b.seek.Value = t.Fraction()
	}
	b.volume.Value = t.Volume * 100
	if i := t.List.Index(); i >= 0 && b.queue.Selected != i {
		b.queue.Selected = i
		b.queue.EnsureVisible(i)
	}
	// Folding hides what will not fit. A hidden component is out of the
	// tab order and out of the accessibility tree, which is what "this
	// control is not in this mode" has to mean.
	for _, c := range []widget.Component{b.display, b.analyser, b.queue, b.queueTag, b.stop, b.shuffle, b.repeat} {
		c.SetVisible(!b.compact)
	}
	b.Invalidate()
}

func (b *body) Measure(c layout.Constraints) paintengine2d.Point {
	lk := b.Look()
	return c.Constrain(paintengine2d.Pt(style.Dip(lk, FullW), style.Dip(lk, 400)))
}

func (b *body) Arrange(r paintengine2d.Rect) {
	b.SetBounds(r)
	if b.compact {
		b.arrangeCompact()
		return
	}
	b.arrangeFull()
}

// arrangeFull is the cabinet: a display with a queue beside it, and the
// curved shelf under both.
func (b *body) arrangeFull() {
	lk := b.Look()
	dip := func(v float32) float32 { return style.Dip(lk, v) }
	r := b.LocalBounds()

	shelfH := dip(84)
	b.shelf = paintengine2d.XYWH(0, r.Dy()-shelfH, r.Dx(), shelfH)
	b.readLine = paintengine2d.Rect{}

	queueW := min(dip(250), r.Dx()*0.38)
	top := r.Dy() - shelfH - dip(12)
	displayW := r.Dx() - queueW - dip(12)
	b.display.Arrange(paintengine2d.XYWH(0, 0, displayW, top))
	// The analyser lies along the bottom third of the display, inside it.
	b.analyser.Arrange(paintengine2d.XYWH(dip(18), top*0.52, displayW-dip(36), top*0.40))

	qx := displayW + dip(12)
	tag := dip(24)
	b.queueTag.Arrange(paintengine2d.XYWH(qx, 0, queueW, tag))
	b.queue.Arrange(paintengine2d.XYWH(qx, tag, queueW, top-tag))

	// The shelf. Two rows inside it: the seek line, and the controls.
	in := b.shelf.Inset(dip(16))
	seekH := dip(12)
	clockW := dip(56)
	b.seek.Arrange(paintengine2d.XYWH(in.Min.X+clockW, in.Min.Y+dip(2), max(in.Dx()-2*clockW, dip(40)), seekH))

	y := in.Min.Y + seekH + dip(10)
	btn := dip(34)
	x := in.Min.X + dip(4)
	for _, c := range []*players.GlyphButton{b.prev, b.play, b.stop, b.next} {
		c.Arrange(paintengine2d.XYWH(x, y, btn, btn))
		x += btn + dip(6)
	}
	x += dip(10)
	for _, c := range []*players.GlyphButton{b.shuffle, b.repeat} {
		c.Arrange(paintengine2d.XYWH(x, y+dip(3), dip(28), dip(28)))
		x += dip(32)
	}
	// The right-hand end: fold, volume, mute, laid out from the edge in so
	// they keep their places when the cabinet is wider.
	fx := in.Max.X - btn
	b.fold.Arrange(paintengine2d.XYWH(fx, y, btn, btn))
	volW := dip(120)
	vx := fx - dip(14) - volW
	b.volume.Arrange(paintengine2d.XYWH(vx, y+(btn-dip(12))/2, volW, dip(12)))
	b.mute.Arrange(paintengine2d.XYWH(vx-dip(32), y+dip(3), dip(28), dip(28)))
}

// arrangeCompact is the folded player: the transport at one end, the
// volume at the other, and the clock over the seek bar between them.
//
// The analyser is not in it. There is room for forty pixels of bars at the
// cost of the title, and a compact player that shows a title it cannot read
// beside an analyser nobody asked for is a worse compact player.
func (b *body) arrangeCompact() {
	lk := b.Look()
	dip := func(v float32) float32 { return style.Dip(lk, v) }
	r := b.LocalBounds()
	b.shelf = paintengine2d.Rect{}

	pad := dip(8)
	btn := min(dip(30), max(r.Dy()-2*pad, dip(16)))
	y := r.Min.Y + (r.Dy()-btn)/2
	x := r.Min.X + pad
	for _, c := range []*players.GlyphButton{b.prev, b.play, b.next} {
		c.Arrange(paintengine2d.XYWH(x, y, btn, btn))
		x += btn + dip(4)
	}
	x += dip(8)

	fx := r.Max.X - pad - btn
	b.fold.Arrange(paintengine2d.XYWH(fx, y, btn, btn))
	volW := min(dip(72), max((r.Dx()-x)*0.3, dip(30)))
	vx := fx - dip(10) - volW
	b.volume.Arrange(paintengine2d.XYWH(vx, y+(btn-dip(10))/2, volW, dip(10)))
	mute := dip(26)
	b.mute.Arrange(paintengine2d.XYWH(vx-mute-dip(6), y+(btn-mute)/2, mute, mute))

	// Between the two ends: the clock and the title on one line, the seek
	// bar under it on the other.
	mid := paintengine2d.XYWH(x, r.Min.Y, max(b.mute.Bounds().Min.X-dip(10)-x, dip(40)), r.Dy())
	b.readLine = paintengine2d.XYWH(mid.Min.X, mid.Min.Y+dip(3), mid.Dx(), dip(22))
	b.seek.Arrange(paintengine2d.XYWH(mid.Min.X, mid.Max.Y-dip(16), mid.Dx(), dip(10)))
	b.analyser.Arrange(paintengine2d.Rect{})
}

// Paint draws what is not a control: the shelf the transport sits on, and
// in compact mode the readout that replaces the display.
func (b *body) Paint(ctx *paintengine2d.Context) {
	lk := b.Look()
	pal := lk.Palette()
	if !b.shelf.Empty() {
		// The curve. A stadium of the shelf's own height, lit from above,
		// with a hairline along its top edge — the shape a transport bar of
		// this era was moulded into.
		r := b.shelf.Dy() / 2
		style.FillV(ctx, b.shelf, r,
			style.Stop(0, style.Mix(pal.Surface, pal.Text, 0.18)),
			style.Stop(0.48, style.Mix(pal.Surface, pal.Text, 0.04)),
			style.Stop(0.52, pal.Surface),
			style.Stop(1, style.Mix(pal.Surface, pal.Shadow, 0.55)))
		ctx.Save()
		ctx.ClipRoundRect(b.shelf, r, r)
		ctx.DrawRect(paintengine2d.XYWH(b.shelf.Min.X, b.shelf.Min.Y+max(style.Dip(lk, 1), 1), b.shelf.Dx(), max(style.Dip(lk, 1), 1)),
			paintengine2d.Fill(style.Mix(pal.Text, pal.Surface, 0.55)))
		ctx.Restore()
		ctx.DrawRoundRect(b.shelf.Inset(0.5), r, r,
			paintengine2d.StrokePaint(style.Mix(pal.Border, pal.Shadow, 0.5), max(style.Dip(lk, 1), 1)))
	}
	if b.compact {
		b.paintCompactReadout(ctx)
		return
	}
	// The clocks at either end of the seek line.
	t := b.p.Transport
	s := b.seek.Bounds()
	h := s.Dy() + style.Dip(lk, 8)
	y := s.Min.Y - style.Dip(lk, 4)
	mono := players.ScaledFace(lk.MonoFont(), style.Dip(lk, 12))
	players.DrawTextIn(ctx, mono, paintengine2d.XYWH(s.Min.X-style.Dip(lk, 54), y, style.Dip(lk, 48), h),
		players.Clock(t.Pos), pal.Text, style.AlignEnd)
	players.DrawTextIn(ctx, mono, paintengine2d.XYWH(s.Max.X+style.Dip(lk, 6), y, style.Dip(lk, 48), h),
		players.Countdown(t.Remaining()), pal.TextMuted, style.AlignStart)
}

// paintCompactReadout is the one line of text the folded player has room
// for: the clock, and the track scrolling beside it.
func (b *body) paintCompactReadout(ctx *paintengine2d.Context) {
	lk := b.Look()
	pal := lk.Palette()
	t := b.p.Transport
	line := b.readLine
	if line.Empty() {
		return
	}
	mono := players.ScaledFace(lk.MonoFont(), style.Dip(lk, 12))
	clock := players.Clock(t.Pos)
	cw := mono.Advance(clock)
	players.DrawTextIn(ctx, mono, paintengine2d.XYWH(line.Min.X, line.Min.Y, cw, line.Dy()),
		clock, pal.Accent, style.AlignStart)
	gap := style.Dip(lk, 8)
	players.DrawScrollingText(ctx, lk, lk.Font(),
		paintengine2d.XYWH(line.Min.X+cw+gap, line.Min.Y, max(line.Dx()-cw-gap, 0), line.Dy()),
		t.Track().Label(), pal.Text, t.Pos)
}

func (b *body) KeyPress(e widget.KeyEvent) bool { return b.p.Keys(b.p.Window, e) }

// MousePress on the cabinet itself moves the window: the shell between the
// controls is a drag handle, which is what the face of a player has always
// been.
func (b *body) MousePress(widget.MouseEvent) bool {
	b.p.Window.StartMove()
	return true
}

func (b *body) CaptionAt(paintengine2d.Point) bool { return true }

func (b *body) Describe(n *a11y.Node) {
	n.Role = a11y.RolePane
	if n.Name == "" {
		n.Name = "Player"
	}
	n.Description = players.Disclaimer
}

// ---- the display -------------------------------------------------------------

// display is the cabinet's screen: a well with the track over it and the
// analyser lying along its floor.
type display struct {
	players.DragsWindow
	p *Player
}

func newDisplay(p *Player) *display {
	d := &display{p: p}
	d.Init(d)
	d.Win = p.Window
	return d
}

func (d *display) Measure(c layout.Constraints) paintengine2d.Point {
	return c.Constrain(paintengine2d.Pt(style.Dip(d.Look(), 420), style.Dip(d.Look(), 280)))
}

func (d *display) Arrange(b paintengine2d.Rect) { d.SetBounds(b) }

func (d *display) Paint(ctx *paintengine2d.Context) {
	lk, b := d.Look(), d.LocalBounds()
	pal := lk.Palette()
	t := d.p.Transport
	dip := func(v float32) float32 { return style.Dip(lk, v) }
	r := dip(10)

	style.FillV(ctx, b, r,
		style.Stop(0, style.Mix(pal.Field, pal.Accent, 0.06)),
		style.Stop(0.65, pal.Field),
		style.Stop(1, style.Mix(pal.Field, pal.Shadow, 0.5)))
	ctx.DrawRoundRect(b.Inset(0.5), r, r, paintengine2d.StrokePaint(pal.FieldBorder, max(dip(1), 1)))

	tr := t.Track()
	pad := dip(20)
	title := players.ScaledFace(lk.TitleFont(), dip(26))
	players.DrawTextIn(ctx, title, paintengine2d.XYWH(b.Min.X+pad, b.Min.Y+pad, b.Dx()-2*pad, dip(34)),
		tr.Title, pal.Text, style.AlignStart)
	sub := players.ScaledFace(lk.Font(), dip(14))
	players.DrawTextIn(ctx, sub, paintengine2d.XYWH(b.Min.X+pad, b.Min.Y+pad+dip(34), b.Dx()-2*pad, dip(20)),
		tr.Artist+"  ·  "+tr.Album, pal.TextMuted, style.AlignStart)

	// The one line that says what this is, under the track it is not
	// playing. It sits above the analyser rather than along the floor of
	// the display, where the bars would run through it.
	small := players.ScaledFace(lk.Font(), dip(11))
	players.DrawTextIn(ctx, small, paintengine2d.XYWH(b.Min.X+pad, b.Min.Y+pad+dip(56), b.Dx()-2*pad, dip(14)),
		players.Disclaimer, style.Mix(pal.TextMuted, pal.Field, 0.3), style.AlignStart)
}

func (d *display) Describe(n *a11y.Node) {
	t := d.p.Transport
	tr := t.Track()
	n.Role = a11y.RoleLabel
	n.Name = fmt.Sprintf("%s by %s, from %s. %s of %s. %s.",
		tr.Title, tr.Artist, tr.Album,
		players.Clock(t.Pos), players.Clock(t.Length()), t.State)
	n.Description = players.Disclaimer
}
