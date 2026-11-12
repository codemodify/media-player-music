package fancy

import (
	"fmt"
	"math"
	"net/url"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/codemodify/media-player-music/internal/players"
	"github.com/codemodify/media-player-music/internal/players/playertest"
	"goframework.io/desktop-ui"
	"goframework.io/desktop-ui-skins"
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
	t.Setenv(style.ThemeEnv, players.FancySkin)
	a := ui.New(ui.Options{Headless: true, Scale: scale, DisableLookWatch: true})
	h := players.NewHost(a)
	h.Packs = players.SkinnedIDs
	p, err := New(h, Options{Headless: true, Scale: scale})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(p.Close)
	a.PumpOnce()
	p.Advance(0)
	a.PumpOnce()
	return a, p
}
func click(w *ui.Window, at paint2d.Point, button platform.MouseButton) {
	w.Inject(platform.Event{Kind: platform.EventMouseMove, Pos: at})
	w.Inject(platform.Event{Kind: platform.EventMouseDown, Pos: at, Button: button})
	w.Inject(platform.Event{Kind: platform.EventMouseUp, Pos: at, Button: button})
}
func key(a *ui.Application, w *ui.Window, k platform.Key, mods platform.Modifiers) {
	w.Inject(platform.Event{Kind: platform.EventKeyDown, Key: k, Mods: mods})
	w.Inject(platform.Event{Kind: platform.EventKeyUp, Key: k, Mods: mods})
	a.PumpOnce()
}

func TestArtworkUtilityWindowsAndCompactIdentity(t *testing.T) {
	for _, scale := range []float32{1, 1.75, 2} {
		t.Run(fmt.Sprintf("%gx", scale), func(t *testing.T) {
			a, p := open(t, scale)
			if p.Main().Title() != "Music Player" || len(p.Windows()) != 3 || p.Main().Resizable() || p.Eq().Resizable() || !p.List().Resizable() {
				t.Fatal("three-window composition changed")
			}
			for _, w := range []*ui.Window{p.Main(), p.Eq(), p.List()} {
				pw, ph := w.PixelSize()
				if !w.ShapeActive() || w.Shape().Raster(pw, ph).Clamped || w.CaptureSurface().NRGBAAt(0, 0).A != 0 {
					t.Fatal("shaped chrome or transparent corners missing")
				}
				playertest.Audit(t, w.Title(), w)
			}
			for _, w := range []*ui.Window{p.Eq(), p.List()} {
				s := w.Surface().(*platform.Offscreen)
				if s.WindowRole() != platform.RoleUtility || s.Owner() != p.Main().Surface() || !s.SkipsTaskbar() {
					t.Fatal("satellite lost utility ownership")
				}
			}
			p.Transport.State, p.Transport.Pos = players.Paused, 17*time.Second
			p.Equaliser.Set(3, 4)
			main, queue := p.Main(), p.Transport.List
			for _, compact := range []bool{true, false, true, false} {
				p.SetCompact(compact)
				a.PumpOnce()
				p.Advance(0)
				a.PumpOnce()
				height := MainH
				pane := p.body
				if compact {
					height = CompactH
					pane = p.compact
				}
				if w, h := p.Main().Size(); w != MainW || h != height {
					t.Fatalf("mode size %dx%d", w, h)
				}
				if p.Main() != main || p.Main().Resizable() || p.Main().Focus() != pane.play || p.Main().Content() != pane {
					t.Fatal("mode swap replaced Main or retained old focus")
				}
				if p.Transport.List != queue || p.Transport.State != players.Paused || p.Transport.Pos != 17*time.Second || p.Equaliser.Gains[3] != 4 {
					t.Fatal("mode swap altered audio model")
				}
				if !p.Eq().Visible() || !p.List().Visible() {
					t.Fatal("compact hid remembered panels")
				}
				if !p.Main().Surface().(*platform.Offscreen).Frame().Alpha {
					t.Fatal("mode swap lost shaped surface")
				}
				playertest.Audit(t, "Main mode", p.Main())
			}
			p.RestoreState(players.PresentationState{Mode: "unknown"})
			if p.Compact() {
				t.Fatal("invalid persisted mode did not select full")
			}
		})
	}
}

func TestSoftEdgesPaintOnceAndUnshapedDesktopStatesRemainOpaque(t *testing.T) {
	for _, scale := range []float32{1, 1.75} {
		t.Run(fmt.Sprintf("%gx", scale), func(t *testing.T) {
			a, p := open(t, scale)
			for _, item := range []struct {
				w    *ui.Window
				role string
				pane widget.Component
			}{{p.Main(), LayoutMain, p.body}, {p.Eq(), LayoutEqualizer, p.eqPane}, {p.List(), LayoutPlaylist, p.listPane}} {
				pw, ph := item.w.PixelSize()
				raw := paint2d.NewImage(pw, ph)
				ctx := paint2d.NewContext(raw)
				ctx.Clear(paint2d.Transparent)
				box := paint2d.XYWH(0, 0, float32(pw), float32(ph))
				slot := "shape." + item.role[len("fancy."):]
				r, ok := skins.SkinSlotRect(item.w.Look(), item.role, slot, box)
				if !ok || !skins.DrawSkinSlot(item.w.Look(), ctx, r, item.role, slot, 0) {
					t.Fatal("source chrome unavailable")
				}
				capture := item.w.CaptureSurface()
				checked, opaque := 0, 0
				for y := 0; y < ph; y++ {
					for x := 0; x < pw; x++ {
						alpha := raw.Pix[(y*pw+x)*4+3]
						if alpha == 0 {
							continue
						}
						at := paint2d.Pt(float32(x)+.5, float32(y)+.5)
						covered := false
						for _, c := range item.pane.Children() {
							if c.Visible() && widget.DeviceBounds(c).Contains(at) {
								covered = true
								break
							}
						}
						// Passive readouts cannot reach the outer antialiased edge.
						if covered || x > int(10*scale) && y > int(25*scale) && x < pw-int(10*scale) && y < ph-int(10*scale) {
							continue
						}
						got := capture.NRGBAAt(x, y).A
						if math.Abs(float64(int(got)-int(alpha))) > 1 {
							t.Fatalf("%s edge%d,%d alpha%d != art%d", item.role, x, y, got, alpha)
						}
						if alpha < 255 {
							checked++
						} else {
							opaque++
						}
					}
				}
				if checked < 10 || opaque < 100 {
					t.Fatal("shape chrome lacks visible soft edges")
				}
			}
			p.List().Surface().(*platform.Offscreen).SimulateWindowState(platform.WindowState{Maximized: true})
			a.PumpOnce()
			if p.List().ShapeActive() || p.List().CaptureSurface().NRGBAAt(0, 0).A != 255 {
				t.Fatal("desktop removed silhouette but rectangular fallback retained holes")
			}
			p.List().Surface().(*platform.Offscreen).SimulateWindowState(platform.WindowState{})
			a.PumpOnce()
			if !p.List().ShapeActive() || p.List().CaptureSurface().NRGBAAt(0, 0).A != 0 {
				t.Fatal("restoring shape left opaque fallback corners")
			}
		})
	}
}

func TestTransportControlsHooksCircularHitAndCloseRequests(t *testing.T) {
	for _, scale := range []float32{1, 1.75} {
		t.Run(fmt.Sprintf("%gx", scale), func(t *testing.T) {
			a, p := open(t, scale)
			var commands []players.Command
			p.Host.Run = func(c players.Command) { commands = append(commands, c); p.Transport.Do(c) }
			press := func(slot string) {
				click(p.Main(), widget.DeviceBounds(p.body.buttons[slot]).Center(), platform.ButtonLeft)
				a.PumpOnce()
			}
			// An empty corner of the circular Play bounding box must not play.
			r := widget.DeviceBounds(p.body.play)
			click(p.Main(), r.Min.Add(paint2d.Pt(2*scale, 2*scale)), platform.ButtonLeft)
			a.PumpOnce()
			if len(commands) != 0 {
				t.Fatal("transparent Play corner activated")
			}
			press("play")
			press("play")
			p.Main().RequestFocus(p.body.play)
			key(a, p.Main(), platform.KeyReturn, 0)
			press("next")
			press("stop")
			want := []players.Command{players.CmdPlay, players.CmdPlayPause, players.CmdPlayPause, players.CmdNext, players.CmdStop}
			if !reflect.DeepEqual(commands, want) {
				t.Fatalf("actions not exactly once: %v", commands)
			}
			opens, adds, settings := 0, 0, 0
			p.Host.LoadFiles = func(_ widget.Component, append bool) {
				if append {
					adds++
				} else {
					opens++
				}
			}
			p.Host.ShowSettings = func() { settings++ }
			press("open")
			press("settings")
			for _, slot := range []string{"open", "add"} {
				if !p.List().AccessibleAction(p.listPane.buttons[slot].ID(), a11y.ActionDefault) {
					t.Fatal("playlist file action inaccessible")
				}
			}
			if opens != 2 || adds != 1 || settings != 1 {
				t.Fatalf("hooks open%d/add%d/settings%d", opens, adds, settings)
			}
			closes := 0
			p.Main().SetOnCloseRequest(func() bool { closes++; return false })
			press("close")
			a.PumpOnce()
			if closes != 1 || p.Main().Closed() {
				t.Fatal("Main bypassed close-to-tray request")
			}
			p.Eq().RequestClose()
			p.List().RequestClose()
			a.PumpOnce()
			if p.Eq().Closed() || p.List().Closed() || p.Host.Model.EqualizerVisible || p.Host.Model.PlaylistVisible {
				t.Fatal("satellite Close did not hide shared panels")
			}
		})
	}
}

func TestPrimarySliderInputCapabilitiesAndMeasuredAnalyzer(t *testing.T) {
	a, p := open(t, 1.75)
	syncs := 0
	p.Host.SyncPlayback = func() { syncs++ }
	for _, s := range []*fancySlider{p.body.seek, p.body.volume, p.body.balance} {
		before := syncs
		if !p.Main().AccessibleAction(s.ID(), a11y.ActionIncrement) || syncs != before+1 {
			t.Fatal("slider did not notify audio exactly once")
		}
		v := s.Value
		if s.MousePress(widget.MouseEvent{Button: platform.ButtonRight, Pos: s.LocalBounds().Center()}) || s.Dragging() || s.Value != v || syncs != before+1 {
			t.Fatal("secondary slider press changed audio")
		}
	}
	before := syncs
	p.Refresh()
	if syncs != before {
		t.Fatal("refresh emitted slider input")
	}
	// Pointer movement around the knob uses a monotonic 270-degree arc.
	s := p.body.volume
	for _, fraction := range []float32{0, .5, 1} {
		angle := float64(135+fraction*270) * math.Pi / 180
		r := widget.DeviceBounds(s)
		at := r.Center().Add(paint2d.Pt(float32(math.Cos(angle))*r.Dx()*.3, float32(math.Sin(angle))*r.Dy()*.3))
		click(p.Main(), at, platform.ButtonLeft)
		a.PumpOnce()
		if math.Abs(float64(p.Transport.Volume-fraction)) > .025 || s.Dragging() {
			t.Fatalf("knob fraction%g -> volume%g", fraction, p.Transport.Volume)
		}
	}
	changes := 0
	refresh := p.Equaliser.Changed
	p.Equaliser.Changed = func() { changes++; refresh() }
	for _, s := range []*fancySlider{p.eqPane.preamp, p.eqPane.bands[2]} {
		if !p.Eq().AccessibleAction(s.ID(), a11y.ActionIncrement) {
			t.Fatal("EQ inaccessible")
		}
	}
	if changes != 2 {
		t.Fatal("EQ actions duplicated")
	}
	p.Spectrum.Live = true
	p.Spectrum.SetLevels([]float32{.8, 1, .6, .9})
	p.Refresh()
	a.PumpOnce()
	active := p.Main().CaptureSurface()
	p.Spectrum.Reset()
	p.Refresh()
	a.PumpOnce()
	silent := p.Main().CaptureSurface()
	r := widget.DeviceBounds(p.body.analyser)
	changed := 0
	for y := int(r.Min.Y); y < int(r.Max.Y); y++ {
		for x := int(r.Min.X); x < int(r.Max.X); x++ {
			if active.NRGBAAt(x, y) != silent.NRGBAAt(x, y) {
				changed++
			}
		}
	}
	if changed < 100 || !p.body.balance.Enabled() || !p.eqPane.bands[2].Enabled() {
		t.Fatal("measured analyzer/capability restoration failed")
	}
}

func TestPlaylistBrowsingActivationDropAndClear(t *testing.T) {
	for _, scale := range []float32{1, 1.75} {
		t.Run(fmt.Sprintf("%gx", scale), func(t *testing.T) {
			a, p := open(t, scale)
			p.Transport.State, p.Transport.Pos = players.Paused, 17*time.Second
			plays := 0
			p.Host.Run = func(c players.Command) {
				if c == players.CmdPlay {
					plays++
				}
				p.Transport.Do(c)
			}
			q := p.listPane.list
			at := q.AccessibleItems()[1].Bounds.Center()
			click(p.List(), at, platform.ButtonLeft)
			a.PumpOnce()
			p.Advance(70 * time.Millisecond)
			if plays != 0 || q.Selected != 1 || p.Transport.List.Index() != 0 || p.Transport.Pos != 17*time.Second {
				t.Fatal("single browse altered playback")
			}
			click(p.List(), at, platform.ButtonLeft)
			a.PumpOnce()
			if plays != 1 || p.Transport.List.Index() != 1 || p.Transport.State != players.Playing {
				t.Fatal("double-click did not activate once")
			}
			key(a, p.List(), platform.KeyDown, 0)
			if plays != 1 {
				t.Fatal("navigation started playback")
			}
			key(a, p.List(), platform.KeyReturn, 0)
			if plays != 2 {
				t.Fatal("Enter did not activate once")
			}
			if !p.List().AccessibleAction(widget.ItemID(q, 0), a11y.ActionDefault) || plays != 3 {
				t.Fatal("accessible row activation failed")
			}
			before := plays
			q.SetEnabled(false)
			click(p.List(), at, platform.ButtonLeft)
			click(p.List(), at, platform.ButtonLeft)
			a.PumpOnce()
			if plays != before {
				t.Fatal("disabled queue activated")
			}
			q.SetEnabled(true)
			uri := func(path string) string { return (&url.URL{Scheme: "file", Path: path}).String() }
			data := []byte(uri("/tmp/fancy song.mp3") + "\r\n" + uri("/tmp/fancy music") + "\r\n")
			calls := 0
			p.Host.DropFiles = func(paths []string) {
				calls++
				if !reflect.DeepEqual(paths, []string{"/tmp/fancy song.mp3", "/tmp/fancy music"}) {
					t.Fatal("drop paths lost")
				}
			}
			header, _ := skins.SkinSlotRect(p.List().Look(), LayoutPlaylist, "header", p.listPane.LocalBounds())
			off := p.List().Surface().(*platform.Offscreen)
			for _, pos := range []paint2d.Point{q.AccessibleItems()[0].Bounds.Center(), header.Center()} {
				before := calls
				off.SimulateDropAction(pos, map[string][]byte{"text/uri-list": data}, platform.DragCopy|platform.DragMove, platform.DragMove)
				a.PumpOnce()
				if taken := off.DropTaken(); taken == nil || !*taken || calls != before+1 {
					t.Fatal("playlist row/header failed local Copy drop")
				}
				off.SimulateDropAction(pos, map[string][]byte{"text/uri-list": []byte("file://remote/tmp/song.mp3\r\n")}, platform.DragCopy, platform.DragCopy)
				a.PumpOnce()
				if taken := off.DropTaken(); taken == nil || *taken || calls != before+1 {
					t.Fatal("remote URI accepted")
				}
			}
			queue := p.Transport.List
			stops := 0
			p.Host.Run = func(c players.Command) {
				if c != players.CmdStop || queue.Len() != 0 {
					t.Fatal("clear did not submit Stop after queue change")
				}
				stops++
				p.Transport.Do(c)
			}
			p.clearPlaylist()
			a.PumpOnce()
			if p.Transport.List != queue || queue.Len() != 0 || q.Count != 0 || stops != 1 || p.Transport.State != players.Stopped {
				t.Fatal("Clear replaced queue or retained playback")
			}
			off.SimulateDropAction(widget.DeviceBounds(q).Center(), map[string][]byte{"text/uri-list": data}, platform.DragCopy, platform.DragCopy)
			a.PumpOnce()
			if taken := off.DropTaken(); taken == nil || !*taken {
				t.Fatal("empty playlist refused file/folder drop")
			}
		})
	}
}

func TestGroupCompactReflowDetachedRestoreAndMalformedDock(t *testing.T) {
	a, p := open(t, 1.75)
	playertest.SettleGroup(t, a, p.g)
	p.Main().Surface().(*platform.Offscreen).SimulateDesktopMove(420, 250)
	playertest.Follow(a)
	main, eq, list := p.g.Rack.Pane(p.iMain), p.g.Rack.Pane(p.iEq), p.g.Rack.Pane(p.iList)
	if eq.Box.Y != main.Box.Bottom() || list.Box.Y != eq.Box.Bottom() {
		t.Fatal("default vertical stack lost")
	}
	p.SetCompact(true)
	a.PumpOnce()
	p.Advance(0)
	if eq.Box.Y != main.Box.Y+CompactH || list.Box.Y != eq.Box.Bottom() {
		t.Fatal("compact failed to reflow attached satellites")
	}
	p.List().Surface().(*platform.Offscreen).SimulateDesktopMove(1400, 800)
	playertest.Follow(a)
	p.List().SetSize(610, 230)
	playertest.Follow(a)
	before := list.Box
	p.SetCompact(false)
	a.PumpOnce()
	p.Advance(0)
	if list.Box != before {
		t.Fatal("mode switch moved detached playlist")
	}
	p.ShowEqualiser(false)
	saved := p.CapturePlacement()
	p.Close()
	a, reopened := open(t, 1.75)
	reopened.ShowEqualiser(false)
	reopened.RestoreState(players.PresentationState{Mode: "compact"})
	reopened.RestorePlacement(saved)
	playertest.SettleGroup(t, a, reopened.g)
	if w, h := reopened.List().Size(); w != 610 || h != 230 {
		t.Fatalf("playlist size not restored: %dx%d", w, h)
	}
	if x, y, ok := reopened.List().Position(); !ok || x != before.X || y != before.Y {
		t.Fatal("detached placement not restored")
	}
	if reopened.Eq().Visible() || reopened.g.Rack.Pane(reopened.iList).To >= 0 || !reopened.Compact() {
		t.Fatal("restored state or detached/hidden policy lost")
	}
	malformed := map[string]players.WindowPlacement{"equaliser": {Dock: &players.WindowDock{Parent: "playlist", Side: "bottom"}}, "playlist": {Width: 1, Height: 1, Dock: &players.WindowDock{Parent: "equaliser", Side: "bottom"}}}
	reopened.RestorePlacement(malformed)
	playertest.Follow(a)
	for i := 0; i < reopened.g.Rack.Len(); i++ {
		seen := map[int]bool{}
		for at := i; at >= 0; at = reopened.g.Rack.Pane(at).To {
			if seen[at] {
				t.Fatal("persisted cycle survived")
			}
			seen[at] = true
		}
	}
	if w, h := reopened.List().Size(); w < MinListW || h < MinListH {
		t.Fatal("invalid persisted size bypassed minimum")
	}
}
