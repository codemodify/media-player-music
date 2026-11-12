package wingspan

import (
	"fmt"
	"time"

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

type body struct {
	widget.Base
	p                        *Player
	slots                    *skins.Slots
	play, eq, pl             *players.GlyphButton
	volume, balance, seek    *players.Fader
	bands                    []*players.Fader
	list                     *playlistList
	visual                   *visualizer
	buttons                  map[string]*players.GlyphButton
	eqControls, listControls []widget.Component
}

func newBody(p *Player) *body {
	b := &body{p: p, slots: skins.NewSlots(LayoutMain), buttons: make(map[string]*players.GlyphButton)}
	b.Init(b)
	b.SetManagesChildren(true)
	b.SetAccessibleName("Headspace music player")
	button := func(slot, name string, glyph players.Glyph, action func()) *players.GlyphButton {
		c := players.NewGlyphButton(glyph, name, action)
		b.paintButton(c, slot)
		b.buttons[slot] = c
		b.slots.Bind(slot, c)
		b.Add(c)
		return c
	}
	button("prev", "Previous track", players.GlyphPrev, func() { p.Host.Command(players.CmdPrev) })
	b.play = button("play", "Play", players.GlyphPlay, func() {
		if p.Transport.State == players.Stopped {
			p.Host.Play()
		} else {
			p.Host.Command(players.CmdPlayPause)
		}
	})
	button("stop", "Stop", players.GlyphStop, func() { p.Host.Command(players.CmdStop) })
	button("next", "Next track", players.GlyphNext, func() { p.Host.Command(players.CmdNext) })
	button("vis", "Switch visualization", players.GlyphNone, func() { b.visual.bars = !b.visual.bars; b.visual.Invalidate() })
	button("minimize", "Minimize Music Player", players.GlyphNone, p.Main.Minimize)
	button("close", "Close to tray", players.GlyphNone, p.Main.RequestClose)
	b.eq = button("eq", "Equalizer wing", players.GlyphSliders, func() { p.ShowEqualiser(!p.Host.Model.EqualizerVisible) })
	b.eq.Toggle = true
	b.pl = button("pl", "Playlist wing", players.GlyphList, func() { p.ShowPlaylist(!p.Host.Model.PlaylistVisible) })
	b.pl.Toggle = true
	button("skin", "Appearance and settings", players.GlyphSkin, func() {
		o := widget.DeviceOrigin(b.buttons["skin"])
		p.Host.ContextMenu(b.buttons["skin"], paint2d.Pt(o.X, o.Y+b.buttons["skin"].Bounds().Dy()))
	})
	button("eq.handle", "Open or close equalizer", players.GlyphNone, func() { p.ShowEqualiser(!p.Host.Model.EqualizerVisible) })
	button("pl.handle", "Open or close playlist", players.GlyphNone, func() { p.ShowPlaylist(!p.Host.Model.PlaylistVisible) })
	eqClose := button("eq.close", "Close equalizer wing", players.GlyphNone, func() { p.ShowEqualiser(false) })
	plClose := button("pl.close", "Close playlist wing", players.GlyphNone, func() { p.ShowPlaylist(false) })
	reset := button("eq.reset", "Equalizer presets and reset", players.GlyphNone, b.eqMenu)
	b.eqControls = append(b.eqControls, eqClose, reset)
	b.listControls = append(b.listControls, plClose)

	b.seek = players.NewBar(0, 1, 0, "Seek", p.Host.Seek)
	b.seek.Step, b.seek.Page = .02, .1
	b.seek.Format = func(v float32) string {
		return players.Clock(time.Duration(float64(p.Transport.Length())*float64(v))) + " of " + players.Clock(p.Transport.Length())
	}
	b.volume = players.NewBar(0, 100, p.Transport.Volume*100, "Volume", func(v float32) { p.Host.SetVolume(v / 100) })
	b.volume.Format = func(v float32) string { return fmt.Sprintf("%d%%", int(v+.5)) }
	b.balance = players.NewBar(-1, 1, p.Transport.Pan, "Balance", p.Host.SetPan)
	b.balance.Format = func(v float32) string {
		if v == 0 {
			return "Center"
		}
		if v < 0 {
			return fmt.Sprintf("%d%% left", int(-v*100+.5))
		}
		return fmt.Sprintf("%d%% right", int(v*100+.5))
	}
	for _, item := range []struct {
		slot string
		f    *players.Fader
	}{{"seek", b.seek}, {"volume", b.volume}, {"balance", b.balance}} {
		b.slots.Bind(item.slot, item.f)
		b.paintSlider(item.f, item.slot)
		b.Add(item.f)
	}
	b.eqControls = append(b.eqControls, b.volume, b.balance)
	for i, name := range players.EqBands {
		band := i
		f := players.NewFader(-players.EqRange, players.EqRange, p.Equaliser.Gains[i], name+" Hz", func(v float32) { p.Equaliser.Set(band, v) })
		f.Format = players.Decibels
		slot := fmt.Sprintf("eq.%d", i)
		b.slots.Bind(slot, f)
		b.paintSlider(f, slot)
		b.bands = append(b.bands, f)
		b.eqControls = append(b.eqControls, f)
		b.Add(f)
	}
	b.list = newPlaylistList(p)
	b.slots.Bind("list", b.list)
	b.listControls = append(b.listControls, b.list)
	b.Add(b.list)
	add := button("list.header", "Open or add music files", players.GlyphNone, func() {
		c := b.buttons["list.header"]
		o := widget.DeviceOrigin(c)
		widgets.ShowContextMenu(c, paint2d.Pt(o.X, o.Y+c.Bounds().Dy()),
			widgets.Item("Open music files…", func() { p.Host.OpenFiles(c, false) }),
			widgets.Item("Add music files…", func() { p.Host.OpenFiles(c, true) }),
			widgets.Item("Remove current track", func() { p.Host.RemoveTrack() }),
			widgets.Sep(), widgets.CheckItem("Shuffle", p.Transport.Random, func() { p.Host.Command(players.CmdShuffle) }),
			widgets.Item("Repeat: "+p.Transport.Repeat.String(), func() { p.Host.Command(players.CmdRepeat) }))
	})
	b.listControls = append(b.listControls, add)
	b.visual = newVisualizer(p)
	b.slots.Bind("analyser", b.visual)
	b.Add(b.visual)
	return b
}

func (b *body) paintButton(c *players.GlyphButton, slot string) {
	c.Art = func(ctx *paint2d.Context, _ paint2d.Rect, st style.ControlState) bool {
		artSlot := slot
		if slot == "play" && b.p.Transport.State == players.Playing {
			artSlot = "pause"
		}
		if slot == "eq.handle" && !b.p.Host.Model.EqualizerVisible {
			artSlot += ".closed"
		}
		if slot == "pl.handle" && !b.p.Host.Model.PlaylistVisible {
			artSlot += ".closed"
		}
		r := skins.SlotArtRect(c, LayoutMain, artSlot)
		return skins.DrawSkinSlot(c.Look(), ctx, r, LayoutMain, artSlot, st)
	}
	c.Shaper = func(lk style.LookAndFeel, size paint2d.Point) *style.Silhouette {
		artSlot := slot
		if slot == "eq.handle" && !b.p.Host.Model.EqualizerVisible {
			artSlot += ".closed"
		}
		if slot == "pl.handle" && !b.p.Host.Model.PlaylistVisible {
			artSlot += ".closed"
		}
		return skins.SkinSlotShape(lk, size, skins.SlotArtRect(c, LayoutMain, artSlot), LayoutMain, artSlot)
	}
}

func (b *body) paintSlider(f *players.Fader, slot string) {
	prefix := "horizontal"
	travel := float32(7)
	if f.Vertical {
		prefix = "vertical"
	}
	if slot == "seek" {
		prefix, travel = "seek", 9
	}
	f.Travel = travel
	f.Painter = func(ctx *paint2d.Context, _ paint2d.Rect, t float32, st style.ControlState) bool {
		lk := f.Look()
		r := skins.SlotArtRect(f, LayoutMain, slot)
		skins.DrawSkinSprite(lk, ctx, r, prefix+".track", paint2d.Color{})
		name := prefix + ".thumb"
		w, h, ok := skins.SkinSpriteSize(lk, name)
		if !ok {
			return false
		}
		w, h = style.Dip(lk, w), style.Dip(lk, h)
		pad := style.Dip(lk, travel)
		at := paint2d.XYWH(r.Min.X+pad+(r.Dx()-2*pad)*t-w/2, r.Min.Y+(r.Dy()-h)/2, w, h)
		if f.Vertical {
			at = paint2d.XYWH(r.Min.X+(r.Dx()-w)/2, r.Min.Y+pad+(r.Dy()-2*pad)*(1-t)-h/2, w, h)
		}
		if st.Pressed() && slot == "seek" {
			name += ".down"
		}
		skins.DrawSkinSprite(lk, ctx, at, name, paint2d.Color{})
		return true
	}
}

func (b *body) eqMenu() {
	c := b.buttons["eq.reset"]
	o := widget.DeviceOrigin(c)
	e := b.p.Equaliser
	items := []*widgets.MenuItem{
		widgets.CheckItem("Equalizer on", e.On, func() { e.SetOn(!e.On) }),
		widgets.CheckItem("Auto preset per track", e.Auto, func() { e.SetAuto(!e.Auto, b.p.Transport.List.Index()) }),
		widgets.Item("Reset", e.Flat), widgets.Sep(),
	}
	var preamp []*widgets.MenuItem
	for _, gain := range []float32{-12, -9, -6, -3, 0, 3, 6, 9, 12} {
		gain := gain
		preamp = append(preamp, widgets.RadioItem(players.Decibels(gain), "preamp", e.Preamp == gain, func() { e.SetPreamp(gain) }))
	}
	items = append(items, widgets.Submenu("Preamp: "+players.Decibels(e.Preamp), preamp...), widgets.Sep())
	for _, preset := range players.EqPresets() {
		name := preset.Name
		items = append(items, widgets.RadioItem(name, "preset", name == e.Preset, func() { e.Apply(name) }))
	}
	widgets.ShowContextMenu(c, paint2d.Pt(o.X, o.Y+c.Bounds().Dy()), items...)
}

func (b *body) controlVisible(c widget.Component) bool {
	for _, control := range b.eqControls {
		if control == c {
			return b.p.Host.Model.EqualizerVisible
		}
	}
	for _, control := range b.listControls {
		if control == c {
			return b.p.Host.Model.PlaylistVisible
		}
	}
	return true
}

func (b *body) sync() {
	p, t := b.p, b.p.Transport
	if t.State == players.Playing {
		b.play.SetGlyph(players.GlyphPause, "Pause")
	} else {
		b.play.SetGlyph(players.GlyphPlay, "Play")
	}
	b.eq.SetChecked(p.Host.Model.EqualizerVisible)
	b.pl.SetChecked(p.Host.Model.PlaylistVisible)
	for _, c := range b.eqControls {
		c.SetVisible(p.Host.Model.EqualizerVisible)
	}
	for _, c := range b.listControls {
		c.SetVisible(p.Host.Model.PlaylistVisible)
	}
	b.volume.Set(t.Volume * 100)
	b.balance.Set(t.Pan)
	if !b.seek.Dragging() {
		b.seek.Set(t.Fraction())
	}
	for i, f := range b.bands {
		f.Set(p.Equaliser.Gains[i])
	}
	b.list.syncQueue()
	b.list.Invalidate()
	b.visual.Invalidate()
	b.Invalidate()
}

func (b *body) Measure(c layout.Constraints) paint2d.Point {
	if sz, ok := b.slots.Measure(b.Look(), c); ok {
		return sz
	}
	return c.Constrain(paint2d.Pt(style.Dip(b.Look(), MainW), style.Dip(b.Look(), MainH)))
}
func (b *body) Arrange(r paint2d.Rect) {
	b.SetBounds(r)
	b.slots.Arrange(b.Look(), b.LocalBounds())
	for _, name := range []string{"eq.handle", "pl.handle"} {
		open := b.p.Host.Model.EqualizerVisible
		if name == "pl.handle" {
			open = b.p.Host.Model.PlaylistVisible
		}
		if !open {
			if at, ok := skins.SkinSlotRect(b.Look(), LayoutMain, name+".closed", b.LocalBounds()); ok {
				b.buttons[name].Arrange(at)
			}
		}
	}
}
func (b *body) Paint(ctx *paint2d.Context) {
	lk, box := b.Look(), b.LocalBounds()
	skins.DrawSkinSprite(lk, ctx, box, "face."+b.p.wingState(), paint2d.Color{})
	font := players.ScaledFace(lk.Font(), style.Dip(lk, 10))
	col := paint2d.RGB(.47, .81, .03)
	at := func(slot string) paint2d.Rect { r, _ := skins.SkinSlotRect(lk, LayoutMain, slot, box); return r }
	title := b.p.Transport.Track().Label()
	if title == "" {
		title = "Music Player"
	}
	if b.p.Host.Model.Error != "" {
		title = b.p.Host.StatusText()
	}
	players.DrawTextIn(ctx, font, at("title"), title, col, style.AlignCenter)
	font = players.ScaledFace(lk.Font(), style.Dip(lk, 9))
	players.DrawTextIn(ctx, font, at("time"), players.Clock(b.p.Transport.Pos)+" / "+players.Clock(b.p.Transport.Length()), col, style.AlignEnd)
	players.DrawTextIn(ctx, font, at("status"), b.p.Transport.State.String(), col, style.AlignStart)
	if b.p.Host.Model.EqualizerVisible {
		font = players.ScaledFace(lk.Font(), style.Dip(lk, 8))
		col = paint2d.RGB(.47, .81, .03)
		for slot, text := range map[string]string{"balance.label": "Balance", "volume.label": "Volume"} {
			players.DrawTextIn(ctx, font, at(slot), text, col, style.AlignCenter)
		}
		for i, name := range players.EqBands {
			players.DrawTextIn(ctx, font, at(fmt.Sprintf("eq.label.%d", i)), name, col, style.AlignCenter)
		}
	}
}
func (b *body) KeyPress(e widget.KeyEvent) bool { return b.p.keys(e) }
func (b *body) MousePress(e widget.MouseEvent) bool {
	if e.Button == platform.ButtonRight {
		b.p.Host.ContextMenu(b, e.Pos.Add(widget.DeviceOrigin(b)))
		return true
	}
	if e.Button == platform.ButtonLeft {
		b.p.restore = nil
		b.p.Main.StartMove()
		return true
	}
	return false
}
func (b *body) Describe(n *a11y.Node) { n.Role, n.Name = a11y.RoleGroup, "Headspace music player" }
func (b *body) DropTypes() []string   { return []string{"text/uri-list"} }
func (b *body) DropActionFor(offered platform.DragAction) platform.DragAction {
	return offered & platform.DragCopy
}
func (b *body) Drop(e widget.DropEvent) bool { return b.p.Host.AcceptFileDrop(e) }
