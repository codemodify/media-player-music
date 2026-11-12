package tui

import (
	"context"
	"errors"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/codemodify/media-player-music/musicapi"
	widgets "goframework.io/terminal-ui-widgets"
)

// openPrompt shows the command line in place of the message, the text
// given typed already (":add "), the focus in it.
func (thisRef *View) openPrompt(text string) {
	thisRef.prompt.SetText(":" + text)
	thisRef.prompt.SetHidden(false)
	thisRef.message.SetHidden(true)
	thisRef.prompt.Focus()
}

// closePrompt hides the command line, the focus back in the view.
func (thisRef *View) closePrompt() {
	thisRef.prompt.SetHidden(true)
	thisRef.message.SetHidden(false)
	thisRef.pages[thisRef.shown].focus()
}

// runPrompt runs the command typed.
func (thisRef *View) runPrompt(text string) {
	thisRef.closePrompt()
	if err := thisRef.run(text); err != nil {
		widgets.Notify(thisRef.app, err.Error(), widgets.Warning)
	}
}

// run runs a command typed after ":", as cmus's: its name, then what it
// takes, a path with its spaces.
func (thisRef *View) run(text string) error {
	text = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(text), ":"))
	if text == "" {
		return nil
	}
	name, argument, _ := strings.Cut(text, " ")
	argument = strings.TrimSpace(argument)
	needs := func(what string) error {
		if argument == "" {
			return errors.New(":" + name + " needs " + what)
		}
		return nil
	}
	switch name {
	case "add", "a":
		if err := needs("a path"); err != nil {
			return err
		}
		thisRef.add([]string{localPath(argument)}, false, false)
	case "replace":
		if err := needs("a path"); err != nil {
			return err
		}
		thisRef.add([]string{localPath(argument)}, true, false)
	case "save", "w":
		if err := needs("a playlist's path (.m3u8, .m3u, .pls)"); err != nil {
			return err
		}
		path := localPath(argument)
		thisRef.call(func(ctx context.Context, player musicapi.Player) error {
			if _, err := player.SavePlaylist(ctx, path); err != nil {
				return err
			}
			thisRef.app.Do(func() { widgets.Notify(thisRef.app, "Saved the queue as "+path, widgets.Info) })
			return nil
		})
	case "clear":
		thisRef.call(func(ctx context.Context, player musicapi.Player) error { _, err := player.Clear(ctx); return err })
	case "seek":
		if err := needs("a time"); err != nil {
			return err
		}
		ms, relative, err := parseTime(argument)
		if err != nil {
			return err
		}
		if relative {
			thisRef.seekBy(ms)()
		} else {
			thisRef.call(func(ctx context.Context, player musicapi.Player) error { _, err := player.Seek(ctx, ms); return err })
		}
	case "vol", "volume":
		percent, err := strconv.ParseFloat(strings.TrimSuffix(argument, "%"), 64)
		if err != nil || math.IsNaN(percent) {
			return errors.New(":vol needs a volume in percent, 0 to 100")
		}
		levels := musicapi.Levels{Volume: min(max(percent, 0), 100) / 100, Balance: thisRef.state.Balance}
		thisRef.call(func(ctx context.Context, player musicapi.Player) error {
			_, err := player.SetLevels(ctx, levels)
			return err
		})
	case "lib-add":
		if err := needs("a folder"); err != nil {
			return err
		}
		thisRef.addFolder(localPath(argument))
	case "lib-remove":
		if err := needs("a folder"); err != nil {
			return err
		}
		path := localPath(argument)
		thisRef.call(func(ctx context.Context, player musicapi.Player) error {
			_, err := player.RemoveFolder(ctx, path)
			return err
		})
	case "rescan":
		thisRef.call(func(ctx context.Context, player musicapi.Player) error { _, err := player.Rescan(ctx); return err })
	case "quit", "q":
		thisRef.app.Quit()
	default:
		return errors.New("no command :" + name + " (5 lists them)")
	}
	return nil
}

// addFolder adds a folder to the library.
func (thisRef *View) addFolder(path string) {
	thisRef.call(func(ctx context.Context, player musicapi.Player) error {
		_, err := player.AddFolder(ctx, path)
		return err
	})
}

// localPath is a path typed, as this machine's: ~ the home folder, a
// relative one from the folder the UI started in. The daemon takes
// absolute paths.
func localPath(path string) string {
	if path == "~" || strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			path = filepath.Join(home, path[1:])
		}
	}
	if absolute, err := filepath.Abs(path); err == nil {
		return absolute
	}
	return path
}

// parseTime reads a time typed: 90, 1:30 or 1:02:30 from the track's
// start; +10 or -10 seconds from where it is (relative). It is in
// milliseconds.
func parseTime(text string) (int64, bool, error) {
	sign := int64(1)
	relative := strings.HasPrefix(text, "+") || strings.HasPrefix(text, "-")
	if strings.HasPrefix(text, "-") {
		sign = -1
	}
	text = strings.TrimLeft(text, "+-")
	var seconds int64
	parts := strings.Split(text, ":")
	if len(parts) > 3 {
		return 0, false, errors.New(":seek needs a time: 90, 1:30, +10, -10")
	}
	for _, part := range parts {
		n, err := strconv.ParseInt(part, 10, 64)
		if err != nil || n < 0 {
			return 0, false, errors.New(":seek needs a time: 90, 1:30, +10, -10")
		}
		seconds = seconds*60 + n
	}
	return sign * seconds * 1000, relative, nil
}
