// Package halflife2 presents the original Half-Life 2 WMP artwork with
// application-owned controls and the shared playback model.
package halflife2

import (
	"fmt"
	"time"

	"github.com/codemodify/media-player-music/internal/players"
	"goframework.io/desktop-ui"
	"goframework.io/desktop-ui-rack"
	"goframework.io/desktop-ui-skins"
	"goframework.io/desktop-ui/platform"
	"goframework.io/desktop-ui/widget"
	"goframework.io/graphics-paint2d"
)

const (
	LayoutMain      = "halflife2.main"
	LayoutEqualizer = "halflife2.equalizer"
	LayoutPlaylist  = "halflife2.playlist"
	MainW           = 339
	MainH           = 300
	EqW             = 430
	EqH             = 239
	ListW           = 393
	ListH           = 226
	MinListW        = ListW
	MinListH        = ListH
)

type Options struct {
	Headless bool
	Scale    float32
	// Group is how the panels are realised: the zero lets the desktop
	// decide; rack.GroupHosted shares windows wherever it runs.
	Group rack.GroupMode
}
type Player struct {
	Host      *players.Host
	App       *ui.Application
	Transport *players.Transport
	Spectrum  *players.Spectrum
	Equaliser *players.Equalizer
	body      *body
	eqPane    *equalizerPane
	listPane  *playlistPane
	dropLook  func()
	// g is the three panels; iMain, iEq and iList are their members.
	g                 *rack.Group
	iMain, iEq, iList int
	// onClose is the application's word on closing the player (SetOnClose).
	onClose        func() bool
	utilityOwnerID string
}

func New(h *players.Host, opts Options) (*Player, error) {
	for _, role := range []string{LayoutMain, LayoutEqualizer, LayoutPlaylist} {
		if _, ok := skins.SkinLayoutOf(h.App.Look(), role); !ok {
			return nil, fmt.Errorf("Half Life 2 requires skin layout %q", role)
		}
	}
	p := &Player{Host: h, App: h.App, Transport: h.Model.Transport, Spectrum: h.Model.Spectrum, Equaliser: h.Model.Equaliser}
	p.g = rack.NewGroup(h.App, rack.GroupOptions{Mode: opts.Group})
	p.body = newBody(p)
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
	// The source skin's arrangement: the equaliser 80 pixels up the
	// player's right edge, the playlist under the equaliser.
	p.g.Stick(p.iEq, p.iMain, rack.Bond{Side: rack.SideRight, Along: -80})
	p.g.Attach(p.iList, p.iEq, rack.SideBottom)
	p.g.Moved = p.syncPanels
	if err := p.g.Realise(); err != nil {
		return nil, err
	}
	p.Transport.Changed, p.Equaliser.Changed = p.Refresh, p.Refresh
	p.dropLook = p.App.OnLookChange(func() {
		p.g.SetShape(p.iMain, p.shape(p.iMain, LayoutMain))
		p.g.SetShape(p.iEq, p.shape(p.iEq, LayoutEqualizer))
		p.g.SetShape(p.iList, p.shape(p.iList, LayoutPlaylist))
		p.configureUtilityWindows()
		p.Refresh()
	})
	p.configureUtilityWindows()
	p.Refresh()
	p.Main().RequestFocus(p.body.transportButton())
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
	p.configureUtilityWindows()
	p.Refresh()
}
func (p *Player) Refresh() {
	if w := p.Main(); w == nil || w.Closed() || p.body == nil || p.eqPane == nil || p.listPane == nil {
		return
	}
	p.Equaliser.AutoForTrack(p.Transport.List.Index())
	p.body.sync()
	p.eqPane.sync()
	p.listPane.sync()
}
func (p *Player) ShowEqualiser(on bool) {
	p.Host.Model.EqualizerVisible = on
	p.showPanel(p.iEq, on)
}
func (p *Player) ShowPlaylist(on bool) {
	p.Host.Model.PlaylistVisible = on
	p.showPanel(p.iList, on)
}
func (p *Player) showPanel(i int, on bool) {
	p.g.Show(i, on)
	p.configureUtilityWindows()
	p.Refresh()
}

// configureUtilityWindows refreshes the satellites' owner when the player's
// native identity changes, where each panel has a window of its own.
func (p *Player) configureUtilityWindows() {
	main := p.Main()
	if p.g.Mode() != rack.GroupWindows || main == nil || main.Closed() {
		return
	}
	id := main.PortalParent()
	if id == "" || id == p.utilityOwnerID {
		return
	}
	for _, w := range []*ui.Window{p.Eq(), p.List()} {
		if w != nil && w != main && !w.Closed() {
			w.SetOwner(main)
		}
	}
	p.utilityOwnerID = id
}

// shape is member i's silhouette, drawn from its role's artwork in the look
// of the window it is in; the player's follows its shutter.
func (p *Player) shape(i int, role string) func(paint2d.Point, float32) *platform.Shape {
	return func(size paint2d.Point, _ float32) *platform.Shape {
		lk := p.App.Look()
		if w := p.g.Window(i); w != nil {
			lk = w.Look()
		}
		im := paint2d.NewImage(int(size.X+.5), int(size.Y+.5))
		ctx := paint2d.NewContext(im)
		ctx.Clear(paint2d.Transparent)
		slot := "shape"
		if role == LayoutMain {
			slot += "." + []string{"open", "inner", "closed"}[p.body.shutter]
		}
		if !skins.DrawSkinSlot(lk, ctx, paint2d.XYWH(0, 0, size.X, size.Y), role, slot, 0) {
			return nil
		}
		return platform.NewShapeImage(im)
	}
}

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
	if e.Key == platform.KeyMenu || (e.Key == platform.KeyF10 && e.Mods.Shift()) {
		from := w.Content()
		at := paint2d.Pt(50*w.Scale(), 40*w.Scale())
		if c := w.Focus(); c != nil {
			from = c
			o := widget.DeviceOrigin(c)
			at = paint2d.Pt(o.X, o.Y+c.Bounds().Dy())
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
