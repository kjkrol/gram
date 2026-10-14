package main

import (
	"embed"
	"log"
	"math"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/game/stage"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/unit"
	"github.com/kjkrol/gram/plugins/bridges"
	"github.com/kjkrol/gram/plugins/bridges/site"
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
	EntitySize  = 14
	DeckWidth   = 23
	UnitSpeed   = HexSize * 3
	MaxEntCount = 16
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
	world     *world.Plugin
	board     *board.Plugin
	bridges   *bridges.Plugin
	nav       *navigation.Plugin
	driving   *driving.Plugin
	collision *collision.Plugin
	selection *selection.Plugin
	players   *players.Plugin
	cameras   *cameras.Plugin
	player    *players.Player
}

func main() {
	props := game.Props{
		Title:       "gram navigation hex demo",
		ScreenWidth: ScreenWidth, ScreenHeight: ScreenHeight, Resizable: true,
		TargetTPS: TPS,
	}
	a := &arena{}
	mainStage := stage.New(BoardNavigationHexStage).
		World(world.Config{
			Space: world.SpaceCfg{Width: uint32(ScreenWidth), Height: uint32(ScreenHeight)},
			Entities: world.EntitiesCfg{
				MaxCount: MaxEntCount,
				MinSize:  EntitySize,
				MaxSize:  2 * HexSize,
			},
		}).
		Plugins(a.makePlugins).
		Players(a.definePlayer).
		Kinds(a.defineCells, a.defineKinds).
		Spawn(a.spawn).
		Scenes(a.defineScenes).
		Update(a.update)
	gram.Run(game.New(props, mainStage))
}

func (s *arena) makePlugins(w *world.Plugin) {
	s.world = w
	grid := grid.DefaultGrids{}.Hex(GridWidth, GridHeight, HexSize)
	s.collision = collision.NewPlugin(s.world)
	s.board = board.NewPlugin(grid, &cell.SingleOccupancy{}, s.world).WithCollision(s.collision)
	s.bridges = bridges.NewPlugin(s.world, s.board, bridges.Config{
		Rules: site.Rules{Longest: 3},
	})
	s.selection = selection.NewPlugin(s.world)
	s.driving = driving.NewPlugin(s.world, s.selection).WithGround(s.board)
	s.nav = navigation.NewPlugin(s.board, s.world, s.selection, s.driving).
		WithCollision(s.collision)
	s.cameras = cameras.NewPlugin(s.world)
	s.players = players.NewPlugin(s.world, s.collision, s.cameras, s.board, s.selection, s.nav, s.driving,
		s.bridges)
}

func (s *arena) definePlayer() error {
	s.player = s.players.Local("player")
	return s.player.Bind(s.players.Defaults()...)
}

func (s *arena) defineCells() {
	kinds := s.board.CellKinds()
	kinds.Define(GrassCell, cell.Kind{Cost: 2, Allows: cell.Land})
	kinds.Define(WaterCell, cell.Kind{Cost: 1, Allows: cell.Water})
	kinds.Define(RoadCell, cell.Kind{Cost: 1, Allows: cell.Land})
}

type unitRow struct {
	Start  cell.ID `yaml:"start"`
	Target cell.ID `yaml:"target"`
}

func (s *arena) defineKinds() {
	brd := s.board.Board()
	units := board.NewUnits[unitRow](
		s.board,
		board.Shape{Size: EntitySize},
		func(u unitRow) geom.Vec { return brd.CellCenter(u.Start) },
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
			func(u unitRow) navigation.MoveOrder { return navigation.MoveOrder{Target: u.Target} },
		),
	}
	units.Define(RedKind, unit.Mover{Domain: cell.Land}, profile, own...)
	units.Define(BlueKind, unit.Mover{Domain: cell.Land}, profile, own...)
	s.bridges.Types().Define(BridgeKind, bridges.Spec{
		Allows: cell.Land,
		Width:  DeckWidth,
		Cost:   1,
		Rail:   cell.Rail{Keeps: cell.Land},
	})
}

func (s *arena) spawn() error {
	level, err := s.board.Lay(mapFile, "map.yaml")
	if err != nil {
		return err
	}
	if err := s.bridges.Spawn(level); err != nil {
		return err
	}
	return board.Spawn[unitRow](level, board.Sides{
		PlayerSide: {players.Give{To: s.player.ID}, selection.Allow{Selected: true}},
	})
}

func (s *arena) defineScenes() []game.Scene {
	main := &mainScene{arena: s}
	main.keys = players.SceneKeys{
		{
			Key:   control.KeyR,
			Label: "Lay a causeway across the river",
			Do: func(game.Runtime, game.Composition) {
				brd, road := s.board.Board(), s.board.CellKinds().Named(RoadCell).Kind()
				for q := uint32(3); q < GridWidth-3; q++ {
					brd.Set(brd.At(q, 2), road)
				}
				log.Print("laid a causeway across the river — " +
					"in-flight units re-path onto it as soon as they deviate")
			},
		},
	}
	s.players.OwnKeys(main.keys)
	return []game.Scene{ui.NewScene(MainScene, main.screen()).
		Input(s.players.Handle).
		Issue(s.players.IssueAs(s.player))}
}

func (s *arena) update(ctx goke.RunCtx, d time.Duration) {
	s.bridges.RunPlan(ctx, d)
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
