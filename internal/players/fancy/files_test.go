package fancy

import (
	"bytes"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/codemodify/media-player-music/internal/players"
	"goframework.io/desktop-ui-widgets"
	"goframework.io/desktop-ui/platform"
)

func TestEqualizerFileRoundTripAtomicValidationAndAudioNotification(t *testing.T) {
	_, p := open(t, 1)
	p.Transport.State, p.Transport.Pos = players.Paused, 17*time.Second
	p.Equaliser.On = true
	p.Equaliser.Preamp = -3
	p.Equaliser.Gains[2] = 6
	p.Equaliser.Preset = "Custom"
	state := snapshotEqualizer(p.Equaliser)
	data, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "eq.json")
	if err := writeFile(path, data); err != nil {
		t.Fatal(err)
	}
	loaded, err := readEqualizerFile(path)
	if err != nil || !reflect.DeepEqual(loaded, state) {
		t.Fatalf("roundtrip: %v, %v", loaded, err)
	}
	changes := 0
	p.Equaliser.Changed = func() { changes++ }
	p.Equaliser.Preamp = 2
	if err := p.applyEqualizerFile(loaded); err != nil || changes != 1 || !reflect.DeepEqual(snapshotEqualizer(p.Equaliser), state) {
		t.Fatal("import failed or duplicated audio notification")
	}
	if p.Transport.State != players.Paused || p.Transport.Pos != 17*time.Second {
		t.Fatal("EQ import restarted playback")
	}
	loaded.Bands[2] = -12
	if p.Equaliser.Gains[2] != 6 {
		t.Fatal("import retained mutable file slice")
	}
	for _, bad := range []equalizerFile{
		{Version: 2, Bands: make([]float32, 10)},
		{Version: 1, Bands: make([]float32, 9)},
		{Version: 1, Preamp: 13, Bands: make([]float32, 10)},
		{Version: 1, Preamp: float32(math.NaN()), Bands: make([]float32, 10)},
		{Version: 1, Bands: make([]float32, 10), Preset: strings.Repeat("x", 129)},
	} {
		if err := p.applyEqualizerFile(bad); err == nil || changes != 1 || !reflect.DeepEqual(snapshotEqualizer(p.Equaliser), state) {
			t.Fatal("invalid import partially mutated EQ")
		}
	}
	for _, bad := range [][]byte{
		append(append([]byte(nil), data...), []byte(" {}")...),
		bytes.Replace(data, []byte(`"version":1`), []byte(`"version":1,"mystery":2`), 1),
		bytes.Repeat([]byte(" "), 65537),
	} {
		if err := os.WriteFile(path, bad, 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := readEqualizerFile(path); err == nil {
			t.Fatal("malformed/unknown/oversized file accepted")
		}
	}
	files, err := filepath.Glob(filepath.Join(filepath.Dir(path), ".fancy-save-*"))
	if err != nil || len(files) != 0 {
		t.Fatal("atomic writer left temporary files")
	}
}

func TestPlaylistExportLocalPathsAndUnsafeInput(t *testing.T) {
	q := players.NewPlaylist(
		players.Track{Path: "/music/one song.mp3", Artist: "An\rArtist", Title: "A\nSong", Length: 61 * time.Second},
		players.Track{Title: "Synthetic fixture", Length: time.Minute},
		players.Track{Path: "/music/two.flac", Title: "Two"},
	)
	data, err := playlistM3U(q)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "#EXTM3U\n#EXTINF:61,An Artist - A Song\n/music/one song.mp3\n#EXTINF:-1,Two\n/music/two.flac\n" {
		t.Fatalf("wrong M3U: %q", data)
	}
	if _, err := playlistM3U(players.NewPlaylist(players.Track{Title: "fixture"})); err == nil {
		t.Fatal("export invented local media paths")
	}
	q.Tracks[1].Path = "/music/evil\n#EXTINF:0,Injected.mp3"
	if _, err := playlistM3U(q); err == nil {
		t.Fatal("newline path inserted playlist directives")
	}
}

func TestChooserFallbackOwnedSizeAndCloseLifetime(t *testing.T) {
	a, p := open(t, 1.75)
	p.SetCompact(true)
	a.PumpOnce()
	picked := 0
	p.chooseFile(p.compact.play, widgets.FileDialogOptions{Title: "Save Fancy settings", Mode: widgets.FileSave, Path: t.TempDir(), Name: "fancy.json"}, func(string) error { picked++; return nil })
	a.PumpOnce()
	if len(p.dialogs) != 1 {
		t.Fatal("small shaped host failed to open fallback chooser")
	}
	var chooser interface{ Cancel() }
	for fd := range p.dialogs {
		chooser = fd
	}
	var ownWindow bool
	for _, w := range a.Windows() {
		if w == p.Main() || w == p.Eq() || w == p.List() || w.Closed() {
			continue
		}
		off := w.Surface().(*platform.Offscreen)
		width, height := w.Size()
		if off.Owner() != p.Main().Surface() || width < 520 || height < 380 {
			t.Fatal("fallback lacks owned readable window")
		}
		ownWindow = true
	}
	if !ownWindow {
		t.Fatal("fallback chooser remained clipped in compact strip")
	}
	p.Close()
	a.PumpOnce()
	chooser.Cancel()
	if picked != 0 || len(p.dialogs) != 0 {
		t.Fatal("closing presentation retained chooser callback")
	}
	for _, w := range a.Windows() {
		if !w.Closed() {
			t.Fatal("closing presentation left owned chooser open")
		}
	}
}
