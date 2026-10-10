package main

import (
	"fmt"
	"math/rand/v2"
	"time"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/entity/kind/comp"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/game/stage"
	"github.com/kjkrol/gram/plugins/cameras"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/players"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/rule"
	"github.com/kjkrol/gram/rule/effect"
	"github.com/kjkrol/gram/ui"
)

const (
	TPS          = 60 * 2
	ScreenWidth  = 1024
	ScreenHeight = 1024

	// hitDuration is how long an entity keeps showing a collision.
	hitDuration = 50 * time.Millisecond
)

const (
	RectSize    = 5
	EntityCount = ScreenWidth * ScreenHeight / 5 / (RectSize * RectSize) // a fifth of the screen

	entityColors = 7
	entityShapes = 4
)

type body struct {
	pos world.Position
	vel world.Velocity
}

// bodyKind names the kind drawn with color ci and shape si.
func bodyKind(ci, si int) string { return fmt.Sprintf("entity-%d-%d", ci, si) }

type arena struct {
	world          *world.Plugin
	collision      *collision.Plugin
	players        *players.Plugin
	cameras        *cameras.Plugin
	player         *players.Player
	collisionStats collision.ContactStats
}

func NewDemo(s *arena) game.Game {
	return game.New(game.Props{
		Title:       "gram collision demo",
		ScreenWidth: ScreenWidth, ScreenHeight: ScreenHeight, Resizable: true,
		TargetTPS: TPS,
	},
		stage.New(CollisionStage).
			World(world.Config{
				Space: world.SpaceCfg{
					Width:  ScreenWidth,
					Height: ScreenHeight,
					Edges:  aabbworld.Torus,
				},
				Entities: world.EntitiesCfg{
					MaxCount: EntityCount,
					MinSize:  RectSize,
					MaxSize:  RectSize,
				},
			}).
			Plugins(s.makePlugins).
			Players(s.definePlayer).
			Effects(s.defineEffects).
			Rules(s.defineRules).
			Kinds(s.defineKinds).
			Spawn(s.spawnUnits).
			Scenes(s.defineScenes).
			Update(s.update))
}

func (s *arena) makePlugins(w *world.Plugin) {
	s.world = w
	s.collision = collision.NewPlugin(s.world).WithStats(&s.collisionStats)
	s.cameras = cameras.NewPlugin(s.world)
	s.players = players.NewPlugin(s.world, s.cameras)
}

func (s *arena) definePlayer() error {
	s.player = s.players.Local("player")
	return s.player.Bind(s.players.Defaults()...)
}

func (s *arena) defineEffects() {
	s.world.Effects().Define(HitEf, effect.Spec{effect.Lasts(hitDuration)})
}

func (s *arena) defineRules() {
	s.world.Roles().Define(BodyRole,
		rule.Then[collision.Struck]("hit", rule.All, rule.Apply(s.world.Effects().Named(HitEf))))
}

func (s *arena) defineScenes(ctx game.Initializer) []game.Scene {
	m := &mainScene{arena: s, tps: ctx.TPS()}
	return []game.Scene{ui.NewScene(MainScene, m.screen()).Input(s.players.Handle)}
}

func (s *arena) defineKinds() {
	kinds := s.world.Kinds()
	for ci := range entityColors {
		for si := range entityShapes {
			kind.Define[body](kinds, bodyKind(ci, si), kind.Spec{
				comp.Load(func(b body) world.Position { return b.pos }),
				comp.Load(func(b body) world.Velocity { return b.vel }),
				comp.Const(collision.Collider{}),
				comp.Const(collision.Physics{Restitution: 1}),
				rule.Plays(s.world.Roles().Named(BodyRole)),
			})
		}
	}
}

func (s *arena) spawnUnits() {
	placement := world.NewGridPlacement(ScreenWidth, ScreenHeight, RectSize)
	motion := newRandomVelocity(200, 50, 10)
	bodyKindOf := func(c, sh int) kind.Of[body] {
		return kind.Named[body](s.world.Kinds(), bodyKind(c, sh))
	}
	entries := make([]kind.Entry, EntityCount)
	for i := range entries {
		entries[i] = bodyKindOf(rand.IntN(entityColors), rand.IntN(entityShapes)).Entry(
			body{pos: placement.Place(i, EntityCount), vel: motion.initialVelocity(i)})
	}
	s.world.Seed(entries...)
}

func (s *arena) update(ctx goke.RunCtx, d time.Duration) {
	s.world.RunPlan(ctx, d)
	s.collision.RunPlan(ctx, d)
	s.cameras.RunPlan(ctx, d)
	s.players.RunPlan(ctx, d)
	ctx.Sync()
}
