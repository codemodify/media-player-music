package itunes

import (
	"fmt"
	"math"
	"os"
	"reflect"
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
	t.Setenv(style.ThemeEnv, players.ITunesSkin)
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

func TestFullMiniResizeLimitsAndCachedSizesKeepTheSameWindows(t *testing.T) {
	for _, scale := range []float32{1, 1.75, 2} {
		t.Run(fmt.Sprintf("%gx", scale), func(t *testing.T) {
			a, p := open(t, scale)
			main, eq := p.Main, p.Eq
			off := eq.Surface().(*platform.Offscreen)
			if main.Title() != "Music Player" || len(p.Windows()) != 2 || p.Windows()[1].Name != "equaliser" || eq.Resizable() || off.WindowRole() != platform.RoleUtility || off.Owner() != main.Surface() || !off.SkipsTaskbar() {
				t.Fatal("main/EQ window policy changed")
			}
			if p.MiniPlayer() || !main.Resizable() {
				t.Fatal("initial mode is not a resizable full player")
			}
			main.SetSize(940, 650)
			main.Move(180, 230)
			a.PumpOnce()
			p.Transport.State, p.Transport.Pos = players.Paused, 17*time.Second
			for cycle := 0; cycle < 3; cycle++ {
				p.SetMiniPlayer(true)
				a.PumpOnce()
				p.Advance(0)
				if p.Main != main || p.Eq != eq || len(a.Windows()) != 2 || !main.Resizable() || !p.MiniPlayer() {
					t.Fatal("mode switch replaced windows or fixed MiniPlayer width")
				}
				sizeW, sizeH := main.Size()
				if cycle == 0 && sizeW != MiniW || sizeH != MiniH {
					t.Fatalf("mini size %dx%d", sizeW, sizeH)
				}
				limits := platform.SurfaceSizeLimits(main.Surface())
				if limits != (platform.SizeLimits{MinWidth: 150, MinHeight: 62, MaxWidth: 1000, MaxHeight: 62}) {
					t.Fatalf("mini is not horizontally resizable with native height pin: %+v", limits)
				}
				for _, width := range []int{150, 380, 430} {
					main.SetSize(width, MiniH)
					a.PumpOnce()
					p.Advance(0)
					pw, ph := main.PixelSize()
					if pw != int(math.Round(float64(width)*float64(scale))) || ph != int(math.Round(float64(MiniH)*float64(scale))) {
						t.Fatal("mini logical/device sizes disagree")
					}
					for _, c := range main.Content().Children() {
						if c.Visible() {
							r := c.Bounds()
							if r.Empty() || r.Min.X < 0 || r.Min.Y < 0 || r.Max.X > float32(pw)+1 || r.Max.Y > float32(ph)+1 {
								t.Fatalf("%dpx mini clips %T: %v", width, c, r)
							}
						}
					}
					playertest.Audit(t, "MiniPlayer", main)
				}
				state := p.CaptureState()
				if state.Mode != "mini" || state.Sizes["mini"] != (players.PresentationSize{Width: 430, Height: 62}) || state.Sizes["full"] != (players.PresentationSize{Width: 940, Height: 650}) {
					t.Fatalf("mode cache drifted: %+v", state)
				}
				// A caller changing its saved snapshot cannot poison the player cache.
				state.Sizes["full"] = players.PresentationSize{Width: 12, Height: 9}
				p.SetMiniPlayer(false)
				a.PumpOnce()
				p.Advance(0)
				w, h := main.Size()
				limits = platform.SurfaceSizeLimits(main.Surface())
				if w != 940 || h != 650 || p.MiniPlayer() || limits != (platform.SizeLimits{MinWidth: 640, MinHeight: 400}) {
					t.Fatalf("full size/limits lost: %dx%d %+v", w, h, limits)
				}
				x, y, known := main.Position()
				if !known || x != 180 || y != 230 {
					t.Fatal("mode switch changed Main location")
				}
				if p.Transport.State != players.Paused || p.Transport.Pos != 17*time.Second || off.Owner() != main.Surface() {
					t.Fatal("mode switch changed playback or utility owner")
				}
			}
		})
	}
}

func TestModeAcknowledgementDoesNotUndoAnImmediateUserResize(t *testing.T) {
	a, p := open(t, 1.75)
	p.SetMiniPlayer(true)
	a.PumpOnce()
	// The next width request deliberately happens before Player.Advance and
	// without the application's native-drag helper, like a WM resize action.
	p.Main.SetSize(430, MiniH)
	a.PumpOnce()
	p.Advance(70 * time.Millisecond)
	a.PumpOnce()
	if w, h := p.Main.Size(); w != 430 || h != MiniH {
		t.Fatalf("stale mode target undid user width: %dx%d", w, h)
	}
	if got := p.CaptureState().Sizes["mini"]; got != (players.PresentationSize{Width: 430, Height: MiniH}) {
		t.Fatalf("snapshot retained old target: %+v", got)
	}
}

func TestModeStateAndPlacementRestoreBeforeFirstLayout(t *testing.T) {
	_, p := open(t, 1.75)
	state := players.PresentationState{Mode: "mini", Sizes: map[string]players.PresentationSize{"full": {Width: 940, Height: 650}, "mini": {Width: 430, Height: 62}}}
	placement := map[string]players.WindowPlacement{"main": {X: 220, Y: 310, Positioned: true, Width: 430, Height: 62}, "equaliser": {X: 800, Y: 500, Positioned: true, Width: 900, Height: 600}}
	// Make a fresh presentation and apply shell's actual startup ordering.
	a := ui.New(ui.Options{Headless: true, Scale: 1.75, DisableLookWatch: true})
	next, err := New(players.NewHost(a), Options{Headless: true, Scale: 1.75})
	if err != nil {
		t.Fatal(err)
	}
	defer next.Close()
	next.RestoreState(state)
	next.RestorePlacement(placement)
	if got := next.CaptureState(); !reflect.DeepEqual(got, state) {
		t.Fatalf("pre-layout state lost: %+v", got)
	}
	for i := 0; i < 5; i++ {
		a.PumpOnce()
		next.Advance(70 * time.Millisecond)
	}
	if next.Main == p.Main || !next.MiniPlayer() {
		t.Fatal("fresh startup mode missing")
	}
	got := next.CapturePlacement()
	if got["main"] != placement["main"] || got["equaliser"].X != 800 || got["equaliser"].Y != 500 || got["equaliser"].Width != EqW || got["equaliser"].Height != EqH {
		t.Fatalf("restored mode/position or fixed EQ size lost: %+v", got)
	}
	next.SetMiniPlayer(false)
	a.PumpOnce()
	next.Advance(0)
	if w, h := next.Main.Size(); w != 940 || h != 650 {
		t.Fatal("startup MiniPlayer forgot full window dimensions")
	}
	next.RestoreState(players.PresentationState{Mode: "invalid", Sizes: map[string]players.PresentationSize{"full": {Width: 20, Height: 10}, "mini": {Width: 2000, Height: 300}}})
	a.PumpOnce()
	gotState := next.CaptureState()
	if gotState.Mode != "full" || gotState.Sizes["full"] != (players.PresentationSize{Width: MinW, Height: MinH}) || gotState.Sizes["mini"] != (players.PresentationSize{Width: MaxMiniW, Height: MiniH}) {
		t.Fatalf("mode cache validation failed: %+v", gotState)
	}
}

func TestMainAndEqualizerMasksStayActiveAcrossModesAndScales(t *testing.T) {
	for _, scale := range []float32{1, 1.75, 2} {
		t.Run(fmt.Sprintf("%gx", scale), func(t *testing.T) {
			a, p := open(t, scale)
			for _, mini := range []bool{false, true, false} {
				p.SetMiniPlayer(mini)
				a.PumpOnce()
				p.Advance(0)
				for _, w := range []*ui.Window{p.Main, p.Eq} {
					width, height := w.PixelSize()
					frame := w.Surface().(*platform.Offscreen).Frame()
					shape := w.Shape()
					if shape == nil || !w.ShapeActive() || !frame.Alpha || len(frame.Shape) == 0 || shape.Raster(width, height).Clamped {
						t.Fatal("mode/scale lost shaped alpha window")
					}
					capture := w.CaptureSurface()
					if capture.NRGBAAt(0, 0).A != 0 || shape.Raster(width, height).Contains(0, 0) {
						t.Fatal("source empty corner is painted or participates in its visual mask")
					}
					playertest.Audit(t, w.Title(), w)
				}
			}
		})
	}
}

func TestModeShortcutsPreserveTransportAndMoveFocusToVisibleContent(t *testing.T) {
	a, p := open(t, 1.75)
	calls := 0
	p.Host.Run = func(c players.Command) { calls++; p.Transport.Do(c) }
	click(p.Main, platform.ButtonLeft, widget.DeviceBounds(p.body.play).Center())
	a.PumpOnce()
	if p.Transport.State != players.Playing || calls != 1 {
		t.Fatal("full Play did not act exactly once")
	}
	key(a, p.Main, platform.KeyM, platform.ModCtrl|platform.ModShift)
	if !p.MiniPlayer() || p.Main.Focus() != p.mini.play || p.Transport.State != players.Playing || calls != 1 {
		t.Fatal("mini shortcut reset playback or focused hidden full content")
	}
	if !p.Main.AccessibleAction(p.mini.play.ID(), a11y.ActionDefault) || p.Transport.State != players.Paused || calls != 2 {
		t.Fatal("mini accessible Pause did not use shared transport")
	}
	p.ShowPlaylist(false)
	key(a, p.Main, platform.KeyF, platform.ModCtrl)
	if p.MiniPlayer() || !p.Host.Model.PlaylistVisible || p.Main.Focus() != p.body.search || calls != 2 {
		t.Fatal("Find failed to expand the hidden library and focus search without changing playback")
	}
	key(a, p.Main, platform.KeyB, 0)
	if calls != 2 {
		t.Fatal("typing a transport letter in search activated playback")
	}
	key(a, p.Main, platform.KeyE, platform.ModCtrl)
	if p.Eq.Visible() {
		t.Fatal("CtrlE failed to hide utility EQ")
	}
	key(a, p.Main, platform.KeyE, platform.ModCtrl)
	if !p.Eq.Visible() {
		t.Fatal("CtrlE failed to reopen utility EQ")
	}
	p.Eq.RequestClose()
	a.PumpOnce()
	if p.Eq.Closed() || p.Host.Model.EqualizerVisible || p.Eq.Visible() {
		t.Fatal("utility close didn't preserve preference and reopenable window")
	}
}

func TestEqualizerUsesSharedHooks(t *testing.T) {
	_, p := open(t, 1.75)
	changes := 0
	refresh := p.Equaliser.Changed
	p.Equaliser.Changed = func() {
		changes++
		if refresh != nil {
			refresh()
		}
	}
	if len(p.eqPane.bands) != len(players.EqBands) {
		t.Fatal("source equalizer lacks ten controls")
	}
	band := p.eqPane.bands[3]
	if !p.Eq.AccessibleAction(band.ID(), a11y.ActionIncrement) || changes != 1 || p.Equaliser.Gains[3] <= 0 {
		t.Fatal("equalizer accessibility input did not use shared hook")
	}
	if !band.Enabled() {
		t.Fatal("the equalizer band is not enabled")
	}
}

func TestNarrowMiniPlayerHidesLCDAndMovesFocusToVisibleTransport(t *testing.T) {
	a, p := open(t, 1.75)
	p.SetMiniPlayer(true)
	p.Main.SetSize(430, MiniH)
	a.PumpOnce()
	if !p.mini.information || !p.mini.seek.Visible() {
		t.Fatal("wide original MiniPlayer LCD/seek is missing")
	}
	p.Main.RequestFocus(p.mini.seek)
	p.Main.SetSize(MinMiniW, MiniH)
	a.PumpOnce()
	if p.mini.information || p.mini.seek.Visible() {
		t.Fatal("narrow original MiniPlayer did not fold its LCD")
	}
	if p.Main.Focus() != p.mini.play {
		t.Fatal("folding LCD left keyboard focus on hidden Seek")
	}
	if p.Main.AccessibleTree().Find(p.mini.seek.ID()) != nil {
		t.Fatal("folded Seek is still advertised to accessibility")
	}
}
