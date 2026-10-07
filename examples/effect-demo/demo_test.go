package main

import (
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/internal/hosts"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/uid"
)

// stageInit is a game.Initializer that drives the real Stage without a window; Scene.Layers() is
// left out.
type stageInit struct {
	hosts   []plugin.Host
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
func (c *stageInit) Hosts(h ...plugin.Host)                          { c.hosts = append(c.hosts, h...) }

func (c *stageInit) Use(p plugin.Plugin) error {
	c.tracked = append(c.tracked, p)
	return p.Install(c)
}

func (c *stageInit) Track(s plugin.Serializable) error {
	c.tracked = append(c.tracked, s)
	return nil
}

func (c *stageInit) UseWorld(cfg world.Config) *world.Plugin {
	c.world = world.NewPlugin(cfg)
	c.tracked = append(c.tracked, c.world)
	if err := c.world.Install(c); err != nil {
		panic(err)
	}
	return c.world
}

// testStage is the demo built fresh, without a window, its scene's layers initialised.
type testStage struct {
	*arena
	stage game.Stage
	ecs   *goke.ECS
	base  goke.Comp[world.Base]
	units *goke.Query
}

func buildStage(t *testing.T) *testStage {
	t.Helper()
	s := &testStage{}
	s.arena, s.stage = newArena()
	ctx := &stageInit{ecs: goke.New()}
	if err := s.stage.Init(ctx); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := hosts.Deliver(ctx.hosts, ctx.world.Kinds().Played()...); err != nil { // as the engine does once Init returns
		t.Fatalf("roles: %v", err)
	}
	if err := s.stage.Spawn(); err != nil {
		t.Fatalf("Spawn: %v", err)
	}
	for _, v := range ctx.tracked {
		if p, ok := v.(plugin.Populator); ok {
			if err := p.Populate(); err != nil {
				t.Fatalf("Populate: %v", err)
			}
		}
	}
	ctx.ecs.SetPlan(func(rc goke.RunCtx, d time.Duration) { s.stage.Update(rc, d); s.world.Clock().Replay(rc, d) })
	var systems []goke.System
	for _, produce := range ctx.pending {
		systems = append(systems, produce()...)
	}
	systems = append(systems, goke.SystemFn{OnInit: func(si *goke.SysInit) { s.units = si.NewQueryBuilder(&s.base).Build() }})
	for _, l := range s.scene.Layers() { // as the engine does, entering the Stage
		systems = append(systems, goke.SystemFn{OnInit: l.Init})
	}
	ctx.ecs.Setup(systems...)
	s.ecs = ctx.ecs
	return s
}

func (s *testStage) tick(n int) {
	for range n {
		s.ecs.Tick(time.Second / TPS)
	}
}

// witch is the witch's entity.
func (s *testStage) witch(t *testing.T) uid.UID64 {
	t.Helper()
	id := kind.Named[unitRow](s.world.Kinds(), WitchKind).ID()
	for s.units.All(); s.units.Next(); {
		cur := s.units.Cursor()
		for i, e := range cur.IDs {
			if s.base.Slice(cur)[i].TypeID == id {
				return e
			}
		}
	}
	t.Fatal("no witch")
	return 0
}

var screen = geom.NewAABB(geom.NewVec(0, 0), geom.NewVec(ScreenWidth, ScreenHeight))

// The witch reaching the ice puts winter on her and holds the game; the window stands above her,
// and its Thaw button thaws the lake, ends the winter, brings her spring and lets the game go on.
func TestDemo_TheWitchOnTheIceAsksForADecision(t *testing.T) {
	s := buildStage(t)
	effects := s.world.Effects()
	winter, spring, iced := effects.Named(WinterEf), effects.Named(SpringEf), effects.Named(IcedEf)
	witch := s.witch(t)
	for range 20 * TPS {
		if effects.Has(witch, winter) {
			break
		}
		s.tick(1)
	}
	s.tick(2) // winter lands in the next step's effects, the pause it brings in the world's next pass
	if !effects.Has(witch, winter) || !s.world.Clock().State().Paused {
		t.Fatalf("winter on the witch %v, the game held %v; want both once she is on the ice",
			effects.Has(witch, winter), s.world.Clock().State().Paused)
	}
	s.scene.Lay(screen)
	if !s.scene.Shown(DecisionWindow) || s.decision.Box().BottomRight.Y <= 0 {
		t.Fatalf("the decision stands at %v, want shown on the screen", s.decision.Box())
	}
	at := s.thaw.Box().TopLeft.Add(geom.NewVec(3, 3))
	s.scene.HandleEvents(&control.InputEvents{MousePos: at, ClickQueue: []control.ClickEvent{
		{Button: control.MouseButtonLeft, Action: control.ActionPress, Pos: at}}}, nil, nil)
	s.tick(2)
	if effects.Has(witch, winter) || !effects.Has(witch, spring) || s.world.Clock().State().Paused {
		t.Errorf("after Thaw: winter %v, spring %v, held %v; want no winter, spring, the game going on",
			effects.Has(witch, winter), effects.Has(witch, spring), s.world.Clock().State().Paused)
	}
	lake, _ := s.board.CellEntity(s.board.Res.Logic.Board.CellIndex(lakeLeft, 8))
	if effects.Has(lake, iced) {
		t.Error("the lake is still iced after Thaw")
	}
}
