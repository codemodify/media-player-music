package minim

import (
	"fmt"

	"github.com/codemodify/media-player-music/internal/players"
	"github.com/codemodify/paintengine2d"
	"github.com/codemodify/uitoolkit/a11y"
	"github.com/codemodify/uitoolkit/layout"
	"github.com/codemodify/uitoolkit/style"
	"github.com/codemodify/uitoolkit/widget"
	"github.com/codemodify/uitoolkit/widgets"
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
	flat    *widgets.Button
	preset  *widgets.ComboBox
	preamp  *players.Fader
	bands   []*players.Fader

	// slots is where each control goes in a panel face.
	slots *widget.Slots
}

func newEqPane(p *Player) *eqPane {
	e := &eqPane{p: p}
	e.Init(e)
	e.SetManagesChildren(true)

	e.on = players.NewGlyphButton(players.GlyphSliders, "Equaliser on", func() {
		p.Equaliser.On = !p.Equaliser.On
		p.refresh()
	})
	e.on.Toggle = true
	e.on.Checked = p.Equaliser.On
	e.auto = players.NewGlyphButton(players.GlyphNone, "Auto: a preset per track", func() {
		p.auto = !p.auto
		p.lastTrack = -1
		p.refresh()
	})
	e.auto.Toggle = true
	e.presets = players.NewGlyphButton(players.GlyphNone, "Presets", func() { e.presetMenu() })

	e.flat = widgets.NewButton("Flat", func() { p.Equaliser.Flat() })
	e.flat.Tip = "Take every band back to zero"

	names := make([]string, 0, len(players.EqPresets()))
	for _, pr := range players.EqPresets() {
		names = append(names, pr.Name)
	}
	e.preset = widgets.NewComboBox(names, 0, func(i int) {
		if i >= 0 && i < len(names) {
			p.Equaliser.Apply(names[i])
		}
	})
	e.preset.SetAccessibleName("Preset")

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

	e.slots = widget.NewSlots(layoutEqualiser).
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
	e.Add(e.flat)
	e.Add(e.preset)
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
	widgets.ShowContextMenu(e.presets, paintengine2d.Pt(o.X, o.Y+e.presets.Bounds().Dy()), items...)
}

// show puts the controls a face has on and takes the others off: the panels
// have ON, AUTO and PRESETS keys, the widget face a Flat button and a combo.
func (e *eqPane) show(f face) {
	panelled := f.panelled()
	e.auto.SetVisible(panelled)
	e.presets.SetVisible(panelled)
	e.flat.SetVisible(!panelled)
	e.preset.SetVisible(!panelled)
}

// sync puts the model back into the faders — after a preset, after Flat.
func (e *eqPane) sync() {
	eq := e.p.Equaliser
	e.on.SetChecked(eq.On)
	e.auto.SetChecked(e.p.auto)
	e.preamp.Set(eq.Preamp)
	for i, f := range e.bands {
		if i < len(eq.Gains) {
			f.Set(eq.Gains[i])
		}
	}
	for i, pr := range players.EqPresets() {
		if pr.Name == eq.Preset {
			e.preset.Selected = i
		}
	}
	e.preset.Invalidate()
	e.Invalidate()
}

func (e *eqPane) Measure(c layout.Constraints) paintengine2d.Point {
	lk := e.Look()
	if sz, ok := e.slots.Measure(lk, c); ok {
		return sz
	}
	return c.Constrain(paintengine2d.Pt(style.Dip(lk, StripW), style.Dip(lk, 80)))
}

func (e *eqPane) Arrange(r paintengine2d.Rect) {
	e.SetBounds(r)
	f := faceOf(e.Look())
	e.show(f)
	if f.panelled() {
		e.slots.Arrange(e.Look(), e.LocalBounds())
		e.flat.Arrange(paintengine2d.Rect{})
		e.preset.Arrange(paintengine2d.Rect{})
		return
	}
	e.arrangeWidgets()
}

func (e *eqPane) arrangeWidgets() {
	lk := e.Look()
	dip := func(v float32) float32 { return style.Dip(lk, v) }
	b := e.LocalBounds()

	row := dip(18)
	e.on.Arrange(paintengine2d.XYWH(0, 0, row, row))
	// The Flat button takes whatever its own label needs: the word is four
	// letters in the skin's face and rather more in some of the other
	// hundred and twenty packs, and a button that elides its own name is a
	// button nobody can read.
	flatW := max(e.flat.Measure(layout.Loose(b.Dx(), row)).X, dip(38))
	e.flat.Arrange(paintengine2d.XYWH(row+dip(4), 0, flatW, row))
	px := row + dip(4) + flatW + dip(4)
	e.preset.Arrange(paintengine2d.XYWH(px, 0, max(b.Dx()-px, dip(60)), row))

	// The faders: the preamp, a gap, then the ten bands sharing what is
	// left. Every one is the same width, so the row reads as a set.
	top := row + dip(6)
	h := max(b.Dy()-top-dip(11), dip(20))
	pre := dip(22)
	e.preamp.Arrange(paintengine2d.XYWH(0, top, pre, h))
	x := pre + dip(8)
	span := b.Dx() - x
	n := float32(len(e.bands))
	each := span / n
	for i, f := range e.bands {
		f.Arrange(paintengine2d.XYWH(x+float32(i)*each, top, each-dip(1), h))
	}
	e.auto.Arrange(paintengine2d.Rect{})
	e.presets.Arrange(paintengine2d.Rect{})
}

// Paint draws the zero rule behind the faders, which is the one mark that
// says which way is up on a control whose whole range is a number nobody
// reads — or, in a panel, the panel and the curve the ten bands make.
func (e *eqPane) Paint(ctx *paintengine2d.Context) {
	lk, b := e.Look(), e.LocalBounds()
	if len(e.bands) == 0 {
		return
	}
	if f := faceOf(lk); f.panelled() {
		style.DrawSkinLayout(lk, ctx, b, layoutEqualiser)
		e.paintCurve(ctx, lk, slotRect(lk, layoutEqualiser, "graph", b), f.ink())
		return
	}
	pal := lk.Palette()
	first := e.bands[0].Bounds()
	y := (first.Min.Y + first.Max.Y) / 2
	ctx.DrawRect(paintengine2d.XYWH(b.Min.X, y, b.Dx(), max(style.Dip(lk, 1), 1)),
		paintengine2d.Fill(pal.Divider))

	// The frequencies, under the faders they belong to. Nine design pixels
	// is small, and it is what the row of a control this size is: the
	// numbers are a scale, not a label anybody reads one of.
	tiny := players.ScaledFace(lk.Font(), style.Dip(lk, 9))
	row := paintengine2d.XYWH(0, first.Max.Y+style.Dip(lk, 1), 0, style.Dip(lk, 10))
	pre := e.preamp.Bounds()
	players.DrawTextIn(ctx, tiny, paintengine2d.XYWH(pre.Min.X, row.Min.Y, pre.Dx(), row.Dy()),
		"pre", pal.TextMuted, style.AlignCenter)
	for i, f := range e.bands {
		fb := f.Bounds()
		players.DrawTextIn(ctx, tiny, paintengine2d.XYWH(fb.Min.X, row.Min.Y, fb.Dx(), row.Dy()),
			players.EqBands[i], pal.TextMuted, style.AlignCenter)
	}
}

// paintCurve draws the response the ten bands make across the graph well g,
// a design pixel at a time so it sits on the panel's grid.
func (e *eqPane) paintCurve(ctx *paintengine2d.Context, lk style.LookAndFeel, g paintengine2d.Rect, in *ink) {
	d := style.Dip(lk, 1)
	n := int(g.Dx()/d+0.5) - 4
	if n < 2 {
		return
	}
	pts := e.p.Equaliser.Curve(n)
	mid := g.Min.Y + g.Dy()/2
	amp := g.Dy()/2 - 2*d
	col := paintengine2d.Fill(in.curve)
	if !e.p.Equaliser.On {
		col = paintengine2d.Fill(style.Mix(in.curve, paintengine2d.RGB(0.4, 0.4, 0.45), 0.35))
	}
	for i, v := range pts {
		x := g.Min.X + 2*d + float32(i)*d
		y := g.Min.Y + snapDesign(mid-g.Min.Y-v*amp, d)
		ctx.DrawRect(paintengine2d.XYWH(x, y, d, d), col)
	}
}

// MousePress on the equaliser's own face moves the stack from there, and
// opens the skin menu on the secondary button.
func (e *eqPane) MousePress(ev widget.MouseEvent) bool {
	if e.p.rightClick(e, ev) {
		return true
	}
	e.p.Eq.StartMove()
	return true
}

func (e *eqPane) KeyPress(ev widget.KeyEvent) bool { return e.p.Keys(e.p.Eq, ev) }

func (e *eqPane) CaptionAt(paintengine2d.Point) bool { return true }

func (e *eqPane) Describe(n *a11y.Node) {
	n.Role = a11y.RoleGroup
	if n.Name == "" {
		n.Name = "Equaliser"
	}
	n.Description = "Ten bands and a preamp. " + players.Disclaimer
}

// ---- the playlist ------------------------------------------------------------

// listPane is the queue: a row per track, and a footer that counts them.
type listPane struct {
	widget.Base
	p *Player

	list   *tracks
	foot   *foot
	toTop  *widgets.Button
	shrink *players.GlyphButton

	// The panel faces' keys along the bottom, and the small transport.
	add, rem, sel, misc, opts *players.GlyphButton
	mini                      [6]*players.GlyphButton

	// slots is where each control goes in a panel face.
	slots *widget.Slots
}

func newListPane(p *Player) *listPane {
	l := &listPane{p: p}
	l.Init(l)
	l.SetManagesChildren(true)

	l.list = newTracks(p)
	l.foot = &foot{p: p}
	l.foot.Init(l.foot)
	l.foot.Win = p.List
	l.toTop = widgets.NewButton("Top", func() {
		p.Transport.SelectTrack(0)
	})
	l.toTop.Tip = "Play the first track"
	l.shrink = players.NewGlyphButton(players.GlyphShrink, "Close the playlist", func() {
		p.toggle(p.iList)
	})

	// The four keys of the era's playlist, and its options key. There is
	// nothing to add or remove — the library is invented and nothing is
	// played — so each opens a menu that says what it would do and does the
	// part that is true here, rather than doing nothing silently.
	l.add = players.NewGlyphButton(players.GlyphNone, "Add", func() {
		l.menu(l.add, widgets.MenuItem{Text: "Adding files needs a media stack: Minim plays nothing", Disabled: true})
	})
	l.rem = players.NewGlyphButton(players.GlyphNone, "Remove", func() {
		l.menu(l.rem, widgets.MenuItem{Text: "The library is invented and stays as it is", Disabled: true})
	})
	l.sel = players.NewGlyphButton(players.GlyphNone, "Select", func() {
		l.menu(l.sel, widgets.MenuItem{Text: "Show the playing track", OnClick: func() {
			l.list.EnsureVisible(p.Transport.List.Index())
		}})
	})
	// The faces go here rather than in the strip's own menu: a menu is
	// drawn inside the window it opens from, and the strip is a hundred
	// and sixteen design pixels tall — three more rows and a separator
	// would have it scroll or cut its labels off. The playlist window is
	// twice that, and the keys work in every window of every face.
	l.misc = players.NewGlyphButton(players.GlyphNone, "Miscellaneous: faces and skins", func() {
		o := widget.DeviceOrigin(l.misc)
		p.Host.ContextMenu(l.misc, paintengine2d.Pt(o.X, o.Y+l.misc.Bounds().Dy()))
	})
	l.opts = players.NewGlyphButton(players.GlyphNone, "List options", func() {
		l.menu(l.opts,
			widgets.MenuItem{Text: "Play the first track", OnClick: func() { p.Transport.SelectTrack(0) }},
			widgets.MenuItem{Text: "Close the playlist", OnClick: func() { p.toggle(p.iList) }})
	})
	l.slots = widget.NewSlots(layoutPlaylist).Bind("list", l.list).Bind("info", l.foot)
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
		{players.GlyphEject, "mini.eject", "Eject: stop and go back to the first track", players.CmdNone},
	} {
		m := m
		b := players.NewGlyphButton(m.g, m.label, func() {
			switch m.name {
			case "mini.play":
				p.Transport.Play()
				p.Command(players.CmdNone)
			case "mini.eject":
				p.Transport.SelectTrack(0)
				p.Command(players.CmdStop)
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
	l.Add(l.toTop)
	l.Add(l.shrink)
	for _, b := range []*players.GlyphButton{l.add, l.rem, l.sel, l.misc} {
		l.Add(b)
	}
	for _, b := range l.mini {
		l.Add(b)
	}
	l.Add(l.opts)
	return l
}

// menu opens a small menu under one of the keys.
func (l *listPane) menu(under *players.GlyphButton, items ...widgets.MenuItem) {
	rows := make([]*widgets.MenuItem, len(items))
	for i := range items {
		rows[i] = &items[i]
	}
	o := widget.DeviceOrigin(under)
	widgets.ShowContextMenu(under, paintengine2d.Pt(o.X, o.Y+under.Bounds().Dy()), rows...)
}

// show puts on the keys a face has and takes off the others.
func (l *listPane) show(f face) {
	panelled := f.panelled()
	for _, b := range []*players.GlyphButton{l.add, l.rem, l.sel, l.misc, l.opts} {
		b.SetVisible(panelled)
	}
	for _, b := range l.mini {
		b.SetVisible(panelled)
	}
	l.toTop.SetVisible(!panelled)
	l.shrink.SetVisible(!panelled)
}

func (l *listPane) sync() {
	if i := l.p.Transport.List.Index(); i >= 0 && l.list.Selected != i {
		l.list.Selected = i
		l.list.EnsureVisible(i)
	}
	l.list.Invalidate()
	l.foot.Invalidate()
	if faceOf(l.Look()).panelled() {
		// The small display and clock are printed on the pane itself.
		l.Invalidate()
	}
}

func (l *listPane) Measure(c layout.Constraints) paintengine2d.Point {
	lk := l.Look()
	if sz, ok := l.slots.Measure(lk, c); ok {
		return sz
	}
	return c.Constrain(paintengine2d.Pt(style.Dip(lk, StripW), style.Dip(lk, 196)))
}

func (l *listPane) Arrange(r paintengine2d.Rect) {
	l.SetBounds(r)
	f := faceOf(l.Look())
	l.show(f)
	if f.panelled() {
		l.arrangePanel()
		return
	}
	l.list.geo = nil
	lk := l.Look()
	dip := func(v float32) float32 { return style.Dip(lk, v) }
	b := l.LocalBounds()
	row := dip(32)
	l.list.Arrange(paintengine2d.XYWH(0, 0, b.Dx(), max(b.Dy()-row-dip(4), dip(40))))
	y := b.Dy() - row
	btn := dip(38)
	ctl := dip(20)
	l.foot.Arrange(paintengine2d.XYWH(0, y, max(b.Dx()-btn-ctl-dip(8), 0), row))
	l.toTop.Arrange(paintengine2d.XYWH(b.Dx()-btn-ctl-dip(4), y+(row-ctl)/2, btn, ctl))
	l.shrink.Arrange(paintengine2d.XYWH(b.Dx()-ctl, y+(row-ctl)/2, ctl, ctl))
	for _, c := range append([]*players.GlyphButton{l.add, l.rem, l.sel, l.misc, l.opts}, l.mini[:]...) {
		c.Arrange(paintengine2d.Rect{})
	}
}

// arrangePanel puts every control on its slot, and tells the list where in
// its own box the panel's rows, its scroll groove and its first row are.
func (l *listPane) arrangePanel() {
	lk, b := l.Look(), l.LocalBounds()
	l.slots.Arrange(lk, b)
	at := func(slot string) paintengine2d.Rect { return slotRect(lk, layoutPlaylist, slot, b) }
	// In the list's own coordinates: its box is whole pixels, and the rows
	// are drawn where the skin's rects are.
	o := l.list.Bounds().Min
	l.list.geo = &listGeo{
		rows: at("rows").Translate(paintengine2d.Pt(-o.X, -o.Y)),
		bar:  at("scroll").Translate(paintengine2d.Pt(-o.X, -o.Y)),
		rowH: at("row").Dy(),
	}
	l.toTop.Arrange(paintengine2d.Rect{})
	l.shrink.Arrange(paintengine2d.Rect{})
}

// Paint is the panel, in a panel face, and over it the small display — the
// playing track's length over the queue's — and the small clock.
func (l *listPane) Paint(ctx *paintengine2d.Context) {
	lk := l.Look()
	f := faceOf(lk)
	if !f.panelled() {
		return
	}
	b := l.LocalBounds()
	style.DrawSkinLayout(lk, ctx, b, layoutPlaylist)
	in := f.ink()
	t := l.p.Transport
	info := slotRect(lk, layoutPlaylist, "info", b)
	line := players.Clock(t.Length()) + "/" + players.Clock(t.List.Total())
	pixText(lk, ctx, info.Min.X+style.Dip(lk, 3), info.Min.Y+(info.Dy()-style.Dip(lk, 6))/2, line, in.info)
	clk := slotRect(lk, layoutPlaylist, "clock", b)
	c := players.Clock(t.Pos)
	if t.State == players.Stopped {
		c = players.Clock(0)
	}
	w := pixWidth(lk, c)
	pixText(lk, ctx, clk.Max.X-w-style.Dip(lk, 3), clk.Min.Y+(clk.Dy()-style.Dip(lk, 6))/2, c, in.info)
}

func (l *listPane) MousePress(e widget.MouseEvent) bool {
	if l.p.rightClick(l, e) {
		return true
	}
	l.p.List.StartMove()
	return true
}

func (l *listPane) KeyPress(e widget.KeyEvent) bool { return l.p.Keys(l.p.List, e) }

func (l *listPane) CaptionAt(paintengine2d.Point) bool { return true }

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

func (f *foot) Measure(c layout.Constraints) paintengine2d.Point {
	return c.Constrain(paintengine2d.Pt(style.Dip(f.Look(), 120), style.Dip(f.Look(), 32)))
}

func (f *foot) Arrange(b paintengine2d.Rect) { f.SetBounds(b) }

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
func (f *foot) Paint(ctx *paintengine2d.Context) {
	lk, b := f.Look(), f.LocalBounds()
	if faceOf(lk).panelled() {
		return
	}
	pal := lk.Palette()
	half := b.Dy() / 2
	lk.DrawLabel(ctx, paintengine2d.XYWH(b.Min.X, b.Min.Y, b.Dx(), half),
		f.count(), pal.TextMuted, style.AlignStart)
	tiny := players.ScaledFace(lk.Font(), style.Dip(lk, 10))
	players.DrawTextIn(ctx, tiny, paintengine2d.XYWH(b.Min.X, b.Min.Y+half, b.Dx(), half),
		players.Short, style.Mix(pal.TextMuted, pal.Background, 0.2), style.AlignStart)
}

func (f *foot) count() string {
	pl := f.p.Transport.List
	return fmt.Sprintf("%d tracks · %s", pl.Len(), players.Clock(pl.Total()))
}

func (f *foot) text() string { return f.count() + " · " + players.Disclaimer }

func (f *foot) Describe(n *a11y.Node) {
	n.Role = a11y.RoleStatusBar
	n.Name = f.text()
}
