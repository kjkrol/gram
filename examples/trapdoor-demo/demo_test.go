package main

import (
	"github.com/kjkrol/gram/internal/engine"
	"github.com/kjkrol/gram/rule"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/selection"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/uid"
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

func (c *stageInit) Commands(cmds ...rule.Casting) error { return c.world.Triggers(cmds...) }

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
	systems = append(systems, goke.SystemFn{OnInit: func(si *goke.SysInit) { s.units = si.NewQueryBuilder(&s.base).Build() }})
	ctx.ecs.Setup(systems...)
	s.ecs = ctx.ecs
	return s
}

func (s *stage) tick(n int) {
	for range n {
		s.ecs.Tick(time.Second / TPS)
	}
}

// onStrip is every unit whose centre stands on a trapdoor of lever i.
func (s *stage) onStrip(i int) map[uid.UID64]bool {
	out := map[uid.UID64]bool{}
	for s.units.All(); s.units.Next(); {
		cur := s.units.Cursor()
		for k, id := range cur.IDs {
			c, ok := s.brd.CellAt(s.base.Slice(cur)[k].Pos.Center())
			if x, y, _ := s.brd.Coords(c); ok && x >= levers[i].left && x <= levers[i].left+1 && y >= stripTop && y <= stripBottom {
				out[id] = true
			}
		}
	}
	return out
}

// holds reports whether the top trapdoor of lever i holds a walker.
func (s *stage) holds(i int) bool {
	c, _ := s.brd.CellIndex(levers[i].left, stripTop)
	return s.brd.Kind(c).Admits(cell.Land)
}

func (s *stage) alive() map[uid.UID64]bool {
	out := map[uid.UID64]bool{}
	for s.units.All(); s.units.Next(); {
		for _, id := range s.units.Cursor().IDs {
			out[id] = true
		}
	}
	return out
}

// A lever opens its own trapdoors for a while, not the other's: whoever stands on one of them then
// falls in, and once the lever goes back they hold again — the fallen holding no cell.
func TestLever_OpensItsTrapdoorsUnderWhoeverStandsOnThem(t *testing.T) {
	s := buildStage(t)
	var caught map[uid.UID64]bool
	for range 20 * TPS {
		s.tick(1)
		if caught = s.onStrip(0); len(caught) > 0 {
			break
		}
	}
	if len(caught) == 0 {
		t.Fatal("nobody walked onto the west strip in twenty seconds")
	}
	s.world.Commands().Put(s.player.ID, s.pulls[0])
	s.tick(TPS / 2)
	alive := s.alive()
	for id := range caught {
		if alive[id] {
			t.Errorf("unit %d stood on a west trapdoor as its lever was pulled and is still here", id)
		}
	}
	if s.holds(0) || !s.holds(1) {
		t.Errorf("west lever pulled: west holds %v, east holds %v; want the west open alone", s.holds(0), s.holds(1))
	}
	s.tick(3 * TPS)
	if !s.holds(0) {
		t.Error("a west trapdoor still open after its lever went back")
	}
	held, alive := s.board.Occupancy().(*cell.SingleOccupancy), s.alive()
	s.brd.EachCell(func(c cell.ID) {
		if id, ok := held.Holder(c, cell.Land); ok && !alive[id] {
			t.Errorf("cell %d is still held by %d, fallen in and gone", c, id)
		}
	})
}

// J hastens the player's selected scouts: the player selecting over the whole meadow selects its
// three scouts, never the wanderers nobody owns, and they alone are hastened.
func TestHaste_HastensTheSelectedScouts(t *testing.T) {
	s := buildStage(t)
	s.tick(1)
	everywhere := geom.NewAABBAt(geom.NewVec(0, 0), ScreenWidth, ScreenHeight)
	s.world.Commands().Put(s.player.ID, selection.Select{Box: everywhere})
	s.world.Commands().Put(s.player.ID, s.hasten)
	s.tick(2)
	hastened := 0
	for id := range s.alive() {
		if s.haste.On(id) {
			hastened++
		}
	}
	if hastened != 3 {
		t.Errorf("%d units hastened, want the three scouts", hastened)
	}
}
