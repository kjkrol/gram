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
	"github.com/kjkrol/gram/rule/plan"
)

// carrierStage is a world with collision and the players, and a leaver whose tree gives itself a
// Despawn after a wait; with boxes, two boxes striking each other that give themselves one from a
// rule of collision's — a pass after the world's, which drains Despawn.
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
	s.world.Roles().Define("fragile", rule.Then[collision.Struck]("gone when struck", rule.All, rule.Order(world.Despawn{})))
	fragile := s.world.Roles().Named("fragile")
	at := func(x float64) world.Position {
		return world.Position{AABB: plane.NewAABB(geom.NewVec(x, 100), 10, 10)}
	}
	s.leaver = kind.Define[float64](s.world.Kinds(), "leaver", kind.Spec{
		comp.Load(at),
		comp.Const(world.Velocity{}),
		plan.New("leaver", func(a *plan.Actor) rule.Step {
			return a.Steps(a.Wait(100*time.Millisecond), a.Order(world.Despawn{}))
		}),
	})
	s.box = kind.Define[float64](s.world.Kinds(), "box", kind.Spec{
		comp.Load(at),
		comp.Const(world.Velocity{}),
		comp.Const(collision.Collider{}),
		rule.Plays(fragile),
	})
	s.players = players.NewPlugin(s.world)
	s.players.Local("first")
	ctx.Setup(carrierProbe{s})
	if err := ctx.Use(s.collision); err != nil {
		return err
	}
	return ctx.Use(s.players)
}

func (s *carrierStage) Restore(game.Persistence) (bool, error) { return false, nil }

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

// count is how many units are in the world.
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

// run runs s in the engine for frames frames, calling each after every one.
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

// Nothing is dropped: the boxes' Despawn, given after the world's pass that drains it, waits for
// the next frame's — the players' pass once cleared every queue at the end of a frame, and struck
// units never went.
func TestCarrier_ACommandGivenAfterItsHandlersPassWaitsForTheNext(t *testing.T) {
	s := &carrierStage{boxes: true}
	run(t, s, 30, func() {})
	if n := s.count(); n != 0 {
		t.Errorf("%d units at the end, want none: the struck boxes and the leaver gave themselves a Despawn", n)
	}
}

// A tree's commands go within the frame they are given in — the trees run in the world's pass,
// before the handlers' — so no queue holds one between frames, and a save loses none.
func TestCarrier_ATreesCommandsGoWithinTheirFrame(t *testing.T) {
	s := &carrierStage{}
	run(t, s, 30, func() {
		if !s.world.Commands().Empty() {
			t.Fatalf("a command waits past the frame at %v of game time", s.world.Clock().Time())
		}
	})
	if n := s.count(); n != 0 {
		t.Errorf("%d units at the end, want none: the leaver gave itself a Despawn", n)
	}
}
