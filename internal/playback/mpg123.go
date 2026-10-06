package playback

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// NewMPG123 controls an installed mpg123 through its documented -R protocol.
// Its native 32-band MPEG synthesis equalizer cannot reproduce this player's
// ten ISO bands, and the remote protocol exports no PCM or spectrum samples.
func NewMPG123(opts EngineOptions) (Engine, error) {
	return newProcessEngine(opts, mpg123Dialect{}, Capabilities{Seek: true, Formats: "MPEG audio (MP1, MP2, MP3)"})
}

type mpg123Dialect struct{}

type mpg123FormatError struct{ extension string }

func (e mpg123FormatError) Error() string {
	return fmt.Sprintf("mpg123 supports MP3/MPEG only; use mpv or VLC for %s.", e.extension)
}
func (mpg123FormatError) Unwrap() error { return ErrUnsupported }

func (mpg123Dialect) name() string { return "mpg123" }
func (mpg123Dialect) args(extra []string) []string {
	return append(append([]string(nil), extra...), "--no-control", "--utf8", "--remote-err", "-R")
}
func (mpg123Dialect) ready(line string) bool { return strings.HasPrefix(line, "@R MPG123") }
func (mpg123Dialect) quit() string           { return "QUIT" }

func (mpg123Dialect) validateLoad(path string) error {
	extension := strings.ToLower(filepath.Ext(path))
	switch extension {
	case ".mp4", ".m4a", ".m4b", ".m4p", ".m4v", ".aac", ".flac", ".wav", ".wave", ".ogg", ".oga", ".opus", ".aif", ".aiff", ".aifc", ".ape", ".alac", ".wma", ".wmv", ".webm", ".mkv", ".avi", ".mov", ".mpg", ".vob", ".ts", ".m2ts", ".ac3", ".dts", ".amr", ".caf":
		return mpg123FormatError{extension: extension}
	}
	// Extensionless files and unfamiliar suffixes still go to the MPEG
	// decoder, so valid audio is not rejected just because of its name.
	return nil
}

func (mpg123Dialect) load(s *remoteSession) []string {
	// The argument is the entire remainder of the line, not a quoted token.
	// Newlines/NUL are rejected before it reaches this protocol.
	remotePoll(s)
	return []string{"LOADPAUSED " + s.path, "FORMAT", "SAMPLE"}
}
func (mpg123Dialect) pause(bool) string              { return "PAUSE" }
func (mpg123Dialect) seek(pos time.Duration) string  { return fmt.Sprintf("JUMP %.6fs", pos.Seconds()) }
func (mpg123Dialect) volume(v float32) string        { return fmt.Sprintf("VOLUME %.6f", v*100) }
func (mpg123Dialect) poll(s *remoteSession) []string { remotePoll(s); return []string{"SAMPLE"} }

func (d mpg123Dialect) parse(s *remoteSession, line string, b *updateBroker) []string {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return nil
	}
	switch fields[0] {
	case "@FORMAT":
		if len(fields) >= 3 {
			rate, _ := strconv.Atoi(fields[1])
			channels, _ := strconv.Atoi(fields[2])
			if rate > 0 && channels > 0 {
				s.info.SampleRate, s.info.Channels = rate, channels
				remoteUpdate(s, b, UpdateAudioFormat)
				return remoteLoaded(s, b, d)
			}
		}
	case "@S":
		if len(fields) >= 12 {
			s.info.SampleRate, _ = strconv.Atoi(fields[3])
			s.info.Channels, _ = strconv.Atoi(fields[7])
			if s.info.Channels == 0 {
				s.info.Channels = 1
			}
			bitrate, _ := strconv.Atoi(fields[11])
			s.info.Bitrate = bitrate * 1000
			remoteUpdate(s, b, UpdateAudioFormat)
		}
	case "@SAMPLE":
		if len(fields) >= 3 && s.info.SampleRate > 0 {
			position, e1 := strconv.ParseInt(fields[1], 10, 64)
			total, e2 := strconv.ParseInt(fields[2], 10, 64)
			if e1 != nil || e2 != nil || position < 0 {
				break
			}
			s.position = time.Duration(float64(position) / float64(s.info.SampleRate) * float64(time.Second))
			if total > 0 {
				duration := time.Duration(float64(total) / float64(s.info.SampleRate) * float64(time.Second))
				if duration != s.duration {
					s.duration = duration
					remoteUpdate(s, b, UpdateDuration)
				}
			}
			return remoteClock(s, b, d)
		}
	case "@F":
		if len(fields) >= 5 && !s.wantPaused && len(s.seeks) == 0 {
			pos, e1 := strconv.ParseFloat(fields[3], 64)
			left, e2 := strconv.ParseFloat(fields[4], 64)
			if e1 != nil || e2 != nil || pos < 0 || left < 0 {
				break
			}
			s.position = secondsDuration(pos)
			// SAMPLE supplies the gapless sample count; frame progress is only
			// a duration fallback for streams without a known total.
			if s.duration == 0 && pos+left > 0 {
				s.duration = secondsDuration(pos + left)
				remoteUpdate(s, b, UpdateDuration)
			}
			remoteUpdate(s, b, UpdatePosition)
		}
	case "@J":
		for i := range s.seeks {
			if !s.seeks[i].acknowledged {
				s.seeks[i].acknowledged = true
				break
			}
		}
	case "@P":
		if len(fields) >= 2 && fields[1] == "3" && s.loaded && !s.ended && !s.failed {
			s.ended = true
			remoteUpdate(s, b, UpdateEnded)
		}
	case "@I":
		value := strings.TrimPrefix(line, "@I ")
		changed := false
		if strings.HasPrefix(value, "ID3v2.") {
			if key, text, ok := strings.Cut(strings.TrimPrefix(value, "ID3v2."), ":"); ok {
				text = strings.TrimSpace(text)
				switch strings.ToLower(key) {
				case "title":
					s.info.Title = text
					changed = true
				case "artist", "author":
					s.info.Artist = text
					changed = true
				case "album":
					s.info.Album = text
					changed = true
				}
			}
		} else if strings.HasPrefix(value, "ID3:") {
			text := strings.TrimPrefix(value, "ID3:")
			if len(text) >= 90 {
				if s.info.Title == "" {
					s.info.Title = strings.TrimSpace(text[:30])
				}
				if s.info.Artist == "" {
					s.info.Artist = strings.TrimSpace(text[30:60])
				}
				if s.info.Album == "" {
					s.info.Album = strings.TrimSpace(text[60:90])
				}
				changed = true
			}
		}
		if changed {
			remoteUpdate(s, b, UpdateMetadata)
		}
	case "@E":
		s.failed = true
		b.offer(Update{Kind: UpdateError, Generation: s.generation, Error: "mpg123: " + strings.TrimPrefix(line, "@E ")})
	}
	return nil
}

func secondsDuration(seconds float64) time.Duration {
	if seconds < 0 || seconds > float64(1<<63-1)/float64(time.Second) {
		return 0
	}
	return time.Duration(seconds * float64(time.Second))
}
