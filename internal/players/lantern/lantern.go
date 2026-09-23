// Package lantern is the shaped player with a menu bar: a main window whose
// outline is its skin's, a playlist window anchored to its right-hand edge,
// and one control that drops the skin and runs the whole thing in an
// ordinary theme.
//
// It is a **visual demo**. Nothing is decoded and nothing is played; the
// status bar says so and so does the About box.
//
// The switch is the point of this one. A skin in this toolkit is a *pack* —
// it lists in Settings beside the other hundred and twenty-odd, it is chosen
// by the same preference, and applying one is the same call as applying
// Breeze. So "show me this app without its skin" is not a mode the app has
// to implement: it is style.WithAppearance with a different pack id, the
// same thing Settings does, and what comes back is the same widget tree in
// another look — the same tab order, the same accessibility tree, the same
// keyboard, and a window that is a rectangle again because the outline
// belonged to the skin and not to the app.
package lantern

import (
	"fmt"
	"time"

	"github.com/codemodify/media-player-music/internal/players"
	"github.com/codemodify/paintengine2d"
	"github.com/codemodify/uitoolkit/a11y"
	"github.com/codemodify/uitoolkit/app"
	"github.com/codemodify/uitoolkit/layout"
	"github.com/codemodify/uitoolkit/platform"
	"github.com/codemodify/uitoolkit/rack"
	"github.com/codemodify/uitoolkit/style"
	"github.com/codemodify/uitoolkit/widget"
	"github.com/codemodify/uitoolkit/widgets"
)

// The window sizes, in logical pixels.
const (
	MainW = 880
	MainH = 560
	ListW = 380
)

// Skin is the pack this player wears, and Themed is the pack the switch
// drops it into: an ordinary era pack with no art in it at all.
const (
	Skin   = "lantern"
	Themed = "breeze-night"
)

// Player is the two windows and the model under them.
type Player struct {
	App  *app.Application
	Desk *rack.Desk

	Transport *players.Transport
	Spectrum  *players.Spectrum

	Main, List *app.Window
	body       *body
	queue      *queuePane
	pulse      *players.Pulse

	iMain, iList int
	skinned      bool
	// anchored is whether the playlist has been put beside the player yet,
	// and tries how many times settle has asked — see settle, which waits
	// for the desktop to place them both and then for it to agree.
	anchored bool
	tries    int
}

// Options is what the command line passes in.
type Options struct {
	Headless bool
	Scale    float32
	// NoList opens the main window on its own.
	NoList bool
	// Themed starts with the skin already dropped.
	Themed bool
}

// New opens the player's windows.
func New(a *app.Application, opts Options) (*Player, error) {
	p := &Player{
		App:       a,
		Transport: players.NewTransport(players.NewLibrary()),
		Spectrum:  players.NewSpectrum(40),
		iList:     -1,
		skinned:   !opts.Themed,
	}
	main, err := p.open(a, opts, "Lantern", MainW, MainH)
	if err != nil {
		return nil, err
	}
	p.Main = main
	p.body = newBody(p)
	main.SetContent(p.body)

	p.Desk = rack.NewDesk(rack.DefaultReach)
	p.iMain = p.Desk.Add("main", main)

	if !opts.NoList {
		w, err := p.open(a, opts, "Lantern playlist", ListW, MainH)
		if err != nil {
			return nil, err
		}
		p.List = w
		p.queue = newQueuePane(p)
		w.SetContent(p.queue)
		p.iList = p.Desk.Add("playlist", w)
		// Anchored to the main window's right-hand edge, flush at the top.
		p.Desk.Attach(p.iList, p.iMain, rack.SideRight)
	}

	p.Transport.Changed = p.refresh
	p.pulse = players.NewPulse(60*time.Millisecond, p.advance)
	p.refresh()
	// Something has the keyboard from the first frame. A player's keys are
	// bare letters that bubble up from whatever is focused, and a window
	// with no focus at all has nothing for them to bubble from.
	p.Main.RequestFocus(p.body.play)
	if p.queue != nil {
		p.List.RequestFocus(p.queue.table)
	}
	return p, nil
}

func (p *Player) open(a *app.Application, opts Options, title string, w, h int) (*app.Window, error) {
	return players.OpenSized(a, platform.WindowOptions{
		Title:       title,
		MinWidth:    300,
		MinHeight:   260,
		Headless:    opts.Headless,
		Decorations: platform.DecorationsClient,
	}, w, h, false)
}

// Start begins the clock.
func (p *Player) Start() {
	p.Transport.Play()
	p.pulse.Start(p.Main)
}

// Pulse is the clock, for a test that drives it by hand.
func (p *Player) Pulse() *players.Pulse { return p.pulse }

// Pose puts the player at a stated position, with no clock involved.
func (p *Player) Pose(pos time.Duration) {
	p.Transport.State = players.Playing
	p.Transport.Pos = pos
	p.Spectrum.Reset()
	p.Spectrum.Advance(pos, true, 250*time.Millisecond)
	p.refresh()
}

// Skinned reports whether the skin is on.
func (p *Player) Skinned() bool { return p.skinned }

// SetSkinned puts the skin on or takes it off, which is the whole argument
// of this demo in four lines.
//
// It is the call Settings makes. A skin is a pack, so changing to one is
// changing the appearance's pack id and rebuilding the look; every window
// of the application follows, because that is what Application.SetLook
// does for any theme change. The app knows nothing about art, silhouettes
// or sprite sheets, and nothing in the widget tree is rebuilt: the same
// buttons, the same tab order, the same accessibility nodes, painted by a
// different engine.
func (p *Player) SetSkinned(on bool) {
	p.skinned = on
	ap := style.LookAppearance(p.App.Look())
	ap.FollowDesktop = false
	if on {
		ap.Name = Skin
	} else {
		ap.Name = Themed
	}
	p.App.SetLook(style.WithAppearance(p.App.Look(), ap))
	p.refresh()
}

func (p *Player) advance(dt time.Duration) {
	p.Transport.Tick(dt)
	p.Spectrum.Advance(p.Transport.Pos, p.Transport.State == players.Playing, dt)
	p.settle()
	p.Desk.Follow()
	p.refresh()
}

// settle anchors the playlist once the desktop has said where it put both
// windows. It cannot be done when they are made: a window manager places a
// window as it maps it and the app is told afterwards, so an anchor set at
// construction is set against a player that is still at the origin.
func (p *Player) settle() {
	if p.anchored || p.iList < 0 || !p.Desk.Places() || !p.Desk.Adopt() {
		return
	}
	p.tries++
	p.Desk.Attach(p.iList, p.iMain, rack.SideRight)
	// The anchor has taken when the window is where the bond says. A
	// window manager may refuse — KWin keeps a window on the screen, and
	// an anchor that would hang off the right-hand edge is exactly that —
	// so it is asked a few times and then left alone. The playlist is
	// then simply loose, and dragging it back to the player's edge snaps
	// it, which is the same path a user takes anyway.
	pane := p.Desk.Rack.Pane(p.iList)
	if x, y, ok := p.List.Position(); ok && x == pane.Box.X && y == pane.Box.Y {
		p.anchored = true
		return
	}
	if p.tries >= settleTries {
		p.anchored = true
	}
}

// settleTries is how many clock ticks the anchor is asked for before the
// desktop's answer is taken as final.
const settleTries = 20

func (p *Player) refresh() {
	if p.body != nil {
		p.body.sync()
	}
	if p.queue != nil {
		p.queue.sync()
	}
}

// Command runs a shared transport command and keeps the clock going.
func (p *Player) Command(c players.Command) {
	p.Transport.Do(c)
	if p.Transport.State == players.Playing {
		p.pulse.Start(p.Main)
	}
}

// ShowList opens or closes the playlist window.
func (p *Player) ShowList(on bool) {
	if p.iList < 0 {
		return
	}
	p.Desk.Show(p.iList, on)
	p.refresh()
}

// ListShown reports whether the playlist window is open.
func (p *Player) ListShown() bool {
	pane := p.Desk.Rack.Pane(p.iList)
	return pane != nil && pane.Shown
}

// Keys is the keyboard both windows share.
//
// It is handed the window the key came from: the guard below asks what has
// the keyboard, and the answer in the playlist window is a text field while
// the answer in the player is a transport button.
func (p *Player) Keys(w *app.Window, e widget.KeyEvent) bool {
	if e.Key == platform.KeyEscape {
		p.App.Quit()
		return true
	}
	if e.Mods.Ctrl() {
		switch e.Key {
		case platform.KeyK:
			p.SetSkinned(!p.skinned)
			return true
		case platform.KeyL:
			p.ShowList(!p.ListShown())
			return true
		case platform.KeyQ:
			p.App.Quit()
			return true
		}
		return false
	}
	if players.Typing(w.Focus()) {
		// The focus is in something that takes text: its letters are its
		// own. Nothing the transport answers to is a modifier chord, so
		// there is nothing left to try.
		return false
	}
	if c := players.CommandFor(e); c != players.CmdNone {
		p.Command(c)
		return true
	}
	return false
}

// Status is the status bar's line, and what a screenshot is read against.
func (p *Player) Status() string {
	look := "theme " + Themed
	if p.skinned {
		look = "skin " + Skin
	}
	shaped := "rectangle"
	if p.Main.ShapeActive() {
		shaped = "shaped"
	}
	anchored := "loose"
	if pane := p.Desk.Rack.Pane(p.iList); pane != nil && pane.To >= 0 {
		anchored = pane.Bond.Side.String()
	}
	if !p.Desk.Places() {
		anchored = "not placeable here"
	}
	return fmt.Sprintf("%s · %s · playlist %s", look, shaped, anchored)
}

// ---- the main window ---------------------------------------------------------

// body is the menu bar, the display and the transport bar.
type body struct {
	widget.Base
	p *Player

	menu     *widgets.MenuBar
	display  *display
	analyser *players.AnalyserView
	status   *widgets.StatusBar

	seek   *widgets.Slider
	volume *widgets.Slider

	prev, play, stop, next *players.GlyphButton
	mute                   *players.GlyphButton
	shuffle, repeat        *players.GlyphButton
	listBtn, skinBtn       *players.GlyphButton

	bar paintengine2d.Rect
}

func newBody(p *Player) *body {
	b := &body{p: p}
	b.Init(b)
	b.SetManagesChildren(true)

	b.display = newDisplay(p)
	b.analyser = players.NewAnalyserView(p.Spectrum)
	b.analyser.Drag = p.Main
	b.analyser.Gap = 2
	b.analyser.MinBarW = 3
	b.status = widgets.NewStatusBar(players.Disclaimer, "")

	b.seek = widgets.NewSlider(0, 1, 0, func(v float32) {
		if b.scrubbing() {
			p.Transport.Seek(v)
		}
	})
	b.seek.SetAccessibleName("Seek")
	b.volume = widgets.NewSlider(0, 100, p.Transport.Volume*100, func(v float32) {
		p.Transport.SetVolume(v / 100)
	})
	b.volume.SetAccessibleName("Volume")

	b.prev = players.NewGlyphButton(players.GlyphPrev, "Previous track", func() { p.Command(players.CmdPrev) })
	b.play = players.NewGlyphButton(players.GlyphPlay, "Play", func() { p.Command(players.CmdPlayPause) })
	b.play.Push = true
	b.stop = players.NewGlyphButton(players.GlyphStop, "Stop", func() { p.Command(players.CmdStop) })
	b.next = players.NewGlyphButton(players.GlyphNext, "Next track", func() { p.Command(players.CmdNext) })
	b.mute = players.NewGlyphButton(players.GlyphVolume, "Mute", func() { p.Command(players.CmdMute) })
	b.mute.Toggle = true
	b.shuffle = players.NewGlyphButton(players.GlyphShuffle, "Shuffle", func() { p.Command(players.CmdShuffle) })
	b.shuffle.Toggle = true
	b.repeat = players.NewGlyphButton(players.GlyphRepeat, "Repeat", func() { p.Command(players.CmdRepeat) })
	b.repeat.Toggle = true
	b.listBtn = players.NewGlyphButton(players.GlyphList, "Playlist window", func() { p.ShowList(!p.ListShown()) })
	b.listBtn.Toggle = true
	b.skinBtn = players.NewGlyphButton(players.GlyphSkin, "Skin", func() { p.SetSkinned(!p.skinned) })
	b.skinBtn.Toggle = true

	b.menu = widgets.NewMenuBar(
		widgets.NewMenu("&Media",
			widgets.ItemAccel("&Open file…", "Ctrl+O", func() { b.explain("Opening files") }),
			widgets.ItemAccel("Open &network stream…", "Ctrl+N", func() { b.explain("Network streams") }),
			widgets.Sep(),
			widgets.ItemAccel("&Quit", "Ctrl+Q", func() { p.App.Quit() }),
		),
		widgets.NewMenu("&Playback",
			widgets.ItemAccel("&Play / Pause", "Space", func() { p.Command(players.CmdPlayPause) }),
			widgets.Item("&Stop", func() { p.Command(players.CmdStop) }),
			widgets.Sep(),
			widgets.Item("&Previous", func() { p.Command(players.CmdPrev) }),
			widgets.Item("&Next", func() { p.Command(players.CmdNext) }),
			widgets.Sep(),
			widgets.CheckItem("Sh&uffle", p.Transport.Random, func() { p.Command(players.CmdShuffle) }),
			widgets.CheckItem("&Repeat", p.Transport.Repeat != players.RepeatOff, func() { p.Command(players.CmdRepeat) }),
		),
		widgets.NewMenu("&View",
			widgets.ItemAccel("&Playlist", "Ctrl+L", func() { p.ShowList(!p.ListShown()) }),
			widgets.ItemAccel("&Skin", "Ctrl+K", func() { p.SetSkinned(!p.skinned) }),
		),
		widgets.NewMenu("&Help",
			widgets.Item("&About Lantern", func() { b.about() }),
		),
	)

	for _, c := range []widget.Component{
		b.menu, b.display, b.analyser, b.status, b.seek, b.volume,
		b.prev, b.play, b.stop, b.next, b.mute, b.shuffle, b.repeat,
		b.listBtn, b.skinBtn,
	} {
		b.Add(c)
	}
	return b
}

// explain is what every menu item that would need a media stack does: it
// says so rather than pretending.
func (b *body) explain(what string) {
	widgets.Info(b, what, what+" would need a media stack. This is a visual demo of skins and\n"+
		"shaped windows: nothing here decodes or plays anything, and the toolkit\n"+
		"has no audio or video dependency at all.", nil)
}

func (b *body) about() {
	widgets.Info(b, "About Lantern",
		"Lantern is one of three player demos in uitoolkit.\n\n"+
			players.Disclaimer+"\n\n"+
			"Its window's outline comes from its skin, which is an ordinary pack;\n"+
			"Ctrl+K drops the skin and runs the same app in "+Themed+".\n\n"+
			players.KeyHelp, nil)
}

func (b *body) scrubbing() bool { return b.seek.Focused() || b.seek.Hovered() }

func (b *body) sync() {
	t := b.p.Transport
	if t.State == players.Playing {
		b.play.SetGlyph(players.GlyphPause, "Pause")
	} else {
		b.play.SetGlyph(players.GlyphPlay, "Play")
	}
	if t.Muted() {
		b.mute.SetGlyph(players.GlyphMute, "Unmute")
	} else {
		b.mute.SetGlyph(players.GlyphVolume, "Mute")
	}
	b.mute.SetChecked(t.Muted())
	b.shuffle.SetChecked(t.Random)
	b.repeat.SetChecked(t.Repeat != players.RepeatOff)
	if t.Repeat == players.RepeatOne {
		b.repeat.SetGlyph(players.GlyphRepeatOne, "Repeat one")
	} else {
		b.repeat.SetGlyph(players.GlyphRepeat, t.Repeat.String())
	}
	b.listBtn.SetChecked(b.p.ListShown())
	b.skinBtn.SetChecked(b.p.skinned)
	if !b.scrubbing() {
		b.seek.Value = t.Fraction()
	}
	b.volume.Value = t.Volume * 100
	b.status.SetParts(players.Disclaimer, b.p.Status())
	b.Invalidate()
}

func (b *body) Measure(c layout.Constraints) paintengine2d.Point {
	lk := b.Look()
	return c.Constrain(paintengine2d.Pt(style.Dip(lk, MainW), style.Dip(lk, 460)))
}

func (b *body) Arrange(r paintengine2d.Rect) {
	b.SetBounds(r)
	lk := b.Look()
	dip := func(v float32) float32 { return style.Dip(lk, v) }
	box := b.LocalBounds()

	menuH := b.menu.Measure(layout.Loose(box.Dx(), box.Dy())).Y
	b.menu.Arrange(paintengine2d.XYWH(0, 0, box.Dx(), menuH))

	statusH := b.status.Measure(layout.Loose(box.Dx(), box.Dy())).Y
	b.status.Arrange(paintengine2d.XYWH(0, box.Dy()-statusH, box.Dx(), statusH))

	barH := dip(76)
	b.bar = paintengine2d.XYWH(0, box.Dy()-statusH-barH, box.Dx(), barH)

	// The display fills everything between the menu and the bar.
	top := menuH + dip(6)
	b.display.Arrange(paintengine2d.XYWH(0, top, box.Dx(), max(b.bar.Min.Y-dip(6)-top, dip(40))))
	d := b.display.Bounds()
	b.analyser.Arrange(paintengine2d.XYWH(d.Min.X+dip(24), d.Min.Y+d.Dy()*0.50, d.Dx()-dip(48), d.Dy()*0.40))

	// The bar: a seek line across the top of it, then the controls.
	in := b.bar.Inset(dip(8))
	clockW := dip(52)
	b.seek.Arrange(paintengine2d.XYWH(in.Min.X+clockW, in.Min.Y, max(in.Dx()-2*clockW, dip(40)), dip(12)))

	y := in.Min.Y + dip(22)
	btn := dip(32)
	x := in.Min.X
	for _, c := range []*players.GlyphButton{b.play, b.prev, b.stop, b.next} {
		c.Arrange(paintengine2d.XYWH(x, y, btn, btn))
		x += btn + dip(4)
	}
	x += dip(12)
	for _, c := range []*players.GlyphButton{b.shuffle, b.repeat} {
		c.Arrange(paintengine2d.XYWH(x, y+dip(2), dip(28), dip(28)))
		x += dip(32)
	}
	// From the right-hand edge in: the skin switch, the playlist toggle,
	// then the volume and its mute.
	rx := in.Max.X - dip(28)
	for _, c := range []*players.GlyphButton{b.skinBtn, b.listBtn} {
		c.Arrange(paintengine2d.XYWH(rx, y+dip(2), dip(28), dip(28)))
		rx -= dip(32)
	}
	volW := min(dip(120), max(rx-x-dip(60), dip(40)))
	rx -= volW
	b.volume.Arrange(paintengine2d.XYWH(rx, y+(btn-dip(12))/2, volW, dip(12)))
	b.mute.Arrange(paintengine2d.XYWH(rx-dip(32), y+dip(2), dip(28), dip(28)))
}

func (b *body) Paint(ctx *paintengine2d.Context) {
	lk := b.Look()
	pal := lk.Palette()
	if !b.bar.Empty() {
		lk.DrawToolBar(ctx, b.bar)
	}
	t := b.p.Transport
	s := b.seek.Bounds()
	mono := players.ScaledFace(lk.MonoFont(), style.Dip(lk, 12))
	h := s.Dy() + style.Dip(lk, 6)
	y := s.Min.Y - style.Dip(lk, 3)
	players.DrawTextIn(ctx, mono, paintengine2d.XYWH(s.Min.X-style.Dip(lk, 50), y, style.Dip(lk, 44), h),
		players.Clock(t.Pos), pal.Text, style.AlignEnd)
	players.DrawTextIn(ctx, mono, paintengine2d.XYWH(s.Max.X+style.Dip(lk, 6), y, style.Dip(lk, 44), h),
		players.Countdown(t.Remaining()), pal.TextMuted, style.AlignStart)
}

func (b *body) KeyPress(e widget.KeyEvent) bool { return b.p.Keys(b.p.Main, e) }

func (b *body) Describe(n *a11y.Node) {
	n.Role = a11y.RolePane
	if n.Name == "" {
		n.Name = "Player"
	}
	n.Description = players.Disclaimer
}

// ---- the display -------------------------------------------------------------

// display is the big well the video would be in: the track over it and the
// analyser along its floor.
type display struct {
	players.DragsWindow
	p *Player
}

func newDisplay(p *Player) *display {
	d := &display{p: p}
	d.Init(d)
	d.Win = p.Main
	return d
}

func (d *display) Measure(c layout.Constraints) paintengine2d.Point {
	return c.Constrain(paintengine2d.Pt(style.Dip(d.Look(), 520), style.Dip(d.Look(), 300)))
}

func (d *display) Arrange(b paintengine2d.Rect) { d.SetBounds(b) }

func (d *display) Paint(ctx *paintengine2d.Context) {
	lk, b := d.Look(), d.LocalBounds()
	pal := lk.Palette()
	t := d.p.Transport
	dip := func(v float32) float32 { return style.Dip(lk, v) }

	// The matte well. It is drawn in palette colours, so the same
	// component is the skin's charcoal screen and breeze-night's sunken
	// view once the skin is dropped.
	r := dip(6)
	ctx.DrawRoundRect(b, r, r, paintengine2d.Fill(pal.Field))
	ctx.DrawRoundRect(b.Inset(0.5), r, r, paintengine2d.StrokePaint(pal.FieldBorder, max(dip(1), 1)))

	tr := t.Track()
	pad := dip(24)
	kind := "Audio"
	if tr.Video() {
		kind = "Video"
	}
	tiny := players.ScaledFace(lk.Font(), dip(11))
	players.DrawTextIn(ctx, tiny, paintengine2d.XYWH(b.Min.X+pad, b.Min.Y+pad, b.Dx()-2*pad, dip(14)),
		kind+" · "+players.Clock(tr.Length)+" · "+fmt.Sprintf("%d kHz", tr.Rate),
		pal.TextMuted, style.AlignStart)
	title := players.ScaledFace(lk.TitleFont(), dip(30))
	players.DrawTextIn(ctx, title, paintengine2d.XYWH(b.Min.X+pad, b.Min.Y+pad+dip(18), b.Dx()-2*pad, dip(38)),
		tr.Title, pal.Text, style.AlignStart)
	sub := players.ScaledFace(lk.Font(), dip(15))
	players.DrawTextIn(ctx, sub, paintengine2d.XYWH(b.Min.X+pad, b.Min.Y+pad+dip(58), b.Dx()-2*pad, dip(20)),
		tr.Artist+"  ·  "+tr.Album, pal.TextMuted, style.AlignStart)
}

func (d *display) Describe(n *a11y.Node) {
	t := d.p.Transport
	tr := t.Track()
	n.Role = a11y.RoleLabel
	n.Name = fmt.Sprintf("%s by %s, from %s. %s of %s. %s.",
		tr.Title, tr.Artist, tr.Album,
		players.Clock(t.Pos), players.Clock(t.Length()), t.State)
	n.Description = players.Disclaimer
}
