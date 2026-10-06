package amarok

import (
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
	p                        *Player
	on, auto, presets, reset *players.GlyphButton
	preamp, balance          *sourceSlider
	bands                    []*sourceSlider
}

func newEqPane(p *Player) *eqPane {
	e := &eqPane{p: p}
	e.Init(e)
	e.SetManagesChildren(true)
	e.on = textButton(p, "On", func() { p.Equaliser.SetOn(!p.Equaliser.On) })
	e.on.Toggle = true
	e.on.SetAccessibleName("Equalizer on")
	e.auto = textButton(p, "Auto", func() { p.Equaliser.SetAuto(!p.Equaliser.Auto, p.Transport.List.Index()); p.Refresh() })
	e.auto.Toggle = true
	e.auto.SetAccessibleName("Automatic equalizer preset per track")
	e.presets = textButton(p, "Presets", e.presetMenu)
	e.reset = textButton(p, "Reset", p.Equaliser.Flat)
	e.preamp = slider(true, -players.EqRange, players.EqRange, p.Equaliser.Preamp, "Preamp", p.Equaliser.SetPreamp)
	e.preamp.Format = players.Decibels
	e.balance = slider(false, -1, 1, p.Transport.Pan, "Balance", p.Host.SetPan)
	e.balance.Step = .1
	for i, name := range players.EqBands {
		band := i
		f := slider(true, -players.EqRange, players.EqRange, p.Equaliser.Gains[i], name+" Hz", func(v float32) { p.Equaliser.Set(band, v) })
		f.Format = players.Decibels
		e.bands = append(e.bands, f)
	}
	for _, c := range []widget.Component{e.on, e.auto, e.presets, e.reset, e.preamp, e.balance} {
		e.Add(c)
	}
	for _, c := range e.bands {
		e.Add(c)
	}
	return e
}
func (e *eqPane) presetMenu() {
	var items []*widgets.MenuItem
	for _, preset := range players.EqPresets() {
		name := preset.Name
		items = append(items, widgets.RadioItem(name, "eq.preset", e.p.Equaliser.Preset == name, func() { e.p.Equaliser.Apply(name) }))
	}
	widgets.ShowContextMenu(e.presets, menuAt(e.presets), items...)
}
func (e *eqPane) sync() {
	available := e.p.Host.EqualizerAvailable()
	tip := e.p.Host.FeatureTip("Equalizer", available)
	for _, c := range []widget.Component{e.on, e.auto, e.presets, e.reset, e.preamp} {
		c.SetEnabled(available)
	}
	e.on.SetChecked(e.p.Equaliser.On)
	e.auto.SetChecked(e.p.Equaliser.Auto)
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
	return c.Constrain(paintengine2d.Pt(style.Dip(e.Look(), 460), style.Dip(e.Look(), 190)))
}
func (e *eqPane) Arrange(r paintengine2d.Rect) {
	e.SetBounds(r)
	b := e.LocalBounds()
	d := func(n float32) float32 { return style.Dip(e.Look(), n) }
	x := d(4)
	for _, c := range []*players.GlyphButton{e.on, e.auto, e.presets, e.reset} {
		w := d(54)
		if c == e.presets {
			w = d(74)
		}
		c.Arrange(paintengine2d.XYWH(x, d(27), w, d(24)))
		x += w + d(5)
	}
	e.balance.Arrange(paintengine2d.XYWH(b.Dx()-d(136), d(30), d(126), d(20)))
	each := (b.Dx() - d(16)) / 11
	for i, c := range append([]*sourceSlider{e.preamp}, e.bands...) {
		c.Arrange(paintengine2d.XYWH(d(8)+float32(i)*each, d(64), each, d(102)))
	}
}
func (e *eqPane) Paint(ctx *paintengine2d.Context) {
	lk, r := e.Look(), e.LocalBounds()
	d := func(n float32) float32 { return style.Dip(lk, n) }
	ctx.DrawRect(r, paintengine2d.Fill(chrome))
	ctx.DrawRect(r.Inset(d(.5)), paintengine2d.StrokePaint(border, d(1)))
	title := "Equalizer · " + e.p.Equaliser.Preset
	if !e.p.Host.EqualizerAvailable() {
		title = e.p.Host.FeatureTip("Equalizer", false)
	}
	players.DrawTextIn(ctx, font(lk, 11), paintengine2d.XYWH(d(6), d(2), r.Dx()-d(12), d(22)), title, ink, style.AlignStart)
	players.DrawTextIn(ctx, font(lk, 9), paintengine2d.XYWH(e.balance.Bounds().Min.X, d(14), e.balance.Bounds().Dx(), d(14)), "Balance", muted, style.AlignCenter)
	for i, f := range append([]*sourceSlider{e.preamp}, e.bands...) {
		name := "Preamp"
		if i > 0 {
			name = players.EqBands[i-1]
		}
		rect := f.Bounds()
		players.DrawTextIn(ctx, font(lk, 9), paintengine2d.XYWH(rect.Min.X, d(167), rect.Dx(), d(19)), name, muted, style.AlignCenter)
	}
}
func (e *eqPane) Describe(n *a11y.Node) { n.Role, n.Name = a11y.RoleGroup, "Equalizer" }
