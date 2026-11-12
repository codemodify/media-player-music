package fandom

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
	p                             *Player
	slots                         *skins.Slots
	buttons                       map[string]*players.GlyphButton
	play, eq, pl                  *players.GlyphButton
	seek, volume, preamp, balance *silverSlider
	bands                         []*silverSlider
	eqControls, settingsControls  []widget.Component
	visual                        *players.AnalyserView
	settingsVisible               bool
}

func newBody(p *Player) *body {
	b := &body{p: p, slots: skins.NewSlots(LayoutMain), buttons: make(map[string]*players.GlyphButton), settingsVisible: true}
	b.Init(b)
	b.SetManagesChildren(true)
	b.SetAccessibleName("Fandom music player")
	b.visual = players.NewAnalyserView(p.Spectrum)
	b.visual.Tint = green()
	b.visual.Peaks = false
	b.visual.Pixelated = true
	b.visual.SetTransparent(true)
	b.slots.Bind("analyser", b.visual)
	b.Add(b.visual)
	button := func(slot, name string, glyph players.Glyph, action func()) *players.GlyphButton {
		c := silverButton(slot, name, glyph, action)
		b.buttons[slot] = c
		b.slots.Bind(slot, c)
		b.Add(c)
		return c
	}
	b.play = button("play", "Play", players.GlyphPlay, func() {
		if p.Transport.State == players.Stopped {
			p.Host.Play()
		} else {
			p.Host.Command(players.CmdPlayPause)
		}
	})
	button("stop", "Stop", players.GlyphStop, func() { p.Host.Command(players.CmdStop) })
	button("prev", "Previous track", players.GlyphPrev, func() { p.Host.Command(players.CmdPrev) })
	button("next", "Next track", players.GlyphNext, func() { p.Host.Command(players.CmdNext) })
	button("open", "Open music files", players.GlyphEject, func() { p.Host.OpenFiles(b.buttons["open"], false) })
	button("minimize", "Minimize Music Player", players.GlyphNone, func() { p.Main().Minimize() })
	button("close", "Close to tray", players.GlyphNone, func() { p.Main().RequestClose() })
	button("logo", "Appearance and settings", players.GlyphSkin, func() {
		c := b.buttons["logo"]
		o := widget.DeviceOrigin(c)
		p.Host.ContextMenu(c, paint2d.Pt(o.X, o.Y+c.Bounds().Dy()))
	})
	button("settings", "Show settings panel", players.GlyphSliders, func() { b.setSettingsVisible(!b.settingsVisible) }).Toggle = true
	button("mute", "Mute", players.GlyphNone, func() { p.Host.Command(players.CmdMute) }).Toggle = true
	b.eq = button("eq", "Show equalizer", players.GlyphSliders, func() { p.ShowEqualiser(!p.Host.Model.EqualizerVisible) })
	b.eq.Toggle = true
	b.pl = button("pl", "Show playlist", players.GlyphList, func() { p.ShowPlaylist(!p.Host.Model.PlaylistVisible) })
	b.pl.Toggle = true
	b.seek = newSilverSlider(false, 0, 1, 0, "Seek", p.Host.Seek)
	b.seek.Step = .02
	b.seek.Page = .1
	b.seek.Format = func(v float32) string {
		return players.Clock(time.Duration(float64(p.Transport.Length())*float64(v))) + " of " + players.Clock(p.Transport.Length())
	}
	b.volume = newSilverSlider(false, 0, 100, p.Transport.Volume*100, "Volume", func(v float32) { p.Host.SetVolume(v / 100) })
	b.volume.Step = 2
	b.volume.Page = 10
	b.volume.Format = func(v float32) string { return fmt.Sprintf("%d%%", int(v+.5)) }
	for _, item := range []struct {
		slot string
		c    widget.Component
	}{{"seek", b.seek}, {"volume", b.volume}} {
		b.slots.Bind(item.slot, item.c)
		b.Add(item.c)
	}
	b.preamp = newSilverSlider(true, -players.EqRange, players.EqRange, p.Equaliser.Preamp, "Preamp", p.Equaliser.SetPreamp)
	b.preamp.Format = players.Decibels
	b.balance = newSilverSlider(false, -1, 1, p.Transport.Pan, "Balance", p.Host.SetPan)
	b.balance.Step = .1
	for _, item := range []struct {
		slot string
		c    widget.Component
	}{{"eq.preamp", b.preamp}, {"eq.balance", b.balance}} {
		b.slots.Bind(item.slot, item.c)
		b.eqControls = append(b.eqControls, item.c)
		b.Add(item.c)
	}
	for i, name := range players.EqBands {
		band := i
		f := newSilverSlider(true, -players.EqRange, players.EqRange, p.Equaliser.Gains[i], name+" Hz", func(v float32) { p.Equaliser.Set(band, v) })
		f.Format = players.Decibels
		b.slots.Bind(fmt.Sprintf("eq.%d", i), f)
		b.bands = append(b.bands, f)
		b.eqControls = append(b.eqControls, f)
		b.Add(f)
	}
	for _, item := range []struct {
		slot, name string
		action     func()
	}{{"eq.preset", "Equalizer presets", b.eqMenu}, {"eq.on", "Equalizer on", func() { p.Equaliser.SetOn(!p.Equaliser.On) }}, {"eq.auto", "Auto preset per track", func() { p.Equaliser.SetAuto(!p.Equaliser.Auto, p.Transport.List.Index()) }}} {
		c := button(item.slot, item.name, players.GlyphNone, item.action)
		c.Toggle = item.slot != "eq.preset"
		b.eqControls = append(b.eqControls, c)
		slot := item.slot
		c.Art = func(ctx *paint2d.Context, r paint2d.Rect, st style.ControlState) bool {
			text := p.Equaliser.Preset
			if slot == "eq.on" {
				text = "On"
			}
			if slot == "eq.auto" {
				text = "Auto"
			}
			col := green()
			if st.Disabled() {
				col = paint2d.RGB(.38, .40, .42)
			}
			if st.Checked() {
				ctx.DrawRect(r, paint2d.Fill(paint2d.RGB(.13, .27, .04)))
			}
			players.DrawTextIn(ctx, players.ScaledFace(c.Look().Font(), style.Dip(c.Look(), 10)), r, text, col, style.AlignCenter)
			return true
		}
	}
	setting := func(slot, label string, action func()) *players.GlyphButton {
		c := button(slot, label, players.GlyphNone, action)
		b.settingsControls = append(b.settingsControls, c)
		c.Art = func(ctx *paint2d.Context, r paint2d.Rect, st style.ControlState) bool {
			if st.Hovered() || st.Pressed() || st.Checked() {
				ctx.DrawRect(r, paint2d.Fill(paint2d.RGB(.16, .27, .42)))
			}
			col := paint2d.RGB(.86, .87, .88)
			if st.Disabled() {
				col = paint2d.RGB(.4, .42, .45)
			}
			players.DrawTextIn(ctx, players.ScaledFace(c.Look().Font(), style.Dip(c.Look(), 10)), r, label, col, style.AlignStart)
			return true
		}
		return c
	}
	setting("settings.general", "General", p.Host.OpenSettings).SetEnabled(p.Host.ShowSettings != nil)
	setting("settings.playback", "Playback", b.playbackMenu)
	setting("settings.library", "Library", func() { p.Host.OpenFiles(b.buttons["settings.library"], true) })
	setting("settings.skins", "Skins", func() {
		c := b.buttons["settings.skins"]
		o := widget.DeviceOrigin(c)
		p.Host.ContextMenu(c, paint2d.Pt(o.X, o.Y+c.Bounds().Dy()))
	})
	setting("settings.equalizer", "Equalizer", func() { p.ShowEqualiser(true) })
	setting("settings.backends", "Playback", p.Host.OpenSettings).SetEnabled(p.Host.ShowSettings != nil)
	setting("settings.nowplaying", "Now Playing", func() { p.ShowEqualiser(false) })
	setting("settings.about", "About", b.aboutMenu)
	c := button("settings.close", "Hide settings panel", players.GlyphNone, func() { b.setSettingsVisible(false) })
	b.settingsControls = append(b.settingsControls, c)
	return b
}

func green() paint2d.Color { return paint2d.RGB(.42, .85, .08) }
func silverButton(slot, name string, glyph players.Glyph, action func()) *players.GlyphButton {
	c := players.NewGlyphButton(glyph, name, action)
	if slot == "play" || slot == "stop" || slot == "logo" {
		c.Shaper = func(_ style.LookAndFeel, size paint2d.Point) *style.Silhouette {
			path := paint2d.NewPath()
			path.AddEllipse(paint2d.Pt(size.X/2, size.Y/2), size.X/2, size.Y/2)
			return &style.Silhouette{Path: path}
		}
	}
	c.Art = func(ctx *paint2d.Context, r paint2d.Rect, st style.ControlState) bool {
		col := paint2d.RGB(.15, .17, .19)
		if slot == "eq" || slot == "pl" {
			col = paint2d.RGB(.75, .77, .79)
		}
		if st.Checked() {
			col = green()
			if slot == "settings" || slot == "mute" {
				col = paint2d.RGB(.12, .35, .02)
			}
		}
		if st.Disabled() {
			col = paint2d.RGB(.4, .42, .44)
		}
		if st.Pressed() || st.Hovered() {
			ctx.DrawRoundRect(r, style.Dip(c.Look(), 3), style.Dip(c.Look(), 3), paint2d.Fill(paint2d.RGBA(.2, .3, .18, .22)))
		}
		switch slot {
		case "close", "settings.close":
			d := min(r.Dx(), r.Dy()) * .28
			o := r.Center()
			p := paint2d.NewPath()
			p.MoveTo(o.X-d, o.Y-d)
			p.LineTo(o.X+d, o.Y+d)
			p.MoveTo(o.X+d, o.Y-d)
			p.LineTo(o.X-d, o.Y+d)
			ctx.DrawPath(p, paint2d.StrokePaint(col, style.Dip(c.Look(), 1.3)))
		case "minimize":
			ctx.DrawRect(paint2d.XYWH(r.Min.X+r.Dx()*.25, r.Min.Y+r.Dy()*.7, r.Dx()*.5, style.Dip(c.Look(), 1.3)), paint2d.Fill(col))
		case "mute":
			players.DrawTextIn(ctx, players.ScaledFace(c.Look().Font(), style.Dip(c.Look(), 10)), r, "M", col, style.AlignCenter)
		case "logo":
			d := style.Dip(c.Look(), 16)
			skins.DrawSkinSprite(c.Look(), ctx, paint2d.XYWH(r.Min.X+style.Dip(c.Look(), 5), r.Min.Y+(r.Dy()-d)/2, d, d), "app.icon", paint2d.Color{})
			players.DrawTextIn(ctx, players.ScaledFace(c.Look().Font(), style.Dip(c.Look(), 9)), paint2d.XYWH(r.Min.X+style.Dip(c.Look(), 24), r.Min.Y, r.Dx()-style.Dip(c.Look(), 29), r.Dy()), "Music Player", paint2d.RGB(.87, .89, .88), style.AlignCenter)
		default:
			players.DrawGlyph(ctx, r, c.Glyph, col, style.Dip(c.Look(), 1.3))
		}
		return true
	}
	return c
}

func (b *body) setSettingsVisible(on bool) {
	if b.settingsVisible == on {
		return
	}
	b.settingsVisible = on
	for _, c := range b.settingsControls {
		c.SetVisible(on)
	}
	b.buttons["settings"].SetChecked(on)
	if c := b.p.Main().Focus(); c != nil && !c.Visible() {
		b.p.Main().RequestFocus(b.play)
	}
	b.p.updateShape()
	b.RequestLayout()
	b.Invalidate()
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
	b.buttons["mute"].SetChecked(t.Muted())
	b.buttons["settings"].SetChecked(b.settingsVisible)
	for _, c := range b.settingsControls {
		c.SetVisible(b.settingsVisible)
	}
	b.buttons["settings.equalizer"].SetChecked(eq)
	b.buttons["settings.nowplaying"].SetChecked(!eq)
	for _, c := range b.eqControls {
		c.SetVisible(eq)
	}
	b.buttons["eq.on"].SetChecked(p.Equaliser.On)
	b.buttons["eq.auto"].SetChecked(p.Equaliser.Auto)
	for i, f := range b.bands {
		f.Set(p.Equaliser.Gains[i])
	}
	b.preamp.Set(p.Equaliser.Preamp)
	b.balance.Set(t.Pan)
	b.visual.SetVisible(!eq)
	b.visual.Invalidate()
	b.seek.SetEnabled(t.Length() > 0)
	if !b.seek.Dragging() {
		b.seek.Set(t.Fraction())
	}
	if !b.volume.Dragging() {
		b.volume.Set(t.Volume * 100)
	}
	b.Invalidate()
}
func (b *body) eqMenu() {
	e := b.p.Equaliser
	items := []*widgets.MenuItem{widgets.Item("Reset equalizer", e.Flat), widgets.Sep()}
	for _, preset := range players.EqPresets() {
		name := preset.Name
		items = append(items, widgets.RadioItem(name, "preset", name == e.Preset, func() { e.Apply(name) }))
	}
	b.menuAt(b.buttons["eq.preset"], items)
}
func (b *body) playbackMenu() {
	t := b.p.Transport
	b.menuAt(b.buttons["settings.playback"], []*widgets.MenuItem{widgets.Item("Play / pause", func() { b.p.Host.Command(players.CmdPlayPause) }), widgets.Item("Stop", func() { b.p.Host.Command(players.CmdStop) }), widgets.Sep(), widgets.CheckItem("Shuffle", t.Random, func() { b.p.Host.Command(players.CmdShuffle) }), widgets.Item("Repeat: "+t.Repeat.String(), func() { b.p.Host.Command(players.CmdRepeat) }), widgets.CheckItem("Playlist", b.p.Host.Model.PlaylistVisible, func() { b.p.ShowPlaylist(!b.p.Host.Model.PlaylistVisible) })})
}
func (b *body) aboutMenu() {
	item := widgets.Item("Music Player · reference-inspired Fandom skin", nil)
	item.Disabled = true
	b.menuAt(b.buttons["settings.about"], []*widgets.MenuItem{item})
}
func (b *body) menuAt(c widget.Component, items []*widgets.MenuItem) {
	o := widget.DeviceOrigin(c)
	widgets.ShowContextMenu(c, paint2d.Pt(o.X, o.Y+c.Bounds().Dy()), items...)
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
	// A shaped toolkit window still starts with its look's background. Erase
	// that fill under the compositor's current damage clip, then paint the
	// translucent artwork once. Clear() would also erase outside that clip.
	ctx.DrawRect(box, paint2d.Paint{Color: paint2d.White, Blend: paint2d.BlendDestOut})
	at := func(slot string) paint2d.Rect { r, _ := skins.SkinSlotRect(lk, LayoutMain, slot, box); return r }
	for _, slot := range []string{"shape.main", "shape.settings"} {
		if slot == "shape.settings" && !b.settingsVisible {
			continue
		}
		skins.DrawSkinSlot(lk, ctx, at(slot), LayoutMain, slot, 0)
	}
	f := players.ScaledFace(lk.Font(), style.Dip(lk, 10))
	players.DrawTextIn(ctx, f, at("header"), "Music Player", paint2d.RGB(.86, .89, .93), style.AlignCenter)
	if b.settingsVisible {
		players.DrawTextIn(ctx, f, at("settings.title"), "Settings", paint2d.RGB(.86, .89, .93), style.AlignStart)
	}
	r := at("metadata")
	title := b.p.Transport.Track().Label()
	if title == "" {
		title = "Open or drop music files"
	}
	if b.p.Host.Model.Error != "" {
		title = b.p.Host.StatusText()
	}
	players.DrawTextIn(ctx, f, paint2d.XYWH(r.Min.X, r.Min.Y, r.Dx(), r.Dy()/2), title, paint2d.RGB(.85, .88, .85), style.AlignStart)
	players.DrawTextIn(ctx, players.ScaledFace(lk.Font(), style.Dip(lk, 9)), paint2d.XYWH(r.Min.X, r.Min.Y+r.Dy()/2, r.Dx(), r.Dy()/2), b.p.Transport.Track().Album, green(), style.AlignStart)
	status := "status"
	if b.p.Host.Model.EqualizerVisible {
		status = "status.eq"
	}
	players.DrawTextIn(ctx, f, at(status), b.p.Transport.State.String()+"  "+players.Clock(b.p.Transport.Pos)+" / "+players.Clock(b.p.Transport.Length()), green(), style.AlignStart)
	if b.p.Host.Model.EqualizerVisible {
		players.DrawTextIn(ctx, players.ScaledFace(lk.Font(), style.Dip(lk, 9)), at("eq.balance.label"), "Balance", green(), style.AlignStart)
		for i, slot := range append([]string{"eq.preamp"}, bandSlots()...) {
			r := at(slot)
			label := "Pre"
			if i > 0 {
				label = players.EqBands[i-1]
			}
			players.DrawTextIn(ctx, players.ScaledFace(lk.Font(), style.Dip(lk, 8)), paint2d.XYWH(r.Min.X, r.Max.Y+style.Dip(lk, 1), r.Dx(), style.Dip(lk, 10)), label, green(), style.AlignCenter)
		}
	}
}
func bandSlots() []string {
	out := make([]string, len(players.EqBands))
	for i := range out {
		out[i] = fmt.Sprintf("eq.%d", i)
	}
	return out
}
func (b *body) MousePress(e widget.MouseEvent) bool {
	if e.Button == platform.ButtonRight {
		b.p.Host.ContextMenu(b, e.Pos.Add(widget.DeviceOrigin(b)))
		return true
	}
	if e.Button == platform.ButtonLeft {
		b.p.g.StartMove(b.p.iMain)
		return true
	}
	return false
}
func (b *body) KeyPress(e widget.KeyEvent) bool { return b.p.keys(b.p.Main(), e) }
func (b *body) Describe(n *a11y.Node)           { n.Role, n.Name = a11y.RoleGroup, "Fandom music player" }
func (b *body) DropTypes() []string             { return []string{"text/uri-list"} }
func (b *body) DropActionFor(offered platform.DragAction) platform.DragAction {
	return offered & platform.DragCopy
}
func (b *body) Drop(e widget.DropEvent) bool { return b.p.Host.AcceptFileDrop(e) }
