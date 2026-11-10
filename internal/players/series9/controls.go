package series9

import (
	"github.com/codemodify/media-player-music/internal/players"
	"goframework.io/desktop-ui-skins"
	"goframework.io/desktop-ui/a11y"
	"goframework.io/desktop-ui/layout"
	"goframework.io/desktop-ui/platform"
	"goframework.io/desktop-ui/style"
	"goframework.io/desktop-ui/widget"
	"goframework.io/graphics-paint2d"
)

// pane supplies native window gestures and file-drop routing around the
// source skin's controls. Layout and actions belong to this application.
type pane struct {
	widget.Base
	p       *Player
	role    string
	name    string
	slots   *skins.Slots
	buttons map[string]*players.GlyphButton
}

func newPane(p *Player, role, name string, self widget.Component) *pane {
	b := &pane{p: p, role: role, name: name, slots: skins.NewSlots(role), buttons: make(map[string]*players.GlyphButton)}
	b.Init(self)
	b.SetManagesChildren(true)
	b.SetAccessibleName(name)
	return b
}
func (b *pane) button(slot, label string, glyph players.Glyph, action func()) *players.GlyphButton {
	c := players.NewGlyphButton(glyph, label, action)
	artSlot := func() string {
		if slot == "play" && c.Glyph == players.GlyphPause {
			return "pause"
		}
		return slot
	}
	c.Art = func(ctx *paint2d.Context, r paint2d.Rect, st style.ControlState) bool {
		if skins.DrawSkinSlot(c.Look(), ctx, skins.SlotArtRect(c, b.role, artSlot()), b.role, artSlot(), st) {
			return true
		}
		players.DrawGlyph(ctx, r, c.Glyph, players.GlyphInk(c.Look(), st), style.Dip(c.Look(), 1))
		return true
	}
	c.Shaper = func(lk style.LookAndFeel, size paint2d.Point) *style.Silhouette {
		return skins.SkinSlotShape(lk, size, skins.SlotArtRect(c, b.role, artSlot()), b.role, artSlot())
	}
	b.buttons[slot] = c
	b.slots.Bind(slot, c)
	b.Add(c)
	return c
}
func (b *pane) Measure(c layout.Constraints) paint2d.Point {
	sz, _ := b.slots.Measure(b.Look(), c)
	return sz
}
func (b *pane) Arrange(r paint2d.Rect) {
	b.SetBounds(r)
	b.slots.Arrange(b.Look(), b.LocalBounds())
}
func (b *pane) Paint(ctx *paint2d.Context) {
	skins.DrawSkinLayout(b.Look(), ctx, b.LocalBounds(), b.role)
}
func (b *pane) MousePress(e widget.MouseEvent) bool {
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
func (b *pane) KeyPress(e widget.KeyEvent) bool { return b.p.keys(b.p.Main, e) }
func (b *pane) Describe(n *a11y.Node)           { n.Role, n.Name = a11y.RoleGroup, b.name }
func (b *pane) DropTypes() []string             { return []string{"text/uri-list"} }
func (b *pane) DropActionFor(offered platform.DragAction) platform.DragAction {
	return offered & platform.DragCopy
}
func (b *pane) Drop(e widget.DropEvent) bool { return b.p.Host.AcceptFileDrop(e) }

type sourceSlider struct{ *players.Fader }

func newSourceSlider(vertical bool, lo, hi, value float32, label string, on func(float32), prefix string) *sourceSlider {
	var f *players.Fader
	if vertical {
		f = players.NewFader(lo, hi, value, label, on)
	} else {
		f = players.NewBar(lo, hi, value, label, on)
	}
	s := &sourceSlider{f}
	s.Init(s)
	thumbW, thumbH := float32(10), float32(18)
	f.Travel = 5
	switch prefix {
	case "seek":
		thumbW, thumbH = 23, 13
		f.Travel = 11.5
	case "eq":
		thumbW, thumbH = 18, 17
		f.Travel = 8.5
	case "balance":
		thumbW, thumbH = 17, 18
		f.Travel = 8.5
	}
	f.Painter = func(ctx *paint2d.Context, r paint2d.Rect, t float32, st style.ControlState) bool {
		lk, d := s.Look(), style.Dip(s.Look(), 1)
		track := prefix + ".track"
		thumbName := prefix + ".thumb"
		if st.Disabled() {
			track += ".disabled"
			thumbName += ".disabled"
		} else if st.Pressed() {
			thumbName += ".down"
		} else if st.Hovered() {
			thumbName += ".hover"
		}
		if !skins.DrawSkinSprite(lk, ctx, r, track, paint2d.Color{}) {
			skins.DrawSkinSprite(lk, ctx, r, prefix+".track", paint2d.Color{})
		}
		thumb := paint2d.XYWH(r.Min.X+t*(r.Dx()-thumbW*d), r.Min.Y+(r.Dy()-thumbH*d)/2, thumbW*d, thumbH*d)
		if vertical {
			thumb = paint2d.XYWH(r.Min.X+(r.Dx()-thumbW*d)/2, r.Min.Y+(1-t)*(r.Dy()-thumbH*d), thumbW*d, thumbH*d)
		} else if !st.Disabled() {
			ctx.Save()
			ctx.ClipRect(paint2d.XYWH(r.Min.X, r.Min.Y, thumb.Min.X+thumb.Dx()/2-r.Min.X, r.Dy()))
			skins.DrawSkinSprite(lk, ctx, r, prefix+".foreground", paint2d.Color{})
			ctx.Restore()
		}
		if !skins.DrawSkinSprite(lk, ctx, thumb, thumbName, paint2d.Color{}) {
			skins.DrawSkinSprite(lk, ctx, thumb, prefix+".thumb", paint2d.Color{})
		}
		if st.Focused() {
			lk.DrawFocusRing(ctx, r)
		}
		return true
	}
	return s
}
func (s *sourceSlider) MousePress(e widget.MouseEvent) bool {
	if e.Button != platform.ButtonLeft {
		return false
	}
	return s.Fader.MousePress(e)
}
func (s *sourceSlider) MouseRelease(e widget.MouseEvent) bool {
	if e.Button != platform.ButtonLeft {
		return false
	}
	return s.Fader.MouseRelease(e)
}

type resizeGrip struct {
	widget.Base
	p *Player
}

func newResizeGrip(p *Player) *resizeGrip {
	g := &resizeGrip{p: p}
	g.Init(g)
	g.SetTransparent(true)
	g.SetAccessibleName("Resize Music Player")
	return g
}
func (g *resizeGrip) Paint(*paint2d.Context) {}
func (g *resizeGrip) MousePress(e widget.MouseEvent) bool {
	if e.Button != platform.ButtonLeft {
		return false
	}
	g.p.userMoved("main")
	return g.p.Main.StartResize(platform.EdgeBottom | platform.EdgeRight)
}
