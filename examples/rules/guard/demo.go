package main

import (
	"embed"
	"log"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/entity"
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
	"github.com/kjkrol/gram/rule"
	"github.com/kjkrol/gram/rule/effect"
	"github.com/kjkrol/gram/ui"
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
	MaxEntCount  = 16
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
		Title: "gram guard demo",
		Description: "Guards on a plan: they patrol the yard, step aside for whoever blocks them " +
			"long enough, rest where they arrive, and run to the gate while the alarm (A) is on.",
		ScreenWidth:  ScreenWidth,
		ScreenHeight: ScreenHeight,
		Resizable:    true,
		TargetTPS:    TPS,
	}
	a := &arena{}
	mainStage := stage.New(GuardStage).
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
		Effects(a.defineEffects).
		Commands(a.defineCommands).
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

func (s *arena) defineEffects() {
	s.world.Effects().Define(AlarmEf, effect.Spec{})
}

func (s *arena) defineCommands() {
	s.world.Commands().Define(AlarmCmd,
		rule.Toggle(s.world.Effects().Named(AlarmEf)).On(entity.World))
}

func (s *arena) defineCells() {
	kinds := s.board.CellKinds()
	kinds.Define(GrassCell, cell.Kind{Cost: 1, Allows: cell.Land})
	kinds.Define(WallCell, cell.Kind{Cost: 1, Solid: true})
}

type guardRow struct {
	Start cell.ID `yaml:"start"`
}

type wandererRow struct {
	Start cell.ID `yaml:"start"`
	To    cell.ID `yaml:"to"`
}

func (s *arena) defineKinds() {
	brd := s.board.Board()
	profile := steering.Steering{
		MaxSpeed: UnitSpeed,
		Accel:    UnitSpeed * 2,
		Brake:    UnitSpeed * 4,
		V0:       UnitSpeed / 2,
		TurnRate: 0.15,
	}
	east, west, gate := brd.At(19, 8), brd.At(4, 8), brd.At(12, 2)
	guard := rule.Plan(GuardPlan, rule.OneOf(
		rule.Branch[any]{
			Name: "to the gate",
			When: rule.During(s.world.Effects().Named(AlarmEf)),
			Do:   rule.Order(navigation.MoveTo{Cell: gate}).Until[navigation.Arrived](),
		},
		rule.Branch[navigation.Blocked]{
			Name: "blocked",
			When: navigation.Blocked.WaitedLong,
			Do:   rule.Order(navigation.StepAside{}),
		},
		rule.Branch[navigation.Arrived]{
			Name:    "rest",
			Through: true,
			Do:      rule.Wait(2 * time.Second),
		},
		rule.Branch[any]{
			Name: "patrol",
			Do: rule.Steps(
				rule.Order(navigation.MoveTo{Cell: east}).Until[navigation.Arrived](),
				rule.Order(navigation.MoveTo{Cell: west}).Until[navigation.Arrived](),
			),
		},
		rule.Idle(),
	))
	guards := board.NewUnits[guardRow](s.board, board.Shape{Size: EntitySize},
		func(g guardRow) geom.Vec { return brd.CellCenter(g.Start) })
	guards.Define(GuardKind, unit.Mover{Domain: cell.Land}, profile, guard)

	wanderers := board.NewUnits[wandererRow](s.board, board.Shape{Size: EntitySize},
		func(w wandererRow) geom.Vec { return brd.CellCenter(w.Start) })
	wanderers.Define(WandererKind, unit.Mover{Domain: cell.Land}, profile,
		comp.Load(func(w wandererRow) navigation.MoveOrder {
			return navigation.Patrol(3*time.Second, w.To, w.Start)
		}))
}

func (s *arena) bindKeys() error {
	return s.player.Bind(control.Give(
		control.KeyPress{Key: control.KeyA},
		"Sound the alarm, or call it off",
		s.world.Commands().Named(AlarmCmd),
	))
}

func (s *arena) spawn() error {
	level, err := s.board.Lay(mapFile, "map.yaml")
	if err != nil {
		return err
	}
	sides := board.Sides{
		PlayerSide: {players.Give{To: s.player.ID}, selection.Allow{Selected: true}},
	}
	if err := board.Spawn[guardRow](level, sides); err != nil {
		return err
	}
	return board.Spawn[wandererRow](level, sides)
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
