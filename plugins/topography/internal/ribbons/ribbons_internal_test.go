package ribbons

import (
	"image/color"
	"math"
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/internal/f32"
	"github.com/kjkrol/gram/plugins/topography/painter"
)

func flat(geom.Vec) float32 { return 5 }

// the pieces of the nodes' roads, by their kind
func pieces(nodes []node) (strips, arms, plates, islands int, r *Renderer) {
	r = New(Config{Sector: func(x, y float32) int { return 0 }})
	r.pieces, r.coarse, r.spans = map[int][]float32{}, map[int][]float32{}, map[int][2]float32{}
	r.walk(nodes)
	r.nodes = nodes
	for i := range nodes {
		r.into = r.pieces
		r.piece(nodes, i, flat, bends)
		r.into = r.coarse
		r.piece(nodes, i, flat, 1)
	}
	all := r.pieces[0]
	for i := 0; i < len(all); i += floats {
		switch kind := int(all[i+13]); {
		case kind&bank != 0:
		case kind&island != 0:
			islands++
		case kind&disc != 0:
			plates++
		case kind&arm != 0:
			arms++
		default:
			strips++
		}
	}
	return strips, arms, plates, islands, r
}

func TestPiece_AStreetBendsThroughItsCellsAxesAndEndsRound(t *testing.T) {
	nodes := []node{
		{at: geom.NewVec(0, 0), width: 20, lanes: 2, arms: []int{1}},
		{at: geom.NewVec(30, 6), width: 20, lanes: 2, arms: []int{0, 2}},
		{at: geom.NewVec(60, 0), width: 20, lanes: 2, arms: []int{1}},
	}
	strips, arms, plates, islands, r := pieces(nodes)
	if strips != bends || arms != 2 || plates != 2 || islands != 0 {
		t.Fatalf("%d strips, %d arms, %d plates, %d islands; want the bend in %d strips and an "+
			"arm and a round end at either end", strips, arms, plates, islands, bends)
	}
	all := r.pieces[0]
	for i := 0; i < len(all); i += floats {
		p := all[i : i+floats]
		if int(p[13])&(disc|arm) != 0 {
			continue
		}
		if p[8] != 5+lift || p[9] != 5+lift {
			t.Errorf("a strip lies at %v and %v, want %v over level ground at 5", p[8], p[9], lift)
		}
		if p[11] <= p[10] {
			t.Errorf("a strip runs from %v to %v along its stretch, want it running on", p[10],
				p[11])
		}
	}
	if far := len(r.coarse[0]) / floats; far != 3 {
		t.Errorf("far off %d pieces, want the bend in one strip and an arm at either end", far)
	}
	if got := nodes[1].along; got < 30 || got > 31 {
		t.Errorf("the middle cell lies %v along its stretch, want its axis's 30.6", got)
	}
}

// roads lays the roads of nodes over the ground, a metre a unit, as a board's would be laid
func roads(nodes []node, ground func(geom.Vec) float32) *Renderer {
	r := New(Config{Sector: func(x, y float32) int { return 0 }, Metre: 1})
	r.build(nodes, ground)
	return r
}

// lines are the edge lines laid, and the broken ones a road gives way at
func lines(r *Renderer) (edges, dashes [][]float32) {
	all := r.pieces[0]
	for i := 0; i < len(all); i += floats {
		p := all[i : i+floats]
		if [3]float32(p[16:19]) != lineRGB {
			continue
		}
		if f32.Hypot(p[4], p[5]) > 1.5*lineHalf {
			dashes = append(dashes, p)
		} else {
			edges = append(edges, p)
		}
	}
	return edges, dashes
}

// paved says the roadway laid covers q: a piece's triangles or its plate
func paved(r *Renderer, q geom.Vec) bool {
	all := r.pieces[0]
	for i := 0; i < len(all); i += floats {
		p := all[i : i+floats]
		if int(p[13])&(bank|island) != 0 || [3]float32(p[16:19]) == lineRGB {
			continue
		}
		for _, f := range facesOf(p) {
			if f.holds(q, 0) {
				return true
			}
		}
	}
	return false
}

// joined counts the runs the edge lines make, each line's end the next one's start, and the
// ends no line runs on from
func joined(edges [][]float32) (runs, loose int) {
	key := func(x, y float32) [2]int32 {
		return [2]int32{int32(f32.Round(x * 20)), int32(f32.Round(y * 20))}
	}
	starts, ends := map[[2]int32]int{}, map[[2]int32]int{}
	for _, p := range edges {
		starts[key(p[0], p[1])]++
		ends[key(p[2], p[3])]++
	}
	for k, n := range ends {
		if starts[k] < n {
			loose += n - starts[k]
		}
	}
	for k, n := range starts {
		if ends[k] < n {
			loose += n - ends[k]
		}
	}
	return len(edges), loose
}

// a street along x, 7 wide, and a lane as wide off it to +y, each two cells of 10 on
func tee() []node {
	return []node{
		{at: geom.NewVec(0, 0), width: 7, lanes: 2, arms: []int{1, 2, 3}},
		{at: geom.NewVec(10, 0), width: 7, lanes: 2, arms: []int{0, 4}},
		{at: geom.NewVec(-10, 0), width: 7, lanes: 2, arms: []int{0, 5}},
		{at: geom.NewVec(0, 10), width: 7, lanes: 2, arms: []int{0, 6}},
		{at: geom.NewVec(20, 0), width: 7, lanes: 2, arms: []int{1}},
		{at: geom.NewVec(-20, 0), width: 7, lanes: 2, arms: []int{2}},
		{at: geom.NewVec(0, 20), width: 7, lanes: 2, arms: []int{3}},
	}
}

func TestOutline_RoadsMeetWithOneEdgeLineRoundTheirCorners(t *testing.T) {
	r := roads(tee(), flat)
	edges, dashes := lines(r)
	if len(edges) == 0 || len(dashes) != 0 {
		t.Fatalf("%d pieces of edge line, %d dashes; want the roadway's edge lined whole, no "+
			"line it gives way at", len(edges), len(dashes))
	}
	if _, loose := joined(edges); loose != 0 {
		t.Errorf("%d ends of edge line no line runs on from, want the edge lined round unbroken",
			loose)
	}
	near := float32(1e9)
	for _, p := range edges {
		mid := geom.NewVec((p[0]+p[2])/2, (p[1]+p[3])/2)
		d := geom.NewVec(p[2]-p[0], p[3]-p[1])
		l := f32.Hypot(d.X, d.Y)
		// the roadway lies on the line's left: lineInset in, and none past the edge
		in := geom.NewVec(mid.X-d.Y/l*0.2, mid.Y+d.X/l*0.2)
		out := geom.NewVec(mid.X+d.Y/l*(lineInset+0.2), mid.Y-d.X/l*(lineInset+0.2))
		if !paved(r, in) || paved(r, out) {
			t.Errorf("an edge line at %v, %v: roadway in %v, past it %v; want the line "+
				"lineInset in from the edge", mid.X, mid.Y, paved(r, in), paved(r, out))
		}
		for _, c := range [2]geom.Vec{geom.NewVec(3.5, 3.5), geom.NewVec(-3.5, 3.5)} {
			near = min(near, gap(mid, c))
		}
	}
	// the corner from the street into the lane rounded: its line runs round outside the
	// corner it would make cut square, turning a little at a time
	if near < 0.8 {
		t.Errorf("the edge line passes %v off a corner where the roads meet, want it rounded",
			near)
	}
	for _, p := range edges {
		for _, q := range edges {
			if q[0] != p[2] || q[1] != p[3] {
				continue
			}
			a := f32.Atan2(p[3]-p[1], p[2]-p[0])
			b := f32.Atan2(q[3]-q[1], q[2]-q[0])
			if turn := f32.Abs(f32.Remainder(b-a, 2*math.Pi)); turn > 0.5 {
				t.Errorf("the edge line turns by %v at %v, %v, want it rounded", turn, p[2],
					p[3])
			}
		}
	}
	if !paved(r, geom.NewVec(4, 4)) {
		t.Errorf("the corner between the street and the lane bare, want it rounded with roadway")
	}
	// no lane marked where the roads lie on each other
	all := r.pieces[0]
	for i := 0; i < len(all); i += floats {
		p := all[i : i+floats]
		if p[12] == 0 || int(p[13])&(disc|bank) != 0 || [3]float32(p[16:19]) == lineRGB {
			continue
		}
		if int(p[19])&3 != 3 {
			t.Errorf("a stretch %v marks its own edges, want the edge's line alone", p)
		}
		mid := geom.NewVec((p[0]+p[2])/2, (p[1]+p[3])/2)
		if f32.Hypot(mid.X, mid.Y) < 3 && int(p[19])&4 == 0 {
			t.Errorf("a stretch at %v, %v marks its lanes within the junction, want them bare",
				mid.X, mid.Y)
		}
	}
}

func TestOutline_AHoleSmallerThanALaneIsRoadwayAnIslandIsLined(t *testing.T) {
	// four roads round a block: the cells 10 apart leave a hole of 3 by 3, 40 apart an island
	ring := func(side float32) []node {
		return []node{
			{at: geom.NewVec(0, 0), width: 7, lanes: 2, arms: []int{1, 3}},
			{at: geom.NewVec(side, 0), width: 7, lanes: 2, arms: []int{0, 2}},
			{at: geom.NewVec(side, side), width: 7, lanes: 2, arms: []int{1, 3}},
			{at: geom.NewVec(0, side), width: 7, lanes: 2, arms: []int{2, 0}},
		}
	}
	r := roads(ring(10), flat)
	if !paved(r, geom.NewVec(5, 5)) {
		t.Errorf("a hole of 3 by 3 between four roads left bare, want it roadway")
	}
	edges, _ := lines(r)
	for _, p := range edges {
		if mid := geom.NewVec((p[0]+p[2])/2, (p[1]+p[3])/2); gap(mid, geom.NewVec(5, 5)) < 3 {
			t.Errorf("an edge line at %v, %v round the hole filled, want none", mid.X, mid.Y)
		}
	}
	r = roads(ring(40), flat)
	if paved(r, geom.NewVec(20, 20)) {
		t.Errorf("the block within four roads paved, want it an island")
	}
	edges, _ = lines(r)
	inner := 0
	for _, p := range edges {
		mid := geom.NewVec((p[0]+p[2])/2, (p[1]+p[3])/2)
		if mid.X > 4 && mid.X < 36 && mid.Y > 4 && mid.Y < 36 {
			inner++
		}
	}
	if _, loose := joined(edges); inner == 0 || loose != 0 {
		t.Errorf("%d pieces of line round the island, %d loose ends; want it lined round", inner,
			loose)
	}
}

// the banks and walls of a street of three cells along x, 20 wide, over ground, a metre a unit
func banked(st painter.Ribbon, ground func(geom.Vec) float32,
	solid func(geom.Vec) bool) (out [][]float32) {
	nodes := []node{
		{at: geom.NewVec(0, 0), width: 20, lanes: 2, style: st, arms: []int{1}},
		{at: geom.NewVec(32, 0), width: 20, lanes: 2, style: st, arms: []int{0, 2}},
		{at: geom.NewVec(64, 0), width: 20, lanes: 2, style: st, arms: []int{1}},
	}
	r := New(Config{Sector: func(x, y float32) int { return 0 }})
	r.pieces, r.spans, r.solid = map[int][]float32{}, map[int][2]float32{}, solid
	r.into, r.nodes = r.pieces, nodes
	r.walk(nodes)
	r.piece(nodes, 1, ground, bends)
	all := r.pieces[0]
	for i := 0; i < len(all); i += floats {
		if int(all[i+13])&bank != 0 {
			out = append(out, all[i:i+floats])
		}
	}
	return out
}

func TestBanks_ARoadOverLowerGroundIsBankedOrWalled(t *testing.T) {
	earth := color.RGBA{R: 90, G: 110, B: 60, A: 255}
	stone := color.RGBA{R: 120, G: 120, B: 120, A: 255}
	st := painter.Ribbon{Bank: earth, Wall: stone, Reach: 16}
	// the ground falls a unit in four towards +y: the road lies level with its high edge
	slope := func(p geom.Vec) float32 { return 100 - p.Y/4 }
	got := banked(st, slope, nil)
	if len(got) != bends {
		t.Fatalf("%d banks along a road across a slope, want one a strip on its low side",
			len(got))
	}
	for _, b := range got {
		// the edge at y 10 lies 5 over the ground; a bank falling 2 in 3 meets it 12 out
		if b[1] != 10 || b[5] != 12 || b[4] != 0 {
			t.Errorf("a bank from y %v reaching %v, %v out; want from the low edge 12 out", b[1],
				b[4], b[5])
		}
		if want := slope(geom.NewVec(0, 22)) - buried; b[10] != want {
			t.Errorf("a bank's foot at %v, want in the ground at %v", b[10], want)
		}
		if b[16] != straight(earth)[0] {
			t.Errorf("a bank coloured %v, want the bank's own", b[16])
		}
	}
	if got := banked(st, flat, nil); len(got) != 0 {
		t.Errorf("%d banks over level ground, want none", len(got))
	}
	if got := banked(painter.Ribbon{}, slope, nil); len(got) != 0 {
		t.Errorf("%d banks of a road styled with none, want none", len(got))
	}
	// a house in the bank's way, and ground falling away faster than the bank: a wall
	house := func(p geom.Vec) bool { return p.Y > 15 }
	cliff := func(p geom.Vec) float32 { return 100 - p.Y }
	for name, walls := range map[string][][]float32{
		"by a house":   banked(st, slope, house),
		"over a cliff": banked(st, cliff, nil),
	} {
		if len(walls) != bends {
			t.Fatalf("%s %d walls, want one a strip", name, len(walls))
		}
		for _, b := range walls {
			if b[5] != batter || b[16] != straight(stone)[0] {
				t.Errorf("%s a face %v out coloured %v, want a wall straight down", name, b[5],
					b[16])
			}
		}
	}
}

func TestRoundabout_ARoadRunsInByAMouthTheRimBrokenAcrossIt(t *testing.T) {
	// a roundabout 20 round, its plate at 5, and a street run in from the east
	nodes := []node{
		{at: geom.NewVec(0, 0), width: 7, lanes: 1, round: 20, ring: 1, plate: 5 + lift,
			arms: []int{1}},
		{at: geom.NewVec(40, 0), width: 7, lanes: 2, arms: []int{0, 2}},
		{at: geom.NewVec(50, 0), width: 7, lanes: 2, arms: []int{1}},
	}
	r := New(Config{Sector: func(x, y float32) int { return 0 }, Metre: 1})
	r.walk(nodes)
	r.nodes = nodes
	if p := r.joint(nodes, 1, 0); p.X != 20+rimSet || p.Y != 0 {
		t.Fatalf("the street hands on at %v, want a mouth's length short of the rim at %v", p,
			20+rimSet)
	}
	r = roads(nodes, flat)
	edges, dashes := lines(r)
	if len(dashes) == 0 {
		t.Fatalf("no line it gives way at, want one across its mouth")
	}
	for _, p := range dashes {
		mid := geom.NewVec((p[0]+p[2])/2, (p[1]+p[3])/2)
		if f32.Abs(gap(mid, geom.Vec{})-(20-lineInset)) > 0.2 ||
			f32.Abs(f32.Atan2(mid.Y, mid.X)) > 0.6 {
			t.Errorf("a dash of the line it gives way at at %v, %v, want across its mouth on "+
				"the rim's line", mid.X, mid.Y)
		}
	}
	rim := 0
	for _, p := range edges {
		mid := geom.NewVec((p[0]+p[2])/2, (p[1]+p[3])/2)
		if f32.Abs(gap(mid, geom.Vec{})-(20-lineInset)) < 0.2 {
			rim++
			if f32.Abs(f32.Atan2(mid.Y, mid.X)) < 0.2 {
				t.Errorf("the rim's line runs on at %v, %v, across the mouth", mid.X, mid.Y)
			}
		}
	}
	if _, loose := joined(edges); rim == 0 || loose != 0 {
		t.Errorf("%d pieces of the rim's line, %d loose ends of edge line; want the rim lined "+
			"round to the mouth and the street's edge lines running on into it", rim, loose)
	}
}

func TestPlane_LiesOnASlopeItIsLaidToHoweverItsReachDividesIntoSteps(t *testing.T) {
	slope := func(p geom.Vec) float32 { return 400 + 0.05*p.X - 0.03*p.Y }
	// a reach its step does not divide: the measuring must not run out short of one side
	h, tilt := plane(geom.NewVec(1000, 2000), 51.9, 5.19001, slope)
	if f32.Abs(tilt.X-0.05) > 1e-3 || f32.Abs(tilt.Y+0.03) > 1e-3 {
		t.Errorf("the plane climbs %v, want the slope's 0.05 and -0.03", tilt)
	}
	if want := slope(geom.NewVec(1000, 2000)); f32.Abs(h-want) > 0.05 {
		t.Errorf("the plane lies at %v at its middle, want on the slope at %v", h, want)
	}
}

// a tee on a hillside, its street climbing across the slope and its lane up it: the
// junction's roadway one surface with its roads, nowhere a piece over another at another
// height for a step to show between them
func TestJunction_OnAHillsideIsOneRoadway(t *testing.T) {
	hill := func(p geom.Vec) float32 { return 20 + 0.12*p.X + 0.08*p.Y }
	r := roads(tee(), hill)
	var faces []face
	all := r.pieces[0]
	for i := 0; i < len(all); i += floats {
		p := all[i : i+floats]
		if int(p[13])&(bank|marking) != 0 || [3]float32(p[16:19]) == lineRGB {
			continue
		}
		faces = append(faces, facesOf(p)...)
	}
	worst, at := float32(0), geom.Vec{}
	for x := float32(-9); x <= 9; x += 0.25 {
		for y := float32(-4); y <= 9; y += 0.25 {
			q := geom.NewVec(x, y)
			lo, hi, n := float32(math.Inf(1)), float32(math.Inf(-1)), 0
			for i := range faces {
				if f := &faces[i]; f.holds(q, 0.01) {
					z := f.zAt(q)
					lo, hi, n = min(lo, z), max(hi, z), n+1
				}
			}
			if n > 1 && hi-lo > worst {
				worst, at = hi-lo, q
			}
		}
	}
	if worst > 0.05 {
		t.Errorf("pieces lying on each other %v apart at %v, want the roadway one surface",
			worst, at)
	}
}

// round a bend on a hillside the banks run on, each landing where the next one begins: no
// gap between two at a turn of the roadway's edge
func TestBanks_RoundABendLandAtOneFoot(t *testing.T) {
	st := painter.Ribbon{Bank: color.RGBA{R: 90, G: 110, B: 60, A: 255}, Reach: 16}
	nodes := []node{
		{at: geom.NewVec(0, 0), width: 7, lanes: 2, style: st, arms: []int{1}},
		{at: geom.NewVec(30, 0), width: 7, lanes: 2, style: st, arms: []int{0, 2}},
		{at: geom.NewVec(30, 30), width: 7, lanes: 2, style: st, arms: []int{1}},
	}
	r := roads(nodes, func(p geom.Vec) float32 { return 100 - (p.X+p.Y)/3 })
	var banks [][]float32
	all := r.pieces[0]
	for i := 0; i < len(all); i += floats {
		if int(all[i+13])&bank != 0 {
			banks = append(banks, all[i:i+floats])
		}
	}
	if len(banks) == 0 {
		t.Fatal("no bank along a road over a hillside")
	}
	joined := 0
	for _, a := range banks {
		for _, b := range banks {
			if a[2] != b[0] || a[3] != b[1] {
				continue
			}
			joined++
			if f32.Abs(a[6]-b[4]) > 1e-3 || f32.Abs(a[7]-b[5]) > 1e-3 ||
				f32.Abs(a[11]-b[10]) > 1e-3 {
				t.Errorf("a bank lands at %v, %v at %v, the next one at %v, %v at %v from the "+
					"same edge", a[6], a[7], a[11], b[4], b[5], b[10])
			}
		}
	}
	if joined == 0 {
		t.Error("no bank runs on from another")
	}
}
