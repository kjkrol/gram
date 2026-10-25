package light_test

import (
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/plugins/light"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

type sheet struct{}

func (sheet) Atlas() *render.Image                            { return nil }
func (sheet) UV(render.SpriteID) (sx0, sy0, sx1, sy1 float32) { return 1, 2, 3, 4 }
func (sheet) White() (u, v float32)                           { return 9, 9 }

// a level relief for the shadows to fall on
type level struct{}

func (level) Lattice() (cols, rows int, cell float32, heights []float32, ok bool) {
	return 9, 9, 128, make([]float32, 81), true
}
func (level) GroundAt(geom.Vec) float32 { return 0 }
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

func TestPlugin_PublishesHowTheLampsAndThePolicyCastAndTheReliefUnderThem(t *testing.T) {
	w, p, tick := litWorld(t, light.Config{LampShadows: 1}, func(w *world.Plugin, pl *light.Plugin) {
		torch := castKind(w, "torch", light.NoShadow, light.Source{Strength: 1, Reach: 40})
		w.Write().Seed(torch.Entry(spot{490, 500}),
			castKind(w, "hero", light.Silhouette).Entry(spot{520, 500}))
	})
	p.WithDistant(light.Distant{Dir: [3]float32{-1, 0, 1}, Strength: 1})
	tick()
	tick()
	lit := lightsOf(w, p)
	if lit.Sun.Strength != 1 || len(lit.Lamps) != 1 || lit.Lamps[0].Z != 10 ||
		lit.LampShadows != 1 || lit.Policy != nil || lit.GroundAt != nil {
		t.Fatalf("published the sun %v strong, the lamps %v, %d casting, a policy %v, a relief "+
			"%v; want the sun, the torch 10 up casting, as declared, on level ground",
			lit.Sun.Strength, lit.Lamps, lit.LampShadows, lit.Policy != nil, lit.GroundAt != nil)
	}
	p.WithShadowPolicy(func(light.Casting, float32) light.Casting { return light.NoShadow })
	p.OverRelief(level{})
	if lit := lightsOf(w, p); lit.Policy == nil || lit.GroundAt == nil {
		t.Errorf("a policy %v, a relief %v published; want both", lit.Policy != nil,
			lit.GroundAt != nil)
	}
}

func TestPlugin_PublishesTheBlocksTheGroundBakesAndTheGroundUnderThem(t *testing.T) {
	sun := light.Distant{Dir: [3]float32{-1, 0, 1}, Strength: 1}
	w, p, tick := litWorld(t, light.Config{}, func(w *world.Plugin, pl *light.Plugin) {
		w.Write().Seed(castKind(w, "walker", light.Patch).Entry(spot{500, 500}))
	})
	p.WithDistant(sun)
	tick()
	if lit := lightsOf(w, p); len(lit.Blocks) != 0 || lit.Ground.Cols != 0 {
		t.Errorf("a walker alone: %d blocks over a ground of %d corners, want nothing to bake",
			len(lit.Blocks), lit.Ground.Cols)
	}
	w, p, tick = litWorld(t, light.Config{}, func(w *world.Plugin, pl *light.Plugin) {
		w.Write().Seed(castKind(w, "tower", light.Baked).Entry(spot{520, 500}))
	})
	tick()
	if lit := lightsOf(w, p); len(lit.Blocks) != 1 || lit.Ground.Cols != 0 {
		t.Errorf("a tower without a sun: %d blocks over a ground of %d corners, want the tower "+
			"and nothing to bake it over", len(lit.Blocks), lit.Ground.Cols)
	}
	p.WithDistant(sun)
	tick()
	lit := lightsOf(w, p)
	if len(lit.Blocks) != 1 || lit.Blocks[0].Top != 20 || lit.Ground.Cols < 2 ||
		lit.Ground.Span != 0 {
		t.Errorf("a tower under the sun: %d blocks %v over a ground of %d corners, span %v; "+
			"want the tower, 20 tall, over level ground", len(lit.Blocks), lit.Blocks,
			lit.Ground.Cols, lit.Ground.Span)
	}
}

func TestPlugin_SaysTheSunsShareOfTheLight(t *testing.T) {
	w, p, tick := litWorld(t, light.Config{}, func(w *world.Plugin, p *light.Plugin) {
		w.Write().Seed(lampKind(w, "torch", light.Source{Strength: 1, Reach: 100}).Entry(spot{500, 500}))
	})
	p.WithDistant(light.Distant{Dir: [3]float32{0, 0, 1}, Strength: 0.6, Ambient: 0.3})
	tick()
	if lit := litAt(w, p, whole(), 10, 10, 0, 0); !near(lit.Direct, 0.6/0.9) {
		t.Errorf("far from the torch the sun's share is %v, want two thirds", lit.Light)
	}
	// at the torch its 1 joins the sun's 0.6 and the sky's 0.3
	if lit := litAt(w, p, whole(), 500, 500, 0, 0); !near(lit.Direct, 0.6/1.9) {
		t.Errorf("at the torch the sun's share is %v, want 0.6 of 1.9", lit.Light)
	}
	if lit := render.WhiteLight; lit.Direct != 0 {
		t.Error("white light has a sun's share: a shadow would darken it")
	}
}
