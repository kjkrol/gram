package main

import (
	"embed"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/game/stage"
	"github.com/kjkrol/gram/plugins/atmosphere"
	"github.com/kjkrol/gram/plugins/atmosphere/calendar"
	"github.com/kjkrol/gram/plugins/atmosphere/climate"
	"github.com/kjkrol/gram/plugins/atmosphere/climate/weather"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/cameras"
	"github.com/kjkrol/gram/plugins/players"
	"github.com/kjkrol/gram/plugins/telemetry"
	"github.com/kjkrol/gram/plugins/topography"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/ui"
)

const (
	TPS          = 60
	GridWidth    = 112
	GridHeight   = 84
	CellSize     = 32
	WorldWidth   = GridWidth * CellSize
	WorldHeight  = GridHeight * CellSize
	ScreenWidth  = 1024
	ScreenHeight = 768
)

//go:embed map.yaml
var mapFile embed.FS

// A cell is 100 m of the valley.
var scale = world.Scale{Metres: 100.0 / CellSize}

type arena struct {
	world      *world.Plugin
	board      *board.Plugin
	topography *topography.Plugin
	atmosphere *atmosphere.Plugin
	cameras    *cameras.Plugin
	telemetry  *telemetry.Plugin
	players    *players.Plugin
	player     *players.Player
}

func main() {
	props := game.Props{
		Title: "gram look sky",
		Description: "A sky of the game's own over a valley, a day in a minute and a half: the " +
			"vault turquoise, violet at night with a band shimmering across it, a big orange " +
			"moon showing by day too, made-up stars. Tab turns to the perspective, Q and E " +
			"turn it, F held lays the stick down to the sky.",
		ScreenWidth: ScreenWidth, ScreenHeight: ScreenHeight, Resizable: true,
		TargetTPS: TPS,
	}
	a := &arena{}
	mainStage := stage.New(LookSkyStage).
		World(world.Config{
			Scale: scale,
			Space: world.SpaceCfg{Width: WorldWidth, Height: WorldHeight},
			Entities: world.EntitiesCfg{
				MaxCount: 1,
				MinSize:  1,
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
	grid := grid.DefaultGrids{}.Square(GridWidth, GridHeight, CellSize)
	a.board = board.NewPlugin(grid, &cell.SingleOccupancy{}, a.world)
	a.topography = topography.NewPlugin(a.world, a.board, topography.Config{})
	a.atmosphere = atmosphere.NewPlugin(a.world, atmosphere.Config{
		Calendar: calendar.Config{
			DayRealTime: 90 * time.Second,
			Start:       17 * time.Hour,
			// the moon past its first quarter, up by day
			Season: calendar.Spring,
		},
		Climate: climate.Config{Start: weather.Clear},
		Running: &atmosphere.Running{Day: true, Stars: true, Moon: true},
	}).Over(a.board)
	a.cameras = cameras.NewPlugin(a.world, cameras.Config{
		Turn:        true,
		Tilt:        true,
		Perspective: true,
		MinPitch:    2,
		Cell:        CellSize,
		HeightUnit:  1,
		Cage:        a.topography,
	})
	a.telemetry = telemetry.NewPlugin(a.world).With(a.atmosphere.Reporter())
	a.players = players.NewPlugin(a.world, a.cameras, a.board, a.atmosphere, a.topography)
}

func (a *arena) defineFigures() {
	a.board.Tiles(tiles()...)
	a.board.Surfaces(surface{})
	a.atmosphere.Skies(dusk{})
}

func (a *arena) definePlayer() error {
	a.player = a.players.Local("player")
	return a.player.Bind(a.players.Defaults()...)
}

func (a *arena) spawn() error {
	level, err := a.board.Lay(mapFile, "map.yaml")
	if err != nil {
		return err
	}
	a.topography.Seed(level.Heights())
	return nil
}

func (a *arena) defineScenes(ctx game.Initializer) []game.Scene {
	main := &mainScene{arena: a}
	return []game.Scene{ui.NewScene(MainScene, main.screen()).
		Input(a.players.Handle).
		Issue(a.players.IssueAs(a.player))}
}

func (a *arena) update(ctx goke.RunCtx, d time.Duration) {
	a.world.RunPlan(ctx, d)
	a.board.RunPlan(ctx, d)
	a.atmosphere.RunPlan(ctx, d)
	a.topography.RunPlan(ctx, d)
	a.cameras.RunPlan(ctx, d)
	a.players.RunPlan(ctx, d)
	ctx.Sync()
}
