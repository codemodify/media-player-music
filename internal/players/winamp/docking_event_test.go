package winamp

import (
	"reflect"
	"testing"
	"time"

	"github.com/codemodify/media-player-music/internal/players"
	"github.com/codemodify/uitoolkit/platform"
	"github.com/codemodify/uitoolkit/rack"
)

func TestDesktopMovesFollowWithoutThePlaybackPulse(t *testing.T) {
	for _, skin := range []string{Skin, SkinSilver} {
		for _, state := range []players.State{players.Stopped, players.Paused, players.Playing} {
			t.Run(skin+"/"+state.String(), func(t *testing.T) {
				a, p := open(t, skin, 1.75)
				p.Advance(0)
				p.Transport.State, p.Transport.Pos = state, 13*time.Second
				before := p.Desk.Rack.Boxes()
				o := p.Main.Surface().(*platform.Offscreen)
				// A configure burst must use its final position, without any
				// explicit Follow or application playback tick.
				for i := 0; i < 80; i++ {
					o.SimulateDesktopMove(400+i, 260+i)
				}
				a.PumpOnce()
				dockingAssertTranslated(t, p, before, 479-before[p.iMain].X, 339-before[p.iMain].Y)
				if p.Transport.State != state || p.Transport.Pos != 13*time.Second {
					t.Fatal("following a window changed the playback state or clock")
				}
			})
		}
	}
}

func TestEventDrivenMainRejoinPreservesAComponentAndHiddenBranches(t *testing.T) {
	for _, skin := range []string{Skin, SkinSilver} {
		t.Run(skin, func(t *testing.T) {
			a, p := open(t, skin, 1.75)
			p.Advance(0)
			p.List.Surface().(*platform.Offscreen).SimulateDesktopMove(1200, 800)
			a.PumpOnce()
			list := p.Desk.Rack.Pane(p.iList).Box
			p.Eq.Surface().(*platform.Offscreen).SimulateDesktopMove(list.X-37, list.Bottom()+3)
			a.PumpOnce()
			if p.Desk.Rack.Pane(p.iEq).To != p.iList {
				t.Fatal("the loose equaliser did not join the playlist")
			}
			branch := dockingAddWindow(t, p, "branch", 93, 58)
			p.Desk.Attach(branch, p.iEq, rack.SideLeft)
			hidden := dockingAddWindow(t, p, "hidden", 67, 41)
			p.Desk.Attach(hidden, branch, rack.SideBottom)
			p.Desk.Show(hidden, false)
			before := p.Desk.Rack.Boxes()
			branchBefore, hiddenBefore := *p.Desk.Rack.Pane(branch), *p.Desk.Rack.Pane(hidden)
			eq := p.Desk.Rack.Pane(p.iEq).Box
			wantMain := (rack.Bond{Side: rack.SideRight, Along: 23}).Place(eq, StripW, StripH)
			p.Main.Surface().(*platform.Offscreen).SimulateDesktopMove(wantMain.X+2, wantMain.Y)
			a.PumpOnce()
			if got := p.Desk.Rack.Pane(p.iMain); got.To != -1 || got.Box != wantMain {
				t.Fatalf("Main did not snap as the group anchor: %+v", got)
			}
			if p.Desk.Rack.Pane(p.iEq).To != p.iMain || p.Desk.Rack.Pane(p.iList).To != p.iEq {
				t.Fatal("joining through the non-root equaliser lost the playlist path")
			}
			for i := 1; i < len(before); i++ {
				if got := p.Desk.Rack.Pane(i).Box; got != before[i] {
					t.Fatalf("joining Main moved stationary pane %d to %+v from %+v", i, got, before[i])
				}
			}
			if *p.Desk.Rack.Pane(branch) != branchBefore || *p.Desk.Rack.Pane(hidden) != hiddenBefore {
				t.Fatal("joining changed a sibling or hidden branch's bond")
			}
			dockingAssertPositions(t, p)
			joined := p.Desk.Rack.Boxes()
			p.Main.Surface().(*platform.Offscreen).SimulateDesktopMove(420, 250)
			a.PumpOnce()
			dockingAssertTranslated(t, p, joined, 420-wantMain.X, 250-wantMain.Y)
			if p.Desk.Window(hidden).Visible() {
				t.Fatal("moving the group exposed a hidden branch")
			}
			fixed := p.Desk.Rack.Boxes()
			p.List.Surface().(*platform.Offscreen).SimulateDesktopMove(1800, 900)
			a.PumpOnce()
			if p.Desk.Rack.Pane(p.iList).To >= 0 || !reflect.DeepEqual(p.Desk.Rack.Boxes()[:2], fixed[:2]) {
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
			playlist := p.List.Surface().(*platform.Offscreen)
			playlist.SimulateDesktopMove(1200, 800)
			a.PumpOnce()
			old := p.Desk.Rack.Pane(p.iList).Box
			// Both native positions update before this event burst is
			// dispatched. Main approaches the previous playlist position.
			p.Main.Surface().(*platform.Offscreen).SimulateDesktopMove(old.Right()+2, old.Y+23)
			playlist.SimulateDesktopMove(1600, 900)
			a.PumpOnce()
			got := p.Desk.Rack.Pane(p.iList)
			if got.Box.X != 1600 || got.Box.Y != 900 || got.To != -1 || p.Desk.Rack.Pane(p.iMain).To != -1 {
				t.Fatalf("rejoining replayed stale foreign geometry: %+v", got)
			}
			dockingAssertPositions(t, p)
		})
	}
}

func TestEventDrivenMainRejoinPreservesThreePaneForeignPath(t *testing.T) {
	a, p := open(t, Skin, 1.75)
	p.Advance(0)
	dockingMove(t, p, p.iList, 1000, 600)
	list := p.Desk.Rack.Pane(p.iList).Box
	dockingMove(t, p, p.iEq, list.X-37, list.Bottom()+3)
	branch := dockingAddWindow(t, p, "third foreign pane", 93, 58)
	p.Desk.Attach(branch, p.iEq, rack.SideBottom)
	before := p.Desk.Rack.Boxes()
	anchor := p.Desk.Rack.Pane(branch).Box
	want := (rack.Bond{Side: rack.SideRight, Along: 23}).Place(anchor, StripW, StripH)
	p.Main.Surface().(*platform.Offscreen).SimulateDesktopMove(want.X+2, want.Y)
	a.PumpOnce()
	if p.Desk.Rack.Pane(p.iMain).Box != want || p.Desk.Rack.Pane(branch).To != p.iMain ||
		p.Desk.Rack.Pane(p.iEq).To != branch || p.Desk.Rack.Pane(p.iList).To != p.iEq {
		t.Fatalf("rejoining dropped a foreign path link: %+v", p.Desk.Rack.Boxes())
	}
	for i := 1; i < len(before); i++ {
		if p.Desk.Rack.Pane(i).Box != before[i] {
			t.Fatalf("rejoining moved stationary pane %d", i)
		}
	}
	joined := p.Desk.Rack.Boxes()
	p.Main.Surface().(*platform.Offscreen).SimulateDesktopMove(420, 250)
	a.PumpOnce()
	dockingAssertTranslated(t, p, joined, 420-want.X, 250-want.Y)
}

func TestAReentrantMoveDuringFollowingPreservesTheFinalPosition(t *testing.T) {
	_, p := open(t, Skin, 1.75)
	p.Advance(0)
	before := p.Desk.Rack.Boxes()
	main := before[p.iMain]
	calls := 0
	p.Desk.Moved = func() {
		calls++
		if calls == 1 {
			p.Main.Move(420, 250)
			p.followWindows()
		}
	}
	p.Main.Move(300, 200)
	p.followWindows()
	if calls != 2 {
		t.Fatalf("a reentrant final move was lost: follow callbacks=%d", calls)
	}
	dockingAssertTranslated(t, p, before, 420-main.X, 250-main.Y)
}
