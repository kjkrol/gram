package pathfind

import (
	"slices"
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/uid"
)

// valley is a 5x5 board of cells 10 wide: high ground 10 up west of x=10 and east of x=40, a valley
// at 0 between them with a river down the middle column, and a deck 10 up over the valley's row 2,
// its underside at 9, carrying walkers.
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
	n, ok := f.Find(uid.UID64(1), cell.Land, body, from, to, steps[:])
	if !ok {
		t.Fatalf("no route from %v to %v for %+v", from, to, body)
	}
	return steps[:n]
}

// A walker crosses the river over the deck; another passes under the deck along the valley, where
// it fits, and goes round where it does not.
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

// From the deck a route goes on along it to its end: never off its side, a fall.
func TestFinder_NeverPlansAFallOffADeck(t *testing.T) {
	v := newValley()
	at := func(x, y uint32) cell.ID { return v.g.At(x, y) }
	route := v.find(t, Body{Feet: 10, Step: 1, Height: 2}, at(1, 2), at(1, 0))
	if route[0] != at(0, 2) {
		t.Errorf("off the deck: %v, want it on to the deck's end first", route)
	}
	f := New(v.g, v.terrain, nil, &cell.MultipleOccupancy{}).WithLevels(v)
	if f.Steps(cell.Land, Body{Feet: 10, Step: 1, Height: 2}, at(1, 2), at(1, 1)) {
		t.Error("a step off the deck's side is allowed")
	}
	if !f.Steps(cell.Land, Body{Step: 1, Height: 2}, at(1, 1), at(1, 2)) {
		t.Error("a step from the valley's floor under the deck is refused")
	}
}

// fenced is a terrain whose cell at has a railing along its north side.
type fenced struct {
	*cell.TerrainMap
	at cell.ID
}

func (f fenced) Railed() bool { return true }

func (f fenced) Built(c cell.ID) cell.Built {
	if c == f.at {
		return cell.Built{Rail: cell.Rail{Keeps: cell.Land, Sides: 1}}
	}
	return cell.Built{}
}

// A route never crosses a railing: into the fenced cell it comes round by a side left open.
func TestFinder_GoesRoundARailing(t *testing.T) {
	g := grid.DefaultGrids{}.Square(3, 3, 10)
	terrain := fenced{openTerrain(), g.At(1, 1)}
	f := New(g, terrain, nil, &cell.MultipleOccupancy{})
	if f.Steps(cell.Land, Body{}, g.At(1, 0), g.At(1, 1)) {
		t.Error("a step across the railing is allowed")
	}
	var steps [64]cell.ID
	n, ok := f.Find(uid.UID64(1), cell.Land, Body{}, g.At(1, 0), g.At(1, 1), steps[:])
	if !ok || n < 2 || steps[n-1] != g.At(1, 1) {
		t.Fatalf("round the railing: %v, want a way round into %v", steps[:n], g.At(1, 1))
	}
}
