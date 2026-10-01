package navigation

import (
	"math"
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/ground"
	"github.com/kjkrol/gram/plugins/topography"
	"github.com/kjkrol/gram/plugins/topography/relief"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

// ends is where a Line's piece starts and ends on screen: the middles of its sides.
func ends(v []render.Vertex) (x0, y0, x1, y1 float32) {
	return (v[0].DstX + v[2].DstX) / 2, (v[0].DstY + v[2].DstY) / 2, (v[1].DstX + v[3].DstX) / 2, (v[1].DstY + v[3].DstY) / 2
}

func near32(a, b float32) bool { return math.Abs(float64(a-b)) < 1e-2 }

// sampled is ground of a height function, sampled step apart: a ground.Heights of the test's own.
type sampled struct {
	at   func(p geom.Vec) float64
	step float64
}

func (s sampled) At(p geom.Vec) float64 { return s.at(p) }
func (s sampled) Step() float64         { return s.step }

// A route lies on the ground as the board's heights have it, in pieces of the ground's step, each
// on the Overlays tier at the depth of the ground under its middle; without heights, one piece on
// the ground at 0.
func TestPathRenderer_LaysTheRouteOnTheGroundInPiecesAtTheirDepth(t *testing.T) {
	grid := grid.DefaultGrids{}.Square(4, 4, 32)
	brd := board.NewBoard(grid)
	brd.SetAll(cell.Kind{Cost: 1, Allows: cell.Land})
	land := sampled{step: 32, at: relief.MeanOfCells(grid, func(c cell.ID) float64 { // a ridge down the right half
		if x, _, _ := grid.Coords(c); x >= 2 {
			return 10
		}
		return 0
	})}
	cam := isoCamera(128, 128, camera.Config{})
	a, b := geom.NewVec(48, 48), geom.NewVec(112, 48)
	drawn := func(r *PathRenderer) (pieces [][]render.Vertex, depths []float32) {
		var f render.Frame
		f.Reset(cam)
		r.Compose(&f, cam) // reads the ground; nothing to draw without a space
		r.line(a, b)
		f.Each(func(tier render.Tier, d float32, v []render.Vertex) {
			if tier != render.Overlays {
				t.Errorf("a route piece on tier %d, want Overlays", tier)
			}
			pieces, depths = append(pieces, append([]render.Vertex(nil), v...)), append(depths, d)
		})
		return
	}
	r := NewPathRenderer(brd, RouteStyle{}, 0).WithHeights(func() ground.Heights { return land })
	pieces, depths := drawn(r)
	want := int(math.Ceil(64 / land.Step()))
	if len(pieces) != want {
		t.Fatalf("%d pieces over 64 units with a step of %v, want %d", len(pieces), land.Step(), want)
	}
	sx, sy, _, _ := ends(pieces[0])
	if ax, ay := cam.Project(48, 48, float32(land.At(a))); !near32(sx, ax) || !near32(sy, ay) {
		t.Errorf("the route starts at (%v, %v), want (%v, %v): on the ground at its start", sx, sy, ax, ay)
	}
	last := pieces[len(pieces)-1]
	_, _, ex, ey := ends(last)
	if bx, by := cam.Project(112, 48, float32(land.At(b))); !near32(ex, bx) || !near32(ey, by) {
		t.Errorf("the route ends at (%v, %v), want (%v, %v): on the ridge at its end", ex, ey, bx, by)
	}
	mid := geom.NewVec(112-32.0/2, 48)
	if d := depths[len(depths)-1]; d != cam.Depth(float32(mid.X), float32(mid.Y), float32(land.At(mid))) {
		t.Errorf("the last piece lies at depth %v, want the ground's under its middle", d)
	}
	flat := NewPathRenderer(brd, RouteStyle{}, 0)
	pieces, _ = drawn(flat)
	_, _, ex, ey = ends(pieces[0])
	if bx, by := cam.Project(112, 48, 0); len(pieces) != 1 || !near32(ex, bx) || !near32(ey, by) {
		t.Errorf("without heights %d pieces ending at (%v, %v), want one on the ground at 0, (%v, %v)", len(pieces), ex, ey, bx, by)
	}
}

// A goal is the entity's outline where it will stand — its box round the spot, or the cell's
// centre — on the Marks tier, over everything.
func TestPathRenderer_OutlinesTheGoalWhereTheEntityWillStand(t *testing.T) {
	grid := grid.DefaultGrids{}.Square(4, 4, 32)
	cam := isoCamera(128, 128, camera.Config{})
	r := NewPathRenderer(grid, RouteStyle{}, 0)
	c, _ := grid.CellIndex(2, 1)
	for _, tc := range []struct {
		spot   geom.Vec
		centre geom.Vec
	}{{geom.NewVec(70, 40), geom.NewVec(70, 40)}, {geom.Vec{}, grid.CellCenter(c)}} {
		var f render.Frame
		f.Reset(cam)
		r.Compose(&f, cam)
		r.goal(geom.NewVec(10, 10), c, tc.spot)
		var starts [][2]float32
		f.Each(func(tier render.Tier, _ float32, v []render.Vertex) {
			if tier != render.Marks {
				t.Errorf("a goal's line on tier %d, want Marks", tier)
			}
			x, y, _, _ := ends(v)
			starts = append(starts, [2]float32{x, y})
		})
		if len(starts) != 4 {
			t.Fatalf("%d lines for a goal, want its outline's 4", len(starts))
		}
		for _, corner := range [4][2]float64{{-5, -5}, {5, -5}, {5, 5}, {-5, 5}} {
			wx, wy := cam.Project(float32(tc.centre.X+corner[0]), float32(tc.centre.Y+corner[1]), 0)
			found := false
			for _, s := range starts {
				if near32(s[0], wx) && near32(s[1], wy) {
					found = true
				}
			}
			if !found {
				t.Errorf("spot %v: no line starts at the box's corner (%v, %v); the lines start at %v", tc.spot, wx, wy, starts)
			}
		}
	}
}

// A step aside is no goal: an order to give way outlines only the goals queued after it.
func TestPathRenderer_OutlinesNoStepAside(t *testing.T) {
	grid := grid.DefaultGrids{}.Square(4, 4, 32)
	cam := isoCamera(128, 128, camera.Config{})
	r := NewPathRenderer(grid, RouteStyle{}, 0)
	a, _ := grid.CellIndex(2, 1)
	b, _ := grid.CellIndex(3, 3)
	for _, tc := range []struct {
		givingWay bool
		lines     int
	}{{false, 8}, {true, 4}} {
		o := MoveOrder{Target: a, GivingWay: tc.givingWay}
		o.Enqueue(Goal{Cell: b})
		var f render.Frame
		f.Reset(cam)
		r.Compose(&f, cam)
		r.goals(geom.NewVec(10, 10), &o)
		n := 0
		f.Each(func(render.Tier, float32, []render.Vertex) { n++ })
		if n != tc.lines {
			t.Errorf("giving way %v: %d lines, want %d: 4 an outline", tc.givingWay, n, tc.lines)
		}
	}
}

// isoCamera is a camera of a width x height world put in the isometric view.
func isoCamera(width, height uint32, cfg camera.Config) camera.Camera {
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: width, Height: height},
		Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 100},
		Camera:   cfg,
		Heights:  true,
	})
	b := board.NewPlugin(grid.DefaultGrids{}.Square(width/32, height/32, 32), &cell.MultipleOccupancy{}, w)
	topography.NewPlugin(w, b, topography.Config{Cell: 32, HeightUnit: 1, Isometric: true})
	return w.Camera()
}
