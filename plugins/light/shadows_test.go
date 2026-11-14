package light_test

import (
	"slices"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/goke/v3"
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
	if lit := lightsOf(w, p); len(blocksOf(lit)) != 0 || lit.Ground.Cols != 0 {
		t.Errorf("a walker alone: %d blocks over a ground of %d corners, want nothing to bake",
			len(blocksOf(lit)), lit.Ground.Cols)
	}
	w, p, tick = litWorld(t, light.Config{}, func(w *world.Plugin, pl *light.Plugin) {
		w.Write().Seed(castKind(w, "tower", light.Baked).Entry(spot{520, 500}))
	})
	tick()
	if lit := lightsOf(w, p); len(blocksOf(lit)) != 1 || lit.Ground.Cols != 0 {
		t.Errorf("a tower without a sun: %d blocks over a ground of %d corners, want the tower "+
			"and nothing to bake it over", len(blocksOf(lit)), lit.Ground.Cols)
	}
	p.WithDistant(sun)
	tick()
	lit := lightsOf(w, p)
	if bs := blocksOf(lit); len(bs) != 1 || bs[0].Top != 20 || lit.Ground.Cols < 2 ||
		lit.Ground.Span != 0 {
		t.Errorf("a tower under the sun: %d blocks %v over a ground of %d corners, span %v; "+
			"want the tower, 20 tall, over level ground", len(bs), bs, lit.Ground.Cols,
			lit.Ground.Span)
	}
}

// blocksOf is every block lit holds, whatever its set
func blocksOf(lit *render.Lights) []render.Block {
	var out []render.Block
	for _, c := range lit.Casts {
		out = append(out, c.Blocks...)
	}
	return out
}

// versionsOf is the version of each of lit's sets
func versionsOf(lit *render.Lights) []uint64 {
	var out []uint64
	for _, c := range lit.Casts {
		out = append(out, c.Version)
	}
	return out
}

func TestPlugin_PublishesTheBlocksASetASectorEachVersioned(t *testing.T) {
	cfg := levelWorld
	// buckets of 32, eight of them a sector's: four by four sectors 256 a side
	cfg.Sectors = world.SectorsCfg{Buckets: 8}
	var tower kind.Of[spot]
	w, p, tick := litWorldIn(t, cfg, light.Config{}, func(w *world.Plugin, p *light.Plugin) {
		tower = castKind(w, "tower", light.Baked)
		w.Write().Seed(tower.Entry(spot{100, 100}), tower.Entry(spot{900, 100}),
			castKind(w, "walker", light.Patch).Entry(spot{600, 600}))
	})
	tick()
	lit := lightsOf(w, p)
	if len(lit.Casts) != 16 {
		t.Fatalf("the blocks are published in %d sets, want one a sector, 16", len(lit.Casts))
	}
	for i, c := range lit.Casts {
		if want := i == 0 || i == 3; (len(c.Blocks) == 1) != want || len(c.Blocks) > 1 {
			t.Errorf("sector %d's set holds %d blocks, want a tower %v", i, len(c.Blocks), want)
		}
	}
	if c := lit.Casts[0]; c.Box != [4]float32{98, 98, 4, 4} || c.Top != 20 {
		t.Errorf("the first tower's set lies over %v up to %v, want its box (98, 98) 4 a side, "+
			"20 high", c.Box, c.Top)
	}
	before := versionsOf(lit)
	if after := versionsOf(lightsOf(w, p)); !slices.Equal(after, before) {
		t.Errorf("published again unchanged, the versions are %v, were %v", after, before)
	}
	w.Write().Spawn(tower.Entry(spot{600, 620}))
	tick()
	after := versionsOf(lightsOf(w, p))
	for i := range after {
		if raised := after[i] != before[i]; raised != (i == 10) {
			t.Errorf("a tower spawned in sector 10, sector %d's version %d, was %d", i, after[i],
				before[i])
		}
	}
	if len(lightsOf(w, p).Casts[10].Blocks) != 1 {
		t.Error("the tower spawned is in no set of its sector")
	}

	w, p, tick = litWorld(t, light.Config{}, func(w *world.Plugin, p *light.Plugin) {
		tower = castKind(w, "tower", light.Baked)
		w.Write().Seed(tower.Entry(spot{100, 100}), tower.Entry(spot{900, 100}))
	})
	tick()
	if lit := lightsOf(w, p); len(lit.Casts) != 1 || len(lit.Casts[0].Blocks) != 2 ||
		lit.Casts[0].Box != [4]float32{98, 98, 804, 4} {
		t.Errorf("a world of no sectors publishes %d sets, the first of %d blocks over %v; want "+
			"one of both towers", len(lit.Casts), len(blocksOf(lit)), lit.Casts[0].Box)
	}
}

// a piece of a deck 16 long and 8 wide over the box 4 a side at x, 500, or the handle of a deck
// of three from x 500 to 548, its box round them, both 20 up and 2 thick, lying east
type laid struct{ x float32 }

func TestPlugin_PublishesABridgesHandleLyingAndNoBlockForItsDecks(t *testing.T) {
	sun := light.Distant{Dir: [3]float32{-1, 0, 1}, Strength: 1}
	w, p, tick := litWorld(t, light.Config{}, func(w *world.Plugin, p *light.Plugin) {
		kind.Define[laid](w.Kinds(), "deck", kind.Spec{
			comp.Load(func(r laid) world.Position {
				return world.Position{AABB: plane.NewAABB(geom.NewVec(r.x-2, 498), 4, 4)}
			}),
			comp.Const(world.Velocity{Dir: geom.NewVec(1, 0)}),
			comp.Const(world.Z{Altitude: 20, Height: 2}),
			comp.Const(world.Lie{Length: 16, Width: 8}),
			comp.Const(light.Shadow{Cast: light.NoShadow}),
			comp.Const(world.Fixed{}),
		})
		kind.Define[laid](w.Kinds(), "handle", kind.Spec{
			comp.Load(func(r laid) world.Position {
				return world.Position{AABB: plane.NewAABB(geom.NewVec(r.x, 496), 48, 8)}
			}),
			comp.Const(world.Velocity{Dir: geom.NewVec(1, 0)}),
			comp.Const(world.Z{Altitude: 20, Height: 2}),
			comp.Const(world.Lie{Length: 48, Width: 8}),
			comp.Const(light.Shadow{Cast: light.Baked}),
			comp.Const(world.Fixed{}),
			comp.Const(world.Whole{}),
		})
		deck := kind.Named[laid](w.Kinds(), "deck")
		w.Write().SeedWhole(kind.Named[laid](w.Kinds(), "handle").Entry(laid{500}),
			deck.Entry(laid{508}), deck.Entry(laid{524}), deck.Entry(laid{540}))
	})
	p.WithDistant(sun)
	tick()
	want := render.Block{X: 524, Y: 500, UX: 1, Along: 24, Wide: 4, Bottom: 18, Top: 20}
	if bs := blocksOf(lightsOf(w, p)); len(bs) != 1 || bs[0] != want {
		t.Errorf("a bridge of three pieces of deck publishes the blocks %v, want its handle's "+
			"alone, %v", bs, want)
	}
}

func TestPlugin_PublishesAThingStandingTurnedAsABlockOverItsStand(t *testing.T) {
	w, p, tick := litWorld(t, light.Config{}, func(w *world.Plugin, p *light.Plugin) {
		kind.Define[laid](w.Kinds(), "house", kind.Spec{
			comp.Load(func(r laid) world.Position {
				return world.Position{AABB: plane.NewAABB(geom.NewVec(r.x-2, 498), 4, 4)}
			}),
			comp.Const(world.Velocity{Dir: geom.NewVec(0, 1)}),
			comp.Const(world.Z{Altitude: 5, Height: 9}),
			comp.Const(world.Stand{Length: 4, Width: 2}),
			comp.Const(light.Shadow{Cast: light.Baked}),
			comp.Const(world.Fixed{}),
		})
		w.Write().Seed(kind.Named[laid](w.Kinds(), "house").Entry(laid{500}))
	})
	p.WithDistant(light.Distant{Dir: [3]float32{-1, 0, 1}, Strength: 1})
	tick()
	want := render.Block{X: 500, Y: 500, UX: 0, UY: 1, Along: 2, Wide: 1, Bottom: 5, Top: 14}
	if bs := blocksOf(lightsOf(w, p)); len(bs) != 1 || bs[0] != want {
		t.Errorf("a house standing turned publishes the blocks %v, want %v", bs, want)
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

func TestPlugin_GathersTheBlocksAnewOnlyAsTheirChunksOrTheWorldsRestillsMove(t *testing.T) {
	cfg := levelWorld
	cfg.Sectors = world.SectorsCfg{Buckets: 8}
	var tower kind.Of[spot]
	r := &raiser{}
	w, p, tick := litWorldIn(t, cfg, light.Config{}, func(w *world.Plugin, p *light.Plugin) {
		tower = castKind(w, "tower", light.Baked)
		w.Write().Seed(tower.Entry(spot{100, 100}), tower.Entry(spot{900, 100}))
	}, r)
	tick()
	lightsOf(w, p)
	lightsOf(w, p)
	gathers := light.Gathers(p)
	before := versionsOf(lightsOf(w, p))
	lightsOf(w, p)
	if got := light.Gathers(p); got != gathers {
		t.Errorf("published twice more unchanged, the blocks were gathered %d times, want %d: "+
			"both frames' lights hold them", got, gathers)
	}
	// a tower raised in place: nothing of it tells the light till the world is told
	r.raise(5)
	if after := versionsOf(lightsOf(w, p)); !slices.Equal(after, before) {
		t.Errorf("a tower raised in place with the world told nothing, the versions are %v, "+
			"were %v", after, before)
	}
	w.Write().Restill()
	after := versionsOf(lightsOf(w, p))
	if slices.Equal(after, before) {
		t.Errorf("a tower raised in place and the world told, the versions are %v still", after)
	}
	if got := light.Gathers(p); got != gathers+1 {
		t.Errorf("the blocks were gathered %d times, want %d: once more for the Restill", got,
			gathers+1)
	}
	w.Write().Spawn(tower.Entry(spot{600, 620}))
	tick()
	if lit := lightsOf(w, p); len(lit.Casts[10].Blocks) != 1 || light.Gathers(p) != gathers+2 {
		t.Errorf("a tower spawned, its sector's set holds %d blocks after %d gathers; want 1, %d",
			len(lit.Casts[10].Blocks), light.Gathers(p), gathers+2)
	}
}

// raiser raises every Z of the world in place, as a relief shaped under what stands would
type raiser struct {
	q *goke.Query
	z goke.Comp[world.Z]
}

func (r *raiser) Init(si *goke.SysInit) { r.q = si.NewQueryBuilder(&r.z).Build() }

func (r *raiser) Update(*goke.CmdBuf, time.Duration) {}

func (r *raiser) raise(by float32) {
	for r.q.All(); r.q.Next(); {
		for i := range r.z.Slice(r.q.Cursor()) {
			r.z.Slice(r.q.Cursor())[i].Altitude += by
		}
	}
}
