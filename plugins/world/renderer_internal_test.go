package world

import (
	"github.com/kjkrol/gram/plugin/host"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	icamera "github.com/kjkrol/gram/internal/camera"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/uid"
)

// flatAtlas is an AtlasSource with no sheet: enough for gathering quads without drawing.
type flatAtlas struct{}

func (flatAtlas) Atlas() *ebiten.Image                            { return nil }
func (flatAtlas) UV(render.SpriteID) (sx0, sy0, sx1, sy1 float32) { return 0, 0, 1, 1 }

// drawThrough spawns one 10x10 entity per position, lets pick say which of them the View holds
// (nil: the zero View, which sees everything), draws once and returns how many quads were drawn
// and how many entities the Drawing behaviors were run for.
func drawThrough(t *testing.T, pick func(ids []uid.UID64, v *View), at ...geom.Vec) (drawn, visited int) {
	t.Helper()
	view := &View{}
	cam := icamera.NewFromSpace(1000, 1000, 0)
	host := &host.EachHost[Drawing]{}
	if err := host.Add(Every(func(plugin.Tick, Drawing) { visited++ })); err != nil {
		t.Fatal(err)
	}
	r := newRenderer(flatAtlas{}, func(camera.Camera) *View { return view }, host, 1000, 1000)

	var base goke.Comp[Base]
	var appearance goke.Comp[Appearance]
	ecs := goke.New()
	ecs.Setup(goke.SystemFn{OnInit: func(si *goke.SysInit) {
		f := si.NewFactory(&base, &appearance)
		f.Create(len(at))
		var ids []uid.UID64
		i := 0
		for f.Next() {
			bases := base.Slice(&f.Cursor)
			for j, id := range f.Cursor.IDs {
				bases[j].Pos = Position{AABB: plane.NewAABB(at[i], 10, 10)}
				ids = append(ids, id)
				i++
			}
		}
		if pick != nil {
			pick(ids, view)
		}
		r.Init(si)
	}})

	r.DrawWorld(nil, cam)
	return r.batch.quads, visited
}

func TestRenderer_Draw_DrawsOnlyWhatTheViewContains(t *testing.T) {
	quarters := []geom.Vec{geom.NewVec(100, 100), geom.NewVec(700, 100), geom.NewVec(100, 700), geom.NewVec(700, 700)}

	firstOnly := func(ids []uid.UID64, v *View) {
		v.Culled = true
		v.In.Add(ids[0])
	}
	if drawn, visited := drawThrough(t, firstOnly, quarters...); drawn != 1 || visited != 4 {
		t.Errorf("a View holding one entity drew %d and visited %d, want 1 drawn of 4 visited", drawn, visited)
	}
	if drawn, _ := drawThrough(t, nil, quarters...); drawn != 4 {
		t.Errorf("the zero View drew %d entities, want all 4", drawn)
	}
}

// submitThrough is drawThrough through a Sink: how many quads were submitted and their depths.
func submitThrough(t *testing.T, at ...geom.Vec) (int, []float32) {
	t.Helper()
	view := &View{}
	cam := icamera.NewFromSpace(1000, 1000, 0)
	r := newRenderer(flatAtlas{}, func(camera.Camera) *View { return view }, &host.EachHost[Drawing]{}, 1000, 1000)
	var base goke.Comp[Base]
	var appearance goke.Comp[Appearance]
	var z goke.Comp[Z]
	ecs := goke.New()
	ecs.Setup(goke.SystemFn{OnInit: func(si *goke.SysInit) {
		f := si.NewFactory(&base, &appearance, &z)
		f.Create(len(at))
		i := 0
		for f.Next() {
			bases, zs := base.Slice(&f.Cursor), z.Slice(&f.Cursor)
			for j := range f.Cursor.IDs {
				bases[j].Pos = Position{AABB: plane.NewAABB(at[i], 10, 10)}
				zs[j] = Z{Altitude: float64(i)}
				i++
			}
		}
		r.Init(si)
	}})
	sorted := render.NewSorted(r)
	sorted.DrawWorld(nil, cam)
	var depths []float32
	for _, p := range at {
		depths = append(depths, cam.Depth(float32(p.X)+5, float32(p.Y)+5, 0))
	}
	return sorted.Gathered(), depths
}

func TestRenderer_Submit_HandsEveryEntityToTheSinkAtItsDepth(t *testing.T) {
	n, depths := submitThrough(t, geom.NewVec(100, 100), geom.NewVec(700, 100), geom.NewVec(100, 700))
	if n != 3 {
		t.Errorf("submitted %d quads, want 3", n)
	}
	if depths[0] != 105 || depths[2] != 705 {
		t.Errorf("top-down depths %v, want the centres' y", depths)
	}
}

func TestRenderer_Submit_StandsEntitiesUpThroughAnIsometricCamera(t *testing.T) {
	view := &View{}
	cam := icamera.NewFromSpaceWithConfig(1000, 1000, 0, camera.Config{ViewportWidth: 800, ViewportHeight: 600, Projection: camera.Isometric{Cell: 32}})
	cam.MoveTo(0, 0)
	r := newRenderer(flatAtlas{}, func(camera.Camera) *View { return view }, &host.EachHost[Drawing]{}, 1000, 1000)
	var base goke.Comp[Base]
	var appearance goke.Comp[Appearance]
	ecs := goke.New()
	ecs.Setup(goke.SystemFn{OnInit: func(si *goke.SysInit) {
		f := si.NewFactory(&base, &appearance)
		f.Create(1)
		for f.Next() {
			base.Slice(&f.Cursor)[0].Pos = Position{AABB: plane.NewAABB(geom.NewVec(100, 100), 10, 10)}
		}
		r.Init(si)
	}})
	var sink render.Sink
	r.Submit(&sink, cam)
	if sink.Len() != 1 {
		t.Fatalf("submitted %d, want the one entity", sink.Len())
	}
	// A billboard is upright: its top edge is level, unlike a projected box, which is a diamond.
	corners := sinkCorners(&sink)
	if corners[0][1] != corners[1][1] || corners[2][1]-corners[0][1] != 10 {
		t.Errorf("entity drawn at %v, want an upright 10-tall rectangle", corners)
	}
}

// sinkCorners is the first submitted quad's screen corners.
func sinkCorners(s *render.Sink) [4][2]float32 {
	var out [4][2]float32
	for i, v := range s.Vertices()[:4] {
		out[i] = [2]float32{v.DstX, v.DstY}
	}
	return out
}
