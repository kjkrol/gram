package main

import (
	"embed"
	"slices"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony/effect"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/game/stage"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/bullet"
	"github.com/kjkrol/gram/plugins/cameras"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/driving"
	"github.com/kjkrol/gram/plugins/motion"
	"github.com/kjkrol/gram/plugins/motion/steering"
	"github.com/kjkrol/gram/plugins/navigation"
	"github.com/kjkrol/gram/plugins/players"
	"github.com/kjkrol/gram/plugins/selection"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/ui"
)

const (
	TPS          = 60
	GridWidth    = 24
	GridHeight   = 16
	CellSize     = 32
	ScreenWidth  = GridWidth * CellSize
	ScreenHeight = GridHeight * CellSize
	MaxEntCount  = 64 // the units and the shots in the air; cell entities do not count

	unitSize   = 22
	unitSpeed  = CellSize * 2
	woundLasts = 5 * time.Second

	tapFor = 160 * time.Millisecond // a key let go sooner turns the soldier without a step
	// Shift held this long throws a grenade its whole range; a tap throws it a fifth of it
	fullThrow = 1200 * time.Millisecond
	tapThrow  = 0.2
)

//go:embed map.yaml
var mapFile embed.FS

type arena struct {
	world     *world.Plugin
	motion    *motion.Plugin
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
		Title: "gram bullet demo",
		Description: "A soldier on WSAD shoots rounds with Space and throws grenades with " +
			"Shift: the longer it is held, the further the grenade flies.",
		ScreenWidth: ScreenWidth, ScreenHeight: ScreenHeight, Resizable: true,
		TargetTPS: TPS,
	}
	a := &arena{}
	mainStage := stage.New(BulletStage).
		World(world.Config{
			Space:    world.SpaceCfg{Width: ScreenWidth, Height: ScreenHeight},
			Entities: world.EntitiesCfg{MaxCount: MaxEntCount, MinSize: 4, MaxSize: unitSize},
			Heights:  true,
		}).
		Plugins(a.makePlugins).
		Figures(a.defineFigures).
		Players(a.definePlayers).
		Effects(a.defineEffects).
		Rules(a.defineRules).
		Controls(a.bindKeys).
		Spawn(a.spawn).
		Scenes(a.defineScenes).
		Update(a.update)
	gram.Run(game.New(props, mainStage))
}

func (a *arena) makePlugins(w *world.Plugin) {
	a.world = w
	a.motion = motion.NewPlugin(a.world)
	grid := grid.DefaultGrids{}.Square(GridWidth, GridHeight, CellSize)
	a.collision = collision.NewPlugin(a.world, a.motion)
	a.board = board.NewPlugin(grid, &cell.SingleOccupancy{}, a.world).
		WithMotion(a.motion).
		WithCollision(a.collision)
	a.selection = selection.NewPlugin(a.world)
	a.driving = driving.NewPlugin(a.world, a.motion, a.selection).
		WithGround(a.board).
		WithTap(tapFor)
	a.nav = navigation.NewPlugin(a.board, a.world, a.motion, a.selection, a.driving).
		WithCollision(a.collision)
	a.bullet = bullet.NewPlugin(a.world, a.motion, a.selection).WithGround(a.board.Heights)
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
	walk := driving.Compass{
		Up: control.KeyW, Down: control.KeyS, Left: control.KeyA, Right: control.KeyD,
	}
	view := cameras.Keys{Wheel: true, ZoomIn: control.KeyEqual, ZoomOut: control.KeyMinus}
	return a.player.Bind(slices.Concat(
		a.players.DefaultBindings(),
		a.world.DefaultBindings(),
		a.collision.DefaultBindings(),
		a.board.DefaultBindings(),
		view.Bindings(),
		walk.Bindings(),
	)...)
}

func (a *arena) defineEffects() {
	a.world.Effects().Define(WoundedEf, effect.Spec{
		effect.Lasts(woundLasts),
		effect.Alter(func(st *steering.Steering) { st.MaxSpeed /= 2 }),
	})
}

func (a *arena) defineRules() {
	a.world.Roles().Define(MortalRole)
}

type unitRow struct {
	Start cell.ID `yaml:"start"`
	To    cell.ID `yaml:"to"`
}

func units(b *board.Plugin) *board.Units[unitRow] {
	return board.NewUnits[unitRow](b, board.Shape{Size: unitSize, Height: unitSize},
		func(u unitRow) geom.Vec { return b.Board().CellCenter(u.Start) })
}

func walking() steering.Steering {
	return steering.Steering{
		MaxSpeed: unitSpeed,
		Accel:    unitSpeed * 2,
		Brake:    unitSpeed * 4,
		V0:       unitSpeed / 2,
		TurnRate: 0.15,
	}
}

func (a *arena) defineFigures() {
	a.board.Tiles(tiles()...)
	a.board.Surfaces(surface{})
	a.board.Figures(soldier{}, wanderer{})
	a.bullet.Figures(round{}, grenade{})
}

func (a *arena) bindKeys() error {
	shots := a.bullet.Shots()
	return a.player.Bind(
		control.Build(control.KeyPress{Key: control.KeySpace},
			"Fire a round the way the soldier faces",
			func(c control.Context) (bullet.Shoot, bool) {
				return bullet.Shoot{Ammo: shots.Named(RoundKind), Camera: c.Camera}, true
			}),
		control.Build(control.KeyReleased{Key: control.KeyShift},
			"Throw a grenade, the further the longer held",
			func(c control.Context) (bullet.Shoot, bool) {
				power := max(float64(c.Held)/float64(fullThrow), tapThrow)
				grenade := shots.Named(GrenadeKind)
				return bullet.Shoot{Ammo: grenade, Camera: c.Camera, Power: power}, true
			}),
	)
}

func (a *arena) spawn() error {
	level, err := a.board.Lay(mapFile, "map.yaml")
	if err != nil {
		return err
	}
	return board.Spawn[unitRow](level, board.Sides{
		PlayerSide: {players.Give{To: a.player.ID}},
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
	a.motion.RunPlan(ctx, d)
	a.collision.RunPlan(ctx, d)
	a.board.RunPlan(ctx, d)
	a.nav.RunPlan(ctx, d)
	a.driving.RunPlan(ctx, d)
	a.selection.RunPlan(ctx, d)
	a.cameras.RunPlan(ctx, d)
	a.players.RunPlan(ctx, d)
	ctx.Sync()
}
