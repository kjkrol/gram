package pathfind

import (
	"slices"
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/uid"
)

// high ground 10 up west of x=10 and east of x=40, a valley at 0 between them with a river down
// the middle column, a deck 10 up over the valley's row 2, its underside at 9
type valley struct {
	g       grid.Grid
	terrain *cell.TerrainMap
}

func newValley() valley {
	g := grid.DefaultGrids{}.Square(5, 5, 10)
	terrain := cell.NewTerrainMap()
	terrain.SetAll(cell.Kind{Cost: 1, Allows: cell.Land})
	for y := range uint32(5) {
		terrain.Set(g.At(2, y), cell.Kind{Cost: 1, Allows: cell.Water})
	}
	return valley{g, terrain}
}

func (v valley) Deck(c cell.ID) (top, under float64, carries cell.Domain, ok bool) {
	if x, y, _ := v.g.Coords(c); y == 2 && x >= 1 && x <= 3 {
		return 10, 9, cell.Land, true
	}
	return 0, 0, 0, false
}

func (v valley) Ground(p geom.Vec) float64 {
	if p.X <= 10 || p.X >= 40 {
		return 10
	}
	return 0
}

func (v valley) Over(p geom.Vec) (float64, bool) {
	if c, ok := v.g.CellAt(p); ok {
		if _, under, _, ok := v.Deck(c); ok {
			return under, true
		}
	}
	return 0, false
}

func (v valley) KindAt(c cell.ID, deck bool) cell.Kind {
	if _, _, _, ok := v.Deck(c); ok && deck {
		return cell.Kind{Cost: 1, Allows: cell.Land, Graded: true}
	}
	return v.terrain.Kind(c)
}

func (v valley) find(t *testing.T, body Body, from, to cell.ID) []cell.ID {
	t.Helper()
	f := New(v.g, v.terrain, nil, &cell.MultipleOccupancy{}).WithLevels(v)
	var steps [64]cell.ID
	n, ok := f.Find(uid.UID64(1), cell.Land, body, from, to, Either, steps[:])
	if !ok {
		t.Fatalf("no route from %v to %v for %+v", from, to, body)
	}
	return steps[:n]
}

func TestFinder_EndsOnTheLevelAsked(t *testing.T) {
	v := newValley()
	f := New(v.g, v.terrain, nil, &cell.MultipleOccupancy{}).WithLevels(v)
	var steps [64]cell.ID
	for name, tc := range map[string]struct {
		from cell.ID
		feet float64
	}{
		"from the rim":   {v.g.At(0, 2), 10},
		"from the floor": {v.g.At(1, 0), 0},
	} {
		walker := Body{Feet: tc.feet, Step: 1, Height: 2}
		for _, end := range []Level{Deck, Ground} {
			_, ok := f.Find(uid.UID64(1), cell.Land, walker, tc.from, v.g.At(1, 2), end, steps[:])
			if !ok || f.Ended() != end {
				t.Errorf("%s, asked to end on %v: found %v, ending on %v", name, end, ok,
					f.Ended())
			}
		}
	}
}

func TestFinder_RoutesOverADeckAndUnderIt(t *testing.T) {
	v := newValley()
	at := func(x, y uint32) cell.ID { return v.g.At(x, y) }
	walker := Body{Feet: 10, Step: 1, Height: 2}
	over := v.find(t, walker, at(0, 2), at(4, 2))
	if !slices.Equal(over, []cell.ID{at(1, 2), at(2, 2), at(3, 2), at(4, 2)}) {
		t.Errorf("across the river: %v, want straight over the deck", over)
	}
	short := Body{Step: 1, Height: 2}
	if under := v.find(t, short, at(1, 0), at(1, 4)); !slices.Contains(under, at(1, 2)) {
		t.Errorf("along the valley: %v, want it under the deck at %v", under, at(1, 2))
	}
	tall := Body{Step: 1, Height: 12}
	if round := v.find(t, tall, at(1, 0), at(1, 4)); slices.Contains(round, at(1, 2)) {
		t.Errorf("a walker 12 tall went under a deck 9 up: %v", round)
	}
}

func TestFinder_UnderADeckOnlyWhereItFits(t *testing.T) {
	v := newValley()
	at := func(x, y uint32) cell.ID { return v.g.At(x, y) }
	f := New(v.g, v.terrain, nil, &cell.MultipleOccupancy{}).WithLevels(v)
	var steps [64]cell.ID
	n, ok := f.Find(uid.UID64(1), cell.Land, Body{Feet: 10, Step: 1, Height: 2}, at(0, 2),
		at(1, 2), Ground, steps[:])
	if !ok || f.Ended() != Ground {
		t.Fatalf("no route under the deck: %v, ending on %v", ok, f.Ended())
	}
	if before := append([]cell.ID{at(0, 2)}, steps[:n]...)[n-1]; before == at(0, 2) {
		t.Errorf("under the deck: %v, want it in from beside, not off the rim through the slab",
			steps[:n])
	}
	short := Body{Step: 1, Height: 2}
	if _, ok := f.Steps(cell.Land, short, at(1, 2), at(0, 2)); ok {
		t.Error("a step out from under the deck onto the rim through its slab is allowed")
	}
	if _, ok := f.Steps(cell.Land, short, at(1, 2), at(1, 1)); !ok {
		t.Error("a step out from under the deck along the valley is refused")
	}
}

func TestFinder_NeverPlansAFallOffADeck(t *testing.T) {
	v := newValley()
	at := func(x, y uint32) cell.ID { return v.g.At(x, y) }
	route := v.find(t, Body{Feet: 10, Step: 1, Height: 2}, at(1, 2), at(1, 0))
	if route[0] != at(0, 2) {
		t.Errorf("off the deck: %v, want it on to the deck's end first", route)
	}
	f := New(v.g, v.terrain, nil, &cell.MultipleOccupancy{}).WithLevels(v)
	if _, ok := f.Steps(cell.Land, Body{Feet: 10, Step: 1, Height: 2}, at(1, 2), at(1, 1)); ok {
		t.Error("a step off the deck's side is allowed")
	}
	if _, ok := f.Steps(cell.Land, Body{Step: 1, Height: 2}, at(1, 1), at(1, 2)); !ok {
		t.Error("a step from the valley's floor under the deck is refused")
	}
}

// a railing on the ground from a to b
type fenced struct {
	*cell.TerrainMap
	g    grid.Grid
	a, b geom.Vec
}

func (f fenced) Reaches(c, n cell.ID) bool { return false }

func (f fenced) Fenced(from, to cell.ID, d cell.Domain, deck bool) bool {
	p, q := f.g.CellCenter(from), f.g.CellCenter(to)
	side := func(o, e, x geom.Vec) float64 { return (e.X-o.X)*(x.Y-o.Y) - (e.Y-o.Y)*(x.X-o.X) }
	return !deck && d&cell.Land != 0 &&
		side(p, q, f.a)*side(p, q, f.b) < 0 && side(f.a, f.b, p)*side(f.a, f.b, q) < 0
}

func TestFinder_GoesRoundARailing(t *testing.T) {
	g := grid.DefaultGrids{}.Square(3, 3, 10)
	terrain := fenced{openTerrain(), g, geom.NewVec(9, 10), geom.NewVec(21, 10)}
	f := New(g, terrain, nil, &cell.MultipleOccupancy{})
	if _, ok := f.Steps(cell.Land, Body{}, g.At(1, 0), g.At(1, 1)); ok {
		t.Error("a step across the railing is allowed")
	}
	var steps [64]cell.ID
	n, ok := f.Find(uid.UID64(1), cell.Land, Body{}, g.At(1, 0), g.At(1, 1), Either, steps[:])
	if !ok || n < 2 || steps[n-1] != g.At(1, 1) {
		t.Fatalf("round the railing: %v, want a way round into %v", steps[:n], g.At(1, 1))
	}
}

// water with land at (0, 0) and (4, 4), a deck 1 up running slantwise between them
type causeway struct {
	g       grid.Grid
	terrain *cell.TerrainMap
}

func (w causeway) on(c cell.ID) bool {
	x, y, _ := w.g.Coords(c)
	return x == y && x >= 1 && x <= 3
}

func (w causeway) Deck(c cell.ID) (top, under float64, carries cell.Domain, ok bool) {
	if w.on(c) {
		return 1, 0.5, cell.Land, true
	}
	return 0, 0, 0, false
}

func (w causeway) Ground(geom.Vec) float64 { return 0 }

func (w causeway) Over(p geom.Vec) (float64, bool) {
	if c, ok := w.g.CellAt(p); ok && w.on(c) {
		return 0.5, true
	}
	return 0, false
}

func (w causeway) KindAt(c cell.ID, deck bool) cell.Kind {
	if deck && w.on(c) {
		return cell.Kind{Cost: 1, Allows: cell.Land}
	}
	return w.terrain.Kind(c)
}

func (w causeway) Reaches(c, n cell.ID) bool {
	cx, cy, _ := w.g.Coords(c)
	nx, ny, _ := w.g.Coords(n)
	return w.on(c) && int(nx)-int(cx) == int(ny)-int(cy)
}

func (w causeway) Fenced(cell.ID, cell.ID, cell.Domain, bool) bool { return false }

func (w causeway) Kind(c cell.ID) cell.Kind { return w.terrain.Kind(c) }

func TestFinder_WalksADeckSlantwise(t *testing.T) {
	g := grid.DefaultGrids{}.Square(5, 5, 10)
	terrain := cell.NewTerrainMap()
	terrain.SetAll(cell.Kind{Cost: 1, Allows: cell.Water})
	terrain.Set(g.At(0, 0), cell.Kind{Cost: 1, Allows: cell.Land})
	terrain.Set(g.At(4, 4), cell.Kind{Cost: 1, Allows: cell.Land})
	w := causeway{g, terrain}
	f := New(g, w, nil, &cell.MultipleOccupancy{}).WithLevels(w)
	var steps [64]cell.ID
	n, ok := f.Find(uid.UID64(1), cell.Land, Body{Step: 1, Height: 2}, g.At(0, 0), g.At(4, 4),
		Either,
		steps[:])
	want := []cell.ID{g.At(1, 1), g.At(2, 2), g.At(3, 3), g.At(4, 4)}
	if !ok || !slices.Equal(steps[:n], want) {
		t.Errorf("along the deck: %v, %v; want %v", steps[:n], ok, want)
	}
	if _, ok := f.Steps(cell.Land, Body{Feet: 1, Step: 1, Height: 2}, g.At(2, 2), g.At(3, 1)); ok {
		t.Error("a step off the deck slantwise across it is allowed")
	}
}
