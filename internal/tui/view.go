// Package tui is media-player-music-tui's screen, on goframework's
// terminal-ui: the player media-player-musicd owns, drawn as cmus draws
// its own, and the person's keys sent to it. It plays nothing itself; it
// shows what the daemon tells (its states) and asks it for the rest
// (Browse, Search). The screen is made apart from the terminal, so that
// tests drive it on uitest's terminal in memory, against the daemon's
// service in their own process.
//
// The views are switched by 1 to 5, as cmus's: the library (artists,
// albums, tracks; genres and years), the queue, the search, the settings
// (the equaliser, ReplayGain, the library's folders) and what plays with
// the keys. Under them, always: the track, where it is, the volume, repeat
// and shuffle, then a line that tells how the connection stands, the
// daemon's error, or takes a command typed after ":".
package tui

import (
	"context"
	"strconv"

	"github.com/codemodify/media-player-music/musicapi"
	ui "goframework.io/terminal-ui"
	widgets "goframework.io/terminal-ui-widgets"
)

// The views, by their keys' order.
const (
	ViewLibrary = iota
	ViewQueue
	ViewSearch
	ViewSettings
	ViewHelp
)

// page is a view: what it shows, and the widget the focus goes to.
type page interface {
	root() ui.Widget
	focus()
}

// View is the screen. Its methods run on the app's goroutine: the
// connection's callbacks post them there (App.Do).
type View struct {
	app    *ui.App
	player musicapi.Player
	calls  chan func(ctx context.Context) error

	state        musicapi.State
	stated       bool // a state came
	queue        musicapi.Queue
	now          musicapi.Now
	library      musicapi.Library
	libraryKnown bool // a library state came
	link         string
	linkProblem  bool
	// starting media-player-musicd (link.Daemon): "Starting…", or why it
	// could not be started, with retry (Ctrl+R)
	daemon        string
	daemonProblem bool
	daemonRetry   func()

	tabs  *widgets.Tabs
	pages []page
	shown int

	library_ *libraryPage
	queue_   *queuePage
	search   *searchPage
	settings *settingsPage
	help     *helpPage

	track     *widgets.Label
	progress  *widgets.ProgressBar
	modes     *widgets.Label
	transport *ui.Flex
	message   *widgets.Label
	prompt    *widgets.Input
	quitting  *widgets.Dialog
}

// New makes the screen of app; SetPlayer gives it the player before the
// app runs.
func New(app *ui.App) *View {
	return &View{app: app, link: "connecting to the music daemon…"}
}

// SetPlayer sets the player the screen asks and tells: link.Start's, or a
// test's.
func (thisRef *View) SetPlayer(player musicapi.Player) { thisRef.player = player }

// Build makes the widgets, and returns the screen's root.
func (thisRef *View) Build() ui.Widget {
	thisRef.library_ = newLibraryPage(thisRef)
	thisRef.queue_ = newQueuePage(thisRef)
	thisRef.search = newSearchPage(thisRef)
	thisRef.settings = newSettingsPage(thisRef)
	thisRef.help = newHelpPage(thisRef)
	thisRef.pages = []page{thisRef.library_, thisRef.queue_, thisRef.search, thisRef.settings, thisRef.help}

	// the pages in the tabs: Left and Right switch them only while the
	// bar has the focus, so a page's seek keys keep the view (R121)
	thisRef.tabs = widgets.NewTabs()
	for index, title := range []string{"Library", "Queue", "Search", "Settings", "Help"} {
		thisRef.tabs.AddTab(strconv.Itoa(index+1)+" "+title, thisRef.pages[index].root())
	}
	thisRef.tabs.OnChange(func(index int) {
		if index != thisRef.shown {
			thisRef.Show(index)
		}
	})
	thisRef.tabs.SetHeight(ui.Fill(1))

	thisRef.track = widgets.NewLabel("")
	thisRef.track.SetTruncate(widgets.TruncateEnd)
	// the bar draws the times after it, in place of its percent (R122)
	thisRef.progress = widgets.NewProgressBar()
	thisRef.modes = widgets.NewLabel("")
	thisRef.modes.SetTone(widgets.ToneMuted)
	thisRef.transport = ui.Row(thisRef.progress, thisRef.modes)
	thisRef.transport.SetGap(1)
	thisRef.message = widgets.NewLabel("")
	thisRef.message.SetTruncate(widgets.TruncateEnd)
	thisRef.prompt = widgets.NewInput("")
	thisRef.prompt.SetHidden(true)
	thisRef.prompt.OnSubmit(thisRef.runPrompt)
	thisRef.prompt.AddCommand(ui.Command{Name: "prompt.cancel", Title: "Cancel", Keys: []string{"escape"}, Hidden: true, Run: thisRef.closePrompt})

	thisRef.addCommands()
	thisRef.showStatus()
	thisRef.app.Do(func() { thisRef.pages[thisRef.shown].focus() })
	return ui.Column(thisRef.tabs, thisRef.track, thisRef.transport, thisRef.message, thisRef.prompt)
}

// Shown is the view shown: ViewLibrary…ViewHelp.
func (thisRef *View) Shown() int { return thisRef.shown }

// Show shows a view, the focus in it.
func (thisRef *View) Show(index int) {
	if index < 0 || index >= len(thisRef.pages) {
		return
	}
	thisRef.shown = index
	if thisRef.tabs.Selected() != index {
		thisRef.tabs.Select(index)
	}
	if thisRef.prompt.Hidden() {
		thisRef.pages[index].focus()
	}
}

// ShowState shows the player as the daemon tells it; an older state than
// the one shown (a call's answer after an event) is left.
func (thisRef *View) ShowState(state musicapi.State) {
	if thisRef.stated && state.Seq != 0 && state.Seq < thisRef.state.Seq {
		return
	}
	thisRef.state, thisRef.stated = state, true
	thisRef.showStatus()
	thisRef.queue_.show()
	thisRef.settings.show()
	thisRef.help.show()
}

// ShowQueue shows the queue's tracks.
func (thisRef *View) ShowQueue(queue musicapi.Queue) {
	thisRef.queue = queue
	thisRef.queue_.show()
}

// ShowNow shows where the track playing is; one of an older state than the
// one shown (from before a seek) is left.
func (thisRef *View) ShowNow(now musicapi.Now) {
	if now.Seq < thisRef.state.Seq {
		return
	}
	thisRef.now = now
	thisRef.showTimes()
}

// ShowLibrary shows the library in short, and lists it again where it
// changed.
func (thisRef *View) ShowLibrary(library musicapi.Library) {
	changed := !thisRef.libraryKnown || library.Version != thisRef.library.Version
	thisRef.library, thisRef.libraryKnown = library, true
	thisRef.settings.show()
	if changed {
		thisRef.library_.refresh()
	}
	thisRef.showStatus()
}

// ShowLink shows how the connection to the daemon stands: "" once
// connected (link's Status).
func (thisRef *View) ShowLink(text string, problem bool) {
	thisRef.link, thisRef.linkProblem = text, problem
	thisRef.showStatus()
}

// ShowDaemon shows how starting media-player-musicd stands (link.Daemon's
// Text and Problem): text while it starts, why it could not be started
// with retry, which Ctrl+R calls, and "" once it runs.
func (thisRef *View) ShowDaemon(text string, problem bool, retry func()) {
	thisRef.daemon, thisRef.daemonProblem, thisRef.daemonRetry = text, problem, retry
	if problem {
		thisRef.daemon += " · Ctrl+R retry"
	}
	thisRef.showStatus()
}

// retryDaemon starts the daemon again after it could not be started.
func (thisRef *View) retryDaemon() {
	if retry := thisRef.daemonRetry; retry != nil {
		thisRef.daemonRetry = nil
		retry()
	}
}

// Connected hears that a connection was made (true) or ended (false):
// the daemon met next may be another, whose states count from 1 again and
// whose library is listed anew.
func (thisRef *View) Connected(connected bool) {
	if !connected {
		thisRef.state.Seq, thisRef.now.Seq = 0, 0
		thisRef.libraryKnown = false
	}
}

// position is where the track playing is, and its length, in
// milliseconds: the Now of the state shown, else the state's.
func (thisRef *View) position() (int64, int64) {
	position, length := thisRef.state.PositionMS, thisRef.state.Track.LengthMS
	if thisRef.now.Seq == thisRef.state.Seq && thisRef.now.Seq != 0 {
		position = thisRef.now.PositionMS
		if thisRef.now.LengthMS > 0 {
			length = thisRef.now.LengthMS
		}
	}
	return position, length
}

// showStatus shows the lines under the views: the track, the transport,
// and the connection's status, the daemon's error or a hint.
func (thisRef *View) showStatus() {
	state := thisRef.state
	thisRef.transport.SetHidden(!thisRef.stated)
	thisRef.track.SetHidden(!thisRef.stated)
	if thisRef.stated {
		text := statusMark(state.Status) + " "
		if state.Index >= 0 && state.Queue.Length > 0 {
			text += trackText(state.Track)
		} else {
			text += "nothing to play: a adds from the library, o a file or folder"
		}
		thisRef.track.SetText(text)
		modes := "vol " + strconv.Itoa(int(state.Volume*100+0.5)) + "%"
		if state.Volume == 0 {
			modes = "muted"
		}
		repeat := state.Repeat
		if repeat == "" {
			repeat = musicapi.RepeatOff
		}
		modes += " rep " + repeat
		if state.Shuffle {
			modes += " shuf"
		}
		thisRef.modes.SetText(modes)
		thisRef.showTimes()
	}
	switch {
	case thisRef.daemon != "":
		thisRef.message.SetText(thisRef.daemon)
		thisRef.message.SetTone(widgets.ToneMuted)
		if thisRef.daemonProblem {
			thisRef.message.SetTone(widgets.ToneDanger)
		}
	case thisRef.link != "":
		thisRef.message.SetText(thisRef.link)
		thisRef.message.SetTone(widgets.ToneMuted)
		if thisRef.linkProblem {
			thisRef.message.SetTone(widgets.ToneDanger)
		}
	case state.Error != "":
		thisRef.message.SetText(state.Error)
		thisRef.message.SetTone(widgets.ToneDanger)
	case thisRef.library.Error != "":
		thisRef.message.SetText("library: " + thisRef.library.Error)
		thisRef.message.SetTone(widgets.ToneWarning)
	case thisRef.library.Scanning:
		thisRef.message.SetText("reading the library… " + strconv.Itoa(thisRef.library.Scanned) + " files")
		thisRef.message.SetTone(widgets.ToneMuted)
	default:
		thisRef.message.SetText("5 help · : command · q quit")
		thisRef.message.SetTone(widgets.ToneMuted)
	}
}

// showTimes shows where the track is: its time, its length and the bar.
func (thisRef *View) showTimes() {
	position, length := thisRef.position()
	text := clock(position)
	if length > 0 {
		text += "/" + clock(length)
		thisRef.progress.SetProgress(float64(position) / float64(length))
	} else {
		thisRef.progress.SetProgress(0)
	}
	thisRef.progress.SetLabel(text)
}

// call calls the daemon off the UI goroutine, in the order the keys came,
// one call at a time; what changes comes back as the daemon's states, to
// every renderer. A call that fails says why in a toast.
func (thisRef *View) call(do func(ctx context.Context, player musicapi.Player) error) {
	if thisRef.player == nil {
		return
	}
	if thisRef.calls == nil {
		thisRef.calls = make(chan func(ctx context.Context) error, 256)
		thisRef.app.Go(func(ctx context.Context) {
			for {
				select {
				case <-ctx.Done():
					return
				case next := <-thisRef.calls:
					if err := next(ctx); err != nil && ctx.Err() == nil {
						thisRef.app.Do(func() { widgets.Notify(thisRef.app, err.Error(), widgets.Warning) })
					}
				}
			}
		})
	}
	player := thisRef.player
	select {
	case thisRef.calls <- func(ctx context.Context) error { return do(ctx, player) }:
	default: // 256 keys ahead of the daemon: this one is dropped
	}
}

// command sends one of the player's keys (musicapi.Command*).
func (thisRef *View) command(name string) func() {
	return func() {
		thisRef.call(func(ctx context.Context, player musicapi.Player) error {
			_, err := player.Command(ctx, name)
			return err
		})
	}
}

// add adds paths to the queue's end, or in its place; play plays the
// first of them once added.
func (thisRef *View) add(paths []string, replace, play bool) {
	if len(paths) == 0 {
		return
	}
	thisRef.call(func(ctx context.Context, player musicapi.Player) error {
		added, err := player.Add(ctx, musicapi.Add{Paths: paths, Replace: replace})
		if err != nil {
			return err
		}
		// an empty queue's first add plays its first track already
		first := added.State.Queue.Length - added.Tracks
		if play && !replace && added.Tracks > 0 && (added.State.Status != musicapi.StatusPlaying || added.State.Index != first) {
			if _, err := player.Play(ctx, first); err != nil {
				return err
			}
		}
		thisRef.app.Do(func() {
			text := strconv.Itoa(added.Tracks) + " tracks added"
			if added.Tracks == 1 {
				text = "1 track added"
			}
			if added.Skipped != "" {
				text += "; " + added.Skipped
			}
			widgets.Notify(thisRef.app, text, widgets.Info)
		})
		return nil
	})
}

// seekBy moves the track playing by delta milliseconds.
func (thisRef *View) seekBy(delta int64) func() {
	return func() {
		position, _ := thisRef.position()
		at := max(position+delta, 0)
		thisRef.call(func(ctx context.Context, player musicapi.Player) error {
			_, err := player.Seek(ctx, at)
			return err
		})
	}
}

// askQuit asks before quitting: the music plays on in the daemon.
func (thisRef *View) askQuit() {
	if thisRef.quitting != nil {
		return
	}
	thisRef.quitting = widgets.Confirm(thisRef.app, "Quit", "Quit the terminal UI? The music plays on in media-player-musicd.", []string{"Quit", "Cancel"}, func(choice int) {
		thisRef.quitting = nil
		if choice == 0 {
			thisRef.app.Quit()
		}
	})
}
