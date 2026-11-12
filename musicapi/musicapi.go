// Package musicapi is the contract between media-player-musicd, which owns
// the music (the queue, the track playing and where it is, the sound, the
// equaliser and ReplayGain, what is kept between runs), and its renderers
// (the desktop, terminal and web UIs), which show it and pass on what the
// person does. Every renderer connected shows the same player: a track
// chosen, a seek, a volume moved in one shows in all of them.
//
// The daemon tells its renderers four states (app-spec's states: each
// renderer is given the newest at each connection, then each newer one,
// never an older one after a newer one):
//   - StateEvent, State: the transport, the levels, the equaliser,
//     ReplayGain, the track playing, the queue's size and version; at each
//     change;
//   - QueueEvent, Queue: the queue's tracks, at each change of the queue;
//   - NowEvent, Now: where the track playing is and the analyser's bands,
//     about twenty times a second while it plays;
//   - LibraryEvent, Library: the library in short (its folders, how much
//     music, whether it is being read), at each change. What it holds is
//     asked: Browse, Search.
//
// The stubs are generated, the Go one beside this file and the web UI's
// TypeScript one into cmd/media-player-music-wui/web/src:
//
//	go generate ./musicapi
package musicapi

import (
	"context"

	"goframework.io/app-spec/protocol"
)

//go:generate go run goframework.io/app-plugin/stub Player
//go:generate go run goframework.io/app-plugin/stub -typescript ../cmd/media-player-music-wui/web/src/musicapi.ts -client app-client-typescript -states Player=PlayerStates -constants -values Presets,Bands Player

// Version is the API's version.
const Version = "1.2.0"

// Name is the application's name, which its folders are named after
// (goframework.io/vfs-source-disk's UserPlaces).
const Name = "media-player-music"

// DaemonName is the daemon's program, which the renderers start when it
// does not answer (goframework R115's ensure).
const DaemonName = "media-player-musicd"

// SocketName is the daemon's Unix socket's name in the user's runtime
// folder, where the renderers on this machine dial it.
const SocketName = "media-player-musicd.sock"

// TokenFile is the token file's name beside the socket: the token the
// user's programs present when they reach the daemon over its WebSocket,
// which every user of the machine reaches (goframework.io/app-spec's
// transport.TokenFile: made when missing, only the user may read it).
const TokenFile = "media-player-musicd.token"

// The states the daemon keeps.
const (
	StateEvent   = "player.state"
	QueueEvent   = "player.queue"
	NowEvent     = "player.now"
	LibraryEvent = "player.library"
)

// PlayerStates are the states the daemon keeps.
var PlayerStates = protocol.States{StateEvent: State{}, QueueEvent: Queue{}, NowEvent: Now{}, LibraryEvent: Library{}}

// Player is offered by the daemon to its renderers.
type Player interface {
	// State is the player as it is now.
	State(ctx context.Context) (State, error)
	// Queue is the queue's tracks.
	Queue(ctx context.Context) (Queue, error)
	// Command does what a player's key does: one of the Command* names.
	Command(ctx context.Context, command string) (State, error)
	// Play plays the queue's track at index from its start.
	Play(ctx context.Context, index int) (State, error)
	// Seek moves the track playing to a time, in milliseconds from its
	// start (held within the track).
	Seek(ctx context.Context, positionMS int64) (State, error)
	// SetLevels sets the volume and the balance.
	SetLevels(ctx context.Context, levels Levels) (State, error)
	// SetMode sets what happens at a track's end and whether the queue
	// plays shuffled.
	SetMode(ctx context.Context, mode Mode) (State, error)
	// SetEqualizer sets the equaliser; Preset names one of Presets, whose
	// gains it takes when Gains is empty.
	SetEqualizer(ctx context.Context, equalizer Equalizer) (State, error)
	// SetReplayGain sets how tracks are levelled by their ReplayGain tags,
	// heard at once.
	SetReplayGain(ctx context.Context, gain ReplayGain) (State, error)
	// Add adds files, playlists and folders (each folder's music within,
	// in name order) to the queue's end, or in its place (Replace). A
	// playlist (M3U, M3U8, PLS, CUE) adds its tracks; a CUE sheet's are
	// parts of a file. A file of a format the player does not play, or a
	// path that is not there, is an error and nothing changes; a folder's
	// such files, and a playlist's missing files and streams, are left out
	// and said in Added.Skipped. The queue starts playing when it was
	// replaced or empty.
	Add(ctx context.Context, add Add) (Added, error)
	// SavePlaylist writes the queue as a playlist file at path: M3U8
	// (".m3u", ".m3u8") or PLS (".pls"), by its extension, with absolute
	// paths.
	SavePlaylist(ctx context.Context, path string) (State, error)
	// Remove takes the tracks at indexes out of the queue.
	Remove(ctx context.Context, indexes []int) (State, error)
	// Move moves the track at From to To, the others closing up.
	Move(ctx context.Context, move Move) (State, error)
	// Clear empties the queue and stops.
	Clear(ctx context.Context) (State, error)

	// Library is the library in short.
	Library(ctx context.Context) (Library, error)
	// AddFolder adds a folder to the library, read in the background and
	// watched: an error for a path that is no folder.
	AddFolder(ctx context.Context, path string) (Library, error)
	// RemoveFolder takes a folder and its music out of the library.
	RemoveFolder(ctx context.Context, path string) (Library, error)
	// Rescan reads every folder of the library again, in the background
	// (a file's tags only when it changed).
	Rescan(ctx context.Context) (Library, error)
	// Browse lists the library: its artists, albums, genres or years, or
	// its tracks, among those of an artist, an album, a genre and a year,
	// those given.
	Browse(ctx context.Context, browse Browse) (Browsed, error)
	// Search finds what was typed in the library: words and "phrases" in
	// the tracks' titles, artists, albums and genres, without case or
	// accents; artist:, album:, genre:, title: and year: keep to a field.
	Search(ctx context.Context, search Search) (Found, error)
}

// The commands, as a player's keys give them.
const (
	CommandPlay        = "play"         // play, from where it is
	CommandPlayPause   = "play-pause"   // pause what plays, else play
	CommandStop        = "stop"         // stop, back to the track's start
	CommandNext        = "next"         // the next track in the play order
	CommandPrevious    = "previous"     // the track's start, or the one before when near it
	CommandSeekBack    = "seek-back"    // five seconds back
	CommandSeekForward = "seek-forward" // five seconds on
	CommandVolumeUp    = "volume-up"    // the volume up a step
	CommandVolumeDown  = "volume-down"  // and down
	CommandMute        = "mute"         // silent, or back to the volume it had
	CommandShuffle     = "shuffle"      // shuffle on or off
	CommandRepeat      = "repeat"       // off, all, one, off
)

// The transport's statuses.
const (
	StatusStopped = "stopped"
	StatusPlaying = "playing"
	StatusPaused  = "paused"
)

// The repeat modes.
const (
	RepeatOff = "off"
	RepeatAll = "all"
	RepeatOne = "one"
)

// The ReplayGain modes.
const (
	ReplayGainOff   = "off"
	ReplayGainTrack = "track"
	ReplayGainAlbum = "album"
)

// The preamp's range for ReplayGain, in dB; the equaliser's bands and
// preamp go EqualizerRange either way.
const (
	MinReplayGainPreamp = -15
	MaxReplayGainPreamp = 15
	EqualizerRange      = 12
)

// Track is a track of the queue.
type Track struct {
	// Path is the file.
	Path string `json:"path"`
	// Title is the tags' title, else the file's name without its
	// extension.
	Title  string `json:"title"`
	Artist string `json:"artist,omitempty"`
	Album  string `json:"album,omitempty"`
	// LengthMS is how long it plays, in milliseconds; 0 when not known
	// before it plays.
	LengthMS int64 `json:"lengthMs,omitempty"`
	// Rate (Hz), Channels and Bitrate (bits a second) are what the decoder
	// reported; 0 until the track has played.
	Rate     int `json:"rate,omitempty"`
	Channels int `json:"channels,omitempty"`
	Bitrate  int `json:"bitrate,omitempty"`
	// AlbumArtist, Genre, Year (0 when none), Number and Disc (the
	// track's and the disc's, 0 when none) are what the tags say.
	AlbumArtist string `json:"albumArtist,omitempty"`
	Genre       string `json:"genre,omitempty"`
	Year        int    `json:"year,omitempty"`
	Number      int    `json:"number,omitempty"`
	Disc        int    `json:"disc,omitempty"`
	// StartMS and EndMS are where a CUE sheet's track is in its file
	// (EndMS 0: to the file's end); LengthMS and the positions are the
	// track's own.
	StartMS int64 `json:"startMs,omitempty"`
	EndMS   int64 `json:"endMs,omitempty"`
}

// Queue is the queue's tracks, in the queue's order (shuffling plays
// them in another order; the queue keeps its own). What a track's decoder
// tells as it plays (its tags, length and format) comes in State.Track at
// once, and in the queue from its next change.
type Queue struct {
	// Version changes at each change of the queue; State.Queue.Version
	// names the queue a State's Index is in.
	Version uint64  `json:"version"`
	Tracks  []Track `json:"tracks"`
}

// QueueInfo is the queue in short.
type QueueInfo struct {
	Version  uint64 `json:"version"`
	Length   int    `json:"length"`
	LengthMS int64  `json:"lengthMs"`
}

// State is what a renderer shows.
type State struct {
	// Seq is the state's number, from 1, higher for a newer one: a
	// renderer given a state by a call's answer and by the states shows
	// the newer.
	Seq uint64 `json:"seq"`
	// Status is StatusStopped, StatusPlaying or StatusPaused.
	Status string `json:"status"`
	// Index is the track playing, or to play, in the queue; -1 for an
	// empty queue.
	Index int `json:"index"`
	// Track is that track, with what its decoder told of it.
	Track Track `json:"track"`
	// PositionMS is where it is, in milliseconds, as the state was made;
	// Now tells it as it moves.
	PositionMS int64 `json:"positionMs"`
	// Volume is from 0 to 1; Balance from -1 (left) to 1 (right).
	Volume  float64 `json:"volume"`
	Balance float64 `json:"balance"`
	// Repeat is RepeatOff, RepeatAll or RepeatOne.
	Repeat  string `json:"repeat"`
	Shuffle bool   `json:"shuffle"`
	// Equalizer and ReplayGain are the sound's settings.
	Equalizer  Equalizer  `json:"equalizer"`
	ReplayGain ReplayGain `json:"replayGain"`
	// Queue is the queue in short.
	Queue QueueInfo `json:"queue"`
	// Error is why the last thing asked failed or the track stopped ("a.wma:
	// WMA files are not supported…"); "" when none.
	Error string `json:"error,omitempty"`
	// Formats names the formats the player plays, for a settings page.
	Formats string `json:"formats"`
	// Clients is how many renderers show this player now.
	Clients int `json:"clients"`
}

// Now is where the track playing is, as it moves.
type Now struct {
	// Seq is the Seq of the State it follows: a renderer ignores a Now of
	// a State older than the one it shows (one from before a seek).
	Seq uint64 `json:"seq"`
	// Index is the track it is of.
	Index      int    `json:"index"`
	Status     string `json:"status"`
	PositionMS int64  `json:"positionMs"`
	// LengthMS is the track's length, as the decoder knows it.
	LengthMS int64 `json:"lengthMs"`
	// Bands are the analyser's 48 bands, each from 0 to 1, lowest
	// frequency first; empty when nothing is heard.
	Bands []float64 `json:"bands,omitempty"`
}

// Levels are the volume and the balance.
type Levels struct {
	Volume  float64 `json:"volume"`
	Balance float64 `json:"balance"`
}

// Mode is what happens at a track's end, and the play order.
type Mode struct {
	Repeat  string `json:"repeat"`
	Shuffle bool   `json:"shuffle"`
}

// Equalizer is the ten-band equaliser.
type Equalizer struct {
	On bool `json:"on"`
	// Auto applies a preset at each track.
	Auto bool `json:"auto,omitempty"`
	// Preamp and Gains are in dB, from -EqualizerRange to EqualizerRange;
	// Gains has a value for each of Bands.
	Preamp float64   `json:"preamp"`
	Gains  []float64 `json:"gains"`
	// Preset names the preset the gains are, "Custom" once a band moved.
	Preset string `json:"preset,omitempty"`
}

// Presets are the equaliser's presets' names.
var Presets = []string{"Flat", "Full bass", "Full treble", "Vocal", "Late night", "Small speakers"}

// Bands are the equaliser's bands' centre frequencies, as labelled.
var Bands = []string{"31", "62", "125", "250", "500", "1k", "2k", "4k", "8k", "16k"}

// ReplayGain is how tracks are levelled by their ReplayGain tags.
type ReplayGain struct {
	// Mode is ReplayGainOff, ReplayGainTrack or ReplayGainAlbum.
	Mode string `json:"mode"`
	// Preamp is added to the tags' gain, in dB, from MinReplayGainPreamp
	// to MaxReplayGainPreamp.
	Preamp float64 `json:"preamp"`
}

// Add is what Add adds.
type Add struct {
	Paths   []string `json:"paths"`
	Replace bool     `json:"replace,omitempty"`
}

// Added is what Add did.
type Added struct {
	// Tracks is how many tracks were added.
	Tracks int `json:"tracks"`
	// Skipped says what was left out, in words ("left out 2 files in
	// formats the player does not play (WMA)"); "" when nothing was.
	Skipped string `json:"skipped,omitempty"`
	State   State  `json:"state"`
}

// Move moves a track of the queue.
type Move struct {
	From int `json:"from"`
	To   int `json:"to"`
}

// Library is the library in short.
type Library struct {
	// Version changes at each change of the library.
	Version uint64 `json:"version"`
	// Folders are the folders the library holds the music of.
	Folders []string `json:"folders"`
	// Tracks, Artists, Albums and Genres are how many there are, and
	// LengthMS how long the music plays.
	Tracks   int   `json:"tracks"`
	Artists  int   `json:"artists"`
	Albums   int   `json:"albums"`
	Genres   int   `json:"genres"`
	LengthMS int64 `json:"lengthMs"`
	// Scanning says the folders are being read; Scanned is how many files
	// were read so far.
	Scanning bool `json:"scanning,omitempty"`
	Scanned  int  `json:"scanned,omitempty"`
	// Error is why a folder could not be read or watched; "" when none.
	Error string `json:"error,omitempty"`
}

// What Browse lists.
const (
	BrowseArtists = "artists"
	BrowseAlbums  = "albums"
	BrowseGenres  = "genres"
	BrowseYears   = "years"
	BrowseTracks  = "tracks"
)

// Browse is what Browse lists: By's groups (BrowseArtists, BrowseAlbums,
// BrowseGenres, BrowseYears) or the tracks (BrowseTracks), among the
// tracks of Artist (an album's artist, or a track's own), Album, Genre and
// Year, those given ("" and 0: any). Offset and Limit (0: all) page the
// list.
type Browse struct {
	By     string `json:"by"`
	Artist string `json:"artist,omitempty"`
	Album  string `json:"album,omitempty"`
	Genre  string `json:"genre,omitempty"`
	Year   int    `json:"year,omitempty"`
	// Unknown names the fields that must be empty: "artist", "album",
	// "genre", "year", for the group of the tracks whose tags do not say
	// (a Group whose Name is "").
	Unknown []string `json:"unknown,omitempty"`
	Offset  int      `json:"offset,omitempty"`
	Limit   int      `json:"limit,omitempty"`
}

// Browsed is what Browse found: Groups or Tracks, in the library's order
// (names as a person sorts them, "The" left out, the unknown last; years
// the newest first; tracks by artist, album, disc and number), and how
// many there are in all.
type Browsed struct {
	Groups []Group `json:"groups,omitempty"`
	Tracks []Track `json:"tracks,omitempty"`
	Total  int     `json:"total"`
}

// Group is an artist, an album, a genre or a year of the library.
type Group struct {
	// Name is the artist's, the album's, the genre's or the year's; ""
	// for the tracks whose tags do not say.
	Name string `json:"name"`
	// Artist is an album's artist.
	Artist string `json:"artist,omitempty"`
	// Year is an album's (its latest track's) or the year's.
	Year     int   `json:"year,omitempty"`
	Tracks   int   `json:"tracks"`
	LengthMS int64 `json:"lengthMs"`
}

// Search is what Search finds: Text as typed, at most Limit of each kind
// (50 when 0).
type Search struct {
	Text  string `json:"text"`
	Limit int    `json:"limit,omitempty"`
}

// Found is what Search found: the artists and albums whose names hold
// every word, and the tracks that do; More says there were more than the
// limit.
type Found struct {
	Artists []Group `json:"artists,omitempty"`
	Albums  []Group `json:"albums,omitempty"`
	Tracks  []Track `json:"tracks,omitempty"`
	More    bool    `json:"more,omitempty"`
}
