package standard

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/codemodify/media-player-music/internal/players"
	"github.com/codemodify/media-player-music/internal/players/playertest"
	"github.com/codemodify/uitoolkit"
	"github.com/codemodify/uitoolkit/a11y"
	"github.com/codemodify/uitoolkit/platform"
	"github.com/codemodify/uitoolkit/style"
	"github.com/codemodify/uitoolkit/widget"
	"github.com/codemodify/uitoolkit/widgets"
)

func TestMain(m *testing.M) { os.Exit(playertest.Main(m)) }

func open(t *testing.T, scale float32) (*uitoolkit.Application, *Player) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv(style.ThemeEnv, players.Themed)
	a := uitoolkit.New(uitoolkit.Options{Headless: true, Scale: scale, DisableLookWatch: true})
	h := players.NewHost(a)
	h.Packs = players.PanelSkins
	p, err := New(h, Options{Headless: true, Scale: scale})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(p.Close)
	a.PumpOnce()
	return a, p
}

func controls(p *Player) []widget.Component {
	m, q, e := p.body.main, p.body.queue, p.body.eq
	out := []widget.Component{m.seek, m.volume, m.balance, m.shuffle, m.repeat, m.eq, m.playlist, m.skin, e.on, e.auto, e.presets, e.preamp, q.list, q.add, q.remove, q.selectTrack, q.misc, q.options}
	for _, c := range m.transport {
		out = append(out, c)
	}
	for _, c := range q.mini {
		out = append(out, c)
	}
	for _, c := range e.bands {
		out = append(out, c)
	}
	return out
}

func TestEveryControlIsAccessibleAndReachable(t *testing.T) {
	a, p := open(t, 1.75)
	if len(p.Windows()) != 1 || p.Main.ShapeActive() {
		t.Fatal("ordinary UI must have one rectangular window")
	}
	tree := playertest.Audit(t, "default", p.Main)
	if playertest.Count(tree, a11y.RoleMenuBar) != 0 || playertest.Count(tree, a11y.RoleStatusBar) != 1 {
		t.Fatal("ordinary UI must have no menu bar and retain its status line")
	}
	ring := playertest.FocusRing(t, a, p.Main)
	for _, c := range controls(p) {
		if tree.Find(c.ID()) == nil {
			t.Errorf("missing accessible control %T", c)
		}
		if !playertest.Reaches(ring, c) {
			t.Errorf("Tab never reaches %T", c)
		}
	}
}

func TestControlsFitAtTheMinimumAndAllDisplayScales(t *testing.T) {
	for _, scale := range []float32{1, 1.25, 1.5, 1.75, 2} {
		t.Run(fmt.Sprintf("%gx", scale), func(t *testing.T) {
			a, p := open(t, scale)
			p.Pose(97 * time.Second)
			for _, size := range [][2]int{{MainW, MainH}, {MinW, MinH}} {
				p.Main.SetSize(size[0], size[1])
				a.PumpOnce()
				for _, c := range controls(p) {
					b := c.Bounds()
					parent := c.Parent().LocalBounds()
					if b.Empty() || b.Min.X < -.5 || b.Min.Y < -.5 || b.Max.X > parent.Max.X+.5 || b.Max.Y > parent.Max.Y+.5 {
						t.Errorf("%dx%d: %T outside its parent: %v in %v", size[0], size[1], c, b, parent)
					}
				}
				m := p.body.main
				if m.repeat.Bounds().Max.X > m.eq.Bounds().Min.X {
					t.Error("transport toggles overlap the view toggles")
				}
				if img := p.Main.Capture(); img == nil || img.Bounds().Dx() < int(float32(size[0])*scale)-2 {
					t.Fatal("ordinary UI did not paint a full window")
				}
			}
		})
	}
}

func TestTransportAndModelInputThroughTheRealControls(t *testing.T) {
	a, p := open(t, 1)
	press := func(c widget.Component) {
		t.Helper()
		if !p.Main.AccessibleAction(c.ID(), a11y.ActionDefault) {
			t.Fatalf("%T rejected activation", c)
		}
		a.PumpOnce()
	}
	main := p.body.main
	press(main.transport[1])
	if p.Transport.State != players.Playing {
		t.Fatal("Play did not start the model")
	}
	press(main.transport[2])
	if p.Transport.State != players.Paused {
		t.Fatal("Pause did not pause the model")
	}
	press(main.transport[4])
	if p.Transport.List.Index() != 1 {
		t.Fatal("Next did not change the track")
	}
	press(p.body.queue.mini[1])
	if p.Transport.State != players.Playing {
		t.Fatal("playlist Play did not resume the same model")
	}
	press(main.transport[5])
	if p.Transport.State != players.Playing || p.Transport.List.Index() != 1 {
		t.Fatal("opening the file picker changed playback before any files were chosen")
	}
	press(main.shuffle)
	press(main.repeat)
	if !p.Transport.Random || p.Transport.Repeat != players.RepeatAll {
		t.Fatal("shuffle/repeat buttons did not update the model")
	}
	for _, test := range []struct {
		c      widget.Component
		action a11y.Action
		check  func() bool
	}{
		{main.seek, a11y.ActionIncrement, func() bool { return p.Transport.Pos > 0 }},
		{main.volume, a11y.ActionDecrement, func() bool { return p.Transport.Volume < .75 }},
		{main.balance, a11y.ActionIncrement, func() bool { return p.Transport.Pan > 0 }},
		{p.body.eq.preamp, a11y.ActionIncrement, func() bool { return p.Equaliser.Preamp > 0 }},
		{p.body.eq.bands[3], a11y.ActionIncrement, func() bool { return p.Equaliser.Gains[3] > 0 }},
	} {
		if !p.Main.AccessibleAction(test.c.ID(), test.action) || !test.check() {
			t.Fatalf("%T input did not reach the model", test.c)
		}
	}
	// A model refresh must never be interpreted as a fresh user edit.
	p.Transport.SetVolume(.42)
	p.Transport.SetPan(-.3)
	p.Equaliser.Apply("Vocal")
	a.PumpOnce()
	if main.volume.Value != 42 || main.balance.Value != -.3 || p.body.eq.bands[3].Value != p.Equaliser.Gains[3] {
		t.Fatal("controls did not follow model changes")
	}
	p.Host.Command(players.CmdStop)
	p.Main.RequestFocus(main.transport[0])
	p.Main.Inject(platform.Event{Kind: platform.EventKeyDown, Rune: 'x'})
	a.PumpOnce()
	if p.Transport.State != players.Playing {
		t.Fatal("shared keyboard transport did not start playback")
	}
}

func TestPlaybackControlsReachTheHost(t *testing.T) {
	a, p := open(t, 1)
	var commands []players.Command
	p.Host.Run = func(c players.Command) {
		commands = append(commands, c)
		p.Transport.Do(c)
	}
	activate := func(c widget.Component, action a11y.Action) {
		t.Helper()
		if !p.Main.AccessibleAction(c.ID(), action) {
			t.Fatalf("%T rejected %v", c, action)
		}
		a.PumpOnce()
	}
	m := p.body.main
	activate(m.transport[1], a11y.ActionDefault)
	activate(m.transport[1], a11y.ActionDefault)
	if len(commands) != 2 || commands[0] != players.CmdPlay || commands[1] != players.CmdPlay || p.Transport.State != players.Playing {
		t.Fatalf("Play must reach the host without toggling Pause: commands=%v state=%v", commands, p.Transport.State)
	}
	activate(m.transport[2], a11y.ActionDefault)
	if commands[len(commands)-1] != players.CmdPlayPause || p.Transport.State != players.Paused {
		t.Fatal("Pause did not reach the shared command")
	}
	var syncs int
	p.Host.SyncPlayback = func() { syncs++ }
	activate(m.seek, a11y.ActionIncrement)
	activate(m.volume, a11y.ActionDecrement)
	activate(m.balance, a11y.ActionIncrement)
	if syncs != 3 || p.Transport.Pos <= 0 || p.Transport.Volume >= .75 || p.Transport.Pan <= 0 {
		t.Fatalf("faders bypassed host playback synchronization: syncs=%d position=%v volume=%g balance=%g", syncs, p.Transport.Pos, p.Transport.Volume, p.Transport.Pan)
	}
	p.Transport.SetVolume(.4)
	p.Transport.SetPan(-.2)
	p.Refresh()
	if syncs != 3 {
		t.Fatalf("model refresh was treated as user playback input: syncs=%d", syncs)
	}
	list := p.body.queue.list
	if !p.Main.AccessibleAction(widget.ItemID(list, 2), a11y.ActionDefault) {
		t.Fatal("track activation was rejected")
	}
	if p.Transport.List.Index() != 2 || commands[len(commands)-1] != players.CmdPlay || p.Transport.State != players.Playing {
		t.Fatal("track activation bypassed host Play")
	}
	commands = nil
	p.Transport.Pause()
	p.Main.RequestFocus(list)
	p.Main.Inject(platform.Event{Kind: platform.EventKeyDown, Key: platform.KeyReturn})
	a.PumpOnce()
	if len(commands) != 1 || commands[0] != players.CmdPlay || p.Transport.State != players.Playing {
		t.Fatal("Return from the playlist bypassed host Play")
	}
	commands = nil
	p.Transport.Pause()
	activate(p.body.queue.options, a11y.ActionDefault)
	for _, key := range []platform.Key{platform.KeyHome, platform.KeyReturn} {
		p.Main.Inject(platform.Event{Kind: platform.EventKeyDown, Key: key})
		a.PumpOnce()
	}
	if len(commands) != 1 || commands[0] != players.CmdPlay || p.Transport.List.Index() != 0 || p.Transport.State != players.Playing {
		t.Fatal("Play the first track menu bypassed host track activation")
	}
}

func TestFileActionsUseHostHooksAndCorrectOrigins(t *testing.T) {
	a, p := open(t, 1)
	type request struct {
		from   widget.Component
		append bool
	}
	var requests []request
	p.Host.LoadFiles = func(from widget.Component, appendToList bool) {
		requests = append(requests, request{from, appendToList})
	}
	removed := 0
	p.Host.DeleteTrack = func() { removed++ }
	p.Transport.SelectTrack(2)
	p.Transport.Play()
	p.Transport.Pos = 17 * time.Second
	for _, c := range []widget.Component{p.body.main.transport[5], p.body.queue.mini[5], p.body.queue.add, p.body.queue.remove} {
		if !p.Main.AccessibleAction(c.ID(), a11y.ActionDefault) {
			t.Fatalf("%T rejected activation", c)
		}
		a.PumpOnce()
	}
	want := []request{{p.body.main.transport[5], false}, {p.body.queue.mini[5], false}, {p.body.queue.add, true}}
	if len(requests) != len(want) {
		t.Fatalf("file requests=%d, want %d", len(requests), len(want))
	}
	for i := range want {
		if requests[i] != want[i] {
			t.Errorf("file request %d=%+v, want %+v", i, requests[i], want[i])
		}
	}
	if removed != 1 {
		t.Fatalf("Remove hook called %d times", removed)
	}
	if p.Transport.State != players.Playing || p.Transport.List.Index() != 2 || p.Transport.Pos != 17*time.Second {
		t.Fatal("requesting files changed playback before the host supplied any result")
	}
}

func TestEmptyLivePlaylistAndPlaybackStatus(t *testing.T) {
	a, p := open(t, 1)
	p.Host.Model.Live = true
	p.Transport.List = players.NewPlaylist()
	p.Transport.Stop()
	p.Refresh()
	a.PumpOnce()
	tree := playertest.Audit(t, "empty live default", p.Main)
	if p.body.queue.list.Count != 0 || tree.Find(p.body.main.display.ID()).Name != "Open a music file to begin" {
		t.Fatal("empty playlist did not expose its file-opening instruction")
	}
	if p.Main.Title() != "Music Player" || p.body.status.Parts()[0] != p.Host.StatusText() {
		t.Fatal("ordinary UI retained demo title/status")
	}
	p.Host.Model.Error = "mpv executable unavailable"
	p.Refresh()
	if p.body.status.Parts()[0] != p.Host.Model.Error {
		t.Fatal("playback error did not reach the status bar")
	}
	changes := 0
	p.Equaliser.Changed = func() { changes++; p.Refresh() }
	if !p.Main.AccessibleAction(p.body.eq.on.ID(), a11y.ActionDefault) || changes != 1 || !p.Equaliser.On {
		t.Fatal("equaliser On bypassed the model change notification")
	}
}

func TestHiddenAreasLeaveTheKeyboardAndPersistInTheModel(t *testing.T) {
	a, p := open(t, 1.75)
	p.Main.RequestFocus(p.body.eq.preamp)
	p.ShowEqualiser(false)
	a.PumpOnce()
	if p.Host.Model.EqualizerVisible || p.Main.Focus() != p.body.main.transport[1] || p.Main.AccessibleTree().Find(p.body.eq.preamp.ID()) != nil {
		t.Fatal("hidden equaliser retained focus or accessibility nodes")
	}
	p.Main.RequestFocus(p.body.queue.list)
	p.Main.Inject(platform.Event{Kind: platform.EventKeyDown, Rune: 'l', Mods: platform.ModCtrl})
	a.PumpOnce()
	if p.Host.Model.PlaylistVisible || p.Main.Focus() != p.body.main.transport[1] {
		t.Fatal("playlist shortcut did not hide it and return focus")
	}
	p.Close()
	reopened, err := New(p.Host, Options{Headless: true, Scale: 1.75})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(reopened.Close)
	a.PumpOnce()
	if reopened.EqualiserShown() || reopened.PlaylistShown() {
		t.Fatal("visibility did not survive reopening the presentation")
	}
	reopened.ShowEqualiser(true)
	reopened.ShowPlaylist(true)
	a.PumpOnce()
	if reopened.body.eq.preamp.Bounds().Empty() || reopened.body.queue.list.Bounds().Empty() {
		t.Fatal("restored areas were not laid out")
	}
}

func TestAutoPresetAndSkinChoicesUseSharedActions(t *testing.T) {
	a, p := open(t, 1)
	if !p.Main.AccessibleAction(p.body.eq.auto.ID(), a11y.ActionDefault) || !p.Equaliser.Auto {
		t.Fatal("AUTO did not update the shared equaliser")
	}
	first := p.Equaliser.Preset
	p.Host.Command(players.CmdNext)
	a.PumpOnce()
	if p.Equaliser.Preset == first {
		t.Fatal("AUTO did not choose for the new track")
	}
	chosen := ""
	p.Host.ChooseSkin = func(id string) { chosen = id }
	p.Main.RequestFocus(p.body.main.skin)
	p.Main.Inject(platform.Event{Kind: platform.EventKeyDown, Rune: 'k', Mods: platform.ModCtrl})
	a.PumpOnce()
	if chosen != "winamp" {
		t.Fatalf("Ctrl+K chose %q", chosen)
	}
	p.body.main.skin.OnClick()
	a.PumpOnce()
	if p.Main.Popup() == nil {
		t.Fatal("skin control did not offer skin choices")
	}
}

func TestPlaylistActionsAndShortcutsUsePlayerSemantics(t *testing.T) {
	for _, scale := range []float32{1, 1.75} {
		t.Run(fmt.Sprintf("%gx", scale), func(t *testing.T) {
			a, p := open(t, scale)
			list := p.body.queue.list
			p.Transport.Play()
			p.Transport.Pause()
			p.Transport.Pos = 17 * time.Second
			p.Refresh()
			a.PumpOnce()
			if !p.Main.AccessibleAction(widget.ItemID(list, list.Selected), a11y.ActionDefault) || p.Transport.State != players.Playing {
				t.Fatal("activating the selected row did not resume the player")
			}
			p.Transport.Pause()
			row := p.Main.AccessibleTree().Find(widget.ItemID(list, 2)).Bounds
			pos := row.Min
			pos.X += row.Dx() / 2
			pos.Y += row.Dy() / 2
			p.Main.Inject(platform.Event{Kind: platform.EventMouseDown, Button: platform.ButtonRight, Pos: pos})
			p.Main.Inject(platform.Event{Kind: platform.EventMouseUp, Button: platform.ButtonRight, Pos: pos})
			a.PumpOnce()
			if p.Main.Popup() == nil || p.Transport.State != players.Paused || p.Transport.List.Index() != 0 || p.Transport.Pos != 17*time.Second || list.Selected != 0 {
				t.Fatal("opening a row context menu changed the selected track or playback")
			}
			p.Main.DismissPopup()
			p.Main.RequestFocus(list)
			p.Main.Inject(platform.Event{Kind: platform.EventKeyDown, Key: platform.KeySpace})
			a.PumpOnce()
			if p.Transport.State != players.Playing {
				t.Fatal("Space from playlist did not resume the player")
			}
			p.Main.Inject(platform.Event{Kind: platform.EventKeyDown, Key: platform.KeySpace})
			a.PumpOnce()
			if p.Transport.State != players.Paused {
				t.Fatal("Space from playlist activated a row instead of pausing")
			}
			p.Main.Inject(platform.Event{Kind: platform.EventKeyDown, Rune: 'b'})
			a.PumpOnce()
			if p.Transport.List.Index() != 1 || p.Transport.State != players.Paused {
				t.Fatal("B from playlist did not reach the shared next command")
			}
			p.Main.Inject(platform.Event{Kind: platform.EventKeyDown, Rune: 'z'})
			a.PumpOnce()
			if p.Transport.List.Index() != 0 || p.Transport.State != players.Paused {
				t.Fatal("Z from playlist did not reach the shared previous command")
			}
			p.Main.Inject(platform.Event{Kind: platform.EventKeyDown, Key: platform.KeyReturn})
			a.PumpOnce()
			if p.Transport.State != players.Playing {
				t.Fatal("Return did not activate the selected track")
			}
		})
	}
}

func TestEqualiserPresetsWorkThroughTheToolkitMenu(t *testing.T) {
	a, p := open(t, 1.75)
	if !p.Main.AccessibleAction(p.body.eq.on.ID(), a11y.ActionDefault) || !p.Equaliser.On {
		t.Fatal("On did not enable the equaliser")
	}
	if !p.Main.AccessibleAction(p.body.eq.presets.ID(), a11y.ActionDefault) {
		t.Fatal("Presets did not activate")
	}
	a.PumpOnce()
	if _, ok := p.Main.Popup().(*widgets.PopupMenu); !ok {
		t.Fatal("Presets did not open a toolkit menu")
	}
	for _, key := range []platform.Key{platform.KeyHome, platform.KeyDown, platform.KeyReturn} {
		p.Main.Inject(platform.Event{Kind: platform.EventKeyDown, Key: key})
		a.PumpOnce()
	}
	if p.Equaliser.Preset != "Full bass" || p.body.eq.bands[0].Value != 9 {
		t.Fatal("preset selection did not update the equaliser and faders")
	}
}
