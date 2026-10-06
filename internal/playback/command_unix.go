//go:build !windows

package playback

import "os/exec"

func configureCommand(cmd *exec.Cmd) {}
