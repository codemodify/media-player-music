package realplayer

import (
	"fmt"
	"github.com/codemodify/media-player-music/internal/players"
	"github.com/codemodify/paintengine2d"
	"github.com/codemodify/uitoolkit/a11y"
	"github.com/codemodify/uitoolkit/layout"
	"github.com/codemodify/uitoolkit/platform"
	"github.com/codemodify/uitoolkit/style"
	"github.com/codemodify/uitoolkit/widget"
	"github.com/codemodify/uitoolkit/widgets"
)

type body struct {
	widget.Base
	p            *Player
	slots        *widget.Slots
	buttons      map[string]*players.GlyphButton
	queue        *queuePane
	eq           *eqPane
	canvas       *canvasPane
	seek, volume *classicFader
	visualOn     bool
}

func newBody(p *Player) *body {
	b := &body{p: p, slots: widget.NewSlots(LayoutMain), buttons: make(map[string]*players.GlyphButton), visualOn: true}
	b.Init(b)
	b.SetManagesChildren(true)
	b.SetAccessibleName("RealPlayer Music Player")
	button := func(slot, name string, g players.Glyph, run func()) *players.GlyphButton {
		c := bindButton(b, b.slots, LayoutMain, slot, name, g, run)
		b.buttons[slot] = c
		return c
	}
	button("play", "Play", players.GlyphPlay, p.Host.Play)
	button("pause", "Pause or resume", players.GlyphPause, func() {
		if p.Transport.State != players.Stopped {
			p.Host.Command(players.CmdPlayPause)
		}
	}).Toggle = true
	button("stop", "Stop", players.GlyphStop, func() { p.Host.Command(players.CmdStop) })
	button("prev", "Previous track", players.GlyphPrev, func() { p.Host.Command(players.CmdPrev) })
	button("next", "Next track", players.GlyphNext, func() { p.Host.Command(players.CmdNext) })
	button("open", "Open music files", players.GlyphEject, func() { p.Host.OpenFiles(b.buttons["open"], false) })
	button("add", "Add music files", players.GlyphEject, func() { p.Host.OpenFiles(b.buttons["add"], true) })
	button("mute", "Mute", players.GlyphNone, func() { p.Host.Command(players.CmdMute) }).Toggle = true
	button("settings", "Backend settings", players.GlyphNone, p.Host.OpenSettings)
	button("visual", "Spectrum analyser", players.GlyphNone, func() { b.visualOn = !b.visualOn; b.sync() }).Toggle = true
	button("logo", "Appearances", players.GlyphNone, func() { b.appearances(b.buttons["logo"]) })
	button("close", "Close Music Player", players.GlyphNone, p.Main.RequestClose)
	button("minimize", "Minimize Music Player", players.GlyphNone, func() { p.Main.Minimize() })
	button("maximize", "Maximize or restore Music Player", players.GlyphNone, func() { p.Main.SetMaximized(!p.Main.WindowState().Maximized) })
	for _, item := range []struct{ slot, text string }{{"file", "File"}, {"view", "View"}, {"play.menu", "Play"}, {"tools", "Tools"}, {"help", "Help"}} {
		item := item
		b.buttons[item.slot] = textButton(b, b.slots, LayoutMain, item.slot, item.text+" menu", item.text, func() { b.menu(item.slot, b.buttons[item.slot]) })
	}
	for _, item := range []struct {
		slot, text string
		run        func()
	}{{"queue", "Playlist", func() { p.ShowPlaylist(!p.Host.Model.PlaylistVisible) }}, {"eq", "Equalizer", func() { p.ShowEqualiser(!p.Host.Model.EqualizerVisible) }}, {"backends", "Settings", p.Host.OpenSettings}, {"skins", "Skins", func() { b.appearances(b.buttons["skins"]) }}} {
		c := textButton(b, b.slots, LayoutMain, item.slot, item.text, item.text, item.run)
		b.buttons[item.slot] = c
		if item.slot == "queue" || item.slot == "eq" {
			c.Toggle = true
		}
	}
	b.seek = classicSlider(false, 0, 1, 0, "Seek", p.Host.Seek)
	b.seek.Format = func(v float32) string { return players.Clock(timeFraction(p.Transport.Length(), v)) }
	b.volume = classicSlider(false, 0, 1, p.Transport.Volume, "Volume", p.Host.SetVolume)
	b.volume.Format = players.Percent
	for _, item := range []struct {
		slot string
		c    widget.Component
	}{{"seek", b.seek}, {"volume", b.volume}} {
		b.slots.Bind(item.slot, item.c)
		b.Add(item.c)
	}
	b.queue = newQueuePane(p)
	b.eq = newEqPane(p)
	b.canvas = newCanvasPane(p)
	for _, item := range []struct {
		slot string
		c    widget.Component
	}{{"playlist", b.queue}, {"equalizer", b.eq}, {"canvas", b.canvas}, {"resize", newResizeGrip(p)}} {
		b.slots.Bind(item.slot, item.c)
		b.Add(item.c)
	}
	return b
}
func (b *body) sync() {
	t := b.p.Transport
	b.buttons["pause"].SetChecked(t.State == players.Paused)
	b.buttons["mute"].SetChecked(t.Muted())
	b.buttons["pause"].SetEnabled(t.State != players.Stopped)
	b.buttons["queue"].SetChecked(b.p.Host.Model.PlaylistVisible)
	b.buttons["eq"].SetChecked(b.p.Host.Model.EqualizerVisible)
	b.buttons["visual"].SetEnabled(b.p.Host.SpectrumAvailable())
	b.buttons["visual"].SetChecked(b.visualOn)
	b.buttons["visual"].Tip = b.p.Host.FeatureTip("Visualizer", b.p.Host.SpectrumAvailable())
	b.seek.SetEnabled(t.Length() > 0)
	if !b.seek.Dragging() {
		b.seek.Set(t.Fraction())
	}
	if !b.volume.Dragging() {
		b.volume.Set(t.Volume)
	}
	b.queue.SetVisible(b.p.Host.Model.PlaylistVisible)
	b.eq.SetVisible(b.p.Host.Model.EqualizerVisible)
	b.queue.sync()
	b.eq.sync()
	b.canvas.sync()
	b.Invalidate()
}
func (b *body) Measure(c layout.Constraints) paintengine2d.Point {
	sz, _ := b.slots.Measure(b.Look(), c)
	return sz
}
func (b *body) Arrange(r paintengine2d.Rect) {
	b.SetBounds(r)
	b.slots.Arrange(b.Look(), b.LocalBounds())
	canvas, _ := style.SkinSlotRect(b.Look(), LayoutMain, "canvas", b.LocalBounds())
	eq, _ := style.SkinSlotRect(b.Look(), LayoutMain, "equalizer", b.LocalBounds())
	queue, _ := style.SkinSlotRect(b.Look(), LayoutMain, "playlist", b.LocalBounds())
	if !b.queue.Visible() {
		canvas.Min.X = queue.Min.X
		eq.Min.X = queue.Min.X
	}
	if b.eq.Visible() {
		canvas.Max.Y = eq.Min.Y - style.Dip(b.Look(), 4)
	}
	b.canvas.Arrange(canvas)
	b.eq.Arrange(eq)
}
func (b *body) Paint(ctx *paintengine2d.Context) {
	lk, box := b.Look(), b.LocalBounds()
	style.DrawSkinLayout(lk, ctx, box, LayoutMain)
	at := func(slot string) paintengine2d.Rect { r, _ := style.SkinSlotRect(lk, LayoutMain, slot, box); return r }
	f := players.ScaledFace(lk.Font(), style.Dip(lk, 11))
	players.DrawTextIn(ctx, players.ScaledFace(lk.BoldFont(), style.Dip(lk, 12)), at("title"), "Music Player", paintengine2d.White, style.AlignStart)
	players.DrawTextIn(ctx, f, at("location.label"), "Location:", paintengine2d.Black, style.AlignStart)
	r := at("location")
	ctx.DrawRect(r, paintengine2d.Fill(paintengine2d.White))
	drawBevel(ctx, r, false)
	track := b.p.Transport.Track()
	location := track.Path
	if location == "" {
		location = track.Label()
	}
	if b.p.Transport.List.Len() == 0 {
		location = "Open a music file"
	}
	players.DrawTextIn(ctx, f, r.Inset(style.Dip(lk, 3)), location, paintengine2d.Black, style.AlignStart)
	status := b.p.Host.StatusText()
	if b.p.Host.Model.Error == "" && b.p.Transport.List.Len() > 0 {
		status = b.p.Transport.State.String() + " · " + status
	}
	players.DrawTextIn(ctx, players.ScaledFace(lk.Font(), style.Dip(lk, 10)), at("status"), status, ink(lk, "info", paintengine2d.RGB(0, 1, 0)), style.AlignStart)
	length := "--:--"
	if b.p.Transport.Length() > 0 {
		length = players.Clock(b.p.Transport.Length())
	}
	bitrate := "-- kbps"
	if track.Bitrate > 0 {
		bitrate = fmt.Sprintf("%d kbps", track.Bitrate/1000)
	}
	players.DrawTextIn(ctx, f, at("clock"), players.Clock(b.p.Transport.Pos)+" / "+length+"  "+bitrate, ink(lk, "info", paintengine2d.RGB(0, 1, 0)), style.AlignEnd)
}
func (b *body) appearances(from widget.Component) {
	b.p.Host.ContextMenu(from, widget.DeviceOrigin(from).Add(paintengine2d.Pt(0, from.Bounds().Dy())))
}
func (b *body) menu(kind string, from widget.Component) {
	h, t, p := b.p.Host, b.p.Transport, b.p
	var items []*widgets.MenuItem
	switch kind {
	case "file":
		items = []*widgets.MenuItem{widgets.Item("Open music files…", func() { h.OpenFiles(from, false) }), widgets.Item("Add music files…", func() { h.OpenFiles(from, true) }), widgets.Item("Remove current track", h.RemoveTrack), widgets.Sep(), widgets.Item("Quit", p.App.Quit)}
	case "view":
		items = []*widgets.MenuItem{widgets.CheckItem("Playlist", h.Model.PlaylistVisible, func() { p.ShowPlaylist(!h.Model.PlaylistVisible) }), widgets.CheckItem("Equalizer", h.Model.EqualizerVisible, func() { p.ShowEqualiser(!h.Model.EqualizerVisible) }), widgets.CheckItem("Spectrum analyser", b.visualOn, func() { b.visualOn = !b.visualOn; b.sync() }), widgets.Submenu("Appearance", h.SkinItems()...)}
	case "play.menu":
		items = []*widgets.MenuItem{widgets.Item("Play", h.Play), widgets.Item("Pause or resume", func() {
			if t.State != players.Stopped {
				h.Command(players.CmdPlayPause)
			}
		}), widgets.Item("Stop", func() { h.Command(players.CmdStop) }), widgets.Item("Previous track", func() { h.Command(players.CmdPrev) }), widgets.Item("Next track", func() { h.Command(players.CmdNext) }), widgets.Sep(), widgets.CheckItem("Shuffle", t.Random, func() { h.Command(players.CmdShuffle) }), widgets.Item("Repeat: "+t.Repeat.String(), func() { h.Command(players.CmdRepeat) })}
	case "tools":
		items = []*widgets.MenuItem{widgets.Item("Backend settings…", h.OpenSettings), widgets.Submenu("Appearance", h.SkinItems()...), widgets.Item("Equalizer presets…", b.eq.presetMenu)}
	case "help":
		info := widgets.Item("Music Player · classic RealPlayer 8 appearance", nil)
		info.Disabled = true
		items = []*widgets.MenuItem{info, widgets.Item("Backend settings…", h.OpenSettings)}
	}
	widgets.ShowContextMenu(from, widget.DeviceOrigin(from).Add(paintengine2d.Pt(0, from.Bounds().Dy())), items...)
}
func (b *body) MousePress(e widget.MouseEvent) bool {
	if e.Button == platform.ButtonRight {
		b.appearances(b)
		return true
	}
	if e.Button == platform.ButtonLeft && e.Pos.Y < style.Dip(b.Look(), 23) {
		b.p.restore = nil
		return b.p.Main.StartMove()
	}
	return false
}
func (b *body) KeyPress(e widget.KeyEvent) bool { return b.p.keys(e) }
func (b *body) Describe(n *a11y.Node)           { n.Role, n.Name = a11y.RoleGroup, "RealPlayer Music Player" }
func (b *body) DropTypes() []string             { return []string{"text/uri-list"} }
func (b *body) DropActionFor(offered platform.DragAction) platform.DragAction {
	return offered & platform.DragCopy
}
func (b *body) Drop(e widget.DropEvent) bool { return b.p.Host.AcceptFileDrop(e) }
