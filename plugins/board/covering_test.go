package board_test

import (
	"testing"

	"github.com/kjkrol/ceremony/act"
	"github.com/kjkrol/ceremony/cond"
	"github.com/kjkrol/ceremony/effect"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/internal/boardtest"
	"github.com/kjkrol/gram/plugins/board/unit"
	"github.com/kjkrol/gram/plugins/world"
)

func TestCovering_AnEffectOnACellShowsOnTheBoardAndToTheRules(t *testing.T) {
	g := grid.DefaultGrids{}.Square(4, 4, boardtest.CellSize)
	here := g.At(1, 1)
	var iced effect.Effect
	var slot [2]int
	bw := boardtest.NewWorldWith(t, g, 4*boardtest.CellSize, 4*boardtest.CellSize, func(w *world.Plugin, brd *board.Plugin) []act.Rule {
		brd.Board().SetAll(cell.Kind{Name: cell.Named("water"), Cost: 1, Allows: cell.Land})
		w.Effects().Define("iced", effect.Spec{})
		iced = w.Effects().Named("iced")
		slot[0], slot[1] = int(brd.Covering(iced)), int(brd.Covering(iced))
		return []act.Rule{
			act.Branch[unit.Standing]{
				Name: "freeze",
				When: cond.Self.If(cond.Always),
				Then: act.Branch[any]{
					When: cond.Around(0).If(cond.Not(iced)),
					Then: act.Around(0).Put(iced),
				},
			},
			act.Branch[unit.Standing]{
				Name: "leave the ice",
				When: cond.Self.If(unit.StandingOn(iced)),
				Then: act.Self.Do(world.Despawn{}),
			},
		}
	}, []boardtest.Mover{{Here: here}})
	if slot[0] != slot[1] {
		t.Errorf("the cover's slot is %d, then %d; want the same", slot[0], slot[1])
	}
	brd := bw.Board.Board()
	before := brd.Changes()
	for range 4 {
		bw.Tick()
	}
	if !brd.States(here).Has(iced.Mark()) {
		t.Error("the cell under the unit does not carry the effect's marker")
	}
	if other := g.At(3, 3); brd.States(other).Has(iced.Mark()) {
		t.Error("a cell far off carries the effect's marker")
	}
	if brd.Changes() == before {
		t.Error("the effect coming on the cell did not change the board")
	}
	if n := len(bw.Snapshot()); n != 0 {
		t.Errorf("%d units left, want none: the one over the ice was to leave", n)
	}
}
