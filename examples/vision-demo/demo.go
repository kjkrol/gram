package main

import (
	"math"
	"math/rand/v2"
	"time"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram"
	"github.com/kjkrol/gram/entity"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/entity/kind/comp"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/game/stage"
	"github.com/kjkrol/gram/plugins/cameras"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/players"
	"github.com/kjkrol/gram/plugins/vision"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/steering"
	"github.com/kjkrol/gram/rule"
	"github.com/kjkrol/gram/rule/effect"
	"github.com/kjkrol/gram/ui"
)

const (
	TPS          = 60
	ScreenWidth  = 1024
	ScreenHeight = 768

	PreyCount = 10
	RectSize  = 16
)

type body struct {
	pos world.Position
	vel world.Velocity
}

type arena struct {
	world     *world.Plugin
	vision    *vision.Plugin
	collision *collision.Plugin
	players   *players.Plugin
	cameras   *cameras.Plugin
	player    *players.Player

	hits collision.ContactStats
}

func main() {
	props := game.Props{
		Title:       "gram vision demo",
		Description: "Sight, avoidance and a hunter.",
		ScreenWidth: ScreenWidth, ScreenHeight: ScreenHeight, Resizable: true,
		TargetTPS: TPS,
	}
	a := &arena{}
	mainStage := stage.New(VisionStage).
		World(world.Config{
			Space: world.SpaceCfg{
				Width:  ScreenWidth,
				Height: ScreenHeight,
				Edges:  aabbworld.Torus,
			},
			Entities: world.EntitiesCfg{
				MaxCount: PreyCount + 1,
				MinSize:  RectSize,
				MaxSize:  RectSize,
			},
		}).
		Plugins(a.makePlugins).
		Players(a.definePlayer).
		Effects(a.defineEffects).
		Rules(a.defineRoles).
		Commands(a.defineCommands).
		Kinds(a.defineKinds).
		Controls(a.bindKeys).
		Spawn(a.spawnUnits).
		Scenes(a.defineScenes).
		Update(a.update)
	gram.Run(game.New(props, mainStage))
}

func (s *arena) makePlugins(w *world.Plugin) {
	s.world = w
	s.vision = vision.NewPlugin(s.world)
	s.collision = collision.NewPlugin(s.world).WithStats(&s.hits)
	s.cameras = cameras.NewPlugin(s.world)
	s.players = players.NewPlugin(s.world, s.cameras, s.vision)
}

func (s *arena) definePlayer() error {
	s.player = s.players.Local("player")
	return s.player.Bind(s.players.Defaults()...)
}

func (s *arena) defineEffects() {
	s.world.Effects().Define(FleeingEf, effect.Spec{})
	s.world.Effects().Define(LookedEf, effect.Spec{effect.Lasts(time.Second)})
}

func (s *arena) defineRoles() {
	s.world.Roles().Define(PreyRole)
	quarter := steering.Turn{Angle: math.Pi / 2}
	s.world.Roles().Define(PredatorRole,
		rule.Then[vision.Sighting](
			"chase",
			rule.Other(s.world.Roles().Named(PreyRole)),
			rule.Order(steering.Toward{}),
		),
		// seeing none, it turns a quarter aside, either way, once every while it has looked // TODO: jesli cos wymaga wyjasnienia to znaczy, ze nasza forma zpisu jest do dupy
		rule.Then[vision.Sighting](
			"search",
			rule.Other(s.world.Roles().Named(PreyRole)),
			rule.If(
				vision.Sighting.Nobody,
				rule.Unless(s.world.Effects().Named(LookedEf), rule.Steps(
					rule.Apply(s.world.Effects().Named(LookedEf)),
					rule.OneOf(
						rule.Chance(0.5, rule.Order(quarter)),
						rule.Order(steering.Turn{Angle: -quarter.Angle}),
					),
				),
				),
			),
		),
		rule.Then[collision.Meeting](
			"caught",
			rule.Other(s.world.Roles().Named(PreyRole)),
			rule.ForOther(rule.Order(world.Despawn{})),
		))
	s.world.Roles().Define(SkittishRole,
		rule.Then[vision.Sighting](
			"flee the hunter",
			rule.Other(s.world.Roles().Named(PredatorRole)),
			rule.During(s.world.Effects().Named(FleeingEf), rule.Order(steering.Away{})),
		),
		rule.Then[vision.Sighting](
			"give way",
			rule.All,
			rule.During(
				s.world.Effects().Named(FleeingEf),
				rule.If(vision.Sighting.Closing, rule.Order(steering.Away{})),
			),
		))
}

func (s *arena) defineCommands() {
	flee := rule.Toggle(s.world.Effects().Named(FleeingEf)).On(entity.World)
	s.world.Commands().Define(FleeCmd, flee)
}

func (s *arena) defineScenes(ctx game.Initializer) []game.Scene {
	m := &mainScene{arena: s, tps: ctx.TPS()}
	return []game.Scene{ui.NewScene(MainScene, m.screen()).Input(s.players.Handle)}
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

func sees() kind.Spec {
	return kind.Spec{
		comp.Load(func(b body) world.Position { return b.pos }),
		comp.Load(func(b body) world.Velocity { return b.vel }),
		comp.Load(func(b body) vision.Sight {
			return vision.Sight{Facing: b.vel.Dir, Radius: 200, Ahead: true}
		}),
		comp.Const(world.Eye{Angle: 2 * math.Pi / 5}),
		comp.Const(vision.SightOutline{}),
		comp.Const(collision.Collider{}),
	}
}

func (s *arena) spawnUnits() {
	placement := world.NewGridPlacement(ScreenWidth, ScreenHeight, RectSize)

	const total = PreyCount + 1
	roam := func(i int, speed float64) body {
		a := rand.Float64() * 2 * math.Pi
		return body{
			pos: placement.Place(i, total),
			vel: world.Velocity{Dir: geom.NewVec(math.Cos(a), math.Sin(a)), Value: speed},
		}
	}

	preyKind := kind.Named[body](s.world.Kinds(), PreyKind)
	hunterKind := kind.Named[body](s.world.Kinds(), HunterKind)
	entries := make([]kind.Entry, 0, total)
	for i := range PreyCount {
		entries = append(entries, preyKind.Entry(roam(i, 90)))
	}
	entries = append(entries, hunterKind.Entry(roam(PreyCount, 81)))
	s.world.Seed(entries...)
	s.world.Carrier().Put(s.player.ID, s.world.Commands().Named(FleeCmd))
}

func (s *arena) update(ctx goke.RunCtx, d time.Duration) {
	s.vision.RunPlan(ctx, d)
	s.world.RunPlan(ctx, d)
	s.collision.RunPlan(ctx, d)
	s.cameras.RunPlan(ctx, d)
	s.players.RunPlan(ctx, d)
	ctx.Sync()
}
