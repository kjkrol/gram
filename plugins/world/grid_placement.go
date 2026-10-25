package world

import (
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/gram/internal/f32"
)

// GridPlacement arranges entities on a regular grid spanning [0,Width)×[0,Height).
type GridPlacement struct {
	Width, Height uint32
	EntitySize    uint32
}

func NewGridPlacement(width, height, entitySize uint32) *GridPlacement {
	return &GridPlacement{Width: width, Height: height, EntitySize: entitySize}
}

func (p *GridPlacement) Place(index, count int) Position {
	cols := uint32(f32.Ceil(f32.Sqrt(float32(count))))
	rows := uint32(f32.Ceil(float32(count) / float32(cols)))
	row := uint32(index) / cols
	col := uint32(index) % cols

	cellWidth := p.Width / cols
	cellHeight := p.Height / rows

	x := (col * cellWidth) + (cellWidth / 2) - (p.EntitySize / 2)
	y := (row * cellHeight) + (cellHeight / 2) - (p.EntitySize / 2)

	return Position{AABB: plane.NewAABB(geom.NewVec(float32(x), float32(y)),
		float32(p.EntitySize),
		float32(p.EntitySize))}
}
