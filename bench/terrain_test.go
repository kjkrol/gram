package bench_test

import (
	"math"
	"math/rand/v2"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/entity/kind/comp"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/unit"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/vision"
	"github.com/kjkrol/gram/plugins/world"
)

// The terrain scene is an 80x80 board of 16-unit cells, a quarter of it rough: forest veiling sight
// by 0.6 and rocks, solid and opaque, cell by cell in turn; 200 walkers bounce about, each looking
// ahead with a 60° cone of 200.
const (
	terrainSide  = 80
	terrainCell  = 16
	terrainUnits = 200
)

// walker is the row every bouncing observer spawns from.
type walker struct {
	pos  world.Position
	vel  world.Velocity
	cell cell.ID
}

// roughCells is a quarter of the board's cells: one square block, or scattered at random.
func roughCells(grid grid.Grid, scattered bool) []cell.ID {
	var out []cell.ID
	if !scattered {
		for y := range uint32(terrainSide / 2) {
			for x := range uint32(terrainSide / 2) {
				c := grid.CellIndex(x+terrainSide/4, y+terrainSide/4)
				out = append(out, c)
			}
		}
		return out
	}
	rng := rand.New(rand.NewPCG(7, 11))
	for _, i := range rng.Perm(terrainSide * terrainSide)[:terrainSide*terrainSide/4] {
		c := grid.CellIndex(uint32(i%terrainSide), uint32(i/terrainSide))
		out = append(out, c)
	}
	return out
}

// benchTerrain builds the scene over the rough ground laid out as asked and runs 60 ticks.
func benchTerrain(b *testing.B, scattered bool) (*goke.ECS, *board.Board, []cell.ID) {
	b.Helper()
	const side = terrainSide * terrainCell
	ctx := newHeadless()
	w := ctx.UseWorld(world.Config{
		Space:    world.SpaceCfg{Width: side, Height: side},
		Entities: world.EntitiesCfg{MaxCount: terrainUnits, MinSize: 10, MaxSize: 10},
	})
	c := collision.NewPlugin(w)
	grid := grid.DefaultGrids{}.Square(terrainSide, terrainSide, terrainCell)
	brd := board.NewPlugin(grid, &cell.MultipleOccupancy{}, w)
	v := vision.NewPlugin(w)
	if err := ctx.Use(c); err != nil {
		b.Fatal(err)
	}
	if err := ctx.Use(brd); err != nil {
		b.Fatal(err)
	}
	if err := ctx.Use(v); err != nil {
		b.Fatal(err)
	}
	terrain := brd.Res.Logic.Board
	terrain.SetAll(cell.Kind{Name: cell.Named("grass"), Cost: 1, Allows: cell.Land})
	rough := [2]cell.Kind{
		{Name: cell.Named("forest"), Cost: 1, Allows: cell.Land, Veil: 0.6},
		{Name: cell.Named("rock"), Cost: 1, Solid: true, Veil: 1},
	}
	cells := roughCells(grid, scattered)
	for i, cell := range cells {
		terrain.Set(cell, rough[i%2])
	}

	rng := rand.New(rand.NewPCG(0x5eed, 0xc0ffee))
	kind.Define[walker](w.Kinds(), "walker", kind.Spec{
		comp.Load(func(r walker) world.Position { return r.pos }),
		comp.Load(func(r walker) world.Velocity { return r.vel }),
		comp.Load(func(r walker) unit.At { return unit.At{Cell: r.cell} }),
		comp.Const(unit.Mover{Domain: cell.Land}),
		comp.Const(collision.Collider{}),
		comp.Const(collision.Physics{Restitution: 1}),
		comp.Const(vision.Sight{Facing: geom.NewVec(1, 0), Radius: 200}), comp.Const(vision.Sighted{}),
		comp.Const(world.Eye{Angle: math.Pi / 3}),
	})
	walkers := kind.Named[walker](w.Kinds(), "walker")
	entries := make([]kind.Entry, terrainUnits)
	for i := range entries {
		at := geom.NewVec(3+rng.Float64()*(side-16), 3+rng.Float64()*(side-16))
		cell, _ := grid.CellAt(at)
		entries[i] = walkers.Entry(walker{pos: world.Position{AABB: plane.NewAABB(at, 10, 10)}, vel: randomVelocity(rng), cell: cell})
	}
	w.Seed(entries...)

	ecs := ctx.start(b, func(rc goke.RunCtx, d time.Duration) {
		w.RunPlan(rc, d)
		c.RunPlan(rc, d)
		brd.RunPlan(rc, d)
		v.RunPlan(rc, d)
		rc.Sync()
	})
	for range 60 {
		ecs.Tick(step)
	}
	return ecs, terrain, cells
}

// Benchmark_Board_Terrain is one tick of world, collision, board and vision with the rough ground
// read straight from the cells, laid out in one block or scattered; with felling, one forest
// cell a tick is cut down and the next grows back, as a woodcutter and a spell might.
func Benchmark_Board_Terrain(b *testing.B) {
	grass := cell.Kind{Name: cell.Named("grass"), Cost: 1, Allows: cell.Land}
	forest := cell.Kind{Name: cell.Named("forest"), Cost: 1, Allows: cell.Land, Veil: 0.6}
	for _, sc := range []struct {
		name            string
		scattered, fell bool
	}{{"rough=compact", false, false}, {"rough=scattered", true, false}, {"rough=scattered,felling", true, true}} {
		b.Run(sc.name, func(b *testing.B) {
			ecs, terrain, cells := benchTerrain(b, sc.scattered)
			b.ReportAllocs()
			i := 0
			for b.Loop() {
				if sc.fell {
					terrain.Set(cells[i%len(cells)&^1], grass)
					terrain.Set(cells[(i+2)%len(cells)&^1], forest)
					i += 2
				}
				ecs.Tick(step)
			}
		})
	}
}
