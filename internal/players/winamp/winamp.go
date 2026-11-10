// Package winamp builds the application's Winamp panel presentations from
// application-owned skin layouts and artwork. Each presentation has a main
// strip, an equaliser and a playlist sharing the application's player model.
// Playback actions go through the shared host and its system mpv backend.
package winamp

import (
	"fmt"
	"time"

	"github.com/codemodify/media-player-music/internal/players"
	"goframework.io/desktop-ui"
	"goframework.io/desktop-ui-rack"
	"goframework.io/desktop-ui/platform"
	"goframework.io/desktop-ui/widget"
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

// Skin is the classic Winamp panel pack.
const Skin = players.WinampSkin

// Player is the three panels and the one model under them. The model is
// the application's (players.Host): this face shows it, and putting on
// another face leaves it exactly where it was.
type Player struct {
	Host *players.Host
	App  *ui.Application
	// g is the panels: a window each, snapped together and travelling
	// together, where the desktop lets a program place windows; the panels
	// stuck together sharing one where it does not (Wayland).
	g *rack.Group

	Transport *players.Transport
	Spectrum  *players.Spectrum
	Equaliser *players.Equalizer

	strip           *strip
	eqPane          *eqPane
	listPane        *listPane
	playlistCaption *playlistCaption
	// dropLook takes the look watcher off again when the face is closed.
	dropLook func()

	// iMain, iEq, iList are the panels' members (-1 for one not opened).
	iMain, iEq, iList int
	// listSize is the restored playlist size, kept while the group
	// settles (R21).
	listSize players.HeldSize
	// onClose is the application's word on closing the player (SetOnClose).
	onClose        func() bool
	utilityOwnerID string
}

// Options is what the command line passes in.
type Options struct {
	Headless bool
	// Scale is the display scale; 0 takes the desktop's.
	Scale float32
	// NoEq and NoList open the main strip on its own.
	NoEq, NoList bool
	// Group is how the panels are realised: the zero lets the desktop
	// decide; rack.GroupHosted shares windows wherever it runs.
	Group rack.GroupMode
}

// New opens the player's windows on the application's own model.
//
// Each panel wears the toolkit's frame for its role, the name of the layout
// it is laid out by: the skin's caption band is the title strip a player of
// this shape had, and a skin can dress one panel differently from the
// others (Winamp Silver's equaliser has a tab for a header where the other
// two have a band). A look that says nothing about the role gives every
// panel its one frame. With a window each, the toolkit frames each window;
// sharing one, the group frames each panel in it the same way.
func New(h *players.Host, opts Options) (*Player, error) {
	a := h.App
	if !faceOf(a.Look()).panelled() {
		return nil, fmt.Errorf("Winamp requires a skin with layout %q", layoutStrip)
	}
	p := &Player{
		Host:      h,
		App:       a,
		Transport: h.Model.Transport,
		Spectrum:  h.Model.Spectrum,
		Equaliser: h.Model.Equaliser,
		iEq:       -1,
		iList:     -1,
	}
	// The rack speaks logical pixels, so its reach grows with the display
	// like every other measurement in the toolkit: ten pixels of slop is a
	// fingertip at 1× and at 2× alike.
	p.g = rack.NewGroup(a, rack.GroupOptions{Mode: opts.Group})
	window := func(title string, w, h int, hidden bool) platform.WindowOptions {
		o := platform.WindowOptions{Title: title, Headless: opts.Headless, Hidden: hidden, Sizing: platform.SizingFixed}
		o.Width, o.Height = players.WindowSize(a, w, h)
		return o
	}
	p.strip = newStrip(p)
	p.iMain = p.g.Add(rack.Member{Name: "main", Content: p.strip, OnClose: p.closeMain, FrameRole: layoutStrip,
		Window: window("Music Player", StripW, StripH, false)})
	if !opts.NoEq {
		p.eqPane = newEqPane(p)
		p.iEq = p.g.Add(rack.Member{Name: "equaliser", Content: p.eqPane, FrameRole: layoutEqualiser,
			OnClose: func() bool { h.Model.EqualizerVisible = false; a.Post(p.refresh); return true },
			Window:  window("Equaliser", StripW, EqH, !h.Model.EqualizerVisible)})
		p.g.Attach(p.iEq, p.iMain, rack.SideBottom)
	}
	if !opts.NoList {
		p.listPane = newListPane(p)
		o := window("Playlist", StripW, ListH, !h.Model.PlaylistVisible)
		o.Sizing, o.MinWidth, o.MinHeight = platform.SizingResizable, StripW, ListH
		p.iList = p.g.Add(rack.Member{Name: "playlist", Content: p.listPane, FrameRole: layoutPlaylist,
			OnClose: func() bool { h.Model.PlaylistVisible = false; a.Post(p.refresh); return true },
			Window:  o})
		p.g.Attach(p.iList, maxInt(p.iEq, p.iMain), rack.SideBottom)
	}
	p.g.Moved = p.syncPanels
	if err := p.g.Realise(); err != nil {
		return nil, err
	}

	p.Transport.Changed = p.refresh
	p.Equaliser.Changed = p.refresh
	// A look can change under the player from outside it too — Settings,
	// an edited look.json, style.ThemeEnv on a watcher — and the titles and the
	// focus follow the face whichever way it changed.
	p.dropLook = a.OnLookChange(p.restyle)
	p.restyle()
	p.refresh()
	// Something in every window has the keyboard from the first frame. A
	// player's keys are bare letters that bubble up from whatever is
	// focused, and a window with no focus at all has nothing for them to
	// bubble from — so the equaliser and the playlist are given one too,
	// or the transport keys would be dead in two windows out of three.
	// Panels sharing a window share its focus, which stays the strip's.
	p.Main().RequestFocus(p.strip.play)
	if w := p.Eq(); p.eqPane != nil && w != nil && w != p.Main() {
		w.RequestFocus(p.eqPane.preamp)
	}
	if w := p.List(); p.listPane != nil && w != nil && w != p.Main() && w != p.Eq() {
		w.RequestFocus(p.listPane.list)
	}
	return p, nil
}

// Main is the strip's window, and Eq and List the windows the equaliser and
// the playlist are in now: their own, or one they share with what they are
// stuck to; nil for a panel not opened, or hidden in a shared window.
func (p *Player) Main() *ui.Window { return p.g.Window(p.iMain) }
func (p *Player) Eq() *ui.Window   { return p.g.Window(p.iEq) }
func (p *Player) List() *ui.Window { return p.g.Window(p.iList) }

// syncPanels keeps the model's panel switches to what the group shows: a
// window the desktop closed hides every panel in it.
func (p *Player) syncPanels() {
	if pane := p.g.Rack.Pane(p.iEq); pane != nil && p.iEq >= 0 {
		p.Host.Model.EqualizerVisible = pane.Shown
	}
	if pane := p.g.Rack.Pane(p.iList); pane != nil && p.iList >= 0 {
		p.Host.Model.PlaylistVisible = pane.Shown
	}
}

// Close takes the face off: its windows go, and the model they were showing
// stays exactly as it is.
func (p *Player) Close() {
	if p.dropLook != nil {
		p.dropLook()
		p.dropLook = nil
	}
	p.g.Close()
}

// Windows are the face's windows, named, for a caller taking a picture of
// each: each window once, where panels share one.
func (p *Player) Windows() []players.NamedWindow {
	return players.GroupWindows(p.g, "strip", "equaliser", "playlist")
}

// MainWindow is the window the application's clock wakes on.
// SetOnClose is what closing the player asks first, false keeping it: its
// close button, or the desktop closing its window. It holds whichever window
// the player is in, as the group's first member's OnClose.
func (p *Player) SetOnClose(fn func() bool) { p.onClose = fn }

// closeMain is the player closed: the application's word, else it goes.
func (p *Player) closeMain() bool { return p.onClose == nil || p.onClose() }

func (p *Player) MainWindow() *ui.Window { return p.Main() }

func (p *Player) ShowEqualiser(on bool) { p.showWindow(p.iEq, on) }
func (p *Player) ShowPlaylist(on bool)  { p.showWindow(p.iList, on) }

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

// Advance keeps what the desktop may remake under the player: the
// playlist caption's drop policy and the satellites' owner. The group keeps
// the panels together itself, on the desktop's word.
func (p *Player) Advance(time.Duration) {
	p.listSize.Keep(p.g)
	p.bindPlaylistCaption()
	p.configureUtilityWindows()
	p.refresh()
}

// Refresh puts the model back into the controls.
func (p *Player) Refresh() { p.refresh() }

// refresh puts the model back into the controls and repaints. Everything
// here is cheap and idempotent, so it runs on every tick rather than being
// threaded through twenty callbacks.
func (p *Player) refresh() {
	p.Equaliser.AutoForTrack(p.Transport.List.Index())
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
func (p *Player) Keys(w *ui.Window, e widget.KeyEvent) bool {
	if p.skinKeys(w, e, w.Content()) {
		return true
	}
	// The transport shortcuts and Escape belong to the application.
	return p.Host.Keys(w, e)
}

// Status is the one line the strip shows about itself, and what the
// screenshots are read against.
func (p *Player) Status() string {
	t := p.Transport
	where := "loose"
	if pane := p.g.Rack.Pane(p.iEq); pane != nil && pane.To >= 0 {
		where = "stacked"
	}
	if p.g.Limits().Has(rack.LimitPlace) {
		where = "the panels share a window on this desktop"
	}
	return fmt.Sprintf("%s · %s · %s", t.State, players.Percent(t.Volume), where)
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// restyle updates accessible skin labels and keeps focus on visible controls.
func (p *Player) restyle() {
	p.bindPlaylistCaption()
	if !faceOf(p.App.Look()).panelled() {
		return
	}
	p.configureUtilityWindows()
	if p.strip != nil {
		p.strip.show()
		keepFocus(p.Main(), p.strip.play)
	}
	if p.eqPane != nil && p.Eq() != p.Main() {
		keepFocus(p.Eq(), p.eqPane.preamp)
	}
	if p.listPane != nil && p.List() != p.Main() && p.List() != p.Eq() {
		keepFocus(p.List(), p.listPane.list)
	}
}

// configureUtilityWindows refreshes ownership only when Main's native identity
// changes, where each panel has a window of its own. v0.23.4 restores a
// recreated child's hints, but does not update existing children when their
// owner is recreated. Keep this public-API refresh until the toolkit carries
// owner relationships across that lifecycle change.
func (p *Player) configureUtilityWindows() {
	main := p.Main()
	if p.g.Mode() != rack.GroupWindows || main == nil || main.Closed() {
		return
	}
	ownerID := main.PortalParent()
	if ownerID == "" || ownerID == p.utilityOwnerID {
		return
	}
	for _, window := range []*ui.Window{p.Eq(), p.List()} {
		if window != nil && window != main && !window.Closed() {
			window.SetOwner(main)
		}
	}
	p.utilityOwnerID = ownerID
}

// keepFocus moves a window's focus to fallback when the control that had it
// is no longer shown. A face that hides the key the keyboard was on hands the
// keyboard to the obvious next thing rather than to nothing.
func keepFocus(w *ui.Window, fallback widget.Component) {
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
