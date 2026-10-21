package navigation

import (
	"testing"

	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/world"
)

// a terrain counting the cells read
type readTerrain struct {
	*cell.TerrainMap
	read int
}

func (t *readTerrain) Kind(c cell.ID) cell.Kind {
	t.read++
	return t.TerrainMap.Kind(c)
}

// the same detours told to a unit whose finder is blind to the others, which keeps its answers,
// and to one whose finder sees them, which keeps none: the same orders, searched only when asked
// something new
func TestKept_ADetourAskedAgainIsAnsweredAsBeforeUnsearched(t *testing.T) {
	g := grid.DefaultGrids{}.Square(6, 3, 10)
	at := func(x, y uint32) cell.ID { return g.At(x, y) }
	board := cell.NewTerrainMap()
	board.SetAll(cell.Kind{Cost: 1, Allows: cell.Land})
	read := &readTerrain{TerrainMap: board}
	kept := newCellKeeping(newPathFinder(g, read, nil, openOccupancy{}), &cell.SingleOccupancy{})
	fresh := newCellKeeping(newPathFinder(g, board, nil, &cell.MultipleOccupancy{}),
		&cell.SingleOccupancy{})
	stand := func(c cell.ID, size uint32) member {
		return member{id: 7, cell: c, from: c, domain: cell.Land,
			pos: world.Position{AABB: cellBox(g, c, size)}}
	}
	m := stand(at(0, 1), 8)
	o := MoveOrder{Target: at(5, 1), Toward: at(1, 1)}
	mf, of := m, o
	other := body{id: 3, cell: at(1, 1)}
	for _, step := range []struct {
		name     string
		change   func(m *member, o *MoveOrder)
		searched bool
	}{
		{"asked first", nil, true},
		{"asked again", nil, false},
		{"another cell learnt", func(_ *member, o *MoveOrder) { o.Toward = at(1, 0) }, true},
		{"asked again", nil, false},
		{"from another cell", func(m *member, _ *MoveOrder) { *m = stand(at(0, 0), 8) }, true},
		{"to another target", func(_ *member, o *MoveOrder) { o.Target = at(5, 2) }, true},
		{"to another level", func(_ *member, o *MoveOrder) { o.Level = OnGround }, true},
		{"a body of another size", func(m *member, _ *MoveOrder) { *m = stand(m.cell, 9) }, true},
		{"the board changed", func(*member, *MoveOrder) {
			for _, y := range []uint32{0, 2} {
				board.Set(at(3, y), wall)
			}
		}, true},
		{"asked again", nil, false},
		{"no way round a cell learnt", func(_ *member, o *MoveOrder) { o.Toward = at(3, 1) }, true},
		{"cornered, waiting", nil, false},
		{"its wait over, asked again", func(_ *member, o *MoveOrder) { o.Retry = 0 }, false},
	} {
		if step.change != nil {
			step.change(&m, &o)
			step.change(&mf, &of)
		}
		before := read.read
		kept.detour(m, &o, other)
		fresh.detour(mf, &of, other)
		if searched := read.read > before; searched != step.searched {
			t.Errorf("%s: searched %v, want %v", step.name, searched, step.searched)
		}
		if o != of {
			t.Fatalf("%s: the order is %+v, want %+v, as a search would give", step.name, o, of)
		}
	}
	if !o.Cornered || o.Fails != 2 {
		t.Errorf("cornered %v after %d searches round in a row found nothing, want "+
			"cornered after 2", o.Cornered, o.Fails)
	}
}

func TestKept_AFinderSeeingTheOthersKeepsNoAnswer(t *testing.T) {
	g := grid.DefaultGrids{}.Square(6, 3, 10)
	read := &readTerrain{TerrainMap: cell.NewTerrainMap()}
	read.SetAll(cell.Kind{Cost: 1, Allows: cell.Land})
	occ := &cell.SingleOccupancy{}
	k := newCellKeeping(newPathFinder(g, read, nil, occ), occ)
	m := member{id: 7, cell: g.At(0, 1), from: g.At(0, 1), domain: cell.Land,
		pos: world.Position{AABB: cellBox(g, g.At(0, 1), 8)}}
	o := MoveOrder{Target: g.At(5, 1), Toward: g.At(1, 1)}
	for i := range 2 {
		before := read.read
		if k.detour(m, &o, body{id: 3, cell: g.At(1, 1)}); read.read == before {
			t.Errorf("detour %d searched nothing, want every one searched: the cells held "+
				"change", i)
		}
	}
}
