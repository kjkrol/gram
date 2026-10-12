package main

import (
	"embed"
	"log"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/entity/kind"
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
	"github.com/kjkrol/uid"
)

const (
	TPS          = 60
	GridWidth    = 24
	GridHeight   = 16
	CellSize     = 32
	ScreenWidth  = GridWidth * CellSize
	ScreenHeight = GridHeight * CellSize
	EntitySize   = 22
	UnitSpeed    = CellSize * 2
	MaxEntCount  = 64
)

//go:embed map.yaml
var mapFile embed.FS

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

func main() {
	props := game.Props{
		Title: "gram navigation spawn demo",
		Description: "Walkers spawned in the game hold their cells at once: N spawns one " +
			"under the cursor, Shift+O shows the cells held.",
		ScreenWidth:  ScreenWidth,
		ScreenHeight: ScreenHeight,
		Resizable:    true,
		TargetTPS:    TPS,
	}
	a := &arena{}
	mainStage := stage.New(NavigationSpawnStage).
		World(world.Config{
			Space: world.SpaceCfg{Width: ScreenWidth, Height: ScreenHeight},
			Entities: world.EntitiesCfg{
				MaxCount: MaxEntCount,
				MinSize:  EntitySize,
				MaxSize:  EntitySize,
			},
		}).
		Plugins(a.makePlugins).
		Players(a.definePlayer).
		Kinds(a.defineCells, a.defineKinds).
		Controls(a.bindKeys).
		Spawn(a.spawn).
		Scenes(a.defineScenes).
		Update(a.update)
	gram.Run(game.New(props, mainStage))
}

func (s *arena) makePlugins(w *world.Plugin) {
	s.world = w
	grid := grid.DefaultGrids{}.Square(GridWidth, GridHeight, CellSize)
	s.collision = collision.NewPlugin(s.world)
	s.board = board.NewPlugin(grid, &cell.SingleOccupancy{}, s.world).
		WithCollision(s.collision).
		WithLog(log.Default())
	s.selection = selection.NewPlugin(s.world)
	s.driving = driving.NewPlugin(s.world, s.selection).WithGround(s.board)
	s.nav = navigation.NewPlugin(s.board, s.world, s.selection, s.driving).
		WithCollision(s.collision)
	s.cameras = cameras.NewPlugin(s.world)
	s.players = players.NewPlugin(s.world, s.cameras, s.board, s.selection, s.nav, s.driving)
}

func (s *arena) definePlayer() error {
	s.player = s.players.Local("player")
	return s.player.Bind(s.players.Defaults()...)
}

func (s *arena) defineCells() {
	kinds := s.board.CellKinds()
	kinds.Define(GrassCell, cell.Kind{Cost: 1, Allows: cell.Land})
	kinds.Define(WallCell, cell.Kind{Cost: 1, Solid: true})
}

type walkerRow struct {
	Start cell.ID `yaml:"start"`
}

func (s *arena) defineKinds() {
	brd := s.board.Board()
	units := board.NewUnits[walkerRow](
		s.board,
		board.Shape{Size: EntitySize},
		func(w walkerRow) geom.Vec { return brd.CellCenter(w.Start) },
	)
	units.Define(WalkerKind, unit.Mover{Domain: cell.Land}, steering.Steering{
		MaxSpeed: UnitSpeed,
		Accel:    UnitSpeed * 2,
		Brake:    UnitSpeed * 4,
		V0:       UnitSpeed / 2,
		TurnRate: 0.15,
	})
}

func (s *arena) bindKeys() error {
	return s.player.Bind(control.Build(
		control.KeyPress{Key: control.KeyN},
		"Spawn a walker under the cursor",
		s.spawnUnderCursor,
	))
}

// spawnUnderCursor is a walker of the player's for the cell under the cursor, if the cell takes a
// walker and nobody holds it.
func (s *arena) spawnUnderCursor(c control.Context) (world.Spawn, bool) {
	brd := s.board.Board()
	at, ok := brd.CellAt(c.World(c.Cursor))
	if !ok || !brd.Kind(at).Admits(cell.Land) {
		return world.Spawn{}, false
	}
	held := false
	s.board.Occupancy().Holders(at, func(uid.UID64, cell.Domain, bool) { held = true })
	if held {
		return world.Spawn{}, false
	}
	walker := kind.Named[walkerRow](s.world.Kinds(), WalkerKind).Entry(walkerRow{Start: at})
	return world.Spawn{Entry: walker.Told(
		players.Give{To: s.player.ID},
		selection.Allow{Selected: true},
	)}, true
}

func (s *arena) spawn() error {
	level, err := s.board.Lay(mapFile, "map.yaml")
	if err != nil {
		return err
	}
	return board.Spawn[walkerRow](level, board.Sides{
		PlayerSide: {players.Give{To: s.player.ID}, selection.Allow{Selected: true}},
	})
}

func (s *arena) defineScenes() []game.Scene {
	main := &mainScene{arena: s}
	return []game.Scene{ui.NewScene(MainScene, main.screen()).Input(s.players.Handle)}
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
