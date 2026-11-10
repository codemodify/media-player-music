package fandom

import (
	"fmt"
	"math"
	"os"
	"reflect"
	"strings"
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
	t.Setenv(style.ThemeEnv, players.FandomSkin)
	a := ui.New(ui.Options{Headless: true, Scale: scale, DisableLookWatch: true})
	h := players.NewHost(a)
	h.Packs = players.SkinnedIDs
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

func TestReferenceGeometryMasksAndEmbeddedSettingsVisibility(t *testing.T) {
	for _, scale := range []float32{1, 1.75, 2} {
		t.Run(fmt.Sprintf("%gx", scale), func(t *testing.T) {
			a, p := open(t, scale)
			if len(p.Windows()) != 2 || p.Main.Title() != "Music Player" || p.Main.Resizable() || !p.List.Resizable() {
				t.Fatal("reference composition/sizing policy changed")
			}
			for _, item := range []struct {
				w             *ui.Window
				width, height int
			}{{p.Main, 490, 330}, {p.List, 190, 246}} {
				w, h := item.w.Size()
				pw, ph := item.w.PixelSize()
				if w != item.width || h != item.height || pw != int(math.Round(float64(w)*float64(scale))) || ph != int(math.Round(float64(h)*float64(scale))) {
					t.Fatal("logical/device geometry does not match reference")
				}
				frame := item.w.Surface().(*platform.Offscreen).Frame()
				if !item.w.ShapeActive() || !frame.Alpha || len(frame.Shape) == 0 || item.w.Shape().Raster(pw, ph).Clamped {
					t.Fatal("shape degraded to opaque rectangle")
				}
				if item.w.CaptureSurface().NRGBAAt(0, 0).A != 0 || item.w.Shape().Raster(pw, ph).Contains(0, 0) {
					t.Fatal("empty corner is painted or accepts input")
				}
				playertest.Audit(t, item.w.Title(), item.w)
			}
			off := p.List.Surface().(*platform.Offscreen)
			if off.WindowRole() != platform.RoleUtility || off.Owner() != p.Main.Surface() || !off.SkipsTaskbar() {
				t.Fatal("playlist lacks public utility/ownership policy")
			}
			p.Transport.State, p.Transport.Pos = players.Paused, 17*time.Second
			for _, visible := range []bool{true, false, true} {
				if !visible {
					p.Main.RequestFocus(p.body.buttons["settings.library"])
				}
				p.ShowSettingsPanel(visible)
				a.PumpOnce()
				pw, ph := p.Main.PixelSize()
				shape := p.Main.Shape().Raster(pw, ph)
				im := p.Main.CaptureSurface()
				x, y := int(61*scale), int(110*scale)
				if shape.Contains(x, y) != visible || (im.NRGBAAt(x, y).A != 0) != visible {
					t.Fatal("settings wing artwork/input does not follow visibility")
				}
				if shape.Contains(int(126*scale), int(110*scale)) || im.NRGBAAt(int(126*scale), int(110*scale)).A != 0 {
					t.Fatal("empty gap between wing and main accepts input or is painted")
				}
				if !shape.Contains(int(300*scale), int(110*scale)) {
					t.Fatal("settings wing toggle changed central chassis mask")
				}
				if !visible && p.Main.Focus() != p.body.play {
					t.Fatal("collapsed settings wing left focus on a hidden control")
				}
				if w, h := p.Main.Size(); w != 490 || h != 330 {
					t.Fatal("wing visibility resized fixed Main")
				}
				if p.Transport.State != players.Paused || p.Transport.Pos != 17*time.Second {
					t.Fatal("wing toggle changed playback")
				}
			}
		})
	}
}

func TestReferenceTransportSettingsActionsAndShortcutBubbling(t *testing.T) {
	for _, scale := range []float32{1, 1.75} {
		t.Run(fmt.Sprintf("%gx", scale), func(t *testing.T) {
			a, p := open(t, scale)
			p.ShowEqualiser(false)
			a.PumpOnce()
			var commands []players.Command
			p.Host.Run = func(c players.Command) { commands = append(commands, c); p.Transport.Do(c) }
			press := func(slot string) {
				click(p.Main, platform.ButtonLeft, widget.DeviceBounds(p.body.buttons[slot]).Center())
				a.PumpOnce()
			}
			press("play")
			if p.Transport.State != players.Playing || p.body.play.Label != "Pause" {
				t.Fatal("Play failed")
			}
			press("play")
			if p.Transport.State != players.Paused {
				t.Fatal("Pause failed")
			}
			p.Main.RequestFocus(p.body.play)
			key(a, p.Main, platform.KeyReturn, 0)
			press("next")
			if p.Transport.List.Index() != 1 || p.Transport.State != players.Playing {
				t.Fatal("keyboard resume/next did not use shared queue")
			}
			if !p.Main.AccessibleAction(p.body.buttons["stop"].ID(), a11y.ActionDefault) || p.Transport.State != players.Stopped {
				t.Fatal("accessible Stop failed")
			}
			p.List.RequestFocus(p.listPane.list)
			key(a, p.List, platform.KeySpace, 0)
			key(a, p.List, platform.KeyB, 0)
			p.Main.RequestFocus(p.body.seek)
			key(a, p.Main, platform.KeyZ, 0)
			key(a, p.Main, platform.KeyV, 0)
			want := []players.Command{players.CmdPlay, players.CmdPlayPause, players.CmdPlayPause, players.CmdNext, players.CmdStop, players.CmdPlayPause, players.CmdNext, players.CmdPrev, players.CmdStop}
			if !reflect.DeepEqual(commands, want) {
				t.Fatalf("duplicated/swallowed actions: %v", commands)
			}
			settings, opens, adds := 0, 0, 0
			p.Host.ShowSettings = func() { settings++ }
			p.Host.LoadFiles = func(_ widget.Component, appendToList bool) {
				if appendToList {
					adds++
				} else {
					opens++
				}
			}
			for _, slot := range []string{"settings.general", "settings.backends", "settings.library", "open"} {
				if !p.Main.AccessibleAction(p.body.buttons[slot].ID(), a11y.ActionDefault) {
					t.Fatalf("%s inaccessible", slot)
				}
			}
			if settings != 2 || opens != 1 || adds != 1 {
				t.Fatalf("settings/file hooks settings%d open%d add%d", settings, opens, adds)
			}
			p.Main.AccessibleAction(p.body.buttons["settings.equalizer"].ID(), a11y.ActionDefault)
			a.PumpOnce()
			if !p.Host.Model.EqualizerVisible || !p.body.bands[0].Visible() || p.body.visual.Visible() {
				t.Fatal("settings Equalizer did not switch inset")
			}
			p.Main.AccessibleAction(p.body.buttons["settings.nowplaying"].ID(), a11y.ActionDefault)
			a.PumpOnce()
			if p.Host.Model.EqualizerVisible || !p.body.visual.Visible() {
				t.Fatal("Now Playing failed to restore visualizer")
			}
		})
	}
}

func TestStraightSlidersEqualizerFocusAndBackendCapabilities(t *testing.T) {
	a, p := open(t, 1.75)
	p.ShowEqualiser(true)
	a.PumpOnce()
	ring := playertest.FocusRing(t, a, p.Main)
	for _, c := range p.body.Children() {
		if c.Visible() && c.Enabled() && c.WantsFocus() && !playertest.Reaches(ring, c) {
			t.Fatalf("visible %T absent from Tab order", c)
		}
	}
	syncs := 0
	p.Host.SyncPlayback = func() { syncs++ }
	for _, s := range []*silverSlider{p.body.seek, p.body.volume, p.body.balance} {
		before := syncs
		if !p.Main.AccessibleAction(s.ID(), a11y.ActionIncrement) || syncs != before+1 {
			t.Fatalf("%s did not notify shared engine exactly once", s.Label)
		}
		value := s.Value
		if s.MousePress(widget.MouseEvent{Button: platform.ButtonRight, Pos: s.LocalBounds().Center()}) || s.Dragging() || value != s.Value || syncs != before+1 {
			t.Fatal("secondary input changed a slider")
		}
	}
	if p.Transport.Pos <= 0 || p.Transport.Volume != p.body.volume.Value/100 || p.Transport.Pan != p.body.balance.Value {
		t.Fatal("slider and playback values disagree")
	}
	before := syncs
	p.Refresh()
	if syncs != before {
		t.Fatal("model refresh emitted user input")
	}
	eqChanges := 0
	refresh := p.Equaliser.Changed
	p.Equaliser.Changed = func() { eqChanges++; refresh() }
	if !p.Main.AccessibleAction(p.body.bands[0].ID(), a11y.ActionIncrement) || !p.Main.AccessibleAction(p.body.preamp.ID(), a11y.ActionIncrement) || eqChanges != 2 {
		t.Fatal("EQ/preamp did not edit shared model once each")
	}
	gain, preamp := p.Equaliser.Gains[0], p.Equaliser.Preamp
	p.Main.RequestFocus(p.body.bands[0])
	p.ShowEqualiser(false)
	a.PumpOnce()
	if p.Main.Focus() != p.body.play || p.Main.AccessibleTree().Find(p.body.bands[0].ID()) != nil {
		t.Fatal("hidden EQ inset retained focus or accessibility controls")
	}
	p.ShowEqualiser(true)
	p.Host.Model.Live = true
	p.Host.Model.BackendLimited = true
	p.Host.AudioBackend = "VLC"
	p.Refresh()
	for _, s := range []*silverSlider{p.body.bands[0], p.body.preamp, p.body.balance} {
		if s.Enabled() {
			t.Fatal("limited backend retained enabled DSP control")
		}
	}
	if !p.body.volume.Enabled() || !strings.Contains(p.Host.StatusText(), "unavailable") {
		t.Fatal("backend limitation disabled basic volume or omitted explanation")
	}
	p.Host.Model.BackendEqualizer, p.Host.Model.BackendBalance, p.Host.Model.BackendSpectrum = true, true, true
	p.Refresh()
	if !p.body.bands[0].Enabled() || !p.body.preamp.Enabled() || !p.body.balance.Enabled() || gain != p.Equaliser.Gains[0] || preamp != p.Equaliser.Preamp {
		t.Fatal("restoring backend capabilities lost audio settings")
	}
}

func TestMeasuredSpectrumChangesVisibleGreenPixels(t *testing.T) {
	for _, scale := range []float32{1, 1.75} {
		t.Run(fmt.Sprintf("%gx", scale), func(t *testing.T) {
			a, p := open(t, scale)
			p.ShowEqualiser(false)
			p.Transport.State = players.Playing
			p.Spectrum.Live = true
			p.Spectrum.SetLevels([]float32{.8, 1, .6, .9})
			p.Refresh()
			a.PumpOnce()
			active := p.Main.CaptureSurface()
			area := widget.DeviceBounds(p.body.visual)
			p.Spectrum.Reset()
			p.Refresh()
			a.PumpOnce()
			silent := p.Main.CaptureSurface()
			changed, green := 0, 0
			for y := int(area.Min.Y); y < int(area.Max.Y); y++ {
				for x := int(area.Min.X); x < int(area.Max.X); x++ {
					c := active.NRGBAAt(x, y)
					if c != silent.NRGBAAt(x, y) {
						changed++
					}
					if int(c.G) > int(c.R)+30 && int(c.G) > int(c.B)+30 {
						green++
					}
				}
			}
			if !p.body.visual.Visible() || changed < 100 || green < 100 {
				t.Fatalf("green analyser is hidden/static: changed%d green%d", changed, green)
			}
		})
	}
}

func TestSoftArtAlphaIsNotSquaredByWindowMask(t *testing.T) {
	for _, scale := range []float32{1, 1.75} {
		t.Run(fmt.Sprintf("%gx", scale), func(t *testing.T) {
			_, p := open(t, scale)
			p.ShowSettingsPanel(false)
			pw, ph := p.Main.PixelSize()
			raw := paint2d.NewImage(pw, ph)
			ctx := paint2d.NewContext(raw)
			ctx.Clear(paint2d.Transparent)
			box := paint2d.XYWH(0, 0, float32(pw), float32(ph))
			r, ok := skins.SkinSlotRect(p.Main.Look(), LayoutMain, "shape.main", box)
			if !ok || !skins.DrawSkinSlot(p.Main.Look(), ctx, r, LayoutMain, "shape.main", 0) {
				t.Fatal("main reference art unavailable")
			}
			capture := p.Main.CaptureSurface()
			// Passive app text also legitimately contributes alpha. Compare
			// the artwork edge outside both text slots and child controls.
			var overlays []paint2d.Rect
			for _, slot := range []string{"header", "settings.title", "metadata", "status", "status.eq", "analyser", "eq.balance.label"} {
				if r, ok := skins.SkinSlotRect(p.Main.Look(), LayoutMain, slot, box); ok {
					overlays = append(overlays, r)
				}
			}
			checked := 0
			for y := 0; y < ph; y++ {
				for x := 0; x < pw; x++ {
					alpha := raw.Pix[(y*pw+x)*4+3]
					if alpha == 0 || alpha == 255 {
						continue
					}
					at := paint2d.Pt(float32(x)+.5, float32(y)+.5)
					covered := false
					for _, r := range overlays {
						if r.Contains(at) {
							covered = true
							break
						}
					}
					for _, c := range p.body.Children() {
						if c.Visible() && widget.DeviceBounds(c).Contains(at) {
							covered = true
							break
						}
					}
					if covered {
						continue
					}
					got := capture.NRGBAAt(x, y).A
					if abs(int(got)-int(alpha)) > 1 {
						t.Fatalf("soft edge%d,%d alpha=%d, art=%d", x, y, got, alpha)
					}
					checked++
				}
			}
			if checked < 10 {
				t.Fatal("fixture lacks uncovered soft-alpha edge pixels")
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
