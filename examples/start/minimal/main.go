package main

import (
	"image/color"
	"math/rand/v2"
	"time"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/game/stage"
	"github.com/kjkrol/gram/plugins/cameras"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/motion"
	"github.com/kjkrol/gram/plugins/players"
	"github.com/kjkrol/gram/plugins/telemetry"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/ui"
)

const (
	screenWidth, screenHeight = 640, 480
	boxSize                   = 12
	boxCount                  = 300
)

type boxRow struct {
	pos world.Position
	vel world.Velocity
}

// box is the one kind: a bouncing square
type box struct{}

var _ world.Figure = box{}

func (box) Define(w *world.Plugin) {
	kind.Define[boxRow](w.Kinds(), BoxKind, kind.Spec{
		comp.Load(func(b boxRow) world.Position { return b.pos }),
		comp.Load(func(b boxRow) world.Velocity { return b.vel }),
		comp.Const(collision.Collider{}),
		comp.Const(collision.Physics{Restitution: 1}),
	})
}

func (box) Look(a *world.Atlas, w *world.Plugin) {
	a.Add(kind.Named[boxRow](w.Kinds(), BoxKind), boxSize, render.Solid(boxColor))
}

const (
	BoxKind    = "box"
	ArenaStage = "arena"
	ViewScene  = "view"
)

type arena struct {
	world     *world.Plugin
	motion    *motion.Plugin
	collision *collision.Plugin
	cameras   *cameras.Plugin
	players   *players.Plugin
	telemetry *telemetry.Plugin
	player    *players.Player
	stats     collision.ContactStats
}

func main() {
	props := game.Props{
		Title:        "gram minimal",
		ScreenWidth:  screenWidth,
		ScreenHeight: screenHeight,
		TargetTPS:    60,
	}
	a := &arena{}
	mainStage := stage.New(ArenaStage).
		World(world.Config{
			Space: world.SpaceCfg{
				Width:  screenWidth,
				Height: screenHeight,
				Edges:  aabbworld.Torus,
			},
			Entities: world.EntitiesCfg{MaxCount: boxCount, MinSize: boxSize, MaxSize: boxSize},
		}).
		Plugins(a.makePlugins).
		Figures(a.defineFigures).
		Players(a.definePlayer).
		Spawn(a.spawnUnits).
		Scenes(a.defineScenes).
		Shows(ViewScene, telemetry.SceneName).
		Update(a.update)
	gram.Run(game.New(props, mainStage))
}

func (a *arena) makePlugins(w *world.Plugin) {
	a.world = w
	a.motion = motion.NewPlugin(a.world)
	a.collision = collision.NewPlugin(a.world, a.motion).WithStats(&a.stats)
	a.cameras = cameras.NewPlugin(a.world, cameras.Config{})
	a.telemetry = telemetry.NewPlugin(a.world)
	a.players = players.NewPlugin(a.world, a.collision, a.cameras)
}

func (a *arena) definePlayer() error {
	a.player = a.players.Local("player")
	return a.player.Bind(a.players.Defaults()...)
}

func (a *arena) defineFigures() { a.world.Figures(box{}) }

func (a *arena) spawnUnits() {
	boxKind := kind.Named[boxRow](a.world.Kinds(), BoxKind)
	rng := rand.New(rand.NewPCG(1, 2))
	placement := world.NewGridPlacement(screenWidth, screenHeight, boxSize)
	entries := make([]kind.Entry, boxCount)
	for i := range entries {
		var vel world.Velocity
		vel.SetDelta(geom.NewVec(rng.Float32()*200-100, rng.Float32()*200-100))
		entries[i] = boxKind.Entry(boxRow{pos: placement.Place(i, boxCount), vel: vel})
	}
	a.world.Write().Seed(entries...)
}

func (a *arena) defineScenes(ctx game.Initializer) []game.Scene {
	m := &mainScene{arena: a}
	a.telemetry.With(a.stats.Reporter(&ctx.TPS().Ticks))
	return []game.Scene{ui.NewScene(ViewScene, m.screen()).Input(a.players.Handle)}
}

func (a *arena) update(ctx goke.RunCtx, d time.Duration) {
	a.world.RunPlan(ctx, d)
	a.motion.RunPlan(ctx, d)
	a.collision.RunPlan(ctx, d)
	a.cameras.RunPlan(ctx, d)
	a.players.RunPlan(ctx, d)
	ctx.Sync()
}

type mainScene struct {
	arena *arena
}

var (
	boxColor        = color.RGBA{R: 90, G: 200, B: 110, A: 255}
	backgroundColor = color.RGBA{R: 30, G: 30, B: 30, A: 255}
)

func (m *mainScene) screen() *ui.Element {
	a := m.arena
	view := render.NewFeed(a.cameras.Write().New(cameras.FromAbove, camera.Config{}),
		a.world.Read(0).Picture())
	return ui.Layers(
		ui.Blank().Fill(backgroundColor),
		ui.Image(view).Input(a.players.Through(a.player)),
	)
}
