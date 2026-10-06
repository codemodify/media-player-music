package halflife2

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/codemodify/media-player-music/internal/players"
	"github.com/codemodify/uitoolkit"
	"github.com/codemodify/uitoolkit/platform"
	"github.com/codemodify/uitoolkit/rack"
	"github.com/codemodify/uitoolkit/style"
)

// The real desk and native event dispatcher can be exercised before a skin is
// painted. This fixture owns no playback engine and never touches the desktop.
func groupFixture(t *testing.T, scale float32) (*uitoolkit.Application, *Player) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv(style.ThemeEnv, "dark")
	a := uitoolkit.New(uitoolkit.Options{Headless: true, Scale: scale, DisableLookWatch: true})
	h := players.NewHost(a)
	p := &Player{App: a, Host: h, Transport: h.Model.Transport, Spectrum: h.Model.Spectrum, Equaliser: h.Model.Equaliser}
	var err error
	p.Main, err = players.OpenSized(a, platform.WindowOptions{Headless: true, Decorations: platform.DecorationsNone}, MainW, MainH, true)
	if err != nil {
		t.Fatal(err)
	}
	p.Eq, err = players.OpenSized(a, platform.WindowOptions{Headless: true, Decorations: platform.DecorationsNone, Role: platform.RoleUtility, Owner: p.Main.Surface()}, EqW, EqH, true)
	if err != nil {
		p.Main.Close()
		t.Fatal(err)
	}
	p.List, err = players.OpenSized(a, platform.WindowOptions{Headless: true, Decorations: platform.DecorationsNone, Role: platform.RoleUtility, Owner: p.Main.Surface(), MinWidth: MinListW, MinHeight: MinListH}, ListW, ListH, false)
	if err != nil {
		p.Eq.Close()
		p.Main.Close()
		t.Fatal(err)
	}
	t.Cleanup(p.Close)
	p.initGroup()
	a.PumpOnce()
	p.AdvanceGeometry(0)
	return a, p
}

func assertGroupPositions(t *testing.T, p *Player) {
	t.Helper()
	for i := 0; i < p.Desk.Rack.Len(); i++ {
		pane, w := p.Desk.Rack.Pane(i), p.Desk.Window(i)
		if !pane.Shown {
			continue
		}
		x, y, known := w.Position()
		if !known || x != pane.Box.X || y != pane.Box.Y {
			t.Fatalf("%s native position %d,%d known=%t differs from rack %+v", pane.Name, x, y, known, pane.Box)
		}
	}
}

func TestSourceArrangementAndDesktopMoveFollowAtEveryScale(t *testing.T) {
	for _, scale := range []float32{1, 1.75, 2} {
		t.Run(fmt.Sprintf("%gx", scale), func(t *testing.T) {
			a, p := groupFixture(t, scale)
			main, eq, list := p.Desk.Rack.Pane(p.iMain), p.Desk.Rack.Pane(p.iEq), p.Desk.Rack.Pane(p.iList)
			if eq.To != p.iMain || eq.Bond != (rack.Bond{Side: rack.SideRight, Along: -80}) ||
				eq.Box.X != main.Box.Right() || eq.Box.Y != main.Box.Y-80 ||
				list.To != p.iEq || list.Box.X != eq.Box.X || list.Box.Y != eq.Box.Bottom() {
				t.Fatalf("original main/EQ/playlist arrangement changed: %+v", p.Desk.Rack.Boxes())
			}
			p.Transport.State, p.Transport.Pos = players.Paused, 17*time.Second
			before := p.Desk.Rack.Boxes()
			for step := 0; step < 40; step++ {
				p.Main.Surface().(*platform.Offscreen).SimulateDesktopMove(420+step, 250+step)
			}
			a.PumpOnce()
			for i, old := range before {
				if got := p.Desk.Rack.Pane(i).Box; got != old.Moved(459-before[p.iMain].X, 289-before[p.iMain].Y) {
					t.Fatalf("pane %d did not follow the final native move: %+v", i, got)
				}
			}
			if p.Transport.State != players.Paused || p.Transport.Pos != 17*time.Second {
				t.Fatal("window movement changed playback")
			}
			assertGroupPositions(t, p)
		})
	}
}

func TestDetachedPlaylistResizeAndHiddenDockRestore(t *testing.T) {
	a, p := groupFixture(t, 1.75)
	p.Main.Surface().(*platform.Offscreen).SimulateDesktopMove(420, 250)
	a.PumpOnce()
	p.List.Surface().(*platform.Offscreen).SimulateDesktopMove(1500, 900)
	a.PumpOnce()
	p.List.SetSize(620, 420)
	a.PumpOnce()
	p.AdvanceGeometry(70 * time.Millisecond)
	p.ShowEqualiser(false)
	before := p.Desk.Rack.Boxes()
	saved := p.CapturePlacement()
	if saved["playlist"].Dock != nil || saved["playlist"].Width != 620 || saved["playlist"].Height != 420 {
		t.Fatalf("detached resized playlist not preserved: %+v", saved["playlist"])
	}
	if saved["equaliser"].Dock == nil || saved["equaliser"].Dock.Parent != "main" || saved["equaliser"].Dock.Along != -80 {
		t.Fatal("hidden EQ lost its source bond")
	}
	// Hiding belongs to the shared model; placement should preserve it while
	// applying the saved desk before the first post-restore pump.
	p.Close()
	a, reopened := groupFixture(t, 1.75)
	reopened.ShowEqualiser(false)
	reopened.RestorePlacement(saved)
	a.PumpOnce()
	reopened.AdvanceGeometry(0)
	if got := reopened.Desk.Rack.Boxes(); !reflect.DeepEqual(got, before) {
		t.Fatalf("restore replaced saved arrangement: got %+v want %+v", got, before)
	}
	if reopened.Eq.Visible() || reopened.Desk.Rack.Pane(reopened.iEq).Shown {
		t.Fatal("placement restore showed the hidden EQ")
	}
	reopened.Main.Surface().(*platform.Offscreen).SimulateDesktopMove(490, 295)
	a.PumpOnce()
	if reopened.Desk.Rack.Pane(reopened.iEq).Box != before[p.iEq].Moved(70, 45) || reopened.Desk.Rack.Pane(reopened.iList).Box != before[p.iList] {
		t.Fatal("restored Main carried a detached playlist or lost its hidden EQ")
	}
	reopened.ShowEqualiser(true)
	assertGroupPositions(t, reopened)
}

func TestMainRejoiningAStationaryForeignPathPreservesGeometry(t *testing.T) {
	a, p := groupFixture(t, 1.75)
	p.List.Surface().(*platform.Offscreen).SimulateDesktopMove(1400, 800)
	a.PumpOnce()
	list := p.Desk.Rack.Pane(p.iList).Box
	p.Eq.Surface().(*platform.Offscreen).SimulateDesktopMove(list.X-37, list.Bottom()+3)
	a.PumpOnce()
	if p.Desk.Rack.Pane(p.iEq).To != p.iList {
		t.Fatal("loose EQ did not join the playlist")
	}
	before := p.Desk.Rack.Boxes()
	want := (rack.Bond{Side: rack.SideRight, Along: 23}).Place(before[p.iEq], MainW, MainH)
	p.Main.Surface().(*platform.Offscreen).SimulateDesktopMove(want.X+2, want.Y)
	a.PumpOnce()
	if p.Desk.Rack.Pane(p.iMain).Box != want || p.Desk.Rack.Pane(p.iEq).To != p.iMain || p.Desk.Rack.Pane(p.iList).To != p.iEq {
		t.Fatalf("Main rejoin dropped the foreign path: %+v", p.Desk.Rack.Boxes())
	}
	if p.Desk.Rack.Pane(p.iEq).Box != before[p.iEq] || p.Desk.Rack.Pane(p.iList).Box != before[p.iList] {
		t.Fatal("Main rejoin moved stationary panels")
	}
	assertGroupPositions(t, p)
}

func TestMalformedSavedGroupCannotCreateACycleOrResizeFixedWindows(t *testing.T) {
	_, p := groupFixture(t, 1)
	p.RestorePlacement(map[string]players.WindowPlacement{
		"main":      {Positioned: true, X: 0, Y: 0, Width: 900, Height: 700, Dock: &players.WindowDock{Parent: "playlist", Side: "left"}},
		"equaliser": {Positioned: true, Width: 900, Height: 700, Dock: &players.WindowDock{Parent: "playlist", Side: "bottom"}},
		"playlist":  {Positioned: true, X: -600, Y: 800, Width: 620, Height: 420, Dock: &players.WindowDock{Parent: "equaliser", Side: "right"}},
	})
	p.AdvanceGeometry(0)
	for i := 0; i < p.Desk.Rack.Len(); i++ {
		seen := make(map[int]bool)
		for at := i; at >= 0; at = p.Desk.Rack.Pane(at).To {
			if seen[at] {
				t.Fatal("malformed session created a docking cycle")
			}
			seen[at] = true
		}
	}
	if p.Desk.Rack.Pane(p.iMain).To != -1 {
		t.Fatal("saved satellite became Main's parent")
	}
	if w, h := p.Main.Size(); w != MainW || h != MainH {
		t.Fatalf("fixed Main resized to %dx%d", w, h)
	}
	if w, h := p.Eq.Size(); w != EqW || h != EqH {
		t.Fatalf("fixed EQ resized to %dx%d", w, h)
	}
	if x, y, known := p.Main.Position(); !known || x != 0 || y != 0 {
		t.Fatal("known origin was discarded")
	}
	assertGroupPositions(t, p)
}
