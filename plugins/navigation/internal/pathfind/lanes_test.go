package pathfind

import (
	"slices"
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/internal/f32"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/uid"
)

// a river down the middle column, a bridge over it along row 1 leaving a lane 6 wide, a ford at
// its foot in row 4
type bridged struct {
	*cell.TerrainMap
	g grid.Grid
}

func newBridged() bridged {
	g := grid.DefaultGrids{}.Square(5, 5, 10)
	terrain := openTerrain()
	for y := range uint32(4) {
		terrain.Set(g.At(2, y), cell.Kind{Cost: 1, Allows: cell.Water})
	}
	terrain.Set(g.At(2, 1), cell.Kind{Cost: 1, Allows: cell.Land | cell.Water})
	return bridged{terrain, g}
}

func (b bridged) Lane(from, to cell.ID, d cell.Domain, deck bool) float32 {
	x0, y0, _ := b.g.Coords(from)
	x1, y1, _ := b.g.Coords(to)
	if d&cell.Land == 0 || deck || y0 != 1 || y1 != 1 || max(x0, x1) < 2 || min(x0, x1) > 2 {
		return f32.Inf(1)
	}
	return 6
}

func TestFind_ABodyWiderThanTheLaneAcrossTheStepGoesRoundByTheFord(t *testing.T) {
	b := newBridged()
	f := New(b.g, b, nil, &cell.MultipleOccupancy{})
	bridge := b.g.At(2, 1)
	for _, tc := range []struct {
		name   string
		size   geom.Vec
		across bool
	}{
		{name: "no box", size: geom.Vec{}, across: true},
		{name: "slim", size: geom.NewVec(4, 4), across: true},
		{name: "wide", size: geom.NewVec(8, 8), across: false},
		{name: "long along the bridge", size: geom.NewVec(8, 4), across: true},
		{name: "long across the bridge", size: geom.NewVec(4, 8), across: false},
	} {
		var steps [64]cell.ID
		body := Body{Size: tc.size}
		n, ok := f.Find(uid.UID64(1), cell.Land, body, b.g.At(0, 1), b.g.At(4, 1), Either, steps[:])
		if !ok {
			t.Errorf("%s: no route, want one by the bridge or the ford", tc.name)
			continue
		}
		if got := slices.Contains(steps[:n], bridge); got != tc.across {
			t.Errorf("%s: over the bridge %v, want %v", tc.name, got, tc.across)
		}
		if _, ok := f.Steps(cell.Land, body, b.g.At(1, 1), bridge); ok != tc.across {
			t.Errorf("%s: steps onto the bridge %v, want %v", tc.name, ok, tc.across)
		}
	}
}
