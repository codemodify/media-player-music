package winamp

import (
	"github.com/codemodify/media-player-music/internal/players"
	"goframework.io/desktop-ui/platform"
	"goframework.io/desktop-ui/widget"
	"goframework.io/graphics-paint2d"
)

const SkinSilver = players.WinampSilverSkin
const SkinClassified = players.WinampClassifiedSkin

var Themed = players.Themed

// Skins are the application-owned panel packs. The shell owns the
// default presentation and replaces the windows when the skin is dropped.
var Skins = players.PanelSkins()

func SkinLabel(id string) string {
	return players.SkinLabel(id)
}

func (p *Player) Worn() string { return p.Host.Worn() }

// Skin choices go through the host so the shell can switch between its
// default windows and the skinned windows without changing player state.
func (p *Player) SetSkin(id string) { p.Host.SetSkin(id) }
func (p *Player) NextSkin()         { p.Host.NextSkin() }
func (p *Player) DropSkin()         { p.Host.DropSkin() }

// The toolkit scrolls the full choice list when it exceeds the popup area,
// including the compact in-window fallback.
func (p *Player) skinMenu(from widget.Component, at paint2d.Point) {
	p.Host.ContextMenu(from, at)
}

func (p *Player) skinKeys(w interface{ Focus() widget.Component }, e widget.KeyEvent, from widget.Component) bool {
	if e.Mods.Ctrl() && e.Key == platform.KeyK {
		if e.Mods.Shift() {
			p.DropSkin()
		} else {
			p.NextSkin()
		}
		return true
	}
	if e.Key == platform.KeyMenu || (e.Key == platform.KeyF10 && e.Mods.Shift()) {
		at := paint2d.Pt(8, 8)
		if f := w.Focus(); f != nil {
			o := widget.DeviceOrigin(f)
			at = paint2d.Pt(o.X, o.Y+f.Bounds().Dy())
			from = f
		}
		p.skinMenu(from, at)
		return true
	}
	return false
}

func (p *Player) rightClick(c widget.Component, e widget.MouseEvent) bool {
	if e.Button != platform.ButtonRight {
		return false
	}
	o := widget.DeviceOrigin(c)
	p.skinMenu(c, paint2d.Pt(o.X+e.Pos.X, o.Y+e.Pos.Y))
	return true
}
