package panel

import (
	"strings"
	"testing"
	"time"

	"github.com/codemodify/media-player-music/internal/players"
	"github.com/codemodify/media-player-music/internal/players/playertest"
	"goframework.io/desktop-ui"
	"goframework.io/desktop-ui-rack"
	"goframework.io/desktop-ui/a11y"
	"goframework.io/desktop-ui/platform"
	"goframework.io/desktop-ui/style"
	"goframework.io/desktop-ui/widget"
	"goframework.io/graphics-paint2d"
)

// open is a headless player in a named pack at a scale.
func open(t *testing.T, pack string, scale float32) (*ui.Application, *Player) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv(style.ThemeEnv, pack)
	a := ui.New(ui.Options{Headless: true, Scale: scale, DisableLookWatch: true})
	// The face on its own, with a model of its own: a host with no
	// application behind it is what a face's own tests open it with.
	h := players.NewHost(a)
	h.Packs = func() []string { return Skins }
	p, err := New(h, Options{Headless: true, Scale: scale})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(p.Close)
	a.PumpOnce()
	playertest.SettleGroup(t, a, p.g)
	return a, p
}

// Every window exposes named controls and track information in every skin.
func TestEveryWindowIsAccessible(t *testing.T) {
	for _, pack := range Skins {
		t.Run(pack, func(t *testing.T) {
			a, p := open(t, pack, 1)
			for name, w := range map[string]*ui.Window{
				"strip": p.Main(), "equaliser": p.Eq(), "playlist": p.List(),
			} {
				tree := playertest.Audit(t, name, w)
				if playertest.Count(tree, a11y.RoleWindow) == 0 {
					t.Errorf("%s: no window node", name)
				}
			}
			// The strip's controls are all named: a transport row is
			// glyphs, so an unnamed one is a button a screen reader calls
			// "button" and nothing else.
			tree := p.Main().AccessibleTree()
			for _, want := range []string{"Play", "Stop", "Next track", "Previous track", "Seek", "Volume"} {
				if !hasNamed(tree, want) {
					t.Errorf("the strip has no node named %q", want)
				}
			}
			if playertest.Count(p.Eq().AccessibleTree(), a11y.RoleSlider) < len(players.EqBands)+1 {
				t.Errorf("the equaliser has %d sliders, want %d",
					playertest.Count(p.Eq().AccessibleTree(), a11y.RoleSlider), len(players.EqBands)+1)
			}
			_ = a
		})
	}
}

// Everything the pointer can work, the keyboard can reach.
func TestEveryControlIsOnTheKeyboard(t *testing.T) {
	a, p := open(t, Skin, 1)
	ring := playertest.FocusRing(t, a, p.Main())
	if len(ring) == 0 {
		t.Fatal("nothing in the strip takes the focus")
	}
	for _, c := range []widget.Component{
		p.strip.prev, p.strip.play, p.strip.stop, p.strip.next,
		p.strip.pause, p.strip.eject, p.strip.balance, p.strip.seek, p.strip.volume,
		p.strip.shuffle, p.strip.repeat, p.strip.eqBtn, p.strip.listBtn, p.strip.skinBtn,
	} {
		if !playertest.Reaches(ring, c) {
			t.Errorf("Tab never reaches %v (%q)", c.Name(), nameOf(p.Main(), c))
		}
	}
	// And the equaliser's eleven faders.
	eqRing := playertest.FocusRing(t, a, p.Eq())
	if len(eqRing) < len(players.EqBands)+1 {
		t.Errorf("the equaliser's focus ring has %d stops, want at least %d",
			len(eqRing), len(players.EqBands)+1)
	}
}

// The shared transport keys work wherever the focus is, in every window.
func TestTheTransportKeysWorkInEveryWindow(t *testing.T) {
	a, p := open(t, Skin, 1)
	for _, w := range []*ui.Window{p.Main(), p.Eq(), p.List()} {
		p.Transport.State = players.Stopped
		a.PumpOnce()
		w.Inject(platform.Event{Kind: platform.EventKeyDown, Key: platform.KeySpace})
		a.PumpOnce()
		if p.Transport.State != players.Playing {
			t.Errorf("%s: space did not start the transport (%v)", w.Title(), p.Transport.State)
		}
	}
	// And the keys that are letters.
	before := p.Transport.List.Index()
	p.Main().Inject(platform.Event{Kind: platform.EventKeyDown, Key: platform.KeyB})
	a.PumpOnce()
	if p.Transport.List.Index() == before {
		t.Error("B did not move to the next track")
	}
	p.Main().Inject(platform.Event{Kind: platform.EventKeyDown, Key: platform.KeyM})
	a.PumpOnce()
	if !p.Transport.Muted() {
		t.Error("M did not mute")
	}
}

// Moving the main window carries its attached equaliser and playlist.
func TestTheStackTravelsWithTheStrip(t *testing.T) {
	a, p := open(t, Skin, 1)
	if p.g.Mode() != rack.GroupWindows {
		t.Skip("this backend does not place windows")
	}
	eq := p.g.Rack.Pane(p.iEq)
	list := p.g.Rack.Pane(p.iList)
	if eq.To != p.iMain || eq.Bond.Side != rack.SideBottom {
		t.Fatalf("the equaliser is bonded to %d on the %v", eq.To, eq.Bond.Side)
	}
	if list.To != p.iEq {
		t.Fatalf("the playlist is bonded to %d, want the equaliser (%d)", list.To, p.iEq)
	}

	// Move the strip the way a window manager would, and let the group
	// follow the desktop's word.
	desktopMove(a, p.Main(), 400, 260)

	main := p.g.Rack.Pane(p.iMain).Box
	if main.X != 400 || main.Y != 260 {
		t.Fatalf("the strip is at %+v", main)
	}
	if eq.Box.X != main.X || eq.Box.Y != main.Bottom() {
		t.Errorf("the equaliser is at %+v, want flush under %+v", eq.Box, main)
	}
	if list.Box.X != eq.Box.X || list.Box.Y != eq.Box.Bottom() {
		t.Errorf("the playlist is at %+v, want flush under %+v", list.Box, eq.Box)
	}
	// And the desktop was actually told.
	if x, y, ok := p.Eq().Position(); !ok || x != eq.Box.X || y != eq.Box.Y {
		t.Errorf("the equaliser's window is at %d,%d, the rack says %+v", x, y, eq.Box)
	}
}

// A window dragged away comes loose, and snaps back when it is let go near
// an edge again.
func TestAWindowComesLooseAndSnapsBack(t *testing.T) {
	a, p := open(t, Skin, 1)
	if p.g.Mode() != rack.GroupWindows {
		t.Skip("this backend does not place windows")
	}
	desktopMove(a, p.List(), 900, 700)
	if pane := p.g.Rack.Pane(p.iList); pane.To != -1 {
		t.Fatalf("the playlist stayed bonded to %d after being dragged away", pane.To)
	}
	// Let it go four pixels off flush under the equaliser.
	eq := p.g.Rack.Pane(p.iEq).Box
	desktopMove(a, p.List(), eq.X+4, eq.Bottom()+3)
	pane := p.g.Rack.Pane(p.iList)
	if pane.To != p.iEq || pane.Bond.Side != rack.SideBottom {
		t.Fatalf("the playlist did not snap back: bonded to %d on the %v", pane.To, pane.Bond.Side)
	}
	if pane.Box.X != eq.X || pane.Box.Y != eq.Bottom() {
		t.Errorf("snapped to %+v, want flush at %d,%d", pane.Box, eq.X, eq.Bottom())
	}
}

// The clock, the seek bar and the analyser agree with one another.
func TestTheDisplayFollowsTheTransport(t *testing.T) {
	a, p := open(t, Skin, 1)
	p.Pose(97 * time.Second)
	a.PumpOnce()
	if got := p.strip.seek.Value; got < 0.4 || got > 0.5 {
		t.Errorf("the seek bar reads %.3f at 1:37 of 3:47", got)
	}
	loud := false
	for _, v := range p.Spectrum.Bars {
		if v > 0.05 {
			loud = true
		}
	}
	if !loud {
		t.Error("the analyser is flat while the player is posed as playing")
	}
	tree := p.Main().AccessibleTree()
	if !hasNamedContaining(tree, "1:37") {
		t.Error("the display does not read the time out to a screen reader")
	}
}

// ---- helpers ------------------------------------------------------------------

func hasNamed(tree *a11y.Node, name string) bool {
	found := false
	tree.Walk(func(n *a11y.Node) bool {
		if n.Name == name {
			found = true
		}
		return !found
	})
	return found
}

func hasNamedContaining(tree *a11y.Node, part string) bool {
	found := false
	tree.Walk(func(n *a11y.Node) bool {
		if strings.Contains(n.Name, part) {
			found = true
		}
		return !found
	})
	return found
}

func nameOf(w *ui.Window, c widget.Component) string {
	if n := w.AccessibleTree().Find(c.ID()); n != nil {
		return n.Name
	}
	return ""
}

// blank reports whether an image is one flat colour, which is what a window
// that painted nothing looks like.
func blank(img *paint2d.Image) bool {
	r0, g0, b0, _ := img.PremulAt(0, 0)
	for y := 0; y < img.Height; y += 3 {
		for x := 0; x < img.Width; x += 3 {
			r, g, b, _ := img.PremulAt(x, y)
			if r != r0 || g != g0 || b != b0 {
				return false
			}
		}
	}
	return true
}

// A default desktop theme has its own presentation in the shell, and must
// never construct the removed compact widget layout.
func TestPanelRequiresAPanelLayout(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv(style.ThemeEnv, Themed)
	a := ui.New(ui.Options{Headless: true, DisableLookWatch: true})
	p, err := New(players.NewHost(a), Options{Headless: true})
	if err == nil || p != nil {
		t.Fatal("an unskinned theme constructed panel windows")
	}
}

func TestThePlayAndPauseKeysKeepTheirSeparateActions(t *testing.T) {
	for _, skin := range []string{Skin, SkinSilver} {
		t.Run(skin, func(t *testing.T) {
			_, p := open(t, skin, 1)
			for _, keys := range [][3]*players.GlyphButton{
				{p.strip.play, p.strip.pause, p.strip.stop},
				{p.listPane.mini[1], p.listPane.mini[2], p.listPane.mini[3]},
			} {
				play, pause, stop := keys[0], keys[1], keys[2]
				play.AccessibleAction(0, a11y.ActionDefault)
				play.AccessibleAction(0, a11y.ActionDefault)
				if p.Transport.State != players.Playing {
					t.Fatal("pressing Play while playing paused the transport")
				}
				pause.AccessibleAction(0, a11y.ActionDefault)
				if p.Transport.State != players.Paused {
					t.Fatal("Pause did not pause")
				}
				play.AccessibleAction(0, a11y.ActionDefault)
				if p.Transport.State != players.Playing {
					t.Fatal("Play did not resume")
				}
				stop.AccessibleAction(0, a11y.ActionDefault)
				pause.AccessibleAction(0, a11y.ActionDefault)
				if p.Transport.State != players.Stopped {
					t.Fatal("Pause started a stopped transport")
				}
			}
		})
	}
}

func TestSatelliteVisibilitySurvivesReopeningThePresentation(t *testing.T) {
	a, p := open(t, Skin, 1)
	p.Eq().Inject(platform.Event{Kind: platform.EventClose})
	p.List().Inject(platform.Event{Kind: platform.EventClose})
	a.PumpOnce()
	if p.Host.Model.EqualizerVisible || p.Host.Model.PlaylistVisible {
		t.Fatal("closing satellites did not update the shared visibility settings")
	}
	if p.Eq().Closed() || p.List().Closed() {
		t.Fatal("closing a satellite destroyed its window instead of hiding it")
	}
	p.strip.eqBtn.AccessibleAction(0, a11y.ActionDefault)
	if !p.Host.Model.EqualizerVisible || !p.g.Rack.Pane(p.iEq).Shown {
		t.Fatal("the equaliser toggle did not reopen its window")
	}
	p.Close()
	next, err := New(p.Host, Options{Headless: true})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(next.Close)
	a.PumpOnce()
	if !next.g.Rack.Pane(next.iEq).Shown || next.g.Rack.Pane(next.iList).Shown {
		t.Fatal("reopening the presentation lost satellite visibility")
	}
}

func TestAutoKeepsManualAdjustmentsWhenTheSkinChanges(t *testing.T) {
	a, p := open(t, Skin, 1)
	p.Transport.SelectTrack(1)
	p.eqPane.auto.AccessibleAction(0, a11y.ActionDefault)
	p.Equaliser.Set(0, 2.5)
	p.SetSkin(SkinSilver)
	a.PumpOnce()
	if !p.Equaliser.Auto || p.Equaliser.Gains[0] != 2.5 {
		t.Fatal("the skin change reapplied Auto and lost the manual adjustment")
	}
	if !p.eqPane.auto.Checked {
		t.Fatal("the new skin lost the Auto button state")
	}
}

// desktopMove moves w the way a window manager would, and lets the group
// follow the desktop's word.
func desktopMove(a *ui.Application, w *ui.Window, x, y int) {
	w.Surface().(*platform.Offscreen).SimulateDesktopMove(x, y)
	playertest.Follow(a)
}
