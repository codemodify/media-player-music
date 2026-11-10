// Package xbox adapts the original Xbox Windows Media Player artwork to the
// application's shared playback model using public toolkit controls.
package xbox

import (
	"fmt"
	"time"

	"github.com/codemodify/media-player-music/internal/players"
	"goframework.io/desktop-ui"
	"goframework.io/desktop-ui-skins"
	"goframework.io/desktop-ui/platform"
	"goframework.io/desktop-ui/widget"
	"goframework.io/graphics-paint2d"
)

const (
	LayoutMain     = "xbox.main"
	LayoutPlaylist = "xbox.playlist"
	MainW          = 350
	MainH          = 216
	ListW          = 448
	ListH          = 316
	MinListW       = 438
	MinListH       = 316
)

type Options struct {
	Headless bool
	Scale    float32
}

type Player struct {
	Host           *players.Host
	App            *ui.Application
	Transport      *players.Transport
	Spectrum       *players.Spectrum
	Equaliser      *players.Equalizer
	Main, List     *ui.Window
	body           *body
	listPane       *playlistPane
	dropLook       func()
	dropResize     []func()
	placementReady map[string]bool
	restore        map[string]*placementRestore
	ownerID        string
}

func New(h *players.Host, opts Options) (*Player, error) {
	for _, role := range []string{LayoutMain, LayoutPlaylist} {
		if _, ok := skins.SkinLayoutOf(h.App.Look(), role); !ok {
			return nil, fmt.Errorf("Xbox requires a skin with layout %q", role)
		}
	}
	p := &Player{Host: h, App: h.App, Transport: h.Model.Transport, Spectrum: h.Model.Spectrum, Equaliser: h.Model.Equaliser,
		placementReady: make(map[string]bool), restore: make(map[string]*placementRestore)}
	w, err := players.OpenSized(h.App, platform.WindowOptions{Title: "Music Player", Headless: opts.Headless, Decorations: platform.DecorationsNone}, MainW, MainH, true)
	if err != nil {
		return nil, err
	}
	p.Main = w
	p.body = newBody(p)
	w.SetContent(p.body)
	p.setShape(w, LayoutMain)
	list, err := players.OpenSized(h.App, platform.WindowOptions{Title: "Playlist", Headless: opts.Headless, Decorations: platform.DecorationsNone,
		Role: platform.RoleUtility, Owner: w.Surface(), MinWidth: MinListW, MinHeight: MinListH}, ListW, ListH, false)
	if err != nil {
		w.Close()
		return nil, err
	}
	p.List = list
	p.listPane = newPlaylistPane(p)
	list.SetContent(p.listPane)
	p.setShape(list, LayoutPlaylist)
	list.SetOnCloseRequest(func() bool { p.ShowPlaylist(false); return false })
	for _, item := range p.Windows() {
		item := item
		p.dropResize = append(p.dropResize, item.Window.OnResize(func(_, _ int) {
			p.placementReady[item.Name] = true
			p.retryPlacement(0)
		}))
	}
	p.Transport.Changed, p.Equaliser.Changed = p.Refresh, p.Refresh
	p.dropLook = p.App.OnLookChange(func() {
		p.setShape(p.Main, LayoutMain)
		p.setShape(p.List, LayoutPlaylist)
		p.refreshOwner()
		p.Refresh()
	})
	p.ShowPlaylist(h.Model.PlaylistVisible)
	p.refreshOwner()
	p.Refresh()
	w.RequestFocus(p.body.play)
	list.RequestFocus(p.listPane.list)
	return p, nil
}

func (p *Player) MainWindow() *ui.Window { return p.Main }
func (p *Player) Windows() []players.NamedWindow {
	return []players.NamedWindow{{Name: "main", Window: p.Main}, {Name: "playlist", Window: p.List}}
}
func (p *Player) Close() {
	p.restore = nil
	if p.dropLook != nil {
		p.dropLook()
		p.dropLook = nil
	}
	for _, stop := range p.dropResize {
		stop()
	}
	p.dropResize = nil
	for _, w := range []*ui.Window{p.List, p.Main} {
		if w != nil {
			w.Close()
		}
	}
}
func (p *Player) Pose(pos time.Duration)   { p.Host.Pose(pos); p.Refresh() }
func (p *Player) Advance(dt time.Duration) { p.retryPlacement(dt); p.refreshOwner(); p.Refresh() }
func (p *Player) Refresh() {
	if p.Main == nil || p.Main.Closed() || p.body == nil {
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
func (p *Player) ShowPlaylist(on bool) {
	p.Host.Model.PlaylistVisible = on
	if on {
		p.List.Show()
	} else {
		p.List.Hide()
	}
	if p.body != nil {
		p.body.pl.SetChecked(on)
	}
}
func (p *Player) refreshOwner() {
	// Shape and alpha changes may replace a platform surface. The playlist
	// must remain owned by the current main surface after such a change.
	if p.List == nil || p.List.Closed() {
		return
	}
	if id := p.Main.PortalParent(); id != p.ownerID {
		p.List.SetOwner(p.Main)
		p.ownerID = id
	}
}
func (p *Player) setShape(w *ui.Window, role string) {
	w.SetShapeFunc(func(size paint2d.Point, _ float32) *platform.Shape {
		mask := paint2d.NewImage(int(size.X+.5), int(size.Y+.5))
		ctx := paint2d.NewContext(mask)
		ctx.Clear(paint2d.Transparent)
		slot := "shape.main"
		if role == LayoutPlaylist {
			slot = "shape.playlist"
		}
		// Whole windows may exceed the small-control SkinSlotShape limit.
		if !skins.DrawSkinSlot(w.Look(), ctx, paint2d.XYWH(0, 0, size.X, size.Y), role, slot, 0) {
			return nil
		}
		return platform.NewShapeImage(mask)
	})
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
		at := paint2d.Pt(60*w.Scale(), 40*w.Scale())
		if f := w.Focus(); f != nil {
			from = f
			o := widget.DeviceOrigin(f)
			at = paint2d.Pt(o.X, o.Y+f.Bounds().Dy())
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
