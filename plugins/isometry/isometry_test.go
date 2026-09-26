package isometry_test

import (
	"math"
	"strings"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/isometry"
	"github.com/kjkrol/gram/plugins/landscape"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

// sheet is an AtlasSource of one sprite with no image behind it.
type sheet struct{}

func (sheet) Atlas() *ebiten.Image                            { return nil }
func (sheet) UV(render.SpriteID) (sx0, sy0, sx1, sy1 float32) { return 0, 0, 8, 8 }
func (sheet) White() (u, v float32)                           { return 9, 9 }

func newWorld(edges aabbworld.Edges) *world.Plugin {
	return world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 128, Height: 128, Edges: edges},
		Entities: world.EntitiesCfg{MaxCount: 4, MinSize: 1, MaxSize: 20},
		Camera:   camera.Config{ViewportWidth: 400, ViewportHeight: 300},
		Quasi3D:  true,
	})
}

func TestPlugin_MakesTheWorldsCamerasIsometric(t *testing.T) {
	w := newWorld(0)
	if w.Camera().Projection().Sorts() {
		t.Fatal("a world is isometric before the plugin")
	}
	isometry.NewPlugin(w, isometry.Config{Cell: 32, HeightUnit: 1})
	for name, cam := range map[string]camera.Camera{"the world's": w.Camera(), "a new one": w.NewCamera()} {
		if !cam.Projection().Sorts() || cam.Projection().Wraps() {
			t.Errorf("%s camera draws through %T, want the plugin's isometric projection", name, cam.Projection())
		}
		if sx, sy := cam.Project(0, 0, 10); sx == 0 && sy == 0 {
			t.Errorf("%s camera draws a height 10 at the screen's origin: heights are not lifted", name)
		}
		if vw, vh := cam.Viewport(); vw != 400 || vh != 300 {
			t.Errorf("%s camera is %v x %v, want the world's 400 x 300", name, vw, vh)
		}
	}
	if w.ViewFor(w.Camera()) != w.View() {
		t.Error("the world's View does not follow its new camera")
	}
}

func TestPlugin_RefusesAWrappingWorld(t *testing.T) {
	defer func() {
		if r, _ := recover().(string); !strings.Contains(r, "wraps") {
			t.Errorf("NewPlugin over a torus: %q, want a refusal", r)
		}
	}()
	isometry.NewPlugin(newWorld(aabbworld.Torus), isometry.Config{Cell: 32})
}

func TestBillboards_StandEntitiesUprightAtTheDepthOfTheirCentre(t *testing.T) {
	w := newWorld(0)
	isometry.NewPlugin(w, isometry.Config{Cell: 32, HeightUnit: 1})
	cam := w.Camera()
	cam.MoveTo(0, 0)
	look := w.Look()
	box := plane.NewAABB(geom.NewVec(40, 40), 10, 10)

	var f render.Frame
	f.Reset(cam)
	look.Sprite(&f, cam, box, 6, sheet{}, 0, render.Light{1, 1, 1}, 0)
	f.Each(func(tier render.Tier, depth float32, v []ebiten.Vertex) {
		if tier != render.Objects || depth != cam.Depth(45, 45, 6) {
			t.Errorf("entity on tier %d at depth %v, want Objects at its centre's %v", tier, depth, cam.Depth(45, 45, 6))
		}
		if v[0].DstY != v[1].DstY || v[2].DstY-v[0].DstY != 10 {
			t.Errorf("entity drawn at %v %v %v, want an upright 10-tall rectangle", v[0], v[1], v[2])
		}
	})
	drawn := look.Drawn(cam, box.AABB, 6)
	bx, by := cam.Project(45, 45, 6)
	if drawn[2][1] != by || (drawn[2][0]+drawn[3][0])/2 != bx {
		t.Errorf("drawn at %v, want the billboard standing on (%v, %v)", drawn, bx, by)
	}
	if fp := look.Footprint(cam, box.AABB, 6, nil); len(fp) != 1 || fp[0][0][1] == fp[0][1][1] {
		t.Errorf("footprint %v, want one diamond on the ground", fp)
	}
}

// hillBoard is a 4x4 board, the cell (1, 1) a hill 10 high sloping into its neighbours.
func hillBoard(t *testing.T, w *world.Plugin, grid board.Grid) *board.Plugin {
	t.Helper()
	b := board.NewPlugin(grid, &board.MultipleOccupancy{}, w)
	brd := b.Res.Logic.Board
	brd.SetAll(board.CellKind{Cost: 1, Allows: board.Land})
	hill, _ := grid.CellIndex(1, 1)
	brd.SetHeights(board.MeanOfCells(grid, func(c board.CellID) float64 {
		if c == hill {
			return 10
		}
		return 0
	}))
	return b
}

// compose composes b's renderer through cam: pieces by tier, and how many are outlined.
func compose(b *board.Plugin, cam camera.Camera) (tiers map[render.Tier]int, outlined int) {
	var f render.Frame
	f.Reset(cam)
	b.Renderer().(render.Source).Compose(&f, cam)
	tiers = map[render.Tier]int{}
	f.Each(func(tier render.Tier, _ float32, v []ebiten.Vertex) {
		tiers[tier]++
		if v[0].Custom0 < 0 {
			outlined++
		}
	})
	return tiers, outlined
}

func TestBlocks_StandTheCellsWithFacesWhereTheyRiseOverTheirNeighbours(t *testing.T) {
	w := newWorld(0)
	p := isometry.NewPlugin(w, isometry.Config{Cell: 32, HeightUnit: 1})
	grid := board.DefaultGrids{}.Square(4, 4, 32)
	b := hillBoard(t, w, grid)
	p.WithBoard(b)
	b.WithRenderer(sheet{})
	b.Res.Render.ShowGridLines = false
	cam := w.Camera()
	cam.MoveTo(0, 0)

	if got, _ := compose(b, cam); got[render.Ground] != 16 {
		t.Errorf("composed %v, want the 16 cells: the hill slopes into its neighbours, no faces", got)
	}
	wall, _ := grid.CellIndex(2, 2)
	b.Res.Logic.Board.Set(wall, board.CellKind{Cost: 1, Allows: board.Land, Solid: true, Height: 8})
	if got, _ := compose(b, cam); got[render.Ground] != 18 {
		t.Errorf("composed %v, want two more for the wall's faces down to the ground", got)
	}
	b.Res.Render.ShowGridLines = true
	if _, outlined := compose(b, cam); outlined != 16 {
		t.Errorf("%d tops outlined with the grid on, want the 16 tops and no face", outlined)
	}
}

// Under a landscape the tops are lit by the sun as the ground slopes.
func TestBlocks_LightTheTopsFromTheUpperLeft(t *testing.T) {
	w := newWorld(0)
	p := isometry.NewPlugin(w, isometry.Config{Cell: 32, HeightUnit: 1})
	grid := board.DefaultGrids{}.Square(4, 4, 32)
	b := hillBoard(t, w, grid)
	p.WithBoard(b)
	landscape.NewPlugin(b, w)
	b.WithRenderer(sheet{})
	cam := w.Camera()
	cam.MoveTo(0, 0)
	var f render.Frame
	f.Reset(cam)
	b.Renderer().(render.Source).Compose(&f, cam)
	shades := map[float32]bool{}
	f.Each(func(_ render.Tier, _ float32, v []ebiten.Vertex) { shades[v[0].ColorR] = true })
	if len(shades) < 3 {
		t.Errorf("tops shaded %v, want level ground and slopes towards and away from the light apart", shades)
	}
	for s := range shades {
		if s <= 0 || s > 1 || math.IsNaN(float64(s)) {
			t.Errorf("a shade of %v", s)
		}
	}
}

func TestBlocks_TurnedShowTheFacesTurnedTowardsTheViewer(t *testing.T) {
	w := newWorld(0)
	p := isometry.NewPlugin(w, isometry.Config{Cell: 32, HeightUnit: 1})
	grid := board.DefaultGrids{}.Square(4, 4, 32)
	b := board.NewPlugin(grid, &board.MultipleOccupancy{}, w)
	b.Res.Logic.Board.SetAll(board.CellKind{Cost: 1, Allows: board.Land})
	p.WithBoard(b)
	b.WithRenderer(sheet{})
	b.Res.Render.ShowGridLines = false
	wall, _ := grid.CellIndex(2, 2)
	b.Res.Logic.Board.Set(wall, board.CellKind{Cost: 1, Allows: board.Land, Solid: true, Height: 8})
	cam := w.Camera()
	// the face along x = x (the wall's west side at 64, its east at 96) standing from 8 down to 0
	faceAt := func(x float32) bool {
		at := func(v ebiten.Vertex, y, z float32) bool {
			sx, sy := cam.Project(x, y, z)
			return near(v.DstX, sx) && near(v.DstY, sy)
		}
		var f render.Frame
		f.Reset(cam)
		b.Renderer().(render.Source).Compose(&f, cam)
		found := false
		f.Each(func(_ render.Tier, _ float32, v []ebiten.Vertex) {
			for _, ends := range [2][2]float32{{64, 96}, {96, 64}} {
				if at(v[0], ends[0], 8) && at(v[1], ends[1], 8) && at(v[2], ends[0], 0) && at(v[3], ends[1], 0) {
					found = true
				}
			}
		})
		return found
	}
	cam.CenterOn(80, 80, 0)
	if !faceAt(96) || faceAt(64) {
		t.Errorf("unturned: east face %v, west face %v; want the east one, towards the viewer", faceAt(96), faceAt(64))
	}
	isometry.TurnCamera(cam, math.Pi)
	if faceAt(96) || !faceAt(64) {
		t.Errorf("turned half round: east face %v, west face %v; want the west one, towards the viewer now", faceAt(96), faceAt(64))
	}
}

func near(a, b float32) bool { return math.Abs(float64(a-b)) < 1e-3 }

func TestBlocksAndBillboards_LeanWithTheWindWhatSways(t *testing.T) {
	w := newWorld(0)
	p := isometry.NewPlugin(w, isometry.Config{Cell: 32, HeightUnit: 1})
	grid := board.DefaultGrids{}.Square(4, 4, 32)
	b := board.NewPlugin(grid, &board.MultipleOccupancy{}, w)
	b.Res.Logic.Board.SetAll(board.CellKind{Cost: 1, Allows: board.Land})
	p.WithBoard(b)
	b.WithRenderer(sheet{})
	b.Res.Render.ShowGridLines = false
	tree, _ := grid.CellIndex(1, 1)
	b.Res.Logic.Board.Set(tree, board.CellKind{Cost: 1, Allows: board.Land, Height: 8, Sway: 1})
	cam := w.Camera()
	cam.CenterOn(48, 48, 0)
	top := func() (float32, float32) { // where the tree top's corner (32, 32) is drawn
		tx, ty := cam.Project(32, 32, 8)
		var f render.Frame
		f.Reset(cam)
		b.Renderer().(render.Source).Compose(&f, cam)
		bx, by := float32(math.NaN()), float32(math.NaN())
		f.Each(func(_ render.Tier, _ float32, v []ebiten.Vertex) {
			if math.Abs(float64(v[0].DstX-tx)) < 6 && math.Abs(float64(v[0].DstY-ty)) < 6 {
				bx, by = v[0].DstX, v[0].DstY
			}
		})
		return bx, by
	}
	cx, cy := top()
	tx, ty := cam.Project(32, 32, 8)
	if !near(cx, tx) || !near(cy, ty) {
		t.Fatalf("in the calm the tree's top is drawn at (%v, %v), want it upright at (%v, %v)", cx, cy, tx, ty)
	}
	w.SetWeather(world.Weather{Wind: [2]float32{40, 0}})
	if wx, wy := top(); near(wx, cx) && near(wy, cy) {
		t.Error("in a wind of 40 the tree's top stands where it did in the calm")
	}

	look := w.Look()
	box := plane.NewAABB(geom.NewVec(40, 40), 10, 10)
	edge := func(sway float32) float32 {
		var f render.Frame
		f.Reset(cam)
		f.Weather(render.Weather{Wind: [2]float32{40, 0}})
		look.Sprite(&f, cam, box, 0, sheet{}, 0, render.Light{1, 1, 1}, sway)
		var x float32
		f.Each(func(_ render.Tier, _ float32, v []ebiten.Vertex) { x = v[0].DstX - v[2].DstX })
		return x
	}
	if still, swaying := edge(0), edge(1); still != 0 || swaying == 0 {
		t.Errorf("a billboard's top edge stands %v off its foot unswaying, %v swaying; want upright and leaning", still, swaying)
	}
}
