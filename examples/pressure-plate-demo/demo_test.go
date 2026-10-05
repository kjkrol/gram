package main

import (
	"github.com/kjkrol/gram/internal/engine"
	"github.com/kjkrol/gram/rule"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
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

// testStage is the demo built fresh, without a window, and a view of its units.
type testStage struct {
	*mainStage
	ecs   *goke.ECS
	base  goke.Comp[world.Base]
	units *goke.Query
}

func buildStage(t *testing.T) *testStage {
	t.Helper()
	s := &testStage{mainStage: newStage()}
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

func (s *testStage) tick(n int) {
	for range n {
		s.ecs.Tick(time.Second / TPS)
	}
}

// strip is every unit whose centre stands on a trapdoor of group i.
func (s *testStage) strip(i int) map[uid.UID64]bool {
	out := map[uid.UID64]bool{}
	for s.units.All(); s.units.Next(); {
		cur := s.units.Cursor()
		for k, id := range cur.IDs {
			c, ok := s.brd.CellAt(s.base.Slice(cur)[k].Pos.Center())
			if x, y, _ := s.brd.Coords(c); ok && x >= groups[i].left && x <= groups[i].left+1 && y >= stripTop && y <= stripBottom {
				out[id] = true
			}
		}
	}
	return out
}

// holds reports whether the top trapdoor of group i holds a walker.
func (s *testStage) holds(i int) bool {
	c, _ := s.brd.CellIndex(groups[i].left, stripTop)
	return s.brd.Kind(c).Admits(cell.Land)
}

func (s *testStage) alive() map[uid.UID64]bool {
	out := map[uid.UID64]bool{}
	for s.units.All(); s.units.Next(); {
		for _, id := range s.units.Cursor().IDs {
			out[id] = true
		}
	}
	return out
}

// scout is a scout: a unit on the plates' row, where only the scouts start.
func (s *testStage) scout() uid.UID64 {
	for s.units.All(); s.units.Next(); {
		cur := s.units.Cursor()
		for k, id := range cur.IDs {
			if at, ok := s.brd.CellAt(s.base.Slice(cur)[k].Pos.Center()); ok {
				if _, y, _ := s.brd.Coords(at); y == plateRow {
					return id
				}
			}
		}
	}
	return 0
}

// put moves the unit id onto cell c.
func (s *testStage) put(id uid.UID64, c cell.ID) {
	to := cellBox(s.brd, c, EntitySize).TopLeft
	for s.units.All(); s.units.Next(); {
		cur := s.units.Cursor()
		for k, at := range cur.IDs {
			if at == id {
				s.world.Space().MoveTo(&s.base.Slice(cur)[k].Pos.AABB, to)
			}
		}
	}
}

// A scout standing on a plate opens its trapdoors, not the other's: whoever stands on one falls
// in; once the scout steps off, the plate stays pressed a while and the trapdoors shut.
func TestPlate_OpensItsTrapdoorsWhileSomeoneStandsOnIt(t *testing.T) {
	s := buildStage(t)
	var caught map[uid.UID64]bool
	for range 20 * TPS {
		s.tick(1)
		if caught = s.strip(0); len(caught) > 0 {
			break
		}
	}
	if len(caught) == 0 {
		t.Fatal("nobody walked onto the west strip in twenty seconds")
	}
	if !s.holds(0) || !s.holds(1) {
		t.Fatal("a trapdoor open before anyone stood on a plate")
	}
	plate, _ := s.brd.CellIndex(groups[0].plate, plateRow)
	scout := s.scout()
	s.put(scout, plate)
	s.tick(TPS / 2)
	alive := s.alive()
	for id := range caught {
		if alive[id] {
			t.Errorf("unit %d stood on a west trapdoor as the plate was pressed and is still here", id)
		}
	}
	if s.holds(0) || !s.holds(1) {
		t.Errorf("west plate pressed: west holds %v, east holds %v; want the west open alone", s.holds(0), s.holds(1))
	}
	away, _ := s.brd.CellIndex(GridWidth/2, plateRow)
	s.put(scout, away)
	s.tick(int(heldAfter.Seconds() * TPS / 2))
	if s.holds(0) {
		t.Error("the west trapdoors shut at once, want them open a while after the scout stepped off")
	}
	s.tick(2 * TPS)
	if !s.holds(0) {
		t.Error("the west trapdoors still open well after the scout stepped off the plate")
	}
}

// cellBox is the size x size box centred on cell c.
func cellBox(g grid.Grid, c cell.ID, size uint32) plane.AABB {
	at := g.CellCenter(c)
	half := float64(size) / 2
	return plane.NewAABB(geom.NewVec(at.X-half, at.Y-half), float64(size), float64(size))
}
