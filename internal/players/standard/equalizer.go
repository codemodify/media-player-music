package standard

import (
	"github.com/codemodify/media-player-music/internal/players"
	"goframework.io/desktop-ui-widgets"
	"goframework.io/desktop-ui/a11y"
	"goframework.io/desktop-ui/layout"
	"goframework.io/desktop-ui/style"
	"goframework.io/desktop-ui/widget"
	"goframework.io/graphics-paint2d"
)

type eqPane struct {
	widget.Base
	p        *Player
	on, auto *widgets.Checkbox
	presets  *widgets.Button
	preamp   *players.Fader
	bands    []*players.Fader
}

func newEqPane(p *Player) *eqPane {
	e := &eqPane{p: p}
	e.Init(e)
	e.SetManagesChildren(true)
	e.on = widgets.NewCheckbox("On", p.Equaliser.On, p.Equaliser.SetOn)
	e.on.SetAccessibleName("Equaliser on")
	e.auto = widgets.NewCheckbox("Auto", p.Equaliser.Auto, func(on bool) { p.Equaliser.SetAuto(on, p.Transport.List.Index()); p.Refresh() })
	e.auto.SetAccessibleName("Auto: a preset per track")
	e.presets = widgets.NewButton("Presets", e.presetMenu)
	e.preamp = players.NewFader(-players.EqRange, players.EqRange, p.Equaliser.Preamp, "Preamp", func(v float32) { p.Equaliser.SetPreamp(v) })
	e.preamp.Format = players.Decibels
	for i, name := range players.EqBands {
		band := i
		f := players.NewFader(-players.EqRange, players.EqRange, p.Equaliser.Gains[i], name+" Hz", func(v float32) { p.Equaliser.Set(band, v) })
		f.Format = players.Decibels
		e.bands = append(e.bands, f)
	}
	for _, c := range []widget.Component{e.on, e.auto, e.presets, e.preamp} {
		e.Add(c)
	}
	for _, f := range e.bands {
		e.Add(f)
	}
	return e
}
func (e *eqPane) presetMenu() {
	var items []*widgets.MenuItem
	for _, preset := range players.EqPresets() {
		name := preset.Name
		items = append(items, widgets.RadioItem(name, "preset", name == e.p.Equaliser.Preset, func() { e.p.Equaliser.Apply(name) }))
	}
	o := widget.DeviceOrigin(e.presets)
	widgets.ShowContextMenu(e.presets, paint2d.Pt(o.X, o.Y+e.presets.Bounds().Dy()), items...)
}
func (e *eqPane) sync() {
	available := e.p.Host.EqualizerAvailable()
	tip := e.p.Host.FeatureTip("EQ", available)
	for _, c := range []widget.Component{e.on, e.auto, e.presets, e.preamp} {
		c.SetEnabled(available)
	}
	e.on.SetAccessibleDescription(tip)
	e.auto.SetAccessibleDescription(tip)
	e.presets.Tip, e.preamp.Tip = tip, tip
	e.on.Checked = e.p.Equaliser.On
	e.auto.Checked = e.p.Equaliser.Auto
	e.on.Invalidate()
	e.auto.Invalidate()
	e.preamp.Set(e.p.Equaliser.Preamp)
	for i, f := range e.bands {
		f.SetEnabled(available)
		f.Tip = tip
		f.Set(e.p.Equaliser.Gains[i])
	}
	e.Invalidate()
}
func (e *eqPane) Measure(c layout.Constraints) paint2d.Point {
	return c.Constrain(paint2d.Pt(style.Dip(e.Look(), 440), style.Dip(e.Look(), 188)))
}
func (e *eqPane) Arrange(r paint2d.Rect) {
	e.SetBounds(r)
	b, lk := e.LocalBounds(), e.Look()
	d := func(v float32) float32 { return style.Dip(lk, v) }
	h := max(lk.Metrics().ControlH, d(26))
	y := d(22)
	x := float32(0)
	for _, c := range []widget.Component{e.on, e.auto, e.presets} {
		w := c.Measure(layout.Loose(b.Dx(), h)).X
		c.Arrange(paint2d.XYWH(x, y, w, h))
		x += w + d(10)
	}
	top := y + h + d(6)
	height := max(b.Dy()-top-d(18), d(32))
	each := (b.Dx() - d(12)) / float32(len(e.bands)+1)
	e.preamp.Arrange(paint2d.XYWH(0, top, each, height))
	for i, f := range e.bands {
		f.Arrange(paint2d.XYWH(each+d(12)+float32(i)*each, top, each, height))
	}
}
func (e *eqPane) Paint(ctx *paint2d.Context) {
	lk, b := e.Look(), e.LocalBounds()
	pal := lk.Palette()
	d := func(v float32) float32 { return style.Dip(lk, v) }
	title := "Equaliser · " + e.p.Equaliser.Preset
	if !e.p.Host.EqualizerAvailable() {
		title = "Equaliser · unavailable with " + e.p.Host.AudioBackend
	}
	lk.DrawLabel(ctx, paint2d.XYWH(0, 0, b.Dx(), d(20)), title, pal.Text, style.AlignStart)
	font := players.ScaledFace(lk.Font(), d(10))
	for i, f := range append([]*players.Fader{e.preamp}, e.bands...) {
		label := "Preamp"
		if i > 0 {
			label = players.EqBands[i-1]
		}
		r := f.Bounds()
		players.DrawTextIn(ctx, font, paint2d.XYWH(r.Min.X, r.Max.Y+d(2), r.Dx(), d(14)), label, pal.TextMuted, style.AlignCenter)
	}
}
func (e *eqPane) Describe(n *a11y.Node) { n.Role, n.Name = a11y.RoleGroup, "Equaliser" }
