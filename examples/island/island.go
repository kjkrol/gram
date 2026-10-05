package island

import (
	"cmp"
	"math"
	"slices"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/network"
	"github.com/kjkrol/gram/plugins/board/water"
)

// Layout draws the island over grid — a GridWidth x GridHeight square grid of CellSize cells: a
// wavy ellipse of land in a sea, a range of mountains along it with sharp peaks and spurs, a
// plateau at its western end, lowland by the coast, and the stops, a ring of them on the lowland
// round the range; along the coast, stretches of cliff, most in the north; streams and rivers
// running down from the heights to the sea (plugins/board/water), roads from stop to stop and
// bridges over what they cross (plugins/board/network). What is high and what is low is the heights
// alone; the ground is earth, sand or rock as they and the coast say — see soil. It gives the
// board's Layout, the heights for a topography to seed, and the stops.
func Layout(grid grid.Grid) (board.Layout, func(geom.Vec) float64, []cell.ID) {
	cellAt := func(x, y int) cell.ID { c, _ := grid.CellIndex(uint32(x), uint32(y)); return c }
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

	land := map[cell.ID]bool{}
	for y := range GridHeight {
		for x := range GridWidth {
			if within(float64(x)+0.5, float64(y)+0.5) {
				land[cellAt(x, y)] = true
			}
		}
	}
	// the sea along the coast, which how far inland a point lies is measured from
	var shore []geom.Vec
	for y := range GridHeight {
		for x := range GridWidth {
			if land[cellAt(x, y)] {
				continue
			}
			for _, d := range [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
				if land[cellAt(x+d[0], y+d[1])] {
					shore = append(shore, geom.NewVec(float64(x)+0.5, float64(y)+0.5))
					break
				}
			}
		}
	}
	high := make([]float64, len(shore)) // how high the cliff stands over each stretch of it
	for i, s := range shore {
		high[i] = cliffs(s.X-cx, s.Y-cy)
	}
	// coast is how far inland (x, y) lies, in cells, and how high the cliffs of the coast round it
	// stand: the nearer the coast, the more it counts, so the cliffs' heights blend inland
	coast := func(x, y float64) (in, cliff float64) {
		in = math.Inf(1)
		for _, s := range shore {
			in = min(in, math.Hypot(x-s.X, y-s.Y))
		}
		spread := 1 + 0.5*in
		sum, weight := 0.0, 0.0
		for i, s := range shore {
			d := math.Hypot(x-s.X, y-s.Y) - in
			w := math.Exp(-d * d / (2 * spread * spread))
			sum, weight = sum+w*high[i], weight+w
		}
		return in - 0.5, sum / weight
	}
	inland := func(x, y float64) float64 {
		d := math.Inf(1)
		for _, s := range shore {
			d = min(d, math.Hypot(x-s.X, y-s.Y))
		}
		return d - 0.5
	}

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
	ground := func(p geom.Vec) float64 {
		if !ashore(p) {
			return 0
		}
		x, y := p.X/cw, p.Y/ch
		in, high := coast(x, y)
		// the lowland rises gently from the sea; a cliff stands its full height a few cells back
		// from it, and sinks inland more slowly than the lowland rises, so the land behind it drains
		// over it
		plain := coastRise * min(max(in-1, 0), coastPlain)
		cliff := high * rise(in, 0.6) * (1 - rise(in-cliffBack, cliffFall))
		h := landHeight + plain + cliff + rise(in-1.5, coastWidth)*relief(x-cx, y-cy)
		top := plateau(x-cx, y-cy)
		return h*(1-top) + max(h, landHeight+plateauHeight)*top
	}
	// the rain runs off it to the sea in streams and rivers, cutting their channels
	rivers, err := water.Drain(grid, ground, func(c cell.ID) bool { return !land[c] }, water.Config{
		BrookAt: brookAt, StreamAt: streamAt, RiverAt: riverAt,
		Rain:       func(level float64) float64 { return 1 + level/100 }, // more on the heights
		BrookDepth: 1, StreamDepth: 2, RiverDepth: 5, FordEvery: fordEvery, FordSlope: 0.15,
		WidthPerRoot: widthPerRoot, Meander: meander,
	})
	if err != nil {
		panic(err)
	}
	heights := rivers.Carved(ground)

	var cells []cell.Entry
	soils := map[cell.ID]string{}
	levels := map[cell.ID]float64{} // the mean of each land cell's corners
	for y := range GridHeight {
		for x := range GridWidth {
			if !land[cellAt(x, y)] {
				continue
			}
			var hs [4]float64
			for k, d := range [4][2]int{{0, 0}, {1, 0}, {0, 1}, {1, 1}} {
				hs[k] = heights(geom.NewVec(float64(x+d[0])*cw, float64(y+d[1])*ch))
			}
			fx, fy := float64(x)+0.5, float64(y)+0.5
			c := cellAt(x, y)
			soils[c] = soil(hs, cw, inland(fx, fy), fx, fy)
			levels[c] = (hs[0] + hs[1] + hs[2] + hs[3]) / 4
			cells = append(cells, cell.Entry{Kind: soils[c], Cell: c})
		}
	}
	// running water crosses the ground as a band down the middle of the cell
	streams := rivers.Net(courses)
	ways := streams.Ways()

	// The stops: a hexagon on the lowland, none at the ends of the range, each opposite one across
	// it, each on the nearest ground that is neither rock nor water.
	var stops []cell.ID
	for k := range Stops {
		a := (float64(k) + 0.5) * 2 * math.Pi / Stops
		dx, dy := math.Cos(a)*islandRX, math.Sin(a)*islandRY
		r := stopsAt * edge(dx, dy)
		sx, sy := int(cx+r*dx), int(cy+r*dy)
	search:
		for ring := 0; ring < 8; ring++ {
			for oy := -ring; oy <= ring; oy++ {
				for ox := -ring; ox <= ring; ox++ {
					at := cellAt(sx+ox, sy+oy)
					if s := soils[at]; (s == EarthCell || s == SandCell) && rivers.Courses[at] == water.Dry {
						sx, sy = sx+ox, sy+oy
						break search
					}
				}
			}
		}
		stops = append(stops, cellAt(sx, sy))
	}

	// Roads run from stop to stop round the hexagon, the cheapest way over the ground: along the
	// lowland rather than up and down, round the rock where they can, never out to sea, on along a
	// road laid already rather than beside it; a bridge carries a road over whatever water runs
	// across it.
	roads := network.New(grid)
	passable := func(c cell.ID) bool { return land[c] }
	cost := func(a, b cell.ID) float64 {
		if !passable(b) {
			return math.Inf(1)
		}
		ax, ay, _ := grid.Coords(a)
		bx, by, _ := grid.Coords(b)
		step := 1.0
		if ax != bx && ay != by {
			// slantwise only between two cells a road may take, and never past water: a road
			// crosses it square, over a bridge
			l, r := cellAt(int(ax), int(by)), cellAt(int(bx), int(ay))
			if !passable(l) || !passable(r) || rivers.Courses[l] != water.Dry || rivers.Courses[r] != water.Dry {
				return math.Inf(1)
			}
			step = math.Sqrt2
		}
		c := step * (1 + roadClimb*math.Abs(levels[b]-levels[a])/cw)
		if rivers.Courses[b] != water.Dry {
			c += roadBridge
		}
		if soils[b] == RockCell {
			c *= roadRock
		}
		if _, laid := roads.Node(b); laid {
			c *= roadReuse
		}
		return c
	}
	for k, from := range stops {
		if path, ok := network.Route(grid, from, stops[(k+1)%len(stops)], cost); ok {
			roads.Path(path, network.Node{Kind: RoadCell, Width: roadWidth})
		}
	}
	roadWays, bridges := roads.Across(streams, BridgeCell)
	ways = append(ways, roadWays...)
	return board.Layout{Default: WaterCell, Cells: cells, Ways: ways, Crossings: bridges}, heights, stops
}

// courses are the kinds of the ways running water lays across the ground.
var courses = map[water.Course]string{water.Brook: BrookCell, water.Stream: StreamCell, water.River: RiverCell, water.Ford: FordCell}

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
		return RockCell
	case in <= beachWidth && top < landHeight+lowlandRoll:
		switch n := fbm(x/6+50, y/6+20); {
		case n > 0.52:
			return SandCell
		case n < 0.38:
			return RockCell
		}
	case steep < duneSlope && top < landHeight+2*lowlandRoll && fbm(x/5+31, y/5+17) > 0.57:
		return SandCell
	}
	return EarthCell
}

// relief is how high the ground stands over the lowland at (x, y) cells from the island's middle:
// the range with its peaks, cut by ridges and valleys, and the lowland rolling.
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
	// the lowland rolls, in small hummocks and in broad swells whose hollows gather the water
	return h + lowlandRoll*fbm(x/4+7, y/4+3) + lowlandSwell*fbm(x/9+21, y/9+13)
}

// plateau is how much of (x, y) cells from the island's middle lies on the plateau's flat top: 1
// on it, 0 off it, between on its edge, which is ragged and steep.
func plateau(x, y float64) float64 {
	a := math.Atan2(y-plateauY, x-plateauX)
	r := plateauR + 2.5*(fbm(3*math.Cos(a)+11, 3*math.Sin(a)+5)-0.5)
	return rise(r-math.Hypot(x-plateauX, y-plateauY), plateauEdge)
}

// Plateau is the cells of the plateau's flat top, nearest its middle first: high ground with a
// steep, ragged edge all round, at the range's western end.
func Plateau(grid grid.Grid) []cell.ID {
	cx, cy := float64(GridWidth)/2, float64(GridHeight)/2
	type at struct {
		c cell.ID
		d float64
	}
	var top []at
	for y := range GridHeight {
		for x := range GridWidth {
			dx, dy := float64(x)+0.5-cx, float64(y)+0.5-cy
			if plateau(dx, dy) < 1 {
				continue
			}
			if c, ok := grid.CellIndex(uint32(x), uint32(y)); ok {
				top = append(top, at{c, math.Hypot(dx-plateauX, dy-plateauY)})
			}
		}
	}
	slices.SortStableFunc(top, func(a, b at) int { return cmp.Compare(a.d, b.d) })
	cells := make([]cell.ID, len(top))
	for i, t := range top {
		cells[i] = t.c
	}
	return cells
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

// Metres is how many metres one of the island's heights stands for (Layout's heights, in its own
// units, 0 at the sea up to about 250): its highest peak rises about 2 km, steep over an island of
// 100 m cells. A game with a scale (world.Scale) seeds the heights times Scale.Units(Metres).
const Metres = 8.0

// The island's grid: GridWidth x GridHeight cells of CellSize world units, and how many Stops ring
// the range.
const (
	GridWidth  = 96
	GridHeight = 64
	CellSize   = 32
	Stops      = 6
)

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
	plateauHeight      = 110.0
	lowlandRoll        = 8.0
	lowlandSwell       = 24.0
	// the highest sea cliffs over the land's height, standing full cliffBack cells inland and
	// sinking over cliffFall more
	cliffHeight = 70.0
	cliffBack   = 3.0
	cliffFall   = 40.0
	// the lowland rises coastRise a cell from the sea, for coastPlain cells
	coastRise  = 2.0
	coastPlain = 15.0
	stopsAt    = 0.78 // how far out to the coast the stops lie
	// how wide a road runs; how much a road's cost grows with the climb, rise over run; what a
	// bridge adds to it; and how much of it a road laid already costs
	roadWidth  = 7.0
	roadClimb  = 6.0
	roadBridge = 4.0
	roadRock   = 8.0
	roadReuse  = 0.3
	// how much rain gathered makes a brook, a stream and a river, every how many cells from its
	// mouth a ford crosses a river, and how wide a course runs by the square root of its water
	brookAt      = 30.0
	streamAt     = 60.0
	riverAt      = 170.0
	fordEvery    = 8
	widthPerRoot = 1.4
	meander      = 6.0 // how far the draining nudges a cell's level, so courses wander
	// rock stands where the ground rises rockSlope across a cell or tops rockHeight; sand lies
	// within beachWidth cells of the sea, and in dunes where the lowland is flatter than duneSlope
	rockSlope  = 0.5
	rockHeight = 140.0
	beachWidth = 1.5
	duneSlope  = 0.2
)
