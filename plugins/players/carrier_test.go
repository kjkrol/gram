package players_test

import (
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/entity/kind/comp"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/internal/engine"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/players"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/rule"
)

type carrierStage struct {
	boxes     bool
	world     *world.Plugin
	collision *collision.Plugin
	players   *players.Plugin
	leaver    kind.Of[float64]
	box       kind.Of[float64]
	units     *goke.Query
	base      goke.Comp[world.Base]
	stack     game.Scenes
}

func (s *carrierStage) Name() string { return "stage" }

func (s *carrierStage) Init(ctx game.Initializer) error {
	s.world = ctx.UseWorld(world.Config{
		Space:    world.SpaceCfg{Width: 1000, Height: 1000},
		Entities: world.EntitiesCfg{MaxCount: 3, MinSize: 10, MaxSize: 10},
	})
	s.collision = collision.NewPlugin(s.world)
	s.world.Roles().Define("fragile", rule.Branch[collision.Struck]{
		Name: "gone when struck",
		Do:   rule.Order(world.Despawn{}),
	})
	fragile := s.world.Roles().Named("fragile")
	at := func(x float64) world.Position {
		return world.Position{AABB: plane.NewAABB(geom.NewVec(x, 100), 10, 10)}
	}
	kind.Define[float64](s.world.Kinds(), "leaver", kind.Spec{
		comp.Load(at),
		comp.Const(world.Velocity{}),
		rule.Plan("leaver", rule.Steps(
			rule.Wait(100*time.Millisecond),
			rule.Order(world.Despawn{}),
		)),
	})
	s.leaver = kind.Named[float64](s.world.Kinds(), "leaver")
	kind.Define[float64](s.world.Kinds(), "box", kind.Spec{
		comp.Load(at),
		comp.Const(world.Velocity{}),
		comp.Const(collision.Collider{}),
		rule.Plays(fragile),
	})
	s.box = kind.Named[float64](s.world.Kinds(), "box")
	s.players = players.NewPlugin(s.world)
	s.players.Local("first")
	ctx.Setup(carrierProbe{s})
	if err := ctx.Use(s.collision); err != nil {
		return err
	}
	return ctx.Use(s.players)
}

func (s *carrierStage) Loaded() error { return nil }

func (s *carrierStage) Spawn() error {
	s.world.Seed(s.leaver.Entry(500))
	if s.boxes {
		s.world.Seed(s.box.Entry(100), s.box.Entry(105))
	}
	return nil
}

func (s *carrierStage) Update(ctx goke.RunCtx, d time.Duration) {
	s.world.RunPlan(ctx, d)
	s.collision.RunPlan(ctx, d)
	s.players.RunPlan(ctx, d)
}

func (s *carrierStage) Stack() game.Scenes {
	if s.stack == nil {
		s.stack, _ = game.NewStack()
	}
	return s.stack
}

func (s *carrierStage) count() int {
	n := 0
	for s.units.All(); s.units.Next(); {
		n += len(s.units.Cursor().IDs)
	}
	return n
}

type carrierProbe struct{ s *carrierStage }

func (p carrierProbe) SetupSystems() []goke.System {
	return []goke.System{goke.SystemFn{OnInit: func(si *goke.SysInit) { p.s.units = si.NewQueryBuilder(&p.s.base).Build() }}}
}

type carrierGame struct{ stage *carrierStage }

func (g carrierGame) Props() game.Props { return game.Props{} }
func (g carrierGame) Stages() (map[string]game.Stage, string) {
	return map[string]game.Stage{g.stage.Name(): g.stage}, g.stage.Name()
}

func run(t *testing.T, s *carrierStage, frames int, each func()) {
	t.Helper()
	eng := engine.NewEngine(carrierGame{s})
	if err := eng.Init(); err != nil {
		t.Fatal(err)
	}
	for range frames {
		if err := eng.Update(); err != nil {
			t.Fatal(err)
		}
		each()
		time.Sleep(10 * time.Millisecond)
	}
}

func TestCarrier_ACommandGivenAfterItsHandlersPassWaitsForTheNext(t *testing.T) {
	s := &carrierStage{boxes: true}
	run(t, s, 30, func() {})
	if n := s.count(); n != 0 {
		t.Errorf("%d units at the end, want none: the struck boxes and the leaver gave themselves a Despawn", n)
	}
}

func TestCarrier_ATreesCommandsGoWithinTheirFrame(t *testing.T) {
	s := &carrierStage{}
	run(t, s, 30, func() {
		if !s.world.Carrier().Empty() {
			t.Fatalf("a command waits past the frame at %v of game time", s.world.Clock().Time())
		}
	})
	if n := s.count(); n != 0 {
		t.Errorf("%d units at the end, want none: the leaver gave itself a Despawn", n)
	}
}
