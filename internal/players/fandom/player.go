// Package fandom presents the silver reference adaptation using the shared
// player model and public toolkit controls. No original skin archive has been
// identified for this reference.
package fandom

import (
	"fmt"
	"time"

	"github.com/codemodify/media-player-music/internal/players"
	fdlayout "github.com/codemodify/media-player-music/internal/skins/fandom"
	"goframework.io/desktop-ui"
	"goframework.io/desktop-ui-rack"
	"goframework.io/desktop-ui-skins"
	"goframework.io/desktop-ui/platform"
	"goframework.io/desktop-ui/widget"
	"goframework.io/graphics-paint2d"
)

const (
	LayoutMain     = fdlayout.LayoutMain
	LayoutPlaylist = fdlayout.LayoutPlaylist
	MainW          = 490
	MainH          = 330
	ListW          = 190
	ListH          = 246
	MinListW       = ListW
	MinListH       = ListH
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
	listPane  *playlistPane
	dropLook  func()
	// g is the two panels, the player and its playlist; iMain and iList
	// are their members.
	g            *rack.Group
	iMain, iList int
	// onClose is the application's word on closing the player (SetOnClose).
	onClose        func() bool
	utilityOwnerID string
}

func New(h *players.Host, opts Options) (*Player, error) {
	for _, role := range []string{LayoutMain, LayoutPlaylist} {
		if _, ok := skins.SkinLayoutOf(h.App.Look(), role); !ok {
			return nil, fmt.Errorf("Fandom requires skin layout %q", role)
		}
	}
	p := &Player{Host: h, App: h.App, Transport: h.Model.Transport, Spectrum: h.Model.Spectrum, Equaliser: h.Model.Equaliser}
	p.g = rack.NewGroup(h.App, rack.GroupOptions{Mode: opts.Group})
	p.body = newBody(p)
	p.listPane = newPlaylistPane(p)
	mw, mh := players.WindowSize(h.App, MainW, MainH)
	p.iMain = p.g.Add(rack.Member{Name: "main", Content: p.body, OnClose: p.closeMain, Shape: p.shape(0, LayoutMain),
		Window: platform.WindowOptions{Title: "Music Player", Headless: opts.Headless, Width: mw, Height: mh, Sizing: platform.SizingFixed}})
	lw, lh := players.WindowSize(h.App, ListW, ListH)
	p.iList = p.g.Add(rack.Member{Name: "playlist", Content: p.listPane, Shape: p.shape(1, LayoutPlaylist),
		OnClose: func() bool { h.Model.PlaylistVisible = false; return true },
		Window: platform.WindowOptions{Title: "Playlist", Headless: opts.Headless, Width: lw, Height: lh,
			MinWidth: MinListW, MinHeight: MinListH, Hidden: !h.Model.PlaylistVisible}})
	// The reference arrangement: the playlist down the player's right edge.
	p.g.Stick(p.iList, p.iMain, rack.Bond{Side: rack.SideRight, Along: 46})
	if err := p.g.Realise(); err != nil {
		return nil, err
	}
	p.Transport.Changed, p.Equaliser.Changed = p.Refresh, p.Refresh
	p.dropLook = p.App.OnLookChange(func() {
		p.updateShape()
		p.g.SetShape(p.iList, p.shape(p.iList, LayoutPlaylist))
		p.Refresh()
	})
	p.configureUtilityWindows()
	p.Refresh()
	p.Main().RequestFocus(p.body.play)
	if w := p.List(); w != nil && w != p.Main() {
		p.List().RequestFocus(p.listPane.list)
	}
	return p, nil
}

// Main is the player's window, and List the window the playlist is in now:
// its own, or the player's where the two share one (nil while it is hidden
// there).
func (p *Player) Main() *ui.Window { return p.g.Window(p.iMain) }
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
	p.g.Close()
}
func (p *Player) Pose(pos time.Duration) { p.Host.Pose(pos); p.Refresh() }
func (p *Player) Advance(time.Duration) {
	p.configureUtilityWindows()
	p.Refresh()
}
func (p *Player) Refresh() {
	if w := p.Main(); w == nil || w.Closed() || p.body == nil || p.listPane == nil {
		return
	}
	p.Equaliser.AutoForTrack(p.Transport.List.Index())
	p.body.sync()
	p.listPane.sync()
}
func (p *Player) ShowEqualiser(on bool) {
	p.Host.Model.EqualizerVisible = on
	p.body.sync()
	if c := p.Main().Focus(); c != nil && !c.Visible() {
		p.Main().RequestFocus(p.body.play)
	}
	p.body.RequestLayout()
}
func (p *Player) ShowPlaylist(on bool) {
	p.Host.Model.PlaylistVisible = on
	p.g.Show(p.iList, on)
	p.configureUtilityWindows()
	p.Refresh()
}

func (p *Player) ShowSettingsPanel(on bool) {
	if p.body != nil {
		p.body.setSettingsVisible(on)
	}
}

// updateShape is called after collapsing or reopening the embedded settings
// wing. Only the visible artwork participates in input and painted coverage.
func (p *Player) updateShape() {
	if p.g == nil {
		return
	}
	p.g.SetShape(p.iMain, p.shape(p.iMain, LayoutMain))
	p.configureUtilityWindows()
}

// shape is member i's silhouette, drawn from its role's artwork in the look
// of the window it is in.
func (p *Player) shape(i int, role string) func(paint2d.Point, float32) *platform.Shape {
	return func(size paint2d.Point, _ float32) *platform.Shape {
		lk := p.App.Look()
		if w := p.g.Window(i); w != nil {
			lk = w.Look()
		}
		mask := paint2d.NewImage(int(size.X+.5), int(size.Y+.5))
		ctx := paint2d.NewContext(mask)
		ctx.Clear(paint2d.Transparent)
		box := paint2d.XYWH(0, 0, size.X, size.Y)
		slots := []string{"shape.playlist"}
		if role == LayoutMain {
			slots = []string{"shape.main"}
			if p.body != nil && p.body.settingsVisible {
				slots = append(slots, "shape.settings")
			}
		}
		for _, slot := range slots {
			r, ok := skins.SkinSlotRect(lk, role, slot, box)
			if !ok || !skins.DrawSkinSlot(lk, ctx, r, role, slot, 0) {
				return nil
			}
		}
		// The artwork already carries soft edge alpha. The window mask
		// provides its extent only; applying the same fractional coverage
		// again would multiply the painted alpha by itself.
		for at := 3; at < len(mask.Pix); at += 4 {
			if mask.Pix[at] != 0 {
				mask.Pix[at-3], mask.Pix[at-2], mask.Pix[at-1], mask.Pix[at] = 255, 255, 255, 255
			}
		}
		return platform.NewShapeImage(mask)
	}
}

// configureUtilityWindows refreshes the playlist's owner when the player's
// native identity changes, where each panel has a window of its own.
func (p *Player) configureUtilityWindows() {
	main, list := p.Main(), p.List()
	if p.g.Mode() != rack.GroupWindows || main == nil || main.Closed() || list == nil || list.Closed() || list == main {
		return
	}
	if id := main.PortalParent(); id != "" && id != p.utilityOwnerID {
		// v0.23.4 recreates child hints but leaves an extant child's owner
		// pointing at a destroyed parent. Retain the public-owner refresh.
		list.SetOwner(main)
		p.utilityOwnerID = id
	}
}

// inPlaylist reports whether w's keyboard focus is in the playlist's pane:
// the playlist may share its window with the player.
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
	if e.Key == platform.KeyMenu || e.Key == platform.KeyF10 && e.Mods.Shift() {
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
