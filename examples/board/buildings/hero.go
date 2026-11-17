package main

import (
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/unit"
	"github.com/kjkrol/gram/plugins/light"
	"github.com/kjkrol/gram/plugins/motion/steering"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

// the hero as in life, metres: its shoulders, its height and its run; and its sprite, pixels
const (
	heroWide   = 0.8
	heroTall   = 1.8
	heroSpeed  = 5
	heroSprite = 16
)

type heroRow struct {
	Start cell.ID `yaml:"start"`
}

// hero walks on WSAD among the buildings and casts the silhouette of its sprite.
type hero struct{}

var _ board.Figure = hero{}

func (hero) Define(b *board.Plugin) {
	shape := board.Shape{Size: scale.Units(heroWide), Height: scale.Units(heroTall)}
	units := board.NewUnits[heroRow](b, shape,
		func(u heroRow) geom.Vec { return b.Read(0).Board().CellCenter(u.Start) })
	speed := scale.Units(heroSpeed)
	profile := steering.Steering{
		MaxSpeed: speed,
		Accel:    speed * 4,
		Brake:    speed * 6,
	}
	units.Define(HeroKind, unit.Mover{Domain: cell.Land}, profile,
		comp.Const(world.Velocity{Dir: geom.NewVec(0, 1)}),
		comp.Const(light.Shadow{Cast: light.Silhouette}))
}

func (hero) Look(a *world.Atlas, w *world.Plugin) {
	a.Add(kind.Named[heroRow](w.Kinds(), HeroKind), heroSprite, func(dst *render.Canvas, size int) {
		s := float32(size)
		dst.FillCircle(s/2, s*0.25, s*0.2, heroColor)
		dst.FillRect(s*0.3, s*0.45, s*0.4, s*0.5, heroColor)
		dst.FillRect(s*0.15, s*0.5, s*0.15, s*0.3, heroColor)
		dst.FillRect(s*0.7, s*0.5, s*0.15, s*0.3, heroColor)
	})
}
