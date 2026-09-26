package board

import (
	"github.com/kjkrol/aabbworld/geom"
	"math"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/kjkrol/gram/camera"
	icamera "github.com/kjkrol/gram/internal/camera"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

type flatAtlas struct{}

func (flatAtlas) Atlas() *ebiten.Image                            { return nil }
func (flatAtlas) UV(render.SpriteID) (sx0, sy0, sx1, sy1 float32) { return 0, 0, 1, 1 }
func (flatAtlas) White() (u, v float32)                           { return 0, 0 }

// flatRenderer is a renderer of brd with the flat look.
func flatRenderer(brd *Board, state *RenderState) *Renderer {
	return newRenderer(brd, flatAtlas{}, state, func() Look { return flatLook{} }, func() world.Sun { return world.DefaultSun })
}

// compose is what r hands a frame through cam, by tier.
func compose(r *Renderer, cam camera.Camera) map[render.Tier]int {
	var f render.Frame
	f.Reset(cam)
	r.Compose(&f, cam)
	out := map[render.Tier]int{}
	f.Each(func(tier render.Tier, _ float32, _ []ebiten.Vertex) { out[tier]++ })
	return out
}

// outlinedTiles counts the tiles r hands a frame through cam that carry an outline.
func outlinedTiles(r *Renderer, cam camera.Camera) (tiles, outlined int) {
	var f render.Frame
	f.Reset(cam)
	r.Compose(&f, cam)
	f.Each(func(tier render.Tier, _ float32, v []ebiten.Vertex) {
		if tier != render.Ground {
			return
		}
		tiles++
		if v[0].Custom0 < 0 {
			outlined++
		}
	})
	return tiles, outlined
}

func TestRenderer_Compose_FlatLaysEveryVisibleCellOverItsBox(t *testing.T) {
	grid := DefaultGrids{}.Square(4, 4, 32)
	brd := NewBoard(grid, NewTerrainMap())
	brd.SetAll(CellKind{Cost: 1, Allows: Land, Height: 0})
	if got := compose(flatRenderer(brd, &RenderState{}), icamera.NewFromSpace(128, 128, 0)); len(got) != 1 || got[render.Ground] != 16 {
		t.Errorf("composed %v, want the 16 cells on the ground and nothing else", got)
	}
}

func TestRenderer_Compose_OutlinesSquareTilesWhenCellsAreLargeEnough(t *testing.T) {
	grid := DefaultGrids{}.Square(4, 4, 32)
	brd := NewBoard(grid, NewTerrainMap())
	brd.SetAll(CellKind{Cost: 1, Allows: Land})
	r := flatRenderer(brd, &RenderState{ShowGridLines: true})

	cam := icamera.NewFromSpace(128, 128, 0)
	if got := compose(r, cam); len(got) != 1 || got[render.Ground] != 16 {
		t.Errorf("composed %v, want the 16 tiles and nothing more", got)
	}
	if tiles, outlined := outlinedTiles(r, cam); outlined != tiles {
		t.Errorf("%d of %d tiles outlined, want all", outlined, tiles)
	}
	cam.SetViewport(20, 20) // the whole board on 20 pixels: a cell is 5 across
	cam.ZoomOut(32, 0, 0)
	if _, outlined := outlinedTiles(r, cam); outlined != 0 {
		t.Errorf("%d tiles outlined with cells a few pixels wide, want none", outlined)
	}
}

func TestRenderer_Compose_StrokesTheOutlinesOfHexCells(t *testing.T) {
	grid := DefaultGrids{}.Hex(3, 3, 32)
	brd := NewBoard(grid, NewTerrainMap())
	brd.SetAll(CellKind{Cost: 1, Allows: Land})
	got := compose(flatRenderer(brd, &RenderState{ShowGridLines: true}), icamera.NewFromSpace(256, 256, 0))
	if got[gridTier] == 0 || got[render.Ground] != 9 {
		t.Errorf("composed %v, want the nine hexes and the lines round them", got)
	}
}

// A Look is asked for every visible cell, and a tile tells it the heights it needs.
func TestRenderer_Compose_HandsTheLookEveryCellWithItsHeights(t *testing.T) {
	grid := DefaultGrids{}.Square(4, 4, 32)
	brd := NewBoard(grid, NewTerrainMap())
	brd.SetAll(CellKind{Cost: 1, Allows: Land})
	hill, _ := grid.CellIndex(1, 1)
	brd.Set(hill, CellKind{Cost: 1, Allows: Land, Height: 8})
	var seen int
	var hillTop, besideHill [4]float32
	look := lookFn(func(_ *render.Frame, _ camera.Camera, t *Tile) {
		seen++
		if t.ID == hill {
			hillTop, _ = t.Top()
		}
		if left, _ := grid.CellIndex(0, 1); t.ID == left {
			besideHill = t.Beside(1, 0)
		}
	})
	r := newRenderer(brd, flatAtlas{}, &RenderState{}, func() Look { return look }, func() world.Sun { return world.DefaultSun })
	compose(r, icamera.NewFromSpace(128, 128, 0))
	if seen != 16 || hillTop != [4]float32{8, 8, 8, 8} || besideHill != hillTop {
		t.Errorf("look saw %d cells, the hill's top %v and beside it %v; want 16 and the kind's 8 everywhere", seen, hillTop, besideHill)
	}
}

// lookFn adapts a function to Look.
type lookFn func(f *render.Frame, cam camera.Camera, t *Tile)

func (fn lookFn) Cell(f *render.Frame, cam camera.Camera, t *Tile) { fn(f, cam, t) }

// lightsOf composes brd from above in a world with heights under sun and gives each cell's light.
func lightsOf(brd *Board, sun world.Sun) map[CellID]render.Shade {
	brd.quasi3D = true
	out := map[CellID]render.Shade{}
	look := lookFn(func(_ *render.Frame, _ camera.Camera, t *Tile) { out[t.ID] = t.Light() })
	r := newRenderer(brd, flatAtlas{}, &RenderState{}, func() Look { return look }, func() world.Sun { return sun })
	compose(r, icamera.NewFromSpace(256, 256, 0))
	return out
}

func TestTile_LightFollowsTheSlopeOfTheGround(t *testing.T) {
	grid := DefaultGrids{}.Square(4, 4, 32)
	brd := NewBoard(grid, NewTerrainMap())
	brd.SetAll(CellKind{Cost: 1, Allows: Land})
	hill, _ := grid.CellIndex(1, 1)
	brd.SetHeights(MeanOfCells(grid, func(c CellID) float64 {
		if c == hill {
			return 16
		}
		return 0
	}))
	sun := world.Sun{Dir: [3]float32{1, 0, 1}, Strength: 0.6, Ambient: 0.3} // from the east, 45° up
	lights := lightsOf(brd, sun)
	at := func(x, y uint32) render.Shade { c, _ := grid.CellIndex(x, y); return lights[c] }

	level := sun.Light(0, 0, 1)
	if far := at(3, 3); far != render.Lit(level) {
		t.Errorf("level ground far from the hill is lit %v, want %v everywhere", far, level)
	}
	// the hill's east side falls away towards the sun, its west side rises away from it
	if east, west := at(2, 1)[0], at(0, 1)[1]; east[0] <= level[0] || west[0] >= level[0] {
		t.Errorf("the hill's sunny side is lit %v and its shady side %v, want above and below level %v", east, west, level)
	}
	// neighbouring tiles agree on the corner they share: the slope runs on without a seam
	if a, b := at(1, 1)[1], at(2, 1)[0]; a != b {
		t.Errorf("the corner the hill shares with its east neighbour is lit %v from one side and %v from the other", a, b)
	}
}

func TestTile_AFlatWorldIsDrawnAsItsSpritesAre(t *testing.T) {
	grid := DefaultGrids{}.Square(2, 2, 32)
	brd := NewBoard(grid, NewTerrainMap())
	brd.SetAll(CellKind{Cost: 1, Allows: Land})
	var got render.Shade
	look := lookFn(func(_ *render.Frame, _ camera.Camera, t *Tile) { got = t.Light() })
	r := newRenderer(brd, flatAtlas{}, &RenderState{}, func() Look { return look }, func() world.Sun { return world.DefaultSun })
	compose(r, icamera.NewFromSpace(64, 64, 0))
	if got != render.Even(1) {
		t.Errorf("a flat world's tile is lit %v, want as drawn", got)
	}
}

// A flat world with heights shows its slopes: ground facing the sun lighter than as drawn, ground
// turned away darker.
func TestTile_AFlatWorldShadesItsSlopes(t *testing.T) {
	grid := DefaultGrids{}.Square(4, 1, 32)
	brd := NewBoard(grid, NewTerrainMap())
	brd.SetAll(CellKind{Cost: 1, Allows: Land})
	brd.SetHeights(func(p geom.Vec) float64 { return 16 - math.Abs(p.X-64)/2 }) // a ridge along x = 64
	got := map[CellID]render.Shade{}
	look := lookFn(func(_ *render.Frame, _ camera.Camera, t *Tile) { got[t.ID] = t.Light() })
	r := newRenderer(brd, flatAtlas{}, &RenderState{}, func() Look { return look }, func() world.Sun { return world.DefaultSun })
	compose(r, icamera.NewFromSpace(128, 32, 0))
	east, _ := grid.CellIndex(2, 0) // falls to the east, where the default sun stands
	west, _ := grid.CellIndex(1, 0)
	if e, w := got[east][1][0], got[west][0][0]; e <= 1 || w >= 1 {
		t.Errorf("the ridge's east side is lit %v and its west side %v, want above and below 1", e, w)
	}
}

// A stream down the middle of a valley runs down it at every corner, as fast as its Flow by the
// square root of the slope; the banks falling into it turn it neither way, and still water does
// not run.
func TestTile_RunningWaterRunsDownItsSlopeAndNotIntoItsBanks(t *testing.T) {
	grid := DefaultGrids{}.Square(3, 3, 10)
	brd := NewBoard(grid, NewTerrainMap())
	brd.SetAll(CellKind{Cost: 1, Allows: Land})
	at := func(x, y uint32) CellID { c, _ := grid.CellIndex(x, y); return c }
	for y := range uint32(3) {
		brd.Set(at(1, y), CellKind{Allows: Water, Shine: 1, Flow: 10})
	}
	still, _ := grid.CellIndex(0, 1)
	brd.Set(still, CellKind{Allows: Water, Shine: 1})
	// falling 0.4 southward, the banks rising half as fast away from the stream
	brd.SetHeights(func(p geom.Vec) float64 { return 40 - 0.4*p.Y + 0.5*math.Abs(p.X-15) })
	flows, runs := map[CellID]render.Flow{}, map[CellID]bool{}
	look := lookFn(func(_ *render.Frame, _ camera.Camera, t *Tile) { flows[t.ID], runs[t.ID] = t.Flow() })
	r := newRenderer(brd, flatAtlas{}, &RenderState{}, func() Look { return look }, func() world.Sun { return world.DefaultSun })
	compose(r, icamera.NewFromSpace(30, 30, 0))

	want := float32(10 * math.Sqrt(0.4))
	for k, v := range flows[at(1, 1)] {
		if math.Abs(float64(v[0])) > 1e-5 || math.Abs(float64(v[1]-want)) > 1e-4 {
			t.Errorf("corner %d runs at %v, want (0, %v): down the valley, not into a bank", k, v, want)
		}
	}
	if runs[still] || runs[at(2, 1)] {
		t.Errorf("still water runs: %v, a bank runs: %v; want neither", runs[still], runs[at(2, 1)])
	}
}

// wayPieces draws brd from above and returns each tile's way pieces.
func wayPieces(t *testing.T, brd *Board, w, h uint32) map[CellID][]WayPiece {
	t.Helper()
	got := map[CellID][]WayPiece{}
	look := lookFn(func(_ *render.Frame, _ camera.Camera, t *Tile) { got[t.ID] = append([]WayPiece(nil), t.Way()...) })
	r := newRenderer(brd, flatAtlas{}, &RenderState{}, func() Look { return look }, func() world.Sun { return world.DefaultSun })
	compose(r, icamera.NewFromSpace(w, h, 0))
	return got
}

// A stream straight across a cell is two bands from its middle to its sides, as wide as it is at
// the middle and as the mean of it and its neighbour at the side, the water running down both.
func TestTile_AWayIsBandsFromTheMiddleToEachNeighbourItRunsOnTo(t *testing.T) {
	grid := DefaultGrids{}.Square(3, 3, 10)
	brd := NewBoard(grid, NewTerrainMap())
	brd.quasi3D = true
	brd.SetAll(CellKind{Cost: 1, Allows: Land})
	brd.SetHeights(func(p geom.Vec) float64 { return 20 - 0.5*p.X }) // falling east
	at := func(x, y uint32) CellID { c, _ := grid.CellIndex(x, y); return c }
	stream := CellKind{Allows: Land | Water, Shine: 1, Flow: 10}
	west, east := Links(1<<2), Links(1<<3)
	brd.SetWay(at(1, 1), Way{Kind: stream, Width: 4, Links: west | east})
	brd.SetWay(at(0, 1), Way{Kind: stream, Width: 8, Links: east})

	pieces := wayPieces(t, brd, 30, 30)[at(1, 1)]
	if len(pieces) != 2 {
		t.Fatalf("%d pieces, want the two bands and no square: the way runs straight", len(pieces))
	}
	want := map[float32]render.World{
		20: {{15, 17}, {20, 17}, {15, 13}, {20, 13}}, // east: its own width at the far end, no way beyond
		10: {{15, 13}, {10, 12}, {15, 17}, {10, 18}}, // west: the mean of 4 and 8 at the side
	}
	speed := float32(10 * math.Sqrt(0.5))
	for _, p := range pieces {
		if w, ok := want[p.World[1][0]]; !ok || p.World != w {
			t.Errorf("band %v, want one of %v", p.World, want)
		}
		if z := p.Z[0]; math.Abs(float64(z-(20-0.5*p.World[0][0]))) > 1e-4 {
			t.Errorf("band corner at %v stands at %v, want on the ground", p.World[0], z)
		}
		for k, f := range p.Flow {
			if math.Abs(float64(f[0]-speed)) > 1e-4 || f[1] != 0 {
				t.Errorf("band %v corner %d runs at %v, want %v eastward, down the slope", p.World[1], k, f, speed)
			}
		}
	}
	if len(wayPieces(t, brd, 30, 30)[at(2, 2)]) != 0 {
		t.Error("a cell with no way drew pieces")
	}
}

// A way running on slantwise reaches the corner the two cells share: within the cell as a band,
// the last stretch, which reaches into the cells either side, as a piece across the corner. One that
// ends in the cell gets a square at its middle.
func TestTile_AWayRunsSlantwiseToTheCornerAndEndsInASquare(t *testing.T) {
	grid := DefaultGrids{}.Square(3, 3, 10)
	brd := NewBoard(grid, NewTerrainMap())
	brd.SetAll(CellKind{Cost: 1, Allows: Land})
	c, _ := grid.CellIndex(1, 1)
	brd.SetWay(c, Way{Kind: CellKind{Allows: Land}, Width: 2, Links: 1 << 7}) // south-east
	pieces := wayPieces(t, brd, 30, 30)[c]
	if len(pieces) != 3 {
		t.Fatalf("%d pieces, want the band, the stretch across the corner and the square", len(pieces))
	}
	for _, at := range pieces[0].World {
		if at[0] < 10 || at[0] > 20 || at[1] < 10 || at[1] > 20 || pieces[0].Across {
			t.Errorf("the band within the cell reaches %v, out of it", at)
		}
	}
	across := pieces[1]
	end := [2]float32{(across.World[1][0] + across.World[3][0]) / 2, (across.World[1][1] + across.World[3][1]) / 2}
	if !across.Across || across.Corner != [2]float32{20, 20} || math.Abs(float64(end[0]-20)) > 1e-4 || math.Abs(float64(end[1]-20)) > 1e-4 {
		t.Errorf("the last stretch ends at %v, across %v at %v; want across the corner (20, 20)", end, across.Across, across.Corner)
	}
	if sq := pieces[2].World; sq != (render.World{{14, 14}, {16, 14}, {14, 16}, {16, 16}}) {
		t.Errorf("the square is %v, want 2 wide round the middle", sq)
	}
}

// wallInSun is a 6x3 board of level grass with a wall 10 tall at (3, 1), under a sun low in the
// east: its shadow falls 50 to the west.
func wallInSun(t *testing.T) (*Board, Grid, world.Sun) {
	t.Helper()
	grid := DefaultGrids{}.Square(6, 3, 32)
	brd := NewBoard(grid, NewTerrainMap())
	brd.SetAll(CellKind{Cost: 1, Allows: Land})
	wall, _ := grid.CellIndex(3, 1)
	brd.Set(wall, CellKind{Cost: 1, Allows: Land, Solid: true, Height: 10})
	return brd, grid, world.Sun{Dir: [3]float32{1, 0, 0.2}, Strength: 0.6, Ambient: 0.3}
}

func TestTile_TheTerrainCastsItsShadowAwayFromTheSun(t *testing.T) {
	brd, grid, sun := wallInSun(t)
	lights := lightsOf(brd, sun)
	at := func(x uint32) render.Shade { c, _ := grid.CellIndex(x, 1); return lights[c] }
	lit, shade := sun.Light(0, 0, 1), sun.Shaded(0, 0, 1, 0)

	// the wall's west edge is at x 96: the grass right behind it is in shadow up to 50 away
	if got := at(2); got[1] != shade || got[0] != shade {
		t.Errorf("the grass behind the wall is lit %v, want its corners 32 and 0 from the wall in shadow %v", got, shade)
	}
	if got := at(1); got[1] != shade || got[0] != lit {
		t.Errorf("the next tile is lit %v, want 32 from the wall in shadow and 64 from it in the sun", got)
	}
	if got := at(4); got[0] != lit || got[1] != lit {
		t.Errorf("the grass on the sunny side is lit %v, want %v", got, lit)
	}
}

func TestTile_AShadowGoesWithWhatCastItAndWithTheSun(t *testing.T) {
	brd, grid, sun := wallInSun(t)
	brd.quasi3D = true
	var got map[CellID]render.Shade
	look := lookFn(func(_ *render.Frame, _ camera.Camera, t *Tile) { got[t.ID] = t.Light() })
	current := sun
	r := newRenderer(brd, flatAtlas{}, &RenderState{}, func() Look { return look }, func() world.Sun { return current })
	behind, _ := grid.CellIndex(2, 1)
	frame := func() render.Shade {
		got = map[CellID]render.Shade{}
		compose(r, icamera.NewFromSpace(192, 96, 0))
		return got[behind]
	}
	if frame()[1] != sun.Shaded(0, 0, 1, 0) {
		t.Fatal("no shadow behind the wall to begin with")
	}
	noon := world.Sun{Dir: [3]float32{0, 0, 1}, Strength: 0.6, Ambient: 0.3}
	current = noon
	if l := frame()[1]; l != noon.Light(0, 0, 1) {
		t.Errorf("under a sun overhead the grass behind the wall is lit %v, want %v: no shadow", l, noon.Light(0, 0, 1))
	}
	current = sun
	wall, _ := grid.CellIndex(3, 1)
	brd.Set(wall, CellKind{Cost: 1, Allows: Land})
	if l := frame()[1]; l != sun.Light(0, 0, 1) {
		t.Errorf("with the wall knocked down the grass is lit %v, want the full sun %v", l, sun.Light(0, 0, 1))
	}
	r.shadows = false
	brd.Set(wall, CellKind{Cost: 1, Allows: Land, Solid: true, Height: 10})
	if l := frame()[1]; l != sun.Light(0, 0, 1) {
		t.Errorf("with shadows off the grass behind the wall is lit %v, want the full sun", l)
	}
}

func TestTile_AShinyCellShinesAsMuchAsTheSunReachesIt(t *testing.T) {
	grid := DefaultGrids{}.Square(4, 4, 32)
	brd := NewBoard(grid, NewTerrainMap())
	brd.SetAll(CellKind{Cost: 1, Allows: Land})
	sea, _ := grid.CellIndex(1, 1)
	grass, _ := grid.CellIndex(3, 3)
	brd.Set(sea, CellKind{Cost: 1, Allows: Water, Shine: 0.8})
	type shining struct {
		shine float32
		lit   [4]float32
	}
	shines := func(sun world.Sun, quasi3D bool) map[CellID]shining {
		brd.quasi3D = quasi3D
		out := map[CellID]shining{}
		look := lookFn(func(_ *render.Frame, _ camera.Camera, t *Tile) {
			if s, lit, ok := t.Shine(); ok {
				out[t.ID] = shining{s, lit}
			}
		})
		r := newRenderer(brd, flatAtlas{}, &RenderState{}, func() Look { return look }, func() world.Sun { return sun })
		compose(r, icamera.NewFromSpace(256, 256, 0))
		return out
	}
	day := world.Sun{Dir: [3]float32{0, 0, 1}, Strength: 0.6, Ambient: 0.3}
	got := shines(day, true)
	if got[sea] != (shining{0.8, [4]float32{1, 1, 1, 1}}) {
		t.Errorf("the sea in the full sun shines %v, want its kind's 0.8, all the sun at every corner", got[sea])
	}
	if _, ok := got[grass]; ok {
		t.Errorf("grass shines %v, want nothing", got[grass])
	}
	if got := shines(world.Sun{Dir: [3]float32{0, 0, -1}, Ambient: 0.1}, true); got[sea] != (shining{0.8, [4]float32{}}) {
		t.Errorf("at night the sea shines %v, want its shine and none of the sun: it reflects the night sky, foams", got[sea])
	}
	if got := shines(day, false); len(got) > 0 {
		t.Errorf("in a flat world %d cells shine, want none: it is drawn as its sprites are", len(got))
	}
}

func TestTile_TheShoreLiesTheWayOfTheNearestCellThatDoesNotShine(t *testing.T) {
	grid := DefaultGrids{}.Square(10, 4, 32)
	brd := NewBoard(grid, NewTerrainMap())
	brd.SetAll(CellKind{Cost: 1, Allows: Water, Shine: 1})
	for y := range uint32(4) {
		land, _ := grid.CellIndex(0, y)
		brd.Set(land, CellKind{Cost: 1, Allows: Land}) // a coast along x = 32, the sea east of it
	}
	brd.quasi3D = true
	shores := map[CellID]render.Shore{}
	look := lookFn(func(_ *render.Frame, _ camera.Camera, t *Tile) { shores[t.ID] = t.Shore() })
	sun := world.Sun{Dir: [3]float32{0, 0, 1}, Strength: 0.6}
	r := newRenderer(brd, flatAtlas{}, &RenderState{}, func() Look { return look }, func() world.Sun { return sun })
	compose(r, icamera.NewFromSpace(320, 128, 0))
	at := func(x uint32) render.Shore { c, _ := grid.CellIndex(x, 1); return shores[c] }

	if c := at(1)[0]; c.Dist != 0 || c.Near != 1 || abs32(c.X+1) > 1e-4 || abs32(c.Y) > 1e-4 {
		t.Errorf("on the coast a corner sees the shore %+v, want it right there to the west", c)
	}
	if c := at(2)[1]; abs32(c.Dist-64) > 1e-4 || abs32(c.Near-1.0/3) > 1e-4 || abs32(c.X+1) > 1e-4 {
		t.Errorf("two cells out a corner sees the shore %+v, want it 64 to the west, a third near", c)
	}
	if c := at(6)[0]; c.Near != 0 || c.X != 0 || c.Y != 0 {
		t.Errorf("out at sea a corner sees the shore %+v, want open water", c)
	}
}

func TestFlatLook_LaysTheWeatherOnTheGroundAndSwaysWhatSways(t *testing.T) {
	grid := DefaultGrids{}.Square(4, 4, 32)
	brd := NewBoard(grid, NewTerrainMap())
	brd.SetAll(CellKind{Cost: 1, Allows: Land})
	brd.quasi3D = true
	r := flatRenderer(brd, &RenderState{})
	cam := icamera.NewFromSpace(128, 128, 0)
	if got := compose(r, cam)[render.Ground]; got != 16 {
		t.Fatalf("under a clear sky %d pieces, want the 16 tiles alone", got)
	}
	air := world.Weather{Clouds: 0.5}
	r.weather = func() world.Weather { return air }
	if got := compose(r, cam)[render.Ground]; got != 32 {
		t.Errorf("under clouds %d pieces, want each tile and the weather over it", got)
	}

	tree, _ := grid.CellIndex(1, 1)
	brd.Set(tree, CellKind{Cost: 1, Allows: Land, Height: 8, Sway: 1})
	at := func() float32 { // where the tree's top is drawn
		var f render.Frame
		f.Reset(cam)
		r.Compose(&f, cam)
		x := float32(-1)
		f.Each(func(_ render.Tier, _ float32, v []ebiten.Vertex) {
			if x < 0 && v[0].DstY > 28 && v[0].DstY < 36 && v[0].DstX > 28 && v[0].DstX < 44 { // cell (1, 1)
				x = v[0].DstX
			}
		})
		return x
	}
	calm := at()
	air.Wind = [2]float32{40, 0}
	if blown := at(); blown <= calm {
		t.Errorf("in an east wind the tree's top is drawn at x %v, in the calm at %v; want it leaning east", blown, calm)
	}
}
