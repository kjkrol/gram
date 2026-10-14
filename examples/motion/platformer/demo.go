package main

import (
	"embed"
	"slices"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony"
	"github.com/kjkrol/ceremony/act"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/ceremony/cond"
	"github.com/kjkrol/ceremony/effect"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram"
	"github.com/kjkrol/gram/control"
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

	// in cells
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

func (a *arena) makePlugins(w *world.Plugin) {
	a.world = w
	grid := grid.DefaultGrids{}.Square(GridWidth, GridHeight, CellSize)
	a.collision = collision.NewPlugin(a.world)
	a.gravity = gravity.NewPlugin(a.world, gravity.Config{Pull: gravity.Pull{Y: pull}})
	a.board = board.NewPlugin(grid, &cell.MultipleOccupancy{}, a.world).WithCollision(a.collision)
	a.selection = selection.NewPlugin(a.world)
	a.driving = driving.NewPlugin(a.world, a.selection).WithGround(a.board)
	a.cameras = cameras.NewPlugin(a.world)
	a.players = players.NewPlugin(
		a.world,
		a.collision, a.cameras,
		a.selection,
		a.driving,
		a.gravity,
	)
}

func (a *arena) definePlayers() error {
	a.player = a.players.Local("player")
	keys := slices.Concat(
		a.players.DefaultBindings(),
		a.world.DefaultBindings(),
		cameras.Keys{Wheel: true}.Bindings(),
	)
	walk := []driving.Compass{
		{Left: control.KeyA, Right: control.KeyD},
		{Left: control.KeyArrowLeft, Right: control.KeyArrowRight},
	}
	for _, w := range walk {
		keys = append(keys, w.Bindings()...)
	}
	return a.player.Bind(append(keys,
		driving.JumpKey(control.KeyW),
		driving.JumpKey(control.KeyArrowUp),
	)...)
}

func (a *arena) defineEffects() {
	a.world.Effects().Define(FlippedEf, effect.Spec{
		effect.Alter(func(p *gravity.Pull) { p.Y = -p.Y }),
	})
	a.world.Effects().Define(DazedEf, effect.Spec{
		effect.Lasts(dazeLasts),
		effect.Alter(func(st *steering.Steering) { st.MaxSpeed /= 4 }),
	})
}

func (a *arena) defineRules() {
	a.world.Roles().Define(HeroRole, act.Branch[gravity.Landing]{
		Name: "daze",
		When: cond.Self.If(gravity.HarderThan(hardLanding)),
		Then: act.Self.Put(a.world.Effects().Named(DazedEf)),
	})
}

func (a *arena) defineCommands() {
	flip := ceremony.Toggle(a.world.Effects().Named(FlippedEf)).On(a.gravity)
	a.world.Commands().Define(FlipCmd, flip)
}

func (a *arena) defineCells() {
	kinds := a.board.CellKinds()
	kinds.Define(SkyCell, cell.Kind{Cost: 1, Allows: cell.Land})
	kinds.Define(RockCell, cell.Kind{Solid: true})
	kinds.Define(LedgeCell, cell.Kind{Solid: true})
}

type heroRow struct {
	Start cell.ID `yaml:"start"`
}

func (a *arena) defineKinds() {
	brd := a.board.Board()
	units := board.NewUnits[heroRow](
		a.board,
		board.Shape{Size: HeroSize},
		func(u heroRow) geom.Vec { return brd.CellCenter(u.Start) },
	)
	units.Define(HeroKind,
		unit.Mover{Domain: cell.Land},
		steering.Steering{MaxSpeed: HeroSpeed, Accel: HeroSpeed * 8, Brake: HeroSpeed * 8},
		ceremony.Plays(a.world.Roles().Named(HeroRole)),
		comp.Const(gravity.Body{Leap: leap}),
		comp.Const(collision.Collider{}),
		comp.Const(collision.Physics{Mass: 1}),
	)
}

func (a *arena) bindKeys() error {
	return a.player.Bind(control.Give(
		control.KeyPress{Key: control.KeyG},
		"Turn gravity over",
		a.world.Commands().Named(FlipCmd),
	))
}

func (a *arena) spawn() error {
	level, err := a.board.Lay(mapFile, "map.yaml")
	if err != nil {
		return err
	}
	return board.Spawn[heroRow](level, board.Sides{
		PlayerSide: {players.Give{To: a.player.ID}},
	})
}

func (a *arena) defineScenes() []game.Scene {
	main := &mainScene{arena: a}
	return []game.Scene{ui.NewScene(MainScene, main.screen()).Input(a.players.Handle)}
}

func (a *arena) update(ctx goke.RunCtx, d time.Duration) {
	a.world.RunPlan(ctx, d)
	a.collision.RunPlan(ctx, d)
	a.gravity.RunPlan(ctx, d)
	a.board.RunPlan(ctx, d)
	a.driving.RunPlan(ctx, d)
	a.selection.RunPlan(ctx, d)
	a.cameras.RunPlan(ctx, d)
	a.players.RunPlan(ctx, d)
	ctx.Sync()
}
