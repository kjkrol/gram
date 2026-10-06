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
	"github.com/kjkrol/gram/game/stage"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/players"
	"github.com/kjkrol/gram/plugins/vision"
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
type Demo struct {
	a     *arena
	stage game.Stage
}

var _ game.Game = (*Demo)(nil)

func NewDemo() *Demo {
	a, st := newArena()
	return &Demo{a: a, stage: st}
}

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

type arena struct {
	world     *world.Plugin
	vision    *vision.Plugin
	collision *collision.Plugin

	// the roles: the prey steer clear of the hunter and of each other, the hunter goes after them
	hits collision.ContactStats

	players *players.Plugin
	player  *players.Player
}

// newStage defines the game a section at a time, each building on those before it.
// newArena makes the arena — the collector of the stage's plugins, which every
// section builds on — and defines the stage on it, a section at a time.
func newArena() (*arena, game.Stage) {
	s := &arena{}
	return s, stage.New("vision-demo").
		Plugins(s.usePlugins).
		Players(s.definePlayer).
		Effects(s.defineEffects).
		Rules(s.defineRoles).
		Commands(s.defineCommands).
		Kinds(s.defineKinds).
		Controls(s.bindKeys).
		Scenes(s.defineScenes).
		Units(s.placeUnits).
		Update(s.update)
}

func (s *arena) usePlugins(ctx game.Initializer) error {
	s.world = ctx.UseWorld(world.Config{
		Space:    world.SpaceCfg{Width: ScreenWidth, Height: ScreenHeight, Edges: aabbworld.Torus},
		Entities: world.EntitiesCfg{MaxCount: PreyCount + 1, MinSize: RectSize, MaxSize: RectSize},
	})
	s.vision = vision.NewPlugin(s.world)
	s.vision.Hide(false) // the cones are what this demo shows: drawn from the start, Shift+C hides them
	s.collision = collision.NewPlugin(s.world).WithStats(&s.hits)
	s.players = players.NewPlugin(s.world, s.vision)
	for _, p := range []plugin.Plugin{s.vision, s.collision, s.players} {
		if err := ctx.Use(p); err != nil {
			return err
		}
	}
	return nil
}

func (s *arena) definePlayer() error {
	s.player = s.players.Local("player")
	return s.player.Bind(s.players.Defaults()...)
}

func (s *arena) defineEffects() {
	s.world.Effects().Define(FleeingEf, effect.Spec{})
	s.world.Effects().Define(LookedEf, effect.Spec{effect.Lasts(hunterLooksEvery)})
}

func (s *arena) defineRoles() {
	s.world.Roles().Define(PreyRole)
	quarter := steering.Turn{Angle: math.Pi / 2}
	s.world.Roles().Define(PredatorRole,
		rule.Then[vision.Sighting]("chase", rule.Other(s.world.Roles().Named(PreyRole)), rule.Order(steering.Toward{})),
		// seeing none, it turns a quarter aside, either way, once every while it has looked
		rule.Then[vision.Sighting]("search", rule.Other(s.world.Roles().Named(PreyRole)),
			rule.If(vision.Sighting.Nobody, rule.Unless(s.world.Effects().Named(LookedEf), rule.Steps(
				rule.Apply(s.world.Effects().Named(LookedEf)),
				rule.OneOf(rule.Chance(0.5, rule.Order(quarter)), rule.Order(steering.Turn{Angle: -quarter.Angle})))))),
		rule.Then[collision.Meeting]("caught", rule.Other(s.world.Roles().Named(PreyRole)), rule.ForOther(rule.Order(world.Despawn{}))))
	s.world.Roles().Define(SkittishRole,
		rule.Then[vision.Sighting]("flee the hunter", rule.Other(s.world.Roles().Named(PredatorRole)),
			rule.During(s.world.Effects().Named(FleeingEf), rule.Order(steering.Away{}))),
		rule.Then[vision.Sighting]("give way", rule.All,
			rule.During(s.world.Effects().Named(FleeingEf), rule.If(vision.Sighting.Closing, rule.Order(steering.Away{})))))
}

func (s *arena) defineCommands() {
	s.world.Commands().Define(FleeCmd, rule.Toggle(s.world.Effects().Named(FleeingEf)).On(entity.World))
}

func (s *arena) defineScenes(ctx game.Initializer) []game.Scene {
	return []game.Scene{&mainScene{arena: s, tps: ctx.TPS()}}
}

func (s *arena) defineKinds() {
	kinds := s.world.Kinds()
	kind.Define[body](kinds, PreyKind, append(sees(),
		comp.Const(steering.Steering{Reflex: 3, TurnRate: 0.12}),
		rule.Plays(s.world.Roles().Named(SkittishRole), s.world.Roles().Named(PreyRole)),
		comp.Const(collision.Physics{Restitution: 1}),
	))
	kind.Define[body](kinds, HunterKind, append(sees(),
		comp.Const(steering.Steering{Reflex: 1, TurnRate: 0.30}),
		rule.Plays(s.world.Roles().Named(PredatorRole)),
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

func (s *arena) placeUnits() {
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
		entries = append(entries, kind.Named[body](s.world.Kinds(), PreyKind).Entry(roam(i, roamSpeed)))
	}
	entries = append(entries, kind.Named[body](s.world.Kinds(), HunterKind).Entry(roam(PreyCount, hunterSpeed)))
	s.world.Seed(entries...)
	s.world.Carrier().Put(s.player.ID, s.world.Commands().Named(FleeCmd))
}

func (s *arena) update(ctx goke.RunCtx, d time.Duration) {
	s.vision.RunPlan(ctx, d)
	s.world.RunPlan(ctx, d)
	s.collision.RunPlan(ctx, d)
	s.players.RunPlan(ctx, d)
	ctx.Sync()
}

// =========================== Scene ===========================

type mainScene struct {
	arena *arena
	tps   *game.TPS
}

var _ game.Scene = (*mainScene)(nil)

func (m *mainScene) Name() string    { return "main" }
func (m *mainScene) Focusable() bool { return true }

func (m *mainScene) Layers() []render.Layer {
	s := m.arena

	atlas := render.NewAtlas()
	atlas.Add(kind.Named[body](s.world.Kinds(), PreyKind).SpriteID(), RectSize, render.Solid(color.RGBA{R: 120, G: 190, B: 255, A: 255}))
	atlas.Add(kind.Named[body](s.world.Kinds(), HunterKind).SpriteID(), RectSize, render.Solid(color.RGBA{R: 225, G: 70, B: 70, A: 255}))
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
	return m.arena.players.Viewports(screen)
}

func (m *mainScene) HandleEvents(events *control.InputEvents, runtime game.Runtime, composition game.Composition) {
	m.arena.players.Handle(events, runtime, composition)
}

func (s *arena) bindKeys() error {
	return s.player.Bind(control.Give(control.KeyPress{Key: control.KeyA}, "The prey flee, or stop fleeing",
		s.world.Commands().Named(FleeCmd)))
}
