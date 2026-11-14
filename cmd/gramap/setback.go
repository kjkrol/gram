package main

import (
	"maps"
	"math"
	"slices"

	"github.com/kjkrol/gram/plugins/board/water"
)

// metres: how far off a road's edge or a water's bank a building's wall stands at least, the
// narrowest a building is left, and how far one too narrow to be cut steps aside
const (
	setbackClear = 1.0
	setbackLeast = 4.0
	setbackAside = 6.0
)

// a lane a building keeps off, a stretch of road or of water: from a to b, half as wide as
// half, world units; a point where a is b
type lane struct {
	a, b [2]float64
	half float64
	// a roundabout's island: free within this far of its middle
	hollow float64
}

// setback has no building stand on a road or on water: a wall standing in a road's way or
// over a bed draws back to its edge, the building as much narrower, and one left narrower
// than a building is is left out. A road is a cell's and runs by its cell's axis, a bed a
// cell wide, where OpenStreetMap's buildings stand where they are.
// lanes are the stretches of road and of water nothing stands in, each clear metres wider
// either side, by the cells they reach
func (w *chart) lanes(clear float64) map[[2]int][]lane {
	s := w.roads.streets
	lanes := map[[2]int][]lane{}
	add := func(t lane) {
		x0, x1 := min(t.a[0], t.b[0])-t.half, max(t.a[0], t.b[0])+t.half
		y0, y1 := min(t.a[1], t.b[1])-t.half, max(t.a[1], t.b[1])+t.half
		for y := int(math.Floor(y0 / w.size)); y <= int(math.Floor(y1/w.size)); y++ {
			for x := int(math.Floor(x0 / w.size)); x <= int(math.Floor(x1/w.size)); x++ {
				lanes[[2]int{x, y}] = append(lanes[[2]int{x, y}], t)
			}
		}
	}
	clear /= w.perUnit
	if s != nil {
		for _, st := range s.legs {
			half := s.width(st.lanes)/2 + clear
			for i := 1; i < len(st.cells); i++ {
				a, b := s.lot(st.cells[i-1]).at, s.lot(st.cells[i]).at
				add(lane{
					a:    [2]float64{a[0] * w.size, a[1] * w.size},
					b:    [2]float64{b[0] * w.size, b[1] * w.size},
					half: half,
				})
			}
		}
		for _, c := range slices.Sorted(maps.Keys(s.rounds)) {
			at := s.lot(c).at
			p := [2]float64{at[0] * w.size, at[1] * w.size}
			// its roadway from its rim in, a lane 3.5 m as it is drawn; the island free
			rim := s.rim(c)
			ring := 3.5 * float64(max(s.lot(c).lanes, 1))
			add(lane{a: p, b: p, half: rim/w.perUnit + clear,
				hollow: max((rim-ring)/w.perUnit-clear, 0)})
		}
	}
	for _, c := range slices.Sorted(maps.Keys(w.rivers.Courses)) {
		if w.rivers.Courses[c] == water.Dry {
			continue
		}
		a, b := w.grid.CellCenter(c), w.grid.CellCenter(w.rivers.Down[c])
		add(lane{
			a:    [2]float64{float64(a.X), float64(a.Y)},
			b:    [2]float64{float64(b.X), float64(b.Y)},
			half: float64(w.rivers.Width(c, float32(w.size)))/2 + clear,
		})
	}
	return lanes
}

// off is p moved off every lane it stands in, out to the lane's edge or in onto a
// roundabout's island, whichever is nearer; false where it comes to rest in none
func (w *chart) off(lanes map[[2]int][]lane, p [2]float64) ([2]float64, bool) {
	for range 4 {
		moved := false
		cx, cy := int(math.Floor(p[0]/w.size)), int(math.Floor(p[1]/w.size))
		for _, t := range lanes[[2]int{cx, cy}] {
			dx, dy := t.b[0]-t.a[0], t.b[1]-t.a[1]
			u := 0.0
			if l := dx*dx + dy*dy; l > 0 {
				u = min(max(((p[0]-t.a[0])*dx+(p[1]-t.a[1])*dy)/l, 0), 1)
			}
			qx, qy := t.a[0]+dx*u, t.a[1]+dy*u
			d := math.Hypot(p[0]-qx, p[1]-qy)
			if d >= t.half || d <= t.hollow {
				continue
			}
			to := t.half * 1.001
			if t.hollow > 0 && d-t.hollow < t.half-d {
				to = t.hollow * 0.999
			}
			if d < 1e-6 {
				p = [2]float64{qx + to, qy}
			} else {
				p = [2]float64{qx + (p[0]-qx)/d*to, qy + (p[1]-qy)/d*to}
			}
			moved = true
		}
		if !moved {
			return p, true
		}
	}
	return p, false
}

func (w *chart) setback() (narrowed, gone int) {
	lanes := w.lanes(setbackClear)
	least, aside := setbackLeast/w.perUnit, setbackAside/w.perUnit
	kept := w.buildings[:0]
	for _, b := range w.buildings {
		was := b.size
		reach := math.Hypot(b.size[0], b.size[1]) / 2
		at := func(v float64) int { return int(math.Floor(v / w.size)) }
		x0, x1 := at(b.mid[0]-reach), at(b.mid[0]+reach)
		y0, y1 := at(b.mid[1]-reach), at(b.mid[1]+reach)
		// narrower first; one too narrow for that steps aside whole, then narrows by what it
		// stepped into
		stands, moved := true, false
		each := func(step bool) {
			for y := y0; y <= y1 && stands; y++ {
				for x := x0; x <= x1 && stands; x++ {
					for _, t := range lanes[[2]int{x, y}] {
						back, off, ok := b.drawBack(t, least)
						if !ok && step && back <= aside {
							b.mid[0] += back * off[0]
							b.mid[1] += back * off[1]
							moved, ok = true, true
						}
						if stands = ok; !stands {
							break
						}
					}
				}
			}
		}
		each(true)
		if moved && stands {
			each(false)
		}
		switch {
		case !stands:
			gone++
			continue
		case b.size != was || moved:
			narrowed++
		}
		kept = append(kept, b)
	}
	w.buildings = kept
	return narrowed, gone
}

// drawBack draws the building's wall back off the lane t where it stands in it, the building
// as much narrower along its side lying more across the lane; false where that leaves it
// narrower than least or the lane runs through its middle
func (b *built) drawBack(t lane, least float64) (back float64, off [2]float64, ok bool) {
	sin, cos := math.Sincos(b.turn * math.Pi / 180)
	axes := [2][2]float64{{cos, sin}, {-sin, cos}}
	dx, dy := t.b[0]-t.a[0], t.b[1]-t.a[1]
	long := math.Hypot(dx, dy)
	// the lane's way and the way off it towards the building's middle
	var u, n [2]float64
	mx, my := b.mid[0]-t.a[0], b.mid[1]-t.a[1]
	if long > 0 {
		u = [2]float64{dx / long, dy / long}
		n = [2]float64{-u[1], u[0]}
		if mx*n[0]+my*n[1] < 0 {
			n = [2]float64{-n[0], -n[1]}
		}
	} else if d := math.Hypot(mx, my); d > 0 {
		n = [2]float64{mx / d, my / d}
		u = [2]float64{-n[1], n[0]}
	} else {
		return math.Inf(1), n, false
	}
	// how far along the lane the building reaches, and how near it its nearest corner comes
	lo, hi, near := math.Inf(1), math.Inf(-1), math.Inf(1)
	for _, sx := range [2]float64{-1, 1} {
		for _, sy := range [2]float64{-1, 1} {
			px := mx + sx*axes[0][0]*b.size[0]/2 + sy*axes[1][0]*b.size[1]/2
			py := my + sx*axes[0][1]*b.size[0]/2 + sy*axes[1][1]*b.size[1]/2
			along := px*u[0] + py*u[1]
			lo, hi = min(lo, along), max(hi, along)
			near = min(near, px*n[0]+py*n[1])
		}
	}
	if hi < 0 || lo > long || near >= t.half {
		return 0, n, true
	}
	back = t.half - near
	// the side lying more across the lane gives way
	k := 0
	if math.Abs(axes[1][0]*n[0]+axes[1][1]*n[1]) > math.Abs(axes[0][0]*n[0]+axes[0][1]*n[1]) {
		k = 1
	}
	across := axes[k][0]*n[0] + axes[k][1]*n[1]
	cut := back / math.Abs(across)
	if b.size[k]-cut < least {
		return back, n, false
	}
	b.size[k] -= cut
	shift := cut / 2 * across / math.Abs(across)
	b.mid[0] += axes[k][0] * shift
	b.mid[1] += axes[k][1] * shift
	return back, n, true
}
