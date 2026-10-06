package realplayer

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/codemodify/media-player-music/internal/players"
	"github.com/codemodify/media-player-music/internal/players/playertest"
	"github.com/codemodify/uitoolkit"
	"github.com/codemodify/uitoolkit/a11y"
	"github.com/codemodify/uitoolkit/app"
	"github.com/codemodify/uitoolkit/platform"
	"github.com/codemodify/uitoolkit/style"
	"github.com/codemodify/uitoolkit/widget"
)

func TestMain(m *testing.M) { os.Exit(playertest.Main(m)) }
func open(t *testing.T, scale float32) (*uitoolkit.Application, *Player) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv(style.ThemeEnv, players.RealPlayerSkin)
	a := uitoolkit.New(uitoolkit.Options{Headless: true, Scale: scale, DisableLookWatch: true})
	h := players.NewHost(a)
	h.Packs = players.SkinnedIDs
	p, err := New(h, Options{Headless: true, Scale: scale})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(p.Close)
	settle(a, p)
	return a, p
}
func settle(a *uitoolkit.Application, p *Player) {
	for i := 0; i < 3; i++ {
		a.PumpOnce()
		p.Advance(70 * time.Millisecond)
	}
}
func click(a *uitoolkit.Application, w *app.Window, c widget.Component) {
	at := widget.DeviceOrigin(c).Add(c.LocalBounds().Center())
	w.Inject(platform.Event{Kind: platform.EventMouseMove, Pos: at})
	w.Inject(platform.Event{Kind: platform.EventMouseDown, Button: platform.ButtonLeft, Pos: at})
	w.Inject(platform.Event{Kind: platform.EventMouseUp, Button: platform.ButtonLeft, Pos: at})
	a.PumpOnce()
}
func key(a *uitoolkit.Application, w *app.Window, k platform.Key, mods platform.Modifiers) {
	w.Inject(platform.Event{Kind: platform.EventKeyDown, Key: k, Mods: mods})
	w.Inject(platform.Event{Kind: platform.EventKeyUp, Key: k, Mods: mods})
	a.PumpOnce()
}

func TestResponsiveLayoutControlsAndAccessibility(t *testing.T) {
	for _, scale := range []float32{1, 1.75, 2} {
		t.Run(fmt.Sprintf("%gx", scale), func(t *testing.T) {
			a, p := open(t, scale)
			if len(p.Windows()) != 1 || !p.Main.Resizable() || p.Main.Title() != "Music Player" {
				t.Fatal("not one resizable titled main")
			}
			for _, size := range [][2]int{{MinW, MinH}, {MainW, MainH}, {940, 650}} {
				p.Main.SetSize(size[0], size[1])
				settle(a, p)
				var bounds func(widget.Component)
				bounds = func(c widget.Component) {
					if !c.Visible() {
						return
					}
					for _, child := range c.Children() {
						if !child.Visible() {
							continue
						}
						r, box := child.Bounds(), c.LocalBounds()
						if r.Empty() || r.Min.X < box.Min.X-1 || r.Min.Y < box.Min.Y-1 || r.Max.X > box.Max.X+1 || r.Max.Y > box.Max.Y+1 {
							t.Errorf("%T bounds%v outside%v at%v", child, r, box, size)
						}
						bounds(child)
					}
				}
				bounds(p.body)
				tree := playertest.Audit(t, "RealPlayer", p.Main)
				if playertest.Count(tree, a11y.RoleSlider) < 14 {
					t.Fatal("seek,volume,preamp,bands,balance missing")
				}
				rows := p.body.queue.list.RowGeo(p.Main.Look())
				if !p.body.queue.list.LocalBounds().Contains(rows.Rows.Min) || rows.Rows.Max.Y > p.body.queue.list.LocalBounds().Max.Y {
					t.Fatal("row viewport clipped")
				}
			}
			p.Main.RequestFocus(p.body.eq.bands[0])
			p.ShowEqualiser(false)
			settle(a, p)
			if p.Main.Focus() != p.body.buttons["play"] || p.Host.Model.EqualizerVisible {
				t.Fatal("hidden equalizer retained focus or model flag")
			}
			key(a, p.Main, platform.KeyE, platform.ModCtrl)
			if !p.Host.Model.EqualizerVisible || !p.body.eq.Visible() {
				t.Fatal("CtrlE did not restore EQ")
			}
			old := p.body.canvas.Bounds()
			p.ShowPlaylist(false)
			settle(a, p)
			if p.body.canvas.Bounds().Min.X >= old.Min.X {
				t.Fatal("canvas did not use hidden queue space")
			}
			key(a, p.Main, platform.KeyL, platform.ModCtrl)
			if !p.body.queue.Visible() {
				t.Fatal("CtrlL did not restore queue")
			}
		})
	}
}
func TestRealPlayerActionsUseHostAndModel(t *testing.T) {
	a, p := open(t, 1.75)
	var commands []players.Command
	p.Host.Run = func(c players.Command) { commands = append(commands, c); p.Transport.Do(c) }
	click(a, p.Main, p.body.buttons["play"])
	p.Transport.Pos = 31 * time.Second
	click(a, p.Main, p.body.buttons["play"])
	if p.Transport.State != players.Playing || p.Transport.Pos != 31*time.Second || !reflect.DeepEqual(commands, []players.Command{players.CmdPlay, players.CmdPlay}) {
		t.Fatal("Play is not idempotent")
	}
	click(a, p.Main, p.body.buttons["pause"])
	if p.Transport.State != players.Paused {
		t.Fatal("pause")
	}
	click(a, p.Main, p.body.buttons["pause"])
	if p.Transport.State != players.Playing {
		t.Fatal("resume")
	}
	var appendModes []bool
	p.Host.LoadFiles = func(from widget.Component, appendToList bool) {
		if from == nil {
			t.Error("no chooser origin")
		}
		appendModes = append(appendModes, appendToList)
	}
	click(a, p.Main, p.body.buttons["open"])
	click(a, p.Main, p.body.buttons["add"])
	click(a, p.Main, p.body.queue.buttons["add"])
	if !reflect.DeepEqual(appendModes, []bool{false, true, true}) {
		t.Fatalf("load modes%v", appendModes)
	}
	deleted, settings := 0, 0
	p.Host.DeleteTrack = func() { deleted++ }
	p.Host.ShowSettings = func() { settings++ }
	click(a, p.Main, p.body.queue.buttons["remove"])
	click(a, p.Main, p.body.buttons["settings"])
	click(a, p.Main, p.body.buttons["backends"])
	if deleted != 1 || settings != 2 {
		t.Fatal("settings/remove callbacks")
	}
	syncs := 0
	p.Host.SyncPlayback = func() { syncs++ }
	p.Main.RequestFocus(p.body.volume)
	key(a, p.Main, platform.KeyHome, 0)
	key(a, p.Main, platform.KeyRight, 0)
	p.Main.RequestFocus(p.body.seek)
	key(a, p.Main, platform.KeyEnd, 0)
	p.Main.RequestFocus(p.body.eq.balance)
	key(a, p.Main, platform.KeyEnd, 0)
	if syncs < 4 || p.Transport.Volume <= 0 || p.Transport.Fraction() < .99 || p.Transport.Pan != 1 {
		t.Fatalf("host slider callbacks %d vol%g seek%g pan%g", syncs, p.Transport.Volume, p.Transport.Fraction(), p.Transport.Pan)
	}
	p.Main.RequestFocus(p.body.eq.bands[0])
	key(a, p.Main, platform.KeyEnd, 0)
	if p.Equaliser.Gains[0] == 0 {
		t.Fatal("EQ gain")
	}
	p.Host.Model.BackendLimited = true
	p.Host.Model.BackendEqualizer = false
	p.Host.Model.BackendBalance = false
	p.Host.Model.BackendSpectrum = false
	p.Refresh()
	if p.body.eq.bands[0].Enabled() || p.body.eq.balance.Enabled() || p.body.buttons["visual"].Enabled() || p.body.canvas.analyser.Visible() {
		t.Fatal("unsupported backend features exposed")
	}
	requested := 0
	p.Main.SetOnCloseRequest(func() bool { requested++; return false })
	click(a, p.Main, p.body.buttons["close"])
	settle(a, p)
	if requested != 1 || p.Main.Closed() {
		t.Fatal("caption close bypassed shell close-request policy")
	}
}

func TestRealPlayerFocusedControlsKeepTransportShortcutRouting(t *testing.T) {
	a, p := open(t, 1.75)
	var commands []players.Command
	p.Host.Run = func(c players.Command) { commands = append(commands, c); p.Transport.Do(c) }
	// A wrapped stock slider remains a publicly identifiable focus target.
	var seek widget.Component
	widget.Walk(p.body, func(c widget.Component) {
		if named, ok := c.(interface{ AccessibleName() string }); ok && named.AccessibleName() == "Seek" {
			seek = c
		}
	})
	if seek == nil {
		t.Fatal("accessible seek control not found")
	}
	for _, c := range []widget.Component{seek, p.body.volume, p.body.eq.preamp, p.body.eq.balance, p.body.queue.list} {
		p.Transport.Play()
		p.Transport.Pos = 17 * time.Second
		p.Main.RequestFocus(c)
		if p.Main.Focus() != c {
			t.Fatalf("%T did not receive focus", c)
		}
		before := len(commands)
		key(a, p.Main, platform.KeySpace, 0)
		if p.Transport.State != players.Paused || p.Transport.Pos != 17*time.Second || len(commands) != before+1 || commands[before] != players.CmdPlayPause {
			t.Fatalf("%T consumed or duplicated shared Space transport shortcut", c)
		}
		track := p.Transport.List.Index()
		key(a, p.Main, platform.KeyB, 0)
		if p.Transport.List.Index() == track || len(commands) != before+2 || commands[before+1] != players.CmdNext {
			t.Fatalf("%T consumed or duplicated shared next-track shortcut", c)
		}
	}
	// Stock button Space activates that focused button. The separate Play
	// button therefore retains idempotent Play rather than becoming Pause.
	p.Transport.Play()
	p.Transport.Pos = 23 * time.Second
	p.Main.RequestFocus(p.body.buttons["play"])
	before := len(commands)
	key(a, p.Main, platform.KeySpace, 0)
	if p.Transport.State != players.Playing || p.Transport.Pos != 23*time.Second || len(commands) != before+1 || commands[before] != players.CmdPlay {
		t.Fatal("focused Play lost stock button Space activation")
	}
}
func TestSavedPlacementAndVisiblePanels(t *testing.T) {
	a, p := open(t, 1.75)
	p.RestorePlacement(map[string]players.WindowPlacement{"main": {X: 420, Y: 210, Width: 940, Height: 650, Positioned: true}})
	settle(a, p)
	saved := p.CapturePlacement()["main"]
	if saved.Width != 940 || saved.Height != 650 || saved.X != 420 || saved.Y != 210 || !saved.Positioned {
		t.Fatalf("restore%+v", saved)
	}
	p.ShowPlaylist(false)
	p.ShowEqualiser(false)
	if p.Host.Model.PlaylistVisible || p.Host.Model.EqualizerVisible {
		t.Fatal("visibility model")
	}
	p.RestorePlacement(map[string]players.WindowPlacement{"main": {Width: 100, Height: 100}})
	settle(a, p)
	w, h := p.Main.Size()
	if w != MinW || h != MinH {
		t.Fatalf("min%d×%d", w, h)
	}
}
func TestRealPlayerReferenceCaptures(t *testing.T) {
	out := os.Getenv("MUSIC_PLAYER_REALPLAYER_CAPTURES")
	if out == "" {
		t.Skip("opt-in appearance captures")
	}
	if err := os.MkdirAll(out, 0700); err != nil {
		t.Fatal(err)
	}
	for _, scale := range []float32{1, 1.75, 2} {
		t.Run(fmt.Sprintf("%gx", scale), func(t *testing.T) {
			a, p := open(t, scale)
			p.Pose(46 * time.Second)
			for _, s := range []struct {
				name string
				w, h int
			}{{"preferred", MainW, MainH}, {"minimum", MinW, MinH}, {"resized", 940, 650}} {
				p.Main.SetSize(s.w, s.h)
				settle(a, p)
				if err := p.Main.Capture().WritePNGFile(filepath.Join(out, fmt.Sprintf("realplayer-%g-%s.png", scale, s.name))); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}
