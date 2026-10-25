package navigation

import (
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
)

func cellAtXY(g grid.Grid, x, y uint32) cell.ID {
	c := g.At(x, y)
	return c
}

func cellBox(g grid.Grid, c cell.ID, size uint32) plane.AABB {
	at := g.CellCenter(c)
	half := float32(size) / 2
	return plane.NewAABB(geom.NewVec(at.X-half, at.Y-half), float32(size), float32(size))
}
