package navigation

import (
	"testing"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/entity"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/unit"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/uid"
)

func TestOnward_SendsTheUnitRoundItsOwnRound(t *testing.T) {
	grid := grid.DefaultGrids{}.Square(10, 1, 10)
	terrain := cell.NewTerrainMap()
	terrain.SetAll(cell.Kind{Cost: 1, Allows: cell.Land})
	start, east, west := grid.At(0, 0), grid.At(8, 0), grid.At(1, 0)

	onwards := &control.Queue[Onward]{}
	cmds := newMoveCommandSystem(newPathFinder(grid, terrain, nil, &cell.SingleOccupancy{}),
		&control.Queue[MoveTo]{}, &control.Queue[LookAt]{}, selTags.Selected)
	cmds.onwards = onwards

	var at goke.Comp[unit.At]
	var pos goke.Comp[world.Base]
	var round goke.Comp[Round]
	var order goke.OptComp[MoveOrder]
	var orders *goke.Query
	var walker, plain uid.UID64
	ecs := goke.New()
	ecs.Setup(goke.SystemFn{OnInit: func(si *goke.SysInit) {
		f := si.NewFactory(&at, &pos, &round)
		f.Create(1)
		f.Next()
		walker = f.Cursor.IDs[0]
		at.Slice(&f.Cursor)[0] = unit.At{Cell: start}
		pos.Slice(&f.Cursor)[0].Pos = world.Position{AABB: cellBox(grid, start, 8)}
		round.Slice(&f.Cursor)[0] = NewRound(east, west)
		g := si.NewFactory(&at, &pos)
		g.Create(1)
		g.Next()
		plain = g.Cursor.IDs[0]
		at.Slice(&g.Cursor)[0] = unit.At{Cell: start}
		pos.Slice(&g.Cursor)[0].Pos = world.Position{AABB: cellBox(grid, start, 8)}
		orders = si.NewQueryBuilder(&at).Optional(&order).Build()
		cmds.Init(si)
	}})
	h := ecs.RegSys(cmds)
	ecs.SetPlan(func(ctx goke.RunCtx, d time.Duration) {
		ctx.Run(h, d)
		ctx.Sync()
	})
	target := func(id uid.UID64) (cell.ID, bool) {
		if !orders.Seek(id) {
			t.Fatal("entity gone")
		}
		o := order.At(orders.Cursor())
		if o == nil {
			return 0, false
		}
		return o.Target, true
	}
	for i, want := range []cell.ID{east, west, east} {
		onwards.AddFrom(walker, Onward{})
		ecs.Tick(time.Millisecond)
		if got, ok := target(walker); !ok || got != want {
			t.Errorf("Onward %d: ordered %v to %v, want %v: east, west, east again",
				i, ok, got, want)
		}
	}
	onwards.AddFrom(plain, Onward{})
	ecs.Tick(time.Millisecond)
	if got, ok := target(plain); ok {
		t.Errorf("a unit without a Round was sent to %v by Onward, want nowhere", got)
	}
}

func TestNewRound_RefusesNoCellAndTooMany(t *testing.T) {
	cases := map[string][]cell.ID{"none": nil, "too many": make([]cell.ID, MaxWaypoints+1)}
	for name, cells := range cases {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Error("taken, want a panic")
				}
			}()
			NewRound(cells...)
		})
	}
}

func TestMoveTo_ByNameGoesToTheNamedCellOrUnit(t *testing.T) {
	grid := grid.DefaultGrids{}.Square(10, 1, 10)
	terrain := cell.NewTerrainMap()
	terrain.SetAll(cell.Kind{Cost: 1, Allows: cell.Land})
	start, post, mate := grid.At(0, 0), grid.At(7, 0), grid.At(4, 0)

	moves := &control.Queue[MoveTo]{}
	cmds := newMoveCommandSystem(newPathFinder(grid, terrain, nil, &cell.MultipleOccupancy{}),
		moves, &control.Queue[LookAt]{}, selTags.Selected)

	var at goke.Comp[unit.At]
	var pos goke.Comp[world.Base]
	var label goke.Comp[entity.Label]
	var plot goke.Comp[cell.Plot]
	var order goke.OptComp[MoveOrder]
	var orders *goke.Query
	var walker uid.UID64
	ecs := goke.New()
	ecs.Setup(goke.SystemFn{OnInit: func(si *goke.SysInit) {
		f := si.NewFactory(&at, &pos)
		f.Create(1)
		f.Next()
		walker = f.Cursor.IDs[0]
		at.Slice(&f.Cursor)[0] = unit.At{Cell: start}
		pos.Slice(&f.Cursor)[0].Pos = world.Position{AABB: cellBox(grid, start, 8)}
		named := si.NewFactory(&label, &plot)
		named.Create(1)
		named.Next()
		label.Slice(&named.Cursor)[0] = entity.LabelOf("post", "")
		plot.Slice(&named.Cursor)[0] = cell.Plot{Cell: post}
		friend := si.NewFactory(&at, &pos, &label)
		friend.Create(1)
		friend.Next()
		at.Slice(&friend.Cursor)[0] = unit.At{Cell: mate}
		pos.Slice(&friend.Cursor)[0].Pos = world.Position{AABB: cellBox(grid, mate, 8)}
		label.Slice(&friend.Cursor)[0] = entity.LabelOf("mate", "")
		orders = si.NewQueryBuilder(&at).Optional(&order).Build()
		cmds.Init(si)
	}})
	h := ecs.RegSys(cmds)
	ecs.SetPlan(func(ctx goke.RunCtx, d time.Duration) {
		ctx.Run(h, d)
		ctx.Sync()
	})
	target := func() (cell.ID, bool) {
		if !orders.Seek(walker) {
			t.Fatal("entity gone")
		}
		o := order.At(orders.Cursor())
		if o == nil {
			return 0, false
		}
		return o.Target, true
	}
	for _, c := range []struct {
		name string
		want cell.ID
		sent bool
	}{{"post", post, true}, {"mate", mate, true}, {"nobody", 0, false}} {
		if orders.Seek(walker) {
			if o := order.At(orders.Cursor()); o != nil {
				*o = MoveOrder{Target: start}
			}
		}
		moves.AddFrom(walker, MoveTo{To: entity.Named(c.name)})
		ecs.Tick(time.Millisecond)
		got, has := target()
		sent := has && got != start
		if sent != c.sent || (sent && got != c.want) {
			t.Errorf("MoveTo %q: sent %v to %v, want sent %v to %v",
				c.name, sent, got, c.sent, c.want)
		}
	}
}
