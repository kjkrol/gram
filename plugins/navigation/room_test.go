package navigation

import (
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/internal/f32"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/unit"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/driving"
	"github.com/kjkrol/gram/plugins/motion"
	"github.com/kjkrol/gram/plugins/motion/steering"
	"github.com/kjkrol/gram/plugins/selection"
	"github.com/kjkrol/gram/plugins/topography"
	"github.com/kjkrol/gram/plugins/world"
)

// a house of two cells, (3, 3) and (4, 3), on ground falling 4 a cell southward, dug into it: its
// floor as high as the ground in the middle of its door, in the first cell's southern edge, 16
const (
	houseFloor float32 = 16
	houseClear float32 = 10
)

var houseDoor = cell.Gap{Width: 6, Height: 8}

type houseRow struct {
	At    cell.ID
	Links cell.Links
}

type housedRow struct {
	At     geom.Vec
	Target cell.ID
}

// housedWorld is the house with walkers sent into it, one too tall for its door, and walkers
// walked north by hand
func housedWorld(t *testing.T, walkers, tall, driven []housedRow) (*goke.ECS,
	map[string]*goke.Query, *goke.Comp[world.Base], *goke.Comp[world.Z], grid.Grid) {
	t.Helper()
	g := grid.DefaultGrids{}.Square(8, 8, 32)
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 256, Height: 256},
		Entities: world.EntitiesCfg{MaxCount: 16, MinSize: 2, MaxSize: 32},
		Heights:  true,
	})
	m := motion.NewPlugin(w)
	c := collision.NewPlugin(w, m)
	brd := board.NewPlugin(g, &cell.MultipleOccupancy{}, w).WithMotion(m, cell.DefaultHiking).
		WithCollision(c)
	topo := topography.NewPlugin(w, brd, nil, topography.Config{})
	sel := selection.NewPlugin(w)
	drive := driving.NewPlugin(w, m, sel).WithGround(brd)
	nav := NewPlugin(brd, w, m, sel, drive).WithCollision(c).WithSpacing(BodySpacing)
	brd.Write().Board().SetAll(cell.Kind{Pace: 1, Allows: cell.Land})
	topo.Write().Relief().SetHeights(func(p geom.Vec) float32 { return (256 - p.Y) / 8 })
	east, _ := grid.Direction(g, 1, 0)
	west, _ := grid.Direction(g, -1, 0)
	south, _ := grid.Direction(g, 0, 1)
	rooms := board.NewEntities(brd, board.Shape{Size: 32}, func(r houseRow) cell.ID { return r.At })
	rooms.Define("room", comp.Load(func(r houseRow) cell.Floor {
		return cell.Floor{
			Allows:   cell.Land,
			Height:   houseFloor - topo.Write().Relief().Altitude(r.At),
			Pace:     1,
			Graded:   true,
			Links:    r.Links,
			Storey:   1,
			Clear:    houseClear,
			Standing: true,
			Door:     houseDoor,
		}
	}))
	w.Write().Seed(rooms.Named("room").Entry(houseRow{At: g.At(3, 3), Links: 1<<east | 1<<south}))
	w.Write().Seed(rooms.Named("room").Entry(houseRow{At: g.At(4, 3), Links: 1 << west}))
	move := comp.Load(func(r housedRow) MoveOrder {
		return MoveOrder{Target: r.Target, Level: EitherLevel}
	})
	steer := steering.Steering{MaxSpeed: 24, Accel: 48, Brake: 96, V0: 12, TurnRate: 0.15}
	at := func(r housedRow) geom.Vec { return r.At }
	board.NewUnits(brd, board.Shape{Size: 3, Height: 6}, at).Define("walker",
		unit.Mover{Domain: cell.Land}, steer, move)
	board.NewUnits(brd, board.Shape{Size: 3, Height: 9}, at).Define("tall",
		unit.Mover{Domain: cell.Land}, steer, move)
	board.NewUnits(brd, board.Shape{Size: 3, Height: 6}, at).Define("driven",
		unit.Mover{Domain: cell.Land}, steer,
		comp.Const(steering.Driven{Ahead: 1, Face: geom.NewVec(0, -1), Held: time.Second}))
	for kind, rows := range map[string][]housedRow{"walker": walkers, "tall": tall,
		"driven": driven} {
		for _, r := range rows {
			w.Write().Seed(board.NewUnits(brd, board.Shape{}, at).Named(kind).Entry(r))
		}
	}
	if err := w.Carry(nav); err != nil {
		t.Fatal(err)
	}
	ctx := &stubInstallCtx{ecs: goke.New()}
	for _, install := range []func() error{
		func() error { return w.Install(ctx) },
		func() error { return m.Install(ctx) },
		func() error { return c.Install(ctx) },
		func() error { return brd.Install(ctx) },
		func() error { return topo.Install(ctx) },
		func() error { return sel.Install(ctx) },
		func() error { return drive.Install(ctx) },
		func() error { return nav.Install(ctx) },
	} {
		if err := install(); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Populate(); err != nil {
		t.Fatal(err)
	}
	var systems []goke.System
	for _, produce := range ctx.pending {
		systems = append(systems, produce()...)
	}
	var base goke.Comp[world.Base]
	var z goke.Comp[world.Z]
	var hand goke.Comp[steering.Driven]
	var steers goke.Comp[steering.Steering]
	queries := map[string]*goke.Query{}
	systems = append(systems, goke.SystemFn{OnInit: func(si *goke.SysInit) {
		queries["all"] = si.NewQueryBuilder(&base, &z, &steers).Build()
		queries["driven"] = si.NewQueryBuilder(&base, &z, &hand).Build()
	}})
	ctx.ecs.Setup(systems...)
	ctx.ecs.SetPlan(func(rc goke.RunCtx, d time.Duration) {
		w.RunPlan(rc, d)
		m.RunPlan(rc, d)
		c.RunPlan(rc, d)
		brd.RunPlan(rc, d)
		topo.RunPlan(rc, d)
		nav.RunPlan(rc, d)
		drive.RunPlan(rc, d)
		rc.Sync()
		w.Write().Clock().Replay(rc, d)
	})
	return ctx.ecs, queries, &base, &z, g
}

// inside says whether p lies in the house's cells
func inside(g grid.Grid, p geom.Vec) bool {
	c, _ := g.CellAt(p)
	return c == g.At(3, 3) || c == g.At(4, 3)
}

// entry follows the bodies of height high: where each first stood in the house, and whether one
// stood in it off its floor
type entry struct {
	first  map[float32]geom.Vec
	offset bool
	last   map[float32]geom.Vec
}

func walk(ecs *goke.ECS, q *goke.Query, base *goke.Comp[world.Base], z *goke.Comp[world.Z],
	g grid.Grid, seconds int) entry {
	e := entry{first: map[float32]geom.Vec{}, last: map[float32]geom.Vec{}}
	for range 60 * seconds {
		ecs.Tick(time.Second / 60)
		for q.All(); q.Next(); {
			zs := z.Slice(q.Cursor())
			for i, b := range base.Slice(q.Cursor()) {
				at, key := b.Pos.Center(), zs[i].Height
				if inside(g, at) {
					if _, ok := e.first[key]; !ok {
						e.first[key] = at
					}
					if f32.Abs(zs[i].Altitude-houseFloor) > 0.5 {
						e.offset = true
					}
				}
				e.last[key] = at
			}
		}
	}
	return e
}

func TestNavigation_WalksIntoAHouseByItsDoorOntoItsLevelFloor(t *testing.T) {
	ecs, qs, base, z, g := housedWorld(t,
		[]housedRow{{At: geom.NewVec(48, 208), Target: g4(4, 3)}}, nil, nil)
	e := walk(ecs, qs["all"], base, z, g, 30)
	first, ok := e.first[6]
	switch {
	case !ok:
		t.Fatalf("after 30 s the walker stands at %v, never in the house", e.last[6])
	case first.Y < 124 || f32.Abs(first.X-112) > houseDoor.Width/2:
		t.Errorf("the walker came in at %v, want through the door at (112, 128)", first)
	}
	if c, _ := g.CellAt(e.last[6]); c != g.At(4, 3) {
		t.Errorf("after 30 s the walker stands at %v, want in the house's far cell", e.last[6])
	}
	if e.offset {
		t.Errorf("in the house the walker stood off its floor %v", houseFloor)
	}
}

func TestNavigation_OneTallerThanTheDoorStaysOut(t *testing.T) {
	ecs, qs, base, z, g := housedWorld(t, nil,
		[]housedRow{{At: geom.NewVec(48, 208), Target: g4(4, 3)}}, nil)
	e := walk(ecs, qs["all"], base, z, g, 20)
	if at, ok := e.first[9]; ok {
		t.Errorf("a walker 9 tall came in by a door 8 tall, at %v", at)
	}
}

func TestDriving_WalksIntoAHouseByItsDoorAndNotThroughItsWall(t *testing.T) {
	ecs, qs, base, z, g := housedWorld(t, nil, nil, []housedRow{
		{At: geom.NewVec(112, 150)},
	})
	e := walk(ecs, qs["driven"], base, z, g, 4)
	if c, _ := g.CellAt(e.last[6]); c != g.At(3, 3) {
		t.Errorf("walked north by the door the walker stands at %v, want in the house", e.last[6])
	}
	if e.offset {
		t.Errorf("in the house the walker stood off its floor %v", houseFloor)
	}
	ecs, qs, base, z, g = housedWorld(t, nil, nil, []housedRow{{At: geom.NewVec(144, 150)}})
	e = walk(ecs, qs["driven"], base, z, g, 4)
	if at, ok := e.first[6]; ok {
		t.Errorf("walked north into the wall the walker came in at %v", at)
	}
}

func g4(x, y uint32) cell.ID { return grid.DefaultGrids{}.Square(8, 8, 32).At(x, y) }
