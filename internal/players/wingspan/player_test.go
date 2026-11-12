package wingspan

import (
	"fmt"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/codemodify/media-player-music/internal/players"
	"github.com/codemodify/media-player-music/internal/players/playertest"
	"goframework.io/desktop-ui"
	"goframework.io/desktop-ui-widgets"
	"goframework.io/desktop-ui/a11y"
	"goframework.io/desktop-ui/platform"
	"goframework.io/desktop-ui/style"
	"goframework.io/desktop-ui/widget"
	"goframework.io/graphics-paint2d"
)

func TestMain(m *testing.M) { os.Exit(playertest.Main(m)) }

func open(t *testing.T, scale float32) (*ui.Application, *Player) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv(style.ThemeEnv, players.WingspanSkin)
	a := ui.New(ui.Options{Headless: true, Scale: scale, DisableLookWatch: true})
	h := players.NewHost(a)
	h.Packs = players.SkinnedIDs
	p, err := New(h, Options{Headless: true, Scale: scale})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(p.Close)
	a.PumpOnce()
	return a, p
}

func injectClick(p *Player, button platform.MouseButton, at paint2d.Point) {
	p.Main.Inject(platform.Event{Kind: platform.EventMouseMove, Pos: at})
	p.Main.Inject(platform.Event{Kind: platform.EventMouseDown, Button: button, Pos: at})
	p.Main.Inject(platform.Event{Kind: platform.EventMouseUp, Button: button, Pos: at})
}

func key(a *ui.Application, p *Player, k platform.Key, mods platform.Modifiers) {
	p.Main.Inject(platform.Event{Kind: platform.EventKeyDown, Key: k, Mods: mods})
	p.Main.Inject(platform.Event{Kind: platform.EventKeyUp, Key: k, Mods: mods})
	a.PumpOnce()
}

func TestEveryWingStateUsesOneFixedShapedWindowAtEveryScale(t *testing.T) {
	for _, scale := range []float32{1, 1.75, 2} {
		t.Run(fmt.Sprintf("%gx", scale), func(t *testing.T) {
			a, p := open(t, scale)
			if windows := p.Windows(); len(windows) != 1 || windows[0].Window != p.Main || windows[0].Name != "main" {
				t.Fatal("Headspace must retain a single main window")
			}
			if p.Main.Resizable() || p.Main.Title() != "Music Player" {
				t.Fatal("Headspace must retain its fixed artwork and application title")
			}
			for _, state := range []struct {
				name   string
				eq, pl bool
			}{{"closed", false, false}, {"eq", true, false}, {"pl", false, true}, {"both", true, true}} {
				t.Run(state.name, func(t *testing.T) {
					p.ShowEqualiser(state.eq)
					p.ShowPlaylist(state.pl)
					a.PumpOnce()
					w, h := p.Main.Size()
					if w != MainW || h != MainH || !p.Main.ShapeActive() {
						t.Fatalf("wing changed fixed geometry or dropped silhouette: %dx%d active=%t", w, h, p.Main.ShapeActive())
					}
					pw, ph := p.Main.PixelSize()
					if pw != int(math.Round(MainW*float64(scale))) || ph != int(math.Round(MainH*float64(scale))) {
						t.Fatalf("wrong display scale: %dx%d", pw, ph)
					}
					shape := p.Main.Shape().Raster(pw, ph)
					frame := p.Main.Surface().(*platform.Offscreen).Frame()
					if shape.Clamped || !frame.Alpha || len(frame.Shape) < 10 {
						t.Fatal("silhouette degraded to a plain rectangle or opaque surface")
					}
					capture := p.Main.CaptureSurface()
					for _, sample := range []struct {
						x, y int
						in   bool
					}{{1, 1, false}, {755, 390, false}, {150, 175, state.eq}, {620, 175, state.pl}, {370, 140, true}} {
						x, y := int(float32(sample.x)*scale), int(float32(sample.y)*scale)
						// The native input region and captured pixels must agree;
						// synthetic events alone do not simulate desktop clickthrough.
						if got := platform.RectsContain(frame.Shape, x, y); got != sample.in || shape.Contains(x, y) != sample.in {
							t.Errorf("input at design %d,%d = %t, want %t", sample.x, sample.y, got, sample.in)
						}
						alpha := capture.NRGBAAt(x, y).A
						if (!sample.in && alpha != 0) || (sample.in && alpha != 255) {
							t.Errorf("alpha at design %d,%d = %d, inside=%t", sample.x, sample.y, alpha, sample.in)
						}
					}
					for _, c := range p.body.Children() {
						if !c.Visible() {
							continue
						}
						b, parent := c.Bounds(), p.body.LocalBounds()
						if b.Empty() || b.Min.X < 0 || b.Min.Y < 0 || b.Max.X > parent.Max.X || b.Max.Y > parent.Max.Y {
							t.Errorf("visible %T escaped the skin layout: %v", c, b)
						}
					}
					if p.body.volume.Visible() != state.eq || p.body.list.Visible() != state.pl {
						t.Fatal("wing artwork and actual control visibility disagree")
					}
					playertest.Audit(t, state.name, p.Main)
				})
			}
		})
	}
}

func TestSpriteTransportMouseKeyboardAndAccessibilityUseSharedHost(t *testing.T) {
	for _, scale := range []float32{1, 1.75, 2} {
		t.Run(fmt.Sprintf("%gx", scale), func(t *testing.T) {
			a, p := open(t, scale)
			var commands []players.Command
			p.Host.Run = func(c players.Command) { commands = append(commands, c); p.Transport.Do(c) }
			click := func(c widget.Component) {
				injectClick(p, platform.ButtonLeft, widget.DeviceBounds(c).Center())
				a.PumpOnce()
			}
			click(p.body.play)
			if p.Transport.State != players.Playing || p.body.play.Label != "Pause" {
				t.Fatal("original play sprite did not start playback")
			}
			click(p.body.play)
			if p.Transport.State != players.Paused || p.body.play.Label != "Play" {
				t.Fatal("play/pause sprite did not pause playback")
			}
			p.Main.RequestFocus(p.body.play)
			key(a, p, platform.KeyReturn, 0)
			if p.Transport.State != players.Playing {
				t.Fatal("focused sprite did not resume through keyboard input")
			}
			click(p.body.buttons["next"])
			if p.Transport.List.Index() != 1 {
				t.Fatal("next sprite did not advance the shared queue")
			}
			if !p.Main.AccessibleAction(p.body.buttons["stop"].ID(), a11y.ActionDefault) || p.Transport.State != players.Stopped {
				t.Fatal("accessible stop sprite did not stop playback")
			}
			want := []players.Command{players.CmdPlay, players.CmdPlayPause, players.CmdPlayPause, players.CmdNext, players.CmdStop}
			if !reflect.DeepEqual(commands, want) {
				t.Fatalf("sprite actions = %v, want %v", commands, want)
			}
			// Unclaimed keys on controls must still reach the shared transport.
			p.Main.RequestFocus(p.body.list)
			key(a, p, platform.KeySpace, 0)
			key(a, p, platform.KeyB, 0)
			p.Main.RequestFocus(p.body.bands[0])
			key(a, p, platform.KeyZ, 0)
			p.Main.RequestFocus(p.body.seek)
			key(a, p, platform.KeyV, 0)
			want = append(want, players.CmdPlayPause, players.CmdNext, players.CmdPrev, players.CmdStop)
			if !reflect.DeepEqual(commands, want) {
				t.Fatalf("controls swallowed transport shortcuts: %v", commands)
			}
		})
	}
}

func TestSharedLevelsAndEqualizer(t *testing.T) {
	_, p := open(t, 1.75)
	syncs := 0
	p.Host.SyncPlayback = func() { syncs++ }
	for _, f := range []*players.Fader{p.body.seek, p.body.volume, p.body.balance} {
		before := syncs
		if !p.Main.AccessibleAction(f.ID(), a11y.ActionIncrement) || syncs != before+1 {
			t.Fatalf("%s did not notify the shared playback hook once", f.Label)
		}
	}
	if p.Transport.Pos <= 0 || p.Transport.Volume != p.body.volume.Value/100 || p.Transport.Pan != p.body.balance.Value {
		t.Fatal("shared transport and visible levels disagree")
	}
	p.Refresh()
	if syncs != 3 {
		t.Fatal("model refresh was reported as another user edit")
	}
	eqChanges := 0
	refresh := p.Equaliser.Changed
	p.Equaliser.Changed = func() { eqChanges++; refresh() }
	if !p.Main.AccessibleAction(p.body.bands[0].ID(), a11y.ActionIncrement) || eqChanges != 1 || p.Equaliser.Gains[0] <= 0 {
		t.Fatal("EQ sprite slider did not edit and notify the shared equalizer")
	}
}

func TestDecodedFileAndFolderDropsUseTheSharedLoader(t *testing.T) {
	a, p := open(t, 1.75)
	p.Transport.State, p.Transport.Pos = players.Paused, 17*time.Second
	p.Refresh()
	a.PumpOnce()
	dir := t.TempDir()
	file, folder := filepath.Join(dir, "track one.mp3"), filepath.Join(dir, "music folder")
	if err := os.WriteFile(file, []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(folder, 0700); err != nil {
		t.Fatal(err)
	}
	uri := func(path string) string { return (&url.URL{Scheme: "file", Path: path}).String() }
	data := []byte("# file and folder\r\n" + uri(file) + "\r\n" + uri(folder) + "\r\n")
	want := []string{file, folder}
	calls := 0
	var got []string
	p.Host.DropFiles = func(paths []string) { calls++; got = paths }
	off := p.Main.Surface().(*platform.Offscreen)
	for _, at := range []paint2d.Point{p.body.list.AccessibleItems()[0].Bounds.Center(), widget.DeviceBounds(p.body.visual).Center()} {
		before := calls
		off.SimulateDropAction(at, map[string][]byte{"text/uri-list": data}, platform.DragCopy|platform.DragMove, platform.DragCopy)
		a.PumpOnce()
		if accepted := off.DropTaken(); accepted == nil || !*accepted || calls != before+1 || !reflect.DeepEqual(got, want) {
			t.Fatalf("drop at %v did not deliver local paths once: paths=%q calls=%d accepted=%v", at, got, calls, accepted)
		}
		if p.Transport.State != players.Paused || p.Transport.Pos != 17*time.Second || p.Transport.List.Index() != 0 {
			t.Fatal("queuing a drop changed playback before the host loaded it")
		}
	}
	for _, remote := range []string{"file://remote-host/tmp/music.mp3\r\n", "https://example.com/music.mp3\r\n"} {
		before := calls
		off.SimulateDrop(widget.DeviceBounds(p.body.visual).Center(), map[string][]byte{"text/uri-list": []byte(remote)})
		a.PumpOnce()
		if accepted := off.DropTaken(); accepted == nil || *accepted || calls != before {
			t.Fatal("remote media was treated as a local drop")
		}
	}
	if p.body.DropActionFor(platform.DragMove)&platform.DragMove != 0 || p.body.list.DropActionFor(platform.DragMove)&platform.DragMove != 0 {
		t.Fatal("Headspace offered to move source files")
	}
	for _, at := range []paint2d.Point{p.body.list.AccessibleItems()[0].Bounds.Center(), widget.DeviceBounds(p.body.visual).Center()} {
		before := calls
		off.SimulateDropAction(at, map[string][]byte{"text/uri-list": data}, platform.DragMove, platform.DragMove)
		a.PumpOnce()
		if accepted := off.DropTaken(); accepted == nil || *accepted || calls != before {
			t.Fatal("a source offering only Move was accepted")
		}
	}
}

func TestPlaylistSelectionActivationAndInterruptedGestures(t *testing.T) {
	for _, scale := range []float32{1, 1.75, 2} {
		t.Run(fmt.Sprintf("%gx", scale), func(t *testing.T) {
			a, p := open(t, scale)
			list := p.body.list
			plays := 0
			p.Host.Run = func(c players.Command) {
				if c == players.CmdPlay {
					plays++
				}
				p.Transport.Do(c)
			}
			p.Transport.State, p.Transport.Pos = players.Paused, 17*time.Second
			at := list.AccessibleItems()[2].Bounds.Center()
			injectClick(p, platform.ButtonLeft, at)
			a.PumpOnce()
			p.Refresh()
			a.PumpOnce()
			if list.Selected != 2 || p.Transport.List.Index() != 0 || p.Transport.State != players.Paused || p.Transport.Pos != 17*time.Second || plays != 0 {
				t.Fatal("single-click browsing changed playback or was overwritten by a refresh")
			}
			key(a, p, platform.KeyDown, 0)
			if list.Selected != 3 || plays != 0 || p.Transport.List.Index() != 0 {
				t.Fatal("arrow selection changed playback")
			}
			key(a, p, platform.KeyReturn, 0)
			if p.Transport.List.Index() != 3 || p.Transport.State != players.Playing || plays != 1 {
				t.Fatal("Return failed to activate the browsed row")
			}
			p.Transport.State = players.Paused
			if !p.Main.AccessibleAction(widget.ItemID(list, 3), a11y.ActionDefault) || p.Transport.State != players.Playing || plays != 2 {
				t.Fatal("accessible activation did not resume the current row")
			}
			injectClick(p, platform.ButtonLeft, at)
			injectClick(p, platform.ButtonLeft, at)
			a.PumpOnce()
			if p.Transport.List.Index() != 2 || plays != 3 {
				t.Fatal("double-click did not activate exactly once")
			}
			for _, interruption := range []string{"pointer-exit", "focus-change", "wheel-at-end"} {
				p.Transport.List.Select(0)
				p.Transport.State, p.Transport.Pos = players.Paused, 17*time.Second
				p.Refresh()
				// Moving away starts a fresh gesture without sleeping through a timer.
				p.Main.Inject(platform.Event{Kind: platform.EventMouseMove, Pos: widget.DeviceBounds(p.body.play).Center()})
				row := 2
				if interruption == "wheel-at-end" {
					list.ScrollTo(list.MaxOffset())
					row = list.Count - 1
				} else {
					list.ScrollTo(0)
				}
				at = list.AccessibleItems()[row].Bounds.Center()
				before := plays
				injectClick(p, platform.ButtonLeft, at)
				switch interruption {
				case "pointer-exit":
					p.Main.Inject(platform.Event{Kind: platform.EventMouseMove, Pos: widget.DeviceBounds(p.body.play).Center()})
				case "focus-change":
					p.Main.Inject(platform.Event{Kind: platform.EventKeyDown, Key: platform.KeyTab})
				case "wheel-at-end":
					p.Main.Inject(platform.Event{Kind: platform.EventScroll, Pos: at, Scroll: paint2d.Pt(0, 1)})
				}
				injectClick(p, platform.ButtonLeft, at)
				a.PumpOnce()
				if list.Selected != row || p.Transport.List.Index() != 0 || plays != before || p.Transport.State != players.Paused {
					t.Fatalf("%s combined interrupted clicks into playback", interruption)
				}
			}
		})
	}
}

func TestPlaylistGutterAndDisabledInputDoNotSelectOrPlay(t *testing.T) {
	a, p := open(t, 1.75)
	list := p.body.list
	geometry := list.geometry(list.Look())
	o := widget.DeviceOrigin(list)
	at := o.Add(paint2d.Pt((geometry.Rows.Max.X+geometry.Bar.Min.X)/2, geometry.Rows.Min.Y+geometry.RowHeight*2.5))
	selected, playing := list.Selected, p.Transport.List.Index()
	injectClick(p, platform.ButtonLeft, at)
	injectClick(p, platform.ButtonLeft, at)
	a.PumpOnce()
	if list.Selected != selected || p.Transport.List.Index() != playing || p.Transport.State != players.Stopped {
		t.Fatal("horizontal space between rows and scrollbar selected or activated a track")
	}
	list.SetEnabled(false)
	at = list.AccessibleItems()[2].Bounds.Center()
	for _, button := range []platform.MouseButton{platform.ButtonLeft, platform.ButtonRight} {
		injectClick(p, button, at)
		a.PumpOnce()
		if list.Selected != selected || p.Transport.List.Index() != playing || p.Main.Popup() != nil {
			t.Fatal("disabled list selected, played or opened its ancestor context menu")
		}
	}
}

func TestHiddenWingsLeaveFocusAndKeyboardAccessOnVisibleControls(t *testing.T) {
	a, p := open(t, 1.75)
	ring := playertest.FocusRing(t, a, p.Main)
	for _, c := range p.body.Children() {
		if c.Visible() && c.WantsFocus() && !playertest.Reaches(ring, c) {
			t.Fatalf("visible control %T is absent from Tab order", c)
		}
	}
	p.Main.RequestFocus(p.body.bands[0])
	key(a, p, platform.KeyE, platform.ModCtrl)
	if p.Host.Model.EqualizerVisible || p.Main.Focus() != p.body.play || p.Main.AccessibleTree().Find(p.body.bands[0].ID()) != nil {
		t.Fatal("closing the EQ wing stranded focus or left its controls accessible")
	}
	p.Main.RequestFocus(p.body.list)
	key(a, p, platform.KeyL, platform.ModCtrl)
	if p.Host.Model.PlaylistVisible || p.Main.Focus() != p.body.play || p.Main.AccessibleTree().Find(p.body.list.ID()) != nil {
		t.Fatal("closing the playlist wing stranded focus or left its controls accessible")
	}
	for range len(ring) + 2 {
		key(a, p, platform.KeyTab, 0)
		if c := p.Main.Focus(); c == nil || !c.Visible() || !p.body.controlVisible(c) {
			t.Fatal("Tab entered a hidden wing")
		}
	}
	for _, c := range []*players.GlyphButton{p.body.buttons["eq.handle"], p.body.buttons["pl.handle"]} {
		if !p.Main.AccessibleAction(c.ID(), a11y.ActionDefault) {
			t.Fatal("closed-wing handle did not reopen its controls")
		}
	}
	a.PumpOnce()
	if !p.Host.Model.EqualizerVisible || !p.Host.Model.PlaylistVisible || !p.body.volume.Visible() || !p.body.list.Visible() {
		t.Fatal("wing handles did not restore the shared visibility flags")
	}
}

func TestStartupPlacementRetainsFixedArtAndAdoptsLaterMovement(t *testing.T) {
	a, p := open(t, 1.75)
	p.Close()
	reopened, err := New(p.Host, Options{Headless: true, Scale: 1.75})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(reopened.Close)
	x, y, known := reopened.Main.Position()
	want := players.WindowPlacement{X: -120, Y: 75, Width: MainW, Height: MainH, Positioned: true}
	reopened.RestorePlacement(map[string]players.WindowPlacement{"main": {X: want.X, Y: want.Y, Width: 90, Height: 40, Positioned: true}})
	if gx, gy, gk := reopened.Main.Position(); gx != x || gy != y || gk != known {
		t.Fatal("startup restore moved before the first toolkit layout")
	}
	if got := reopened.CapturePlacement()["main"]; got != want {
		t.Fatalf("pending restore saved incorrect fixed placement: %+v", got)
	}
	a.PumpOnce()
	reopened.Advance(70 * time.Millisecond)
	if got := reopened.CapturePlacement()["main"]; got != want || reopened.restore != nil {
		t.Fatalf("startup placement was not acknowledged: %+v pending=%+v", got, reopened.restore)
	}
	reopened.Main.Move(420, 210)
	a.PumpOnce()
	reopened.Advance(70 * time.Millisecond)
	if got := reopened.CapturePlacement()["main"]; got.X != 420 || got.Y != 210 || got.Width != MainW || got.Height != MainH {
		t.Fatal("completed startup restoration fought later user placement")
	}
	reopened.RestorePlacement(map[string]players.WindowPlacement{"main": {Width: 900, Height: 500}})
	reopened.Advance(70 * time.Millisecond)
	if gx, gy, gk := reopened.Main.Position(); !gk || gx != 420 || gy != 210 {
		t.Fatal("unknown saved position became a request to move to the origin")
	}
}

func TestPlaylistHeaderAndSkinMenuExposeSharedActions(t *testing.T) {
	a, p := open(t, 1)
	opens, settings := 0, 0
	appendFiles := false
	p.Host.LoadFiles = func(_ widget.Component, appendToList bool) { opens++; appendFiles = appendToList }
	p.Host.ShowSettings = func() { settings++ }
	activate := func(button *players.GlyphButton, itemName string) {
		t.Helper()
		if !p.Main.AccessibleAction(button.ID(), a11y.ActionDefault) {
			t.Fatal("menu button unavailable")
		}
		a.PumpOnce()
		menu, ok := p.Main.Popup().(*widgets.PopupMenu)
		if !ok {
			t.Fatal("menu button did not open a toolkit popup")
		}
		for i, item := range menu.AccessibleItems() {
			if strings.Contains(item.Name, itemName) {
				if !p.Main.AccessibleAction(widget.ItemID(menu, i), a11y.ActionDefault) {
					t.Fatal("menu action refused")
				}
				a.PumpOnce()
				return
			}
		}
		t.Fatalf("menu omitted %q", itemName)
	}
	activate(p.body.buttons["list.header"], "Add music files")
	if opens != 1 || !appendFiles {
		t.Fatal("playlist header did not route Add to the shared host")
	}
	activate(p.body.buttons["list.header"], "Open music files")
	if opens != 2 || appendFiles {
		t.Fatal("playlist header did not route Open to the shared host")
	}
	activate(p.body.buttons["skin"], "Settings")
	if settings != 1 {
		t.Fatal("skin/context menu omitted the backend Settings action")
	}
}
