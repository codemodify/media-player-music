package playback

import (
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestRealMPlayerSlavePlayback(t *testing.T) {
	exe := os.Getenv("MEDIA_PLAYER_TEST_MPLAYER")
	if exe == "" {
		var err error
		exe, err = exec.LookPath("mplayer")
		if err != nil {
			t.Skip("system MPlayer not installed")
		}
	}
	t.Setenv("MPLAYER_HOME", t.TempDir())
	e, err := NewMPlayer(EngineOptions{Executable: exe, ExtraArgs: []string{"-ao", "null"}})
	if err != nil {
		t.Fatal(err)
	}
	remoteCleanup(t, e)
	l := remoteEventLog{engine: e}
	path := wav(t, slaveFixtureName("wav"), 2.5)
	remoteApply(t, e, Command{Op: OpLoad, Generation: 1, Path: path, Paused: true, Position: time.Second, Epoch: 1})
	l.wait(t, 0, "loaded", func(u Update) bool { return u.Kind == UpdateLoaded && u.Generation == 1 })
	l.wait(t, 0, "audio format", func(u Update) bool {
		return u.Kind == UpdateAudioFormat && u.Info.SampleRate == 44100 && u.Info.Channels == 2
	})
	l.wait(t, 0, "duration", func(u Update) bool { return u.Kind == UpdateDuration && math.Abs(u.Duration.Seconds()-2.5) < .01 })
	l.wait(t, 0, "initial paused seek", func(u Update) bool {
		return u.Kind == UpdateSeeked && u.Epoch == 1 && math.Abs(u.Position.Seconds()-1) < .25
	})
	remoteApply(t, e, Command{Op: OpVolume, Volume: .25})
	mark := len(l.seen)
	remoteApply(t, e, Command{Op: OpPause, Generation: 1, Paused: false})
	remoteApply(t, e, Command{Op: OpPause, Generation: 1, Paused: false})
	l.wait(t, mark, "resume", func(u Update) bool { return u.Kind == UpdatePosition && u.Position > 1200*time.Millisecond })
	remoteApply(t, e, Command{Op: OpPause, Generation: 1, Paused: true})
	remoteApply(t, e, Command{Op: OpSeek, Generation: 1, Position: 1800 * time.Millisecond, Epoch: 2})
	l.wait(t, mark, "seek", func(u Update) bool {
		return u.Kind == UpdateSeeked && u.Epoch == 2 && math.Abs(u.Position.Seconds()-1.8) < .25
	})
	remoteApply(t, e, Command{Op: OpPause, Generation: 1, Paused: false})
	l.wait(t, mark, "natural EOF", func(u Update) bool { return u.Kind == UpdateEnded && u.Generation == 1 })
	remoteApply(t, e, Command{Op: OpLoad, Generation: 2, Path: mp3Fixture(t), Paused: true})
	l.wait(t, mark, "tagged MP3 loaded", func(u Update) bool { return u.Kind == UpdateLoaded && u.Generation == 2 })
	l.wait(t, mark, "MP3 metadata", func(u Update) bool {
		return u.Kind == UpdateMetadata && u.Generation == 2 && u.Info.Title == "Remote Title" && u.Info.Artist == "Remote Artist"
	})
	remoteApply(t, e, Command{Op: OpLoad, Generation: 3, Path: filepath.Join(t.TempDir(), "missing.wav"), Paused: true})
	l.wait(t, mark, "missing file", func(u Update) bool { return u.Kind == UpdateError && u.Generation == 3 })
}
