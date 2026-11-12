package itunes

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

type eqPane struct {
	widget.Base
	p               *Player
	slots           *skins.Slots
	buttons         map[string]*players.GlyphButton
	bands           []*players.Fader
	preamp, balance *sourceSlider
}

func newEqPane(p *Player) *eqPane {
	b := &eqPane{p: p, slots: skins.NewSlots(LayoutEqualizer), buttons: make(map[string]*players.GlyphButton)}
	b.Init(b)
	b.SetManagesChildren(true)
	b.SetAccessibleName("iTunes equalizer")
	button := func(slot, label string, glyph players.Glyph, action func()) *players.GlyphButton {
		c := skinButton(LayoutEqualizer, slot, label, glyph, action)
		b.buttons[slot] = c
		b.slots.Bind(slot, c)
		b.Add(c)
		return c
	}
	button("close", "Close equalizer", players.GlyphNone, func() { p.ShowEqualiser(false) })
	button("on", "Equalizer on", players.GlyphNone, func() { p.Equaliser.SetOn(!p.Equaliser.On) }).Toggle = true
	button("auto", "Auto", players.GlyphNone, func() { p.Equaliser.SetAuto(!p.Equaliser.Auto, p.Transport.List.Index()) }).Toggle = true
	button("reset", "Reset", players.GlyphNone, p.Equaliser.Flat)
	preset := button("preset", "Equalizer presets", players.GlyphNone, b.presetMenu)
	for _, slot := range []string{"preset", "auto", "reset"} {
		c := b.buttons[slot]
		c.Shaper = nil
		name := c.Label
		c.Art = func(ctx *paint2d.Context, r paint2d.Rect, st style.ControlState) bool {
			skins.DrawSkinSlot(c.Look(), ctx, r, LayoutEqualizer, slot, st)
			if slot != "preset" {
				ctx.DrawRoundRect(r, style.Dip(c.Look(), 3), style.Dip(c.Look(), 3), paint2d.Fill(paint2d.RGB(.87, .89, .9)))
			}
			if st.Checked() {
				ctx.DrawRoundRect(r, style.Dip(c.Look(), 3), style.Dip(c.Look(), 3), paint2d.Fill(paint2d.RGB(.65, .77, .86)))
			}
			label := name
			if slot == "preset" {
				label = p.Equaliser.Preset
			}
			ink := paint2d.RGB(.13, .15, .17)
			if st.Disabled() {
				ink = paint2d.RGB(.5, .52, .54)
			}
			players.DrawTextIn(ctx, players.ScaledFace(c.Look().Font(), style.Dip(c.Look(), 10)), r, label, ink, style.AlignCenter)
			return true
		}
	}
	preset.Tip = "Equalizer preset"
	b.preamp = newSourceSlider(true, -players.EqRange, players.EqRange, p.Equaliser.Preamp, "Preamp", p.Equaliser.SetPreamp, "eq")
	b.preamp.Format = players.Decibels
	b.slots.Bind("preamp", b.preamp)
	b.Add(b.preamp)
	for i, name := range players.EqBands {
		band := i
		f := newSourceSlider(true, -players.EqRange, players.EqRange, p.Equaliser.Gains[i], name+" Hz", func(v float32) { p.Equaliser.Set(band, v) }, "eq")
		f.Format = players.Decibels
		b.bands = append(b.bands, f.Fader)
		b.slots.Bind(fmt.Sprintf("eq.%d", i), f)
		b.Add(f)
	}
	b.balance = newSourceSlider(false, -1, 1, p.Transport.Pan, "Balance", p.Host.SetPan, "volume")
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
	b.slots.Bind("balance", b.balance)
	b.Add(b.balance)
	return b
}
func (b *eqPane) presetMenu() {
	c := b.buttons["preset"]
	o := widget.DeviceOrigin(c)
	var items []*widgets.MenuItem
	for _, pr := range players.EqPresets() {
		name := pr.Name
		items = append(items, widgets.RadioItem(name, "preset", name == b.p.Equaliser.Preset, func() { b.p.Equaliser.Apply(name) }))
	}
	widgets.ShowContextMenu(c, paint2d.Pt(o.X, o.Y+c.Bounds().Dy()), items...)
}
func (b *eqPane) sync() {
	e := b.p.Equaliser
	b.buttons["on"].SetChecked(e.On)
	b.buttons["auto"].SetChecked(e.Auto)
	if !b.preamp.Dragging() {
		b.preamp.Set(e.Preamp)
	}
	for i, f := range b.bands {
		if !f.Dragging() {
			f.Set(e.Gains[i])
		}
	}
	if !b.balance.Dragging() {
		b.balance.Set(b.p.Transport.Pan)
	}
	b.Invalidate()
}
func (b *eqPane) Measure(c layout.Constraints) paint2d.Point {
	sz, _ := b.slots.Measure(b.Look(), c)
	return sz
}
func (b *eqPane) Arrange(r paint2d.Rect) {
	b.SetBounds(r)
	b.slots.Arrange(b.Look(), b.LocalBounds())
}
func (b *eqPane) Paint(ctx *paint2d.Context) {
	lk, box := b.Look(), b.LocalBounds()
	ctx.DrawRect(box, paint2d.Paint{Color: paint2d.White, Blend: paint2d.BlendDestOut})
	skins.DrawSkinLayout(lk, ctx, box, LayoutEqualizer)
	at := func(slot string) paint2d.Rect {
		r, _ := skins.SkinSlotRect(lk, LayoutEqualizer, slot, box)
		return r
	}
	f := players.ScaledFace(lk.Font(), style.Dip(lk, 9))
	ink := paint2d.RGB(.14, .16, .18)
	players.DrawTextIn(ctx, players.ScaledFace(lk.Font(), style.Dip(lk, 12)), at("title"), "Equalizer", ink, style.AlignCenter)
	players.DrawTextIn(ctx, f, at("on.label"), "On", ink, style.AlignStart)
	players.DrawTextIn(ctx, f, at("preamp.label"), "Preamp", ink, style.AlignCenter)
	for i, name := range players.EqBands {
		players.DrawTextIn(ctx, f, at(fmt.Sprintf("eq.label.%d", i)), name, ink, style.AlignCenter)
	}
	players.DrawTextIn(ctx, f, paint2d.XYWH(style.Dip(lk, 224), style.Dip(lk, 210), style.Dip(lk, 54), style.Dip(lk, 15)), "Balance", ink, style.AlignStart)
}
func (b *eqPane) MousePress(e widget.MouseEvent) bool {
	if e.Button == platform.ButtonRight {
		b.p.body.contextMenu(b, e.Pos.Add(widget.DeviceOrigin(b)))
		return true
	}
	if e.Button == platform.ButtonLeft {
		b.p.userMoved("equaliser")
		b.p.Eq.StartMove()
		return true
	}
	return false
}
func (b *eqPane) KeyPress(e widget.KeyEvent) bool { return b.p.keys(b.p.Eq, e) }
func (b *eqPane) Describe(n *a11y.Node)           { n.Role, n.Name = a11y.RoleGroup, "iTunes equalizer" }
func (b *eqPane) DropTypes() []string             { return []string{"text/uri-list"} }
func (b *eqPane) DropActionFor(offered platform.DragAction) platform.DragAction {
	return offered & platform.DragCopy
}
func (b *eqPane) Drop(e widget.DropEvent) bool { return b.p.Host.AcceptFileDrop(e) }
