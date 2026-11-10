package fandom

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

func TestDesktopMoveCarriesOnlyTheSnappedPlaylist(t *testing.T) {
	for _, scale := range []float32{1, 1.75, 2} {
		t.Run(fmt.Sprintf("%gx", scale), func(t *testing.T) {
			a, p := groupFixture(t, scale)
			p.Transport.State, p.Transport.Pos = players.Paused, 17*time.Second
			main, list := p.g.Rack.Pane(p.iMain), p.g.Rack.Pane(p.iList)
			if list.To != p.iMain || list.Bond != (rack.Bond{Side: rack.SideRight, Along: 46}) || list.Box.X != main.Box.Right() || list.Box.Y != main.Box.Y+46 {
				t.Fatal("reference side-by-side arrangement changed")
			}
			before := p.g.Rack.Boxes()
			for i := 0; i < 40; i++ {
				p.Main().Surface().(*platform.Offscreen).SimulateDesktopMove(420+i, 250+i)
			}
			playertest.Follow(a)
			for i, old := range before {
				if got := p.g.Rack.Pane(i).Box; got != old.Moved(459-before[p.iMain].X, 289-before[p.iMain].Y) {
					t.Fatalf("pane%d did not follow final event: %+v", i, got)
				}
			}
			p.List().Surface().(*platform.Offscreen).SimulateDesktopMove(1500, 900)
			playertest.Follow(a)
			if p.g.Rack.Pane(p.iList).To != -1 {
				t.Fatal("dragged playlist stayed attached")
			}
			detached := p.g.Rack.Pane(p.iList).Box
			p.Main().Surface().(*platform.Offscreen).SimulateDesktopMove(510, 330)
			playertest.Follow(a)
			if p.g.Rack.Pane(p.iList).Box != detached {
				t.Fatal("detached playlist followed Main")
			}
			if p.Transport.State != players.Paused || p.Transport.Pos != 17*time.Second {
				t.Fatal("window motion changed playback")
			}
			assertGroupPositions(t, p)
		})
	}
}

func TestMainRejoinKeepsAStationaryPlaylistInPlace(t *testing.T) {
	a, p := groupFixture(t, 1.75)
	p.List().Surface().(*platform.Offscreen).SimulateDesktopMove(1400, 800)
	playertest.Follow(a)
	list := p.g.Rack.Pane(p.iList).Box
	want := (rack.Bond{Side: rack.SideRight, Along: 23}).Place(list, MainW, MainH)
	p.Main().Surface().(*platform.Offscreen).SimulateDesktopMove(want.X+2, want.Y)
	playertest.Follow(a)
	if p.g.Rack.Pane(p.iMain).Box != want || p.g.Rack.Pane(p.iList).To != p.iMain || p.g.Rack.Pane(p.iList).Box != list {
		t.Fatal("Main rejoin shifted stationary playlist geometry")
	}
	assertGroupPositions(t, p)
}

func TestPlacementRestoresResizedHiddenPlaylistWithoutResizingMain(t *testing.T) {
	a, p := groupFixture(t, 1.75)
	p.Main().Surface().(*platform.Offscreen).SimulateDesktopMove(420, 250)
	playertest.Follow(a)
	p.List().SetSize(330, 390)
	playertest.Follow(a)
	p.ShowPlaylist(false)
	before := p.g.Rack.Boxes()
	saved := p.CapturePlacement()
	main := saved["main"]
	main.Width, main.Height = 900, 700
	saved["main"] = main
	p.Close()
	a, reopened := open(t, 1.75)
	reopened.ShowPlaylist(false)
	reopened.RestorePlacement(saved)
	playertest.SettleGroup(t, a, reopened.g)
	if got := reopened.g.Rack.Boxes(); !reflect.DeepEqual(got, before) {
		t.Fatalf("restore replaced original group: got%+v want%+v", got, before)
	}
	if reopened.List().Visible() || reopened.Host.Model.PlaylistVisible {
		t.Fatal("restore showed hidden playlist")
	}
	if w, h := reopened.Main().Size(); w != MainW || h != MainH {
		t.Fatal("saved dimensions resized fixed Main")
	}
	reopened.Main().Surface().(*platform.Offscreen).SimulateDesktopMove(490, 295)
	playertest.Follow(a)
	if reopened.g.Rack.Pane(reopened.iList).Box != before[p.iList].Moved(70, 45) {
		t.Fatal("hidden playlist lost its restored bond")
	}
	reopened.ShowPlaylist(true)
	assertGroupPositions(t, reopened)
}
