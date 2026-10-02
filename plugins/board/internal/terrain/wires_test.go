package terrain_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/entity/kind/comp"
	"github.com/kjkrol/gram/entity/tag"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/internal/engine"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/internal/boardtest"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/rule"
	"github.com/kjkrol/uid"
)

// cellProbe reads every cell entity's roles, wire and fuel once the cells are made.
type cellProbe struct {
	q     *goke.Query
	plot  goke.Comp[cell.Plot]
	roles goke.OptComp[tag.Tags[rule.Roles]]
	wired goke.OptComp[rule.Wired]
	fuel  goke.OptComp[fuel]
}

// fuel is a game's own component on every cell, given through the world's roster.
type fuel struct{ Left int }

func (p *cellProbe) SetupSystems() []goke.System {
	return []goke.System{goke.SystemFn{OnInit: func(si *goke.SysInit) {
		p.q = si.NewQueryBuilder(&p.plot).Optional(&p.roles, &p.wired, &p.fuel).Build()
	}}}
}

// cellState is what a cell entity carries of roles and wires.
type cellState struct {
	id       uid.UID64
	hasRoles bool
	roles    tag.Tags[rule.Roles]
	wired    bool
	to       uint64 // the wire's name, hashed
	fuel     *fuel  // nil for none
}

// read is every cell entity's state, by its cell.
func (p *cellProbe) read(t *testing.T) map[cell.ID]cellState {
	t.Helper()
	out := map[cell.ID]cellState{}
	for p.q.All(); p.q.Next(); {
		cur := p.q.Cursor()
		roles, wired, fuels := p.roles.Slice(cur), p.wired.Slice(cur), p.fuel.Slice(cur)
		for i, plot := range p.plot.Slice(cur) {
			st := cellState{id: cur.IDs[i], hasRoles: roles != nil, wired: wired != nil}
			if fuels != nil {
				st.fuel = &fuels[i]
			}
			if roles != nil {
				st.roles = roles[i]
			}
			if wired != nil {
				st.to = wired[i].To
			}
			if _, dup := out[plot.Cell]; dup {
				t.Fatalf("two entities hold cell %d", plot.Cell)
			}
			out[plot.Cell] = st
		}
	}
	return out
}

// installCells installs w and brd alone, with the probe's query, and sets the ECS up; prepare
// runs before, to define roles and wires and seed the Layout.
func installCells(t *testing.T, g grid.Grid, prepare func(w *world.Plugin, brd *board.Plugin)) (*goke.ECS, *world.Plugin, *board.Plugin, *cellProbe) {
	t.Helper()
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 4 * boardtest.CellSize, Height: 4 * boardtest.CellSize},
		Entities: world.EntitiesCfg{MaxCount: 4, MinSize: boardtest.UnitSize, MaxSize: boardtest.UnitSize},
	})
	brd := board.NewPlugin(g, &cell.MultipleOccupancy{}, w)
	brd.Res.Logic.Board.SetAll(cell.Kind{Name: cell.Named("grass"), Cost: 1, Allows: cell.Land})
	prepare(w, brd)
	if err := brd.Populate(); err != nil {
		t.Fatal(err)
	}
	ctx := boardtest.NewInstallCtx()
	if err := w.Install(ctx); err != nil {
		t.Fatal(err)
	}
	if err := brd.Install(ctx); err != nil {
		t.Fatal(err)
	}
	probe := &cellProbe{}
	ctx.ECS().Setup(append(ctx.Systems(), probe.SetupSystems()...)...)
	return ctx.ECS(), w, brd, probe
}

func wireGrids() map[string]grid.Grid {
	return map[string]grid.Grid{
		"square": grid.DefaultGrids{}.Square(4, 4, boardtest.CellSize),
		"hex":    grid.DefaultGrids{}.Hex(4, 4, boardtest.CellSize/2),
	}
}

// The Layout's roles and wires reach the cell entities: every cell carries its roles, none for
// most; only the wired ones carry a Wired, naming their wire's entity; the cells wired apart are
// still each its own cell's entity, of the kind the seed gave it.
func TestCells_RolesAndWiresReachTheCellEntities(t *testing.T) {
	for name, g := range wireGrids() {
		t.Run(name, func(t *testing.T) {
			var trapdoor, plate *rule.Part
			var west, east *rule.Wire
			cells := make([]cell.ID, 0, 5)
			g.EachCell(func(c cell.ID) {
				if len(cells) < 5 {
					cells = append(cells, c)
				}
			})
			a, b, c, d, e := cells[0], cells[1], cells[2], cells[3], cells[4]
			snow := cell.Kind{Name: cell.Named("snow"), Cost: 3, Allows: cell.Land}
			_, _, brd, probe := installCells(t, g, func(w *world.Plugin, brd *board.Plugin) {
				trapdoor, plate = rule.Role("trapdoor"), rule.Role("plate")
				west, east = w.Wire("west"), w.Wire("east")
				brd.Res.Logic.Board.Set(b, snow)
				brd.Seed(board.Layout{Cells: []cell.Entry{
					{Cell: a, Roles: []*rule.Part{trapdoor}, Wired: west},
					{Cell: b, Roles: []*rule.Part{plate}, Wired: west},
					{Cell: c, Roles: []*rule.Part{trapdoor, plate}, Wired: east},
					{Cell: d, Roles: []*rule.Part{trapdoor}},
					{Cell: e, Wired: east},
				}})
			})
			westID, westMade := west.Entity()
			eastID, eastMade := east.Entity()
			if !westMade || !eastMade || westID == eastID {
				t.Fatalf("wires' entities west %d (%v), east %d (%v): want two, made", westID, westMade, eastID, eastMade)
			}
			westTo, eastTo := west.Wired().To, east.Wired().To
			want := map[cell.ID]cellState{
				a: {hasRoles: true, roles: tag.Tags[rule.Roles](0).With(trapdoor.Tag()), wired: true, to: westTo},
				b: {hasRoles: true, roles: tag.Tags[rule.Roles](0).With(plate.Tag()), wired: true, to: westTo},
				c: {hasRoles: true, roles: tag.Tags[rule.Roles](0).With(trapdoor.Tag(), plate.Tag()), wired: true, to: eastTo},
				d: {hasRoles: true, roles: tag.Tags[rule.Roles](0).With(trapdoor.Tag())},
				e: {hasRoles: true, wired: true, to: eastTo},
			}
			got := probe.read(t)
			if len(got) != g.CellCount() {
				t.Fatalf("%d cell entities, want one per cell, %d", len(got), g.CellCount())
			}
			for at, st := range got {
				w, ok := want[at]
				if !ok {
					w = cellState{hasRoles: true}
				}
				w.id = st.id
				if st != w {
					t.Errorf("cell %d carries %+v, want %+v", at, st, w)
				}
				if id, _ := brd.CellEntity(at); id != st.id {
					t.Errorf("cell %d's entity is %d, the one holding its Plot %d", at, id, st.id)
				}
			}
			if k := brd.Res.Logic.Board.Kind(b); k != snow {
				t.Errorf("wired cell %d is %q, want the snow the seed gave it", b, k.Name)
			}
			if k := brd.Res.Logic.Board.Kind(a); k.Name.String() != "grass" {
				t.Errorf("wired cell %d is %q, want grass", a, k.Name)
			}
		})
	}
}

// panicsWith runs f and fails the test unless it panics with a message holding every one of want.
func panicsWith(t *testing.T, f func(), want ...string) {
	t.Helper()
	defer func() {
		t.Helper()
		r := recover()
		if r == nil {
			t.Errorf("no panic, want one mentioning %q", want)
			return
		}
		msg := fmt.Sprint(r)
		for _, w := range want {
			if !strings.Contains(msg, w) {
				t.Errorf("panic %q, want one mentioning %q", msg, w)
			}
		}
	}()
	f()
}

// A cell wired to a wire no world defined panics as the cells are made, naming the wire.
func TestCells_ACellWiredToAWireNoWorldDefinedPanicsNamingIt(t *testing.T) {
	g := grid.DefaultGrids{}.Square(4, 4, boardtest.CellSize)
	wire := rule.NewWire("ghost")
	c, _ := g.CellIndex(2, 1)
	panicsWith(t, func() {
		installCells(t, g, func(_ *world.Plugin, brd *board.Plugin) {
			brd.Seed(board.Layout{Cells: []cell.Entry{{Cell: c, Wired: wire}}})
		})
	}, wire.String(), "world.Plugin.Wire")
}

// Roles and wires are the Layout's: given once the cells are made, they panic.
func TestCells_RolesAndWiresAfterTheCellsAreMadePanic(t *testing.T) {
	g := grid.DefaultGrids{}.Square(4, 4, boardtest.CellSize)
	c, _ := g.CellIndex(1, 1)
	var trapdoor *rule.Part
	var west *rule.Wire
	_, _, brd, _ := installCells(t, g, func(w *world.Plugin, _ *board.Plugin) {
		trapdoor, west = rule.Role("trapdoor"), w.Wire("west")
	})
	brd.Seed(board.Layout{Cells: []cell.Entry{{Cell: c, Roles: []*rule.Part{trapdoor}}}})
	panicsWith(t, func() { _ = brd.Populate() }, "roles", "Layout")
	brd.Seed(board.Layout{Cells: []cell.Entry{{Cell: c, Wired: west}}})
	panicsWith(t, func() { _ = brd.Populate() }, "wire", "Layout")
}

// wiredStage is a board whose cells play roles and are wired, saved or loaded; decoys defines a
// role and a wire before the ones the cells name, so a load meets them in another order.
type wiredStage struct {
	grid     grid.Grid
	loadFrom string
	decoys   bool

	world    *world.Plugin
	board    *board.Plugin
	trapdoor *rule.Part
	plate    *rule.Part
	west     *rule.Wire
	east     *rule.Wire
	probe    cellProbe
	stack    game.Scenes
}

func (s *wiredStage) Name() string { return "stage" }

func (s *wiredStage) Init(ctx game.Initializer) error {
	s.world = ctx.UseWorld(world.Config{
		Space:    world.SpaceCfg{Width: 4 * boardtest.CellSize, Height: 4 * boardtest.CellSize},
		Entities: world.EntitiesCfg{MaxCount: 4, MinSize: boardtest.UnitSize, MaxSize: boardtest.UnitSize},
	})
	if s.decoys { // a wire defined first: the others' entities are found by name, not by order
		s.world.Wire("decoy")
	}
	s.trapdoor, s.plate = rule.Role("trapdoor"), rule.Role("plate")
	s.west, s.east = s.world.Wire("west"), s.world.Wire("east")
	s.world.Roster().Cell.Default(comp.Const(fuel{Left: 3}))
	s.board = board.NewPlugin(s.grid, &cell.MultipleOccupancy{}, s.world)
	s.board.CellKinds().Create(cell.Kind{Name: cell.Named("grass"), Cost: 1, Allows: cell.Land})
	if err := ctx.Use(s.board); err != nil {
		return err
	}
	ctx.Setup(&s.probe)
	return nil
}

func (s *wiredStage) Restore(p game.Persistence) (bool, error) {
	if s.loadFrom == "" {
		return false, nil
	}
	return true, p.Load(s.loadFrom, "")
}

func (s *wiredStage) Spawn() error {
	a, _ := s.grid.CellIndex(1, 1)
	b, _ := s.grid.CellIndex(2, 3)
	c, _ := s.grid.CellIndex(3, 0)
	s.board.Seed(board.Layout{Default: "grass", Cells: []cell.Entry{
		{Cell: a, Roles: []*rule.Part{s.trapdoor}, Wired: s.west},
		{Cell: b, Roles: []*rule.Part{s.plate}, Wired: s.east},
		{Cell: c, Roles: []*rule.Part{s.trapdoor, s.plate}},
	}})
	return nil
}

func (s *wiredStage) Update(ctx goke.RunCtx, d time.Duration) {
	s.world.RunPlan(ctx, d)
	s.board.RunPlan(ctx, d)
	ctx.Sync()
}

func (s *wiredStage) Stack() game.Scenes {
	if s.stack == nil {
		s.stack, _ = game.NewStack()
	}
	return s.stack
}

// state is what the stage's cells carry, in the stage's own terms: the roles by name, the wires
// by name, the entities as they are.
func (s *wiredStage) state(t *testing.T) map[cell.ID]string {
	t.Helper()
	westTo, eastTo := s.west.Wired().To, s.east.Wired().To
	out := map[cell.ID]string{}
	for c, st := range s.probe.read(t) {
		desc := fmt.Sprintf("entity %d", st.id)
		if !st.hasRoles {
			desc += ", no roles"
		}
		for _, r := range []*rule.Part{s.trapdoor, s.plate} {
			if st.roles.Has(r.Tag()) {
				desc += ", " + r.String()
			}
		}
		if rest := st.roles.Without(s.trapdoor.Tag(), s.plate.Tag()); rest != 0 {
			desc += fmt.Sprintf(", roles %b besides", rest)
		}
		if st.fuel != nil {
			desc += fmt.Sprintf(", fuel %d", st.fuel.Left)
		} else {
			desc += ", no fuel"
		}
		switch {
		case !st.wired:
		case st.to == westTo:
			desc += ", wired west"
		case st.to == eastTo:
			desc += ", wired east"
		default:
			desc += fmt.Sprintf(", wired to %x, no wire's", st.to)
		}
		out[c] = desc
	}
	return out
}

// A loaded game's cells play the roles and are wired to the wires they were saved with, by name:
// the wires found again on their own entities however the build defines them, the cells the
// entities saved.
func TestPlugin_SaveLoad_CellsKeepTheirRolesAndWires(t *testing.T) {
	path := t.TempDir() + "/save"
	g := grid.DefaultGrids{}.Square(4, 4, boardtest.CellSize)

	saved := &wiredStage{grid: g}
	eng := engine.NewEngine(boardtest.OneStageGame{Stage: saved, GameProps: game.Props{}})
	if err := eng.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	before := saved.state(t)
	a, _ := g.CellIndex(1, 1)
	b, _ := g.CellIndex(2, 3)
	c, _ := g.CellIndex(3, 0)
	for at, want := range map[cell.ID]string{a: ", the role trapdoor, fuel 3, wired west", b: ", the role plate, fuel 3, wired east", c: ", the role trapdoor, the role plate, fuel 3"} {
		if !strings.HasSuffix(before[at], want) {
			t.Fatalf("fresh cell %d: %s, want it to end %q", at, before[at], want)
		}
	}
	westBefore, _ := saved.west.Entity()
	if err := eng.Persistence().Save(path, ""); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded := &wiredStage{grid: g, loadFrom: path, decoys: true}
	eng2 := engine.NewEngine(boardtest.OneStageGame{Stage: loaded, GameProps: game.Props{}})
	if err := eng2.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	after := loaded.state(t)
	for at, want := range before {
		if after[at] != want {
			t.Errorf("cell %d after Load: %s, want %s", at, after[at], want)
		}
	}
	if westAfter, _ := loaded.west.Entity(); westAfter != westBefore {
		t.Errorf("wire west's entity after Load is %d, want %d, the one saved", westAfter, westBefore)
	}
}
