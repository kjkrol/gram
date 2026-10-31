package pathfind

import (
	"maps"
	"math"
	"slices"
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/uid"
)

// a body bigger than a cell of 10 reaching over a cell round its own, three cells a side, and one
// within a cell
var (
	big   = Body{Size: box(25)}
	small = Body{Size: box(4)}
)

// a body's footprint takes a cell up to a cell, past it unit.Footprint's side; its window is laid
// from a node: an odd one round it, an even one from it right and down, and turned through it
func TestFootprint_AWindowIsLaidRoundANodeOddAndFromItEven(t *testing.T) {
	g := grid.DefaultGrids{}.Square(9, 9, 10)
	f := New(g, openTerrain(), nil, &cell.MultipleOccupancy{})
	for _, tc := range []struct {
		size float32
		side int
	}{{10, 1}, {10.5, 2}, {19.9, 2}, {20, 3}, {29.9, 3}, {30, 4}} {
		if got := f.sideOf(Body{Size: geom.NewVec(4, tc.size)}); got != tc.side {
			t.Errorf("a body %v long takes %d cells a side, want %d", tc.size, got, tc.side)
		}
	}
	window := func(side int, turned bool) map[[2]int]bool {
		ball, ok := f.round(g.At(4, 4), f.windowOf(side, turned), nil)
		if !ok {
			t.Fatalf("a window of %d cells off the grid", side)
		}
		got := map[[2]int]bool{}
		for _, c := range ball {
			x, y, _ := g.Coords(c)
			got[[2]int{int(x) - 4, int(y) - 4}] = true
		}
		return got
	}
	block := func(lo, hi int) map[[2]int]bool {
		want := map[[2]int]bool{}
		for a := lo; a <= hi; a++ {
			for b := lo; b <= hi; b++ {
				want[[2]int{a, b}] = true
			}
		}
		return want
	}
	for _, tc := range []struct {
		side   int
		turned bool
		lo, hi int
	}{{2, false, 0, 1}, {2, true, -1, 0}, {3, false, -1, 1}, {3, true, -1, 1}, {4, false, -1, 2},
		{4, true, -2, 1}} {
		if got, want := window(tc.side, tc.turned), block(tc.lo, tc.hi); !maps.Equal(got, want) {
			t.Errorf("the window of %d cells, turned %v: %v, want %v", tc.side, tc.turned, got,
				want)
		}
	}
}

// narrowed is a board 20 cells by 10 of 10, its columns 5 to 14 walls but in the rows open
func narrowed(open ...uint32) (grid.Grid, *cellsPassing) {
	g := grid.DefaultGrids{}.Square(20, 10, 10)
	terrain := passingTerrain()
	for y := range uint32(10) {
		if slices.Contains(open, y) {
			continue
		}
		for x := uint32(5); x < 15; x++ {
			terrain.Set(g.At(x, y), wall)
		}
	}
	return g, terrain
}

// crossed says whether steps cross the walled columns of narrowed in one of rows
func crossed(g grid.Grid, steps []cell.ID, rows ...uint32) bool {
	for _, c := range steps {
		if x, y, _ := g.Coords(c); x >= 5 && x < 15 && slices.Contains(rows, y) {
			return true
		}
	}
	return false
}

// passages one, two and three cells wide across a wall: a body reaching over a cell round its own
// takes the one of three alone, its middle row; one within a cell the narrowest, the nearest
func TestFind_ABodyBiggerThanACellGoesOnlyWhereItsReachFits(t *testing.T) {
	g, terrain := narrowed(1, 3, 4, 6, 7, 8)
	from, to := g.At(2, 1), g.At(17, 1)
	for name, f := range map[string]*Finder{
		"plain":     New(g, terrain, nil, &cell.MultipleOccupancy{}),
		"by levels": New(g, terrain, nil, &cell.MultipleOccupancy{}).WithLevels(flat{terrain}),
	} {
		var steps [64]cell.ID
		n, ok := f.Find(uid.UID64(1), cell.Land, big, from, to, Either, steps[:])
		if !ok || crossed(g, steps[:n], 1, 3, 4, 6, 8) || !crossed(g, steps[:n], 7) {
			t.Errorf("%s: the big body found %v by %v; want the middle of the widest passage",
				name, ok, steps[:n])
		}
		n, ok = f.Find(uid.UID64(1), cell.Land, small, from, to, Either, steps[:])
		if !ok || !crossed(g, steps[:n], 1) {
			t.Errorf("%s: the small body found %v by %v; want the narrowest", name, ok, steps[:n])
		}
		for _, tc := range []struct {
			row   uint32
			steps bool
		}{{1, false}, {3, false}, {4, false}, {7, true}} {
			if _, ok := f.Steps(cell.Land, big, g.At(4, tc.row), g.At(5, tc.row)); ok != tc.steps {
				t.Errorf("%s: the big body steps into row %d's passage %v, want %v", name, tc.row,
					ok, tc.steps)
			}
		}
	}
	g, terrain = narrowed(1, 3, 4)
	f := New(g, terrain, nil, &cell.MultipleOccupancy{}).WithLevels(flat{terrain})
	var steps [64]cell.ID
	if n, ok := f.Find(uid.UID64(1), cell.Land, big, from, to, Either, steps[:]); ok {
		t.Errorf("past passages of one and two cells the big body found %v", steps[:n])
	}
}

// even is a body of a footprint two cells a side, on cells of 10
var even = Body{Size: box(15)}

// passages one, two and three cells wide across a wall: a body of a footprint two cells a side
// takes the one of two, laid from its first row, the nearer of those it fits
func TestFind_AnEvenFootprintGoesOnlyWhereItsWindowFits(t *testing.T) {
	g, terrain := narrowed(1, 3, 4, 6, 7, 8)
	from, to := g.At(2, 1), g.At(17, 1)
	for name, f := range map[string]*Finder{
		"plain":     New(g, terrain, nil, &cell.MultipleOccupancy{}),
		"by levels": New(g, terrain, nil, &cell.MultipleOccupancy{}).WithLevels(flat{terrain}),
	} {
		var steps [64]cell.ID
		n, ok := f.Find(uid.UID64(1), cell.Land, even, from, to, Either, steps[:])
		if !ok || crossed(g, steps[:n], 1, 4, 6, 7, 8) || !crossed(g, steps[:n], 3) {
			t.Errorf("%s: the even body found %v by %v; want the passage of two, laid from its "+
				"first row", name, ok, steps[:n])
		}
		for _, tc := range []struct {
			row   uint32
			steps bool
		}{{1, false}, {3, true}, {4, false}, {6, true}, {7, true}, {8, false}} {
			if _, ok := f.Steps(cell.Land, even, g.At(4, tc.row), g.At(5, tc.row)); ok != tc.steps {
				t.Errorf("%s: the even body steps into row %d's passage %v, want %v", name,
					tc.row, ok, tc.steps)
			}
		}
	}
}

// a slantwise step sweeps a box of an even footprint past its flanks' windows too: a wall in one
// of them refuses the step, the two straight steps round it taken
func TestFind_ASlantwiseStepOfAnEvenFootprintNeedsItsFlanksToo(t *testing.T) {
	g := grid.DefaultGrids{}.Square(10, 10, 10)
	terrain := passingTerrain()
	terrain.Set(g.At(6, 4), wall)
	f := New(g, terrain, nil, &cell.MultipleOccupancy{}).WithLevels(flat{terrain})
	from, to, flank := g.At(4, 4), g.At(5, 5), g.At(4, 5)
	if _, ok := f.Steps(cell.Land, even, from, to); ok {
		t.Error("the even body stepped slantwise past a wall in a flank's window")
	}
	var steps [64]cell.ID
	if n, ok := f.Find(uid.UID64(1), cell.Land, even, from, to, Either,
		steps[:]); !ok || !slices.Equal(steps[:n], []cell.ID{flank, to}) {
		t.Errorf("found %v by %v, want by %v", ok, steps[:n], []cell.ID{flank, to})
	}
}

// a slantwise step sweeps a box reaching over a cell past the cells round its flanks too: a wall
// by one of them refuses the step, the two straight steps round it taken
func TestFind_ASlantwiseStepOfABodyBiggerThanACellNeedsItsFlanksToo(t *testing.T) {
	g := grid.DefaultGrids{}.Square(10, 10, 10)
	terrain := passingTerrain()
	terrain.Set(g.At(6, 3), wall)
	f := New(g, terrain, nil, &cell.MultipleOccupancy{}).WithLevels(flat{terrain})
	from, to, flank := g.At(4, 4), g.At(5, 5), g.At(4, 5)
	for _, tc := range []struct {
		name   string
		body   Body
		route  []cell.ID
		across bool
	}{
		{"big", big, []cell.ID{flank, to}, false},
		{"small", small, []cell.ID{to}, true},
	} {
		if _, ok := f.Steps(cell.Land, tc.body, from, to); ok != tc.across {
			t.Errorf("%s: steps slantwise %v, want %v", tc.name, ok, tc.across)
		}
		var steps [64]cell.ID
		if n, ok := f.Find(uid.UID64(1), cell.Land, tc.body, from, to, Either,
			steps[:]); !ok || !slices.Equal(steps[:n], tc.route) {
			t.Errorf("%s: found %v by %v, want by %v", tc.name, ok, steps[:n], tc.route)
		}
	}
	for _, step := range [][2]cell.ID{{from, flank}, {flank, to}} {
		if _, ok := f.Steps(cell.Land, big, step[0], step[1]); !ok {
			t.Errorf("the big body refused the straight step %v", step)
		}
	}
}

// hexNarrowed is a hex board 20 cells by 9 of 10, its cells q 6 to 13 walls but in row 1 and rows
// 5 to 7
func hexNarrowed() (grid.Grid, *cellsPassing) {
	g := grid.DefaultGrids{}.Hex(20, 9, 10)
	terrain := passingTerrain()
	for r := range uint32(9) {
		if r == 1 || r >= 5 && r <= 7 {
			continue
		}
		for q := uint32(6); q < 14; q++ {
			terrain.Set(g.At(q, r), wall)
		}
	}
	return g, terrain
}

// on a hex board a body reaching over a cell round its own takes a passage three rows wide, its
// middle row, and none of a row; one within a cell the narrow one, the nearer
func TestFind_OnAHexBoardABodyBiggerThanACellGoesOnlyWhereItsReachFits(t *testing.T) {
	g, terrain := hexNarrowed()
	f := New(g, terrain, nil, &cell.MultipleOccupancy{}).WithLevels(flat{terrain})
	rows := func(steps []cell.ID) map[uint32]bool {
		crossed := map[uint32]bool{}
		for _, c := range steps {
			if q, r, _ := g.Coords(c); q >= 6 && q < 14 {
				crossed[r] = true
			}
		}
		return crossed
	}
	var steps [64]cell.ID
	for _, tc := range []struct {
		name string
		body Body
		rows map[uint32]bool
	}{{"big", big, map[uint32]bool{6: true}}, {"small", small, map[uint32]bool{1: true}}} {
		n, ok := f.Find(uid.UID64(1), cell.Land, tc.body, g.At(2, 1), g.At(17, 1), Either,
			steps[:])
		if got := rows(steps[:n]); !ok || !maps.Equal(got, tc.rows) {
			t.Errorf("%s: found %v by %v, crossing in rows %v, want %v", tc.name, ok, steps[:n],
				got, tc.rows)
		}
	}
	if _, ok := f.Steps(cell.Land, big, g.At(5, 1), g.At(6, 1)); ok {
		t.Error("the big body steps into the passage of a row")
	}
}

// a platform: whole-cell decks over a block of cells, x and y from 3 to 7, top up and their
// underside under, one cell's deck rise higher
type platform struct {
	g          grid.Grid
	terrain    cell.Terrain
	top, under float32
	raised     cell.ID
	rise       float32
}

func (p platform) on(c cell.ID) bool {
	x, y, _ := p.g.Coords(c)
	return x >= 3 && x <= 7 && y >= 3 && y <= 7
}

func (p platform) Deck(c cell.ID) (top, under float32, carries cell.Domain, ok bool) {
	if !p.on(c) {
		return 0, 0, 0, false
	}
	if c == p.raised {
		return p.top + p.rise, p.under + p.rise, cell.Land, true
	}
	return p.top, p.under, cell.Land, true
}

func (p platform) Ground(geom.Vec) float32 { return 0 }

func (p platform) Over(at geom.Vec) (float32, bool) {
	if c, ok := p.g.CellAt(at); ok {
		if _, under, _, ok := p.Deck(c); ok {
			return under, true
		}
	}
	return 0, false
}

func (p platform) KindAt(c cell.ID, deck bool) cell.Kind {
	if deck && p.on(c) {
		return cell.Kind{Pace: 1, Allows: cell.Land}
	}
	return p.terrain.Kind(c)
}

// a body reaching over a cell round its own stands on one level of the cells round it: on a
// platform's decks within its edge, and on the ground clear of it unless it fits under; decks it
// would not step between hold it off
func TestFootprint_StandsOnOneLevelOfEveryCellItReachesOver(t *testing.T) {
	g := grid.DefaultGrids{}.Square(12, 12, 10)
	terrain := passingTerrain()
	body := Body{Step: 1, Height: 2, Size: box(25)}
	within := func(c cell.ID, x0, x1 uint32) bool {
		x, y, _ := g.Coords(c)
		return x >= x0 && x <= x1 && y >= x0 && y <= x1
	}
	for _, tc := range []struct {
		name      string
		levels    platform
		deck      func(c cell.ID) bool
		grounding func(c cell.ID) bool
	}{
		{"a low platform", platform{g: g, terrain: terrain, top: 1, under: 0.5},
			func(c cell.ID) bool { return within(c, 4, 6) },
			func(c cell.ID) bool { return !within(c, 2, 8) }},
		{"a high platform", platform{g: g, terrain: terrain, top: 10, under: 9},
			func(c cell.ID) bool { return within(c, 4, 6) },
			func(c cell.ID) bool { return true }},
		{"a high platform, a deck past a step", platform{g: g, terrain: terrain, top: 10,
			under: 9, raised: g.At(4, 4), rise: 2},
			func(c cell.ID) bool { return within(c, 4, 6) && !within(c, 3, 5) },
			func(c cell.ID) bool { return true }},
	} {
		f := New(g, terrain, nil, &cell.MultipleOccupancy{}).WithLevels(tc.levels)
		f.begin(cell.Land)
		fp := f.footprintOf(body)
		g.EachCell(func(c cell.ID) {
			x, y, _ := g.Coords(c)
			edge := x == 0 || y == 0 || x == 11 || y == 11
			if got, want := f.standsOn(fp, c, ground), !edge && tc.grounding(c); got != want {
				t.Errorf("%s: stands on %d,%d's ground %v, want %v", tc.name, x, y, got, want)
			}
			if got, want := f.standsOn(fp, c, deck), tc.deck(c); got != want {
				t.Errorf("%s: stands on %d,%d's deck %v, want %v", tc.name, x, y, got, want)
			}
		})
	}
}

// what limits a body on a cell holds a body bigger than a cell off the cells round it: a floor
// too weak for it and a gap too narrow for its narrower side
func TestFootprint_ALimitHoldsABodyOffTheCellsRoundIt(t *testing.T) {
	g := grid.DefaultGrids{}.Square(10, 10, 10)
	middle := g.At(5, 5)
	weak := limitedTerrain{TerrainMap: openTerrain(),
		loads: map[cell.ID]float32{middle: ropeLoad}}
	gapped := limitedTerrain{TerrainMap: openTerrain(),
		gaps: map[cell.ID]cell.Gap{middle: {Width: gapWidth}}}
	gapped.Set(middle, cell.Kind{Pace: 1, Allows: cell.Land, Solid: true})
	for _, tc := range []struct {
		name    string
		terrain limitedTerrain
		body    Body
		held    bool
	}{
		{"a light body by a weak floor", weak, Body{Size: box(25), Weight: 80}, false},
		{"a heavy body by a weak floor", weak, Body{Size: box(25), Weight: 1500}, true},
		{"a wide body by a gap", gapped, Body{Size: box(25)}, true},
		{"a long body narrower than the gap by it", gapped, Body{Size: geom.NewVec(25, 4)},
			false},
	} {
		f := New(g, tc.terrain, nil, &cell.MultipleOccupancy{}).WithLevels(flat{tc.terrain})
		f.begin(cell.Land)
		fp := f.footprintOf(tc.body)
		for _, c := range []cell.ID{g.At(4, 4), g.At(5, 6), g.At(6, 5)} {
			if got := f.standsOn(fp, c, ground); got == tc.held {
				t.Errorf("%s: stands by it on %v %v, want %v", tc.name, c, got, !tc.held)
			}
		}
		if !f.standsOn(fp, g.At(2, 2), ground) {
			t.Errorf("%s: stands nowhere near it", tc.name)
		}
	}
}

// a cell changed has the footprint read anew on the nodes whose windows hold it alone, odd and
// even: what lies past them is left as it was, and what lies within as a footprint laid afresh
// has it
func TestFootprint_ACellChangedIsReadAnewOnTheNodesWhoseWindowsHoldItAlone(t *testing.T) {
	for name, g := range map[string]grid.Grid{
		"square": grid.DefaultGrids{}.Square(16, 16, 10),
		"hex":    grid.DefaultGrids{}.Hex(16, 16, 10),
		"torus":  wrapped(grid.DefaultGrids{}.Square(16, 16, 10)),
	} {
		t.Run(name, func(t *testing.T) {
			terrain := passingTerrain()
			f := New(g, terrain, nil, &cell.MultipleOccupancy{}).WithLevels(flat{terrain})
			for _, body := range []Body{big, {Size: box(45)}, {Size: box(15)}, {Size: box(35)}} {
				f.begin(cell.Land)
				fp := f.footprintOf(body)
				changed := g.At(0, 7)
				near := map[cell.ID]bool{}
				ring, on := f.rim(changed, fp.turned, nil, nil)
				for j, c := range ring {
					if on[j] {
						near[c] = true
					}
				}
				// a mark the fix would overwrite, past those nodes where it must not
				g.EachCell(func(c cell.ID) {
					if o, _ := g.Ordinal(c); !near[c] {
						fp.stands[2*o] = !fp.stands[2*o]
					}
				})
				if terrain.Kind(changed).Admits(cell.Land) {
					terrain.Set(changed, wall)
				} else {
					terrain.Set(changed, cell.Kind{Pace: 1, Allows: cell.Land})
				}
				f.begin(cell.Land)
				if f.footprintOf(body) != fp {
					t.Fatalf("side %d: the footprint asked again is another", fp.side)
				}
				fresh := New(g, terrain, nil, &cell.MultipleOccupancy{}).WithLevels(flat{terrain})
				fresh.begin(cell.Land)
				want := fresh.footprintOf(body)
				g.EachCell(func(c cell.ID) {
					o, _ := g.Ordinal(c)
					switch {
					case near[c] && fp.stands[2*o] != want.stands[2*o]:
						t.Errorf("side %d: %v within it %v, laid afresh %v", fp.side, c,
							fp.stands[2*o], want.stands[2*o])
					case !near[c] && fp.stands[2*o] == want.stands[2*o]:
						t.Errorf("side %d: %v past it read anew", fp.side, c)
					}
				})
				// back as it was for the next body
				f.layPrint(fp)
			}
		})
	}
}

// a footprint kept to the board's passage follows the board's changes: a wall in the middle of
// the one passage a body fits shuts it, the wall taken away opens it again
func TestFind_ABodyBiggerThanACellFindsTheBoardAsItChanges(t *testing.T) {
	g, terrain := narrowed(1, 6, 7, 8)
	f := New(g, terrain, nil, &cell.MultipleOccupancy{}).WithLevels(flat{terrain})
	from, to, middle := g.At(2, 7), g.At(17, 7), g.At(10, 6)
	var steps [64]cell.ID
	for k, open := range []bool{true, false, true} {
		if open {
			terrain.Set(middle, cell.Kind{Pace: 1, Allows: cell.Land})
		} else {
			terrain.Set(middle, wall)
		}
		n, ok := f.Find(uid.UID64(1), cell.Land, big, from, to, Either, steps[:])
		fresh := New(g, terrain, nil, &cell.MultipleOccupancy{}).WithLevels(flat{terrain})
		var want [64]cell.ID
		m, wok := fresh.Find(uid.UID64(1), cell.Land, big, from, to, Either, want[:])
		if ok != open || ok != wok || !slices.Equal(steps[:n], want[:m]) {
			t.Errorf("change %d, the passage open %v: found %v by %v, afresh %v by %v", k, open,
				ok, steps[:n], wok, want[:m])
		}
	}
}

// a body bigger than a cell searched for, the board changing between, allocates nothing once its
// footprint is laid and its scratch grown
func TestFind_ABodyBiggerThanACellAllocatesNothingOnceLaid(t *testing.T) {
	g, terrain := narrowed(1, 6, 7, 8)
	f := New(g, terrain, nil, &cell.MultipleOccupancy{}).WithLevels(flat{terrain})
	from, to, middle := g.At(2, 7), g.At(17, 7), g.At(10, 1)
	var steps [64]cell.ID
	search := func() {
		if terrain.Kind(middle).Admits(cell.Land) {
			terrain.Set(middle, wall)
		} else {
			terrain.Set(middle, cell.Kind{Pace: 1, Allows: cell.Land})
		}
		if _, ok := f.Find(uid.UID64(1), cell.Land, big, from, to, Either, steps[:]); !ok {
			t.Fatal("no route by the wide passage")
		}
	}
	search()
	search()
	if allocs := testing.AllocsPerRun(10, search); allocs != 0 {
		t.Errorf("a search as the board changes allocates %v times; want none", allocs)
	}
}

// a field filled for a body bigger than a cell costs from every cell what a search from it costs,
// over the passages it fits alone
func TestField_ABodyBiggerThanACellCostsWhatItsSearchCosts(t *testing.T) {
	g, terrain := narrowed(1, 3, 4, 6, 7, 8)
	f := New(g, terrain, nil, &cell.MultipleOccupancy{}).WithLevels(flat{terrain})
	goal := g.At(17, 2)
	var field Field
	var cells []cell.ID
	g.EachCell(func(c cell.ID) { cells = append(cells, c) })
	f.FillField(&field, goal, Either, cell.Land, big, cells, float32(math.Inf(1)))
	reached := 0
	for _, c := range cells {
		var steps [256]cell.ID
		n, ok := f.Find(uid.UID64(1), cell.Land, big, c, goal, Either, steps[:])
		cost, on := field.Cost(c, Either)
		if c == goal {
			continue
		}
		if on != ok {
			t.Fatalf("%v: the field reached it %v, a search %v", c, on, ok)
		}
		if ok {
			reached++
			if price := priceOf(t, f, c, steps[:n], f.Decks()); !near(price, cost) {
				t.Errorf("%v: the field costs %v, a search %v", c, cost, price)
			}
		}
	}
	if reached == 0 {
		t.Error("no cell reached")
	}
}

// a terrain some of whose cells' bands carry a railing keeping Land off, on their level
type railedTerrain struct {
	*cell.TerrainMap
	rails map[cell.ID]bool // by cell, whether its railing is raised
}

func (r railedTerrain) Railed(c cell.ID, d cell.Domain, deck bool) bool {
	raised, ok := r.rails[c]
	return ok && raised == deck && d&cell.Land != 0
}

// a railing along a band on a body's level holds a body bigger than a cell off the cells round
// it; one on the decks' level, over it, does not
func TestFootprint_ARailingOnItsLevelHoldsABodyOffTheCellsRoundIt(t *testing.T) {
	g := grid.DefaultGrids{}.Square(10, 10, 10)
	for _, raised := range []bool{false, true} {
		terrain := railedTerrain{openTerrain(), map[cell.ID]bool{g.At(5, 5): raised}}
		f := New(g, terrain, nil, &cell.MultipleOccupancy{}).WithLevels(flat{terrain})
		f.begin(cell.Land)
		fp := f.footprintOf(big)
		if got := f.standsOn(fp, g.At(4, 4), ground); got != raised {
			t.Errorf("raised %v: stands by the railing %v, want %v", raised, got, raised)
		}
		if !f.standsOn(fp, g.At(2, 2), ground) {
			t.Errorf("raised %v: stands nowhere near it", raised)
		}
	}
}

// mazed is g walled down columns 15, 31 and 47 but for passages, in rows 4 to 6 and 20, 10 and 11
// and 24 to 26, 14 to 16 and 2, and a pocket walled round x and y 52 to 60, entered by rows 25
// and 26: passages three cells wide and narrower, the pocket's mouth two
func mazed(g grid.Grid) *cellsPassing {
	terrain := passingTerrain()
	open := map[uint32][]uint32{15: {4, 5, 6, 20}, 31: {10, 11, 24, 25, 26}, 47: {14, 15, 16, 2}}
	for x, rows := range open {
		for y := range uint32(32) {
			if !slices.Contains(rows, y) {
				terrain.Set(g.At(x, y), wall)
			}
		}
	}
	for k := uint32(52); k <= 60; k++ {
		for _, c := range []cell.ID{g.At(k, 22), g.At(k, 30), g.At(60, k-30)} {
			terrain.Set(c, wall)
		}
		if y := k - 30; y != 25 && y != 26 {
			terrain.Set(g.At(52, y), wall)
		}
	}
	return terrain
}

// pairsAs are count pairs of nodes body stands on, at least apart by the grid's distance, a route
// between them or not
func pairsAs(t *testing.T, f *Finder, body Body, count int, apart float64,
	seed uint64) [][2]cell.ID {
	t.Helper()
	f.begin(cell.Land)
	fp := f.footprintOf(body)
	var cells []cell.ID
	f.grid.EachCell(func(c cell.ID) {
		if fp == nil && f.terrain.Kind(c).Admits(cell.Land) || fp != nil && f.standsOn(fp, c,
			ground) {
			cells = append(cells, c)
		}
	})
	if len(cells) == 0 {
		t.Fatal("the body stands nowhere")
	}
	return pairs(f.grid, standing(cells), count, apart, seed)
}

// standing is a terrain admitting Land on the cells given alone
type standing []cell.ID

func (s standing) Kind(c cell.ID) cell.Kind {
	if slices.Contains(s, c) {
		return cell.Kind{Pace: 1, Allows: cell.Land}
	}
	return cell.Kind{}
}

// a body bigger than a cell climbs a hierarchy of its class wherever a search finds it a route,
// by steps it takes, costing at most a tenth more than the search's, and is reached where it is
// found: through passages it fits and past those it does not, into a pocket whose mouth of two
// cells takes an even footprint and not a wider one
func TestHierarchy_ABodyBiggerThanACellClimbsWhereItIsFound(t *testing.T) {
	for _, tc := range []struct {
		name string
		g    grid.Grid
		// a torus's seams cost the climbs of every class more
		worst float64
	}{
		{"square", grid.DefaultGrids{}.Square(64, 32, 10), 1.10},
		{"hex", grid.DefaultGrids{}.Hex(64, 32, 10), 1.10},
		{"torus", wrapped(grid.DefaultGrids{}.Square(64, 32, 10)), 1.15},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g := tc.g
			terrain := mazed(g)
			f := New(g, terrain, nil, &cell.MultipleOccupancy{}).WithLevels(flat{terrain})
			h := builtWhole(t, f, land, 16, small, even, big)
			var climbed, found [1024]cell.ID
			climbs, refused := 0, 0
			pocket := [2]cell.ID{g.At(24, 8), g.At(56, 26)}
			for _, pr := range append(pairsAs(t, f, big, 64, 32, 5), pocket) {
				for _, body := range []Body{even, big} {
					from, to := pr[0], pr[1]
					m, fok := f.Find(1, cell.Land, body, from, to, Either, found[:])
					if pr == pocket && fok != (f.sideOf(body) == 2) {
						t.Errorf("%v into the pocket found %v; want an even footprint alone",
							body.Size, fok)
					}
					search := 0.0
					if fok {
						search = priceOf(t, f, from, found[:m], f.Decks())
					}
					n, cok := f.Climb(h, 1, cell.Land, body, from, to, Either, climbed[:])
					if cok != fok {
						t.Fatalf("%v from %v to %v: climbed %v, found %v", body.Size, from, to, cok,
							fok)
					}
					if reached := f.Reaches(h, cell.Land, body, from, to); reached != fok {
						t.Fatalf("%v from %v to %v: reached %v, found %v", body.Size, from, to,
							reached, fok)
					}
					if !cok {
						refused++
						continue
					}
					climbs++
					decks := f.Decks()
					if !takenBy(f, body, from, to, climbed[:n]) {
						t.Fatalf("%v from %v to %v: the climb %v takes a step the body does not",
							body.Size, from, to, climbed[:n])
					}
					ratio := priceOf(t, f, from, climbed[:n], decks) / search
					if ratio > tc.worst {
						t.Errorf("%v from %v to %v: the climb costs %.3f of the search's",
							body.Size, from, to, ratio)
					}
				}
			}
			if climbs == 0 || refused == 0 {
				t.Errorf("%d climbed and %d with no route; want both", climbs, refused)
			}
		})
	}
}

// a cell changed stales the clusters of the cells round it up to a step past the reach of the
// hierarchy's largest class, and the clusters built again stand and climb as a hierarchy built
// afresh over the board as it is
func TestHierarchy_ACellChangedStalesTheRingPastTheLargestReach(t *testing.T) {
	g := grid.DefaultGrids{}.Square(64, 64, 16)
	wide := Body{Size: box(40)}
	for _, tc := range []struct {
		name    string
		classes []Body
		x, y    uint32
		marked  int
	}{
		{"within a cell, two from a border", []Body{{}}, 22, 20, 1},
		{"reaching over a cell, two from a border", []Body{{}, wide}, 22, 20, 2},
		{"reaching over a cell, two from a corner", []Body{{}, wide}, 22, 22, 4},
		{"reaching over a cell, three from a border", []Body{{}, wide}, 21, 20, 1},
		{"two cells a side, two from a border", []Body{{}, Body{Size: box(24)}}, 22, 20, 2},
		{"two cells a side, two from a corner", []Body{{}, Body{Size: box(24)}}, 22, 22, 4},
	} {
		t.Run(tc.name, func(t *testing.T) {
			terrain := passingTerrain()
			f := New(g, terrain, nil, &cell.MultipleOccupancy{}).WithLevels(flat{terrain})
			h := builtWhole(t, f, land, 8, tc.classes...)
			c := g.At(tc.x, tc.y)
			for _, kind := range []cell.Kind{wall, {Pace: 1, Allows: cell.Land}} {
				since := terrain.Passage()
				terrain.Set(c, kind)
				if marked := f.Stale(h, since); marked != tc.marked {
					t.Errorf("%d clusters marked, want %d", marked, tc.marked)
				}
				f.Build(h, math.MaxInt)
				fresh := New(g, terrain, nil, &cell.MultipleOccupancy{}).WithLevels(flat{terrain})
				want := builtWhole(t, fresh, land, 8, tc.classes...)
				if !slices.Equal(h.on, want.on) {
					t.Error("the classes stand otherwise than over a hierarchy built afresh")
				}
				body := tc.classes[len(tc.classes)-1]
				var got, wanted [256]cell.ID
				for _, pr := range pairsAs(t, f, body, 24, 24, 9) {
					n, ok := f.Climb(h, 1, cell.Land, body, pr[0], pr[1], Either, got[:])
					m, wok := fresh.Climb(want, 1, cell.Land, body, pr[0], pr[1], Either,
						wanted[:])
					if ok != wok || !slices.Equal(got[:n], wanted[:m]) {
						t.Errorf("from %v to %v: climbed %v %v, afresh %v %v", pr[0], pr[1], ok,
							got[:n], wok, wanted[:m])
					}
				}
			}
		})
	}
}

// a hierarchy serving a class bigger than a cell, a cell changed, staled and built again,
// allocates nothing once its footprint and scratch are grown
func TestHierarchy_ABodyBiggerThanACellsHierarchyBuildsAgainAllocatingNothing(t *testing.T) {
	g := grid.DefaultGrids{}.Square(64, 32, 10)
	terrain := mazed(g)
	f := New(g, terrain, nil, &cell.MultipleOccupancy{}).WithLevels(flat{terrain})
	h := builtWhole(t, f, land, 8, small, big)
	c := g.At(40, 8)
	rebuild := func() {
		since := terrain.Passage()
		if terrain.Kind(c).Admits(cell.Land) {
			terrain.Set(c, wall)
		} else {
			terrain.Set(c, cell.Kind{Pace: 1, Allows: cell.Land})
		}
		if f.Stale(h, since); h.Ready() {
			t.Fatal("ready as a cell changed")
		}
		f.Build(h, math.MaxInt)
	}
	rebuild()
	rebuild()
	if allocs := testing.AllocsPerRun(4, rebuild); allocs != 0 {
		t.Errorf("a cluster built again allocates %v times; want none", allocs)
	}
}
