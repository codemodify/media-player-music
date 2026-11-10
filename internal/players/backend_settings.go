package players

import (
	"strings"

	"goframework.io/desktop-ui-widgets"
	"goframework.io/desktop-ui/a11y"
	"goframework.io/desktop-ui/layout"
	"goframework.io/desktop-ui/platform"
	"goframework.io/desktop-ui/style"
	"goframework.io/desktop-ui/widget"
	"goframework.io/graphics-paint2d"
)

const (
	BackendSettingsWidth  = 600
	BackendSettingsHeight = 470
)

// BackendInfo describes installed playback programs without making the UI
// depend on their process or protocol implementations.
type BackendInfo struct {
	ID, Name, Executable           string
	Available                      bool
	Equalizer, Balance, Visualizer bool
}

type BackendSettingsState struct {
	Backends []BackendInfo
	Selected string
	Busy     bool
	Error    string
}

// BackendSettingsPanel uses ordinary radio buttons for selection. The green
// check and red cross report installation separately from the selected radio.
// Its owner supplies new state after choosing a program or rescanning PATH.
type BackendSettingsPanel struct {
	widget.Base
	OnClose             func()
	state               BackendSettingsState
	choose              func(string)
	recheck             func()
	title, hint, status *widgets.Label
	refresh             *widgets.Button
	rows                []*backendSettingsRow
}

func NewBackendSettings(state BackendSettingsState, choose func(string), recheck func()) *BackendSettingsPanel {
	p := &BackendSettingsPanel{choose: choose, recheck: recheck}
	p.Init(p)
	p.SetManagesChildren(true)
	p.title = widgets.NewTitle("Audio backend")
	p.hint = widgets.NewLabel("Green check: installed. Red cross: missing. Pick an available backend.")
	p.hint.Wrap = true
	p.status = widgets.NewLabel("")
	p.status.Wrap = true
	p.refresh = widgets.NewButton("Refresh", func() {
		if !p.state.Busy && p.recheck != nil {
			p.recheck()
		}
	})
	p.refresh.Tip = "Check installed playback programs again"
	for _, c := range []widget.Component{p.title, p.hint, p.status, p.refresh} {
		p.Add(c)
	}
	p.SetState(state)
	return p
}

// SetState must run on the UI thread, like the controls it updates.
func (p *BackendSettingsPanel) SetState(state BackendSettingsState) {
	state.Backends = append([]BackendInfo(nil), state.Backends...)
	rebuild := len(p.rows) != len(state.Backends)
	if !rebuild {
		for i, info := range state.Backends {
			if p.rows[i].info.ID != info.ID {
				rebuild = true
				break
			}
		}
	}
	p.state = state
	if rebuild {
		for _, r := range p.rows {
			p.Remove(r)
		}
		p.rows = nil
		for _, info := range state.Backends {
			r := newBackendSettingsRow(info, func(id string) {
				if p.state.Busy || (id == p.state.Selected && p.state.Error == "") || p.choose == nil {
					return
				}
				p.choose(id)
				// Selection describes the active engine, not a pending or
				// rejected request. The owner can update it synchronously.
				p.SetState(p.state)
			})
			p.rows = append(p.rows, r)
			p.Add(r)
		}
	}
	for i, info := range state.Backends {
		p.rows[i].setInfo(info, info.ID == state.Selected, state.Busy)
	}
	p.refresh.SetEnabled(!state.Busy)
	message := "Selection is saved for the next launch."
	if state.Selected == "mpg123" {
		message = "MP3/MPEG only. MP4/AAC/FLAC need another backend."
	}
	if state.Selected != "" {
		for _, info := range state.Backends {
			if info.ID == state.Selected && !info.Available {
				message = info.Name + " is selected but unavailable. Install it or choose another backend."
				if info.ID == "mpg123" {
					message = "mpg123 is missing (MP3/MPEG only). Choose another backend."
				}
			}
		}
	}
	if state.Busy {
		message = "Switching audio backend…"
	}
	if state.Error != "" {
		message = state.Error
	}
	p.status.SetText(message)
	p.RequestLayout()
	p.Invalidate()
}

func (p *BackendSettingsPanel) Measure(c layout.Constraints) paint2d.Point {
	return c.Constrain(paint2d.Pt(style.Dip(p.Look(), BackendSettingsWidth), style.Dip(p.Look(), BackendSettingsHeight)))
}

func (p *BackendSettingsPanel) Arrange(r paint2d.Rect) {
	p.SetBounds(r)
	b, lk := p.LocalBounds(), p.Look()
	d := func(v float32) float32 { return style.Dip(lk, v) }
	x, w, y := d(16), max(0, b.Dx()-d(32)), d(12)
	p.title.Arrange(paint2d.XYWH(x, y, w, d(26)))
	y += d(28)
	hintH := p.hint.Measure(layout.Loose(w, d(42))).Y
	p.hint.Arrange(paint2d.XYWH(x, y, w, hintH))
	y += hintH + d(10)
	actionH := max(lk.Metrics().ControlH, d(28))
	footerH := max(actionH, d(48))
	rowH := d(76)
	if len(p.rows) > 0 {
		rowH = min(rowH, max(0, b.Dy()-y-footerH-d(18))/float32(len(p.rows)))
	}
	for _, row := range p.rows {
		row.Arrange(paint2d.XYWH(x, y, w, rowH))
		y += rowH
	}
	actionY := max(y+d(6), b.Dy()-footerH-d(12))
	p.refresh.Arrange(paint2d.XYWH(x+w-d(100), actionY+(footerH-actionH)/2, d(100), actionH))
	p.status.Arrange(paint2d.XYWH(x, actionY, max(0, w-d(116)), footerH))
}

func (p *BackendSettingsPanel) Paint(ctx *paint2d.Context) {
	ctx.DrawRect(p.LocalBounds(), paint2d.Fill(p.Look().Palette().Background))
}

func (p *BackendSettingsPanel) Describe(n *a11y.Node) {
	n.Role, n.Name = a11y.RoleGroup, "Audio backend settings"
}

func (p *BackendSettingsPanel) KeyPress(e widget.KeyEvent) bool {
	if e.Key == platform.KeyEscape && p.OnClose != nil {
		p.OnClose()
		return true
	}
	return false
}

type backendSettingsRow struct {
	widget.Base
	info                            BackendInfo
	radio                           *backendChoiceRadio
	availability                    *backendAvailabilityMark
	installed, features, executable *widgets.Label
}

func newBackendSettingsRow(info BackendInfo, choose func(string)) *backendSettingsRow {
	r := &backendSettingsRow{info: info}
	r.Init(r)
	r.SetManagesChildren(true)
	r.radio = &backendChoiceRadio{RadioButton: *widgets.NewRadio(info.Name, false, nil)}
	r.radio.Init(r.radio)
	r.radio.OnRepeat = func() {
		if r.info.Available {
			choose(r.info.ID)
		}
	}
	r.radio.OnInput = func(on bool) {
		if on && r.info.Available {
			choose(r.info.ID)
		}
	}
	r.availability = &backendAvailabilityMark{}
	r.availability.Init(r.availability)
	r.installed = widgets.NewLabel("")
	r.features = widgets.NewLabel("")
	r.executable = widgets.NewLabel("")
	for _, c := range []widget.Component{r.radio, r.availability, r.installed, r.features, r.executable} {
		r.Add(c)
	}
	return r
}

func (r *backendSettingsRow) setInfo(info BackendInfo, selected, busy bool) {
	r.info = info
	r.radio.Text = info.Name
	if info.ID == "mpg123" {
		r.radio.Text += " (MP3/MPEG only)"
	}
	r.radio.SetSelected(selected)
	r.radio.SetEnabled(info.Available && !busy)
	r.availability.available = info.Available
	r.availability.Invalidate()
	installed := "Unavailable"
	path := "Not found on PATH"
	if info.Available {
		installed, path = "Available", info.Executable
	}
	r.installed.SetText(installed)
	r.executable.SetText(path)
	var features []string
	for _, feature := range []struct {
		name string
		on   bool
	}{
		{"EQ", info.Equalizer}, {"Balance", info.Balance}, {"Visualizer", info.Visualizer},
	} {
		availability := "available"
		if !feature.on || !info.Available {
			availability = "unavailable"
		}
		features = append(features, feature.name+" "+availability)
	}
	r.features.SetText(strings.Join(features, " · "))
	r.Invalidate()
}

func (r *backendSettingsRow) Measure(c layout.Constraints) paint2d.Point {
	return c.Constrain(paint2d.Pt(style.Dip(r.Look(), 568), style.Dip(r.Look(), 76)))
}

func (r *backendSettingsRow) Arrange(b paint2d.Rect) {
	r.SetBounds(b)
	w, lk := r.LocalBounds().Dx(), r.Look()
	d := func(v float32) float32 { return style.Dip(lk, v) }
	h := max(lk.Metrics().ControlH, d(26))
	r.radio.Arrange(paint2d.XYWH(0, 0, max(0, w-d(148)), h))
	r.availability.Arrange(paint2d.XYWH(w-d(140), 0, d(20), h))
	r.installed.Arrange(paint2d.XYWH(w-d(116), 0, d(116), h))
	r.features.Arrange(paint2d.XYWH(d(4), h+d(2), w-d(4), d(19)))
	r.executable.Arrange(paint2d.XYWH(d(4), h+d(21), w-d(4), d(18)))
}

func (r *backendSettingsRow) Describe(n *a11y.Node) {
	n.Role, n.Name = a11y.RoleGroup, r.info.Name
}

// A selected radio normally makes no change. Settings also permits choosing
// that program again after a startup error, using the stock radio's gesture,
// keyboard and accessibility behavior with this repeated-choice hook.
type backendChoiceRadio struct {
	widgets.RadioButton
	OnRepeat      func()
	repeatPressed bool
}

func (r *backendChoiceRadio) MousePress(e widget.MouseEvent) bool {
	if e.Button != platform.ButtonLeft {
		return false
	}
	r.repeatPressed = r.Selected && r.Enabled()
	return r.RadioButton.MousePress(e)
}

func (r *backendChoiceRadio) MouseRelease(e widget.MouseEvent) bool {
	repeat := r.repeatPressed && e.Button == platform.ButtonLeft && r.Enabled() && r.LocalBounds().Contains(e.Pos)
	r.repeatPressed = false
	handled := r.RadioButton.MouseRelease(e)
	if repeat && r.OnRepeat != nil {
		r.OnRepeat()
	}
	return handled
}

func (r *backendChoiceRadio) MouseExit() { r.repeatPressed = false; r.RadioButton.MouseExit() }
func (r *backendChoiceRadio) FocusLost() { r.repeatPressed = false; r.RadioButton.FocusLost() }
func (r *backendChoiceRadio) KeyPress(e widget.KeyEvent) bool {
	r.repeatPressed = false
	repeat := r.Selected && r.Enabled() && (e.Key == platform.KeyReturn || e.Key == platform.KeySpace)
	handled := r.RadioButton.KeyPress(e)
	if repeat && handled && r.OnRepeat != nil {
		r.OnRepeat()
	}
	return handled
}

func (r *backendChoiceRadio) AccessibleAction(item int, action a11y.Action) bool {
	repeat := r.Selected && r.Enabled() && action == a11y.ActionDefault
	handled := r.RadioButton.AccessibleAction(item, action)
	if repeat && handled && r.OnRepeat != nil {
		r.OnRepeat()
	}
	return handled
}

type backendAvailabilityMark struct {
	widget.Base
	available bool
}

func (m *backendAvailabilityMark) Measure(c layout.Constraints) paint2d.Point {
	return c.Constrain(paint2d.Pt(style.Dip(m.Look(), 20), style.Dip(m.Look(), 26)))
}

func (m *backendAvailabilityMark) Arrange(r paint2d.Rect) { m.SetBounds(r) }

func (m *backendAvailabilityMark) Paint(ctx *paint2d.Context) {
	b := m.LocalBounds()
	s := min(b.Dx(), b.Dy()) * .7
	x, y := b.Center().X-s/2, b.Center().Y-s/2
	color := paint2d.RGB(.95, .18, .2)
	if m.available {
		color = paint2d.RGB(.12, .78, .32)
	}
	stroke := paint2d.StrokePaint(color, style.Dip(m.Look(), 2))
	stroke.Stroke.Cap, stroke.Stroke.Join = paint2d.CapRound, paint2d.JoinRound
	p := paint2d.NewPath()
	if m.available {
		p.MoveTo(x, y+s*.5)
		p.LineTo(x+s*.35, y+s*.85)
		p.LineTo(x+s, y+s*.12)
	} else {
		p.MoveTo(x, y)
		p.LineTo(x+s, y+s)
		p.MoveTo(x+s, y)
		p.LineTo(x, y+s)
	}
	ctx.DrawPath(p, stroke)
}

func (m *backendAvailabilityMark) Describe(n *a11y.Node) {
	n.Role, n.Name = a11y.RoleImage, "Unavailable"
	if m.available {
		n.Name = "Available"
	}
}
