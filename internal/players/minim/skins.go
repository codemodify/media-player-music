package minim

import (
	"github.com/codemodify/paintengine2d"
	"github.com/codemodify/uitoolkit/platform"
	"github.com/codemodify/uitoolkit/style"
	"github.com/codemodify/uitoolkit/widget"
	"github.com/codemodify/uitoolkit/widgets"
)

// The skin switch.
//
// This face wears three skins, and changing between them is the call
// Settings makes for any theme — a skin is a pack, so choosing one is choosing the
// appearance's pack id and rebuilding the look. Every window of the
// application follows, because that is what Application.SetLook does, and
// nothing in the widget tree is rebuilt: the faces (face.go) move and
// repaint the same controls.
//
// It is reachable three ways, because a control only the pointer can find is
// not finished:
//
//   - the skin key in the strip's own chrome, which steps to the next skin;
//   - Ctrl+K from any of the three windows, which does the same, and
//     Ctrl+Shift+K, which drops the skin for the themed fallback and puts it
//     back — Lantern's switch, kept;
//   - a menu listing the skins by name, on a right-click anywhere on the
//     face of any window, or on the menu key (Shift+F10) from the keyboard.
//
// The switching itself is the application's (players.Host): a skin is a
// pack, the same call whichever face asks for it, and the choice is written
// down per face in the application's settings for the next run.

// The skins, and the pack the skin drops to.
const (
	SkinClassic = "minim-classic"
	SkinSilver  = "minim-silver"
	Themed      = "breeze-night"
)

// Skins is the order the skin key and Ctrl+K step through.
var Skins = []string{Skin, SkinClassic, SkinSilver}

// SkinLabel is what a pack is called in the skin menu and the skin key's
// name.
func SkinLabel(id string) string {
	switch id {
	case Skin:
		return "Minim"
	case SkinClassic:
		return "Minim Classic"
	case SkinSilver:
		return "Minim Silver"
	case Themed:
		return "No skin (" + Themed + ")"
	}
	if sk, ok := style.LoadSkin(id); ok && sk.Label != "" {
		return sk.Label
	}
	return id
}

// Worn is the pack the player is wearing now.
func (p *Player) Worn() string { return p.Host.Worn() }

// SetSkin puts the player in a pack — one of its skins, or any other. The
// application applies it to every window it has and remembers the choice.
func (p *Player) SetSkin(id string) {
	p.Host.SetSkin(id)
	p.restyle()
	p.refresh()
}

// NextSkin steps to the next of this face's skins, and from the themed
// fallback back to the first.
func (p *Player) NextSkin() {
	p.Host.NextSkin()
	p.restyle()
	p.refresh()
}

// DropSkin drops the skin for the themed fallback, or puts back the skin
// that was dropped.
func (p *Player) DropSkin() {
	p.Host.DropSkin()
	p.restyle()
	p.refresh()
}

// skinMenu opens the menu of skins at a point in from's window: the three
// by name and the themed fallback, the one being worn ticked.
//
// It is four rows and no more, on purpose. A menu is drawn inside the window
// it opens from, and the strip is a hundred and sixteen design pixels tall:
// a fifth row, a separator or a shortcut column would have the menu scroll
// or cut its own labels off in the one window it is most often opened from.
// The keys are on the skin key's name and in docs/faces.md instead.
func (p *Player) skinMenu(from widget.Component, at paintengine2d.Point) {
	cur := p.Worn()
	var items []*widgets.MenuItem
	for _, id := range append(append([]string(nil), Skins...), Themed) {
		id := id
		label := SkinLabel(id)
		if id == Themed {
			label = "No skin"
		}
		items = append(items, widgets.RadioItem(label, "skin", id == cur, func() { p.SetSkin(id) }))
	}
	widgets.ShowContextMenu(from, at, items...)
}

// skinKeys is the part of the keyboard the switch answers to, shared by the
// three windows. It reports whether it took the key.
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
		// The menu opens at the focused control, or at the window's top
		// left when nothing is — somewhere the eye already is.
		at := paintengine2d.Pt(8, 8)
		if f := w.Focus(); f != nil {
			o := widget.DeviceOrigin(f)
			at = paintengine2d.Pt(o.X, o.Y+f.Bounds().Dy())
			from = f
		}
		p.skinMenu(from, at)
		return true
	}
	return false
}

// rightClick is a pane's answer to a press: the skin menu for the secondary
// button, and nothing for the others.
func (p *Player) rightClick(c widget.Component, e widget.MouseEvent) bool {
	if e.Button != platform.ButtonRight {
		return false
	}
	o := widget.DeviceOrigin(c)
	p.skinMenu(c, paintengine2d.Pt(o.X+e.Pos.X, o.Y+e.Pos.Y))
	return true
}
