package vlc

import (
	"fmt"
	"github.com/codemodify/media-player-music/internal/players"
	"github.com/codemodify/paintengine2d"
	"github.com/codemodify/uitoolkit/a11y"
	"github.com/codemodify/uitoolkit/layout"
	"github.com/codemodify/uitoolkit/style"
	"github.com/codemodify/uitoolkit/widget"
	"github.com/codemodify/uitoolkit/widgets"
)

type eqPane struct {
	widget.Base
	p               *Player
	on, auto        *widgets.Checkbox
	presets, reset  *widgets.Button
	preamp, balance *sourceSlider
	bands           []*sourceSlider
}

func newEqPane(p *Player) *eqPane {
	e := &eqPane{p: p}
	e.Init(e)
	e.SetManagesChildren(true)
	e.SetAccessibleName("Equalizer")
	e.on = widgets.NewCheckbox("Enable", p.Equaliser.On, p.Equaliser.SetOn)
	e.on.SetAccessibleName("Enable equalizer")
	e.auto = widgets.NewCheckbox("Auto", p.Equaliser.Auto, func(on bool) { p.Equaliser.SetAuto(on, p.Transport.List.Index()) })
	e.auto.SetAccessibleName("Auto preset per track")
	e.presets = widgets.NewButton("Presets", e.presetMenu)
	e.reset = widgets.NewButton("Reset", func() { p.Equaliser.Apply("Flat") })
	e.preamp = newSlider(true, -players.EqRange, players.EqRange, p.Equaliser.Preamp, "Equalizer preamp", p.Equaliser.SetPreamp)
	e.preamp.Format = players.Decibels
	e.balance = newSlider(false, -1, 1, p.Transport.Pan, "Balance", p.Host.SetPan)
	e.balance.Format = func(v float32) string { return fmt.Sprintf("Balance %+.0f%%", v*100) }
	for i, name := range players.EqBands {
		band := i
		f := newSlider(true, -players.EqRange, players.EqRange, p.Equaliser.Gains[i], name+" Hz", func(v float32) { p.Equaliser.Set(band, v) })
		f.Format = players.Decibels
		e.bands = append(e.bands, f)
	}
	for _, c := range []widget.Component{e.on, e.auto, e.presets, e.reset, e.preamp, e.balance} {
		e.Add(c)
	}
	for _, f := range e.bands {
		e.Add(f)
	}
	return e
}
func (e *eqPane) presetMenu() {
	var items []*widgets.MenuItem
	for _, p := range players.EqPresets() {
		name := p.Name
		items = append(items, widgets.RadioItem(name, "preset", name == e.p.Equaliser.Preset, func() { e.p.Equaliser.Apply(name) }))
	}
	at := widget.DeviceOrigin(e.presets).Add(paintengine2d.Pt(0, e.presets.Bounds().Dy()))
	widgets.ShowContextMenu(e.presets, at, items...)
}
func (e *eqPane) sync() {
	available := e.p.Host.EqualizerAvailable()
	tip := e.p.Host.FeatureTip("EQ", available)
	for _, c := range []widget.Component{e.on, e.auto, e.presets, e.reset, e.preamp} {
		c.SetEnabled(available)
	}
	e.on.SetAccessibleDescription(tip)
	e.auto.SetAccessibleDescription(tip)
	e.presets.Tip, e.reset.Tip, e.preamp.Tip = tip, tip, tip
	e.on.Checked, e.auto.Checked = e.p.Equaliser.On, e.p.Equaliser.Auto
	e.on.Invalidate()
	e.auto.Invalidate()
	e.preamp.Set(e.p.Equaliser.Preamp)
	for i, f := range e.bands {
		f.SetEnabled(available)
		f.Tip = tip
		f.Set(e.p.Equaliser.Gains[i])
	}
	e.balance.SetEnabled(e.p.Host.BalanceAvailable())
	e.balance.Tip = e.p.Host.FeatureTip("Balance", e.p.Host.BalanceAvailable())
	e.balance.Set(e.p.Transport.Pan)
	e.Invalidate()
}
func (e *eqPane) Measure(c layout.Constraints) paintengine2d.Point {
	return c.Constrain(paintengine2d.Pt(style.Dip(e.Look(), 428), style.Dip(e.Look(), 116)))
}
func (e *eqPane) Arrange(r paintengine2d.Rect) {
	e.SetBounds(r)
	b, lk := e.LocalBounds(), e.Look()
	d := func(v float32) float32 { return style.Dip(lk, v) }
	for _, item := range []struct {
		c    widget.Component
		x, w float32
	}{{e.on, 0, 74}, {e.auto, 78, 60}, {e.presets, 144, 98}, {e.reset, 248, 52}} {
		item.c.Arrange(paintengine2d.XYWH(d(item.x), 0, d(item.w), d(23)))
	}
	e.balance.Arrange(paintengine2d.XYWH(d(357), 0, max(d(54), b.Dx()-d(357)), d(23)))
	each := b.Dx() / 11
	for i, f := range append([]*sourceSlider{e.preamp}, e.bands...) {
		f.Arrange(paintengine2d.XYWH(float32(i)*each, d(28), each, max(d(36), b.Dy()-d(47))))
	}
}
func (e *eqPane) Paint(ctx *paintengine2d.Context) {
	lk, b := e.Look(), e.LocalBounds()
	d := func(v float32) float32 { return style.Dip(lk, v) }
	font := players.ScaledFace(lk.Font(), d(9))
	players.DrawTextIn(ctx, font, paintengine2d.XYWH(d(304), 0, d(50), d(23)), "Balance", paintengine2d.Black, style.AlignEnd)
	for i, f := range append([]*sourceSlider{e.preamp}, e.bands...) {
		label := "Preamp"
		if i > 0 {
			label = players.EqBands[i-1]
		}
		r := f.Bounds()
		players.DrawTextIn(ctx, font, paintengine2d.XYWH(r.Min.X, b.Dy()-d(17), r.Dx(), d(15)), label, paintengine2d.Black, style.AlignCenter)
	}
	ctx.DrawRect(paintengine2d.XYWH(0, b.Dy()-d(1), b.Dx(), d(1)), paintengine2d.Fill(paintengine2d.RGB(.5, .5, .5)))
}
func (e *eqPane) Describe(n *a11y.Node) { n.Role, n.Name = a11y.RoleGroup, "Equalizer" }
