package relief

import (
	"math"
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/internal/f32"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/ground"
)

func near(a, b geom.Vec) bool {
	return f32.Abs(a.X-b.X) <= 1e-3 && f32.Abs(a.Y-b.Y) <= 1e-3
}

func TestHex_EveryCornerOnceTheCellsMeetingAtItSharingIt(t *testing.T) {
	for _, size := range [][2]uint32{{1, 1}, {4, 3}, {7, 5}} {
		g := grid.DefaultGrids{}.Hex(size[0], size[1], 16)
		r := onBoard(g)
		at := map[int]geom.Vec{}
		g.EachCell(func(c cell.ID) {
			q, row, _ := g.Coords(c)
			outline := g.CellOutline(c, nil)
			for k, i := range HexCorners(int(size[0]), int(q), int(row)) {
				if i < 0 || i >= len(r.values) {
					t.Fatalf("%v: corner %d of (%d, %d) at %d, outside the %d heights", size, k, q,
						row, i, len(r.values))
				}
				if p, ok := at[i]; ok && !near(p, outline[k]) {
					t.Errorf("%v: the height %d at %v and at %v", size, i, p, outline[k])
				}
				at[i] = outline[k]
			}
		})
		cornered := 0
		for i, is := range r.cornered {
			if _, ok := at[i]; ok != is {
				t.Errorf("%v: the height %d is a corner's: %v, marked so: %v", size, i, ok, is)
			}
			if is {
				cornered++
			}
		}
		// a corner a cell's top and one its top-right, and those of the board's rim
		w, h := int(size[0]), int(size[1])
		if want := 2*w*h + 2*w + 2*h; len(at) != want || cornered != want {
			t.Errorf("%v: %d corners of the cells, %d heights marked: want %d, one a corner", size,
				len(at), cornered, want)
		}
	}
}

func TestHex_SetHeightsSamplesTheCornersAndACellsLevelIsTheirMean(t *testing.T) {
	g := grid.DefaultGrids{}.Hex(6, 5, 16)
	r := onBoard(g)
	slope := func(p geom.Vec) float32 { return p.X/8 + p.Y/4 }
	r.SetHeights(slope)
	g.EachCell(func(c cell.ID) {
		corners, level, ok := r.Hex(c)
		if !ok {
			t.Fatalf("no corners of %v", c)
		}
		mean := float32(0)
		for k, p := range g.CellOutline(c, nil) {
			if want := slope(p); f32.Abs(corners[k]-want) > 1e-3 {
				t.Errorf("corner %d of %v stands at %v, want %v, the heights at it", k, c,
					corners[k], want)
			}
			mean += corners[k]
		}
		if mean /= 6; f32.Abs(level-mean) > f32.Eps(mean) || r.Altitude(c) != level {
			t.Errorf("%v's level %v, altitude %v, want the corners' mean %v", c, level,
				r.Altitude(c), mean)
		}
		if got := r.Corners(c); got != (Corners{level, level, level, level}) {
			t.Errorf("%v's Corners %v, want its level four times", c, got)
		}
	})
}

func TestHex_AMapsCellsMeetInTheirCorners(t *testing.T) {
	// far from the origin, where float32 points are coarse
	g := grid.DefaultGrids{}.Hex(160, 120, 16)
	r := onBoard(g)
	levels := func(c cell.ID) float32 {
		x, y, _ := g.Coords(c)
		return float32((x*7 + y*13) % 10)
	}
	r.SetHeights(ground.MeanOfCells(g, levels))
	for i := range r.values {
		if !r.cornered[i] {
			continue
		}
		sum, n := 0.0, 0.0
		r.touching(r.hexVertex(i), func(c cell.ID) { sum, n = sum+float64(levels(c)), n+1 })
		if want := float32(sum / n); r.values[i] != want {
			t.Fatalf("the corner at %v stands at %v, want %v, the mean of its %v cells",
				r.hexPoint(r.hexVertex(i)), r.values[i], want, n)
		}
	}
}

func TestHex_TheGroundRunsFromTheMiddleToTheCorners(t *testing.T) {
	g := grid.DefaultGrids{}.Hex(5, 4, 16)
	r := onBoard(g)
	rnd := rand.New(rand.NewPCG(1, 2))
	r.SetHeights(func(geom.Vec) float32 { return float32(30 * rnd.Float64()) })
	g.EachCell(func(c cell.ID) {
		corners, level, _ := r.Hex(c)
		middle := g.CellCenter(c)
		if got := r.GroundAt(middle); f32.Abs(got-level) > 1e-4 {
			t.Errorf("the ground in %v's middle is %v, want its level %v", c, got, level)
		}
		// on the board's edge the ground is either side's: the rim's corners and sides aside
		outline := g.CellOutline(c, nil)
		for k, p := range outline {
			if got := r.GroundAt(p); !byEdge(g, p) && f32.Abs(got-corners[k]) > 1e-4 {
				t.Errorf("the ground at %v's corner %d is %v, want %v", c, k, got, corners[k])
			}
			// halfway from the middle to the corner, and to the middle of the corner's side
			half := geom.NewVec((middle.X+p.X)/2, (middle.Y+p.Y)/2)
			if got, want := r.GroundAt(half), (level+corners[k])/2; f32.Abs(got-want) > 1e-4 {
				t.Errorf("halfway to %v's corner %d the ground is %v, want %v", c, k, got, want)
			}
			next := outline[(k+1)%6]
			side := geom.NewVec((p.X+next.X)/2, (p.Y+next.Y)/2)
			want := (corners[k] + corners[(k+1)%6]) / 2
			if got := r.GroundAt(side); !byEdge(g, side) && f32.Abs(got-want) > 1e-4 {
				t.Errorf("the middle of %v's side %d is %v, want %v", c, k, got, want)
			}
		}
	})
	for range 2000 {
		p := geom.NewVec(float32(150*rnd.Float64()), float32(110*rnd.Float64()))
		if _, ok := g.CellAt(p); !ok || byEdge(g, p) {
			continue
		}
		for _, d := range []geom.Vec{{X: 0.005}, {Y: 0.005}, {X: 0.004, Y: -0.004}} {
			q := geom.NewVec(p.X+d.X, p.Y+d.Y)
			if a, b := r.GroundAt(p), r.GroundAt(q); f32.Abs(a-b) > 0.2 {
				t.Fatalf("the ground at %v is %v and at %v %v: a step", p, a, q, b)
			}
		}
	}
	if got := r.GroundAt(geom.NewVec(-30, -30)); got != 0 {
		t.Errorf("the ground off the board is %v, want 0", got)
	}
}

func TestHex_AlongIsTheGroundAlongARay(t *testing.T) {
	g := grid.DefaultGrids{}.Hex(12, 9, 16)
	r := onBoard(g)
	rnd := rand.New(rand.NewPCG(3, 5))
	r.SetHeights(func(geom.Vec) float32 { return float32(40 * rnd.Float64()) })
	for trial := range 300 {
		origin := geom.NewVec(float32(420*rnd.Float64()), float32(240*rnd.Float64()))
		a := float32(2 * math.Pi * rnd.Float64())
		if trial%10 == 0 {
			a = math.Pi/6 + float32(trial/10%6)*math.Pi/3 // along the lattice's lines
		}
		if trial%10 == 1 {
			// from a middle through the corners and middles along a line of the lattice
			origin = g.CellCenter(g.At(uint32(rnd.IntN(12)), uint32(rnd.IntN(9))))
			a = float32(trial/10%6)*math.Pi/3 - math.Pi/2
		}
		dir := geom.NewVec(f32.Cos(a), f32.Sin(a))
		length := float32(30 + 300*rnd.Float64())
		ds, alts := []float32{0}, []float32{r.GroundAt(origin)}
		r.Along(origin, dir, length, func(d, alt float32) bool {
			if d < ds[len(ds)-1] {
				t.Fatalf("trial %d: the profile goes back from %.4f to %.4f",
					trial, ds[len(ds)-1], d)
			}
			ds, alts = append(ds, d), append(alts, alt)
			return true
		})
		if last := ds[len(ds)-1]; last != length {
			t.Fatalf("trial %d: the profile ends at %.4f, want %.4f", trial, last, length)
		}
		k := 1
		for i := range 4000 {
			d := length * float32(i) / 3999
			for k < len(ds)-1 && ds[k] < d {
				k++
			}
			// where the ground drops off the board a float32 point may round to either side, and
			// along the board's edge it lies on both
			p := geom.NewVec(origin.X+dir.X*d, origin.Y+dir.Y*d)
			if d-ds[k-1] < 1e-3 || ds[k]-d < 1e-3 || byEdge(g, p) {
				continue
			}
			want := r.GroundAt(p)
			got := alts[k]
			if span := ds[k] - ds[k-1]; span > 0 {
				got = alts[k-1] + (d-ds[k-1])/span*(alts[k]-alts[k-1])
			}
			// a float32 point along the ray is off by its rounding, the ground by as much times
			// its slope
			if f32.Abs(got-want) > 1e-4 {
				t.Fatalf("trial %d: %.3f along the ground is %.6f, the profile says %.6f",
					trial, d, want, got)
			}
		}
	}
}

// whether the board's edge runs within a hundredth of p
func byEdge(g grid.Grid, p geom.Vec) bool {
	_, in := g.CellAt(p)
	for _, d := range [4][2]float32{{-0.01, -0.01}, {0.01, -0.01}, {-0.01, 0.01}, {0.01, 0.01}} {
		if _, ok := g.CellAt(geom.NewVec(p.X+d[0], p.Y+d[1])); ok != in {
			return true
		}
	}
	return false
}

// sampled a cell apart, a ray along a row of corners would miss a peak at one of them
func TestHex_AlongMeetsAPeakAtACorner(t *testing.T) {
	g := grid.DefaultGrids{}.Hex(8, 6, 16)
	r := onBoard(g)
	c := g.At(3, 2)
	peak := g.CellOutline(c, nil)[1]
	r.Lift(peak, 25, 0)
	origin := g.CellCenter(g.At(1, 2))
	dir := geom.NewVec(peak.X-origin.X, peak.Y-origin.Y)
	far := f32.Hypot(dir.X, dir.Y)
	dir = geom.NewVec(dir.X/far, dir.Y/far)
	highest := float32(0)
	r.Along(origin, dir, 2*far, func(_, alt float32) bool {
		highest = max(highest, alt)
		return true
	})
	if f32.Abs(highest-25) > 1e-3 {
		t.Errorf("the ray over the peak rises to %v, want the peak's 25", highest)
	}
}

func TestHex_LiftMovesTheNearestCornerTouchingTheCellsMeetingAtIt(t *testing.T) {
	g := grid.DefaultGrids{}.Hex(8, 6, 16)
	brd, write := board.NewBoard(g)
	r := New(brd, write.Touch)
	corner := g.CellOutline(g.At(3, 2), nil)[2]
	before := map[cell.ID]uint64{}
	g.EachCell(func(c cell.ID) { before[c] = brd.CellVersion(c) })
	r.Lift(geom.NewVec(corner.X-2, corner.Y+1), 3, 0)
	if got := r.GroundAt(corner); f32.Abs(got-3) > 1e-4 {
		t.Errorf("the corner nearest the point stands at %v, want lifted to 3", got)
	}
	touched := 0
	g.EachCell(func(c cell.ID) {
		if brd.CellVersion(c) == before[c] {
			return
		}
		touched++
		if !nearAny(g.CellOutline(c, nil), corner) {
			t.Errorf("%v, away from the corner, was touched", c)
		}
	})
	if touched != 3 {
		t.Errorf("%d cells touched, want the three meeting at the corner", touched)
	}
	r.Lift(corner, 10, 2)
	if got := r.GroundAt(corner); f32.Abs(got-13) > 1e-4 {
		t.Errorf("the corner lifted again stands at %v, want 13", got)
	}
	r.eachEdge(func(a, b vertex) {
		if d := f32.Abs(r.heightAt(a) - r.heightAt(b)); d > 2+shapeTolerance {
			t.Fatalf("the corners at %v and %v differ by %v along their edge, more than 2",
				r.hexPoint(a), r.hexPoint(b), d)
		}
	})
}

func TestHex_LevelAndFlattenLayTheCornersFlat(t *testing.T) {
	g := grid.DefaultGrids{}.Hex(8, 6, 16)
	r := onBoard(g)
	r.SetHeights(func(p geom.Vec) float32 { return p.X / 20 })
	flat := g.At(5, 3)
	r.Level(flat, 7, 0)
	if corners, level, _ := r.Hex(flat); level != 7 || corners != [6]float32{7, 7, 7, 7, 7, 7} {
		t.Errorf("the levelled cell's corners %v, level %v; want all 7", corners, level)
	}
	middle := g.CellCenter(flat)
	if got := r.GroundAt(geom.NewVec(middle.X+5, middle.Y+3)); got != 7 {
		t.Errorf("the ground within the levelled cell stands at %v, want 7", got)
	}

	r = onBoard(g)
	r.SetHeights(func(p geom.Vec) float32 { return p.X / 20 })
	from, to := g.CellOutline(g.At(1, 1), nil)[0], g.CellCenter(g.At(3, 4))
	h := r.GroundAt(from)
	r.Flatten(from, to, 0)
	end := r.hexPoint(r.nearestCorner(to))
	flattened := 0
	g.EachCell(func(c cell.ID) {
		corners, _, _ := r.Hex(c)
		for k, p := range g.CellOutline(c, nil) {
			if p.X < min(from.X, end.X)-1e-2 || p.X > max(from.X, end.X)+1e-2 ||
				p.Y < min(from.Y, end.Y)-1e-2 || p.Y > max(from.Y, end.Y)+1e-2 {
				continue
			}
			flattened++
			if corners[k] != h {
				t.Errorf("the corner at %v between the ends stands at %v, want flattened to %v",
					p, corners[k], h)
			}
		}
	})
	if flattened < 6 {
		t.Errorf("%d corners between the ends, want a few cells' worth", flattened)
	}
}

func nearAny(ps []geom.Vec, p geom.Vec) bool {
	return slices.ContainsFunc(ps, func(q geom.Vec) bool { return near(p, q) })
}

// every edge of the cells once
func (r *Relief) eachEdge(fn func(a, b vertex)) {
	for i := range r.values {
		v := r.hexVertex(i)
		for _, n := range r.vertexNeighbours(v, nil) {
			if j, _ := r.hexIndex(n); j > i {
				fn(v, n)
			}
		}
	}
}

func TestHex_KeepRoadsTakesBackWhatBreaksARoad(t *testing.T) {
	g := grid.DefaultGrids{}.Hex(6, 4, 16)
	brd, brdW := board.NewBoard(g)
	brdW.SetAll(cell.Kind{Name: cell.Named("grass"), Pace: 1, Allows: cell.Land})
	road := cell.Kind{Name: cell.Named("road"), Pace: 1, Allows: cell.Land, Graded: true}
	east, _ := grid.Direction(g, 1, 0)
	west := g.Opposite(east)
	for q := range uint32(3) {
		brdW.SetWay(g.At(q, 1), cell.Way{Kind: road, Width: 6, Links: 1<<east | 1<<west})
	}
	r := New(brd, brdW.Touch)
	before := slices.Clone(r.values)
	under := g.CellOutline(g.At(1, 1), nil)[0] // a corner of a road's cell
	if r.KeepRoads(func() { r.Lift(under, 5, 0) }) || !slices.Equal(r.values, before) {
		t.Errorf("a corner under the road raised, the heights %v; want them as they were",
			r.values)
	}
	away := g.At(4, 3)
	lifted := r.KeepRoads(func() { r.Lift(g.CellOutline(away, nil)[3], 5, 0) })
	if corners, _, _ := r.Hex(away); !lifted || corners[3] != 5 {
		t.Errorf("a corner away from the road stands at %v, want raised to 5", corners[3])
	}
}

func TestHex_KeepRoadsHoldsARoadUpAnEvenRamp(t *testing.T) {
	g := grid.DefaultGrids{}.Hex(6, 4, 16)
	brd, brdW := board.NewBoard(g)
	road := cell.Kind{Name: cell.Named("road"), Pace: 1, Allows: cell.Land, Graded: true}
	east, _ := grid.Direction(g, 1, 0)
	west := g.Opposite(east)
	for q := range uint32(3) {
		brdW.SetWay(g.At(q, 1), cell.Way{Kind: road, Width: 6, Links: 1<<east | 1<<west})
	}
	r := New(brd, brdW.Touch)
	r.SetHeights(func(p geom.Vec) float32 { return p.X / 20 })
	before := slices.Clone(r.values)
	beside := g.CellOutline(g.At(1, 1), nil)[0] // halfway up a road's ramp, off its way
	if r.KeepRoads(func() { r.Lift(beside, 1, 0) }) || !slices.Equal(r.values, before) {
		t.Error("a corner of a road's even ramp was lifted off it")
	}
}

func TestHex_ARunGoesThroughASave(t *testing.T) {
	g := grid.DefaultGrids{}.Hex(30, 24, 16)
	r := onBoard(g)
	rnd := rand.New(rand.NewPCG(7, 9))
	r.SetHeights(func(geom.Vec) float32 { return float32(20 * rnd.Float64()) })
	runs := make([]Heights, r.Runs())
	for i := range runs {
		runs[i].First = uint32(i * HeightsRun)
		r.fill(&runs[i])
	}
	back := onBoard(g)
	if !back.adopt(runs) || !slices.Equal(back.values, r.values) {
		t.Fatal("the corners did not come back from their runs as they went")
	}
	// a save of another count of heights, as one by cell was once, is no relief's
	if onBoard(g).adopt(runs[:len(runs)-1]) {
		t.Error("a save short of a run was taken for the relief's")
	}
}

func TestHex_GroundAtAllocatesNothing(t *testing.T) {
	g := grid.DefaultGrids{}.Hex(8, 6, 16)
	r := onBoard(g)
	r.SetHeights(func(p geom.Vec) float32 { return p.X / 10 })
	p := geom.NewVec(60, 40)
	if n := testing.AllocsPerRun(100, func() { r.GroundAt(p) }); n != 0 {
		t.Errorf("GroundAt allocates %v a call, want none", n)
	}
	visit := func(float32, float32) bool { return true }
	if n := testing.AllocsPerRun(100, func() {
		r.Along(p, geom.NewVec(0.6, 0.8), 120, visit)
	}); n != 0 {
		t.Errorf("Along allocates %v a call, want none", n)
	}
}

func TestHex_SetHexAndHexAreAPair(t *testing.T) {
	g := grid.DefaultGrids{}.Hex(5, 4, 16)
	brd, write := board.NewBoard(g)
	r := New(brd, write.Touch)
	c := g.At(2, 1)
	set := [6]float32{1, 2, 3, 4, 5, 6}
	before := map[cell.ID]uint64{}
	g.EachCell(func(n cell.ID) { before[n] = brd.CellVersion(n) })
	version := r.Version()
	r.SetHex(c, set)
	if corners, level, ok := r.Hex(c); !ok || corners != set || level != 3.5 {
		t.Errorf("set to %v, the cell reads %v, level %v; want them back, level 3.5", set,
			corners, level)
	}
	if r.Version() == version {
		t.Error("setting the corners left the relief's Version as it was")
	}
	touched := 0
	g.EachCell(func(n cell.ID) {
		if brd.CellVersion(n) != before[n] {
			touched++
		}
	})
	if touched != 7 {
		t.Errorf("%d cells touched, want the cell and its six neighbours", touched)
	}
	version = r.Version()
	r.SetHex(c, set)
	if r.Version() != version {
		t.Error("set as they stand, the corners counted a change")
	}
	// on the board's rim too
	g.EachCell(func(n cell.ID) {
		o, _ := g.Ordinal(n)
		var want [6]float32
		for k := range want {
			want[k] = float32(10*o + k)
		}
		r.SetHex(n, want)
		if got, _, _ := r.Hex(n); got != want {
			t.Errorf("%v set to %v reads %v", n, want, got)
		}
	})

	square := grid.DefaultGrids{}.Square(4, 4, 32)
	sq := onBoard(square)
	version = sq.Version()
	sq.SetHex(square.At(1, 1), set)
	if sq.Version() != version || slices.ContainsFunc(sq.values, func(h float32) bool {
		return h != 0
	}) {
		t.Error("SetHex moved a square relief")
	}
}

func TestHex_GradePutsTheSixCornersInTheOutlinesOrder(t *testing.T) {
	g := grid.DefaultGrids{}.Hex(6, 5, 16)
	r := onBoard(g)
	c := g.At(2, 2)
	heights := []float32{1, 2, 3, 4, 5, 6}
	if !r.Grade(c, heights, 0) {
		t.Fatal("grading level ground reports it unmoved")
	}
	if corners, level, _ := r.Hex(c); corners != [6]float32{1, 2, 3, 4, 5, 6} || level != 3.5 {
		t.Errorf("the cell's corners %v, level %v; want 1 to 6 clockwise from the top, 3.5",
			corners, level)
	}
	outline := g.CellOutline(c, nil)
	for k, p := range outline {
		if got := r.GroundAt(p); f32.Abs(got-heights[k]) > 1e-4 {
			t.Errorf("the ground at the outline's corner %d is %v, want %v", k, got, heights[k])
		}
	}
	shared := 0
	g.EachCell(func(n cell.ID) {
		corners, _, _ := r.Hex(n)
		for k, p := range g.CellOutline(n, nil) {
			want := float32(0)
			for j, q := range outline {
				if near(p, q) {
					want = heights[j]
					shared++
				}
			}
			if corners[k] != want {
				t.Errorf("%v's corner %d stands at %v, want %v", n, k, corners[k], want)
			}
		}
	})
	if shared != 18 {
		t.Errorf("the cell's corners are %d cells' corners, want each three cells'", shared)
	}
	before := slices.Clone(r.values)
	if r.Grade(c, heights, 0) || !slices.Equal(r.values, before) {
		t.Error("graded again as it stands, the ground moved or reports it did")
	}
}

// a ramp up east steeper than MaxStep along the cell's own edges
func TestHex_AGradedCellsCornersStayAndTheGroundRoundFollowsWithinMaxStep(t *testing.T) {
	g := grid.DefaultGrids{}.Hex(8, 6, 16)
	r := onBoard(g)
	c := g.At(3, 2)
	if !r.Grade(c, []float32{3, 6, 6, 3, 0, 0}, 1) {
		t.Fatal("grading a ramp into level ground reports it unmoved")
	}
	if corners, _, _ := r.Hex(c); corners != [6]float32{3, 6, 6, 3, 0, 0} {
		t.Errorf("the ramp's corners stand at %v, want them as graded", corners)
	}
	vs, n, _ := r.outline(c)
	own := vs[:n]
	r.eachEdge(func(a, b vertex) {
		if slices.Contains(own, a) || slices.Contains(own, b) {
			return
		}
		if d := f32.Abs(r.heightAt(a) - r.heightAt(b)); d > 1+shapeTolerance {
			t.Errorf("the corners at %v and %v differ by %v along their edge, more than 1",
				r.hexPoint(a), r.hexPoint(b), d)
		}
	})
	moved := 0
	for i, h := range r.values {
		if h != 0 && !slices.Contains(own, r.hexVertex(i)) {
			moved++
		}
	}
	if moved == 0 {
		t.Error("the ground round the graded cell stayed level")
	}
}

func TestHex_ACopysGroundIsTheReliefs(t *testing.T) {
	g := grid.DefaultGrids{}.Hex(6, 5, 16)
	r := onBoard(g)
	r.SetHeights(func(p geom.Vec) float32 { return p.X/8 + p.Y/4 })
	var copied Relief
	r.CopyInto(&copied)
	g.EachCell(func(c cell.ID) {
		for _, p := range append(g.CellOutline(c, nil), g.CellCenter(c)) {
			if a, b := r.GroundAt(p), copied.GroundAt(p); a != b {
				t.Fatalf("the ground at %v is %v, its copy's %v", p, a, b)
			}
		}
	})
}

func TestHex_TheHemFillsTheBoardsRectangle(t *testing.T) {
	for _, size := range [][2]uint32{{6, 5}, {5, 4}, {3, 1}} {
		g := grid.DefaultGrids{}.Hex(size[0], size[1], 16)
		r := onBoard(g)
		slope := func(p geom.Vec) float32 { return 3 + p.X/8 + p.Y/4 }
		r.SetHeights(slope)
		rim := HexRim(int(size[0]), int(size[1]), 16)
		// the rectangle is the cells' own bounds
		lo, hi := geom.NewVec(1e9, 1e9), geom.NewVec(-1e9, -1e9)
		g.EachCell(func(c cell.ID) {
			for _, p := range g.CellOutline(c, nil) {
				lo = geom.NewVec(min(lo.X, p.X), min(lo.Y, p.Y))
				hi = geom.NewVec(max(hi.X, p.X), max(hi.Y, p.Y))
			}
		})
		if !near(lo, rim.TopLeft) || !near(hi, rim.BottomRight) {
			t.Fatalf("%v: the rim is %v, the cells lie from %v to %v", size, rim, lo, hi)
		}
		// everywhere in it there is ground, the slope's wherever a cell or its hem's own corners
		// span it: an even slope is the same from corner to corner
		off, hemmed := 0, 0
		for x := rim.TopLeft.X + 0.5; x < rim.BottomRight.X; x += 1.7 {
			for y := rim.TopLeft.Y + 0.5; y < rim.BottomRight.Y; y += 1.3 {
				p := geom.NewVec(x, y)
				got := r.GroundAt(p)
				if got == 0 {
					off++
					continue
				}
				if _, on := g.CellAt(p); on {
					if f32.Abs(got-slope(p)) > 1e-2 {
						t.Fatalf("%v: a cell's ground at %v is %v, the slope's %v", size, p, got, slope(p))
					}
					continue
				}
				hemmed++
				// between a hem's corners the ground lies between their heights
				if got < slope(rim.TopLeft)-1 || got > slope(rim.BottomRight)+1 {
					t.Fatalf("%v: the hem's ground at %v is %v, off the slope", size, p, got)
				}
			}
		}
		if off > 0 || hemmed == 0 {
			t.Errorf("%v: %d places of the rim without ground, %d on its hem", size, off, hemmed)
		}
		// and none past it
		for _, p := range []geom.Vec{
			geom.NewVec(rim.TopLeft.X-1, 30),
			geom.NewVec(rim.BottomRight.X+1, 30),
			geom.NewVec(40, rim.BottomRight.Y+1),
			geom.NewVec(40, -1),
		} {
			if got := r.GroundAt(p); got != 0 {
				t.Errorf("%v: past the rim at %v the ground is %v, want none", size, p, got)
			}
		}
	}
}
