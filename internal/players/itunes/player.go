// Package itunes presents the brushed-metal iTunes appearance using the
// application's shared queue, playback engine and public toolkit controls.
package itunes

import (
	"fmt"
	"time"

	"github.com/codemodify/media-player-music/internal/players"
	"github.com/codemodify/paintengine2d"
	"github.com/codemodify/uitoolkit/app"
	"github.com/codemodify/uitoolkit/platform"
	"github.com/codemodify/uitoolkit/style"
	"github.com/codemodify/uitoolkit/widget"
)

const (
	LayoutMain      = "itunes.main"
	LayoutMini      = "itunes.mini"
	LayoutEqualizer = "itunes.equalizer"
	MainW           = 800
	MainH           = 560
	MinW            = 640
	MinH            = 400
	MiniW           = 380
	MiniH           = 62
	MinMiniW        = 150
	MaxMiniW        = 1000
	EqW             = 430
	EqH             = 230
)

type Options struct {
	Headless bool
	Scale    float32
}

type Player struct {
	Host           *players.Host
	App            *app.Application
	Transport      *players.Transport
	Spectrum       *players.Spectrum
	Equaliser      *players.Equalizer
	Main, Eq       *app.Window
	body           *body
	mini           *miniPane
	eqPane         *eqPane
	mode           string
	sizes          map[string]players.PresentationSize
	dropLook       func()
	dropResize     []func()
	placementReady map[string]bool
	restore        map[string]*placementRestore
	ownerID        string
	modeResize     *modeResize
}

func New(h *players.Host, opts Options) (*Player, error) {
	for _, role := range []string{LayoutMain, LayoutMini, LayoutEqualizer} {
		if _, ok := style.SkinLayoutOf(h.App.Look(), role); !ok {
			return nil, fmt.Errorf("iTunes requires skin layout %q", role)
		}
	}
	p := &Player{Host: h, App: h.App, Transport: h.Model.Transport, Spectrum: h.Model.Spectrum, Equaliser: h.Model.Equaliser,
		mode: modeFull, sizes: map[string]players.PresentationSize{modeFull: {Width: MainW, Height: MainH}, modeMini: {Width: MiniW, Height: MiniH}},
		placementReady: make(map[string]bool), restore: make(map[string]*placementRestore)}
	var err error
	p.Main, err = players.OpenSized(h.App, platform.WindowOptions{Title: "Music Player", Headless: opts.Headless, Decorations: platform.DecorationsNone, MinWidth: MinW, MinHeight: MinH}, MainW, MainH, false)
	if err != nil {
		return nil, err
	}
	p.body, p.mini = newBody(p), newMiniPane(p)
	p.body.SetHost(p.Main)
	p.mini.SetHost(p.Main)
	p.Main.SetContent(p.body)
	p.setShape(p.Main, LayoutMain, "shape.main")
	p.Eq, err = players.OpenSized(h.App, platform.WindowOptions{Title: "Equalizer", Headless: opts.Headless, Decorations: platform.DecorationsNone, Role: platform.RoleUtility, Owner: p.Main.Surface()}, EqW, EqH, true)
	if err != nil {
		p.Main.Close()
		return nil, err
	}
	p.eqPane = newEqPane(p)
	p.Eq.SetContent(p.eqPane)
	p.setShape(p.Eq, LayoutEqualizer, "shape.eq")
	p.Eq.SetOnCloseRequest(func() bool { p.ShowEqualiser(false); return false })
	for _, named := range p.Windows() {
		named := named
		p.dropResize = append(p.dropResize, named.Window.OnResize(func(width, height int) {
			p.placementReady[named.Name] = true
			if named.Name == "main" {
				p.acknowledgeModeSize(width, height)
			}
			p.retryPlacement(0)
		}))
	}
	p.Transport.Changed, p.Equaliser.Changed = p.Refresh, p.Refresh
	p.dropLook = p.App.OnLookChange(func() {
		p.updateShape()
		p.setShape(p.Eq, LayoutEqualizer, "shape.eq")
		p.refreshOwner()
		p.Refresh()
	})
	p.ShowEqualiser(h.Model.EqualizerVisible)
	p.refreshOwner()
	p.Refresh()
	p.Main.RequestFocus(p.body.play)
	return p, nil
}

func (p *Player) MainWindow() *app.Window { return p.Main }
func (p *Player) Windows() []players.NamedWindow {
	return []players.NamedWindow{{Name: "main", Window: p.Main}, {Name: "equaliser", Window: p.Eq}}
}
func (p *Player) Close() {
	p.restore, p.modeResize = nil, nil
	if p.dropLook != nil {
		p.dropLook()
		p.dropLook = nil
	}
	for _, drop := range p.dropResize {
		drop()
	}
	p.dropResize = nil
	for _, w := range []*app.Window{p.Eq, p.Main} {
		if w != nil {
			w.Close()
		}
	}
}
func (p *Player) Pose(pos time.Duration) { p.Host.Pose(pos); p.Refresh() }
func (p *Player) Advance(dt time.Duration) {
	p.retryModeResize(dt)
	p.retryPlacement(dt)
	p.refreshOwner()
	p.Refresh()
}
func (p *Player) Refresh() {
	if p.Main == nil || p.Main.Closed() || p.body == nil || p.mini == nil || p.eqPane == nil {
		return
	}
	p.Equaliser.AutoForTrack(p.Transport.List.Index())
	p.body.sync()
	p.mini.sync()
	p.eqPane.sync()
}
func (p *Player) ShowEqualiser(on bool) {
	p.Host.Model.EqualizerVisible = on
	if p.Eq != nil && !p.Eq.Closed() {
		if on {
			p.Eq.Show()
		} else {
			p.Eq.Hide()
		}
	}
	p.refreshOwner()
	p.Refresh()
}
func (p *Player) ShowPlaylist(on bool) {
	p.Host.Model.PlaylistVisible = on
	if on && p.MiniPlayer() {
		p.SetMiniPlayer(false)
	}
	p.Refresh()
	if p.body != nil {
		p.body.RequestLayout()
	}
}
func (p *Player) userMoved(name string) {
	delete(p.restore, name)
	if name == "main" {
		p.modeResize = nil
	}
}
func (p *Player) refreshOwner() {
	if p.Main == nil || p.Main.Closed() || p.Eq == nil || p.Eq.Closed() {
		return
	}
	if id := p.Main.PortalParent(); id != "" && id != p.ownerID {
		// v0.23.4 can leave a utility's owner pointing at a replaced parent.
		// Refresh only through the public ownership API when identity changes.
		p.Eq.SetOwner(p.Main)
		p.ownerID = id
	}
}
func (p *Player) updateShape() {
	role, slot := LayoutMain, "shape.main"
	if p.MiniPlayer() {
		role, slot = LayoutMini, "shape.mini"
	}
	p.setShape(p.Main, role, slot)
	p.refreshOwner()
}
func (p *Player) setShape(w *app.Window, role, slot string) {
	w.SetShapeFunc(func(size paintengine2d.Point, _ float32) *platform.Shape {
		image := paintengine2d.NewImage(int(size.X+.5), int(size.Y+.5))
		ctx := paintengine2d.NewContext(image)
		ctx.Clear(paintengine2d.Transparent)
		box := paintengine2d.XYWH(0, 0, size.X, size.Y)
		r, ok := style.SkinSlotRect(w.Look(), role, slot, box)
		if !ok || !style.DrawSkinSlot(w.Look(), ctx, r, role, slot, 0) {
			return nil
		}
		// Artwork paints its own alpha once; the window mask supplies extent.
		for at := 3; at < len(image.Pix); at += 4 {
			if image.Pix[at] != 0 {
				image.Pix[at-3], image.Pix[at-2], image.Pix[at-1], image.Pix[at] = 255, 255, 255, 255
			}
		}
		return platform.NewShapeImage(image)
	})
}
func (p *Player) keys(w *app.Window, e widget.KeyEvent) bool {
	if e.Mods.Ctrl() {
		switch e.Key {
		case platform.KeyM:
			if e.Mods.Shift() {
				p.SetMiniPlayer(!p.MiniPlayer())
				return true
			}
		case platform.KeyF:
			p.ShowPlaylist(true)
			p.Main.RequestFocus(p.body.search)
			return true
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
		p.Host.ContextMenu(from, widget.DeviceOrigin(from))
		return true
	}
	return p.Host.Keys(w, e)
}
