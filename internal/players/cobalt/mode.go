package cobalt

import (
	"time"

	"github.com/codemodify/media-player-music/internal/players"
	"goframework.io/desktop-ui/platform"
)

const modeFull = "full"
const modeCompact = "compact"

type modeResize struct {
	size             players.PresentationSize
	previousRequests map[players.PresentationSize]bool
	elapsed          time.Duration
	settled          time.Duration
	attempts         int
}

func (p *Player) Compact() bool { return p.mode == modeCompact }
func (p *Player) SetCompact(on bool) {
	mode := modeFull
	if on {
		mode = modeCompact
	}
	if p.mode == mode || p.Main == nil || p.Main.Closed() {
		return
	}
	p.captureModeSize()
	p.applyMode(mode)
}
func validBase(size players.PresentationSize) players.PresentationSize {
	if size.Width <= 0 {
		size.Width = MainW
	}
	if size.Height <= 0 {
		size.Height = MainH
	}
	return players.PresentationSize{Width: max(size.Width, MinW), Height: max(size.Height, MinH)}
}
func (p *Player) actualFullSize(base players.PresentationSize) players.PresentationSize {
	base = validBase(base)
	if p.drawPlaylist {
		base.Width += PlaylistTravel
	}
	if p.drawEqualizer {
		base.Height += EqualizerTravel
	}
	return base
}
func (p *Player) baseFromActual(actual players.PresentationSize) players.PresentationSize {
	if p.drawPlaylist {
		actual.Width -= PlaylistTravel
	}
	if p.drawEqualizer {
		actual.Height -= EqualizerTravel
	}
	return validBase(actual)
}
func (p *Player) captureModeSize() {
	if p.Compact() || p.Main == nil || p.Main.Closed() {
		return
	}
	width, height := p.Main.Size()
	actual := players.PresentationSize{Width: width, Height: height}
	if p.modeResize != nil {
		actual = p.modeResize.size
	}
	if p.restore != nil {
		actual = players.PresentationSize{Width: p.restore.desired.Width, Height: p.restore.desired.Height}
	}
	p.fullSize = p.baseFromActual(actual)
}
func (p *Player) applyMode(mode string) {
	if p.Main == nil || p.Main.Closed() {
		return
	}
	p.restore = nil
	oldW, oldH := p.Main.Size()
	previous := map[players.PresentationSize]bool{{Width: oldW, Height: oldH}: true}
	if pending := p.modeResize; pending != nil {
		for size := range pending.previousRequests {
			previous[size] = true
		}
		previous[pending.size] = true
	}
	p.mode = mode
	p.fullSize = validBase(p.fullSize)
	p.drawPlaylist, p.drawEqualizer = p.Host.Model.PlaylistVisible, p.Host.Model.EqualizerVisible
	size := players.PresentationSize{Width: CompactW, Height: CompactH}
	limits := platform.SizeLimits{MinWidth: CompactW, MinHeight: CompactH, MaxWidth: CompactW, MaxHeight: CompactH}
	if p.Compact() {
		p.Main.SetFullscreen(false)
		p.Main.SetMaximized(false)
		p.Main.SetContent(p.compact)
		p.Main.SetResizable(false)
	} else {
		size = p.actualFullSize(p.fullSize)
		minimum := p.actualFullSize(players.PresentationSize{Width: MinW, Height: MinH})
		limits = platform.SizeLimits{MinWidth: minimum.Width, MinHeight: minimum.Height}
		p.Main.SetContent(p.body)
		p.Main.SetResizable(true)
	}
	if surface, ok := p.Main.Surface().(platform.SizeLimitSurface); ok {
		surface.SetSizeLimits(limits)
	}
	p.modeResize = &modeResize{size: size, previousRequests: previous, attempts: 1}
	p.Main.SetSize(size.Width, size.Height)
	p.updateShape()
	p.Refresh()
	if p.Compact() {
		p.Main.RequestFocus(p.compact.play)
	} else {
		p.Main.RequestFocus(p.body.play)
	}
}
func (p *Player) validActual(size players.PresentationSize) bool {
	if p.Compact() {
		return size.Width == CompactW && size.Height == CompactH
	}
	minimum := p.actualFullSize(players.PresentationSize{Width: MinW, Height: MinH})
	return size.Width >= minimum.Width && size.Height >= minimum.Height
}
func (p *Player) acknowledgeModeSize(width, height int) {
	if p.modeResize == nil {
		return
	}
	size := players.PresentationSize{Width: width, Height: height}
	if size == p.modeResize.size {
		return
	}
	// X11 may lay out the requested backing size before a delayed answer
	// for an earlier request arrives. Keep the final request during the
	// transition rather than treating its optimistic layout as settled.
	if !p.validActual(size) || p.modeResize.previousRequests[size] {
		p.modeResize.settled = 0
		return
	}
	if !p.Compact() {
		p.fullSize = p.baseFromActual(size)
	}
	p.modeResize = nil
}
func (p *Player) retryModeResize(dt time.Duration) {
	pending := p.modeResize
	if pending == nil || p.Main == nil || p.Main.Closed() {
		return
	}
	pending.elapsed += dt
	width, height := p.Main.Size()
	if width == pending.size.Width && height == pending.size.Height {
		pending.settled += dt
		if pending.settled >= 250*time.Millisecond {
			p.modeResize = nil
		}
		return
	}
	pending.settled = 0
	if pending.elapsed >= placementRestoreTimeout || pending.attempts >= placementRetryLimit {
		p.modeResize = nil
		return
	}
	if !p.Main.WindowState().Maximized && !p.Main.WindowState().Fullscreen {
		p.Main.SetSize(pending.size.Width, pending.size.Height)
		pending.attempts++
	}
}
func (p *Player) CaptureState() players.PresentationState {
	p.captureModeSize()
	return players.PresentationState{Mode: p.mode, Sizes: map[string]players.PresentationSize{
		modeFull: validBase(p.fullSize), modeCompact: {Width: CompactW, Height: CompactH}}}
}
func (p *Player) RestoreState(state players.PresentationState) {
	if size, ok := state.Sizes[modeFull]; ok {
		p.fullSize = validBase(size)
	}
	mode := modeFull
	if state.Mode == modeCompact {
		mode = modeCompact
	}
	p.applyMode(mode)
}
