package pod

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
	p            *Player
	slots        *skins.Slots
	buttons      map[string]*players.GlyphButton
	play, eq, pl *players.GlyphButton
	seek, volume *curvedSlider
	bands        []*players.Fader
	eqControls   []widget.Component
	visual       *visualizer
}

func newBody(p *Player) *body {
	b := &body{p: p, slots: skins.NewSlots(LayoutMain), buttons: make(map[string]*players.GlyphButton)}
	b.Init(b)
	b.SetManagesChildren(true)
	b.SetAccessibleName("Pod music player")
	// The visualization is behind the curved volume track and its controls.
	b.visual = newVisualizer(p)
	b.slots.Bind("analyser", b.visual)
	b.Add(b.visual)
	button := func(slot, name string, glyph players.Glyph, action func()) *players.GlyphButton {
		c := skinButton(LayoutMain, slot, name, glyph, action, func() bool { return p.Transport.State == players.Playing })
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
	button("open", "Open music files", players.GlyphEject, func() { p.Host.OpenFiles(b.buttons["open"], false) })
	button("next", "Next track", players.GlyphNext, func() { p.Host.Command(players.CmdNext) })
	button("minimize", "Minimize Music Player", players.GlyphNone, p.Main.Minimize)
	button("close", "Close to tray", players.GlyphNone, p.Main.RequestClose)
	menu := func() {
		c := b.buttons["skin"]
		o := widget.DeviceOrigin(c)
		p.Host.ContextMenu(c, paint2d.Pt(o.X, o.Y+c.Bounds().Dy()))
	}
	button("skin", "Appearance and settings", players.GlyphSkin, menu)
	button("logo", "Appearance and settings", players.GlyphSkin, menu)
	b.eq = button("eq", "Show equalizer", players.GlyphSliders, func() { p.ShowEqualiser(!p.Host.Model.EqualizerVisible) })
	b.eq.Toggle = true
	b.pl = button("pl", "Show playlist", players.GlyphList, func() { p.ShowPlaylist(!p.Host.Model.PlaylistVisible) })
	b.pl.Toggle = true
	switchVisual := func() { p.ShowEqualiser(false); b.visual.mode = (b.visual.mode + 1) % 2; b.visual.Invalidate() }
	button("vis", "Switch visualization", players.GlyphNone, switchVisual)
	button("vis.prev", "Previous visualization", players.GlyphNone, switchVisual)
	button("vis.next", "Next visualization", players.GlyphNone, switchVisual)
	b.seek = newCurvedSlider("seek", 0, 1, 0, "Seek", p.Host.Seek)
	b.seek.Step, b.seek.Page = .02, .1
	b.seek.Format = func(v float32) string {
		return players.Clock(time.Duration(float64(p.Transport.Length())*float64(v))) + " of " + players.Clock(p.Transport.Length())
	}
	b.volume = newCurvedSlider("volume", 0, 100, p.Transport.Volume*100, "Volume", func(v float32) { p.Host.SetVolume(v / 100) })
	b.volume.Step, b.volume.Page = 2, 10
	b.volume.Format = func(v float32) string { return fmt.Sprintf("%d%%", int(v+.5)) }
	for _, item := range []struct {
		slot   string
		f      *curvedSlider
		frames int
	}{{"seek", b.seek, 32}, {"volume", b.volume, 30}} {
		item := item
		// These are the original custom-slider animation frames, not a
		// straight track painted over the skin's curved display.
		item.f.Travel = .001
		item.f.Painter = func(ctx *paint2d.Context, _ paint2d.Rect, t float32, _ style.ControlState) bool {
			n := int(t*float32(item.frames-1) + .5)
			return skins.DrawSkinSprite(item.f.Look(), ctx, skins.SlotArtRect(item.f, LayoutMain, item.slot), fmt.Sprintf("%s.%d", item.slot, n), paint2d.Color{})
		}
		b.slots.Bind(item.slot, item.f)
		b.Add(item.f)
	}
	for i, name := range players.EqBands {
		band := i
		f := players.NewFader(-players.EqRange, players.EqRange, p.Equaliser.Gains[i], name+" Hz", func(v float32) { p.Equaliser.Set(band, v) })
		f.Format = players.Decibels
		f.Travel = 2
		f.Painter = func(ctx *paint2d.Context, r paint2d.Rect, t float32, st style.ControlState) bool {
			d := style.Dip(f.Look(), 1)
			thumb := paint2d.XYWH(r.Min.X, r.Min.Y+(1-t)*(r.Dy()-4*d), r.Dx(), 4*d)
			return skins.DrawSkinSprite(f.Look(), ctx, thumb, "eq.thumb", paint2d.Color{})
		}
		b.slots.Bind(fmt.Sprintf("eq.%d", i), f)
		b.bands = append(b.bands, f)
		b.eqControls = append(b.eqControls, f)
		b.Add(f)
	}
	for _, slot := range []string{"eq.prev", "eq.next"} {
		slot := slot
		c := button(slot, "Change equalizer preset", players.GlyphNone, func() { b.stepPreset(slot == "eq.next") })
		b.eqControls = append(b.eqControls, c)
	}
	presets := button("eq.preset", "Equalizer presets and reset", players.GlyphNone, b.eqMenu)
	// This slot was text in the original skin. Keep it text, with ordinary
	// button input and focus, so every EQ action remains reachable.
	presets.Shaper = nil
	presets.Art = func(ctx *paint2d.Context, r paint2d.Rect, _ style.ControlState) bool {
		players.DrawTextIn(ctx, players.ScaledFace(presets.Look().Font(), style.Dip(presets.Look(), 7)), r, p.Equaliser.Preset, green(), style.AlignCenter)
		return true
	}
	b.eqControls = append(b.eqControls, presets)
	return b
}

func skinButton(role, slot, name string, glyph players.Glyph, action func(), playing func() bool) *players.GlyphButton {
	c := players.NewGlyphButton(glyph, name, action)
	artSlot := func() string {
		if slot == "play" && playing != nil && playing() {
			return "pause"
		}
		return slot
	}
	c.Art = func(ctx *paint2d.Context, _ paint2d.Rect, st style.ControlState) bool {
		return skins.DrawSkinSlot(c.Look(), ctx, skins.SlotArtRect(c, role, artSlot()), role, artSlot(), st)
	}
	c.Shaper = func(lk style.LookAndFeel, size paint2d.Point) *style.Silhouette {
		return skins.SkinSlotShape(lk, size, skins.SlotArtRect(c, role, slot), role, slot)
	}
	return c
}
func green() paint2d.Color { return paint2d.RGB(.62, .95, .38) }
func (b *body) stepPreset(next bool) {
	list := players.EqPresets()
	if len(list) == 0 {
		return
	}
	idx := 0
	for i, v := range list {
		if v.Name == b.p.Equaliser.Preset {
			idx = i
			break
		}
	}
	if next {
		idx = (idx + 1) % len(list)
	} else {
		idx = (idx + len(list) - 1) % len(list)
	}
	b.p.Equaliser.Apply(list[idx].Name)
}
func (b *body) eqMenu() {
	c := b.buttons["eq.preset"]
	e := b.p.Equaliser
	o := widget.DeviceOrigin(c)
	items := []*widgets.MenuItem{widgets.CheckItem("Equalizer on", e.On, func() { e.SetOn(!e.On) }), widgets.CheckItem("Auto preset per track", e.Auto, func() { e.SetAuto(!e.Auto, b.p.Transport.List.Index()) }), widgets.Item("Reset", e.Flat), widgets.Sep()}
	var preamp []*widgets.MenuItem
	for _, gain := range []float32{-12, -9, -6, -3, 0, 3, 6, 9, 12} {
		gain := gain
		preamp = append(preamp, widgets.RadioItem(players.Decibels(gain), "preamp", e.Preamp == gain, func() { e.SetPreamp(gain) }))
	}
	items = append(items, widgets.Submenu("Preamp: "+players.Decibels(e.Preamp), preamp...), widgets.Sep())
	var balance []*widgets.MenuItem
	for _, pan := range []float32{-1, -.5, 0, .5, 1} {
		pan := pan
		name := "Center"
		if pan < 0 {
			name = fmt.Sprintf("%d%% left", int(-pan*100))
		}
		if pan > 0 {
			name = fmt.Sprintf("%d%% right", int(pan*100))
		}
		balance = append(balance, widgets.RadioItem(name, "balance", b.p.Transport.Pan == pan, func() { b.p.Host.SetPan(pan) }))
	}
	panMenu := widgets.Submenu("Balance", balance...)
	items = append(items, panMenu, widgets.Sep())
	for _, pr := range players.EqPresets() {
		name := pr.Name
		items = append(items, widgets.RadioItem(name, "preset", name == e.Preset, func() { e.Apply(name) }))
	}
	widgets.ShowContextMenu(c, paint2d.Pt(o.X, o.Y+c.Bounds().Dy()), items...)
}
func (b *body) sync() {
	p, t := b.p, b.p.Transport
	eq := p.Host.Model.EqualizerVisible
	if t.State == players.Playing {
		b.play.SetGlyph(players.GlyphPause, "Pause")
	} else {
		b.play.SetGlyph(players.GlyphPlay, "Play")
	}
	b.eq.SetChecked(eq)
	b.pl.SetChecked(p.Host.Model.PlaylistVisible)
	for _, c := range b.eqControls {
		c.SetVisible(eq)
	}
	for i, f := range b.bands {
		f.Set(p.Equaliser.Gains[i])
	}
	b.visual.SetVisible(!eq)
	b.visual.Invalidate()
	if !b.seek.Dragging() {
		b.seek.Set(t.Fraction())
	}
	if !b.volume.Dragging() {
		b.volume.Set(t.Volume * 100)
	}
	b.Invalidate()
}
func (b *body) Measure(c layout.Constraints) paint2d.Point {
	sz, _ := b.slots.Measure(b.Look(), c)
	return sz
}
func (b *body) Arrange(r paint2d.Rect) {
	b.SetBounds(r)
	b.slots.Arrange(b.Look(), b.LocalBounds())
}
func (b *body) Paint(ctx *paint2d.Context) {
	lk, box := b.Look(), b.LocalBounds()
	skins.DrawSkinLayout(lk, ctx, box, LayoutMain)
	if b.p.Host.Model.EqualizerVisible {
		r, _ := skins.SkinSlotRect(lk, LayoutMain, "eq.panel", box)
		skins.DrawSkinSprite(lk, ctx, r, "eq.panel", paint2d.Color{})
	}
	at := func(slot string) paint2d.Rect { r, _ := skins.SkinSlotRect(lk, LayoutMain, slot, box); return r }
	title := b.p.Transport.Track().Label()
	if title == "" {
		title = "Music Player"
	}
	if b.p.Host.Model.Error != "" {
		title = b.p.Host.StatusText()
	}
	players.DrawTextIn(ctx, players.ScaledFace(lk.Font(), style.Dip(lk, 8)), at("title"), title, green(), style.AlignCenter)
	players.DrawTextIn(ctx, players.ScaledFace(lk.Font(), style.Dip(lk, 8)), at("time"), players.Clock(b.p.Transport.Pos), green(), style.AlignCenter)
}
func (b *body) MousePress(e widget.MouseEvent) bool {
	if e.Button == platform.ButtonRight {
		b.p.Host.ContextMenu(b, e.Pos.Add(widget.DeviceOrigin(b)))
		return true
	}
	if e.Button == platform.ButtonLeft {
		delete(b.p.restore, "main")
		b.p.Main.StartMove()
		return true
	}
	return false
}
func (b *body) KeyPress(e widget.KeyEvent) bool { return b.p.keys(b.p.Main, e) }
func (b *body) Describe(n *a11y.Node)           { n.Role, n.Name = a11y.RoleGroup, "Pod music player" }
func (b *body) DropTypes() []string             { return []string{"text/uri-list"} }
func (b *body) DropActionFor(offered platform.DragAction) platform.DragAction {
	return offered & platform.DragCopy
}
func (b *body) Drop(e widget.DropEvent) bool { return b.p.Host.AcceptFileDrop(e) }
