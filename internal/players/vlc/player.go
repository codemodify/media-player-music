// Package vlc presents the classic wxWidgets-era VLC interface. The
// presentation uses the application's shared model and selected backend.
package vlc

import (
	"fmt"
	"time"

	"github.com/codemodify/media-player-music/internal/players"
	vlclayout "github.com/codemodify/media-player-music/internal/skins/vlc"
	"goframework.io/desktop-ui"
	"goframework.io/desktop-ui-skins"
	"goframework.io/desktop-ui/platform"
)

const (
	LayoutMain = vlclayout.LayoutMain
	MainW      = 500
	MainH      = 430
	MinW       = 440
	MinH       = 300
)

type Options struct {
	Headless bool
	Scale    float32
}

type Player struct {
	Host      *players.Host
	App       *ui.Application
	Transport *players.Transport
	Spectrum  *players.Spectrum
	Equaliser *players.Equalizer
	Main      *ui.Window
	body      *body
	dropLook  func()
}

func New(h *players.Host, opts Options) (*Player, error) {
	if _, ok := skins.SkinLayoutOf(h.App.Look(), LayoutMain); !ok {
		return nil, fmt.Errorf("classic VLC requires skin layout %q", LayoutMain)
	}
	p := &Player{Host: h, App: h.App, Transport: h.Model.Transport, Spectrum: h.Model.Spectrum, Equaliser: h.Model.Equaliser}
	var err error
	p.Main, err = players.OpenSized(h.App, platform.WindowOptions{Title: "Music Player", Headless: opts.Headless, Decorations: platform.DecorationsNone, MinWidth: MinW, MinHeight: MinH}, MainW, MainH, false)
	if err != nil {
		return nil, err
	}
	p.body = newBody(p)
	p.Main.SetContent(p.body)
	p.Transport.Changed, p.Equaliser.Changed = p.Refresh, p.Refresh
	p.dropLook = p.App.OnLookChange(p.Refresh)
	p.Refresh()
	p.Main.RequestFocus(p.body.buttons["play"])
	return p, nil
}

func (p *Player) MainWindow() *ui.Window { return p.Main }
func (p *Player) Windows() []players.NamedWindow {
	return []players.NamedWindow{{Name: "main", Window: p.Main}}
}
func (p *Player) Close() {
	if p.dropLook != nil {
		p.dropLook()
		p.dropLook = nil
	}
	if p.Main != nil {
		p.Main.Close()
	}
}
func (p *Player) Pose(pos time.Duration) { p.Host.Pose(pos); p.Refresh() }
func (p *Player) Advance(time.Duration)  { p.Refresh() }
func (p *Player) Refresh() {
	if p.Main == nil || p.Main.Closed() || p.body == nil {
		return
	}
	p.Equaliser.AutoForTrack(p.Transport.List.Index())
	p.body.sync()
}
func (p *Player) ShowEqualiser(on bool) {
	p.Host.Model.EqualizerVisible = on
	p.Refresh()
	p.repairFocus()
	p.body.RequestLayout()
}
func (p *Player) ShowPlaylist(on bool) {
	p.Host.Model.PlaylistVisible = on
	p.Refresh()
	p.repairFocus()
	p.body.RequestLayout()
}
func (p *Player) repairFocus() {
	for c := p.Main.Focus(); c != nil; c = c.Parent() {
		if !c.Visible() {
			p.Main.RequestFocus(p.body.buttons["play"])
			return
		}
	}
}

func (p *Player) CapturePlacement() map[string]players.WindowPlacement {
	if p.Main == nil || p.Main.Closed() {
		return nil
	}
	x, y, known := p.Main.Position()
	w, h := p.Main.Size()
	return map[string]players.WindowPlacement{"main": {X: x, Y: y, Width: w, Height: h, Positioned: known}}
}
func (p *Player) RestorePlacement(saved map[string]players.WindowPlacement) {
	if p.Main == nil || p.Main.Closed() {
		return
	}
	v, ok := saved["main"]
	if !ok {
		return
	}
	w, h := p.Main.Size()
	if v.Width > 0 {
		w = max(v.Width, MinW)
	}
	if v.Height > 0 {
		h = max(v.Height, MinH)
	}
	if v.Width > 0 || v.Height > 0 {
		p.Main.SetSize(w, h)
		p.Main.RequestLayout()
	}
	if v.Positioned && p.Main.CanMove() {
		p.Main.Move(v.X, v.Y)
	}
}
