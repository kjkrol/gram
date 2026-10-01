package navigation

import (
	"math/rand/v2"
	"testing"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/world"
)

// BenchmarkPathFinder_Terrain finds one route corner to corner over a 128x128 board with a quarter
// of its cells walls, reading the terrain from a plain TerrainMap and from the board's cell entities.
func BenchmarkPathFinder_Terrain(b *testing.B) {
	const side, size = 128, 16
	grid := board.DefaultGrids{}.Square(side, side, size)
	at := func(x, y uint32) cell.ID { c, _ := grid.CellIndex(x, y); return c }
	from, to := at(0, 0), at(side-1, side-1)
	lay := func(brd *board.Board) {
		rng := rand.New(rand.NewPCG(3, 5))
		brd.SetAll(cell.Kind{Cost: 1, Allows: cell.Land})
		brd.EachCell(func(c cell.ID) {
			if c != from && c != to && rng.IntN(4) == 0 {
				brd.Set(c, cell.Kind{Cost: 1, Solid: true})
			}
		})
	}
	run := func(b *testing.B, terrain board.Terrain) {
		pf := newPathFinder(grid, terrain, nil, &board.MultipleOccupancy{})
		if _, ok := pf.findPath(1, cell.Land, from, to); !ok {
			b.Fatal("no route across the board")
		}
		for b.Loop() {
			pf.findPath(1, cell.Land, from, to)
		}
	}

	b.Run("map", func(b *testing.B) {
		terrain := board.NewTerrainMap()
		lay(board.NewBoard(grid, terrain))
		run(b, terrain)
	})
	b.Run("cells", func(b *testing.B) {
		w := world.NewPlugin(world.Config{
			Space:    world.SpaceCfg{Width: side * size, Height: side * size},
			Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: size},
		})
		brd := board.NewPlugin(grid, &board.MultipleOccupancy{}, w)
		lay(brd.Res.Logic.Board)
		ctx := &stubInstallCtx{ecs: goke.New()}
		if err := w.Install(ctx); err != nil {
			b.Fatal(err)
		}
		if err := brd.Install(ctx); err != nil {
			b.Fatal(err)
		}
		var systems []goke.System
		for _, produce := range ctx.pending {
			systems = append(systems, produce()...)
		}
		ctx.ecs.Setup(systems...)
		if _, ok := brd.CellEntity(from); !ok {
			b.Fatal("the board has no cell entities")
		}
		run(b, brd.Res.Logic.Board)
	})
}
