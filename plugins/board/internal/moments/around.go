package moments

import (
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/unit"
	"github.com/kjkrol/uid"
)

// rings is the scratch for the cells round where someone stands: each cell marked with the pass
// that met it, and the edge of the last ring.
type rings struct {
	met        []uint32
	pass       uint32
	edge, next []cell.ID
}

// around tells each the entity of every cell within rings of where a moment of the board stands —
// the cells under a Standing's box, a cell.Now's own cell — those first: the board's
// plugin.Tick.Around, for a rule's Here and Around.
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
	})
}

// ringsRound calls each with every cell within n rings of the cells seed adds, those first, each
// cell once. It is not reentrant: each must not ask for cells around again.
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
