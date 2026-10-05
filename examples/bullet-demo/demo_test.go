package main

import (
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/entity/tag"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/internal/hosts"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/bullet"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/rule"
	"github.com/kjkrol/gram/rule/effect"
	"github.com/kjkrol/uid"
)

// stageInit is a game.Initializer that drives the real Stage without a window;
// Scene.Layers() is left out.
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

// testStage is the demo built fresh, without a window, and a view of its units and shots.
type testStage struct {
	*mainStage
	test   *testing.T
	ecs    *goke.ECS
	base   goke.Comp[world.Base]
	marks  goke.OptComp[tag.Tags[effect.States]]
	flight goke.OptComp[bullet.Flight]
	units  *goke.Query
}

func buildStage(t *testing.T) *testStage {
	t.Helper()
	s := &testStage{mainStage: newStage()}
	ctx := &stageInit{ecs: goke.New()}
	if err := s.Init(ctx); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := ctx.Deliver(ctx.world.Kinds().Played()...); err != nil { // as the engine does once Init returns
		t.Fatalf("roles: %v", err)
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
		s.units = si.NewQueryBuilder(&s.base).Optional(&s.marks).Optional(&s.flight).Build()
	}})
	ctx.ecs.Setup(systems...)
	s.ecs = ctx.ecs
	return s
}

func (s *testStage) tick(n int) {
	for range n {
		s.ecs.Tick(time.Second / TPS)
	}
}

// wandererOn is the wanderer whose round begins on cell c, by where it stands as the game begins.
func (s *testStage) wandererOn(c cell.ID) uid.UID64 {
	for s.units.All(); s.units.Next(); {
		cur := s.units.Cursor()
		for k, id := range cur.IDs {
			if s.base.Slice(cur)[k].TypeID == s.wanderer.ID() && s.base.Slice(cur)[k].Pos.Center() == s.brd.CellCenter(c) {
				return id
			}
		}
	}
	s.t().Fatalf("no wanderer begins on %v", c)
	return 0
}

func (s *testStage) t() *testing.T { return s.test }

// state is whether id is still there and whether it is wounded.
func (s *testStage) state(id uid.UID64) (alive, wounded bool) {
	for s.units.All(); s.units.Next(); {
		cur := s.units.Cursor()
		for k, got := range cur.IDs {
			if got != id {
				continue
			}
			alive = true
			if m := s.marks.Slice(cur); m != nil {
				wounded = m[k].Has(s.wounded.Mark())
			}
		}
	}
	return
}

// shots is how many shots are in the air or lying.
func (s *testStage) shots() int {
	n := 0
	for s.units.All(); s.units.Next(); {
		if s.flight.Slice(s.units.Cursor()) != nil {
			n += len(s.units.Cursor().IDs)
		}
	}
	return n
}

// A round fired along the road wounds the wanderer walking it.
func TestShoot_ARoundWoundsTheWandererOnTheRoad(t *testing.T) {
	s := buildStage(t)
	s.test = t
	walker := s.wandererOn(s.cellAt(8, roadRow))
	for i := 0; i < 5*TPS; i++ {
		if i%15 == 0 {
			if err := s.players.Issue(s.player, bullet.Shoot{Ammo: s.round}); err != nil {
				t.Fatal(err)
			}
		}
		s.tick(1)
		if alive, wounded := s.state(walker); !alive || wounded {
			return
		}
	}
	t.Error("the wanderer on the road walked five seconds under fire unwounded")
}

// A grenade thrown behind the high wall comes down there and bursts: the wanderer walking its
// round there is wounded, or taken, and the grenade is gone.
func TestThrow_AGrenadeBurstsBehindTheHighWall(t *testing.T) {
	s := buildStage(t)
	s.test = t
	walker := s.wandererOn(s.cellAt(9, 4))
	at := s.brd.CellCenter(s.cellAt(10, 4))
	s.tick(1) // the soldier is told whose it is and selected as it is made: carried out in the first tick
	if err := s.players.Issue(s.player, bullet.Shoot{Ammo: s.grenade, At: geom.NewVec(at.X, at.Y), Targeted: true}); err != nil {
		t.Fatal(err)
	}
	s.tick(1)
	if s.shots() != 1 {
		t.Fatalf("%d shots after the throw, want the grenade", s.shots())
	}
	for i := 0; i < 6*TPS; i++ {
		s.tick(1)
		if alive, wounded := s.state(walker); !alive || wounded {
			if s.shots() != 0 {
				t.Errorf("%d shots after the burst, want the grenade gone", s.shots())
			}
			return
		}
	}
	t.Error("the wanderer behind the wall walked six seconds after the throw unwounded")
}

// Hosts keeps the hosts of the rules of the moments a plugin catches.
func (c *stageInit) Hosts(h ...plugin.Host) { c.hosts = append(c.hosts, h...) }

// Deliver hands rules — a role's, each of its own — to the hosts of their moments, as the engine
// does with the roles played once a Stage's Init returns.
func (c *stageInit) Deliver(rules ...rule.Rule) error { return hosts.Deliver(c.hosts, rules...) }
