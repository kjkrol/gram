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

// hierarchies over an open 16x16 board of four clusters 8 cells a side on one level, a route
// climbed from 8 cells apart, built through the finder given
func newHierarchies(t *testing.T) (*Hierarchies, passingMap, grid.Grid, *Finder) {
	t.Helper()
	g := grid.DefaultGrids{}.Square(16, 16, 10)
	terrain := newPassingMap()
	f := NewFinder(g, terrain, nil, OpenOccupancy{}).WithLevels(flat{terrain})
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

// the class of units whose boxes are side across
func classOf(t *testing.T, hs *Hierarchies, side float32) pathfind.Key {
	t.Helper()
	m := bodies.Member{Domain: cell.Land, Pos: world.Position{}}
	m.Pos.Size = geom.NewVec(side, side)
	m.Z.Altitude = 3
	key, ok := hs.Key(m)
	if !ok || key.Body.Feet != 0 || key.Body.Size != m.Pos.Size || key.Domain != cell.Land {
		t.Fatalf("the class of %v: %+v, %v; want its domain and body, its feet left out", m, key,
			ok)
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
func TestHierarchies_AClassIsTakenOnceExpectedAndBuilt(t *testing.T) {
	hs, _, _, _ := newHierarchies(t)
	hs.budget = 3000
	key := classOf(t, hs, 4)
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
	if h == nil || h.Key() != key || h.Side() != 8 || !h.Ready() {
		t.Fatalf("taken once built: %v; want the class's, of regions 8 a side", h)
	}
	hs.Expect(key)
	if hs.Sweep(); hs.Take(key) != h {
		t.Error("asked for again, the class's hierarchy made anew")
	}
}

// an Update wants a hierarchy by expecting it; a fifth class takes the place of the one asked
// for least lately, none wanted the same Update
func TestHierarchies_TheLeastLatelyAskedForIsLetGo(t *testing.T) {
	hs, _, _, _ := newHierarchies(t)
	var keys [keptHierarchies + 1]pathfind.Key
	for i := range keys {
		keys[i] = classOf(t, hs, float32(2+i))
	}
	update := func(asked ...int) {
		for _, i := range asked {
			hs.Expect(keys[i])
		}
		hs.Sweep()
		settle(t, hs)
	}
	kept := func(want ...int) {
		t.Helper()
		for i, key := range keys {
			if taken := hs.Take(key) != nil; taken != slices.Contains(want, i) {
				t.Errorf("class %d kept %v, want the classes %v kept", i, taken, want)
			}
		}
	}
	update(0, 1, 2, 3)
	kept(0, 1, 2, 3)
	update(1, 2)
	update(3, 4)
	kept(1, 2, 3, 4)
	update(1, 2, 3, 4, 0)
	kept(1, 2, 3, 4)
	update(2, 3, 4)
	update(0)
	kept(0, 2, 3, 4)
}

// a Sweep builds the hierarchy asked for most lately of those not ready, the one asked for
// before it waiting till it is
func TestHierarchies_ASweepBuildsTheLatestAskedFirst(t *testing.T) {
	hs, _, _, f := newHierarchies(t)
	hs.budget = 3000
	first, second := classOf(t, hs, 4), classOf(t, hs, 6)
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

// two classes of three clusters each asked for alike: a Sweep builds one hierarchy, the first's
// till it is built, then the second's, a cluster's work spread over Sweeps of a small budget
func TestHierarchies_ASweepBuildsOneHierarchyAtATime(t *testing.T) {
	g := grid.DefaultGrids{}.Square(24, 8, 10)
	terrain := newPassingMap()
	f := NewFinder(g, terrain, nil, OpenOccupancy{}).WithLevels(flat{terrain})
	hs := &Hierarchies{}
	if hs.Bind(f, 8, 8); !hs.On() {
		t.Fatal("no hierarchies kept")
	}
	hs.budget = 3000
	first, second := classOf(t, hs, 4), classOf(t, hs, 6)
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
		t.Errorf("built %v a Sweep; want three clusters of each class, at most one a Sweep, "+
			"both taken", built)
	}
}

// a Sweep marks the clusters stale where the board's passage moved, as a climb asks, and builds
// them again: a cell inside a cluster, that cluster alone
func TestHierarchies_ASweepStalesWhereThePassageMoved(t *testing.T) {
	hs, terrain, g, f := newHierarchies(t)
	key := classOf(t, hs, 4)
	hs.Expect(key)
	settle(t, hs)
	e := &hs.entries[0]
	if e.key != key || !e.ready || e.since != terrain.Passage() {
		t.Fatalf("the class's hierarchy %+v, want it built at the board's passage %d", e,
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
