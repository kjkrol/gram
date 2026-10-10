package main

import (
	"log"
	"math"
	"slices"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/entity/kind/comp"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/game/stage"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/unit"
	"github.com/kjkrol/gram/plugins/cameras"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/driving"
	"github.com/kjkrol/gram/plugins/navigation"
	"github.com/kjkrol/gram/plugins/players"
	"github.com/kjkrol/gram/plugins/selection"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/steering"
	"github.com/kjkrol/gram/ui"
)

const (
	TPS         = 60
	GridWidth   = 20
	GridHeight  = 12
	HexSize     = 24 // circumradius
	EntitySize  = 22
	UnitSpeed   = HexSize * 3
	MaxEntCount = 16

	saveBasePath = "board-navigation-hex-demo"
)

var (
	ScreenWidth = int(
		math.Ceil(HexSize*(math.Sqrt(3)*(GridWidth-1)+math.Sqrt(3)/2*(GridHeight-1)) + 2*HexSize),
	)
	ScreenHeight = int(math.Ceil(HexSize * (1.5*(GridHeight-1) + 2)))
)

type arena struct {
	world     *world.Plugin
	board     *board.Plugin
	nav       *navigation.Plugin
	driving   *driving.Plugin
	collision *collision.Plugin
	selection *selection.Plugin
	players   *players.Plugin
	cameras   *cameras.Plugin
	player    *players.Player
}

func NewDemo(s *arena) game.Game {
	return game.New(game.Props{
		Title:       "gram navigation hex demo",
		ScreenWidth: ScreenWidth, ScreenHeight: ScreenHeight, Resizable: true,
		TargetTPS: TPS,
	},
		stage.New(BoardNavigationHexStage).
			World(world.Config{
				Space: world.SpaceCfg{Width: uint32(ScreenWidth), Height: uint32(ScreenHeight)},
				Entities: world.EntitiesCfg{
					MaxCount: MaxEntCount,
					MinSize:  EntitySize,
					MaxSize:  EntitySize,
				},
			}).
			Plugins(s.makePlugins).
			Players(s.definePlayer).
			Kinds(s.defineCells, s.defineKinds).
			Restore(s.restore).
			Spawn(s.spawnCells, s.spawnUnits).
			Scenes(s.defineScenes).
			Update(s.update))
}

func (s *arena) makePlugins(w *world.Plugin) {
	s.world = w
	grid := grid.DefaultGrids{}.Hex(GridWidth, GridHeight, HexSize)
	s.collision = collision.NewPlugin(s.world)
	s.board = board.NewPlugin(grid, &cell.SingleOccupancy{}, s.world).WithCollision(s.collision)
	s.selection = selection.NewPlugin(s.world)
	s.driving = driving.NewPlugin(s.world, s.selection).WithGround(s.board)
	s.nav = navigation.NewPlugin(s.board, s.world, s.selection, s.driving).
		WithCollision(s.collision)
	s.cameras = cameras.NewPlugin(s.world)
	s.players = players.NewPlugin(s.world, s.cameras, s.board, s.selection, s.nav, s.driving).
		WithSaves(saveBasePath)
}

func (s *arena) definePlayer() error {
	s.player = s.players.Local("player")
	return s.player.Bind(s.players.Defaults()...)
}

func (s *arena) defineScenes() []game.Scene {
	main := &mainScene{arena: s}
	main.keys = players.SceneKeys{
		{
			Key:   control.KeyR,
			Label: "Build a road through the wall",
			Do: func(game.Runtime, game.Composition) {
				brd, road := s.board.Board(), s.board.CellKinds().Named(RoadCell).Kind()
				for q := uint32(3); q < GridWidth-3; q++ {
					brd.Set(brd.At(q, 6), road)
				}
				log.Print("built a road through the wall — " +
					"in-flight units re-path onto it as soon as they deviate")
			},
		},
	}
	s.players.OwnKeys(main.keys)
	return []game.Scene{ui.NewScene(MainScene, main.screen()).Input(s.players.Handle)}
}

func (s *arena) defineCells() {
	kinds := s.board.CellKinds()
	kinds.Define(GrassCell, cell.Kind{Cost: 2, Allows: cell.Land})
	kinds.Define(WallCell, cell.Kind{Cost: 1, Solid: true})
	kinds.Define(RoadCell, cell.Kind{Cost: 1, Allows: cell.Land})
}

// TODO: ponownie widac, ze to jest copy-paste i powinno byc dostarczone
func (s *arena) restore(p game.Persistence) (bool, error) {
	saves, err := p.List(saveBasePath)
	if err != nil {
		return false, err
	}
	if !slices.Contains(saves, "") {
		return false, nil
	}
	if err := p.Load(saveBasePath, ""); err != nil {
		return false, err
	}
	log.Print("loaded saved board")
	return true, nil
}

type unitRow struct{ start, target cell.ID }

func (s *arena) defineKinds() {
	brd := s.board.Board()
	units := board.NewUnits[unitRow](
		s.board,
		board.Shape{Size: EntitySize},
		func(u unitRow) geom.Vec { return brd.CellCenter(u.start) },
	)
	profile := steering.Steering{
		MaxSpeed: UnitSpeed,
		Accel:    UnitSpeed * 2,
		Brake:    UnitSpeed * 4,
		V0:       UnitSpeed / 2,
		TurnRate: 0.15,
	}
	own := []comp.Comp{
		comp.Load(
			func(u unitRow) navigation.MoveOrder { return navigation.MoveOrder{Target: u.target} },
		),
	}
	units.Define(RedKind, unit.Mover{Domain: cell.Land}, profile, own...)
	units.Define(BlueKind, unit.Mover{Domain: cell.Land}, profile, own...)
}

// TODO: tak z pliku...
func (s *arena) spawnCells() {
	brd := s.board.Board()
	// A wall down the q = 10 column from r = 1 to the bottom, and a road round it: along r = 0 and
	// down both flanks (which slant with the rows, as every hex column does).
	var cells []cell.Entry
	for r := uint32(1); r < GridHeight; r++ {
		cells = append(cells, cell.Entry{Kind: WallCell, Cell: brd.At(10, r)})
	}
	for q := uint32(2); q <= GridWidth-3; q++ {
		cells = append(cells, cell.Entry{Kind: RoadCell, Cell: brd.At(q, 0)})
	}
	for r := uint32(1); r < GridHeight; r++ {
		cells = append(cells, cell.Entry{Kind: RoadCell, Cell: brd.At(2, r)})
		cells = append(cells, cell.Entry{Kind: RoadCell, Cell: brd.At(GridWidth-3, r)})
	}
	s.board.Seed(board.Layout{Default: GrassCell, Cells: cells})
}

// TODO: tak z pliku...
func (s *arena) spawnUnits() {
	redKind := kind.Named[unitRow](s.world.Kinds(), RedKind)
	blueKind := kind.Named[unitRow](s.world.Kinds(), BlueKind)
	brd := s.board.Board()
	player := []control.Command{players.Give{To: s.player.ID}, selection.Allow{Selected: true}}
	s.world.Seed(
		redKind.Entry(unitRow{start: brd.At(3, 3), target: brd.At(GridWidth-4, 3)}).Told(player...),
		blueKind.Entry(unitRow{start: brd.At(3, 9), target: brd.At(GridWidth-4, 9)}).
			Told(player...),
	)
}

func (s *arena) update(ctx goke.RunCtx, d time.Duration) {
	s.world.RunPlan(ctx, d)
	s.collision.RunPlan(ctx, d)
	s.board.RunPlan(ctx, d)
	s.nav.RunPlan(ctx, d)
	s.driving.RunPlan(ctx, d)
	s.selection.RunPlan(ctx, d)
	s.cameras.RunPlan(ctx, d)
	s.players.RunPlan(ctx, d)
	ctx.Sync()
}
