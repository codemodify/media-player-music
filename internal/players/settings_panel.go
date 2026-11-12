package players

import (
	"fmt"

	"goframework.io/desktop-ui-widgets"
	"goframework.io/desktop-ui/a11y"
	"goframework.io/desktop-ui/layout"
	"goframework.io/desktop-ui/platform"
	"goframework.io/desktop-ui/style"
	"goframework.io/desktop-ui/widget"
	"goframework.io/graphics-paint2d"
)

const (
	SettingsWidth  = 520
	SettingsHeight = 480
	// The smallest the settings window is made: everything still fits.
	SettingsMinWidth  = 460
	SettingsMinHeight = 460
)

// SettingsState is what the settings window shows: the formats the player
// plays, the ReplayGain settings and an error to report.
type SettingsState struct {
	Formats    string
	ReplayGain ReplayGain
	Error      string
}

// replayGainModes are the mode radios' order.
var replayGainModes = []string{ReplayGainOff, ReplayGainTrack, ReplayGainAlbum}

// SettingsPanel is the player's settings: what it plays, and ReplayGain's
// mode and preamp. Its owner is told each change the user makes.
type SettingsPanel struct {
	widget.Base
	OnClose func()
	state   SettingsState
	change  func(ReplayGain)

	title, formats, gainTitle, preampLabel, hint, status *widgets.Label
	modes                                                *widgets.RadioGroup
	preamp                                               *widgets.Slider
}

func NewSettings(state SettingsState, change func(ReplayGain)) *SettingsPanel {
	p := &SettingsPanel{change: change}
	p.Init(p)
	p.SetManagesChildren(true)
	p.title = widgets.NewTitle("Playback")
	p.formats = widgets.NewLabel("")
	p.formats.Wrap = true
	p.gainTitle = widgets.NewTitle("ReplayGain")
	p.modes = widgets.NewRadioGroup([]string{"Off", "Track gain", "Album gain"}, 0, nil)
	p.modes.OnInput = func(i int) { p.edited() }
	p.preampLabel = widgets.NewLabel("")
	p.preamp = widgets.NewSlider(MinPreamp, MaxPreamp, 0, nil)
	p.preamp.Step, p.preamp.Page, p.preamp.Label = 0.5, 3, "Preamp"
	p.preamp.OnInput = func(float32) { p.edited() }
	p.hint = widgets.NewLabel("Tracks are levelled by their ReplayGain tags (ID3, APE, Vorbis comments, MP4). " +
		"A track without the chosen gain takes the other one, and its peak keeps the gain from clipping. " +
		"A change is heard at once, on the track playing too.")
	p.hint.Wrap = true
	p.status = widgets.NewLabel("")
	p.status.Wrap = true
	for _, c := range []widget.Component{p.title, p.formats, p.gainTitle, p.modes, p.preampLabel, p.preamp, p.hint, p.status} {
		p.Add(c)
	}
	p.SetState(state)
	return p
}

// edited reports the controls' settings to the owner.
func (p *SettingsPanel) edited() {
	gain := ReplayGain{Mode: replayGainModes[max(0, p.modes.Selected())], Preamp: float64(p.preamp.Value)}.Normal()
	if gain == p.state.ReplayGain {
		return
	}
	p.state.ReplayGain = gain
	p.preampLabel.SetText(preampText(gain.Preamp))
	if p.change != nil {
		p.change(gain)
	}
}

// State is what the panel shows.
func (p *SettingsPanel) State() SettingsState { return p.state }

// SetState must run on the UI thread, like the controls it updates.
func (p *SettingsPanel) SetState(state SettingsState) {
	state.ReplayGain = state.ReplayGain.Normal()
	p.state = state
	p.formats.SetText("Plays " + state.Formats + ". WMA, Musepack and WavPack files are not supported.")
	for i, mode := range replayGainModes {
		if mode == state.ReplayGain.Mode && p.modes.Selected() != i {
			p.modes.Select(i)
		}
	}
	p.preamp.SetValue(float32(state.ReplayGain.Preamp))
	p.preampLabel.SetText(preampText(state.ReplayGain.Preamp))
	p.status.SetText(state.Error)
	p.RequestLayout()
	p.Invalidate()
}

func preampText(db float64) string { return fmt.Sprintf("Preamp: %+.1f dB", db) }

func (p *SettingsPanel) Measure(c layout.Constraints) paint2d.Point {
	return c.Constrain(paint2d.Pt(style.Dip(p.Look(), SettingsWidth), style.Dip(p.Look(), SettingsHeight)))
}

func (p *SettingsPanel) Arrange(r paint2d.Rect) {
	p.SetBounds(r)
	b, lk := p.LocalBounds(), p.Look()
	d := func(v float32) float32 { return style.Dip(lk, v) }
	x, w, y := d(16), max(0, b.Dx()-d(32)), d(12)
	text := func(l *widgets.Label, gap float32) {
		h := l.Measure(layout.Loose(w, b.Dy())).Y
		l.Arrange(paint2d.XYWH(x, y, w, h))
		y += h + gap
	}
	text(p.title, d(4))
	text(p.formats, d(14))
	text(p.gainTitle, d(4))
	modesH := p.modes.Measure(layout.Loose(w, b.Dy())).Y
	p.modes.Arrange(paint2d.XYWH(x, y, w, modesH))
	y += modesH + d(10)
	controlH := max(lk.Metrics().ControlH, d(24))
	labelW := d(130)
	p.preampLabel.Arrange(paint2d.XYWH(x, y, labelW, controlH))
	p.preamp.Arrange(paint2d.XYWH(x+labelW, y, max(0, w-labelW), controlH))
	y += controlH + d(10)
	text(p.hint, d(8))
	p.status.Arrange(paint2d.XYWH(x, y, w, max(0, b.Dy()-y-d(8))))
}

func (p *SettingsPanel) Paint(ctx *paint2d.Context) {
	ctx.DrawRect(p.LocalBounds(), paint2d.Fill(p.Look().Palette().Background))
}

func (p *SettingsPanel) Describe(n *a11y.Node) {
	n.Role, n.Name = a11y.RoleGroup, "Playback settings"
}

func (p *SettingsPanel) KeyPress(e widget.KeyEvent) bool {
	if e.Key == platform.KeyEscape && p.OnClose != nil {
		p.OnClose()
		return true
	}
	return false
}
