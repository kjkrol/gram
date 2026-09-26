package navigation

import (
	"testing"

	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/uid"
)

// hill is level ground with a few cells raised, priced by DefaultClimbing over steps of 10.
type hill map[board.CellID]float64

func (h hill) Climbing() board.Climbing { return board.DefaultClimbing }

func (h hill) Climb(from, to board.CellID, d board.Domain) float64 {
	c := board.DefaultClimbing
	if !c.Feels(d) {
		return 1
	}
	return c.Factor((h[to] - h[from]) / 10)
}

// A walker goes round a hill across its way, a flyer straight over it.
func TestFindPath_GoesRoundAHillUnlessItFlies(t *testing.T) {
	grid := board.DefaultGrids{}.Square(7, 5, 10)
	terrain := board.NewTerrainMap()
	terrain.SetAll(board.CellKind{Cost: 1, Allows: board.Land | board.Air})
	at := func(x, y uint32) board.CellID { c, _ := grid.CellIndex(x, y); return c }
	h := hill{}
	for y := uint32(0); y < 4; y++ {
		h[at(3, y)] = 20 // a ridge across the middle, open at the bottom row
	}
	pf := newPathFinder(grid, terrain, h, &board.MultipleOccupancy{})

	walk, ok := pf.findPath(uid.UID64(1), board.Land, at(0, 1), at(6, 1))
	if !ok {
		t.Fatal("no way for the walker")
	}
	for _, s := range walk.Steps[:walk.Length] {
		if h[s] > 0 {
			t.Fatalf("the walker's route climbs the ridge at %v", s)
		}
	}
	fly, ok := pf.findPath(uid.UID64(2), board.Air, at(0, 1), at(6, 1))
	if !ok || fly.Length != 6 {
		t.Errorf("the flyer: ok=%v, %d steps, want 6 straight over", ok, fly.Length)
	}
}
