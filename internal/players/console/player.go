// Package console presents the Console skin around the
// application's shared model, executable playback engines and public controls.
package console

import (
	"fmt"
	"time"

	"github.com/codemodify/media-player-music/internal/players"
	rplayout "github.com/codemodify/media-player-music/internal/skins/realplayer"
	"goframework.io/desktop-ui"
	"goframework.io/desktop-ui-skins"
	"goframework.io/desktop-ui/platform"
	"goframework.io/desktop-ui/widget"
)

const (
	LayoutMain      = rplayout.LayoutMain
	LayoutPlaylist  = rplayout.LayoutPlaylist
	LayoutEqualizer = rplayout.LayoutEqualizer
	MainW           = 720
	MainH           = 520
	MinW            = 606
	MinH            = 500
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
	for _, role := range []string{LayoutMain, LayoutPlaylist, LayoutEqualizer} {
		if _, ok := skins.SkinLayoutOf(h.App.Look(), role); !ok {
			return nil, fmt.Errorf("Console requires skin layout %q", role)
		}
	}
	p := &Player{Host: h, App: h.App, Transport: h.Model.Transport, Spectrum: h.Model.Spectrum, Equaliser: h.Model.Equaliser}
	w, err := players.OpenSized(h.App, platform.WindowOptions{Title: "Music Player", Headless: opts.Headless, Decorations: platform.DecorationsNone, MinWidth: MinW, MinHeight: MinH}, MainW, MainH, false)
	if err != nil {
		return nil, err
	}
	p.Main = w
	p.body = newBody(p)
	w.SetContent(p.body)
	p.dropResize = w.OnResize(func(_, _ int) { p.placementReady = true; p.retryPlacement(0) })
	p.Transport.Changed, p.Equaliser.Changed = p.Refresh, p.Refresh
	p.dropLook = p.App.OnLookChange(p.Refresh)
	p.Refresh()
	w.RequestFocus(p.body.buttons["play"])
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
func (p *Player) Pose(pos time.Duration)   { p.Host.Pose(pos); p.Refresh() }
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
	p.relayout()
}
func (p *Player) ShowPlaylist(on bool) {
	p.Host.Model.PlaylistVisible = on
	p.body.queue.SetVisible(on)
	p.relayout()
}
func (p *Player) relayout() {
	if focus := p.Main.Focus(); focus != nil {
		for c := focus; c != nil; c = c.Parent() {
			if !c.Visible() {
				p.Main.RequestFocus(p.body.buttons["play"])
				break
			}
		}
	}
	p.body.RequestLayout()
	p.Refresh()
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
		p.body.appearances(p.body)
		return true
	}
	return p.Host.Keys(p.Main, e)
}
