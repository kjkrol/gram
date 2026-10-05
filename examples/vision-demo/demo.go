// Command vision-demo shows ten entities keeping out of each other's way by sight,
// and one red hunter that lives off the ones who fail at it. Press A to switch the
// avoidance off and watch the entity count fall; Shift+C shows the cones of sight.
package main

import (
	"image/color"
	"math"
	"math/rand/v2"
	"time"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/entity"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/entity/kind/comp"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/players"
	"github.com/kjkrol/gram/plugins/vision"
	vhooks "github.com/kjkrol/gram/plugins/vision/hooks"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/steering"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/rule"
	"github.com/kjkrol/gram/rule/effect"
)

const (
	TPS          = 60
	ScreenWidth  = 1024
	ScreenHeight = 768

	PreyCount = 10
	RectSize  = 16

	sightRadius = 200
	sightHalf   = math.Pi / 5
	roamSpeed   = 90
	// The hunter is the slower one: it only ever catches what steers badly.
	hunterSpeed = roamSpeed * 0.9
	// hunterLooksEvery is how long a hunter seeing nobody runs on before it looks aside again.
	hunterLooksEvery = time.Second
	backdropGrey     = 40
)

// =========================== Game ===========================

// Demo is the vision demo — exactly one Stage.
type Demo struct{ stage *mainStage }

var _ game.Game = (*Demo)(nil)

func NewDemo() *Demo { return &Demo{stage: &mainStage{}} }

func (d *Demo) Props() game.Props {
	return game.Props{
		Title:       "gram — sight, avoidance and a hunter",
		ScreenWidth: ScreenWidth, ScreenHeight: ScreenHeight, Resizable: true,
		TargetTPS: TPS,
	}
}

func (d *Demo) Stages() (map[string]game.Stage, string) {
	return map[string]game.Stage{d.stage.Name(): d.stage}, d.stage.Name()
}

// =========================== Stage ===========================

// body is the row both kinds spawn from: where an entity starts and where it heads.
type body struct {
	pos world.Position
	vel world.Velocity
}

type mainStage struct {
	world     *world.Plugin
	vision    *vision.Plugin
	prey      kind.Of[body]
	hunter    kind.Of[body]
	collision *collision.Plugin

	// the roles: the prey steer clear of the hunter and of each other, the hunter goes after them
	skittish, hunted, predator *rule.Part
	fleeing                    effect.Effect // on the world while the prey flee
	hits                       collision.ContactStats

	players *players.Plugin
	player  *players.Player

	stack game.Scenes
}

var _ game.Stage = (*mainStage)(nil)

func (s *mainStage) Name() string       { return "vision-demo" }
func (s *mainStage) Stack() game.Scenes { return s.stack }

func (s *mainStage) Init(ctx game.Initializer) error {
	s.world = ctx.UseWorld(world.Config{
		Space:    world.SpaceCfg{Width: ScreenWidth, Height: ScreenHeight, Edges: aabbworld.Torus},
		Entities: world.EntitiesCfg{MaxCount: PreyCount + 1, MinSize: RectSize, MaxSize: RectSize},
	})

	s.fleeing = s.world.Effects().Define("fleeing", effect.Spec{})
	looked := vhooks.Looked(s.world, hunterLooksEvery)
	s.hunted = rule.Role("prey")
	s.predator = rule.Role("predator").Obeys(vhooks.Chase(s.hunted), vhooks.Search(s.hunted, looked),
		rule.Then[collision.Meeting]("caught", rule.Other(s.hunted), rule.ForOther(rule.Order(world.Despawn{})))) // the hunter's prey is gone
	s.skittish = rule.Role("skittish").Obeys(vhooks.Flee(s.predator, s.fleeing)...)
	s.defineKinds()

	s.vision = vision.NewPlugin(s.world)
	s.collision = collision.NewPlugin(s.world).WithStats(&s.hits)

	if err := ctx.Use(s.vision); err != nil {
		return err
	}
	if err := ctx.Use(s.collision); err != nil {
		return err
	}

	// The player's camera: drag with the middle button, scroll with the wheel, push an edge.
	s.players = players.NewPlugin(s.world, s.vision)
	s.player = s.players.Local("player")
	if err := s.player.Bind(s.players.Defaults()...); err != nil {
		return err
	}
	if err := ctx.Use(s.players); err != nil {
		return err
	}
	// the roles' rules, each on the plugin that hosts its moment: vision's, and collision's
	if err := ctx.Hook(s.predator, s.skittish); err != nil {
		return err
	}

	main := &mainScene{stage: s, tps: ctx.TPS()}
	stack, err := game.NewStack(main)
	if err != nil {
		return err
	}
	s.stack = stack
	comp := stack.Composition()
	comp.Show(main.Name())
	return ctx.Track(comp)
}

func (s *mainStage) Restore(game.Persistence) (bool, error) { return false, nil }

// defineKinds says what this game's entities are, fresh or restored.
func (s *mainStage) defineKinds() {
	kinds := s.world.Kinds()
	s.prey = kind.Define[body](kinds, "prey", append(sees(),
		comp.Const(steering.Steering{Reflex: 3, TurnRate: 0.12}),
		rule.Plays(s.skittish, s.hunted),
		comp.Const(collision.Physics{Restitution: 1}),
	))
	s.hunter = kind.Define[body](kinds, "hunter", append(sees(),
		comp.Const(steering.Steering{Reflex: 1, TurnRate: 0.30}),
		rule.Plays(s.predator),
	))
}

// sees is what every kind here shares: a place, a heading, a cone looking that way, a collider.
func sees() kind.Spec {
	return kind.Spec{
		comp.Load(func(b body) world.Position { return b.pos }),
		comp.Load(func(b body) world.Velocity { return b.vel }),
		comp.Load(func(b body) vision.Sight {
			return vision.Sight{Facing: b.vel.Dir, Radius: sightRadius, Ahead: true}
		}),
		comp.Const(world.Eye{Angle: 2 * sightHalf}),
		comp.Const(vision.SightOutline{}),
		comp.Const(collision.Collider{}),
	}
}

// Spawn says who is there when the game starts fresh.
func (s *mainStage) Spawn() error {
	placement := world.NewGridPlacement(ScreenWidth, ScreenHeight, RectSize)

	const total = PreyCount + 1
	roam := func(i int, speed float64) body {
		a := rand.Float64() * 2 * math.Pi
		return body{
			pos: placement.Place(i, total),
			vel: world.Velocity{Dir: geom.NewVec(math.Cos(a), math.Sin(a)), Value: speed},
		}
	}

	entries := make([]kind.Entry, 0, total)
	for i := range PreyCount {
		entries = append(entries, s.prey.Entry(roam(i, roamSpeed)))
	}
	entries = append(entries, s.hunter.Entry(roam(PreyCount, hunterSpeed)))
	s.world.Seed(entries...)
	s.world.Commands().Put(s.player.ID, rule.Cast(s.fleeing).On(entity.World)) // the prey flee from the start
	return nil
}

func (s *mainStage) Update(ctx goke.RunCtx, d time.Duration) {
	s.vision.RunPlan(ctx, d)
	s.world.RunPlan(ctx, d)
	s.collision.RunPlan(ctx, d)
	s.players.RunPlan(ctx, d)
	ctx.Sync()
}

// =========================== Scene ===========================

type mainScene struct {
	stage *mainStage
	tps   *game.TPS
}

var _ game.Scene = (*mainScene)(nil)

func (m *mainScene) Name() string    { return "main" }
func (m *mainScene) Focusable() bool { return true }

func (m *mainScene) Layers() []render.Layer {
	s := m.stage

	atlas := render.NewAtlas()
	atlas.RegisterAt(s.prey.SpriteID(), RectSize, render.Solid(color.RGBA{R: 120, G: 190, B: 255, A: 255}))
	atlas.RegisterAt(s.hunter.SpriteID(), RectSize, render.Solid(color.RGBA{R: 225, G: 70, B: 70, A: 255}))
	atlas.Close()
	s.world.WithRenderer(atlas)
	s.vision.WithRenderer(atlas)

	count := func() int { return s.world.Res.Telemetry.Count }
	return []render.Layer{
		render.NewCachedRenderer(
			render.SolidBackground{Color: color.RGBA{R: backdropGrey, G: backdropGrey, B: backdropGrey + 6, A: 255}},
			ScreenWidth, ScreenHeight,
		),
		render.NewComposer(s.vision.Renderer(), s.world.Renderer()),
		render.NewTelemetryRenderer(&m.tps.Ticks, count).With(s.hits.Reporter(&m.tps.Ticks)),
	}
}

// Viewports are where the world is shown: the local players' views.
func (m *mainScene) Viewports(screen geom.AABB) []render.Viewport {
	return m.stage.players.Viewports(screen)
}

func (m *mainScene) HandleEvents(events *control.InputEvents, runtime game.Runtime, _ game.Composition) {
	m.stage.players.EventHandler().HandleEvents(events)
	for _, k := range events.KeyEvents {
		if k.Action != control.ActionPress {
			continue
		}
		switch k.Key {
		case control.KeyEscape:
			runtime.Quit()
		case control.KeyA:
			m.stage.switchFleeing()
		}
	}
}

// switchFleeing has the player take the fleeing off the world, or put it back on.
func (s *mainStage) switchFleeing() {
	s.world.Commands().Put(s.player.ID, rule.Toggle(s.fleeing).On(entity.World))
}
