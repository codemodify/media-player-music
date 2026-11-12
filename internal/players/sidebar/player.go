// Package sidebar presents the Sidebar skin's composition using the shared
// player model, the skin's icons and public toolkit controls.
package sidebar

import (
	"fmt"
	"time"

	"github.com/codemodify/media-player-music/internal/players"
	sidebarlayout "github.com/codemodify/media-player-music/internal/skins/sidebar"
	"goframework.io/desktop-ui"
	"goframework.io/desktop-ui-skins"
	"goframework.io/desktop-ui/platform"
	"goframework.io/desktop-ui/widget"
)

const (
	LayoutMain = sidebarlayout.LayoutMain
	MainW      = 920
	MainH      = 620
	MinW       = 740
	MinH       = 480
)

type Options struct {
	Headless bool
	Scale    float32
}
type Player struct {
	Host                 *players.Host
	App                  *ui.Application
	Transport            *players.Transport
	Spectrum             *players.Spectrum
	Equaliser            *players.Equalizer
	Main                 *ui.Window
	body                 *body
	dropLook, dropResize func()
	placementReady       bool
	restore              *placementRestore
}

func New(h *players.Host, opts Options) (*Player, error) {
	if _, ok := skins.SkinLayoutOf(h.App.Look(), LayoutMain); !ok {
		return nil, fmt.Errorf("Amarok requires skin layout %q", LayoutMain)
	}
	p := &Player{Host: h, App: h.App, Transport: h.Model.Transport, Spectrum: h.Model.Spectrum, Equaliser: h.Model.Equaliser}
	var err error
	p.Main, err = players.OpenSized(h.App, platform.WindowOptions{Title: "Music Player", Headless: opts.Headless, Decorations: platform.DecorationsNone, MinWidth: MinW, MinHeight: MinH}, MainW, MainH, false)
	if err != nil {
		return nil, err
	}
	p.body = newBody(p)
	p.Main.SetContent(p.body)
	p.dropResize = p.Main.OnResize(func(_, _ int) { p.placementReady = true; p.retryPlacement(0) })
	p.Transport.Changed, p.Equaliser.Changed = p.Refresh, p.Refresh
	p.dropLook = p.App.OnLookChange(p.Refresh)
	p.Refresh()
	p.Main.RequestFocus(p.body.play)
	return p, nil
}
func (p *Player) MainWindow() *ui.Window { return p.Main }
func (p *Player) Windows() []players.NamedWindow {
	return []players.NamedWindow{{Name: "main", Window: p.Main}}
}
func (p *Player) Close() {
	p.restore = nil
	if p.dropLook != nil {
		p.dropLook()
		p.dropLook = nil
	}
	if p.dropResize != nil {
		p.dropResize()
		p.dropResize = nil
	}
	if p.Main != nil {
		p.Main.Close()
	}
}
func (p *Player) Pose(at time.Duration)    { p.Host.Pose(at); p.Refresh() }
func (p *Player) Advance(dt time.Duration) { p.retryPlacement(dt); p.Refresh() }
func (p *Player) Refresh() {
	if p.Main == nil || p.Main.Closed() || p.body == nil {
		return
	}
	p.Equaliser.AutoForTrack(p.Transport.List.Index())
	p.body.sync()
}
func (p *Player) ShowEqualiser(on bool) {
	p.Host.Model.EqualizerVisible = on
	p.body.eq.SetVisible(on)
	if !on && within(p.Main.Focus(), p.body.eq) {
		p.Main.RequestFocus(p.body.play)
	}
	p.Refresh()
	p.body.RequestLayout()
}
func (p *Player) ShowPlaylist(on bool) {
	p.Host.Model.PlaylistVisible = on
	p.body.list.SetVisible(on)
	for _, c := range p.body.headers {
		c.SetVisible(on)
	}
	if focus := p.Main.Focus(); !on && (within(focus, p.body.list) || focus != nil && !focus.Visible()) {
		p.Main.RequestFocus(p.body.play)
	}
	p.Refresh()
	p.body.RequestLayout()
}
func within(c, ancestor widget.Component) bool {
	for c != nil {
		if c == ancestor {
			return true
		}
		c = c.Parent()
	}
	return false
}
func (p *Player) keys(e widget.KeyEvent) bool {
	if e.Mods.Ctrl() {
		switch e.Key {
		case platform.KeyE:
			p.ShowEqualiser(!p.Host.Model.EqualizerVisible)
			return true
		case platform.KeyL:
			p.ShowPlaylist(!p.Host.Model.PlaylistVisible)
			return true
		case platform.KeyF:
			p.ShowPlaylist(true)
			p.Main.RequestFocus(p.body.search)
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
		p.body.contextMenu(p.Main.Focus())
		return true
	}
	return p.Host.Keys(p.Main, e)
}
