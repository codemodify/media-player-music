package fancy

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/codemodify/media-player-music/internal/players"
	"goframework.io/desktop-ui-widgets"
	"goframework.io/desktop-ui/widget"
)

type equalizerFile struct {
	Version int       `json:"version"`
	On      bool      `json:"on"`
	Preamp  float32   `json:"preamp"`
	Bands   []float32 `json:"bands"`
	Preset  string    `json:"preset"`
}

func validEqualizerFile(s equalizerFile) error {
	if s.Version != 1 || len(s.Bands) != len(players.EqBands) {
		return errors.New("not a supported ten-band equalizer file")
	}
	gains := append([]float32{s.Preamp}, s.Bands...)
	for _, v := range gains {
		if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) || v < -players.EqRange || v > players.EqRange {
			return errors.New("equalizer gains must be finite and between -12 and +12 dB")
		}
	}
	if len(s.Preset) > 128 {
		return errors.New("equalizer preset name is too long")
	}
	return nil
}
func readEqualizerFile(path string) (equalizerFile, error) {
	f, err := os.Open(path)
	if err != nil {
		return equalizerFile{}, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, 65537))
	if err != nil {
		return equalizerFile{}, err
	}
	if len(data) > 65536 {
		return equalizerFile{}, errors.New("equalizer file is too large")
	}
	var state equalizerFile
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&state); err != nil {
		return state, err
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return state, errors.New("equalizer file has trailing data")
	}
	return state, validEqualizerFile(state)
}
func snapshotEqualizer(e *players.Equalizer) equalizerFile {
	return equalizerFile{Version: 1, On: e.On, Preamp: e.Preamp, Bands: append([]float32(nil), e.Gains...), Preset: e.Preset}
}
func writeFile(path string, data []byte) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".fancy-save-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if _, err = f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
func (p *Player) fileError(kind string, err error) {
	if err != nil {
		p.Host.Model.Error = kind + ": " + err.Error()
		p.Refresh()
	}
}
func (p *Player) chooseFile(from widget.Component, opts widgets.FileDialogOptions, pick func(string) error) {
	var fd *widgets.FileDialog
	opts.Native = true
	opts.OnCancel = func() { delete(p.dialogs, fd) }
	opts.OnPick = func(path string) {
		delete(p.dialogs, fd)
		if w := p.Main(); w == nil || w.Closed() {
			return
		}
		if err := pick(path); err != nil {
			p.fileError(opts.Title, err)
		} else {
			p.Host.Model.Error = ""
			p.Refresh()
		}
	}
	fd = widgets.ShowFileDialog(from, opts)
	if fd != nil {
		if p.dialogs == nil {
			p.dialogs = make(map[*widgets.FileDialog]bool)
		}
		p.dialogs[fd] = true
	}
}
func (p *Player) loadEqualizer(from widget.Component) {
	p.chooseFile(from, widgets.FileDialogOptions{Title: "Load equalizer settings", Filter: "*.json", Mode: widgets.FileOpen}, func(path string) error {
		state, err := readEqualizerFile(path)
		if err != nil {
			return err
		}
		return p.applyEqualizerFile(state)
	})
}
func (p *Player) applyEqualizerFile(state equalizerFile) error {
	if err := validEqualizerFile(state); err != nil {
		return err
	}
	eq := p.Equaliser
	eq.On, eq.Auto, eq.Preamp = state.On, false, state.Preamp
	eq.Gains = append(eq.Gains[:0], state.Bands...)
	eq.Preset = state.Preset
	if eq.Preset == "" {
		eq.Preset = "Custom"
	}
	if eq.Changed != nil {
		eq.Changed()
	}
	return nil
}
func (p *Player) saveEqualizer(from widget.Component) {
	p.chooseFile(from, widgets.FileDialogOptions{Title: "Save equalizer settings", Filter: "*.json", Name: "fancy-equalizer.json", Mode: widgets.FileSave}, func(path string) error {
		state := snapshotEqualizer(p.Equaliser)
		if err := validEqualizerFile(state); err != nil {
			return err
		}
		data, err := json.MarshalIndent(state, "", "  ")
		if err != nil {
			return err
		}
		return writeFile(path, append(data, '\n'))
	})
}
func playlistM3U(queue *players.Playlist) ([]byte, error) {
	var b strings.Builder
	b.WriteString("#EXTM3U\n")
	count := 0
	for _, track := range queue.Tracks {
		if track.Path == "" {
			continue
		}
		if strings.ContainsAny(track.Path, "\r\n") {
			return nil, errors.New("a file path contains a newline and cannot be saved in M3U format")
		}
		label := strings.NewReplacer("\r", " ", "\n", " ").Replace(track.Label())
		seconds := int64(-1)
		if track.Length > 0 {
			seconds = int64(track.Length.Seconds())
		}
		fmt.Fprintf(&b, "#EXTINF:%d,%s\n%s\n", seconds, label, track.Path)
		count++
	}
	if count == 0 {
		return nil, errors.New("the playlist has no local music files to export")
	}
	return []byte(b.String()), nil
}
func (p *Player) savePlaylist(from widget.Component) {
	p.chooseFile(from, widgets.FileDialogOptions{Title: "Save playlist", Filter: "*.m3u;*.m3u8", Name: "playlist.m3u8", Mode: widgets.FileSave}, func(path string) error {
		data, err := playlistM3U(p.Transport.List)
		if err != nil {
			return err
		}
		return writeFile(path, data)
	})
}
