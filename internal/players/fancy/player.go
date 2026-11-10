// Package fancy presents the sculpted cyan player with shared playback state.
package fancy

import (
	"fmt"
	"time"

	"github.com/codemodify/media-player-music/internal/players"
	"goframework.io/desktop-ui"
	"goframework.io/desktop-ui-rack"
	"goframework.io/desktop-ui-skins"
	"goframework.io/desktop-ui-widgets"
	"goframework.io/desktop-ui/platform"
	"goframework.io/desktop-ui/widget"
	"goframework.io/graphics-paint2d"
)

const (
	LayoutMain      = "fancy.main"
	LayoutEqualizer = "fancy.equalizer"
	LayoutPlaylist  = "fancy.playlist"
	LayoutCompact   = "fancy.compact"
	MainW           = 420
	MainH           = 220
	CompactH        = 40
	EqW             = 420
	EqH             = 124
	ListW           = 420
	ListH           = 144
	MinListW        = ListW
	MinListH        = 124
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
	body, compact     *mainPane
	eqPane            *equalizerPane
	listPane          *playlistPane
	compactMode       bool
	modePending       bool
	modeTries         int
	dropLook          func()
	dialogs           map[*widgets.FileDialog]bool
	Desk              *rack.Desk
	iMain, iEq, iList int
	stacked           bool
	tries             int
	restoredPlacement map[string]players.WindowPlacement
}

func New(h *players.Host, opts Options) (*Player, error) {
	for _, role := range []string{LayoutMain, LayoutCompact, LayoutEqualizer, LayoutPlaylist} {
		if _, ok := skins.SkinLayoutOf(h.App.Look(), role); !ok {
			return nil, fmt.Errorf("Fancy requires skin layout %q", role)
		}
	}
	p := &Player{Host: h, App: h.App, Transport: h.Model.Transport, Spectrum: h.Model.Spectrum, Equaliser: h.Model.Equaliser}
	var err error
	p.Main, err = p.open(opts, "Music Player", LayoutMain, MainW, MainH, true)
	if err != nil {
		return nil, err
	}
	p.body, p.compact = newMainPane(p, false), newMainPane(p, true)
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
		p.updateMainShape()
		p.setShape(p.Eq, LayoutEqualizer)
		p.setShape(p.List, LayoutPlaylist)
		p.Refresh()
	})
	p.ShowEqualiser(h.Model.EqualizerVisible)
	p.ShowPlaylist(h.Model.PlaylistVisible)
	p.Refresh()
	p.Main.RequestFocus(p.body.play)
	p.Eq.RequestFocus(p.eqPane.bands[0])
	p.List.RequestFocus(p.listPane.list)
	return p, nil
}
func (p *Player) open(opts Options, title, role string, width, height int, fixed bool) (*ui.Window, error) {
	o := platform.WindowOptions{Title: title, Headless: opts.Headless, Decorations: platform.DecorationsNone}
	if role != LayoutMain {
		o.Role = platform.RoleUtility
		o.Owner = p.Main.Surface()
	}
	if role == LayoutPlaylist {
		o.MinWidth, o.MinHeight = MinListW, MinListH
	}
	return players.OpenSized(p.App, o, width, height, fixed)
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
	for fd := range p.dialogs {
		fd.Cancel()
	}
	p.dialogs = nil
	for _, w := range []*ui.Window{p.List, p.Eq, p.Main} {
		if w != nil {
			w.Close()
		}
	}
}
func (p *Player) Pose(pos time.Duration)   { p.Host.Pose(pos); p.Refresh() }
func (p *Player) Advance(dt time.Duration) { p.retryMode(); p.AdvanceGeometry(dt); p.Refresh() }
func (p *Player) Refresh() {
	if p.Main == nil || p.Main.Closed() || p.eqPane == nil || p.listPane == nil {
		return
	}
	p.Equaliser.AutoForTrack(p.Transport.List.Index())
	p.body.sync()
	p.compact.sync()
	p.eqPane.sync()
	p.listPane.sync()
}
func (p *Player) ShowEqualiser(on bool) { p.showWindow(p.iEq, on) }
func (p *Player) ShowPlaylist(on bool)  { p.showWindow(p.iList, on) }
func (p *Player) updateMainShape() {
	role := LayoutMain
	if p.Compact() {
		role = LayoutCompact
	}
	p.setShape(p.Main, role)
}
func (p *Player) setShape(w *ui.Window, role string) {
	w.SetShapeFunc(func(size paint2d.Point, _ float32) *platform.Shape {
		im := paint2d.NewImage(int(size.X+.5), int(size.Y+.5))
		ctx := paint2d.NewContext(im)
		ctx.Clear(paint2d.Transparent)
		box := paint2d.XYWH(0, 0, size.X, size.Y)
		r, ok := skins.SkinSlotRect(w.Look(), role, "shape."+role[len("fancy."):], box)
		if !ok || !skins.DrawSkinSlot(w.Look(), ctx, r, role, "shape."+role[len("fancy."):], 0) {
			return nil
		}
		// Source artwork contains its antialias coverage. The window mask supplies
		// only the extent so those translucent edge pixels are applied once.
		for a := 3; a < len(im.Pix); a += 4 {
			if im.Pix[a] != 0 {
				im.Pix[a-3], im.Pix[a-2], im.Pix[a-1], im.Pix[a] = 255, 255, 255, 255
			}
		}
		return platform.NewShapeImage(im)
	})
}
func (p *Player) Compact() bool { return p.compactMode }
func (p *Player) SetCompact(on bool) {
	if p.Main == nil || p.Main.Closed() || p.compactMode == on {
		return
	}
	p.compactMode = on
	p.modeTries = 0
	p.modePending = true
	p.Main.SetFullscreen(false)
	p.Main.SetMaximized(false)
	pane := p.body
	h := MainH
	if on {
		pane = p.compact
		h = CompactH
	}
	p.Main.SetContent(pane)
	p.Main.SetSize(MainW, h)
	if p.Desk != nil {
		p.Desk.Rack.Resize(p.iMain, MainW, h)
		p.placeDockedWindows()
		p.Desk.Apply()
	}
	p.updateMainShape()
	p.Refresh()
	p.Main.RequestFocus(pane.play)
}
func (p *Player) retryMode() {
	if !p.modePending || p.Main == nil || p.Main.Closed() {
		return
	}
	height := MainH
	if p.Compact() {
		height = CompactH
	}
	w, h := p.Main.Size()
	if w == MainW && h == height || p.modeTries >= settleTries {
		p.modePending = false
		return
	}
	p.modeTries++
	p.Main.SetSize(MainW, height)
}
func (p *Player) CaptureState() players.PresentationState {
	mode := "full"
	if p.Compact() {
		mode = "compact"
	}
	return players.PresentationState{Mode: mode}
}
func (p *Player) RestoreState(s players.PresentationState) { p.SetCompact(s.Mode == "compact") }
func (p *Player) keys(w *ui.Window, e widget.KeyEvent) bool {
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
		at := widget.DeviceOrigin(from).Add(paint2d.Pt(12*w.Scale(), 16*w.Scale()))
		if c := w.Focus(); c != nil {
			from = c
			at = widget.DeviceOrigin(c).Add(paint2d.Pt(0, c.Bounds().Dy()))
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
