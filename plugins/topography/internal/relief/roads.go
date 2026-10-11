package relief

import (
	"slices"

	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/ground"
)

// KeepRoads runs shape and takes back all it did when it left a road lying where Transport Tycoon
// would not lay it (ground.RoadFits; ground.RoadJoins between level cells) that lay well before.
func (r *Relief) KeepRoads(shape func()) {
	if r.board == nil {
		shape()
		return
	}
	var kept []cell.ID
	r.grid.EachCell(func(c cell.ID) {
		if w := r.board.Way(c); w.Runs() && w.Kind.Graded && r.roadFits(c, w.Links) {
			kept = append(kept, c)
		}
	})
	before, v := slices.Clone(r.values), r.version
	shape()
	if r.version == v {
		return
	}
	for _, c := range kept {
		if !r.roadFits(c, r.board.Way(c).Links) {
			r.restore(before)
			return
		}
	}
}

// roadFits reports whether a road running out of c as links say lies as Transport Tycoon lays it.
func (r *Relief) roadFits(c cell.ID, links cell.Links) bool {
	if !r.square {
		for i := range 8 {
			nb, ok := r.grid.Toward(c, i)
			if ok && links&(1<<i) != 0 && !ground.RoadJoins(r.Altitude(c), r.Altitude(nb)) {
				return false
			}
		}
		return true
	}
	k := r.Corners(c)
	corners := [4]float64{float64(k[0]), float64(k[1]), float64(k[2]), float64(k[3])}
	return ground.RoadFits(corners, links, r.sq.Cell)
}

// restore puts every height back as values hold it.
func (r *Relief) restore(values []float32) {
	for i, h := range values {
		v := vertex{x: i}
		if r.square {
			v = vertex{x: i % r.cols, y: i / r.cols}
		}
		r.setValue(v, h)
	}
	r.version++
}
