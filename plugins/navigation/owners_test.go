package navigation

import (
	"github.com/kjkrol/gram/plugins/navigation/internal/routing"
	"github.com/kjkrol/gram/plugins/navigation/internal/spacing"
	"testing"
	"time"

	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/ceremony/tag"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/unit"
	"github.com/kjkrol/gram/plugins/players/owner"
	"github.com/kjkrol/gram/plugins/selection"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/uid"
)

func TestMoveTo_SendsThePlayersOwnSelectedUnitsAlone(t *testing.T) {
	grid := grid.DefaultGrids{}.Square(10, 1, 10)
	terrain := cell.NewTerrainMap()
	terrain.SetAll(cell.Kind{Cost: 1, Allows: cell.Land})
	start := grid.At(0, 0)
	oldTarget := grid.At(3, 0)
	newTarget := grid.At(8, 0)

	moves := &command.Queue[MoveTo]{}
	cmds := routing.NewSystem(spacing.NewFinder(grid, terrain, nil, &cell.MultipleOccupancy{}), moves, &command.Queue[LookAt]{}, selTags.Selected)
	var at goke.Comp[unit.At]
	var pos goke.Comp[world.Base]
	var orderComp goke.Comp[MoveOrder]
	var marks goke.Comp[tag.Tags[selection.Family]]
	var owners goke.Comp[tag.Tags[owner.Family]]
	var read *goke.Query
	var ids []uid.UID64
	ecs := goke.New()
	ecs.Setup(goke.SystemFn{OnInit: func(si *goke.SysInit) {
		f := si.NewFactory(&at, &pos, &orderComp, &marks, &owners)
		f.Create(3)
		f.Next()
		ids = append(ids, f.Cursor.IDs...)
		for i, by := range []command.Issuer{1, 2, command.Nobody} {
			at.Slice(&f.Cursor)[i] = unit.At{Cell: start}
			pos.Slice(&f.Cursor)[i].Pos = world.Position{AABB: cellBox(grid, start, 8)}
			orderComp.Slice(&f.Cursor)[i] = MoveOrder{Target: oldTarget, Path: Path{Length: 1}}
			marks.Slice(&f.Cursor)[i] = selectedMarks
			if by != command.Nobody {
				owners.Slice(&f.Cursor)[i] = tag.Tags[owner.Family](0).With(owner.Of(by))
			}
		}
		read = si.NewQueryBuilder(&orderComp).Build()
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
			got[id] = orderComp.Slice(read.Cursor())[i].Target
		}
	}
	if got[ids[0]] != newTarget || got[ids[1]] != oldTarget || got[ids[2]] != oldTarget {
		t.Errorf("targets: player 1's %v, player 2's %v, nobody's %v; want player 1's alone sent to %v",
			got[ids[0]], got[ids[1]], got[ids[2]], newTarget)
	}
}
