package routes

import (
	"slices"
	"testing"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/ceremony/tag"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/unit"
	"github.com/kjkrol/gram/plugins/navigation/internal/order"
	"github.com/kjkrol/gram/plugins/navigation/internal/pathfind"
	"github.com/kjkrol/gram/plugins/navigation/internal/spacing"
	"github.com/kjkrol/gram/plugins/selection"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/uid"
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

// the side of every unit's box: within a cell 10 across
const unitSide = 4

var (
	unitBody = pathfind.Body{Size: geom.NewVec(unitSide, unitSide)}
	water    = cell.Kind{Pace: 1, Allows: cell.Water}
	mud      = cell.Kind{Pace: 0.5, Allows: cell.Land}
)

const selectedTag = tag.Tag[selection.Family](1)

// a unit sent from its cell to target, holding window of its route
type sent struct {
	at, target cell.ID
	window     []cell.ID
}

// selected units on a board, their routes drawn by r
type rig struct {
	t   *testing.T
	g   grid.Grid
	r   *Renderer
	ids []uid.UID64
}

func newRig(t *testing.T, g grid.Grid, finder *spacing.Finder, units ...sent) *rig {
	t.Helper()
	space, err := aabbworld.NewSpace(aabbworld.Config{Width: 1024, Height: 1024, BucketSize: 32})
	if err != nil {
		t.Fatal(err)
	}
	r := NewRenderer(g, RouteStyle{}, selectedTag)
	r.BindSpace(space)
	r.ShowRoutes(true)
	r.Finder = finder
	k := &rig{t: t, g: g, r: r}
	var base goke.Comp[world.Base]
	var at goke.Comp[unit.At]
	var orders goke.Comp[order.MoveOrder]
	var marks goke.Comp[tag.Tags[selection.Family]]
	goke.New().Setup(
		goke.SystemFn{OnInit: func(si *goke.SysInit) {
			f := si.NewFactory(&base, &at, &orders, &marks)
			f.Create(len(units))
			for f.Next() {
				for i, id := range f.Cursor.IDs {
					u := units[len(k.ids)]
					c := g.CellCenter(u.at)
					corner := geom.NewVec(c.X-unitSide/2, c.Y-unitSide/2)
					base.Slice(&f.Cursor)[i].Pos = world.Position{
						AABB: plane.NewAABB(corner, unitSide, unitSide),
					}
					at.Slice(&f.Cursor)[i] = unit.At{Cell: u.at}
					orders.Slice(&f.Cursor)[i] = orderOf(u.target, u.window)
					marks.Slice(&f.Cursor)[i] = tag.Tags[selection.Family](0).With(selectedTag)
					k.ids = append(k.ids, id)
				}
			}
		}},
		goke.SystemFn{OnInit: r.Init},
	)
	return k
}

func orderOf(target cell.ID, window []cell.ID) order.MoveOrder {
	o := order.MoveOrder{Target: target}
	o.Path.Length = uint16(copy(o.Path.Steps[:], window))
	return o
}

// frame publishes what the next frame draws
func (k *rig) frame() {
	k.r.Publish()
	k.r.Swap()
}

// drawn are the cells unit i's route is drawn through, from its own
func (k *rig) drawn(i int) []cell.ID {
	k.t.Helper()
	d := k.r.drawn.Front()
	for _, u := range d.units {
		cells := d.cells[u.path[0]:u.path[1]]
		if len(cells) > 0 && cells[0] == k.at(i).Cell {
			return cells
		}
	}
	k.t.Fatalf("no route drawn from unit %d's cell", i)
	return nil
}

func (k *rig) seek(i int) *goke.Cursor {
	k.t.Helper()
	if !k.r.query.Seek(k.ids[i]) {
		k.t.Fatalf("unit %d not found", i)
	}
	return k.r.query.Cursor()
}

func (k *rig) at(i int) *unit.At { return k.r.cell.At(k.seek(i)) }

func (k *rig) order(i int) *order.MoveOrder { return k.r.order.At(k.seek(i)) }

func (k *rig) selected(i int, on bool) {
	marks := k.r.marks.At(k.seek(i))
	if on {
		*marks = marks.With(selectedTag)
	} else {
		*marks = marks.Without(selectedTag)
	}
}

// whole says the route runs from cell to neighbouring cell to target, no stretch straight across
func (k *rig) whole(cells []cell.ID, target cell.ID) bool {
	for i := 1; i < len(cells); i++ {
		if _, ok := grid.Way(k.g, cells[i-1], cells[i]); !ok {
			return false
		}
	}
	return len(cells) > 0 && cells[len(cells)-1] == target
}

// route is the route from from to to found by a finder of its own over terrain, from first
func route(g grid.Grid, terrain cell.Terrain, from, to cell.ID) []cell.ID {
	steps := make([]cell.ID, wholeSteps)
	f := spacing.NewFinder(g, terrain, nil, spacing.OpenOccupancy{})
	n, _ := f.Find(0, cell.Land, unitBody, from, to, pathfind.Either, steps)
	return append([]cell.ID{from}, steps[:n]...)
}

// a board 30 by 10 walled across at x 15 but for its bottom row, a unit sent past the wall
// holding the first 6 steps of its route
func walled(t *testing.T) (*rig, passingMap, []cell.ID) {
	g := grid.DefaultGrids{}.Square(30, 10, 10)
	terrain := newPassingMap()
	for y := range uint32(9) {
		terrain.Set(g.At(15, y), water)
	}
	from, to := g.At(2, 1), g.At(27, 1)
	full := route(g, terrain, from, to)
	finder := spacing.NewFinder(g, terrain, nil, spacing.OpenOccupancy{})
	return newRig(t, g, finder, sent{at: from, target: to, window: full[1:7]}), terrain, full
}

func TestPathRenderer_DrawsARouteWholePastItsWindow(t *testing.T) {
	k, terrain, full := walled(t)
	to := full[len(full)-1]
	k.frame()
	cells := k.drawn(0)
	want := append(append([]cell.ID(nil), full[:7]...), route(k.g, terrain, full[6], to)[1:]...)
	if len(cells) != len(want) {
		t.Fatalf("drawn through %v, want the window then the rest found: %v", cells, want)
	}
	for i := range want {
		if cells[i] != want[i] {
			t.Fatalf("drawn through %v, want the window then the rest found: %v", cells, want)
		}
	}
	if !k.whole(cells, to) {
		t.Errorf("drawn through %v, want cell by cell to the target, nothing straight", cells)
	}
	if gap := k.g.At(15, 9); !slices.Contains(cells, gap) {
		t.Errorf("drawn through %v, want through the wall's gap %v", cells, gap)
	}
}

func TestPathRenderer_FindsAWholeRouteOnceAndKeepsIt(t *testing.T) {
	k, _, _ := walled(t)
	k.frame()
	first := append([]cell.ID(nil), k.drawn(0)...)
	for range 4 {
		k.frame()
	}
	if n := k.r.Finder.Searches(); n != 1 {
		t.Errorf("%d searches over five frames, want the rest found once", n)
	}
	if again := k.drawn(0); len(again) != len(first) || again[len(again)-1] != first[len(first)-1] {
		t.Errorf("drawn through %v, want as on the first frame: %v", again, first)
	}
}

func TestPathRenderer_FindsTheRestAnewAsTheWindowMovesOnOrThePassageChanges(t *testing.T) {
	k, terrain, full := walled(t)
	to := full[len(full)-1]
	k.frame()
	k.at(0).Cell = full[6]
	*k.order(0) = orderOf(to, full[7:13])
	k.frame()
	if n := k.r.Finder.Searches(); n != 2 {
		t.Errorf("%d searches, want the rest found anew as the window's last cell moved on", n)
	}
	if cells := k.drawn(0); !k.whole(cells, to) || cells[0] != full[6] {
		t.Errorf("drawn through %v, want whole from %v", cells, full[6])
	}
	terrain.Set(k.g.At(0, 9), mud)
	k.frame()
	if n := k.r.Finder.Searches(); n != 3 {
		t.Errorf("%d searches, want the rest found anew as the board's passage changed", n)
	}
	if cells := k.drawn(0); !k.whole(cells, to) {
		t.Errorf("drawn through %v, want whole", cells)
	}
}

func TestPathRenderer_DropsAWholeRouteWithTheSelection(t *testing.T) {
	k, _, _ := walled(t)
	k.frame()
	k.selected(0, false)
	k.frame()
	if len(k.r.previews) != 0 || k.r.wholes != 0 {
		t.Errorf("%d previews, %d whole, want none once the unit is not selected",
			len(k.r.previews), k.r.wholes)
	}
	k.selected(0, true)
	k.frame()
	if n := k.r.Finder.Searches(); n != 2 {
		t.Errorf("%d searches, want the rest found anew once selected again", n)
	}
}

func TestPathRenderer_FindsNoWholeRouteWithTheRoutesHidden(t *testing.T) {
	k, _, _ := walled(t)
	k.r.ShowRoutes(false)
	k.frame()
	if n := k.r.Finder.Searches(); n != 0 || len(k.r.previews) != 0 {
		t.Errorf("%d searches, %d previews with the routes hidden, want none", n,
			len(k.r.previews))
	}
}

// units on the rows of an open board 40 cells wide, each sent from its west end to its east end
// holding 5 steps of its route
func rows(t *testing.T, n int) (*rig, []cell.ID) {
	g := grid.DefaultGrids{}.Square(40, uint32(n), 10)
	finder := spacing.NewFinder(g, newPassingMap(), nil, spacing.OpenOccupancy{})
	units := make([]sent, n)
	targets := make([]cell.ID, n)
	for y := range units {
		var window []cell.ID
		for x := range uint32(5) {
			window = append(window, g.At(x+1, uint32(y)))
		}
		targets[y] = g.At(39, uint32(y))
		units[y] = sent{at: g.At(0, uint32(y)), target: targets[y], window: window}
	}
	return newRig(t, g, finder, units...), targets
}

func TestPathRenderer_FindsOneWholeRouteAPublish(t *testing.T) {
	k, targets := rows(t, 3)
	k.frame()
	wholes := 0
	for i := range targets {
		if k.whole(k.drawn(i), targets[i]) {
			wholes++
		}
	}
	if n := k.r.Finder.Searches(); n != 1 || wholes != 1 {
		t.Errorf("first frame: %d searches, %d routes whole, want one of each", n, wholes)
	}
	for f := 2; f <= 4; f++ {
		k.frame()
		if n, want := k.r.Finder.Searches(), uint64(min(f, 3)); n != want {
			t.Errorf("frame %d: %d searches, want %d: one a frame till each is found", f, n, want)
		}
	}
	for i := range targets {
		if !k.whole(k.drawn(i), targets[i]) {
			t.Errorf("unit %d drawn through %v, want whole", i, k.drawn(i))
		}
	}
}

func TestPathRenderer_DrawsTheRestFoundBeforeTillItIsFoundAnew(t *testing.T) {
	k, targets := rows(t, 2)
	k.frame()
	k.frame()
	for i, target := range targets {
		y := uint32(i)
		k.at(i).Cell = k.g.At(5, y)
		window := []cell.ID{k.g.At(6, y), k.g.At(7, y), k.g.At(8, y), k.g.At(9, y)}
		*k.order(i) = orderOf(target, window)
	}
	k.frame()
	if n := k.r.Finder.Searches(); n != 3 {
		t.Errorf("%d searches, want one rest found anew this frame, after the first two", n)
	}
	for i, target := range targets {
		if cells := k.drawn(i); !k.whole(cells, target) {
			t.Errorf("unit %d drawn through %v, want whole: the rest found before, from the "+
				"window's new last cell on", i, cells)
		}
	}
	k.frame()
	if n := k.r.Finder.Searches(); n != 4 {
		t.Errorf("%d searches, want the other rest found anew the next frame", n)
	}
}

func TestPathRenderer_DrawsTheSeventeenthSelectedUnitsRouteStraight(t *testing.T) {
	k, targets := rows(t, maxWholes+1)
	for range maxWholes + 2 {
		k.frame()
	}
	straight := -1
	for i := range targets {
		if !k.whole(k.drawn(i), targets[i]) {
			if straight >= 0 {
				t.Fatalf("units %d and %d drawn straight, want one", straight, i)
			}
			straight = i
		}
	}
	if n := k.r.Finder.Searches(); n != maxWholes || straight < 0 {
		t.Fatalf("%d searches, unit %d straight; want %d found, the one past them straight", n,
			straight, maxWholes)
	}
	if cells := k.drawn(straight); cells[len(cells)-1] != k.order(straight).Path.Steps[4] {
		t.Errorf("unit %d drawn through %v, want its window, then straight", straight, cells)
	}
	k.selected((straight+1)%len(targets), false)
	k.frame()
	k.frame()
	if !k.whole(k.drawn(straight), targets[straight]) {
		t.Errorf("unit %d drawn through %v, want whole once another is not selected", straight,
			k.drawn(straight))
	}
}

func TestPathRenderer_ClimbsAFarRestThroughTheHierarchy(t *testing.T) {
	g := grid.DefaultGrids{}.Square(32, 8, 10)
	terrain := newPassingMap()
	var classes pathfind.Classes
	classes.Set([]pathfind.Kind{{Domain: cell.Land, Body: unitBody}})
	planner := spacing.NewFinder(g, terrain, nil, spacing.OpenOccupancy{}).
		WithLevels(flat{terrain}).WithClasses(&classes)
	hs := &spacing.Hierarchies{}
	if hs.Bind(planner, 8, 8); !hs.On() {
		t.Fatal("no hierarchies kept")
	}
	finder := spacing.NewFinder(g, terrain, nil, spacing.OpenOccupancy{}).
		WithLevels(flat{terrain}).WithClasses(&classes)
	from, to := g.At(0, 3), g.At(31, 3)
	window := []cell.ID{g.At(1, 3), g.At(2, 3), g.At(3, 3), g.At(4, 3)}
	k := newRig(t, g, finder, sent{at: from, target: to, window: window})
	k.r.Hierarchies, k.r.Classes = hs, &classes
	k.frame()
	hs.Expect(pathfind.Key{Domain: cell.Land})
	k.frame()
	if c, s := finder.Climbs(), finder.Searches(); c != 0 || s != 0 || k.whole(k.drawn(0), to) {
		t.Errorf("%d climbs, %d searches, drawn through %v before the hierarchy is built; want "+
			"none, straight past the window", c, s, k.drawn(0))
	}
	settle(t, hs)
	k.frame()
	if c, s := finder.Climbs(), finder.Searches(); c != 1 || s != 0 {
		t.Errorf("%d climbs, %d searches, want the far rest climbed once", c, s)
	}
	if cells := k.drawn(0); !k.whole(cells, to) {
		t.Errorf("drawn through %v, want whole", cells)
	}
	terrain.Set(g.At(16, 7), mud)
	k.frame()
	if c, s := finder.Climbs(), finder.Searches(); c != 1 || s != 0 {
		t.Errorf("%d climbs, %d searches as the hierarchy is staled, want none more: it waits",
			c, s)
	}
	if cells := k.drawn(0); k.whole(cells, to) {
		t.Errorf("drawn through %v over a passage moved, want straight past the window", cells)
	}
	settle(t, hs)
	k.frame()
	if c := finder.Climbs(); c != 2 || !k.whole(k.drawn(0), to) {
		t.Errorf("%d climbs, drawn through %v; want climbed again, whole, once the hierarchy "+
			"is staled", c, k.drawn(0))
	}
}

// settle sweeps hs till nothing is left to stale or build
func settle(t *testing.T, hs *spacing.Hierarchies) {
	t.Helper()
	for range 64 {
		if !hs.Due() {
			return
		}
		hs.Sweep()
	}
	t.Fatal("hierarchies never built")
}
