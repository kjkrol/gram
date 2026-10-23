package light_test

import (
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/plugins/cameras/cameratest"
	"github.com/kjkrol/gram/plugins/light"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

func TestLamp_LaysAShadowAwayFromItLongerTheNearerAndTheTallerTheEntity(t *testing.T) {
	lamp := light.Lamp{X: 100, Y: 100, Z: 20, Reach: 200, Color: render.Light{1, 1, 1}}
	box := geom.NewAABBAt(geom.NewVec(145, 95), 10, 10) // round (150, 100)
	// 50 east of the lamp, 4 tall under a lamp 20 up: the shadow runs east 50·4/16 past its feet
	walker, ok := lamp.ShadowOf(box, world.Z{Height: 4}, false)
	if !ok || walker.UX != 1 || walker.UY != 0 || !near(walker.X, 150+12.5/2) ||
		!near(walker.Along, 12.5/2+5) {
		t.Errorf("the walker's shadow is %+v, want it east, 12.5 long past its feet", walker)
	}
	if tower, ok := lamp.ShadowOf(box, world.Z{Height: 30}, false); !ok ||
		!near(tower.Along, 30*6/2+5) {
		t.Errorf("a tower taller than the lamp casts %+v, want a shadow as long as the sun's "+
			"longest, six times its height", tower)
	}
	if shape, _ := lamp.ShadowOf(box, world.Z{Height: 4}, true); shape.Fade != 0 ||
		!near(shape.Along, 12.5/2) || shape.Wide != 5 {
		t.Errorf("the shape cast is %+v, want it unfaded, as long as the shadow, as wide as the "+
			"box", shape)
	}
	far := geom.NewAABBAt(geom.NewVec(395, 95), 10, 10)
	if _, ok := lamp.ShadowOf(far, world.Z{Height: 4}, false); ok {
		t.Error("past the lamp's reach it casts a shadow")
	}
}

type sheet struct{}

func (sheet) Atlas() *render.Image                            { return nil }
func (sheet) UV(render.SpriteID) (sx0, sy0, sx1, sy1 float32) { return 1, 2, 3, 4 }
func (sheet) White() (u, v float32)                           { return 9, 9 }

// a level relief for the shadows to fall on
type level struct{}

func (level) Lattice() (cols, rows int, cell float32, heights []float32, ok bool) {
	return 9, 9, 128, make([]float32, 81), true
}
func (level) GroundAt(geom.Vec) float64 { return 0 }
func (level) Version() uint64           { return 0 }

// a kind 20 tall casting its shadow as cast says, or shining with src given one with a Reach
func castKind(w *world.Plugin, name string, cast light.Casting, src ...light.Source) kind.Of[spot] {
	spec := kind.Spec{
		comp.Load(func(s spot) world.Position {
			return world.Position{AABB: plane.NewAABB(geom.NewVec(s.x-2, s.y-2), 4, 4)}
		}),
		comp.Const(world.Velocity{}),
		comp.Const(world.Z{Height: 20}),
		comp.Const(light.Shadow{Cast: cast}),
	}
	for _, s := range src {
		spec = append(spec, comp.Const(s))
	}
	kind.Define[spot](w.Kinds(), name, spec)
	return kind.Named[spot](w.Kinds(), name)
}

// gathered is every cast the plugin gathers for a picture of the world through cam, Of asked of
// every entity as the world's renderer asks it
func gathered(p *light.Plugin, q *goke.Query, base goke.Comp[world.Base],
	cam camera.Camera) ([]light.Umbra, []bool) {
	p.Write().Begin(&render.Frame{}, cam)
	for q.All(); q.Next(); {
		cur := q.Cursor()
		for i := range cur.IDs {
			box := base.Slice(cur)[i].Pos.AABB
			p.Write().Of(cur, i, box, world.Z{Height: 4}, render.Appearance{SpriteID: 1})
		}
	}
	return light.Casts(p), light.Shaped(p)
}

func TestPlugin_GathersTheShadowsOfWhatItLightsAsTheirKindsAndThePolicySay(t *testing.T) {
	var base goke.Comp[world.Base]
	var q *goke.Query
	var p *light.Plugin
	joined := goke.SystemFn{OnInit: func(si *goke.SysInit) {
		qb := si.NewQueryBuilder(&base)
		light.Lighting(p).Join(qb)
		q = qb.Build()
	}}
	var torch kind.Of[spot]
	cfg := light.Config{LampShadows: 1}
	w, _, tick := litWorld(t, cfg, func(w *world.Plugin, pl *light.Plugin) {
		p = pl
		torch = castKind(w, "torch", light.NoShadow, light.Source{Strength: 1, Reach: 40})
		w.Write().Seed(castKind(w, "walker", light.Patch).Entry(spot{500, 500}),
			castKind(w, "hero", light.Silhouette).Entry(spot{520, 500}),
			castKind(w, "ghost", light.NoShadow).Entry(spot{540, 500}),
			castKind(w, "tower", light.Baked).Entry(spot{560, 500}))
	}, joined)
	w.Write().SetAtlas(sheet{})
	cam := cameratest.NewFromSpace(1000, 1000, 0)
	tick()
	if casts, _ := gathered(p, q, base, cam); len(casts) != 0 {
		t.Errorf("without a sun %d shadows are gathered, want none", len(casts))
	}
	p.WithDistant(light.Distant{Dir: [3]float32{-1, 0, 1}, Strength: 1})
	tick()
	casts, shaped := gathered(p, q, base, cam)
	if len(casts) != 2 || casts[0].UX != 1 || casts[0].Y != 500 || shaped[0] || !shaped[1] {
		t.Fatalf("the sun casts %v, shaped %v; want the walker's patch and the hero's shape, "+
			"east of them", casts, shaped)
	}
	p.WithShadowPolicy(func(declared light.Casting, distance float32) light.Casting {
		if declared == light.Silhouette {
			return light.Patch
		}
		return light.NoShadow
	})
	if casts, shaped := gathered(p, q, base, cam); len(casts) != 1 || shaped[0] {
		t.Errorf("under the policy %v are cast, shaped %v; want the hero's patch alone", casts,
			shaped)
	}
	p.WithShadowPolicy(nil)

	// a torch west of the walker, 10 up, reaching the hero too: their shadows fall east of it
	w.Write().Spawn(torch.Entry(spot{490, 500}))
	tick()
	tick()
	casts, _ = gathered(p, q, base, cam)
	if len(casts) != 4 || casts[1].UX != 1 || casts[1].Along <= casts[0].Along {
		t.Fatalf("with a torch %v are cast, want the sun's two and the torch's two, the walker's "+
			"east and longer than the sun's", casts)
	}
	p.Write().SetLampShadows(0)
	if casts, _ := gathered(p, q, base, cam); len(casts) != 2 {
		t.Errorf("the lamps' shadows off, %d are cast, want the sun's two", len(casts))
	}
	p.Write().SetLampShadows(1)
	p.OverRelief(level{})
	if casts, _ := gathered(p, q, base, cam); len(casts) != 2 {
		t.Errorf("on a relief %d are cast, want the sun's two: the lamps cast on level ground "+
			"alone", len(casts))
	}
}

func TestPlugin_ComposeBakesTheBlocksOnLevelGroundAndLaysTheCastsOverIt(t *testing.T) {
	var base goke.Comp[world.Base]
	var q *goke.Query
	var p *light.Plugin
	joined := goke.SystemFn{OnInit: func(si *goke.SysInit) {
		qb := si.NewQueryBuilder(&base)
		light.Lighting(p).Join(qb)
		q = qb.Build()
	}}
	_, _, tick := litWorld(t, light.Config{}, func(w *world.Plugin, pl *light.Plugin) {
		p = pl
		w.Write().Seed(castKind(w, "walker", light.Patch).Entry(spot{500, 500}))
	}, joined)
	cam := cameratest.NewFromSpace(1000, 1000, 0)
	places := func() (opaque, over int) {
		var f render.Frame
		f.Reset(cam)
		p.Compose(&f, cam)
		f.Places(func(pass render.Pass, _ render.At) {
			if pass == render.Opaque {
				opaque++
			} else {
				over++
			}
		})
		return opaque, over
	}
	tick()
	if o, s := places(); o != 0 || s != 0 {
		t.Errorf("without a sun the light draws %d opaque and %d over, want nothing", o, s)
	}
	p.WithDistant(light.Distant{Dir: [3]float32{-1, 0, 1}, Strength: 1})
	tick()
	if o, s := places(); o != 0 || s != 0 {
		t.Errorf("with nothing Baked on level ground the light draws %d opaque and %d over, "+
			"want nothing", o, s)
	}
	gathered(p, q, base, cam)
	if o, s := places(); o != 0 || s != 1 {
		t.Errorf("a shadow gathered, the light draws %d opaque and %d over, want the cast over "+
			"the ground", o, s)
	}
}

func TestPlugin_ComposePlansTheBakeOfATowerOnLevelGround(t *testing.T) {
	_, p, tick := litWorld(t, light.Config{}, func(w *world.Plugin, pl *light.Plugin) {
		w.Write().Seed(castKind(w, "tower", light.Baked).Entry(spot{500, 500}))
	})
	p.WithDistant(light.Distant{Dir: [3]float32{-1, 0, 1}, Strength: 1})
	tick()
	cam := cameratest.NewFromSpace(1000, 1000, 0)
	var f render.Frame
	f.Reset(cam)
	p.Compose(&f, cam)
	opaque := false
	f.Places(func(pass render.Pass, _ render.At) { opaque = opaque || pass == render.Opaque })
	if !opaque {
		t.Error("a tower standing Baked on level ground, the light bakes nothing")
	}
}

func TestPlugin_SaysTheSunsShareOfTheLight(t *testing.T) {
	_, p, tick := litWorld(t, light.Config{}, func(w *world.Plugin, p *light.Plugin) {
		w.Write().Seed(lampKind(w, "torch", light.Source{Strength: 1, Reach: 100}).Entry(spot{500, 500}))
	})
	p.WithDistant(light.Distant{Dir: [3]float32{0, 0, 1}, Strength: 0.6, Ambient: 0.3})
	tick()
	p.Write().Begin(&render.Frame{}, cameratest.NewFromSpace(1000, 1000, 0))
	if lit := p.Read(0).At(10, 10, 0); !near(lit.Direct, 0.6/0.9) {
		t.Errorf("far from the torch the sun's share is %v, want two thirds", lit.Light)
	}
	// at the torch its 1 joins the sun's 0.6 and the sky's 0.3
	if lit := p.Read(0).At(500, 500, 0); !near(lit.Direct, 0.6/1.9) {
		t.Errorf("at the torch the sun's share is %v, want 0.6 of 1.9", lit.Light)
	}
	if lit := render.WhiteLight; lit.Direct != 0 {
		t.Error("white light has a sun's share: a shadow would darken it")
	}
}
