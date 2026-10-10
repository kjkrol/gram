package main

import (
	"embed"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/entity"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/entity/kind/comp"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/game/stage"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/cameras"
	"github.com/kjkrol/gram/plugins/players"
	"github.com/kjkrol/gram/plugins/world"
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

	// WardPx is the ward's box side: how far its glow reaches.
	WardPx = 9 * CellSize
)

//go:embed ward.wgsl
var wardFS embed.FS

// wardMaterial joins the composer's one shader as the package is set up, before it compiles.
var wardMaterial = render.RegisterMaterials(render.Files(wardFS, "ward.wgsl"), nil, "Ward")[0]

type arena struct {
	world   *world.Plugin
	board   *board.Plugin
	players *players.Plugin
	cameras *cameras.Plugin
	player  *players.Player
}

func NewDemo(s *arena) game.Game {
	return game.New(game.Props{
		Title:       "gram material demo",
		Description: "An entity drawn by a material of the game's own WGSL; C calms it.",
		ScreenWidth: ScreenWidth, ScreenHeight: ScreenHeight, Resizable: true,
		TargetTPS: TPS,
	},
		stage.New(MaterialStage).
			World(world.Config{
				Space:    world.SpaceCfg{Width: ScreenWidth, Height: ScreenHeight},
				Entities: world.EntitiesCfg{MaxCount: 1, MinSize: CellSize, MaxSize: WardPx},
			}).
			Plugins(s.makePlugins).
			Players(s.definePlayer).
			Effects(s.defineEffects).
			Commands(s.defineCommands).
			Kinds(s.defineCells, s.defineKinds).
			Controls(s.bindKeys).
			Spawn(s.spawnCells, s.spawnUnits).
			Scenes(s.defineScenes).
			Update(s.update))
}

func (s *arena) makePlugins(w *world.Plugin) {
	s.world = w
	grid := grid.DefaultGrids{}.Square(GridWidth, GridHeight, CellSize)
	s.board = board.NewPlugin(grid, &cell.SingleOccupancy{}, s.world)
	s.cameras = cameras.NewPlugin(s.world)
	s.players = players.NewPlugin(s.world, s.cameras, s.board)
}

func (s *arena) definePlayer() error {
	s.player = s.players.Local("player")
	return s.player.Bind(s.players.Defaults()...)
}

func (s *arena) defineEffects() {
	s.world.Effects().Define(CalmEf, effect.Spec{
		effect.Described("Calmed: the ward's glow put out for a while."),
		effect.Lasts(2 * time.Second),
	})
}

func (s *arena) defineCommands() {
	s.world.Commands().Define(CalmCmd,
		rule.Cast(s.world.Effects().Named(CalmEf)).On(entity.Named(WardName)))
}

func (s *arena) defineCells() {
	s.board.CellKinds().Define(GrassCell, cell.Kind{Cost: 1, Allows: cell.Land})
}

type wardRow struct{ at geom.Vec }

func (s *arena) defineKinds() {
	kind.Define[wardRow](s.world.Kinds(), WardKind, kind.Spec{
		comp.Load(func(w wardRow) world.Position {
			return world.Position{
				AABB: plane.NewAABB(geom.NewVec(w.at.X-WardPx/2, w.at.Y-WardPx/2), WardPx, WardPx),
			}
		}),
		comp.Const(world.Velocity{}),
	})
}

func (s *arena) bindKeys() error {
	return s.player.Bind(
		control.Give(control.KeyPress{Key: control.KeyC}, "Calm the ward for a while",
			s.world.Commands().Named(CalmCmd)),
	)
}

func (s *arena) defineScenes() []game.Scene {
	m := &mainScene{arena: s}
	return []game.Scene{ui.NewScene(MainScene, m.screen()).Input(s.players.Handle)}
}

func (s *arena) spawnCells() {
	s.board.Seed(board.Layout{Default: GrassCell})
}

func (s *arena) spawnUnits() {
	wardKind := kind.Named[wardRow](s.world.Kinds(), WardKind)
	s.world.Seed(
		wardKind.Entry(wardRow{at: geom.NewVec(ScreenWidth/2, ScreenHeight/2)}).Named(WardName),
	)
}

func (s *arena) update(ctx goke.RunCtx, d time.Duration) {
	s.world.RunPlan(ctx, d)
	s.board.RunPlan(ctx, d)
	s.cameras.RunPlan(ctx, d)
	s.players.RunPlan(ctx, d)
	ctx.Sync()
}
