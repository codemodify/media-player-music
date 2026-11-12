package tui

import (
	"context"
	"math"
	"slices"
	"strconv"
	"strings"

	"github.com/codemodify/media-player-music/musicapi"
	ui "goframework.io/terminal-ui"
	widgets "goframework.io/terminal-ui-widgets"
)

// setting is a row of the settings: what it is, its value, and what Left
// and Right (change), Enter (activate) and Delete (remove) do there.
type setting struct {
	label, value string
	heading      bool
	change       func(step int)
	activate     func()
	remove       func()
}

// settingsPage is the settings' view: the equaliser, ReplayGain and the
// library's folders, as the daemon has them.
type settingsPage struct {
	view   *View
	box    *ui.Flex
	list   *widgets.List
	rows   []setting
	asking *widgets.Dialog
}

func newSettingsPage(view *View) *settingsPage {
	thisRef := &settingsPage{view: view}
	thisRef.list = widgets.NewList(0, thisRef.paint)
	thisRef.list.SetHeight(ui.Fill(1))
	thisRef.list.OnActivate(func(index int) { thisRef.activate(index) })
	thisRef.list.AddCommand(ui.Command{Name: "settings.less", Title: "Less", Keys: []string{"left", "h"}, Hidden: true, Run: func() { thisRef.change(-1) }})
	thisRef.list.AddCommand(ui.Command{Name: "settings.more", Title: "More", Keys: []string{"right", "l"}, Hidden: true, Run: func() { thisRef.change(+1) }})
	thisRef.list.AddCommand(ui.Command{Name: "settings.activate", Title: "Choose", Keys: []string{"space"}, Hidden: true, Run: func() { thisRef.activate(thisRef.list.Selected()) }})
	thisRef.list.AddCommand(ui.Command{Name: "settings.remove", Title: "Remove", Keys: []string{"D", "delete"}, Run: thisRef.remove})
	heading := widgets.NewLabel("Settings")
	heading.SetTone(widgets.ToneAccent)
	heading.SetPadding(ui.Symmetric(0, 1))
	thisRef.box = ui.Column(heading, thisRef.list)
	return thisRef
}

func (thisRef *settingsPage) root() ui.Widget { return thisRef.box }
func (thisRef *settingsPage) focus()          { thisRef.list.Focus() }

// show makes the rows again from the state and the library.
func (thisRef *settingsPage) show() {
	view := thisRef.view
	thisRef.rows = nil
	if view.stated {
		thisRef.rows = append(thisRef.rows, thisRef.equalizerRows()...)
		thisRef.rows = append(thisRef.rows, thisRef.replayGainRows()...)
	}
	if view.libraryKnown {
		thisRef.rows = append(thisRef.rows, thisRef.libraryRows()...)
	}
	if view.stated && view.state.Formats != "" {
		thisRef.rows = append(thisRef.rows, setting{label: "Plays", value: view.state.Formats})
	}
	thisRef.list.SetCount(len(thisRef.rows))
	thisRef.list.Invalidate()
}

// decibels writes a gain: "+3 dB", "-1.5 dB".
func decibels(gain float64) string {
	text := strconv.FormatFloat(gain, 'f', -1, 64)
	if gain >= 0 {
		text = "+" + text
	}
	return text + " dB"
}

// equalizerRows are the equaliser's: on or off, its preset, its preamp
// and its ten bands.
func (thisRef *settingsPage) equalizerRows() []setting {
	equalizer := thisRef.view.state.Equalizer
	on := "off"
	if equalizer.On {
		on = "on"
	}
	preset := equalizer.Preset
	if preset == "" {
		preset = "Custom"
	}
	rows := []setting{
		{label: "Equaliser", value: on, activate: func() {
			thisRef.setEqualizer(func(equalizer *musicapi.Equalizer) { equalizer.On = !equalizer.On })
		}, change: func(int) {
			thisRef.setEqualizer(func(equalizer *musicapi.Equalizer) { equalizer.On = !equalizer.On })
		}},
		{label: "  Preset", value: preset, change: thisRef.stepPreset, activate: func() { thisRef.stepPreset(+1) }},
		{label: "  Preamp", value: decibels(equalizer.Preamp), change: func(step int) {
			thisRef.setEqualizer(func(equalizer *musicapi.Equalizer) {
				equalizer.Preamp = math.Round(min(max(equalizer.Preamp+float64(step), -musicapi.EqualizerRange), musicapi.EqualizerRange))
			})
		}},
	}
	for band, name := range musicapi.Bands {
		gain := 0.0
		if band < len(equalizer.Gains) {
			gain = equalizer.Gains[band]
		}
		rows = append(rows, setting{label: "  " + name + " Hz", value: decibels(gain), change: func(step int) {
			thisRef.setEqualizer(func(equalizer *musicapi.Equalizer) {
				for len(equalizer.Gains) < len(musicapi.Bands) {
					equalizer.Gains = append(equalizer.Gains, 0)
				}
				equalizer.Gains[band] = math.Round(min(max(equalizer.Gains[band]+float64(step), -musicapi.EqualizerRange), musicapi.EqualizerRange))
				equalizer.Preset = "Custom"
			})
		}})
	}
	return rows
}

// stepPreset takes the next of the presets, or the one before.
func (thisRef *settingsPage) stepPreset(step int) {
	at := slices.Index(musicapi.Presets, thisRef.view.state.Equalizer.Preset)
	if at < 0 && step < 0 {
		at = 0
	}
	next := musicapi.Presets[((at+step)%len(musicapi.Presets)+len(musicapi.Presets))%len(musicapi.Presets)]
	thisRef.setEqualizer(func(equalizer *musicapi.Equalizer) {
		equalizer.Preset, equalizer.Gains = next, nil // the preset's gains, the daemon's
	})
}

// setEqualizer changes the equaliser as the state has it, and sends it;
// the state shown has it at once, so that keys pressed fast step from
// one another, until the daemon's comes.
func (thisRef *settingsPage) setEqualizer(change func(*musicapi.Equalizer)) {
	equalizer := thisRef.view.state.Equalizer
	equalizer.Gains = slices.Clone(equalizer.Gains)
	change(&equalizer)
	if equalizer.Gains != nil {
		thisRef.view.state.Equalizer = equalizer
	} else {
		thisRef.view.state.Equalizer.Preset = equalizer.Preset
	}
	thisRef.show()
	thisRef.view.call(func(ctx context.Context, player musicapi.Player) error {
		_, err := player.SetEqualizer(ctx, equalizer)
		return err
	})
}

// replayGainModes are ReplayGain's modes, in Enter's order.
var replayGainModes = []string{musicapi.ReplayGainOff, musicapi.ReplayGainTrack, musicapi.ReplayGainAlbum}

// replayGainRows are ReplayGain's: its mode and its preamp.
func (thisRef *settingsPage) replayGainRows() []setting {
	gain := thisRef.view.state.ReplayGain
	mode := gain.Mode
	if mode == "" {
		mode = musicapi.ReplayGainTrack // the daemon's, for none
	}
	stepMode := func(step int) {
		thisRef.setReplayGain(func(gain *musicapi.ReplayGain) {
			at := max(slices.Index(replayGainModes, gain.Mode), 0)
			gain.Mode = replayGainModes[((at+step)%len(replayGainModes)+len(replayGainModes))%len(replayGainModes)]
		})
	}
	return []setting{
		{label: "ReplayGain", value: mode, change: stepMode, activate: func() { stepMode(+1) }},
		{label: "  Preamp", value: decibels(gain.Preamp), change: func(step int) {
			thisRef.setReplayGain(func(gain *musicapi.ReplayGain) {
				gain.Preamp = math.Round(min(max(gain.Preamp+float64(step), musicapi.MinReplayGainPreamp), musicapi.MaxReplayGainPreamp))
			})
		}},
	}
}

// setReplayGain changes ReplayGain as the state has it, and sends it.
func (thisRef *settingsPage) setReplayGain(change func(*musicapi.ReplayGain)) {
	gain := thisRef.view.state.ReplayGain
	if gain.Mode == "" {
		gain.Mode = musicapi.ReplayGainTrack
	}
	change(&gain)
	thisRef.view.state.ReplayGain = gain
	thisRef.show()
	thisRef.view.call(func(ctx context.Context, player musicapi.Player) error {
		_, err := player.SetReplayGain(ctx, gain)
		return err
	})
}

// libraryRows are the library's: how much music, its folders (Delete
// takes one out), a folder to add and a rescan.
func (thisRef *settingsPage) libraryRows() []setting {
	library := thisRef.view.library
	summary := itoa(library.Tracks) + " tracks, " + itoa(library.Artists) + " artists, " + itoa(library.Albums) + " albums, " + clock(library.LengthMS)
	if library.Scanning {
		summary = "reading… " + itoa(library.Scanned) + " files"
	}
	rows := []setting{{label: "Library", value: summary, heading: true}}
	for _, folder := range library.Folders {
		rows = append(rows, setting{label: "  " + folder, remove: func() { thisRef.askRemove(folder) }})
	}
	rows = append(rows,
		setting{label: "  Add a folder…", activate: func() { thisRef.view.openPrompt("lib-add ") }},
		setting{label: "  Read the folders again", activate: func() {
			thisRef.view.call(func(ctx context.Context, player musicapi.Player) error { _, err := player.Rescan(ctx); return err })
		}},
	)
	return rows
}

// askRemove takes a folder out of the library, once asked.
func (thisRef *settingsPage) askRemove(folder string) {
	if thisRef.asking != nil {
		return
	}
	thisRef.asking = widgets.Confirm(thisRef.view.app, "Remove a folder", "Take "+folder+" and its music out of the library? Its files stay where they are.", []string{"Remove", "Cancel"}, func(choice int) {
		thisRef.asking = nil
		if choice == 0 {
			thisRef.view.call(func(ctx context.Context, player musicapi.Player) error {
				_, err := player.RemoveFolder(ctx, folder)
				return err
			})
		}
		thisRef.focus()
	})
}

func (thisRef *settingsPage) row(index int) *setting {
	if index < 0 || index >= len(thisRef.rows) {
		return nil
	}
	return &thisRef.rows[index]
}

func (thisRef *settingsPage) change(step int) {
	if row := thisRef.row(thisRef.list.Selected()); row != nil && row.change != nil {
		row.change(step)
	}
}

func (thisRef *settingsPage) activate(index int) {
	if row := thisRef.row(index); row != nil && row.activate != nil {
		row.activate()
	}
}

func (thisRef *settingsPage) remove() {
	if row := thisRef.row(thisRef.list.Selected()); row != nil && row.remove != nil {
		row.remove()
	}
}

// paint draws a setting: its name, and its value with what changes it.
func (thisRef *settingsPage) paint(painter *ui.Painter, index int, style ui.Style) {
	row := thisRef.row(index)
	if row == nil {
		return
	}
	if row.heading || strings.TrimSpace(row.label) == row.label && row.value != "" && row.change == nil && row.activate == nil {
		accent := painter.Theme().Accent
		accent.Background = style.Background
		paintRow(painter, row.label+": "+row.value, "", accent)
		return
	}
	value := row.value
	if row.change != nil && value != "" {
		value = "‹ " + value + " ›"
	}
	if value == "" {
		paintRow(painter, row.label, "", style)
		return
	}
	// the name, then the value at a column of its own where there is room
	width := painter.Size().Width
	column := min(18, max(width-painter.Width(value)-2, 0))
	label := row.label
	if painter.Width(label) > column-1 {
		label = painter.Model().Truncate(label, max(column-1, 0), "…")
	}
	painter.Text(1, 0, label, style)
	paintRow(painter.Area(ui.Rect{X: column, Width: max(width-column, 0), Height: 1}), value, "", style)
}
