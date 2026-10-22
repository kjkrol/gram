package topotest

import (
	"testing"
	"time"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony"
	"github.com/kjkrol/ceremony/act"
	"github.com/kjkrol/ceremony/host"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/internal/parallel"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/ground"
	"github.com/kjkrol/gram/plugins/cameras"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/motion"
	"github.com/kjkrol/gram/plugins/topography"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

// Hill is the kind of a QuasiWorld's hill.
var Hill = cell.Kind{Name: cell.Named("hill"), Cost: 1, Allows: cell.Land | cell.Air}

// RaiseHills puts the hill cells at 12 and the rest at 0, each corner at the mean of its cells.
func RaiseHills(r topography.Relief, grid grid.Grid, hills ...cell.ID) {
	r.SetHeights(ground.MeanOfCells(grid, func(c cell.ID) float64 {
		for _, h := range hills {
			if c == h {
				return 12
			}
		}
		return 0
	}))
}

// InstallCtx is a plugin.Installer over a bare ECS.
type InstallCtx struct {
	hosts   []host.Host
	ecs     *goke.ECS
	pending []func() []goke.System
	drawn   []render.Source
}

func (c *InstallCtx) UseModule(m goke.Module) {
	regSys := goke.SystemFn{OnInit: func(*goke.SysInit) { m.RegSystems(c.ecs) }}
	c.pending = append(c.pending, func() []goke.System { return append(m.SetupSystems(), regSys) })
}
func (c *InstallCtx) Setup(providers ...goke.SetupProvider) {
	for _, p := range providers {
		c.pending = append(c.pending, p.SetupSystems)
	}
}
func (c *InstallCtx) RegSys(factory func() goke.System) goke.Runnable { return c.ecs.RegSys(factory()) }
func (c *InstallCtx) ECS() *goke.ECS                                  { return c.ecs }

// Recruit is a QuasiWorld unit's row: the cell it starts in.
type Recruit struct{ Start cell.ID }

// QuasiWorld is a world with heights over a 4x4 board in relief with a hill at (2,1).
type QuasiWorld struct {
	ECS    *goke.ECS
	World  *world.Plugin
	Motion *motion.Plugin
	Board  *board.Plugin
	Topo   *topography.Plugin
	Grid   grid.Grid
}

// NewQuasiWorld is a QuasiWorld with the units define makes; collide adds the collision plugin.
func NewQuasiWorld(t *testing.T, collide bool, define func(units *board.Units[Recruit], grid grid.Grid) []kind.Entry) *QuasiWorld {
	t.Helper()
	withWorld := func(qw *QuasiWorld, units *board.Units[Recruit]) []kind.Entry {
		return define(units, qw.Grid)
	}
	return NewQuasiWorldWith(t, collide, withWorld)
}

// NewQuasiWorldWith is NewQuasiWorld with the world handed to define, for its board's own
// kinds besides.
func NewQuasiWorldWith(
	t *testing.T,
	collide bool,
	define func(qw *QuasiWorld, units *board.Units[Recruit]) []kind.Entry,
) *QuasiWorld {
	t.Helper()
	qw := &QuasiWorld{Grid: grid.DefaultGrids{}.Square(4, 4, 32)}
	qw.World = world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 128, Height: 128},
		Entities: world.EntitiesCfg{MaxCount: 8, MinSize: 20, MaxSize: 20},
		Heights:  true,
	})
	qw.Motion = motion.NewPlugin(qw.World)
	var c *collision.Plugin
	if collide {
		c = collision.NewPlugin(qw.World, qw.Motion)
	}
	qw.Board = board.NewPlugin(qw.Grid, &cell.MultipleOccupancy{}, qw.World).WithMotion(qw.Motion)
	qw.Topo = topography.NewPlugin(qw.World, qw.Board, nil, topography.Config{})
	qw.Board.Write().Board().SetAll(cell.Kind{Name: cell.Named("grass"), Cost: 1, Allows: cell.Land | cell.Air})
	hillCell := qw.Grid.At(2, 1)
	qw.Board.Write().Board().Set(hillCell, Hill)
	RaiseHills(qw.Topo.Relief(), qw.Grid, hillCell)
	units := board.NewUnits[Recruit](qw.Board, board.Shape{Size: 20, Height: 2}, func(r Recruit) geom.Vec { return qw.Grid.CellCenter(r.Start) })
	entries := define(qw, units)

	ctx := NewInstallCtx()
	if err := qw.World.Install(ctx); err != nil {
		t.Fatal(err)
	}
	if err := qw.Motion.Install(ctx); err != nil {
		t.Fatal(err)
	}
	if c != nil {
		if err := c.Install(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if err := qw.Board.Install(ctx); err != nil {
		t.Fatal(err)
	}
	if err := qw.Topo.Install(ctx); err != nil {
		t.Fatal(err)
	}
	qw.World.Write().Seed(entries...)
	if err := qw.World.Populate(); err != nil {
		t.Fatal(err)
	}
	ctx.ecs.Setup(ctx.Systems()...)
	ctx.ecs.SetPlan(func(rc goke.RunCtx, d time.Duration) {
		settling := qw.World.Write().Clock().Settling(rc)
		qw.World.RunPlan(settling, d)
		qw.Motion.RunPlan(settling, d)
		if c != nil {
			c.RunPlan(settling, d)
		}
		qw.Board.RunPlan(settling, d)
		qw.Topo.RunPlan(settling, d)
		settling.Sync()
		qw.World.Write().Clock().Replay(rc, d)
	})
	qw.ECS = ctx.ecs
	return qw
}

// Zs lists every entity's Z with its kind.
func (qw *QuasiWorld) Zs() map[kind.ID][]world.Z {
	var base goke.Comp[world.Base]
	var z goke.Comp[world.Z]
	var q *goke.Query
	qw.ECS.RegSys(goke.SystemFn{OnInit: func(si *goke.SysInit) { q = si.NewQueryBuilder(&base, &z).Build() }})
	out := map[kind.ID][]world.Z{}
	for q.All(); q.Next(); {
		cur := q.Cursor()
		for i := range cur.IDs {
			out[base.Slice(cur)[i].TypeID] = append(out[base.Slice(cur)[i].TypeID], z.Slice(cur)[i])
		}
	}
	return out
}

// NewInstallCtx is an InstallCtx over a new ECS.
func NewInstallCtx() *InstallCtx { return &InstallCtx{ecs: goke.New()} }

// Systems are the systems the installed plugins asked for, to hand to the ECS's Setup.
func (c *InstallCtx) Systems() []goke.System {
	var systems []goke.System
	for _, produce := range c.pending {
		systems = append(systems, produce()...)
	}
	return systems
}

// NewWorld is a world with heights 256 x 256, its edges as given.
func NewWorld(edges aabbworld.Edges) *world.Plugin {
	return world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 256, Height: 256, Edges: edges},
		Entities: world.EntitiesCfg{MaxCount: 4, MinSize: 1, MaxSize: 20},
		Heights:  true,
	})
}

// Camera is the main camera of a cameras plugin over w as cfg says, 128 x 64, beginning
// isometrically.
func Camera(w *world.Plugin, cfg cameras.Config) camera.Camera {
	cam := cameras.NewPlugin(w, cfg).New(cameras.Isometrically, camera.Config{})
	cam.SetViewport(128, 64)
	cam.MoveTo(0, 0)
	return cam
}

// LevelBoard is a 4x4 board of level grass over w.
func LevelBoard(w *world.Plugin) (*board.Plugin, grid.Grid) {
	grid := grid.DefaultGrids{}.Square(4, 4, 32)
	b := board.NewPlugin(grid, &cell.MultipleOccupancy{}, w)
	b.Write().Board().SetAll(cell.Kind{Cost: 1, Allows: cell.Land})
	return b, grid
}

// IsometricIsland is a world with a level board in relief, seen isometrically.
func IsometricIsland() (*world.Plugin, *board.Plugin, grid.Grid, *topography.Plugin) {
	w := NewWorld(0)
	b, grid := LevelBoard(w)
	p := topography.NewPlugin(w, b, nil, topography.Config{})
	return w, b, grid, p
}

// Hosts keeps the hosts a plugin installs its rules' moments on.
func (c *InstallCtx) Hosts(h ...host.Host)     { c.hosts = append(c.hosts, h...) }
func (c *InstallCtx) Draws(s ...render.Source) { c.drawn = append(c.drawn, s...) }

// Sources are what the plugins installed draw, in the order their Installs handed them over.
func (c *InstallCtx) Sources() []render.Source { return c.drawn }

// Drawn is what p draws, as its Install hands it over.
func Drawn(t testing.TB, p plugin.Plugin) []render.Source {
	t.Helper()
	ctx := NewInstallCtx()
	if err := p.Install(ctx); err != nil {
		t.Fatal(err)
	}
	return ctx.drawn
}
func (c *InstallCtx) Looks(func())       {}
func (*InstallCtx) Pool() *parallel.Pool { return nil }

// Deliver hands rules to the hosts of their moments, as the engine does once a Stage's Init
// returns.
func (c *InstallCtx) Deliver(rules ...act.Rule) error { return ceremony.Deliver(c.hosts, rules...) }
