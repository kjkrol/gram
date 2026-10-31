package relief

import (
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/internal/f32"
	"github.com/kjkrol/gram/plugins/board/cell"
)

// keeps float32 heights from chasing a slope limit they cannot store exactly
const shapeTolerance = 1e-3

// Lift moves the ground at the corner nearest p by `by`, then the ground round it until no two
// corners along a cell's edge differ by more than maxStep (zero: any).
func (r *Relief) Lift(p geom.Vec, by, maxStep float32) {
	v, ok := r.vertexAt(p)
	if !ok || by == 0 {
		return
	}
	if r.setValue(v, r.heightAt(v)+by) {
		r.settle([]vertex{v}, maxStep)
		r.changed()
	}
}

// Flatten brings the corners nearest from and to and those between them to the height of the one
// nearest from, then the ground round them within maxStep, as Lift.
func (r *Relief) Flatten(from, to geom.Vec, maxStep float32) {
	v0, ok := r.vertexAt(from)
	if !ok {
		return
	}
	h := r.heightAt(v0)
	var set []vertex
	if !r.square {
		r.hexCornersIn(r.hexPoint(v0), r.hexPoint(r.nearestCorner(to)), func(v vertex) {
			if r.setValue(v, h) {
				set = append(set, v)
			}
		})
	} else {
		x0, y0 := r.lattice(from)
		x1, y1 := r.lattice(to)
		for y := min(y0, y1); y <= max(y0, y1); y++ {
			for x := min(x0, x1); x <= max(x0, x1); x++ {
				if v, ok := r.corner(x, y); ok && r.setValue(v, h) {
					set = append(set, v)
				}
			}
		}
	}
	if len(set) > 0 {
		r.settle(set, maxStep)
		r.changed()
	}
}

// Level lays the cell c flat at h, then the ground round it within maxStep, as Lift.
func (r *Relief) Level(c cell.ID, h, maxStep float32) {
	var set []vertex
	if !r.square {
		vs, n, _ := r.corners(c)
		for _, v := range vs[:n] {
			if r.setValue(v, h) {
				set = append(set, v)
			}
		}
	} else {
		var buf [4]geom.Vec
		for _, p := range r.grid.CellOutline(c, buf[:0]) {
			if v, ok := r.corner(r.lattice(p)); ok && r.setValue(v, h) {
				set = append(set, v)
			}
		}
	}
	if len(set) > 0 {
		r.settle(set, maxStep)
		r.changed()
	}
}

func (r *Relief) settle(queue []vertex, maxStep float32) {
	if maxStep <= 0 {
		return
	}
	var ns []vertex
	for len(queue) > 0 {
		v := queue[0]
		queue = queue[1:]
		h := r.heightAt(v)
		ns = r.vertexNeighbours(v, ns[:0])
		for _, n := range ns {
			hn, want := r.heightAt(n), float32(0)
			switch {
			case hn < h-maxStep-shapeTolerance:
				want = h - maxStep
			case hn > h+maxStep+shapeTolerance:
				want = h + maxStep
			default:
				continue
			}
			if r.setValue(n, want) {
				queue = append(queue, n)
			}
		}
	}
}

// unfolded
func (r *Relief) lattice(p geom.Vec) (x, y int64) {
	w, h := r.grid.CellBounds()
	return int64(f32.Round(p.X / w)), int64(f32.Round(p.Y / h))
}

func (r *Relief) vertexAt(p geom.Vec) (vertex, bool) {
	if !r.square {
		v := r.nearestCorner(p)
		_, ok := r.hexIndex(v)
		return v, ok
	}
	return r.corner(r.lattice(p))
}

func (r *Relief) heightAt(v vertex) float32 { return r.values[r.index(v)] }

// the corners along the cells' edges from v
func (r *Relief) vertexNeighbours(v vertex, dst []vertex) []vertex {
	if !r.square {
		for _, d := range hexSpokes[2-class(v.x, v.y)] {
			n := vertex{v.x + d[0], v.y + d[1]}
			if _, ok := r.hexIndex(n); ok {
				dst = append(dst, n)
			}
		}
		return dst
	}
	for _, d := range [4][2]int64{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
		if n, ok := r.corner(int64(v.x)+d[0], int64(v.y)+d[1]); ok {
			dst = append(dst, n)
		}
	}
	return dst
}
