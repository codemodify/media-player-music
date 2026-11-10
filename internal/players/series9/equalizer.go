package series9

import (
	"fmt"
	"math"

	"github.com/codemodify/media-player-music/internal/players"
	"goframework.io/desktop-ui-skins"
	"goframework.io/desktop-ui-widgets"
	"goframework.io/desktop-ui/style"
	"goframework.io/desktop-ui/widget"
	"goframework.io/graphics-paint2d"
)

type eqPane struct {
	*pane
	bands   []*players.Fader
	sliders []*sourceSlider
	balance *sourceSlider
}

func newEqPane(p *Player) *eqPane {
	b := &eqPane{}
	b.pane = newPane(p, LayoutEqualizer, "Series 9 equalizer", b)
	b.button("on", "Equalizer on", players.GlyphNone, func() { p.Equaliser.SetOn(!p.Equaliser.On) }).Toggle = true
	b.button("reset", "Reset equalizer", players.GlyphNone, p.Equaliser.Flat)
	b.button("preset", "Equalizer presets and preamp", players.GlyphNone, b.presetMenu)
	b.button("close", "Hide equalizer", players.GlyphNone, func() { p.ShowEqualiser(false) })
	for i, name := range players.EqBands {
		band := i
		f := newSourceSlider(true, -players.EqRange, players.EqRange, p.Equaliser.Gains[i], name+" Hz", func(v float32) { p.Equaliser.Set(band, v) }, "eq")
		f.Format = players.Decibels
		b.sliders = append(b.sliders, f)
		b.bands = append(b.bands, f.Fader)
		b.Add(f)
	}
	b.balance = newSourceSlider(false, -1, 1, p.Transport.Pan, "Balance", p.Host.SetPan, "balance")
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
func (b *eqPane) bandRect(i int) paint2d.Rect {
	d := style.Dip(b.Look(), 1)
	w := b.LocalBounds().Dx() / d
	// Original Corona.js GetEqSliderLeft distributes ten18px tracks inside
	// the328px middle panel. Keep design-pixel flooring before display scale.
	x := 2 + float32(math.Floor(float64((w-6-180)/11*float32(i+1)+18*float32(i))))
	return paint2d.XYWH(x*d, 33*d, 18*d, 67*d)
}
func (b *eqPane) Arrange(r paint2d.Rect) {
	b.pane.Arrange(r)
	for i, f := range b.sliders {
		f.Arrange(b.bandRect(i))
	}
}
func (b *eqPane) sync() {
	e, available := b.p.Equaliser, b.p.Host.EqualizerAvailable()
	for _, slot := range []string{"on", "reset", "preset"} {
		c := b.buttons[slot]
		c.SetEnabled(available)
		c.Tip = b.p.Host.FeatureTip("EQ", available)
	}
	b.buttons["on"].SetChecked(e.On)
	for i, f := range b.bands {
		f.SetEnabled(available)
		f.Tip = b.p.Host.FeatureTip("EQ", available)
		if !f.Dragging() {
			f.Set(e.Gains[i])
		}
	}
	b.balance.SetEnabled(b.p.Host.BalanceAvailable())
	b.balance.Tip = b.p.Host.FeatureTip("Balance", b.p.Host.BalanceAvailable())
	if !b.balance.Dragging() {
		b.balance.Set(b.p.Transport.Pan)
	}
	b.Invalidate()
}
func (b *eqPane) presetMenu() {
	c, e := b.buttons["preset"], b.p.Equaliser
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
func (b *eqPane) Paint(ctx *paint2d.Context) {
	b.pane.Paint(ctx)
	lk, box := b.Look(), b.LocalBounds()
	f := players.ScaledFace(lk.Font(), style.Dip(lk, 9))
	ink := paint2d.RGB(.08, .12, .19)
	if !b.p.Host.EqualizerAvailable() {
		ink = paint2d.RGB(.5, .5, .5)
	}
	d := style.Dip(lk, 1)
	for i, name := range players.EqBands {
		r := b.bandRect(i)
		label := paint2d.XYWH(r.Min.X-6*d, 104*d, 30*d, 15*d)
		players.DrawTextIn(ctx, f, label, name, ink, style.AlignCenter)
	}
	for _, item := range []struct{ slot, text string }{{"preset.label", b.p.Equaliser.Preset}, {"balance.label", "Balance"}} {
		r, _ := skins.SkinSlotRect(lk, LayoutEqualizer, item.slot, box)
		players.DrawTextIn(ctx, f, r, item.text, ink, style.AlignStart)
	}
}
