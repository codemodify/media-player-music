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

// Player is the three windows and the one model under them. The model is
// the application's (players.Host): this face shows it, and putting on
// another face leaves it exactly where it was.
type Player struct {
	Host *players.Host
	App  *ui.Application
	Desk *rack.Desk

	Transport *players.Transport
	Spectrum  *players.Spectrum
	Equaliser *players.Equalizer

	Main, Eq, List  *ui.Window
	strip           *strip
	eqPane          *eqPane
	listPane        *listPane
	playlistCaption *playlistCaption
	// dropLook takes the look watcher off again when the face is closed.
	dropLook func()

	// iMain, iEq, iList are the panes in the desk's rack.
	iMain, iEq, iList int
	// stacked is whether the three have been put in a stack yet, and tries
	// how many times settle has asked — see settle, which cannot run until
	// the desktop has placed them and may not be obeyed when it does.
	stacked bool
	tries   int
	// restoredPlacement survives the desktop's initial mapping/configure.
	// Once the requested arrangement arrives, ordinary dragging owns it again.
	restoredPlacement map[string]players.WindowPlacement
	utilityOwnerID    string
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
	main, err := p.open(a, opts, "Music Player", layoutStrip, StripW, StripH)
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
		w, err := p.open(a, opts, "Equaliser", layoutEqualiser, StripW, EqH)
		if err != nil {
			return nil, err
		}
		p.Eq = w
		p.eqPane = newEqPane(p)
		w.SetContent(p.eqPane)
		p.iEq = p.Desk.Add("equaliser", w)
		p.Desk.Attach(p.iEq, p.iMain, rack.SideBottom)
		w.SetOnCloseRequest(func() bool {
			p.showWindow(p.iEq, false)
			return false
		})
		p.Desk.Show(p.iEq, h.Model.EqualizerVisible)
	}
	if !opts.NoList {
		w, err := p.open(a, opts, "Playlist", layoutPlaylist, StripW, ListH)
		if err != nil {
			return nil, err
		}
		p.List = w
		p.listPane = newListPane(p)
		w.SetContent(p.listPane)
		p.iList = p.Desk.Add("playlist", w)
		p.Desk.Attach(p.iList, maxInt(p.iEq, p.iMain), rack.SideBottom)
		w.SetOnCloseRequest(func() bool {
			p.showWindow(p.iList, false)
			return false
		})
		p.Desk.Show(p.iList, h.Model.PlaylistVisible)
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
// skin can dress one window differently from the others: Winamp Silver's
// equaliser has a tab for a header where the other two have a band. A look
// that says nothing about the role gives every window its one frame.
func (p *Player) open(a *ui.Application, opts Options, title, role string, w, h int) (*ui.Window, error) {
	windowRole := platform.RoleNormal
	if role != layoutStrip {
		// Satellite panels belong to Main and stay out of the desktop's
		// taskbar and pager on backends supporting utility policy.
		windowRole = platform.RoleUtility
	}
	windowOptions := platform.WindowOptions{
		Title:       title,
		Headless:    opts.Headless,
		Decorations: platform.DecorationsClient,
		Role:        windowRole,
	}
	if role != layoutStrip {
		windowOptions.Owner = p.Main.Surface()
	}
	if role == layoutPlaylist {
		windowOptions.MinWidth, windowOptions.MinHeight = StripW, ListH
	}
	win, err := players.OpenSized(a, windowOptions, w, h, role != layoutPlaylist)
	if err != nil {
		return nil, err
	}
	win.SetFrameRole(role)
	if role != layoutStrip {
		win.SetCaptionButtonVisible(platform.CaptionMinimize, false)
		win.SetCaptionButtonVisible(platform.CaptionMaximize, false)
	}
	return win, nil
}

// Close takes the face off: its windows go, and the model they were showing
// stays exactly as it is.
func (p *Player) Close() {
	if p.dropLook != nil {
		p.dropLook()
		p.dropLook = nil
	}
	for _, w := range []*ui.Window{p.List, p.Eq, p.Main} {
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
func (p *Player) MainWindow() *ui.Window { return p.Main }

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

// Advance settles startup and maintains resized geometry. Desk handles native
// move bursts and request deadlines independently of the playback clock;
// backends without those events retain polling. A zero-duration call explicitly
// observes geometry, as capture/restore and scripted callers expect.
func (p *Player) Advance(dt time.Duration) {
	p.bindPlaylistCaption()
	p.configureUtilityWindows()
	p.settle()
	if p.restoredPlacement == nil && p.needsFollow(dt == 0) {
		p.followWindows()
	}
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
	if p.restoredPlacement != nil {
		p.restorePlaylistSize()
		p.applyPlacement(p.restoredPlacement)
		p.Desk.Apply()
		if p.placed() || p.tries >= settleTries {
			p.stacked = true
			p.restoredPlacement = nil
		}
		return
	}
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
		if pane == nil || w == nil || w.Closed() || !pane.Shown {
			continue
		}
		if x, y, ok := w.Position(); !ok || x != pane.Box.X || y != pane.Box.Y {
			return false
		}
		if width, height := w.Size(); width != pane.Box.W || height != pane.Box.H {
			return false
		}
	}
	return true
}

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

// restyle updates accessible skin labels and keeps focus on visible controls.
func (p *Player) restyle() {
	p.bindPlaylistCaption()
	if !faceOf(p.App.Look()).panelled() {
		return
	}
	p.configureUtilityWindows()
	if p.strip != nil {
		p.strip.show()
		keepFocus(p.Main, p.strip.play)
	}
	if p.eqPane != nil {
		keepFocus(p.Eq, p.eqPane.preamp)
	}
	if p.listPane != nil {
		keepFocus(p.List, p.listPane.list)
	}
}

// configureUtilityWindows refreshes ownership only when Main's native identity
// changes. v0.23.4 restores a recreated child's hints, but does not update
// existing children when their owner is recreated. Keep this public-API refresh
// until the toolkit carries owner relationships across that lifecycle change.
func (p *Player) configureUtilityWindows() {
	if p.Main == nil || p.Main.Closed() {
		return
	}
	ownerID := p.Main.PortalParent()
	if ownerID == "" || ownerID == p.utilityOwnerID {
		return
	}
	for _, window := range []*ui.Window{p.Eq, p.List} {
		if window != nil && !window.Closed() {
			window.SetOwner(p.Main)
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
