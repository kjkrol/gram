package pathfind

import (
	"math"
	"math/rand/v2"
	"testing"

	"github.com/kjkrol/aabbworld/geom"
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

// the terrain rungs' board: 128x128 cells of 16, a quarter of them walls, from and to clear
const (
	terrainSide = 128
	terrainCell = 16
)

func layWalls(brd *board.BoardRead, brdW *board.BoardWrite, from, to cell.ID) {
	rng := rand.New(rand.NewPCG(3, 5))
	brdW.SetAll(cell.Kind{Pace: 1, Allows: cell.Land})
	brd.EachCell(func(c cell.ID) {
		if c != from && c != to && rng.IntN(4) == 0 {
			brdW.Set(c, cell.Kind{Pace: 1, Solid: true})
		}
	})
}

// the same board over rolling hills: the mobility asked for every step, the heights read as a
// relief reads them, by ordinal
func terrainHills(grid grid.Grid) rolling {
	hills := rolling{grid: grid, heights: make([]float64, grid.CellCount())}
	grid.EachCell(func(c cell.ID) {
		x, y, _ := grid.Coords(c)
		o, _ := grid.Ordinal(c)
		hills.heights[o] = 6 * (1 + math.Sin(float64(x)/7)*math.Cos(float64(y)/9))
	})
	return hills
}

// one route corner to corner over a 128x128 board with a quarter of its cells walls
func BenchmarkPathFinder_Terrain(b *testing.B) {
	const side, size = terrainSide, terrainCell
	grid := grid.DefaultGrids{}.Square(side, side, size)
	at := func(x, y uint32) cell.ID { c := grid.At(x, y); return c }
	from, to := at(0, 0), at(side-1, side-1)
	lay := func(brd *board.BoardRead, brdW *board.BoardWrite) { layWalls(brd, brdW, from, to) }
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
	b.Run("relief", func(b *testing.B) {
		brd, brdW := board.NewBoard(grid)
		lay(brd, brdW)
		run(b, brd, terrainHills(grid))
	})
	// by levels, as the navigation plans and a hierarchy climbs
	b.Run("levels", func(b *testing.B) {
		brd, brdW := board.NewBoard(grid)
		lay(brd, brdW)
		pf := New(grid, brd, nil, &cell.MultipleOccupancy{}).WithLevels(flat{brd})
		var path Path
		if _, ok := pf.Find(1, cell.Land, Body{}, from, to, Either, path.Steps[:]); !ok {
			b.Fatal("no route across the board")
		}
		for b.Loop() {
			pf.Find(1, cell.Land, Body{}, from, to, Either, path.Steps[:])
		}
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

// one field filled over the terrain rungs' board from its far corner to a seed in the near one,
// by levels as the navigation fills them, the front settled half as far again past the seed
func BenchmarkPathFinder_Field(b *testing.B) {
	grid := grid.DefaultGrids{}.Square(terrainSide, terrainSide, terrainCell)
	from, to := grid.At(0, 0), grid.At(terrainSide-1, terrainSide-1)
	run := func(b *testing.B, terrain cell.Terrain, pacing Pacing) {
		pf := New(grid, terrain, pacing, &cell.MultipleOccupancy{}).WithLevels(flat{terrain})
		var field Field
		seeds := []cell.ID{from}
		if pf.FillField(&field, to, Either, cell.Land, Body{}, seeds, 1.5) != 1 {
			b.Fatal("the seed is not reached across the board")
		}
		for b.Loop() {
			pf.FillField(&field, to, Either, cell.Land, Body{}, seeds, 1.5)
		}
	}
	b.Run("map", func(b *testing.B) {
		brd, brdW := board.NewBoard(grid)
		layWalls(brd, brdW, from, to)
		run(b, brd, nil)
	})
	b.Run("relief", func(b *testing.B) {
		brd, brdW := board.NewBoard(grid)
		layWalls(brd, brdW, from, to)
		run(b, brd, terrainHills(grid))
	})
}

// four bodies the terrain rungs' board takes alike, as hierarchies of four classes serve them
var fourBodies = []Body{{}, {Size: geom.NewVec(4, 4)}, {Size: geom.NewVec(8, 8)}, {Weight: 80}}

// the hierarchy of the terrain rungs' board, its clusters 16 cells a side: built whole, of one
// body, of four bodies as one hierarchy of four classes or four of one class each; the route
// corner to corner climbed a chunk at a time, of one class or the last of four; a cell inside a
// cluster turned wall or back and its clusters built again
func BenchmarkPathFinder_Hierarchy(b *testing.B) {
	g := grid.DefaultGrids{}.Square(terrainSide, terrainSide, terrainCell)
	from, to := g.At(0, 0), g.At(terrainSide-1, terrainSide-1)
	boards := []struct {
		name   string
		pacing Pacing
	}{{"map", nil}, {"relief", terrainHills(g)}}
	laid := func(pacing Pacing) (*Finder, *board.BoardRead, *board.BoardWrite) {
		brd, brdW := board.NewBoard(g)
		layWalls(brd, brdW, from, to)
		return New(g, brd, pacing, &cell.MultipleOccupancy{}).WithLevels(flat{brd}), brd, brdW
	}
	whole := func(f *Finder, h *Hierarchy) { f.Build(h, math.MaxInt) }
	b.Run("build", func(b *testing.B) {
		for _, bd := range boards {
			b.Run(bd.name, func(b *testing.B) {
				f, _, _ := laid(bd.pacing)
				for b.Loop() {
					whole(f, f.NewHierarchy(land, 16, unsized))
				}
				b.ReportMetric(float64(f.Builds())/float64(b.N), "clusters/op")
			})
		}
		for _, bd := range boards {
			b.Run("classes=4/"+bd.name, func(b *testing.B) {
				f, _, _ := laid(bd.pacing)
				for b.Loop() {
					whole(f, f.NewHierarchy(land, 16, fourBodies))
				}
				b.ReportMetric(float64(f.Builds())/float64(b.N), "clusters/op")
			})
			b.Run("solvers=4/"+bd.name, func(b *testing.B) {
				f, _, _ := laid(bd.pacing)
				for b.Loop() {
					for _, body := range fourBodies {
						whole(f, f.NewHierarchy(land, 16, []Body{body}))
					}
				}
				b.ReportMetric(float64(f.Builds())/float64(b.N), "clusters/op")
			})
		}
	})
	b.Run("find", func(b *testing.B) {
		for _, classes := range [][]Body{unsized, fourBodies} {
			for _, bd := range boards {
				name := bd.name
				if len(classes) > 1 {
					name = "classes=4/" + name
				}
				b.Run(name, func(b *testing.B) {
					f, _, _ := laid(bd.pacing)
					h := f.NewHierarchy(land, 16, classes)
					whole(f, h)
					body := classes[len(classes)-1]
					var path Path
					if _, ok := f.Climb(h, 1, cell.Land, body, from, to, Either,
						path.Steps[:]); !ok {
						b.Fatal("no climb across the board")
					}
					expanded := f.Expansions()
					for b.Loop() {
						f.Climb(h, 1, cell.Land, body, from, to, Either, path.Steps[:])
					}
					b.ReportMetric(float64(f.Expansions()-expanded)/float64(b.N),
						"expansions/op")
				})
			}
		}
	})
	b.Run("rebuild", func(b *testing.B) {
		for _, bd := range boards {
			b.Run(bd.name, func(b *testing.B) {
				f, brd, brdW := laid(bd.pacing)
				h := f.NewHierarchy(land, 16, unsized)
				whole(f, h)
				c := g.At(40, 40)
				builds := f.Builds()
				for b.Loop() {
					since := brd.Passage()
					if brd.Kind(c).Admits(cell.Land) {
						brdW.Set(c, cell.Kind{Pace: 1, Solid: true})
					} else {
						brdW.Set(c, cell.Kind{Pace: 1, Allows: cell.Land})
					}
					f.Stale(h, since)
					whole(f, h)
				}
				b.ReportMetric(float64(f.Builds()-builds)/float64(b.N), "clusters/op")
			})
		}
	})
}
