package moments

import (
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/unit"
	"github.com/kjkrol/uid"
)

type rings struct {
	met        []uint32
	pass       uint32
	edge, next []cell.ID
}

func (r *Rules) around(moment any, rings int, each func(uid.UID64)) {
	var seed func(add func(cell.ID))
	switch m := moment.(type) {
	case *unit.Standing:
		seed = func(add func(cell.ID)) { r.grid.CellsUnder(m.Box, add) }
	case *cell.Now:
		seed = func(add func(cell.ID)) { add(m.Cell) }
	default:
		return
	}
	r.ringsRound(seed, rings, func(c cell.ID) {
		if id, ok := r.cells.Entity(c); ok {
			each(id)
		}
		r.cells.Standing(c, each)
	})
}

// the cells one to n steps ahead of the unit, the grid's direction nearest its heading, stopping
// at a closed edge; a cell faces nowhere, so a cell.Now has none
func (r *Rules) ahead(moment any, n int, each func(uid.UID64)) {
	m, ok := moment.(*unit.Standing)
	if !ok || (m.Heading.X == 0 && m.Heading.Y == 0) {
		return
	}
	way, c := grid.Nearest(r.grid, m.Heading), m.Cell
	for range n {
		if c, ok = r.grid.Toward(c, way); !ok {
			return
		}
		if id, ok := r.cells.Entity(c); ok {
			each(id)
		}
		r.cells.Standing(c, each)
	}
}

// not reentrant: each must not ask for cells around again
func (r *Rules) ringsRound(seed func(add func(cell.ID)), n int, each func(cell.ID)) {
	g := &r.rings
	if count := r.grid.CellCount(); len(g.met) != count {
		g.met, g.pass = make([]uint32, count), 0
	}
	if g.pass++; g.pass == 0 {
		clear(g.met)
		g.pass = 1
	}
	meet := func(c cell.ID) bool {
		i, ok := r.cells.Ordinal(c)
		if !ok || g.met[i] == g.pass {
			return false
		}
		g.met[i] = g.pass
		return true
	}
	g.edge = g.edge[:0]
	seed(func(c cell.ID) {
		if meet(c) {
			g.edge = append(g.edge, c)
		}
	})
	for ring := 0; ; ring++ {
		for _, c := range g.edge {
			each(c)
		}
		if ring == n {
			return
		}
		g.next = g.next[:0]
		for _, c := range g.edge {
			for _, nb := range r.grid.Neighbors(c) {
				if meet(nb) {
					g.next = append(g.next, nb)
				}
			}
		}
		g.edge, g.next = g.next, g.edge
	}
}
