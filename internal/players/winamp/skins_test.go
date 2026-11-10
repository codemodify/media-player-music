package winamp

import (
	"strings"
	"testing"
	"time"

	"github.com/codemodify/media-player-music/internal/players"
	"github.com/codemodify/media-player-music/internal/players/playertest"
	"github.com/codemodify/media-player-music/internal/skins/panel"
	"goframework.io/desktop-ui"
	"goframework.io/desktop-ui-skins"
	"goframework.io/desktop-ui-widgets"
	"goframework.io/desktop-ui/a11y"
	"goframework.io/desktop-ui/platform"
	"goframework.io/desktop-ui/style"
	"goframework.io/desktop-ui/widget"
	"goframework.io/graphics-paint2d"
)

// scales is every display scale the panels are checked at.
var scales = []float32{1, 1.25, 1.5, 1.75, 2}

// Every panel skin ships the artwork the player paints. The two original
// packs also retain their specific rectangular/rounded-window contracts.
func TestThePanelSkinsAreThere(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	for _, id := range Skins {
		sk, ok := skins.LoadSkin(id)
		if !ok {
			t.Fatalf("%s did not load", id)
		}
		for _, sh := range sk.Sheets {
			if !sh.Covers(2) {
				t.Errorf("%s: sheet %s has no asset at 2x", id, sh.Name)
			}
			// Loaded sheet sampling includes any inherited design default.
			// Polished chrome is smooth; the original bitmap readouts stay
			// nearest on their own sheet. Silver is smooth throughout.
			if want := id != SkinSilver && sh.Name == "pixels"; sh.Pixelated != want {
				t.Errorf("%s: sheet %s pixelated = %v, want %v", id, sh.Name, sh.Pixelated, want)
			}
		}
		if id != SkinSilver && sk.Sheets["pixels"] == nil {
			t.Errorf("%s: no nearest-sampled original bitmap sheet", id)
		}
		if id == Skin || id == SkinSilver {
			shaped := sk.Window != nil && len(sk.Window.Shape) > 0
			if shaped != (id == SkinSilver) {
				t.Errorf("%s: declares a window shape = %v", id, shaped)
			}
		}
		p, _ := style.LoadTheme(id)
		for _, name := range panelSprites() {
			if _, _, ok := skins.SkinSpriteSize(p.Look(), name); !ok {
				t.Errorf("%s: no sprite %q, which the player paints", id, name)
			}
		}
	}
}

// panelSprites is every sprite a panel face paints by name.
func panelSprites() []string {
	out := []string{
		"main.face", "eq.face", "list.face", "thumb", "seek.thumb", "eq.thumb", "list.thumb",
		"led.colon", "state.play", "state.pause", "state.stop", "lamp.mono", "lamp.stereo",
		"key.skin",
	}
	for _, k := range []string{"prev", "play", "pause", "stop", "next", "eject", "shuffle", "repeat",
		"eq", "pl", "on", "auto", "presets", "add", "rem", "sel", "misc", "opts"} {
		out = append(out, "key."+k, "key."+k+".down")
	}
	for _, k := range []string{"prev", "play", "pause", "stop", "next", "eject"} {
		out = append(out, "mini."+k)
	}
	for _, d := range "0123456789" {
		out = append(out, "led."+string(d))
	}
	for _, r := range "ABCXYZ0189.:-()" {
		out = append(out, glyphName(r))
	}
	return out
}

// Every skin paints all three windows at every scale: the right size, not a
// blank, and — for the panels — the panel's own display where the layout
// says it is, which is what proves the art and the app agree about where
// things go at 1.25 and 1.75 as well as at the whole multiples.
func TestEverySkinPaintsEveryWindowAtEveryScale(t *testing.T) {
	for _, id := range Skins {
		for _, scale := range scales {
			a, p := open(t, id, scale)
			p.Pose(97 * time.Second)
			a.PumpOnce()
			for _, w := range []struct {
				name string
				win  *ui.Window
				h    int
			}{{"strip", p.Main(), panel.MainH}, {"equaliser", p.Eq(), panel.EqH}, {"playlist", p.List(), panel.ListH}} {
				img := w.win.Capture()
				if img == nil || blank(img) {
					t.Fatalf("%s@%gx: the %s painted nothing", id, scale, w.name)
				}
				if ww, hh := w.win.Size(); ww != panel.WindowW || hh != w.h {
					t.Errorf("%s@%gx: the %s is %dx%d, want %dx%d", id, scale, w.name, ww, hh, panel.WindowW, w.h)
				}
			}
			f := faceOf(p.Main().Look())
			want := faceClassic
			if id == SkinSilver {
				want = faceSilver
			}
			if f != want {
				t.Errorf("%s@%gx: laid out as face %d, want %d", id, scale, f, want)
			}
			if f.panelled() {
				checkDisplay(t, id, scale, p)
			}
		}
	}
}

// checkDisplay samples the middle of the clock's well in the strip and asks
// that it be the display's colour — near black in the classic panel, blue in
// the silver one — rather than chrome, which is what it would be if the art
// and the skin's layout had drifted apart.
func checkDisplay(t *testing.T, id string, scale float32, p *Player) {
	t.Helper()
	img := p.Main().Capture()
	o := widget.DeviceOrigin(p.strip)
	// Capture crops the surface's shadow margin; DeviceOrigin includes it.
	window := p.Main().WindowRect().Min
	o.X, o.Y = o.X-window.X, o.Y-window.Y
	d, ok := skins.SkinSlotRect(p.strip.Look(), layoutStrip, "display", p.strip.LocalBounds())
	if !ok {
		t.Fatalf("%s@%gx: the skin states no display slot", id, scale)
	}
	// A point between the clock and the analyser: the well's own colour.
	x := int(o.X + d.Min.X + 4*scale)
	y := int(o.Y + d.Max.Y - 3*scale)
	if x >= img.Width || y >= img.Height {
		t.Fatalf("%s@%gx: the display is off the window", id, scale)
	}
	r, gg, b, _ := img.PremulAt(x, y)
	switch id {
	case Skin:
		if r > 40 || gg > 40 || b > 40 {
			t.Errorf("%s@%gx: the display well is %d,%d,%d, not black", id, scale, r, gg, b)
		}
	case SkinSilver:
		if b < 80 || int(b) < int(r)+30 {
			t.Errorf("%s@%gx: the display well is %d,%d,%d, not blue", id, scale, r, gg, b)
		}
	}
}

// The silver windows are cut to four round corners, and it is the same
// outline at every scale: the corner rows are narrower than the window,
// every row between them is the whole width, and the fractions agree across
// scales, because the shape is stated in design pixels and resolved rather
// than stretched.
func TestTheSilverWindowsAreRoundAtEveryScale(t *testing.T) {
	const n = 64
	for _, w := range []string{"strip", "equaliser", "playlist"} {
		var base []float64
		for _, scale := range scales {
			_, p := open(t, SkinSilver, scale)
			win := map[string]*ui.Window{"strip": p.Main(), "equaliser": p.Eq(), "playlist": p.List()}[w]
			if !win.ShapeActive() {
				t.Fatalf("%s@%gx: not shaped", w, scale)
			}
			out := playertest.Outline(win, n)
			if len(out) != n {
				t.Fatalf("%s@%gx: no silhouette", w, scale)
			}
			if out[0] > 0.99 || out[n-1] > 0.99 {
				t.Errorf("%s@%gx: the corners are not cut (%.3f top, %.3f bottom)", w, scale, out[0], out[n-1])
			}
			if out[0] < 0.9 || out[n-1] < 0.9 {
				t.Errorf("%s@%gx: the corners cut too deep (%.3f top, %.3f bottom)", w, scale, out[0], out[n-1])
			}
			for i := 4; i < n-4; i++ {
				if out[i] < 0.999 {
					t.Errorf("%s@%gx: row %d of %d covers %.3f — only the corners should be cut", w, scale, i, n, out[i])
					break
				}
			}
			if base == nil {
				base = out
				continue
			}
			if d := playertest.OutlineDiff(base, out); d > 0.02 {
				t.Errorf("%s@%gx: the outline differs from 1x by %.3f of the width", w, scale, d)
			}
		}
	}
}

// The skin key cycles through the optional Winamp skins,
// every window following each step, the key naming the skin it is on.
func TestTheSkinKeySelectsTheOtherPanel(t *testing.T) {
	a, p := open(t, Skin, 1)
	p.strip.skinBtn.AccessibleAction(0, a11y.ActionDefault)
	a.PumpOnce()
	for _, w := range []*ui.Window{p.Main(), p.Eq(), p.List()} {
		if got := style.LookAppearance(w.Look()).Name; got != SkinSilver {
			t.Fatalf("after the skin key: %s wears %q, want %q", w.Title(), got, SkinSilver)
		}
	}
	if !strings.Contains(p.strip.skinBtn.Label, SkinLabel(SkinSilver)) {
		t.Errorf("the skin key says %q on Silver", p.strip.skinBtn.Label)
	}
	auditAll(t, SkinSilver, p)
	var chosen string
	p.Host.ChooseSkin = func(id string) { chosen = id }
	p.strip.skinBtn.AccessibleAction(0, a11y.ActionDefault)
	if chosen != SkinClassified {
		t.Fatalf("next skin from Silver chose %q, want Classified", chosen)
	}
}

func TestTheSkinSwitchIsOnTheKeyboard(t *testing.T) {
	a, p := open(t, Skin, 1)
	for _, w := range []*ui.Window{p.Main(), p.Eq(), p.List()} {
		p.SetSkin(Skin)
		w.Inject(platform.Event{Kind: platform.EventKeyDown, Key: platform.KeyK, Rune: 'k', Mods: platform.ModCtrl})
		a.PumpOnce()
		if p.Worn() != SkinSilver {
			t.Fatalf("Ctrl+K in %s gave %q, want Silver", w.Title(), p.Worn())
		}
		auditAll(t, SkinSilver, p)
	}
	var chosen string
	p.Host.ChooseSkin = func(id string) { chosen = id }
	p.Main().Inject(platform.Event{Kind: platform.EventKeyDown, Key: platform.KeyK, Rune: 'k', Mods: platform.ModCtrl | platform.ModShift})
	a.PumpOnce()
	if chosen != Themed {
		t.Fatalf("Ctrl+Shift+K chose %q, want default", chosen)
	}
	p.Transport.State = players.Stopped
	p.Main().Inject(platform.Event{Kind: platform.EventKeyDown, Key: platform.KeySpace})
	a.PumpOnce()
	if p.Transport.State != players.Playing {
		t.Error("Space stopped working after the skin was switched")
	}
}

func TestTheSkinMenuListsTheSkinsByName(t *testing.T) {
	a, p := open(t, Skin, 1)
	// A right-click on a key is a right-click on the face, not a press.
	if p.strip.stop.MousePress(widget.MouseEvent{Button: platform.ButtonRight}) {
		t.Error("a right-click pressed the stop key")
	}
	if !p.strip.MousePress(widget.MouseEvent{Button: platform.ButtonRight, Pos: paint2d.Pt(2, 60)}) {
		t.Fatal("the strip did not take a right-click")
	}
	a.PumpOnce()
	pop, ok := p.Main().Popup().(*widgets.PopupMenu)
	if !ok {
		t.Fatalf("a right-click opened %T, not a menu", p.Main().Popup())
	}
	if len(pop.Items) != len(Skins)+1 {
		t.Fatalf("skin menu has %d items, want default and %d skins", len(pop.Items), len(Skins))
	}
	names := map[string]*widgets.MenuItem{}
	for _, it := range pop.Items {
		names[it.Text] = it
	}
	for _, id := range append(append([]string(nil), Skins...), Themed) {
		label := SkinLabel(id)
		it := names[label]
		if it == nil {
			t.Fatalf("the menu has no %q: %v", label, pop.Items)
		}
		if it.Checked != (id == Skin) {
			t.Errorf("%q ticked = %v", it.Text, it.Checked)
		}
	}
	names[SkinLabel(SkinSilver)].OnClick()
	a.PumpOnce()
	if p.Worn() != SkinSilver {
		t.Fatalf("picking Winamp Silver wore %q", p.Worn())
	}
	auditAll(t, SkinSilver, p)

	// And from the keyboard: the menu key, in the equaliser.
	p.Eq().Inject(platform.Event{Kind: platform.EventKeyDown, Key: platform.KeyMenu})
	a.PumpOnce()
	if _, ok := p.Eq().Popup().(*widgets.PopupMenu); !ok {
		t.Errorf("the menu key opened %T, not the skin menu", p.Eq().Popup())
	}
}

// Every control in both panel layouts is reachable from the keyboard.
func TestEveryPanelControlIsOnTheKeyboard(t *testing.T) {
	for _, id := range Skins {
		a, p := open(t, id, 1)
		s := p.strip
		ring := playertest.FocusRing(t, a, p.Main())
		for _, c := range []widget.Component{
			s.prev, s.play, s.pause, s.stop, s.next, s.eject, s.seek, s.volume, s.balance,
			s.shuffle, s.repeat, s.eqBtn, s.listBtn, s.skinBtn,
		} {
			if !playertest.Reaches(ring, c) {
				t.Errorf("%s: Tab never reaches %q", id, nameOf(p.Main(), c))
			}
		}
		e := p.eqPane
		eqRing := playertest.FocusRing(t, a, p.Eq())
		for _, c := range []widget.Component{e.on, e.auto, e.presets, e.preamp} {
			if !playertest.Reaches(eqRing, c) {
				t.Errorf("%s: Tab never reaches the equaliser's %q", id, nameOf(p.Eq(), c))
			}
		}
		l := p.listPane
		listRing := playertest.FocusRing(t, a, p.List())
		for _, c := range []widget.Component{l.list, l.add, l.rem, l.sel, l.misc, l.opts, l.mini[0], l.mini[5]} {
			if !playertest.Reaches(listRing, c) {
				t.Errorf("%s: Tab never reaches the playlist's %q", id, nameOf(p.List(), c))
			}
		}
	}
}

// A switch is written down, and read back, in the config directory the
// test gave the player — which is where the next run looks. The writing is
// the application's (players.Settings), and the face only asks for it.
func auditAll(t *testing.T, id string, p *Player) {
	t.Helper()
	for name, w := range map[string]*ui.Window{"strip": p.Main(), "equaliser": p.Eq(), "playlist": p.List()} {
		playertest.Audit(t, id+" "+name, w)
	}
}
