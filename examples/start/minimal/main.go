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

type box struct {
	pos world.Position
	vel world.Velocity
}

const (
	BoxKind    = "box"
	ArenaStage = "arena"
	ViewScene  = "view"
)

type arena struct {
	world     *world.Plugin
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
		Players(a.definePlayer).
		Kinds(a.defineKinds).
		Spawn(a.spawnUnits).
		Scenes(a.defineScenes).
		Shows(ViewScene, telemetry.SceneName).
		Update(a.update)
	gram.Run(game.New(props, mainStage))
}

func (a *arena) makePlugins(w *world.Plugin) {
	a.world = w
	a.collision = collision.NewPlugin(a.world).WithStats(&a.stats)
	a.cameras = cameras.NewPlugin(a.world)
	a.telemetry = telemetry.NewPlugin(a.world)
	a.players = players.NewPlugin(a.world, a.cameras)
}

func (a *arena) definePlayer() error {
	a.player = a.players.Local("player")
	return a.player.Bind(a.players.Defaults()...)
}

func (a *arena) defineKinds() {
	kind.Define[box](a.world.Kinds(), BoxKind, kind.Spec{
		comp.Load(func(b box) world.Position { return b.pos }),
		comp.Load(func(b box) world.Velocity { return b.vel }),
		comp.Const(collision.Collider{}),
		comp.Const(collision.Physics{Restitution: 1}),
	})
}

func (a *arena) spawnUnits() {
	boxKind := kind.Named[box](a.world.Kinds(), BoxKind)
	rng := rand.New(rand.NewPCG(1, 2))
	placement := world.NewGridPlacement(screenWidth, screenHeight, boxSize)
	entries := make([]kind.Entry, boxCount)
	for i := range entries {
		var vel world.Velocity
		vel.SetDelta(geom.NewVec(rng.Float64()*200-100, rng.Float64()*200-100))
		entries[i] = boxKind.Entry(box{pos: placement.Place(i, boxCount), vel: vel})
	}
	a.world.Seed(entries...)
}

func (a *arena) defineScenes(ctx game.Initializer) []game.Scene {
	m := &mainScene{arena: a}
	a.telemetry.With(a.stats.Reporter(&ctx.TPS().Ticks))
	return []game.Scene{ui.NewScene(ViewScene, m.screen()).Input(a.players.Handle)}
}

func (a *arena) update(ctx goke.RunCtx, d time.Duration) {
	a.world.RunPlan(ctx, d)
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
	view := render.NewFeed(a.cameras.New(cameras.TopDown(), camera.Config{}), m.picture())
	return ui.Layers(
		ui.Blank().Fill(backgroundColor),
		ui.Image(view).Input(a.players.Through(a.player)),
	)
}

func (m *mainScene) picture() render.Picture {
	a := m.arena
	boxKind := kind.Named[box](a.world.Kinds(), BoxKind)
	atlas := render.NewAtlas()
	atlas.Add(boxKind, boxSize, render.Solid(boxColor))
	atlas.Close()
	a.world.WithRenderer(atlas)
	return render.NewComposer(a.world.Renderer())
}
