package pathfind

import (
	"slices"
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/uid"
)

// a terrain some of whose cells limit whom they take beyond their kinds, for Land: a floor's
// load, on the decks' level where raised, and a wall's gap
type limitedTerrain struct {
	*cell.TerrainMap
	loads  map[cell.ID]float32
	raised bool
	gaps   map[cell.ID]cell.Gap
}

func (l limitedTerrain) Limited() bool { return len(l.loads) > 0 || len(l.gaps) > 0 }

func (l limitedTerrain) Limits(c cell.ID, d cell.Domain, deck bool) (float32, cell.Gap) {
	if d&cell.Land == 0 {
		return 0, cell.Gap{}
	}
	var load float32
	if deck == l.raised {
		load = l.loads[c]
	}
	return load, l.gaps[c]
}

const ropeLoad = 100

// the river of newBridged, a rope over it along row 1 bearing ropeLoad, the ford at its foot
func roped() (grid.Grid, limitedTerrain) {
	b := newBridged()
	return b.g, limitedTerrain{TerrainMap: b.TerrainMap,
		loads: map[cell.ID]float32{b.g.At(2, 1): ropeLoad}}
}

func TestFind_ARopeBearsTheLightAndTheHeavyGoRoundByTheFord(t *testing.T) {
	g, terrain := roped()
	rope := g.At(2, 1)
	for name, f := range map[string]*Finder{
		"plain":     New(g, terrain, nil, &cell.MultipleOccupancy{}),
		"by levels": New(g, terrain, nil, &cell.MultipleOccupancy{}).WithLevels(flat{terrain}),
	} {
		for _, tc := range []struct {
			weight float32
			across bool
		}{{0, true}, {80, true}, {ropeLoad, true}, {ropeLoad + 1, false}, {1500, false}} {
			var steps [64]cell.ID
			body := Body{Weight: tc.weight}
			n, ok := f.Find(uid.UID64(1), cell.Land, body, g.At(0, 1), g.At(4, 1), Either,
				steps[:])
			if !ok {
				t.Errorf("%s, %v kg: no route, want one by the rope or the ford", name, tc.weight)
				continue
			}
			if got := slices.Contains(steps[:n], rope); got != tc.across {
				t.Errorf("%s, %v kg: over the rope %v, want %v", name, tc.weight, got, tc.across)
			}
			if _, ok := f.Steps(cell.Land, body, g.At(1, 1), rope); ok != tc.across {
				t.Errorf("%s, %v kg: steps onto the rope %v, want %v", name, tc.weight, ok,
					tc.across)
			}
		}
	}
}

// the deck of bridgedRiver over the river of moated(11) bearing ropeLoad
func weakDeck(g grid.Grid, river *cell.TerrainMap) limitedTerrain {
	terrain := limitedTerrain{TerrainMap: river, raised: true, loads: map[cell.ID]float32{}}
	for x := uint32(10); x <= 12; x++ {
		terrain.loads[g.At(x, 4)] = ropeLoad
	}
	return terrain
}

// a deck the only way over the river to the island: whom it does not bear reaches nothing past
// it
func TestFind_ARaisedDeckBearsTheLightAlone(t *testing.T) {
	g, river := moated(11)
	f := New(g, weakDeck(g, river), nil, &cell.MultipleOccupancy{}).
		WithLevels(bridgedRiver{g, river})
	from, island := g.At(1, 4), g.At(14, 4)
	for _, tc := range []struct {
		weight float32
		over   bool
	}{{0, true}, {80, true}, {1500, false}} {
		body := Body{Step: 1, Height: 2, Weight: tc.weight}
		var steps [64]cell.ID
		n, ok := f.Find(uid.UID64(1), cell.Land, body, from, island, Either, steps[:])
		if ok != tc.over {
			t.Errorf("%v kg: a route to the island %v by %v, want %v", tc.weight, ok,
				steps[:n], tc.over)
		}
		if _, ok := f.Steps(cell.Land, body, g.At(9, 4), g.At(10, 4)); ok != tc.over {
			t.Errorf("%v kg: steps onto the deck %v, want %v", tc.weight, ok, tc.over)
		}
	}
}

const (
	gapWidth  = 6
	gapHeight = 2
)

// a wall down column 3 but its last row, with a gap gapWidth wide and gapHeight tall in row 1;
// the cell of the gap admits whom the wall keeps off, as a board's does
func gapped(g grid.Grid) limitedTerrain {
	terrain := openTerrain()
	for y := range uint32(4) {
		terrain.Set(g.At(3, y), cell.Kind{Pace: 1, Solid: true})
	}
	gap := g.At(3, 1)
	terrain.Set(gap, cell.Kind{Pace: 1, Allows: cell.Land, Solid: true})
	return limitedTerrain{TerrainMap: terrain,
		gaps: map[cell.ID]cell.Gap{gap: {Width: gapWidth, Height: gapHeight}}}
}

func TestFind_AGapLetsThroughWhomItFitsAndTheRestGoRound(t *testing.T) {
	g := grid.DefaultGrids{}.Square(7, 5, 10)
	terrain := gapped(g)
	gap := g.At(3, 1)
	for name, f := range map[string]*Finder{
		"plain":     New(g, terrain, nil, &cell.MultipleOccupancy{}),
		"by levels": New(g, terrain, nil, &cell.MultipleOccupancy{}).WithLevels(flat{terrain}),
	} {
		for _, tc := range []struct {
			name    string
			body    Body
			through bool
		}{
			{"no box", Body{}, true},
			{"small", Body{Size: geom.NewVec(4, 4), Height: 1.5}, true},
			{"as wide and as tall", Body{Size: geom.NewVec(gapWidth, gapWidth),
				Height: gapHeight}, true},
			{"wide", Body{Size: geom.NewVec(8, 8)}, false},
			{"long along the step", Body{Size: geom.NewVec(8, 4)}, true},
			{"long across the step", Body{Size: geom.NewVec(4, 8)}, false},
			{"tall", Body{Size: geom.NewVec(4, 4), Height: 3}, false},
		} {
			var steps [64]cell.ID
			n, ok := f.Find(uid.UID64(1), cell.Land, tc.body, g.At(0, 1), g.At(6, 1), Either,
				steps[:])
			if !ok {
				t.Errorf("%s, %s: no route, want one through the gap or round the wall", name,
					tc.name)
				continue
			}
			if got := slices.Contains(steps[:n], gap); got != tc.through {
				t.Errorf("%s, %s: through the gap %v, want %v", name, tc.name, got, tc.through)
			}
			for _, step := range [][2]cell.ID{{g.At(2, 1), gap}, {gap, g.At(4, 1)}} {
				if _, ok := f.Steps(cell.Land, tc.body, step[0], step[1]); ok != tc.through {
					t.Errorf("%s, %s: steps %v %v, want %v", name, tc.name, step, ok,
						tc.through)
				}
			}
		}
	}
}

// a body long along one way through a gap standing alone in an open field: every step through
// it is taken back alike, so no step leads one way only
func TestFind_AGapIsPassedEitherWayAlike(t *testing.T) {
	g := grid.DefaultGrids{}.Square(5, 5, 10)
	terrain := limitedTerrain{TerrainMap: openTerrain(), gaps: map[cell.ID]cell.Gap{}}
	gap := g.At(2, 2)
	terrain.Set(gap, cell.Kind{Pace: 1, Allows: cell.Land, Solid: true})
	terrain.gaps[gap] = cell.Gap{Width: gapWidth}
	f := New(g, terrain, nil, &cell.MultipleOccupancy{}).WithLevels(flat{terrain})
	long := Body{Size: geom.NewVec(8, 4)}
	through := 0
	for i := range g.Directions() {
		n, _ := g.Toward(gap, i)
		_, in := f.Steps(cell.Land, long, n, gap)
		_, out := f.Steps(cell.Land, long, gap, n)
		if in != out {
			t.Errorf("into the gap from %v %v, out of it to there %v", n, in, out)
		}
		if in {
			through++
		}
	}
	if through != 2 {
		t.Errorf("%d ways through the gap, want the two along the body", through)
	}
}

// a corner is cut past a gap whom it lets through whichever way it turns, as collision does
func TestFind_ACornerIsCutPastAGapWhomItLetsThrough(t *testing.T) {
	g := grid.DefaultGrids{}.Square(5, 5, 10)
	terrain := limitedTerrain{TerrainMap: openTerrain(), gaps: map[cell.ID]cell.Gap{}}
	gap := g.At(2, 2)
	terrain.Set(gap, cell.Kind{Pace: 1, Allows: cell.Land, Solid: true})
	terrain.gaps[gap] = cell.Gap{Width: gapWidth}
	f := New(g, terrain, nil, &cell.MultipleOccupancy{}).WithLevels(flat{terrain})
	from, to := g.At(1, 2), g.At(2, 1)
	for _, tc := range []struct {
		name string
		size geom.Vec
		cut  bool
	}{
		{"small", geom.NewVec(4, 4), false},
		{"long", geom.NewVec(4, 8), false},
		{"wide", geom.NewVec(8, 8), true},
	} {
		f.body = Body{Size: tc.size}
		f.begin(cell.Land)
		if got := f.cutOff(f.flanks(from, to)); got != tc.cut {
			t.Errorf("%s: the corner past the gap cut off %v, want %v", tc.name, got, tc.cut)
		}
	}
}

// a class the deck does not bear stands on none of it: the island is out of its reach, as a
// search finds it, and the class it bears climbs over as a search goes
func TestHierarchy_ALimitHoldsItsClassBack(t *testing.T) {
	g, river := moated(11)
	f := New(g, weakDeck(g, river), nil, &cell.MultipleOccupancy{}).
		WithLevels(bridgedRiver{g, river})
	from, island := g.At(1, 4), g.At(14, 4)
	for _, tc := range []struct {
		weight float32
		over   bool
	}{{80, true}, {1500, false}} {
		class := Body{Step: 1, Height: 2, Weight: tc.weight}
		h := builtWhole(t, f, Key{Domain: cell.Land, Body: class}, 4)
		if reached := f.Reaches(h, cell.Land, class, from, island); reached != tc.over {
			t.Errorf("%v kg: the island reached %v, want %v", tc.weight, reached, tc.over)
		}
		var climbed, found [64]cell.ID
		n, ok := f.Climb(h, 1, cell.Land, class, from, island, Either, climbed[:])
		m, fok := f.Find(1, cell.Land, class, from, island, Either, found[:])
		if ok != tc.over || fok != tc.over || !slices.Equal(climbed[:n], found[:m]) {
			t.Errorf("%v kg: climbed %v %v, found %v %v; want both %v and alike", tc.weight,
				ok, climbed[:n], fok, found[:m], tc.over)
		}
		deck := node{g.At(11, 4), deck}
		if h.stands(deck) != tc.over {
			t.Errorf("%v kg: the deck over the river held %v, want %v", tc.weight,
				h.stands(deck), tc.over)
		}
	}
}

func TestFind_AStepTouchingNoLimitReadsNone(t *testing.T) {
	g := grid.DefaultGrids{}.Square(7, 5, 10)
	terrain := gapped(g)
	terrain.gaps = nil
	f := New(g, terrain, nil, &cell.MultipleOccupancy{}).WithLevels(flat{terrain})
	f.begin(cell.Land)
	if f.limited {
		t.Fatal("a board limiting nothing read as limited")
	}
	var steps [64]cell.ID
	if _, ok := f.Find(uid.UID64(1), cell.Land, Body{}, g.At(0, 1), g.At(6, 1), Either,
		steps[:]); !ok {
		t.Fatal("no route round the wall")
	}
	for i := range f.reads {
		if r := &f.reads[i]; r.search == f.search && r.has&(hasLimits|hasLimits<<1) != 0 {
			t.Fatalf("the limits of %v read on a board limiting nothing", r.c)
		}
	}
}
