package vlc

import (
	"fmt"
	"image/color"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/codemodify/media-player-music/internal/players"
	"github.com/codemodify/media-player-music/internal/players/playertest"
	"github.com/codemodify/media-player-music/internal/skins/plans"
	vlcsource "github.com/codemodify/media-player-music/internal/skins/sources/vlc"
	"github.com/codemodify/paintengine2d"
	"github.com/codemodify/uitoolkit"
	"github.com/codemodify/uitoolkit/a11y"
	"github.com/codemodify/uitoolkit/app"
	"github.com/codemodify/uitoolkit/platform"
	"github.com/codemodify/uitoolkit/style"
	"github.com/codemodify/uitoolkit/widget"
)

func open(t *testing.T, scale float32) (*uitoolkit.Application, *Player) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	plan := plans.VLC()
	plan.Name = "vlc-private-ui-probe"
	dir := t.TempDir()
	if err := plans.Write(dir, plan); err != nil {
		t.Fatal(err)
	}
	if err := style.RegisterSkinFS(os.DirFS(filepath.Join(dir, plan.Name)), plan.Name); err != nil {
		t.Fatal(err)
	}
	t.Setenv(style.ThemeEnv, plan.Name)
	a := uitoolkit.New(uitoolkit.Options{Headless: true, Scale: scale, DisableLookWatch: true})
	h := players.NewHost(a)
	h.Packs = func() []string { return []string{plan.Name} }
	h.ShowSettings = func() {}
	p, err := New(h, Options{Headless: true, Scale: scale})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(p.Close)
	a.PumpOnce()
	p.Advance(0)
	return a, p
}
func click(w *app.Window, at paintengine2d.Point) {
	w.Inject(platform.Event{Kind: platform.EventMouseMove, Pos: at})
	w.Inject(platform.Event{Kind: platform.EventMouseDown, Button: platform.ButtonLeft, Pos: at})
	w.Inject(platform.Event{Kind: platform.EventMouseUp, Button: platform.ButtonLeft, Pos: at})
}
func center(c widget.Component) paintengine2d.Point {
	return widget.DeviceOrigin(c).Add(paintengine2d.Pt(c.Bounds().Dx()/2, c.Bounds().Dy()/2))
}
func key(a *uitoolkit.Application, w *app.Window, k platform.Key, mods platform.Modifiers) {
	w.Inject(platform.Event{Kind: platform.EventKeyDown, Key: k, Mods: mods})
	w.Inject(platform.Event{Kind: platform.EventKeyUp, Key: k, Mods: mods})
	a.PumpOnce()
}

func TestVLCOneWindowPanelsGeometryStateAndPlacement(t *testing.T) {
	for _, scale := range []float32{1, 1.75, 2} {
		t.Run(fmt.Sprintf("%gx", scale), func(t *testing.T) {
			a, p := open(t, scale)
			if len(p.Windows()) != 1 || len(a.Windows()) != 1 || p.Main.Title() != "Music Player" || !p.Main.Resizable() {
				t.Fatal("classic VLC must be one resizable Music Player window")
			}
			p.Transport.State, p.Transport.Pos = players.Paused, 17*time.Second
			p.Main.SetSize(620, 510)
			p.Main.Move(120, 75)
			a.PumpOnce()
			for _, flags := range [][2]bool{{false, true}, {true, false}, {false, false}, {true, true}} {
				p.ShowEqualiser(flags[0])
				p.ShowPlaylist(flags[1])
				a.PumpOnce()
				if len(a.Windows()) != 1 || p.Transport.State != players.Paused || p.Transport.Pos != 17*time.Second {
					t.Fatal("panel visibility replaced windows or changed playback")
				}
				if w, h := p.Main.Size(); w != 620 || h != 510 {
					t.Fatalf("panel toggle changed size%d×%d", w, h)
				}
				playertest.Audit(t, "classic VLC", p.Main)
			}
			p.Main.RequestFocus(p.body.queue.list)
			p.ShowPlaylist(false)
			a.PumpOnce()
			if p.Main.Focus() != p.body.buttons["play"] {
				t.Fatal("hidden playlist retained focus")
			}
			p.ShowPlaylist(true)
			p.Main.RequestFocus(p.body.eq.preamp)
			p.ShowEqualiser(false)
			a.PumpOnce()
			if p.Main.Focus() != p.body.buttons["play"] {
				t.Fatal("hidden equalizer retained focus")
			}
			p.ShowEqualiser(true)
			for _, size := range [][2]int{{MinW, MinH}, {800, 640}} {
				p.Main.SetSize(size[0], size[1])
				a.PumpOnce()
				q := p.body.queue
				if q.Bounds().Max.X > p.body.Bounds().Dx() || q.list.RowGeo(q.list.Look()).Rows.Dy() < 20*scale || p.body.buttons["mute"].Bounds().Max.X >= p.body.volume.Bounds().Min.X {
					t.Fatal("resized controls overlap or leave the viewport")
				}
			}
			saved := p.CapturePlacement()
			p.Main.Move(300, 300)
			p.Main.SetSize(500, 430)
			a.PumpOnce()
			p.RestorePlacement(saved)
			a.PumpOnce()
			if got := p.CapturePlacement(); !reflect.DeepEqual(got, saved) {
				t.Fatalf("placement roundtrip %v want%v", got, saved)
			}
		})
	}
}

func TestVLCRealPointerKeyboardAndAccessibleQueueActivation(t *testing.T) {
	for _, scale := range []float32{1, 1.75, 2} {
		t.Run(fmt.Sprintf("%gx", scale), func(t *testing.T) {
			a, p := open(t, scale)
			l := p.body.queue.list
			p.Transport.State, p.Transport.Pos = players.Paused, 17*time.Second
			commands := []players.Command{}
			p.Host.Run = func(c players.Command) { commands = append(commands, c); p.Transport.Do(c) }
			at := widget.DeviceOrigin(l).Add(paintengine2d.Pt(20*scale, 52*scale))
			click(p.Main, at)
			a.PumpOnce()
			p.Advance(70 * time.Millisecond)
			a.PumpOnce()
			if len(commands) != 0 || l.Selected != 2 || p.Transport.List.Index() != 0 || p.Transport.State != players.Paused || p.Transport.Pos != 17*time.Second {
				t.Fatalf("single-click changed transport: selected%d commands%v", l.Selected, commands)
			}
			// Queue both clicks together so the test is independent of raster
			// duration on race builds; the previous click is explicitly interrupted.
			l.InterruptActivation()
			click(p.Main, at)
			click(p.Main, at)
			a.PumpOnce()
			if !reflect.DeepEqual(commands, []players.Command{players.CmdPlay}) || p.Transport.List.Index() != 2 || p.Transport.State != players.Playing {
				t.Fatalf("double-click did not play exactly once: %v", commands)
			}
			commands = nil
			p.Main.RequestFocus(l)
			key(a, p.Main, platform.KeyDown, 0)
			if len(commands) != 0 || l.Selected != 3 {
				t.Fatal("arrow browsing activated")
			}
			key(a, p.Main, platform.KeyReturn, 0)
			if !reflect.DeepEqual(commands, []players.Command{players.CmdPlay}) || p.Transport.List.Index() != 3 {
				t.Fatal("Enter did not play selected track exactly once")
			}
			commands = nil
			if !l.AccessibleAction(1, a11y.ActionDefault) || !reflect.DeepEqual(commands, []players.Command{players.CmdPlay}) || p.Transport.List.Index() != 1 {
				t.Fatal("accessible default activation failed")
			}
			commands = nil
			l.SetEnabled(false)
			click(p.Main, at)
			click(p.Main, at)
			a.PumpOnce()
			if len(commands) != 0 || p.Transport.List.Index() != 1 {
				t.Fatal("disabled queue activated")
			}
		})
	}
}

func TestVLCActionsDropAndBackendCapabilities(t *testing.T) {
	a, p := open(t, 1.75)
	var opened []bool
	p.Host.LoadFiles = func(_ widget.Component, appendToList bool) { opened = append(opened, appendToList) }
	click(p.Main, center(p.body.buttons["open"]))
	a.PumpOnce()
	click(p.Main, center(p.body.queue.add))
	a.PumpOnce()
	if !reflect.DeepEqual(opened, []bool{false, true}) {
		t.Fatalf("open/add hooks%v", opened)
	}
	var dropped []string
	p.Host.DropFiles = func(paths []string) { dropped = paths }
	e := widget.DropEvent{Mime: "text/uri-list", Action: platform.DragCopy, Paths: []string{"/tmp/music", "/tmp/a.mp4"}}
	if !p.body.queue.Drop(e) || !reflect.DeepEqual(dropped, e.Paths) || p.body.queue.DropActionFor(platform.DragCopy|platform.DragMove) != platform.DragCopy {
		t.Fatal("copy-only local file/folder drop was not routed")
	}
	dropped = nil
	e.Action = platform.DragMove
	if p.body.Drop(e) || len(dropped) != 0 {
		t.Fatal("move drop was accepted")
	}
	p.Host.Model.BackendLimited = true
	p.Host.Model.BackendEqualizer = false
	p.Host.Model.BackendBalance = false
	p.Host.Model.BackendSpectrum = false
	p.Equaliser.Set(4, 6)
	p.Equaliser.SetPreamp(-2)
	p.Host.SetPan(.25)
	p.Refresh()
	p.ShowPlaylist(false)
	a.PumpOnce()
	if p.body.eq.on.Enabled() || p.body.eq.bands[4].Enabled() || p.body.eq.balance.Enabled() || p.body.analyser.Visible() {
		t.Fatal("unavailable backend controls remain active")
	}
	if p.Equaliser.Gains[4] != 6 || p.Equaliser.Preamp != -2 || p.Transport.Pan != .25 {
		t.Fatal("capability changes erased shared settings")
	}
	p.Host.Model.BackendEqualizer = true
	p.Host.Model.BackendBalance = true
	p.Host.Model.BackendSpectrum = true
	p.Refresh()
	if !p.body.eq.on.Enabled() || !p.body.eq.balance.Enabled() || !p.body.analyser.Visible() {
		t.Fatal("restored backend capabilities stay disabled")
	}
	closed := 0
	p.Main.SetOnCloseRequest(func() bool { closed++; return false })
	click(p.Main, center(p.body.buttons["close"]))
	a.PumpOnce()
	a.PumpOnce()
	if closed != 1 || p.Main.Closed() {
		t.Fatal("caption bypassed close-to-tray request")
	}
}

func TestVLCActualClassicToolbarPaletteAndCaptures(t *testing.T) {
	for _, scale := range []float32{1, 1.75, 2} {
		t.Run(fmt.Sprintf("%gx", scale), func(t *testing.T) {
			a, p := open(t, scale)
			p.Main.RequestFocus(p.body.seek)
			a.PumpOnce()
			image := p.Main.Capture()
			if image == nil {
				t.Fatal("classic VLC cannot capture its painted UI")
			}
			// The source16x16 toolbar images must remain palette-sharp in
			// actual rendering, including fractional display scaling.
			for _, item := range []struct{ button, icon string }{{"open", "eject"}, {"play", "play"}, {"stop", "stop"}, {"prev", "prev"}, {"next", "next"}} {
				original, err := vlcsource.Picture(item.icon)
				if err != nil {
					t.Fatal(err)
				}
				palette := map[color.NRGBA]bool{{R: 212, G: 208, B: 200, A: 255}: true}
				for y := 0; y < 16; y++ {
					for x := 0; x < 16; x++ {
						c := color.NRGBAModel.Convert(original.At(x, y)).(color.NRGBA)
						if c.A != 0 {
							palette[c] = true
						}
					}
				}
				c := p.body.buttons[item.button]
				o := widget.DeviceOrigin(c)
				size := 16 * scale
				r := paintengine2d.XYWH(o.X+(c.Bounds().Dx()-size)/2, o.Y+(c.Bounds().Dy()-size)/2, size, size)
				for y := int(r.Min.Y + .5); y < int(r.Max.Y); y++ {
					for x := int(r.Min.X + .5); x < int(r.Max.X); x++ {
						got := image.NRGBAAt(x, y)
						if !palette[got] {
							t.Fatalf("%s introduces interpolated color%v at%d,%d", item.icon, got, x, y)
						}
					}
				}
			}
			if sk, ok := style.LoadSkin(style.LookAppearance(p.Main.Look()).Name); !ok {
				t.Fatal("private VLC pack cannot load")
			} else {
				for _, warning := range style.LintSkin(sk) {
					if warning.Key != "sprites" {
						t.Errorf("private VLC pack: %s", warning)
					}
				}
			}
			if out := os.Getenv("MEDIA_PLAYER_VLC_PREVIEWS"); out != "" {
				if err := os.MkdirAll(out, 0o700); err != nil {
					t.Fatal(err)
				}
				write := func(name string) {
					t.Helper()
					if err := p.Main.Capture().WritePNGFile(filepath.Join(out, fmt.Sprintf("vlc-%g-%s.png", scale, name))); err != nil {
						t.Fatal(err)
					}
				}
				write("main")
				p.ShowEqualiser(false)
				a.PumpOnce()
				write("playlist")
				p.ShowPlaylist(false)
				p.Pose(17 * time.Second)
				a.PumpOnce()
				write("display")
				p.ShowPlaylist(true)
				p.ShowEqualiser(true)
				p.Main.SetSize(760, 560)
				a.PumpOnce()
				write("resized")
				p.Main.SetSize(MinW, MinH)
				a.PumpOnce()
				write("minimum")
			}
		})
	}
}

func TestVLCSecondarySliderPressOpensContextWithoutEditing(t *testing.T) {
	for _, scale := range []float32{1, 1.75, 2} {
		t.Run(fmt.Sprintf("%gx", scale), func(t *testing.T) {
			a, p := open(t, scale)
			p.Transport.State, p.Transport.Pos = players.Paused, 17*time.Second
			p.Host.SetVolume(.6)
			p.Host.SetPan(.25)
			p.Equaliser.SetPreamp(-2)
			p.Equaliser.Set(3, 6)
			p.Refresh()
			a.PumpOnce()
			updates := 0
			p.Host.SyncPlayback = func() { updates++ }
			for _, s := range []*sourceSlider{p.body.seek, p.body.volume, p.body.eq.balance, p.body.eq.preamp, p.body.eq.bands[3]} {
				before, pos := s.Value, p.Transport.Pos
				at := widget.DeviceOrigin(s).Add(paintengine2d.Pt(2*scale, 2*scale))
				p.Main.Inject(platform.Event{Kind: platform.EventMouseMove, Pos: at})
				p.Main.Inject(platform.Event{Kind: platform.EventMouseDown, Button: platform.ButtonRight, Pos: at})
				p.Main.Inject(platform.Event{Kind: platform.EventMouseUp, Button: platform.ButtonRight, Pos: at})
				a.PumpOnce()
				if s.Value != before || p.Transport.Pos != pos || updates != 0 || s.Dragging() {
					t.Fatalf("secondary %s press edited playback or started dragging", s.Label)
				}
				if p.Main.Popup() == nil {
					t.Fatalf("secondary %s press did not open the context menu", s.Label)
				}
				key(a, p.Main, platform.KeyEscape, 0)
			}
			// Primary input, keyboard and accessibility still edit the stock
			// slider after dismissing a secondary-click menu.
			click(p.Main, widget.DeviceOrigin(p.body.volume).Add(paintengine2d.Pt(p.body.volume.Bounds().Dx()-2*scale, p.body.volume.Bounds().Dy()/2)))
			a.PumpOnce()
			if updates == 0 || p.Transport.Volume < .9 {
				t.Fatal("primary volume input stopped working")
			}
			p.Main.RequestFocus(p.body.volume)
			key(a, p.Main, platform.KeyHome, 0)
			if p.Transport.Volume != 0 || !p.body.volume.AccessibleAction(0, a11y.ActionIncrement) || p.Transport.Volume <= 0 {
				t.Fatal("keyboard/accessibility slider input stopped working")
			}
		})
	}
}
