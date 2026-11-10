package winamp

import (
	"fmt"

	"github.com/codemodify/media-player-music/internal/players"
	"goframework.io/desktop-ui-skins"
	"goframework.io/desktop-ui-widgets"
	"goframework.io/desktop-ui/a11y"
	"goframework.io/desktop-ui/layout"
	"goframework.io/desktop-ui/platform"
	"goframework.io/desktop-ui/style"
	"goframework.io/desktop-ui/widget"
	"goframework.io/graphics-paint2d"
)

// The two windows that stick to the strip.
//
// They are separate top-level windows rather than panels that fold out of
// the main one, because that is the thing being demonstrated: three windows
// that behave as one object, snapping flush and travelling together. The
// arithmetic is in rack.Rack and the desktop half is rack.Desk; all
// either window does here is lay itself out — in the face its look asks for
// (face.go), like the strip.

// ---- the equaliser -----------------------------------------------------------

// eqPane is a preamp, ten bands and a preset picker.
type eqPane struct {
	widget.Base
	p *Player

	on      *players.GlyphButton
	auto    *players.GlyphButton
	presets *players.GlyphButton
	preamp  *players.Fader
	bands   []*players.Fader

	// slots is where each control goes in a panel face.
	slots *skins.Slots
}

func newEqPane(p *Player) *eqPane {
	e := &eqPane{p: p}
	e.Init(e)
	e.SetManagesChildren(true)

	e.on = players.NewGlyphButton(players.GlyphSliders, "Equaliser on", func() {
		p.Equaliser.SetOn(!p.Equaliser.On)
	})
	e.on.Toggle = true
	e.on.Checked = p.Equaliser.On
	e.auto = players.NewGlyphButton(players.GlyphNone, "Auto: a preset per track", func() {
		p.Equaliser.SetAuto(!p.Equaliser.Auto, p.Transport.List.Index())
	})
	e.auto.Toggle = true
	e.presets = players.NewGlyphButton(players.GlyphNone, "Presets", func() { e.presetMenu() })

	e.preamp = players.NewFader(-players.EqRange, players.EqRange, p.Equaliser.Preamp, "Preamp",
		func(v float32) { p.Equaliser.SetPreamp(v) })
	e.preamp.Format = players.Decibels
	for i, name := range players.EqBands {
		band := i
		f := players.NewFader(-players.EqRange, players.EqRange, p.Equaliser.Gains[i], name+" Hz",
			func(v float32) { p.Equaliser.Set(band, v) })
		f.Format = players.Decibels
		e.bands = append(e.bands, f)
	}

	e.slots = skins.NewSlots(layoutEqualiser).
		Bind("on", e.on).Bind("auto", e.auto).Bind("presets", e.presets).Bind("preamp", e.preamp)
	for i, f := range e.bands {
		e.slots.Bind("band."+string(rune('0'+i)), f)
	}
	paintAsKey(e.on, layoutEqualiser, "on")
	paintAsKey(e.auto, layoutEqualiser, "auto")
	paintAsKey(e.presets, layoutEqualiser, "presets")
	paintAsThumb(e.preamp, "eq.thumb", layoutEqualiser, "preamp")
	for i, f := range e.bands {
		paintAsThumb(f, "eq.thumb", layoutEqualiser, "band."+string(rune('0'+i)))
	}

	e.Add(e.on)
	e.Add(e.auto)
	e.Add(e.presets)
	e.Add(e.preamp)
	for _, f := range e.bands {
		e.Add(f)
	}
	return e
}

// presetMenu is the PRESETS key: the presets by name, the one in force
// ticked.
func (e *eqPane) presetMenu() {
	var items []*widgets.MenuItem
	for _, pr := range players.EqPresets() {
		name := pr.Name
		items = append(items, widgets.RadioItem(name, "preset", name == e.p.Equaliser.Preset,
			func() { e.p.Equaliser.Apply(name) }))
	}
	o := widget.DeviceOrigin(e.presets)
	widgets.ShowContextMenu(e.presets, paint2d.Pt(o.X, o.Y+e.presets.Bounds().Dy()), items...)
}

// sync puts the shared model's equaliser settings into the controls.
func (e *eqPane) sync() {
	eq := e.p.Equaliser
	available := e.p.Host.EqualizerAvailable()
	tip := e.p.Host.FeatureTip("EQ", available)
	for _, c := range []widget.Component{e.on, e.auto, e.presets, e.preamp} {
		c.SetEnabled(available)
	}
	e.on.Tip, e.auto.Tip, e.presets.Tip, e.preamp.Tip = tip, tip, tip, tip
	if available {
		e.on.Tip, e.auto.Tip, e.presets.Tip = e.on.Label, e.auto.Label, e.presets.Label
	}
	e.on.SetChecked(eq.On)
	e.auto.SetChecked(eq.Auto)
	e.preamp.Set(eq.Preamp)
	for i, f := range e.bands {
		f.SetEnabled(available)
		f.Tip = tip
		if i < len(eq.Gains) {
			f.Set(eq.Gains[i])
		}
	}
	e.Invalidate()
}

func (e *eqPane) Measure(c layout.Constraints) paint2d.Point {
	lk := e.Look()
	if sz, ok := e.slots.Measure(lk, c); ok {
		return sz
	}
	return c.Constrain(paint2d.Pt(style.Dip(lk, StripW), style.Dip(lk, EqH)))
}

func (e *eqPane) Arrange(r paint2d.Rect) {
	e.SetBounds(r)
	e.slots.Arrange(e.Look(), e.LocalBounds())
}

func (e *eqPane) Paint(ctx *paint2d.Context) {
	lk, b := e.Look(), e.LocalBounds()
	if len(e.bands) == 0 {
		return
	}
	if f := faceOf(lk); f.panelled() {
		skins.DrawSkinLayout(lk, ctx, b, layoutEqualiser)
		e.paintCurve(ctx, lk, slotRect(lk, layoutEqualiser, "graph", b), f.ink(lk))
		return
	}
	pal := lk.Palette()
	first := e.bands[0].Bounds()
	y := (first.Min.Y + first.Max.Y) / 2
	ctx.DrawRect(paint2d.XYWH(b.Min.X, y, b.Dx(), max(style.Dip(lk, 1), 1)),
		paint2d.Fill(pal.Divider))

	// The frequencies, under the faders they belong to. Nine design pixels
	// is small, and it is what the row of a control this size is: the
	// numbers are a scale, not a label anybody reads one of.
	tiny := players.ScaledFace(lk.Font(), style.Dip(lk, 9))
	row := paint2d.XYWH(0, first.Max.Y+style.Dip(lk, 1), 0, style.Dip(lk, 10))
	pre := e.preamp.Bounds()
	players.DrawTextIn(ctx, tiny, paint2d.XYWH(pre.Min.X, row.Min.Y, pre.Dx(), row.Dy()),
		"pre", pal.TextMuted, style.AlignCenter)
	for i, f := range e.bands {
		fb := f.Bounds()
		players.DrawTextIn(ctx, tiny, paint2d.XYWH(fb.Min.X, row.Min.Y, fb.Dx(), row.Dy()),
			players.EqBands[i], pal.TextMuted, style.AlignCenter)
	}
}

// paintCurve draws the response the ten bands make across the graph well g,
// a design pixel at a time so it sits on the panel's grid.
func (e *eqPane) paintCurve(ctx *paint2d.Context, lk style.LookAndFeel, g paint2d.Rect, in *ink) {
	d := style.Dip(lk, 1)
	n := int(g.Dx()/d+0.5) - 4
	if n < 2 {
		return
	}
	pts := e.p.Equaliser.Curve(n)
	mid := g.Min.Y + g.Dy()/2
	amp := g.Dy()/2 - 2*d
	col := paint2d.Fill(in.curve)
	if !e.p.Equaliser.On {
		col = paint2d.Fill(style.Mix(in.curve, paint2d.RGB(0.4, 0.4, 0.45), 0.35))
	}
	for i, v := range pts {
		x := g.Min.X + 2*d + float32(i)*d
		y := g.Min.Y + snapDesign(mid-g.Min.Y-v*amp, d)
		ctx.DrawRect(paint2d.XYWH(x, y, d, d), col)
	}
}

// MousePress on the equaliser's own face moves the stack from there, and
// opens the skin menu on the secondary button.
func (e *eqPane) MousePress(ev widget.MouseEvent) bool {
	if e.p.rightClick(e, ev) {
		return true
	}
	e.p.g.StartMove(e.p.iEq)
	return true
}

func (e *eqPane) KeyPress(ev widget.KeyEvent) bool { return e.p.Keys(e.p.Eq(), ev) }

func (e *eqPane) CaptionAt(paint2d.Point) bool { return true }

func (e *eqPane) Describe(n *a11y.Node) {
	n.Role = a11y.RoleGroup
	if n.Name == "" {
		n.Name = "Equaliser"
	}
	n.Description = "Ten bands and a preamp. " + e.p.Host.StatusText()
}

// ---- the playlist ------------------------------------------------------------

// listPane is the queue: a row per track, and a footer that counts them.
type listPane struct {
	widget.Base
	p *Player

	list *tracks
	foot *foot

	// The panel faces' keys along the bottom, and the small transport.
	add, rem, sel, misc, opts *players.GlyphButton
	mini                      [6]*players.GlyphButton

	// slots is where each control goes in a panel face.
	slots *skins.Slots
}

func newListPane(p *Player) *listPane {
	l := &listPane{p: p}
	l.Init(l)
	l.SetManagesChildren(true)

	l.list = newTracks(p)
	l.foot = &foot{p: p}
	l.foot.Init(l.foot)
	l.foot.Move = func() { p.g.StartMove(p.iList) }
	// The playlist controls operate the same queue as the main transport.
	l.add = players.NewGlyphButton(players.GlyphNone, "Add", func() {
		p.Host.OpenFiles(l.add, true)
	})
	l.rem = players.NewGlyphButton(players.GlyphNone, "Remove", p.Host.RemoveTrack)
	l.sel = players.NewGlyphButton(players.GlyphNone, "Select", func() {
		l.menu(l.sel, widgets.MenuItem{Text: "Show the playing track", OnClick: func() {
			l.list.EnsureVisible(p.Transport.List.Index())
		}})
	})
	// The playlist's miscellaneous key opens the same skin choices as
	// the main window's context menu.
	l.misc = players.NewGlyphButton(players.GlyphNone, "Skins", func() {
		o := widget.DeviceOrigin(l.misc)
		p.skinMenu(l.misc, paint2d.Pt(o.X, o.Y+l.misc.Bounds().Dy()))
	})
	l.opts = players.NewGlyphButton(players.GlyphNone, "List options", func() {
		l.menu(l.opts,
			widgets.MenuItem{Text: "Play the first track", Disabled: p.Transport.List.Len() == 0, OnClick: func() { p.Host.SelectTrack(0) }},
			widgets.MenuItem{Text: "Close the playlist", OnClick: func() { p.toggle(p.iList) }})
	})
	l.slots = skins.NewSlots(layoutPlaylist).Bind("list", l.list).Bind("info", l.foot)
	for b, slot := range map[*players.GlyphButton]string{
		l.add: "add", l.rem: "rem", l.sel: "sel", l.misc: "misc", l.opts: "opts",
	} {
		l.slots.Bind(slot, b)
		paintAsKey(b, layoutPlaylist, slot)
	}
	for i, m := range []struct {
		g     players.Glyph
		name  string
		label string
		c     players.Command
	}{
		{players.GlyphPrev, "mini.prev", "Previous track", players.CmdPrev},
		{players.GlyphPlay, "mini.play", "Play", players.CmdNone},
		{players.GlyphPause, "mini.pause", "Pause", players.CmdPlayPause},
		{players.GlyphStop, "mini.stop", "Stop", players.CmdStop},
		{players.GlyphNext, "mini.next", "Next track", players.CmdNext},
		{players.GlyphEject, "mini.eject", "Eject: open audio files", players.CmdNone},
	} {
		m := m
		var b *players.GlyphButton
		b = players.NewGlyphButton(m.g, m.label, func() {
			switch m.name {
			case "mini.play":
				p.Host.Play()
			case "mini.pause":
				if p.Transport.State != players.Stopped {
					p.Command(players.CmdPlayPause)
				}
			case "mini.eject":
				p.Host.OpenFiles(b, false)
			default:
				p.Command(m.c)
			}
		})
		slot := "mini." + string(rune('0'+i))
		l.slots.Bind(slot, b)
		paintAsKey(b, layoutPlaylist, slot)
		l.mini[i] = b
	}

	l.Add(l.list)
	l.Add(l.foot)
	for _, b := range []*players.GlyphButton{l.add, l.rem, l.sel, l.misc} {
		l.Add(b)
	}
	for _, b := range l.mini {
		l.Add(b)
	}
	l.Add(l.opts)
	return l
}

// Drops on the playlist's background or controls use the same toolkit
// file decoding and shared append action as a drop on its rows.
func (l *listPane) DropTypes() []string          { return []string{"text/uri-list"} }
func (l *listPane) Drop(e widget.DropEvent) bool { return l.p.Host.AcceptFileDrop(e) }
func (l *listPane) DropActionFor(offered platform.DragAction) platform.DragAction {
	return offered & platform.DragCopy
}

// menu opens a small menu under one of the keys.
func (l *listPane) menu(under *players.GlyphButton, items ...widgets.MenuItem) {
	rows := make([]*widgets.MenuItem, len(items))
	for i := range items {
		rows[i] = &items[i]
	}
	o := widget.DeviceOrigin(under)
	widgets.ShowContextMenu(under, paint2d.Pt(o.X, o.Y+under.Bounds().Dy()), rows...)
}

// sync updates the queue and status without discarding a browsed selection.
func (l *listPane) sync() {
	l.list.syncQueue()
	l.list.Invalidate()
	l.foot.Invalidate()
	if faceOf(l.Look()).panelled() {
		// The small display and clock are printed on the pane itself.
		l.Invalidate()
	}
}

func (l *listPane) Measure(c layout.Constraints) paint2d.Point {
	lk := l.Look()
	if sz, ok := l.slots.Measure(lk, c); ok {
		return sz
	}
	return c.Constrain(paint2d.Pt(style.Dip(lk, StripW), style.Dip(lk, ListH)))
}

func (l *listPane) Arrange(r paint2d.Rect) {
	l.SetBounds(r)
	l.arrangePanel()
}

func (l *listPane) arrangePanel() {
	lk, b := l.Look(), l.LocalBounds()
	l.slots.Arrange(lk, b)
	at := func(slot string) paint2d.Rect { return slotRect(lk, layoutPlaylist, slot, b) }
	// In the list's own coordinates: its box is whole pixels, and the rows
	// are drawn where the skin's rects are.
	o := l.list.Bounds().Min
	l.list.geo = &listGeo{
		rows: at("rows").Translate(paint2d.Pt(-o.X, -o.Y)),
		bar:  at("scroll").Translate(paint2d.Pt(-o.X, -o.Y)),
		rowH: at("row").Dy(),
	}
	l.list.Arrange(l.list.Bounds())
}

// Paint is the panel, in a panel face, and over it the small display — the
// playing track's length over the queue's — and the small clock.
func (l *listPane) Paint(ctx *paint2d.Context) {
	lk := l.Look()
	f := faceOf(lk)
	if !f.panelled() {
		return
	}
	b := l.LocalBounds()
	skins.DrawSkinLayout(lk, ctx, b, layoutPlaylist)
	for _, slot := range []string{"well.rows", "well.scroll", "well.info", "well.clock", "scroll.arrows"} {
		skins.DrawSkinSlot(lk, ctx, slotRect(lk, layoutPlaylist, slot, b), layoutPlaylist, slot, style.ControlState(0))
	}
	in := f.ink(lk)
	t := l.p.Transport
	info := slotRect(lk, layoutPlaylist, "info", b)
	line := durationText(t.Length()) + "/" + playlistDurationText(t.List)
	pixInfoText(lk, ctx, info.Min.X+style.Dip(lk, 3), info.Min.Y+(info.Dy()-style.Dip(lk, 6))/2, line, in.info)
	clk := slotRect(lk, layoutPlaylist, "clock", b)
	c := players.Clock(t.Pos)
	if t.State == players.Stopped {
		c = players.Clock(0)
	}
	w := pixInfoWidth(lk, c)
	pixInfoText(lk, ctx, clk.Max.X-w-style.Dip(lk, 3), clk.Min.Y+(clk.Dy()-style.Dip(lk, 6))/2, c, in.info)
}

func (l *listPane) MousePress(e widget.MouseEvent) bool {
	if l.p.rightClick(l, e) {
		return true
	}
	l.p.g.StartMove(l.p.iList)
	return true
}

func (l *listPane) KeyPress(e widget.KeyEvent) bool { return l.p.Keys(l.p.List(), e) }

func (l *listPane) CaptionAt(paint2d.Point) bool { return true }

func (l *listPane) Describe(n *a11y.Node) {
	n.Role = a11y.RoleGroup
	if n.Name == "" {
		n.Name = "Playlist"
	}
}

// foot is the playlist's status line: how many tracks and how long they run.
type foot struct {
	players.DragsWindow
	p *Player
}

func (f *foot) Measure(c layout.Constraints) paint2d.Point {
	return c.Constrain(paint2d.Pt(style.Dip(f.Look(), 120), style.Dip(f.Look(), 32)))
}

func (f *foot) Arrange(b paint2d.Rect) { f.SetBounds(b) }

// Paint is two lines: what the queue holds, and what this application is.
//
// The second line is here rather than in the strip because the strip is two
// hundred and sixty-seven design pixels wide with a display, a seek bar and
// thirteen controls in it, and there is no honest place to put a sentence.
// The playlist window has the room, so this is where the compact player
// says out loud what it is — and it says it in the accessibility tree as
// well, where a window this small is read rather than looked at. A panel
// face prints its small display here instead, and says what it is in its
// title bar.
func (f *foot) Paint(ctx *paint2d.Context) {
	lk, b := f.Look(), f.LocalBounds()
	if faceOf(lk).panelled() {
		return
	}
	pal := lk.Palette()
	half := b.Dy() / 2
	lk.DrawLabel(ctx, paint2d.XYWH(b.Min.X, b.Min.Y, b.Dx(), half),
		f.count(), pal.TextMuted, style.AlignStart)
	tiny := players.ScaledFace(lk.Font(), style.Dip(lk, 10))
	players.DrawTextIn(ctx, tiny, paint2d.XYWH(b.Min.X, b.Min.Y+half, b.Dx(), half),
		f.p.Host.StatusText(), style.Mix(pal.TextMuted, pal.Background, 0.2), style.AlignStart)
}

func (f *foot) count() string {
	pl := f.p.Transport.List
	return fmt.Sprintf("%d tracks · %s", pl.Len(), playlistDurationText(pl))
}

func (f *foot) text() string { return f.count() + " · " + f.p.Host.StatusText() }

func (f *foot) Describe(n *a11y.Node) {
	n.Role = a11y.RoleStatusBar
	n.Name = f.text()
}
