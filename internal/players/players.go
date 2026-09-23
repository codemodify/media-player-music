// Package players is the model the three player demos share.
//
// The three apps are **visual** demos. Nothing here decodes, mixes or plays
// anything: there is no audio stack, no video stack and no media dependency
// of any kind in this toolkit, and adding one would make the demos a
// different project. What they demonstrate is skins, shaped windows and the
// chrome a media player of its era was made of — so the model underneath is
// a clock that counts, a list of invented tracks, a band of numbers that
// look like an analyser, and the arithmetic of windows that stick together.
//
// The model half of it — this file, spectrum.go and rack.go — is plain data
// with no toolkit type anywhere in it, so the parts that are easy to get
// wrong are tested headless without opening a window at all: the transport's
// wrap at the end of a track, the seek bar's fraction, where a snapped
// window lands. ui.go and desk.go are the other half, the pieces of
// *interface* all three share — the transport glyphs, the analyser view, the
// clock that drives them and the driver that keeps real windows stuck
// together.
//
// The three apps are in their own packages beside this one, because a
// player's whole point is its look and no two of these three share any of
// it:
//
//	internal/players/minim     the compact one, 275x116, a pixel skin
//	internal/players/marquee   the big one, with a compact mode
//	internal/players/lantern   the shaped one, with a themed switch
package players

import (
	"fmt"
	"strings"
	"time"
)

// Track is one entry in a playlist. Every one of them is invented: no real
// recording, artist or label is named anywhere in this repository.
type Track struct {
	Title  string
	Artist string
	Album  string
	Length time.Duration
	// Kind is "audio" or "video". Only the demo that shows both cares,
	// but a playlist that cannot say which is which would be a strange
	// model for a player that plays both.
	Kind string
	// Rate and Channels are what a player of the era printed in its title
	// bar and its status line. They are invented too.
	Rate     int
	Channels int
}

// Video reports whether the track is one.
func (t Track) Video() bool { return t.Kind == "video" }

// Label is "Artist - Title", the one-line form every player of every era
// used for the thing it was playing.
func (t Track) Label() string {
	if t.Artist == "" {
		return t.Title
	}
	return t.Artist + " - " + t.Title
}

// Playlist is an ordered list of tracks with one of them current.
//
// The shuffle order is a permutation held beside the list rather than a
// shuffle of the list itself, so turning shuffle off puts the queue back in
// the order the user typed it in — which is what every player that got this
// right did, and what the ones that shuffled the list in place did not.
type Playlist struct {
	Tracks []Track
	// order is the play order: the identity permutation, or a shuffle.
	order []int
	// cur is an index into Tracks, not into order.
	cur int
}

// NewPlaylist wraps a slice of tracks. It takes the slice as given.
func NewPlaylist(tracks ...Track) *Playlist {
	p := &Playlist{Tracks: tracks}
	p.reorder(false)
	return p
}

// Len is how many tracks there are.
func (p *Playlist) Len() int {
	if p == nil {
		return 0
	}
	return len(p.Tracks)
}

// Index is the current track's position in the list, or -1 for an empty
// playlist.
func (p *Playlist) Index() int {
	if p.Len() == 0 {
		return -1
	}
	return p.cur
}

// Current is the track playing now. The zero Track for an empty playlist,
// so a caller that paints a title never has to check.
func (p *Playlist) Current() Track {
	if p.Len() == 0 {
		return Track{}
	}
	return p.Tracks[p.cur]
}

// At is the i'th track, or the zero Track when i is out of range.
func (p *Playlist) At(i int) Track {
	if p == nil || i < 0 || i >= len(p.Tracks) {
		return Track{}
	}
	return p.Tracks[i]
}

// Select makes i current. Out-of-range indices are ignored, so a list view
// that hands back its own selection can do so without guarding.
func (p *Playlist) Select(i int) bool {
	if p.Len() == 0 || i < 0 || i >= len(p.Tracks) || i == p.cur {
		return false
	}
	p.cur = i
	return true
}

// Find is the tracks whose title, artist or album contain text, without
// regard to case, as indices into Tracks. An empty query is nil, which
// means "everything" rather than "nothing" — a filter that emptied the list
// when it was cleared would be a filter with a bug in it.
func (p *Playlist) Find(text string) []int {
	text = strings.TrimSpace(strings.ToLower(text))
	if p == nil || text == "" {
		return nil
	}
	var out []int
	for i, t := range p.Tracks {
		hay := strings.ToLower(t.Title + " " + t.Artist + " " + t.Album)
		if strings.Contains(hay, text) {
			out = append(out, i)
		}
	}
	if out == nil {
		// A query that matches nothing is an empty list, not "everything".
		out = []int{}
	}
	return out
}

// Total is the playing time of every track in the list.
func (p *Playlist) Total() time.Duration {
	var d time.Duration
	for _, t := range p.Tracks {
		d += t.Length
	}
	return d
}

// Advance steps n places through the play order and reports whether the
// move wrapped past the end of the list. A caller with repeat off stops
// there; one with repeat on keeps going.
func (p *Playlist) Advance(n int) (wrapped bool) {
	if p.Len() == 0 || n == 0 {
		return false
	}
	at := p.orderIndex(p.cur) + n
	size := len(p.order)
	switch {
	case at >= size:
		wrapped = true
		at %= size
	case at < 0:
		wrapped = true
		at = ((at % size) + size) % size
	}
	p.cur = p.order[at]
	return wrapped
}

// Shuffle turns the shuffled play order on or off. The track playing now
// stays current either way: a shuffle that jumped to another track the
// moment it was switched on would be a different feature.
func (p *Playlist) Shuffle(on bool) {
	if p.Len() == 0 {
		return
	}
	p.reorder(on)
}

// orderIndex is where a track index sits in the play order.
func (p *Playlist) orderIndex(track int) int {
	for i, t := range p.order {
		if t == track {
			return i
		}
	}
	return 0
}

// reorder rebuilds the play order: the identity, or a deterministic
// shuffle of it.
//
// The shuffle is deterministic on purpose. A demo whose screenshot differs
// between two runs cannot be compared with the one before it, and a test
// that asserts where "next" goes needs an answer that does not come from a
// clock. It is a fixed-increment walk over a size coprime with the stride,
// which is the cheapest permutation that looks nothing like the order it
// came from.
func (p *Playlist) reorder(shuffled bool) {
	n := len(p.Tracks)
	p.order = make([]int, n)
	if !shuffled {
		for i := range p.order {
			p.order[i] = i
		}
		return
	}
	stride := n/2 + 1
	for stride > 1 && gcd(stride, n) != 1 {
		stride--
	}
	at := 0
	for i := 0; i < n; i++ {
		p.order[i] = at
		at = (at + stride) % n
	}
}

func gcd(a, b int) int {
	for b != 0 {
		a, b = b, a%b
	}
	return a
}

// Repeat is what happens at the end of the list.
type Repeat uint8

// The three answers every player has offered.
const (
	RepeatOff Repeat = iota
	RepeatAll
	RepeatOne
)

// String is what the control says.
func (r Repeat) String() string {
	switch r {
	case RepeatAll:
		return "Repeat all"
	case RepeatOne:
		return "Repeat one"
	}
	return "Repeat off"
}

// Transport is the head that moves along a track and the buttons around it.
//
// It plays nothing. Tick is handed however much time really passed, which
// is what makes the same model work under a window that repaints ten times
// a second and under a test that advances it a minute at a time.
type Transport struct {
	List   *Playlist
	Pos    time.Duration
	State  State
	Volume float32 // 0..1
	Pan    float32 // -1 (left) .. +1 (right)
	Repeat Repeat
	Random bool
	// Rate is how fast the head moves, so the demos can offer a speed
	// control that does something visible. 1 is real time.
	Rate float32
	// Changed runs after anything here moves, so a window can repaint
	// without polling. It is called on whatever goroutine moved the
	// transport, which in every app here is the UI one.
	Changed func()
	// premute is the volume mute took away, so unmuting puts it back.
	premute float32
}

// State is what the transport is doing.
type State uint8

// The three states a player is ever in.
const (
	Stopped State = iota
	Playing
	Paused
)

// String is what a status line says.
func (s State) String() string {
	switch s {
	case Playing:
		return "Playing"
	case Paused:
		return "Paused"
	}
	return "Stopped"
}

// NewTransport is a stopped transport at the head of a playlist.
func NewTransport(list *Playlist) *Transport {
	return &Transport{List: list, Volume: 0.72, Rate: 1}
}

// Track is the track under the head.
func (t *Transport) Track() Track {
	if t == nil || t.List == nil {
		return Track{}
	}
	return t.List.Current()
}

// Length is the current track's length, or zero when there is none.
func (t *Transport) Length() time.Duration { return t.Track().Length }

// Fraction is how far through the track the head is, 0..1. A track with no
// length reads zero rather than dividing by it.
func (t *Transport) Fraction() float32 {
	d := t.Length()
	if d <= 0 {
		return 0
	}
	f := float32(t.Pos) / float32(d)
	return clamp01(f)
}

// Remaining is what is left of the track, never negative.
func (t *Transport) Remaining() time.Duration {
	if d := t.Length() - t.Pos; d > 0 {
		return d
	}
	return 0
}

// Tick moves the head on by dt of wall-clock time and rolls into the next
// track at the end of this one. It reports whether anything moved, so a
// paused window can go back to sleep.
//
// The roll is a loop rather than a single step because dt is whatever the
// caller was away for: a test that advances an hour, or a window that was
// not repainted while another app had the screen, must not land in the
// middle of a track it skipped over.
func (t *Transport) Tick(dt time.Duration) bool {
	if t == nil || t.State != Playing || dt <= 0 {
		return false
	}
	// Real time at rate 1 is added as it came, not multiplied by a float
	// and rounded back: a float32 has 24 bits and a minute of nanoseconds
	// has 36, so scaling every tick by 1.0 loses a hundredth of a second
	// per minute and the clock drifts visibly against the wall.
	switch rate := t.Rate; {
	case rate == 1 || rate <= 0:
		t.Pos += dt
	default:
		t.Pos += time.Duration(float64(dt) * float64(rate))
	}
	for {
		d := t.Length()
		if d <= 0 || t.Pos < d {
			break
		}
		t.Pos -= d
		if !t.rollOver() {
			t.Pos = 0
			t.State = Stopped
			break
		}
	}
	t.changed()
	return true
}

// rollOver moves to whatever plays after the current track and reports
// whether there is one. RepeatOne stays put, which is the one case where
// the answer is yes and nothing moved.
func (t *Transport) rollOver() bool {
	if t.List == nil || t.List.Len() == 0 {
		return false
	}
	if t.Repeat == RepeatOne {
		return true
	}
	wrapped := t.List.Advance(1)
	return !wrapped || t.Repeat == RepeatAll
}

// Play starts the head moving.
func (t *Transport) Play() {
	if t.List == nil || t.List.Len() == 0 {
		return
	}
	t.State = Playing
	t.changed()
}

// Pause leaves the head where it is. Pausing a stopped transport starts it,
// which is what the one button every player has actually does.
func (t *Transport) Pause() {
	switch t.State {
	case Playing:
		t.State = Paused
	case Paused:
		t.State = Playing
	default:
		t.Play()
		return
	}
	t.changed()
}

// Stop takes the head back to the start of the track.
func (t *Transport) Stop() {
	t.State = Stopped
	t.Pos = 0
	t.changed()
}

// Next and Prev step through the play order, keeping whatever the
// transport was doing. Prev restarts the track when the head has moved
// more than a moment into it — the rule every player settled on, because
// the button is reached for twice as often to mean "from the top".
func (t *Transport) Next() { t.step(1) }

// Prev is Next's mirror, with the restart rule above.
func (t *Transport) Prev() {
	if t.Pos > prevRestart {
		t.Pos = 0
		t.changed()
		return
	}
	t.step(-1)
}

// prevRestart is how far into a track the previous-track button starts
// meaning "back to the top of this one".
const prevRestart = 3 * time.Second

func (t *Transport) step(n int) {
	if t.List == nil || t.List.Len() == 0 {
		return
	}
	wrapped := t.List.Advance(n)
	t.Pos = 0
	if wrapped && t.Repeat == RepeatOff {
		t.State = Stopped
	}
	t.changed()
}

// SelectTrack makes i the current track and starts it from the top.
func (t *Transport) SelectTrack(i int) {
	if t.List == nil || !t.List.Select(i) {
		return
	}
	t.Pos = 0
	t.changed()
}

// Seek puts the head at a fraction of the track: what a seek bar does when
// it is scrubbed. Out-of-range fractions are clamped rather than refused,
// because a drag that leaves the window is a fraction outside 0..1.
func (t *Transport) Seek(f float32) {
	d := t.Length()
	if d <= 0 {
		return
	}
	t.Pos = time.Duration(float64(clamp01(f)) * float64(d))
	t.changed()
}

// Skip moves the head by a fixed step, which is what the arrow keys do. It
// stops at either end of the track rather than rolling over: a keyboard
// nudge that changed track would be a surprise.
func (t *Transport) Skip(d time.Duration) {
	length := t.Length()
	if length <= 0 {
		return
	}
	t.Pos += d
	if t.Pos < 0 {
		t.Pos = 0
	}
	if t.Pos > length {
		t.Pos = length
	}
	t.changed()
}

// SetVolume and SetPan clamp and notify.
func (t *Transport) SetVolume(v float32) {
	v = clamp01(v)
	if v == t.Volume {
		return
	}
	t.Volume = v
	t.changed()
}

// SetPan is the balance control, -1 to +1.
func (t *Transport) SetPan(v float32) {
	if v < -1 {
		v = -1
	}
	if v > 1 {
		v = 1
	}
	if v == t.Pan {
		return
	}
	t.Pan = v
	t.changed()
}

// SetRandom turns shuffle on or off on the playlist under the transport.
func (t *Transport) SetRandom(on bool) {
	if t.Random == on {
		return
	}
	t.Random = on
	if t.List != nil {
		t.List.Shuffle(on)
	}
	t.changed()
}

// CycleRepeat steps off → all → one → off, which is the order the one
// button in a compact player has always stepped in.
func (t *Transport) CycleRepeat() {
	t.Repeat = (t.Repeat + 1) % 3
	t.changed()
}

func (t *Transport) changed() {
	if t != nil && t.Changed != nil {
		t.Changed()
	}
}

// ---- what a keyboard can ask for --------------------------------------------

// Command is one of the things a player's keyboard does. It is here rather
// than in the apps so that all three answer the same keys, and so that the
// test which walks the keyboard has one table to walk.
type Command uint8

// The commands, in the order the keys for them are usually reached for.
const (
	CmdNone Command = iota
	CmdPlayPause
	CmdStop
	CmdNext
	CmdPrev
	CmdSeekBack
	CmdSeekForward
	CmdVolumeUp
	CmdVolumeDown
	CmdMute
	CmdShuffle
	CmdRepeat
)

// SeekStep is how far the seek keys move the head, and VolumeStep how much
// the volume keys move the volume.
const (
	SeekStep   = 5 * time.Second
	VolumeStep = 0.05
)

// Do applies a command. An unknown one does nothing, so a caller need not
// check before dispatching.
//
// Mute is a memory rather than a flag: the volume it was at is kept on the
// transport so that unmuting puts it back, which is what every player does
// and what a bare boolean cannot.
func (t *Transport) Do(c Command) {
	if t == nil {
		return
	}
	switch c {
	case CmdPlayPause:
		t.Pause()
	case CmdStop:
		t.Stop()
	case CmdNext:
		t.Next()
	case CmdPrev:
		t.Prev()
	case CmdSeekBack:
		t.Skip(-SeekStep)
	case CmdSeekForward:
		t.Skip(SeekStep)
	case CmdVolumeUp:
		t.SetVolume(t.Volume + VolumeStep)
	case CmdVolumeDown:
		t.SetVolume(t.Volume - VolumeStep)
	case CmdMute:
		t.ToggleMute()
	case CmdShuffle:
		t.SetRandom(!t.Random)
	case CmdRepeat:
		t.CycleRepeat()
	}
}

// ToggleMute drops the volume to nothing and remembers what it was, or puts
// it back. A transport muted at nothing unmutes to a sensible reading rather
// than to nothing, so the button is never a no-op.
func (t *Transport) ToggleMute() {
	if t.Volume > 0 {
		t.premute = t.Volume
		t.SetVolume(0)
		return
	}
	v := t.premute
	if v <= 0 {
		v = 0.5
	}
	t.SetVolume(v)
}

// Muted reports whether the volume is at nothing.
func (t *Transport) Muted() bool { return t != nil && t.Volume <= 0 }

// ---- formatting -------------------------------------------------------------

// Clock is a duration as a player writes it: "3:47", and "1:02:11" once
// there is an hour of it. Negative durations read as zero, because a clock
// that can show "-0:01" is a clock with an arithmetic bug in it.
func Clock(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	s := int(d / time.Second)
	if h := s / 3600; h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, (s/60)%60, s%60)
	}
	return fmt.Sprintf("%d:%02d", s/60, s%60)
}

// Countdown is the same clock counting down, with the minus sign every
// player put in front of it.
func Countdown(d time.Duration) string { return "-" + Clock(d) }

// Decibels is an equaliser reading: "+4.5 dB", "0.0 dB".
func Decibels(v float32) string { return fmt.Sprintf("%+.1f dB", v) }

// Percent is a volume reading.
func Percent(v float32) string { return fmt.Sprintf("%d%%", int(clamp01(v)*100+0.5)) }

func clamp01(v float32) float32 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

// ---- the library ------------------------------------------------------------

// Library is the invented playlist all three demos start with.
//
// Every title, artist and album here was made up for this file. Naming a
// real recording would be the one thing in a demo about *skins* that could
// not be defended, and the lengths are chosen to be short enough that a
// minute of watching the clock crosses a track boundary.
func Library() []Track {
	return []Track{
		{Title: "Harbour Lights", Artist: "The Tessellators", Album: "Low Tide", Length: 3*time.Minute + 47*time.Second, Kind: "audio", Rate: 44, Channels: 2},
		{Title: "Paper Anemometer", Artist: "Ruth Alder", Album: "Field Notes", Length: 4*time.Minute + 12*time.Second, Kind: "audio", Rate: 44, Channels: 2},
		{Title: "Slow Ferry", Artist: "Kite & Tallow", Album: "Low Tide", Length: 2*time.Minute + 58*time.Second, Kind: "audio", Rate: 48, Channels: 2},
		{Title: "Nine Volt", Artist: "The Tessellators", Album: "Low Tide", Length: 3*time.Minute + 5*time.Second, Kind: "audio", Rate: 44, Channels: 2},
		{Title: "Marram", Artist: "Ostro", Album: "Dune Slack", Length: 5*time.Minute + 21*time.Second, Kind: "audio", Rate: 44, Channels: 2},
		{Title: "Winter Ferry (live)", Artist: "Kite & Tallow", Album: "Bootleg Tape", Length: 6*time.Minute + 2*time.Second, Kind: "audio", Rate: 48, Channels: 2},
		{Title: "Test Card", Artist: "uitoolkit", Album: "Demo Reel", Length: 1*time.Minute + 30*time.Second, Kind: "video", Rate: 48, Channels: 2},
		{Title: "Beacon", Artist: "Ruth Alder", Album: "Field Notes", Length: 3*time.Minute + 33*time.Second, Kind: "audio", Rate: 44, Channels: 2},
		{Title: "Copper Wire", Artist: "Ostro", Album: "Dune Slack", Length: 4*time.Minute + 44*time.Second, Kind: "audio", Rate: 44, Channels: 2},
		{Title: "Long Exposure", Artist: "The Tessellators", Album: "Demo Reel", Length: 7*time.Minute + 18*time.Second, Kind: "video", Rate: 48, Channels: 2},
	}
}

// NewLibrary is the invented library as a playlist.
func NewLibrary() *Playlist { return NewPlaylist(Library()...) }

// Disclaimer is the line every one of the three apps shows, in a status
// line or an About box, and Short is the same thing where there is no room
// for it — a compact player's footer is two hundred and sixty design pixels
// wide and this has to fit in it at 1x.
//
// They are here rather than in each app so they cannot drift, and so that it
// is impossible to ship one of the three without one.
const (
	Disclaimer = "A visual demo — nothing is decoded or played."
	Short      = "visual demo · nothing is played"
)
