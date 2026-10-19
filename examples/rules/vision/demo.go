package main

import (
	"math"
	"math/rand/v2"
	"time"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/ceremony/effect"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/game/stage"
	"github.com/kjkrol/gram/plugins/cameras"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/motion"
	"github.com/kjkrol/gram/plugins/players"
	"github.com/kjkrol/gram/plugins/telemetry"
	"github.com/kjkrol/gram/plugins/vision"
	"github.com/kjkrol/gram/plugins/world"
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
	motion    *motion.Plugin
	vision    *vision.Plugin
	collision *collision.Plugin
	players   *players.Plugin
	telemetry *telemetry.Plugin
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
		Figures(a.defineFigures).
		Players(a.definePlayer).
		Effects(a.defineEffects).
		Rules(a.defineRoles).
		Commands(a.defineCommands).
		Controls(a.bindKeys).
		Spawn(a.spawnUnits).
		Scenes(a.defineScenes).
		Shows(MainScene, telemetry.SceneName).
		Update(a.update)
	gram.Run(game.New(props, mainStage))
}

func (a *arena) makePlugins(w *world.Plugin) {
	a.world = w
	a.motion = motion.NewPlugin(a.world)
	a.vision = vision.NewPlugin(a.world)
	a.collision = collision.NewPlugin(a.world, a.motion).WithStats(&a.hits)
	a.cameras = cameras.NewPlugin(a.world, cameras.Config{})
	a.telemetry = telemetry.NewPlugin(a.world)
	a.players = players.NewPlugin(a.world, a.collision, a.cameras, a.vision)
	a.vision.ShowCones()
}

func (a *arena) definePlayer() error {
	a.player = a.players.Local("player")
	return a.player.Bind(a.players.Defaults()...)
}

// the flight is a state of the whole world
func (a *arena) defineEffects() {
	a.world.Effects().Define(FleeingEf, effect.Spec{})
}

// prey is what the hunter hunts: a role named by its rules, played by the prey
func (a *arena) defineRoles() { a.world.Roles().Define(PreyRole) }

func (a *arena) defineCommands() {
	flee := ceremony.Toggle(a.world.Effects().Named(FleeingEf)).On(ceremony.World)
	a.world.Commands().Define(FleeCmd, flee)
}

func (a *arena) defineFigures() { a.world.Figures(hunter{}, prey{}) }

func (a *arena) bindKeys() error {
	return a.player.Bind(
		control.Give(control.KeyPress{Key: control.KeyA}, "The prey flee, or stop fleeing",
			a.world.Commands().Named(FleeCmd)),
	)
}

func (a *arena) spawnUnits() {
	placement := world.NewGridPlacement(ScreenWidth, ScreenHeight, RectSize)

	const total = PreyCount + 1
	roam := func(i int, speed float64) body {
		angle := rand.Float64() * 2 * math.Pi
		return body{
			pos: placement.Place(i, total),
			vel: world.Velocity{Dir: geom.NewVec(math.Cos(angle), math.Sin(angle)), Value: speed},
		}
	}

	preyKind := kind.Named[body](a.world.Kinds(), PreyKind)
	hunterKind := kind.Named[body](a.world.Kinds(), HunterKind)
	entries := make([]kind.Entry, 0, total)
	for i := range PreyCount {
		entries = append(entries, preyKind.Entry(roam(i, 90)))
	}
	entries = append(entries, hunterKind.Entry(roam(PreyCount, 81)))
	a.world.Seed(entries...)
	a.world.Carrier().Put(a.player.ID, a.world.Commands().Named(FleeCmd))
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

func (a *arena) defineScenes(ctx game.Initializer) []game.Scene {
	m := &mainScene{arena: a}
	a.telemetry.With(a.hits.Reporter(&ctx.TPS().Ticks))
	return []game.Scene{ui.NewScene(MainScene, m.screen()).Input(a.players.Handle)}
}

func (a *arena) update(ctx goke.RunCtx, d time.Duration) {
	a.vision.RunPlan(ctx, d)
	a.world.RunPlan(ctx, d)
	a.motion.RunPlan(ctx, d)
	a.collision.RunPlan(ctx, d)
	a.cameras.RunPlan(ctx, d)
	a.players.RunPlan(ctx, d)
	ctx.Sync()
}
