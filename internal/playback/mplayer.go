package playback

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// NewMPlayer controls an installed MPlayer through slave stdin/stdout. Native
// equalizer/pan filters are not advertised as our exact DSP or spectrum API.
func NewMPlayer(opts EngineOptions) (Engine, error) {
	return newProcessEngine(opts, mplayerDialect{}, Capabilities{Seek: true, Formats: "Audio/media formats supported by the installed MPlayer build"})
}

type mplayerDialect struct{}

func (mplayerDialect) name() string { return "mplayer" }
func (mplayerDialect) args(extra []string) []string {
	return append(append([]string(nil), extra...), "-slave", "-idle", "-quiet", "-identify", "-noconfig", "all",
		"-novideo", "-nolirc", "-nojoystick", "-nomouseinput", "-input", "nodefault-bindings", "-softvol", "-volume", "0",
		"-demuxer", "lavf", "-msglevel", "global=6")
}
func (mplayerDialect) ready(line string) bool { return strings.HasPrefix(line, "MPlayer ") }
func (mplayerDialect) quit() string           { return "quit" }
func (mplayerDialect) load(s *remoteSession) []string {
	return []string{"pausing loadfile " + mplayerQuote(s.path) + " 0", "pausing get_property length"}
}
func (mplayerDialect) pause(bool) string { return "pause" }
func (mplayerDialect) seek(pos time.Duration) string {
	return fmt.Sprintf("pausing_keep seek %.6f 2", pos.Seconds())
}
func (mplayerDialect) volume(v float32) string {
	return fmt.Sprintf("pausing_keep_force volume %.6f 1", v*100)
}
func (mplayerDialect) poll(s *remoteSession) []string {
	remotePoll(s)
	return []string{"pausing_keep_force get_property time_pos"}
}

func mplayerQuote(value string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(value) + `"`
}

func (d mplayerDialect) parse(s *remoteSession, line string, b *updateBroker) []string {
	// -idle suppresses ID_EXIT=EOF until process exit. MPlayer's global
	// verbose channel emits the internal EOF code for each finished file.
	if fields := strings.Fields(line); len(fields) >= 3 && fields[0] == "EOF" && fields[1] == "code:" && fields[2] == "1" && s.loaded && !s.ended && !s.failed {
		s.ended = true
		remoteUpdate(s, b, UpdateEnded)
	}
	key, value, ok := strings.Cut(strings.TrimSpace(line), "=")
	if ok {
		value = strings.Trim(value, "'\"")
		switch key {
		case "ID_AUDIO_RATE":
			s.info.SampleRate, _ = strconv.Atoi(value)
			remoteUpdate(s, b, UpdateAudioFormat)
		case "ID_AUDIO_NCH":
			s.info.Channels, _ = strconv.Atoi(value)
			remoteUpdate(s, b, UpdateAudioFormat)
		case "ID_AUDIO_BITRATE":
			s.info.Bitrate, _ = strconv.Atoi(value)
			remoteUpdate(s, b, UpdateAudioFormat)
		case "ID_LENGTH", "ANS_length":
			seconds, err := strconv.ParseFloat(value, 64)
			if err == nil && seconds > 0 {
				s.duration = secondsDuration(seconds)
				remoteUpdate(s, b, UpdateDuration)
			}
			// ID_LENGTH is demuxer identification, not playback readiness.
			if key == "ANS_length" {
				return remoteLoaded(s, b, d)
			}
		case "ANS_time_pos":
			seconds, err := strconv.ParseFloat(value, 64)
			if err == nil && seconds >= 0 {
				s.position = secondsDuration(seconds)
				return remoteClock(s, b, d)
			}
		case "ID_EXIT":
			if value == "EOF" && s.loaded && !s.ended && !s.failed {
				s.ended = true
				remoteUpdate(s, b, UpdateEnded)
			}
		default:
			if strings.HasPrefix(key, "ID_CLIP_INFO_NAME") {
				s.metadataKeys[strings.TrimPrefix(key, "ID_CLIP_INFO_NAME")] = strings.ToLower(value)
			}
			if strings.HasPrefix(key, "ID_CLIP_INFO_VALUE") {
				switch s.metadataKeys[strings.TrimPrefix(key, "ID_CLIP_INFO_VALUE")] {
				case "title":
					s.info.Title = value
				case "artist", "author":
					s.info.Artist = value
				case "album":
					s.info.Album = value
				default:
					return nil
				}
				remoteUpdate(s, b, UpdateMetadata)
			}
		}
	}
	if strings.Contains(line, "Starting playback...") {
		// Queries submitted while the idle loop loads a file can be discarded.
		// Ask again after the decoder reports playback startup.
		return []string{"pausing get_property length"}
	}
	if strings.Contains(line, "Failed to open") || strings.Contains(line, "Error opening") || strings.Contains(line, "Cannot open") || strings.Contains(line, "File not found:") || strings.Contains(line, "No stream found to handle") || strings.Contains(line, "Failed to recognize file format") || strings.Contains(line, "Cannot find codec matching") {
		s.failed = true
		b.offer(Update{Kind: UpdateError, Generation: s.generation, Error: "MPlayer: " + strings.TrimSpace(line)})
	}
	return nil
}
