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

const (
	heroPx     = 22
	heroHeight = 36
	heroSpeed  = CellSize * 3
)

type heroRow struct {
	Start cell.ID `yaml:"start"`
}

// hero walks on WSAD and casts the Silhouette of its sprite: its shape laid on the ground away
// from the light.
type hero struct{}

var _ board.Figure = hero{}

func (hero) Define(b *board.Plugin) {
	units := board.NewUnits[heroRow](b, board.Shape{Size: heroPx, Height: heroHeight},
		func(u heroRow) geom.Vec { return b.Board().CellCenter(u.Start) })
	profile := steering.Steering{
		MaxSpeed: heroSpeed,
		Accel:    heroSpeed * 4,
		Brake:    heroSpeed * 6,
	}
	units.Define(HeroKind, unit.Mover{Domain: cell.Land}, profile,
		comp.Const(world.Velocity{Dir: geom.NewVec(0, 1)}),
		comp.Const(light.Shadow{Cast: light.Silhouette}))
}

func (hero) Look(a *world.Atlas, w *world.Plugin) {
	a.Add(kind.Named[heroRow](w.Kinds(), HeroKind), heroPx, func(dst *render.Canvas, size int) {
		s := float32(size)
		dst.FillCircle(s/2, s*0.25, s*0.2, heroColor)
		dst.FillRect(s*0.3, s*0.45, s*0.4, s*0.5, heroColor)
		dst.FillRect(s*0.15, s*0.5, s*0.15, s*0.3, heroColor)
		dst.FillRect(s*0.7, s*0.5, s*0.15, s*0.3, heroColor)
	})
}
