package main

import (
	"embed"
	"log"
	"slices"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/control"
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
	"github.com/kjkrol/gram/render"
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
	EntitySize   = 28
	UnitSpeed    = CellSize * 3
)

//go:embed ember.wgsl
var emberFS embed.FS

// emberMaterial joins the composer's one shader as the package is set up, before it compiles.
var emberMaterial = render.RegisterMaterials(render.Files(emberFS, "ember.wgsl"), nil, "Ember")[0]

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
		Title:       "gram ember demo",
		Description: "Embers animated by their own state; WSAD drives the selected, F douses.",
		ScreenWidth: ScreenWidth, ScreenHeight: ScreenHeight, Resizable: true,
		TargetTPS: TPS,
	}
	a := &arena{}
	mainStage := stage.New(EmberStage).
		World(world.Config{
			Space:    world.SpaceCfg{Width: ScreenWidth, Height: ScreenHeight},
			Entities: world.EntitiesCfg{MaxCount: 8, MinSize: EntitySize, MaxSize: EntitySize},
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
	s.collision = collision.NewPlugin(s.world)
	grid := grid.DefaultGrids{}.Square(GridWidth, GridHeight, CellSize)
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
	view := cameras.Keys{Wheel: true, Drag: true, Edge: true}
	return s.player.Bind(slices.Concat(
		s.players.DefaultBindings(),
		s.world.DefaultBindings(),
		view.Bindings(),
		s.board.DefaultBindings(),
		s.selection.DefaultBindings(),
		s.nav.DefaultBindings(),
		s.driving.DefaultBindings(),
	)...)
}

func (s *arena) defineEffects() {
	s.world.Effects().Define(DousedEf, effect.Spec{
		effect.Described("Doused: the flame put out — soot until it rekindles."),
		effect.Lasts(3 * time.Second),
	})
}

func (s *arena) defineCommands() {
	s.world.Commands().Define(DouseCmd,
		rule.Cast(s.world.Effects().Named(DousedEf)).On(s.selection.Selected()))
}

func (s *arena) defineCells() {
	s.board.CellKinds().Define(GrassCell, cell.Kind{Cost: 1, Allows: cell.Land})
}

type unitRow struct {
	Start cell.ID    `yaml:"start"`
	Round [4]cell.ID `yaml:"round"`
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
		TurnRate: 0.1,
	}
	round := comp.Load(func(u unitRow) navigation.MoveOrder {
		return navigation.Patrol(2*time.Second, u.Round[0], u.Round[1], u.Round[2], u.Round[3])
	})
	units.Define(EmberKind, unit.Mover{Domain: cell.Land}, profile, round)
}

func (s *arena) bindKeys() error {
	drive := driving.Tank{
		Ahead: control.KeyW,
		Back:  control.KeyS,
		Left:  control.KeyA,
		Right: control.KeyD,
		In:    camera.Outside,
	}
	return s.player.Bind(append(drive.Bindings(),
		control.Give(control.KeyPress{Key: control.KeyF}, "Douse the selected embers for a while",
			s.world.Commands().Named(DouseCmd)))...)
}

func (s *arena) defineScenes() []game.Scene {
	m := &mainScene{arena: s}
	return []game.Scene{ui.NewScene(MainScene, m.screen()).Input(s.players.Handle)}
}

func (s *arena) spawn() error {
	level, err := s.board.Lay(mapFile, "map.yaml")
	if err != nil {
		return err
	}
	return board.Spawn[unitRow](level, board.Sides{
		PlayerSide: {players.Give{To: s.player.ID}, selection.Allow{}},
	})
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
