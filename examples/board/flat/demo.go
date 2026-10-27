package main

import (
	"embed"
	"log"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/game/stage"
	"github.com/kjkrol/gram/plugins/atmosphere"
	"github.com/kjkrol/gram/plugins/atmosphere/calendar"
	"github.com/kjkrol/gram/plugins/atmosphere/climate"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/bridges"
	"github.com/kjkrol/gram/plugins/bridges/site"
	"github.com/kjkrol/gram/plugins/cameras"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/driving"
	"github.com/kjkrol/gram/plugins/light"
	"github.com/kjkrol/gram/plugins/minimap"
	"github.com/kjkrol/gram/plugins/motion"
	"github.com/kjkrol/gram/plugins/navigation"
	"github.com/kjkrol/gram/plugins/players"
	"github.com/kjkrol/gram/plugins/selection"
	"github.com/kjkrol/gram/plugins/telemetry"
	"github.com/kjkrol/gram/plugins/vision"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/ui"
)

const (
	TPS          = 60
	GridWidth    = 204
	GridHeight   = 106
	CellSize     = 50
	WorldWidth   = GridWidth * CellSize
	WorldHeight  = GridHeight * CellSize
	ScreenWidth  = 1024
	ScreenHeight = 768
	MaxEntCount  = 128
)

//go:embed map.yaml
var mapFile embed.FS

type arena struct {
	world      *world.Plugin
	motion     *motion.Plugin
	board      *board.Plugin
	bridges    *bridges.Plugin
	nav        *navigation.Plugin
	driving    *driving.Plugin
	collision  *collision.Plugin
	selection  *selection.Plugin
	players    *players.Plugin
	telemetry  *telemetry.Plugin
	cameras    *cameras.Plugin
	minimap    *minimap.Plugin
	light      *light.Plugin
	player     *players.Player
	vision     *vision.Plugin
	atmosphere *atmosphere.Plugin
}

func main() {
	props := game.Props{
		Title: "gram board",
		Description: "Madeira on the simple map, a minimap round the camera; a click on it " +
			"takes the camera there.",
		ScreenWidth: ScreenWidth, ScreenHeight: ScreenHeight, Resizable: true,
		TargetTPS: TPS,
	}
	a := &arena{}
	mainStage := stage.New(BoardStage).
		World(world.Config{
			Space: world.SpaceCfg{Width: WorldWidth, Height: WorldHeight},
			Entities: world.EntitiesCfg{
				MaxCount: MaxEntCount,
				MinSize:  walkerPx,
				MaxSize:  CellSize,
			},
			Heights: true,
		}).
		Plugins(a.makePlugins).
		Figures(a.defineFigures).
		Players(a.definePlayer).
		Spawn(a.spawn).
		Scenes(a.defineScenes).
		Shows(MainScene, telemetry.SceneName).
		Update(a.update)
	gram.Run(game.New(props, mainStage))
}

func (a *arena) makePlugins(w *world.Plugin) {
	a.world = w
	a.motion = motion.NewPlugin(a.world)
	a.collision = collision.NewPlugin(a.world, a.motion)
	a.light = light.NewPlugin(a.world, light.Config{LampShadows: 2})
	grid := grid.DefaultGrids{}.Square(GridWidth, GridHeight, CellSize)
	a.board = board.NewPlugin(grid, &cell.SingleOccupancy{}, a.world).
		WithMotion(a.motion).
		WithCollision(a.collision).
		WithLog(log.Default())
	a.bridges = bridges.NewPlugin(a.world, a.board, bridges.Config{
		Rules: site.Rules{Longest: 3},
	})
	a.selection = selection.NewPlugin(a.world)
	a.driving = driving.NewPlugin(a.world, a.motion, a.selection).WithGround(a.board)
	a.nav = navigation.NewPlugin(a.board, a.world, a.motion, a.selection, a.driving).
		WithCollision(a.collision)
	a.vision = vision.NewPlugin(a.world).WithBoard(a.board)
	a.atmosphere = atmosphere.NewPlugin(
		a.world,
		a.light,
		atmosphere.Config{
			Calendar: calendar.Config{Start: 8 * time.Hour, Season: calendar.Summer},
			Climate: climate.Config{
				Zone:     climate.Temperate,
				Seed:     uint64(time.Now().UnixNano()),
				OneLayer: true,
			},
		},
	).WithWeathering(a.board, a.defineClimate())
	a.cameras = cameras.NewPlugin(a.world, cameras.Config{Grab: true})
	a.minimap = minimap.NewPlugin(a.world, a.board.Write(), minimap.Config{Reach: 40 * CellSize})
	a.telemetry = telemetry.NewPlugin(a.world).With(a.atmosphere.Read(0).Reporter(),
		a.light.Read(0).Reporter())
	a.players = players.NewPlugin(a.world, a.collision, a.cameras, a.board, a.selection,
		a.nav, a.driving,
		a.atmosphere, a.vision, a.bridges, a.minimap, a.light)
	a.atmosphere.Write().ShowClouds(atmosphere.AllBodies)
	a.atmosphere.CastClouds(atmosphere.CloudShadows{Ground: true, Figures: true, High: true})
	a.board.Write().ShowGrid(false)
}

func (a *arena) definePlayer() error {
	a.player = a.players.Local("player")
	return a.player.Bind(a.players.Defaults()...)
}

func (a *arena) defineFigures() {
	a.board.Tiles(tiles()...)
	a.board.Surfaces(surface{})
	a.board.Figures(walker{}, ford{})
	a.bridges.Figures(bridge{})
	a.atmosphere.Skies(earth{})
}

func (a *arena) spawn() error {
	level, err := a.board.Lay(mapFile, "map.yaml")
	if err != nil {
		return err
	}
	if err := board.Spawn[fordRow](level, nil); err != nil {
		return err
	}
	if err := a.bridges.Write().Spawn(level); err != nil {
		return err
	}
	return board.Spawn[unitRow](level, board.Sides{
		PlayerSide: {players.Give{To: a.player.ID}, selection.Allow{}},
	})
}

func (a *arena) defineScenes(ctx game.Initializer) []game.Scene {
	main := &mainScene{arena: a}
	return []game.Scene{ui.NewScene(MainScene, main.screen()).
		Input(a.players.Handle).
		Issue(a.players.IssueAs(a.player))}
}

func (a *arena) update(ctx goke.RunCtx, d time.Duration) {
	a.bridges.RunPlan(ctx, d)
	a.world.RunPlan(ctx, d)
	a.light.RunPlan(ctx, d)
	a.motion.RunPlan(ctx, d)
	a.collision.RunPlan(ctx, d)
	a.board.RunPlan(ctx, d)
	a.nav.RunPlan(ctx, d)
	a.driving.RunPlan(ctx, d)
	a.vision.RunPlan(ctx, d)
	a.atmosphere.RunPlan(ctx, d)
	a.selection.RunPlan(ctx, d)
	a.cameras.RunPlan(ctx, d)
	a.minimap.RunPlan(ctx, d)
	a.players.RunPlan(ctx, d)
	ctx.Sync()
}
