package navigation

import (
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
)

func cellAtXY(g board.Grid, x, y uint32) cell.ID {
	c, _ := g.CellIndex(x, y)
	return c
}
