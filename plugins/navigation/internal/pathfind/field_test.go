package pathfind

import (
	"maps"
	"math"
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
)

var grass = cell.Kind{Name: cell.Named("grass"), Pace: 1, Allows: cell.Land}

// grass with rocks, forest and sand scattered over it, a road along a row and one slantwise,
// linked cell to cell
func scattered(g grid.Grid, along, slantwise int) (*board.BoardRead, *board.BoardWrite) {
	brd, brdW := board.NewBoard(g)
	brdW.SetAll(grass)
	rng := rand.New(rand.NewPCG(13, 29))
	brd.EachCell(func(c cell.ID) {
		switch r := rng.IntN(100); {
		case r < 14:
			brdW.Set(c, cell.Kind{Name: cell.Named("rock"), Pace: 1, Solid: true})
		case r < 26:
			brdW.Set(c, cell.Kind{Name: cell.Named("forest"), Pace: 0.4, Allows: cell.Land})
		case r < 32:
			brdW.Set(c, cell.Kind{Name: cell.Named("sand"), Pace: 2.0 / 3,
				Allows: cell.Land | cell.Air}.Paced(cell.Land, 0.8))
		}
	})
	road := cell.Kind{Name: cell.Named("road"), Pace: 1, Allows: cell.Land}
	for _, way := range [2]struct {
		from cell.ID
		dir  int
	}{{g.At(1, 4), along}, {g.At(2, 17), slantwise}} {
		cells := []cell.ID{way.from}
		for c := way.from; len(cells) < 16; cells = append(cells, c) {
			next, ok := g.Toward(c, way.dir)
			if !ok {
				break
			}
			c = next
		}
		for _, c := range cells {
			brdW.Set(c, grass)
		}
		roadOver(brd, brdW, road, cells...)
	}
	return brd, brdW
}

// the hills of rolling over g, a height a cell by its coordinates
func hillsOver(g grid.Grid) rolling {
	hills := rolling{grid: g, heights: make([]float64, g.CellCount())}
	g.EachCell(func(c cell.ID) {
		x, y, _ := g.Coords(c)
		o, _ := g.Ordinal(c)
		hills.heights[o] = 6 * (1 + math.Sin(float64(x)/3)*math.Cos(float64(y)/4))
	})
	return hills
}

// what the route from from along steps costs, step by step as Price prices it
func priceOf(t *testing.T, f *Finder, from cell.ID, steps []cell.ID, decks uint64) float64 {
	t.Helper()
	sum, prev := 0.0, from
	for k, s := range steps {
		level := cell.OnGround
		if k < 64 && decks>>k&1 != 0 {
			level = cell.OnDeck
		}
		p, ok := f.Price(prev, s, f.KindAt(s, level), level, cell.Land)
		if !ok {
			t.Fatalf("the step %v->%v of the route from %v is refused", prev, s, from)
		}
		sum, prev = sum+p, s
	}
	return sum
}

func near(a float64, b float32) bool { return math.Abs(a-float64(b)) <= 1e-4*a }

// a field filled from every cell a unit stands on: each cell costs what the route Find gives
// from it costs, and a walk off the field from it as much
func fieldMatchesFind(t *testing.T, g grid.Grid, terrain cell.Terrain, pacing Pacing,
	goal cell.ID) {
	t.Helper()
	f := New(g, terrain, pacing, &cell.MultipleOccupancy{}).WithLevels(flat{terrain})
	var seeds []cell.ID
	g.EachCell(func(c cell.ID) {
		if terrain.Kind(c).Admits(cell.Land) {
			seeds = append(seeds, c)
		}
	})
	var field Field
	reached := f.FillField(&field, goal, Either, cell.Land, Body{}, seeds, 1)
	var dst [1024]cell.ID
	routes := 0
	for _, c := range seeds {
		if c == goal {
			continue
		}
		cost, reachedC := field.Cost(c, Either)
		n, found := f.Find(1, cell.Land, Body{}, c, goal, Either, dst[:])
		if found != reachedC {
			t.Errorf("from %v: Find found a route %v, the field reached it %v", c, found,
				reachedC)
			continue
		}
		if !found {
			continue
		}
		routes++
		if p := priceOf(t, f, c, dst[:n], f.Decks()); !near(p, cost) {
			t.Errorf("from %v: the field costs %v, Find's route %v", c, cost, p)
		}
		n, walked := f.Walk(&field, 1, cell.Land, Body{}, c, goal, Either, dst[:])
		if !walked || n == 0 || dst[n-1] != goal {
			t.Errorf("from %v: walked %v to %v, want the way to %v", c, walked, dst[:n], goal)
			continue
		}
		if p := priceOf(t, f, c, dst[:n], f.Decks()); !near(p, cost) {
			t.Errorf("from %v: the field costs %v, the walk off it %v", c, cost, p)
		}
	}
	if routes < len(seeds)/2 || reached != routes+1 {
		t.Fatalf("%d routes of %d cells, %d seeds reached; want most cells routed, each reached",
			routes, len(seeds), reached)
	}
}

func TestField_EveryCellCostsWhatASearchFromItCosts(t *testing.T) {
	g := grid.DefaultGrids{}.Square(24, 24, 10)
	brd, brdW := scattered(g, 3, 5)
	brdW.Set(g.At(17, 15), grass)
	fieldMatchesFind(t, g, brd, nil, g.At(17, 15))
}

// the slope of a step is priced from where it is taken: a mirror priced the other way round would
// cost what climbing down costs to climb up
func TestField_EveryCellCostsWhatASearchFromItCostsOverARelief(t *testing.T) {
	for name, tc := range map[string]struct {
		g                grid.Grid
		along, slantwise int
	}{
		"square": {grid.DefaultGrids{}.Square(24, 24, 10), 3, 5},
		"hex":    {grid.DefaultGrids{}.Hex(24, 24, 10), 0, 1},
	} {
		t.Run(name, func(t *testing.T) {
			brd, brdW := scattered(tc.g, tc.along, tc.slantwise)
			brdW.Set(tc.g.At(17, 15), grass)
			fieldMatchesFind(t, tc.g, brd, hillsOver(tc.g), tc.g.At(17, 15))
		})
	}
}

func TestField_AWalkArrivesOnTheGoalsLevel(t *testing.T) {
	v := newValley()
	at := func(x, y uint32) cell.ID { return v.g.At(x, y) }
	f := New(v.g, v.terrain, nil, &cell.MultipleOccupancy{}).WithLevels(v)
	for _, tc := range []struct {
		name       string
		from, goal cell.ID
		feet       float32
	}{
		{"from the rim onto the deck's end", at(0, 2), at(1, 2), 10},
		{"from the floor to under the deck or onto it", at(1, 0), at(1, 2), 0},
		{"across the river over the deck", at(0, 2), at(4, 2), 10},
		{"from the far floor round to the deck", at(3, 4), at(1, 2), 0},
	} {
		walker := Body{Feet: tc.feet, Step: 1, Height: 2}
		for _, end := range []Level{Either, Ground, Deck} {
			var field Field
			f.FillField(&field, tc.goal, end, cell.Land, Body{Step: 1, Height: 2},
				[]cell.ID{tc.from}, 1.5)
			var walked, found [64]cell.ID
			n, ok := f.Walk(&field, 1, cell.Land, walker, tc.from, tc.goal, end, walked[:])
			decks := f.Decks()
			m, fok := f.Find(1, cell.Land, walker, tc.from, tc.goal, end, found[:])
			if !ok || !fok || n != m || decks != f.Decks() || walked[n-1] != tc.goal {
				t.Errorf("%s, to end on %v: walked %v %v on decks %b, found %v %v on %b",
					tc.name, end, ok, walked[:n], decks, fok, found[:m], f.Decks())
				continue
			}
			_, _, _, decked := v.Deck(tc.goal)
			onDeck := decks>>(n-1)&1 != 0
			if decked && (end == Deck && !onDeck || end == Ground && onDeck) {
				t.Errorf("%s, to end on %v: the walk ends on the deck %v", tc.name, end, onDeck)
			}
		}
	}
}

func TestField_ACellTheFieldDidNotReachWalksNowhere(t *testing.T) {
	g := grid.DefaultGrids{}.Square(20, 20, 10)
	terrain := openTerrain()
	walled := g.At(2, 2)
	for _, n := range g.Neighbors(walled) {
		terrain.Set(n, cell.Kind{Pace: 1, Solid: true})
	}
	f := New(g, terrain, nil, &cell.MultipleOccupancy{}).WithLevels(flat{terrain})
	goal, seed := g.At(10, 10), g.At(12, 10)
	var dst [64]cell.ID
	for name, tc := range map[string]struct {
		seeds []cell.ID
		from  cell.ID
	}{
		"walled in, a seed":             {[]cell.ID{seed, walled}, walled},
		"far past the seeds":            {[]cell.ID{seed}, g.At(0, 19)},
		"a step past the farthest seed": {[]cell.ID{seed}, g.At(13, 10)},
	} {
		var field Field
		reached := f.FillField(&field, goal, Either, cell.Land, Body{}, tc.seeds, 1)
		if reached != 1 {
			t.Errorf("%s: %d seeds reached, want the one not walled in", name, reached)
		}
		if field.Reached(tc.from) {
			t.Errorf("%s: %v reached", name, tc.from)
		}
		if n, ok := f.Walk(&field, 1, cell.Land, Body{}, tc.from, goal, Either, dst[:]); ok {
			t.Errorf("%s: walked %v", name, dst[:n])
		}
		if n, ok := f.Walk(&field, 1, cell.Land, Body{}, seed, goal, Either, dst[:]); !ok ||
			n != 2 {
			t.Errorf("%s, from the seed: walked %v %v, want the 2 steps to the goal", name, ok,
				dst[:n])
		}
	}
	if n, ok := f.Walk(&Field{}, 1, cell.Land, Body{}, seed, goal, Either, dst[:]); ok {
		t.Errorf("off a field never filled: walked %v", dst[:n])
	}
}

func TestField_TheLastLegReachesTheMembersOwnCell(t *testing.T) {
	g := grid.DefaultGrids{}.Square(24, 24, 10)
	brd, brdW := scattered(g, 3, 5)
	f := New(g, brd, nil, &cell.MultipleOccupancy{}).WithLevels(flat{brd})
	body := Body{Step: 1, Height: 2}
	goal, from := g.At(17, 15), g.At(2, 2)
	targets := map[string]cell.ID{
		"short of the goal": g.At(14, 15),
		"past the goal":     g.At(20, 15),
		"aside of the goal": g.At(17, 18),
	}
	for _, c := range append([]cell.ID{goal, from}, slices.Collect(maps.Values(targets))...) {
		brdW.Set(c, grass)
	}
	for name, target := range targets {
		var field Field
		f.FillField(&field, goal, Either, cell.Land, body, []cell.ID{from}, 1.5)
		var dst [128]cell.ID
		before := f.Searches()
		n, ok := f.Walk(&field, 1, cell.Land, body, from, target, Either, dst[:])
		if !ok || n == 0 || dst[n-1] != target {
			t.Errorf("%s: walked %v %v, want the way to %v", name, ok, dst[:n], target)
			continue
		}
		if searched := f.Searches() - before; searched != 1 {
			t.Errorf("%s: %d searches, want the last leg's alone", name, searched)
		}
		prev := from
		for _, s := range dst[:n] {
			if _, ok := grid.Way(g, prev, s); !ok {
				t.Errorf("%s: %v steps to %v, no neighbour", name, prev, s)
			}
			if _, ok := f.Steps(cell.Land, body, prev, s); !ok {
				t.Errorf("%s: the step %v->%v is refused", name, prev, s)
			}
			prev = s
		}
	}
}

// over a terrain reading nothing anew, as a board with its cells does
func TestField_AFillAllocatesNothingTwice(t *testing.T) {
	g := grid.DefaultGrids{}.Square(24, 24, 10)
	terrain := openTerrain()
	for y := range uint32(20) {
		terrain.Set(g.At(12, y), cell.Kind{Pace: 1, Solid: true})
	}
	f := New(g, terrain, hillsOver(g), &cell.MultipleOccupancy{}).WithLevels(flat{terrain})
	goal, seeds := g.At(17, 15), []cell.ID{g.At(2, 2), g.At(3, 20)}
	var field Field
	fill := func() {
		if f.FillField(&field, goal, Either, cell.Land, Body{}, seeds, 1.5) != len(seeds) {
			t.Fatal("a seed not reached")
		}
	}
	fill()
	if allocs := testing.AllocsPerRun(10, fill); allocs != 0 {
		t.Errorf("a field filled again allocates %v times; want none", allocs)
	}
	var dst [64]cell.ID
	walk := func() {
		if _, ok := f.Walk(&field, 1, cell.Land, Body{}, seeds[0], g.At(16, 13), Either,
			dst[:]); !ok {
			t.Fatal("no walk from the seed")
		}
	}
	walk()
	if allocs := testing.AllocsPerRun(10, walk); allocs != 0 {
		t.Errorf("a walk allocates %v times; want none", allocs)
	}
}
