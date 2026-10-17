package main

import (
	"embed"
	"log"
	"math"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/game/stage"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/bridges"
	"github.com/kjkrol/gram/plugins/bridges/site"
	"github.com/kjkrol/gram/plugins/cameras"
	"github.com/kjkrol/gram/plugins/driving"
	"github.com/kjkrol/gram/plugins/motion"
	"github.com/kjkrol/gram/plugins/navigation"
	"github.com/kjkrol/gram/plugins/players"
	"github.com/kjkrol/gram/plugins/selection"
	"github.com/kjkrol/gram/plugins/topography"
	"github.com/kjkrol/gram/plugins/vision"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/ui"
)

const (
	TPS         = 60
	GridWidth   = 20
	GridHeight  = 12
	HexSize     = 24
	MaxEntCount = 32
)

var (
	ScreenWidth = int(
		math.Ceil(HexSize*(math.Sqrt(3)*(GridWidth-1)+math.Sqrt(3)/2*(GridHeight-1)) + 2*HexSize),
	)
	ScreenHeight = int(math.Ceil(HexSize * (1.5*(GridHeight-1) + 2)))
)

//go:embed map.yaml
var mapFile embed.FS

type arena struct {
	world      *world.Plugin
	motion     *motion.Plugin
	board      *board.Plugin
	topography *topography.Plugin
	bridges    *bridges.Plugin
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
		Title: "gram navigation hex demo",
		Description: "Units selected and sent by a right click across a hex board, through " +
			"the wall's gap and over the bridge; the wagon, too wide for the bridge, goes round " +
			"by the ford. Sight: walls cut, forests dim, a hawk flies over.",
		ScreenWidth: ScreenWidth, ScreenHeight: ScreenHeight, Resizable: true,
		TargetTPS: TPS,
	}
	a := &arena{}
	mainStage := stage.New(BoardNavigationHexStage).
		World(world.Config{
			Space: world.SpaceCfg{Width: uint32(ScreenWidth), Height: uint32(ScreenHeight)},
			Entities: world.EntitiesCfg{
				MaxCount: MaxEntCount,
				MinSize:  scoutPx,
				MaxSize:  HexSize, // a bridge's deck spans a cell
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
	a.motion = motion.NewPlugin(a.world)
	grid := grid.DefaultGrids{}.Hex(GridWidth, GridHeight, HexSize)
	a.board = board.NewPlugin(grid, &cell.SingleOccupancy{}, a.world).WithMotion(a.motion)
	a.topography = topography.NewPlugin(a.world, a.board, topography.Config{})
	a.bridges = bridges.NewPlugin(a.world, a.board, bridges.Config{Rules: site.Rules{Longest: 3}})
	a.selection = selection.NewPlugin(a.world)
	a.driving = driving.NewPlugin(a.world, a.motion, a.selection).WithGround(a.board)
	a.nav = navigation.NewPlugin(a.board, a.world, a.motion, a.selection, a.driving)
	a.vision = vision.NewPlugin(a.world).WithBoard(a.board).WithLog(log.Default())
	a.cameras = cameras.NewPlugin(a.world, cameras.Config{
		Turn: true,
		Tilt: true,
		Cell: HexSize,
		Cage: a.topography,
	})
	a.players = players.NewPlugin(
		a.world,
		a.cameras,
		a.board,
		a.selection,
		a.nav,
		a.driving,
		a.topography,
		a.vision,
		a.bridges,
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
	a.board.Surfaces(surface{})
	a.board.Figures(
		scout{kind: RedKind, color: redColor},
		scout{kind: BlueKind, color: blueColor},
		scout{kind: YellowKind, color: yellowColor},
		hawk{},
		wagon{},
	)
	a.bridges.Figures(bridge{})
}

func (a *arena) spawn() error {
	level, err := a.board.Lay(mapFile, "map.yaml")
	if err != nil {
		return err
	}
	a.topography.Seed(level.Heights())
	if err := a.bridges.Spawn(level); err != nil {
		return err
	}
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
	a.bridges.RunPlan(ctx, d)
	a.world.RunPlan(ctx, d)
	a.motion.RunPlan(ctx, d)
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
