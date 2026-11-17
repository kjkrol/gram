package main

import (
	"embed"
	"errors"
	"slices"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/game/stage"
	"github.com/kjkrol/gram/lod"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/buildings"
	"github.com/kjkrol/gram/plugins/cameras"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/driving"
	"github.com/kjkrol/gram/plugins/light"
	"github.com/kjkrol/gram/plugins/motion"
	"github.com/kjkrol/gram/plugins/players"
	"github.com/kjkrol/gram/plugins/selection"
	"github.com/kjkrol/gram/plugins/telemetry"
	"github.com/kjkrol/gram/plugins/topography"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/ui"
)

const (
	TPS          = 60
	GridWidth    = 32
	GridHeight   = 20
	CellSize     = 32
	WorldWidth   = GridWidth * CellSize
	WorldHeight  = GridHeight * CellSize
	ScreenWidth  = 1024
	ScreenHeight = 640
	MaxEntCount  = 64
	StartZoom    = 4
	// a building's box at the most, a turned hall's, and a sector's side in the space's buckets
	LargestBox    = 72
	SectorBuckets = 1
)

// a cell is 8 m across and everything on it as big as in life
var scale = world.Scale{Metres: 8.0 / CellSize}

// the middle of the lot the game's own cottage stands on, world units
var lastLot = [2]float32{26.5 * CellSize, 12.5 * CellSize}

//go:embed map.yaml
var mapFile embed.FS

type arena struct {
	world      *world.Plugin
	motion     *motion.Plugin
	light      *light.Plugin
	collision  *collision.Plugin
	board      *board.Plugin
	topography *topography.Plugin
	buildings  *buildings.Plugin
	selection  *selection.Plugin
	driving    *driving.Plugin
	cameras    *cameras.Plugin
	players    *players.Plugin

	player *players.Player
}

func main() {
	props := game.Props{
		Title: "gram board buildings",
		Description: "A village street on a slope under a sun circling in a minute and a half: " +
			"cottages under hipped roofs, a street's houses under gabled ones, a hall under a " +
			"flat one with a room on it, a chapel with its tower, each as long, as wide and as " +
			"high as its line of the map says, turned as it says, and one stood by the game's " +
			"own code. The hero walks on WSAD, the camera over it (= and - or the wheel " +
			"zoom, the cursor in a corner turns). Near, the buildings stand whole, their " +
			"windows lit as the " +
			"sun goes down and their shadows swinging round; zoomed out to the whole village " +
			"they are bodies alone, and no longer entities.",
		ScreenWidth: ScreenWidth, ScreenHeight: ScreenHeight, Resizable: true,
		TargetTPS: TPS,
	}
	a := &arena{}
	mainStage := stage.New(BoardBuildingsStage).
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
				// whole from a metre of 14 pixels, bodies alone under 12: the camera drawn all
				// the way out shows a metre in 10
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
	a.buildings = buildings.NewPlugin(a.world, a.board)
	a.selection = selection.NewPlugin(a.world)
	a.driving = driving.NewPlugin(a.world, a.motion, a.selection).WithGround(a.board)
	a.cameras = cameras.NewPlugin(a.world, cameras.Config{
		Turn:       true,
		Cell:       CellSize,
		HeightUnit: 1,
		Cage:       a.topography.Read(0),
	})
	a.players = players.NewPlugin(a.world, a.collision, a.cameras, a.board, a.topography,
		a.selection, a.driving, a.light)
	a.board.Write().ShowGrid(false)
}

func (a *arena) definePlayer() error {
	a.player = a.players.Local("player")
	walk := driving.Compass{
		Up: control.KeyW, Down: control.KeyS, Left: control.KeyA, Right: control.KeyD,
	}
	view := cameras.Keys{
		Wheel:       true,
		ZoomIn:      control.KeyEqual,
		ZoomOut:     control.KeyMinus,
		CornersTurn: true,
	}
	return a.player.Bind(slices.Concat(
		a.players.DefaultBindings(),
		a.world.DefaultBindings(),
		a.collision.DefaultBindings(),
		a.board.DefaultBindings(),
		a.topography.DefaultBindings(),
		a.selection.DefaultBindings(),
		a.light.DefaultBindings(),
		view.Bindings(),
		walk.Bindings(),
	)...)
}

func (a *arena) defineFigures() {
	a.board.Tiles(grass{}, street{}, built{})
	a.board.Surfaces(surface{})
	a.board.Figures(village{}, hero{})
	a.buildings.Figures(homes{})
}

func (a *arena) spawn() error {
	level, err := a.board.Lay(mapFile, "map.yaml")
	if err != nil {
		return err
	}
	a.topography.Write().Seed(level.Heights())
	if err := a.buildings.Write().Spawn(level); err != nil {
		return err
	}
	// one more stood by the game's own code, as a line of the map does
	a.buildings.Write().Stand(CottageKind, buildings.Row{
		Mid:    lastLot,
		Size:   [2]float32{scale.Units(7), scale.Units(5.5)},
		Turn:   200,
		Floors: 2,
	})
	return errors.Join(
		board.Spawn[villageRow](level, nil),
		board.Spawn[heroRow](level, board.Sides{
			PlayerSide: {players.Give{To: a.player.ID}, selection.Allow{}},
		}),
	)
}

func (a *arena) defineScenes() []game.Scene {
	main := &mainScene{arena: a}
	return []game.Scene{ui.NewScene(MainScene, main.screen()).
		Input(a.players.Handle).
		Issue(a.players.IssueAs(a.player))}
}

func (a *arena) update(ctx goke.RunCtx, d time.Duration) {
	a.buildings.RunPlan(ctx, d)
	a.world.RunPlan(ctx, d)
	a.light.WithDistant(sunAt(a.world.Read(0).Clock().Time())) // the game's own sun, of the moment
	a.light.RunPlan(ctx, d)
	a.motion.RunPlan(ctx, d)
	a.collision.RunPlan(ctx, d)
	a.board.RunPlan(ctx, d)
	a.topography.RunPlan(ctx, d)
	a.driving.RunPlan(ctx, d)
	a.selection.RunPlan(ctx, d)
	a.cameras.RunPlan(ctx, d)
	a.players.RunPlan(ctx, d)
	ctx.Sync()
}
