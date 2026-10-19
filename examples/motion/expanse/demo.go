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
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/game/stage"
	"github.com/kjkrol/gram/plugins/cameras"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/minimap"
	"github.com/kjkrol/gram/plugins/motion"
	"github.com/kjkrol/gram/plugins/players"
	"github.com/kjkrol/gram/plugins/telemetry"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/sector"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/ui"
)

const (
	TPS          = 60
	ScreenWidth  = 1024
	ScreenHeight = 768

	// a world of about 98 km a side, a unit 6 m
	WorldSide = 1 << 14

	hitDuration = 50 * time.Millisecond
)

const (
	BoxSize     = 5
	EntityCount = 100_000

	entityColors = 6
)

var scale = world.Scale{Metres: 6}

type body struct {
	pos world.Position
	vel world.Velocity
}

func bodyKind(ci int) string { return fmt.Sprintf("box-%d", ci) }

type arena struct {
	world          *world.Plugin
	motion         *motion.Plugin
	collision      *collision.Plugin
	players        *players.Plugin
	telemetry      *telemetry.Plugin
	cameras        *cameras.Plugin
	minimap        *minimap.Plugin
	player         *players.Player
	collisionStats collision.ContactStats
}

func main() {
	props := game.Props{
		Title: "gram expanse",
		Description: "A world of about 98 km a side, a torus, and a hundred thousand boxes " +
			"bouncing about it. The world lives in bands round the camera: near it everything " +
			"moves and collides, farther the boxes move four times as seldom and pass through " +
			"each other, beyond the bands nothing stirs. Shift+G shows the sectors by their " +
			"bands, by what the camera sees, or not at all; Ctrl+G switches the bands off, the " +
			"whole world alive, and on again. The minimap shows 9 km round the camera (Shift+M " +
			"hides it); a click on it takes the camera there. WSAD scroll, = and - zoom, ~ the " +
			"telemetry.",
		ScreenWidth:  ScreenWidth,
		ScreenHeight: ScreenHeight,
		Resizable:    true,
		TargetTPS:    TPS,
	}
	a := &arena{}
	mainStage := stage.New(ExpanseStage).
		World(world.Config{
			Space: world.SpaceCfg{
				Width:  WorldSide,
				Height: WorldSide,
				Edges:  aabbworld.Torus,
			},
			Entities: world.EntitiesCfg{
				MaxCount: EntityCount,
				MinSize:  BoxSize,
				MaxSize:  BoxSize,
			},
			Sectors: world.SectorsCfg{
				Buckets: 8,
				Bands: []sector.Band{
					{Reach: scale.Units(4_500)},
					{Reach: scale.Units(12_000), Pace: 4},
				},
			},
		}).
		Plugins(a.makePlugins).
		Figures(a.defineFigures).
		Players(a.definePlayer).
		Effects(a.defineEffects).
		Rules(a.defineRules).
		Spawn(a.spawnBoxes).
		Scenes(a.defineScenes).
		Shows(MainScene, telemetry.SceneName).
		Update(a.update)
	gram.Run(game.New(props, mainStage))
}

func (a *arena) makePlugins(w *world.Plugin) {
	a.world = w
	a.motion = motion.NewPlugin(a.world)
	a.collision = collision.NewPlugin(a.world, a.motion).WithStats(&a.collisionStats)
	a.cameras = cameras.NewPlugin(a.world, cameras.Config{
		Detail: camera.Detail{
			Sight:  scale.Units(15_000),
			Fog:    scale.Units(3_000),
			Coarse: 0.5,
		},
	})
	a.minimap = minimap.NewPlugin(a.world, nil, minimap.Config{Reach: scale.Units(9_000)})
	a.telemetry = telemetry.NewPlugin(a.world)
	a.players = players.NewPlugin(a.world, a.collision, a.cameras, a.minimap)
	a.world.ShowSectors(world.SectorBands)
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

// box is a body of one colour, flashing while hit
type box struct{ color int }

var _ world.Figure = box{}

func (b box) Define(w *world.Plugin) {
	kind.Define[body](w.Kinds(), bodyKind(b.color), kind.Spec{
		comp.Load(func(b body) world.Position { return b.pos }),
		comp.Load(func(b body) world.Velocity { return b.vel }),
		comp.Const(collision.Collider{}),
		comp.Const(collision.Physics{Restitution: 1}),
		ceremony.Plays(w.Roles().Named(BodyRole)),
	})
}

func (b box) Look(a *world.Atlas, w *world.Plugin) {
	a.Add(kind.Named[body](w.Kinds(), bodyKind(b.color)), BoxSize, render.Solid(palette[b.color])).
		Under(w.Effects().Named(HitEf), render.Solid(hitColor)).
		Styled(minimap.Mark{Color: palette[b.color], Size: 2})
}

func (a *arena) defineFigures() {
	var boxes []world.Figure
	for ci := range entityColors {
		boxes = append(boxes, box{color: ci})
	}
	a.world.Figures(boxes...)
}

func (a *arena) spawnBoxes() {
	placement := world.NewGridPlacement(WorldSide, WorldSide, BoxSize)
	motion := newRandomVelocity(30, 8, 3)
	entries := make([]kind.Entry, EntityCount)
	for i := range entries {
		of := kind.Named[body](a.world.Kinds(), bodyKind(rand.IntN(entityColors)))
		entries[i] = of.Entry(body{pos: placement.Place(i, EntityCount),
			vel: motion.initialVelocity(i)})
	}
	a.world.Seed(entries...)
}

func (a *arena) defineScenes(ctx game.Initializer) []game.Scene {
	m := &mainScene{arena: a}
	a.telemetry.With(a.collisionStats.Reporter(&ctx.TPS().Ticks), a.world.Sectors(),
		a.world.Seen())
	return []game.Scene{ui.NewScene(MainScene, m.screen()).Input(a.players.Handle)}
}

func (a *arena) update(ctx goke.RunCtx, d time.Duration) {
	a.world.RunPlan(ctx, d)
	a.motion.RunPlan(ctx, d)
	a.collision.RunPlan(ctx, d)
	a.cameras.RunPlan(ctx, d)
	a.minimap.RunPlan(ctx, d)
	a.players.RunPlan(ctx, d)
	ctx.Sync()
}
