package main

import (
	"math"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugins/board"
)

// islandLayout draws a fixed island: a wavy ellipse of land in a sea, a range of mountains along
// it with sharp peaks and spurs, a plateau at its western end, lowland by the coast, and the
// stops, a ring of them on the lowland round the range; along the coast, stretches of cliff, most
// in the north. What is high and what is low is the heights alone; the ground is earth, sand or
// rock as they and the coast say — see soil.
func islandLayout(grid board.Grid) (board.Layout, []board.CellID) {
	cell := func(x, y int) board.CellID { c, _ := grid.CellIndex(uint32(x), uint32(y)); return c }
	cx, cy := float64(GridWidth)/2, float64(GridHeight)/2
	// edge is how far the coast lies from the middle, as a share of the ellipse, the way (dx, dy) goes
	edge := func(dx, dy float64) float64 {
		a := math.Atan2(dy, dx)
		return 1 + 0.12*math.Sin(3*a+1) + 0.08*math.Sin(5*a+2) + 0.05*math.Sin(7*a)
	}
	within := func(x, y float64) bool {
		dx, dy := x-cx, y-cy
		r := edge(dx, dy)
		return (dx*dx)/(islandRX*islandRX)+(dy*dy)/(islandRY*islandRY) <= r*r
	}

	land := map[board.CellID]bool{}
	for y := range GridHeight {
		for x := range GridWidth {
			if within(float64(x)+0.5, float64(y)+0.5) {
				land[cell(x, y)] = true
			}
		}
	}
	// the sea along the coast, which how far inland a point lies is measured from
	var shore []geom.Vec
	for y := range GridHeight {
		for x := range GridWidth {
			if land[cell(x, y)] {
				continue
			}
			for _, d := range [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
				if land[cell(x+d[0], y+d[1])] {
					shore = append(shore, geom.NewVec(float64(x)+0.5, float64(y)+0.5))
					break
				}
			}
		}
	}
	// coast is how far inland (x, y) lies, in cells, and the nearest sea
	coast := func(x, y float64) (float64, geom.Vec) {
		d, at := math.Inf(1), geom.Vec{}
		for _, s := range shore {
			if e := math.Hypot(x-s.X, y-s.Y); e < d {
				d, at = e, s
			}
		}
		return d - 0.5, at
	}
	inland := func(x, y float64) float64 { d, _ := coast(x, y); return d }

	cw, ch := grid.CellBounds()
	// the land stands a little above the sea: a corner is raised where every cell round it is land,
	// so the shore slopes down into water that stays level
	ashore := func(p geom.Vec) bool {
		const eps = 1e-6
		for _, d := range [4][2]float64{{-eps, -eps}, {eps, -eps}, {-eps, eps}, {eps, eps}} {
			c, ok := grid.CellAt(geom.NewVec(p.X+d[0], p.Y+d[1]))
			if !ok || !land[c] {
				return false
			}
		}
		return true
	}
	heights := func(p geom.Vec) float64 {
		if !ashore(p) {
			return 0
		}
		x, y := p.X/cw, p.Y/ch
		in, sea := coast(x, y)
		// a cliff stands its full height a few cells back from the sea, then gives way to the relief
		cliff := cliffs(sea.X-cx, sea.Y-cy) * rise(in, 0.6) * (1 - rise(in-cliffBack, cliffFall))
		return landHeight + max(cliff, rise(in-1.5, coastWidth)*relief(x-cx, y-cy))
	}

	var cells []board.CellEntry
	soils := map[board.CellID]string{}
	for y := range GridHeight {
		for x := range GridWidth {
			if !land[cell(x, y)] {
				continue
			}
			var hs [4]float64
			for k, d := range [4][2]int{{0, 0}, {1, 0}, {0, 1}, {1, 1}} {
				hs[k] = heights(geom.NewVec(float64(x+d[0])*cw, float64(y+d[1])*ch))
			}
			fx, fy := float64(x)+0.5, float64(y)+0.5
			soils[cell(x, y)] = soil(hs, cw, inland(fx, fy), fx, fy)
			cells = append(cells, board.CellEntry{Kind: soils[cell(x, y)], Cell: cell(x, y)})
		}
	}

	// The stops: a hexagon on the lowland, none at the ends of the range, each opposite one across
	// it, each on the nearest ground that is not rock.
	var stops []board.CellID
	for k := range UnitCount {
		a := (float64(k) + 0.5) * 2 * math.Pi / UnitCount
		dx, dy := math.Cos(a)*islandRX, math.Sin(a)*islandRY
		r := stopsAt * edge(dx, dy)
		sx, sy := int(cx+r*dx), int(cy+r*dy)
	search:
		for ring := 0; ring < 8; ring++ {
			for oy := -ring; oy <= ring; oy++ {
				for ox := -ring; ox <= ring; ox++ {
					if s := soils[cell(sx+ox, sy+oy)]; s == "earth" || s == "sand" {
						sx, sy = sx+ox, sy+oy
						break search
					}
				}
			}
		}
		stops = append(stops, cell(sx, sy))
	}
	return board.Layout{Default: "water", Cells: cells, Heights: heights}, stops
}

// soil is the ground of a cell whose corners stand at hs, w wide, in cells from the coast, at
// (x, y): rock where it is steep or high, sand on the lowland by the sea and in dunes, earth
// elsewhere. Along the coast, beaches, rocky shore and earth take turns as a noise says.
func soil(hs [4]float64, w, in, x, y float64) string {
	top, steep := 0.0, 0.0
	for k, a := range hs {
		top = max(top, a)
		for _, b := range hs[k+1:] {
			steep = max(steep, math.Abs(a-b))
		}
	}
	steep /= w // a diagonal counted as an edge: a little steeper than it is
	switch {
	case steep >= rockSlope || top >= rockHeight:
		return "rock"
	case in <= beachWidth && top < landHeight+lowlandRoll:
		switch n := fbm(x/6+50, y/6+20); {
		case n > 0.52:
			return "sand"
		case n < 0.38:
			return "rock"
		}
	case steep < duneSlope && top < landHeight+2*lowlandRoll && fbm(x/5+31, y/5+17) > 0.57:
		return "sand"
	}
	return "earth"
}

// relief is how high the ground stands over the lowland at (x, y) cells from the island's middle:
// the range with its peaks, cut by ridges and valleys, the plateau flat on top, and the lowland
// rolling a little.
func relief(x, y float64) float64 {
	// the range: a crest along a bent line, highest in the middle, falling away to either side
	t := min(max((x-rangeFrom)/(rangeTo-rangeFrom), 0), 1)
	across := y - ridgeY(rangeFrom+t*(rangeTo-rangeFrom))
	along := max(rangeFrom-x, x-rangeTo, 0)
	h := rangeHeight * (1 - 0.35*math.Abs(2*t-1)) * sharp(math.Hypot(along, across)/rangeWidth, 1.3)
	// ridges and valleys running down from the crest, and the peaks standing clear over it all
	cut := ridged(x/5, y/5)
	h *= 0.55 + 0.45*cut
	for _, k := range peaks {
		h = smoothMax(h, k.h*(0.85+0.15*cut)*sharp(math.Hypot(x-k.x, y-ridgeY(k.x)-k.y)/k.r, 1.8))
	}
	// the plateau: a flat top, its edge ragged and steep
	a := math.Atan2(y-plateauY, x-plateauX)
	r := plateauR + 2.5*(fbm(3*math.Cos(a)+11, 3*math.Sin(a)+5)-0.5)
	top := rise(r-math.Hypot(x-plateauX, y-plateauY), plateauEdge)
	h = h*(1-top) + max(h, plateauHeight)*top
	return h + (1-top)*lowlandRoll*fbm(x/4+7, y/4+3)
}

// smoothMax is the larger of a and b, rounded where they are near: two slopes meet in a saddle,
// not a crease.
func smoothMax(a, b float64) float64 {
	const k = 0.08
	m := max(a, b)
	return m + math.Log(math.Exp(k*(a-m))+math.Exp(k*(b-m)))/k
}

// ridged is noise in [0, 1] made of sharp crests: three octaves of folded value noise.
func ridged(x, y float64) float64 {
	sum, amp, norm := 0.0, 1.0, 0.0
	for range 3 {
		n := 1 - math.Abs(2*valueNoise(x, y)-1)
		sum += amp * n * n
		norm += amp
		x, y, amp = 2*x+13, 2*y+7, amp/2
	}
	return sum / norm
}

// cliffs is how high the cliff stands over the lowland where the coast is at (x, y) cells from the
// island's middle: stretches of cliff, most along the north coast, and none between them.
func cliffs(x, y float64) float64 {
	north := min(max(-y/islandRY, -1), 1)
	along := rise(fbm(x/8+90, y/8+40)+0.2*north-0.52, 0.08)
	return along * cliffHeight * (0.6 + 0.4*fbm(x/10+60, y/10+10))
}

// ridgeY is where across the island the range's crest runs at x cells from the middle.
func ridgeY(x float64) float64 { return -0.08*x + 3*math.Sin(x/9) }

// sharp is 1 at 0 falling to 0 at 1 and beyond, the steeper towards the top the larger p: a crest,
// a peak.
func sharp(s, p float64) float64 { return math.Pow(max(1-s, 0), p) }

// rise is 0 below 0 and 1 from w up, easing between the two: a slope w cells wide.
func rise(v, w float64) float64 {
	t := min(max(v/w, 0), 1)
	return t * t * (3 - 2*t)
}

// fbm is smooth noise in [0, 1): three octaves of value noise.
func fbm(x, y float64) float64 {
	return (valueNoise(x, y) + 0.5*valueNoise(2*x, 2*y) + 0.25*valueNoise(4*x, 4*y)) / 1.75
}

func valueNoise(x, y float64) float64 {
	x0, y0 := math.Floor(x), math.Floor(y)
	u, v := x-x0, y-y0
	u, v = u*u*(3-2*u), v*v*(3-2*v)
	ix, iy := int64(x0), int64(y0)
	a, b := lattice(ix, iy), lattice(ix+1, iy)
	c, d := lattice(ix, iy+1), lattice(ix+1, iy+1)
	return (a*(1-u)+b*u)*(1-v) + (c*(1-u)+d*u)*v
}

// lattice is a fixed random number in [0, 1) for the lattice point (x, y).
func lattice(x, y int64) float64 {
	h := uint64(x)*0x9E3779B97F4A7C15 ^ uint64(y)*0xC2B2AE3D27D4EB4F ^ 0x5DEECE66D
	h ^= h >> 31
	h *= 0xBF58476D1CE4E5B9
	h ^= h >> 29
	return float64(h>>11) / (1 << 53)
}

// peaks rise over the range: x cells from the middle, y off the crest, h high, r across.
var peaks = []struct{ x, y, h, r float64 }{
	{-4, 0, 200, 9},
	{7, -1, 170, 8},
	{16, 0, 150, 8},
	{24, 1, 105, 6},
	{1, 8, 115, 6},   // a spur to the south
	{11, -8, 125, 6}, // and to the north
}

const (
	islandRX, islandRY = 34.0, 22.0
	landHeight         = 8.0
	coastWidth         = 5.0 // cells from the coast before the heights stand full
	// the range runs from rangeFrom to rangeTo cells from the middle, rangeHeight at its crest's
	// highest, reaching rangeWidth to either side
	rangeFrom, rangeTo = -13.0, 26.0
	rangeHeight        = 135.0
	rangeWidth         = 13.0
	// the plateau at its western end
	plateauX, plateauY = -21.0, 1.0
	plateauR           = 7.0
	plateauEdge        = 2.0
	plateauHeight      = 80.0
	lowlandRoll        = 8.0
	// the highest sea cliffs over the land's height, standing full cliffBack cells inland and
	// sinking over cliffFall more
	cliffHeight = 70.0
	cliffBack   = 3.0
	cliffFall   = 8.0
	stopsAt     = 0.78 // how far out to the coast the stops lie
	// rock stands where the ground rises rockSlope across a cell or tops rockHeight; sand lies
	// within beachWidth cells of the sea, and in dunes where the lowland is flatter than duneSlope
	rockSlope  = 0.5
	rockHeight = 140.0
	beachWidth = 1.5
	duneSlope  = 0.2
)
