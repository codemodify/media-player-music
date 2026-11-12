package winamp

import (
	"fmt"
	"time"

	"github.com/codemodify/media-player-music/internal/players"
	"goframework.io/desktop-ui-skins"
	"goframework.io/desktop-ui/a11y"
	"goframework.io/desktop-ui/layout"
	"goframework.io/desktop-ui/style"
	"goframework.io/desktop-ui/widget"
	"goframework.io/graphics-paint2d"
)

// ---- the strip ---------------------------------------------------------------

// strip is the main window's content: a display, a seek bar and a transport
// row, laid out by hand because a compact player's proportions *are* its
// design and a flex box would give them away.
//
// Both skins place these controls on their named layout slots. Controls
// keep the same actions and keyboard order when the artwork changes.
type strip struct {
	widget.Base
	p *Player

	readout  *readout
	analyser *players.AnalyserView
	seek     *players.Fader
	volume   *players.Fader
	balance  *players.Fader

	prev, play, pause, stop, next *players.GlyphButton
	eject                         *players.GlyphButton
	shuffle, repeat               *players.GlyphButton
	eqBtn, listBtn, skinBtn       *players.GlyphButton

	// slots is where each control goes in a panel face: the skin's strip
	// layout says where each slot is.
	slots *skins.Slots
}

func newStrip(p *Player) *strip {
	s := &strip{p: p}
	s.Init(s)
	s.SetManagesChildren(true)

	s.readout = newReadout(p)
	s.analyser = players.NewAnalyserView(p.Spectrum)
	s.analyser.Move = func() { p.g.StartMove(p.iMain) }
	s.analyser.Peaks = true

	// The seek bar is a fader on its side rather than the toolkit's slider,
	// because a panel draws its thumb as a picture and the toolkit's slider
	// draws its own. The clock does not move it while the pointer does.
	s.seek = players.NewBar(0, 1, 0, "Seek", p.Host.Seek)
	s.seek.Step, s.seek.Page = 0.02, 0.1
	s.seek.Format = func(v float32) string {
		n := p.Transport.Length()
		return players.Clock(time.Duration(float64(n)*float64(v))) + " of " + durationText(n)
	}
	s.volume = players.NewBar(0, 100, p.Transport.Volume*100, "Volume", func(v float32) {
		p.Host.SetVolume(v / 100)
	})
	s.volume.Format = func(v float32) string { return fmt.Sprintf("%d%%", int(v+0.5)) }
	s.balance = players.NewBar(-1, 1, p.Transport.Pan, "Balance", p.Host.SetPan)
	s.balance.Format = balanceText

	s.prev = players.NewGlyphButton(players.GlyphPrev, "Previous track", func() { p.Command(players.CmdPrev) })
	s.play = players.NewGlyphButton(players.GlyphPlay, "Play", p.Host.Play)
	s.pause = players.NewGlyphButton(players.GlyphPause, "Pause", func() {
		// The panel's own pause key: it pauses and resumes and never
		// starts, which is the one thing it has over the play key.
		if p.Transport.State != players.Stopped {
			p.Command(players.CmdPlayPause)
		}
	})
	s.stop = players.NewGlyphButton(players.GlyphStop, "Stop", func() { p.Command(players.CmdStop) })
	s.next = players.NewGlyphButton(players.GlyphNext, "Next track", func() { p.Command(players.CmdNext) })
	s.eject = players.NewGlyphButton(players.GlyphEject, "Eject: open audio files", func() {
		p.Host.OpenFiles(s.eject, false)
	})

	s.shuffle = players.NewGlyphButton(players.GlyphShuffle, "Shuffle", func() { p.Command(players.CmdShuffle) })
	s.shuffle.Toggle = true
	s.repeat = players.NewGlyphButton(players.GlyphRepeat, "Repeat", func() { p.Command(players.CmdRepeat) })
	s.repeat.Toggle = true
	s.eqBtn = players.NewGlyphButton(players.GlyphSliders, "Equaliser window", func() { p.toggle(p.iEq) })
	s.eqBtn.Toggle = true
	s.listBtn = players.NewGlyphButton(players.GlyphList, "Playlist window", func() { p.toggle(p.iList) })
	s.listBtn.Toggle = true
	s.skinBtn = players.NewGlyphButton(players.GlyphSkin, "Skin", func() { p.NextSkin() })

	// In a panel face every control sits in a slot of the skin's strip
	// layout, and every key is the picture its slot gives it (face.go).
	s.slots = skins.NewSlots(layoutStrip).
		Bind("display", s.readout).Bind("analyser", s.analyser).Bind("seek", s.seek).
		Bind("volume", s.volume).Bind("balance", s.balance)
	for b, slot := range map[*players.GlyphButton]string{
		s.prev: "prev", s.play: "play", s.pause: "pause", s.stop: "stop",
		s.next: "next", s.eject: "eject", s.shuffle: "shuffle", s.repeat: "repeat",
		s.eqBtn: "eq", s.listBtn: "pl", s.skinBtn: "skin",
	} {
		s.slots.Bind(slot, b)
		paintAsKey(b, layoutStrip, slot)
	}
	paintAsThumb(s.seek, "seek.thumb", layoutStrip, "seek")
	paintAsThumb(s.volume, "thumb", layoutStrip, "volume")
	paintAsThumb(s.balance, "thumb", layoutStrip, "balance")

	for _, c := range s.kids() {
		s.Add(c)
	}
	return s
}

// kids is the strip's controls in tab order. It is component order, not art
// order, and the same in every face: the transport, then the levels, then
// the toggles, then the skin key.
func (s *strip) kids() []widget.Component {
	return []widget.Component{
		s.readout, s.analyser, s.seek,
		s.prev, s.play, s.pause, s.stop, s.next, s.eject,
		s.volume, s.balance,
		s.shuffle, s.repeat, s.eqBtn, s.listBtn, s.skinBtn,
	}
}

// show keeps the skin key's accessible label in sync with the selected skin.
func (s *strip) show() {
	s.skinBtn.SetAccessibleName("Skin: " + SkinLabel(s.p.Worn()) + " (next skin, Ctrl+K)")
	s.skinBtn.Label = "Skin: " + SkinLabel(s.p.Worn()) + " — Ctrl+K for the next, right-click for all"
}

func (p *Player) toggle(i int) {
	pane := p.g.Rack.Pane(i)
	if pane == nil {
		return
	}
	p.showWindow(i, !pane.Shown)
}

func (p *Player) showWindow(i int, on bool) {
	if i == p.iEq {
		p.Host.Model.EqualizerVisible = on
	} else if i == p.iList {
		p.Host.Model.PlaylistVisible = on
	}
	p.g.Show(i, on)
	p.configureUtilityWindows()
	p.refresh()
}

// sync puts the model into the controls: the play button's glyph, the
// toggles, the seek bar's thumb.
func (s *strip) sync() {
	t := s.p.Transport
	s.play.SetGlyph(players.GlyphPlay, "Play")
	s.pause.SetChecked(t.State == players.Paused)
	s.shuffle.SetChecked(t.Random)
	s.repeat.SetChecked(t.Repeat != players.RepeatOff)
	if t.Repeat == players.RepeatOne {
		s.repeat.SetGlyph(players.GlyphRepeatOne, "Repeat one")
	} else {
		s.repeat.SetGlyph(players.GlyphRepeat, t.Repeat.String())
	}
	if p := s.p.g.Rack.Pane(s.p.iEq); p != nil {
		s.eqBtn.SetChecked(p.Shown)
	}
	if p := s.p.g.Rack.Pane(s.p.iList); p != nil {
		s.listBtn.SetChecked(p.Shown)
	}
	if !s.seek.Dragging() {
		s.seek.Set(t.Fraction())
	}
	s.volume.Set(t.Volume * 100)
	s.balance.Set(t.Pan)
	s.readout.Invalidate()
	s.analyser.Invalidate()
	s.seek.Invalidate()
	s.volume.Invalidate()
	s.Invalidate()
}

func (s *strip) Arrange(r paint2d.Rect) {
	s.SetBounds(r)
	s.show()
	s.styleAnalyser(faceOf(s.Look()))
	s.slots.Arrange(s.Look(), s.LocalBounds())
}

func (s *strip) styleAnalyser(f face) {
	a := s.analyser
	a.Pixelated, a.Gap = true, 1
	a.Ramp, a.PeakTint = nil, paint2d.Color{}
	a.MinBarW = 2
	if in := f.ink(s.Look()); in != nil {
		a.Ramp, a.PeakTint = in.ramp, in.peak
		a.MinBarW = 3
	}
}

func (s *strip) Measure(c layout.Constraints) paint2d.Point {
	lk := s.Look()
	if sz, ok := s.slots.Measure(lk, c); ok {
		return sz
	}
	return c.Constrain(paint2d.Pt(style.Dip(lk, StripW), style.Dip(lk, StripH)))
}

// Paint draws the skin's artwork, clock, title, figures and status lamps.
func (s *strip) Paint(ctx *paint2d.Context) {
	lk := s.Look()
	f := faceOf(lk)
	if !f.panelled() {
		return
	}
	b := s.LocalBounds()
	skins.DrawSkinLayout(lk, ctx, b, layoutStrip)
	// A pack may preserve source-pixel display wells while independently
	// scaling the surrounding chrome. Their geometry and art stay skin-owned.
	for _, slot := range []string{"backdrop.display", "backdrop.title", "backdrop.rate", "backdrop.freq", "backdrop.labels"} {
		if r, ok := skins.SkinSlotRect(lk, layoutStrip, slot, b); ok {
			skins.DrawSkinSlot(lk, ctx, r, layoutStrip, slot, style.ControlState(0))
		}
	}
	at := func(slot string) paint2d.Rect { return slotRect(lk, layoutStrip, slot, b) }
	in := f.ink(lk)
	t := s.p.Transport
	tr := t.Track()

	state := "state.stop"
	switch t.State {
	case players.Playing:
		state = "state.play"
	case players.Paused:
		state = "state.pause"
	}
	art(lk, ctx, at("state"), state)
	if t.State != players.Stopped {
		ledClock(lk, ctx, b, players.Clock(t.Pos))
	}

	// The title as the era printed it: the track's place, the artist and
	// title, and its length.
	title := currentTitle(t)
	if s.p.Host.Model.Error != "" {
		title = s.p.Host.StatusText()
	}
	pixScroll(lk, ctx, at("title"), title, in.text, s.p)

	rt, ft := at("rate"), at("freq")
	rate := "--"
	if tr.Bitrate > 0 {
		rate = fmt.Sprint(tr.Bitrate / 1000)
	}
	pixText(lk, ctx, rt.Max.X-pixWidth(lk, rate), rt.Min.Y, rate, in.text)
	freq := "--"
	if tr.Rate > 0 {
		freq = fmt.Sprint(tr.Rate)
	}
	pixText(lk, ctx, ft.Max.X-pixWidth(lk, freq), ft.Min.Y, freq, in.text)
	if tr.Channels >= 2 {
		art(lk, ctx, at("stereo"), "lamp.stereo")
	} else if tr.Channels == 1 {
		art(lk, ctx, at("mono"), "lamp.mono")
	}
}

// MousePress on the strip itself starts a window move, so the shell between
// the controls is a drag handle — which is what the whole face of a player
// this size has always been. The secondary button opens the skin menu.
func (s *strip) MousePress(e widget.MouseEvent) bool {
	if s.p.rightClick(s, e) {
		return true
	}
	s.p.g.StartMove(s.p.iMain)
	return true
}

func (s *strip) KeyPress(e widget.KeyEvent) bool { return s.p.Keys(s.p.Main(), e) }

// CaptionAt tells the frame that the strip's own background is caption: a
// press there moves the window rather than landing in the app.
func (s *strip) CaptionAt(paint2d.Point) bool { return true }

func (s *strip) Describe(n *a11y.Node) {
	n.Role = a11y.RolePane
	if n.Name == "" {
		n.Name = "Player"
	}
	n.Description = s.p.Host.StatusText()
}

// balanceText reads a balance the way a person says it.
func balanceText(v float32) string {
	switch {
	case v < -0.02:
		return fmt.Sprintf("%d%% left", int(-v*100+0.5))
	case v > 0.02:
		return fmt.Sprintf("%d%% right", int(v*100+0.5))
	}
	return "centre"
}
