// Package standard presents the player in an ordinary themed window.
// The controls use the shared host's playback and playlist actions.
package standard

import (
	"time"

	"github.com/codemodify/media-player-music/internal/players"
	"goframework.io/desktop-ui"
	"goframework.io/desktop-ui/platform"
	"goframework.io/desktop-ui/widget"
	"goframework.io/graphics-paint2d"
)

const (
	MainW = 920
	MainH = 560
	MinW  = 760
	MinH  = 460
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
	Main           *ui.Window
	body           *body
	dropLook       func()
	dropResize     func()
	placementReady bool
	restore        *placementRestore
}

func New(h *players.Host, opts Options) (*Player, error) {
	p := &Player{Host: h, App: h.App, Transport: h.Model.Transport, Spectrum: h.Model.Spectrum, Equaliser: h.Model.Equaliser}
	w, err := players.OpenSized(h.App, platform.WindowOptions{
		Title: "Music Player", Headless: opts.Headless,
		Decorations: platform.DecorationsClient, MinWidth: MinW, MinHeight: MinH,
	}, MainW, MainH, false)
	if err != nil {
		return nil, err
	}
	p.Main = w
	p.dropResize = w.OnResize(func(_, _ int) {
		// OnResize follows the toolkit's frame setup, including its shadow
		// margin. A move before that setup uses the wrong visible origin.
		p.placementReady = true
		p.retryPlacement(0)
	})
	w.SetOnCaptionDrag(func(paint2d.Point) bool {
		// A user drag takes precedence over unfinished startup restoration.
		p.restore = nil
		return false
	})
	p.body = newBody(p)
	p.body.eq.SetVisible(h.Model.EqualizerVisible)
	p.body.queue.SetVisible(h.Model.PlaylistVisible)
	w.SetContent(p.body)
	p.Transport.Changed = p.Refresh
	p.Equaliser.Changed = p.Refresh
	p.dropLook = p.App.OnLookChange(p.Refresh)
	p.Refresh()
	w.RequestFocus(p.body.main.transport[1])
	return p, nil
}

func (p *Player) MainWindow() *ui.Window { return p.Main }
func (p *Player) Windows() []players.NamedWindow {
	return []players.NamedWindow{{Name: "main", Window: p.Main}}
}
func (p *Player) Close() {
	p.restore = nil
	if p.dropResize != nil {
		p.dropResize()
		p.dropResize = nil
	}
	if p.dropLook != nil {
		p.dropLook()
		p.dropLook = nil
	}
	p.Main.Close()
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

func (p *Player) keys(e widget.KeyEvent) bool {
	if e.Mods.Ctrl() {
		switch {
		case e.Key == platform.KeyE || e.Rune == 'e' || e.Rune == 'E':
			p.ShowEqualiser(!p.EqualiserShown())
			return true
		case e.Key == platform.KeyL || e.Rune == 'l' || e.Rune == 'L':
			p.ShowPlaylist(!p.PlaylistShown())
			return true
		case e.Key == platform.KeyK || e.Rune == 'k' || e.Rune == 'K':
			if e.Mods.Shift() {
				p.Host.DropSkin()
			} else {
				p.Host.NextSkin()
			}
			return true
		}
	}
	if e.Key == platform.KeyMenu || (e.Key == platform.KeyF10 && e.Mods.Shift()) {
		from := widget.Component(p.body)
		at := paint2d.Pt(8, 8)
		if f := p.Main.Focus(); f != nil {
			from = f
			o := widget.DeviceOrigin(f)
			at = paint2d.Pt(o.X, o.Y+f.Bounds().Dy())
		}
		p.Host.ContextMenu(from, at)
		return true
	}
	return p.Host.Keys(p.Main, e)
}

func (p *Player) EqualiserShown() bool { return p.Host.Model.EqualizerVisible }
func (p *Player) PlaylistShown() bool  { return p.Host.Model.PlaylistVisible }
func (p *Player) ShowEqualiser(on bool) {
	p.Host.Model.EqualizerVisible = on
	p.show(p.body.eq, on)
}
func (p *Player) ShowPlaylist(on bool) {
	p.Host.Model.PlaylistVisible = on
	p.show(p.body.queue, on)
}
func (p *Player) show(c widget.Component, on bool) {
	if !on && within(p.Main.Focus(), c) {
		p.Main.RequestFocus(p.body.main.transport[1])
	}
	c.SetVisible(on)
	p.body.RequestLayout()
	p.Refresh()
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

func (p *Player) transportButtons() [6]*players.GlyphButton {
	var out [6]*players.GlyphButton
	for i, spec := range []struct {
		glyph players.Glyph
		name  string
		run   func()
	}{
		{players.GlyphPrev, "Previous track", func() { p.Host.Command(players.CmdPrev) }},
		{players.GlyphPlay, "Play", p.Host.Play},
		{players.GlyphPause, "Pause", func() {
			if p.Transport.State != players.Stopped {
				p.Host.Command(players.CmdPlayPause)
			}
		}},
		{players.GlyphStop, "Stop", func() { p.Host.Command(players.CmdStop) }},
		{players.GlyphNext, "Next track", func() { p.Host.Command(players.CmdNext) }},
		{players.GlyphEject, "Eject: open music files", func() { p.Host.OpenFiles(out[5], false) }},
	} {
		out[i] = players.NewGlyphButton(spec.glyph, spec.name, spec.run)
		out[i].Push = true
	}
	return out
}
