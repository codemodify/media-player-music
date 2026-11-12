package panel

import (
	"reflect"
	"testing"
	"time"

	"github.com/codemodify/media-player-music/internal/players"
	"github.com/codemodify/media-player-music/internal/players/playertest"
	"goframework.io/desktop-ui-rack"
	"goframework.io/desktop-ui/platform"
)

func TestDesktopMovesFollowWithoutThePlaybackPulse(t *testing.T) {
	for _, skin := range []string{Skin, SkinSilver} {
		for _, state := range []players.State{players.Stopped, players.Paused, players.Playing} {
			t.Run(skin+"/"+state.String(), func(t *testing.T) {
				a, p := open(t, skin, 1.75)
				p.Advance(0)
				p.Transport.State, p.Transport.Pos = state, 13*time.Second
				before := p.g.Rack.Boxes()
				o := p.Main().Surface().(*platform.Offscreen)
				// A configure burst must use its final position, without any
				// explicit Follow or application playback tick.
				for i := 0; i < 80; i++ {
					o.SimulateDesktopMove(400+i, 260+i)
				}
				playertest.Follow(a)
				dockingAssertTranslated(t, p, before, 479-before[p.iMain].X, 339-before[p.iMain].Y)
				if p.Transport.State != state || p.Transport.Pos != 13*time.Second {
					t.Fatal("following a window changed the playback state or clock")
				}
			})
		}
	}
}

func TestEventDrivenMainRejoinPreservesAComponent(t *testing.T) {
	for _, skin := range []string{Skin, SkinSilver} {
		t.Run(skin, func(t *testing.T) {
			a, p := open(t, skin, 1.75)
			p.Advance(0)
			p.List().Surface().(*platform.Offscreen).SimulateDesktopMove(1200, 800)
			playertest.Follow(a)
			list := p.g.Rack.Pane(p.iList).Box
			p.Eq().Surface().(*platform.Offscreen).SimulateDesktopMove(list.X-37, list.Bottom()+3)
			playertest.Follow(a)
			if p.g.Rack.Pane(p.iEq).To != p.iList {
				t.Fatal("the loose equaliser did not join the playlist")
			}
			before := p.g.Rack.Boxes()
			eq := p.g.Rack.Pane(p.iEq).Box
			wantMain := (rack.Bond{Side: rack.SideRight, Along: 23}).Place(eq, StripW, StripH)
			p.Main().Surface().(*platform.Offscreen).SimulateDesktopMove(wantMain.X+2, wantMain.Y)
			playertest.Follow(a)
			if got := p.g.Rack.Pane(p.iMain); got.To != -1 || got.Box != wantMain {
				t.Fatalf("Main did not snap as the group anchor: %+v", got)
			}
			if p.g.Rack.Pane(p.iEq).To != p.iMain || p.g.Rack.Pane(p.iList).To != p.iEq {
				t.Fatal("joining through the non-root equaliser lost the playlist path")
			}
			for i := 1; i < len(before); i++ {
				if got := p.g.Rack.Pane(i).Box; got != before[i] {
					t.Fatalf("joining Main moved stationary pane %d to %+v from %+v", i, got, before[i])
				}
			}
			dockingAssertPositions(t, p)
			joined := p.g.Rack.Boxes()
			p.Main().Surface().(*platform.Offscreen).SimulateDesktopMove(420, 250)
			playertest.Follow(a)
			dockingAssertTranslated(t, p, joined, 420-wantMain.X, 250-wantMain.Y)
			fixed := p.g.Rack.Boxes()
			p.List().Surface().(*platform.Offscreen).SimulateDesktopMove(1800, 900)
			playertest.Follow(a)
			if p.g.Rack.Pane(p.iList).To >= 0 || !reflect.DeepEqual(p.g.Rack.Boxes()[:2], fixed[:2]) {
				t.Fatal("detaching the playlist moved its former anchors")
			}
		})
	}
}

func TestMainRejoinDoesNotOverwriteACoalescedForeignMove(t *testing.T) {
	for _, skin := range []string{Skin, SkinSilver} {
		t.Run(skin, func(t *testing.T) {
			a, p := open(t, skin, 1.75)
			p.Advance(0)
			playlist := p.List().Surface().(*platform.Offscreen)
			playlist.SimulateDesktopMove(1200, 800)
			playertest.Follow(a)
			old := p.g.Rack.Pane(p.iList).Box
			// Both native positions update before this event burst is
			// dispatched. Main approaches the previous playlist position.
			p.Main().Surface().(*platform.Offscreen).SimulateDesktopMove(old.Right()+2, old.Y+23)
			playlist.SimulateDesktopMove(1600, 900)
			playertest.Follow(a)
			got := p.g.Rack.Pane(p.iList)
			if got.Box.X != 1600 || got.Box.Y != 900 || got.To != -1 || p.g.Rack.Pane(p.iMain).To != -1 {
				t.Fatalf("rejoining replayed stale foreign geometry: %+v", got)
			}
			dockingAssertPositions(t, p)
		})
	}
}
