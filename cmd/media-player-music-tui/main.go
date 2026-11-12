// Command media-player-music-tui is media-player-music's terminal UI, on
// goframework's terminal-ui, as cmus is laid out: the player
// media-player-musicd owns, in a terminal or over SSH, the same one the
// desktop and web UIs show. It plays nothing itself.
//
//	media-player-music-tui                                  the daemon on this machine
//	media-player-music-tui --daemon ws://host:50130/clients
//
// 1 to 5 show the library (artists, albums, tracks; Tab for genres and
// years), the queue, the search (/ from anywhere), the settings (the
// equaliser, ReplayGain, the library's folders) and what plays with every
// key. The keys are cmus's: c play or pause, x play, v stop, b next, z
// previous, Left and Right (h l , .) seek, + and - the volume, s shuffle,
// r repeat, o adds a path, Ctrl+S saves the queue, : a command, q quits.
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/codemodify/media-player-music/internal/link"
	"github.com/codemodify/media-player-music/internal/tui"
	"github.com/codemodify/media-player-music/musicapi"
	"goframework.io/cliflags"
	"goframework.io/terminal"
	ui "goframework.io/terminal-ui"
	widgets "goframework.io/terminal-ui-widgets"
)

// Version is media-player-music-tui's; a release may set it with
// -ldflags "-X main.Version=...".
var Version = "0.0.1"

type flags struct {
	Daemon string `flagName:"daemon" flagDescription:"media-player-musicd's address: unix://… (its socket in the user's runtime folder by default) or ws://…"`
	Token  string `flagName:"token" flagDescription:"the token media-player-musicd admits a ws:// renderer by; by default the one in media-player-musicd.token in the user's runtime folder"`
}

func main() {
	width := 0
	if size, err := terminal.SizeOf(os.Stdout); err == nil {
		width = size.Columns
	}
	if err := cliflags.Execute(command, cliflags.HelpWidth(width)); err != nil {
		fmt.Fprintln(os.Stderr, "media-player-music-tui:", err)
		os.Exit(1)
	}
}

func command() (*cliflags.Command, error) {
	return &cliflags.Command{
		Name:        "media-player-music-tui",
		Version:     Version, // --version (goframework R105)
		Description: "media-player-music in a terminal: the player media-player-musicd owns, with cmus's keys",
		Flags:       &flags{},
		Examples:    []string{"media-player-music-tui", "media-player-music-tui --daemon ws://127.0.0.1:50130/clients"},
		Handler:     func(invocation *cliflags.Invocation) error { return run(invocation.Flags.(*flags)) },
	}, nil
}

func run(given *flags) error {
	address := given.Daemon
	if address == "" {
		var err error
		if address, err = link.DefaultAddress(); err != nil {
			return err
		}
	}
	// the socket needs no token (only the user reaches it); a WebSocket,
	// which every user of the machine reaches, the one the user's programs
	// share (media-player-musicd.token), unless one is given
	token, err := link.Token(address, given.Token)
	if err != nil {
		return err
	}
	ctx, quit := context.WithCancel(context.Background())
	defer quit()

	app := ui.New(ui.Options{Mouse: ui.MouseClicks, Title: "media-player-music"})
	view := tui.New(app)
	view.SetPlayer(link.Start(ctx, options(app, view, address, token)))
	return app.Run(view.Build())
}

// options are the connection's: each state the daemon tells, and how the
// connection stands, posted to the screen.
func options(app *ui.App, view *tui.View, address, token string) link.Options {
	return link.Options{
		Address: address, Name: "terminal", Version: Version, Token: token,
		State:     func(state musicapi.State) { app.Do(func() { view.ShowState(state) }) },
		Queue:     func(queue musicapi.Queue) { app.Do(func() { view.ShowQueue(queue) }) },
		Now:       func(now musicapi.Now) { app.Do(func() { view.ShowNow(now) }) },
		Library:   func(library musicapi.Library) { app.Do(func() { view.ShowLibrary(library) }) },
		Status:    func(text string, problem bool) { app.Do(func() { view.ShowLink(text, problem) }) },
		Connected: func(connected bool) { app.Do(func() { view.Connected(connected) }) },
		Failed: func(err error) {
			app.Do(func() {
				view.ShowLink(err.Error(), true)
				widgets.Notify(app, err.Error(), widgets.Failure)
			})
		},
		// F1: goframework's ensure-daemon facility starts media-player-musicd
		// here when none answers, once it lands (release-gaps.md)
		Ensure: nil,
	}
}
