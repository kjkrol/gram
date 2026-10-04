package main

import (
	"testing"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/entity/tag"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/internal/engine"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/steering"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/rule"
	"github.com/kjkrol/gram/rule/effect"
)

// stageInit is a game.Initializer that drives the real Stage without a window;
// Scene.Layers() is left out.
type stageInit struct {
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

func (c *stageInit) Hook(rules ...rule.Rule) error { return engine.HookOn(c.tracked, rules...) }

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

// stage is the demo built fresh, without a window, and a view of its units.
type stage struct {
	*mainStage
	ecs   *goke.ECS
	base  goke.Comp[world.Base]
	look  goke.Comp[world.Appearance]
	steer goke.Comp[steering.Steering]
	marks goke.OptComp[tag.Tags[effect.States]]
	units *goke.Query
}

func buildStage(t *testing.T) *stage {
	t.Helper()
	s := &stage{mainStage: &mainStage{}}
	ctx := &stageInit{ecs: goke.New()}
	if err := s.Init(ctx); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := s.Spawn(); err != nil {
		t.Fatalf("Spawn: %v", err)
	}
	for _, v := range ctx.tracked {
		if p, ok := v.(plugin.Populator); ok {
			if err := p.Populate(); err != nil {
				t.Fatalf("Populate: %v", err)
			}
		}
	}
	ctx.ecs.SetPlan(func(rc goke.RunCtx, d time.Duration) { s.Update(rc, d); s.world.Clock().Replay(rc, d) })
	var systems []goke.System
	for _, produce := range ctx.pending {
		systems = append(systems, produce()...)
	}
	systems = append(systems, goke.SystemFn{OnInit: func(si *goke.SysInit) {
		s.units = si.NewQueryBuilder(&s.base, &s.look, &s.steer).Optional(&s.marks).Build()
	}})
	ctx.ecs.Setup(systems...)
	s.ecs = ctx.ecs
	return s
}

func (s *stage) tick(n int) {
	for range n {
		s.ecs.Tick(time.Second / TPS)
	}
}

// boat is the boat as it stands: whether it is frozen in, halted, and what its Appearance says.
func (s *stage) boatState() (frozen, halted bool, sprite render.SpriteID, found bool) {
	for s.units.All(); s.units.Next(); {
		cur := s.units.Cursor()
		bases, looks, steers, marks := s.base.Slice(cur), s.look.Slice(cur), s.steer.Slice(cur), s.marks.Slice(cur)
		for i := range cur.IDs {
			if bases[i].TypeID != s.boat.ID() {
				continue
			}
			found, halted, sprite = true, steers[i].Halted, looks[i].SpriteID
			if marks != nil {
				frozen = marks[i].Has(s.frozen.Mark())
			}
		}
	}
	return
}

// The boat sails into the witch's trail and is frozen in: halted by the effect, under its marker,
// its Appearance still the boat's own sprite — its frozen look is the drawing rule's, swapped in
// from its kind's twin.
func TestFrozen_ABoatInTheIceIsHaltedAndKeepsItsOwnSpriteTheLookBeingTheRules(t *testing.T) {
	s := buildStage(t)
	var frozen, halted, found bool
	var sprite render.SpriteID
	for i := 0; i < 15*TPS && !frozen; i++ {
		s.tick(1)
		frozen, halted, sprite, found = s.boatState()
	}
	if !found {
		t.Fatal("the boat is gone")
	}
	if !frozen || !halted {
		t.Fatalf("after fifteen seconds the boat is frozen %v, halted %v; want it frozen in on the witch's trail", frozen, halted)
	}
	if sprite != s.boat.SpriteID() {
		t.Errorf("the frozen boat's Appearance is sprite %v, want its own %v: the look is the drawing rule's", sprite, s.boat.SpriteID())
	}
	if twin, ok := s.frozenLook[s.boat.SpriteID()]; !ok || twin == s.boat.SpriteID() {
		t.Errorf("the boat's frozen look is %v, %v; want a twin of its own", twin, ok)
	}
}
