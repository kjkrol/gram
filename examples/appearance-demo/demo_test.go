package main

import (
	"math"
	"math/rand/v2"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/entity"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/internal/hosts"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/rule"
)

// stageInit is a game.Initializer that drives the real Stage without a window.
type stageInit struct {
	hosts   []plugin.Host // of the rules of the moments the plugins installed catch
	ecs     *goke.ECS
	world   *world.Plugin
	tracked []any
	pending []func() []goke.System
	tps     game.TPS
}

var _ game.Initializer = (*stageInit)(nil)

func (c *stageInit) UseModule(m goke.Module) {
	regSys := goke.SystemFn{OnInit: func(*goke.SysInit) { m.RegSystems(c.ecs) }}
	c.tracked = append(c.tracked, m)
	c.pending = append(c.pending, func() []goke.System { return append(m.SetupSystems(), regSys) })
}

func (c *stageInit) Setup(providers ...goke.SetupProvider) {
	for _, p := range providers {
		c.tracked = append(c.tracked, p)
		c.pending = append(c.pending, p.SetupSystems)
	}
}

func (c *stageInit) RegSys(factory func() goke.System) goke.Runnable { return c.ecs.RegSys(factory()) }
func (c *stageInit) ECS() *goke.ECS                                  { return c.ecs }
func (c *stageInit) TPS() *game.TPS                                  { return &c.tps }

func (c *stageInit) Use(p plugin.Plugin) error {
	c.tracked = append(c.tracked, p)
	return p.Install(c)
}

func (c *stageInit) Track(s plugin.Serializable) error {
	c.tracked = append(c.tracked, s)
	return nil
}

// Screen is the window's size, which the cameras are sized to, as the engine's Initializer says.
func (c *stageInit) Screen() (int, int) { return ScreenWidth, ScreenHeight }

func (c *stageInit) UseWorld(cfg world.Config) *world.Plugin {
	c.world = world.NewPlugin(cfg)
	c.tracked = append(c.tracked, c.world)
	if err := c.world.Install(c); err != nil {
		panic(err)
	}
	return c.world
}

// recording is a Look that remembers the Appearance each box is drawn with, layer by layer.
type recording struct {
	world.Look
	drawn map[geom.Vec][]render.Appearance
}

func (r *recording) Sprite(f *render.Frame, cam camera.Camera, box plane.AABB, z world.Z, atlas render.AtlasSource, a render.Appearance, light render.Light) {
	r.drawn[box.TopLeft] = append(r.drawn[box.TopLeft], a)
}

// noAtlas is an AtlasSource with no sheet: what a recording look needs.
type noAtlas struct{}

func (noAtlas) Atlas() *render.Image                                    { return nil }
func (noAtlas) UV(render.SpriteID) (float32, float32, float32, float32) { return 0, 0, 1, 1 }
func (noAtlas) White() (float32, float32)                               { return 0, 0 }

// shown is one entity as it was last drawn: its kind, the way it went and its looks.
type shown struct {
	kind kind.ID
	vel  world.Velocity
	apps []render.Appearance
}

// headingIdx4 is which of the four ways round the circle v heads — east, north, west, south —
// in the engine's one convention, as the walkers' Facing picks its twin.
func headingIdx4(v world.Velocity) int {
	i := int(math.Round(math.Atan2(-v.Dir.Y, v.Dir.X) / (2 * math.Pi) * 4))
	return ((i % 4) + 4) % 4
}

// angleDeg is the way v heads as a turned sprite's Angle.
func angleDeg(v world.Velocity) float32 {
	return float32(math.Atan2(-v.Dir.Y, v.Dir.X) * 180 / math.Pi)
}

// drawnStage is the demo's Stage built without a window and drawn through a recording look.
type drawnStage struct {
	t     *testing.T
	ecs   *goke.ECS
	arena *arena
	stage game.Stage
	rec   *recording
	base  goke.Comp[world.Base]
	all   *goke.Query
}

func newDrawnStage(t *testing.T) *drawnStage {
	t.Helper()
	rng = rand.New(rand.NewPCG(0x5eed, 0xc0ffee))
	ds := &drawnStage{t: t}
	ds.arena, ds.stage = newArena()
	ctx := &stageInit{ecs: goke.New()}
	if err := ds.stage.Init(ctx); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := ctx.Deliver(ctx.world.Kinds().Played()...); err != nil { // as the engine does once Init returns
		t.Fatalf("roles: %v", err)
	}
	if err := ds.stage.Spawn(); err != nil {
		t.Fatalf("Spawn: %v", err)
	}
	for _, v := range ctx.tracked {
		if p, ok := v.(plugin.Populator); ok {
			if err := p.Populate(); err != nil {
				t.Fatalf("Populate: %v", err)
			}
		}
	}
	w := ds.arena.world
	w.WithRenderer(ds.arena.looks())
	ds.rec = &recording{Look: w.Look()}
	w.SetLook(ds.rec)
	ctx.ecs.SetPlan(func(rc goke.RunCtx, d time.Duration) { ds.stage.Update(rc, d); w.Clock().Replay(rc, d) })
	var systems []goke.System
	for _, produce := range ctx.pending {
		systems = append(systems, produce()...)
	}
	systems = append(systems, goke.SystemFn{OnInit: func(si *goke.SysInit) {
		w.Renderer().(interface{ Init(*goke.SysInit) }).Init(si)
		ds.all = si.NewQueryBuilder(&ds.base).Build()
	}})
	ctx.ecs.Setup(systems...)
	ds.ecs = ctx.ecs
	return ds
}

func (ds *drawnStage) tick(n int) {
	for range n {
		ds.ecs.Tick(time.Second / TPS)
	}
}

// draw composes a frame and tells what every entity was drawn with.
func (ds *drawnStage) draw() []shown {
	ds.rec.drawn = map[geom.Vec][]render.Appearance{}
	cam := ds.arena.player.Camera
	var f render.Frame
	f.Reset(cam)
	ds.arena.world.Renderer().(interface {
		Compose(*render.Frame, camera.Camera)
	}).Compose(&f, cam)
	var out []shown
	for ds.all.All(); ds.all.Next(); {
		for _, b := range ds.base.Slice(ds.all.Cursor()) {
			out = append(out, shown{kind: b.TypeID, vel: b.Vel, apps: ds.rec.drawn[b.Pos.TopLeft]})
		}
	}
	if len(out) != Walkers+Ghosts+Leaders {
		ds.t.Fatalf("drew %d entities, want %d", len(out), Walkers+Ghosts+Leaders)
	}
	return out
}

// Every look comes from the atlas alone: a walker is drawn by the twin of the way it goes, a
// ghost as a ghost turned the way it drifts, the leader turned with his crown; the rage swaps
// the angry looks in — the leader still turned, the walker's anger without a direction — and
// lets them go when it ends.
func TestAppearance_DrawnAsTheAtlasSays(t *testing.T) {
	ds := newDrawnStage(t)
	walkerBase := kind.Named[walker](ds.arena.world.Kinds(), WalkerKind).SpriteID()
	ghostBase := kind.Named[walker](ds.arena.world.Kinds(), GhostKind).SpriteID()
	leaderBase := kind.Named[walker](ds.arena.world.Kinds(), LeaderKind).SpriteID()
	ghostKind := kind.Named[walker](ds.arena.world.Kinds(), GhostKind).ID()
	leaderKind := kind.Named[walker](ds.arena.world.Kinds(), LeaderKind).ID()
	angry := ds.arena.world.Effects().Named(AngryEf)
	angryWalker, angryLeader := angry.Look(walkerBase), angry.Look(leaderBase)

	check := func(phase string, raging bool) {
		t.Helper()
		twins := map[int]render.SpriteID{} // the walkers' faced twin seen per way
		for _, e := range ds.draw() {
			if len(e.apps) != 1 {
				t.Fatalf("%s: a %v drawn with %d layers, want 1", phase, e.kind, len(e.apps))
			}
			a := e.apps[0]
			switch e.kind {
			case ghostKind: // a ghost whatever happens, turned the way it drifts
				if a.SpriteID != ghostBase || a.Angle != angleDeg(e.vel) {
					t.Fatalf("%s: a ghost drawn with %v, want sprite %v at %v°", phase, a, ghostBase, angleDeg(e.vel))
				}
			case leaderKind: // turned with his crown, red while raging — and still turned
				want := leaderBase
				if raging {
					want = angryLeader
				}
				if a.SpriteID != want || a.Angle != angleDeg(e.vel) {
					t.Fatalf("%s: the leader drawn with %v, want sprite %v at %v°", phase, a, want, angleDeg(e.vel))
				}
			default: // a walker: the twin of its way, or anger without a direction
				if raging {
					if a.SpriteID != angryWalker || a.Angle != 0 {
						t.Fatalf("%s: a raging walker drawn with %v, want sprite %v unturned", phase, a, angryWalker)
					}
					continue
				}
				if a.SpriteID == walkerBase || a.Angle != 0 {
					t.Fatalf("%s: a walker drawn with %v, want a faced twin, unturned", phase, a)
				}
				way := headingIdx4(e.vel)
				if seen, ok := twins[way]; ok && seen != a.SpriteID {
					t.Fatalf("%s: two walkers going the same way drawn with %v and %v", phase, seen, a.SpriteID)
				}
				twins[way] = a.SpriteID
			}
		}
		if !raging {
			seen := map[render.SpriteID]bool{}
			for _, id := range twins {
				if seen[id] {
					t.Fatalf("%s: two ways share the twin %v", phase, id)
				}
				seen[id] = true
			}
		}
	}

	ds.tick(2)
	check("calm", false)

	if !ds.arena.world.Carrier().Put(1, rule.Cast(ds.arena.world.Effects().Named(RageEf)).On(entity.World)) {
		t.Fatal("the world carries no Apply")
	}
	ds.tick(5)
	check("in a rage", true)

	ds.tick(int(rageFor/(time.Second/TPS)) + 10)
	check("after the rage", false)
}

// Hosts keeps the hosts of the rules of the moments a plugin catches.
func (c *stageInit) Hosts(h ...plugin.Host) { c.hosts = append(c.hosts, h...) }

// Deliver hands rules — a role's, each of its own — to the hosts of their moments, as the engine
// does with the roles played once a Stage's Init returns.
func (c *stageInit) Deliver(rules ...rule.Rule) error { return hosts.Deliver(c.hosts, rules...) }
