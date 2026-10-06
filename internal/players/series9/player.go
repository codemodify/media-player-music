// Package series9 presents Microsoft's original Series 9 skin using the
// application's shared playback model and public toolkit controls.
package series9

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
	LayoutMain      = "series9.main"
	LayoutTransport = "series9.transport"
	LayoutEqualizer = "series9.equalizer"
	LayoutPlaylist  = "series9.playlist"
	LayoutCompact   = "series9.compact"
	MainW           = 346
	MainH           = 344
	MinW            = MainW
	MinH            = MainH
	CompactW        = 346
	CompactH        = 103
	PlaylistTravel  = 250
	EqualizerTravel = 124
)

type Options struct {
	Headless bool
	Scale    float32
}

type Player struct {
	Host                        *players.Host
	App                         *app.Application
	Transport                   *players.Transport
	Spectrum                    *players.Spectrum
	Equaliser                   *players.Equalizer
	Main                        *app.Window
	body                        *body
	compact                     *compactPane
	mode                        string
	fullSize                    players.PresentationSize
	drawPlaylist, drawEqualizer bool
	modeResize                  *modeResize
	dropLook, dropResize        func()
	placementReady              bool
	restore                     *placementRestore
}

func New(h *players.Host, opts Options) (*Player, error) {
	for _, role := range []string{LayoutMain, LayoutTransport, LayoutEqualizer, LayoutPlaylist, LayoutCompact} {
		if _, ok := style.SkinLayoutOf(h.App.Look(), role); !ok {
			return nil, fmt.Errorf("Series 9 requires skin layout %q", role)
		}
	}
	p := &Player{Host: h, App: h.App, Transport: h.Model.Transport, Spectrum: h.Model.Spectrum, Equaliser: h.Model.Equaliser,
		mode: modeFull, fullSize: players.PresentationSize{Width: MainW, Height: MainH},
		drawPlaylist: h.Model.PlaylistVisible, drawEqualizer: h.Model.EqualizerVisible}
	size := p.actualFullSize(p.fullSize)
	width, height := size.Width, size.Height
	var err error
	p.Main, err = players.OpenSized(h.App, platform.WindowOptions{Title: "Music Player", Headless: opts.Headless,
		Decorations: platform.DecorationsNone, MinWidth: width, MinHeight: height}, width, height, false)
	if err != nil {
		return nil, err
	}
	p.body, p.compact = newBody(p), newCompactPane(p)
	p.body.SetHost(p.Main)
	p.compact.SetHost(p.Main)
	p.Main.SetContent(p.body)
	p.updateShape()
	p.dropResize = p.Main.OnResize(func(width, height int) {
		p.placementReady = true
		p.acknowledgeModeSize(width, height)
		p.retryPlacement(0)
	})
	p.Transport.Changed, p.Equaliser.Changed = p.Refresh, p.Refresh
	p.dropLook = p.App.OnLookChange(func() { p.updateShape(); p.Refresh() })
	p.Refresh()
	p.Main.RequestFocus(p.body.play)
	return p, nil
}

func (p *Player) MainWindow() *app.Window { return p.Main }
func (p *Player) Windows() []players.NamedWindow {
	return []players.NamedWindow{{Name: "main", Window: p.Main}}
}
func (p *Player) Close() {
	p.restore, p.modeResize = nil, nil
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
func (p *Player) Advance(dt time.Duration) { p.retryModeResize(dt); p.retryPlacement(dt); p.Refresh() }
func (p *Player) Refresh() {
	if p.Main == nil || p.Main.Closed() || p.body == nil || p.compact == nil {
		return
	}
	if !p.Compact() && (p.drawPlaylist != p.Host.Model.PlaylistVisible || p.drawEqualizer != p.Host.Model.EqualizerVisible) {
		p.captureModeSize()
		p.applyMode(modeFull)
		return
	}
	p.Equaliser.AutoForTrack(p.Transport.List.Index())
	p.body.sync()
	p.compact.sync()
}
func (p *Player) ShowEqualiser(on bool) {
	if !p.Compact() && p.drawEqualizer == on && p.Host.Model.EqualizerVisible == on {
		p.Refresh()
		return
	}
	if !p.Compact() {
		p.captureModeSize()
	}
	p.Host.Model.EqualizerVisible = on
	if on && p.Compact() {
		p.applyMode(modeFull)
	} else if !p.Compact() {
		p.applyMode(modeFull)
	} else {
		p.Refresh()
	}
}
func (p *Player) ShowPlaylist(on bool) {
	if !p.Compact() && p.drawPlaylist == on && p.Host.Model.PlaylistVisible == on {
		p.Refresh()
		return
	}
	if !p.Compact() {
		p.captureModeSize()
	}
	p.Host.Model.PlaylistVisible = on
	if on && p.Compact() {
		p.applyMode(modeFull)
	} else if !p.Compact() {
		p.applyMode(modeFull)
	} else {
		p.Refresh()
	}
}
func (p *Player) userMoved(name string) {
	if name == "main" {
		p.restore, p.modeResize = nil, nil
	}
}

type panelGeometry struct{ Main, Transport, Equalizer, Playlist paintengine2d.Rect }

// panelRects keeps the original drawer offsets in device coordinates. Base
// dimensions exclude the additional space occupied by either open drawer.
func (p *Player) panelRects(box paintengine2d.Rect) panelGeometry {
	scale := p.Main.Scale()
	mainX := float32(0)
	if p.drawPlaylist {
		mainX = PlaylistTravel * scale
	}
	baseW, baseH := box.Dx()-mainX, box.Dy()
	if p.drawEqualizer {
		baseH -= EqualizerTravel * scale
	}
	return panelGeometry{
		Main:      paintengine2d.XYWH(box.Min.X+mainX, box.Min.Y, baseW, baseH-55*scale),
		Transport: paintengine2d.XYWH(box.Min.X+mainX+12*scale, box.Max.Y-60*scale, baseW-12*scale, 60*scale),
		Equalizer: paintengine2d.XYWH(box.Min.X+mainX+12*scale, box.Max.Y-183*scale, baseW-12*scale, 123*scale),
		Playlist:  paintengine2d.XYWH(box.Min.X, box.Min.Y+33*scale, 263*scale, baseH-107*scale),
	}
}
func (p *Player) updateShape() {
	if p.Main == nil || p.Main.Closed() {
		return
	}
	p.Main.SetShapeFunc(func(size paintengine2d.Point, _ float32) *platform.Shape {
		mask := paintengine2d.NewImage(int(size.X+.5), int(size.Y+.5))
		ctx := paintengine2d.NewContext(mask)
		ctx.Clear(paintengine2d.Transparent)
		p.drawShape(ctx, size)
		// The art carries its own alpha; this mask supplies its extent only.
		for at := 3; at < len(mask.Pix); at += 4 {
			if mask.Pix[at] != 0 {
				mask.Pix[at-3], mask.Pix[at-2], mask.Pix[at-1], mask.Pix[at] = 255, 255, 255, 255
			}
		}
		return platform.NewShapeImage(mask)
	})
}
func (p *Player) drawShape(ctx *paintengine2d.Context, size paintengine2d.Point) {
	box := paintengine2d.XYWH(0, 0, size.X, size.Y)
	if p.Compact() {
		style.DrawSkinSlot(p.Main.Look(), ctx, box, LayoutCompact, "shape.background", 0)
		return
	}
	r := p.panelRects(box)
	if p.drawPlaylist {
		style.DrawSkinSlot(p.Main.Look(), ctx, r.Playlist, LayoutPlaylist, "shape.background", 0)
	}
	if p.drawEqualizer {
		style.DrawSkinSlot(p.Main.Look(), ctx, r.Equalizer, LayoutEqualizer, "shape.background", 0)
	}
	style.DrawSkinSlot(p.Main.Look(), ctx, r.Main, LayoutMain, "shape.background", 0)
	style.DrawSkinSlot(p.Main.Look(), ctx, r.Transport, LayoutTransport, "shape.background", 0)
}
func (p *Player) keys(w *app.Window, e widget.KeyEvent) bool {
	if e.Mods.Ctrl() {
		switch e.Key {
		case platform.KeyM:
			if e.Mods.Shift() {
				p.SetCompact(!p.Compact())
				return true
			}
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
		at := widget.DeviceOrigin(from)
		if c := w.Focus(); c != nil {
			from = c
			at = widget.DeviceOrigin(c).Add(paintengine2d.Pt(0, c.Bounds().Dy()))
		}
		p.Host.ContextMenu(from, at)
		return true
	}
	return p.Host.Keys(w, e)
}
