package main

import (
	"embed"
	"slices"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/game/stage"
	"github.com/kjkrol/gram/lod"
	"github.com/kjkrol/gram/plugins/cameras"
	"github.com/kjkrol/gram/plugins/light"
	"github.com/kjkrol/gram/plugins/mosaic"
	"github.com/kjkrol/gram/plugins/players"
	"github.com/kjkrol/gram/plugins/telemetry"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/ui"
)

const (
	TPS          = 60
	WorldSide    = 1536
	ScreenWidth  = 1280
	ScreenHeight = 800
	StartZoom    = 0.8
	// a sector's side in the space's buckets
	SectorBuckets = 2
)

type arena struct {
	world   *world.Plugin
	light   *light.Plugin
	ground  *mosaic.Plugin
	cameras *cameras.Plugin
	players *players.Plugin

	player *players.Player
}

func main() {
	props := game.Props{
		Title: "gram look ground",
		Description: "Hills and three roads on a ground laid as a mosaic: from far off, the " +
			"camera all the way out (- and the wheel), the roads are ribbons as on a map and " +
			"the hills a few large faces; drawn in (=) the ground grows finer sector by " +
			"sector, the roads get their lanes, their kerbs standing over them, pavements " +
			"and verges, a crossing and a fork their rounded corners. The cursor in a corner " +
			"turns the camera, at an edge scrolls it; ~ shows the sectors' levels.",
		ScreenWidth: ScreenWidth, ScreenHeight: ScreenHeight, Resizable: true,
		TargetTPS: TPS,
	}
	a := &arena{}
	mainStage := stage.New(LookGroundStage).
		World(world.Config{
			Space:    world.SpaceCfg{Width: WorldSide, Height: WorldSide},
			Entities: world.EntitiesCfg{MaxCount: 4096, MinSize: 1, MaxSize: 16},
			Sectors:  world.SectorsCfg{Buckets: SectorBuckets},
			LOD: lod.Ladder{
				// a metre in 40 pixels is the close scale, in 6 the standard one, under 0.6
				// the outline
				Scales: [lod.Levels - 1]float32{40, 16, 6, 2.5, 0.6, 0},
			},
			Heights: true,
		}).
		Plugins(a.makePlugins).
		Players(a.definePlayer).
		Scenes(a.defineScenes).
		Shows(MainScene, telemetry.SceneName).
		Update(a.update)
	gram.Run(game.New(props, mainStage))
}

//go:embed level
var levelFiles embed.FS

func (a *arena) makePlugins(w *world.Plugin) {
	a.world = w
	land, err := mosaic.Load(levelFiles, "level/level.yaml")
	if err != nil {
		panic(err)
	}
	a.light = light.NewPlugin(a.world, light.Config{})
	a.ground = mosaic.NewPlugin(a.world, land, mosaic.Config{})
	a.cameras = cameras.NewPlugin(a.world, cameras.Config{
		Turn:        true,
		Tilt:        true,
		Perspective: true,
		Cell:        8,
		HeightUnit:  1,
		Cage:        a.ground.Read(0),
	})
	a.players = players.NewPlugin(a.world, a.cameras, a.light)
	a.light.WithDistant(sun)
}

func (a *arena) definePlayer() error {
	a.player = a.players.Local("player")
	view := cameras.Keys{
		Wheel:       true,
		ZoomIn:      control.KeyEqual,
		ZoomOut:     control.KeyMinus,
		CornersTurn: true,
	}
	return a.player.Bind(slices.Concat(
		a.players.DefaultBindings(),
		a.world.DefaultBindings(),
		a.light.DefaultBindings(),
		a.cameras.Bindings(view),
	)...)
}

func (a *arena) defineScenes() []game.Scene {
	main := &mainScene{arena: a}
	return []game.Scene{ui.NewScene(MainScene, main.screen()).
		Input(a.players.Handle).
		Issue(a.players.IssueAs(a.player))}
}

func (a *arena) update(ctx goke.RunCtx, d time.Duration) {
	a.world.RunPlan(ctx, d)
	a.light.RunPlan(ctx, d)
	a.ground.RunPlan(ctx, d)
	a.cameras.RunPlan(ctx, d)
	a.players.RunPlan(ctx, d)
	ctx.Sync()
}
