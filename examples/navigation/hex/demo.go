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

func (a *arena) makePlugins(w *world.Plugin) {
	a.world = w
	grid := grid.DefaultGrids{}.Hex(GridWidth, GridHeight, HexSize)
	a.collision = collision.NewPlugin(a.world)
	a.board = board.NewPlugin(grid, &cell.SingleOccupancy{}, a.world).WithCollision(a.collision)
	a.bridges = bridges.NewPlugin(a.world, a.board, bridges.Config{
		Rules: site.Rules{Longest: 3},
	})
	a.selection = selection.NewPlugin(a.world)
	a.driving = driving.NewPlugin(a.world, a.selection).WithGround(a.board)
	a.nav = navigation.NewPlugin(a.board, a.world, a.selection, a.driving).
		WithCollision(a.collision)
	a.cameras = cameras.NewPlugin(a.world)
	a.players = players.NewPlugin(a.world, a.collision, a.cameras, a.board, a.selection, a.nav, a.driving,
		a.bridges)
}

func (a *arena) definePlayer() error {
	a.player = a.players.Local("player")
	return a.player.Bind(a.players.Defaults()...)
}

func (a *arena) defineCells() {
	kinds := a.board.CellKinds()
	kinds.Define(GrassCell, cell.Kind{Cost: 2, Allows: cell.Land})
	kinds.Define(WaterCell, cell.Kind{Cost: 1, Allows: cell.Water})
	kinds.Define(RoadCell, cell.Kind{Cost: 1, Allows: cell.Land})
}

type unitRow struct {
	Start  cell.ID `yaml:"start"`
	Target cell.ID `yaml:"target"`
}

func (a *arena) defineKinds() {
	brd := a.board.Board()
	units := board.NewUnits[unitRow](
		a.board,
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
	a.bridges.Types().Define(BridgeKind, bridges.Spec{
		Allows: cell.Land,
		Width:  DeckWidth,
		Cost:   1,
		Rail:   cell.Rail{Keeps: cell.Land},
	})
}

func (a *arena) spawn() error {
	level, err := a.board.Lay(mapFile, "map.yaml")
	if err != nil {
		return err
	}
	if err := a.bridges.Spawn(level); err != nil {
		return err
	}
	return board.Spawn[unitRow](level, board.Sides{
		PlayerSide: {players.Give{To: a.player.ID}, selection.Allow{Selected: true}},
	})
}

func (a *arena) defineScenes() []game.Scene {
	main := &mainScene{arena: a}
	main.keys = players.SceneKeys{
		{
			Key:   control.KeyR,
			Label: "Lay a causeway across the river",
			Do: func(game.Runtime, game.Composition) {
				brd, road := a.board.Board(), a.board.CellKinds().Named(RoadCell).Kind()
				for q := uint32(3); q < GridWidth-3; q++ {
					brd.Set(brd.At(q, 2), road)
				}
				log.Print("laid a causeway across the river — " +
					"in-flight units re-path onto it as soon as they deviate")
			},
		},
	}
	a.players.OwnKeys(main.keys)
	return []game.Scene{ui.NewScene(MainScene, main.screen()).
		Input(a.players.Handle).
		Issue(a.players.IssueAs(a.player))}
}

func (a *arena) update(ctx goke.RunCtx, d time.Duration) {
	a.bridges.RunPlan(ctx, d)
	a.world.RunPlan(ctx, d)
	a.collision.RunPlan(ctx, d)
	a.board.RunPlan(ctx, d)
	a.nav.RunPlan(ctx, d)
	a.driving.RunPlan(ctx, d)
	a.selection.RunPlan(ctx, d)
	a.cameras.RunPlan(ctx, d)
	a.players.RunPlan(ctx, d)
	ctx.Sync()
}
