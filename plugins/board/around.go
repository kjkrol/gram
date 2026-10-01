package board

import (
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/uid"
)

// rings is the board's scratch for the cells round where someone stands: each cell marked with
// the pass that met it, and the edge of the last ring.
type rings struct {
	met        []uint32
	pass       uint32
	edge, next []cell.ID
}

// around calls each with every cell within n rings of the cells seed adds, those first, each cell
// once. It is not reentrant: each must not ask for cells around again.
func (b *Board) around(seed func(add func(cell.ID)), n int, each func(cell.ID)) {
	r := &b.rings
	if len(r.met) != b.CellCount() {
		r.met, r.pass = make([]uint32, b.CellCount()), 0
	}
	if r.pass++; r.pass == 0 {
		clear(r.met)
		r.pass = 1
	}
	meet := func(c cell.ID) bool {
		i, ok := b.Ordinal(c)
		if !ok || r.met[i] == r.pass {
			return false
		}
		r.met[i] = r.pass
		return true
	}
	r.edge = r.edge[:0]
	seed(func(c cell.ID) {
		if meet(c) {
			r.edge = append(r.edge, c)
		}
	})
	for ring := 0; ; ring++ {
		for _, c := range r.edge {
			each(c)
		}
		if ring == n {
			return
		}
		r.next = r.next[:0]
		for _, c := range r.edge {
			for _, nb := range b.Neighbors(c) {
				if meet(nb) {
					r.next = append(r.next, nb)
				}
			}
		}
		r.edge, r.next = r.next, r.edge
	}
}

// placesAround tells each the entity of every cell within rings of where a moment of the board's
// stands — the cells under a Standing's box, a Cell itself — those first: the board's
// plugin.Tick.Around, for a rule's Here and Around.
func (b *Board) placesAround(moment any, rings int, each func(uid.UID64)) {
	switch m := moment.(type) {
	case *Standing:
		b.entitiesAround(func(add func(cell.ID)) { b.CellsUnder(m.Box, add) }, rings, each)
	case *cell.Now:
		b.entitiesAround(func(add func(cell.ID)) { add(m.Cell) }, rings, each)
	}
}

// entitiesAround is around telling each cell's entity.
func (b *Board) entitiesAround(seed func(add func(cell.ID)), n int, each func(uid.UID64)) {
	b.around(seed, n, func(c cell.ID) {
		if id, ok := b.CellEntity(c); ok {
			each(id)
		}
	})
}
