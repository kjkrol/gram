// Command split-screen-demo is two players at one keyboard: red drives its block with WSAD, blue
// with the arrows, each through a camera of its own in its half of the screen, and a minimap at
// the bottom shows the whole arena through a camera nobody drives.
package main

import (
	"image/color"
	"math"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/game/stage"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/unit"
	"github.com/kjkrol/gram/plugins/cameras"
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
	GridWidth    = 40
	GridHeight   = 30
	CellSize     = 32
	WorldWidth   = GridWidth * CellSize
	WorldHeight  = GridHeight * CellSize
	ScreenWidth  = 1280
	ScreenHeight = 720
	BlockSize    = 24
	BlockSpeed   = CellSize * 5
	MaxEntCount  = 8 // the two blocks; the walls are cells, not entities

	// MinimapWidth is the minimap's width in pixels; its height keeps the arena's proportions.
	MinimapWidth = 240
)

// =========================== Game ===========================

// Demo is the split-screen demo, one Stage.
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
		Title:       "gram split-screen demo — red: WSAD, blue: arrows",
		ScreenWidth: ScreenWidth, ScreenHeight: ScreenHeight, Resizable: true,
		TargetTPS: TPS,
	}
}

func (d *Demo) Stages() (map[string]game.Stage, string) {
	return map[string]game.Stage{d.stage.Name(): d.stage}, d.stage.Name()
}

// =========================== Stage ===========================

// arena is the arena, its two blocks and the players driving them: each block is its player's,
// selected and followed by its camera from the start (kind.Entry.Told), driven by the player's
// own keys (players.DriveKeys) through navigation.
type arena struct {
	world      *world.Plugin
	collision  *collision.Plugin
	board      *board.Plugin
	selection  *selection.Plugin
	nav        *navigation.Plugin
	cameras    *cameras.Plugin
	players    *players.Plugin
	redPlayer  *players.Player
	bluePlayer *players.Player
	minimapCam camera.Camera
	composer   *render.Composer
}

// newStage defines the game a section at a time, each building on those before it.
// newArena makes the arena — the collector of the stage's plugins, which every
// section builds on — and defines the stage on it, a section at a time.
func newArena() (*arena, game.Stage) {
	s := &arena{}
	return s, stage.New("split-screen-demo").
		Plugins(s.usePlugins).
		Players(s.definePlayers).
		Cells(s.defineCells).
		Kinds(s.defineKinds).
		Controls(s.bindKeys).
		Scenes(s.defineScenes).
		Shows("main", "minimap").
		Layout(s.layOut).
		Units(s.placeUnits).
		Update(s.update)
}

// picture is the one composer of the arena, shared by the players' views and the minimap.
func (s *arena) picture() *render.Composer {
	if s.composer == nil {
		s.composer = render.NewComposer(s.board.Renderer(), s.world.Renderer())
	}
	return s.composer
}

func (s *arena) usePlugins(ctx game.Initializer) error {
	s.world = ctx.UseWorld(world.Config{
		Space:    world.SpaceCfg{Width: WorldWidth, Height: WorldHeight},
		Entities: world.EntitiesCfg{MaxCount: MaxEntCount, MinSize: BlockSize, MaxSize: BlockSize},
	})
	grid := grid.DefaultGrids{}.Square(GridWidth, GridHeight, CellSize)
	s.collision = collision.NewPlugin(s.world)
	s.board = board.NewPlugin(grid, &cell.MultipleOccupancy{}, s.world).WithCollision(s.collision)
	s.selection = selection.NewPlugin(s.world)
	s.nav = navigation.NewPlugin(s.board, s.world, s.selection).WithCollision(s.collision)
	s.cameras = cameras.NewPlugin(s.world, cameras.TopDown(), camera.Config{ViewportWidth: ScreenWidth / 2, ViewportHeight: ScreenHeight})
	s.players = players.NewPlugin(s.world, s.cameras, s.board, s.selection, s.nav)
	s.nav.WithPlayers(s.players)
	for _, p := range []plugin.Plugin{s.collision, s.board, s.selection, s.nav, s.cameras, s.players} {
		if err := ctx.Use(p); err != nil {
			return err
		}
	}
	s.minimapCam = s.cameras.New()
	return nil
}

func (s *arena) definePlayers() {
	s.redPlayer = s.players.Local("red", s.cameras.Main())
	s.bluePlayer = s.players.Local("blue", s.cameras.New())
}

func (s *arena) defineCells() {
	kinds := s.board.CellKinds()
	kinds.Define(FloorCell, cell.Kind{Cost: 1, Allows: cell.Land})
	kinds.Define(WallCell, cell.Kind{Cost: 1, Solid: true})
}

// bindKeys gives each player its own keys to drive its block the way they say: WSAD the red,
// the arrows the blue.
func (s *arena) bindKeys() error {
	if err := s.redPlayer.Bind(players.DriveKeys(control.KeyW, control.KeyS, control.KeyA, control.KeyD)...); err != nil {
		return err
	}
	// the keyboard is one: the game's keys and the world's (Space pauses) are bound once
	if err := s.redPlayer.Bind(append(players.GameBindings(), s.world.DefaultBindings()...)...); err != nil {
		return err
	}
	return s.bluePlayer.Bind(players.DriveKeys(control.KeyArrowUp, control.KeyArrowDown, control.KeyArrowLeft, control.KeyArrowRight)...)
}

func (s *arena) defineScenes() []game.Scene {
	return []game.Scene{&mainScene{arena: s}, &minimapScene{arena: s}}
}

// block is the row a block spawns from: where it starts.
type block struct{ start cell.ID }

func (s *arena) defineKinds() {
	brd := s.board.Res.Logic.Board
	units := board.NewUnits[block](s.board, board.Shape{Size: BlockSize}, func(b block) geom.Vec { return brd.CellCenter(b.start) })
	profile := steering.Steering{MaxSpeed: BlockSpeed, Accel: BlockSpeed * 3, Brake: BlockSpeed * 6, TurnRate: 0.3}
	// each block is its player's: it takes that player's hand alone
	units.Define(RedKind, unit.Mover{Domain: cell.Land}, profile)
	units.Define(BlueKind, unit.Mover{Domain: cell.Land}, profile)
}

func (s *arena) layOut() {
	cellAt := s.board.Res.Logic.Board.CellIndex
	var cells []cell.Entry
	wall := func(x, y uint32) { cells = append(cells, cell.Entry{Kind: WallCell, Cell: cellAt(x, y)}) }
	for x := uint32(0); x < GridWidth; x++ {
		wall(x, 0)
		wall(x, GridHeight-1)
	}
	for y := uint32(1); y < GridHeight-1; y++ {
		wall(0, y)
		wall(GridWidth-1, y)
	}
	for y := uint32(6); y < 24; y++ {
		wall(20, y)
	}
	for x := uint32(8); x < 16; x++ {
		wall(x, 10)
		wall(x+16, 20)
	}
	for _, p := range [][2]uint32{{6, 20}, {10, 24}, {30, 6}, {34, 12}, {26, 26}, {14, 4}} {
		wall(p[0], p[1])
		wall(p[0]+1, p[1])
		wall(p[0], p[1]+1)
		wall(p[0]+1, p[1]+1)
	}
	s.board.Seed(board.Layout{Default: FloorCell, Cells: cells})
}

func (s *arena) placeUnits() {
	redKind := kind.Named[block](s.world.Kinds(), RedKind)
	blueKind := kind.Named[block](s.world.Kinds(), BlueKind)
	brd := s.board.Res.Logic.Board
	// each block its player's, selected — the player's hand is on it — and followed by its camera
	s.world.Seed(
		redKind.Entry(block{start: brd.CellIndex(3, 3)}).Told(players.Give{To: s.redPlayer.ID}, selection.Allow{Selected: true}, cameras.Follow{Camera: s.redPlayer.Camera, On: true}),
		blueKind.Entry(block{start: brd.CellIndex(GridWidth-4, GridHeight-4)}).Told(players.Give{To: s.bluePlayer.ID}, selection.Allow{Selected: true}, cameras.Follow{Camera: s.bluePlayer.Camera, On: true}),
	)
}

func (s *arena) update(ctx goke.RunCtx, d time.Duration) {
	s.world.RunPlan(ctx, d)
	s.collision.RunPlan(ctx, d)
	s.board.RunPlan(ctx, d)
	s.nav.RunPlan(ctx, d)
	s.selection.RunPlan(ctx, d)
	s.cameras.RunPlan(ctx, d)
	s.players.RunPlan(ctx, d)
	ctx.Sync()
}

// =========================== Scenes ===========================

var (
	floorColor   = color.RGBA{R: 70, G: 80, B: 70, A: 255}
	wallColor    = color.RGBA{R: 30, G: 30, B: 35, A: 255}
	redColor     = color.RGBA{R: 220, G: 80, B: 80, A: 255}
	blueColor    = color.RGBA{R: 80, G: 130, B: 230, A: 255}
	dividerColor = color.RGBA{R: 240, G: 240, B: 240, A: 255}
)

// mainScene is the arena seen by both players, each in its half, with a line between the halves.
type mainScene struct {
	arena *arena
	right geom.AABB // the right half, as Viewports last laid it out
}

var _ game.Scene = (*mainScene)(nil)
var _ game.Viewer = (*mainScene)(nil)

func (m *mainScene) Name() string    { return "main" }
func (m *mainScene) Focusable() bool { return true }

func (m *mainScene) Layers() []render.Layer {
	s := m.arena
	redKind := kind.Named[block](s.world.Kinds(), RedKind)
	blueKind := kind.Named[block](s.world.Kinds(), BlueKind)
	worldAtlas := render.NewAtlas()
	worldAtlas.Add(redKind, BlockSize, render.Solid(redColor))
	worldAtlas.Add(blueKind, BlockSize, render.Solid(blueColor))
	worldAtlas.Close()
	s.world.WithRenderer(worldAtlas)

	boardAtlas := s.board.NewAtlas(CellSize)
	boardAtlas.Add(FloorCell, render.Solid(floorColor))
	boardAtlas.Add(WallCell, render.Solid(wallColor))
	boardAtlas.Close()
	s.board.WithRenderer(boardAtlas)
	s.board.Res.Render.ShowGridLines = false

	return []render.Layer{s.picture(), divider{&m.right}}
}

// Viewports are the two players' halves; the right one is kept for the divider.
func (m *mainScene) Viewports(screen geom.AABB) []render.Viewport {
	vps := m.arena.players.Viewports(screen)
	if len(vps) > 1 {
		m.right = vps[1].Area
	}
	return vps
}

func (m *mainScene) HandleEvents(events *control.InputEvents, runtime game.Runtime, composition game.Composition) {
	m.arena.players.Handle(events, runtime, composition)
}

// divider draws the line between the halves, at the left edge of the right one, as the scene last
// laid them out.
type divider struct{ right *geom.AABB }

func (divider) Init(*goke.SysInit) {}

func (d divider) Draw(screen *render.Image) {
	if x := float32(d.right.TopLeft.X); x > 0 {
		render.StrokeLine(screen, x, 0, x, float32(screen.Bounds().Dy()), 2, dividerColor)
	}
}

// minimapScene shows the whole arena through a camera of its own, in a frame at the bottom of the
// screen; it never takes input.
type minimapScene struct {
	arena *arena
	area  geom.AABB
}

var _ game.Scene = (*minimapScene)(nil)
var _ game.Viewer = (*minimapScene)(nil)

func (m *minimapScene) Name() string    { return "minimap" }
func (m *minimapScene) Focusable() bool { return false }

func (m *minimapScene) HandleEvents(*control.InputEvents, game.Runtime, game.Composition) {}

// Layers are the picture the players' views draw, through the minimap's camera, and the frame.
func (m *minimapScene) Layers() []render.Layer {
	return []render.Layer{m.arena.picture(), frame{m}}
}

// Viewports is the minimap: the arena's proportions, MinimapWidth wide, at the bottom middle of
// the screen, the camera zoomed out until the whole arena fits.
func (m *minimapScene) Viewports(screen geom.AABB) []render.Viewport {
	const w, h = MinimapWidth, MinimapWidth * WorldHeight / WorldWidth
	x := math.Round((screen.TopLeft.X + screen.BottomRight.X - w) / 2)
	area := geom.NewAABBAt(geom.NewVec(x, screen.BottomRight.Y-h-10), w, h)
	if area.BottomRight.Sub(area.TopLeft) != m.area.BottomRight.Sub(m.area.TopLeft) {
		cam := m.arena.minimapCam
		cam.SetViewport(w, h)
		cam.ZoomOut(1e6, WorldWidth/2, WorldHeight/2)
		cam.CenterOn(WorldWidth/2, WorldHeight/2, 0)
	}
	m.area = area
	return []render.Viewport{{Camera: m.arena.minimapCam, Area: area}}
}

// frame outlines the minimap.
type frame struct{ m *minimapScene }

func (frame) Init(*goke.SysInit) {}

func (f frame) Draw(screen *render.Image) {
	a := f.m.area
	size := a.BottomRight.Sub(a.TopLeft)
	render.StrokeRect(screen, float32(a.TopLeft.X), float32(a.TopLeft.Y), float32(size.X), float32(size.Y), 2, dividerColor)
}
