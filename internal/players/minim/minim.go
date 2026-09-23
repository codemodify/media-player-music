// Package minim is the compact player: a strip 275 by 116 design pixels
// with an equaliser and a playlist that stick to its edges and travel with
// it.
//
// It is a **visual demo**. Nothing is decoded and nothing is played; the
// clock counts, the seek bar scrubs the clock, the analyser is a function of
// where the clock is, and the equaliser's ten faders move and filter
// nothing. The window says so and so does its About box.
//
// What it is a demo *of* is three things the toolkit gained and no toolkit
// of the era had all of:
//
//   - A skin is a pack. Minim wears the "minim" skin, which is a pixel sheet
//     over win95; run it with UITK_THEME=breeze-night and it is the same app
//     in an ordinary theme, with every control still a control. It has two
//     more, "minim-classic" and "minim-silver", which are panels rather than
//     dressings (face.go), and the skin key, Ctrl+K or a right-click
//     switches between all three live (skins.go).
//   - A skinned window need not be a rectangle. The silhouette is the skin's
//     (window.shape), so the app never mentions it, and the strip stands on
//     a stepped chin with the desktop showing through beside it.
//   - The proportions are design pixels rather than a frozen size. 275 by
//     116 is what this shape was at 1×; at 1.75 it is 481 by 203 and the art
//     is drawn from the 2× sheet at whole multiples, because the skin is
//     declared pixelated.
package minim

import (
	"fmt"
	"time"

	"github.com/codemodify/media-player-music/internal/players"
	"github.com/codemodify/uitoolkit/app"
	"github.com/codemodify/uitoolkit/platform"
	"github.com/codemodify/uitoolkit/rack"
	"github.com/codemodify/uitoolkit/widget"
)

// The window sizes, in logical pixels. They are the design pixels the skin
// is drawn on, so the strip is exactly the shape it was drawn as at 1× and
// exactly that shape scaled at every other scale.
const (
	StripW = 275
	StripH = 116
	EqH    = 116
	ListH  = 232
)

// Skin is the pack this player wears by default. It is an ordinary pack id:
// anything else in Settings runs the same app, and Skins lists the other two
// it was drawn for.
const Skin = "minim"

// Player is the three windows and the one model under them. The model is
// the application's (players.Host): this face shows it, and putting on
// another face leaves it exactly where it was.
type Player struct {
	Host *players.Host
	App  *app.Application
	Desk *rack.Desk

	Transport *players.Transport
	Spectrum  *players.Spectrum
	Equaliser *players.Equalizer

	Main, Eq, List *app.Window
	strip          *strip
	eqPane         *eqPane
	listPane       *listPane
	// dropLook takes the look watcher off again when the face is closed.
	dropLook func()

	// iMain, iEq, iList are the panes in the desk's rack.
	iMain, iEq, iList int
	// stacked is whether the three have been put in a stack yet, and tries
	// how many times settle has asked — see settle, which cannot run until
	// the desktop has placed them and may not be obeyed when it does.
	stacked bool
	tries   int

	// auto is the equaliser's AUTO key: a preset per track. lastTrack is the
	// track it last chose for, so it chooses once per track and then leaves
	// the faders to whoever moves them.
	auto      bool
	lastTrack int
}

// Options is what the command line passes in.
type Options struct {
	Headless bool
	// Scale is the display scale; 0 takes the desktop's.
	Scale float32
	// NoEq and NoList open the main strip on its own.
	NoEq, NoList bool
}

// New opens the player's windows on the application's own model.
func New(h *players.Host, opts Options) (*Player, error) {
	a := h.App
	p := &Player{
		Host:      h,
		App:       a,
		Transport: h.Model.Transport,
		Spectrum:  h.Model.Spectrum,
		Equaliser: h.Model.Equaliser,
		iEq:       -1,
		iList:     -1,
		lastTrack: -1,
	}
	// This face's analyser falls a little faster than the others: its
	// window is a hundred and sixteen design pixels tall and a marker that
	// hangs about is a marker in the way.
	p.Spectrum.Fall = 0.7

	main, err := p.open(a, opts, "Minim — a visual demo", layoutStrip, StripW, StripH)
	if err != nil {
		return nil, err
	}
	p.Main = main
	p.strip = newStrip(p)
	main.SetContent(p.strip)

	// The rack speaks logical pixels, so its reach grows with the display
	// like every other measurement in the toolkit: ten pixels of slop is a
	// fingertip at 1× and at 2× alike.
	p.Desk = rack.NewDesk(rack.DefaultReach)
	p.iMain = p.Desk.Add("main", main)

	if !opts.NoEq {
		w, err := p.open(a, opts, "Minim equaliser", layoutEqualiser, StripW, EqH)
		if err != nil {
			return nil, err
		}
		p.Eq = w
		p.eqPane = newEqPane(p)
		w.SetContent(p.eqPane)
		p.iEq = p.Desk.Add("equaliser", w)
		p.Desk.Attach(p.iEq, p.iMain, rack.SideBottom)
	}
	if !opts.NoList {
		w, err := p.open(a, opts, "Minim playlist", layoutPlaylist, StripW, ListH)
		if err != nil {
			return nil, err
		}
		p.List = w
		p.listPane = newListPane(p)
		w.SetContent(p.listPane)
		p.iList = p.Desk.Add("playlist", w)
		p.Desk.Attach(p.iList, maxInt(p.iEq, p.iMain), rack.SideBottom)
	}

	p.Transport.Changed = p.refresh
	p.Equaliser.Changed = p.refresh
	// A look can change under the player from outside it too — Settings,
	// an edited look.json, UITK_THEME on a watcher — and the titles and the
	// focus follow the face whichever way it changed.
	p.dropLook = a.OnLookChange(p.restyle)
	p.restyle()
	p.refresh()
	// Something in every window has the keyboard from the first frame. A
	// player's keys are bare letters that bubble up from whatever is
	// focused, and a window with no focus at all has nothing for them to
	// bubble from — so the equaliser and the playlist are given one too,
	// or the transport keys would be dead in two windows out of three.
	p.Main.RequestFocus(p.strip.play)
	if p.eqPane != nil {
		p.Eq.RequestFocus(p.eqPane.preamp)
	}
	if p.listPane != nil {
		p.List.RequestFocus(p.listPane.list)
	}
	return p, nil
}

// open makes one of the player's windows. All three take the toolkit's own
// frame: the skin's caption band is the title strip a player of this shape
// had, and a *look's* silhouette is only asked of a window the toolkit
// frames — where the desktop draws the frame, the window is the rectangle
// inside it and there is nothing to cut.
//
// Each is given its role — the name of the layout it is laid out by — so a
// skin can dress one window differently from the others: Minim Silver's
// equaliser has a tab for a header where the other two have a band. A look
// that says nothing about the role gives every window its one frame.
func (p *Player) open(a *app.Application, opts Options, title, role string, w, h int) (*app.Window, error) {
	win, err := players.OpenSized(a, platform.WindowOptions{
		Title:       title,
		Headless:    opts.Headless,
		Decorations: platform.DecorationsClient,
	}, w, h, true)
	if err != nil {
		return nil, err
	}
	win.SetFrameRole(role)
	return win, nil
}

// Close takes the face off: its windows go, and the model they were showing
// stays exactly as it is.
func (p *Player) Close() {
	if p.dropLook != nil {
		p.dropLook()
		p.dropLook = nil
	}
	for _, w := range []*app.Window{p.List, p.Eq, p.Main} {
		if w != nil {
			w.Close()
		}
	}
}

// Windows are the face's windows, named, for a caller taking a picture of
// each.
func (p *Player) Windows() []players.NamedWindow {
	out := []players.NamedWindow{{Name: "strip", Window: p.Main}}
	if p.Eq != nil {
		out = append(out, players.NamedWindow{Name: "equaliser", Window: p.Eq})
	}
	if p.List != nil {
		out = append(out, players.NamedWindow{Name: "playlist", Window: p.List})
	}
	return out
}

// MainWindow is the window the application's clock wakes on.
func (p *Player) MainWindow() *app.Window { return p.Main }

// Pose puts the player in a stated position with the analyser advanced to
// match, and touches no clock at all.
//
// It is what the screenshots and the golden tests use. A player posed at
// ninety-seven seconds is the same picture every time it is posed there —
// the clock reads the same, the seek bar is at the same place and the
// analyser is in the same shape, because the analyser is a function of the
// position and not of a timer.
func (p *Player) Pose(pos time.Duration) {
	p.Host.Pose(pos)
	p.refresh()
}

// Advance is what this face does on each of the application's ticks, beyond
// the model the application has already moved: it looks at where the desktop
// has put the three windows.
func (p *Player) Advance(time.Duration) {
	p.settle()
	p.Desk.Follow()
	p.refresh()
}

// Refresh puts the model back into the controls.
func (p *Player) Refresh() { p.refresh() }

// settle builds the stack once the desktop has said where it put the
// windows, and then never again.
//
// It cannot be done when the windows are made: a window manager places a
// window as it maps it and the app is told afterwards, so a stack arranged
// at construction is arranged around a strip that is still at the origin.
// On a desktop that will not place windows at all this never runs, which is
// right — there is nothing to arrange.
func (p *Player) settle() {
	if p.stacked || !p.Desk.Places() || !p.Desk.Adopt() {
		return
	}
	p.tries++
	if p.iEq >= 0 {
		p.Desk.Attach(p.iEq, p.iMain, rack.SideBottom)
	}
	if p.iList >= 0 {
		p.Desk.Attach(p.iList, maxInt(p.iEq, p.iMain), rack.SideBottom)
	}
	// It has taken when the panes are where their bonds say. A window
	// manager may refuse a move that would hang a window off the screen,
	// so it is asked a few times and then left alone; the panes are then
	// loose, and dragging one back to an edge snaps it as it always does.
	if p.placed() || p.tries >= settleTries {
		p.stacked = true
	}
}

// settleTries is how many clock ticks the stack is asked for before the
// desktop's answer is taken as final.
const settleTries = 20

// placed reports whether every pane's window is where the rack says.
func (p *Player) placed() bool {
	for i := 0; i < p.Desk.Rack.Len(); i++ {
		pane, w := p.Desk.Rack.Pane(i), p.Desk.Window(i)
		if pane == nil || w == nil || !pane.Shown {
			continue
		}
		if x, y, ok := w.Position(); !ok || x != pane.Box.X || y != pane.Box.Y {
			return false
		}
	}
	return true
}

// refresh puts the model back into the controls and repaints. Everything
// here is cheap and idempotent, so it runs on every tick rather than being
// threaded through twenty callbacks.
func (p *Player) refresh() {
	p.autoPreset()
	if p.strip != nil {
		p.strip.sync()
	}
	if p.eqPane != nil {
		p.eqPane.sync()
	}
	if p.listPane != nil {
		p.listPane.sync()
	}
}

// Command runs one of the shared transport commands through the
// application, which keeps the clock going behind it.
func (p *Player) Command(c players.Command) { p.Host.Command(c) }

// Keys is the handler every one of the three windows shares, so the keys
// work wherever the focus is. It returns false for anything it does not
// know, which is what lets Tab, the arrows inside a list and the focus ring
// keep working.
//
// It is handed the window the key came from rather than reading the main
// one: the guard below asks what has the keyboard, and in a player with
// three windows that is a different answer in each.
func (p *Player) Keys(w *app.Window, e widget.KeyEvent) bool {
	if p.skinKeys(w, e, w.Content()) {
		return true
	}
	// Everything else — the transport table, the face switch, Escape — is
	// the application's, and is the same in every face.
	return p.Host.Keys(w, e)
}

// Status is the one line the strip shows about itself, and what the
// screenshots are read against.
func (p *Player) Status() string {
	t := p.Transport
	where := "loose"
	if pane := p.Desk.Rack.Pane(p.iEq); pane != nil && pane.To >= 0 {
		where = "stacked"
	}
	if !p.Desk.Places() {
		where = "this desktop places windows"
	}
	return fmt.Sprintf("%s · %s · %s", t.State, players.Percent(t.Volume), where)
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// autoPreset is the equaliser's AUTO key at work: once per track, a preset
// chosen by the track's place in the library — which is what a player of
// the era did from a file's genre tag, and what an invented library without
// tags can do honestly.
func (p *Player) autoPreset() {
	i := p.Transport.List.Index()
	if !p.auto || i == p.lastTrack {
		return
	}
	p.lastTrack = i
	if pr := players.EqPresets(); len(pr) > 0 && i >= 0 {
		p.Equaliser.Apply(pr[i%len(pr)].Name)
	}
}

// restyle is what follows a change of face: the focus, which must not be
// left on a control the new face has hidden.
//
// The titles do not change. The panels print theirs in capitals, as the era
// did, and that is each skin's own choice about how its caption reads
// ("case": "upper" on its caption's text role): the windows keep the
// player's titles, which is what a screen reader and the desktop's window
// list read.
func (p *Player) restyle() {
	f := faceOf(p.App.Look())
	if p.strip != nil {
		p.strip.show(f)
		keepFocus(p.Main, p.strip.play)
	}
	if p.eqPane != nil {
		p.eqPane.show(f)
		keepFocus(p.Eq, p.eqPane.preamp)
	}
	if p.listPane != nil {
		p.listPane.show(f)
		keepFocus(p.List, p.listPane.list)
	}
}

// keepFocus moves a window's focus to fallback when the control that had it
// is no longer shown. A face that hides the key the keyboard was on hands the
// keyboard to the obvious next thing rather than to nothing.
func keepFocus(w *app.Window, fallback widget.Component) {
	if w == nil {
		return
	}
	f := w.Focus()
	if f == nil {
		return
	}
	for c := f; c != nil; c = c.Parent() {
		if !c.Visible() {
			w.RequestFocus(fallback)
			return
		}
	}
}
