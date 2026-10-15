package navigation

import (
	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/ceremony/tag"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugins/board/unit"
	"github.com/kjkrol/gram/plugins/selection"
	"image/color"
	"math"
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/ground"
	"github.com/kjkrol/gram/plugins/cameras"
	"github.com/kjkrol/gram/plugins/topography"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

// the middles of the piece's sides
func ends(v []render.Vertex) (x0, y0, x1, y1 float32) {
	return (v[0].DstX + v[2].DstX) / 2, (v[0].DstY + v[2].DstY) / 2, (v[1].DstX + v[3].DstX) / 2, (v[1].DstY + v[3].DstY) / 2
}

func near32(a, b float32) bool { return math.Abs(float64(a-b)) < 1e-2 }

type sampled struct {
	at   func(p geom.Vec) float64
	step float64
}

func (s sampled) At(p geom.Vec) float64 { return s.at(p) }
func (s sampled) Step() float64         { return s.step }

func TestPathRenderer_LaysTheRouteOnTheGroundInPiecesAtTheirDepth(t *testing.T) {
	grid := grid.DefaultGrids{}.Square(4, 4, 32)
	brd := board.NewBoard(grid)
	brd.SetAll(cell.Kind{Cost: 1, Allows: cell.Land})
	land := sampled{step: 32, at: ground.MeanOfCells(grid, func(c cell.ID) float64 { // a ridge down the right half
		if x, _, _ := grid.Coords(c); x >= 2 {
			return 10
		}
		return 0
	})}
	cam := isoCamera(128, 128, camera.Config{})
	a, b := geom.NewVec(48, 48), geom.NewVec(112, 48)
	drawn := func(r *pathRenderer) (pieces [][]render.Vertex, lies []render.At) {
		var f render.Frame
		f.Reset(cam)
		r.Compose(&f, cam) // reads the ground; nothing to draw without a space
		r.line(a, b)
		f.Each(func(at render.At, v []render.Vertex) {
			if at == render.OnTop {
				t.Error("a route piece is a mark, want it lying on the ground")
			}
			pieces, lies = append(pieces, append([]render.Vertex(nil), v...)), append(lies, at)
		})
		return
	}
	r := newPathRenderer(brd, RouteStyle{}, 0).WithHeights(func() ground.Heights { return land })
	pieces, lies := drawn(r)
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
	if at, want := lies[len(lies)-1], float32(land.At(mid))+routeLift; at.Z() != want {
		t.Errorf("the last piece lies at %v, want just over the ground under its middle, %v",
			at.Z(), want)
	}
	flat := newPathRenderer(brd, RouteStyle{}, 0)
	pieces, _ = drawn(flat)
	_, _, ex, ey = ends(pieces[0])
	if bx, by := cam.Project(112, 48, 0); len(pieces) != 1 || !near32(ex, bx) || !near32(ey, by) {
		t.Errorf("without heights %d pieces ending at (%v, %v), want one on the ground at 0, (%v, %v)", len(pieces), ex, ey, bx, by)
	}
}

func TestPathRenderer_OutlinesTheGoalWhereTheEntityWillStand(t *testing.T) {
	grid := grid.DefaultGrids{}.Square(4, 4, 32)
	cam := isoCamera(128, 128, camera.Config{})
	r := newPathRenderer(grid, RouteStyle{}, 0)
	c := grid.At(2, 1)
	for _, tc := range []struct {
		spot   geom.Vec
		centre geom.Vec
	}{{geom.NewVec(70, 40), geom.NewVec(70, 40)}, {geom.Vec{}, grid.CellCenter(c)}} {
		var f render.Frame
		f.Reset(cam)
		r.Compose(&f, cam)
		r.goal(geom.NewVec(10, 10), c, tc.spot, 0)
		var starts [][2]float32
		f.Each(func(at render.At, v []render.Vertex) {
			if at != render.OnTop {
				t.Errorf("a goal's line lies at %v, want a mark on top", at)
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

func TestPathRenderer_OutlinesNoStepAside(t *testing.T) {
	grid := grid.DefaultGrids{}.Square(4, 4, 32)
	cam := isoCamera(128, 128, camera.Config{})
	r := newPathRenderer(grid, RouteStyle{}, 0)
	a := grid.At(2, 1)
	b := grid.At(3, 3)
	for _, tc := range []struct {
		givingWay bool
		lines     int
	}{{false, 8}, {true, 4}} {
		o := MoveOrder{Target: a, GivingWay: tc.givingWay}
		o.Enqueue(Goal{Cell: b})
		var f render.Frame
		f.Reset(cam)
		r.Compose(&f, cam)
		r.goals(geom.NewVec(10, 10), cell.Land, &o)
		n := 0
		f.Each(func(render.At, []render.Vertex) { n++ })
		if n != tc.lines {
			t.Errorf("giving way %v: %d lines, want %d: 4 an outline", tc.givingWay, n, tc.lines)
		}
	}
}

func isoCamera(width, height uint32, cfg camera.Config) camera.Camera {
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: width, Height: height},
		Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 100},
		Heights:  true,
	})
	b := board.NewPlugin(grid.DefaultGrids{}.Square(width/32, height/32, 32), &cell.MultipleOccupancy{}, w)
	p := topography.NewPlugin(w, b, topography.Config{Cell: 32, HeightUnit: 1})
	return cameras.NewPlugin(w).New(p.Views(topography.Isometrically), cfg)
}

func TestPathRenderer_OutlinesEachKindsGoalInItsRouteStyle(t *testing.T) {
	g := grid.DefaultGrids{}.Square(4, 4, 32)
	space, err := aabbworld.NewSpace(aabbworld.Config{Width: 128, Height: 128, BucketSize: 32})
	if err != nil {
		t.Fatal(err)
	}
	selected := tag.Tag[selection.Family](1)
	var styles render.Styles
	azure := color.RGBA{B: 255, A: 255}
	styles.Set(7, RouteStyle{Line: azure, Goal: azure, Width: 1})
	r := newPathRenderer(g, RouteStyle{}, selected).
		WithStyles(func() *render.Styles { return &styles })
	r.BindSpace(space)
	var base goke.Comp[world.Base]
	var at goke.Comp[unit.At]
	var order goke.Comp[MoveOrder]
	var marks goke.Comp[tag.Tags[selection.Family]]
	var app goke.Comp[render.Appearance]
	ecs := goke.New()
	ecs.Setup(
		goke.SystemFn{OnInit: func(si *goke.SysInit) {
			f := si.NewFactory(&base, &at, &order, &marks, &app)
			f.Create(2)
			for f.Next() {
				for i := range f.Cursor.IDs {
					c := g.At(uint32(i), 1)
					box := plane.NewAABB(g.CellCenter(c), 10, 10)
					base.Slice(&f.Cursor)[i].Pos = world.Position{AABB: box}
					at.Slice(&f.Cursor)[i] = unit.At{Cell: c}
					order.Slice(&f.Cursor)[i] = MoveOrder{Target: g.At(uint32(i), 3)}
					marks.Slice(&f.Cursor)[i] = tag.Tags[selection.Family](0).With(selected)
					app.Slice(&f.Cursor)[i] = render.Appearance{SpriteID: render.SpriteID(6 + i)}
				}
			}
		}},
		goke.SystemFn{OnInit: r.Init},
	)
	cam := isoCamera(128, 128, camera.Config{})
	var f render.Frame
	f.Reset(cam)
	r.Compose(&f, cam)
	blue, orange := 0, 0
	f.Each(func(_ render.At, v []render.Vertex) {
		switch {
		case v[0].ColorB == 1:
			blue++
		case v[0].ColorR == 1:
			orange++
		}
	})
	if blue != 4 || orange != 4 {
		t.Errorf("%d blue and %d orange sides, want 4 each: the styled kind's goal in its "+
			"RouteStyle, the other's in the default", blue, orange)
	}
}
