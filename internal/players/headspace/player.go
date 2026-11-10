// Package headspace presents the original Windows Media Player Headspace
// artwork as a single shaped window. The app owns the layout and artwork;
// ordinary toolkit controls route their actions through the shared host.
package headspace

import (
	"fmt"
	"time"

	"github.com/codemodify/media-player-music/internal/players"
	hslayout "github.com/codemodify/media-player-music/internal/skins/headspace"
	"goframework.io/desktop-ui"
	"goframework.io/desktop-ui-skins"
	"goframework.io/desktop-ui/platform"
	"goframework.io/desktop-ui/widget"
	"goframework.io/graphics-paint2d"
)

const (
	LayoutMain = hslayout.LayoutMain
	MainW      = 760
	MainH      = 394
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
		return nil, fmt.Errorf("Headspace requires a skin with layout %q", LayoutMain)
	}
	p := &Player{Host: h, App: h.App, Transport: h.Model.Transport, Spectrum: h.Model.Spectrum, Equaliser: h.Model.Equaliser}
	w, err := players.OpenSized(h.App, platform.WindowOptions{
		Title: "Music Player", Headless: opts.Headless,
		Decorations: platform.DecorationsNone,
	}, MainW, MainH, true)
	if err != nil {
		return nil, err
	}
	p.Main = w
	p.dropResize = w.OnResize(func(_, _ int) {
		p.placementReady = true
		p.retryPlacement(0)
	})
	p.body = newBody(p)
	w.SetContent(p.body)
	p.updateShape()
	p.Transport.Changed = p.Refresh
	p.Equaliser.Changed = p.Refresh
	p.dropLook = p.App.OnLookChange(func() { p.updateShape(); p.Refresh() })
	p.Refresh()
	w.RequestFocus(p.body.play)
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

func (p *Player) ShowEqualiser(on bool) {
	if p.Host.Model.EqualizerVisible == on {
		return
	}
	p.Host.Model.EqualizerVisible = on
	p.wingsChanged()
}
func (p *Player) ShowPlaylist(on bool) {
	if p.Host.Model.PlaylistVisible == on {
		return
	}
	p.Host.Model.PlaylistVisible = on
	p.wingsChanged()
}
func (p *Player) wingsChanged() {
	if c := p.Main.Focus(); c != nil && !p.body.controlVisible(c) {
		p.Main.RequestFocus(p.body.play)
	}
	p.updateShape()
	p.body.RequestLayout()
	p.Refresh()
}
func (p *Player) wingState() string {
	if p.Host.Model.EqualizerVisible {
		if p.Host.Model.PlaylistVisible {
			return "both"
		}
		return "eq"
	}
	if p.Host.Model.PlaylistVisible {
		return "pl"
	}
	return "closed"
}
func (p *Player) updateShape() {
	// The pack supplies the union of the head, display and visible wings.
	// Matching the alpha used for painting makes empty space click through.
	slot := "shape." + p.wingState()
	p.Main.SetShapeFunc(func(size paint2d.Point, _ float32) *platform.Shape {
		// SkinSlotShape is for controls and deliberately declines masks over
		// 512² device pixels. A whole window paints its alpha through the public
		// sprite API instead, then uses the platform's unrestricted image mask.
		mask := paint2d.NewImage(int(size.X+.5), int(size.Y+.5))
		ctx := paint2d.NewContext(mask)
		ctx.Clear(paint2d.Transparent)
		if !skins.DrawSkinSlot(p.Main.Look(), ctx, paint2d.XYWH(0, 0, size.X, size.Y), LayoutMain, slot, 0) {
			return nil
		}
		return platform.NewShapeImage(mask)
	})
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
	if e.Key == platform.KeyMenu || (e.Key == platform.KeyF10 && e.Mods.Shift()) {
		from := widget.Component(p.body)
		at := paint2d.Pt(320*p.Main.Scale(), 48*p.Main.Scale())
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
