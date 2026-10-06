package winamp

import (
	"fmt"
	"testing"
	"time"

	"github.com/codemodify/media-player-music/internal/players"
	"github.com/codemodify/paintengine2d"
	"github.com/codemodify/uitoolkit"
	"github.com/codemodify/uitoolkit/a11y"
	"github.com/codemodify/uitoolkit/platform"
	"github.com/codemodify/uitoolkit/style"
	"github.com/codemodify/uitoolkit/widget"
)

func TestPlaylistPointerSelectsBeforeDoubleClickActivates(t *testing.T) {
	for _, skin := range Skins {
		for _, scale := range []float32{1, 1.75} {
			for _, state := range []players.State{players.Stopped, players.Paused, players.Playing} {
				t.Run(fmt.Sprintf("%s/%gx/%s", skin, scale, state), func(t *testing.T) {
					a, p := open(t, skin, scale)
					list := p.listPane.list
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
						p.List.Inject(platform.Event{Kind: platform.EventMouseMove, Pos: pos})
						p.List.Inject(platform.Event{Kind: platform.EventMouseDown, Button: platform.ButtonLeft, Pos: pos})
						p.List.Inject(platform.Event{Kind: platform.EventMouseUp, Button: platform.ButtonLeft, Pos: pos})
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
}

func TestPlaylistPointerInterruptionsDoNotActivate(t *testing.T) {
	for _, interruption := range []string{"pointer-exit", "focus-change", "wheel-at-end"} {
		t.Run(interruption, func(t *testing.T) {
			a, p := open(t, Skin, 1)
			list := p.listPane.list
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
				p.List.Inject(platform.Event{Kind: platform.EventMouseMove, Pos: pos})
				p.List.Inject(platform.Event{Kind: platform.EventMouseDown, Button: platform.ButtonLeft, Pos: pos})
				p.List.Inject(platform.Event{Kind: platform.EventMouseUp, Button: platform.ButtonLeft, Pos: pos})
			}
			inject()
			switch interruption {
			case "pointer-exit":
				p.List.Inject(platform.Event{Kind: platform.EventMouseMove, Pos: widget.DeviceBounds(p.listPane.add).Center()})
			case "focus-change":
				p.List.Inject(platform.Event{Kind: platform.EventKeyDown, Key: platform.KeyTab})
			case "wheel-at-end":
				p.List.Inject(platform.Event{Kind: platform.EventScroll, Pos: pos, Scroll: paintengine2d.Pt(0, 1)})
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
	a, p := open(t, Skin, 1)
	list := p.listPane.list
	selected := list.Selected
	list.SetEnabled(false)
	pos := list.AccessibleItems()[2].Bounds.Center()
	for _, button := range []platform.MouseButton{platform.ButtonLeft, platform.ButtonRight} {
		p.List.Inject(platform.Event{Kind: platform.EventMouseMove, Pos: pos})
		p.List.Inject(platform.Event{Kind: platform.EventMouseDown, Button: button, Pos: pos})
		p.List.Inject(platform.Event{Kind: platform.EventMouseUp, Button: button, Pos: pos})
		a.PumpOnce()
		if list.Selected != selected || p.Transport.List.Index() != 0 || p.Transport.State != players.Stopped {
			t.Fatal("disabled pointer press selected or played a row")
		}
		if p.List.Popup() != nil {
			t.Fatal("disabled pointer press opened a context menu")
		}
	}
}

func TestPlaylistSelectionAndActivationGestures(t *testing.T) {
	a, p := open(t, Skin, 1)
	list := p.listPane.list
	plays := 0
	p.Host.Run = func(c players.Command) {
		if c == players.CmdPlay {
			plays++
		}
		p.Transport.Do(c)
	}
	click := func(button platform.MouseButton, row int) {
		pos := list.AccessibleItems()[row].Bounds.Center()
		p.List.Inject(platform.Event{Kind: platform.EventMouseMove, Pos: pos})
		p.List.Inject(platform.Event{Kind: platform.EventMouseDown, Button: button, Pos: pos})
		p.List.Inject(platform.Event{Kind: platform.EventMouseUp, Button: button, Pos: pos})
		a.PumpOnce()
	}
	press := func(key platform.Key) {
		p.List.Inject(platform.Event{Kind: platform.EventKeyDown, Key: key})
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
	if p.List.Popup() == nil || list.Selected != 3 || plays != 0 {
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
	_, p := open(t, Skin, 1)
	list := p.listPane.list
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
	for _, pack := range []string{Skin, SkinSilver} {
		t.Run(pack, func(t *testing.T) {
			_, p := open(t, pack, 1.75)
			list := p.listPane.list
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
		})
	}
}

// Exercise the routed window input, including the wrapper's focus identity,
// so replacing the list implementation cannot silently steal player keys.
func TestPlaylistUsesToolkitInputAndPlayerActions(t *testing.T) {
	for _, pack := range []string{Skin, SkinSilver, Themed} {
		for _, scale := range []float32{1, 1.5} {
			t.Run(fmt.Sprintf("%s/%.1fx", pack, scale), func(t *testing.T) {
				a, p := openPlaylist(t, pack, scale)
				list := p.listPane.list
				press := func(key platform.Key) {
					p.List.Inject(platform.Event{Kind: platform.EventKeyDown, Key: key})
					a.PumpOnce()
				}
				injectClick := func(button platform.MouseButton, row int) {
					pos := list.AccessibleItems()[row].Bounds.Center()
					p.List.Inject(platform.Event{Kind: platform.EventMouseDown, Button: button, Pos: pos})
					p.List.Inject(platform.Event{Kind: platform.EventMouseUp, Button: button, Pos: pos})
				}
				click := func(button platform.MouseButton, row int) { injectClick(button, row); a.PumpOnce() }
				click(platform.ButtonLeft, 2)
				if list.Selected != 2 || p.Transport.List.Index() != 0 || p.Transport.State != players.Stopped {
					t.Fatalf("single click: selected=%d track=%d state=%v", list.Selected, p.Transport.List.Index(), p.Transport.State)
				}
				injectClick(platform.ButtonLeft, 0)
				injectClick(platform.ButtonLeft, 2)
				injectClick(platform.ButtonLeft, 2)
				a.PumpOnce()
				if list.Selected != 2 || p.Transport.List.Index() != 2 || p.Transport.State != players.Playing {
					t.Fatalf("double click: selected=%d track=%d state=%v", list.Selected, p.Transport.List.Index(), p.Transport.State)
				}
				if p.List.Focus() != list {
					t.Fatalf("playlist focus belongs to %T, want outer tracks", p.List.Focus())
				}
				press(platform.KeyDown)
				if list.Selected != 3 || p.Transport.List.Index() != 2 {
					t.Fatalf("Down: selected=%d track=%d", list.Selected, p.Transport.List.Index())
				}
				press(platform.KeyEnd)
				if list.Selected != list.Count-1 || p.Transport.List.Index() != 2 {
					t.Fatal("End did not select the last track without changing playback")
				}
				press(platform.KeyHome)
				press(platform.KeySpace)
				if p.Transport.State != players.Paused || p.Transport.List.Index() != 2 {
					t.Fatalf("Space should pause the current track: track=%d state=%v", p.Transport.List.Index(), p.Transport.State)
				}
				press(platform.KeyReturn)
				if p.Transport.State != players.Playing || p.Transport.List.Index() != 0 {
					t.Fatal("Return did not play the selected track")
				}
				press(platform.KeyB)
				if p.Transport.List.Index() != 1 {
					t.Fatal("the playlist swallowed the B transport shortcut")
				}
				p.Transport.State = players.Paused
				click(platform.ButtonRight, 2)
				if list.Selected != 1 || p.Transport.List.Index() != 1 || p.Transport.State != players.Paused {
					t.Fatal("opening the skin menu selected or started a track")
				}
				if p.List.Popup() == nil {
					t.Fatal("secondary press did not open the skin menu")
				}
			})
		}
	}
}

func TestPlaylistScrollAndAccessibleRowsFollowTheSkin(t *testing.T) {
	for _, pack := range []string{Skin, SkinSilver, Themed} {
		t.Run(pack, func(t *testing.T) {
			a, p := openPlaylist(t, pack, 1.5)
			list := p.listPane.list
			var library []players.Track
			for range 4 {
				library = append(library, players.Library()...)
			}
			p.Transport.List = players.NewPlaylist(library...)
			p.listPane.sync()
			a.PumpOnce()
			if list.Count != len(library) || list.MaxOffset() <= 0 {
				t.Fatalf("playlist did not pick up the longer queue: Count=%d MaxOffset=%f", list.Count, list.MaxOffset())
			}
			rows := style.ViewFrameInsetsOf(list.Look()).Apply(list.LocalBounds())
			if faceOf(list.Look()).panelled() {
				// Compare against the app's manifest slot, not ListView's
				// implementation of its geometry hook.
				rows = slotRect(list.Look(), layoutPlaylist, "rows", p.listPane.LocalBounds())
				rows = rows.Translate(paintengine2d.Pt(-list.Bounds().Min.X, -list.Bounds().Min.Y))
			} else {
				rows.Max.X -= style.ScrollGutter(list.Look())
			}
			want := widget.LocalToWindow(list, paintengine2d.XYWH(rows.Min.X, rows.Min.Y, rows.Dx(), list.RowHeightPx()))
			items := list.AccessibleItems()
			if items[0].Bounds != want {
				t.Fatalf("first accessible row is at %+v, artwork row is at %+v", items[0].Bounds, want)
			}
			if got := items[0].Name; got != "1. The Tessellators - Harbour Lights, 3:47" {
				t.Fatalf("accessible name lost title or duration: %q", got)
			}
			last := list.Count - 1
			if items[last].State&a11y.StateOffscreen == 0 {
				t.Fatal("last row is reported onscreen before scrolling")
			}
			if !list.AccessibleAction(last, a11y.ActionScrollIntoView) {
				t.Fatal("accessible scroll action failed")
			}
			if list.OffsetY <= 0 || list.AccessibleItems()[last].State&a11y.StateOffscreen != 0 {
				t.Fatal("accessible scroll did not reveal the last row")
			}
			list.ScrollTo(0)
			pos := list.AccessibleItems()[0].Bounds.Center()
			p.List.Inject(platform.Event{Kind: platform.EventScroll, Pos: pos, Scroll: paintengine2d.Pt(0, 1)})
			a.PumpOnce()
			if list.OffsetY <= 0 {
				t.Fatal("wheel event did not scroll the stock list")
			}
			list.ScrollTo(0)
			track, thumb := list.ScrollTrack()
			if track.Empty() || thumb.Empty() {
				t.Fatal("overflow list has no scroll thumb")
			}
			o := widget.DeviceOrigin(list)
			pos = thumb.Center().Add(o)
			p.List.Inject(platform.Event{Kind: platform.EventMouseDown, Button: platform.ButtonLeft, Pos: pos})
			pos.Y += (track.Dy() - thumb.Dy()) / 2
			p.List.Inject(platform.Event{Kind: platform.EventMouseMove, Pos: pos})
			p.List.Inject(platform.Event{Kind: platform.EventMouseUp, Button: platform.ButtonLeft, Pos: pos})
			a.PumpOnce()
			if list.OffsetY <= 0 || list.OffsetY >= list.MaxOffset() {
				t.Fatalf("thumb drag did not scroll into the middle: offset=%f max=%f", list.OffsetY, list.MaxOffset())
			}
			list.ScrollTo(list.MaxOffset())
			if list.MouseWheel(widget.MouseEvent{Scroll: paintengine2d.Pt(0, 1)}) {
				t.Fatal("wheel at the end should bubble to the parent")
			}
		})
	}
}

func TestPlaylistAccessibleActivationResumesCurrentTrack(t *testing.T) {
	_, p := open(t, Skin, 1)
	list := p.listPane.list
	p.Transport.State = players.Paused
	if !list.AccessibleAction(list.Selected, a11y.ActionDefault) || p.Transport.State != players.Playing {
		t.Fatal("activating the already selected row did not resume playback")
	}
	if list.AccessibleAction(-1, a11y.ActionDefault) || list.AccessibleAction(list.Count, a11y.ActionDefault) {
		t.Fatal("invalid accessible item was accepted")
	}
	list.SetEnabled(false)
	if list.AccessibleAction(list.Selected, a11y.ActionDefault) {
		t.Fatal("disabled playlist accepted an activation")
	}
}

// An independently supplied layout can inset its rows anywhere within the
// list. The accessible bounds and offscreen flags follow that viewport,
// including partially visible rows after scrolling by less than one row.
func TestPlaylistAccessibleBoundsFollowInsetRowsWhenScrolled(t *testing.T) {
	_, p := open(t, Skin, 1.5)
	list := p.listPane.list
	rows := paintengine2d.XYWH(12, 20, 200, 80)
	list.geo = &listGeo{rows: rows, rowH: 10}
	list.Arrange(paintengine2d.XYWH(40, 44, 240, 120))
	for _, offset := range []float32{0, 5, 20} {
		list.ScrollTo(offset)
		for i, n := range list.AccessibleItems() {
			local := paintengine2d.XYWH(12, 20+float32(i)*10-offset, 200, 10)
			want := widget.LocalToWindow(list, local)
			if n.Bounds != want {
				t.Errorf("offset=%g row=%d: accessible bounds=%v, painted bounds=%v", offset, i, n.Bounds, want)
			}
			if offscreen := n.State&a11y.StateOffscreen != 0; offscreen == local.Overlaps(rows) {
				t.Errorf("offset=%g row=%d: offscreen=%v for local bounds=%v", offset, i, offscreen, local)
			}
		}
	}
}

type playlistRowLook struct {
	style.LookAndFeel
	states []style.ControlState
}

func (lk *playlistRowLook) DrawListRow(ctx *paintengine2d.Context, row paintengine2d.Rect, st style.ControlState, text string) {
	lk.states = append(lk.states, st)
	lk.LookAndFeel.DrawListRow(ctx, row, st, text)
}

// Paint through ListView's real row callback and the selected theme. The
// playing marker is app state, while keyboard focus belongs to the outer
// playlist component that the window dispatches input to.
func TestPlaylistPaintTracksOuterFocusAndPlayingRow(t *testing.T) {
	_, p := openPlaylist(t, Themed, 1)
	list := p.listPane.list
	lk := &playlistRowLook{LookAndFeel: list.Look()}
	list.SetLook(lk)
	list.Selected = 2
	p.List.RequestFocus(list)
	list.MarkKeyboardFocus()
	paint := func() {
		lk.states = nil
		b := list.LocalBounds()
		list.Paint(paintengine2d.NewContext(paintengine2d.NewImage(int(b.Dx()), int(b.Dy()))))
	}
	paint()
	if len(lk.states) < 3 {
		t.Fatalf("only %d rows were painted", len(lk.states))
	}
	if !lk.states[0].Checked() || lk.states[0].Focused() {
		t.Fatalf("playing row lost its marker or took keyboard focus: %v", lk.states[0])
	}
	if !lk.states[2].Focused() || lk.states[2].Checked() {
		t.Fatalf("current row lost keyboard focus or took the playing marker: %v", lk.states[2])
	}
	p.List.RequestFocus(p.listPane.mini[0])
	paint()
	for i, st := range lk.states {
		if st.Focused() {
			t.Errorf("row %d kept keyboard focus after focus moved to Previous", i)
		}
	}
}

// Exercise a helper's generic paint path without constructing a compact
// unskinned app. The shell's default presentation is tested separately.
func openPlaylist(t *testing.T, pack string, scale float32) (*uitoolkit.Application, *Player) {
	t.Helper()
	if pack != Themed {
		return open(t, pack, scale)
	}
	a, p := open(t, Skin, scale)
	_, ok := style.LoadTheme(Themed)
	if !ok {
		t.Fatalf("theme %q did not load", Themed)
	}
	ap := style.LookAppearance(p.App.Look())
	ap.Name, ap.FollowDesktop = Themed, false
	p.listPane.list.SetLook(style.WithAppearance(p.App.Look(), ap))
	p.listPane.list.geo = nil
	p.listPane.list.Arrange(p.listPane.list.Bounds())
	return a, p
}
