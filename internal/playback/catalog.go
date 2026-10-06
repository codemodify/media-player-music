package playback

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

type BackendInfo struct {
	ID           string
	Name         string
	Executable   string
	Available    bool
	Capabilities Capabilities
}

// Detect discovers programs without starting them or changing the machine.
// PATH wins, followed by common application installation locations. A program
// being found is distinct from it successfully starting its control interface.
func Detect() []BackendInfo { return detectFor(runtime.GOOS, exec.LookPath, os.Getenv) }

func detectFor(goos string, look func(string) (string, error), env func(string) string) []BackendInfo {
	definitions := []BackendInfo{
		{ID: "mpv", Name: "mpv", Capabilities: Capabilities{Seek: true, Equalizer: true, Balance: true, Spectrum: goos != "windows", Formats: "Most audio formats"}},
		{ID: "vlc", Name: "VLC", Capabilities: Capabilities{Seek: true, Formats: "Most audio formats"}},
		{ID: "mplayer", Name: "MPlayer", Capabilities: Capabilities{Seek: true, Formats: "Most audio formats supported by the installed build"}},
		{ID: "mpg123", Name: "mpg123", Capabilities: Capabilities{Seek: true, Formats: "MP3 / MPEG audio"}},
	}
	for i := range definitions {
		info := &definitions[i]
		candidates := []string{info.ID}
		if goos == "darwin" {
			candidates = append(candidates, filepath.Join("/opt/homebrew/bin", info.ID), filepath.Join("/usr/local/bin", info.ID))
			if info.ID == "vlc" {
				candidates = append(candidates, "/Applications/VLC.app/Contents/MacOS/VLC")
			}
			if info.ID == "mpv" {
				candidates = append(candidates, "/Applications/mpv.app/Contents/MacOS/mpv")
			}
		}
		if goos == "windows" {
			candidates = []string{info.ID + ".exe", info.ID}
			for _, variable := range []string{"ProgramFiles", "ProgramFiles(x86)", "LOCALAPPDATA"} {
				if root := env(variable); root != "" {
					if info.ID == "vlc" {
						candidates = append(candidates, filepath.Join(root, "VideoLAN", "VLC", "vlc.exe"))
					}
					candidates = append(candidates, filepath.Join(root, info.ID, info.ID+".exe"))
				}
			}
		}
		for _, candidate := range candidates {
			if path, err := look(candidate); err == nil {
				info.Executable, info.Available = path, true
				break
			}
		}
	}
	return definitions
}

func Open(info BackendInfo) (Engine, error) {
	if !info.Available || info.Executable == "" {
		return nil, fmt.Errorf("%s is not installed; install it and refresh Settings", info.Name)
	}
	opts := EngineOptions{Executable: info.Executable}
	switch info.ID {
	case "mpv":
		return NewMPVEngine(opts)
	case "vlc":
		return NewVLC(opts)
	case "mplayer":
		return NewMPlayer(opts)
	case "mpg123":
		return NewMPG123(opts)
	default:
		return nil, fmt.Errorf("unknown playback backend %q", info.ID)
	}
}
