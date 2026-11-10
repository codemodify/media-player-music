package amarok

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
	"github.com/codemodify/media-player-music/internal/skins"
	"github.com/codemodify/media-player-music/internal/skins/plans"
	"goframework.io/desktop-ui"
	uiskins "goframework.io/desktop-ui-skins"
	"goframework.io/desktop-ui/a11y"
	"goframework.io/desktop-ui/platform"
	"goframework.io/desktop-ui/style"
	"goframework.io/desktop-ui/widget"
	"goframework.io/graphics-paint2d"
)

func TestMain(m *testing.M) {
	if err := skins.Register(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	// Standalone plan validation is useful before the shared catalogue is
	// generated. Once shipped, all UI tests consume the actual embedded pack.
	var temporary string
	if _, ok := uiskins.LoadSkin("amarok"); !ok {
		var err error
		temporary, err = os.MkdirTemp("", "music-player-amarok-test-")
		if err == nil {
			err = plans.Write(temporary, plans.Amarok())
		}
		if err == nil {
			err = uiskins.RegisterSkinFS(os.DirFS(filepath.Join(temporary, "amarok")), "amarok")
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
	code := m.Run()
	if temporary != "" {
		os.RemoveAll(temporary)
	}
	os.Exit(code)
}
func open(t *testing.T, scale float32) (*ui.Application, *Player) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv(style.ThemeEnv, "amarok")
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

func TestSingleResizableWindowScalesAllContentAndPaintsOriginalIcons(t *testing.T) {
	for _, scale := range []float32{1, 1.75} {
		t.Run(fmt.Sprintf("%gx", scale), func(t *testing.T) {
			a, p := open(t, scale)
			if p.Main.Title() != "Music Player" || len(a.Windows()) != 1 || len(p.Windows()) != 1 || !p.Main.Resizable() {
				t.Fatal("Amarok must be one resizable Music Player window")
			}
			for _, size := range [][2]int{{MainW, MainH}, {MinW, MinH}, {1100, 700}} {
				p.Main.SetSize(size[0], size[1])
				a.PumpOnce()
				p.Advance(0)
				if w, h := p.Main.Size(); w != size[0] || h != size[1] {
					t.Fatalf("logical size %dx%d", w, h)
				}
				if w, h := p.Main.PixelSize(); w != int(math.Round(float64(size[0])*float64(scale))) || h != int(math.Round(float64(size[1])*float64(scale))) {
					t.Fatal("device dimensions don't follow display scale")
				}
				root := p.body.LocalBounds()
				for _, c := range append([]widget.Component{p.body.play, p.body.search, p.body.list, p.body.eq, p.body.volume, p.body.seek}, p.body.eq.preamp, p.body.eq.balance) {
					r := c.Bounds()
					if r.Empty() || r.Min.X < root.Min.X || r.Min.Y < root.Min.Y || r.Max.X > root.Max.X+.5 || r.Max.Y > root.Max.Y+.5 {
						t.Fatalf("%T bounds %v outside %v", c, r, root)
					}
				}
				geometry := p.body.list.RowGeo(p.body.list.Look())
				if geometry.Rows.Max.X > geometry.Bar.Min.X || geometry.Rows.Empty() || geometry.RowHeight < 18*scale {
					t.Fatal("playlist rows/scrollbar overlap")
				}
				for _, band := range p.body.eq.bands {
					if !p.body.eq.LocalBounds().Contains(band.Bounds().Center()) {
						t.Fatal("EQ fader clipped")
					}
				}
				image := p.Main.CaptureSurface()
				if image == nil {
					t.Fatal("missing capture")
				}
				bluePixels := 0
				r := widget.DeviceBounds(p.body.play)
				origin := widget.DeviceOrigin(p.body)
				for y := int(r.Min.Y - origin.Y); y < int(r.Max.Y-origin.Y); y++ {
					for x := int(r.Min.X - origin.X); x < int(r.Max.X-origin.X); x++ {
						c := image.NRGBAAt(x, y)
						if c.B > 100 && int(c.B) > int(c.R)+30 {
							bluePixels++
						}
					}
				}
				if bluePixels < 10 {
					t.Fatal("original blue transport icon was not painted")
				}
				playertest.Audit(t, "Amarok", p.Main)
			}
		})
	}
}
func TestTransportSharedHooksCapabilitiesAndHiddenFocus(t *testing.T) {
	a, p := open(t, 1.75)
	plays := 0
	p.Host.Run = func(c players.Command) {
		if c == players.CmdPlay {
			plays++
		}
		p.Transport.Do(c)
	}
	click(p.Main, widget.DeviceBounds(p.body.play).Center(), platform.ButtonLeft)
	a.PumpOnce()
	if p.Transport.State != players.Playing || plays != 1 {
		t.Fatal("play icon did not start shared transport once")
	}
	key(a, p.Main, platform.KeySpace, 0)
	if p.Transport.State != players.Paused {
		t.Fatal("Space failed to pause")
	}
	key(a, p.Main, platform.KeyB, 0)
	if p.Transport.List.Index() != 1 {
		t.Fatal("next shortcut did not reach shared transport")
	}
	key(a, p.Main, platform.KeyV, 0)
	if p.Transport.State != players.Stopped {
		t.Fatal("stop shortcut failed")
	}
	updates := 0
	p.Host.SyncPlayback = func() { updates++ }
	if !p.Main.AccessibleAction(p.body.volume.ID(), a11y.ActionIncrement) || updates != 1 {
		t.Fatal("volume accessibility did not notify playback")
	}
	p.Main.RequestFocus(p.body.eq.balance)
	key(a, p.Main, platform.KeyRight, 0)
	if p.Transport.Pan <= 0 || updates != 2 {
		t.Fatal("balance keyboard did not notify playback")
	}
	if !p.Main.AccessibleAction(p.body.eq.bands[3].ID(), a11y.ActionIncrement) || p.Equaliser.Gains[3] <= 0 {
		t.Fatal("EQ accessibility failed")
	}
	p.Main.RequestFocus(p.body.eq.bands[3])
	p.ShowEqualiser(false)
	a.PumpOnce()
	if p.Main.Focus() != p.body.play || p.body.eq.Visible() {
		t.Fatal("hiding equalizer stranded keyboard focus")
	}
	key(a, p.Main, platform.KeyE, platform.ModCtrl)
	if !p.body.eq.Visible() {
		t.Fatal("Ctrl+E did not restore EQ")
	}
	p.Host.Model.BackendLimited = true
	p.Host.Model.BackendEqualizer = false
	p.Host.Model.BackendBalance = false
	p.Host.Model.BackendSpectrum = false
	p.Refresh()
	a.PumpOnce()
	gain, pan := p.Equaliser.Gains[3], p.Transport.Pan
	if p.body.eq.bands[3].Enabled() || p.body.eq.balance.Enabled() || p.Main.AccessibleAction(p.body.eq.bands[3].ID(), a11y.ActionIncrement) || p.Equaliser.Gains[3] != gain || p.Transport.Pan != pan {
		t.Fatal("unsupported backend controls changed saved values")
	}
}
func TestFilteredSortedQueueBrowsesWithoutPlayingAndActivatesMappedIdentity(t *testing.T) {
	a, p := open(t, 1)
	p.Transport.List = players.NewPlaylist(players.Track{Title: "Zulu", Artist: "Alpha", Length: time.Minute}, players.Track{Title: "One", Artist: "Target", Length: 2 * time.Minute}, players.Track{Title: "Two", Artist: "Target", Length: 3 * time.Minute})
	p.Transport.State = players.Paused
	p.Refresh()
	a.PumpOnce()
	calls := 0
	p.Host.Run = func(c players.Command) {
		if c == players.CmdPlay {
			calls++
		}
		p.Transport.Do(c)
	}
	p.body.query = "Target"
	p.body.syncRows()
	a.PumpOnce()
	if p.body.list.Count != 2 || p.body.list.QueueIndex(0) != 1 {
		t.Fatal("search did not map matching shared tracks")
	}
	at := p.body.list.AccessibleItems()[1].Bounds.Center()
	click(p.Main, at, platform.ButtonLeft)
	a.PumpOnce()
	p.Refresh()
	if calls != 0 || p.Transport.List.Index() != 0 || p.body.list.Selected != 1 {
		t.Fatal("single-click browse started playback or lost selection")
	}
	key(a, p.Main, platform.KeyReturn, 0)
	if calls != 1 || p.Transport.List.Index() != 2 || p.Transport.State != players.Playing {
		t.Fatal("Enter activated a view row instead of its track")
	}
	p.Transport.State = players.Paused
	if !p.Main.AccessibleAction(widget.ItemID(p.body.list, 1), a11y.ActionDefault) || calls != 2 || p.Transport.State != players.Playing {
		t.Fatal("repeated accessible activation failed to resume")
	}
	p.body.sortColumn, p.body.descending = 0, true
	p.body.syncRows()
	a.PumpOnce()
	if p.body.list.QueueIndex(0) != 2 || p.body.list.Selected != 0 {
		t.Fatal("sort did not preserve browsed/playing identity")
	}
	p.Transport.State = players.Paused
	at = p.body.list.AccessibleItems()[1].Bounds.Center()
	click(p.Main, at, platform.ButtonLeft)
	a.PumpOnce()
	click(p.Main, at, platform.ButtonRight)
	a.PumpOnce()
	key(a, p.Main, platform.KeyEscape, 0)
	click(p.Main, at, platform.ButtonLeft)
	a.PumpOnce()
	if calls != 2 {
		t.Fatal("context interruption retained double-click")
	}
	click(p.Main, at, platform.ButtonLeft)
	a.PumpOnce()
	if calls != 3 || p.Transport.List.Index() != 1 {
		t.Fatal("uninterrupted double-click did not play once")
	}
	key(a, p.Main, platform.KeyL, platform.ModCtrl)
	key(a, p.Main, platform.KeyF, platform.ModCtrl)
	if !p.body.list.Visible() || p.Main.Focus() != p.body.search {
		t.Fatal("search shortcut did not open hidden playlist")
	}
	p.Main.RequestFocus(p.body.headers[0])
	p.ShowPlaylist(false)
	if p.Main.Focus() != p.body.play {
		t.Fatal("hidden playlist header stranded keyboard focus")
	}
}
func TestFileFolderCopyDropsAndSavedPlacement(t *testing.T) {
	a, p := open(t, 1.75)
	file := filepath.Join(t.TempDir(), "track with space.mp3")
	folder := filepath.Join(t.TempDir(), "album")
	os.WriteFile(file, []byte("fixture"), 0600)
	os.Mkdir(folder, 0700)
	uri := func(path string) string { return (&url.URL{Scheme: "file", Path: path}).String() }
	data := []byte(uri(file) + "\r\n" + uri(folder) + "\r\n")
	calls := 0
	var got []string
	p.Host.DropFiles = func(paths []string) { calls++; got = paths }
	off := p.Main.Surface().(*platform.Offscreen)
	for _, at := range []paint2d.Point{p.body.list.AccessibleItems()[0].Bounds.Center(), widget.DeviceBounds(p.body.headers[0]).Center(), widget.DeviceBounds(p.body.play).Center()} {
		before := calls
		off.SimulateDropAction(at, map[string][]byte{"text/uri-list": data}, platform.DragCopy|platform.DragMove, platform.DragMove)
		a.PumpOnce()
		if off.DropTaken() == nil || !*off.DropTaken() || calls != before+1 || !reflect.DeepEqual(got, []string{file, folder}) {
			t.Fatal("local file/folder drop failed Copy routing")
		}
		for _, payload := range []struct {
			data   []byte
			action platform.DragAction
		}{{data, platform.DragMove}, {[]byte("file://remote-host/tmp/track.mp3\r\n"), platform.DragCopy}, {append(append([]byte{}, data...), []byte("https://example.com/music.mp3\r\n")...), platform.DragCopy}} {
			before := calls
			off.SimulateDropAction(at, map[string][]byte{"text/uri-list": payload.data}, payload.action, payload.action)
			a.PumpOnce()
			if calls != before || off.DropTaken() == nil || *off.DropTaken() {
				t.Fatal("Move/remote drop reached the shared loader")
			}
		}
	}
	saved := map[string]players.WindowPlacement{"main": {X: 170, Y: 210, Positioned: true, Width: 1060, Height: 670}}
	p.RestorePlacement(saved)
	for i := 0; i < 3; i++ {
		a.PumpOnce()
		p.Advance(70 * time.Millisecond)
	}
	if !reflect.DeepEqual(p.CapturePlacement(), saved) {
		t.Fatalf("placement did not round-trip: %+v", p.CapturePlacement())
	}
	settingsCalls := 0
	p.Host.ShowSettings = func() { settingsCalls++ }
	if !p.Main.AccessibleAction(p.body.buttons["settings"].ID(), a11y.ActionDefault) || settingsCalls != 1 {
		t.Fatal("backend Settings button did not use Host hook")
	}
	if !strings.Contains(p.body.buttons["remove"].AccessibleName(), "playing") {
		t.Fatal("remove action is misleading about current-track policy")
	}
}

func TestEveryVisibleControlIsReachableAndQueueActionsUseSharedHooks(t *testing.T) {
	a, p := open(t, 1.75)
	ring := playertest.FocusRing(t, a, p.Main)
	for name, c := range p.body.buttons {
		if !playertest.Reaches(ring, c) {
			t.Errorf("keyboard cannot reach %s", name)
		}
	}
	for _, c := range append([]widget.Component{p.body.search, p.body.list, p.body.volume, p.body.seek, p.body.eq.on, p.body.eq.auto, p.body.eq.presets, p.body.eq.reset, p.body.eq.preamp, p.body.eq.balance}, p.body.headers[0], p.body.headers[1], p.body.headers[2], p.body.headers[3]) {
		if !playertest.Reaches(ring, c) {
			t.Errorf("keyboard cannot reach %T", c)
		}
	}
	for _, c := range p.body.eq.bands {
		if !playertest.Reaches(ring, c) {
			t.Error("keyboard cannot reach EQ band")
		}
	}
	var loads []bool
	p.Host.LoadFiles = func(_ widget.Component, appendQueue bool) { loads = append(loads, appendQueue) }
	removed := 0
	p.Host.DeleteTrack = func() { removed++ }
	for _, name := range []string{"open", "add", "remove"} {
		if !p.Main.AccessibleAction(p.body.buttons[name].ID(), a11y.ActionDefault) {
			t.Fatalf("%s accessible action failed", name)
		}
	}
	if !reflect.DeepEqual(loads, []bool{false, true}) || removed != 1 {
		t.Fatal("file/queue actions bypassed shared policy")
	}
	p.Equaliser.SetPreamp(5)
	p.Equaliser.Set(2, 8)
	if !p.Main.AccessibleAction(p.body.eq.reset.ID(), a11y.ActionDefault) || p.Equaliser.Preamp != 0 || p.Equaliser.Gains[2] != 0 {
		t.Fatal("Reset did not flatten preamp and bands")
	}
	closeRequests := 0
	p.Main.SetOnCloseRequest(func() bool { closeRequests++; return false })
	if !p.Main.AccessibleAction(p.body.buttons["close"].ID(), a11y.ActionDefault) {
		t.Fatal("close action failed")
	}
	a.PumpOnce()
	if closeRequests != 1 || p.Main.Closed() {
		t.Fatal("skin close bypassed shell close-to-tray hook")
	}
}

func TestSecondarySliderClickOpensMenuWithoutEditingAudio(t *testing.T) {
	a, p := open(t, 1.75)
	updates := 0
	p.Host.SyncPlayback = func() { updates++ }
	for _, c := range []*sourceSlider{p.body.volume, p.body.seek, p.body.eq.balance, p.body.eq.bands[3]} {
		before := c.Value
		at := widget.DeviceBounds(c).Min.Add(paint2d.Pt(2, 2))
		click(p.Main, at, platform.ButtonRight)
		a.PumpOnce()
		if c.Value != before || updates != 0 || c.Dragging() {
			t.Fatal("secondary click began an audio edit/drag")
		}
		key(a, p.Main, platform.KeyEscape, 0)
	}
}
