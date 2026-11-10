package headspace

import (
	"math"

	"goframework.io/desktop-ui/a11y"
	"goframework.io/desktop-ui/platform"
	"goframework.io/desktop-ui/style"
	"goframework.io/desktop-ui/widget"
	"goframework.io/graphics-paint2d"
)

// The original lightning effect was a WMP visualization plug-in, not bitmap
// artwork in the skin. This display draws from the app's measured spectrum;
// screenshot fixtures use the same deterministic spectrum as the other skins.
type visualizer struct {
	widget.Base
	p    *Player
	bars bool
}

func newVisualizer(p *Player) *visualizer {
	v := &visualizer{p: p}
	v.Init(v)
	v.SetAccessibleName("Audio visualization")
	return v
}

func (v *visualizer) Paint(ctx *paint2d.Context) {
	r := v.LocalBounds()
	lk := v.Look()
	ctx.Save()
	defer ctx.Restore()
	ctx.ClipRoundRect(r, style.Dip(lk, 10), style.Dip(lk, 10))
	ctx.DrawRect(r, paint2d.Fill(paint2d.RGB(.025, .035, .12)))
	levels := v.p.Spectrum.Bars
	if len(levels) == 0 {
		return
	}
	if v.bars {
		for i, level := range levels {
			w := r.Dx() / float32(len(levels))
			h := max(0, min(1, level)) * r.Dy() * .88
			col := paint2d.RGB(.35+.5*level, .45+.4*level, 1)
			ctx.DrawRect(paint2d.XYWH(float32(i)*w+1, r.Dy()-h-4, max(w-2, 1), h), paint2d.Fill(col))
		}
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
	phase := float64(v.p.Transport.Pos.Seconds())
	center := paint2d.Pt(r.Dx()*(.5+.08*float32(math.Sin(phase*.43))), r.Dy()*(.47+.06*float32(math.Cos(phase*.37))))
	for cloud := 0; cloud < 15; cloud++ {
		angle := float64(cloud)*2.39996 + phase*.08
		distance := float64(cloud%5) * float64(r.Dx()) * .065
		at := paint2d.Pt(center.X+float32(math.Cos(angle)*distance), center.Y+float32(math.Sin(angle)*distance))
		radius := r.Dy() * (.12 + .1*levels[cloud*len(levels)/15])
		ctx.DrawCircle(at, radius, paint2d.Radial(paint2d.RadialGradient{
			Center: at, Radius: radius,
			Stops: []paint2d.GradientStop{
				{Offset: 0, Color: paint2d.RGBA(.32, .43, .76, energy*.65)},
				{Offset: .5, Color: paint2d.RGBA(.14, .23, .61, energy*.4)},
				{Offset: 1, Color: paint2d.RGBA(.07, .1, .32, 0)},
			},
		}))
	}
	// Branched paths recall Headspace's blue lightning display. Their length
	// and brightness depend on frequency bands, so silence leaves it at rest.
	for ray := 0; ray < 7; ray++ {
		level := levels[ray*len(levels)/7]
		angle := float64(ray)*math.Pi*2/7 + .3*math.Sin(phase*.7+float64(ray*11))
		length := float64(max(r.Dx(), r.Dy())) * (.26 + .36*float64(level))
		p := paint2d.NewPath()
		p.MoveTo(center.X, center.Y)
		for step := 1; step <= 14; step++ {
			d := length * float64(step) / 14
			noise := math.Sin(float64(step*71+ray*137) + math.Floor(phase*5)*2.31)
			wiggle := noise * float64(5+12*level)
			x := center.X + float32(math.Cos(angle)*d-math.Sin(angle)*wiggle)
			y := center.Y + float32(math.Sin(angle)*d+math.Cos(angle)*wiggle)
			p.LineTo(x, y)
			if step == 7 {
				branch := paint2d.NewPath()
				branch.MoveTo(x, y)
				branch.LineTo(x+float32(math.Cos(angle+1.1)*length*.15), y+float32(math.Sin(angle+1.1)*length*.15))
				branch.LineTo(x+float32(math.Cos(angle+.8)*length*.25), y+float32(math.Sin(angle+.8)*length*.25))
				ctx.DrawPath(branch, paint2d.StrokePaint(paint2d.RGBA(.65, .75, 1, .35+level*.3), style.Dip(lk, .7)))
			}
		}
		ctx.DrawPath(p, paint2d.StrokePaint(paint2d.RGBA(.25, .32, .85, .13+energy*.15), style.Dip(lk, 6)))
		ctx.DrawPath(p, paint2d.StrokePaint(paint2d.RGBA(.45, .56, 1, .3+level*.3), style.Dip(lk, 2)))
		ctx.DrawPath(p, paint2d.StrokePaint(paint2d.RGBA(.82, .87, 1, .6+level*.4), style.Dip(lk, .8)))
	}
}
func (v *visualizer) MousePress(e widget.MouseEvent) bool {
	if e.Button != platform.ButtonLeft {
		return false
	}
	v.p.restore = nil
	v.p.Main.StartMove()
	return true
}
func (v *visualizer) Describe(n *a11y.Node) {
	n.Role, n.Name = a11y.RoleImage, "Audio visualization"
	if n.Description == "" {
		n.Description = "Lightning or spectrum bars driven by audio frequency levels"
	}
}
