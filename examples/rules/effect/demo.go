package main

import (
	"embed"
	"log"
	"math"
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
	"github.com/kjkrol/gram/plugins/navigation"
	"github.com/kjkrol/gram/plugins/players"
	"github.com/kjkrol/gram/plugins/selection"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/steering"
	"github.com/kjkrol/gram/ui"
)

const (
	TPS          = 60
	GridWidth    = 24
	GridHeight   = 16
	CellSize     = 32
	ScreenWidth  = GridWidth * CellSize
	ScreenHeight = GridHeight * CellSize
	unitSize     = 22
	unitSpeed    = CellSize * 2
	MaxEntCount  = 16

	Frost = cell.Domain(1 << 3)
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
		Title:       "gram effect demo",
		Description: "An ice witch writes the terrain.",
		ScreenWidth: ScreenWidth, ScreenHeight: ScreenHeight, Resizable: true,
		TargetTPS: TPS,
	}
	a := &arena{}
	mainStage := stage.New(EffectStage).
		World(world.Config{
			Space: world.SpaceCfg{Width: ScreenWidth, Height: ScreenHeight},
			Entities: world.EntitiesCfg{
				MaxCount: MaxEntCount,
				MinSize:  unitSize,
				MaxSize:  unitSize,
			},
		}).
		Plugins(a.makePlugins).
		Figures(a.defineFigures).
		Players(a.definePlayer).
		Effects(a.defineEffects).
		Rules(a.defineRoles).
		Commands(a.defineCommands).
		Controls(a.bindKeys).
		Spawn(a.spawn).
		Scenes(a.defineScenes).
		Update(a.update)
	gram.Run(game.New(props, mainStage))
}

func (a *arena) makePlugins(w *world.Plugin) {
	a.world = w
	a.collision = collision.NewPlugin(a.world)
	grid := grid.DefaultGrids{}.Square(GridWidth, GridHeight, CellSize)
	a.board = board.NewPlugin(grid, &cell.SingleOccupancy{}, a.world).
		WithCollision(a.collision).
		WithLog(log.Default())
	a.selection = selection.NewPlugin(a.world)
	a.driving = driving.NewPlugin(a.world, a.selection).WithGround(a.board)
	a.nav = navigation.NewPlugin(a.board, a.world, a.selection, a.driving).
		WithCollision(a.collision)
	a.cameras = cameras.NewPlugin(a.world)
	a.players = players.NewPlugin(a.world, a.collision,
		a.cameras, a.board, a.selection, a.nav, a.driving)
}

func (a *arena) definePlayer() error {
	a.player = a.players.Local("player")
	return a.player.Bind(a.players.Defaults()...)
}

// the states every unit may be in, fallen into the lake or walking the ice
func (a *arena) defineEffects() {
	effects := a.world.Effects()
	effects.Define(FrozenEf, effect.Spec{
		effect.Described("Frozen stiff: stood dead still, an immovable block."),
		effect.Alter(func(p *collision.Physics) { p.Mass = math.Inf(1) }),
		effect.Alter(func(st *steering.Steering) { st.Halted = true }),
	})
	effects.Define(SlipEf, effect.Spec{
		effect.Described("On the ice: hardly any braking, whoever moves slides on."),
		effect.Alter(func(st *steering.Steering) { st.Brake = st.Accel / 8 }),
	})
}

// mortal is everyone's: the witch, the walker and the boat
func (a *arena) defineRoles() {
	effects := a.world.Effects()
	iced, frozen, slip := effects.Named(IcedEf), effects.Named(FrozenEf), effects.Named(SlipEf)
	a.world.Roles().Define(MortalRole,
		act.Branch[unit.Standing]{
			Name: "fallen in",
			When: cond.Self.If(unit.Fallen()),
			Then: act.OneOf(
				act.Branch[unit.Standing]{
					When: cond.Self.If(unit.StandingOn(iced)),
					Then: act.Self.Hold(frozen),
				},
				act.Self.Do(world.Despawn{}),
			),
		},
		act.Branch[unit.Standing]{
			Name: "on the ice",
			When: cond.Self.If(cond.Not(unit.Fallen())).
				And(cond.Self.If(unit.StandingOn(iced))),
			Then: act.Self.Hold(slip),
		},
	)
}

func (a *arena) defineCommands() {
	freeze := ceremony.Put(a.world.Effects().Named(FrozenEf)).On(a.selection.Pointed())
	a.world.Commands().Define(FreezeCmd, freeze.For(3*time.Second))
}

type unitRow struct {
	Start  cell.ID `yaml:"start"`
	Target cell.ID `yaml:"target"`
}

func units(b *board.Plugin) *board.Units[unitRow] {
	return board.NewUnits[unitRow](b, board.Shape{Size: unitSize},
		func(u unitRow) geom.Vec { return b.Board().CellCenter(u.Start) })
}

func profile(brake float64) steering.Steering {
	return steering.Steering{
		MaxSpeed: unitSpeed,
		Accel:    unitSpeed * 2,
		Brake:    brake,
		V0:       unitSpeed / 2,
		TurnRate: 0.15,
	}
}

func ordered() comp.Comp {
	return comp.Load(func(u unitRow) navigation.MoveOrder {
		return navigation.MoveOrder{Target: u.Target}
	})
}

// the lake's role before the witch's rule that names it
func (a *arena) defineFigures() {
	a.board.Tiles(grass{}, water{})
	a.board.Figures(witch{}, walker{}, boat{})
}

func (a *arena) bindKeys() error {
	return a.player.Bind(
		control.Give(control.KeyPress{Key: control.KeyF}, "Freeze the one pointed at",
			a.world.Commands().Named(FreezeCmd)),
	)
}

func (a *arena) spawn() error {
	level, err := a.board.Lay(mapFile, "map.yaml")
	if err != nil {
		return err
	}
	return board.Spawn[unitRow](level, board.Sides{
		PlayerSide: {players.Give{To: a.player.ID}, selection.Allow{}},
	})
}

func (a *arena) defineScenes() []game.Scene {
	main := &mainScene{arena: a}
	return []game.Scene{ui.NewScene(MainScene, main.screen()).Input(a.players.Handle)}
}

func (a *arena) update(ctx goke.RunCtx, d time.Duration) {
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
