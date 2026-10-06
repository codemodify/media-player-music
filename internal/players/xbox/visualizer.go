package xbox

import (
	"math"

	"github.com/codemodify/paintengine2d"
	"github.com/codemodify/uitoolkit/a11y"
	"github.com/codemodify/uitoolkit/platform"
	"github.com/codemodify/uitoolkit/style"
	"github.com/codemodify/uitoolkit/widget"
)

// The original effects were WMP plug-ins. The frontend renders its own
// green spectrum or glow from the same audio levels as every other skin.
type visualizer struct {
	widget.Base
	p    *Player
	mode int
}

func newVisualizer(p *Player) *visualizer {
	v := &visualizer{p: p}
	v.Init(v)
	v.SetTransparent(true)
	v.SetAccessibleName("Audio visualization")
	return v
}
func (v *visualizer) Paint(ctx *paintengine2d.Context) {
	r := v.LocalBounds()
	lk := v.Look()
	levels := v.p.Spectrum.Bars
	ctx.Save()
	defer ctx.Restore()
	// Keep the lower corners inside the original rounded display well.
	ctx.ClipRoundRect(r, style.Dip(lk, 18), style.Dip(lk, 18))
	if len(levels) == 0 {
		return
	}
	energy := float32(0)
	for _, level := range levels {
		energy += level
	}
	energy /= float32(len(levels))
	if energy < .002 {
		return
	}
	if v.mode == 1 {
		for i, level := range levels {
			w := r.Dx() / float32(len(levels))
			h := max(0, min(1, level)) * r.Dy() * .75
			ctx.DrawRect(paintengine2d.XYWH(float32(i)*w+1, r.Dy()-h-8, max(w-2, 1), h), paintengine2d.Fill(paintengine2d.RGBA(.35, .8, .02, .8)))
		}
		return
	}
	phase := v.p.Transport.Pos.Seconds()
	for i := 0; i < 9; i++ {
		level := levels[i*len(levels)/9]
		at := paintengine2d.Pt(r.Dx()*(.18+.08*float32(i)), r.Dy()*(.45+.18*float32(math.Sin(phase*.7+float64(i)))))
		rad := r.Dy() * (.15 + .55*level)
		ctx.DrawCircle(at, rad, paintengine2d.Radial(paintengine2d.RadialGradient{Center: at, Radius: rad, Stops: []paintengine2d.GradientStop{
			{Offset: 0, Color: paintengine2d.RGBA(.66, .9, .1, .3+energy*.35)},
			{Offset: .4, Color: paintengine2d.RGBA(.24, .5, .02, energy*.45)},
			{Offset: 1, Color: paintengine2d.RGBA(.06, .11, .01, 0)},
		}}))
	}
}
func (v *visualizer) MousePress(e widget.MouseEvent) bool {
	if e.Button != platform.ButtonLeft {
		return false
	}
	delete(v.p.restore, "main")
	v.p.Main.StartMove()
	return true
}
func (v *visualizer) Describe(n *a11y.Node) {
	n.Role, n.Name = a11y.RoleImage, "Audio visualization"
	if n.Description == "" {
		n.Description = "Green glow or spectrum bars driven by audio frequency levels"
	}
}
