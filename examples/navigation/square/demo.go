package main

import (
	"embed"
	"log"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/game/stage"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/cameras"
	"github.com/kjkrol/gram/plugins/driving"
	"github.com/kjkrol/gram/plugins/navigation"
	"github.com/kjkrol/gram/plugins/players"
	"github.com/kjkrol/gram/plugins/selection"
	"github.com/kjkrol/gram/plugins/topography"
	"github.com/kjkrol/gram/plugins/vision"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/ui"
)

const (
	TPS          = 60
	GridWidth    = 24
	GridHeight   = 16
	CellSize     = 32
	ScreenWidth  = GridWidth * CellSize
	ScreenHeight = GridHeight * CellSize
	MaxEntCount  = 32
)

//go:embed map.yaml
var mapFile embed.FS

type arena struct {
	world      *world.Plugin
	board      *board.Plugin
	topography *topography.Plugin
	nav        *navigation.Plugin
	driving    *driving.Plugin
	selection  *selection.Plugin
	players    *players.Plugin
	cameras    *cameras.Plugin
	player     *players.Player
	vision     *vision.Plugin
}

func main() {
	props := game.Props{
		Title: "gram navigation demo",
		Description: "Units selected and sent by a right click along routes that go round the " +
			"wall, sight across the board: walls cut, forests dim, a hawk flies over.",
		ScreenWidth: ScreenWidth, ScreenHeight: ScreenHeight, Resizable: true,
		TargetTPS: TPS,
	}
	a := &arena{}
	mainStage := stage.New(BoardNavigationStage).
		World(world.Config{
			Space: world.SpaceCfg{Width: ScreenWidth, Height: ScreenHeight},
			Entities: world.EntitiesCfg{
				MaxCount: MaxEntCount,
				MinSize:  scoutPx,
				MaxSize:  scoutPx,
			},
			Heights: true,
		}).
		Plugins(a.makePlugins).
		Figures(a.defineFigures).
		Players(a.definePlayer).
		Spawn(a.spawn).
		Scenes(a.defineScenes).
		Update(a.update)
	gram.Run(game.New(props, mainStage))
}

func (a *arena) makePlugins(w *world.Plugin) {
	a.world = w
	grid := grid.DefaultGrids{}.Square(GridWidth, GridHeight, CellSize)
	a.board = board.NewPlugin(grid, &cell.SingleOccupancy{}, a.world)
	a.topography = topography.NewPlugin(a.world, a.board, topography.Config{Cell: CellSize})
	a.selection = selection.NewPlugin(a.world)
	a.driving = driving.NewPlugin(a.world, a.selection).WithGround(a.board)
	a.nav = navigation.NewPlugin(a.board, a.world, a.selection, a.driving)
	a.vision = vision.NewPlugin(a.world).WithBoard(a.board).WithLog(log.Default())
	a.cameras = cameras.NewPlugin(a.world)
	a.players = players.NewPlugin(
		a.world,
		a.cameras,
		a.board,
		a.selection,
		a.nav,
		a.driving,
		a.topography,
		a.vision,
	)
}

func (a *arena) definePlayer() error {
	a.player = a.players.Local("player")
	return a.player.Bind(a.players.Defaults()...)
}

type unitRow struct {
	Start  cell.ID `yaml:"start"`
	Target cell.ID `yaml:"target"`
}

func (a *arena) defineFigures() {
	a.board.Tiles(tiles()...)
	a.board.Figures(
		scout{kind: RedKind, color: redColor},
		scout{kind: BlueKind, color: blueColor},
		scout{kind: YellowKind, color: yellowColor},
		hawk{},
	)
}

func (a *arena) spawn() error {
	level, err := a.board.Lay(mapFile, "map.yaml")
	if err != nil {
		return err
	}
	a.topography.Seed(level.Heights())
	return board.Spawn[unitRow](level, board.Sides{
		PlayerSide:  {players.Give{To: a.player.ID}, selection.Allow{Selected: true}},
		ReserveSide: {players.Give{To: a.player.ID}, selection.Allow{}},
	})
}

func (a *arena) defineScenes() []game.Scene {
	main := &mainScene{arena: a}
	return []game.Scene{ui.NewScene(MainScene, main.screen()).Input(a.players.Handle)}
}

func (a *arena) update(ctx goke.RunCtx, d time.Duration) {
	a.world.RunPlan(ctx, d)
	a.board.RunPlan(ctx, d)
	a.nav.RunPlan(ctx, d)
	a.driving.RunPlan(ctx, d)
	a.vision.RunPlan(ctx, d)
	a.selection.RunPlan(ctx, d)
	a.topography.RunPlan(ctx, d)
	a.cameras.RunPlan(ctx, d)
	a.players.RunPlan(ctx, d)
	ctx.Sync()
}
