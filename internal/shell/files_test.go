package shell

import (
	"fmt"
	"github.com/codemodify/media-player-music/internal/playback"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/codemodify/media-player-music/internal/players"
	"goframework.io/desktop-ui"
	"goframework.io/desktop-ui/platform"
)

func writeMusicEntry(t *testing.T, root, relative string) string {
	t.Helper()
	path := filepath.Join(root, relative)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("fixture; decoding belongs to the engine's tests"), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestTrackedChooserOwnerAndCancelSurviveSkinReplacement(t *testing.T) {
	for _, skin := range []string{players.DefaultSkin, players.SlateSkin, players.SilverSkin} {
		t.Run(skin, func(t *testing.T) {
			a, s, b := liveShell(t, skin)
			main := s.MainWindow()
			s.Host.OpenFiles(main.Content(), false)
			a.PumpOnce()
			chooser := s.fileWindow
			if chooser == nil || chooser.Surface().(*platform.Offscreen).Owner() != main.Surface() {
				t.Fatal("tracked chooser has no requesting-window owner")
			}
			s.Host.OpenFiles(main.Content(), true)
			a.PumpOnce()
			if s.fileWindow != chooser {
				t.Fatal("a second Open replaced the active chooser")
			}
			next := players.SlateSkin
			if skin != players.DefaultSkin {
				next = players.DefaultSkin
			}
			if err := s.SetSkin(next); err != nil {
				t.Fatal(err)
			}
			a.PumpOnce()
			if chooser.Closed() || s.fileWindow != chooser || b.closed {
				t.Fatal("skin replacement destroyed the independently tracked chooser or backend")
			}
			if chooser.Surface().(*platform.Offscreen).Owner() != s.MainWindow().Surface() {
				t.Fatal("retained chooser still belongs to the destroyed presentation")
			}
			chooser.RequestClose()
			a.PumpOnce()
			if !chooser.Closed() || s.fileWindow != nil || s.Host.Model.Transport.List.Len() != 0 || countCommand(b, playback.OpLoad) != 0 {
				t.Fatal("frame cancellation retained the chooser or changed playback")
			}
			s.Host.OpenFiles(s.MainWindow().Content(), false)
			a.PumpOnce()
			if s.fileWindow == nil || s.fileWindow == chooser || s.fileWindow.Closed() {
				t.Fatal("cancellation left a stale chooser that could not reopen")
			}
		})
	}
}

func waitFileLoad(t *testing.T, a *ui.Application, description string, ready func() bool) {
	t.Helper()
	deadline := time.Now().Add(4 * time.Second)
	for {
		a.PumpOnce()
		if ready() {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for " + description)
		}
		time.Sleep(time.Millisecond)
	}
}

func TestLoadFolderUsesRecursiveDeterministicAudioOrder(t *testing.T) {
	_, s, b := liveShell(t, players.DefaultSkin)
	root := t.TempDir()
	want := []string{
		writeMusicEntry(t, root, "A-first.FLAC"),
		writeMusicEntry(t, root, "nested/a.opus"),
		writeMusicEntry(t, root, "nested/b.wav"),
		writeMusicEntry(t, root, "video.mp4"),
		writeMusicEntry(t, root, "z-last.mp3"),
	}
	for _, other := range []string{"cover.jpg", "readme.txt", "nested/list.m3u"} {
		writeMusicEntry(t, root, other)
	}
	if err := s.LoadFiles([]string{root}, false); err != nil {
		t.Fatal(err)
	}
	list := s.Host.Model.Transport.List
	if list.Len() != len(want) {
		t.Fatalf("folder queue has %d tracks, want %d", list.Len(), len(want))
	}
	for i, path := range want {
		if got := list.At(i); got.Path != path || got.Kind != "audio" {
			t.Fatalf("entry %d=%+v, want %q", i, got, path)
		}
	}
	if list.Index() != 0 || s.Host.Model.Transport.State != players.Playing || countCommand(b, playback.OpLoad) != 1 {
		t.Fatal("opening folder did not start its first track exactly once")
	}
}

func TestDroppedFolderAppendsAudioFromMediaContainers(t *testing.T) {
	a, s, b := liveShell(t, players.DefaultSkin)
	if err := s.LoadFiles(mediaPaths(t, 1), false); err != nil {
		t.Fatal(err)
	}
	loaded(s)
	tr := s.Host.Model.Transport
	pauseAt(t, s, 12*time.Second)
	loads := countCommand(b, playback.OpLoad)
	root := t.TempDir()
	want := []string{
		writeMusicEntry(t, root, "A-song.MP4"),
		writeMusicEntry(t, root, "B-song.mp3"),
		writeMusicEntry(t, root, "nested/a.webm"),
		writeMusicEntry(t, root, "nested/b.m4v"),
		writeMusicEntry(t, root, "z-song.mkv"),
	}
	writeMusicEntry(t, root, "cover.jpg")
	writeMusicEntry(t, root, "notes.txt")
	s.Host.DropFiles([]string{root})
	waitFileLoad(t, a, "folder media containers", func() bool { return tr.List.Len() == len(want)+1 })
	for i, path := range want {
		if tr.List.At(i+1).Path != path {
			t.Fatalf("folder entry %d out of order: %q", i, tr.List.At(i+1).Path)
		}
	}
	if tr.List.Index() != 0 || tr.State != players.Paused || tr.Pos != 12*time.Second || countCommand(b, playback.OpLoad) != loads {
		t.Fatal("folder append interrupted existing playback")
	}
	if s.Host.Model.Error != "" {
		t.Fatalf("folder drop failed: %s", s.Host.Model.Error)
	}
}

func TestFolderSymlinksCannotLoopAndExplicitAliasesRemainUsable(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("requires filesystem symlink privileges")
	}
	_, s, _ := liveShell(t, players.DefaultSkin)
	root := t.TempDir()
	first := writeMusicEntry(t, root, "a.mp3")
	second := writeMusicEntry(t, root, "nested/b.flac")
	if err := os.Symlink(root, filepath.Join(root, "nested", "loop")); err != nil {
		t.Skip(err)
	}
	if err := os.Symlink(first, filepath.Join(root, "c-alias.mp3")); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(t.TempDir(), "folder-alias")
	if err := os.Symlink(root, alias); err != nil {
		t.Fatal(err)
	}
	if err := s.LoadFiles([]string{alias}, false); err != nil {
		t.Fatal(err)
	}
	want := []string{first, filepath.Join(root, "c-alias.mp3"), second}
	list := s.Host.Model.Transport.List
	if list.Len() != len(want) {
		t.Fatalf("symlink scan has %d entries", list.Len())
	}
	for i, path := range want {
		if list.At(i).Path != path {
			t.Fatalf("entry %d=%q, want %q", i, list.At(i).Path, path)
		}
	}
}

func TestFileValidationIsAtomicAndExplicitFormatsAreNotFiltered(t *testing.T) {
	_, s, b := liveShell(t, players.DefaultSkin)
	root := t.TempDir()
	first := writeMusicEntry(t, root, "music.custom-format")
	second := writeMusicEntry(t, root, "second.binary")
	if err := s.LoadFiles([]string{first}, false); err != nil {
		t.Fatal(err)
	}
	loaded(s)
	tr := s.Host.Model.Transport
	pauseAt(t, s, 12*time.Second)
	list, loads := tr.List, countCommand(b, playback.OpLoad)
	for _, paths := range [][]string{{second, filepath.Join(root, "missing.wav")}, {t.TempDir()}} {
		if err := s.LoadFiles(paths, false); err == nil {
			t.Fatalf("invalid replacement accepted: %v", paths)
		}
		if tr.List != list || tr.State != players.Paused || tr.Pos != 12*time.Second || countCommand(b, playback.OpLoad) != loads {
			t.Fatal("invalid replacement changed the current queue or transport")
		}
	}
	if err := s.LoadFiles([]string{second}, true); err != nil {
		t.Fatal(err)
	}
	if list = tr.List; list.Len() != 2 || list.At(1).Path != second || tr.State != players.Paused || tr.Pos != 12*time.Second || countCommand(b, playback.OpLoad) != loads {
		t.Fatal("append restarted or replaced the current transport")
	}
}

func TestFolderSkipsUnreadableEntriesWithoutInterruptingCurrentAudio(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("requires Unix permission enforcement")
	}
	_, s, b := liveShell(t, players.DefaultSkin)
	if err := s.LoadFiles(mediaPaths(t, 1), false); err != nil {
		t.Fatal(err)
	}
	loaded(s)
	tr := s.Host.Model.Transport
	pauseAt(t, s, 9*time.Second)
	loads, stops := countCommand(b, playback.OpLoad), countCommand(b, playback.OpStop)
	root := t.TempDir()
	good := writeMusicEntry(t, root, "good.mp3")
	blocked := writeMusicEntry(t, root, "blocked.wav")
	private := filepath.Join(root, "private")
	writeMusicEntry(t, private, "hidden.flac")
	if err := os.Chmod(blocked, 0000); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(private, 0000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(blocked, 0600); _ = os.Chmod(private, 0700) })
	if err := os.Symlink(filepath.Join(root, "missing"), filepath.Join(root, "broken.mp3")); err != nil {
		t.Fatal(err)
	}
	if err := s.LoadFiles([]string{root}, true); err != nil {
		t.Fatal(err)
	}
	if tr.List.Len() != 2 || tr.List.At(1).Path != good || tr.State != players.Paused || tr.Pos != 9*time.Second || countCommand(b, playback.OpLoad) != loads || countCommand(b, playback.OpStop) != stops {
		t.Fatal("unreadable folder entries disrupted playable music")
	}
	if !strings.Contains(s.Host.Model.Error, "skipped 3 unreadable entries") {
		t.Fatalf("skipped entries were not reported: %q", s.Host.Model.Error)
	}
}

func TestAsyncFileAndFolderDropsAppendInInvocationOrder(t *testing.T) {
	a, s, b := liveShell(t, players.DefaultSkin)
	root := t.TempDir()
	const count = 120
	for i := count - 1; i >= 0; i-- {
		writeMusicEntry(t, root, fmt.Sprintf("%03d.mp3", i))
	}
	last := writeMusicEntry(t, t.TempDir(), "last.custom-format")
	s.Host.DropFiles([]string{root})
	s.Host.DropFiles([]string{last})
	if s.Host.Model.Transport.List.Len() != 0 || countCommand(b, playback.OpLoad) != 0 {
		t.Fatal("drop mutated the UI model before the posted commit")
	}
	waitFileLoad(t, a, "ordered drop commits", func() bool { return s.Host.Model.Transport.List.Len() == count+1 })
	tr := s.Host.Model.Transport
	for i := 0; i < count; i++ {
		if tr.List.At(i).Path != filepath.Join(root, fmt.Sprintf("%03d.mp3", i)) {
			t.Fatalf("folder entry %d out of order", i)
		}
	}
	if tr.List.At(count).Path != last || tr.List.Index() != 0 || tr.State != players.Playing || countCommand(b, playback.OpLoad) != 1 {
		t.Fatal("later drop overtook the folder or restarted the selected track")
	}
}

func TestAsyncFileLoadErrorAndClosePreserveQueue(t *testing.T) {
	t.Run("bad drop", func(t *testing.T) {
		a, s, b := liveShell(t, players.DefaultSkin)
		if err := s.LoadFiles(mediaPaths(t, 1), false); err != nil {
			t.Fatal(err)
		}
		list, loads, stops := s.Host.Model.Transport.List, countCommand(b, playback.OpLoad), countCommand(b, playback.OpStop)
		s.Host.DropFiles([]string{filepath.Join(t.TempDir(), "missing.wav")})
		waitFileLoad(t, a, "drop failure", func() bool { return s.Host.Model.Error != "" })
		if s.Host.Model.Transport.List != list || s.Host.Model.Transport.State != players.Playing || countCommand(b, playback.OpLoad) != loads || countCommand(b, playback.OpStop) != stops {
			t.Fatal("bad drop interrupted the existing queue/audio")
		}
	})
	t.Run("close during scan", func(t *testing.T) {
		a, s, b := liveShell(t, players.DefaultSkin)
		root := t.TempDir()
		for i := 0; i < 120; i++ {
			writeMusicEntry(t, root, fmt.Sprintf("%03d.wav", i))
		}
		s.Host.DropFiles([]string{root})
		loader := s.fileLoader
		s.Close()
		select {
		case <-loader.done:
		case <-time.After(2 * time.Second):
			t.Fatal("folder loader did not stop on Close")
		}
		a.PumpOnce()
		if s.Host.Model.Transport.List.Len() != 0 || countCommand(b, playback.OpLoad) != 0 {
			t.Fatal("completed scan changed a closed player")
		}
	})
}
