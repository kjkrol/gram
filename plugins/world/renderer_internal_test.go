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
func (flatAtlas) White() (u, v float32)                           { return 0, 0 }

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
	r := newRenderer(flatAtlas{}, func(camera.Camera) *View { return view }, host, func() Look { return &flatLook{worldW: 1000, worldH: 1000} })

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

	var f render.Frame
	f.Reset(cam)
	r.Compose(&f, cam)
	return f.Len(), visited
}

func TestRenderer_Compose_DrawsOnlyWhatTheViewContains(t *testing.T) {
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

// shadowsOf composes one 10x10 entity at (100, 100) standing as z says under sun, from above over
// level ground, and gives the shadow pieces: their tier and middle.
func shadowsOf(t *testing.T, z Z, sun Sun) (tiers []render.Tier, middles []geom.Vec) {
	t.Helper()
	view := &View{}
	cam := icamera.NewFromSpace(1000, 1000, 0)
	r := newRenderer(flatAtlas{}, func(camera.Camera) *View { return view }, &host.EachHost[Drawing]{}, func() Look { return &flatLook{worldW: 1000, worldH: 1000} })
	r.sun, r.ground = func() Sun { return sun }, func() Ground { return nil }
	var base goke.Comp[Base]
	var appearance goke.Comp[Appearance]
	var zc goke.Comp[Z]
	ecs := goke.New()
	ecs.Setup(goke.SystemFn{OnInit: func(si *goke.SysInit) {
		f := si.NewFactory(&base, &appearance, &zc)
		f.Create(1)
		for f.Next() {
			base.Slice(&f.Cursor)[0].Pos = Position{AABB: plane.NewAABB(geom.NewVec(100, 100), 10, 10)}
			zc.Slice(&f.Cursor)[0] = z
		}
		r.Init(si)
	}})
	var f render.Frame
	f.Reset(cam)
	r.Compose(&f, cam)
	f.Each(func(tier render.Tier, _ float32, v []ebiten.Vertex) {
		if tier != shadowTier {
			return
		}
		tiers = append(tiers, tier)
		var x, y float32
		for _, p := range v {
			x, y = x+p.DstX/4, y+p.DstY/4
		}
		middles = append(middles, geom.NewVec(float64(x), float64(y)))
	})
	return tiers, middles
}

func TestRenderer_LaysAShadowAwayFromTheSunPushedOffByHowHighItStands(t *testing.T) {
	west := Sun{Dir: [3]float32{-1, 0, 1}, Strength: 0.6, Ambient: 0.3} // 45° up in the west
	_, walker := shadowsOf(t, Z{Altitude: 0, Height: 4}, west)
	_, hawk := shadowsOf(t, Z{Altitude: 40, Height: 4}, west)
	if len(walker) != 1 || len(hawk) != 1 {
		t.Fatalf("shadows %v and %v, want one each", walker, hawk)
	}
	// the walker's centre is at (105, 105): its shadow reaches east by its height, 4 at 45°
	if walker[0].X != 107 || walker[0].Y != 105 {
		t.Errorf("the walker's shadow lies round %v, want (107, 105): east of it by half its height", walker[0])
	}
	if hawk[0].X != 147 || hawk[0].Y != 105 {
		t.Errorf("the hawk's shadow lies round %v, want (147, 105): pushed 40 further east", hawk[0])
	}
	if tiers, _ := shadowsOf(t, Z{Height: 4}, Sun{Dir: [3]float32{-1, 0, -0.1}}); len(tiers) != 0 {
		t.Errorf("%d shadows with the sun down, want none", len(tiers))
	}
}
