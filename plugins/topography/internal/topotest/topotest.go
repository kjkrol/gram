package topotest

import (
	"testing"
	"time"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/internal/hosts"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/topography"
	"github.com/kjkrol/gram/plugins/topography/relief"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/rule"
)

// Hill is the kind of a QuasiWorld's hill.
var Hill = cell.Kind{Name: cell.Named("hill"), Cost: 1, Allows: cell.Land | cell.Air}

// RaiseHills puts the hill cells at 12 and the rest at 0, each corner at the mean of its cells.
func RaiseHills(r topography.Relief, grid grid.Grid, hills ...cell.ID) {
	r.SetHeights(relief.MeanOfCells(grid, func(c cell.ID) float64 {
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
	hosts   []plugin.Host // of the rules of the moments the plugins installed catch
	ecs     *goke.ECS
	pending []func() []goke.System
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

// QuasiWorld is a world with heights with a board over a 4x4 square grid in relief, a hill at (2,1), and
// the units define makes; collide adds the collision plugin.
type QuasiWorld struct {
	ECS   *goke.ECS
	World *world.Plugin
	Board *board.Plugin
	Topo  *topography.Plugin
	Grid  grid.Grid
}

// NewQuasiWorld is a QuasiWorld with the units define makes; collide adds the collision plugin.
func NewQuasiWorld(t *testing.T, collide bool, define func(units *board.Units[Recruit], grid grid.Grid) []kind.Entry) *QuasiWorld {
	t.Helper()
	qw := &QuasiWorld{Grid: grid.DefaultGrids{}.Square(4, 4, 32)}
	qw.World = world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 128, Height: 128},
		Entities: world.EntitiesCfg{MaxCount: 8, MinSize: 20, MaxSize: 20},
		Heights:  true,
	})
	var c *collision.Plugin
	if collide {
		c = collision.NewPlugin(qw.World)
	}
	qw.Board = board.NewPlugin(qw.Grid, &cell.MultipleOccupancy{}, qw.World)
	qw.Topo = topography.NewPlugin(qw.World, qw.Board, topography.Config{Cell: 32})
	qw.Board.Res.Logic.Board.SetAll(cell.Kind{Name: cell.Named("grass"), Cost: 1, Allows: cell.Land | cell.Air})
	hillCell := qw.Grid.CellIndex(2, 1)
	qw.Board.Res.Logic.Board.Set(hillCell, Hill)
	RaiseHills(qw.Topo.Relief(), qw.Grid, hillCell)
	units := board.NewUnits[Recruit](qw.Board, board.Shape{Size: 20, Height: 2}, func(r Recruit) geom.Vec { return qw.Grid.CellCenter(r.Start) })
	entries := define(units, qw.Grid)

	ctx := NewInstallCtx()
	if err := qw.World.Install(ctx); err != nil {
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
	qw.World.Seed(entries...)
	if err := qw.World.Populate(); err != nil {
		t.Fatal(err)
	}
	ctx.ecs.Setup(ctx.Systems()...)
	ctx.ecs.SetPlan(func(rc goke.RunCtx, d time.Duration) {
		qw.World.RunPlan(rc, d)
		if c != nil {
			c.RunPlan(rc, d)
		}
		qw.Board.RunPlan(rc, d)
		qw.Topo.RunPlan(rc, d)
		rc.Sync()
		qw.World.Clock().Replay(rc, d)
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

// NewWorld is a world with heights 256 x 256, its edges as given, a camera 128 x 64.
func NewWorld(edges aabbworld.Edges) *world.Plugin {
	return world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 256, Height: 256, Edges: edges},
		Entities: world.EntitiesCfg{MaxCount: 4, MinSize: 1, MaxSize: 20},
		Camera:   camera.Config{ViewportWidth: 128, ViewportHeight: 64},
		Heights:  true,
	})
}

// LevelBoard is a 4x4 board of level grass over w.
func LevelBoard(w *world.Plugin) (*board.Plugin, grid.Grid) {
	grid := grid.DefaultGrids{}.Square(4, 4, 32)
	b := board.NewPlugin(grid, &cell.MultipleOccupancy{}, w)
	b.Res.Logic.Board.SetAll(cell.Kind{Cost: 1, Allows: cell.Land})
	return b, grid
}

// IsometricIsland is a world with a level board in relief, seen isometrically.
func IsometricIsland() (*world.Plugin, *board.Plugin, grid.Grid, *topography.Plugin) {
	w := NewWorld(0)
	b, grid := LevelBoard(w)
	p := topography.NewPlugin(w, b, topography.Config{Cell: 32, HeightUnit: 1, Isometric: true})
	return w, b, grid, p
}

// Hosts keeps the hosts of the rules of the moments a plugin catches.
func (c *InstallCtx) Hosts(h ...plugin.Host) { c.hosts = append(c.hosts, h...) }

// Deliver hands rules — a role's, each of its own — to the hosts of their moments, as the engine
// does with the roles played once a Stage's Init returns.
func (c *InstallCtx) Deliver(rules ...rule.Rule) error { return hosts.Deliver(c.hosts, rules...) }
