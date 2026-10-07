package main

import (
	"cmp"
	"slices"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
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

// units are the entities of kind k, top to bottom.
func (s *testStage) all(t *testing.T, k string) []uid.UID64 {
	t.Helper()
	id := kind.Named[unitRow](s.world.Kinds(), k).ID()
	type at struct {
		id uid.UID64
		y  float64
	}
	var found []at
	for s.units.All(); s.units.Next(); {
		cur := s.units.Cursor()
		for i, e := range cur.IDs {
			if b := s.base.Slice(cur)[i]; b.TypeID == id {
				found = append(found, at{e, b.Pos.TopLeft.Y})
			}
		}
	}
	slices.SortFunc(found, func(a, b at) int { return cmp.Compare(a.y, b.y) })
	var out []uid.UID64
	for _, f := range found {
		out = append(out, f.id)
	}
	if len(out) == 0 {
		t.Fatalf("no %s", k)
	}
	return out
}

// put stands the entity id on cell (x, y).
func (s *testStage) put(id uid.UID64, x, y uint32) {
	at := s.board.Res.Logic.Board.CellCenter(s.board.Res.Logic.Board.CellIndex(x, y))
	if s.units.Seek(id) {
		pos := &s.base.At(s.units.Cursor()).Pos
		pos.AABB = plane.NewAABB(geom.NewVec(at.X-UnitSize/2, at.Y-UnitSize/2), UnitSize, UnitSize)
	}
}

var screen = geom.NewAABB(geom.NewVec(0, 0), geom.NewVec(ScreenWidth, ScreenHeight))

// click presses the left button in the middle of the element called name.
func (s *testStage) click(t *testing.T, name string) {
	t.Helper()
	s.scene.Lay(screen)
	e := s.scene.Element(name)
	if e == nil || !s.scene.Shown(name) {
		t.Fatalf("no %q shown", name)
	}
	b := e.Box()
	at := geom.NewVec((b.TopLeft.X+b.BottomRight.X)/2, (b.TopLeft.Y+b.BottomRight.Y)/2)
	s.scene.HandleEvents(&control.InputEvents{MousePos: at, ClickQueue: []control.ClickEvent{
		{Button: control.MouseButtonLeft, Action: control.ActionPress, Pos: at}}}, nil, nil)
}

// The traveller beside the host is greeted; an answer ends the hello, shows what the host made of
// it and keeps it from saying hello again for a while.
func TestDemo_TheHostGreetsTheTravellerWhoAnswers(t *testing.T) {
	s := buildStage(t)
	effects := s.world.Effects()
	greeting, talked, pleased := effects.Named(GreetingEf), effects.Named(TalkedEf), effects.Named(PleasedEf)
	s.tick(2)
	hosts, traveller := s.all(t, HostKind), s.all(t, TravellerKind)[0]
	host, other := hosts[1], hosts[0] // the lower host, at (16, 10); the upper one far off
	if effects.Has(host, greeting) {
		t.Fatal("the host says hello to a traveller far off")
	}
	s.put(traveller, 15, 10)
	s.tick(3)
	if !effects.Has(host, greeting) || effects.Has(other, greeting) {
		t.Fatalf("greeting: the host beside %v, the other %v; want the one beside alone",
			effects.Has(host, greeting), effects.Has(other, greeting))
	}
	s.click(t, PleaseCmd)
	s.tick(2)
	if effects.Has(host, greeting) || !effects.Has(host, talked) || !effects.Has(host, pleased) {
		t.Fatalf("after the answer: greeting %v, talked %v, pleased %v; want the hello over, the host pleased",
			effects.Has(host, greeting), effects.Has(host, talked), effects.Has(host, pleased))
	}
	if effects.Has(other, talked) || effects.Has(other, pleased) {
		t.Error("the answer reached the other host too")
	}
	s.tick(TPS)
	if effects.Has(host, greeting) {
		t.Error("the host said hello again right after it was answered")
	}
}

// The traveller walking off without a word lets the hello go.
func TestDemo_TheHelloGoesWithTheTraveller(t *testing.T) {
	s := buildStage(t)
	greeting := s.world.Effects().Named(GreetingEf)
	s.tick(2)
	host, traveller := s.all(t, HostKind)[1], s.all(t, TravellerKind)[0]
	s.put(traveller, 15, 10)
	s.tick(3)
	s.put(traveller, 3, 7)
	s.tick(3)
	if s.world.Effects().Has(host, greeting) {
		t.Error("the hello stays with the traveller gone")
	}
}
