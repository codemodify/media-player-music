package shell

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/codemodify/media-player-music/internal/players"
	"goframework.io/desktop-ui-widgets"
	"goframework.io/desktop-ui/platform"
	"goframework.io/desktop-ui/style"
	"goframework.io/desktop-ui/widget"
)

// LoadFiles validates and expands paths before replacing or extending the queue.
// Explicit regular files retain mpv's format support; directories contribute
// known audio files and media containers recursively. GUI loads use the
// asynchronous worker below; mpv plays only their audio tracks.
func (s *Shell) LoadFiles(paths []string, appendToList bool) error {
	if s.closed {
		return fmt.Errorf("the player is closed")
	}
	if len(paths) == 0 {
		return nil
	}
	result, err := expandMusicPaths(context.Background(), paths)
	if err != nil {
		return err
	}
	s.commitFiles(result, appendToList)
	return nil
}

func (s *Shell) commitFiles(result fileLoadResult, appendToList bool) {
	if s.closed || len(result.tracks) == 0 {
		return
	}
	t := s.Host.Model.Transport
	start := !appendToList || t.List.Len() == 0
	if appendToList {
		t.List.Append(result.tracks, t.Random)
	} else {
		t.List = players.NewPlaylist(result.tracks...)
		t.List.Shuffle(t.Random)
	}
	s.Host.Model.Error = ""
	if start {
		t.Pos, t.State = 0, players.Playing
	}
	players.TraceFileDrop("loaded %d tracks; playlist now has %d; append=%t", len(result.tracks), t.List.Len(), appendToList)
	s.refresh()
	s.syncPlayback()
	if result.skipped > 0 {
		s.fileError(fmt.Errorf("Added %d tracks; skipped %d unreadable entries: %v", len(result.tracks), result.skipped, result.firstError))
	}
	s.remember()
}

type fileLoadResult struct {
	tracks     []players.Track
	skipped    int
	firstError error
}

func (r *fileLoadResult) skip(err error) {
	r.skipped++
	if r.firstError == nil {
		r.firstError = err
	}
}

// WalkDir sorts each directory lexically and does not follow directory
// symlinks. File symlinks remain useful, while cycles cannot recurse forever.
// A bad explicitly supplied path aborts the request before the queue changes;
// inaccessible descendants are skipped and reported without stopping audio.
func expandMusicPaths(ctx context.Context, paths []string) (fileLoadResult, error) {
	var result fileLoadResult
	for _, path := range paths {
		if err := ctx.Err(); err != nil {
			return fileLoadResult{}, err
		}
		absolute, err := filepath.Abs(path)
		if err != nil {
			return fileLoadResult{}, err
		}
		info, err := os.Stat(absolute)
		if err != nil {
			return fileLoadResult{}, fmt.Errorf("open music path: %w", err)
		}
		if !info.IsDir() {
			track, err := musicTrack(absolute)
			if err != nil {
				return fileLoadResult{}, err
			}
			result.tracks = append(result.tracks, track)
			continue
		}
		// A directory explicitly dropped through a symlink is a valid root.
		// Resolve it once; nested directory symlinks are still not traversed.
		root, err := filepath.EvalSymlinks(absolute)
		if err != nil {
			return fileLoadResult{}, fmt.Errorf("open music folder: %w", err)
		}
		err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
			if err := ctx.Err(); err != nil {
				return err
			}
			if walkErr != nil {
				result.skip(walkErr)
				if entry != nil && entry.IsDir() {
					return fs.SkipDir
				}
				return nil
			}
			if entry.IsDir() || !mediaExtension(filepath.Ext(entry.Name())) {
				return nil
			}
			// Stat follows file symlinks; a symlink to a directory is ignored.
			info, err := os.Stat(path)
			if err != nil {
				result.skip(err)
				return nil
			}
			if !info.Mode().IsRegular() {
				return nil
			}
			track, err := musicTrack(path)
			if err != nil {
				result.skip(err)
				return nil
			}
			result.tracks = append(result.tracks, track)
			return nil
		})
		if err != nil {
			return fileLoadResult{}, fmt.Errorf("scan music folder: %w", err)
		}
	}
	if len(result.tracks) == 0 && len(paths) > 0 {
		if result.firstError != nil {
			return fileLoadResult{}, fmt.Errorf("no readable audio files found: %w", result.firstError)
		}
		return fileLoadResult{}, errors.New("no audio files found in the selected folders")
	}
	return result, nil
}

func musicTrack(path string) (players.Track, error) {
	info, err := os.Stat(path)
	if err != nil {
		return players.Track{}, fmt.Errorf("open music file: %w", err)
	}
	if !info.Mode().IsRegular() {
		return players.Track{}, fmt.Errorf("%s is not a regular music file", path)
	}
	f, err := os.Open(path)
	if err != nil {
		return players.Track{}, fmt.Errorf("read music file: %w", err)
	}
	if err := f.Close(); err != nil {
		return players.Track{}, fmt.Errorf("close music file: %w", err)
	}
	title := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	return players.Track{Path: path, Title: title, Kind: "audio"}, nil
}

func mediaExtension(ext string) bool {
	switch strings.ToLower(ext) {
	case ".mp3", ".flac", ".wav", ".wave", ".ogg", ".oga", ".opus", ".aac",
		".m4a", ".m4b", ".aif", ".aiff", ".aifc", ".au", ".snd", ".wma",
		".ape", ".wv", ".mpc", ".mka", ".ac3", ".dts", ".amr", ".caf",
		".tta", ".alac", ".dsf", ".dff", ".mpga", ".m4p",
		".mp4", ".m4v", ".webm", ".mkv", ".mov", ".avi", ".flv",
		".mpeg", ".mpg", ".3gp", ".3g2", ".ts", ".m2ts", ".mts", ".vob", ".asf":
		return true
	}
	return false
}

type fileLoadRequest struct {
	paths        []string
	appendToList bool
}
type fileLoader struct {
	requests chan fileLoadRequest
	cancel   context.CancelFunc
	done     chan struct{}
}

// Drop only queues work. The loader commits requests in invocation order on
// the UI thread, so a large folder cannot freeze native drag/drop dispatch or
// finish after a later request and rearrange the user's intended queue order.
func (s *Shell) dropFiles(paths []string) { s.queueFileLoad(paths, true) }

func (s *Shell) queueFileLoad(paths []string, appendToList bool) {
	if s.closed || len(paths) == 0 {
		return
	}
	players.TraceFileDrop("queued paths=%q append=%t", paths, appendToList)
	if s.fileLoader == nil {
		ctx, cancel := context.WithCancel(context.Background())
		loader := &fileLoader{requests: make(chan fileLoadRequest, 32), cancel: cancel, done: make(chan struct{})}
		s.fileLoader = loader
		go func() {
			defer close(loader.done)
			for {
				select {
				case <-ctx.Done():
					return
				case request := <-loader.requests:
					result, err := expandMusicPaths(ctx, request.paths)
					if ctx.Err() != nil {
						return
					}
					s.App.Post(func() {
						if s.closed {
							return
						}
						if err != nil {
							s.fileError(err)
							return
						}
						s.commitFiles(result, request.appendToList)
					})
				}
			}
		}()
	}
	request := fileLoadRequest{append([]string(nil), paths...), appendToList}
	select {
	case s.fileLoader.requests <- request:
	default:
		s.fileError(errors.New("too many pending music file loads; try again when the current scan finishes"))
	}
}

func (s *Shell) cancelFileLoads() {
	if s.fileLoader != nil {
		s.fileLoader.cancel()
	}
}

func (s *Shell) removeTrack() {
	t := s.Host.Model.Transport
	if t.List.Len() == 0 {
		return
	}
	t.List.Remove(t.List.Index(), t.Random)
	t.Pos = 0
	if t.List.Len() == 0 {
		t.State = players.Stopped
	}
	// Even a repeated copy of the same filename is a different queue entry.
	s.playback.list = nil
	s.Host.Model.Error = ""
	s.refresh()
	s.syncPlayback()
	s.remember()
}

func (s *Shell) openFiles(_ widget.Component, appendToList bool) {
	s.App.Post(func() {
		if s.closed {
			return
		}
		if s.fileWindow != nil && !s.fileWindow.Closed() {
			s.fileWindow.Activate()
			return
		}
		title := "Open music file"
		if appendToList {
			title = "Add music file"
		}
		// Keep one independently tracked chooser across skin replacement and
		// temporary tray hiding. The toolkit also automatically hosts a stock
		// chooser when the requesting window is too small for an overlay.
		w, err := players.OpenSized(s.App, platform.WindowOptions{
			Title: title, Role: platform.RoleDialog, Center: true,
			Owner:    s.MainWindow().Surface(),
			Headless: s.opts.Headless, Decorations: platform.DecorationsServer,
			MinWidth: 620, MinHeight: 460,
		}, 680, 500, false)
		if err != nil {
			s.fileError(err)
			return
		}
		s.fileWindow = w
		finish := func() {
			w.Close()
			if s.fileWindow == w {
				s.fileWindow = nil
			}
		}
		path, err := os.UserHomeDir()
		if err != nil {
			path = "."
		}
		if info, err := os.Stat(filepath.Join(path, "Music")); err == nil && info.IsDir() {
			path = filepath.Join(path, "Music")
		}
		if current := s.Host.Model.Transport.Track().Path; current != "" {
			path = filepath.Dir(current)
		}
		fd := widgets.NewFileDialog(widgets.FileDialogOptions{
			Title: title, Mode: widgets.FileOpen, Path: path,
			OnPick: func(path string) {
				finish()
				if !s.closed {
					s.queueFileLoad([]string{path}, appendToList)
				}
			},
			OnCancel: finish,
		})
		ap := style.LookAppearance(s.App.Look())
		ap.Name, ap.FollowDesktop = players.Themed, false
		fd.Overlay().SetLook(style.WithAppearance(s.App.Look(), ap))
		w.SetContent(fd.Overlay())
		w.SetOnCloseRequest(func() bool {
			if s.fileWindow == w {
				s.fileWindow = nil
			}
			return true
		})
		widget.FocusFirstIn(fd.Overlay().Card)
		w.Activate()
	})
}

func (s *Shell) fileError(err error) {
	players.TraceFileDrop("load failed: %v", err)
	s.Host.Model.Error = err.Error()
	s.refresh()
}
