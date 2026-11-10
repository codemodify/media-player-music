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
}

type Player struct {
	Host              *players.Host
	App               *ui.Application
	Transport         *players.Transport
	Spectrum          *players.Spectrum
	Equaliser         *players.Equalizer
	Main, List        *ui.Window
	body              *body
	listPane          *playlistPane
	dropLook          func()
	Desk              *rack.Desk
	iMain, iList      int
	stacked           bool
	tries             int
	restoredPlacement map[string]players.WindowPlacement
	utilityOwnerID    string
}

func New(h *players.Host, opts Options) (*Player, error) {
	for _, role := range []string{LayoutMain, LayoutPlaylist} {
		if _, ok := skins.SkinLayoutOf(h.App.Look(), role); !ok {
			return nil, fmt.Errorf("Fandom requires skin layout %q", role)
		}
	}
	p := &Player{Host: h, App: h.App, Transport: h.Model.Transport, Spectrum: h.Model.Spectrum, Equaliser: h.Model.Equaliser}
	var err error
	p.Main, err = players.OpenSized(h.App, platform.WindowOptions{Title: "Music Player", Headless: opts.Headless, Decorations: platform.DecorationsNone}, MainW, MainH, true)
	if err != nil {
		return nil, err
	}
	p.body = newBody(p)
	p.Main.SetContent(p.body)
	p.setShape(p.Main, LayoutMain)
	p.List, err = players.OpenSized(h.App, platform.WindowOptions{Title: "Playlist", Headless: opts.Headless, Decorations: platform.DecorationsNone,
		Role: platform.RoleUtility, Owner: p.Main.Surface(), MinWidth: MinListW, MinHeight: MinListH}, ListW, ListH, false)
	if err != nil {
		p.Main.Close()
		return nil, err
	}
	p.listPane = newPlaylistPane(p)
	p.List.SetContent(p.listPane)
	p.setShape(p.List, LayoutPlaylist)
	p.initGroup()
	p.List.SetOnCloseRequest(func() bool { p.ShowPlaylist(false); return false })
	p.Transport.Changed, p.Equaliser.Changed = p.Refresh, p.Refresh
	p.dropLook = p.App.OnLookChange(func() {
		p.setShape(p.Main, LayoutMain)
		p.setShape(p.List, LayoutPlaylist)
		p.configureUtilityWindows()
		p.Refresh()
	})
	p.ShowPlaylist(h.Model.PlaylistVisible)
	p.configureUtilityWindows()
	p.Refresh()
	p.Main.RequestFocus(p.body.play)
	p.List.RequestFocus(p.listPane.list)
	return p, nil
}

func (p *Player) MainWindow() *ui.Window { return p.Main }
func (p *Player) Windows() []players.NamedWindow {
	return []players.NamedWindow{{Name: "main", Window: p.Main}, {Name: "playlist", Window: p.List}}
}
func (p *Player) Close() {
	p.restoredPlacement = nil
	if p.dropLook != nil {
		p.dropLook()
		p.dropLook = nil
	}
	for _, w := range []*ui.Window{p.List, p.Main} {
		if w != nil {
			w.Close()
		}
	}
}
func (p *Player) Pose(pos time.Duration)   { p.Host.Pose(pos); p.Refresh() }
func (p *Player) Advance(dt time.Duration) { p.AdvanceGeometry(dt); p.Refresh() }
func (p *Player) Refresh() {
	if p.Main == nil || p.Main.Closed() || p.body == nil || p.listPane == nil {
		return
	}
	p.Equaliser.AutoForTrack(p.Transport.List.Index())
	p.body.sync()
	p.listPane.sync()
}
func (p *Player) ShowEqualiser(on bool) {
	p.Host.Model.EqualizerVisible = on
	p.body.sync()
	if c := p.Main.Focus(); c != nil && !c.Visible() {
		p.Main.RequestFocus(p.body.play)
	}
	p.body.RequestLayout()
}
func (p *Player) ShowPlaylist(on bool) { p.showWindow(p.iList, on) }

func (p *Player) ShowSettingsPanel(on bool) {
	if p.body != nil {
		p.body.setSettingsVisible(on)
	}
}

// userMoved releases startup placement before a native drag or resize begins.
// A user gesture owns the whole group's geometry once restoration is cancelled.
func (p *Player) userMoved(name string) {
	if p.Desk == nil {
		return
	}
	if pane, _ := p.Desk.Rack.Find(name); pane == nil {
		return
	}
	p.restoredPlacement = nil
	p.stacked = true
}

// updateShape is called after collapsing or reopening the embedded settings
// wing. Only the visible artwork participates in input and painted coverage.
func (p *Player) updateShape() {
	p.setShape(p.Main, LayoutMain)
	p.configureUtilityWindows()
}

func (p *Player) setShape(w *ui.Window, role string) {
	w.SetShapeFunc(func(size paint2d.Point, _ float32) *platform.Shape {
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
			r, ok := skins.SkinSlotRect(w.Look(), role, slot, box)
			if !ok || !skins.DrawSkinSlot(w.Look(), ctx, r, role, slot, 0) {
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
	})
}

func (p *Player) configureUtilityWindows() {
	if p.Main == nil || p.Main.Closed() || p.List == nil || p.List.Closed() {
		return
	}
	if id := p.Main.PortalParent(); id != "" && id != p.utilityOwnerID {
		// v0.23.4 recreates child hints but leaves an extant child's owner
		// pointing at a destroyed parent. Retain the public-owner refresh.
		p.List.SetOwner(p.Main)
		p.utilityOwnerID = id
	}
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
		if w == p.List {
			p.listPane.contextMenu(from, at)
		} else {
			p.Host.ContextMenu(from, at)
		}
		return true
	}
	return p.Host.Keys(w, e)
}
