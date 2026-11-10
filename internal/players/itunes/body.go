package itunes

import (
	"fmt"
	"time"

	"github.com/codemodify/media-player-music/internal/players"
	uiskins "goframework.io/desktop-ui-skins"
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
	slots        *uiskins.Slots
	buttons      map[string]*players.GlyphButton
	play         *players.GlyphButton
	seek, volume *sourceSlider
	search       *widgets.TextField
	searchLook   style.LookAndFeel
	list         *playlistList
	headers      [4]*players.GlyphButton
	sidebar      []widget.Component
	analyser     *players.AnalyserView
	visualOn     bool
}

func newBody(p *Player) *body {
	b := &body{p: p, slots: uiskins.NewSlots(LayoutMain), buttons: make(map[string]*players.GlyphButton), visualOn: true}
	b.Init(b)
	b.SetManagesChildren(true)
	b.SetAccessibleName("iTunes music library")
	button := func(slot, label string, glyph players.Glyph, action func()) *players.GlyphButton {
		c := skinButton(LayoutMain, slot, label, glyph, action)
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
	button("prev", "Previous track", players.GlyphPrev, func() { p.Host.Command(players.CmdPrev) })
	button("next", "Next track", players.GlyphNext, func() { p.Host.Command(players.CmdNext) })
	button("close", "Close to tray", players.GlyphNone, p.Main.RequestClose)
	button("minimize", "Minimize Music Player", players.GlyphNone, p.Main.Minimize)
	button("mini", "Switch to MiniPlayer", players.GlyphNone, func() { p.SetMiniPlayer(true) })
	button("open", "Add music files", players.GlyphEject, func() { p.Host.OpenFiles(b.buttons["open"], true) })
	button("shuffle", "Shuffle", players.GlyphShuffle, func() { p.Host.Command(players.CmdShuffle) }).Toggle = true
	button("repeat", "Repeat", players.GlyphRepeat, func() { p.Host.Command(players.CmdRepeat) }).Toggle = true
	button("eq", "Equalizer", players.GlyphSliders, func() { p.ShowEqualiser(!p.Host.Model.EqualizerVisible) }).Toggle = true
	button("visual", "Show or hide spectrum", players.GlyphNone, func() { b.visualOn = !b.visualOn; b.sync(); b.RequestLayout() }).Toggle = true
	button("settings", "Appearance and settings", players.GlyphSkin, func() {
		c := b.buttons["settings"]
		o := widget.DeviceOrigin(c)
		p.Host.ContextMenu(c, paint2d.Pt(o.X, o.Y+c.Bounds().Dy()))
	})
	button("mute", "Mute", players.GlyphNone, func() { p.Host.Command(players.CmdMute) }).Toggle = true
	b.volume = newSourceSlider(false, 0, 100, p.Transport.Volume*100, "Volume", func(v float32) { p.Host.SetVolume(v / 100) }, "volume")
	b.volume.Step = 2
	b.volume.Page = 10
	b.volume.Format = func(v float32) string { return fmt.Sprintf("%d%%", int(v+.5)) }
	b.seek = newSourceSlider(false, 0, 1, 0, "Seek", p.Host.Seek, "seek")
	b.seek.Step = .02
	b.seek.Page = .1
	b.seek.Format = func(v float32) string {
		return players.Clock(time.Duration(float64(p.Transport.Length()) * float64(v)))
	}
	for _, item := range []struct {
		slot string
		c    widget.Component
	}{{"volume", b.volume}, {"seek", b.seek}} {
		b.slots.Bind(item.slot, item.c)
		b.Add(item.c)
	}
	b.analyser = players.NewAnalyserView(p.Spectrum)
	b.analyser.Tint = lcdInk()
	b.analyser.Peaks = false
	b.analyser.SetTransparent(true)
	b.slots.Bind("analyser", b.analyser)
	b.Add(b.analyser)
	b.list = newPlaylistList(p)
	b.slots.Bind("list", b.list)
	b.Add(b.list)
	b.search = widgets.NewTextField("", "Search", nil)
	b.search.Clearable = true
	b.search.PreferredWidth = 174
	b.search.SetAccessibleName("Search music library")
	b.search.OnInput = b.list.applyFilter
	b.search.OnSubmit = func(string) {
		if b.list.Count > 0 {
			p.Main.RequestFocus(b.list)
		}
	}
	b.search.OnEscape = func() { b.search.SetText(""); b.list.applyFilter(""); p.Main.RequestFocus(b.list) }
	b.slots.Bind("search", b.search)
	b.Add(b.search)
	for i, name := range []string{"Name", "Time", "Artist", "Album"} {
		column := i
		c := players.NewGlyphButton(players.GlyphNone, "Sort by "+name, func() { b.list.sortBy(column); b.Invalidate() })
		label := name
		c.Art = func(ctx *paint2d.Context, r paint2d.Rect, st style.ControlState) bool {
			return paintHeader(ctx, c.Look(), r, label, column == b.list.sortColumn, b.list.descending, st)
		}
		b.headers[i] = c
		b.Add(c)
	}
	sidebarButton := func(slot, label string, action func()) *players.GlyphButton {
		c := button(slot, label, players.GlyphNone, action)
		c.Shaper = nil
		c.Art = func(ctx *paint2d.Context, r paint2d.Rect, st style.ControlState) bool {
			if st.Checked() || st.Hovered() {
				ctx.DrawRect(r, paint2d.Fill(paint2d.RGB(.62, .72, .8)))
			}
			ink := paint2d.RGB(.1, .12, .14)
			if st.Disabled() {
				ink = paint2d.RGB(.52, .55, .58)
			}
			pad := style.Dip(c.Look(), 4)
			text := r
			text.Min.X += pad
			if slot == "library" {
				icon := paint2d.XYWH(r.Min.X+pad, r.Min.Y+style.Dip(c.Look(), 2), style.Dip(c.Look(), 21), style.Dip(c.Look(), 21))
				uiskins.DrawSkinSprite(c.Look(), ctx, icon, "library.icon", paint2d.Color{})
				text.Min.X += style.Dip(c.Look(), 27)
			}
			players.DrawTextIn(ctx, players.ScaledFace(c.Look().Font(), style.Dip(c.Look(), 11)), text, label, ink, style.AlignStart)
			return true
		}
		b.sidebar = append(b.sidebar, c)
		return c
	}
	sidebarButton("library", "Library", func() { p.ShowPlaylist(true); b.search.SetText(""); b.list.applyFilter("") }).Toggle = true
	sidebarButton("nowplaying", "Now Playing", func() {
		p.ShowPlaylist(true)
		i := b.list.viewIndex(p.Transport.List.Index())
		if i >= 0 {
			b.list.choose(i)
			p.Main.RequestFocus(b.list)
		}
	})
	sidebarButton("addfiles", "Add music files…", func() { p.Host.OpenFiles(b.buttons["addfiles"], true) })
	sidebarButton("backends", "Audio backend…", p.Host.OpenSettings).SetEnabled(p.Host.ShowSettings != nil)
	sidebarButton("skins", "Appearance…", func() {
		c := b.buttons["skins"]
		o := widget.DeviceOrigin(c)
		p.Host.ContextMenu(c, paint2d.Pt(o.X, o.Y+c.Bounds().Dy()))
	})
	sidebarButton("store", "Music Store", nil).SetEnabled(false)
	b.buttons["store"].Tip = "The original Music Store service is not provided by this music player"
	grip := newResizeGrip(p, p.Main, platform.EdgeBottom|platform.EdgeRight)
	b.slots.Bind("resize", grip)
	b.Add(grip)
	return b
}

func lcdInk() paint2d.Color { return paint2d.RGB(.16, .2, .1) }
func paintHeader(ctx *paint2d.Context, lk style.LookAndFeel, r paint2d.Rect, label string, sorted, descending bool, st style.ControlState) bool {
	ctx.DrawRect(r, paint2d.Fill(paint2d.RGB(.85, .89, .9)))
	if sorted {
		ctx.DrawRect(r, paint2d.Fill(paint2d.RGB(.65, .77, .86)))
	}
	ctx.DrawRect(paint2d.XYWH(r.Min.X, r.Min.Y, r.Dx(), 1), paint2d.Fill(paint2d.White))
	ctx.DrawRect(paint2d.XYWH(r.Max.X-1, r.Min.Y, 1, r.Dy()), paint2d.Fill(paint2d.RGB(.56, .59, .61)))
	if sorted {
		d := style.Dip(lk, 1)
		x, y := r.Max.X-8*d, (r.Min.Y+r.Max.Y)/2
		var arrow paint2d.Path
		if descending {
			arrow.MoveTo(x-3*d, y-2*d)
			arrow.LineTo(x+3*d, y-2*d)
			arrow.LineTo(x, y+2*d)
		} else {
			arrow.MoveTo(x-3*d, y+2*d)
			arrow.LineTo(x+3*d, y+2*d)
			arrow.LineTo(x, y-2*d)
		}
		arrow.Close()
		ctx.DrawPath(&arrow, paint2d.Fill(paint2d.RGB(.2, .3, .4)))
		r.Max.X -= 14 * d
	}
	players.DrawTextIn(ctx, players.ScaledFace(lk.Font(), style.Dip(lk, 11)), r.Inset(style.Dip(lk, 4)), label, paint2d.RGB(.1, .12, .14), style.AlignStart)
	return true
}
func (b *body) sync() {
	if lk := b.Look(); lk != nil && lk != b.searchLook {
		b.searchLook = lk
		b.search.SetLook(&fieldLook{LookAndFeel: lk})
	}
	t := b.p.Transport
	if t.State == players.Playing {
		b.play.SetGlyph(players.GlyphPause, "Pause")
	} else {
		b.play.SetGlyph(players.GlyphPlay, "Play")
	}
	b.buttons["shuffle"].SetChecked(t.Random)
	b.buttons["repeat"].SetChecked(t.Repeat != players.RepeatOff)
	b.buttons["repeat"].Tip = "Repeat: " + t.Repeat.String()
	b.buttons["eq"].SetChecked(b.p.Host.Model.EqualizerVisible)
	b.buttons["mute"].SetChecked(t.Muted())
	b.buttons["visual"].SetChecked(b.visualOn)
	b.list.syncQueue()
	on := b.p.Host.Model.PlaylistVisible
	b.list.SetVisible(on)
	for _, h := range b.headers {
		h.SetVisible(on)
	}
	b.buttons["library"].SetChecked(on)
	if !b.seek.Dragging() {
		b.seek.Set(t.Fraction())
	}
	b.seek.SetEnabled(t.Length() > 0)
	if !b.volume.Dragging() {
		b.volume.Set(t.Volume * 100)
	}
	b.analyser.SetVisible(b.visualOn && b.p.Host.SpectrumAvailable())
	b.analyser.SetAccessibleDescription(b.p.Host.FeatureTip("Visualizer", b.p.Host.SpectrumAvailable()))
	b.analyser.Invalidate()
	b.list.Invalidate()
	b.Invalidate()
}
func (b *body) Measure(c layout.Constraints) paint2d.Point {
	sz, _ := b.slots.Measure(b.Look(), c)
	return sz
}
func (b *body) Arrange(r paint2d.Rect) {
	b.SetBounds(r)
	b.slots.Arrange(b.Look(), b.LocalBounds())
	header, _ := uiskins.SkinSlotRect(b.Look(), LayoutMain, "header", b.LocalBounds())
	rows, _ := uiskins.SkinSlotRect(b.Look(), LayoutMain, "rows", b.LocalBounds())
	header.Min.X, header.Max.X = rows.Min.X, rows.Max.X
	for i, cell := range columnRects(header, b.Look()) {
		b.headers[i].Arrange(cell)
	}
}
func (b *body) Paint(ctx *paint2d.Context) {
	lk, box := b.Look(), b.LocalBounds()
	ctx.DrawRect(box, paint2d.Paint{Color: paint2d.White, Blend: paint2d.BlendDestOut})
	uiskins.DrawSkinLayout(lk, ctx, box, LayoutMain)
	at := func(slot string) paint2d.Rect { r, _ := uiskins.SkinSlotRect(lk, LayoutMain, slot, box); return r }
	for _, slot := range []string{"controlwell", "lcd"} {
		uiskins.DrawSkinSlot(lk, ctx, at(slot), LayoutMain, slot, 0)
	}
	players.DrawTextIn(ctx, players.ScaledFace(lk.Font(), style.Dip(lk, 12)), at("title"), "Music Player", paint2d.RGB(.09, .1, .11), style.AlignCenter)
	side := at("sidebar")
	ctx.DrawRect(side, paint2d.Fill(paint2d.White))
	ctx.DrawRect(side, paint2d.StrokePaint(paint2d.RGB(.46, .5, .52), style.Dip(lk, 1)))
	paintHeader(ctx, lk, at("sourceheader"), "Source", false, false, 0)
	if b.p.Host.Model.PlaylistVisible {
		table := at("list")
		ctx.DrawRect(table, paint2d.Fill(paint2d.White))
		ctx.DrawRect(table, paint2d.StrokePaint(paint2d.RGB(.48, .5, .53), style.Dip(lk, 1)))
		paintHeader(ctx, lk, at("header"), "", false, false, 0)
		if b.list.Count == 0 {
			text := "Drop music files or folders here"
			if b.list.query != "" {
				text = "No matching songs"
			}
			players.DrawTextIn(ctx, players.ScaledFace(lk.Font(), style.Dip(lk, 12)), table, text, paint2d.RGB(.45, .47, .5), style.AlignCenter)
		}
	}
	if !b.analyser.Visible() {
		uiskins.DrawSkinSprite(lk, ctx, at("logo"), "app.icon", paint2d.Color{})
	}
	drawDisplay(ctx, lk, at("display"), b.p, false)
	players.DrawTextIn(ctx, players.ScaledFace(lk.Font(), style.Dip(lk, 10)), at("searchlabel"), "Search Library", paint2d.RGB(.23, .25, .27), style.AlignCenter)
	status := fmt.Sprintf("%d songs · %s · volume %d%%", b.list.Count, players.Clock(b.p.Transport.List.Total()), int(b.p.Transport.Volume*100+.5))
	if b.p.Host.Model.Error != "" {
		status = b.p.Host.StatusText()
	}
	players.DrawTextIn(ctx, players.ScaledFace(lk.Font(), style.Dip(lk, 10)), at("status"), status, paint2d.RGB(.15, .17, .18), style.AlignCenter)
}
func drawDisplay(ctx *paint2d.Context, lk style.LookAndFeel, r paint2d.Rect, p *Player, mini bool) {
	title := p.Transport.Track().Label()
	if title == "" {
		title = "Music Player"
	}
	if p.Host.Model.Error != "" {
		title = p.Host.StatusText()
	}
	f := players.ScaledFace(lk.Font(), style.Dip(lk, 10))
	h := r.Dy() / 2
	if mini {
		players.DrawTextIn(ctx, f, r, title, lcdInk(), style.AlignCenter)
		return
	}
	players.DrawTextIn(ctx, f, paint2d.XYWH(r.Min.X, r.Min.Y, r.Dx(), h), title, lcdInk(), style.AlignCenter)
	players.DrawTextIn(ctx, f, paint2d.XYWH(r.Min.X, r.Min.Y+h, r.Dx(), h), p.Transport.State.String()+" · "+players.Clock(p.Transport.Pos)+" / "+players.Clock(p.Transport.Length()), lcdInk(), style.AlignCenter)
}
func (b *body) contextMenu(from widget.Component, at paint2d.Point) {
	h, t := b.p.Host, b.p.Transport
	remove := widgets.Item("Remove current track", h.RemoveTrack)
	remove.Disabled = t.List.Len() == 0
	items := []*widgets.MenuItem{widgets.Item("Open music files…", func() { h.OpenFiles(from, false) }), widgets.Item("Add music files…", func() { h.OpenFiles(from, true) }), remove, widgets.Sep(), widgets.Item("Play / pause", func() { h.Command(players.CmdPlayPause) }), widgets.Item("Stop", func() { h.Command(players.CmdStop) }), widgets.CheckItem("Shuffle", t.Random, func() { h.Command(players.CmdShuffle) }), widgets.Item("Repeat: "+t.Repeat.String(), func() { h.Command(players.CmdRepeat) }), widgets.Sep(), widgets.CheckItem("MiniPlayer", b.p.MiniPlayer(), func() { b.p.SetMiniPlayer(!b.p.MiniPlayer()) }), widgets.CheckItem("Equalizer", h.Model.EqualizerVisible, func() { b.p.ShowEqualiser(!h.Model.EqualizerVisible) })}
	if skins := h.SkinItems(); len(skins) > 0 {
		items = append(items, widgets.Sep(), widgets.Submenu("Appearance", skins...))
	}
	if h.ShowSettings != nil {
		items = append(items, widgets.ItemAccel("Settings…", "Ctrl+,", h.OpenSettings))
	}
	widgets.ShowContextMenu(from, at, items...)
}
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
func (b *body) Describe(n *a11y.Node)           { n.Role, n.Name = a11y.RoleGroup, "iTunes music library" }
func (b *body) DropTypes() []string             { return []string{"text/uri-list"} }
func (b *body) DropActionFor(offered platform.DragAction) platform.DragAction {
	return offered & platform.DragCopy
}
func (b *body) Drop(e widget.DropEvent) bool { return b.p.Host.AcceptFileDrop(e) }
