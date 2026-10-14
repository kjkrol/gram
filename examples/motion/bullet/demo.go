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
	"github.com/kjkrol/ceremony/plan"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/game/stage"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/unit"
	"github.com/kjkrol/gram/plugins/bullet"
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
	EntitySize   = 22
	UnitSpeed    = CellSize * 2
	MaxEntCount  = 64 // the units and the shots in the air; cell entities do not count

	fuseLength  = 2 * time.Second
	woundLasts  = 5 * time.Second
	blastRadius = 2.5 * CellSize
	blastKills  = CellSize
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
	bullet    *bullet.Plugin
	players   *players.Plugin
	cameras   *cameras.Plugin
	player    *players.Player
	wild      *players.Player
}

func main() {
	props := game.Props{
		Title:       "gram bullet demo",
		Description: "A soldier shoots.",
		ScreenWidth: ScreenWidth, ScreenHeight: ScreenHeight, Resizable: true,
		TargetTPS: TPS,
	}
	a := &arena{}
	mainStage := stage.New(BulletStage).
		World(world.Config{
			Space:    world.SpaceCfg{Width: ScreenWidth, Height: ScreenHeight},
			Entities: world.EntitiesCfg{MaxCount: MaxEntCount, MinSize: 4, MaxSize: EntitySize},
			Heights:  true,
		}).
		Plugins(a.makePlugins).
		Players(a.definePlayers).
		Effects(a.defineEffects).
		Rules(a.defineRules).
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
	a.board = board.NewPlugin(grid, &cell.SingleOccupancy{}, a.world).WithCollision(a.collision)
	a.selection = selection.NewPlugin(a.world)
	a.driving = driving.NewPlugin(a.world, a.selection).WithGround(a.board)
	a.nav = navigation.NewPlugin(a.board, a.world, a.selection, a.driving).
		WithCollision(a.collision)
	a.bullet = bullet.NewPlugin(a.world, a.selection).WithGround(a.board.Heights)
	a.cameras = cameras.NewPlugin(a.world)
	a.players = players.NewPlugin(
		a.world,
		a.collision, a.cameras,
		a.board,
		a.selection,
		a.nav,
		a.driving,
		a.bullet,
	)
}

func (a *arena) definePlayers() error {
	a.player = a.players.Local("player")
	a.wild = a.players.Add("wild")
	view := cameras.Keys{Wheel: true, Drag: true, Edge: true}
	drive := driving.Tank{
		Ahead: control.KeyW,
		Back:  control.KeyS,
		Left:  control.KeyA,
		Right: control.KeyD,
		In:    camera.Outside,
	}
	return a.player.Bind(slices.Concat(
		a.players.DefaultBindings(),
		a.world.DefaultBindings(),
		view.Bindings(),
		a.board.DefaultBindings(),
		a.selection.DefaultBindings(),
		a.nav.DefaultBindings(),
		a.driving.DefaultBindings(),
		a.bullet.DefaultBindings(),
		drive.Bindings(),
	)...)
}

func (a *arena) defineEffects() {
	a.world.Effects().Define(WoundedEf, effect.Spec{
		effect.Lasts(woundLasts),
		effect.Alter(func(st *steering.Steering) { st.MaxSpeed /= 2 }),
	})
	a.world.Effects().Define(FuseEf, effect.Spec{effect.Lasts(fuseLength)})
}

func (a *arena) defineRules() {
	roles := a.world.Roles()
	wounded := a.world.Effects().Named(WoundedEf)
	roles.Define(MortalRole)
	mortal := roles.Named(MortalRole)
	roles.Define(RoundRole, act.Branch[collision.Meeting]{
		Name: "shot",
		When: cond.Other.If(mortal),
		Then: act.OneOf(
			act.Branch[any]{When: cond.Other.If(wounded), Then: act.Other.Do(world.Despawn{})},
			act.Other.Put(wounded),
		),
	})
	roles.Define(GrenadeRole, act.Branch[bullet.Blast]{
		Name: "blast",
		When: cond.Other.If(mortal),
		Then: act.OneOf(
			act.Branch[bullet.Blast]{
				When: cond.Other.If(bullet.Within(blastKills)),
				Then: act.Other.Do(world.Despawn{}),
			},
			act.Other.Put(wounded),
		),
	})
}

func (a *arena) defineCells() {
	kinds := a.board.CellKinds()
	kinds.Define(GrassCell, cell.Kind{Cost: 2, Allows: cell.Land})
	kinds.Define(RoadCell, cell.Kind{Cost: 1, Allows: cell.Land})
	kinds.Define(WaterCell, cell.Kind{Cost: 1, Allows: cell.Water})
	kinds.Define(WallCell, cell.Kind{Cost: 1, Solid: true, Height: 30})
	kinds.Define(LowWallCell, cell.Kind{Cost: 1, Solid: true, Height: 10})
}

type unitRow struct {
	Start cell.ID `yaml:"start"`
	To    cell.ID `yaml:"to"`
}

func (a *arena) defineKinds() {
	brd := a.board.Board()
	units := board.NewUnits[unitRow](
		a.board,
		board.Shape{Size: EntitySize, Height: EntitySize},
		func(u unitRow) geom.Vec { return brd.CellCenter(u.Start) },
	)
	profile := steering.Steering{
		MaxSpeed: UnitSpeed,
		Accel:    UnitSpeed * 2,
		Brake:    UnitSpeed * 4,
		V0:       UnitSpeed / 2,
		TurnRate: 0.15,
	}
	a.bullet.Shots().Define(RoundKind,
		bullet.Body{Size: 4, Speed: 10 * CellSize, Range: 10 * CellSize},
		ceremony.Plays(a.world.Roles().Named(RoundRole)))
	fuse := plan.New(FusePlan, plan.Steps(
		plan.Loop[bullet.Flight]{
			Name: "lit",
			When: cond.Self.If(bullet.Lying()),
			Then: plan.Steps(
				plan.Self.Put(a.world.Effects().Named(FuseEf)),
				plan.Self.Wait(fuseLength),
			),
		},
		plan.Loop[any]{
			Name: "burst",
			When: cond.Self.If(cond.Always),
			Then: plan.Self.Do(bullet.Burst{Radius: blastRadius}),
		},
	))
	a.bullet.Shots().Define(GrenadeKind,
		bullet.Body{Size: 8, Speed: 5 * CellSize, Range: 9 * CellSize, Gravity: 240, Lands: true},
		ceremony.Plays(a.world.Roles().Named(GrenadeRole)), fuse)
	mortal := ceremony.Plays(a.world.Roles().Named(MortalRole))
	units.Define(SoldierKind, unit.Mover{Domain: cell.Land}, profile, mortal,
		comp.Const(world.Velocity{Dir: geom.NewVec(1, 0)}), comp.Const(world.Eye{Height: 16}))
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
	units.Define(WandererKind, unit.Mover{Domain: cell.Land}, profile, mortal, round, patrol)
}

func (a *arena) bindKeys() error {
	return a.player.Bind(
		control.Give(
			control.KeyPress{Key: control.KeyF},
			"Fire a round the way the soldier faces",
			bullet.Shoot{Ammo: a.bullet.Shots().Named(RoundKind)},
		),
		control.Build(control.KeyPress{Key: control.KeyG}, "Throw a grenade at the cursor",
			func(c control.Context) (bullet.Shoot, bool) {
				return bullet.Shoot{
					Ammo:     a.bullet.Shots().Named(GrenadeKind),
					At:       c.World(c.Cursor),
					Targeted: true,
				}, true
			}),
	)
}

func (a *arena) spawn() error {
	level, err := a.board.Lay(mapFile, "map.yaml")
	if err != nil {
		return err
	}
	return board.Spawn[unitRow](level, board.Sides{
		PlayerSide: {players.Give{To: a.player.ID}, selection.Allow{Selected: true}},
		WildSide:   {players.Give{To: a.wild.ID}},
	})
}

func (a *arena) defineScenes() []game.Scene {
	main := &mainScene{arena: a}
	return []game.Scene{ui.NewScene(MainScene, main.screen()).Input(a.players.Handle)}
}

func (a *arena) update(ctx goke.RunCtx, d time.Duration) {
	a.bullet.RunPlan(ctx, d)
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
