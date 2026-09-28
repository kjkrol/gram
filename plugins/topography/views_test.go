package topography_test

import (
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/plugins/players"
	"github.com/kjkrol/gram/plugins/selection"
	"math"
	"reflect"
	"strings"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/atmosphere/air"
	"github.com/kjkrol/gram/plugins/atmosphere/sky"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/topography"
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
		Space:    world.SpaceCfg{Width: 256, Height: 256, Edges: edges},
		Entities: world.EntitiesCfg{MaxCount: 4, MinSize: 1, MaxSize: 20},
		Camera:   camera.Config{ViewportWidth: 128, ViewportHeight: 64},
		Heights:  true,
	})
}

// levelBoard is a 4x4 board of level grass over w.
func levelBoard(w *world.Plugin) (*board.Plugin, board.Grid) {
	grid := board.DefaultGrids{}.Square(4, 4, 32)
	b := board.NewPlugin(grid, &board.MultipleOccupancy{}, w)
	b.Res.Logic.Board.SetAll(board.CellKind{Cost: 1, Allows: board.Land})
	return b, grid
}

// isometricIsland is a world with a level board in relief, seen isometrically.
func isometricIsland() (*world.Plugin, *board.Plugin, board.Grid, *topography.Plugin) {
	w := newWorld(0)
	b, grid := levelBoard(w)
	p := topography.NewPlugin(w, b, topography.Config{Cell: 32, HeightUnit: 1, Isometric: true})
	return w, b, grid, p
}

// raiseHill puts the cell (1, 1) 10 high, sloping into its neighbours.
func raiseHill(p *topography.Plugin, grid board.Grid) {
	hill, _ := grid.CellIndex(1, 1)
	p.Relief().SetHeights(topography.MeanOfCells(grid, func(c board.CellID) float64 {
		if c == hill {
			return 10
		}
		return 0
	}))
}

func TestPlugin_MakesTheWorldsCamerasIsometric(t *testing.T) {
	w := newWorld(0)
	if w.Camera().Projection().Sorts() {
		t.Fatal("a world is isometric before the plugin")
	}
	b, _ := levelBoard(w)
	topography.NewPlugin(w, b, topography.Config{Cell: 32, HeightUnit: 1, Isometric: true})
	for name, cam := range map[string]camera.Camera{"the world's": w.Camera(), "a new one": w.NewCamera()} {
		if !cam.Projection().Sorts() || cam.Projection().Wraps() {
			t.Errorf("%s camera draws through %T, want the plugin's isometric projection", name, cam.Projection())
		}
		if sx, sy := cam.Project(0, 0, 10); sx == 0 && sy == 0 {
			t.Errorf("%s camera draws a height 10 at the screen's origin: heights are not lifted", name)
		}
		if vw, vh := cam.Viewport(); vw != 128 || vh != 64 {
			t.Errorf("%s camera is %v x %v, want the world's 128 x 64", name, vw, vh)
		}
	}
	if w.ViewFor(w.Camera()) != w.View() {
		t.Error("the world's View does not follow its new camera")
	}
}

// From above the camera is flat — heights not drawn, screen x and y the world's — and View turns
// it isometric and back, keeping the ground point in the middle of the screen and a cell as wide.
func TestPlugin_ViewSwitchesBetweenAboveAndIsometric(t *testing.T) {
	// a world wide enough that no zoom floor holds the view back
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 2048, Height: 2048},
		Entities: world.EntitiesCfg{MaxCount: 4, MinSize: 1, MaxSize: 20},
		Camera:   camera.Config{ViewportWidth: 128, ViewportHeight: 64},
		Heights:  true,
	})
	b := board.NewPlugin(board.DefaultGrids{}.Square(64, 64, 32), &board.MultipleOccupancy{}, w)
	p := topography.NewPlugin(w, b, topography.Config{Cell: 32, TileW: 64, HeightUnit: 1})
	cam := w.Camera()
	if cam.Projection().Sorts() {
		t.Fatal("a game not begun Isometric looks isometrically")
	}
	x0, y0 := cam.Project(32, 32, 0)
	x1, y1 := cam.Project(32, 32, 10)
	if x0 != x1 || y0 != y1 {
		t.Errorf("from above a height is drawn %v, %v away from the ground, want not at all", x1-x0, y1-y0)
	}
	cam.CenterOn(1024, 1024, 0)
	cellBefore, _ := cam.Project(1024, 1024, 0)
	cellBefore2, _ := cam.Project(1056, 1024, 0)
	topography.SwitchView(cam)
	if !cam.Projection().Sorts() {
		t.Fatal("after View the camera does not look isometrically")
	}
	vw, vh := cam.Viewport()
	if x, y := cam.Unproject(vw/2, vh/2, 0); math.Abs(float64(x-1024)) > 0.5 || math.Abs(float64(y-1024)) > 0.5 {
		t.Errorf("after View the middle of the screen is over (%v, %v), want (1024, 1024) still", x, y)
	}
	// a cell's diamond is as wide as the cell was: TileW of zoom to Cell of zoom
	left, _ := cam.Project(1024, 1056, 0)
	right, _ := cam.Project(1056, 1024, 0)
	if width := right - left; math.Abs(float64(width-(cellBefore2-cellBefore))) > 0.5 {
		t.Errorf("after View a cell spans %v pixels, want the %v it did from above", width, cellBefore2-cellBefore)
	}
	topography.SwitchView(cam)
	if cam.Projection().Sorts() {
		t.Error("View again does not look from above")
	}
	_ = p
}

func TestPlugin_RefusesAWrappingWorld(t *testing.T) {
	defer func() {
		if r, _ := recover().(string); !strings.Contains(r, "wraps") {
			t.Errorf("NewPlugin over a torus: %q, want a refusal", r)
		}
	}()
	w := newWorld(aabbworld.Torus)
	b, _ := levelBoard(w)
	topography.NewPlugin(w, b, topography.Config{Cell: 32})
}

func TestBillboards_StandEntitiesUprightAtTheDepthOfTheirCentre(t *testing.T) {
	w, _, _, _ := isometricIsland()
	cam := w.Camera()
	cam.CenterOn(64, 64, 0)
	look := w.Look()
	box := plane.NewAABB(geom.NewVec(40, 40), 10, 10)

	var f render.Frame
	f.Reset(cam)
	look.Sprite(&f, cam, box, world.Z{Altitude: 6}, sheet{}, 0, render.Light{1, 1, 1}, 0)
	f.Each(func(tier render.Tier, depth float32, v []ebiten.Vertex) {
		if tier == sky.ShadowTier {
			return // its shadow on the ground, laid before it
		}
		if tier != render.Objects || depth != cam.Depth(45, 45, 6) {
			t.Errorf("entity on tier %d at depth %v, want Objects at its centre's %v", tier, depth, cam.Depth(45, 45, 6))
		}
		if v[0].DstY != v[1].DstY || v[2].DstY-v[0].DstY != 10 {
			t.Errorf("entity drawn at %v %v %v, want an upright 10-tall rectangle", v[0], v[1], v[2])
		}
	})
	drawn := look.Drawn(cam, box.AABB, world.Z{Altitude: 6})
	bx, by := cam.Project(45, 45, 6)
	if drawn[2][1] != by || (drawn[2][0]+drawn[3][0])/2 != bx {
		t.Errorf("drawn at %v, want the billboard standing on (%v, %v)", drawn, bx, by)
	}
	if fp := look.Footprint(cam, box.AABB, 6, nil); len(fp) != 1 || fp[0][0][1] == fp[0][1][1] {
		t.Errorf("footprint %v, want one diamond on the ground", fp)
	}
	// an entity 30 tall stands as a billboard 30 tall on its 10-wide box: as tall as its Z says
	f.Reset(cam)
	look.Sprite(&f, cam, box, world.Z{Altitude: 6, Height: 30}, sheet{}, 0, render.Light{1, 1, 1}, 0)
	f.Each(func(tier render.Tier, _ float32, v []ebiten.Vertex) {
		if tier == sky.ShadowTier {
			return
		}
		if v[2].DstY-v[0].DstY != 30 || v[1].DstX-v[0].DstX != 10 {
			t.Errorf("an entity 30 tall is drawn %v wide and %v tall, want 10 by 30", v[1].DstX-v[0].DstX, v[2].DstY-v[0].DstY)
		}
	})
	// from above the world's own flat look: the sprite over its box
	topography.SwitchView(cam)
	f.Reset(cam)
	look.Sprite(&f, cam, box, world.Z{Altitude: 6}, sheet{}, 0, render.Light{1, 1, 1}, 0)
	f.Each(func(tier render.Tier, _ float32, v []ebiten.Vertex) {
		if tier == sky.ShadowTier {
			return
		}
		if x0, y0 := cam.Project(40, 40, 0); v[0].DstX != x0 || v[0].DstY != y0 {
			t.Errorf("from above the entity is drawn at (%v, %v), want over its box's corner (%v, %v)", v[0].DstX, v[0].DstY, x0, y0)
		}
	})
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
	w, b, grid, p := isometricIsland()
	raiseHill(p, grid)
	b.WithRenderer(sheet{})
	b.Res.Render.ShowGridLines = false
	cam := w.Camera()
	cam.CenterOn(64, 64, 0)

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
	// from above the same cells lie flat, no faces: the screen zoomed out over the whole board
	topography.SwitchView(cam)
	cam.ZoomOut(2, 64, 64)
	cam.CenterOn(64, 64, 0)
	if got, _ := compose(b, cam); got[render.Ground] != 16 {
		t.Errorf("from above composed %v, want the 16 cells alone", got)
	}
}

// The tops are lit by the sun as the ground slopes.
func TestBlocks_LightTheTopsFromTheUpperLeft(t *testing.T) {
	w, b, grid, p := isometricIsland()
	raiseHill(p, grid)
	b.WithRenderer(sheet{})
	cam := w.Camera()
	cam.CenterOn(64, 64, 0)
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
	w, b, grid, _ := isometricIsland()
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
	topography.TurnCamera(cam, math.Pi)
	if faceAt(96) || !faceAt(64) {
		t.Errorf("turned half round: east face %v, west face %v; want the west one, towards the viewer now", faceAt(96), faceAt(64))
	}
}

func near(a, b float32) bool { return math.Abs(float64(a-b)) < 1e-3 }

// fixedSky is a topography.Atmosphere of a fixed sun and weather.
type fixedSky struct {
	sun sky.Sun
	air air.Weather
}

func (s fixedSky) Sun() sky.Sun     { return s.sun }
func (s fixedSky) Air() air.Weather { return s.air }

func TestBlocksAndBillboards_LeanWithTheWindWhatSways(t *testing.T) {
	w, b, grid, p := isometricIsland()
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
	p.WithAtmosphere(fixedSky{sun: sky.DefaultSun, air: air.Weather{Wind: [2]float32{40, 0}}})
	if wx, wy := top(); near(wx, cx) && near(wy, cy) {
		t.Error("in a wind of 40 the tree's top stands where it did in the calm")
	}

	look := w.Look()
	box := plane.NewAABB(geom.NewVec(40, 40), 10, 10)
	edge := func(sway float32) float32 {
		var f render.Frame
		f.Reset(cam)
		look.Sprite(&f, cam, box, world.Z{}, sheet{}, 0, render.Light{1, 1, 1}, sway)
		var x float32
		f.Each(func(_ render.Tier, _ float32, v []ebiten.Vertex) { x = v[0].DstX - v[2].DstX })
		return x
	}
	if still, swaying := edge(0), edge(1); still != 0 || swaying == 0 {
		t.Errorf("a billboard's top edge stands %v off its foot unswaying, %v swaying; want upright and leaning", still, swaying)
	}
}

// Given the perspective, V rides in the selected unit: riding, W, S, A and D drive it, the mouse
// looks round, V and Tab leave, and Q, E, R and F are not bound; free, V rides in, Q and E turn
// and R and F tilt. Without the perspective V follows the unit from behind and the arrows drive it.
func TestDefaultBindings_FirstPersonKeysHoldRidingOnly(t *testing.T) {
	w := newWorld(0)
	b, _ := levelBoard(w)
	p := topography.NewPlugin(w, b, topography.Config{Cell: 32, HeightUnit: 1, Isometric: true, Perspective: true}).WithSelection(selection.NewPlugin(w))
	holding := func(mode camera.Mode) map[string]control.Binding {
		out := map[string]control.Binding{}
		for _, bd := range p.DefaultBindings() {
			if bd.Holds(mode) {
				out[players.Written(bd.Trigger)] = bd
			}
		}
		return out
	}
	riding, free := holding(camera.FirstPerson), holding(camera.Free)
	for key, want := range map[string]topography.Drive{"W (held)": {Ahead: 1}, "S (held)": {Ahead: -1}, "A (held)": {Turn: -1}, "D (held)": {Turn: 1}} {
		bd, ok := riding[key]
		if !ok {
			t.Errorf("riding, %s is not bound", key)
			continue
		}
		cmd, _ := bd.Build(control.Context{})
		if d, ok := cmd.(topography.Drive); !ok || d.Ahead != want.Ahead || d.Turn != want.Turn {
			t.Errorf("riding, %s issues %+v, want %+v", key, cmd, want)
		}
		if _, ok := free[key]; ok {
			t.Errorf("free, %s is the topography's too: the camera's WASD would be taken", key)
		}
	}
	for _, key := range []string{"Q (held)", "E (held)", "R (held)", "F (held)"} {
		if _, ok := riding[key]; ok {
			t.Errorf("riding, %s is bound, want nothing: the mouse looks round", key)
		}
		if _, ok := free[key]; !ok {
			t.Errorf("free, %s is not bound", key)
		}
	}
	if bd, ok := riding["mouse"]; !ok {
		t.Error("riding, the mouse does not look round")
	} else if cmd, _ := bd.Build(control.Context{Delta: geom.NewVec(3, -2)}); cmd != (topography.Look{Dx: 3, Dy: -2}) {
		t.Errorf("riding, a mouse move of (3, -2) issues %+v, want Look{Dx: 3, Dy: -2}", cmd)
	}
	if _, ok := free["mouse"]; ok {
		t.Error("free, the mouse looks round")
	}
	for _, key := range []string{"V", "Tab"} {
		if _, ok := riding[key]; !ok {
			t.Errorf("riding, %s is not bound", key)
		}
		if _, ok := free[key]; !ok {
			t.Errorf("free, %s is not bound", key)
		}
	}
	if cmd, _ := riding["V"].Build(control.Context{}); reflect.TypeOf(cmd) != reflect.TypeFor[topography.LookOut]() {
		t.Errorf("riding, V issues %T, want LookOut: it leaves", cmd)
	}
	if cmd, _ := free["V"].Build(control.Context{}); reflect.TypeOf(cmd) != reflect.TypeFor[topography.LookOut]() {
		t.Errorf("free, V issues %T, want LookOut: it rides in", cmd)
	}

	w2 := newWorld(0)
	b2, _ := levelBoard(w2)
	flat := topography.NewPlugin(w2, b2, topography.Config{Cell: 32, HeightUnit: 1, Isometric: true}).WithSelection(selection.NewPlugin(w2))
	keys := map[string]control.Binding{}
	for _, bd := range flat.DefaultBindings() {
		keys[players.Written(bd.Trigger)] = bd
	}
	if cmd, _ := keys["V"].Build(control.Context{}); reflect.TypeOf(cmd) != reflect.TypeFor[topography.Follow]() {
		t.Errorf("without the perspective V issues %T, want Follow", cmd)
	}
	if _, ok := keys["Up (held)"]; !ok {
		t.Error("without the perspective the arrows do not drive the followed unit")
	}
	if _, ok := keys["W (held)"]; ok {
		t.Error("without the perspective the topography binds W")
	}
}

// A click on the top of a kind standing on its cell lands on that cell, not on the ground behind
// it, isometrically and in perspective: the camera picks the top as it is drawn.
func TestPlugin_ThePickLandsOnTheTopOfAKindStandingOnItsCell(t *testing.T) {
	for _, perspective := range []bool{false, true} {
		w := newWorld(0)
		b, grid := levelBoard(w)
		topography.NewPlugin(w, b, topography.Config{Cell: 32, HeightUnit: 1, Isometric: true, Perspective: perspective})
		wall, _ := grid.CellIndex(2, 1)
		b.Res.Logic.Board.Set(wall, board.CellKind{Name: board.Named("wall"), Cost: 1, Height: 30})
		cam := w.Camera()
		picker, ok := cam.(camera.Picker)
		if !ok {
			t.Fatal("the topography's camera is no camera.Picker")
		}
		if perspective && !cam.(interface{ LookFrom(x, y, z float32) bool }).LookFrom(200, 200, 120) {
			t.Fatal("the perspective view is not reached")
		}
		centre := grid.CellCenter(wall)
		cam.CenterOn(centre.X, centre.Y, 30)
		sx, sy := cam.Project(float32(centre.X), float32(centre.Y), 30)
		x, y, ok := picker.Pick(sx, sy)
		if c, in := grid.CellAt(geom.NewVec(float64(x), float64(y))); !ok || !in || c != wall {
			t.Errorf("perspective %v: the top of the wall picked (%v, %v) %v, cell %v, want the wall's cell %v", perspective, x, y, ok, c, wall)
		}
	}
}
