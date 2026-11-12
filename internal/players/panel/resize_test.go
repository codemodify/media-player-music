package panel

import (
	"fmt"
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/codemodify/media-player-music/internal/players"
	"github.com/codemodify/media-player-music/internal/players/playertest"
	"goframework.io/desktop-ui"
	"goframework.io/desktop-ui/a11y"
	"goframework.io/desktop-ui/platform"
	"goframework.io/desktop-ui/widget"
	"goframework.io/graphics-paint2d"
)

func TestPlaylistResizeKeepsControlsUsableAtEveryScale(t *testing.T) {
	for _, skin := range Skins {
		for _, scale := range []float32{1, 1.5, 1.75} {
			t.Run(fmt.Sprintf("%s/%gx", skin, scale), func(t *testing.T) {
				a, p := open(t, skin, scale)
				var tracks []players.Track
				for i := 0; i < 100; i++ {
					tracks = append(tracks, players.Track{Title: fmt.Sprintf("Track %d", i+1), Length: time.Minute})
				}
				p.Transport.List = players.NewPlaylist(tracks...)
				p.Refresh()
				a.PumpOnce()
				p.Advance(0)
				if !p.List().Resizable() || p.Main().Resizable() || p.Eq().Resizable() {
					t.Fatal("resize policy must permit the playlist and keep the other panels fixed")
				}
				limits := platform.SurfaceSizeLimits(p.List().Surface())
				if limits.MinWidth != StripW || limits.MinHeight != ListH || limits.MaxWidth != 0 || limits.MaxHeight != 0 {
					t.Fatalf("playlist size limits %+v", limits)
				}
				off := p.List().Surface().(*platform.Offscreen)
				box := p.List().WindowRect()
				// The toolkit puts the band inside a shaped skin and in the
				// shadow margin of a rectangular frame. Find its public hit.
				at, resizeHit := paint2d.Point{}, false
				for y := box.Max.Y - 3*scale; y < box.Max.Y+6*scale; y++ {
					candidate := paint2d.Pt(box.Center().X, y)
					if region, edge := p.List().NonClientHit(candidate); region == ui.RegionResize && edge == platform.EdgeBottom {
						at, resizeHit = candidate, true
						break
					}
				}
				if !resizeHit {
					t.Fatal("the playlist has no toolkit resize hit along its bottom edge")
				}
				p.List().Inject(platform.Event{Kind: platform.EventMouseDown, Button: platform.ButtonLeft, Pos: at})
				p.List().Inject(platform.Event{Kind: platform.EventMouseUp, Button: platform.ButtonLeft, Pos: at})
				a.PumpOnce()
				if calls := off.FrameCalls().Resizes; len(calls) != 1 || calls[0] != platform.EdgeBottom {
					t.Fatalf("edge press did not start one toolkit resize: %v", calls)
				}
				l := p.listPane
				beforeBox, beforeRows, beforeOffset := l.LocalBounds(), l.list.geo.rows, l.list.MaxOffset()
				beforeAdd, beforeOpts, beforeMini, beforeInfo := l.add.Bounds(), l.opts.Bounds(), l.mini[1].Bounds(), l.foot.Bounds()
				beforeVisible := visiblePlaylistRows(l.list)
				p.List().SetSize(510, 410)
				a.PumpOnce()
				p.Advance(0)
				a.PumpOnce()
				afterBox := l.LocalBounds()
				if l.list.geo.rows.Dx() <= beforeRows.Dx() || l.list.geo.rows.Dy() <= beforeRows.Dy() ||
					l.list.MaxOffset() >= beforeOffset || visiblePlaylistRows(l.list) <= beforeVisible {
					t.Fatal("enlarging the playlist did not expand the visible scrollable rows")
				}
				near := func(a, b float32) bool { return math.Abs(float64(a-b)) <= 1 }
				for _, pair := range [][2]float32{
					{l.add.Bounds().Min.X, beforeAdd.Min.X},
					{afterBox.Max.Y - l.add.Bounds().Max.Y, beforeBox.Max.Y - beforeAdd.Max.Y},
					{afterBox.Max.X - l.opts.Bounds().Max.X, beforeBox.Max.X - beforeOpts.Max.X},
					{afterBox.Max.Y - l.opts.Bounds().Max.Y, beforeBox.Max.Y - beforeOpts.Max.Y},
					{afterBox.Max.X - l.mini[1].Bounds().Max.X, beforeBox.Max.X - beforeMini.Max.X},
					{l.mini[1].Bounds().Dx(), beforeMini.Dx()},
					{afterBox.Max.X - l.foot.Bounds().Max.X, beforeBox.Max.X - beforeInfo.Max.X},
				} {
					if !near(pair[0], pair[1]) {
						t.Fatalf("resizing moved or stretched a fixed footer control: got %g, before %g", pair[0], pair[1])
					}
				}
				for _, control := range []widget.Component{l.add, l.rem, l.sel, l.misc, l.opts, l.mini[0], l.mini[1], l.mini[5], l.foot, l.list} {
					if bounds := widget.DeviceBounds(control); !bounds.Empty() &&
						(bounds.Min.X < p.List().WindowRect().Min.X-1 || bounds.Min.Y < p.List().WindowRect().Min.Y-1 ||
							bounds.Max.X > p.List().WindowRect().Max.X+1 || bounds.Max.Y > p.List().WindowRect().Max.Y+1) {
						t.Fatalf("control %T extends outside the resized playlist: %v", control, bounds)
					}
				}
				p.List().RequestFocus(l.list)
				p.List().Inject(platform.Event{Kind: platform.EventKeyDown, Key: platform.KeyEnd})
				a.PumpOnce()
				last := l.list.AccessibleItems()[99]
				if last.State&a11y.StateOffscreen != 0 || l.list.OffsetY <= 0 || l.list.Selected != 99 || p.Transport.List.Index() != 0 {
					t.Fatal("End did not scroll to and select the last row without changing playback")
				}
				p.List().Inject(platform.Event{Kind: platform.EventKeyDown, Key: platform.KeyReturn})
				a.PumpOnce()
				if p.Transport.List.Index() != 99 {
					t.Fatal("Return did not play the selected row in the resized viewport")
				}
				at = widget.DeviceBounds(l.mini[0]).Center()
				p.List().Inject(platform.Event{Kind: platform.EventMouseDown, Button: platform.ButtonLeft, Pos: at})
				p.List().Inject(platform.Event{Kind: platform.EventMouseUp, Button: platform.ButtonLeft, Pos: at})
				a.PumpOnce()
				if p.Transport.List.Index() != 98 {
					t.Fatal("the moved footer transport no longer handles pointer input")
				}
				assertPlaylistWellColour(t, p, skin, scale)
			})
		}
	}
}

func TestResizedPlaylistRestoresAcrossSkinsAndRemainsDocked(t *testing.T) {
	for _, skin := range []string{Skin, SkinSilver} {
		t.Run(skin, func(t *testing.T) {
			a, p := open(t, skin, 1.75)
			p.Advance(0)
			p.List().SetSize(540, 420)
			playertest.Follow(a)
			before := p.g.Rack.Boxes()
			saved := placementJSONRoundTrip(t, p.CapturePlacement())
			if size := saved["playlist"]; size.Width != 540 || size.Height != 420 || size.Dock == nil {
				t.Fatalf("resized playlist did not save its dimensions and dock: %+v", size)
			}
			for _, appearance := range []string{skin, otherPanelSkin(skin)} {
				p.Close()
				p.Host.ApplySkin(appearance)
				var err error
				p, err = New(p.Host, Options{Headless: true, Scale: 1.75})
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(p.Close)
				p.RestorePlacement(saved)
				if immediately := p.CapturePlacement()["playlist"]; immediately.Width != 540 || immediately.Height != 420 {
					t.Fatal("capturing before the first frame discarded the restored playlist size")
				}
				playertest.SettleGroup(t, a, p.g)
				if w, h := p.List().Size(); w != 540 || h != 420 {
					t.Fatalf("%s restored playlist %dx%d, want 540x420", appearance, w, h)
				}
				if got := p.g.Rack.Boxes(); !reflect.DeepEqual(got, before) {
					t.Fatalf("restoring the larger playlist changed its docking: %+v, want %+v", got, before)
				}
				dockingMove(t, p, p.iMain, before[p.iMain].X+65, before[p.iMain].Y+35)
				dockingAssertTranslated(t, p, before, 65, 35)
				dockingMove(t, p, p.iMain, before[p.iMain].X, before[p.iMain].Y)
			}
			p.RestorePlacement(map[string]players.WindowPlacement{"playlist": {Width: 20, Height: 10}})
			playertest.Follow(a)
			if w, h := p.List().Size(); w != StripW || h != ListH {
				t.Fatalf("invalid saved dimensions did not clamp to the control minimum: %dx%d", w, h)
			}
		})
	}
}

func TestPlaylistStartupResizeSurvivesInitialDesktopConfiguration(t *testing.T) {
	for _, skin := range []string{Skin, SkinSilver} {
		t.Run(skin, func(t *testing.T) {
			a, previous := open(t, skin, 1.75)
			previous.Advance(0)
			saved := previous.CapturePlacement()
			playlist := saved["playlist"]
			playlist.Width, playlist.Height = 510, 410
			saved["playlist"] = playlist
			previous.Close()
			p, err := New(previous.Host, Options{Headless: true, Scale: 1.75})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(p.Close)
			p.RestorePlacement(saved)
			// A native window may receive its initial map/configure after
			// the app requested the saved size. Reproduce that late baseline
			// response through the same public event the backend delivers.
			p.List().Inject(platform.Event{Kind: platform.EventResize, Width: StripW, Height: ListH})
			for range 3 {
				a.PumpOnce()
				p.Advance(0)
			}
			playertest.SettleGroup(t, a, p.g)
			if w, h := p.List().Size(); w != 510 || h != 410 {
				t.Fatalf("initial desktop configuration replaced the restored size: %dx%d", w, h)
			}
			got := p.CapturePlacement()["playlist"]
			if got.Width != 510 || got.Height != 410 || !reflect.DeepEqual(got.Dock, playlist.Dock) {
				t.Fatalf("restoration discarded dimensions or docking: %+v", got)
			}
			desktopMove(a, p.List(), 900, 450)
			if got := p.CapturePlacement()["playlist"]; got.X != 900 || got.Y != 450 || got.Dock != nil {
				t.Fatal("completed startup restoration fought a later satellite drag")
			}
		})
	}
}

func visiblePlaylistRows(list *tracks) int {
	count := 0
	for _, row := range list.AccessibleItems() {
		if row.State&a11y.StateOffscreen == 0 {
			count++
		}
	}
	return count
}

func assertPlaylistWellColour(t *testing.T, p *Player, skin string, scale float32) {
	t.Helper()
	image := p.List().Capture()
	rows := p.listPane.list.geo.rows
	point := widget.LocalToWindow(p.listPane.list, paint2d.XYWH(rows.Max.X-60*scale, rows.Max.Y-4*scale, 1, 1)).Min
	point = point.Sub(p.List().WindowRect().Min)
	r, g, b, _ := image.PremulAt(int(point.X), int(point.Y))
	if skin == Skin && (r > 40 || g > 40 || b > 40) {
		t.Fatalf("expanded row well is chrome rather than black: %d,%d,%d", r, g, b)
	}
	if skin == SkinSilver && (b < 70 || int(b) < int(r)+20) {
		t.Fatalf("expanded row well is chrome rather than blue: %d,%d,%d", r, g, b)
	}
}
