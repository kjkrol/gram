package relief

import (
	"slices"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/ground"
)

// KeepRoads runs shape and takes back all it did when it left a road lying where Transport Tycoon
// would not lay it (ground.RoadFits; ground.RoadJoins between level cells) that lay well before,
// and reports whether it kept it.
func (r *Relief) KeepRoads(shape func()) bool {
	if r.board == nil {
		shape()
		return true
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
		return true
	}
	for _, c := range kept {
		if !r.roadFits(c, r.board.Way(c).Links) {
			r.restore(before)
			return false
		}
	}
	return true
}

// roadFits reports whether a road running out of c as links say lies as Transport Tycoon lays it.
func (r *Relief) roadFits(c cell.ID, links cell.Links) bool {
	if !r.square {
		for i := range r.grid.Directions() {
			nb, ok := r.grid.Toward(c, i)
			if ok && links.Has(i) && !ground.RoadJoins(r.Altitude(c), r.Altitude(nb)) {
				return false
			}
		}
		return true
	}
	var buf [4]geom.Vec
	var corners [4]float64
	for k, p := range r.grid.CellOutline(c, buf[:0]) {
		corners[k] = r.GroundAt(p)
	}
	return ground.RoadFits(r.grid, c, corners[:], links, r.sq.Cell)
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
