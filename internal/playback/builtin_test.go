package playback

import (
	"bytes"
	"math"
	"os"
	"path/filepath"
	"testing"
	"testing/synctest"
	"time"

	"github.com/codemodify/media-player-music/internal/tags/tagstest"
	all "goframework.io/system-audio-all"
	"goframework.io/system-audio-spec/audiotest"
)

// The built-in engine plays into audiotest's sound system in memory: no
// device, nothing heard on the host. synctest makes its clocks instant.

func builtinOn(t *testing.T) (*Builtin, *audiotest.System) {
	t.Helper()
	system := audiotest.New()
	b := NewBuiltin(BuiltinOptions{System: all.Wrap(system)})
	t.Cleanup(func() { b.Close() })
	return b, system
}

// collect gathers updates until one of kind for generation, or the timeout
// of the bubble's clock.
func collect(t *testing.T, b *Builtin, generation uint64, kind UpdateKind, timeout time.Duration) []Update {
	t.Helper()
	var got []Update
	deadline := time.After(timeout)
	for {
		select {
		case u := <-b.Events():
			got = append(got, u)
			if u.Generation == generation && u.Kind == kind {
				return got
			}
		case <-deadline:
			t.Fatalf("no update %d for generation %d in %v; got %+v", kind, generation, timeout, got)
		}
	}
}

func find(updates []Update, kind UpdateKind) (Update, bool) {
	for _, u := range updates {
		if u.Kind == kind {
			return u, true
		}
	}
	return Update{}, false
}

func energy(t *testing.T, system *audiotest.System) float64 {
	t.Helper()
	played, err := system.Played(audiotest.Speaker)
	if err != nil {
		t.Fatal(err)
	}
	sum := 0.0
	for _, s := range played.F32 {
		sum += float64(s) * float64(s)
	}
	return sum
}

func TestBuiltinPlaysMP3AndAACToTheirEnd(t *testing.T) {
	for _, tc := range []struct {
		file string
		rate int
	}{{"lame-mono22.mp3", 22050}, {"ffmpeg-mono22.aac", 22050}} {
		t.Run(tc.file, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				b, system := builtinOn(t)
				if err := b.Apply(Command{Op: OpVolume, Volume: 1}); err != nil {
					t.Fatal(err)
				}
				if err := b.Apply(Command{Op: OpLoad, Generation: 1, Path: filepath.Join("testdata", tc.file), Epoch: 1}); err != nil {
					t.Fatal(err)
				}
				updates := collect(t, b, 1, UpdateEnded, 10*time.Second)
				if _, ok := find(updates, UpdateLoaded); !ok {
					t.Fatal("no Loaded")
				}
				duration, ok := find(updates, UpdateDuration)
				if !ok || duration.Duration < 900*time.Millisecond || duration.Duration > 1200*time.Millisecond {
					t.Fatalf("the length of a one-second file: %+v", duration)
				}
				format, ok := find(updates, UpdateAudioFormat)
				if !ok || format.Info.SampleRate != tc.rate || format.Info.Channels != 1 {
					t.Fatalf("the format: %+v", format)
				}
				var last time.Duration
				positions, bands := 0, 0
				for _, u := range updates {
					switch u.Kind {
					case UpdatePosition:
						if u.Position < last || u.Epoch != 1 {
							t.Fatalf("a position going back, or of another epoch: %+v after %v", u, last)
						}
						last = u.Position
						positions++
					case UpdateSpectrum:
						if len(u.Bands) != 48 {
							t.Fatalf("%d bands", len(u.Bands))
						}
						for _, v := range u.Bands {
							if v < 0 || v > 1 || math.IsNaN(float64(v)) {
								t.Fatalf("a band outside 0..1: %v", v)
							}
							if v > 0 {
								bands++
							}
						}
					case UpdateError:
						t.Fatalf("an error: %s", u.Error)
					}
				}
				if positions < 5 || last < 500*time.Millisecond || bands == 0 {
					t.Fatalf("%d positions to %v, %d bands lit", positions, last, bands)
				}
				if energy(t, system) == 0 {
					t.Fatal("nothing was played into the sound system")
				}
			})
		})
	}
}

func TestBuiltinReportsID3TagsAsMetadata(t *testing.T) {
	audio, err := os.ReadFile(filepath.Join("testdata", "lame-mono22.mp3"))
	if err != nil {
		t.Fatal(err)
	}
	tag := tagstest.V2(3,
		tagstest.Frame{ID: "TIT2", Data: tagstest.Text(3, "Slow Ferry")},
		tagstest.Frame{ID: "TPE1", Data: tagstest.Text(3, "Kite & Tallow")},
		tagstest.Frame{ID: "TALB", Data: tagstest.Text(3, "Low Tide")})
	path := filepath.Join(t.TempDir(), "tagged.mp3")
	if err := os.WriteFile(path, bytes.Join([][]byte{tag, audio}, nil), 0o600); err != nil {
		t.Fatal(err)
	}
	synctest.Test(t, func(t *testing.T) {
		b, _ := builtinOn(t)
		b.Apply(Command{Op: OpLoad, Generation: 7, Path: path})
		updates := collect(t, b, 7, UpdateMetadata, 5*time.Second)
		meta, _ := find(updates, UpdateMetadata)
		if meta.Info != (TrackInfo{Title: "Slow Ferry", Artist: "Kite & Tallow", Album: "Low Tide"}) {
			t.Fatalf("metadata: %+v", meta.Info)
		}
	})
}

func TestBuiltinLoadsAtAPositionPausedThenPlays(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		b, _ := builtinOn(t)
		b.Apply(Command{Op: OpLoad, Generation: 3, Path: filepath.Join("testdata", "lame-mono22.mp3"), Position: 600 * time.Millisecond, Paused: true, Epoch: 4})
		updates := collect(t, b, 3, UpdateSeeked, 5*time.Second)
		if seeked, _ := find(updates, UpdateSeeked); seeked.Epoch != 4 {
			t.Fatalf("the seek on load's epoch: %+v", seeked)
		}
		// Paused: it stays where it was put.
		time.Sleep(500 * time.Millisecond)
		for len(b.Events()) > 0 {
			<-b.Events()
		}
		b.Apply(Command{Op: OpPause, Generation: 3, Paused: false})
		updates = collect(t, b, 3, UpdateEnded, 5*time.Second)
		first, ok := find(updates, UpdatePosition)
		if !ok || first.Position < 550*time.Millisecond {
			t.Fatalf("it did not play on from 600 ms: %+v", first)
		}
	})
}

// A seek right after a load, before the track has opened, is made as it
// opens (goframework R95): one Seeked, of the seek's epoch, and the track
// heard from there.
func TestBuiltinSeeksBeforeTheTrackOpens(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		b, _ := builtinOn(t)
		b.Apply(Command{Op: OpLoad, Generation: 5, Path: filepath.Join("testdata", "lame-mono22.mp3"), Epoch: 1})
		b.Apply(Command{Op: OpSeek, Generation: 5, Position: 800 * time.Millisecond, Epoch: 2})
		updates := collect(t, b, 5, UpdateSeeked, 5*time.Second)
		if seeked, _ := find(updates, UpdateSeeked); seeked.Epoch != 2 || seeked.Position != 800*time.Millisecond {
			t.Fatalf("the seek's report: %+v", seeked)
		}
		if u, ok := find(updates, UpdateError); ok {
			t.Fatalf("an error: %+v", u)
		}
		updates = collect(t, b, 5, UpdatePosition, 5*time.Second)
		if at := updates[len(updates)-1]; at.Position < 800*time.Millisecond {
			t.Fatalf("after a seek to 800 ms: %+v", at)
		}
	})
}

func TestBuiltinSeekReplaceAndStop(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		b, _ := builtinOn(t)
		mp3, aac := filepath.Join("testdata", "lame-mono22.mp3"), filepath.Join("testdata", "ffmpeg-mono22.aac")
		b.Apply(Command{Op: OpDSP, DSP: DSP{Balance: -.5, Enabled: true, Preamp: -3, Gains: [10]float32{6, 0, 0, 0, 0, 0, 0, 0, 0, -6}}})
		b.Apply(Command{Op: OpLoad, Generation: 1, Path: mp3, Epoch: 1})
		collect(t, b, 1, UpdatePosition, 5*time.Second)
		b.Apply(Command{Op: OpSeek, Generation: 1, Position: 700 * time.Millisecond, Epoch: 2})
		updates := collect(t, b, 1, UpdateSeeked, 5*time.Second)
		if seeked, _ := find(updates, UpdateSeeked); seeked.Epoch != 2 {
			t.Fatalf("the seek's epoch: %+v", seeked)
		}
		update := collect(t, b, 1, UpdatePosition, 5*time.Second)
		if at := update[len(update)-1]; at.Position < 650*time.Millisecond {
			t.Fatalf("after a seek to 700 ms: %+v", at)
		}
		// A new load replaces the old one: nothing of generation 1 after it.
		b.Apply(Command{Op: OpLoad, Generation: 2, Path: aac, Epoch: 3})
		updates = collect(t, b, 2, UpdateEnded, 5*time.Second)
		for i, u := range updates {
			if u.Generation == 1 && u.Kind != UpdatePosition && u.Kind != UpdateSpectrum && u.Kind != UpdateSeeked {
				t.Fatalf("update %d of a replaced load: %+v", i, u)
			}
		}
		// A file that does not exist fails as its own generation.
		b.Apply(Command{Op: OpLoad, Generation: 3, Path: filepath.Join(t.TempDir(), "missing.mp3")})
		updates = collect(t, b, 3, UpdateError, 5*time.Second)
		if u, _ := find(updates, UpdateError); u.Error == "" {
			t.Fatal("an error without a message")
		}
		b.Apply(Command{Op: OpLoad, Generation: 4, Path: mp3})
		collect(t, b, 4, UpdatePosition, 5*time.Second)
		b.Apply(Command{Op: OpStop})
		time.Sleep(time.Second)
		for len(b.Events()) > 0 {
			if u := <-b.Events(); u.Kind == UpdateEnded {
				t.Fatalf("a stopped track ended: %+v", u)
			}
		}
		if err := b.Close(); err != nil {
			t.Fatal(err)
		}
		if err := b.Apply(Command{Op: OpStop}); err != ErrClosed {
			t.Fatalf("Apply after Close: %v", err)
		}
	})
}

// Monkey's Audio (goframework R101, codecs-audio-ape): a file of each
// compression level (fast, normal, high, extra high, insane; 0.6 s of
// 44.1 kHz stereo made by Monkey's Audio 13.46, the module's testdata)
// plays to its end into the sound system, with its length, format,
// positions and bands.
func TestBuiltinPlaysMonkeysAudio(t *testing.T) {
	for _, level := range []string{"1000", "2000", "3000", "4000", "5000"} {
		t.Run("c"+level, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				b, system := builtinOn(t)
				b.Apply(Command{Op: OpVolume, Volume: 1})
				b.Apply(Command{Op: OpLoad, Generation: 1, Path: filepath.Join("testdata", "stereo-c"+level+".ape"), Epoch: 1})
				updates := collect(t, b, 1, UpdateEnded, 10*time.Second)
				if u, ok := find(updates, UpdateError); ok {
					t.Fatalf("an error: %s", u.Error)
				}
				duration, ok := find(updates, UpdateDuration)
				if !ok || duration.Duration < 550*time.Millisecond || duration.Duration > 650*time.Millisecond {
					t.Fatalf("the length of a 0.6 s file: %+v", duration)
				}
				format, ok := find(updates, UpdateAudioFormat)
				if !ok || format.Info.SampleRate != 44100 || format.Info.Channels != 2 {
					t.Fatalf("the format: %+v", format)
				}
				positions, bands := 0, 0
				var last time.Duration
				for _, u := range updates {
					switch u.Kind {
					case UpdatePosition:
						last = u.Position
						positions++
					case UpdateSpectrum:
						for _, v := range u.Bands {
							if v > 0 {
								bands++
							}
						}
					}
				}
				if positions < 3 || last < 300*time.Millisecond || bands == 0 {
					t.Fatalf("%d positions to %v, %d bands lit", positions, last, bands)
				}
				if energy(t, system) == 0 {
					t.Fatal("nothing was played into the sound system")
				}
			})
		})
	}
}

// An APE file seeks (by frame, then to the sample), and its APE tag is the
// now-playing text.
func TestBuiltinSeeksMonkeysAudioAndReadsItsTags(t *testing.T) {
	audio, err := os.ReadFile(filepath.Join("testdata", "stereo-c2000.ape"))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "tagged.ape")
	tag := tagstest.APE([2]string{"Title", "Slow Ferry"}, [2]string{"Artist", "Kite & Tallow"}, [2]string{"Album", "Low Tide"})
	if err := os.WriteFile(path, append(audio, tag...), 0o600); err != nil {
		t.Fatal(err)
	}
	synctest.Test(t, func(t *testing.T) {
		b, _ := builtinOn(t)
		b.Apply(Command{Op: OpLoad, Generation: 2, Path: path, Epoch: 1})
		updates := collect(t, b, 2, UpdateMetadata, 5*time.Second)
		if meta, _ := find(updates, UpdateMetadata); meta.Info != (TrackInfo{Title: "Slow Ferry", Artist: "Kite & Tallow", Album: "Low Tide"}) {
			t.Fatalf("metadata: %+v", meta.Info)
		}
		b.Apply(Command{Op: OpSeek, Generation: 2, Position: 400 * time.Millisecond, Epoch: 2})
		updates = collect(t, b, 2, UpdateSeeked, 5*time.Second)
		if seeked, _ := find(updates, UpdateSeeked); seeked.Epoch != 2 || seeked.Position != 400*time.Millisecond {
			t.Fatalf("the seek's report: %+v", seeked)
		}
		updates = collect(t, b, 2, UpdatePosition, 5*time.Second)
		if at := updates[len(updates)-1]; at.Position < 400*time.Millisecond {
			t.Fatalf("after a seek to 400 ms: %+v", at)
		}
		if u, ok := find(updates, UpdateError); ok {
			t.Fatalf("an error: %+v", u)
		}
	})
}
