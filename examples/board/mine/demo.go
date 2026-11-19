package main

import (
	"embed"
	"slices"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/game/stage"
	"github.com/kjkrol/gram/lod"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/cameras"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/driving"
	"github.com/kjkrol/gram/plugins/interiors"
	"github.com/kjkrol/gram/plugins/light"
	"github.com/kjkrol/gram/plugins/motion"
	"github.com/kjkrol/gram/plugins/players"
	"github.com/kjkrol/gram/plugins/selection"
	"github.com/kjkrol/gram/plugins/telemetry"
	"github.com/kjkrol/gram/plugins/topography"
	"github.com/kjkrol/gram/plugins/underground"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/ui"
)

const (
	TPS          = 60
	GridWidth    = 20
	GridHeight   = 14
	CellSize     = 32
	WorldWidth   = GridWidth * CellSize
	WorldHeight  = GridHeight * CellSize
	ScreenWidth  = 1024
	ScreenHeight = 640
	MaxEntCount  = 64
	StartZoom    = 3
	// a box at the most, the shaft's whole, and a sector's side in the space's buckets
	LargestBox    = 224
	SectorBuckets = 1
)

// a cell is 4 m across and everything on it as big as in life
var scale = world.Scale{Metres: 4.0 / CellSize}

//go:embed map.yaml
var mapFile embed.FS

type arena struct {
	world      *world.Plugin
	motion     *motion.Plugin
	light      *light.Plugin
	collision  *collision.Plugin
	board      *board.Plugin
	topography *topography.Plugin
	mine       *underground.Plugin
	interiors  *interiors.Plugin
	selection  *selection.Plugin
	driving    *driving.Plugin
	cameras    *cameras.Plugin
	players    *players.Plugin

	player *players.Player
}

func main() {
	props := game.Props{
		Title: "gram board mine",
		Description: "A mine bitten into a hillock: from its mouth at the hill's foot a " +
			"timbered shaft runs under the slope and down below the plain into a cave beneath " +
			"the hilltop. The miner walks on WSAD along the track and in by the mouth, lit " +
			"inside by the mine's lamps as the shaft is; from above the shaft's course and " +
			"whoever is in it show as marks over the ground, and V rides the miner through it, " +
			"behind him and in him. Shift+T digs another shaft.",
		ScreenWidth: ScreenWidth, ScreenHeight: ScreenHeight, Resizable: true,
		TargetTPS: TPS,
	}
	a := &arena{}
	mainStage := stage.New(BoardMineStage).
		World(world.Config{
			Space: world.SpaceCfg{Width: WorldWidth, Height: WorldHeight},
			Entities: world.EntitiesCfg{
				MaxCount: MaxEntCount,
				MinSize:  1,
				MaxSize:  LargestBox,
			},
			Scale:   scale,
			Sectors: world.SectorsCfg{Buckets: SectorBuckets},
			LOD: lod.Ladder{
				// whole from a metre of 14 pixels, coarse under 12
				Scales: [lod.Levels - 1]float32{0, 0, 3.4, 3, 0.1, 0},
				Fold:   1,
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
	a.light = light.NewPlugin(a.world, light.Config{})
	a.collision = collision.NewPlugin(a.world, a.motion)
	grid := grid.DefaultGrids{}.Square(GridWidth, GridHeight, CellSize)
	a.board = board.NewPlugin(grid, &cell.SingleOccupancy{}, a.world).
		WithMotion(a.motion, cell.DefaultHiking).
		WithCollision(a.collision)
	a.topography = topography.NewPlugin(a.world, a.board, a.light, topography.Config{})
	a.mine = underground.NewPlugin(a.world, a.board, underground.Config{Rules: digging}).
		WithShaper(a.topography.Write())
	a.interiors = interiors.NewPlugin(a.world, a.board)
	a.selection = selection.NewPlugin(a.world)
	a.driving = driving.NewPlugin(a.world, a.motion, a.selection).WithGround(a.board)
	a.cameras = cameras.NewPlugin(a.world, cameras.Config{
		Turn:        true,
		Tilt:        true,
		Perspective: true,
		Cell:        CellSize,
		HeightUnit:  1,
		Cage:        a.topography.Read(0),
	})
	a.players = players.NewPlugin(a.world, a.collision, a.cameras, a.board, a.topography,
		a.mine, a.selection, a.driving, a.light)
	a.board.Write().ShowGrid(false)
}

func (a *arena) definePlayer() error {
	a.player = a.players.Local("player")
	walk := driving.Compass{
		Up: control.KeyW, Down: control.KeyS, Left: control.KeyA, Right: control.KeyD,
		In: camera.Outside,
	}
	// the views' keys, Tab to the perspective among them, the keys walking the miner in place
	// of those scrolling the camera
	view := cameras.Keys{
		Wheel:       true,
		ZoomIn:      control.KeyEqual,
		ZoomOut:     control.KeyMinus,
		CornersTurn: true,
	}
	inside := driving.Shooter{
		Ahead: control.KeyW, Back: control.KeyS, Left: control.KeyA, Right: control.KeyD,
		In: camera.Inside,
	}
	return a.player.Bind(slices.Concat(
		a.players.DefaultBindings(),
		a.world.DefaultBindings(),
		a.collision.DefaultBindings(),
		a.board.DefaultBindings(),
		a.topography.DefaultBindings(),
		a.mine.DefaultBindings(),
		a.selection.DefaultBindings(),
		a.light.DefaultBindings(),
		a.cameras.Bindings(view),
		walk.Bindings(),
		inside.Bindings(),
	)...)
}

func (a *arena) defineFigures() {
	a.board.Tiles(grass{}, scree{}, track{})
	a.board.Surfaces(surface{})
	a.board.Figures(miner{})
	a.mine.Figures(mine{})
}

func (a *arena) spawn() error {
	level, err := a.board.Lay(mapFile, "map.yaml")
	if err != nil {
		return err
	}
	a.topography.Write().Seed(level.Heights())
	if err := a.mine.Write().Spawn(level); err != nil {
		return err
	}
	return board.Spawn[minerRow](level, board.Sides{
		PlayerSide: {players.Give{To: a.player.ID}, selection.Allow{}},
	})
}

func (a *arena) defineScenes() []game.Scene {
	main := &mainScene{arena: a}
	return []game.Scene{ui.NewScene(MainScene, main.screen()).
		Input(a.players.Handle).
		Issue(a.players.IssueAs(a.player))}
}

func (a *arena) update(ctx goke.RunCtx, d time.Duration) {
	a.mine.RunPlan(ctx, d)
	a.world.RunPlan(ctx, d)
	a.light.RunPlan(ctx, d)
	a.motion.RunPlan(ctx, d)
	a.collision.RunPlan(ctx, d)
	a.board.RunPlan(ctx, d)
	a.interiors.RunPlan(ctx, d)
	a.topography.RunPlan(ctx, d)
	a.driving.RunPlan(ctx, d)
	a.selection.RunPlan(ctx, d)
	a.cameras.RunPlan(ctx, d)
	a.players.RunPlan(ctx, d)
	ctx.Sync()
	a.world.Write().Clock().Replay(ctx, d)
}
