package playback

import (
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

type mpvEngineFixture struct {
	mu       sync.Mutex
	commands [][]any
	events   chan Event
	closed   bool
}

func (w *mpvEngineFixture) Send(c ...any) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return ErrClosed
	}
	w.commands = append(w.commands, append([]any(nil), c...))
	return nil
}
func (w *mpvEngineFixture) Events() <-chan Event { return w.events }
func (w *mpvEngineFixture) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.closed {
		w.closed = true
		close(w.events)
	}
	return nil
}
func (w *mpvEngineFixture) seeks() []float64 {
	w.mu.Lock()
	defer w.mu.Unlock()
	var positions []float64
	for _, c := range w.commands {
		if len(c) > 1 && c[0] == "seek" {
			positions = append(positions, c[1].(float64))
		}
	}
	return positions
}

func mpvFixture(t *testing.T) (*MPVEngine, *mpvEngineFixture) {
	t.Helper()
	w := &mpvEngineFixture{events: make(chan Event)}
	p := WrapMPV(w)
	t.Cleanup(func() {
		if err := p.Close(); err != nil {
			t.Error(err)
		}
		for range p.Events() {
		}
	})
	return p, w
}

func mpvReply(p *MPVEngine, name, data string, args ...any) []Update {
	command, _ := json.Marshal(args)
	return p.Translate(Event{Event: "command-reply", Name: name, Command: command, Data: json.RawMessage(data), Error: "success"})
}

func mpvFixtureLoad(t *testing.T, p *MPVEngine, c Command, id int64) []Update {
	t.Helper()
	if err := p.Apply(c); err != nil {
		t.Fatal(err)
	}
	mpvReply(p, "loadfile", `{"playlist_entry_id":`+jsonNumber(id)+`}`, "loadfile", c.Path, "replace")
	p.Translate(Event{Event: "start-file", PlaylistEntryID: id})
	return p.Translate(Event{Event: "file-loaded"})
}

func jsonNumber(n int64) string { b, _ := json.Marshal(n); return string(b) }

func hasSeeked(updates []Update, epoch int64) bool {
	for _, u := range updates {
		if u.Kind == UpdateSeeked && u.Epoch == epoch {
			return true
		}
	}
	return false
}

func TestMPVSeeksSerializeAndIgnoreUnrelatedRestarts(t *testing.T) {
	p, w := mpvFixture(t)
	mpvFixtureLoad(t, p, Command{Op: OpLoad, Generation: 1, Path: "track.wav", Paused: true, Epoch: 1}, 17)
	for _, c := range []Command{{Op: OpSeek, Generation: 1, Position: 10 * time.Second, Epoch: 2}, {Op: OpSeek, Generation: 1, Position: 20 * time.Second, Epoch: 3}, {Op: OpSeek, Generation: 1, Position: 30 * time.Second, Epoch: 4}} {
		if err := p.Apply(c); err != nil {
			t.Fatal(err)
		}
	}
	if seeks := w.seeks(); len(seeks) != 1 || seeks[0] != 10 {
		t.Fatalf("overlapping seeks sent to untagged protocol: %v", seeks)
	}
	if got := p.Translate(Event{Event: "playback-restart"}); hasSeeked(got, 4) || hasSeeked(got, 2) {
		t.Fatalf("restart before native seek completed an operation: %+v", got)
	}
	p.Translate(Event{Event: "seek"})
	got := p.Translate(Event{Event: "playback-restart"})
	if !hasSeeked(got, 2) || hasSeeked(got, 4) {
		t.Fatalf("earlier completion borrowed newest epoch: %+v", got)
	}
	if seeks := w.seeks(); len(seeks) != 2 || seeks[1] != 30 {
		t.Fatalf("latest queued intent was not selected: %v", seeks)
	}
	if got := p.Translate(Event{Event: "playback-restart"}); hasSeeked(got, 4) {
		t.Fatalf("extra earlier restart completed queued seek: %+v", got)
	}
	p.Translate(Event{Event: "seek"})
	if got := p.Translate(Event{Event: "playback-restart"}); !hasSeeked(got, 4) {
		t.Fatalf("actual newest seek never completed: %+v", got)
	}
	// A clock query issued at load time must retain that epoch even if its
	// reply arrives after the newest seek has finished.
	if got := mpvReply(p, "get_property", "0", "get_property", "time-pos"); len(got) != 0 {
		t.Fatalf("stale query clock accepted after seek: %+v", got)
	}
	got = p.Translate(Event{Event: "property-change", Name: "time-pos", Data: json.RawMessage(`30`)})
	if len(got) != 1 || got[0].Epoch != 4 || got[0].Position != 30*time.Second {
		t.Fatalf("fresh clock lacks current epoch: %+v", got)
	}
}

func TestMPVInitialSeekWaitsForItsNativeSeekEvent(t *testing.T) {
	p, _ := mpvFixture(t)
	mpvFixtureLoad(t, p, Command{Op: OpLoad, Generation: 1, Path: "track.wav", Position: 5 * time.Second, Paused: true, Epoch: 8}, 1)
	if got := p.Translate(Event{Event: "playback-restart"}); hasSeeked(got, 8) {
		t.Fatalf("load restart completed initial seek: %+v", got)
	}
	p.Translate(Event{Event: "seek"})
	if got := p.Translate(Event{Event: "playback-restart"}); !hasSeeked(got, 8) {
		t.Fatalf("initial seek did not complete: %+v", got)
	}
}

func TestMPVFailedLoadIdentityDoesNotRequirePlaylistEntryToRemain(t *testing.T) {
	p, _ := mpvFixture(t)
	if err := p.Apply(Command{Op: OpLoad, Generation: 9, Path: "missing.wav", Epoch: 1}); err != nil {
		t.Fatal(err)
	}
	p.Translate(Event{Event: "start-file", PlaylistEntryID: 44})
	if got := p.Translate(Event{Event: "end-file", PlaylistEntryID: 44, Reason: "error", FileError: "loading failed"}); len(got) != 0 {
		t.Fatalf("unidentified failure was guessed: %+v", got)
	}
	got := mpvReply(p, "loadfile", `{"playlist_entry_id":44}`, "loadfile", "missing.wav", "replace")
	if len(got) != 1 || got[0].Kind != UpdateError || got[0].Generation != 9 || got[0].Error != "loading failed" {
		t.Fatalf("failed entry lost when removed from playlist: %+v", got)
	}
}

func TestMPVOlderFailedLoadWithEmptyPlaylistReportsFailureWithoutGuessingEntry(t *testing.T) {
	p, _ := mpvFixture(t)
	if err := p.Apply(Command{Op: OpLoad, Generation: 9, Path: "missing.wav", Epoch: 1}); err != nil {
		t.Fatal(err)
	}
	mpvReply(p, "loadfile", "null", "loadfile", "missing.wav", "replace")
	p.Translate(Event{Event: "start-file", PlaylistEntryID: 44})
	p.Translate(Event{Event: "end-file", PlaylistEntryID: 44, Reason: "error", FileError: "an untagged failure"})
	got := mpvReply(p, "get_property", "[]", "get_property", "playlist")
	if len(got) != 1 || got[0].Kind != UpdateError || got[0].Generation != 9 || got[0].Error == "an untagged failure" {
		t.Fatalf("empty current-load playlist did not fail safely: %+v", got)
	}
}

func TestMPVOlderReplacedFailureCannotFailPendingValidLoad(t *testing.T) {
	p, _ := mpvFixture(t)
	if err := p.Apply(Command{Op: OpLoad, Generation: 1, Path: "missing.wav"}); err != nil {
		t.Fatal(err)
	}
	mpvReply(p, "loadfile", "null", "loadfile", "missing.wav", "replace")
	p.Translate(Event{Event: "start-file", PlaylistEntryID: 1})
	p.Translate(Event{Event: "end-file", PlaylistEntryID: 1, Reason: "error", FileError: "old untagged failure"})
	if err := p.Apply(Command{Op: OpLoad, Generation: 2, Path: "valid.wav"}); err != nil {
		t.Fatal(err)
	}
	mpvReply(p, "loadfile", "null", "loadfile", "valid.wav", "replace")
	mpvReply(p, "get_property", "[]", "get_property", "playlist") // Old load snapshot.
	if got := mpvReply(p, "get_property", "[]", "get_property", "playlist"); len(got) != 0 {
		t.Fatalf("old failure poisoned current pending load: %+v", got)
	}
	p.Translate(Event{Event: "start-file", PlaylistEntryID: 2})
	p.Translate(Event{Event: "file-loaded"}) // Rechecks identity if the earlier snapshot was empty.
	got := mpvReply(p, "get_property", `[{"id":2}]`, "get_property", "playlist")
	loaded := false
	for _, u := range got {
		if u.Kind == UpdateError {
			t.Fatalf("old failure reached valid load: %+v", u)
		}
		if u.Kind == UpdateLoaded && u.Generation == 2 {
			loaded = true
		}
	}
	if !loaded {
		t.Fatalf("valid pending load never reached ready state: %+v", got)
	}
}

func TestMPVLoadReplyIdentityRejectsReplacedFailure(t *testing.T) {
	p, _ := mpvFixture(t)
	if err := p.Apply(Command{Op: OpLoad, Generation: 1, Path: "first.wav"}); err != nil {
		t.Fatal(err)
	}
	if err := p.Apply(Command{Op: OpLoad, Generation: 2, Path: "second.wav"}); err != nil {
		t.Fatal(err)
	}
	mpvReply(p, "loadfile", `{"playlist_entry_id":1}`, "loadfile", "first.wav", "replace")
	p.Translate(Event{Event: "start-file", PlaylistEntryID: 1})
	if got := p.Translate(Event{Event: "end-file", PlaylistEntryID: 1, Reason: "error", FileError: "old failure"}); len(got) != 0 {
		t.Fatalf("replaced failure reached current load: %+v", got)
	}
	mpvReply(p, "loadfile", `{"playlist_entry_id":2}`, "loadfile", "second.wav", "replace")
	p.Translate(Event{Event: "start-file", PlaylistEntryID: 2})
	got := p.Translate(Event{Event: "end-file", PlaylistEntryID: 2, Reason: "error", FileError: "current failure"})
	if len(got) != 1 || got[0].Kind != UpdateError || got[0].Generation != 2 {
		t.Fatalf("current failure lost: %+v", got)
	}
}

func TestRealMPVEngineFailedLoadsAndRapidPausedSeeks(t *testing.T) {
	if _, err := exec.LookPath("mpv"); err != nil {
		t.Skip("system mpv is not installed")
	}
	e, err := NewMPVEngine(EngineOptions{ExtraArgs: []string{"--ao=null"}})
	if err != nil {
		t.Fatal(err)
	}
	remoteCleanup(t, e)
	l := remoteEventLog{engine: e}
	remoteApply(t, e, Command{Op: OpLoad, Generation: 1, Path: filepath.Join(t.TempDir(), "missing.wav"), Paused: true, Epoch: 1})
	l.wait(t, 0, "missing load error", func(u Update) bool { return u.Kind == UpdateError && u.Generation == 1 })
	corrupt := filepath.Join(t.TempDir(), "corrupt.mp3")
	if err := os.WriteFile(corrupt, []byte("this is not an audio stream"), 0600); err != nil {
		t.Fatal(err)
	}
	remoteApply(t, e, Command{Op: OpLoad, Generation: 2, Path: corrupt, Paused: true, Epoch: 2})
	l.wait(t, 0, "corrupt load error", func(u Update) bool { return u.Kind == UpdateError && u.Generation == 2 })
	remoteApply(t, e, Command{Op: OpLoad, Generation: 3, Path: wav(t, "seek.wav", 2.5), Position: 500 * time.Millisecond, Paused: true, Epoch: 3})
	l.wait(t, 0, "valid load", func(u Update) bool { return u.Kind == UpdateLoaded && u.Generation == 3 })
	l.wait(t, 0, "initial paused seek", func(u Update) bool { return u.Kind == UpdateSeeked && u.Generation == 3 && u.Epoch == 3 })
	for _, c := range []Command{{Op: OpSeek, Generation: 3, Position: time.Second, Epoch: 4}, {Op: OpSeek, Generation: 3, Position: 1500 * time.Millisecond, Epoch: 5}, {Op: OpSeek, Generation: 3, Position: 750 * time.Millisecond, Epoch: 6}} {
		remoteApply(t, e, c)
	}
	l.wait(t, 0, "latest rapid seek", func(u Update) bool { return u.Kind == UpdateSeeked && u.Generation == 3 && u.Epoch == 6 })
	l.wait(t, 0, "authoritative newest clock", func(u Update) bool {
		return u.Kind == UpdatePosition && u.Generation == 3 && u.Epoch == 6 && math.Abs(u.Position.Seconds()-.75) < .05
	})
}
