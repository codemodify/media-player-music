package vlc

import (
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
	buttons      map[string]*widgets.Button
	menu         *widgets.MenuBar
	menuLook     *menuLook
	seek, volume *sourceSlider
	eq           *eqPane
	queue        *queuePane
	analyser     *players.AnalyserView
}

func newBody(p *Player) *body {
	b := &body{p: p, slots: widget.NewSlots(LayoutMain), buttons: map[string]*widgets.Button{}}
	b.Init(b)
	b.SetManagesChildren(true)
	b.SetAccessibleName("Classic VLC Music Player")
	b.button("open", "Open music files", "eject", func() { p.Host.OpenFiles(b.buttons["open"], false) })
	b.button("play", "Play / pause", "play", func() { p.Host.Command(players.CmdPlayPause) })
	b.button("stop", "Stop", "stop", func() { p.Host.Command(players.CmdStop) })
	b.button("prev", "Previous track", "prev", func() { p.Host.Command(players.CmdPrev) })
	b.button("next", "Next track", "next", func() { p.Host.Command(players.CmdNext) })
	b.button("pl", "Show playlist", "playlist_small", func() { p.ShowPlaylist(!p.Host.Model.PlaylistVisible) }).Toggle = true
	b.button("eq", "Show equalizer", "", func() { p.ShowEqualiser(!p.Host.Model.EqualizerVisible) }).Toggle = true
	b.button("shuffle", "Shuffle", "shuffle", func() { p.Host.Command(players.CmdShuffle) }).Toggle = true
	b.button("repeat", "Repeat", "", func() { p.Host.Command(players.CmdRepeat) }).Toggle = true
	b.button("mute", "Mute / restore volume", "speaker", func() { p.Host.Command(players.CmdMute) }).Toggle = true
	b.button("close", "Close to tray", "", p.Main.RequestClose)
	b.button("minimize", "Minimize Music Player", "", p.Main.Minimize)
	b.button("maximize", "Maximize / restore Music Player", "", p.Main.ToggleMaximize)
	b.seek = newSlider(false, 0, 1, p.Transport.Fraction(), "Seek", p.Host.Seek)
	b.seek.Format = func(float32) string { return players.Clock(p.Transport.Pos) }
	b.volume = newSlider(false, 0, 1, p.Transport.Volume, "Volume", p.Host.SetVolume)
	b.volume.Format = players.Percent
	// The original wxWidgets volume control was a small horizontal gauge,
	// rather than a second seek thumb. Keep stock slider input/a11y behavior.
	b.volume.Painter = func(ctx *paintengine2d.Context, r paintengine2d.Rect, t float32, st style.ControlState) bool {
		d := style.Dip(b.volume.Look(), 1)
		box := paintengine2d.XYWH(r.Min.X, r.Min.Y+(r.Dy()-14*d)/2, r.Dx(), 14*d)
		ctx.DrawRect(box, paintengine2d.Fill(paintengine2d.RGB(.45, .45, .45)))
		inner := box.Inset(d)
		ctx.DrawRect(inner, paintengine2d.Fill(paintengine2d.White))
		inner = inner.Inset(2 * d)
		filled := inner
		filled.Max.X = filled.Min.X + filled.Dx()*t
		ctx.DrawRect(filled, paintengine2d.Fill(paintengine2d.RGB(.35, .65, .28)))
		if st.Focused() {
			b.volume.Look().DrawFocusRing(ctx, r)
		}
		return true
	}
	b.slots.Bind("seek", b.seek)
	b.slots.Bind("volume", b.volume)
	b.Add(b.seek)
	b.Add(b.volume)
	b.eq = newEqPane(p)
	b.queue = newQueuePane(p)
	b.Add(b.eq)
	b.Add(b.queue)
	b.analyser = players.NewAnalyserView(p.Spectrum)
	b.analyser.Tint = paintengine2d.RGB(.2, .65, .16)
	b.Add(b.analyser)
	b.menu = b.newMenu()
	b.slots.Bind("menu", b.menu)
	b.Add(b.menu)
	grip := newResizeGrip(p)
	b.slots.Bind("resize", grip)
	b.Add(grip)
	return b
}

func (b *body) button(name, label, icon string, action func()) *widgets.Button {
	c := widgets.NewButton("", action)
	c.SetAccessibleName(label)
	c.Tip = label
	c.SetFocusVisibleOnly(true)
	c.Painter = func(ctx *paintengine2d.Context, r paintengine2d.Rect, st style.ControlState) bool {
		lk := c.Look()
		d := func(v float32) float32 { return style.Dip(lk, v) }
		if name == "close" || name == "minimize" || name == "maximize" {
			lk.DrawButton(ctx, r, st, "")
		} else if st.Hovered() || st.Pressed() || st.Checked() {
			lk.DrawButton(ctx, r, st, "")
		}
		mark := icon
		if name == "play" && b.p.Transport.State == players.Playing {
			mark = "pause"
		}
		if name == "mute" && b.p.Transport.Muted() {
			mark = "speaker_mute"
		}
		if mark != "" {
			size := d(16)
			rr := paintengine2d.XYWH(r.Min.X+(r.Dx()-size)/2, r.Min.Y+(r.Dy()-size)/2, size, size)
			style.DrawSkinSprite(lk, ctx, rr, "icon."+mark, paintengine2d.Color{})
			return true
		}
		g := players.GlyphNone
		if name == "eq" {
			g = players.GlyphSliders
		}
		if name == "repeat" {
			g = players.GlyphRepeat
		}
		if g != players.GlyphNone {
			players.DrawGlyph(ctx, r, g, paintengine2d.Black, d(1))
			return true
		}
		cx, cy := (r.Min.X+r.Max.X)/2, (r.Min.Y+r.Max.Y)/2
		if name == "close" {
			path := paintengine2d.NewPath()
			path.MoveTo(cx-d(3), cy-d(3))
			path.LineTo(cx+d(3), cy+d(3))
			path.MoveTo(cx+d(3), cy-d(3))
			path.LineTo(cx-d(3), cy+d(3))
			ctx.DrawPath(path, paintengine2d.StrokePaint(paintengine2d.Black, d(1.2)))
		} else if name == "maximize" {
			ctx.DrawRect(paintengine2d.XYWH(cx-d(4), cy-d(4), d(8), d(8)), paintengine2d.StrokePaint(paintengine2d.Black, d(1)))
			ctx.DrawRect(paintengine2d.XYWH(cx-d(4), cy-d(4), d(8), d(2)), paintengine2d.Fill(paintengine2d.Black))
		} else {
			ctx.DrawRect(paintengine2d.XYWH(cx-d(3), cy+d(2), d(6), d(2)), paintengine2d.Fill(paintengine2d.Black))
		}
		return true
	}
	b.buttons[name] = c
	b.slots.Bind(name, c)
	b.Add(c)
	return c
}

func (b *body) newMenu() *widgets.MenuBar {
	p, h := b.p, b.p.Host
	return widgets.NewMenuBar(
		widgets.NewMenu("File", widgets.ItemAccel("Open music files…", "Ctrl+O", func() { h.OpenFiles(b, false) }), widgets.Item("Add music files…", func() { h.OpenFiles(b, true) }), widgets.Sep(), widgets.Item("Close to tray", p.Main.RequestClose)),
		widgets.NewMenu("View", widgets.ItemAccel("Playlist", "Ctrl+L", func() { p.ShowPlaylist(!h.Model.PlaylistVisible) }), widgets.ItemAccel("Equalizer", "Ctrl+E", func() { p.ShowEqualiser(!h.Model.EqualizerVisible) })),
		widgets.NewMenu("Settings", widgets.ItemAccel("Audio backend…", "Ctrl+,", h.OpenSettings), widgets.Item("Appearance…", func() {
			h.ContextMenu(b, widget.DeviceOrigin(b).Add(paintengine2d.Pt(8*p.Main.Scale(), 48*p.Main.Scale())))
		})),
		widgets.NewMenu("Audio", widgets.Item("Play / pause", func() { h.Command(players.CmdPlayPause) }), widgets.Item("Stop", func() { h.Command(players.CmdStop) }), widgets.Item("Mute / restore", func() { h.Command(players.CmdMute) })),
		widgets.NewMenu("Help", widgets.Item("About this appearance", func() {
			widgets.ShowContextMenu(b, widget.DeviceOrigin(b).Add(paintengine2d.Pt(8*p.Main.Scale(), 48*p.Main.Scale())), infoItem("VLC 0.8.6 wxWidgets appearance"), infoItem("Original VideoLAN toolbar artwork"), infoItem("Playback backend: "+h.AudioBackend))
		})),
	)
}

func (b *body) sync() {
	p, t := b.p, b.p.Transport
	if lk := p.Main.Look(); b.menuLook == nil || b.menuLook.LookAndFeel != lk {
		b.menuLook = &menuLook{LookAndFeel: lk}
		b.menu.SetLook(b.menuLook)
	}
	b.buttons["pl"].Checked = p.Host.Model.PlaylistVisible
	b.buttons["eq"].Checked = p.Host.Model.EqualizerVisible
	b.buttons["shuffle"].Checked = t.Random
	b.buttons["repeat"].Checked = t.Repeat != players.RepeatOff
	b.buttons["mute"].Checked = t.Muted()
	for _, c := range b.buttons {
		c.Invalidate()
	}
	if !b.seek.Dragging() {
		b.seek.Set(t.Fraction())
	}
	if !b.volume.Dragging() {
		b.volume.Set(t.Volume)
	}
	b.eq.SetVisible(p.Host.Model.EqualizerVisible)
	b.queue.SetVisible(p.Host.Model.PlaylistVisible)
	b.eq.sync()
	b.queue.sync()
	b.analyser.SetVisible(!p.Host.Model.PlaylistVisible && p.Host.SpectrumAvailable())
	b.analyser.Invalidate()
	b.Invalidate()
}
func (b *body) Measure(c layout.Constraints) paintengine2d.Point {
	return c.Constrain(paintengine2d.Pt(style.Dip(b.Look(), MainW), style.Dip(b.Look(), MainH)))
}
func (b *body) Arrange(r paintengine2d.Rect) {
	b.SetBounds(r)
	box, lk := b.LocalBounds(), b.Look()
	b.slots.Arrange(lk, box)
	at := func(name string) paintengine2d.Rect { r, _ := style.SkinSlotRect(lk, LayoutMain, name, box); return r }
	eq, queue := at("equalizer"), at("playlist")
	// Keep at least one complete song row usable at the minimum size.
	// The equalizer's fader travel can shorten without hiding its controls.
	if b.eq.Visible() && b.queue.Visible() && queue.Dy() < style.Dip(lk, 54) {
		less := style.Dip(lk, 54) - queue.Dy()
		eq.Max.Y -= less
		queue.Min.Y -= less
	}
	if b.eq.Visible() {
		b.eq.Arrange(eq)
	} else {
		b.eq.Arrange(paintengine2d.Rect{})
		queue.Min.Y = eq.Min.Y
	}
	if b.queue.Visible() {
		b.queue.Arrange(queue)
	} else {
		b.queue.Arrange(paintengine2d.Rect{})
	}
	b.analyser.Arrange(paintengine2d.XYWH(queue.Min.X+style.Dip(lk, 6), queue.Min.Y+style.Dip(lk, 42), max(0, queue.Dx()-style.Dip(lk, 12)), max(0, queue.Dy()-style.Dip(lk, 50))))
}
func (b *body) Paint(ctx *paintengine2d.Context) {
	lk, box := b.Look(), b.LocalBounds()
	style.DrawSkinLayout(lk, ctx, box, LayoutMain)
	at := func(name string) paintengine2d.Rect { r, _ := style.SkinSlotRect(lk, LayoutMain, name, box); return r }
	style.DrawSkinSprite(lk, ctx, at("logo"), "app.icon", paintengine2d.Color{})
	players.DrawTextIn(ctx, players.ScaledFace(lk.Font(), style.Dip(lk, 11)), at("title"), "Music Player", paintengine2d.White, style.AlignStart)
	status := at("status")
	ctx.DrawRect(status, paintengine2d.Fill(paintengine2d.RGB(.83, .82, .79)))
	clock := players.Clock(b.p.Transport.Pos) + " / " + players.Clock(b.p.Transport.List.Current().Length)
	players.DrawTextIn(ctx, players.ScaledFace(lk.Font(), style.Dip(lk, 10)), paintengine2d.XYWH(status.Min.X+style.Dip(lk, 3), status.Min.Y, style.Dip(lk, 120), status.Dy()), clock, paintengine2d.Black, style.AlignStart)
	players.DrawTextIn(ctx, players.ScaledFace(lk.Font(), style.Dip(lk, 10)), paintengine2d.XYWH(status.Min.X+style.Dip(lk, 128), status.Min.Y, max(0, status.Dx()-style.Dip(lk, 132)), status.Dy()), b.p.Host.StatusText(), paintengine2d.Black, style.AlignStart)
	if !b.queue.Visible() {
		display := at("playlist")
		if !b.eq.Visible() {
			eq := at("equalizer")
			display.Min.Y = eq.Min.Y
		}
		text := b.p.Transport.List.Current().Label()
		if b.p.Transport.List.Len() == 0 {
			text = "Open a music file"
		}
		players.DrawTextIn(ctx, players.ScaledFace(lk.Font(), style.Dip(lk, 12)), paintengine2d.XYWH(display.Min.X, display.Min.Y, display.Dx(), style.Dip(lk, 32)), text, paintengine2d.Black, style.AlignCenter)
	}
}
func (b *body) KeyPress(e widget.KeyEvent) bool {
	if e.Mods.Ctrl() {
		switch e.Key {
		case platform.KeyE:
			b.p.ShowEqualiser(!b.p.Host.Model.EqualizerVisible)
			return true
		case platform.KeyL:
			b.p.ShowPlaylist(!b.p.Host.Model.PlaylistVisible)
			return true
		case platform.KeyK:
			if e.Mods.Shift() {
				b.p.Host.DropSkin()
			} else {
				b.p.Host.NextSkin()
			}
			return true
		}
	}
	if e.Key == platform.KeyMenu || e.Key == platform.KeyF10 && e.Mods.Shift() {
		b.contextMenu(b, widget.DeviceOrigin(b))
		return true
	}
	return b.p.Host.Keys(b.p.Main, e)
}
func (b *body) MousePress(e widget.MouseEvent) bool {
	if e.Button == platform.ButtonRight {
		b.contextMenu(b, e.Pos.Add(widget.DeviceOrigin(b)))
		return true
	}
	if e.Button == platform.ButtonLeft && e.Pos.Y < style.Dip(b.Look(), 24) {
		b.p.Main.StartMove()
		return true
	}
	return false
}
func (b *body) Describe(n *a11y.Node) {
	n.Role, n.Name, n.Description = a11y.RoleGroup, "Classic VLC Music Player", b.p.Host.StatusText()
}
func (b *body) DropTypes() []string { return []string{"text/uri-list"} }
func (b *body) DropActionFor(offered platform.DragAction) platform.DragAction {
	return offered & platform.DragCopy
}
func (b *body) Drop(e widget.DropEvent) bool { return b.p.Host.AcceptFileDrop(e) }
func (b *body) contextMenu(from widget.Component, at paintengine2d.Point) {
	h, t := b.p.Host, b.p.Transport
	items := []*widgets.MenuItem{widgets.Item("Add music files…", func() { h.OpenFiles(from, true) }), widgets.Item("Remove current track", h.RemoveTrack), widgets.Sep(), widgets.CheckItem("Shuffle", t.Random, func() { h.Command(players.CmdShuffle) }), widgets.Item("Repeat: "+t.Repeat.String(), func() { h.Command(players.CmdRepeat) }), widgets.Sep(), widgets.Item("Equalizer", func() { b.p.ShowEqualiser(!h.Model.EqualizerVisible) }), widgets.Item("Playlist", func() { b.p.ShowPlaylist(!h.Model.PlaylistVisible) }), widgets.Item("Settings…", h.OpenSettings)}
	if looks := h.SkinItems(); len(looks) > 0 {
		items = append(items, widgets.Submenu("Appearance", looks...))
	}
	widgets.ShowContextMenu(from, at, items...)
}

func infoItem(text string) *widgets.MenuItem { return &widgets.MenuItem{Text: text, Disabled: true} }

type resizeGrip struct {
	widget.Base
	p *Player
}

func newResizeGrip(p *Player) *resizeGrip {
	g := &resizeGrip{p: p}
	g.Init(g)
	g.SetAccessibleName("Resize Music Player")
	g.SetTransparent(true)
	return g
}
func (g *resizeGrip) Paint(ctx *paintengine2d.Context) {
	b, d := g.LocalBounds(), style.Dip(g.Look(), 1)
	for i := float32(3); i < 13; i += 4 {
		p := paintengine2d.NewPath()
		p.MoveTo(b.Max.X-i*d, b.Max.Y-d)
		p.LineTo(b.Max.X-d, b.Max.Y-i*d)
		ctx.DrawPath(p, paintengine2d.StrokePaint(paintengine2d.RGB(.5, .5, .5), d))
	}
}
func (g *resizeGrip) MousePress(e widget.MouseEvent) bool {
	return e.Button == platform.ButtonLeft && g.p.Main.StartResize(platform.EdgeBottom|platform.EdgeRight)
}
