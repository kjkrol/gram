package main

import (
	"math/rand/v2"
	"slices"
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

func (c *stageInit) UseWorld(cfg world.Config) *world.Plugin {
	cfg.Camera.ViewportWidth = ScreenWidth
	cfg.Camera.ViewportHeight = ScreenHeight
	c.world = world.NewPlugin(cfg)
	c.tracked = append(c.tracked, c.world)
	if err := c.world.Install(c); err != nil {
		panic(err)
	}
	return c.world
}

// recording is a Look that remembers the sprites each box is drawn with, layer by layer.
type recording struct {
	world.Look
	drawn map[geom.Vec][]render.SpriteID
}

func (r *recording) Sprite(f *render.Frame, cam camera.Camera, box plane.AABB, z world.Z, atlas render.AtlasSource, id render.SpriteID, light render.Light, sway float32) {
	r.drawn[box.TopLeft] = append(r.drawn[box.TopLeft], id)
}

// noAtlas is an AtlasSource with no sheet: what a recording look needs.
type noAtlas struct{}

func (noAtlas) Atlas() *render.Image                                    { return nil }
func (noAtlas) UV(render.SpriteID) (float32, float32, float32, float32) { return 0, 0, 1, 1 }
func (noAtlas) White() (float32, float32)                               { return 0, 0 }

// shown is one entity as it was last drawn: its kind, the way it went and its sprites.
type shown struct {
	kind    kind.ID
	heading heading
	sprites []render.SpriteID
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
	w.WithRenderer(noAtlas{})
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
	ds.rec.drawn = map[geom.Vec][]render.SpriteID{}
	cam := ds.arena.world.Camera()
	var f render.Frame
	f.Reset(cam)
	ds.arena.world.Renderer().(interface {
		Compose(*render.Frame, camera.Camera)
	}).Compose(&f, cam)
	var out []shown
	for ds.all.All(); ds.all.Next(); {
		for _, b := range ds.base.Slice(ds.all.Cursor()) {
			out = append(out, shown{kind: b.TypeID, heading: headingOf(b.Vel), sprites: ds.rec.drawn[b.Pos.TopLeft]})
		}
	}
	if len(out) != Walkers+Ghosts+Leaders {
		ds.t.Fatalf("drew %d entities, want %d", len(out), Walkers+Ghosts+Leaders)
	}
	return out
}

// want is what an entity of its kind going its way is drawn with, angry or not.
func (ds *drawnStage) want(e shown, angry bool) []render.SpriteID {
	s := ds.arena
	sprite := s.facing[e.heading]
	if angry {
		sprite = s.angrySprite[e.heading]
	}
	switch e.kind {
	case kind.Named[walker](s.world.Kinds(), GhostKind).ID():
		return []render.SpriteID{s.spook}
	case kind.Named[walker](s.world.Kinds(), LeaderKind).ID():
		return []render.SpriteID{sprite, s.crown}
	}
	return []render.SpriteID{sprite}
}

// Every walker is drawn facing the way it goes, every ghost as a ghost, the leader with a crown;
// the rage turns the walkers and the leader red, and leaves the ghosts ghosts; its Appearance is
// never touched.
func TestAppearance_DrawnAsTheRulesSayAndFollowingTheMood(t *testing.T) {
	ds := newDrawnStage(t)
	ds.tick(2)
	for _, e := range ds.draw() {
		if want := ds.want(e, false); !slices.Equal(e.sprites, want) {
			t.Fatalf("calm: a %v going %v drawn with %v, want %v", e.kind, e.heading, e.sprites, want)
		}
	}

	if !ds.arena.world.Carrier().Put(1, rule.Cast(ds.arena.world.Effects().Named(RageEf)).On(entity.World)) {
		t.Fatal("the world carries no Apply")
	}
	ds.tick(5)
	for _, e := range ds.draw() {
		if want := ds.want(e, true); !slices.Equal(e.sprites, want) {
			t.Fatalf("in a rage: a %v going %v drawn with %v, want %v", e.kind, e.heading, e.sprites, want)
		}
	}

	ds.tick(int(rageFor/(time.Second/TPS)) + 10)
	for _, e := range ds.draw() {
		if want := ds.want(e, false); !slices.Equal(e.sprites, want) {
			t.Fatalf("after the rage: a %v going %v drawn with %v, want %v", e.kind, e.heading, e.sprites, want)
		}
	}
}

// Hosts keeps the hosts of the rules of the moments a plugin catches.
func (c *stageInit) Hosts(h ...plugin.Host) { c.hosts = append(c.hosts, h...) }

// Deliver hands rules — a role's, each of its own — to the hosts of their moments, as the engine
// does with the roles played once a Stage's Init returns.
func (c *stageInit) Deliver(rules ...rule.Rule) error { return hosts.Deliver(c.hosts, rules...) }
