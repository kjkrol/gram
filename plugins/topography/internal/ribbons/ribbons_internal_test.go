package ribbons

import (
	"image/color"
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

func TestPiece_RoadsMeetWithTheirLinesRunningOnRoundTheCorners(t *testing.T) {
	cross := func(round float32) []node {
		return []node{
			{at: geom.NewVec(0, 0), width: 20, lanes: 2, round: round, arms: []int{1, 2, 3}},
			{at: geom.NewVec(32, 0), width: 20, lanes: 2, arms: []int{0}},
			{at: geom.NewVec(-32, 0), width: 20, lanes: 2, arms: []int{0}},
			{at: geom.NewVec(0, 32), width: 12, lanes: 1, arms: []int{0}},
		}
	}
	nodes := cross(0)
	r := New(Config{Sector: func(x, y float32) int { return 0 }})
	r.pieces, r.spans = map[int][]float32{}, map[int][2]float32{}
	r.into, r.nodes = r.pieces, nodes
	r.walk(nodes)
	r.piece(nodes, 0, flat, bends)
	roads, lines, far := 0, 0, 0
	all := r.pieces[0]
	for i := 0; i < len(all); i += floats {
		p := all[i : i+floats]
		switch {
		case int(p[13])&(disc|arm) != 0:
			t.Errorf("a plate or a bare arm %v where roads meet, want roads and corners", p)
		case p[12] > 0:
			roads++
			// from its mouth, off the middle, to halfway to the next cell
			from, to := gap(geom.Vec{}, geom.NewVec(p[0], p[1])), gap(geom.Vec{},
				geom.NewVec(p[2], p[3]))
			if from < mouthLeast || from >= 16 || to != 16 || p[10] != from {
				t.Errorf("a road from %v to %v off the middle, its lines from %v; want from "+
					"its mouth to 16, the lines beginning there", from, to, p[10])
			}
		case p[16] == lineRGB[0]:
			lines++
			// the street's far edge, opposite the lane: straight on through the junction
			if p[1] < 0 {
				far++
				if f32.Abs(p[1]+10-lineInset) > 1e-4 || f32.Abs(p[3]+10-lineInset) > 1e-4 {
					t.Errorf("the far edge's line at y %v to %v, want straight at %v", p[1],
						p[3], -10+lineInset)
				}
			}
		}
	}
	if roads != 3 || lines != 12 || far != 4 {
		t.Errorf("%d roads, %d pieces of edge line, %d along the far edge; want three roads "+
			"and a corner of four pieces between each two, one corner the far edge", roads,
			lines, far)
	}
	// the corner from the street into the lane begins at the street's edge line and ends at
	// the lane's
	var first, last []float32
	for i := 0; i < len(all); i += floats {
		p := all[i : i+floats]
		if p[12] == 0 && p[16] == lineRGB[0] && p[0] > 0 && p[1] > 0 {
			if first == nil {
				first = p
			}
			last = p
		}
	}
	if first == nil || f32.Abs(first[1]-(10-lineInset)) > 1e-4 ||
		f32.Abs(last[2]-(6-lineInset)) > 1e-4 {
		t.Errorf("the corner's line from %v to %v, want from the street's edge line at y %v "+
			"round to the lane's at x %v", first, last, 10-lineInset, 6-lineInset)
	}
	// far off: the roads and the ground between them, no line
	r.pieces = map[int][]float32{}
	r.into = r.pieces
	r.piece(nodes, 0, flat, 1)
	for i := 0; i < len(r.pieces[0]); i += floats {
		if p := r.pieces[0][i : i+floats]; p[12] == 0 && p[16] == lineRGB[0] {
			t.Fatalf("an edge line %v far off, want none", p)
		}
	}
	_, _, plates, islands, r := pieces(cross(40))
	if plates != 4 || islands != 1 {
		t.Fatalf("a roundabout: %d plates, %d islands; want its plate with an island", plates,
			islands)
	}
	all = r.pieces[0]
	for i := 0; i < len(all); i += floats {
		p := all[i : i+floats]
		if int(p[13])&island == 0 {
			continue
		}
		// two lanes of 3.5 round it
		if radius := f32.Hypot(p[4], p[5]); radius != 40-7 {
			t.Errorf("the island is %v round, want the plate's 40 less two lanes", radius)
		}
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

func TestRoundabout_ARoadRunsInByAMouthItsLinesRoundingIntoTheRim(t *testing.T) {
	// a roundabout 40 round, its plate at 5, and a street run in from the east
	nodes := []node{
		{at: geom.NewVec(0, 0), width: 20, lanes: 2, round: 40, ring: 1, plate: 5, arms: []int{1}},
		{at: geom.NewVec(80, 0), width: 20, lanes: 2, arms: []int{0}},
	}
	r := New(Config{Sector: func(x, y float32) int { return 0 }})
	r.pieces, r.spans = map[int][]float32{}, map[int][2]float32{}
	r.into, r.nodes = r.pieces, nodes
	r.walk(nodes)
	r.piece(nodes, 1, flat, bends)
	street := r.pieces[0]
	if len(street) < floats || street[2] != 40+rimSet || street[9] != 5 {
		t.Fatalf("the street ends at x %v, %v high; want a mouth's length short of the rim at "+
			"%v, down on the plate at 5", street[2], street[9], 40+rimSet)
	}
	r.pieces = map[int][]float32{}
	r.into = r.pieces
	r.piece(nodes, 0, flat, bends)
	plates, mouths, corners, dashes, rim := 0, 0, 0, 0, 0
	all := r.pieces[0]
	for i := 0; i < len(all); i += floats {
		p := all[i : i+floats]
		far := gap(geom.Vec{}, geom.NewVec((p[0]+p[2])/2, (p[1]+p[3])/2))
		switch {
		case int(p[13])&disc != 0:
			plates++
		case p[16] != lineRGB[0]:
			// the mouth: from where the street ends in under the plate, as wide as it
			if p[4] != 0 || p[5] != 10 {
				continue // a corner's ground
			}
			mouths++
			// in under the plate, short of the island 7 within the rim
			if p[0] != 40+rimSet || f32.Abs(p[2]-(40-5.6)) > 1e-4 || p[12] != 0 {
				t.Errorf("the mouth %v, want from x %v to %v, bare of lines", p, 40+rimSet,
					40-5.6)
			}
		case far > 40:
			corners++
		case f32.Hypot(p[4], p[5]) > 1.5*lineHalf:
			dashes++
			if f32.Abs(f32.Atan2((p[1]+p[3])/2, (p[0]+p[2])/2)) > 0.26 {
				t.Errorf("a dash of the line it gives way at lies at %v, %v, off its mouth",
					p[0], p[1])
			}
		default:
			rim++
			if f32.Abs(f32.Atan2((p[1]+p[3])/2, (p[0]+p[2])/2)) < 0.3 {
				t.Errorf("the rim's line runs on at %v, %v, across the mouth", p[0], p[1])
			}
		}
	}
	if plates != 2 || mouths != 1 || corners == 0 || dashes == 0 || rim == 0 {
		t.Errorf("%d plates, %d mouths, %d pieces of corner line, %d dashes, %d of rim line; "+
			"want the roadway and its island, the mouth, both corners, the line it gives way "+
			"at and the rim's line round the rest", plates, mouths, corners, dashes, rim)
	}
}
