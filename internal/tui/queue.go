package tui

import (
	"context"
	"strconv"

	"github.com/codemodify/media-player-music/musicapi"
	ui "goframework.io/terminal-ui"
	widgets "goframework.io/terminal-ui-widgets"
)

// queuePage is the queue's view: the daemon's queue, the track playing
// marked.
type queuePage struct {
	view    *View
	box     *ui.Flex
	heading *widgets.Label
	count   *widgets.Label
	list    *widgets.List
	asking  *widgets.Dialog
}

func newQueuePage(view *View) *queuePage {
	thisRef := &queuePage{view: view}
	thisRef.heading = widgets.NewLabel("Queue")
	thisRef.heading.SetTone(widgets.ToneAccent)
	thisRef.heading.SetWidth(ui.Fill(1))
	thisRef.count = widgets.NewLabel("")
	thisRef.count.SetTone(widgets.ToneMuted)
	thisRef.list = widgets.NewList(0, thisRef.paint)
	thisRef.list.SetHeight(ui.Fill(1))
	thisRef.list.OnActivate(func(index int) {
		thisRef.view.call(func(ctx context.Context, player musicapi.Player) error { _, err := player.Play(ctx, index); return err })
	})
	thisRef.list.AddCommand(ui.Command{Name: "queue.remove", Title: "Remove", Keys: []string{"D", "delete"}, Run: thisRef.remove})
	thisRef.list.AddCommand(ui.Command{Name: "queue.down", Title: "Move down", Keys: []string{"p"}, Run: func() { thisRef.move(+1) }})
	thisRef.list.AddCommand(ui.Command{Name: "queue.up", Title: "Move up", Keys: []string{"P"}, Run: func() { thisRef.move(-1) }})
	thisRef.list.AddCommand(ui.Command{Name: "queue.clear", Title: "Clear", Keys: []string{"C"}, Run: thisRef.clear})
	header := ui.Row(thisRef.heading, thisRef.count)
	header.SetGap(1)
	header.SetPadding(ui.Symmetric(0, 1))
	thisRef.box = ui.Column(header, thisRef.list)
	return thisRef
}

func (thisRef *queuePage) root() ui.Widget { return thisRef.box }
func (thisRef *queuePage) focus()          { thisRef.list.Focus() }

// show shows the queue as the daemon tells it.
func (thisRef *queuePage) show() {
	tracks := thisRef.view.queue.Tracks
	if thisRef.list.Selected() >= len(tracks) && len(tracks) > 0 {
		thisRef.list.SetSelected(len(tracks) - 1)
	}
	thisRef.list.SetCount(len(tracks))
	thisRef.list.Invalidate()
	count := ""
	if thisRef.view.stated {
		count = strconv.Itoa(len(tracks)) + " · " + clock(thisRef.view.state.Queue.LengthMS)
		if len(tracks) == 0 {
			count = "empty"
		}
	}
	thisRef.count.SetText(count)
}

// playing is the queue's track playing, -1 for none: the state's, when it
// is of the queue shown.
func (thisRef *queuePage) playing() int {
	state := thisRef.view.state
	if state.Queue.Version != thisRef.view.queue.Version {
		return -1
	}
	return state.Index
}

// paint draws a track of the queue: its number, a mark when it plays,
// "Artist - Title" and its length.
func (thisRef *queuePage) paint(painter *ui.Painter, index int, style ui.Style) {
	tracks := thisRef.view.queue.Tracks
	if index >= len(tracks) {
		return
	}
	track := tracks[index]
	if index == thisRef.playing() {
		painter.Text(0, 0, statusMark(thisRef.view.state.Status), style)
		if index != thisRef.list.Selected() {
			style = painter.Theme().Accent
			style.Background = painter.Theme().Text.Background
		}
	}
	right := ""
	if track.LengthMS > 0 {
		right = clock(track.LengthMS)
	}
	paintRow(painter, strconv.Itoa(index+1)+". "+trackText(track), right, style)
}

// remove takes the track chosen out of the queue.
func (thisRef *queuePage) remove() {
	index := thisRef.list.Selected()
	if index < 0 || index >= len(thisRef.view.queue.Tracks) {
		return
	}
	thisRef.view.call(func(ctx context.Context, player musicapi.Player) error {
		_, err := player.Remove(ctx, []int{index})
		return err
	})
}

// move moves the track chosen down (+1) or up (−1), the choice with it.
func (thisRef *queuePage) move(step int) {
	from := thisRef.list.Selected()
	to := from + step
	if from < 0 || to < 0 || to >= len(thisRef.view.queue.Tracks) {
		return
	}
	thisRef.list.SetSelected(to)
	thisRef.view.call(func(ctx context.Context, player musicapi.Player) error {
		_, err := player.Move(ctx, musicapi.Move{From: from, To: to})
		return err
	})
}

// clear empties the queue, once asked.
func (thisRef *queuePage) clear() {
	if len(thisRef.view.queue.Tracks) == 0 || thisRef.asking != nil {
		return
	}
	thisRef.asking = widgets.Confirm(thisRef.view.app, "Clear the queue", "Take every track out of the queue, and stop?", []string{"Clear", "Cancel"}, func(choice int) {
		thisRef.asking = nil
		if choice == 0 {
			thisRef.view.call(func(ctx context.Context, player musicapi.Player) error { _, err := player.Clear(ctx); return err })
		}
		thisRef.focus()
	})
}
