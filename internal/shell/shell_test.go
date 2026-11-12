package shell

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/codemodify/media-player-music/internal/players"
	"github.com/codemodify/media-player-music/internal/players/panel"
	"github.com/codemodify/media-player-music/internal/players/playertest"
	"github.com/codemodify/media-player-music/internal/players/standard"
	"goframework.io/desktop-ui"
	"goframework.io/desktop-ui-skins"
	"goframework.io/desktop-ui/platform"
	"goframework.io/desktop-ui/style"
	"goframework.io/desktop-ui/widget"
)

func open(t *testing.T, skin string) (*ui.Application, *Shell) {
	return openAtScale(t, skin, 0)
}

func openAtScale(t *testing.T, skin string, scale float32) (*ui.Application, *Shell) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	opts := Options{Skin: skin, Headless: true, Scale: scale}
	t.Setenv(style.ThemeEnv, LookFor(SkinFor(opts)))
	a := ui.New(ui.Options{Headless: true, Scale: scale, DisableLookWatch: true})
	s, err := New(a, opts)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	a.PumpOnce()
	return a, s
}

func TestDefaultAndEverySkinOpenTheirCompleteUI(t *testing.T) {
	for _, id := range Appearances {
		t.Run(id, func(t *testing.T) {
			_, s := open(t, id)
			count := 1
			if players.IsPanelSkin(id) || id == players.GaugeSkin || id == players.FancySkin {
				count = 3
			} else if id == players.PodSkin || id == players.FandomSkin || id == players.AluminiumSkin {
				count = 2
			}
			if id == players.DefaultSkin {
				count = 1
				if _, ok := s.Presentation().(*standard.Player); !ok || skins.IsSkin(s.Host.Worn()) {
					t.Fatal("default did not use the ordinary unskinned UI")
				}
			}
			if s.Skin() != id || s.Host.Worn() != LookFor(id) || len(s.Windows()) != count {
				t.Fatalf("choice=%s look=%s windows=%d", s.Skin(), s.Host.Worn(), len(s.Windows()))
			}
			for _, w := range s.Windows() {
				playertest.Audit(t, id+" "+w.Name, w.Window)
			}
		})
	}
}

func TestSkinChangesKeepTheModelAndEveryPlayerSetting(t *testing.T) {
	a, s := open(t, players.DefaultSkin)
	m := s.Host.Model
	m.Transport.SelectTrack(2)
	m.Transport.Play()
	m.Transport.Pos = 41 * time.Second
	m.Transport.SetVolume(.31)
	m.Transport.SetPan(-.3)
	m.Transport.SetRandom(true)
	m.Transport.CycleRepeat()
	m.Equaliser.On = true
	m.Equaliser.SetAuto(true, 2)
	m.Equaliser.Set(0, 6)
	m.Equaliser.SetPreamp(3)
	m.EqualizerVisible, m.PlaylistVisible = false, false
	s.refresh()

	beforeTransport := *m.Transport
	beforeGains := append([]float32(nil), m.Equaliser.Gains...)
	beforeFall := m.Spectrum.Fall
	beforePlaylist := m.Transport.List
	for _, id := range append(players.SkinnedIDs(), players.DefaultSkin) {
		if err := s.SetSkin(id); err != nil {
			t.Fatal(err)
		}
		// Freeze time while comparing state. Clock continuity is checked
		// separately; a slow paint under -race must not count as a reset.
		s.Pulse().Stop()
		a.PumpOnce()
		tr := m.Transport
		if s.Host.Model != m || tr.List != beforePlaylist || tr.List.Index() != 2 || tr.Pos != beforeTransport.Pos ||
			tr.Volume != beforeTransport.Volume || tr.Pan != beforeTransport.Pan || tr.State != beforeTransport.State ||
			tr.Random != beforeTransport.Random || tr.Repeat != beforeTransport.Repeat {
			t.Fatalf("%s changed the player or playlist", id)
		}
		if !m.Equaliser.On || !m.Equaliser.Auto || m.Equaliser.Preamp != 3 || !reflect.DeepEqual(m.Equaliser.Gains, beforeGains) || m.Equaliser.Preset != "Custom" {
			t.Fatalf("%s changed the equalizer or reapplied Auto on the same track", id)
		}
		if m.Spectrum.Fall != beforeFall {
			t.Fatalf("%s changed the shared analyser policy", id)
		}
		if m.EqualizerVisible || m.PlaylistVisible {
			t.Fatalf("%s reopened a hidden area", id)
		}
		for _, w := range s.Windows()[1:] {
			if w.Window.Visible() {
				t.Fatalf("%s opened hidden %s", id, w.Name)
			}
		}
	}
}

func TestDefaultSwitchReplacesWindowsAndPanelSwitchKeepsControls(t *testing.T) {
	a, s := open(t, players.DefaultSkin)
	initial := s.MainWindow()
	if err := s.SetSkin(panel.Skin); err != nil {
		t.Fatal(err)
	}
	a.PumpOnce()
	if !initial.Closed() || len(s.Windows()) != 3 {
		t.Fatal("the default window was not replaced by three panel windows")
	}
	panel := s.Presentation()
	windows := s.Windows()
	focus := s.MainWindow().Focus()
	for _, id := range players.PanelSkins()[1:] {
		if err := s.SetSkin(id); err != nil {
			t.Fatal(err)
		}
		a.PumpOnce()
		if s.Presentation() != panel || !reflect.DeepEqual(s.Windows(), windows) || s.MainWindow().Focus() != focus {
			t.Fatalf("%s replaced controls or lost focus", id)
		}
	}
	if err := s.SetSkin(players.DefaultSkin); err != nil {
		t.Fatal(err)
	}
	a.PumpOnce()
	for _, w := range windows {
		if !w.Window.Closed() {
			t.Fatalf("old %s window survived returning to default", w.Name)
		}
	}
	if s.MainWindow().Closed() || len(s.Windows()) != 1 {
		t.Fatal("the default UI did not reopen")
	}
	if a.Quitting() {
		t.Fatal("programmatically replacing windows quit the player")
	}
}

func TestClosingMainExitsTheWholePlayer(t *testing.T) {
	for _, id := range Appearances {
		for _, hidden := range []bool{false, true} {
			name := id + "/shown"
			if hidden {
				name = id + "/hidden"
			}
			t.Run(name, func(t *testing.T) {
				a, s := open(t, id)
				s.Start()
				if hidden {
					s.Presentation().ShowEqualiser(false)
					s.Presentation().ShowPlaylist(false)
				}
				windows := s.Windows()
				s.MainWindow().Inject(platform.Event{Kind: platform.EventClose})
				a.PumpOnce()
				if !a.Quitting() || !s.MainWindow().Closed() || s.Pulse().Running() {
					t.Fatal("closing the primary window did not quit and stop its clock")
				}
				// run() defers this cleanup after the application loop exits.
				s.Close()
				for _, w := range windows {
					if !w.Window.Closed() {
						t.Fatalf("%s survived application shutdown", w.Name)
					}
				}
			})
		}
	}
}

func TestClockAndTransportContinueAcrossAppearanceChanges(t *testing.T) {
	a, s := open(t, players.DefaultSkin)
	s.Start()
	for _, id := range append(players.SkinnedIDs(), players.DefaultSkin) {
		if err := s.SetSkin(id); err != nil {
			t.Fatal(err)
		}
		a.PumpOnce()
		if !s.Pulse().Running() {
			t.Fatalf("%s stopped the clock", id)
		}
		at := s.Host.Model.Transport.Pos
		s.Pulse().Tick()
		if s.Host.Model.Transport.Pos <= at {
			t.Fatalf("%s clock did not advance the track", id)
		}
	}
	for _, id := range Appearances {
		if err := s.SetSkin(id); err != nil {
			t.Fatal(err)
		}
		a.PumpOnce()
		tr := s.Host.Model.Transport
		tr.Play()
		// Focused buttons use Space to press that button. A seek slider
		// leaves it to the shared transport shortcut. Find the public name
		// so skin-specific slider wrappers participate in this check too.
		var seek widget.Component
		widget.Walk(s.MainWindow().Content(), func(c widget.Component) {
			if named, ok := c.(interface{ AccessibleName() string }); ok && named.AccessibleName() == "Seek" {
				seek = c
			}
		})
		if seek == nil {
			t.Fatalf("%s: no accessible seek slider", id)
		}
		s.MainWindow().RequestFocus(seek)
		if s.MainWindow().Focus() != seek {
			t.Fatalf("%s: seek slider did not receive focus", id)
		}
		s.MainWindow().Inject(platform.Event{Kind: platform.EventKeyDown, Key: platform.KeySpace})
		a.PumpOnce()
		if tr.State != players.Paused {
			t.Fatalf("%s: Space did not pause", id)
		}
		track := tr.List.Index()
		s.MainWindow().Inject(platform.Event{Kind: platform.EventKeyDown, Key: platform.KeyB})
		a.PumpOnce()
		if tr.List.Index() == track {
			t.Fatalf("%s: B did not change track", id)
		}
	}
}

func TestSkinKeysCycleEveryChoiceAndReturnToLastSkin(t *testing.T) {
	a, s := open(t, players.DefaultSkin)
	key := func(mods platform.Modifiers) {
		s.MainWindow().Inject(platform.Event{Kind: platform.EventKeyDown, Key: platform.KeyK, Mods: mods})
		a.PumpOnce()
		a.PumpOnce()
	}
	for _, id := range append(players.SkinnedIDs(), players.DefaultSkin) {
		key(platform.ModCtrl)
		if s.Skin() != id {
			t.Fatalf("Ctrl+K chose %s, want %s", s.Skin(), id)
		}
	}
	key(platform.ModCtrl | platform.ModShift)
	if s.Skin() != players.FancySkin {
		t.Fatal("Ctrl+Shift+K did not restore the last skin")
	}
	for _, last := range []string{players.FancySkin, players.SidebarSkin, players.ConsoleSkin, players.UtilitySkin, players.CobaltSkin, players.AluminiumSkin, players.FandomSkin, players.GaugeSkin, players.PodSkin, players.WingspanSkin, players.SilverSkin} {
		if err := s.SetSkin(last); err != nil {
			t.Fatal(err)
		}
		a.PumpOnce()
		for i := range s.Windows() {
			w := s.Windows()[i].Window
			w.Inject(platform.Event{Kind: platform.EventKeyDown, Key: platform.KeyK, Mods: platform.ModCtrl | platform.ModShift})
			a.PumpOnce()
			a.PumpOnce()
			if s.Skin() != players.DefaultSkin {
				t.Fatalf("skin key in %s window %d did not return to default", last, i)
			}
			key(platform.ModCtrl | platform.ModShift)
			if s.Skin() != last {
				t.Fatalf("last skin was %s, want %s", s.Skin(), last)
			}
		}
	}
}

func TestSinglePreferenceAndScreenshotIsolation(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if got := SkinFor(Options{Remember: true}); got != players.DefaultSkin {
		t.Fatalf("new install opens %s, want default", got)
	}
	opts := Options{Headless: true, Remember: true}
	t.Setenv(style.ThemeEnv, LookFor(SkinFor(opts)))
	a := ui.New(ui.Options{Headless: true, DisableLookWatch: true})
	s, err := New(a, opts)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	if err := s.SetSkin(panel.SkinSilver); err != nil {
		t.Fatal(err)
	}
	if got := SkinFor(Options{Remember: true}); got != panel.SkinSilver {
		t.Fatalf("next run opens %s, want silver", got)
	}
	if got := SkinFor(Options{Skin: panel.Skin, Remember: true}); got != panel.Skin {
		t.Fatal("explicit skin did not override the preference")
	}
	if got := SkinFor(Options{}); got != players.DefaultSkin {
		t.Fatal("screenshot run read the saved preference")
	}
	data, err := os.ReadFile(players.SettingsPath())
	if err != nil || string(data) != "{\n  \"skin\": \"silver\"\n}\n" {
		t.Fatalf("preference is not one skin choice: %s, %v", data, err)
	}
}

func TestUnknownChoiceLeavesTheExistingPlayerOpen(t *testing.T) {
	_, s := open(t, players.DefaultSkin)
	w, model := s.MainWindow(), s.Host.Model
	if err := s.SetSkin("unsupported"); err == nil {
		t.Fatal("unknown skin accepted")
	}
	if s.Skin() != players.DefaultSkin || s.MainWindow() != w || w.Closed() || s.Host.Model != model {
		t.Fatal("invalid selection damaged the existing player")
	}
}

func TestAnIncompleteOverrideKeepsTheDefaultAndLastWorkingSkin(t *testing.T) {
	for _, failedSkin := range []string{panel.Skin, players.WingspanSkin, players.PodSkin, players.GaugeSkin, players.FandomSkin, players.AluminiumSkin, players.CobaltSkin} {
		t.Run(failedSkin, func(t *testing.T) {
			a, s := open(t, players.DefaultSkin)
			if err := s.SetSkin(panel.SkinSilver); err != nil {
				t.Fatal(err)
			}
			if err := s.SetSkin(players.DefaultSkin); err != nil {
				t.Fatal(err)
			}
			a.PumpOnce()
			w := s.MainWindow()
			dir := skins.UserSkinDir(failedSkin)
			if err := os.MkdirAll(dir, 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "skin.json"), []byte(`{"skin":1,"label":"Incomplete","base":"dark"}`), 0o600); err != nil {
				t.Fatal(err)
			}
			skins.InvalidateSkinCache()
			t.Cleanup(skins.InvalidateSkinCache)
			if err := s.SetSkin(failedSkin); err == nil {
				t.Fatal("incomplete appearance override was accepted")
			}
			if s.MainWindow() != w || w.Closed() || s.Skin() != players.DefaultSkin || s.Host.Worn() != players.Themed {
				t.Fatal("failed choice damaged the default UI")
			}
			s.Host.DropSkin()
			a.PumpOnce()
			if s.Skin() != panel.SkinSilver {
				t.Fatal("failed choice replaced the last working skin")
			}
		})
	}
}

func TestAreaShortcutsWorkInDefaultAndSkinnedWindows(t *testing.T) {
	for _, id := range Appearances {
		a, s := open(t, id)
		w := s.MainWindow()
		for _, key := range []platform.Key{platform.KeyE, platform.KeyL} {
			for _, shown := range []bool{false, true} {
				w.Inject(platform.Event{Kind: platform.EventKeyDown, Key: key, Mods: platform.ModCtrl})
				a.PumpOnce()
				actual := s.Host.Model.EqualizerVisible
				if key == platform.KeyL {
					actual = s.Host.Model.PlaylistVisible
				}
				if actual != shown {
					t.Fatalf("%s: key %v visibility=%v, want %v", id, key, actual, shown)
				}
				if players.IsPanelSkin(id) {
					index := 1
					if key == platform.KeyL {
						index = 2
					}
					if s.Windows()[index].Window.Visible() != shown {
						t.Fatalf("%s: panel visibility did not follow the shortcut", id)
					}
				}
			}
		}
	}
}
