package minim

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/codemodify/paintengine2d"
	"github.com/codemodify/uitoolkit/platform"
	"github.com/codemodify/uitoolkit/style"
	"github.com/codemodify/uitoolkit/widget"
	"github.com/codemodify/uitoolkit/widgets"
)

// The skin switch.
//
// Minim wears three skins, and changing between them is the call Settings
// makes for any theme — a skin is a pack, so choosing one is choosing the
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
// The choice is kept for the next run in $XDG_CONFIG_HOME/uitoolkit/
// minim.json, the one piece of state the player keeps; examples/minim reads
// it back before it builds the application, since the look is chosen when
// the application is.

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
func (p *Player) Worn() string { return style.LookAppearance(p.App.Look()).Name }

// SetSkin puts the player in a pack — one of its skins, or any other — and
// remembers the choice for the next run.
func (p *Player) SetSkin(id string) {
	if id == "" {
		return
	}
	if isSkin(id) {
		p.lastSkin = id
	}
	ap := style.LookAppearance(p.App.Look())
	ap.FollowDesktop = false
	ap.Name = id
	p.App.SetLook(style.WithAppearance(p.App.Look(), ap))
	if p.remember {
		_ = Remember(id)
	}
	p.restyle()
	p.refresh()
}

// NextSkin steps to the next of Minim's skins, and from the themed fallback
// back to the first.
func (p *Player) NextSkin() {
	cur := p.Worn()
	for i, id := range Skins {
		if id == cur {
			p.SetSkin(Skins[(i+1)%len(Skins)])
			return
		}
	}
	p.SetSkin(Skins[0])
}

// DropSkin drops the skin for the themed fallback, or puts back the skin
// that was dropped.
func (p *Player) DropSkin() {
	if isSkin(p.Worn()) {
		p.SetSkin(Themed)
		return
	}
	last := p.lastSkin
	if last == "" {
		last = Skin
	}
	p.SetSkin(last)
}

func isSkin(id string) bool {
	for _, s := range Skins {
		if s == id {
			return true
		}
	}
	return false
}

// skinMenu opens the menu of skins at a point in from's window: the three
// by name and the themed fallback, the one being worn ticked.
//
// It is four rows and no more, on purpose. A menu is drawn inside the window
// it opens from, and the strip is a hundred and sixteen design pixels tall:
// a fifth row, a separator or a shortcut column would have the menu scroll
// or cut its own labels off in the one window it is most often opened from.
// The keys are on the skin key's name and in docs/players.md instead.
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

// ---- remembering it --------------------------------------------------------------

// stateFile is where the choice is kept.
func stateFile() string { return filepath.Join(style.ConfigDir(), "minim.json") }

type savedState struct {
	Skin string `json:"skin"`
}

// Remembered is the pack the player was last switched to, or "" when it
// never was or the file says something this build cannot wear.
func Remembered() string {
	b, err := os.ReadFile(stateFile())
	if err != nil {
		return ""
	}
	var s savedState
	if json.Unmarshal(b, &s) != nil || s.Skin == "" {
		return ""
	}
	if _, ok := style.LoadTheme(s.Skin); !ok && !style.IsSkin(s.Skin) {
		return ""
	}
	return s.Skin
}

// Remember keeps a pack as the one to start in next time.
func Remember(id string) error {
	path := stateFile()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(savedState{Skin: id}, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(b, '\n'), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
