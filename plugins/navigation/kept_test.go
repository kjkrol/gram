package navigation

import (
	"github.com/kjkrol/gram/plugins/navigation/internal/bodies"
	"github.com/kjkrol/gram/plugins/navigation/internal/spacing"
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
	kept := spacing.NewCellKeeping(spacing.NewFinder(g, read, nil, spacing.OpenOccupancy{}),
		&cell.SingleOccupancy{})
	fresh := spacing.NewCellKeeping(spacing.NewFinder(g, board, nil, &cell.MultipleOccupancy{}),
		&cell.SingleOccupancy{})
	stand := func(c cell.ID, size uint32) bodies.Member {
		return bodies.Member{ID: 7, Cell: c, From: c, Domain: cell.Land,
			Pos: world.Position{AABB: cellBox(g, c, size)}}
	}
	m := stand(at(0, 1), 8)
	o := MoveOrder{Target: at(5, 1), Toward: at(1, 1)}
	mf, of := m, o
	other := bodies.Body{ID: 3, Cell: at(1, 1)}
	for _, step := range []struct {
		name     string
		change   func(m *bodies.Member, o *MoveOrder)
		searched bool
	}{
		{"asked first", nil, true},
		{"asked again", nil, false},
		{"another cell learnt", func(_ *bodies.Member, o *MoveOrder) { o.Toward = at(1, 0) }, true},
		{"asked again", nil, false},
		{"from another cell", func(m *bodies.Member, _ *MoveOrder) { *m = stand(at(0, 0), 8) }, true},
		{"to another target", func(_ *bodies.Member, o *MoveOrder) { o.Target = at(5, 2) }, true},
		{"to another level", func(_ *bodies.Member, o *MoveOrder) { o.Level = OnGround }, true},
		{"a body of another size", func(m *bodies.Member, _ *MoveOrder) { *m = stand(m.Cell, 9) }, true},
		{"the board changed", func(*bodies.Member, *MoveOrder) {
			for _, y := range []uint32{0, 2} {
				board.Set(at(3, y), wall)
			}
		}, true},
		{"asked again", nil, false},
		{"no way round a cell learnt", func(_ *bodies.Member, o *MoveOrder) { o.Toward = at(3, 1) },
			true},
		{"cornered, waiting", nil, false},
		{"its wait over, asked again", func(_ *bodies.Member, o *MoveOrder) { o.Retry = 0 }, false},
	} {
		if step.change != nil {
			step.change(&m, &o)
			step.change(&mf, &of)
		}
		before := read.read
		kept.Detour(m, &o, other)
		fresh.Detour(mf, &of, other)
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
	k := spacing.NewCellKeeping(spacing.NewFinder(g, read, nil, occ), occ)
	m := bodies.Member{ID: 7, Cell: g.At(0, 1), From: g.At(0, 1), Domain: cell.Land,
		Pos: world.Position{AABB: cellBox(g, g.At(0, 1), 8)}}
	o := MoveOrder{Target: g.At(5, 1), Toward: g.At(1, 1)}
	for i := range 2 {
		before := read.read
		if k.Detour(m, &o, bodies.Body{ID: 3, Cell: g.At(1, 1)}); read.read == before {
			t.Errorf("detour %d searched nothing, want every one searched: the cells held "+
				"change", i)
		}
	}
}
