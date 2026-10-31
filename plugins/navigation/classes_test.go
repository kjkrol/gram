package navigation

import (
	"slices"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/unit"
	"github.com/kjkrol/gram/plugins/driving"
	"github.com/kjkrol/gram/plugins/motion"
	"github.com/kjkrol/gram/plugins/motion/steering"
	"github.com/kjkrol/gram/plugins/navigation/internal/pathfind"
	"github.com/kjkrol/gram/plugins/navigation/internal/routing"
	"github.com/kjkrol/gram/plugins/selection"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/uid"
)

// a kind of unit defined over the board for the classes' world: its name, its box's side and its
// weight, the domain it moves in, the row it stands at in the west and is sent east along
type classedKind struct {
	name   string
	side   float32
	weight float32
	domain cell.Domain
	row    uint32
}

// the classes' world: a board 24 cells by 8 of 32, a river down column 12 crossed by a rope
// bearing 100 kg in row 1 and a ford in row 7, a unit of each kind sent across it
type classedWorld struct {
	nav    *Plugin
	coord  *routing.Coordinator
	grid   grid.Grid
	ids    map[string]uid.UID64
	orders goke.Comp[MoveOrder]
	q      *goke.Query
	ecs    *goke.ECS
}

var (
	classedGrid        = grid.DefaultGrids{}.Square(24, 8, 32)
	ropeCell, fordCell = classedGrid.At(12, 1), classedGrid.At(12, 7)
)

func newClassedWorld(t *testing.T, kinds ...classedKind) *classedWorld {
	t.Helper()
	g := classedGrid
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 24 * 32, Height: 8 * 32},
		Entities: world.EntitiesCfg{MaxCount: 16, MinSize: 2, MaxSize: 32},
		Heights:  true,
	})
	m := motion.NewPlugin(w)
	brd := board.NewPlugin(g, &cell.MultipleOccupancy{}, w).WithMotion(m, cell.DefaultHiking)
	sel := selection.NewPlugin(w)
	drive := driving.NewPlugin(w, m, sel).WithGround(brd)
	nav := NewPlugin(brd, w, m, sel, drive).WithSpacing(CellSpacing).
		WithHierarchy(Hierarchy{Side: 4, From: 8})
	b := brd.Write().Board()
	b.SetAll(cell.Kind{Pace: 1, Allows: cell.Land})
	for y := range uint32(8) {
		b.Set(g.At(12, y), cell.Kind{Pace: 1, Allows: cell.Water})
	}
	floors := board.NewEntities(brd, board.Shape{Size: 32},
		func(r barrierRow) cell.ID { return r.At })
	floors.Define("rope", comp.Const(cell.Floor{Allows: cell.Land, Load: 100}))
	floors.Define("ford", comp.Const(cell.Floor{Allows: cell.Land, Pace: 0.5}))
	w.Write().Seed(floors.Named("rope").Entry(barrierRow{At: ropeCell}))
	w.Write().Seed(floors.Named("ford").Entry(barrierRow{At: fordCell}))
	profile := steering.Steering{MaxSpeed: 48, Accel: 96, Brake: 192, V0: 24, TurnRate: 0.15}
	for _, k := range kinds {
		units := board.NewUnits(brd, board.Shape{Size: k.side, Height: 1.5},
			func(r walkerRow) geom.Vec { return r.At })
		units.Define(k.name, unit.Mover{Domain: k.domain, Weight: k.weight}, profile,
			comp.Load(func(r walkerRow) MoveOrder { return MoveOrder{Target: r.Target} }))
		w.Write().Seed(units.Named(k.name).Entry(walkerRow{At: g.CellCenter(g.At(1, k.row)),
			Target: g.At(22, k.row)}))
	}
	if err := w.Carry(nav); err != nil {
		t.Fatal(err)
	}
	ctx := &stubInstallCtx{ecs: goke.New()}
	for _, install := range []func() error{
		func() error { return w.Install(ctx) },
		func() error { return m.Install(ctx) },
		func() error { return brd.Install(ctx) },
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
	cw := &classedWorld{nav: nav, grid: g, ids: map[string]uid.UID64{}}
	var systems []goke.System
	for _, produce := range ctx.pending {
		systems = append(systems, produce()...)
	}
	var base goke.Comp[world.Base]
	systems = append(systems, goke.SystemFn{OnInit: func(si *goke.SysInit) {
		cw.q = si.NewQueryBuilder(&cw.orders, &base).Build()
	}})
	ctx.ecs.Setup(systems...)
	ctx.ecs.SetPlan(func(rc goke.RunCtx, d time.Duration) {
		w.RunPlan(rc, d)
		m.RunPlan(rc, d)
		brd.RunPlan(rc, d)
		nav.RunPlan(rc, d)
		drive.RunPlan(rc, d)
		rc.Sync()
		w.Write().Clock().Replay(rc, d)
	})
	for cw.q.All(); cw.q.Next(); {
		cur := cw.q.Cursor()
		for i, id := range cur.IDs {
			for _, k := range kinds {
				if kid, _, _, _ := w.Kinds().Lookup(k.name); base.Slice(cur)[i].TypeID == kid {
					cw.ids[k.name] = id
				}
			}
		}
	}
	cw.ecs, cw.coord = ctx.ecs, nav.module.navigationSystem.Routing.(*routing.Coordinator)
	return cw
}

func (cw *classedWorld) order(name string) *MoveOrder {
	for cw.q.All(); cw.q.Next(); {
		cur := cw.q.Cursor()
		for i, id := range cur.IDs {
			if id == cw.ids[name] {
				return &cw.orders.Slice(cur)[i]
			}
		}
	}
	panic("no unit of " + name)
}

// climbed is how many routes the units climbed as the first tick sent them, the hierarchy of a
// board this small built as it began
func (cw *classedWorld) climbed() uint64 {
	cw.ecs.Tick(time.Second / 60)
	return cw.coord.Climbs()
}

var (
	light  = classedKind{name: "light", side: 12, weight: 80, domain: cell.Land, row: 1}
	heavy  = classedKind{name: "heavy", side: 12, weight: 1500, domain: cell.Land, row: 2}
	lithe  = classedKind{name: "lithe", side: 12, weight: 80, domain: cell.Land, row: 3}
	flying = classedKind{name: "flying", side: 6, domain: cell.Air, row: 4}
)

// the kinds defined over the board are the classes: one a body of a domain, the kinds alike
// one, each domain's apart, a body as the board gives its units
func TestNavigation_TheKindsOfUnitAreItsClasses(t *testing.T) {
	cw := newClassedWorld(t, heavy, light, lithe, flying)
	body := func(k classedKind) pathfind.Body {
		return pathfind.Body{Step: 1.5 / 4, Height: 1.5, Size: geom.NewVec(k.side, k.side),
			Weight: k.weight}
	}
	land := []pathfind.Body{body(light), body(heavy)}
	if got := cw.nav.classes.Of(cell.Land); !slices.Equal(got, land) {
		t.Errorf("the land's classes %v, want %v", got, land)
	}
	if got := cw.nav.classes.Of(cell.Air); !slices.Equal(got, []pathfind.Body{body(flying)}) {
		t.Errorf("the air's classes %v, want the flying kind's", got)
	}
}

// the light and the heavy climb their far routes through one hierarchy, built once for both: the
// light over the rope, the heavy round by the ford
func TestNavigation_TheClassesOfADomainClimbThroughOneHierarchy(t *testing.T) {
	cw := newClassedWorld(t, light, heavy)
	if climbed := cw.climbed(); climbed != 2 {
		t.Fatalf("%d routes climbed, want both", climbed)
	}
	lightRoute, heavyRoute := cw.order(light.name).Path, cw.order(heavy.name).Path
	steps := func(p Path) []cell.ID { return p.Steps[:p.Length] }
	if !slices.Contains(steps(lightRoute), ropeCell) {
		t.Errorf("the light's route %v, want it over the rope", steps(lightRoute))
	}
	if slices.Contains(steps(heavyRoute), ropeCell) ||
		!slices.Contains(steps(heavyRoute), fordCell) {
		t.Errorf("the heavy's route %v, want it round by the ford", steps(heavyRoute))
	}
	alone := newClassedWorld(t, light)
	alone.climbed()
	if built, once := cw.coord.Builds(), alone.coord.Builds(); built != once {
		t.Errorf("%d clusters built for the two classes, want the %d of the light's alone",
			built, once)
	}
}
