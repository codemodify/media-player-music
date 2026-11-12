// Package players holds the pieces of interface the default UI and its
// optional skins share: the model they show (internal/model's, by the
// names below), the host that gives them the application's actions, the
// controls and the keyboard commands. The shell keeps the model in step
// with media-player-musicd, which plays; screenshot mode uses a simulated
// clock. Window composition and appearance switching belong to
// internal/shell.
package players

import (
	"time"

	"github.com/codemodify/media-player-music/internal/model"
)

// Version is media-player-music's: what -version prints and an About
// shows. A release may set it with -ldflags
// "-X github.com/codemodify/media-player-music/internal/players.Version=...".
var Version = "0.0.1"

// The model's names, as the faces have always used them.
type (
	Track     = model.Track
	Playlist  = model.Playlist
	Transport = model.Transport
	Repeat    = model.Repeat
	State     = model.State
	Command   = model.Command
	Equalizer = model.Equalizer
	EqPreset  = model.EqPreset
)

const (
	RepeatOff = model.RepeatOff
	RepeatAll = model.RepeatAll
	RepeatOne = model.RepeatOne

	Stopped = model.Stopped
	Playing = model.Playing
	Paused  = model.Paused

	CmdNone        = model.CmdNone
	CmdPlayPause   = model.CmdPlayPause
	CmdStop        = model.CmdStop
	CmdNext        = model.CmdNext
	CmdPrev        = model.CmdPrev
	CmdSeekBack    = model.CmdSeekBack
	CmdSeekForward = model.CmdSeekForward
	CmdVolumeUp    = model.CmdVolumeUp
	CmdVolumeDown  = model.CmdVolumeDown
	CmdMute        = model.CmdMute
	CmdShuffle     = model.CmdShuffle
	CmdRepeat      = model.CmdRepeat
	CmdPlay        = model.CmdPlay

	SeekStep   = model.SeekStep
	VolumeStep = model.VolumeStep
	EqRange    = model.EqRange
	Disclaimer = model.Disclaimer
)

// EqBands are the ten bands' labels.
var EqBands = model.EqBands

func NewPlaylist(tracks ...Track) *Playlist  { return model.NewPlaylist(tracks...) }
func NewTransport(list *Playlist) *Transport { return model.NewTransport(list) }
func NewEqualizer() *Equalizer               { return model.NewEqualizer() }
func EqPresets() []EqPreset                  { return model.EqPresets() }
func Library() []Track                       { return model.Library() }
func NewLibrary() *Playlist                  { return model.NewLibrary() }
func Clock(d time.Duration) string           { return model.Clock(d) }
func Countdown(d time.Duration) string       { return model.Countdown(d) }
func Decibels(v float32) string              { return model.Decibels(v) }
func Percent(v float32) string               { return model.Percent(v) }
func clamp01(v float32) float32              { return min(max(v, 0), 1) }
