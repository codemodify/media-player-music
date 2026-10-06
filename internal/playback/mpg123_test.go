package playback

import (
	"errors"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestMPG123RejectsKnownContainersWithoutReplacingActiveAudio(t *testing.T) {
	e, commands := helperEngine(t, "mpg123")
	l := remoteEventLog{engine: e}
	remoteApply(t, e, Command{Op: OpLoad, Generation: 1, Path: "active.mp3", Paused: true})
	l.wait(t, 0, "first load", func(u Update) bool { return u.Kind == UpdateLoaded && u.Generation == 1 })
	for _, extension := range []string{".mp4", ".M4A", ".aac", ".flac", ".wav", ".ogg", ".opus", ".webm"} {
		err := e.Apply(Command{Op: OpLoad, Generation: 2, Path: "unsupported" + extension})
		if !errors.Is(err, ErrUnsupported) || !strings.Contains(err.Error(), "use mpv or VLC for "+strings.ToLower(extension)) {
			t.Fatalf("%s: wanted a clear format error, got %v", extension, err)
		}
	}
	remoteApply(t, e, Command{Op: OpSeek, Generation: 1, Position: time.Second, Epoch: 2})
	l.wait(t, 0, "original load remains controllable", func(u Update) bool {
		return u.Kind == UpdateSeeked && u.Generation == 1 && u.Epoch == 2
	})
	if err := e.Close(); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(commands)
	if err != nil {
		t.Fatal(err)
	}
	if loads := strings.Count(string(contents), "LOADPAUSED "); loads != 1 || strings.Contains(string(contents), "unsupported") {
		t.Fatalf("rejected media reached the decoder or replaced playback: %s", contents)
	}
}

func mp3Fixture(t *testing.T) string {
	t.Helper()
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg is required to encode an isolated MP3 fixture")
	}
	path := filepath.Join(t.TempDir(), slaveFixtureName("mp3"))
	if out, err := exec.Command(ffmpeg, "-v", "error", "-f", "lavfi", "-i", "sine=frequency=1000:duration=2", "-metadata", "title=Remote Title", "-metadata", "artist=Remote Artist", "-ar", "44100", "-ac", "2", path).CombinedOutput(); err != nil {
		t.Fatalf("create MP3 fixture: %v: %s", err, out)
	}
	return path
}

func TestRealMPG123RemotePlayback(t *testing.T) {
	if _, err := exec.LookPath("mpg123"); err != nil {
		t.Skip("system mpg123 not installed")
	}
	e, err := NewMPG123(EngineOptions{ExtraArgs: []string{"-o", "sleep"}})
	if err != nil {
		t.Fatal(err)
	}
	remoteCleanup(t, e)
	l := remoteEventLog{engine: e}
	path := mp3Fixture(t)
	remoteApply(t, e, Command{Op: OpLoad, Generation: 1, Path: path, Paused: true, Epoch: 1})
	l.wait(t, 0, "loaded", func(u Update) bool { return u.Kind == UpdateLoaded && u.Generation == 1 })
	l.wait(t, 0, "metadata", func(u Update) bool {
		return u.Kind == UpdateMetadata && u.Info.Title == "Remote Title" && u.Info.Artist == "Remote Artist"
	})
	l.wait(t, 0, "format", func(u Update) bool {
		return u.Kind == UpdateAudioFormat && u.Info.SampleRate == 44100 && u.Info.Channels == 2
	})
	l.wait(t, 0, "duration", func(u Update) bool { return u.Kind == UpdateDuration && math.Abs(u.Duration.Seconds()-2) < .02 })
	remoteApply(t, e, Command{Op: OpVolume, Volume: .25})
	mark := len(l.seen)
	remoteApply(t, e, Command{Op: OpPause, Generation: 1, Paused: false})
	remoteApply(t, e, Command{Op: OpPause, Generation: 1, Paused: false})
	l.wait(t, mark, "play idempotently", func(u Update) bool { return u.Kind == UpdatePosition && u.Position > 150*time.Millisecond })
	remoteApply(t, e, Command{Op: OpPause, Generation: 1, Paused: true})
	remoteApply(t, e, Command{Op: OpSeek, Generation: 1, Position: time.Second, Epoch: 2})
	l.wait(t, mark, "seek while paused", func(u Update) bool {
		return u.Kind == UpdateSeeked && u.Epoch == 2 && math.Abs(u.Position.Seconds()-1) < .05
	})
	remoteApply(t, e, Command{Op: OpPause, Generation: 1, Paused: false})
	l.wait(t, mark, "natural EOF", func(u Update) bool { return u.Kind == UpdateEnded && u.Generation == 1 })
	// MPEG detection remains content based for an unfamiliar file suffix.
	unknown := filepath.Join(t.TempDir(), "mpeg-audio.data")
	bytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(unknown, bytes, 0600); err != nil {
		t.Fatal(err)
	}
	remoteApply(t, e, Command{Op: OpLoad, Generation: 2, Path: unknown, Paused: true})
	l.wait(t, mark, "MPEG with unknown extension", func(u Update) bool { return u.Kind == UpdateLoaded && u.Generation == 2 })
	remoteApply(t, e, Command{Op: OpLoad, Generation: 3, Path: filepath.Join(t.TempDir(), "missing.mp3"), Paused: true})
	l.wait(t, mark, "missing file error", func(u Update) bool { return u.Kind == UpdateError && u.Generation == 3 })
}
