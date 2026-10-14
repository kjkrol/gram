package main

import (
	"fmt"
	"math/rand/v2"
	"time"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/ceremony"
	"github.com/kjkrol/ceremony/act"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/ceremony/cond"
	"github.com/kjkrol/ceremony/effect"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/game/stage"
	"github.com/kjkrol/gram/plugins/cameras"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/players"
	"github.com/kjkrol/gram/plugins/telemetry"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/ui"
)

const (
	TPS          = 60 * 2
	ScreenWidth  = 1024
	ScreenHeight = 1024

	hitDuration = 50 * time.Millisecond
)

const (
	RectSize    = 5
	EntityCount = ScreenWidth * ScreenHeight / 5 / (RectSize * RectSize)

	entityColors = 7
	entityShapes = 4
)

type body struct {
	pos world.Position
	vel world.Velocity
}

func bodyKind(ci, si int) string { return fmt.Sprintf("entity-%d-%d", ci, si) }

type arena struct {
	world          *world.Plugin
	collision      *collision.Plugin
	players        *players.Plugin
	telemetry      *telemetry.Plugin
	cameras        *cameras.Plugin
	player         *players.Player
	collisionStats collision.ContactStats
}

func main() {
	props := game.Props{
		Title:       "gram collision demo",
		ScreenWidth: ScreenWidth, ScreenHeight: ScreenHeight, Resizable: true,
		TargetTPS: TPS,
	}
	a := &arena{}
	mainStage := stage.New(CollisionStage).
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
		Plugins(a.makePlugins).
		Players(a.definePlayer).
		Effects(a.defineEffects).
		Rules(a.defineRules).
		Kinds(a.defineKinds).
		Spawn(a.spawnUnits).
		Scenes(a.defineScenes).
		Shows(MainScene, telemetry.SceneName).
		Update(a.update)
	gram.Run(game.New(props, mainStage))
}

func (a *arena) makePlugins(w *world.Plugin) {
	a.world = w
	a.collision = collision.NewPlugin(a.world).WithStats(&a.collisionStats)
	a.cameras = cameras.NewPlugin(a.world)
	a.telemetry = telemetry.NewPlugin(a.world)
	a.players = players.NewPlugin(a.world, a.collision, a.cameras)
}

func (a *arena) definePlayer() error {
	a.player = a.players.Local("player")
	return a.player.Bind(a.players.Defaults()...)
}

func (a *arena) defineEffects() {
	a.world.Effects().Define(HitEf, effect.Spec{effect.Lasts(hitDuration)})
}

func (a *arena) defineRules() {
	a.world.Roles().Define(BodyRole, act.Branch[collision.Struck]{
		Name: "hit",
		When: cond.Self.If(cond.Always),
		Then: act.Self.Put(a.world.Effects().Named(HitEf)),
	})
}

func (a *arena) defineKinds() {
	kinds := a.world.Kinds()
	for ci := range entityColors {
		for si := range entityShapes {
			kind.Define[body](kinds, bodyKind(ci, si), kind.Spec{
				comp.Load(func(b body) world.Position { return b.pos }),
				comp.Load(func(b body) world.Velocity { return b.vel }),
				comp.Const(collision.Collider{}),
				comp.Const(collision.Physics{Restitution: 1}),
				ceremony.Plays(a.world.Roles().Named(BodyRole)),
			})
		}
	}
}

func (a *arena) spawnUnits() {
	placement := world.NewGridPlacement(ScreenWidth, ScreenHeight, RectSize)
	motion := newRandomVelocity(200, 50, 10)
	bodyKindOf := func(c, sh int) kind.Of[body] {
		return kind.Named[body](a.world.Kinds(), bodyKind(c, sh))
	}
	entries := make([]kind.Entry, EntityCount)
	for i := range entries {
		entries[i] = bodyKindOf(rand.IntN(entityColors), rand.IntN(entityShapes)).Entry(
			body{pos: placement.Place(i, EntityCount), vel: motion.initialVelocity(i)})
	}
	a.world.Seed(entries...)
}

func (a *arena) defineScenes(ctx game.Initializer) []game.Scene {
	m := &mainScene{arena: a}
	a.telemetry.With(a.collisionStats.Reporter(&ctx.TPS().Ticks))
	return []game.Scene{ui.NewScene(MainScene, m.screen()).Input(a.players.Handle)}
}

func (a *arena) update(ctx goke.RunCtx, d time.Duration) {
	a.world.RunPlan(ctx, d)
	a.collision.RunPlan(ctx, d)
	a.cameras.RunPlan(ctx, d)
	a.players.RunPlan(ctx, d)
	ctx.Sync()
}
