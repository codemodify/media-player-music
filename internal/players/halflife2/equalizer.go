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

type equalizerPane struct {
	widget.Base
	p       *Player
	slots   *skins.Slots
	buttons map[string]*players.GlyphButton
	bands   []*players.Fader
	balance *players.Fader
	legacy  []*players.Fader
}

func newEqualizerPane(p *Player) *equalizerPane {
	b := &equalizerPane{p: p, slots: skins.NewSlots(LayoutEqualizer), buttons: make(map[string]*players.GlyphButton)}
	b.Init(b)
	b.SetManagesChildren(true)
	b.SetAccessibleName("Half Life 2 equalizer")
	button := func(slot, name string, glyph players.Glyph, action func()) *players.GlyphButton {
		c := skinButton(LayoutEqualizer, slot, name, glyph, action)
		b.buttons[slot] = c
		b.slots.Bind(slot, c)
		b.Add(c)
		return c
	}
	button("close", "Close equalizer", players.GlyphNone, func() { p.ShowEqualiser(false) })
	button("eq.prev", "Previous equalizer preset", players.GlyphNone, func() { b.stepPreset(false) })
	button("eq.next", "Next equalizer preset", players.GlyphNone, func() { b.stepPreset(true) })
	button("reset", "Reset equalizer and balance", players.GlyphNone, func() {
		p.Equaliser.Flat()
		p.Host.SetPan(0)
	})
	button("on", "Equalizer on", players.GlyphSliders, func() { p.Equaliser.SetOn(!p.Equaliser.On) }).Toggle = true
	for _, item := range []struct{ slot, label string }{{"crossfade", "Crossfade"}, {"srs", "SRS WOW Effects"}, {"speaker", "SRS speaker processing"}} {
		c := button(item.slot, item.label, players.GlyphNone, nil)
		c.SetEnabled(false)
		c.Tip = item.label + " is not in this player"
		c.SetAccessibleDescription(c.Tip)
	}
	preset := button("preset", "Equalizer presets and preamp", players.GlyphNone, b.presetMenu)
	preset.Shaper = nil
	preset.Art = func(ctx *paint2d.Context, r paint2d.Rect, _ style.ControlState) bool {
		players.DrawTextIn(ctx, players.ScaledFace(preset.Look().Font(), style.Dip(preset.Look(), 8)), r, p.Equaliser.Preset, paint2d.RGB(.1, .1, .1), style.AlignCenter)
		return true
	}
	for i, name := range players.EqBands {
		band := i
		f := players.NewFader(-players.EqRange, players.EqRange, p.Equaliser.Gains[i], name+" Hz", func(v float32) { p.Equaliser.Set(band, v) })
		f.Format = players.Decibels
		f.Travel = 6
		paintBand(f)
		b.slots.Bind(fmt.Sprintf("eq.%d", i), f)
		b.bands = append(b.bands, f)
		b.Add(f)
	}
	b.balance = players.NewBar(-1, 1, p.Transport.Pan, "Balance", p.Host.SetPan)
	b.balance.Travel = 5.5
	b.balance.Step, b.balance.Page = .1, .25
	b.balance.Format = func(v float32) string {
		if v == 0 {
			return "Center"
		}
		if v < 0 {
			return fmt.Sprintf("%d%% left", int(-v*100+.5))
		}
		return fmt.Sprintf("%d%% right", int(v*100+.5))
	}
	paintHorizontal(b.balance, "balance")
	b.slots.Bind("balance", b.balance)
	b.Add(b.balance)
	for _, item := range []struct{ slot, label string }{{"trubass", "TruBass"}, {"wow", "WOW Effect"}} {
		f := players.NewBar(0, 100, 0, item.label, nil)
		f.Travel = 5.5
		paintHorizontal(f, "effect")
		f.SetEnabled(false)
		f.Tip = item.label + " is not in this player"
		f.SetAccessibleDescription(f.Tip)
		b.slots.Bind(item.slot, f)
		b.legacy = append(b.legacy, f)
		b.Add(f)
	}
	return b
}
func paintBand(f *players.Fader) {
	f.Painter = func(ctx *paint2d.Context, r paint2d.Rect, t float32, _ style.ControlState) bool {
		lk := f.Look()
		skins.DrawSkinSprite(lk, ctx, r, "band.track", paint2d.Color{})
		d := style.Dip(lk, 1)
		thumb := paint2d.XYWH(r.Min.X, r.Min.Y+(1-t)*(r.Dy()-12*d), r.Dx(), 12*d)
		// WMP's foregroundImage is a level fill, revealed only below the
		// moving thumb. Its source thumb remains above that clipped fill.
		ctx.Save()
		ctx.ClipRect(paint2d.XYWH(r.Min.X, thumb.Min.Y+thumb.Dy()/2, r.Dx(), r.Max.Y-thumb.Min.Y-thumb.Dy()/2))
		skins.DrawSkinSprite(lk, ctx, r, "band.foreground", paint2d.Color{})
		ctx.Restore()
		return skins.DrawSkinSprite(lk, ctx, thumb, "eq.thumb", paint2d.Color{})
	}
}
func paintHorizontal(f *players.Fader, prefix string) {
	f.Painter = func(ctx *paint2d.Context, r paint2d.Rect, t float32, _ style.ControlState) bool {
		lk := f.Look()
		skins.DrawSkinSprite(lk, ctx, r, prefix+".track", paint2d.Color{})
		d := style.Dip(lk, 1)
		thumb := paint2d.XYWH(r.Min.X+t*(r.Dx()-11*d), r.Min.Y+(r.Dy()-10*d)/2, 11*d, 10*d)
		ctx.Save()
		ctx.ClipRect(paint2d.XYWH(r.Min.X, r.Min.Y, thumb.Min.X+thumb.Dx()/2-r.Min.X, r.Dy()))
		skins.DrawSkinSprite(lk, ctx, r, prefix+".foreground", paint2d.Color{})
		ctx.Restore()
		return skins.DrawSkinSprite(lk, ctx, thumb, "horizontal.thumb", paint2d.Color{})
	}
}
func (b *equalizerPane) stepPreset(next bool) {
	list := players.EqPresets()
	if len(list) == 0 {
		return
	}
	idx := 0
	for i, pr := range list {
		if pr.Name == b.p.Equaliser.Preset {
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
func (b *equalizerPane) presetMenu() {
	e := b.p.Equaliser
	c := b.buttons["preset"]
	o := widget.DeviceOrigin(c)
	items := []*widgets.MenuItem{widgets.CheckItem("Equalizer on", e.On, func() { e.SetOn(!e.On) }), widgets.CheckItem("Auto preset per track", e.Auto, func() { e.SetAuto(!e.Auto, b.p.Transport.List.Index()) }), widgets.Item("Reset", e.Flat), widgets.Sep()}
	var preamp []*widgets.MenuItem
	for _, gain := range []float32{-12, -9, -6, -3, 0, 3, 6, 9, 12} {
		gain := gain
		preamp = append(preamp, widgets.RadioItem(players.Decibels(gain), "preamp", e.Preamp == gain, func() { e.SetPreamp(gain) }))
	}
	items = append(items, widgets.Submenu("Preamp: "+players.Decibels(e.Preamp), preamp...), widgets.Sep())
	for _, pr := range players.EqPresets() {
		name := pr.Name
		items = append(items, widgets.RadioItem(name, "preset", name == e.Preset, func() { e.Apply(name) }))
	}
	widgets.ShowContextMenu(c, paint2d.Pt(o.X, o.Y+c.Bounds().Dy()), items...)
}
func (b *equalizerPane) sync() {
	eq := b.p.Equaliser
	b.buttons["on"].SetChecked(eq.On)
	for i, f := range b.bands {
		f.Set(eq.Gains[i])
	}
	b.balance.Set(b.p.Transport.Pan)
	b.Invalidate()
}
func (b *equalizerPane) Measure(c layout.Constraints) paint2d.Point {
	sz, _ := b.slots.Measure(b.Look(), c)
	return sz
}
func (b *equalizerPane) Arrange(r paint2d.Rect) {
	b.SetBounds(r)
	b.slots.Arrange(b.Look(), b.LocalBounds())
}
func (b *equalizerPane) Paint(ctx *paint2d.Context) {
	lk, box := b.Look(), b.LocalBounds()
	skins.DrawSkinLayout(lk, ctx, box, LayoutEqualizer)
	f := players.ScaledFace(lk.Font(), style.Dip(lk, 8))
	col := paint2d.RGB(.1, .1, .1)
	for _, item := range []struct{ slot, text string }{{"balance.label", "Balance"}, {"srs.label", "SRS WOW Effects"}, {"speaker.label", "Normal Speakers"}, {"trubass.label", "TruBass"}, {"wow.label", "WOW Effect"}} {
		r, _ := skins.SkinSlotRect(lk, LayoutEqualizer, item.slot, box)
		players.DrawTextIn(ctx, f, r, item.text, col, style.AlignStart)
	}
}
func (b *equalizerPane) MousePress(e widget.MouseEvent) bool {
	if e.Button == platform.ButtonRight {
		b.p.Host.ContextMenu(b, e.Pos.Add(widget.DeviceOrigin(b)))
		return true
	}
	if e.Button == platform.ButtonLeft {
		b.p.g.StartMove(b.p.iEq)
		return true
	}
	return false
}
func (b *equalizerPane) KeyPress(e widget.KeyEvent) bool { return b.p.keys(b.p.Eq(), e) }
func (b *equalizerPane) Describe(n *a11y.Node) {
	n.Role, n.Name = a11y.RoleGroup, "Half Life 2 equalizer"
}
func (b *equalizerPane) DropTypes() []string { return []string{"text/uri-list"} }
func (b *equalizerPane) DropActionFor(offered platform.DragAction) platform.DragAction {
	return offered & platform.DragCopy
}
func (b *equalizerPane) Drop(e widget.DropEvent) bool { return b.p.Host.AcceptFileDrop(e) }
