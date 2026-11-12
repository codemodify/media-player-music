package aluminium

import (
	"time"

	"github.com/codemodify/media-player-music/internal/players"
	"goframework.io/desktop-ui/platform"
)

const modeFull = "full"
const modeMini = "mini"

type modeResize struct {
	size     players.PresentationSize
	elapsed  time.Duration
	attempts int
}

func (p *Player) MiniPlayer() bool { return p.mode == modeMini }

// SetMiniPlayer changes content and resize limits on the existing Main window.
// Both modes retain the current desktop position and their own dimensions.
// The original MiniPlayer fixes height while letting the LCD fold at narrow
// widths, as described in the preserved iTunes 4 help.
func (p *Player) SetMiniPlayer(on bool) {
	mode := modeFull
	if on {
		mode = modeMini
	}
	if p.mode == mode || p.Main == nil || p.Main.Closed() {
		return
	}
	p.captureModeSize()
	p.applyMode(mode)
}

func (p *Player) captureModeSize() {
	if p.Main == nil || p.Main.Closed() {
		return
	}
	size := players.PresentationSize{}
	size.Width, size.Height = p.Main.Size()
	if p.modeResize != nil {
		size = p.modeResize.size
	}
	if pending := p.restore["main"]; pending != nil {
		size.Width, size.Height = pending.desired.Width, pending.desired.Height
	}
	if p.sizes == nil {
		p.sizes = make(map[string]players.PresentationSize)
	}
	p.sizes[p.mode] = validSize(p.mode, size)
}

func validSize(mode string, size players.PresentationSize) players.PresentationSize {
	if mode == modeMini {
		if size.Width <= 0 {
			size.Width = MiniW
		}
		return players.PresentationSize{Width: min(max(size.Width, MinMiniW), MaxMiniW), Height: MiniH}
	}
	if size.Width <= 0 {
		size.Width = MainW
	}
	if size.Height <= 0 {
		size.Height = MainH
	}
	return players.PresentationSize{Width: max(size.Width, MinW), Height: max(size.Height, MinH)}
}

func (p *Player) applyMode(mode string) {
	if p.Main == nil || p.Main.Closed() {
		return
	}
	delete(p.restore, "main")
	p.mode = mode
	size := validSize(mode, p.sizes[mode])
	p.sizes[mode] = size
	if mode == modeMini {
		p.Main.SetFullscreen(false)
		p.Main.SetMaximized(false)
		p.Main.SetContent(p.mini)
	} else {
		p.Main.SetContent(p.body)
	}
	// Both views resize horizontally; MiniPlayer's equal min/max height is
	// a native size constraint, rather than an app-only drag restriction.
	p.Main.SetResizable(true)
	if surface, ok := p.Main.Surface().(platform.SizeLimitSurface); ok {
		limits := platform.SizeLimits{MinWidth: MinW, MinHeight: MinH}
		if mode == modeMini {
			limits = platform.SizeLimits{MinWidth: MinMiniW, MinHeight: MiniH, MaxWidth: MaxMiniW, MaxHeight: MiniH}
		}
		surface.SetSizeLimits(limits)
	}
	p.modeResize = &modeResize{size: size}
	p.Main.SetSize(size.Width, size.Height)
	p.modeResize.attempts++
	p.updateShape()
	p.Refresh()
	if mode == modeMini {
		p.Main.RequestFocus(p.mini.play)
	} else {
		p.Main.RequestFocus(p.body.play)
	}
}

func (p *Player) retryModeResize(dt time.Duration) {
	pending := p.modeResize
	if pending == nil || p.Main == nil || p.Main.Closed() {
		return
	}
	pending.elapsed += dt
	width, height := p.Main.Size()
	if width == pending.size.Width && height == pending.size.Height {
		p.modeResize = nil
		return
	}
	if pending.elapsed >= placementRestoreTimeout || pending.attempts >= placementRetryLimit {
		p.modeResize = nil
		return
	}
	// A desktop may first restore a maximized window, then acknowledge its
	// new mode dimensions. Retry on the UI pulse until settled, with a bound.
	if !p.Main.WindowState().Maximized && !p.Main.WindowState().Fullscreen {
		p.Main.SetSize(pending.size.Width, pending.size.Height)
		pending.attempts++
	}
}

func (p *Player) CaptureState() players.PresentationState {
	p.captureModeSize()
	out := players.PresentationState{Mode: p.mode, Sizes: make(map[string]players.PresentationSize, 2)}
	for _, mode := range []string{modeFull, modeMini} {
		out.Sizes[mode] = validSize(mode, p.sizes[mode])
	}
	return out
}
func (p *Player) RestoreState(state players.PresentationState) {
	if p.sizes == nil {
		p.sizes = make(map[string]players.PresentationSize)
	}
	for _, mode := range []string{modeFull, modeMini} {
		if size, ok := state.Sizes[mode]; ok {
			p.sizes[mode] = validSize(mode, size)
		}
	}
	mode := modeFull
	if state.Mode == modeMini {
		mode = modeMini
	}
	p.applyMode(mode)
}

// acknowledgeModeSize adopts the first legal answer for the active mode.
// Once the desktop has acknowledged a MiniPlayer height, a later width drag
// belongs to the user, even when it arrives before the next application pulse.
func (p *Player) acknowledgeModeSize(width, height int) {
	if p.modeResize == nil {
		return
	}
	size := players.PresentationSize{Width: width, Height: height}
	if size != validSize(p.mode, size) {
		return
	}
	p.sizes[p.mode] = size
	p.modeResize = nil
}
