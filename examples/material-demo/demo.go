// The material demo: a ward of the game's own WGSL pulses inside a stone ring
// (render.RegisterMaterials — the game's material joins the composer's one shader), and darts
// patrol round it, each drawn turned the way it moves (world.Turning: Appearance.Angle from
// Vel.Dir, the engine's one angle convention — 0 east, against the clock).
package main

import (
	"embed"
	"image/color"
	"log"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/entity/kind/comp"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/game/stage"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/unit"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/navigation"
	"github.com/kjkrol/gram/plugins/players"
	"github.com/kjkrol/gram/plugins/selection"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/steering"
	"github.com/kjkrol/gram/render"
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
	MaxEntCount  = 8
)

// The ward: where its centre lies on the board and how far it reaches, in cells.
const (
	wardX, wardY uint32 = 12, 8
	wardReach           = 3.2
	ringRadius          = 4 // the stone ring round it
	roundRadius         = 6 // the darts' patrol round, outside the ring
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
		Title:       "gram — a ward of the game's own WGSL, darts turned the way they fly",
		ScreenWidth: ScreenWidth, ScreenHeight: ScreenHeight, Resizable: true,
		TargetTPS: TPS,
	}
}

func (d *Demo) Stages() (map[string]game.Stage, string) {
	return map[string]game.Stage{d.stage.Name(): d.stage}, d.stage.Name()
}

// =========================== Stage ===========================

type unitRow struct {
	start cell.ID
	round [4]cell.ID
}

type arena struct {
	world     *world.Plugin
	board     *board.Plugin
	nav       *navigation.Plugin
	collision *collision.Plugin
	selection *selection.Plugin
	players   *players.Plugin
	player    *players.Player
}

// newArena makes the arena — the collector of the stage's plugins, which every
// section builds on — and defines the stage on it, a section at a time.
func newArena() (*arena, game.Stage) {
	s := &arena{}
	return s, stage.New("material-demo").
		Plugins(s.usePlugins).
		Players(s.definePlayer).
		Cells(s.defineCells).
		Kinds(s.defineKinds).
		Looks(s.defineLooks).
		Scenes(s.defineScenes).
		Layout(s.layOut).
		Units(s.placeUnits).
		Update(s.update)
}

func (s *arena) usePlugins(ctx game.Initializer) error {
	s.world = ctx.UseWorld(world.Config{
		Space:    world.SpaceCfg{Width: ScreenWidth, Height: ScreenHeight},
		Entities: world.EntitiesCfg{MaxCount: MaxEntCount, MinSize: EntitySize, MaxSize: EntitySize},
	})
	s.collision = collision.NewPlugin(s.world)
	grid := grid.DefaultGrids{}.Square(GridWidth, GridHeight, CellSize)
	s.board = board.NewPlugin(grid, &cell.SingleOccupancy{}, s.world).WithCollision(s.collision).WithLog(log.Default())
	s.selection = selection.NewPlugin(s.world)
	s.nav = navigation.NewPlugin(s.board, s.world, s.selection).WithCollision(s.collision)
	s.players = players.NewPlugin(s.world, s.board, s.selection, s.nav)
	for _, p := range []plugin.Plugin{s.collision, s.board, s.selection, s.nav, s.players} {
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
	kinds := s.board.CellKinds()
	kinds.Define(GrassCell, cell.Kind{Cost: 2, Allows: cell.Land})
	kinds.Define(StoneCell, cell.Kind{Cost: 1, Solid: true})
}

func (s *arena) defineKinds() {
	brd := s.board.Res.Logic.Board
	units := board.NewUnits[unitRow](s.board, board.Shape{Size: EntitySize}, func(u unitRow) geom.Vec { return brd.CellCenter(u.start) })
	profile := steering.Steering{MaxSpeed: UnitSpeed, Accel: UnitSpeed * 2, Brake: UnitSpeed * 4, V0: UnitSpeed / 2, TurnRate: 0.08}
	round := comp.Load(func(u unitRow) navigation.MoveOrder {
		return navigation.Patrol(0, u.round[0], u.round[1], u.round[2], u.round[3])
	})
	units.Define(DartKind, unit.Mover{Domain: cell.Land}, profile, round)
}

// defineLooks turns every dart the way it moves: Appearance.Angle from Vel.Dir, smoothly, in
// place of directional twins.
func (s *arena) defineLooks() error {
	return s.world.Draw(world.Turning())
}

func (s *arena) defineScenes() []game.Scene {
	return []game.Scene{&mainScene{arena: s}}
}

func (s *arena) layOut() {
	brd := s.board.Res.Logic.Board
	stone := s.board.CellKinds().Named(StoneCell)
	var cells []cell.Entry
	brd.EachCell(func(c cell.ID) { // the stone ring round the ward
		x, y, _ := brd.Coords(c)
		dx, dy := int(x)-int(wardX), int(y)-int(wardY)
		if r := dx*dx + dy*dy; r >= (ringRadius-1)*(ringRadius-1) && r <= ringRadius*ringRadius {
			cells = append(cells, stone.Entry(c))
		}
	})
	s.board.Seed(board.Layout{Default: GrassCell, Cells: cells})
}

func (s *arena) placeUnits() {
	brd := s.board.Res.Logic.Board
	dartKind := kind.Named[unitRow](s.world.Kinds(), DartKind)
	player := []any{players.Give{To: s.player.ID}, selection.Allow{}}
	// the darts' round: the four corners of a square outside the ring, one dart started at each
	corners := [4]cell.ID{
		brd.CellIndex(wardX-roundRadius, wardY-roundRadius),
		brd.CellIndex(wardX+roundRadius, wardY-roundRadius),
		brd.CellIndex(wardX+roundRadius, wardY+roundRadius),
		brd.CellIndex(wardX-roundRadius, wardY+roundRadius),
	}
	for i := range corners {
		var round [4]cell.ID
		for j := range round {
			round[j] = corners[(i+1+j)%4]
		}
		s.world.Seed(dartKind.Entry(unitRow{start: corners[i], round: round}).Told(player...))
	}
}

func (s *arena) update(ctx goke.RunCtx, d time.Duration) {
	s.world.RunPlan(ctx, d)
	s.collision.RunPlan(ctx, d)
	s.board.RunPlan(ctx, d)
	s.nav.RunPlan(ctx, d)
	s.selection.RunPlan(ctx, d)
	s.players.RunPlan(ctx, d)
	ctx.Sync()
}

// =========================== Scene ===========================

type mainScene struct {
	arena *arena
}

var _ game.Scene = (*mainScene)(nil)

func (m *mainScene) Name() string { return "main" }

// The scene's colours: the darts — head and shaft, drawn facing east, within the circle
// inscribed in their box, so the turned look stays inside it — and the ground.
var (
	dartColor     = color.RGBA{R: 240, G: 220, B: 120, A: 255}
	dartHeadColor = color.RGBA{R: 250, G: 245, B: 220, A: 255}
	grassColor    = color.RGBA{R: 60, G: 95, B: 60, A: 255}
	stoneColor    = color.RGBA{R: 110, G: 110, B: 120, A: 255}
)

func (m *mainScene) Layers() []render.Layer {
	s := m.arena
	dartKind := kind.Named[unitRow](s.world.Kinds(), DartKind)

	worldAtlas := render.NewAtlas()
	worldAtlas.Add(dartKind, EntitySize, func(dst *render.Canvas, size int) {
		render.Arrow(0, 3, dartColor)(dst, size) // east: the way Angle 0 points
		render.Dot(3, dartHeadColor)(dst, size)
	})
	worldAtlas.Close()
	s.world.WithRenderer(worldAtlas)

	boardAtlas := s.board.NewAtlas(CellSize)
	boardAtlas.Add(GrassCell, render.Solid(grassColor))
	boardAtlas.Add(StoneCell, render.Solid(stoneColor))
	boardAtlas.Close()
	s.board.WithRenderer(boardAtlas)

	s.nav.WithRenderer(nil)
	s.selection.WithRenderer(nil)

	ward := &wardSource{brd: s.board.Res.Logic.Board}
	return []render.Layer{render.NewComposer(s.board.Renderer(), ward, s.world.Renderer(), s.selection.Renderer(), s.nav.Renderer())}
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
// per pixel, between the ground and the units.
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
