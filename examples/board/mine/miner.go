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

// the miner as in life, metres: his shoulders, his height and his run; and his sprite, pixels
const (
	minerWide   = 0.8
	minerTall   = 1.8
	minerSpeed  = 4
	minerSprite = 16
)

type minerRow struct {
	Start cell.ID `yaml:"start"`
}

// miner walks on WSAD into the mine and casts the silhouette of his sprite.
type miner struct{}

var _ board.Figure = miner{}

func (miner) Define(b *board.Plugin) {
	shape := board.Shape{Size: scale.Units(minerWide), Height: scale.Units(minerTall)}
	units := board.NewUnits[minerRow](b, shape,
		func(u minerRow) geom.Vec { return b.Read(0).Board().CellCenter(u.Start) })
	speed := scale.Units(minerSpeed)
	profile := steering.Steering{
		MaxSpeed: speed,
		Accel:    speed * 4,
		Brake:    speed * 6,
	}
	units.Define(MinerKind, unit.Mover{Domain: cell.Land}, profile,
		comp.Const(world.Velocity{Dir: geom.NewVec(1, 0)}),
		comp.Const(light.Shadow{Cast: light.Silhouette}))
}

func (miner) Look(a *world.Atlas, w *world.Plugin) {
	a.Add(kind.Named[minerRow](w.Kinds(), MinerKind), minerSprite,
		func(dst *render.Canvas, size int) {
			s := float32(size)
			dst.FillCircle(s/2, s*0.25, s*0.2, skinColor)
			dst.FillRect(s*0.3, s*0.08, s*0.4, s*0.12, helmetColor)
			dst.FillRect(s*0.3, s*0.45, s*0.4, s*0.5, overallColor)
			dst.FillRect(s*0.15, s*0.5, s*0.15, s*0.3, overallColor)
			dst.FillRect(s*0.7, s*0.5, s*0.15, s*0.3, overallColor)
		}).Uncovered()
}
