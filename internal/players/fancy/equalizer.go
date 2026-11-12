package fancy

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
	bands   []*fancySlider
	preamp  *fancySlider
}

func newEqualizerPane(p *Player) *equalizerPane {
	b := &equalizerPane{p: p, slots: skins.NewSlots(LayoutEqualizer), buttons: make(map[string]*players.GlyphButton)}
	b.Init(b)
	b.SetManagesChildren(true)
	b.SetAccessibleName("Fancy equalizer")
	add := func(slot, label, text string, action func()) *players.GlyphButton {
		c := button(LayoutEqualizer, slot, label, text, players.GlyphNone, action)
		b.buttons[slot] = c
		b.slots.Bind(slot, c)
		b.Add(c)
		return c
	}
	add("close", "Close equalizer", "", func() { p.ShowEqualiser(false) })
	add("on", "Equalizer on", "ON", func() { p.Equaliser.SetOn(!p.Equaliser.On) }).Toggle = true
	add("auto", "Auto preset per track", "AUTO", func() { p.Equaliser.SetAuto(!p.Equaliser.Auto, p.Transport.List.Index()) }).Toggle = true
	add("load", "Load equalizer settings", "LOAD", func() { p.loadEqualizer(b.buttons["load"]) })
	add("save", "Save equalizer settings", "SAVE", func() { p.saveEqualizer(b.buttons["save"]) })
	add("reset", "Reset equalizer", "RESET", p.Equaliser.Flat)
	add("presets", "Equalizer presets", "PRESETS", b.presetMenu)
	b.preamp = slider(true, false, -players.EqRange, players.EqRange, p.Equaliser.Preamp, "Preamp", p.Equaliser.SetPreamp)
	b.preamp.Format = players.Decibels
	b.slots.Bind("preamp", b.preamp)
	b.Add(b.preamp)
	for i, label := range players.EqBands {
		band := i
		s := slider(true, false, -players.EqRange, players.EqRange, p.Equaliser.Gains[i], label+" Hz", func(v float32) { p.Equaliser.Set(band, v) })
		s.Format = players.Decibels
		b.bands = append(b.bands, s)
		b.slots.Bind(fmt.Sprintf("band.%d", i), s)
		b.Add(s)
	}
	return b
}
func (b *equalizerPane) presetMenu() {
	c := b.buttons["presets"]
	eq := b.p.Equaliser
	items := []*widgets.MenuItem{widgets.Item("Flat", eq.Flat), widgets.Sep()}
	for _, preset := range players.EqPresets() {
		name := preset.Name
		items = append(items, widgets.RadioItem(name, "preset", eq.Preset == name, func() { eq.Apply(name) }))
	}
	widgets.ShowContextMenu(c, widget.DeviceOrigin(c).Add(paint2d.Pt(0, c.Bounds().Dy())), items...)
}
func (b *equalizerPane) sync() {
	eq := b.p.Equaliser
	b.buttons["on"].SetChecked(eq.On)
	b.buttons["auto"].SetChecked(eq.Auto)
	b.preamp.Set(eq.Preamp)
	for i, s := range b.bands {
		s.Set(eq.Gains[i])
	}
	if w := b.p.Eq(); w != nil && w.Focus() != nil && !w.Focus().Enabled() {
		w.RequestFocus(b.buttons["close"])
	}
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
	paintLayout(ctx, lk, box, LayoutEqualizer, b.p.Eq())
	textSlot(ctx, lk, box, LayoutEqualizer, "header", "Equalizer", 9, ink(), style.AlignStart)
	for i, name := range append([]string{"PRE"}, players.EqBands...) {
		slot := "preamp"
		if i > 0 {
			slot = fmt.Sprintf("band.%d", i-1)
		}
		if r, ok := skins.SkinSlotRect(lk, LayoutEqualizer, slot, box); ok {
			label := paint2d.XYWH(r.Center().X-style.Dip(lk, 11), r.Max.Y+style.Dip(lk, 1), style.Dip(lk, 22), style.Dip(lk, 10))
			players.DrawTextIn(ctx, face(lk, 7), label, name, ink(), style.AlignCenter)
		}
	}
	if r, ok := skins.SkinSlotRect(lk, LayoutEqualizer, "status", box); ok {
		text := "EQ off"
		if b.p.Equaliser.On {
			text = b.p.Equaliser.Preset
		}
		r.Min.Y = r.Max.Y - style.Dip(lk, 12)
		players.DrawTextIn(ctx, face(lk, 7), r, text, cyan(), style.AlignCenter)
	}
}
func (b *equalizerPane) MousePress(e widget.MouseEvent) bool {
	if e.Button == platform.ButtonRight {
		b.p.Host.ContextMenu(b, e.Pos.Add(widget.DeviceOrigin(b)))
		return true
	}
	if e.Button == platform.ButtonLeft {
		return b.p.g.StartMove(b.p.iEq)
	}
	return false
}
func (b *equalizerPane) CaptionAt(paint2d.Point) bool    { return true }
func (b *equalizerPane) KeyPress(e widget.KeyEvent) bool { return b.p.keys(b.p.Eq(), e) }
func (b *equalizerPane) Describe(n *a11y.Node)           { n.Role, n.Name = a11y.RoleGroup, "Fancy equalizer" }
