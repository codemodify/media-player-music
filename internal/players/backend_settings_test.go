package players

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/codemodify/paintengine2d"
	"github.com/codemodify/uitoolkit"
	"github.com/codemodify/uitoolkit/a11y"
	"github.com/codemodify/uitoolkit/layout"
	"github.com/codemodify/uitoolkit/platform"
	"github.com/codemodify/uitoolkit/style"
	"github.com/codemodify/uitoolkit/widget"
	"github.com/codemodify/uitoolkit/widgets"
)

func backendSettingsFixture() BackendSettingsState {
	return BackendSettingsState{Selected: "mpv", Backends: []BackendInfo{
		{ID: "mpv", Name: "mpv", Executable: "/usr/bin/mpv", Available: true, Equalizer: true, Balance: true, Visualizer: true},
		{ID: "vlc", Name: "VLC", Available: false},
		{ID: "mplayer", Name: "MPlayer", Executable: "/usr/bin/mplayer", Available: true},
		{ID: "mpg123", Name: "mpg123", Executable: "/usr/bin/mpg123", Available: true},
	}}
}

func TestBackendPreferencePreservesMissingProgramsAndHistoricalSessions(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	want := Settings{Skin: WinampSkin, AudioBackend: "vlc", Panels: &SavedPanels{Playlist: true}}
	if err := want.Save(); err != nil {
		t.Fatal(err)
	}
	if got := LoadSettings(); got.AudioBackend != "vlc" || got.Panels == nil || !got.Panels.Playlist {
		t.Fatalf("lost backend or session preference: %#v", got)
	}
	for _, tc := range []struct{ input, want string }{
		{`{"skin":"winamp"}`, ""},
		{`{"skin":"winamp","audio_backend":"mplayer"}`, "mplayer"},
		{`{"skin":"winamp","audio_backend":"mpg123"}`, "mpg123"},
		{`{"skin":"winamp","audio_backend":"unknown"}`, "mpv"},
	} {
		if err := os.WriteFile(SettingsPath(), []byte(tc.input), 0600); err != nil {
			t.Fatal(err)
		}
		if got := LoadSettings(); got.AudioBackend != tc.want {
			t.Fatalf("%s: got %q", tc.input, got.AudioBackend)
		}
	}
	data, err := json.Marshal(Settings{Skin: DefaultSkin})
	if err != nil || strings.Contains(string(data), "audio_backend") {
		t.Fatal("historical zero preference should remain omitted")
	}
	if id, ok := NormalizeAudioBackend(""); id != "mpv" || !ok {
		t.Fatal("missing preference must resolve to mpv")
	}
	if _, err := os.Stat(filepath.Dir(SettingsPath())); err != nil {
		t.Fatal(err)
	}
}

func TestBackendSettingsControlsHaveDistinctSelectionAvailabilityAndRealActions(t *testing.T) {
	for _, scale := range []float32{1, 1.75} {
		t.Run(fmt.Sprintf("%gx", scale), func(t *testing.T) {
			t.Setenv(style.ThemeEnv, Themed)
			a := uitoolkit.New(uitoolkit.Options{Headless: true, Scale: scale, DisableLookWatch: true})
			w, err := a.NewWindow(platform.WindowOptions{Headless: true, Decorations: platform.DecorationsServer, Width: BackendSettingsWidth, Height: BackendSettingsHeight})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(w.Close)
			state := backendSettingsFixture()
			chosen, rechecks := "", 0
			var p *BackendSettingsPanel
			p = NewBackendSettings(state, func(id string) { chosen = id; state.Selected = id; p.SetState(state) }, func() { rechecks++ })
			w.SetContent(p)
			a.PumpOnce()
			for _, row := range p.rows {
				for _, c := range row.Children() {
					b := c.Bounds()
					if b.Empty() || b.Min.X < 0 || b.Min.Y < 0 || b.Max.X > row.LocalBounds().Max.X+.1 || b.Max.Y > row.LocalBounds().Max.Y+.1 {
						t.Errorf("%s control outside row: %v in %v", row.info.ID, b, row.LocalBounds())
					}
				}
				if b := row.Bounds(); b.Max.Y > p.LocalBounds().Max.Y {
					t.Fatal("backend row clipped")
				}
			}
			if p.refresh.Bounds().Max.Y > p.LocalBounds().Max.Y || p.status.Bounds().Overlaps(p.refresh.Bounds()) {
				t.Fatal("footer controls clipped or overlapping")
			}
			tree := w.AccessibleTree()
			if n := tree.Find(p.rows[0].radio.ID()); n == nil || !n.State.Has(a11y.StateChecked) {
				t.Fatal("selected backend not marked as a radio selection")
			}
			if n := tree.Find(p.rows[1].radio.ID()); n == nil || !n.State.Has(a11y.StateDisabled) {
				t.Fatal("unavailable backend was not disabled")
			}
			if w.AccessibleAction(p.rows[1].radio.ID(), a11y.ActionDefault) || chosen != "" {
				t.Fatal("unavailable backend executed")
			}
			w.RequestFocus(p.rows[2].radio)
			w.Inject(platform.Event{Kind: platform.EventKeyDown, Key: platform.KeySpace})
			a.PumpOnce()
			if chosen != "mplayer" || !p.rows[2].radio.Selected || p.rows[0].radio.Selected {
				t.Fatal("keyboard selection did not choose exactly one engine")
			}
			if !w.AccessibleAction(p.rows[3].radio.ID(), a11y.ActionDefault) || chosen != "mpg123" {
				t.Fatal("accessible backend selection did not execute")
			}
			if !strings.Contains(p.rows[3].features.Text, "EQ unavailable") || !strings.Contains(p.rows[3].features.Text, "Balance unavailable") || !strings.Contains(p.rows[3].features.Text, "Visualizer unavailable") {
				t.Fatal("limited engine capabilities were not disclosed")
			}
			if !w.AccessibleAction(p.refresh.ID(), a11y.ActionDefault) || rechecks != 1 {
				t.Fatal("Refresh did not rescan")
			}
			state.Backends[1].Available, state.Backends[1].Executable = true, "/usr/bin/vlc"
			p.SetState(state)
			a.PumpOnce()
			if !w.AccessibleAction(p.rows[1].radio.ID(), a11y.ActionDefault) || chosen != "vlc" {
				t.Fatal("newly installed program did not become selectable")
			}
			state.Busy = true
			p.SetState(state)
			if w.AccessibleAction(p.rows[0].radio.ID(), a11y.ActionDefault) || w.AccessibleAction(p.refresh.ID(), a11y.ActionDefault) {
				t.Fatal("busy engine allowed another operation")
			}
			state.Busy, state.Backends[1].Available = false, false
			p.SetState(state)
			if !p.rows[1].radio.Selected || p.rows[1].radio.Enabled() || !strings.Contains(p.status.Text, "selected but unavailable") {
				t.Fatal("missing selected engine lost its distinct saved state")
			}
			if img := w.Capture(); img == nil || img.Width == 0 {
				t.Fatal("settings did not paint")
			}
			state.Backends[3].Executable = "/opt/very-long-installed-program-directory/another-very-long-directory/player-binaries/mpg123"
			p.SetState(state)
			for _, size := range [][2]int{{BackendSettingsWidth, BackendSettingsHeight}, {560, 440}} {
				w.SetSize(size[0], size[1])
				a.PumpOnce()
				for _, row := range p.rows {
					if row.Bounds().Max.Y > p.LocalBounds().Max.Y {
						t.Fatal("backend rows clipped at minimum size")
					}
					if row.features.Measure(layout.Unbounded()).X > row.features.Bounds().Dx()+.1 {
						t.Fatalf("capabilities elided at %dx%d: %s", size[0], size[1], row.features.Text)
					}
					for _, c := range row.Children() {
						if c.Bounds().Max.X > row.LocalBounds().Max.X+.1 || c.Bounds().Max.Y > row.LocalBounds().Max.Y+.1 {
							t.Fatalf("row control %T clipped at %dx%d: %v in %v; content=%v hint=%v", c, size[0], size[1], c.Bounds(), row.LocalBounds(), p.LocalBounds(), p.hint.Bounds())
						}
					}
				}
				if p.status.Bounds().Max.Y > p.LocalBounds().Max.Y || p.refresh.Bounds().Max.Y > p.LocalBounds().Max.Y {
					t.Fatal("settings footer clipped at minimum size")
				}
				if n := w.AccessibleTree().Find(p.rows[3].executable.ID()); n == nil || n.Name != state.Backends[3].Executable {
					t.Fatal("elided executable path not available to accessibility")
				}
			}
			closed := false
			p.OnClose = func() { closed = true }
			w.RequestFocus(p.refresh)
			w.Inject(platform.Event{Kind: platform.EventKeyDown, Key: platform.KeyEscape})
			a.PumpOnce()
			if !closed {
				t.Fatal("Escape did not close settings through the panel")
			}
		})
	}
}

func TestBackendAvailabilityMarksAreGreenAndRedVectorPaths(t *testing.T) {
	for _, available := range []bool{false, true} {
		m := &backendAvailabilityMark{available: available}
		m.Init(m)
		m.SetLook(style.DarkLook())
		m.Arrange(paintengine2d.XYWH(0, 0, 32, 32))
		im := paintengine2d.NewImage(32, 32)
		m.Paint(paintengine2d.NewContext(im))
		pixels := 0
		for y := 0; y < im.Height; y++ {
			for x := 0; x < im.Width; x++ {
				c := im.NRGBAAt(x, y)
				if c.A > 100 && ((available && int(c.G) > int(c.R)*2 && int(c.G) > int(c.B)*2) || (!available && int(c.R) > int(c.G)*2 && int(c.R) > int(c.B)*2)) {
					pixels++
				}
			}
		}
		if pixels < 15 {
			t.Fatalf("availability=%t drew only %d correctly colored pixels", available, pixels)
		}
	}
}

func TestMPG123FormatScopeIsVisibleBeforeAndAfterSelection(t *testing.T) {
	for _, scale := range []float32{1, 1.75} {
		for _, size := range [][2]int{{BackendSettingsWidth, BackendSettingsHeight}, {560, 440}} {
			t.Run(fmt.Sprintf("%gx/%dx%d", scale, size[0], size[1]), func(t *testing.T) {
				t.Setenv(style.ThemeEnv, Themed)
				a := uitoolkit.New(uitoolkit.Options{Headless: true, Scale: scale, DisableLookWatch: true})
				w, err := a.NewWindow(platform.WindowOptions{Headless: true, Decorations: platform.DecorationsServer, Width: size[0], Height: size[1]})
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(w.Close)
				state := backendSettingsFixture()
				var p *BackendSettingsPanel
				p = NewBackendSettings(state, func(id string) { state.Selected = id; p.SetState(state) }, nil)
				w.SetContent(p)
				a.PumpOnce()
				radio := p.rows[3].radio
				if n := w.AccessibleTree().Find(radio.ID()); n == nil || n.Name != "mpg123 (MP3/MPEG only)" {
					t.Fatal("mpg123 format limit was not exposed with the backend choice")
				}
				if radio.Measure(layout.Unbounded()).X > radio.Bounds().Dx()+.1 {
					t.Fatal("mpg123 format label was clipped")
				}
				if !w.AccessibleAction(radio.ID(), a11y.ActionDefault) {
					t.Fatal("mpg123 format disclosure made the backend unselectable")
				}
				a.PumpOnce()
				if !radio.Selected || !strings.Contains(p.status.Text, "MP3/MPEG only") || !strings.Contains(p.status.Text, "MP4/AAC/FLAC need another backend") {
					t.Fatal("selected mpg123 did not explain which files require another backend")
				}
				if got := p.status.Measure(layout.Loose(p.status.Bounds().Dx(), 10000)).Y; got > p.status.Bounds().Dy()+.1 {
					t.Fatalf("mpg123 format footer clipped: needs %g, has %g", got, p.status.Bounds().Dy())
				}
				state.Error = "mpg123: this media file is unsupported"
				p.SetState(state)
				if p.status.Text != state.Error || !strings.Contains(radio.Text, "MP3/MPEG only") {
					t.Fatal("format hint displaced the playback error")
				}
			})
		}
	}
}

func TestSelectedBackendCanBeRetriedAfterAnError(t *testing.T) {
	t.Setenv(style.ThemeEnv, Themed)
	a := uitoolkit.New(uitoolkit.Options{Headless: true, DisableLookWatch: true})
	w, err := a.NewWindow(platform.WindowOptions{Headless: true, Width: BackendSettingsWidth, Height: BackendSettingsHeight})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(w.Close)
	state := backendSettingsFixture()
	calls := 0
	var p *BackendSettingsPanel
	p = NewBackendSettings(state, func(id string) {
		if id != "mpv" {
			t.Fatal("retry changed the saved selection")
		}
		calls++
		state.Error = ""
		p.SetState(state)
	}, nil)
	w.SetContent(p)
	a.PumpOnce()
	radio := p.rows[0].radio
	activate := func(input string) {
		switch input {
		case "keyboard":
			w.RequestFocus(radio)
			w.Inject(platform.Event{Kind: platform.EventKeyDown, Key: platform.KeySpace})
		case "mouse":
			o, b := widget.DeviceOrigin(radio), radio.LocalBounds()
			at := paintengine2d.Pt(o.X+b.Dx()/2, o.Y+b.Dy()/2)
			w.Inject(platform.Event{Kind: platform.EventMouseDown, Button: platform.ButtonLeft, Pos: at})
			w.Inject(platform.Event{Kind: platform.EventMouseUp, Button: platform.ButtonLeft, Pos: at})
		case "accessibility":
			if !w.AccessibleAction(radio.ID(), a11y.ActionDefault) {
				t.Fatal("selected retry action was refused")
			}
		}
		a.PumpOnce()
	}
	for _, input := range []string{"keyboard", "mouse", "accessibility"} {
		before := calls
		activate(input)
		if calls != before {
			t.Fatal("healthy selected engine was restarted")
		}
		state.Error = "Could not start the selected program"
		p.SetState(state)
		activate(input)
		if calls != before+1 || !radio.Selected || !p.rows[0].availability.available {
			t.Fatalf("%s did not retry exactly once while preserving installed/selected indicators", input)
		}
	}
	state.Error, state.Busy = "Could not start", true
	p.SetState(state)
	before := calls
	activate("mouse")
	activate("keyboard")
	if w.AccessibleAction(radio.ID(), a11y.ActionDefault) || calls != before {
		t.Fatal("busy retry accepted another operation")
	}
}

func TestContextMenuAddsAccessibleSettingsWithoutChangingSkinChoices(t *testing.T) {
	t.Setenv(style.ThemeEnv, "dark")
	a := uitoolkit.New(uitoolkit.Options{Headless: true, DisableLookWatch: true})
	w, err := a.NewWindow(platform.WindowOptions{Headless: true, Width: 400, Height: 320})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(w.Close)
	from := widgets.NewButton("Skins", nil)
	w.SetContent(from)
	a.PumpOnce()
	h := NewHost(a)
	h.Packs = PanelSkins
	count := len(h.SkinItems())
	hits := 0
	h.ShowSettings = func() { hits++ }
	h.ContextMenu(from, paintengine2d.Pt(8, 8))
	a.PumpOnce()
	menu := w.Popup().(*widgets.PopupMenu)
	items := menu.AccessibleItems()
	last := len(items) - 1
	if len(h.SkinItems()) != count || !strings.Contains(items[last].Name, "Settings") {
		t.Fatal("settings altered or displaced the appearance choices")
	}
	if !w.AccessibleAction(widget.ItemID(menu, last), a11y.ActionDefault) || hits != 1 || w.Popup() != nil {
		t.Fatal("settings context action did not execute and dismiss")
	}
}
