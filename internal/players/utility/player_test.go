package utility

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
	"goframework.io/desktop-ui"
	"goframework.io/desktop-ui-skingen"
	"goframework.io/desktop-ui-skins"
	"goframework.io/desktop-ui/a11y"
	"goframework.io/desktop-ui/platform"
	"goframework.io/desktop-ui/style"
	"goframework.io/desktop-ui/widget"
	"goframework.io/graphics-paint2d"
)

func open(t *testing.T, scale float32) (*ui.Application, *Player) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	plan := plans.Utility()
	plan.Name = "vlc-private-ui-probe"
	dir := t.TempDir()
	if err := plans.Write(dir, plan); err != nil {
		t.Fatal(err)
	}
	if err := skins.RegisterSkinFS(os.DirFS(filepath.Join(dir, plan.Name)), plan.Name); err != nil {
		t.Fatal(err)
	}
	t.Setenv(style.ThemeEnv, plan.Name)
	a := ui.New(ui.Options{Headless: true, Scale: scale, DisableLookWatch: true})
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
func click(w *ui.Window, at paint2d.Point) {
	w.Inject(platform.Event{Kind: platform.EventMouseMove, Pos: at})
	w.Inject(platform.Event{Kind: platform.EventMouseDown, Button: platform.ButtonLeft, Pos: at})
	w.Inject(platform.Event{Kind: platform.EventMouseUp, Button: platform.ButtonLeft, Pos: at})
}
func center(c widget.Component) paint2d.Point {
	return widget.DeviceOrigin(c).Add(paint2d.Pt(c.Bounds().Dx()/2, c.Bounds().Dy()/2))
}
func key(a *ui.Application, w *ui.Window, k platform.Key, mods platform.Modifiers) {
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
			at := widget.DeviceOrigin(l).Add(paint2d.Pt(20*scale, 52*scale))
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

func TestVLCActionsDropAndEqualizer(t *testing.T) {
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
	p.Equaliser.Set(4, 6)
	p.Equaliser.SetPreamp(-2)
	p.Host.SetPan(.25)
	p.Refresh()
	p.ShowPlaylist(false)
	a.PumpOnce()
	if !p.body.eq.on.Enabled() || !p.body.eq.bands[4].Enabled() || !p.body.eq.balance.Enabled() || !p.body.analyser.Visible() {
		t.Fatal("equaliser, balance or analyser not offered")
	}
	if p.Equaliser.Gains[4] != 6 || p.Equaliser.Preamp != -2 || p.Transport.Pan != .25 {
		t.Fatal("a refresh changed the shared settings")
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
			// The 16 × 16 toolbar icons are pixel art: in actual rendering,
			// including fractional display scaling, they must show only their
			// own inks and the toolbar under them, never an interpolated mix.
			for _, item := range []struct{ button, icon string }{{"open", "eject"}, {"play", "play"}, {"stop", "stop"}, {"prev", "prev"}, {"next", "next"}} {
				palette := iconInks(t, item.icon)
				c0 := p.body.buttons[item.button]
				o0 := widget.DeviceOrigin(c0)
				palette[image.NRGBAAt(int(o0.X+1), int(o0.Y+1))] = true // the toolbar behind the icon
				c, o := c0, o0
				size := 16 * scale
				r := paint2d.XYWH(o.X+(c.Bounds().Dx()-size)/2, o.Y+(c.Bounds().Dy()-size)/2, size, size)
				for y := int(r.Min.Y + .5); y < int(r.Max.Y); y++ {
					for x := int(r.Min.X + .5); x < int(r.Max.X); x++ {
						got := image.NRGBAAt(x, y)
						if !palette[got] {
							t.Fatalf("%s introduces interpolated color%v at%d,%d", item.icon, got, x, y)
						}
					}
				}
			}
			if sk, ok := skins.LoadSkin(style.LookAppearance(p.Main.Look()).Name); !ok {
				t.Fatal("private VLC pack cannot load")
			} else {
				for _, warning := range skins.LintSkin(sk) {
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
				at := widget.DeviceOrigin(s).Add(paint2d.Pt(2*scale, 2*scale))
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
			click(p.Main, widget.DeviceOrigin(p.body.volume).Add(paint2d.Pt(p.body.volume.Bounds().Dx()-2*scale, p.body.volume.Bounds().Dy()/2)))
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

// iconInks are the colours the Utility skin's icon of that name is drawn in.
func iconInks(t *testing.T, name string) map[color.NRGBA]bool {
	t.Helper()
	inks := map[color.NRGBA]bool{}
	for _, sh := range plans.Utility().Sheets {
		for _, c := range sh.Cells {
			if c.Name != "icon."+name {
				continue
			}
			if !sh.Pixelated {
				t.Fatalf("%s is not on a pixelated sheet", c.Name)
			}
			img := skingen.Render(sh, 1)
			for y := c.Y; y < c.Y+c.H; y++ {
				for x := c.X; x < c.X+c.W; x++ {
					if got := img.NRGBAAt(x, y); got.A != 0 {
						if got.A != 255 {
							t.Fatalf("%s has a partly covered pixel at %d,%d", c.Name, x-c.X, y-c.Y)
						}
						inks[got] = true
					}
				}
			}
			return inks
		}
	}
	t.Fatalf("no icon.%s", name)
	return nil
}
