package realplayer

import (
	"fmt"
	"github.com/codemodify/media-player-music/internal/players"
	"goframework.io/desktop-ui-skins"
	"goframework.io/desktop-ui-widgets"
	"goframework.io/desktop-ui/a11y"
	"goframework.io/desktop-ui/layout"
	"goframework.io/desktop-ui/style"
	"goframework.io/desktop-ui/widget"
	"goframework.io/graphics-paint2d"
)

type eqPane struct {
	widget.Base
	p               *Player
	slots           *skins.Slots
	buttons         map[string]*players.GlyphButton
	bands           []*classicFader
	preamp, balance *classicFader
}

func newEqPane(p *Player) *eqPane {
	b := &eqPane{p: p, slots: skins.NewSlots(LayoutEqualizer), buttons: make(map[string]*players.GlyphButton)}
	b.Init(b)
	b.SetManagesChildren(true)
	b.SetAccessibleName("Equalizer")
	for _, item := range []struct {
		slot, text string
		run        func()
	}{{"on", "On", func() { p.Equaliser.SetOn(!p.Equaliser.On) }}, {"auto", "Auto", func() { p.Equaliser.SetAuto(!p.Equaliser.Auto, p.Transport.List.Index()) }}, {"preset", "Presets…", b.presetMenu}, {"reset", "Reset", p.Equaliser.Flat}} {
		b.buttons[item.slot] = textButton(b, b.slots, LayoutEqualizer, item.slot, "Equalizer "+item.text, item.text, item.run)
	}
	b.buttons["on"].Toggle = true
	b.buttons["auto"].Toggle = true
	b.preamp = classicSlider(true, -players.EqRange, players.EqRange, p.Equaliser.Preamp, "Preamp", p.Equaliser.SetPreamp)
	b.preamp.Format = players.Decibels
	b.slots.Bind("preamp", b.preamp)
	b.Add(b.preamp)
	for i, name := range players.EqBands {
		band := i
		f := classicSlider(true, -players.EqRange, players.EqRange, p.Equaliser.Gains[i], name+" Hz", func(v float32) { p.Equaliser.Set(band, v) })
		f.Format = players.Decibels
		b.bands = append(b.bands, f)
		b.slots.Bind(fmt.Sprintf("band.%d", i), f)
		b.Add(f)
	}
	b.balance = classicSlider(false, -1, 1, p.Transport.Pan, "Balance", p.Host.SetPan)
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
func (b *eqPane) sync() {
	e := b.p.Equaliser
	b.buttons["on"].SetChecked(e.On)
	b.buttons["auto"].SetChecked(e.Auto)
	b.buttons["preset"].Tip = e.Preset
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
	skins.DrawSkinLayout(lk, ctx, box, LayoutEqualizer)
	d := style.Dip(lk, 1)
	f := players.ScaledFace(lk.Font(), 9*d)
	for i, name := range players.EqBands {
		r := b.bands[i].Bounds()
		players.DrawTextIn(ctx, f, paint2d.XYWH(r.Min.X-5*d, r.Max.Y+2*d, r.Dx()+10*d, 12*d), name, paint2d.Black, style.AlignCenter)
	}
	players.DrawTextIn(ctx, players.ScaledFace(lk.Font(), 8*d), paint2d.XYWH(4*d, 141*d, 38*d, 12*d), "Preamp", paint2d.Black, style.AlignCenter)
	players.DrawTextIn(ctx, f, paint2d.XYWH(8*d, 156*d, 44*d, 16*d), "Balance", paint2d.Black, style.AlignStart)
}
func (b *eqPane) presetMenu() {
	e := b.p.Equaliser
	c := b.buttons["preset"]
	var items []*widgets.MenuItem
	for _, pr := range players.EqPresets() {
		name := pr.Name
		items = append(items, widgets.RadioItem(name, "preset", name == e.Preset, func() { e.Apply(name) }))
	}
	widgets.ShowContextMenu(c, widget.DeviceOrigin(c).Add(paint2d.Pt(0, c.Bounds().Dy())), items...)
}
func (b *eqPane) Describe(n *a11y.Node) { n.Role, n.Name = a11y.RoleGroup, "Equalizer" }
