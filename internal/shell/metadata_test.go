package shell

import (
	"encoding/json"
	"testing"

	"github.com/codemodify/media-player-music/internal/playback"
	"github.com/codemodify/media-player-music/internal/players"
)

func failParamsSnapshot(s *Shell) {
	command, _ := json.Marshal([]string{"get_property", "audio-params"})
	s.playbackEvent(playback.Event{
		Event: "command-error", Name: "get_property", Command: command, Error: "property unavailable",
	})
}

func paramsQueryCount(b *recordingBackend) int {
	count := 0
	for _, command := range b.commands {
		if len(command) == 2 && command[0] == "get_property" && command[1] == "audio-params" {
			count++
		}
	}
	return count
}

// mpv may emit file-loaded before its decoder format is available. A paused
// replacement with the same format can emit only playback-restart afterwards,
// without an audio-params property-change. Its snapshot must still recover.
func TestDecoderParamsSnapshotRecoversAtPlaybackRestart(t *testing.T) {
	for _, replyFirst := range []bool{true, false} {
		name := "restart before unavailable reply"
		if replyFirst {
			name = "unavailable reply before restart"
		}
		t.Run(name, func(t *testing.T) {
			_, s, b := liveShell(t, players.DefaultSkin)
			if err := s.LoadFiles(mediaPaths(t, 1), false); err != nil {
				t.Fatal(err)
			}
			// A queue entry can carry metadata from an earlier play. Recovery
			// must be based on this decoder's readiness, not cached values.
			track := &s.Host.Model.Transport.List.Tracks[0]
			track.Rate, track.Channels = 96, 6
			s.Host.Command(players.CmdPlayPause)
			identify(s, 101)
			if got := paramsQueryCount(b); got != 1 {
				t.Fatalf("file-loaded requested %d parameter snapshots, want one", got)
			}
			if replyFirst {
				failParamsSnapshot(s)
			}
			s.playbackEvent(playback.Event{Event: "playback-restart"})
			if !replyFirst {
				failParamsSnapshot(s)
			}
			if got := paramsQueryCount(b); got != 2 {
				t.Fatalf("decoder readiness requested %d snapshots, want initial and recovery", got)
			}
			reply(s, "audio-params", `{"samplerate":44100,"channel-count":2}`)
			if track.Rate != 44 || track.Channels != 2 || s.Host.Model.Transport.State != players.Paused {
				t.Fatalf("paused decoder metadata did not recover: rate=%d channels=%d state=%s", track.Rate, track.Channels, s.Host.Model.Transport.State)
			}
			s.playbackEvent(playback.Event{Event: "playback-restart"})
			if got := paramsQueryCount(b); got != 2 {
				t.Fatalf("already known decoder format requested another snapshot: %d", got)
			}
		})
	}
}

func TestDecoderParamsRecoveryRejectsSupersededLoad(t *testing.T) {
	_, s, b := liveShell(t, players.DefaultSkin)
	if err := s.LoadFiles(mediaPaths(t, 2), false); err != nil {
		t.Fatal(err)
	}
	identify(s, 201)
	failParamsSnapshot(s)
	s.playbackEvent(playback.Event{Event: "playback-restart"})
	oldGeneration := s.playback.generation
	s.Host.Command(players.CmdNext)
	identify(s, 202)
	if s.playback.generation == oldGeneration {
		t.Fatal("Next did not create a new decoder load")
	}
	// The first reply belongs to the old decoder's recovery request. Even
	// after the new file is loaded, its format must not leak into that row.
	reply(s, "audio-params", `{"samplerate":96000,"channel-count":6}`)
	track := &s.Host.Model.Transport.List.Tracks[1]
	if track.Rate != 0 || track.Channels != 0 {
		t.Fatalf("superseded decoder changed the current row: rate=%d channels=%d", track.Rate, track.Channels)
	}
	failParamsSnapshot(s)
	s.playbackEvent(playback.Event{Event: "playback-restart"})
	if got := paramsQueryCount(b); got != 4 {
		t.Fatalf("current decoder did not recover independently: %d snapshots", got)
	}
	reply(s, "audio-params", `{"samplerate":48000,"channel-count":2}`)
	if track.Rate != 48 || track.Channels != 2 {
		t.Fatalf("current decoder format did not recover: rate=%d channels=%d", track.Rate, track.Channels)
	}
}
