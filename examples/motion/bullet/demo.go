package main

import (
	"embed"
	"slices"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony/effect"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram"
	"github.com/kjkrol/gram/camera"
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
	MaxEntCount  = 64 // the units and the shots in the air; cell entities do not count

	unitSize   = 22
	unitSpeed  = CellSize * 2
	woundLasts = 5 * time.Second
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
	a.board.Figures(soldier{}, wanderer{})
	a.bullet.Figures(round{}, grenade{})
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
