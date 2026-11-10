package xbox

import (
	"archive/zip"
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/gif"
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
	xbsource "github.com/codemodify/media-player-music/internal/skins/sources/xbox"
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
	t.Setenv(style.ThemeEnv, players.XboxSkin)
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

func click(w *ui.Window, button platform.MouseButton, at paint2d.Point) {
	w.Inject(platform.Event{Kind: platform.EventMouseMove, Pos: at})
	w.Inject(platform.Event{Kind: platform.EventMouseDown, Button: button, Pos: at})
	w.Inject(platform.Event{Kind: platform.EventMouseUp, Button: button, Pos: at})
}

func key(a *ui.Application, w *ui.Window, k platform.Key, mods platform.Modifiers) {
	w.Inject(platform.Event{Kind: platform.EventKeyDown, Key: k, Mods: mods})
	w.Inject(platform.Event{Kind: platform.EventKeyUp, Key: k, Mods: mods})
	a.PumpOnce()
}

func TestOriginalWindowGeometryAndShapedTransparencyAtEveryScale(t *testing.T) {
	for _, scale := range []float32{1, 1.75, 2} {
		t.Run(fmt.Sprintf("%gx", scale), func(t *testing.T) {
			_, p := open(t, scale)
			windows := p.Windows()
			if len(windows) != 2 || windows[0].Window != p.Main || windows[1].Window != p.List {
				t.Fatal("Xbox must retain its main view and separate playlist")
			}
			if p.Main.Title() != "Music Player" || p.Main.Resizable() || !p.List.Resizable() {
				t.Fatal("application title or original window sizing policy changed")
			}
			listSurface := p.List.Surface().(*platform.Offscreen)
			if listSurface.Owner() != p.Main.Surface() || listSurface.WindowRole() != platform.RoleUtility || !listSurface.SkipsTaskbar() {
				t.Fatal("playlist must use the public utility/ownership policy")
			}
			for _, window := range []struct {
				w             *ui.Window
				width, height int
			}{{p.Main, 350, 216}, {p.List, 448, 316}} {
				w, h := window.w.Size()
				if w != window.width || h != window.height {
					t.Fatalf("%q size = %dx%d, want %dx%d", window.w.Title(), w, h, window.width, window.height)
				}
				pw, ph := window.w.PixelSize()
				if pw != int(math.Round(float64(w)*float64(scale))) || ph != int(math.Round(float64(h)*float64(scale))) {
					t.Fatal("native dimensions do not follow the requested scale")
				}
				f := window.w.Surface().(*platform.Offscreen).Frame()
				if !window.w.ShapeActive() || !f.Alpha || len(f.Shape) < 10 || window.w.Shape().Raster(pw, ph).Clamped {
					t.Fatal("original silhouette degraded to an opaque rectangle")
				}
				capture := window.w.CaptureSurface()
				silhouette := window.w.Shape().Raster(pw, ph)
				for _, sample := range []struct {
					x, y   int
					inside bool
				}{{0, 0, false}, {w - 1, h - 1, false}, {w / 2, h / 2, true}} {
					x, y := int(float32(sample.x)*scale), int(float32(sample.y)*scale)
					if got := silhouette.Contains(x, y); got != sample.inside {
						t.Errorf("%q input at %d,%d = %t, want %t", window.w.Title(), sample.x, sample.y, got, sample.inside)
					}
					// A resizable silhouette deliberately adds native input along
					// its outer resize band, including some transparent pixels.
					if (window.w == p.Main || sample.inside) && platform.RectsContain(f.Shape, x, y) != sample.inside {
						t.Fatal("native input lost the fixed silhouette or visible window interior")
					}
					alpha := capture.NRGBAAt(x, y).A
					if !sample.inside && alpha != 0 || sample.inside && alpha != 255 {
						t.Errorf("%q alpha at %d,%d = %d", window.w.Title(), sample.x, sample.y, alpha)
					}
				}
				playertest.Audit(t, window.w.Title(), window.w)
			}
		})
	}
}

func originalPositionMap(t *testing.T, name string) image.Image {
	t.Helper()
	data := xbsource.Archive()
	z, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range z.File {
		if entry.Name != name {
			continue
		}
		r, err := entry.Open()
		if err != nil {
			t.Fatal(err)
		}
		im, err := gif.Decode(r)
		r.Close()
		if err != nil {
			t.Fatal(err)
		}
		return im
	}
	t.Fatalf("original archive omitted %s", name)
	return nil
}

func originalCurvePoint(t *testing.T, im image.Image, target uint8) (image.Point, uint8) {
	t.Helper()
	best, point, value := 256, image.Point{}, uint8(0)
	for y := im.Bounds().Min.Y + 1; y < im.Bounds().Max.Y-1; y++ {
		for x := im.Bounds().Min.X + 1; x < im.Bounds().Max.X-1; x++ {
			c := color.NRGBAModel.Convert(im.At(x, y)).(color.NRGBA)
			if c.A != 255 || c.R != c.G || c.R != c.B {
				continue
			}
			difference := int(c.R) - int(target)
			if difference < 0 {
				difference = -difference
			}
			if difference < best {
				best, point, value = difference, image.Pt(x, y), c.R
			}
		}
	}
	if best > 10 {
		t.Fatal("source position map lacks the expected grayscale curve")
	}
	return point, value
}

func TestCurvedSourcePositionMapsDrivePointerValuesAtEveryScale(t *testing.T) {
	for _, scale := range []float32{1, 1.75, 2} {
		t.Run(fmt.Sprintf("%gx", scale), func(t *testing.T) {
			a, p := open(t, scale)
			p.ShowEqualiser(false)
			a.PumpOnce()
			syncs := 0
			p.Host.SyncPlayback = func() { syncs++ }
			for _, item := range []struct {
				f      *curvedSlider
				source string
				reset  func()
			}{
				{p.body.seek, "seek_map.gif", func() { p.Host.Seek(0) }},
				{p.body.volume, "vol_map.gif", func() { p.Host.SetVolume(0) }},
			} {
				im := originalPositionMap(t, item.source)
				item.reset()
				p.Refresh()
				a.PumpOnce()
				for _, target := range []uint8{64, 224} {
					point, gray := originalCurvePoint(t, im, target)
					local := paint2d.Pt((float32(point.X-im.Bounds().Min.X)+.5)*scale, (float32(point.Y-im.Bounds().Min.Y)+.5)*scale)
					at := widget.DeviceOrigin(item.f).Add(local)
					if item.f.HitTest(local) != item.f {
						t.Fatal("original grayscale curve is absent from the slider hit area")
					}
					before, value := syncs, item.f.Value
					if item.f.MousePress(widget.MouseEvent{Button: platform.ButtonRight, Pos: local}) || item.f.MouseRelease(widget.MouseEvent{Button: platform.ButtonRight, Pos: local}) || item.f.Dragging() || syncs != before || item.f.Value != value {
						t.Fatal("secondary pointer input started or notified a curved-slider drag")
					}
					click(p.Main, platform.ButtonLeft, at)
					a.PumpOnce()
					want := item.f.Min + (item.f.Max-item.f.Min)*float32(gray)/255
					if syncs != before+1 || math.Abs(float64(item.f.Value-want)) > float64(item.f.Max-item.f.Min)*2/255 {
						t.Fatalf("%s source gray%d at%v gave value%g, want%g; hookdelta%d", item.source, gray, point, item.f.Value, want, syncs-before)
					}
					before, value = syncs, item.f.Value
					click(p.Main, platform.ButtonRight, at)
					a.PumpOnce()
					if item.f.Dragging() || syncs != before || item.f.Value != value {
						t.Fatal("routed secondary click changed a curved-slider value")
					}
					key(a, p.Main, platform.KeyEscape, 0)
				}
			}
		})
	}
}

func TestEqualizerInsetAndPlaylistVisibilityKeepWindowComposition(t *testing.T) {
	a, p := open(t, 1.75)
	for _, eq := range []bool{false, true} {
		for _, pl := range []bool{false, true} {
			p.ShowEqualiser(eq)
			p.ShowPlaylist(pl)
			a.PumpOnce()
			if len(p.Windows()) != 2 || p.Host.Model.EqualizerVisible != eq || p.Host.Model.PlaylistVisible != pl || p.List.Visible() != pl {
				t.Fatal("inset/satellite visibility lost shared model state or added windows")
			}
			if w, h := p.Main.Size(); w != 350 || h != 216 {
				t.Fatal("showing an inset changed the fixed primary view")
			}
		}
	}
	p.ShowPlaylist(true)
	a.PumpOnce()
	p.List.RequestClose()
	a.PumpOnce()
	if p.List.Closed() || p.List.Visible() || p.Host.Model.PlaylistVisible || p.Main.Closed() {
		t.Fatal("closing the playlist must hide its utility window and retain the player")
	}
	p.ShowPlaylist(true)
	a.PumpOnce()
	if !p.List.Visible() || !p.Host.Model.PlaylistVisible {
		t.Fatal("playlist toggle did not reopen the same satellite")
	}
}

func TestOriginalSpriteActionsAndShortcutBubblingAtEveryScale(t *testing.T) {
	for _, scale := range []float32{1, 1.75, 2} {
		t.Run(fmt.Sprintf("%gx", scale), func(t *testing.T) {
			a, p := open(t, scale)
			var commands []players.Command
			p.Host.Run = func(c players.Command) { commands = append(commands, c); p.Transport.Do(c) }
			press := func(slot string) {
				click(p.Main, platform.ButtonLeft, widget.DeviceBounds(p.body.buttons[slot]).Center())
				a.PumpOnce()
			}
			press("play")
			if p.Transport.State != players.Playing || p.body.play.Label != "Pause" {
				t.Fatal("original play sprite did not start playback")
			}
			press("play")
			if p.Transport.State != players.Paused || p.body.play.Label != "Play" {
				t.Fatal("original pause sprite did not pause playback")
			}
			p.Main.RequestFocus(p.body.play)
			key(a, p.Main, platform.KeyReturn, 0)
			press("next")
			if p.Transport.State != players.Playing || p.Transport.List.Index() != 1 {
				t.Fatal("keyboard resume or next sprite did not act on the shared queue")
			}
			if !p.Main.AccessibleAction(p.body.buttons["stop"].ID(), a11y.ActionDefault) || p.Transport.State != players.Stopped {
				t.Fatal("accessible stop did not stop playback")
			}
			want := []players.Command{players.CmdPlay, players.CmdPlayPause, players.CmdPlayPause, players.CmdNext, players.CmdStop}
			if !reflect.DeepEqual(commands, want) {
				t.Fatalf("sprite actions = %v, want %v", commands, want)
			}
			p.ShowPlaylist(true)
			p.ShowEqualiser(true)
			a.PumpOnce()
			p.List.RequestFocus(p.listPane.list)
			key(a, p.List, platform.KeySpace, 0)
			key(a, p.List, platform.KeyB, 0)
			p.Main.RequestFocus(p.body.bands[0])
			key(a, p.Main, platform.KeyZ, 0)
			p.Main.RequestFocus(p.body.seek)
			key(a, p.Main, platform.KeyV, 0)
			want = append(want, players.CmdPlayPause, players.CmdNext, players.CmdPrev, players.CmdStop)
			if !reflect.DeepEqual(commands, want) {
				t.Fatalf("original controls swallowed shared shortcuts: %v", commands)
			}
			opens := 0
			p.Host.LoadFiles = func(_ widget.Component, appendToList bool) {
				if appendToList {
					t.Fatal("main Open sprite unexpectedly appended")
				}
				opens++
			}
			press("open")
			if opens != 1 {
				t.Fatal("original Open sprite did not invoke the shared file chooser")
			}
		})
	}
}

func TestInsetEqualizerFocusLevelsAndBackendLimits(t *testing.T) {
	a, p := open(t, 1.75)
	p.ShowEqualiser(true)
	a.PumpOnce()
	if p.body.visual.Visible() || len(p.body.bands) != 10 {
		t.Fatal("original inset did not switch from visualization to ten EQ bands")
	}
	ring := playertest.FocusRing(t, a, p.Main)
	for _, c := range p.body.Children() {
		if c.Visible() && c.WantsFocus() && !playertest.Reaches(ring, c) {
			t.Fatalf("visible %T is absent from the main Tab order", c)
		}
	}
	syncs := 0
	p.Host.SyncPlayback = func() { syncs++ }
	for _, f := range []*players.Fader{p.body.seek.Fader, p.body.volume.Fader} {
		before := syncs
		if !p.Main.AccessibleAction(f.ID(), a11y.ActionIncrement) || syncs != before+1 {
			t.Fatalf("%s did not notify the shared playback hook exactly once", f.Label)
		}
	}
	if p.Transport.Pos <= 0 || p.Transport.Volume != p.body.volume.Value/100 {
		t.Fatal("source slider artwork and shared playback values disagree")
	}
	p.Refresh()
	if syncs != 2 {
		t.Fatal("model refresh generated additional slider input")
	}
	eqChanges := 0
	refresh := p.Equaliser.Changed
	p.Equaliser.Changed = func() { eqChanges++; refresh() }
	if !p.Main.AccessibleAction(p.body.bands[0].ID(), a11y.ActionIncrement) || eqChanges != 1 || p.Equaliser.Gains[0] <= 0 {
		t.Fatal("inset EQ did not edit the shared equalizer")
	}
	gain := p.Equaliser.Gains[0]
	p.Main.RequestFocus(p.body.bands[0])
	key(a, p.Main, platform.KeyE, platform.ModCtrl)
	if p.Host.Model.EqualizerVisible || !p.body.visual.Visible() || p.Main.Focus() != p.body.play || p.Main.AccessibleTree().Find(p.body.bands[0].ID()) != nil {
		t.Fatal("closing the inset stranded focus or retained hidden EQ controls")
	}
	key(a, p.Main, platform.KeyE, platform.ModCtrl)
	p.Host.Model.Live, p.Host.Model.BackendLimited = true, true
	p.Host.AudioBackend = "VLC"
	p.Refresh()
	a.PumpOnce()
	for _, c := range p.body.eqControls {
		if !c.Visible() || c.Enabled() || p.Main.AccessibleAction(c.ID(), a11y.ActionIncrement) || p.Main.AccessibleAction(c.ID(), a11y.ActionDefault) {
			t.Fatal("unsupported equalizer remained usable")
		}
	}
	if !p.body.volume.Enabled() || !strings.Contains(p.body.bands[0].Tooltip(), "VLC") || !strings.Contains(p.Host.StatusText(), "visualizer") {
		t.Fatal("backend limits disabled basic volume or hid the limitation")
	}
	p.Host.Model.BackendEqualizer, p.Host.Model.BackendSpectrum, p.Host.Model.BackendBalance = true, true, true
	p.Host.AudioBackend = "mpv"
	p.Refresh()
	for _, c := range p.body.eqControls {
		if !c.Enabled() {
			t.Fatal("capable backend did not restore inset controls")
		}
	}
	if p.Equaliser.Gains[0] != gain {
		t.Fatal("backend selection erased saved EQ adjustments")
	}
}

func TestBothViewsRouteDecodedFileAndFolderDropsWithoutMovingSources(t *testing.T) {
	a, p := open(t, 1.75)
	p.ShowPlaylist(true)
	p.ShowEqualiser(false)
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
	for _, target := range []struct {
		w  *ui.Window
		at paint2d.Point
	}{{p.Main, widget.DeviceBounds(p.body.visual).Center()}, {p.List, p.listPane.list.AccessibleItems()[0].Bounds.Center()}} {
		off := target.w.Surface().(*platform.Offscreen)
		before := calls
		off.SimulateDropAction(target.at, map[string][]byte{"text/uri-list": data}, platform.DragCopy|platform.DragMove, platform.DragMove)
		a.PumpOnce()
		if accepted := off.DropTaken(); accepted == nil || !*accepted || calls != before+1 || !reflect.DeepEqual(got, want) {
			t.Fatalf("%q did not deliver decoded file/folder paths once", target.w.Title())
		}
		if p.Transport.State != players.Paused || p.Transport.Pos != 17*time.Second || p.Transport.List.Index() != 0 {
			t.Fatal("accepting a drop changed playback before the shared loader ran")
		}
		for _, payload := range []struct {
			data               []byte
			offered, preferred platform.DragAction
		}{
			{[]byte("file://remote-host/tmp/music.mp3\r\n"), platform.DragCopy, platform.DragCopy},
			{[]byte("https://example.com/music.mp3\r\n"), platform.DragCopy, platform.DragCopy},
			{data, platform.DragMove, platform.DragMove},
		} {
			before := calls
			off.SimulateDropAction(target.at, map[string][]byte{"text/uri-list": payload.data}, payload.offered, payload.preferred)
			a.PumpOnce()
			if accepted := off.DropTaken(); accepted == nil || *accepted || calls != before {
				t.Fatal("unsupported remote or Move-only drop was accepted")
			}
		}
	}
}

func TestPlaylistResizeSelectionActivationAndInterruptedGestures(t *testing.T) {
	for _, scale := range []float32{1, 1.75, 2} {
		t.Run(fmt.Sprintf("%gx", scale), func(t *testing.T) {
			a, p := open(t, scale)
			p.ShowPlaylist(true)
			list := p.listPane.list
			for _, size := range [][2]int{{438, 316}, {620, 420}} {
				p.List.SetSize(size[0], size[1])
				a.PumpOnce()
				if w, h := p.List.Size(); w != size[0] || h != size[1] {
					t.Fatal("playlist resize was not acknowledged")
				}
				if w, h := p.Main.Size(); w != 350 || h != 216 {
					t.Fatal("playlist resize changed the fixed main view")
				}
				for _, c := range p.listPane.Children() {
					b, parent := c.Bounds(), p.listPane.LocalBounds()
					if c.Visible() && (b.Empty() || b.Min.X < 0 || b.Min.Y < 0 || b.Max.X > parent.Max.X || b.Max.Y > parent.Max.Y) {
						t.Fatalf("resized playlist clipped %T: %v", c, b)
					}
				}
				if !p.List.ShapeActive() || p.List.CaptureSurface().NRGBAAt(0, 0).A != 0 {
					t.Fatal("resizing lost the playlist silhouette")
				}
				playertest.Audit(t, "resized playlist", p.List)
			}
			plays := 0
			p.Host.Run = func(c players.Command) {
				if c == players.CmdPlay {
					plays++
				}
				p.Transport.Do(c)
			}
			p.Transport.State, p.Transport.Pos = players.Paused, 17*time.Second
			at := list.AccessibleItems()[2].Bounds.Center()
			click(p.List, platform.ButtonLeft, at)
			a.PumpOnce()
			p.Refresh()
			a.PumpOnce()
			if list.Selected != 2 || p.Transport.List.Index() != 0 || p.Transport.State != players.Paused || p.Transport.Pos != 17*time.Second || plays != 0 {
				t.Fatal("browsing a row changed playback or refresh erased the selection")
			}
			key(a, p.List, platform.KeyDown, 0)
			if list.Selected != 3 || p.Transport.List.Index() != 0 || plays != 0 {
				t.Fatal("arrow selection started playback")
			}
			key(a, p.List, platform.KeyReturn, 0)
			if p.Transport.List.Index() != 3 || p.Transport.State != players.Playing || plays != 1 {
				t.Fatal("Enter failed to play the selected row once")
			}
			p.Transport.State = players.Paused
			if !p.List.AccessibleAction(widget.ItemID(list, 3), a11y.ActionDefault) || p.Transport.State != players.Playing || plays != 2 {
				t.Fatal("accessible row activation did not resume playback")
			}
			click(p.List, platform.ButtonLeft, at)
			click(p.List, platform.ButtonLeft, at)
			a.PumpOnce()
			if p.Transport.List.Index() != 2 || plays != 3 {
				t.Fatal("double-click did not play one track exactly once")
			}
			for _, interruption := range []string{"pointer-exit", "focus-change", "wheel-at-end"} {
				p.Transport.List.Select(0)
				p.Transport.State, p.Transport.Pos = players.Paused, 17*time.Second
				p.Refresh()
				p.List.Inject(platform.Event{Kind: platform.EventMouseMove, Pos: widget.DeviceBounds(p.listPane.buttons["shuffle"]).Center()})
				row := 2
				if interruption == "wheel-at-end" {
					list.ScrollTo(list.MaxOffset())
					row = list.Count - 1
				} else {
					list.ScrollTo(0)
				}
				at = list.AccessibleItems()[row].Bounds.Center()
				before := plays
				click(p.List, platform.ButtonLeft, at)
				switch interruption {
				case "pointer-exit":
					p.List.Inject(platform.Event{Kind: platform.EventMouseMove, Pos: widget.DeviceBounds(p.listPane.buttons["shuffle"]).Center()})
				case "focus-change":
					p.List.Inject(platform.Event{Kind: platform.EventKeyDown, Key: platform.KeyTab})
				case "wheel-at-end":
					p.List.Inject(platform.Event{Kind: platform.EventScroll, Pos: at, Scroll: paint2d.Pt(0, 1)})
				}
				click(p.List, platform.ButtonLeft, at)
				a.PumpOnce()
				if list.Selected != row || p.Transport.List.Index() != 0 || p.Transport.State != players.Paused || plays != before {
					t.Fatalf("%s combined interrupted clicks into playback", interruption)
				}
			}
			selected := list.Selected
			list.SetEnabled(false)
			at = list.AccessibleItems()[2].Bounds.Center()
			for _, button := range []platform.MouseButton{platform.ButtonLeft, platform.ButtonRight} {
				before := plays
				click(p.List, button, at)
				a.PumpOnce()
				if list.Selected != selected || plays != before || p.List.Popup() != nil {
					t.Fatal("disabled playlist selected, activated or opened a context menu")
				}
			}
		})
	}
}

func TestPlacementRoundTripRetainsFixedMainAndResizedHiddenPlaylist(t *testing.T) {
	a, p := open(t, 1.75)
	p.Main.Move(-120, 75)
	p.List.Move(620, 260)
	p.List.SetSize(620, 420)
	a.PumpOnce()
	p.ShowPlaylist(false)
	saved := p.CapturePlacement()
	p.Close()
	q, err := New(p.Host, Options{Headless: true, Scale: 1.75})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(q.Close)
	// Historical sessions can contain dimensions belonging to another skin.
	wrong := saved["main"]
	wrong.Width, wrong.Height = 900, 700
	saved["main"] = wrong
	q.RestorePlacement(saved)
	want := make(map[string]players.WindowPlacement, len(saved))
	for k, v := range saved {
		want[k] = v
	}
	main := want["main"]
	main.Width, main.Height = 350, 216
	want["main"] = main
	if got := q.CapturePlacement(); !reflect.DeepEqual(got, want) {
		t.Fatalf("pending restore lost saved geometry: %+v", got)
	}
	a.PumpOnce()
	q.Advance(70 * time.Millisecond)
	// A hidden utility must retain its saved geometry even if its first
	// native layout occurs only when the user shows it later.
	q.Advance(3 * time.Second)
	if got := q.CapturePlacement(); !reflect.DeepEqual(got, want) || q.List.Visible() {
		t.Fatalf("hidden playlist restore lost geometry: got%+v want%+v", got, want)
	}
	q.ShowPlaylist(true)
	a.PumpOnce()
	q.Advance(70 * time.Millisecond)
	if got := q.CapturePlacement(); !reflect.DeepEqual(got, want) || len(q.restore) != 0 {
		t.Fatalf("reopened geometry not acknowledged: %+v pending%+v", got, q.restore)
	}
	q.Main.Move(420, 210)
	q.List.Move(840, 510)
	a.PumpOnce()
	q.Advance(70 * time.Millisecond)
	got := q.CapturePlacement()
	if got["main"].X != 420 || got["main"].Y != 210 || got["playlist"].X != 840 || got["playlist"].Y != 510 {
		t.Fatal("completed restore fought later user movement")
	}
}

func TestPlaylistContextMenuDispatchesSharedActions(t *testing.T) {
	a, p := open(t, 1.75)
	p.ShowPlaylist(true)
	p.Transport.State, p.Transport.Pos = players.Paused, 17*time.Second
	list := p.listPane.list
	list.Selected = 2
	opens, adds, removes, settings := 0, 0, 0, 0
	p.Host.LoadFiles = func(_ widget.Component, appendToList bool) {
		if appendToList {
			adds++
		} else {
			opens++
		}
	}
	p.Host.DeleteTrack = func() {
		if p.Transport.List.Index() != 0 {
			t.Fatal("current-track removal was silently redirected to the browsed row")
		}
		removes++
	}
	p.Host.ShowSettings = func() { settings++ }
	for _, name := range []string{"Open music files", "Add music files", "Remove current track", "Settings"} {
		click(p.List, platform.ButtonRight, list.AccessibleItems()[3].Bounds.Center())
		a.PumpOnce()
		menu, ok := p.List.Popup().(*widgets.PopupMenu)
		if !ok {
			t.Fatal("playlist right-click did not open its queue actions")
		}
		if list.Selected != 2 || p.Transport.List.Index() != 0 || p.Transport.State != players.Paused || p.Transport.Pos != 17*time.Second {
			t.Fatal("opening queue actions changed selection or playback")
		}
		found := false
		for i, item := range menu.AccessibleItems() {
			if strings.Contains(item.Name, name) {
				if !p.List.AccessibleAction(widget.ItemID(menu, i), a11y.ActionDefault) {
					t.Fatalf("%s refused accessible activation", name)
				}
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("playlist omitted %s", name)
		}
		a.PumpOnce()
	}
	if opens != 1 || adds != 1 || removes != 1 || settings != 1 {
		t.Fatalf("queue menu hooks: open%d add%d remove%d settings%d", opens, adds, removes, settings)
	}
	p.List.RequestFocus(list)
	key(a, p.List, platform.KeyMenu, 0)
	menu, ok := p.List.Popup().(*widgets.PopupMenu)
	if !ok {
		t.Fatal("keyboard Menu did not expose the playlist queue actions")
	}
	for _, item := range menu.AccessibleItems() {
		if strings.Contains(item.Name, "Add music files") {
			return
		}
	}
	t.Fatal("keyboard Menu opened appearance-only choices instead of queue actions")
}
