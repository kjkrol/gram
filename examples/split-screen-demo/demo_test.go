package main

import (
	"math"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/internal/engine"
	"github.com/kjkrol/gram/internal/hosts"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/driving"
	"github.com/kjkrol/gram/plugins/players"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/steering"
	"github.com/kjkrol/gram/ui"
)

func TestDemo_TwoHalvesAndAMinimapOfTheWholeArena(t *testing.T) {
	d := NewDemo()
	if err := engine.NewEngine(d).Init(); err != nil {
		t.Fatal(err)
	}
	s := d.a
	main, _ := d.stage.Stack().Get(MainScene)
	main.(*ui.Scene).Lay(geom.NewAABB(geom.NewVec(0, 0), geom.NewVec(ScreenWidth, ScreenHeight)))

	half := (ScreenWidth - 2) / 2.0
	if a := s.redPlayer.Area(); a != geom.NewAABB(geom.NewVec(0, 0), geom.NewVec(half, ScreenHeight)) {
		t.Errorf("red looks over %v, want the left half", a)
	}
	if a := s.bluePlayer.Area(); a != geom.NewAABB(geom.NewVec(half+2, 0), geom.NewVec(ScreenWidth, ScreenHeight)) {
		t.Errorf("blue looks over %v, want the right half", a)
	}
	if s.redPlayer.Camera == s.bluePlayer.Camera {
		t.Error("the players look through a shared camera, want one of their own each")
	}
	if w, h := s.redPlayer.Camera.Viewport(); w != float32(math.Round(half)) || h != ScreenHeight {
		t.Errorf("red's camera sees %vx%v, want its half", w, h)
	}
	if w, _ := s.minimapCam.Viewport(); w != MinimapWidth {
		t.Errorf("the minimap is %v wide, want %d", w, MinimapWidth)
	}
	b := s.minimapCam.Bounds()
	if b.TopLeft.X > 0 || b.TopLeft.Y > 0 || b.BottomRight.X < WorldWidth-1 || b.BottomRight.Y < WorldHeight-1 {
		t.Errorf("the minimap shows %v, want the whole %dx%d arena", b, WorldWidth, WorldHeight)
	}
}

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

// testStage is the demo built fresh, without a window, and a view of its blocks.
type testStage struct {
	*arena
	stage  game.Stage
	ecs    *goke.ECS
	base   goke.Comp[world.Base]
	driven goke.OptComp[steering.Driven]
	blocks *goke.Query
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
	systems = append(systems, goke.SystemFn{OnInit: func(si *goke.SysInit) {
		s.blocks = si.NewQueryBuilder(&s.base).Optional(&s.driven).Build()
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

// block is where the block of kind k stands and how it is driven, if at all.
func (s *testStage) block(k string) (at geom.Vec, driven *steering.Driven) {
	id := kind.Named[block](s.world.Kinds(), k).ID()
	for s.blocks.All(); s.blocks.Next(); {
		cur := s.blocks.Cursor()
		for i := range cur.IDs {
			if b := s.base.Slice(cur)[i]; b.TypeID == id {
				at = b.Pos.Center()
				if d := s.driven.Slice(cur); d != nil {
					driven = &d[i]
				}
			}
		}
	}
	return at, driven
}

// From the start each player's camera follows its own block, and a player's keys drive its block
// the way they say and nobody else's.
func TestDemo_EachPlayerFollowsAndDrivesItsOwnBlock(t *testing.T) {
	s := buildStage(t)
	s.tick(2)
	redAt, _ := s.block(RedKind)
	blueAt, _ := s.block(BlueKind)
	for _, pl := range []*players.Player{s.redPlayer, s.bluePlayer} {
		f := pl.Camera.(camera.Fastenable).Fastening()
		if f.How != camera.Centred {
			t.Errorf("%s's camera is fastened %+v, want Centred over its block from the start", pl.Name, f)
		}
	}
	// a block near the arena's edge is kept as near the middle as the window may go: in view
	shows := func(pl *players.Player, at geom.Vec) bool {
		b := pl.Camera.Bounds()
		return at.X >= b.TopLeft.X && at.X <= b.BottomRight.X && at.Y >= b.TopLeft.Y && at.Y <= b.BottomRight.Y
	}
	if !shows(s.redPlayer, redAt) || !shows(s.bluePlayer, blueAt) {
		t.Error("the players' cameras do not show their blocks")
	}
	for range 12 {
		if err := s.players.Issue(s.redPlayer, driving.Toward{Camera: s.redPlayer.Camera, Way: geom.NewVec(1, 0)}); err != nil {
			t.Fatal(err)
		}
		s.tick(1)
	}
	redNow, redDriven := s.block(RedKind)
	blueNow, blueDriven := s.block(BlueKind)
	if redNow.X <= redAt.X+4 || math.Abs(redNow.Y-redAt.Y) > 1 {
		t.Errorf("driven right from %v the red block is at %v, want it gone right", redAt, redNow)
	}
	if redDriven == nil || redDriven.Face != geom.NewVec(1, 0) {
		t.Errorf("the red block is driven %+v, want to face right", redDriven)
	}
	if blueNow != blueAt || blueDriven != nil {
		t.Errorf("the blue block moved to %v or is driven %+v, want it left alone by red's keys", blueNow, blueDriven)
	}
	if !shows(s.redPlayer, redNow) {
		t.Error("red's camera did not follow its block")
	}
}
