package spacing

import (
	"slices"
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/navigation/internal/bodies"
	"github.com/kjkrol/gram/plugins/navigation/internal/pathfind"
	"github.com/kjkrol/gram/plugins/world"
)

// a terrain map counting the passage of each cell as a board does
type passingMap struct {
	*cell.TerrainMap
	cells map[cell.ID]uint64
}

func newPassingMap() passingMap {
	m := passingMap{cell.NewTerrainMap(), map[cell.ID]uint64{}}
	m.SetAll(cell.Kind{Pace: 1, Allows: cell.Land})
	return m
}

func (m passingMap) Set(c cell.ID, k cell.Kind) {
	m.TerrainMap.Set(c, k)
	m.cells[c] = m.Passage()
}

func (m passingMap) CellPassage(c cell.ID) uint64 { return m.cells[c] }

// a board of one level, flat
type flat struct{ terrain cell.Terrain }

func (flat) Deck(cell.ID) (top, under float32, carries cell.Domain, ok bool) {
	return 0, 0, 0, false
}
func (flat) Ground(geom.Vec) float32              { return 0 }
func (flat) Over(geom.Vec) (float32, bool)        { return 0, false }
func (f flat) KindAt(c cell.ID, _ bool) cell.Kind { return f.terrain.Kind(c) }

// units moving on land and in the water alike, a domain of its own
const amphibious = cell.Land | cell.Water

// the classes of the hierarchies' tests: units of boxes 2 to 6 across on land, of 4 amphibious
func testClasses() *pathfind.Classes {
	var kinds []pathfind.Kind
	for side := float32(2); side <= 6; side++ {
		kinds = append(kinds, pathfind.Kind{Domain: cell.Land,
			Body: pathfind.Body{Size: geom.NewVec(side, side)}})
	}
	kinds = append(kinds, pathfind.Kind{Domain: amphibious,
		Body: pathfind.Body{Size: geom.NewVec(4, 4)}})
	var c pathfind.Classes
	c.Set(kinds)
	return &c
}

// hierarchies over an open 16x16 board of four clusters 8 cells a side on one level, a route
// climbed from 8 cells apart, built through the finder given, planning the testClasses
func newHierarchies(t *testing.T) (*Hierarchies, passingMap, grid.Grid, *Finder) {
	t.Helper()
	g := grid.DefaultGrids{}.Square(16, 16, 10)
	terrain := newPassingMap()
	f := NewFinder(g, terrain, nil, OpenOccupancy{}).WithLevels(flat{terrain}).
		WithClasses(testClasses())
	hs := &Hierarchies{}
	if hs.Bind(f, 8, 8); !hs.On() {
		t.Fatal("no hierarchies kept over a blind finder of a board counting its passage")
	}
	return hs, terrain, g, f
}

// settle sweeps hs till nothing is left to stale or build
func settle(t *testing.T, hs *Hierarchies) {
	t.Helper()
	for range 64 {
		if !hs.Due() {
			return
		}
		hs.Sweep()
	}
	t.Fatal("hierarchies never built")
}

// a unit moving in domain, its box side across, its feet anywhere
func unitOf(domain cell.Domain, side float32) bodies.Member {
	m := bodies.Member{Domain: domain, Pos: world.Position{}}
	m.Pos.Size = geom.NewVec(side, side)
	m.Z.Altitude = 3
	return m
}

// the hierarchy a unit moving in domain, its box side across, climbs through
func keyOf(t *testing.T, hs *Hierarchies, domain cell.Domain, side float32) pathfind.Key {
	t.Helper()
	key, ok := hs.Key(unitOf(domain, side))
	if !ok || key != (pathfind.Key{Domain: domain}) {
		t.Fatalf("a unit of %v across in %v climbs through %+v, %v; want its domain's", side,
			domain, key, ok)
	}
	return key
}

func TestHierarchies_NoneKeptButOverABlindLeveledFinderOfABoardCountingItsCells(t *testing.T) {
	g := grid.DefaultGrids{}.Square(8, 8, 10)
	open, passing := cell.NewTerrainMap(), newPassingMap()
	seeing := NewFinder(g, passing, nil, &cell.SingleOccupancy{}).WithLevels(flat{passing})
	uncounted := NewFinder(g, open, nil, OpenOccupancy{}).WithLevels(flat{open})
	plain := NewFinder(g, passing, nil, OpenOccupancy{})
	for name, f := range map[string]*Finder{
		"seeing the others":                       seeing,
		"over a board counting no cell's passage": uncounted,
		"planning without levels":                 plain,
	} {
		var hs Hierarchies
		if hs.Bind(f, 8, 8); hs.On() || hs.Far(g.At(0, 0), g.At(7, 7)) {
			t.Errorf("%s: hierarchies kept", name)
		}
	}
	var hs Hierarchies
	leveled := NewFinder(g, passing, nil, OpenOccupancy{}).WithLevels(flat{passing})
	if hs.Bind(leveled, 0, 8); hs.On() {
		t.Error("hierarchies of regions of no side kept")
	}
}

func TestHierarchies_ARouteIsFarByTheDistanceOfItsEnds(t *testing.T) {
	hs, _, g, _ := newHierarchies(t)
	for _, c := range []struct {
		from, to cell.ID
		far      bool
	}{
		{g.At(0, 0), g.At(7, 0), false},
		{g.At(0, 0), g.At(8, 0), true},
		{g.At(0, 0), g.At(6, 6), true}, // 6 + 6(√2 - 1) apart
		{g.At(0, 0), g.At(5, 5), false},
	} {
		if far := hs.Far(c.from, c.to); far != c.far {
			t.Errorf("%v to %v: far %v, want %v", c.from, c.to, far, c.far)
		}
	}
}

// four clusters, built over Sweeps of a budget below a cluster's work
func TestHierarchies_ADomainsIsTakenOnceExpectedAndBuilt(t *testing.T) {
	hs, _, _, f := newHierarchies(t)
	hs.budget = 3000
	key := keyOf(t, hs, cell.Land, 4)
	if hs.Take(key) != nil {
		t.Fatal("a hierarchy taken before it was asked for")
	}
	hs.Expect(key)
	if hs.Take(key) != nil {
		t.Fatal("a hierarchy taken before an Update built it")
	}
	if hs.Sweep(); hs.Take(key) != nil || !hs.Due() {
		t.Fatal("a hierarchy of four clusters taken, or no more to build, after a Sweep of a " +
			"budget below a cluster's work")
	}
	settle(t, hs)
	h := hs.Take(key)
	if h == nil || h.Key() != key || h.Side() != 8 || !h.Ready() ||
		!slices.Equal(h.Classes(), f.classes.Of(cell.Land)) {
		t.Fatalf("taken once built: %v; want the domain's, of regions 8 a side, serving its "+
			"classes", h)
	}
	hs.Expect(key)
	if hs.Sweep(); hs.Take(key) != h {
		t.Error("asked for again, the domain's hierarchy made anew")
	}
}

// the units of five classes on land climb through one hierarchy, the amphibious through
// another; one whose body no class holds through none
func TestHierarchies_OneADomainServesItsClasses(t *testing.T) {
	hs, _, _, _ := newHierarchies(t)
	for side := float32(2); side <= 6; side += 0.5 {
		hs.Expect(keyOf(t, hs, cell.Land, side))
	}
	hs.Expect(keyOf(t, hs, amphibious, 4))
	if _, ok := hs.Key(unitOf(cell.Land, 7)); ok {
		t.Error("a unit wider than every class of its domain climbs")
	}
	if _, ok := hs.Key(unitOf(cell.Water, 4)); ok {
		t.Error("a unit of a domain no class moves in climbs")
	}
	settle(t, hs)
	if len(hs.entries) != 2 {
		t.Fatalf("%d hierarchies kept, want one a domain asked for", len(hs.entries))
	}
	land, water := hs.Take(pathfind.Key{Domain: cell.Land}),
		hs.Take(pathfind.Key{Domain: amphibious})
	if land == nil || water == nil || len(land.Classes()) != 5 || len(water.Classes()) != 1 {
		t.Errorf("the land's hierarchy %v, the amphibious' %v; want five classes and one", land,
			water)
	}
}

// a Sweep builds the hierarchy asked for most lately of those not ready, the one asked for
// before it waiting till it is
func TestHierarchies_ASweepBuildsTheLatestAskedFirst(t *testing.T) {
	hs, _, _, f := newHierarchies(t)
	hs.budget = 3000
	first, second := keyOf(t, hs, cell.Land, 4), keyOf(t, hs, amphibious, 4)
	hs.Expect(first)
	hs.Sweep()
	hs.Expect(second)
	builds := f.Builds()
	for sweeps := 0; hs.Take(second) == nil; sweeps++ {
		if hs.Take(first) != nil || sweeps == 64 {
			t.Fatalf("after %d Sweeps: the one asked for first taken %v, the one asked for "+
				"later not; want the later built first", sweeps, hs.Take(first) != nil)
		}
		hs.Sweep()
	}
	if f.Builds()-builds != 4 {
		t.Errorf("%d clusters built while the later was, want its four alone", f.Builds()-builds)
	}
	settle(t, hs)
	if hs.Take(first) == nil {
		t.Error("the one asked for first never built")
	}
}

// the hierarchies of two domains of three clusters each asked for alike: a Sweep builds one
// hierarchy, the first's till it is built, then the second's, a cluster's work spread over Sweeps
// of a small budget
func TestHierarchies_ASweepBuildsOneHierarchyAtATime(t *testing.T) {
	g := grid.DefaultGrids{}.Square(24, 8, 10)
	terrain := newPassingMap()
	f := NewFinder(g, terrain, nil, OpenOccupancy{}).WithLevels(flat{terrain}).
		WithClasses(testClasses())
	hs := &Hierarchies{}
	if hs.Bind(f, 8, 8); !hs.On() {
		t.Fatal("no hierarchies kept")
	}
	hs.budget = 3000
	first, second := keyOf(t, hs, cell.Land, 4), keyOf(t, hs, amphibious, 4)
	hs.Expect(first)
	hs.Expect(second)
	var built []uint64
	for sweeps := 0; hs.Due(); sweeps++ {
		if sweeps == 64 {
			t.Fatal("the hierarchies never built")
		}
		builds := f.Builds()
		hs.Sweep()
		built = append(built, f.Builds()-builds)
		if hs.Take(second) != nil && hs.Take(first) == nil {
			t.Fatal("the second built before the first")
		}
	}
	finished := 0
	for _, b := range built {
		finished += int(b)
	}
	if finished != 6 || slices.Max(built) > 1 || len(built) <= 6 || hs.Take(first) == nil ||
		hs.Take(second) == nil {
		t.Errorf("built %v a Sweep; want three clusters of each domain's, at most one a Sweep, "+
			"both taken", built)
	}
}

// a Sweep marks the clusters stale where the board's passage moved, as a climb asks, and builds
// them again: a cell inside a cluster, that cluster alone
func TestHierarchies_ASweepStalesWhereThePassageMoved(t *testing.T) {
	hs, terrain, g, f := newHierarchies(t)
	key := keyOf(t, hs, cell.Land, 4)
	hs.Expect(key)
	settle(t, hs)
	e := &hs.entries[0]
	if e.key != key || !e.ready || e.since != terrain.Passage() {
		t.Fatalf("the domain's hierarchy %+v, want it built at the board's passage %d", e,
			terrain.Passage())
	}
	terrain.Set(g.At(3, 3), cell.Kind{Pace: 1, Solid: true})
	if !hs.Due() {
		t.Fatal("no Sweep due once a cell's passage changed")
	}
	builds := f.Builds()
	hs.Sweep()
	if e.since != terrain.Passage() || !e.ready || f.Builds()-builds != 1 {
		t.Errorf("swept after a change: %+v, %d clusters built; want it staled and its cluster "+
			"built again at the passage %d", e, f.Builds()-builds, terrain.Passage())
	}
	if hs.Due() {
		t.Error("a Sweep due with nothing changed")
	}
}
