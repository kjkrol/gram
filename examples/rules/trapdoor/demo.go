package main

import (
	"embed"
	"errors"
	"log"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony"
	"github.com/kjkrol/ceremony/act"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/ceremony/cond"
	"github.com/kjkrol/ceremony/effect"
	"github.com/kjkrol/ceremony/plan"
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
	EntitySize   = 20
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
		Title:       "gram trapdoor demo",
		Description: "Trapdoors under a lever.",
		ScreenWidth: ScreenWidth, ScreenHeight: ScreenHeight, Resizable: true,
		TargetTPS: TPS,
	}
	a := &arena{}
	mainStage := stage.New(TrapdoorStage).
		World(world.Config{
			Space: world.SpaceCfg{Width: ScreenWidth, Height: ScreenHeight},
			Entities: world.EntitiesCfg{
				MaxCount: MaxEntCount,
				MinSize:  EntitySize,
				MaxSize:  CellSize,
			},
		}).
		Plugins(a.makePlugins).
		Players(a.definePlayer).
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
	a.board = board.NewPlugin(grid, &cell.SingleOccupancy{}, a.world).
		WithCollision(a.collision).
		WithLog(log.Default())
	a.selection = selection.NewPlugin(a.world)
	a.driving = driving.NewPlugin(a.world, a.selection).WithGround(a.board)
	a.nav = navigation.NewPlugin(a.board, a.world, a.selection, a.driving).
		WithCollision(a.collision)
	a.cameras = cameras.NewPlugin(a.world)
	a.players = players.NewPlugin(a.world, a.collision, a.cameras, a.board, a.selection, a.nav, a.driving)
}

func (a *arena) definePlayer() error {
	a.player = a.players.Local("player")
	return a.player.Bind(a.players.Defaults()...)
}

func (a *arena) defineEffects() {
	fx := a.world.Effects()
	fx.Define(OpenEf, effect.Spec{effect.Alter(func(f *cell.Floor) { f.Allows = 0 })})
	fx.Define(HasteEf, effect.Spec{
		effect.Lasts(2 * time.Second),
		effect.Alter(
			func(st *steering.Steering) { st.MaxSpeed, st.Accel = st.MaxSpeed*2, st.Accel*2 },
		),
	})
}

func (a *arena) defineRules() {
	a.world.Roles().Define(MortalRole, act.Branch[unit.Standing]{
		Name: "fall in",
		When: cond.Self.If(unit.Fallen()),
		Then: act.Self.Do(world.Despawn{}),
	})
}

var levers = []struct {
	name string
	key  control.Key
}{{"west", control.Key1}, {"east", control.Key2}}

func (a *arena) defineCommands() {
	cmds := a.world.Commands()
	for _, l := range levers {
		cmds.Define(
			pullCmd(l.name),
			ceremony.Put(a.world.Effects().Named(OpenEf)).
				On(ceremony.Group("trapdoors "+l.name)).
				For(3*time.Second),
		)
	}
	cmds.Define(HastenCmd, ceremony.Put(a.world.Effects().Named(HasteEf)).On(a.selection.Selected()))
}

func (a *arena) defineCells() {
	kinds := a.board.CellKinds()
	kinds.Define(GrassCell, cell.Kind{Cost: 1, Allows: cell.Land})
	kinds.Define(PitCell, cell.Kind{Cost: 1}) // holds nobody
}

type unitRow struct {
	Start cell.ID `yaml:"start"`
	To    cell.ID `yaml:"to"`
}

type standingRow struct {
	At cell.ID `yaml:"at"`
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
		V0:       UnitSpeed / 2,
		TurnRate: 0.15,
	}
	land := unit.Mover{Domain: cell.Land}
	units.Define(ScoutKind, land, profile, ceremony.Plays(a.world.Roles().Named(MortalRole)))
	patrol := plan.New(PatrolPlan, plan.Steps(
		plan.Loop[any]{
			Name:  "leg",
			When:  cond.Self.If(cond.Always),
			Then:  plan.Self.Do(navigation.Onward{}),
			Until: cond.Self.If(navigation.Arrived{}),
		},
		plan.Loop[any]{Name: "rest", When: cond.Self.If(cond.Always), Then: plan.Self.Wait(time.Second)},
	))
	round := comp.Load(func(u unitRow) navigation.Round {
		return navigation.NewRound(u.To, u.Start)
	})
	units.Define(WandererKind, land, profile, ceremony.Plays(a.world.Roles().Named(MortalRole)),
		round, patrol)
	standing := board.NewEntities(
		a.board,
		board.Shape{Size: CellSize},
		func(r standingRow) cell.ID { return r.At },
	)
	standing.Define(TrapdoorKind, comp.Const(cell.Floor{Allows: cell.Land}))
}

func (a *arena) bindKeys() error {
	for _, l := range levers {
		pull := control.Give(control.KeyPress{Key: l.key},
			"Pull the "+l.name+" lever: its trapdoors open",
			a.world.Commands().Named(pullCmd(l.name)))
		if err := a.player.Bind(pull); err != nil {
			return err
		}
	}
	return a.player.Bind(
		control.Give(
			control.KeyPress{Key: control.KeyJ},
			"Hasten the selected scouts",
			a.world.Commands().Named(HastenCmd),
		),
	)
}

func (a *arena) spawn() error {
	level, err := a.board.Lay(mapFile, "map.yaml")
	if err != nil {
		return err
	}
	sides := board.Sides{
		PlayerSide: {players.Give{To: a.player.ID}, selection.Allow{}},
	}
	return errors.Join(
		board.Spawn[standingRow](level, nil),
		board.Spawn[unitRow](level, sides),
	)
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
