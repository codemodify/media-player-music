// Package fancy presents the sculpted cyan player with shared playback state.
package fancy

import (
	"fmt"
	"time"

	"github.com/codemodify/media-player-music/internal/players"
	"goframework.io/desktop-ui"
	"goframework.io/desktop-ui-rack"
	"goframework.io/desktop-ui-skins"
	"goframework.io/desktop-ui-widgets"
	"goframework.io/desktop-ui/platform"
	"goframework.io/desktop-ui/widget"
	"goframework.io/graphics-paint2d"
)

const (
	LayoutMain      = "fancy.main"
	LayoutEqualizer = "fancy.equalizer"
	LayoutPlaylist  = "fancy.playlist"
	LayoutCompact   = "fancy.compact"
	MainW           = 420
	MainH           = 220
	CompactH        = 40
	EqW             = 420
	EqH             = 124
	ListW           = 420
	ListH           = 144
	MinListW        = ListW
	MinListH        = 124
)

type Options struct {
	Headless bool
	Scale    float32
	// Group is how the panels are realised: the zero lets the desktop
	// decide; rack.GroupHosted shares windows wherever it runs.
	Group rack.GroupMode
}
type Player struct {
	Host          *players.Host
	App           *ui.Application
	Transport     *players.Transport
	Spectrum      *players.Spectrum
	Equaliser     *players.Equalizer
	body, compact *mainPane
	eqPane        *equalizerPane
	listPane      *playlistPane
	compactMode   bool
	modePending   bool
	modeTries     int
	dropLook      func()
	dialogs       map[*widgets.FileDialog]bool
	// g is the three panels; iMain, iEq and iList are their members.
	g                 *rack.Group
	iMain, iEq, iList int
	// onClose is the application's word on closing the player (SetOnClose).
	onClose func() bool
}

func New(h *players.Host, opts Options) (*Player, error) {
	for _, role := range []string{LayoutMain, LayoutCompact, LayoutEqualizer, LayoutPlaylist} {
		if _, ok := skins.SkinLayoutOf(h.App.Look(), role); !ok {
			return nil, fmt.Errorf("Fancy requires skin layout %q", role)
		}
	}
	p := &Player{Host: h, App: h.App, Transport: h.Model.Transport, Spectrum: h.Model.Spectrum, Equaliser: h.Model.Equaliser}
	p.g = rack.NewGroup(h.App, rack.GroupOptions{Mode: opts.Group})
	p.body, p.compact = newMainPane(p, false), newMainPane(p, true)
	p.eqPane = newEqualizerPane(p)
	p.listPane = newPlaylistPane(p)
	window := func(title string, w, h int, fixed, hidden bool) platform.WindowOptions {
		o := platform.WindowOptions{Title: title, Headless: opts.Headless, Hidden: hidden}
		o.Width, o.Height = players.WindowSize(p.App, w, h)
		if fixed {
			o.Sizing = platform.SizingFixed
		} else {
			o.MinWidth, o.MinHeight = MinListW, MinListH
		}
		return o
	}
	p.iMain = p.g.Add(rack.Member{Name: "main", Content: p.body, OnClose: p.closeMain, Shape: p.shape(0, LayoutMain),
		Window: window("Music Player", MainW, MainH, true, false)})
	p.iEq = p.g.Add(rack.Member{Name: "equaliser", Content: p.eqPane, Shape: p.shape(1, LayoutEqualizer),
		OnClose: func() bool { h.Model.EqualizerVisible = false; return true },
		Window:  window("Equalizer", EqW, EqH, true, !h.Model.EqualizerVisible)})
	p.iList = p.g.Add(rack.Member{Name: "playlist", Content: p.listPane, Shape: p.shape(2, LayoutPlaylist),
		OnClose: func() bool { h.Model.PlaylistVisible = false; return true },
		Window:  window("Playlist", ListW, ListH, false, !h.Model.PlaylistVisible)})
	// The stack: the equaliser under the player, the playlist under it.
	p.g.Attach(p.iEq, p.iMain, rack.SideBottom)
	p.g.Attach(p.iList, p.iEq, rack.SideBottom)
	p.g.Moved = p.syncPanels
	if err := p.g.Realise(); err != nil {
		return nil, err
	}
	p.Transport.Changed, p.Equaliser.Changed = p.Refresh, p.Refresh
	p.dropLook = p.App.OnLookChange(func() {
		p.updateMainShape()
		p.g.SetShape(p.iEq, p.shape(p.iEq, LayoutEqualizer))
		p.g.SetShape(p.iList, p.shape(p.iList, LayoutPlaylist))
		p.Refresh()
	})
	p.Refresh()
	p.Main().RequestFocus(p.body.play)
	if w := p.Eq(); w != nil && w != p.Main() {
		w.RequestFocus(p.eqPane.bands[0])
	}
	if w := p.List(); w != nil && w != p.Main() && w != p.Eq() {
		w.RequestFocus(p.listPane.list)
	}
	return p, nil
}

// Main is the player's window, and Eq and List the windows the equaliser
// and the playlist are in now: their own, or one they share with what they
// are stuck to (nil while hidden there).
func (p *Player) Main() *ui.Window { return p.g.Window(p.iMain) }
func (p *Player) Eq() *ui.Window   { return p.g.Window(p.iEq) }
func (p *Player) List() *ui.Window { return p.g.Window(p.iList) }

// SetOnClose is what closing the player asks first, false keeping it: its
// close button, or the desktop closing its window. It holds whichever window
// the player is in, as the group's first member's OnClose.
func (p *Player) SetOnClose(fn func() bool) { p.onClose = fn }

// Settled reports whether the panels' group has put its windows in place
// (rack.Group.Settled). Until then a window the desktop moves is put back
// where its bond or its saved place says: the window manager placing it,
// not the user.
func (p *Player) Settled() bool { return p.g.Settled() }

// closeMain is the player closed: the application's word, else it goes.
func (p *Player) closeMain() bool { return p.onClose == nil || p.onClose() }

func (p *Player) MainWindow() *ui.Window { return p.Main() }
func (p *Player) Windows() []players.NamedWindow {
	return players.GroupWindows(p.g)
}
func (p *Player) Close() {
	if p.dropLook != nil {
		p.dropLook()
		p.dropLook = nil
	}
	for fd := range p.dialogs {
		fd.Cancel()
	}
	p.dialogs = nil
	p.g.Close()
}

// syncPanels keeps the model's panel switches to what the group shows: a
// window the desktop closed hides every panel in it.
func (p *Player) syncPanels() {
	if pane := p.g.Rack.Pane(p.iEq); pane != nil {
		p.Host.Model.EqualizerVisible = pane.Shown
	}
	if pane := p.g.Rack.Pane(p.iList); pane != nil {
		p.Host.Model.PlaylistVisible = pane.Shown
	}
}
func (p *Player) Pose(pos time.Duration) { p.Host.Pose(pos); p.Refresh() }
func (p *Player) Advance(time.Duration) {
	p.retryMode()
	p.Refresh()
}
func (p *Player) Refresh() {
	if w := p.Main(); w == nil || w.Closed() || p.eqPane == nil || p.listPane == nil {
		return
	}
	p.Equaliser.AutoForTrack(p.Transport.List.Index())
	p.body.sync()
	p.compact.sync()
	p.eqPane.sync()
	p.listPane.sync()
}
func (p *Player) ShowEqualiser(on bool) {
	p.Host.Model.EqualizerVisible = on
	p.g.Show(p.iEq, on)
	p.Refresh()
}
func (p *Player) ShowPlaylist(on bool) {
	p.Host.Model.PlaylistVisible = on
	p.g.Show(p.iList, on)
	p.Refresh()
}
func (p *Player) updateMainShape() {
	role := LayoutMain
	if p.Compact() {
		role = LayoutCompact
	}
	p.g.SetShape(p.iMain, p.shape(p.iMain, role))
}

// shape is member i's silhouette in a role, drawn from the role's artwork
// in the look of the window it is in.
func (p *Player) shape(i int, role string) func(paint2d.Point, float32) *platform.Shape {
	slot := "shape." + role[len("fancy."):]
	return func(size paint2d.Point, _ float32) *platform.Shape {
		lk := p.App.Look()
		if w := p.g.Window(i); w != nil {
			lk = w.Look()
		}
		im := paint2d.NewImage(int(size.X+.5), int(size.Y+.5))
		ctx := paint2d.NewContext(im)
		ctx.Clear(paint2d.Transparent)
		box := paint2d.XYWH(0, 0, size.X, size.Y)
		r, ok := skins.SkinSlotRect(lk, role, slot, box)
		if !ok || !skins.DrawSkinSlot(lk, ctx, r, role, slot, 0) {
			return nil
		}
		// Source artwork contains its antialias coverage. The window mask supplies
		// only the extent so those translucent edge pixels are applied once.
		for a := 3; a < len(im.Pix); a += 4 {
			if im.Pix[a] != 0 {
				im.Pix[a-3], im.Pix[a-2], im.Pix[a-1], im.Pix[a] = 255, 255, 255, 255
			}
		}
		return platform.NewShapeImage(im)
	}
}
func (p *Player) Compact() bool { return p.compactMode }
func (p *Player) SetCompact(on bool) {
	if w := p.Main(); w == nil || w.Closed() || p.compactMode == on {
		return
	}
	p.compactMode = on
	p.modeTries = 0
	p.modePending = true
	if p.g.Mode() == rack.GroupWindows {
		p.Main().SetFullscreen(false)
		p.Main().SetMaximized(false)
	}
	pane := p.body
	h := MainH
	if on {
		pane = p.compact
		h = CompactH
	}
	p.g.SetContent(p.iMain, pane)
	p.g.SetSize(p.iMain, MainW, h)
	p.updateMainShape()
	p.Refresh()
	p.Main().RequestFocus(pane.play)
}

// retryMode asks again for the player's size in its mode, where it has a
// window of its own and the desktop did not take the first request; sharing
// a window, the group sizes it.
func (p *Player) retryMode() {
	main := p.Main()
	if !p.modePending || main == nil || main.Closed() || p.g.Mode() != rack.GroupWindows {
		p.modePending = false
		return
	}
	height := MainH
	if p.Compact() {
		height = CompactH
	}
	w, h := main.Size()
	if w == MainW && h == height || p.modeTries >= modeTries {
		p.modePending = false
		return
	}
	p.modeTries++
	p.g.SetSize(p.iMain, MainW, height)
}

// modeTries is how many clock ticks a mode's size is asked for before the
// desktop's answer is taken as final.
const modeTries = 20

func (p *Player) CaptureState() players.PresentationState {
	mode := "full"
	if p.Compact() {
		mode = "compact"
	}
	return players.PresentationState{Mode: mode}
}
func (p *Player) RestoreState(s players.PresentationState) { p.SetCompact(s.Mode == "compact") }

// inPlaylist reports whether w's keyboard focus is in the playlist's pane:
// the playlist may share its window with the other panels.
func (p *Player) inPlaylist(w *ui.Window) bool {
	for c := w.Focus(); c != nil; c = c.Parent() {
		if c == p.listPane {
			return true
		}
	}
	return false
}
func (p *Player) keys(w *ui.Window, e widget.KeyEvent) bool {
	if e.Mods.Ctrl() {
		switch e.Key {
		case platform.KeyM:
			if e.Mods.Shift() {
				p.SetCompact(!p.Compact())
				return true
			}
		case platform.KeyE:
			p.ShowEqualiser(!p.Host.Model.EqualizerVisible)
			return true
		case platform.KeyL:
			p.ShowPlaylist(!p.Host.Model.PlaylistVisible)
			return true
		case platform.KeyK:
			if e.Mods.Shift() {
				p.Host.DropSkin()
			} else {
				p.Host.NextSkin()
			}
			return true
		}
	}
	if e.Key == platform.KeyMenu || e.Key == platform.KeyF10 && e.Mods.Shift() {
		from := w.Content()
		at := widget.DeviceOrigin(from).Add(paint2d.Pt(12*w.Scale(), 16*w.Scale()))
		if c := w.Focus(); c != nil {
			from = c
			at = widget.DeviceOrigin(c).Add(paint2d.Pt(0, c.Bounds().Dy()))
		}
		if p.inPlaylist(w) {
			p.listPane.contextMenu(from, at)
		} else {
			p.Host.ContextMenu(from, at)
		}
		return true
	}
	return p.Host.Keys(w, e)
}
