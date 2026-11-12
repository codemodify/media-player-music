package shell

import (
	"github.com/codemodify/media-player-music/internal/players"
	"goframework.io/desktop-ui"
	"goframework.io/desktop-ui-widgets"
	"goframework.io/desktop-ui/a11y"
	"goframework.io/desktop-ui/layout"
	"goframework.io/desktop-ui/platform"
	"goframework.io/desktop-ui/style"
	"goframework.io/desktop-ui/widget"
	"goframework.io/graphics-paint2d"
)

// Starting media-player-musicd (goframework R115's ensure, through
// internal/link): while it starts, every face says "Starting
// media-player-musicd…" where it says the player's errors; when it cannot
// be started, they say why, and a small window over the player says it
// too, with Retry and Close.

// daemonNote is how starting the daemon stands, as the shell shows it.
type daemonNote struct {
	text    string
	problem bool
	retry   func()
	window  *ui.Window
	title   *widgets.Label
	detail  *widgets.Label
	again   *widgets.Button
}

// ShowDaemon shows how starting media-player-musicd stands (link.Daemon's
// Text and Problem): text while it starts, why it could not be started
// with retry (which starts it again), "" once it runs.
func (s *Shell) ShowDaemon(text string, problem bool, retry func()) {
	if s.closed {
		return
	}
	s.note.text, s.note.problem, s.note.retry = text, problem, retry
	if text != "" {
		s.Host.Model.Error = text
	} else {
		s.Host.Model.Error = s.daemon.state.Error
	}
	if problem && retry != nil {
		s.openDaemonNote()
	} else {
		s.closeDaemonNote()
	}
	s.refreshSettings()
	s.refresh()
}

// DaemonNote is what the shell says of starting the daemon, and whether
// its window is open: for the tests.
func (s *Shell) DaemonNote() (text string, problem bool, window *ui.Window) {
	if s.note.window != nil && s.note.window.Closed() {
		s.note.window = nil
	}
	return s.note.text, s.note.problem, s.note.window
}

// RetryDaemon starts the daemon again, as the note's Retry does.
func (s *Shell) RetryDaemon() {
	if retry := s.note.retry; retry != nil {
		s.note.retry = nil
		s.closeDaemonNote()
		retry()
	}
}

// openDaemonNote opens, or fills, the window that says why the daemon
// could not be started.
func (s *Shell) openDaemonNote() {
	n := &s.note
	if n.window != nil && !n.window.Closed() {
		n.detail.SetText(n.text)
		n.window.Show()
		return
	}
	options := platform.WindowOptions{Title: "Music Player", Role: platform.RoleDialog, Center: true, Headless: s.opts.Headless, Decorations: platform.DecorationsServer, MinWidth: 360, MinHeight: 170}
	if main := s.MainWindow(); main != nil {
		options.Owner = main.Surface()
	}
	w, err := players.OpenSized(s.App, options, 480, 190, false)
	if err != nil {
		return
	}
	n.window = w
	n.title = widgets.NewTitle("The music player cannot reach media-player-musicd")
	n.title.Wrap = true
	n.detail = widgets.NewLabel(n.text)
	n.detail.Wrap = true
	n.again = widgets.NewButton("Retry", s.RetryDaemon)
	closeButton := widgets.NewButton("Close", s.closeDaemonNote)
	buttons := widgets.NewRow(n.again, closeButton).WithGap(8)
	column := widgets.NewColumn(n.title, n.detail, buttons).WithGap(12).WithPadding(20, 18, 20, 18)
	box := newDialogBox("The music daemon", column, s.closeDaemonNote)
	appearance := style.LookAppearance(s.App.Look())
	appearance.Name, appearance.FollowDesktop = players.Themed, false
	box.SetLook(style.WithAppearance(s.App.Look(), appearance))
	w.SetContent(box)
	w.SetOnCloseRequest(func() bool {
		if n.window == w {
			n.window = nil
		}
		return true
	})
	w.FocusOn(n.again)
	w.Activate()
}

func (s *Shell) closeDaemonNote() {
	if s.note.window != nil {
		w := s.note.window
		s.note.window = nil
		if !w.Closed() {
			w.Close()
		}
	}
}

// dialogBox holds a window's content: its background, its name for
// assistive technology, and Escape to close it.
type dialogBox struct {
	widget.Base
	content widget.Component
	name    string
	onClose func()
}

func newDialogBox(name string, content widget.Component, onClose func()) *dialogBox {
	d := &dialogBox{content: content, name: name, onClose: onClose}
	d.Init(d)
	d.SetManagesChildren(true)
	d.Add(content)
	return d
}

func (d *dialogBox) Measure(c layout.Constraints) paint2d.Point { return d.content.Measure(c) }

func (d *dialogBox) Arrange(r paint2d.Rect) {
	d.SetBounds(r)
	d.content.Arrange(d.LocalBounds())
}

func (d *dialogBox) Paint(ctx *paint2d.Context) {
	ctx.DrawRect(d.LocalBounds(), paint2d.Fill(d.Look().Palette().Background))
}

func (d *dialogBox) Describe(n *a11y.Node) { n.Role, n.Name = a11y.RoleDialog, d.name }

func (d *dialogBox) KeyPress(e widget.KeyEvent) bool {
	if e.Key == platform.KeyEscape && d.onClose != nil {
		d.onClose()
		return true
	}
	return false
}
