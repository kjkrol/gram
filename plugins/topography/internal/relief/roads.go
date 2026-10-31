package relief

import (
	"slices"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/ground"
)

// KeepRoads runs shape and takes back all it did when it broke a road that lay well before
// (ground.RoadFits), reporting whether it kept it.
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

func (r *Relief) roadFits(c cell.ID, links cell.Links) bool {
	if !r.square {
		corners, _, _ := r.Hex(c)
		return ground.RoadFits(r.grid, c, corners[:], links)
	}
	var buf [4]geom.Vec
	var corners [4]float32
	for k, p := range r.grid.CellOutline(c, buf[:0]) {
		corners[k] = r.GroundAt(p)
	}
	return ground.RoadFits(r.grid, c, corners[:], links)
}

func (r *Relief) restore(values []float32) {
	for i, h := range values {
		var v vertex
		if r.square {
			v = vertex{x: i % r.cols, y: i / r.cols}
		} else {
			v = r.hexVertex(i)
		}
		r.setValue(v, h)
	}
	r.changed()
}
