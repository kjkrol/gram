package navigation

import (
	"testing"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/players/owner"
	"github.com/kjkrol/gram/plugins/selection"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/entity/tag"
	"github.com/kjkrol/uid"
)

// A MoveTo sends the selected units of the player who gave it alone: another player's selected
// units, and those nobody owns, keep their orders.
func TestMoveTo_SendsThePlayersOwnSelectedUnitsAlone(t *testing.T) {
	grid := board.DefaultGrids{}.Square(10, 1, 10)
	terrain := board.NewTerrainMap()
	terrain.SetAll(cell.Kind{Cost: 1, Allows: cell.Land})
	start, _ := grid.CellIndex(0, 0)
	oldTarget, _ := grid.CellIndex(3, 0)
	newTarget, _ := grid.CellIndex(8, 0)

	moves := &control.Queue[MoveTo]{}
	cmds := newMoveCommandSystem(newPathFinder(grid, terrain, nil, &board.MultipleOccupancy{}), moves, &control.Queue[LookAt]{}, selTags.Selected)
	var at goke.Comp[board.At]
	var pos goke.Comp[world.Base]
	var order goke.Comp[MoveOrder]
	var marks goke.Comp[tag.Tags[selection.Family]]
	var owners goke.Comp[tag.Tags[owner.Family]]
	var read *goke.Query
	var ids []uid.UID64
	ecs := goke.New()
	ecs.Setup(goke.SystemFn{OnInit: func(si *goke.SysInit) {
		f := si.NewFactory(&at, &pos, &order, &marks, &owners)
		f.Create(3)
		f.Next()
		ids = append(ids, f.Cursor.IDs...)
		for i, by := range []control.PlayerID{1, 2, control.Nobody} {
			at.Slice(&f.Cursor)[i] = board.At{Cell: start}
			pos.Slice(&f.Cursor)[i].Pos = world.Position{AABB: board.CellAABB(grid, start, 8)}
			order.Slice(&f.Cursor)[i] = MoveOrder{Target: oldTarget, Path: Path{Length: 1}}
			marks.Slice(&f.Cursor)[i] = selectedMarks
			if by != control.Nobody {
				owners.Slice(&f.Cursor)[i] = tag.Tags[owner.Family](0).With(owner.Of(by))
			}
		}
		read = si.NewQueryBuilder(&order).Build()
		cmds.Init(si)
	}})
	run := ecs.RegSys(cmds)
	ecs.SetPlan(func(ctx goke.RunCtx, d time.Duration) { ctx.Run(run, d); ctx.Sync() })

	moves.Add(1, MoveTo{Cell: newTarget})
	ecs.Tick(time.Second)

	got := map[uid.UID64]cell.ID{}
	read.All()
	for read.Next() {
		for i, id := range read.Cursor().IDs {
			got[id] = order.Slice(read.Cursor())[i].Target
		}
	}
	if got[ids[0]] != newTarget || got[ids[1]] != oldTarget || got[ids[2]] != oldTarget {
		t.Errorf("targets: player 1's %v, player 2's %v, nobody's %v; want player 1's alone sent to %v",
			got[ids[0]], got[ids[1]], got[ids[2]], newTarget)
	}
}
