package tui

import (
	"strings"

	"github.com/codemodify/media-player-music/musicapi"
	ui "goframework.io/terminal-ui"
	widgets "goframework.io/terminal-ui-widgets"
)

// binding is a key of the app's, everywhere but where text is typed: its
// command, and what the help says of it.
type binding struct {
	name, title string
	keys        []string
	run         func()
}

// help is a line of the help's keys: those of a view of its own too.
type help struct{ keys, title string }

// bindings are the app's keys, as cmus has them where it has them.
func (thisRef *View) bindings() []binding {
	view := func(index int) func() { return func() { thisRef.Show(index) } }
	return []binding{
		{"view.library", "Library", []string{"1"}, view(ViewLibrary)},
		{"view.queue", "Queue", []string{"2"}, view(ViewQueue)},
		{"view.search", "Search", []string{"3"}, view(ViewSearch)},
		{"view.settings", "Settings", []string{"4"}, view(ViewSettings)},
		{"view.help", "Now playing and keys", []string{"5"}, view(ViewHelp)},
		{"search.start", "Search the library", []string{"/"}, thisRef.startSearch},
		{"player.playPause", "Play or pause", []string{"c"}, thisRef.command(musicapi.CommandPlayPause)},
		{"player.play", "Play", []string{"x"}, thisRef.command(musicapi.CommandPlay)},
		{"player.stop", "Stop", []string{"v"}, thisRef.command(musicapi.CommandStop)},
		{"player.next", "Next track", []string{"b"}, thisRef.command(musicapi.CommandNext)},
		{"player.previous", "Previous track", []string{"z"}, thisRef.command(musicapi.CommandPrevious)},
		{"player.seekBack", "Seek 5 s back", []string{"left", "h", ","}, thisRef.command(musicapi.CommandSeekBack)},
		{"player.seekForward", "Seek 5 s on", []string{"right", "l", "."}, thisRef.command(musicapi.CommandSeekForward)},
		{"player.seekBackMinute", "Seek a minute back", []string{"<"}, thisRef.seekBy(-60000)},
		{"player.seekForwardMinute", "Seek a minute on", []string{">"}, thisRef.seekBy(60000)},
		{"player.volumeUp", "Volume up", []string{"+", "="}, thisRef.command(musicapi.CommandVolumeUp)},
		{"player.volumeDown", "Volume down", []string{"-"}, thisRef.command(musicapi.CommandVolumeDown)},
		{"player.mute", "Mute, or the volume back", []string{"m"}, thisRef.command(musicapi.CommandMute)},
		{"player.shuffle", "Shuffle on or off", []string{"s"}, thisRef.command(musicapi.CommandShuffle)},
		{"player.repeat", "Repeat: off, all, one", []string{"r"}, thisRef.command(musicapi.CommandRepeat)},
		{"prompt.add", "Add a file, folder or playlist (:add)", []string{"o"}, func() { thisRef.openPrompt("add ") }},
		{"prompt.save", "Save the queue as a playlist (:save)", []string{"ctrl+s"}, func() { thisRef.openPrompt("save ") }},
		{"prompt.open", "Type a command", []string{":"}, func() { thisRef.openPrompt("") }},
		{"daemon.retry", "Retry starting the music daemon", []string{"ctrl+r"}, thisRef.retryDaemon},
		{"app.askQuit", "Quit", []string{"q"}, thisRef.askQuit},
	}
}

// viewKeys are the keys of the views' own, for the help.
var viewKeys = []help{
	{"Up Down k j", "move; Page Up, Page Down, Home g, End G"},
	{"Library: Enter", "open an artist, album, genre or year; play a track"},
	{"Library: a e", "add to the queue"},
	{"Library: E", "replace the queue"},
	{"Library: Backspace Esc", "back up a level"},
	{"Library: Tab", "artists, genres, years"},
	{"Queue: Enter", "play"},
	{"Queue: D Delete", "remove"},
	{"Queue: p P", "move down, up"},
	{"Queue: C", "clear the queue"},
	{"Search: Enter", "play a track, open an artist or album"},
	{"Search: a e E", "add, replace"},
	{"Settings: Left Right h l", "change"},
	{"Settings: Enter Space", "switch, choose, do"},
	{"Settings: D Delete", "remove a library folder"},
	{"Ctrl+C", "quit at once"},
}

// commandKeys are the commands typed after ":", for the help.
var commandKeys = []help{
	{":add PATH", "add a file, a folder or a playlist"},
	{":replace PATH", "the same, in the queue's place"},
	{":save PATH", "save the queue (.m3u8, .m3u, .pls)"},
	{":clear", "clear the queue"},
	{":seek 1:30", "seek to a time, or +10 -10 seconds"},
	{":vol 50", "the volume, in percent"},
	{":lib-add PATH", "add a folder to the library"},
	{":lib-remove PATH", "take a folder out of the library"},
	{":rescan", "read the library's folders again"},
	{":quit", "quit"},
}

// addCommands adds the app's keys, and the lists' cmus keys. While a
// dialog asks (Quit, Clear the queue, Remove a folder) they do nothing: a
// key pressed there is the dialog's answer or nothing.
func (thisRef *View) addCommands() {
	for _, each := range thisRef.bindings() {
		run := each.run
		thisRef.app.AddCommand(ui.Command{Name: each.name, Title: each.title, Keys: each.keys, Handle: func() bool {
			if !thisRef.asking() {
				run()
			}
			return true
		}})
	}
	thisRef.app.Bind("list.up", "up", "k")
	thisRef.app.Bind("list.down", "down", "j")
	thisRef.app.Bind("list.first", "home", "g")
	thisRef.app.Bind("list.last", "end", "G")
}

// asking reports whether a dialog is open.
func (thisRef *View) asking() bool {
	for _, layer := range thisRef.app.Layers() {
		if layer.Kind() == ui.Dialog {
			return true
		}
	}
	return false
}

// keyText writes bindings as the help shows them: "Left h ,".
func keyText(keys []string) string {
	var labels []string
	for _, key := range keys {
		labels = append(labels, widgets.KeyLabel(key))
	}
	return strings.Join(labels, " ")
}
