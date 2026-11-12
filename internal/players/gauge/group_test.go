package gauge

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/codemodify/media-player-music/internal/players"
	"github.com/codemodify/media-player-music/internal/players/playertest"
	"goframework.io/desktop-ui"
	"goframework.io/desktop-ui-rack"
	"goframework.io/desktop-ui/platform"
)

// groupFixture is the face with its panels settled where the group put them.
// It owns no playback engine and never touches the desktop.
func groupFixture(t *testing.T, scale float32) (*ui.Application, *Player) {
	t.Helper()
	a, p := open(t, scale)
	playertest.SettleGroup(t, a, p.g)
	return a, p
}

func assertGroupPositions(t *testing.T, p *Player) {
	t.Helper()
	for i := 0; i < p.g.Rack.Len(); i++ {
		pane, w := p.g.Rack.Pane(i), p.g.Window(i)
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
			main, eq, list := p.g.Rack.Pane(p.iMain), p.g.Rack.Pane(p.iEq), p.g.Rack.Pane(p.iList)
			if eq.To != p.iMain || eq.Bond != (rack.Bond{Side: rack.SideRight, Along: -80}) ||
				eq.Box.X != main.Box.Right() || eq.Box.Y != main.Box.Y-80 ||
				list.To != p.iEq || list.Box.X != eq.Box.X || list.Box.Y != eq.Box.Bottom() {
				t.Fatalf("original main/EQ/playlist arrangement changed: %+v", p.g.Rack.Boxes())
			}
			p.Transport.State, p.Transport.Pos = players.Paused, 17*time.Second
			before := p.g.Rack.Boxes()
			for step := 0; step < 40; step++ {
				p.Main().Surface().(*platform.Offscreen).SimulateDesktopMove(420+step, 250+step)
			}
			playertest.Follow(a)
			for i, old := range before {
				if got := p.g.Rack.Pane(i).Box; got != old.Moved(459-before[p.iMain].X, 289-before[p.iMain].Y) {
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
	p.Main().Surface().(*platform.Offscreen).SimulateDesktopMove(420, 250)
	playertest.Follow(a)
	p.List().Surface().(*platform.Offscreen).SimulateDesktopMove(1500, 900)
	playertest.Follow(a)
	p.List().SetSize(620, 420)
	playertest.Follow(a)
	p.ShowEqualiser(false)
	before := p.g.Rack.Boxes()
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
	a, reopened := open(t, 1.75)
	reopened.ShowEqualiser(false)
	reopened.RestorePlacement(saved)
	playertest.SettleGroup(t, a, reopened.g)
	if got := reopened.g.Rack.Boxes(); !reflect.DeepEqual(got, before) {
		t.Fatalf("restore replaced saved arrangement: got %+v want %+v", got, before)
	}
	if reopened.Eq().Visible() || reopened.g.Rack.Pane(reopened.iEq).Shown {
		t.Fatal("placement restore showed the hidden EQ")
	}
	reopened.Main().Surface().(*platform.Offscreen).SimulateDesktopMove(490, 295)
	playertest.Follow(a)
	if reopened.g.Rack.Pane(reopened.iEq).Box != before[p.iEq].Moved(70, 45) || reopened.g.Rack.Pane(reopened.iList).Box != before[p.iList] {
		t.Fatal("restored Main carried a detached playlist or lost its hidden EQ")
	}
	reopened.ShowEqualiser(true)
	assertGroupPositions(t, reopened)
}

func TestMainRejoiningAStationaryForeignPathPreservesGeometry(t *testing.T) {
	a, p := groupFixture(t, 1.75)
	p.List().Surface().(*platform.Offscreen).SimulateDesktopMove(1400, 800)
	playertest.Follow(a)
	list := p.g.Rack.Pane(p.iList).Box
	p.Eq().Surface().(*platform.Offscreen).SimulateDesktopMove(list.X-37, list.Bottom()+3)
	playertest.Follow(a)
	if p.g.Rack.Pane(p.iEq).To != p.iList {
		t.Fatal("loose EQ did not join the playlist")
	}
	before := p.g.Rack.Boxes()
	want := (rack.Bond{Side: rack.SideRight, Along: 23}).Place(before[p.iEq], MainW, MainH)
	p.Main().Surface().(*platform.Offscreen).SimulateDesktopMove(want.X+2, want.Y)
	playertest.Follow(a)
	if p.g.Rack.Pane(p.iMain).Box != want || p.g.Rack.Pane(p.iEq).To != p.iMain || p.g.Rack.Pane(p.iList).To != p.iEq {
		t.Fatalf("Main rejoin dropped the foreign path: %+v", p.g.Rack.Boxes())
	}
	if p.g.Rack.Pane(p.iEq).Box != before[p.iEq] || p.g.Rack.Pane(p.iList).Box != before[p.iList] {
		t.Fatal("Main rejoin moved stationary panels")
	}
	assertGroupPositions(t, p)
}

func TestMalformedSavedGroupCannotCreateACycleOrResizeFixedWindows(t *testing.T) {
	a, p := open(t, 1)
	p.RestorePlacement(map[string]players.WindowPlacement{
		"main":      {Positioned: true, X: 0, Y: 0, Width: 900, Height: 700, Dock: &players.WindowDock{Parent: "playlist", Side: "left"}},
		"equaliser": {Positioned: true, Width: 900, Height: 700, Dock: &players.WindowDock{Parent: "playlist", Side: "bottom"}},
		"playlist":  {Positioned: true, X: -600, Y: 800, Width: 620, Height: 420, Dock: &players.WindowDock{Parent: "equaliser", Side: "right"}},
	})
	playertest.SettleGroup(t, a, p.g)
	for i := 0; i < p.g.Rack.Len(); i++ {
		seen := make(map[int]bool)
		for at := i; at >= 0; at = p.g.Rack.Pane(at).To {
			if seen[at] {
				t.Fatal("malformed session created a docking cycle")
			}
			seen[at] = true
		}
	}
	if p.g.Rack.Pane(p.iMain).To != -1 {
		t.Fatal("saved satellite became Main's parent")
	}
	if w, h := p.Main().Size(); w != MainW || h != MainH {
		t.Fatalf("fixed Main resized to %dx%d", w, h)
	}
	if w, h := p.Eq().Size(); w != EqW || h != EqH {
		t.Fatalf("fixed EQ resized to %dx%d", w, h)
	}
	if x, y, known := p.Main().Position(); !known || x != 0 || y != 0 {
		t.Fatal("known origin was discarded")
	}
	assertGroupPositions(t, p)
}
