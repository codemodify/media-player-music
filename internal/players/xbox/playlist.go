package xbox

import (
	"github.com/codemodify/media-player-music/internal/players"
	"goframework.io/desktop-ui-skins"
	"goframework.io/desktop-ui-widgets"
	"goframework.io/desktop-ui/a11y"
	"goframework.io/desktop-ui/layout"
	"goframework.io/desktop-ui/platform"
	"goframework.io/desktop-ui/widget"
	"goframework.io/graphics-paint2d"
)

type playlistPane struct {
	widget.Base
	p       *Player
	slots   *skins.Slots
	list    *playlistList
	buttons map[string]*players.GlyphButton
}

func newPlaylistPane(p *Player) *playlistPane {
	b := &playlistPane{p: p, slots: skins.NewSlots(LayoutPlaylist), buttons: make(map[string]*players.GlyphButton)}
	b.Init(b)
	b.SetManagesChildren(true)
	b.SetAccessibleName("Pod playlist")
	b.list = newPlaylistList(p)
	b.slots.Bind("list", b.list)
	b.Add(b.list)
	button := func(slot, name string, glyph players.Glyph, action func()) *players.GlyphButton {
		c := skinButton(LayoutPlaylist, slot, name, glyph, action, nil)
		b.buttons[slot] = c
		b.slots.Bind(slot, c)
		b.Add(c)
		return c
	}
	button("close", "Close playlist", players.GlyphNone, func() { p.ShowPlaylist(false) })
	button("shuffle", "Shuffle", players.GlyphShuffle, func() { p.Host.Command(players.CmdShuffle) }).Toggle = true
	button("repeat", "Repeat", players.GlyphRepeat, func() { p.Host.Command(players.CmdRepeat) }).Toggle = true
	// Native resizing begins on pointer down, while the desktop still has
	// the input serial. A click action fires too late on Wayland.
	grip := &resizeGrip{p: p}
	grip.Init(grip)
	grip.SetTransparent(true)
	grip.SetAccessibleName("Resize playlist")
	b.slots.Bind("resize", grip)
	b.Add(grip)
	return b
}
func (b *playlistPane) sync() {
	b.buttons["shuffle"].SetChecked(b.p.Transport.Random)
	b.buttons["repeat"].SetChecked(b.p.Transport.Repeat != players.RepeatOff)
	b.buttons["repeat"].Tip = "Repeat: " + b.p.Transport.Repeat.String()
	b.list.syncQueue()
	b.list.Invalidate()
	b.Invalidate()
}
func (b *playlistPane) Measure(c layout.Constraints) paint2d.Point {
	sz, _ := b.slots.Measure(b.Look(), c)
	return sz
}
func (b *playlistPane) Arrange(r paint2d.Rect) {
	b.SetBounds(r)
	b.slots.Arrange(b.Look(), b.LocalBounds())
}
func (b *playlistPane) Paint(ctx *paint2d.Context) {
	skins.DrawSkinLayout(b.Look(), ctx, b.LocalBounds(), LayoutPlaylist)
}
func (b *playlistPane) MousePress(e widget.MouseEvent) bool {
	if e.Button == platform.ButtonRight {
		b.contextMenu(b, e.Pos.Add(widget.DeviceOrigin(b)))
		return true
	}
	if e.Button == platform.ButtonLeft {
		delete(b.p.restore, "playlist")
		b.p.List.StartMove()
		return true
	}
	return false
}
func (b *playlistPane) contextMenu(from widget.Component, at paint2d.Point) {
	h, t := b.p.Host, b.p.Transport
	remove := widgets.Item("Remove current track", h.RemoveTrack)
	remove.Disabled = t.List.Len() == 0
	items := []*widgets.MenuItem{
		widgets.Item("Open music files…", func() { h.OpenFiles(from, false) }),
		widgets.Item("Add music files…", func() { h.OpenFiles(from, true) }),
		remove, widgets.Sep(),
		widgets.CheckItem("Shuffle", t.Random, func() { h.Command(players.CmdShuffle) }),
		widgets.Item("Repeat: "+t.Repeat.String(), func() { h.Command(players.CmdRepeat) }),
	}
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
func (b *playlistPane) KeyPress(e widget.KeyEvent) bool { return b.p.keys(b.p.List, e) }
func (b *playlistPane) Describe(n *a11y.Node)           { n.Role, n.Name = a11y.RoleGroup, "Pod playlist" }
func (b *playlistPane) DropTypes() []string             { return []string{"text/uri-list"} }
func (b *playlistPane) DropActionFor(offered platform.DragAction) platform.DragAction {
	return offered & platform.DragCopy
}
func (b *playlistPane) Drop(e widget.DropEvent) bool { return b.p.Host.AcceptFileDrop(e) }

type resizeGrip struct {
	widget.Base
	p *Player
}

func (g *resizeGrip) Paint(*paint2d.Context) {}
func (g *resizeGrip) MousePress(e widget.MouseEvent) bool {
	if e.Button != platform.ButtonLeft {
		return false
	}
	delete(g.p.restore, "playlist")
	return g.p.List.StartResize(platform.EdgeBottom | platform.EdgeRight)
}
