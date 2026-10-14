package main

import (
	"image/color"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/unit"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/steering"
	"github.com/kjkrol/gram/render"
)

const (
	blockSize  = 24
	blockSpeed = CellSize * 5
)

type blockRow struct {
	Start cell.ID `yaml:"start"`
}

// block is a player's unit, one kind a player in its colour.
type block struct {
	kind  string
	color color.RGBA
}

var _ board.Figure = block{}

func (bk block) Define(b *board.Plugin) {
	units := board.NewUnits[blockRow](b, board.Shape{Size: blockSize},
		func(r blockRow) geom.Vec { return b.Board().CellCenter(r.Start) })
	profile := steering.Steering{
		MaxSpeed: blockSpeed,
		Accel:    blockSpeed * 3,
		Brake:    blockSpeed * 6,
		TurnRate: 0.3,
	}
	units.Define(bk.kind, unit.Mover{Domain: cell.Land}, profile)
}

func (bk block) Look(a *world.Atlas, w *world.Plugin) {
	a.Add(kind.Named[blockRow](w.Kinds(), bk.kind), blockSize, render.Solid(bk.color))
}
