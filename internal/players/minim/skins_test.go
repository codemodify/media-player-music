package minim

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/codemodify/paintengine2d"
	"github.com/codemodify/uitoolkit/app"
	"github.com/codemodify/media-player-music/internal/players"
	"github.com/codemodify/media-player-music/internal/players/playertest"
	"github.com/codemodify/uitoolkit/platform"
	"github.com/codemodify/media-player-music/internal/players/minim/panel"
	"github.com/codemodify/uitoolkit/style"
	"github.com/codemodify/uitoolkit/widget"
	"github.com/codemodify/uitoolkit/widgets"
)

// scales is every display scale the panels are checked at.
var scales = []float32{1, 1.25, 1.5, 1.75, 2}

// The two panel skins ship, load and are what they say they are: the
// classic one a pixel sheet with no outline of its own, the silver one drawn
// from paths and cut to a rounded window. Both carry every face and every
// key the player paints, so a skin missing one fails here rather than on a
// screen as a blank hole in a panel.
func TestThePanelSkinsAreThere(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	for _, id := range []string{SkinClassic, SkinSilver} {
		sk, ok := style.LoadSkin(id)
		if !ok {
			t.Fatalf("%s did not load", id)
		}
		for _, sh := range sk.Sheets {
			if !sh.Covers(2) {
				t.Errorf("%s: sheet %s has no asset at 2x", id, sh.Name)
			}
		}
		if got := sk.Design.Pixelated; got != (id == SkinClassic) {
			t.Errorf("%s: pixelated = %v", id, got)
		}
		shaped := sk.Window != nil && len(sk.Window.Shape) > 0
		if shaped != (id == SkinSilver) {
			t.Errorf("%s: declares a window shape = %v", id, shaped)
		}
		p, _ := style.LoadTheme(id)
		for _, name := range panelSprites() {
			if _, _, ok := style.SkinSpriteSize(p.Look(), name); !ok {
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
	for _, id := range append(append([]string(nil), Skins...), Themed) {
		for _, scale := range scales {
			a, p := open(t, id, scale)
			p.Pose(97 * time.Second)
			a.PumpOnce()
			for _, w := range []struct {
				name string
				win  *app.Window
				h    int
			}{{"strip", p.Main, panel.MainH}, {"equaliser", p.Eq, panel.EqH}, {"playlist", p.List, panel.ListH}} {
				img := w.win.Capture()
				if img == nil || blank(img) {
					t.Fatalf("%s@%gx: the %s painted nothing", id, scale, w.name)
				}
				if ww, hh := w.win.Size(); ww != panel.WindowW || hh != w.h {
					t.Errorf("%s@%gx: the %s is %dx%d, want %dx%d", id, scale, w.name, ww, hh, panel.WindowW, w.h)
				}
			}
			f := faceOf(p.Main.Look())
			if want := map[string]face{SkinClassic: faceClassic, SkinSilver: faceSilver}[id]; f != want {
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
	img := p.Main.Capture()
	o := widget.DeviceOrigin(p.strip)
	d, ok := style.SkinSlotRect(p.strip.Look(), layoutStrip, "display", p.strip.LocalBounds())
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
	case SkinClassic:
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
			win := map[string]*app.Window{"strip": p.Main, "equaliser": p.Eq, "playlist": p.List}[w]
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

// The skin key steps through Minim's three skins and back to the first,
// every window following each step, the key naming the skin it is on.
func TestTheSkinKeyCyclesEverySkinAndBack(t *testing.T) {
	a, p := open(t, Skin, 1)
	want := []string{SkinClassic, SkinSilver, Skin}
	for _, id := range want {
		p.strip.skinBtn.AccessibleAction(0)
		a.PumpOnce()
		for _, w := range []*app.Window{p.Main, p.Eq, p.List} {
			if got := style.LookAppearance(w.Look()).Name; got != id {
				t.Fatalf("after the skin key: %s wears %q, want %q", w.Title(), got, id)
			}
		}
		if !strings.Contains(p.strip.skinBtn.Label, SkinLabel(id)) {
			t.Errorf("the skin key says %q on %s", p.strip.skinBtn.Label, id)
		}
		auditAll(t, id, p)
	}
}

// Ctrl+K steps from any of the three windows, Ctrl+Shift+K drops the skin
// for the themed fallback and puts back the one it dropped, and every stop
// on the way keeps a11y.Check clean.
func TestTheSkinSwitchIsOnTheKeyboard(t *testing.T) {
	a, p := open(t, Skin, 1)
	key := func(w *app.Window, mods platform.Modifiers) {
		w.Inject(platform.Event{Kind: platform.EventKeyDown, Key: platform.KeyK, Rune: 'k', Mods: mods})
		a.PumpOnce()
	}
	key(p.Eq, platform.ModCtrl)
	if p.Worn() != SkinClassic {
		t.Fatalf("Ctrl+K in the equaliser gave %q", p.Worn())
	}
	key(p.List, platform.ModCtrl)
	if p.Worn() != SkinSilver {
		t.Fatalf("Ctrl+K in the playlist gave %q", p.Worn())
	}
	key(p.Main, platform.ModCtrl|platform.ModShift)
	if p.Worn() != Themed {
		t.Fatalf("Ctrl+Shift+K gave %q, want the themed fallback", p.Worn())
	}
	auditAll(t, Themed, p)
	if p.Main.ShapeActive() {
		t.Error("the themed fallback is still shaped")
	}
	key(p.Main, platform.ModCtrl|platform.ModShift)
	if p.Worn() != SkinSilver {
		t.Fatalf("Ctrl+Shift+K again gave %q, want the skin it dropped", p.Worn())
	}
	key(p.Main, platform.ModCtrl)
	if p.Worn() != Skin {
		t.Fatalf("Ctrl+K from silver gave %q, want the first again", p.Worn())
	}
	// The transport keys are still the transport's after all of that.
	p.Transport.State = players.Stopped
	p.Main.Inject(platform.Event{Kind: platform.EventKeyDown, Key: platform.KeySpace})
	a.PumpOnce()
	if p.Transport.State != players.Playing {
		t.Error("Space stopped working after the skin was switched")
	}
}

// A right-click on the face, and the menu key from the keyboard, open a menu
// that lists the skins by name with the one being worn ticked; picking one
// wears it.
func TestTheSkinMenuListsTheSkinsByName(t *testing.T) {
	a, p := open(t, SkinClassic, 1)
	// A right-click on a key is a right-click on the face, not a press.
	if p.strip.stop.MousePress(widget.MouseEvent{Button: platform.ButtonRight}) {
		t.Error("a right-click pressed the stop key")
	}
	if !p.strip.MousePress(widget.MouseEvent{Button: platform.ButtonRight, Pos: paintengine2d.Pt(2, 60)}) {
		t.Fatal("the strip did not take a right-click")
	}
	a.PumpOnce()
	pop, ok := p.Main.Popup().(*widgets.PopupMenu)
	if !ok {
		t.Fatalf("a right-click opened %T, not a menu", p.Main.Popup())
	}
	names := map[string]*widgets.MenuItem{}
	for _, it := range pop.Items {
		names[it.Text] = it
	}
	for _, id := range append(append([]string(nil), Skins...), Themed) {
		label := SkinLabel(id)
		if id == Themed {
			label = "No skin"
		}
		it := names[label]
		if it == nil {
			t.Fatalf("the menu has no %q: %v", label, pop.Items)
		}
		if it.Checked != (id == SkinClassic) {
			t.Errorf("%q ticked = %v", it.Text, it.Checked)
		}
	}
	names[SkinLabel(SkinSilver)].OnClick()
	a.PumpOnce()
	if p.Worn() != SkinSilver {
		t.Fatalf("picking Minim Silver wore %q", p.Worn())
	}
	auditAll(t, SkinSilver, p)

	// And from the keyboard: the menu key, in the equaliser.
	p.Eq.Inject(platform.Event{Kind: platform.EventKeyDown, Key: platform.KeyMenu})
	a.PumpOnce()
	if _, ok := p.Eq.Popup().(*widgets.PopupMenu); !ok {
		t.Errorf("the menu key opened %T, not the skin menu", p.Eq.Popup())
	}
}

// Every control a panel face shows is on the keyboard — the skin key and the
// three keys the panels add among them — and the ones it hides are not.
func TestEveryPanelControlIsOnTheKeyboard(t *testing.T) {
	for _, id := range []string{SkinClassic, SkinSilver} {
		a, p := open(t, id, 1)
		s := p.strip
		ring := playertest.FocusRing(t, a, p.Main)
		for _, c := range []widget.Component{
			s.prev, s.play, s.pause, s.stop, s.next, s.eject, s.seek, s.volume, s.balance,
			s.shuffle, s.repeat, s.eqBtn, s.listBtn, s.skinBtn,
		} {
			if !playertest.Reaches(ring, c) {
				t.Errorf("%s: Tab never reaches %q", id, nameOf(p.Main, c))
			}
		}
		if playertest.Reaches(ring, s.mute) {
			t.Errorf("%s: Tab reaches the mute key the panel hides", id)
		}
		e := p.eqPane
		eqRing := playertest.FocusRing(t, a, p.Eq)
		for _, c := range []widget.Component{e.on, e.auto, e.presets, e.preamp} {
			if !playertest.Reaches(eqRing, c) {
				t.Errorf("%s: Tab never reaches the equaliser's %q", id, nameOf(p.Eq, c))
			}
		}
		l := p.listPane
		listRing := playertest.FocusRing(t, a, p.List)
		for _, c := range []widget.Component{l.list, l.add, l.rem, l.sel, l.misc, l.opts, l.mini[0], l.mini[5]} {
			if !playertest.Reaches(listRing, c) {
				t.Errorf("%s: Tab never reaches the playlist's %q", id, nameOf(p.List, c))
			}
		}
	}
}

// A switch is written down, and read back, in the config directory the
// test gave the player — which is where the next run looks.
func TestTheSkinIsRememberedForTheNextRun(t *testing.T) {
	a, p := open(t, Skin, 1)
	p.remember = true
	if got := Remembered(); got != "" {
		t.Fatalf("a fresh config remembers %q", got)
	}
	p.NextSkin()
	a.PumpOnce()
	if got := Remembered(); got != SkinClassic {
		t.Fatalf("remembered %q, want %q", got, SkinClassic)
	}
	// A file that names something this build cannot wear is not trusted.
	if err := os.WriteFile(stateFile(), []byte(`{"skin": "no-such-pack"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := Remembered(); got != "" {
		t.Errorf("remembered %q from a file naming no pack", got)
	}
}

// auditAll runs a11y.Check over the three windows.
func auditAll(t *testing.T, id string, p *Player) {
	t.Helper()
	for name, w := range map[string]*app.Window{"strip": p.Main, "equaliser": p.Eq, "playlist": p.List} {
		playertest.Audit(t, id+" "+name, w)
	}
}
