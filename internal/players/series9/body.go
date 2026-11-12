package series9

import (
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
	p         *Player
	main      *mainPane
	transport *transportPane
	eq        *eqPane
	playlist  *playlistPane
	play      *players.GlyphButton
}

func newBody(p *Player) *body {
	b := &body{p: p}
	b.Init(b)
	b.SetManagesChildren(true)
	b.SetAccessibleName("Series 9 Music Player")
	b.main = newMainPane(p)
	b.eq = newEqPane(p)
	b.playlist = newPlaylistPane(p)
	b.transport = newTransportPane(p, LayoutTransport)
	b.play = b.transport.play
	// The original transport overlaps the upper chassis by five pixels.
	for _, c := range []widget.Component{b.playlist, b.eq, b.main, b.transport} {
		b.Add(c)
	}
	return b
}
func (b *body) sync() {
	b.eq.SetVisible(b.p.Host.Model.EqualizerVisible)
	b.playlist.SetVisible(b.p.Host.Model.PlaylistVisible)
	b.main.sync()
	b.transport.sync()
	b.eq.sync()
	b.playlist.sync()
	b.Invalidate()
}
func (b *body) Measure(c layout.Constraints) paint2d.Point {
	w, h := float32(346), float32(344)
	if b.p.Host.Model.PlaylistVisible {
		w += 250
	}
	if b.p.Host.Model.EqualizerVisible {
		h += 124
	}
	return c.Constrain(paint2d.Pt(style.Dip(b.Look(), w), style.Dip(b.Look(), h)))
}
func (b *body) Arrange(r paint2d.Rect) {
	b.SetBounds(r)
	g := b.p.panelRects(b.LocalBounds())
	b.main.Arrange(g.Main)
	b.transport.Arrange(g.Transport)
	b.eq.Arrange(g.Equalizer)
	b.playlist.Arrange(g.Playlist)
}
func eraseFrame(ctx *paint2d.Context, box paint2d.Rect, p *Player) {
	if p.Main.ShapeActive() {
		ctx.DrawRect(box, paint2d.Paint{Color: paint2d.White, Blend: paint2d.BlendDestOut})
	} else {
		ctx.DrawRect(box, paint2d.Fill(paint2d.RGB(.65, .7, .8)))
	}
}
func (b *body) Paint(ctx *paint2d.Context) { eraseFrame(ctx, b.LocalBounds(), b.p) }
func (b *body) MousePress(e widget.MouseEvent) bool {
	if e.Button == platform.ButtonRight {
		b.contextMenu(b, e.Pos.Add(widget.DeviceOrigin(b)))
		return true
	}
	if e.Button == platform.ButtonLeft {
		b.p.userMoved("main")
		b.p.Main.StartMove()
		return true
	}
	return false
}
func (b *body) KeyPress(e widget.KeyEvent) bool { return b.p.keys(b.p.Main, e) }
func (b *body) Describe(n *a11y.Node)           { n.Role, n.Name = a11y.RoleGroup, "Series 9 Music Player" }
func (b *body) DropTypes() []string             { return []string{"text/uri-list"} }
func (b *body) DropActionFor(offered platform.DragAction) platform.DragAction {
	return offered & platform.DragCopy
}
func (b *body) Drop(e widget.DropEvent) bool { return b.p.Host.AcceptFileDrop(e) }
func (b *body) contextMenu(from widget.Component, at paint2d.Point) {
	p, h, t := b.p, b.p.Host, b.p.Transport
	remove := widgets.Item("Remove current track", h.RemoveTrack)
	remove.Disabled = t.List.Len() == 0
	items := []*widgets.MenuItem{widgets.Item("Open music files…", func() { h.OpenFiles(from, false) }), widgets.Item("Add music files…", func() { h.OpenFiles(from, true) }), remove, widgets.Sep(), widgets.Item("Play / pause", func() { h.Command(players.CmdPlayPause) }), widgets.Item("Stop", func() { h.Command(players.CmdStop) }), widgets.CheckItem("Shuffle", t.Random, func() { h.Command(players.CmdShuffle) }), widgets.Item("Repeat: "+t.Repeat.String(), func() { h.Command(players.CmdRepeat) }), widgets.Sep(), widgets.CheckItem("Compact mode", p.Compact(), func() { p.SetCompact(!p.Compact()) }), widgets.CheckItem("Playlist", h.Model.PlaylistVisible, func() { p.ShowPlaylist(!h.Model.PlaylistVisible) }), widgets.CheckItem("Equalizer", h.Model.EqualizerVisible, func() { p.ShowEqualiser(!h.Model.EqualizerVisible) })}
	if appearances := h.SkinItems(); len(appearances) > 0 {
		items = append(items, widgets.Sep(), widgets.Submenu("Appearance", appearances...))
	}
	if library := h.LibraryItem(); library != nil {
		items = append(items, library)
	}
	if h.ShowSettings != nil {
		items = append(items, widgets.ItemAccel("Settings…", "Ctrl+,", h.OpenSettings))
	}
	widgets.ShowContextMenu(from, at, items...)
}

type mainPane struct {
	*pane
	analyser *players.AnalyserView
	visualOn bool
}

func newMainPane(p *Player) *mainPane {
	b := &mainPane{visualOn: true}
	b.pane = newPane(p, LayoutMain, "Series 9 display", b)
	b.button("open", "Open music files", players.GlyphEject, func() { p.Host.OpenFiles(b.buttons["open"], false) })
	b.button("pl", "Playlist", players.GlyphList, func() { p.ShowPlaylist(!p.Host.Model.PlaylistVisible) }).Toggle = true
	b.button("eq", "Equalizer", players.GlyphSliders, func() { p.ShowEqualiser(!p.Host.Model.EqualizerVisible) }).Toggle = true
	b.button("vis", "Show or hide spectrum", players.GlyphNone, func() { b.visualOn = !b.visualOn; b.sync(); b.RequestLayout() }).Toggle = true
	b.button("minimize", "Minimize Music Player", players.GlyphNone, p.Main.Minimize)
	b.button("close", "Close to tray", players.GlyphNone, p.Main.RequestClose)
	b.analyser = players.NewAnalyserView(p.Spectrum)
	b.analyser.Tint = paint2d.RGB(.38, .84, .2)
	b.analyser.Peaks = true
	b.analyser.SetTransparent(true)
	b.slots.Bind("video", b.analyser)
	b.Add(b.analyser)
	return b
}
func (b *mainPane) sync() {
	b.buttons["pl"].SetChecked(b.p.Host.Model.PlaylistVisible)
	b.buttons["eq"].SetChecked(b.p.Host.Model.EqualizerVisible)
	b.buttons["vis"].SetChecked(b.visualOn)
	b.analyser.SetVisible(b.visualOn)
	b.analyser.Invalidate()
	b.Invalidate()
}
func (b *mainPane) Paint(ctx *paint2d.Context) {
	b.pane.Paint(ctx)
	lk, box := b.Look(), b.LocalBounds()
	at := func(slot string) paint2d.Rect { r, _ := skins.SkinSlotRect(lk, LayoutMain, slot, box); return r }
	title := at("title")
	players.DrawTextIn(ctx, players.ScaledFace(lk.Font(), style.Dip(lk, 9)), title, "Music Player", paint2d.White, style.AlignStart)
	if !b.analyser.Visible() {
		r := at("video")
		s := style.Dip(lk, 48)
		logo := paint2d.XYWH((r.Min.X+r.Max.X-s)/2, (r.Min.Y+r.Max.Y-s)/2, s, s)
		skins.DrawSkinSprite(lk, ctx, logo, "app.logo", paint2d.Color{})
	}
	drawStatus(ctx, lk, at("metadata"), at("time"), b.p)
}
func drawStatus(ctx *paint2d.Context, lk style.LookAndFeel, label, clock paint2d.Rect, p *Player) {
	text := p.Transport.Track().Label()
	if p.Transport.State == players.Stopped && p.Host.Model.Error == "" {
		text = "Ready"
	}
	if p.Host.Model.Error != "" {
		text = p.Host.StatusText()
	}
	f := players.ScaledFace(lk.BoldFont(), style.Dip(lk, 10))
	ink := paint2d.RGB(.52, .95, .35)
	players.DrawScrollingText(ctx, lk, f, label, text, ink, p.Transport.Pos)
	players.DrawTextIn(ctx, f, clock, players.Clock(p.Transport.Pos), ink, style.AlignEnd)
}
