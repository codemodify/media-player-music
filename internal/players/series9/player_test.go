package series9

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
	t.Setenv(style.ThemeEnv, players.Series9Skin)
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
func settle(a *ui.Application, p *Player) {
	for i := 0; i < 3; i++ {
		a.PumpOnce()
		p.Advance(70 * time.Millisecond)
	}
}

func TestDrawersAndCompactKeepBaseSizePositionAndOneWindow(t *testing.T) {
	for _, scale := range []float32{1, 1.75, 2} {
		t.Run(fmt.Sprintf("%gx", scale), func(t *testing.T) {
			a, p := open(t, scale)
			main, surface := p.Main, p.Main.Surface()
			if len(p.Windows()) != 1 || len(a.Windows()) != 1 || main.Title() != "Music Player" || !main.Resizable() {
				t.Fatal("Series 9 is not one resizable main window")
			}
			main.Move(170, 210)
			p.ShowPlaylist(false)
			p.ShowEqualiser(false)
			settle(a, p)
			main.SetSize(450, 400)
			settle(a, p)
			p.Transport.State, p.Transport.Pos = players.Paused, 17*time.Second
			for _, flags := range []struct{ playlist, eq bool }{{true, false}, {true, true}, {false, true}, {false, false}, {true, true}} {
				p.ShowPlaylist(flags.playlist)
				p.ShowEqualiser(flags.eq)
				settle(a, p)
				width, height := 450, 400
				minWidth, minHeight := MinW, MinH
				if flags.playlist {
					width += 250
					minWidth += 250
				}
				if flags.eq {
					height += 124
					minHeight += 124
				}
				if w, h := main.Size(); w != width || h != height {
					t.Fatalf("drawer geometry = %dx%d, want %dx%d", w, h, width, height)
				}
				if p.CaptureState().Sizes[modeFull] != (players.PresentationSize{Width: 450, Height: 400}) {
					t.Fatal("drawer travel changed remembered base dimensions")
				}
				if limits := platform.SurfaceSizeLimits(surface); limits != (platform.SizeLimits{MinWidth: minWidth, MinHeight: minHeight}) {
					t.Fatalf("drawer native limits = %+v", limits)
				}
				pw, ph := main.PixelSize()
				if pw != int(math.Round(float64(width)*float64(scale))) || ph != int(math.Round(float64(height)*float64(scale))) {
					t.Fatal("logical and device dimensions disagree")
				}
				playertest.Audit(t, "Series 9 full", main)
			}
			for cycle := 0; cycle < 3; cycle++ {
				p.SetCompact(true)
				settle(a, p)
				if !p.Compact() || main.Resizable() || !p.Host.Model.EqualizerVisible || !p.Host.Model.PlaylistVisible {
					t.Fatal("compact mode did not suspend remembered drawers")
				}
				if w, h := main.Size(); w != CompactW || h != CompactH {
					t.Fatalf("compact dimensions %dx%d", w, h)
				}
				if limits := platform.SurfaceSizeLimits(surface); limits != (platform.SizeLimits{MinWidth: CompactW, MinHeight: CompactH, MaxWidth: CompactW, MaxHeight: CompactH}) {
					t.Fatalf("compact native limits %+v", limits)
				}
				state := p.CaptureState()
				if state.Mode != modeCompact || state.Sizes[modeFull] != (players.PresentationSize{Width: 450, Height: 400}) {
					t.Fatalf("compact lost full cache: %+v", state)
				}
				state.Sizes[modeFull] = players.PresentationSize{Width: 10, Height: 10}
				playertest.Audit(t, "Series 9 compact", main)
				p.SetCompact(false)
				settle(a, p)
				if w, h := main.Size(); w != 700 || h != 524 {
					t.Fatalf("restored full dimensions %dx%d", w, h)
				}
				if p.Main != main || p.Main.Surface() != surface || len(a.Windows()) != 1 {
					t.Fatal("mode change replaced the native window")
				}
				if x, y, known := main.Position(); !known || x != 170 || y != 210 {
					t.Fatal("mode or drawer change moved the main window")
				}
				if p.Transport.State != players.Paused || p.Transport.Pos != 17*time.Second {
					t.Fatal("mode change altered playback")
				}
			}
		})
	}
}

func TestCompactStateAndPlacementRestoreBeforeFirstLayout(t *testing.T) {
	_, previous := open(t, 1.75)
	a := ui.New(ui.Options{Headless: true, Scale: 1.75, DisableLookWatch: true})
	p, err := New(players.NewHost(a), Options{Headless: true, Scale: 1.75})
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	state := players.PresentationState{Mode: modeCompact, Sizes: map[string]players.PresentationSize{modeFull: {Width: 450, Height: 400}, modeCompact: {Width: CompactW, Height: CompactH}}}
	placement := map[string]players.WindowPlacement{"main": {X: 220, Y: 310, Positioned: true, Width: CompactW, Height: CompactH}}
	p.RestoreState(state)
	p.RestorePlacement(placement)
	if got := p.CaptureState(); !reflect.DeepEqual(got, state) {
		t.Fatalf("pre-layout state drifted: %+v", got)
	}
	settle(a, p)
	if p.Main == previous.Main || !p.Compact() || !reflect.DeepEqual(p.CapturePlacement(), placement) {
		t.Fatalf("startup mode/placement failed: %+v", p.CapturePlacement())
	}
	p.ShowPlaylist(true)
	settle(a, p)
	if p.Compact() {
		t.Fatal("opening a drawer did not return to full mode")
	}
	if w, h := p.Main.Size(); w != 700 || h != 524 {
		t.Fatal("startup compact mode forgot the full base dimensions")
	}
	p.SetCompact(true)
	settle(a, p)
	p.ShowEqualiser(true)
	settle(a, p)
	if p.Compact() {
		t.Fatal("opening EQ did not return to full mode")
	}
	p.RestoreState(players.PresentationState{Mode: "invalid", Sizes: map[string]players.PresentationSize{modeFull: {Width: 20, Height: 10}, modeCompact: {Width: 900, Height: 600}}})
	settle(a, p)
	if got := p.CaptureState(); got.Mode != modeFull || got.Sizes[modeFull] != (players.PresentationSize{Width: MinW, Height: MinH}) || got.Sizes[modeCompact] != (players.PresentationSize{Width: CompactW, Height: CompactH}) {
		t.Fatalf("invalid saved state was not normalized: %+v", got)
	}
}

func TestAcknowledgedDrawerResizeDoesNotUndoLaterUserResize(t *testing.T) {
	a, p := open(t, 1.75)
	p.ShowPlaylist(false)
	settle(a, p)
	p.ShowPlaylist(true)
	a.PumpOnce()
	p.Main.SetSize(760, 560)
	a.PumpOnce()
	p.Advance(70 * time.Millisecond)
	a.PumpOnce()
	if w, h := p.Main.Size(); w != 760 || h != 560 {
		t.Fatalf("stale drawer target undid user size: %dx%d", w, h)
	}
	if got := p.CaptureState().Sizes[modeFull]; got != (players.PresentationSize{Width: 510, Height: 436}) {
		t.Fatalf("user base size was lost: %+v", got)
	}
}

func TestDelayedPriorResizeDoesNotUndoDrawerTransition(t *testing.T) {
	a, p := open(t, 1.75)
	p.ShowPlaylist(false)
	p.ShowEqualiser(false)
	a.PumpOnce()
	// The backing image is already at the final requested size. A late
	// desktop answer for the earlier EQ-open request remains legal, but
	// is not a new user request and must not poison the cached base size.
	p.Main.Inject(platform.Event{Kind: platform.EventResize, Width: MainW, Height: MainH + EqualizerTravel})
	a.PumpOnce()
	p.Advance(70 * time.Millisecond)
	a.PumpOnce()
	if w, h := p.Main.Size(); w != MainW || h != MainH {
		t.Fatalf("late earlier request replaced the final drawer size: %dx%d", w, h)
	}
	if got := p.CaptureState().Sizes[modeFull]; got != (players.PresentationSize{Width: MainW, Height: MainH}) {
		t.Fatalf("late earlier request changed the full base cache: %+v", got)
	}
	p.ShowPlaylist(true)
	a.PumpOnce()
	p.Main.Inject(platform.Event{Kind: platform.EventResize, Width: MainW, Height: MainH})
	a.PumpOnce()
	p.Advance(70 * time.Millisecond)
	a.PumpOnce()
	if w, h := p.Main.Size(); w != MainW+PlaylistTravel || h != MainH {
		t.Fatalf("late closed drawer reply undid opening: %dx%d", w, h)
	}
	// A distinct legal resize belongs to the user and ends correction.
	p.Main.SetSize(760, 400)
	a.PumpOnce()
	p.Advance(70 * time.Millisecond)
	a.PumpOnce()
	if w, h := p.Main.Size(); w != 760 || h != 400 || p.modeResize != nil {
		t.Fatal("drawer correction fought a subsequent legal user resize")
	}
}

func TestOneWindowMasksFollowDrawerAndCompactStates(t *testing.T) {
	for _, scale := range []float32{1, 1.75, 2} {
		t.Run(fmt.Sprintf("%gx", scale), func(t *testing.T) {
			a, p := open(t, scale)
			for _, flags := range []struct{ playlist, eq bool }{{false, false}, {true, false}, {false, true}, {true, true}} {
				p.ShowPlaylist(flags.playlist)
				p.ShowEqualiser(flags.eq)
				settle(a, p)
				assertMask(t, p)
			}
			p.SetCompact(true)
			settle(a, p)
			assertMask(t, p)
			p.SetCompact(false)
			settle(a, p)
			assertMask(t, p)
		})
	}
}
func assertMask(t *testing.T, p *Player) {
	t.Helper()
	w := p.Main
	width, height := w.PixelSize()
	shape := w.Shape()
	frame := w.Surface().(*platform.Offscreen).Frame()
	if shape == nil || !w.ShapeActive() || !frame.Alpha || len(frame.Shape) == 0 || shape.Raster(width, height).Clamped {
		t.Fatal("window lost its full-size source mask")
	}
	image := w.CaptureSurface()
	if image == nil || image.NRGBAAt(0, 0).A != 0 || shape.Raster(width, height).Contains(0, 0) {
		t.Fatal("empty original corner is painted or part of the visual mask")
	}
	if len(shape.Raster(width, height).Rects) < 2 {
		t.Fatal("source mask degraded to a rectangle")
	}
}

func TestTransportAndCompactShortcutsUseSharedActions(t *testing.T) {
	a, p := open(t, 1.75)
	calls := 0
	p.Host.Run = func(command players.Command) { calls++; p.Transport.Do(command) }
	click(p.Main, platform.ButtonLeft, widget.DeviceBounds(p.body.play).Center())
	a.PumpOnce()
	if calls != 1 || p.Transport.State != players.Playing {
		t.Fatal("full Play did not act once through the host")
	}
	key(a, p.Main, platform.KeyM, platform.ModCtrl|platform.ModShift)
	if !p.Compact() || p.Main.Focus() != p.compact.play || calls != 1 || p.Transport.State != players.Playing {
		t.Fatal("compact shortcut changed playback or focused hidden content")
	}
	if !p.Main.AccessibleAction(p.compact.play.ID(), a11y.ActionDefault) || calls != 2 || p.Transport.State != players.Paused {
		t.Fatal("compact accessible Play/Pause failed")
	}
	key(a, p.Main, platform.KeySpace, 0)
	if calls != 3 || p.Transport.State != players.Playing {
		t.Fatal("transport shortcut did not bubble from compact control")
	}
	p.ShowEqualiser(false)
	key(a, p.Main, platform.KeyE, platform.ModCtrl)
	if p.Compact() || !p.Host.Model.EqualizerVisible {
		t.Fatal("CtrlE did not reveal the EQ drawer from compact mode")
	}
	key(a, p.Main, platform.KeyL, platform.ModCtrl)
	if p.Host.Model.PlaylistVisible {
		t.Fatal("CtrlL did not hide the remembered playlist")
	}
}

func TestOriginalEqualizerPositionsSharedInputAndCapabilityLimits(t *testing.T) {
	for _, scale := range []float32{1, 1.75} {
		t.Run(fmt.Sprintf("%gx", scale), func(t *testing.T) {
			a, p := open(t, scale)
			// Corona.js GetEqSliderLeft and the WMS top-middle offset give
			// these ten design positions at the original334px drawer width.
			originalX := []float32{15, 46, 78, 109, 141, 172, 204, 235, 267, 298}
			if len(p.body.eq.bands) != len(originalX) {
				t.Fatal("original ten-band equalizer controls are missing")
			}
			for i, band := range p.body.eq.bands {
				actual := widget.DeviceBounds(band)
				want := paint2d.XYWH((262+originalX[i])*scale, 318*scale, 18*scale, 67*scale)
				if math.Abs(float64(actual.Min.X-want.Min.X)) > 1 || math.Abs(float64(actual.Min.Y-want.Min.Y)) > 1 || math.Abs(float64(actual.Dx()-want.Dx())) > 1 || math.Abs(float64(actual.Dy()-want.Dy())) > 1 {
					t.Fatalf("band%d moved from original WMS geometry: actual%v want%v", i, actual, want)
				}
			}
			changes := 0
			refresh := p.Equaliser.Changed
			p.Equaliser.Changed = func() {
				changes++
				if refresh != nil {
					refresh()
				}
			}
			band := p.body.eq.bands[3]
			if !p.Main.AccessibleAction(band.ID(), a11y.ActionIncrement) || changes != 1 || p.Equaliser.Gains[3] <= 0 {
				t.Fatal("source equalizer accessibility did not act once on the shared model")
			}
			gain := p.Equaliser.Gains[3]
			p.Main.RequestFocus(band)
			p.ShowEqualiser(false)
			settle(a, p)
			if p.Main.Focus() != p.body.play || p.Main.AccessibleTree().Find(band.ID()) != nil {
				t.Fatal("hidden EQ drawer retained keyboard/accessibility focus")
			}
			p.ShowEqualiser(true)
			settle(a, p)
			p.Host.Model.Live, p.Host.Model.BackendLimited = true, true
			p.Host.AudioBackend = "VLC"
			p.Refresh()
			a.PumpOnce()
			if band.Enabled() || p.body.eq.balance.Enabled() || p.body.main.analyser.Visible() || !strings.Contains(p.Host.StatusText(), "unavailable") {
				t.Fatal("unsupported EQ, balance or spectrum was offered as functional")
			}
			p.Host.Model.BackendEqualizer, p.Host.Model.BackendBalance, p.Host.Model.BackendSpectrum = true, true, true
			p.Refresh()
			a.PumpOnce()
			if !band.Enabled() || !p.body.eq.balance.Enabled() || !p.body.main.analyser.Visible() || p.Equaliser.Gains[3] != gain || changes != 1 {
				t.Fatal("restoring backend features lost intended values or emitted user input")
			}
		})
	}
}
