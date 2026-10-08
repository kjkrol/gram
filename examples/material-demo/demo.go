// The material demo: instead of a sprite, an entity drawn by a material of the game's own WGSL.
// The ward is an ordinary entity of the world, placed on the board; its look is the Ward material
// (render.RegisterMaterials joins it to the composer's one shader; the world's atlas declares it:
// Add(wardKind, …, wardMaterial)), worked out per pixel in the entity's box on the composer's
// Clock. Its state changes its look the usual way — C casts calm on it, and under the calm effect
// the ward is a plain ashen sprite until the calm wears off.
package main

import (
	"embed"
	"image/color"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/entity"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/entity/kind/comp"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/game/stage"
	"github.com/kjkrol/gram/plugin"
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
	// calmFor is how long C keeps the ward calm — a plain sprite in the material's place.
	calmFor = 2 * time.Second
)

//go:embed ward.wgsl
var wardFS embed.FS

// wardMaterial joins the composer's one shader as the package is set up, before it compiles.
var wardMaterial = render.RegisterMaterials(render.Files(wardFS, "ward.wgsl"), nil, "Ward")[0]

// =========================== Game ===========================

type Demo struct {
	a     *arena
	stage game.Stage
}

var _ game.Game = (*Demo)(nil)

func NewDemo() *Demo {
	a, st := newArena()
	return &Demo{a: a, stage: st}
}

func (d *Demo) Props() game.Props {
	return game.Props{
		Title:       "gram — an entity drawn by a material of the game's own WGSL; C calms it",
		ScreenWidth: ScreenWidth, ScreenHeight: ScreenHeight, Resizable: true,
		TargetTPS: TPS,
	}
}

func (d *Demo) Stages() (map[string]game.Stage, string) {
	return map[string]game.Stage{d.stage.Name(): d.stage}, d.stage.Name()
}

// =========================== Stage ===========================

// wardRow is the row the ward spawns from: where it stands.
type wardRow struct{ at geom.Vec }

type arena struct {
	world   *world.Plugin
	board   *board.Plugin
	players *players.Plugin
	cameras *cameras.Plugin
	player  *players.Player
}

// newArena makes the arena — the collector of the stage's plugins, which every
// section builds on — and defines the stage on it, a section at a time.
func newArena() (*arena, game.Stage) {
	s := &arena{}
	return s, stage.New(MaterialStage).
		Plugins(s.usePlugins).
		Players(s.definePlayer).
		Effects(s.defineEffects).
		Commands(s.defineCommands).
		Kinds(s.defineCells, s.defineKinds).
		Controls(s.bindKeys).
		Spawn(s.spawnCells, s.spawnUnits).
		Scenes(s.defineScenes).
		Update(s.update)
}

func (s *arena) usePlugins(ctx game.Initializer) error {
	s.world = ctx.UseWorld(world.Config{
		Space:    world.SpaceCfg{Width: ScreenWidth, Height: ScreenHeight},
		Entities: world.EntitiesCfg{MaxCount: 1, MinSize: CellSize, MaxSize: WardPx},
	})
	grid := grid.DefaultGrids{}.Square(GridWidth, GridHeight, CellSize)
	s.board = board.NewPlugin(grid, &cell.SingleOccupancy{}, s.world)
	s.cameras = cameras.NewPlugin(s.world)
	s.players = players.NewPlugin(s.world, s.cameras, s.board)
	for _, p := range []plugin.Plugin{s.board, s.cameras, s.players} {
		if err := ctx.Use(p); err != nil {
			return err
		}
	}
	return nil
}

func (s *arena) definePlayer() error {
	s.player = s.players.Local("player")
	return s.player.Bind(s.players.Defaults()...)
}

func (s *arena) defineEffects() {
	s.world.Effects().Define(CalmEf, effect.Spec{
		effect.Described("Calmed: the ward's glow put out for a while."),
		effect.Lasts(calmFor),
	})
}

func (s *arena) defineCommands() {
	s.world.Commands().Define(CalmCmd,
		rule.Cast(s.world.Effects().Named(CalmEf)).On(entity.Named(WardName)))
}

func (s *arena) defineCells() {
	s.board.CellKinds().Define(GrassCell, cell.Kind{Cost: 1, Allows: cell.Land})
}

func (s *arena) defineKinds() {
	kind.Define[wardRow](s.world.Kinds(), WardKind, kind.Spec{
		comp.Load(func(w wardRow) world.Position {
			return world.Position{AABB: plane.NewAABB(geom.NewVec(w.at.X-WardPx/2, w.at.Y-WardPx/2), WardPx, WardPx)}
		}),
		comp.Const(world.Velocity{}),
	})
}

func (s *arena) bindKeys() error {
	return s.player.Bind(control.Give(control.KeyPress{Key: control.KeyC}, "Calm the ward for a while",
		s.world.Commands().Named(CalmCmd)))
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
	s.world.Seed(wardKind.Entry(wardRow{at: geom.NewVec(ScreenWidth/2, ScreenHeight/2)}).Named(WardName))
}

func (s *arena) update(ctx goke.RunCtx, d time.Duration) {
	s.world.RunPlan(ctx, d)
	s.board.RunPlan(ctx, d)
	s.cameras.RunPlan(ctx, d)
	s.players.RunPlan(ctx, d)
	ctx.Sync()
}

// =========================== Scene ===========================

type mainScene struct {
	arena *arena
}

// The scene's colours: the meadow and the calmed ward's ash.
var (
	grassColor = color.RGBA{R: 60, G: 95, B: 60, A: 255}
	ashColor   = color.RGBA{R: 120, G: 115, B: 110, A: 255}
)

// picture dresses the world and hands its picture.
func (m *mainScene) picture() render.Picture {
	s := m.arena
	wardKind := kind.Named[wardRow](s.world.Kinds(), WardKind)
	calm := s.world.Effects().Named(CalmEf)

	worldAtlas := s.world.NewAtlas()
	worldAtlas.Add(wardKind, WardPx, wardMaterial). // the look is a material, not a sprite
							Under(calm, render.Dot(8, ashColor)) // calmed: a plain sprite until it wears off
	worldAtlas.Close()
	s.world.WithRenderer(worldAtlas)

	boardAtlas := s.board.NewAtlas(CellSize)
	boardAtlas.Add(GrassCell, render.Solid(grassColor))
	boardAtlas.Close()
	s.board.WithRenderer(boardAtlas)

	return render.NewComposer(s.board.Renderer(), s.world.Renderer())
}

// screen is the world through a camera of its own, the player's view.
func (m *mainScene) screen() *ui.Element {
	s := m.arena
	return ui.Image(render.NewFeed(s.cameras.New(cameras.TopDown(), camera.Config{}), m.picture())).Input(s.players.Through(s.player))
}
