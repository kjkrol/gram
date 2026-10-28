package pathfind

import (
	"math"
	"math/rand/v2"
	"testing"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/world"
)

// rolling hills paced by the default hiking: a height a cell, the slope between two the rise
// over the cell's side
type rolling struct {
	grid    grid.Grid
	heights []float64
}

// the default hiking held as a Mobility once: converted at every call it would allocate
var hiking cell.Mobility = cell.DefaultHiking

func (r rolling) Pace(from, to cell.ID, k cell.Kind, level cell.Level, d cell.Domain) float64 {
	a, _ := r.grid.Ordinal(from)
	b, _ := r.grid.Ordinal(to)
	w, _ := r.grid.CellBounds()
	return cell.PaceOf(hiking, cell.Step{From: from, To: to, Level: level, Kind: k,
		Domain: d, Slope: (r.heights[b] - r.heights[a]) / float64(w)})
}

func (r rolling) Flat(to cell.ID, k cell.Kind, level cell.Level, d cell.Domain) float64 {
	return cell.PaceOf(hiking, cell.Step{From: to, To: to, Level: level, Kind: k, Domain: d})
}

func (rolling) Sloped() bool { return true }

// one route corner to corner over a 128x128 board with a quarter of its cells walls
func BenchmarkPathFinder_Terrain(b *testing.B) {
	const side, size = 128, 16
	grid := grid.DefaultGrids{}.Square(side, side, size)
	at := func(x, y uint32) cell.ID { c := grid.At(x, y); return c }
	from, to := at(0, 0), at(side-1, side-1)
	lay := func(brd *board.BoardRead, brdW *board.BoardWrite) {
		rng := rand.New(rand.NewPCG(3, 5))
		brdW.SetAll(cell.Kind{Pace: 1, Allows: cell.Land})
		brd.EachCell(func(c cell.ID) {
			if c != from && c != to && rng.IntN(4) == 0 {
				brdW.Set(c, cell.Kind{Pace: 1, Solid: true})
			}
		})
	}
	run := func(b *testing.B, terrain cell.Terrain, pacing Pacing) {
		pf := newPathFinder(grid, terrain, pacing, &cell.MultipleOccupancy{})
		if _, ok := pf.findPath(1, cell.Land, from, to); !ok {
			b.Fatal("no route across the board")
		}
		for b.Loop() {
			pf.findPath(1, cell.Land, from, to)
		}
	}

	b.Run("map", func(b *testing.B) {
		brd, brdW := board.NewBoard(grid) // no ECS: the board reads its seed
		lay(brd, brdW)
		run(b, brd, nil)
	})
	// the same board over rolling hills: the mobility asked for every step, the heights read as a
	// relief reads them, by ordinal
	b.Run("relief", func(b *testing.B) {
		brd, brdW := board.NewBoard(grid)
		lay(brd, brdW)
		hills := rolling{grid: grid, heights: make([]float64, grid.CellCount())}
		brd.EachCell(func(c cell.ID) {
			x, y, _ := grid.Coords(c)
			o, _ := grid.Ordinal(c)
			hills.heights[o] = 6 * (1 + math.Sin(float64(x)/7)*math.Cos(float64(y)/9))
		})
		run(b, brd, hills)
	})
	b.Run("cells", func(b *testing.B) {
		w := world.NewPlugin(world.Config{
			Space:    world.SpaceCfg{Width: side * size, Height: side * size},
			Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: size},
			Heights:  true,
		})
		brd := board.NewPlugin(grid, &cell.MultipleOccupancy{}, w)
		lay(brd.Read(0).Board(), brd.Write().Board())
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
		if _, ok := brd.Read(0).Board().CellEntity(from); !ok {
			b.Fatal("the board has no cell entities")
		}
		run(b, brd.Read(0).Board(), nil)
	})
}
