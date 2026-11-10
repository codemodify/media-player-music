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
}
type Player struct {
	Host              *players.Host
	App               *ui.Application
	Transport         *players.Transport
	Spectrum          *players.Spectrum
	Equaliser         *players.Equalizer
	Main, Eq, List    *ui.Window
	body              *body
	eqPane            *equalizerPane
	listPane          *playlistPane
	dropLook          func()
	Desk              *rack.Desk
	iMain, iEq, iList int
	stacked           bool
	tries             int
	restoredPlacement map[string]players.WindowPlacement
	utilityOwnerID    string
}

func New(h *players.Host, opts Options) (*Player, error) {
	for _, role := range []string{LayoutMain, LayoutEqualizer, LayoutPlaylist} {
		if _, ok := skins.SkinLayoutOf(h.App.Look(), role); !ok {
			return nil, fmt.Errorf("Half Life 2 requires skin layout %q", role)
		}
	}
	p := &Player{Host: h, App: h.App, Transport: h.Model.Transport, Spectrum: h.Model.Spectrum, Equaliser: h.Model.Equaliser}
	var err error
	p.Main, err = p.open(opts, "Music Player", LayoutMain, MainW, MainH, true)
	if err != nil {
		return nil, err
	}
	p.body = newBody(p)
	p.Main.SetContent(p.body)
	p.setShape(p.Main, LayoutMain)
	p.Eq, err = p.open(opts, "Equalizer", LayoutEqualizer, EqW, EqH, true)
	if err != nil {
		p.Main.Close()
		return nil, err
	}
	p.eqPane = newEqualizerPane(p)
	p.Eq.SetContent(p.eqPane)
	p.setShape(p.Eq, LayoutEqualizer)
	p.List, err = p.open(opts, "Playlist", LayoutPlaylist, ListW, ListH, false)
	if err != nil {
		p.Eq.Close()
		p.Main.Close()
		return nil, err
	}
	p.listPane = newPlaylistPane(p)
	p.List.SetContent(p.listPane)
	p.setShape(p.List, LayoutPlaylist)
	p.initGroup()
	p.Eq.SetOnCloseRequest(func() bool { p.ShowEqualiser(false); return false })
	p.List.SetOnCloseRequest(func() bool { p.ShowPlaylist(false); return false })
	p.Transport.Changed, p.Equaliser.Changed = p.Refresh, p.Refresh
	p.dropLook = p.App.OnLookChange(func() {
		for _, named := range p.Windows() {
			role := LayoutMain
			if named.Window == p.Eq {
				role = LayoutEqualizer
			}
			if named.Window == p.List {
				role = LayoutPlaylist
			}
			p.setShape(named.Window, role)
		}
		p.configureUtilityWindows()
		p.Refresh()
	})
	p.ShowEqualiser(h.Model.EqualizerVisible)
	p.ShowPlaylist(h.Model.PlaylistVisible)
	p.configureUtilityWindows()
	p.Refresh()
	p.Main.RequestFocus(p.body.transportButton())
	p.Eq.RequestFocus(p.eqPane.bands[0])
	p.List.RequestFocus(p.listPane.list)
	return p, nil
}
func (p *Player) open(opts Options, title, role string, width, height int, fixed bool) (*ui.Window, error) {
	options := platform.WindowOptions{Title: title, Headless: opts.Headless, Decorations: platform.DecorationsNone}
	if role != LayoutMain {
		options.Role = platform.RoleUtility
		options.Owner = p.Main.Surface()
	}
	if role == LayoutPlaylist {
		options.MinWidth, options.MinHeight = MinListW, MinListH
	}
	return players.OpenSized(p.App, options, width, height, fixed)
}
func (p *Player) MainWindow() *ui.Window { return p.Main }
func (p *Player) Windows() []players.NamedWindow {
	return []players.NamedWindow{{Name: "main", Window: p.Main}, {Name: "equaliser", Window: p.Eq}, {Name: "playlist", Window: p.List}}
}
func (p *Player) Close() {
	if p.dropLook != nil {
		p.dropLook()
		p.dropLook = nil
	}
	p.restoredPlacement = nil
	for _, w := range []*ui.Window{p.List, p.Eq, p.Main} {
		if w != nil {
			w.Close()
		}
	}
}
func (p *Player) Pose(pos time.Duration) { p.Host.Pose(pos); p.Refresh() }
func (p *Player) Advance(dt time.Duration) {
	p.AdvanceGeometry(dt)
	p.configureUtilityWindows()
	p.Refresh()
}
func (p *Player) Refresh() {
	if p.Main == nil || p.Main.Closed() || p.body == nil || p.eqPane == nil || p.listPane == nil {
		return
	}
	p.Equaliser.AutoForTrack(p.Transport.List.Index())
	p.body.sync()
	p.eqPane.sync()
	p.listPane.sync()
}
func (p *Player) ShowEqualiser(on bool) { p.showWindow(p.iEq, on) }
func (p *Player) ShowPlaylist(on bool)  { p.showWindow(p.iList, on) }
func (p *Player) configureUtilityWindows() {
	if p.Main == nil || p.Main.Closed() {
		return
	}
	id := p.Main.PortalParent()
	if id == "" || id == p.utilityOwnerID {
		return
	}
	for _, w := range []*ui.Window{p.Eq, p.List} {
		if w != nil && !w.Closed() {
			w.SetOwner(p.Main)
		}
	}
	p.utilityOwnerID = id
}
func (p *Player) setShape(w *ui.Window, role string) {
	w.SetShapeFunc(func(size paint2d.Point, _ float32) *platform.Shape {
		im := paint2d.NewImage(int(size.X+.5), int(size.Y+.5))
		ctx := paint2d.NewContext(im)
		ctx.Clear(paint2d.Transparent)
		slot := "shape"
		if role == LayoutMain {
			slot += "." + []string{"open", "inner", "closed"}[p.body.shutter]
		}
		if !skins.DrawSkinSlot(w.Look(), ctx, paint2d.XYWH(0, 0, size.X, size.Y), role, slot, 0) {
			return nil
		}
		return platform.NewShapeImage(im)
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
