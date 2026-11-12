package halflife2

import (
	"archive/zip"
	"bytes"
	"fmt"
	"image"
	"image/color"
	_ "image/png"
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
	source "github.com/codemodify/media-player-music/internal/skins/sources/half-life-2"
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
	t.Setenv(style.ThemeEnv, players.HalfLife2Skin)
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

func TestOriginalThreeWindowGeometryAndMasksAtEveryScale(t *testing.T) {
	for _, scale := range []float32{1, 1.75, 2} {
		t.Run(fmt.Sprintf("%gx", scale), func(t *testing.T) {
			a, p := open(t, scale)
			if len(p.Windows()) != 3 || p.Main().Title() != "Music Player" || p.Main().Resizable() || p.Eq().Resizable() || !p.List().Resizable() {
				t.Fatal("source window composition or sizing policy changed")
			}
			for _, window := range []struct {
				w      *ui.Window
				width  int
				height int
			}{{p.Main(), 339, 300}, {p.Eq(), 430, 239}, {p.List(), 393, 226}} {
				w, h := window.w.Size()
				if w != window.width || h != window.height {
					t.Fatalf("%q geometry %dx%d", window.w.Title(), w, h)
				}
				pw, ph := window.w.PixelSize()
				if pw != int(math.Round(float64(w)*float64(scale))) || ph != int(math.Round(float64(h)*float64(scale))) {
					t.Fatal("device geometry does not follow scale")
				}
				frame := window.w.Surface().(*platform.Offscreen).Frame()
				if !window.w.ShapeActive() || !frame.Alpha || len(frame.Shape) == 0 {
					t.Fatalf("%q shape fell back to an opaque rectangle", window.w.Title())
				}
				shape, capture := window.w.Shape().Raster(pw, ph), window.w.CaptureSurface()
				if shape.Clamped || shape.Contains(0, 0) || capture.NRGBAAt(0, 0).A != 0 {
					t.Fatalf("%q empty corner is painted or accepts input", window.w.Title())
				}
				if !shape.Contains(pw/2, ph/2) || capture.NRGBAAt(pw/2, ph/2).A == 0 {
					t.Fatalf("%q center is missing from its image/input mask", window.w.Title())
				}
				if window.w != p.Main() {
					off := window.w.Surface().(*platform.Offscreen)
					if off.WindowRole() != platform.RoleUtility || off.Owner() != p.Main().Surface() || !off.SkipsTaskbar() {
						t.Fatal("satellite lacks public utility ownership/taskbar policy")
					}
				}
				playertest.Audit(t, window.w.Title(), window.w)
			}
			for state := 0; state < 3; state++ {
				if p.body.shutter != state {
					t.Fatal("source shutter sequence changed")
				}
				if p.Main().CaptureSurface().NRGBAAt(0, 0).A != 0 || !p.Main().ShapeActive() {
					t.Fatal("shutter changed the transparent primary outline")
				}
				// In the original artwork the opened outer shutter leaves
				// this frame tip empty. A union of all shutter masks would
				// still accept input here even though nothing is painted.
				x, y := int(209*scale), int(3*scale)
				pw, ph := p.Main().PixelSize()
				inside := p.Main().Shape().Raster(pw, ph).Contains(x, y)
				alpha := p.Main().CaptureSurface().NRGBAAt(x, y).A
				if state == 0 && (inside || alpha != 0) || state != 0 && (!inside || alpha == 0) {
					t.Fatalf("shutter%d frame tip input=%t alpha=%d does not follow its visible artwork", state, inside, alpha)
				}
				p.Main().RequestFocus(p.body.seek)
				p.body.cycleShutter()
				a.PumpOnce()
				if p.body.shutter != 0 && p.Main().Focus() != p.body.buttons["shutter"] {
					t.Fatal("closing a shutter left keyboard focus on a hidden inner control")
				}
				for _, c := range p.body.innerControls {
					if p.body.shutter != 0 && c.Visible() {
						t.Fatal("closed shutter left inner controls exposed")
					}
				}
			}
		})
	}
}

func TestOriginalTransportSpritesAndCrossWindowShortcuts(t *testing.T) {
	for _, scale := range []float32{1, 1.75, 2} {
		t.Run(fmt.Sprintf("%gx", scale), func(t *testing.T) {
			a, p := open(t, scale)
			var commands []players.Command
			p.Host.Run = func(c players.Command) { commands = append(commands, c); p.Transport.Do(c) }
			press := func(slot string) {
				click(p.Main(), platform.ButtonLeft, widget.DeviceBounds(p.body.buttons[slot]).Center())
				a.PumpOnce()
			}
			press("play")
			if p.Transport.State != players.Playing || p.body.play.Visible() || !p.body.buttons["pause"].Visible() {
				t.Fatal("original Play sprite did not start and expose Pause")
			}
			if p.Main().Focus() != nil && !p.Main().Focus().Visible() {
				t.Fatal("Play left focus on its hidden sprite")
			}
			press("pause")
			if p.Transport.State != players.Paused || !p.body.play.Visible() {
				t.Fatal("original Pause sprite did not pause")
			}
			p.Main().RequestFocus(p.body.play)
			key(a, p.Main(), platform.KeyReturn, 0)
			press("next")
			if p.Transport.State != players.Playing || p.Transport.List.Index() != 1 {
				t.Fatal("keyboard Play/next failed to use the shared queue")
			}
			if !p.Main().AccessibleAction(p.body.buttons["stop"].ID(), a11y.ActionDefault) || p.Transport.State != players.Stopped {
				t.Fatal("accessible Stop failed")
			}
			p.List().RequestFocus(p.listPane.list)
			key(a, p.List(), platform.KeySpace, 0)
			key(a, p.List(), platform.KeyB, 0)
			p.Eq().RequestFocus(p.eqPane.bands[0])
			key(a, p.Eq(), platform.KeyZ, 0)
			p.Main().RequestFocus(p.body.seek)
			key(a, p.Main(), platform.KeyV, 0)
			want := []players.Command{players.CmdPlay, players.CmdPlayPause, players.CmdPlayPause, players.CmdNext, players.CmdStop, players.CmdPlayPause, players.CmdNext, players.CmdPrev, players.CmdStop}
			if !reflect.DeepEqual(commands, want) {
				t.Fatalf("controls swallowed or duplicated transport actions: %v", commands)
			}
			opens := 0
			p.Host.LoadFiles = func(_ widget.Component, appendToList bool) {
				if appendToList {
					t.Fatal("main Open appended unexpectedly")
				}
				opens++
			}
			press("open")
			if opens != 1 {
				t.Fatal("Open did not invoke the file chooser")
			}
		})
	}
}

func TestOriginalCurvedPositionMapsDriveSharedPlayback(t *testing.T) {
	for _, scale := range []float32{1, 1.75} {
		t.Run(fmt.Sprintf("%gx", scale), func(t *testing.T) {
			a, p := open(t, scale)
			syncs := 0
			p.Host.SyncPlayback = func() { syncs++ }
			archiveBytes := source.Archive()
			archive, err := zip.NewReader(bytes.NewReader(archiveBytes), int64(len(archiveBytes)))
			if err != nil {
				t.Fatal(err)
			}
			for _, item := range []struct {
				f    *curvedSlider
				name string
			}{{p.body.seek, "seek_map.png"}, {p.body.volume, "volume_map.png"}} {
				reader, err := archive.Open(item.name)
				if err != nil {
					t.Fatal(err)
				}
				im, _, err := image.Decode(reader)
				reader.Close()
				if err != nil {
					t.Fatal(err)
				}
				for _, target := range []int{64, 224} {
					best, point, gray := 256, image.Point{}, uint8(0)
					for y := im.Bounds().Min.Y; y < im.Bounds().Max.Y; y++ {
						for x := im.Bounds().Min.X; x < im.Bounds().Max.X; x++ {
							c := color.NRGBAModel.Convert(im.At(x, y)).(color.NRGBA)
							if c.A < 128 || c.R != c.G || c.R != c.B {
								continue
							}
							if distance := abs(int(c.R) - target); distance < best {
								best, point, gray = distance, image.Pt(x, y), c.R
							}
						}
					}
					if best > 2 {
						t.Fatalf("%s lacks source grayscale near %d", item.name, target)
					}
					local := paint2d.Pt((float32(point.X)+.5)*scale, (float32(point.Y)+.5)*scale)
					if item.f.HitTest(local) != item.f {
						t.Fatal("original curve point is absent from the control hit map")
					}
					at, before := widget.DeviceOrigin(item.f).Add(local), syncs
					click(p.Main(), platform.ButtonLeft, at)
					a.PumpOnce()
					want := item.f.Min + (item.f.Max-item.f.Min)*float32(gray)/255
					if syncs != before+1 || math.Abs(float64(item.f.Value-want)) > float64(item.f.Max-item.f.Min)*3/255 {
						t.Fatalf("%s gray%d gave %g, want %g, callbacks%d", item.name, gray, item.f.Value, want, syncs-before)
					}
					before, value := syncs, item.f.Value
					if item.f.MousePress(widget.MouseEvent{Button: platform.ButtonRight, Pos: local}) || item.f.Dragging() || syncs != before || item.f.Value != value {
						t.Fatal("secondary input changed the curve or started a drag")
					}
				}
			}
			before := syncs
			p.Refresh()
			if syncs != before {
				t.Fatal("refresh generated slider input")
			}
		})
	}
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

func TestSeparateEqualizerHooksAndHonestEffects(t *testing.T) {
	a, p := open(t, 1.75)
	changes, syncs := 0, 0
	refresh := p.Equaliser.Changed
	p.Equaliser.Changed = func() { changes++; refresh() }
	p.Host.SyncPlayback = func() { syncs++ }
	if !p.Eq().AccessibleAction(p.eqPane.bands[0].ID(), a11y.ActionIncrement) || changes != 1 || p.Equaliser.Gains[0] <= 0 {
		t.Fatal("EQ band did not edit the shared equalizer once")
	}
	if !p.Eq().AccessibleAction(p.eqPane.balance.ID(), a11y.ActionIncrement) || syncs != 1 || p.Transport.Pan <= 0 {
		t.Fatal("balance did not invoke the shared audio hook once")
	}
	ring := playertest.FocusRing(t, a, p.Eq())
	for _, c := range p.eqPane.Children() {
		if c.Visible() && c.Enabled() && c.WantsFocus() && !playertest.Reaches(ring, c) {
			t.Fatalf("enabled EQ control %T is absent from keyboard order", c)
		}
	}
	for _, slot := range []string{"crossfade", "srs", "speaker"} {
		if p.eqPane.buttons[slot].Enabled() || !strings.Contains(p.eqPane.buttons[slot].Tip, "not in this player") {
			t.Fatal("unsupported original audio effect is exposed as working")
		}
	}
	for _, window := range []*ui.Window{p.Eq(), p.List()} {
		window.RequestClose()
		a.PumpOnce()
		if window.Closed() || window.Visible() || p.Main().Closed() {
			t.Fatal("satellite close must hide its panel and keep Main")
		}
	}
	if p.Host.Model.EqualizerVisible || p.Host.Model.PlaylistVisible {
		t.Fatal("satellite close did not persist visibility")
	}
}

func TestEqualizerSourceThumbRemainsVisibleAboveChangingLevelFill(t *testing.T) {
	a, p := open(t, 1)
	data := source.Archive()
	archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	r, err := archive.Open("eq_thumb.png")
	if err != nil {
		t.Fatal(err)
	}
	thumb, _, err := image.Decode(r)
	r.Close()
	if err != nil {
		t.Fatal(err)
	}
	want := color.NRGBAModel.Convert(thumb.At(thumb.Bounds().Dx()/2, thumb.Bounds().Dy()/2)).(color.NRGBA)
	band := p.eqPane.bands[0]
	origin := widget.DeviceOrigin(band)
	var counts []int
	for _, gain := range []float32{-players.EqRange, players.EqRange} {
		p.Equaliser.Set(0, gain)
		a.PumpOnce()
		im := p.Eq().CaptureSurface()
		top := int(origin.Y)
		if gain < 0 {
			top += int(band.Bounds().Dy()) - thumb.Bounds().Dy()
		}
		got := im.NRGBAAt(int(origin.X)+thumb.Bounds().Dx()/2, top+thumb.Bounds().Dy()/2)
		if got != want {
			t.Fatalf("gain%g thumb was covered by its level fill: got%v original%v", gain, got, want)
		}
		count := 0
		for y := int(origin.Y); y < int(origin.Y+band.Bounds().Dy()); y++ {
			for x := int(origin.X); x < int(origin.X+band.Bounds().Dx()); x++ {
				c := im.NRGBAAt(x, y)
				if c.A > 128 && int(c.R) > int(c.G)+30 && int(c.G) > int(c.B)+20 {
					count++
				}
			}
		}
		counts = append(counts, count)
	}
	if counts[1] <= counts[0] {
		t.Fatalf("source orange level did not increase with gain: min%d max%d", counts[0], counts[1])
	}
}

func TestAllThreeViewsAcceptDecodedLocalFileAndFolderCopyDrops(t *testing.T) {
	a, p := open(t, 1.75)
	dir := t.TempDir()
	file, folder := filepath.Join(dir, "song one.mp3"), filepath.Join(dir, "music folder")
	if err := os.WriteFile(file, []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(folder, 0700); err != nil {
		t.Fatal(err)
	}
	uri := func(path string) string { return (&url.URL{Scheme: "file", Path: path}).String() }
	data := []byte(uri(file) + "\r\n" + uri(folder) + "\r\n")
	calls := 0
	p.Host.DropFiles = func(paths []string) {
		calls++
		if !reflect.DeepEqual(paths, []string{file, folder}) {
			t.Fatalf("URI decoder delivered %q", paths)
		}
	}
	p.Transport.State, p.Transport.Pos = players.Paused, 17*time.Second
	for _, target := range []struct {
		w  *ui.Window
		at paint2d.Point
	}{{p.Main(), paint2d.Pt(150*p.Main().Scale(), 150*p.Main().Scale())}, {p.Eq(), widget.DeviceBounds(p.eqPane.bands[0]).Center()}, {p.List(), p.listPane.list.AccessibleItems()[0].Bounds.Center()}} {
		off := target.w.Surface().(*platform.Offscreen)
		before := calls
		off.SimulateDropAction(target.at, map[string][]byte{"text/uri-list": data}, platform.DragCopy|platform.DragMove, platform.DragMove)
		a.PumpOnce()
		if accepted := off.DropTaken(); accepted == nil || !*accepted || calls != before+1 {
			t.Fatalf("%q did not accept a decoded Copy drop once", target.w.Title())
		}
		off.SimulateDropAction(target.at, map[string][]byte{"text/uri-list": data}, platform.DragMove, platform.DragMove)
		a.PumpOnce()
		if accepted := off.DropTaken(); accepted == nil || *accepted || calls != before+1 {
			t.Fatal("Move-only source drop was accepted")
		}
		if p.Transport.State != players.Paused || p.Transport.Pos != 17*time.Second {
			t.Fatal("drop changed playback before the shared loader")
		}
	}
}

func TestVisualizerToggleAndMeasuredBandsChangeVisiblePixels(t *testing.T) {
	for _, scale := range []float32{1, 1.75} {
		t.Run(fmt.Sprintf("%gx", scale), func(t *testing.T) {
			a, p := open(t, scale)
			if p.body.visual.Visible() || !p.body.buttons["logo"].Visible() {
				t.Fatal("default view must show the original Lambda artwork")
			}
			p.Transport.State = players.Playing
			p.Spectrum.Live = true
			p.Spectrum.SetLevels([]float32{.8, 1, .6, .9})
			p.Refresh()
			a.PumpOnce()
			original := p.Main().CaptureSurface()
			if !p.Main().AccessibleAction(p.body.buttons["vis"].ID(), a11y.ActionDefault) {
				t.Fatal("source visualization button refused activation")
			}
			a.PumpOnce()
			if !p.body.visual.Visible() || p.body.buttons["logo"].Visible() {
				t.Fatal("visualization toggle left the opaque Lambda over the analyser")
			}
			active := p.Main().CaptureSurface()
			p.Spectrum.Reset()
			p.Refresh()
			a.PumpOnce()
			silent := p.Main().CaptureSurface()
			area := widget.DeviceBounds(p.body.visual)
			appearance, signal := 0, 0
			for y := int(area.Min.Y); y < int(area.Max.Y); y++ {
				for x := int(area.Min.X); x < int(area.Max.X); x++ {
					if active.NRGBAAt(x, y) != original.NRGBAAt(x, y) {
						appearance++
					}
					if active.NRGBAAt(x, y) != silent.NRGBAAt(x, y) {
						signal++
					}
				}
			}
			if appearance < 100 || signal < 100 {
				t.Fatalf("visible analyser did not replace logo/respond to measured levels: appearance%d signal%d", appearance, signal)
			}
		})
	}
}

func TestPlaylistResizeBrowseActivateAndInterruptedGesture(t *testing.T) {
	for _, scale := range []float32{1, 1.75} {
		t.Run(fmt.Sprintf("%gx", scale), func(t *testing.T) {
			a, p := open(t, scale)
			p.List().SetSize(620, 420)
			a.PumpOnce()
			p.Advance(0)
			if w, h := p.List().Size(); w != 620 || h != 420 {
				t.Fatal("playlist resize was not acknowledged")
			}
			for _, c := range p.listPane.Children() {
				r, parent := c.Bounds(), p.listPane.LocalBounds()
				if c.Visible() && (r.Empty() || r.Min.X < 0 || r.Min.Y < 0 || r.Max.X > parent.Max.X || r.Max.Y > parent.Max.Y) {
					t.Fatalf("resized playlist clipped %T %v", c, r)
				}
			}
			if p.List().CaptureSurface().NRGBAAt(0, 0).A != 0 {
				t.Fatal("resizing discarded the source outline")
			}
			list, plays := p.listPane.list, 0
			p.Host.Run = func(c players.Command) {
				if c == players.CmdPlay {
					plays++
				}
				p.Transport.Do(c)
			}
			p.Transport.State, p.Transport.Pos = players.Paused, 17*time.Second
			at := list.AccessibleItems()[2].Bounds.Center()
			click(p.List(), platform.ButtonLeft, at)
			a.PumpOnce()
			p.Refresh()
			if list.Selected != 2 || plays != 0 || p.Transport.List.Index() != 0 || p.Transport.State != players.Paused {
				t.Fatal("single-click browsing changed playback or refresh reset selection")
			}
			key(a, p.List(), platform.KeyDown, 0)
			if list.Selected != 3 || plays != 0 {
				t.Fatal("arrow navigation started playback")
			}
			key(a, p.List(), platform.KeyReturn, 0)
			if plays != 1 || p.Transport.List.Index() != 3 {
				t.Fatal("Enter did not play the selected row once")
			}
			p.Transport.State = players.Paused
			if !p.List().AccessibleAction(widget.ItemID(list, 3), a11y.ActionDefault) || p.Transport.State != players.Playing || plays != 2 {
				t.Fatal("accessible repeated activation did not resume")
			}
			click(p.List(), platform.ButtonLeft, at)
			click(p.List(), platform.ButtonLeft, at)
			a.PumpOnce()
			if plays != 3 || p.Transport.List.Index() != 2 {
				t.Fatal("double-click did not activate once")
			}
			p.Transport.List.Select(0)
			p.Transport.State = players.Paused
			p.Refresh()
			before := plays
			click(p.List(), platform.ButtonLeft, at)
			p.List().Inject(platform.Event{Kind: platform.EventMouseMove, Pos: widget.DeviceBounds(p.listPane.buttons["shuffle"]).Center()})
			click(p.List(), platform.ButtonLeft, at)
			a.PumpOnce()
			if plays != before || p.Transport.List.Index() != 0 {
				t.Fatal("pointer exit combined interrupted clicks into playback")
			}
			g := list.geometry(list.Look())
			gutter := widget.DeviceOrigin(list).Add(paint2d.Pt(g.Rows.Min.X-2*scale, at.Y-widget.DeviceOrigin(list).Y))
			selected := list.Selected
			click(p.List(), platform.ButtonLeft, gutter)
			a.PumpOnce()
			if list.Selected != selected || plays != before {
				t.Fatal("a horizontal row gutter acted as a track")
			}
			list.SetEnabled(false)
			for _, button := range []platform.MouseButton{platform.ButtonLeft, platform.ButtonRight} {
				click(p.List(), button, at)
				a.PumpOnce()
				if list.Selected != selected || plays != before || p.List().Popup() != nil {
					t.Fatal("disabled playlist selected, played or opened a menu")
				}
			}
		})
	}
}

func TestSatelliteControlsMatchOriginalWMSRightAndBottomOffsets(t *testing.T) {
	for _, scale := range []float32{1, 1.75} {
		t.Run(fmt.Sprintf("%gx", scale), func(t *testing.T) {
			a, p := open(t, scale)
			// The source WMS places both pl_set and vis_set groups at
			// view.width-78,top5. Their three mapping-image cells begin
			// at x0,x23,x46; the bottom grip is width-34,height-30.
			check := func(name string, got paint2d.Rect, x, y, w, h int) {
				want := paint2d.XYWH(float32(x)*scale, float32(y)*scale, float32(w)*scale, float32(h)*scale)
				// Slots snap their margins and extents to device pixels.
				// One-pixel fractional rounding cannot mask the original
				// 23px logical shift from an incorrect FromRight margin.
				if math.Abs(float64(got.Min.X-want.Min.X)) > 1.01 || math.Abs(float64(got.Min.Y-want.Min.Y)) > 1.01 ||
					math.Abs(float64(got.Dx()-want.Dx())) > 1.01 || math.Abs(float64(got.Dy()-want.Dy())) > 1.01 {
					t.Fatalf("%s rectangle %v, original WMS %v", name, got, want)
				}
			}
			for _, key := range []struct {
				slot          string
				offset, width int
			}{{"eq.prev", 0, 23}, {"eq.next", 23, 23}, {"close", 46, 22}} {
				check("EQ "+key.slot, p.eqPane.buttons[key.slot].Bounds(), EqW-78+key.offset, 5, key.width, 23)
			}
			for _, size := range [][2]int{{ListW, ListH}, {620, 420}} {
				p.List().SetSize(size[0], size[1])
				a.PumpOnce()
				for _, key := range []struct {
					slot          string
					offset, width int
				}{{"shuffle", 0, 23}, {"repeat", 23, 23}, {"close", 46, 22}} {
					check("PL "+key.slot, p.listPane.buttons[key.slot].Bounds(), size[0]-78+key.offset, 5, key.width, 23)
				}
				grip, ok := skins.SkinSlotRect(p.List().Look(), LayoutPlaylist, "resize", p.listPane.LocalBounds())
				if !ok {
					t.Fatal("source playlist resize slot missing")
				}
				check("PL grip", grip, size[0]-34, size[1]-30, 34, 30)
			}
		})
	}
}
