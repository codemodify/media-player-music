package standard

import (
	"fmt"
	"testing"
	"time"

	"github.com/codemodify/media-player-music/internal/players"
	"goframework.io/desktop-ui/a11y"
	"goframework.io/desktop-ui/platform"
	"goframework.io/desktop-ui/widget"
	"goframework.io/graphics-paint2d"
)

func TestPlaylistPointerSelectsBeforeDoubleClickActivates(t *testing.T) {
	for _, scale := range []float32{1, 1.75} {
		for _, state := range []players.State{players.Stopped, players.Paused, players.Playing} {
			t.Run(fmt.Sprintf("%gx/%s", scale, state), func(t *testing.T) {
				a, p := open(t, scale)
				list := p.body.queue.list
				p.Transport.State, p.Transport.Pos = state, 17*time.Second
				p.Refresh()
				plays := 0
				p.Host.Run = func(c players.Command) {
					if c == players.CmdPlay {
						plays++
					}
					p.Transport.Do(c)
				}
				injectClick := func(row int) {
					pos := list.AccessibleItems()[row].Bounds.Center()
					p.Main.Inject(platform.Event{Kind: platform.EventMouseMove, Pos: pos})
					p.Main.Inject(platform.Event{Kind: platform.EventMouseDown, Button: platform.ButtonLeft, Pos: pos})
					p.Main.Inject(platform.Event{Kind: platform.EventMouseUp, Button: platform.ButtonLeft, Pos: pos})
				}
				click := func() { injectClick(2); a.PumpOnce() }
				click()
				p.Refresh()
				a.PumpOnce()
				if list.Selected != 2 || p.Transport.List.Index() != 0 || p.Transport.State != state || p.Transport.Pos != 17*time.Second || plays != 0 {
					t.Fatalf("single click: selected=%d playing=%d state=%s offset=%s play calls=%d", list.Selected, p.Transport.List.Index(), p.Transport.State, p.Transport.Pos, plays)
				}
				// Start a fresh gesture and queue its two clicks before painting.
				injectClick(0)
				injectClick(2)
				injectClick(2)
				a.PumpOnce()
				if p.Transport.List.Index() != 2 || p.Transport.State != players.Playing || plays != 1 {
					t.Fatalf("double click: playing=%d state=%s play calls=%d", p.Transport.List.Index(), p.Transport.State, plays)
				}
				p.Transport.State, p.Transport.Pos = players.Paused, 5*time.Second
				click()
				if plays != 1 || p.Transport.State != players.Paused || p.Transport.Pos != 5*time.Second {
					t.Fatal("a third click resumed playback instead of beginning a new selection gesture")
				}
			})
		}
	}
}

func TestPlaylistPointerInterruptionsDoNotActivate(t *testing.T) {
	for _, interruption := range []string{"pointer-exit", "focus-change", "wheel-at-end"} {
		t.Run(interruption, func(t *testing.T) {
			a, p := open(t, 1)
			list := p.body.queue.list
			plays := 0
			p.Host.Run = func(c players.Command) {
				if c == players.CmdPlay {
					plays++
				}
				p.Transport.Do(c)
			}
			row := 2
			if interruption == "wheel-at-end" {
				list.ScrollTo(list.MaxOffset())
				row = list.Count - 1
			}
			pos := list.AccessibleItems()[row].Bounds.Center()
			inject := func() {
				p.Main.Inject(platform.Event{Kind: platform.EventMouseMove, Pos: pos})
				p.Main.Inject(platform.Event{Kind: platform.EventMouseDown, Button: platform.ButtonLeft, Pos: pos})
				p.Main.Inject(platform.Event{Kind: platform.EventMouseUp, Button: platform.ButtonLeft, Pos: pos})
			}
			inject()
			switch interruption {
			case "pointer-exit":
				p.Main.Inject(platform.Event{Kind: platform.EventMouseMove, Pos: widget.DeviceBounds(p.body.queue.add).Center()})
			case "focus-change":
				p.Main.Inject(platform.Event{Kind: platform.EventKeyDown, Key: platform.KeyTab})
			case "wheel-at-end":
				p.Main.Inject(platform.Event{Kind: platform.EventScroll, Pos: pos, Scroll: paint2d.Pt(0, 1)})
			}
			inject()
			a.PumpOnce()
			if list.Selected != row || p.Transport.List.Index() != 0 || plays != 0 {
				t.Fatalf("interrupted gesture activated playback: selected=%d playing=%d plays=%d", list.Selected, p.Transport.List.Index(), plays)
			}
		})
	}
}

func TestDisabledPlaylistPointerDoesNotSelectOrOpenMenu(t *testing.T) {
	a, p := open(t, 1)
	list := p.body.queue.list
	selected := list.Selected
	list.SetEnabled(false)
	pos := list.AccessibleItems()[2].Bounds.Center()
	for _, button := range []platform.MouseButton{platform.ButtonLeft, platform.ButtonRight} {
		p.Main.Inject(platform.Event{Kind: platform.EventMouseMove, Pos: pos})
		p.Main.Inject(platform.Event{Kind: platform.EventMouseDown, Button: button, Pos: pos})
		p.Main.Inject(platform.Event{Kind: platform.EventMouseUp, Button: button, Pos: pos})
		a.PumpOnce()
		if list.Selected != selected || p.Transport.List.Index() != 0 || p.Transport.State != players.Stopped {
			t.Fatal("disabled pointer press selected or played a row")
		}
		if p.Main.Popup() != nil {
			t.Fatal("disabled pointer press opened a context menu")
		}
	}
}

func TestPlaylistSelectionAndActivationGestures(t *testing.T) {
	a, p := open(t, 1)
	list := p.body.queue.list
	plays := 0
	p.Host.Run = func(c players.Command) {
		if c == players.CmdPlay {
			plays++
		}
		p.Transport.Do(c)
	}
	click := func(button platform.MouseButton, row int) {
		pos := list.AccessibleItems()[row].Bounds.Center()
		p.Main.Inject(platform.Event{Kind: platform.EventMouseMove, Pos: pos})
		p.Main.Inject(platform.Event{Kind: platform.EventMouseDown, Button: button, Pos: pos})
		p.Main.Inject(platform.Event{Kind: platform.EventMouseUp, Button: button, Pos: pos})
		a.PumpOnce()
	}
	press := func(key platform.Key) {
		p.Main.Inject(platform.Event{Kind: platform.EventKeyDown, Key: key})
		a.PumpOnce()
	}
	click(platform.ButtonLeft, 2)
	click(platform.ButtonLeft, 3)
	if list.Selected != 3 || plays != 0 {
		t.Fatal("clicking different rows activated playback")
	}
	list.lastClickAt = time.Now().Add(-time.Second)
	click(platform.ButtonLeft, 3)
	if plays != 0 {
		t.Fatal("separate single clicks were treated as a double click")
	}
	click(platform.ButtonRight, 3)
	if p.Main.Popup() == nil || list.Selected != 3 || plays != 0 {
		t.Fatal("context click changed the selection or playback")
	}
	press(platform.KeyEscape)
	click(platform.ButtonLeft, 3)
	if plays != 0 {
		t.Fatal("context click did not reset the double-click gesture")
	}
	press(platform.KeyUp)
	if list.Selected != 2 || p.Transport.List.Index() != 0 || plays != 0 {
		t.Fatal("arrow navigation changed playback")
	}
	press(platform.KeyReturn)
	if list.Selected != 2 || p.Transport.List.Index() != 2 || p.Transport.State != players.Playing || plays != 1 {
		t.Fatal("Enter did not activate the selected track exactly once")
	}
	p.Transport.State = players.Paused
	press(platform.KeyReturn)
	if p.Transport.State != players.Playing || plays != 2 {
		t.Fatal("Enter did not resume the selected track exactly once")
	}
	if !list.AccessibleAction(4, a11y.ActionDefault) || p.Transport.List.Index() != 4 || plays != 3 {
		t.Fatal("accessible activation did not play a new selection exactly once")
	}
	p.Transport.State = players.Paused
	if !list.AccessibleAction(4, a11y.ActionDefault) || p.Transport.State != players.Playing || plays != 4 {
		t.Fatal("accessible activation did not resume the selected track exactly once")
	}
	if list.AccessibleAction(-1, a11y.ActionDefault) || list.AccessibleAction(list.Count, a11y.ActionDefault) || plays != 4 {
		t.Fatal("invalid accessible activation changed playback")
	}
	list.SetEnabled(false)
	if list.AccessibleAction(4, a11y.ActionDefault) || plays != 4 {
		t.Fatal("disabled list activated playback")
	}
}

func TestPlaylistSelectionSurvivesRefreshAndQueueChanges(t *testing.T) {
	_, p := open(t, 1)
	list := p.body.queue.list
	if list.Selected != 0 {
		t.Fatal("initial selection does not follow the current track")
	}
	list.choose(2)
	p.Transport.List.Select(1)
	p.Refresh()
	if list.Selected != 2 {
		t.Fatal("playback refresh discarded a browsed row")
	}
	list.choose(1)
	p.Transport.List.Select(3)
	p.Refresh()
	if list.Selected != 3 {
		t.Fatal("selection following playback did not follow the next track")
	}
	list.choose(2)
	p.Transport.List.Append([]players.Track{{Title: "Added"}}, false)
	p.Refresh()
	if list.Selected != 2 || list.Count != p.Transport.List.Len() {
		t.Fatal("appending tracks discarded a valid selection")
	}
	list.choose(list.Count - 1)
	p.Transport.List.Remove(list.Count-1, false)
	p.Refresh()
	if list.Selected < 0 || list.Selected >= list.Count {
		t.Fatal("removing the selected final row left an invalid selection")
	}
	queue := players.NewPlaylist(players.Library()...)
	queue.Select(1)
	p.Transport.List = queue
	p.Refresh()
	if list.Selected != 1 {
		t.Fatal("replacement queue did not initialize from its current track")
	}
	p.Transport.List = players.NewPlaylist()
	p.Refresh()
	if list.Selected != -1 || list.Count != 0 || list.AccessibleAction(0, a11y.ActionDefault) {
		t.Fatal("empty queue retains an actionable selection")
	}
}

func TestToolkitActivationResumesSelectedTrack(t *testing.T) {
	_, p := open(t, 1)
	list := p.body.queue.list
	p.Transport.State = players.Paused
	plays := 0
	p.Host.Run = func(c players.Command) {
		if c == players.CmdPlay {
			plays++
		}
		p.Transport.Do(c)
	}
	list.Activate(2)
	if list.Selected != 2 || p.Transport.List.Index() != 2 || p.Transport.State != players.Playing || plays != 1 {
		t.Fatal("public toolkit activation did not select and play exactly once")
	}
	p.Transport.State = players.Paused
	list.Activate(2)
	if p.Transport.State != players.Playing || plays != 2 {
		t.Fatal("public toolkit activation did not resume exactly once")
	}
	list.SetEnabled(false)
	list.Activate(3)
	if list.Selected != 2 || p.Transport.List.Index() != 2 || plays != 2 {
		t.Fatal("disabled player playlist accepted a programmatic playback action")
	}
}
