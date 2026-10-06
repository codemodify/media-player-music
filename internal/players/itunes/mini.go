package itunes

import (
	"fmt"
	"time"

	"github.com/codemodify/media-player-music/internal/players"
	"github.com/codemodify/paintengine2d"
	"github.com/codemodify/uitoolkit/a11y"
	"github.com/codemodify/uitoolkit/app"
	"github.com/codemodify/uitoolkit/layout"
	"github.com/codemodify/uitoolkit/platform"
	"github.com/codemodify/uitoolkit/style"
	"github.com/codemodify/uitoolkit/widget"
)

// miniPane uses the original compact transport artwork, not scaled main buttons.
// The historical green zoom control and horizontal resize keep the same window.
type miniPane struct {
	widget.Base
	p            *Player
	slots        *widget.Slots
	buttons      map[string]*players.GlyphButton
	play         *players.GlyphButton
	seek, volume *sourceSlider
	information  bool
}

func newMiniPane(p *Player) *miniPane {
	b := &miniPane{p: p, slots: widget.NewSlots(LayoutMini), buttons: make(map[string]*players.GlyphButton), information: true}
	b.Init(b)
	b.SetManagesChildren(true)
	b.SetAccessibleName("iTunes MiniPlayer")
	button := func(slot, label string, glyph players.Glyph, action func()) *players.GlyphButton {
		c := skinButton(LayoutMini, slot, label, glyph, action)
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
	button("mini", "Return to full player", players.GlyphNone, func() { p.SetMiniPlayer(false) })
	b.volume = newSourceSlider(false, 0, 100, p.Transport.Volume*100, "Volume", func(v float32) { p.Host.SetVolume(v / 100) }, "volume")
	b.volume.Step, b.volume.Page = 2, 10
	b.volume.Format = func(v float32) string { return fmt.Sprintf("%d%%", int(v+.5)) }
	b.seek = newSourceSlider(false, 0, 1, 0, "Seek", p.Host.Seek, "seek")
	b.seek.Step, b.seek.Page = .02, .1
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
	grip := newResizeGrip(p, p.Main, platform.EdgeRight)
	b.slots.Bind("resize", grip)
	b.Add(grip)
	return b
}
func (b *miniPane) sync() {
	t := b.p.Transport
	if t.State == players.Playing {
		b.play.SetGlyph(players.GlyphPause, "Pause")
	} else {
		b.play.SetGlyph(players.GlyphPlay, "Play")
	}
	if !b.volume.Dragging() {
		b.volume.Set(t.Volume * 100)
	}
	if !b.seek.Dragging() {
		b.seek.Set(t.Fraction())
	}
	b.seek.SetEnabled(t.Length() > 0)
	b.Invalidate()
}
func (b *miniPane) Measure(c layout.Constraints) paintengine2d.Point {
	sz, _ := b.slots.Measure(b.Look(), c)
	return sz
}
func (b *miniPane) Arrange(r paintengine2d.Rect) {
	b.SetBounds(r)
	b.information = r.Dx() >= style.Dip(b.Look(), 300)
	b.seek.SetVisible(b.information)
	if !b.information && b.p.Main.Focus() == b.seek {
		b.p.Main.RequestFocus(b.play)
	}
	b.slots.Arrange(b.Look(), b.LocalBounds())
}
func (b *miniPane) Paint(ctx *paintengine2d.Context) {
	lk, box := b.Look(), b.LocalBounds()
	ctx.DrawRect(box, paintengine2d.Paint{Color: paintengine2d.White, Blend: paintengine2d.BlendDestOut})
	style.DrawSkinLayout(lk, ctx, box, LayoutMini)
	at := func(slot string) paintengine2d.Rect { r, _ := style.SkinSlotRect(lk, LayoutMini, slot, box); return r }
	style.DrawSkinSlot(lk, ctx, at("controlwell"), LayoutMini, "controlwell", 0)
	if b.information {
		style.DrawSkinSlot(lk, ctx, at("lcd"), LayoutMini, "lcd", 0)
		drawDisplay(ctx, lk, at("display"), b.p, true)
	}
}
func (b *miniPane) MousePress(e widget.MouseEvent) bool {
	if e.Button == platform.ButtonRight {
		b.p.body.contextMenu(b, e.Pos.Add(widget.DeviceOrigin(b)))
		return true
	}
	if e.Button == platform.ButtonLeft {
		b.p.userMoved("main")
		b.p.Main.StartMove()
		return true
	}
	return false
}
func (b *miniPane) KeyPress(e widget.KeyEvent) bool { return b.p.keys(b.p.Main, e) }
func (b *miniPane) Describe(n *a11y.Node)           { n.Role, n.Name = a11y.RoleGroup, "iTunes MiniPlayer" }
func (b *miniPane) DropTypes() []string             { return []string{"text/uri-list"} }
func (b *miniPane) DropActionFor(offered platform.DragAction) platform.DragAction {
	return offered & platform.DragCopy
}
func (b *miniPane) Drop(e widget.DropEvent) bool { return b.p.Host.AcceptFileDrop(e) }

type resizeGrip struct {
	widget.Base
	p      *Player
	window *app.Window
	edges  platform.Edges
}

func newResizeGrip(p *Player, w *app.Window, edges platform.Edges) *resizeGrip {
	g := &resizeGrip{p: p, window: w, edges: edges}
	g.Init(g)
	g.SetAccessibleName("Resize Music Player")
	return g
}
func (g *resizeGrip) Paint(ctx *paintengine2d.Context) {
	r, lk := g.LocalBounds(), g.Look()
	role := LayoutMain
	if g.p.MiniPlayer() {
		role = LayoutMini
	}
	if style.DrawSkinSlot(lk, ctx, r, role, "resize", 0) {
		return
	}
	d := style.Dip(lk, 1)
	ink := paintengine2d.RGB(.43, .46, .48)
	for _, n := range []float32{3, 6, 9} {
		ctx.DrawLine(paintengine2d.Pt(r.Max.X-n*d, r.Max.Y-2*d), paintengine2d.Pt(r.Max.X-2*d, r.Max.Y-n*d), paintengine2d.StrokePaint(ink, d))
	}
}
func (g *resizeGrip) MousePress(e widget.MouseEvent) bool {
	if e.Button != platform.ButtonLeft {
		return false
	}
	g.p.userMoved("main")
	return g.window.StartResize(g.edges)
}
