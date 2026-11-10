package standard

import (
	"fmt"
	"time"

	"github.com/codemodify/media-player-music/internal/players"
	"goframework.io/desktop-ui-widgets"
	"goframework.io/desktop-ui/a11y"
	"goframework.io/desktop-ui/layout"
	"goframework.io/desktop-ui/platform"
	"goframework.io/desktop-ui/style"
	"goframework.io/desktop-ui/widget"
	"goframework.io/graphics-paint2d"
)

type body struct {
	widget.Base
	p      *Player
	status *widgets.StatusBar
	main   *mainPane
	eq     *eqPane
	queue  *queuePane
}

func newBody(p *Player) *body {
	b := &body{p: p}
	b.Init(b)
	b.SetManagesChildren(true)
	b.main = newMainPane(p)
	b.eq = newEqPane(p)
	b.queue = newQueuePane(p)
	b.status = widgets.NewStatusBar(p.Host.StatusText(), "")
	for _, c := range []widget.Component{b.main, b.eq, b.queue, b.status} {
		b.Add(c)
	}
	return b
}

func (b *body) sync() {
	b.main.sync()
	b.eq.sync()
	b.queue.sync()
	b.status.SetParts(b.p.Host.StatusText(), fmt.Sprintf("%s · %s", b.p.Transport.State, b.p.Host.PackLabel(b.p.Host.Worn())))
	b.Invalidate()
}
func (b *body) Measure(c layout.Constraints) paint2d.Point {
	return c.Constrain(paint2d.Pt(style.Dip(b.Look(), MainW), style.Dip(b.Look(), MainH)))
}
func (b *body) Arrange(r paint2d.Rect) {
	b.SetBounds(r)
	box, lk := b.LocalBounds(), b.Look()
	d := func(v float32) float32 { return style.Dip(lk, v) }
	statusH := b.status.Measure(layout.Loose(box.Dx(), box.Dy())).Y
	b.status.Arrange(paint2d.XYWH(0, box.Dy()-statusH, box.Dx(), statusH))
	x, y, w := d(12), d(8), box.Dx()-d(24)
	available := max(box.Dy()-statusH-y-d(8), 0)
	left := w
	if b.queue.Visible() {
		right := min(d(320), w*.39)
		left = w - right - d(12)
		b.queue.Arrange(paint2d.XYWH(x+left+d(12), y, right, available))
	} else {
		b.queue.Arrange(paint2d.Rect{})
	}
	mainH := available
	eqH := float32(0)
	if b.eq.Visible() {
		eqH = min(d(188), available*.42)
		mainH -= eqH + d(8)
	}
	b.main.Arrange(paint2d.XYWH(x, y, left, mainH))
	y += mainH + d(8)
	if b.eq.Visible() {
		b.eq.Arrange(paint2d.XYWH(x, y, left, eqH))
	} else {
		b.eq.Arrange(paint2d.Rect{})
	}
}
func (b *body) KeyPress(e widget.KeyEvent) bool { return b.p.keys(e) }
func (b *body) MousePress(e widget.MouseEvent) bool {
	if e.Button != platform.ButtonRight {
		return false
	}
	o := widget.DeviceOrigin(b)
	b.p.Host.ContextMenu(b, paint2d.Pt(o.X+e.Pos.X, o.Y+e.Pos.Y))
	return true
}
func (b *body) Describe(n *a11y.Node) {
	n.Role, n.Name, n.Description = a11y.RolePane, "Music player", b.p.Host.StatusText()
}

type mainPane struct {
	widget.Base
	p                                   *Player
	display                             *display
	analyser                            *players.AnalyserView
	transport                           [6]*players.GlyphButton
	seek, volume, balance               *players.Fader
	shuffle, repeat, eq, playlist, skin *players.GlyphButton
	settings                            *widgets.ToolButton
}

func newMainPane(p *Player) *mainPane {
	m := &mainPane{p: p}
	m.Init(m)
	m.SetManagesChildren(true)
	m.display = &display{p: p}
	m.display.Init(m.display)
	m.analyser = players.NewAnalyserView(p.Spectrum)
	m.analyser.Peaks = true
	m.transport = p.transportButtons()
	m.seek = players.NewBar(0, 1, 0, "Seek", p.Host.Seek)
	m.seek.Step, m.seek.Page = .02, .1
	m.seek.Format = func(v float32) string {
		return players.Clock(time.Duration(float64(p.Transport.Length()) * float64(v)))
	}
	m.volume = players.NewBar(0, 100, p.Transport.Volume*100, "Volume", func(v float32) { p.Host.SetVolume(v / 100) })
	m.volume.Format = func(v float32) string { return fmt.Sprintf("%d%%", int(v+.5)) }
	m.balance = players.NewBar(-1, 1, p.Transport.Pan, "Balance", p.Host.SetPan)
	m.balance.Step = .1
	m.shuffle = players.NewGlyphButton(players.GlyphShuffle, "Shuffle", func() { p.Host.Command(players.CmdShuffle) })
	m.shuffle.Toggle = true
	m.repeat = players.NewGlyphButton(players.GlyphRepeat, "Repeat", func() { p.Host.Command(players.CmdRepeat) })
	m.repeat.Toggle = true
	m.eq = players.NewGlyphButton(players.GlyphSliders, "Equaliser", func() { p.ShowEqualiser(!p.EqualiserShown()) })
	m.eq.Toggle = true
	m.playlist = players.NewGlyphButton(players.GlyphList, "Playlist", func() { p.ShowPlaylist(!p.PlaylistShown()) })
	m.playlist.Toggle = true
	m.skin = players.NewGlyphButton(players.GlyphSkin, "Skins", func() {
		o := widget.DeviceOrigin(m.skin)
		p.Host.ContextMenu(m.skin, paint2d.Pt(o.X, o.Y+m.skin.Bounds().Dy()))
	})
	m.settings = widgets.NewToolButton("", style.IconSettings, p.Host.OpenSettings)
	m.settings.SetAccessibleName("Settings")
	m.settings.Tip = "Settings (Ctrl+,)"
	m.Add(m.display)
	m.Add(m.analyser)
	m.Add(m.seek)
	for _, c := range m.transport {
		m.Add(c)
	}
	for _, c := range []widget.Component{m.volume, m.balance, m.shuffle, m.repeat, m.eq, m.playlist, m.skin, m.settings} {
		m.Add(c)
	}
	return m
}
func (m *mainPane) sync() {
	t := m.p.Transport
	if !m.seek.Dragging() {
		m.seek.Set(t.Fraction())
	}
	m.volume.Set(t.Volume * 100)
	m.balance.Set(t.Pan)
	m.balance.SetEnabled(m.p.Host.BalanceAvailable())
	m.balance.Tip = m.p.Host.FeatureTip("Balance", m.p.Host.BalanceAvailable())
	m.analyser.SetAccessibleDescription(m.p.Host.FeatureTip("Visualizer", m.p.Host.SpectrumAvailable()))
	m.shuffle.SetChecked(t.Random)
	m.repeat.SetChecked(t.Repeat != players.RepeatOff)
	m.repeat.SetGlyph(players.GlyphRepeat, t.Repeat.String())
	if t.Repeat == players.RepeatOne {
		m.repeat.SetGlyph(players.GlyphRepeatOne, t.Repeat.String())
	}
	m.eq.SetChecked(m.p.EqualiserShown())
	m.playlist.SetChecked(m.p.PlaylistShown())
	m.display.Invalidate()
	m.analyser.Invalidate()
	m.Invalidate()
}
func (m *mainPane) Measure(c layout.Constraints) paint2d.Point {
	return c.Constrain(paint2d.Pt(style.Dip(m.Look(), MinW-24), style.Dip(m.Look(), 226)))
}
func (m *mainPane) Arrange(r paint2d.Rect) {
	m.SetBounds(r)
	b, lk := m.LocalBounds(), m.Look()
	d := func(v float32) float32 { return style.Dip(lk, v) }
	displayH := max(b.Dy()-d(100), d(100))
	m.display.Arrange(paint2d.XYWH(0, 0, b.Dx(), displayH))
	m.analyser.Arrange(paint2d.XYWH(d(16), displayH-d(38), b.Dx()-d(32), d(26)))
	y := displayH + d(4)
	m.seek.Arrange(paint2d.XYWH(d(58), y, b.Dx()-d(116), d(20)))
	y += d(26)
	x := float32(0)
	button := d(36)
	for _, c := range m.transport {
		c.Arrange(paint2d.XYWH(x, y, button, button))
		x += button + d(4)
	}
	x += d(10)
	for _, c := range []*players.GlyphButton{m.shuffle, m.repeat} {
		c.Arrange(paint2d.XYWH(x, y, d(32), d(36)))
		x += d(36)
	}
	x = b.Dx() - d(32)
	for _, c := range []*players.GlyphButton{m.skin, m.playlist, m.eq} {
		c.Arrange(paint2d.XYWH(x, y, d(32), d(36)))
		x -= d(36)
	}
	y += d(40)
	half := (b.Dx() - d(24)) / 2
	label := m.levelLabelWidth()
	m.volume.Arrange(paint2d.XYWH(label, y, half-label, d(22)))
	m.balance.Arrange(paint2d.XYWH(half+d(24)+label, y, half-label-d(40), d(22)))
	m.settings.Arrange(paint2d.XYWH(b.Dx()-d(32), y-d(4), d(32), d(30)))
}
func (m *mainPane) Paint(ctx *paint2d.Context) {
	lk := m.Look()
	pal := lk.Palette()
	d := func(v float32) float32 { return style.Dip(lk, v) }
	s := m.seek.Bounds()
	font := players.ScaledFace(lk.MonoFont(), d(12))
	players.DrawTextIn(ctx, font, paint2d.XYWH(0, s.Min.Y, d(50), s.Dy()), players.Clock(m.p.Transport.Pos), pal.Text, style.AlignStart)
	players.DrawTextIn(ctx, font, paint2d.XYWH(s.Max.X+d(8), s.Min.Y, d(50), s.Dy()), players.Countdown(m.p.Transport.Remaining()), pal.TextMuted, style.AlignEnd)
	for _, row := range []struct {
		bar   *players.Fader
		label string
	}{{m.volume, "Volume"}, {m.balance, "Balance"}} {
		r := row.bar.Bounds()
		label := m.levelLabelWidth()
		lk.DrawLabel(ctx, paint2d.XYWH(r.Min.X-label, r.Min.Y, label-d(4), r.Dy()), row.label, pal.TextMuted, style.AlignStart)
	}
}
func (m *mainPane) Describe(n *a11y.Node) { n.Role, n.Name = a11y.RoleGroup, "Playback" }

func (m *mainPane) levelLabelWidth() float32 {
	lk := m.Look()
	return max(lk.Font().Advance("Volume"), lk.Font().Advance("Balance")) + style.Dip(lk, 8)
}

type display struct {
	widget.Base
	p *Player
}

func (d *display) Measure(c layout.Constraints) paint2d.Point {
	return c.Constrain(paint2d.Pt(style.Dip(d.Look(), 500), style.Dip(d.Look(), 116)))
}
func (d *display) Arrange(r paint2d.Rect) { d.SetBounds(r) }
func (d *display) Paint(ctx *paint2d.Context) {
	lk, b := d.Look(), d.LocalBounds()
	pal := lk.Palette()
	dip := func(v float32) float32 { return style.Dip(lk, v) }
	ctx.DrawRoundRect(b, dip(6), dip(6), paint2d.Fill(pal.Field))
	ctx.DrawRoundRect(b.Inset(.5), dip(6), dip(6), paint2d.StrokePaint(pal.FieldBorder, max(dip(1), 1)))
	t := d.p.Transport.Track()
	pad := dip(16)
	title, detail := t.Title, t.Artist
	if d.p.Transport.List.Len() == 0 {
		title, detail = "Open a music file to begin", "Use Ctrl+O, Eject or Add, or drop files/folders onto the playlist"
	} else {
		if t.Album != "" {
			if detail != "" {
				detail += " · "
			}
			detail += t.Album
		}
		if t.Rate > 0 {
			if detail != "" {
				detail += " · "
			}
			detail += fmt.Sprintf("%d kHz", t.Rate)
		}
	}
	players.DrawTextIn(ctx, players.ScaledFace(lk.TitleFont(), dip(23)), paint2d.XYWH(pad, dip(9), b.Dx()-2*pad, dip(30)), title, pal.Text, style.AlignStart)
	players.DrawTextIn(ctx, lk.Font(), paint2d.XYWH(pad, dip(42), b.Dx()-2*pad, dip(20)), detail, pal.TextMuted, style.AlignStart)
}
func (d *display) Describe(n *a11y.Node) {
	n.Role, n.Name = a11y.RoleLabel, d.p.Transport.Track().Label()
	if d.p.Transport.List.Len() == 0 {
		n.Name = "Open a music file to begin"
	}
}
