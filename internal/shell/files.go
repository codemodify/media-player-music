package shell

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/codemodify/media-player-music/internal/players"
	"github.com/codemodify/media-player-music/musicapi"
	"goframework.io/desktop-ui-widgets"
	"goframework.io/desktop-ui/platform"
	"goframework.io/desktop-ui/style"
	"goframework.io/desktop-ui/widget"
)

// LoadFiles gives paths to the daemon, which expands them and replaces or
// extends the queue (musicapi.Player's Add): an explicit file in a format
// the player does not play is an error and nothing changes; folders give
// the files the player plays within. The desktop's drops and the file
// dialog go through the asynchronous worker below.
func (s *Shell) LoadFiles(paths []string, appendToList bool) error {
	if s.closed {
		return fmt.Errorf("the player is closed")
	}
	if len(paths) == 0 {
		return nil
	}
	if s.player == nil {
		return errors.New("the player is not connected to media-player-musicd")
	}
	added, err := s.add(context.Background(), paths, appendToList)
	s.ShowState(added.State)
	if err != nil {
		s.fileError(err)
		return err
	}
	s.remember()
	return nil
}

// addTimeout bounds an Add: a folder of many files is read whole.
const addTimeout = 10 * time.Minute

// add asks the daemon to add paths, made absolute: the daemon's folder is
// not the shell's.
func (s *Shell) add(ctx context.Context, paths []string, appendToList bool) (musicapi.Added, error) {
	absolute := make([]string, len(paths))
	for i, path := range paths {
		var err error
		if absolute[i], err = filepath.Abs(path); err != nil {
			return musicapi.Added{}, err
		}
	}
	ctx, cancel := context.WithTimeout(ctx, addTimeout)
	defer cancel()
	added, err := s.player.Add(ctx, musicapi.Add{Paths: absolute, Replace: !appendToList})
	players.TraceFileDrop("added %d tracks; append=%t; err=%v", added.Tracks, appendToList, err)
	return added, err
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
	if s.closed || len(paths) == 0 || s.player == nil {
		return
	}
	players.TraceFileDrop("queued paths=%q append=%t", paths, appendToList)
	if s.fileLoader == nil {
		ctx, cancel := context.WithCancel(context.Background())
		loader := &fileLoader{requests: make(chan fileLoadRequest, 32), cancel: cancel, done: make(chan struct{})}
		s.fileLoader = loader
		s.workers.Add(1)
		go func() {
			defer s.workers.Done()
			defer close(loader.done)
			for {
				select {
				case <-ctx.Done():
					return
				case request := <-loader.requests:
					added, err := s.add(ctx, request.paths, request.appendToList)
					if ctx.Err() != nil {
						return
					}
					s.App.Post(func() {
						if s.closed {
							return
						}
						s.ShowState(added.State)
						if err != nil {
							s.fileError(err)
							return
						}
						s.remember()
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
	if t.List.Len() == 0 || s.player == nil {
		return
	}
	index := t.List.Index()
	s.call(func(ctx context.Context) (musicapi.State, error) { return s.player.Remove(ctx, []int{index}) })
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
