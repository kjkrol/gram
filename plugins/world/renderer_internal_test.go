package world

import (
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	icamera "github.com/kjkrol/gram/internal/camera"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugin/host"
	"github.com/kjkrol/gram/plugins/world/view"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/uid"
)

// flatAtlas is an AtlasSource with no sheet: enough for gathering quads without drawing.
type flatAtlas struct{}

func (flatAtlas) Atlas() *render.Image                            { return nil }
func (flatAtlas) UV(render.SpriteID) (sx0, sy0, sx1, sy1 float32) { return 0, 0, 1, 1 }
func (flatAtlas) White() (u, v float32)                           { return 0, 0 }

// drawThrough spawns one 10x10 entity per position, lets pick say which of them the View holds
// (nil: the zero View, which sees everything), draws once and returns how many quads were drawn
// and how many entities the Drawing behaviors were run for.
func drawThrough(t *testing.T, pick func(ids []uid.UID64, v *view.View), at ...geom.Vec) (drawn, visited int) {
	t.Helper()
	v := &view.View{}
	cam := icamera.NewFromSpace(1000, 1000, 0)
	drawers := &host.EachHost[Drawing]{}
	if err := drawers.Add(host.Every(func(plugin.Tick, Drawing) { visited++ })); err != nil {
		t.Fatal(err)
	}
	look := &flatLook{worldW: 1000, worldH: 1000}
	r := newRenderer(flatAtlas{}, func(camera.Camera) *view.View { return v }, drawers, func() Look { return look })

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
			pick(ids, v)
		}
		r.Init(si)
	}})

	var f render.Frame
	f.Reset(cam)
	r.Compose(&f, cam)
	return f.Len() + look.sprites.Len(), visited // laid on the frame, or gathered for the GPU
}

func TestRenderer_Compose_DrawsOnlyWhatTheViewContains(t *testing.T) {
	quarters := []geom.Vec{geom.NewVec(100, 100), geom.NewVec(700, 100), geom.NewVec(100, 700), geom.NewVec(700, 700)}

	firstOnly := func(ids []uid.UID64, v *view.View) {
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
