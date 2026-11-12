package players

import (
	"fmt"
	"strings"
	"testing"

	"goframework.io/desktop-ui"
	"goframework.io/desktop-ui-widgets"
	"goframework.io/desktop-ui/a11y"
	"goframework.io/desktop-ui/platform"
	"goframework.io/desktop-ui/style"
	"goframework.io/desktop-ui/widget"
	"goframework.io/graphics-paint2d"
)

func TestSettingsPanelChangesReplayGainAndFits(t *testing.T) {
	for _, scale := range []float32{1, 1.75} {
		t.Run(fmt.Sprintf("%gx", scale), func(t *testing.T) {
			t.Setenv(style.ThemeEnv, Themed)
			a := ui.New(ui.Options{Headless: true, Scale: scale, DisableLookWatch: true})
			w, err := a.NewWindow(platform.WindowOptions{Headless: true, Decorations: platform.DecorationsServer, Width: SettingsWidth, Height: SettingsHeight})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(w.Close)
			var changes []ReplayGain
			p := NewSettings(SettingsState{Formats: "MP3, FLAC", ReplayGain: ReplayGain{Mode: ReplayGainAlbum, Preamp: -1.5}}, func(g ReplayGain) { changes = append(changes, g) })
			w.SetContent(p)
			a.PumpOnce()
			if !strings.Contains(p.formats.Text, "Plays MP3, FLAC.") || !strings.Contains(p.formats.Text, "WMA") {
				t.Fatalf("the formats: %q", p.formats.Text)
			}
			if p.modes.Selected() != 2 || p.preamp.Value != -1.5 || p.preampLabel.Text != "Preamp: -1.5 dB" {
				t.Fatal("the state is not shown")
			}
			for _, size := range [][2]int{{SettingsWidth, SettingsHeight}, {SettingsMinWidth, SettingsMinHeight}} {
				w.SetSize(size[0], size[1])
				a.PumpOnce()
				for _, c := range p.Children() {
					if b := c.Bounds(); b.Min.Y < 0 || b.Max.X > p.LocalBounds().Max.X+.1 || b.Max.Y > p.LocalBounds().Max.Y+.1 {
						t.Fatalf("%T clipped at %dx%d: %v in %v", c, size[0], size[1], b, p.LocalBounds())
					}
				}
				if b, h := p.hint.Bounds(), p.preamp.Bounds(); b.Overlaps(h) || p.modes.Bounds().Overlaps(h) {
					t.Fatalf("controls overlap at %dx%d", size[0], size[1])
				}
			}
			// The user picks a mode and moves the preamp, by keyboard and
			// by accessibility; each change is reported once.
			track := p.modes.Buttons()[1]
			if !w.AccessibleAction(track.ID(), a11y.ActionDefault) || len(changes) != 1 || changes[0] != (ReplayGain{Mode: ReplayGainTrack, Preamp: -1.5}) {
				t.Fatalf("choosing Track gain reported %+v", changes)
			}
			w.RequestFocus(p.preamp)
			w.Inject(platform.Event{Kind: platform.EventKeyDown, Key: platform.KeyRight})
			a.PumpOnce()
			if len(changes) != 2 || changes[1] != (ReplayGain{Mode: ReplayGainTrack, Preamp: -1}) || p.preampLabel.Text != "Preamp: -1.0 dB" {
				t.Fatalf("a preamp step reported %+v, shows %q", changes, p.preampLabel.Text)
			}
			if !w.AccessibleAction(p.modes.Buttons()[0].ID(), a11y.ActionDefault) || changes[len(changes)-1].Mode != ReplayGainOff {
				t.Fatal("choosing Off did not report it")
			}
			// The owner's state does not echo back as a change.
			before := len(changes)
			p.SetState(SettingsState{Formats: "MP3", ReplayGain: ReplayGain{Mode: ReplayGainAlbum, Preamp: 4}, Error: "song.wma: WMA files are not supported."})
			a.PumpOnce()
			if len(changes) != before || p.modes.Selected() != 2 || p.preamp.Value != 4 || p.status.Text == "" {
				t.Fatal("SetState was reported as the user's change, or not shown")
			}
			if img := w.Capture(); img == nil || img.Width == 0 {
				t.Fatal("settings did not paint")
			}
			closed := false
			p.OnClose = func() { closed = true }
			w.Inject(platform.Event{Kind: platform.EventKeyDown, Key: platform.KeyEscape})
			a.PumpOnce()
			if !closed {
				t.Fatal("Escape did not close settings through the panel")
			}
		})
	}
}

func TestContextMenuAddsAccessibleSettingsWithoutChangingSkinChoices(t *testing.T) {
	t.Setenv(style.ThemeEnv, "dark")
	a := ui.New(ui.Options{Headless: true, DisableLookWatch: true})
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
	h.ContextMenu(from, paint2d.Pt(8, 8))
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
