// The material demo: one thing and one thing only — a material of the game's own WGSL. The ward
// joins the composer's single shader at package set-up (render.RegisterMaterials), a demo-local
// render.Source lays its quad each frame (render.Frame.Material), and the shader works every
// pixel out in the world's coordinates, on the composer's Clock.
package main

import (
	"embed"
	"image/color"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/game/stage"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/players"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

const (
	TPS          = 60
	GridWidth    = 24
	GridHeight   = 16
	CellSize     = 32
	ScreenWidth  = GridWidth * CellSize
	ScreenHeight = GridHeight * CellSize
)

// The ward: where its centre lies on the board and how far it reaches, in cells.
const (
	wardX, wardY uint32 = 12, 8
	wardReach           = 4.5
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
		Title:       "gram — a ward of the game's own WGSL",
		ScreenWidth: ScreenWidth, ScreenHeight: ScreenHeight, Resizable: true,
		TargetTPS: TPS,
	}
}

func (d *Demo) Stages() (map[string]game.Stage, string) {
	return map[string]game.Stage{d.stage.Name(): d.stage}, d.stage.Name()
}

// =========================== Stage ===========================

type arena struct {
	world   *world.Plugin
	board   *board.Plugin
	players *players.Plugin
	player  *players.Player
}

// newArena makes the arena — the collector of the stage's plugins, which every
// section builds on — and defines the stage on it, a section at a time.
func newArena() (*arena, game.Stage) {
	s := &arena{}
	return s, stage.New("material-demo").
		Plugins(s.usePlugins).
		Players(s.definePlayer).
		Cells(s.defineCells).
		Scenes(s.defineScenes).
		Layout(s.layOut).
		Update(s.update)
}

func (s *arena) usePlugins(ctx game.Initializer) error {
	s.world = ctx.UseWorld(world.Config{
		Space:    world.SpaceCfg{Width: ScreenWidth, Height: ScreenHeight},
		Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: CellSize},
	})
	grid := grid.DefaultGrids{}.Square(GridWidth, GridHeight, CellSize)
	s.board = board.NewPlugin(grid, &cell.SingleOccupancy{}, s.world)
	s.players = players.NewPlugin(s.world, s.board)
	for _, p := range []plugin.Plugin{s.board, s.players} {
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

func (s *arena) defineCells() {
	s.board.CellKinds().Define(GrassCell, cell.Kind{Cost: 1, Allows: cell.Land})
}

func (s *arena) defineScenes() []game.Scene {
	return []game.Scene{&mainScene{arena: s}}
}

func (s *arena) layOut() {
	s.board.Seed(board.Layout{Default: GrassCell})
}

func (s *arena) update(ctx goke.RunCtx, d time.Duration) {
	s.world.RunPlan(ctx, d)
	s.board.RunPlan(ctx, d)
	s.players.RunPlan(ctx, d)
	ctx.Sync()
}

// =========================== Scene ===========================

type mainScene struct {
	arena *arena
}

var _ game.Scene = (*mainScene)(nil)

func (m *mainScene) Name() string { return "main" }

// grassColor is the meadow the ward glows over: the one colour the scene names.
var grassColor = color.RGBA{R: 60, G: 95, B: 60, A: 255}

func (m *mainScene) Layers() []render.Layer {
	s := m.arena

	boardAtlas := s.board.NewAtlas(CellSize)
	boardAtlas.Add(GrassCell, render.Solid(grassColor))
	boardAtlas.Close()
	s.board.WithRenderer(boardAtlas)

	ward := &wardSource{brd: s.board.Res.Logic.Board}
	return []render.Layer{render.NewComposer(s.board.Renderer(), ward)}
}

func (m *mainScene) Viewports(screen geom.AABB) []render.Viewport {
	return m.arena.players.Viewports(screen)
}

func (m *mainScene) HandleEvents(events *control.InputEvents, runtime game.Runtime, composition game.Composition) {
	m.arena.players.Handle(events, runtime, composition)
}

func (m *mainScene) Focusable() bool { return true }

// =========================== The ward ===========================

// wardSource lays the ward on the frame every frame: one quad for the Ward material to work out
// per pixel, over the ground.
type wardSource struct {
	brd *board.Board
}

var _ render.Source = (*wardSource)(nil)

func (w *wardSource) Init(*goke.SysInit) {}

func (w *wardSource) Compose(f *render.Frame, cam camera.Camera) {
	centre := w.brd.CellCenter(w.brd.CellIndex(wardX, wardY))
	cx, cy := float32(centre.X), float32(centre.Y)
	reach := float32(wardReach * CellSize)
	x0, y0, x1, y1 := cx-reach, cy-reach, cx+reach, cy+reach
	custom := [4]float32{cx, cy, reach, 0}
	f.Material(render.Ground+50, 0, render.ProjectCorners(cam, x0, y0, x1, y1, 0), &render.Overlay{
		Material: wardMaterial,
		World:    render.Box(x0, y0, x1, y1),
		Red:      [4]float32{1, 1, 1, 1},
		Custom:   [4][4]float32{custom, custom, custom, custom},
	})
}
