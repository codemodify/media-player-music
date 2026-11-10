package halflife2

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

type playlistPane struct {
	widget.Base
	p       *Player
	slots   *skins.Slots
	list    *playlistList
	buttons map[string]*players.GlyphButton
	footer  *playlistStatus
}

func newPlaylistPane(p *Player) *playlistPane {
	b := &playlistPane{p: p, slots: skins.NewSlots(LayoutPlaylist), buttons: make(map[string]*players.GlyphButton)}
	b.Init(b)
	b.SetManagesChildren(true)
	b.SetAccessibleName("Half Life 2 playlist")
	b.list = newPlaylistList(p)
	b.slots.Bind("list", b.list)
	b.Add(b.list)
	button := func(slot, name string, glyph players.Glyph, action func()) *players.GlyphButton {
		c := skinButton(LayoutPlaylist, slot, name, glyph, action)
		b.buttons[slot] = c
		b.slots.Bind(slot, c)
		b.Add(c)
		return c
	}
	button("close", "Close playlist", players.GlyphNone, func() { p.ShowPlaylist(false) })
	button("shuffle", "Shuffle", players.GlyphShuffle, func() { p.Host.Command(players.CmdShuffle) }).Toggle = true
	button("repeat", "Repeat", players.GlyphRepeat, func() { p.Host.Command(players.CmdRepeat) }).Toggle = true
	// WMS asks its playlist for these controls, but the preserved archive
	// omits the two referenced dropdown images. The app paints the original
	// light dropdown and columns above the original orange-on-black rows.
	dropdown := players.NewGlyphButton(players.GlyphNone, "Now Playing playlist options", nil)
	dropdown.OnClick = func() { b.options(dropdown) }
	dropdown.Art = func(ctx *paint2d.Context, r paint2d.Rect, _ style.ControlState) bool {
		lk := dropdown.Look()
		ctx.DrawRect(r, paint2d.Fill(paint2d.RGB(.84, .84, .84)))
		f := players.ScaledFace(lk.Font(), style.Dip(lk, 11))
		text := r
		text.Min.X += style.Dip(lk, 3)
		text.Max.X -= style.Dip(lk, 18)
		ink := paint2d.RGB(.18, .18, .18)
		players.DrawTextIn(ctx, f, text, "Now Playing", ink, style.AlignStart)
		cx, cy := r.Max.X-style.Dip(lk, 9), (r.Min.Y+r.Max.Y)/2
		arrow := paint2d.NewPath()
		arrow.MoveTo(cx-style.Dip(lk, 3), cy-style.Dip(lk, 1))
		arrow.LineTo(cx+style.Dip(lk, 3), cy-style.Dip(lk, 1))
		arrow.LineTo(cx, cy+style.Dip(lk, 3))
		arrow.Close()
		ctx.DrawPath(arrow, paint2d.Fill(ink))
		return true
	}
	b.buttons["playlist.dropdown"] = dropdown
	b.slots.Bind("playlist.dropdown", dropdown)
	b.Add(dropdown)
	header := players.NewGlyphButton(players.GlyphNone, "Title, Artist, Album, Type, Length", nil)
	header.SetAccessibleDescription("Open playlist options")
	header.OnClick = func() { b.options(header) }
	header.Art = func(ctx *paint2d.Context, r paint2d.Rect, _ style.ControlState) bool {
		lk := header.Look()
		ctx.DrawRect(r, paint2d.Fill(paint2d.White))
		f, pad := players.ScaledFace(lk.Font(), style.Dip(lk, 11)), style.Dip(lk, 2)
		for i, cell := range playlistColumnRects(r, lk) {
			players.DrawTextIn(ctx, f, cell.Inset(pad), [...]string{"Title", "Artist", "Album", "Type", "Length"}[i], paint2d.RGB(.2, .2, .2), style.AlignStart)
		}
		ctx.DrawRect(paint2d.XYWH(r.Min.X, r.Max.Y-1, r.Dx(), 1), paint2d.Fill(paint2d.RGB(.2, .2, .2)))
		return true
	}
	b.buttons["playlist.header"] = header
	b.slots.Bind("playlist.header", header)
	b.Add(header)
	b.footer = &playlistStatus{pane: b}
	b.footer.Init(b.footer)
	b.slots.Bind("playlist.footer", b.footer)
	b.Add(b.footer)
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
func (b *playlistPane) options(from widget.Component) {
	o := widget.DeviceOrigin(from)
	b.contextMenu(from, paint2d.Pt(o.X, o.Y+from.Bounds().Dy()))
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
		b.p.restoredPlacement = nil
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
	if h.ShowSettings != nil {
		items = append(items, widgets.ItemAccel("Settings…", "Ctrl+,", h.OpenSettings))
	}
	widgets.ShowContextMenu(from, at, items...)
}
func (b *playlistPane) KeyPress(e widget.KeyEvent) bool { return b.p.keys(b.p.List, e) }
func (b *playlistPane) Describe(n *a11y.Node) {
	n.Role, n.Name = a11y.RoleGroup, "Half Life 2 playlist"
}
func (b *playlistPane) DropTypes() []string { return []string{"text/uri-list"} }
func (b *playlistPane) DropActionFor(offered platform.DragAction) platform.DragAction {
	return offered & platform.DragCopy
}
func (b *playlistPane) Drop(e widget.DropEvent) bool { return b.p.Host.AcceptFileDrop(e) }

type playlistStatus struct {
	widget.Base
	pane *playlistPane
}

func (s *playlistStatus) text() (selected, total string) {
	queue := s.pane.p.Transport.List
	selected = "Selected: " + playlistDuration(queue.At(s.pane.list.Selected).Length)
	known, unknown := 0, false
	for _, track := range queue.Tracks {
		if track.Length > 0 {
			known++
		} else {
			unknown = true
		}
	}
	duration := players.Clock(queue.Total())
	if unknown && known == 0 {
		duration = "--:--"
	} else if unknown {
		duration += "+"
	}
	return selected, "Total Time: " + duration
}

func (s *playlistStatus) Paint(ctx *paint2d.Context) {
	lk, r := s.Look(), s.LocalBounds()
	ctx.DrawRect(r, paint2d.Fill(paint2d.Black))
	f, pad := players.ScaledFace(lk.Font(), style.Dip(lk, 10)), style.Dip(lk, 2)
	selected, total := s.text()
	left := paint2d.XYWH(r.Min.X+pad, r.Min.Y, r.Dx()/2-pad, r.Dy())
	right := paint2d.XYWH(r.Min.X+r.Dx()/2, r.Min.Y, r.Dx()/2-pad, r.Dy())
	players.DrawTextIn(ctx, f, left, selected, orange(), style.AlignStart)
	players.DrawTextIn(ctx, f, right, total, orange(), style.AlignEnd)
}

func (s *playlistStatus) Describe(n *a11y.Node) {
	selected, total := s.text()
	n.Role, n.Name = a11y.RoleStatusBar, fmt.Sprintf("%s; %s", selected, total)
}

type resizeGrip struct {
	widget.Base
	p *Player
}

func (g *resizeGrip) Paint(*paint2d.Context) {}
func (g *resizeGrip) MousePress(e widget.MouseEvent) bool {
	if e.Button != platform.ButtonLeft {
		return false
	}
	g.p.restoredPlacement = nil
	return g.p.List.StartResize(platform.EdgeBottom | platform.EdgeRight)
}
