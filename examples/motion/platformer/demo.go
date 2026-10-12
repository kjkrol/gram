package main

import (
	"embed"
	"slices"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram"
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
	"github.com/kjkrol/gram/plugins/gravity"
	"github.com/kjkrol/gram/plugins/players"
	"github.com/kjkrol/gram/plugins/selection"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/steering"
	"github.com/kjkrol/gram/rule"
	"github.com/kjkrol/gram/rule/effect"
	"github.com/kjkrol/gram/ui"
)

const (
	TPS         = 60
	GridWidth   = 48
	GridHeight  = 18
	CellSize    = 16
	HeroSize    = 12
	HeroSpeed   = 140
	MaxEntCount = 8

	// how hard the world pulls down the screen, and how high the hero leaps: three and a half cells
	pull = 900
	leap = 3.5 * CellSize

	// a landing faster than this dazes, falls of four cells and more
	hardLanding = 340
	dazeLasts   = time.Second
)

//go:embed map.yaml
var mapFile embed.FS

type arena struct {
	world     *world.Plugin
	board     *board.Plugin
	collision *collision.Plugin
	gravity   *gravity.Plugin
	selection *selection.Plugin
	driving   *driving.Plugin
	cameras   *cameras.Plugin
	players   *players.Plugin
	player    *players.Player
}

func main() {
	props := game.Props{
		Title: "gram platformer demo",
		Description: "A hero in a side view under gravity: A and D or the arrows walk, " +
			"W or the up arrow jumps, G turns gravity over; a hard landing dazes.",
		ScreenWidth:  960,
		ScreenHeight: 540,
		Resizable:    true,
		TargetTPS:    TPS,
	}
	a := &arena{}
	mainStage := stage.New(PlatformerStage).
		World(world.Config{
			Space: world.SpaceCfg{Width: GridWidth * CellSize, Height: GridHeight * CellSize},
			Entities: world.EntitiesCfg{
				MaxCount: MaxEntCount,
				MinSize:  HeroSize,
				MaxSize:  HeroSize,
			},
		}).
		Plugins(a.makePlugins).
		Players(a.definePlayers).
		Effects(a.defineEffects).
		Rules(a.defineRules).
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
	s.gravity = gravity.NewPlugin(s.world, gravity.Config{Pull: gravity.Pull{Y: pull}})
	s.board = board.NewPlugin(grid, &cell.MultipleOccupancy{}, s.world).WithCollision(s.collision)
	s.selection = selection.NewPlugin(s.world)
	s.driving = driving.NewPlugin(s.world, s.selection).WithGround(s.board)
	s.cameras = cameras.NewPlugin(s.world)
	s.players = players.NewPlugin(
		s.world,
		s.cameras,
		s.selection,
		s.driving,
		s.gravity,
	)
}

func (s *arena) definePlayers() error {
	s.player = s.players.Local("player")
	keys := slices.Concat(
		s.players.DefaultBindings(),
		s.world.DefaultBindings(),
		cameras.Keys{Wheel: true}.Bindings(),
	)
	walk := []driving.Compass{
		{Left: control.KeyA, Right: control.KeyD},
		{Left: control.KeyArrowLeft, Right: control.KeyArrowRight},
	}
	for _, w := range walk {
		keys = append(keys, w.Bindings()...)
	}
	return s.player.Bind(append(keys,
		driving.JumpKey(control.KeyW),
		driving.JumpKey(control.KeyArrowUp),
	)...)
}

func (s *arena) defineEffects() {
	s.world.Effects().Define(FlippedEf, effect.Spec{
		effect.Alter(func(p *gravity.Pull) { p.Y = -p.Y }),
	})
	s.world.Effects().Define(DazedEf, effect.Spec{
		effect.Lasts(dazeLasts),
		effect.Alter(func(st *steering.Steering) { st.MaxSpeed /= 4 }),
	})
}

func (s *arena) defineRules() {
	s.world.Roles().Define(HeroRole,
		rule.Then[gravity.Landing](
			"daze",
			rule.All,
			rule.If(
				func(l gravity.Landing) bool { return l.Speed > hardLanding },
				rule.Apply(s.world.Effects().Named(DazedEf)),
			),
		))
}

func (s *arena) defineCommands() {
	flip := rule.Toggle(s.world.Effects().Named(FlippedEf)).On(s.gravity)
	s.world.Commands().Define(FlipCmd, flip)
}

func (s *arena) defineCells() {
	kinds := s.board.CellKinds()
	kinds.Define(SkyCell, cell.Kind{Cost: 1, Allows: cell.Land})
	kinds.Define(RockCell, cell.Kind{Solid: true})
	kinds.Define(LedgeCell, cell.Kind{Solid: true})
}

type heroRow struct {
	Start cell.ID `yaml:"start"`
}

func (s *arena) defineKinds() {
	brd := s.board.Board()
	units := board.NewUnits[heroRow](
		s.board,
		board.Shape{Size: HeroSize},
		func(u heroRow) geom.Vec { return brd.CellCenter(u.Start) },
	)
	units.Define(HeroKind,
		unit.Mover{Domain: cell.Land},
		steering.Steering{MaxSpeed: HeroSpeed, Accel: HeroSpeed * 8, Brake: HeroSpeed * 8},
		rule.Plays(s.world.Roles().Named(HeroRole)),
		comp.Const(gravity.Body{Leap: leap}),
		comp.Const(collision.Collider{}),
		comp.Const(collision.Physics{Mass: 1}),
	)
}

func (s *arena) bindKeys() error {
	return s.player.Bind(control.Give(
		control.KeyPress{Key: control.KeyG},
		"Turn gravity over",
		s.world.Commands().Named(FlipCmd),
	))
}

func (s *arena) spawn() error {
	level, err := s.board.Lay(mapFile, "map.yaml")
	if err != nil {
		return err
	}
	return board.Spawn[heroRow](level, board.Sides{
		PlayerSide: {players.Give{To: s.player.ID}},
	})
}

func (s *arena) defineScenes() []game.Scene {
	main := &mainScene{arena: s}
	return []game.Scene{ui.NewScene(MainScene, main.screen()).Input(s.players.Handle)}
}

func (s *arena) update(ctx goke.RunCtx, d time.Duration) {
	s.world.RunPlan(ctx, d)
	s.collision.RunPlan(ctx, d)
	s.gravity.RunPlan(ctx, d)
	s.board.RunPlan(ctx, d)
	s.driving.RunPlan(ctx, d)
	s.selection.RunPlan(ctx, d)
	s.cameras.RunPlan(ctx, d)
	s.players.RunPlan(ctx, d)
	ctx.Sync()
}
