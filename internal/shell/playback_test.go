package shell

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/codemodify/media-player-music/internal/playback"
	"github.com/codemodify/media-player-music/internal/players"
	"github.com/codemodify/media-player-music/internal/players/winamp"
	"github.com/codemodify/uitoolkit"
	"github.com/codemodify/uitoolkit/a11y"
	"github.com/codemodify/uitoolkit/style"
	"github.com/codemodify/uitoolkit/widget"
	"github.com/codemodify/uitoolkit/widgets"
)

type recordingBackend struct {
	commands [][]any
	events   chan playback.Event
	closed   bool
}

func (b *recordingBackend) Send(args ...any) error {
	if b.closed {
		return playback.ErrClosed
	}
	b.commands = append(b.commands, append([]any(nil), args...))
	return nil
}
func (b *recordingBackend) Events() <-chan playback.Event { return b.events }
func (b *recordingBackend) Close() error {
	if !b.closed {
		b.closed = true
		close(b.events)
	}
	return nil
}

func liveShell(t *testing.T, skin string) (*uitoolkit.Application, *Shell, *recordingBackend) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv(style.ThemeEnv, LookFor(skin))
	a := uitoolkit.New(uitoolkit.Options{Headless: true, DisableLookWatch: true})
	b := &recordingBackend{events: make(chan playback.Event, 100)}
	s, err := New(a, Options{Skin: skin, Headless: true, Backend: b})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	s.Pulse().Stop()
	a.PumpOnce()
	return a, s, b
}

func mediaPaths(t *testing.T, count int) []string {
	t.Helper()
	var paths []string
	for i := 0; i < count; i++ {
		path := filepath.Join(t.TempDir(), "music.wav")
		if err := os.WriteFile(path, []byte("fixture; decoding belongs to the real-engine tests"), 0600); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, path)
	}
	return paths
}

func reply(s *Shell, property, data string) {
	command, _ := json.Marshal([]string{"get_property", property})
	s.playbackEvent(playback.Event{Event: "command-reply", Name: "get_property", Command: command, Data: json.RawMessage(data)})
}

// Protocol regression fixtures enter through the mpv adapter. Production
// shell code consumes only engine-neutral updates.
func (s *Shell) playbackEvent(event playback.Event) {
	adapter := s.engine.(*playback.MPVEngine)
	for _, update := range adapter.Translate(event) {
		s.engineEvent(update)
	}
}

func identify(s *Shell, id int64) {
	s.playbackEvent(playback.Event{Event: "start-file", PlaylistEntryID: id})
	data, _ := json.Marshal([]map[string]any{{"id": id}})
	reply(s, "playlist", string(data))
	s.playbackEvent(playback.Event{Event: "file-loaded"})
}

func countCommand(b *recordingBackend, name string) int {
	count := 0
	for _, args := range b.commands {
		if args[0] == name {
			count++
		}
	}
	return count
}

func TestLiveClockAndEventsRejectSupersededTrack(t *testing.T) {
	_, s, b := liveShell(t, players.DefaultSkin)
	paths := mediaPaths(t, 2)
	if err := s.LoadFiles(paths, false); err != nil {
		t.Fatal(err)
	}
	identify(s, 11)
	s.playbackEvent(playback.Event{Event: "property-change", Name: "duration", Data: json.RawMessage(`90`)})
	s.playbackEvent(playback.Event{Event: "property-change", Name: "time-pos", Data: json.RawMessage(`12.5`)})
	pos := s.Host.Model.Transport.Pos
	s.tick(time.Second)
	if s.Host.Model.Transport.Pos != pos {
		t.Fatal("UI pulse drove the actual audio clock")
	}
	for _, v := range s.Host.Model.Spectrum.Bars {
		if v != 0 {
			t.Fatal("live playback fabricated spectrum data")
		}
	}
	s.Host.Command(players.CmdNext)
	s.playbackEvent(playback.Event{Event: "property-change", Name: "time-pos", Data: json.RawMessage(`78`)})
	s.playbackEvent(playback.Event{Event: "property-change", Name: "metadata", Data: json.RawMessage(`{"title":"old title"}`)})
	s.playbackEvent(playback.Event{Event: "end-file", PlaylistEntryID: 11, Reason: "eof"})
	if s.Host.Model.Transport.List.Index() != 1 || s.Host.Model.Transport.Pos != 0 || s.Host.Model.Transport.Track().Title == "old title" {
		t.Fatal("an old decoder event changed the newly selected track")
	}
	identify(s, 12)
	s.playbackEvent(playback.Event{Event: "property-change", Name: "duration", Data: json.RawMessage(`100`)})
	s.playbackEvent(playback.Event{Event: "property-change", Name: "metadata", Data: json.RawMessage(`{"TITLE":"new title","ARTIST":"artist"}`)})
	s.playbackEvent(playback.Event{Event: "property-change", Name: "time-pos", Data: json.RawMessage(`2`)})
	if track := s.Host.Model.Transport.Track(); track.Title != "new title" || track.Artist != "artist" || s.Host.Model.Transport.Pos != 2*time.Second {
		t.Fatal("fresh metadata/clock not applied")
	}
	s.Host.SetVolume(.23)
	if countCommand(b, "seek") != 0 {
		t.Fatal("volume edit sought to a stale UI position")
	}
}

func TestEOFArrivingBeforeIdentityAndWhilePaused(t *testing.T) {
	_, s, b := liveShell(t, players.DefaultSkin)
	if err := s.LoadFiles(mediaPaths(t, 2), false); err != nil {
		t.Fatal(err)
	}
	s.playbackEvent(playback.Event{Event: "start-file", PlaylistEntryID: 31})
	s.playbackEvent(playback.Event{Event: "end-file", PlaylistEntryID: 31, Reason: "eof"})
	if s.Host.Model.Transport.List.Index() != 0 {
		t.Fatal("unidentified EOF was attributed immediately")
	}
	reply(s, "playlist", `[{"id":31}]`)
	if s.Host.Model.Transport.List.Index() != 1 {
		t.Fatal("EOF lost while waiting for identity")
	}
	identify(s, 32)
	s.Host.Command(players.CmdPlayPause)
	s.playbackEvent(playback.Event{Event: "end-file", PlaylistEntryID: 32, Reason: "eof"})
	if s.Host.Model.Transport.State != players.Stopped || s.playback.path != "" {
		t.Fatal("paused EOF left an unloaded file cached as active")
	}
	before := countCommand(b, "loadfile")
	s.Host.Play()
	if countCommand(b, "loadfile") != before+1 {
		t.Fatal("Play did not reload the ended file")
	}
}

func TestSeekWaitsForLoadAndErrorsStopAudio(t *testing.T) {
	_, s, b := liveShell(t, players.DefaultSkin)
	if err := s.LoadFiles(mediaPaths(t, 1), false); err != nil {
		t.Fatal(err)
	}
	s.Host.Model.Transport.List.Tracks[0].Length = time.Minute
	s.Host.Seek(.5)
	if countCommand(b, "seek") != 0 {
		t.Fatal("seek was sent before mpv loaded the file")
	}
	identify(s, 41)
	if countCommand(b, "seek") != 1 {
		t.Fatal("pending seek not applied after load")
	}
	s.playbackEvent(playback.Event{Event: "seek"})
	s.playbackEvent(playback.Event{Event: "playback-restart"})
	s.playbackEvent(playback.Event{Event: "command-error", Name: "set_property", Error: "filter rejected"})
	if s.Host.Model.Transport.State != players.Stopped || !strings.Contains(s.Host.Model.Error, "filter rejected") || countCommand(b, "stop") == 0 {
		t.Fatal("engine command failure did not stop actual audio and show its error")
	}
	if s.playback.initialized {
		t.Fatal("rejected settings remained cached")
	}
	s.Host.Play()
	if s.Host.Model.Error != "" {
		t.Fatal("retry did not clear error")
	}
}

func TestPlaylistValidationAppendRemoveAndShutdown(t *testing.T) {
	_, s, b := liveShell(t, players.DefaultSkin)
	paths := mediaPaths(t, 2)
	if err := s.LoadFiles(paths[:1], false); err != nil {
		t.Fatal(err)
	}
	list := s.Host.Model.Transport.List
	if err := s.LoadFiles([]string{filepath.Join(t.TempDir(), "missing")}, false); err == nil || s.Host.Model.Transport.List != list {
		t.Fatal("invalid file replaced the working playlist")
	}
	if err := s.LoadFiles([]string{t.TempDir()}, false); err == nil {
		t.Fatal("a directory was accepted as a music file")
	}
	before := countCommand(b, "loadfile")
	if err := s.LoadFiles(paths[1:], true); err != nil {
		t.Fatal(err)
	}
	if s.Host.Model.Transport.List.Len() != 2 || countCommand(b, "loadfile") != before {
		t.Fatal("Add interrupted the current file")
	}
	s.Host.RemoveTrack()
	if s.Host.Model.Transport.Track().Path != paths[1] || countCommand(b, "loadfile") != before+1 {
		t.Fatal("removing current track did not load its successor")
	}
	s.Host.RemoveTrack()
	if s.Host.Model.Transport.List.Len() != 0 || s.Host.Model.Transport.State != players.Stopped || countCommand(b, "stop") == 0 {
		t.Fatal("removing last track did not stop audio")
	}
	for i := 0; i < 80; i++ {
		b.events <- playback.Event{Event: "property-change", Name: "time-pos", Data: json.RawMessage(`1`)}
	}
	s.Close()
	s.Close()
	if !b.closed {
		t.Fatal("shell shutdown left engine running")
	}
}

func TestFileChooserFitsAllPresentationsAndLoadsPickedFile(t *testing.T) {
	for _, skin := range []string{players.DefaultSkin, winamp.Skin, winamp.SkinSilver} {
		t.Run(skin, func(t *testing.T) {
			a, s, _ := liveShell(t, skin)
			path := mediaPaths(t, 1)[0]
			s.Host.OpenFiles(s.MainWindow().Content(), false)
			a.PumpOnce()
			w := s.fileWindow
			if w == nil || w.Closed() {
				t.Fatal("no file chooser window")
			}
			var field *widgets.TextField
			var open *widgets.Button
			var visit func(widget.Component)
			visit = func(c widget.Component) {
				switch x := c.(type) {
				case *widgets.TextField:
					field = x
				case *widgets.Button:
					if x.Text == "Open" {
						open = x
					}
				}
				for _, child := range c.Children() {
					visit(child)
				}
			}
			visit(w.Content())
			if field == nil || open == nil {
				t.Fatal("missing file path and Open controls")
			}
			bounds := widget.LocalToWindow(open, open.LocalBounds())
			width, height := w.PixelSize()
			if bounds.Min.Y < 0 || bounds.Max.Y > float32(height) || bounds.Max.X > float32(width) {
				t.Fatalf("chooser Open button clipped: %v in %dx%d", bounds, width, height)
			}
			field.SetText(path)
			if !open.AccessibleAction(0, a11y.ActionDefault) {
				t.Fatal("could not activate chooser Open")
			}
			waitFileLoad(t, a, "picked file load", func() bool {
				return s.Host.Model.Transport.Track().Path == path && s.Host.Model.Transport.State == players.Playing
			})
			if !w.Closed() || s.Host.Model.Transport.Track().Path != path || s.Host.Model.Transport.State != players.Playing {
				t.Fatal("picked file was not loaded and started")
			}
			// A temporary file chooser must never become part of the window rack.
			if skin != players.DefaultSkin && len(s.Windows()) != 3 {
				t.Fatal("file chooser entered the skin rack")
			}
		})
	}
}
