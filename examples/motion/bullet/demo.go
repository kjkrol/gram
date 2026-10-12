package main

import (
	"embed"
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
	"github.com/kjkrol/gram/plugins/bullet"
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

func (s *arena) makePlugins(w *world.Plugin) {
	s.world = w
	grid := grid.DefaultGrids{}.Square(GridWidth, GridHeight, CellSize)
	s.collision = collision.NewPlugin(s.world)
	s.board = board.NewPlugin(grid, &cell.SingleOccupancy{}, s.world).WithCollision(s.collision)
	s.selection = selection.NewPlugin(s.world)
	s.driving = driving.NewPlugin(s.world, s.selection).WithGround(s.board)
	s.nav = navigation.NewPlugin(s.board, s.world, s.selection, s.driving).
		WithCollision(s.collision)
	s.bullet = bullet.NewPlugin(s.world, s.selection).WithGround(s.board.Heights)
	s.cameras = cameras.NewPlugin(s.world)
	s.players = players.NewPlugin(
		s.world,
		s.cameras,
		s.board,
		s.selection,
		s.nav,
		s.driving,
		s.bullet,
	)
}

func (s *arena) definePlayers() error {
	s.player = s.players.Local("player")
	s.wild = s.players.Add("wild")
	view := cameras.Keys{Wheel: true, Drag: true, Edge: true}
	drive := driving.Tank{
		Ahead: control.KeyW,
		Back:  control.KeyS,
		Left:  control.KeyA,
		Right: control.KeyD,
		In:    camera.Outside,
	}
	return s.player.Bind(slices.Concat(
		s.players.DefaultBindings(),
		s.world.DefaultBindings(),
		view.Bindings(),
		s.board.DefaultBindings(),
		s.selection.DefaultBindings(),
		s.nav.DefaultBindings(),
		s.driving.DefaultBindings(),
		s.bullet.DefaultBindings(),
		drive.Bindings(),
	)...)
}

func (s *arena) defineEffects() {
	s.world.Effects().Define(WoundedEf, effect.Spec{
		effect.Lasts(woundLasts),
		effect.Alter(func(st *steering.Steering) { st.MaxSpeed /= 2 }),
	})
	s.world.Effects().Define(FuseEf, effect.Spec{effect.Lasts(fuseLength)})
}

func (s *arena) defineRules() {
	roles := s.world.Roles()
	wounded := s.world.Effects().Named(WoundedEf)
	roles.Define(MortalRole)
	mortal := roles.Named(MortalRole)
	roles.Define(RoundRole, rule.Branch[collision.Meeting]{
		Name: "shot",
		When: collision.Met(mortal),
		Do: rule.ForOther(rule.OneOf(
			rule.Branch[any]{When: rule.Under(wounded), Do: rule.Order(world.Despawn{})},
			rule.Apply(wounded),
		)),
	})
	roles.Define(GrenadeRole, rule.Branch[bullet.Blast]{
		Name: "blast",
		When: bullet.Reaching(mortal),
		Do: rule.ForOther(rule.OneOf(
			rule.Branch[bullet.Blast]{
				When: bullet.Within(blastKills),
				Do:   rule.Order(world.Despawn{}),
			},
			rule.Apply(wounded),
		)),
	})
}

func (s *arena) defineCells() {
	kinds := s.board.CellKinds()
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

func (s *arena) defineKinds() {
	brd := s.board.Board()
	units := board.NewUnits[unitRow](
		s.board,
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
	s.bullet.Shots().Define(RoundKind,
		bullet.Body{Size: 4, Speed: 10 * CellSize, Range: 10 * CellSize},
		rule.Plays(s.world.Roles().Named(RoundRole)))
	fuse := rule.Plan(FusePlan, rule.Steps(
		rule.Until[bullet.Flight](bullet.Flight.Lying),
		rule.Apply(s.world.Effects().Named(FuseEf)),
		rule.Wait(fuseLength),
		rule.Order(bullet.Burst{Radius: blastRadius}),
	))
	s.bullet.Shots().Define(GrenadeKind,
		bullet.Body{Size: 8, Speed: 5 * CellSize, Range: 9 * CellSize, Gravity: 240, Lands: true},
		rule.Plays(s.world.Roles().Named(GrenadeRole)), fuse)
	mortal := rule.Plays(s.world.Roles().Named(MortalRole))
	units.Define(SoldierKind, unit.Mover{Domain: cell.Land}, profile, mortal,
		comp.Const(world.Velocity{Dir: geom.NewVec(1, 0)}), comp.Const(world.Eye{Height: 16}))
	units.Define(
		WandererKind,
		unit.Mover{Domain: cell.Land},
		profile,
		mortal,
		comp.Load(func(u unitRow) navigation.MoveOrder {
			return navigation.Patrol(time.Second, u.To, u.Start)
		}),
	)
}

func (s *arena) bindKeys() error {
	return s.player.Bind(
		control.Give(
			control.KeyPress{Key: control.KeyF},
			"Fire a round the way the soldier faces",
			bullet.Shoot{Ammo: s.bullet.Shots().Named(RoundKind)},
		),
		control.Build(control.KeyPress{Key: control.KeyG}, "Throw a grenade at the cursor",
			func(c control.Context) (bullet.Shoot, bool) {
				return bullet.Shoot{
					Ammo:     s.bullet.Shots().Named(GrenadeKind),
					At:       c.World(c.Cursor),
					Targeted: true,
				}, true
			}),
	)
}

func (s *arena) spawn() error {
	level, err := s.board.Lay(mapFile, "map.yaml")
	if err != nil {
		return err
	}
	return board.Spawn[unitRow](level, board.Sides{
		PlayerSide: {players.Give{To: s.player.ID}, selection.Allow{Selected: true}},
		WildSide:   {players.Give{To: s.wild.ID}},
	})
}

func (s *arena) defineScenes() []game.Scene {
	main := &mainScene{arena: s}
	return []game.Scene{ui.NewScene(MainScene, main.screen()).Input(s.players.Handle)}
}

func (s *arena) update(ctx goke.RunCtx, d time.Duration) {
	s.bullet.RunPlan(ctx, d)
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
