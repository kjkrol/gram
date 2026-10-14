package main

import (
	"embed"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/ceremony"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/ceremony/effect"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/game/stage"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/cameras"
	"github.com/kjkrol/gram/plugins/players"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/ui"
)

const (
	TPS          = 60
	GridWidth    = 24
	GridHeight   = 16
	CellSize     = 32
	ScreenWidth  = GridWidth * CellSize
	ScreenHeight = GridHeight * CellSize

	WardPx = 9 * CellSize
)

//go:embed ward.wgsl
var wardFS embed.FS

// registered before the composer's shader compiles
var wardMaterial = render.RegisterMaterials(render.Files(wardFS, "ward.wgsl"), nil, "Ward")[0]

//go:embed map.yaml
var mapFile embed.FS

type arena struct {
	world   *world.Plugin
	board   *board.Plugin
	players *players.Plugin
	cameras *cameras.Plugin
	player  *players.Player
}

func main() {
	props := game.Props{
		Title:       "gram material demo",
		Description: "An entity drawn by a material of the game's own WGSL; C calms it.",
		ScreenWidth: ScreenWidth, ScreenHeight: ScreenHeight, Resizable: true,
		TargetTPS: TPS,
	}
	a := &arena{}
	mainStage := stage.New(MaterialStage).
		World(world.Config{
			Space:    world.SpaceCfg{Width: ScreenWidth, Height: ScreenHeight},
			Entities: world.EntitiesCfg{MaxCount: 1, MinSize: CellSize, MaxSize: WardPx},
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

func (a *arena) makePlugins(w *world.Plugin) {
	a.world = w
	grid := grid.DefaultGrids{}.Square(GridWidth, GridHeight, CellSize)
	a.board = board.NewPlugin(grid, &cell.SingleOccupancy{}, a.world)
	a.cameras = cameras.NewPlugin(a.world)
	a.players = players.NewPlugin(a.world, a.cameras, a.board)
}

func (a *arena) definePlayer() error {
	a.player = a.players.Local("player")
	return a.player.Bind(a.players.Defaults()...)
}

func (a *arena) defineEffects() {
	a.world.Effects().Define(CalmEf, effect.Spec{
		effect.Described("Calmed: the ward's glow put out for a while."),
		effect.Lasts(2 * time.Second),
	})
}

func (a *arena) defineCommands() {
	a.world.Commands().Define(CalmCmd,
		ceremony.Put(a.world.Effects().Named(CalmEf)).On(ceremony.Named(WardName)))
}

func (a *arena) defineCells() {
	a.board.CellKinds().Define(GrassCell, cell.Kind{Cost: 1, Allows: cell.Land})
}

type wardRow struct {
	At geom.Vec `yaml:"at"`
}

func (a *arena) defineKinds() {
	kind.Define[wardRow](a.world.Kinds(), WardKind, kind.Spec{
		comp.Load(func(w wardRow) world.Position {
			return world.Position{
				AABB: plane.NewAABB(geom.NewVec(w.At.X-WardPx/2, w.At.Y-WardPx/2), WardPx, WardPx),
			}
		}),
		comp.Const(world.Velocity{}),
	})
}

func (a *arena) bindKeys() error {
	return a.player.Bind(
		control.Give(control.KeyPress{Key: control.KeyC}, "Calm the ward for a while",
			a.world.Commands().Named(CalmCmd)),
	)
}

func (a *arena) spawn() error {
	level, err := a.board.Lay(mapFile, "map.yaml")
	if err != nil {
		return err
	}
	return board.Spawn[wardRow](level, nil)
}

func (a *arena) defineScenes() []game.Scene {
	m := &mainScene{arena: a}
	return []game.Scene{ui.NewScene(MainScene, m.screen()).Input(a.players.Handle)}
}

func (a *arena) update(ctx goke.RunCtx, d time.Duration) {
	a.world.RunPlan(ctx, d)
	a.board.RunPlan(ctx, d)
	a.cameras.RunPlan(ctx, d)
	a.players.RunPlan(ctx, d)
	ctx.Sync()
}
