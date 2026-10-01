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
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/unit"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/players"
	"github.com/kjkrol/gram/plugins/players/owner"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/entity/kind"
	"github.com/kjkrol/gram/plugins/world/entity/kind/comp"
	"github.com/kjkrol/gram/plugins/world/entity/tag"
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
type Demo struct{ stage *mainStage }

var _ game.Game = (*Demo)(nil)

func NewDemo() *Demo { return &Demo{stage: &mainStage{}} }

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

// =========================== Commands ===========================

// Drive is the command to drive the issuing player's block one way this tick; the ways of the
// keys held add up.
type Drive struct{ Dir geom.Vec }

// =========================== Stage ===========================

// mainStage is the arena, its two blocks and the players driving them. It handles Drive itself.
type mainStage struct {
	world      *world.Plugin
	collision  *collision.Plugin
	board      *board.Plugin
	players    *players.Plugin
	redPlayer  *players.Player
	bluePlayer *players.Player
	redBlock   kind.Of[block]
	blueBlock  kind.Of[block]
	drives     control.Queue[Drive]
	drive      goke.Runnable
	follow     goke.Runnable
	minimapCam camera.Camera
	composer   *render.Composer
	stack      game.Scenes
}

var _ game.Stage = (*mainStage)(nil)

func (s *mainStage) Name() string       { return "split-screen-demo" }
func (s *mainStage) Stack() game.Scenes { return s.stack }

// picture is the one composer of the arena, shared by the players' views and the minimap.
func (s *mainStage) picture() *render.Composer {
	if s.composer == nil {
		s.composer = render.NewComposer(s.board.Renderer(), s.world.Renderer())
	}
	return s.composer
}

// Queues is where Drive lands — the stage is the handler of its own command.
func (s *mainStage) Queues() []control.CommandQueue { return []control.CommandQueue{&s.drives} }

// DefaultBindings is none: each player is bound to its own keys in Init.
func (s *mainStage) DefaultBindings() []control.Binding { return nil }

func (s *mainStage) Init(ctx game.Initializer) error {
	s.world = ctx.UseWorld(world.Config{
		Space:    world.SpaceCfg{Width: WorldWidth, Height: WorldHeight},
		Entities: world.EntitiesCfg{MaxCount: MaxEntCount, MinSize: BlockSize, MaxSize: BlockSize},
		Camera:   camera.Config{ViewportWidth: ScreenWidth / 2, ViewportHeight: ScreenHeight},
	})
	s.collision = collision.NewPlugin(s.world)
	if err := ctx.Use(s.collision); err != nil {
		return err
	}
	grid := grid.DefaultGrids{}.Square(GridWidth, GridHeight, CellSize)
	s.board = board.NewPlugin(grid, &cell.MultipleOccupancy{}, s.world).WithCollision(s.collision)
	s.board.CellKinds().Create(
		cell.Kind{Name: cell.Named("floor"), Cost: 1, Allows: cell.Land},
		cell.Kind{Name: cell.Named("wall"), Cost: 1, Solid: true},
	)
	if err := ctx.Use(s.board); err != nil {
		return err
	}

	s.players = players.NewPlugin(s.world, s)
	s.redPlayer = s.players.Local("red").OwnCamera()
	s.bluePlayer = s.players.Local("blue").OwnCamera()
	if err := s.redPlayer.Bind(driveKeys(control.KeyW, control.KeyS, control.KeyA, control.KeyD)...); err != nil {
		return err
	}
	if err := s.bluePlayer.Bind(driveKeys(control.KeyArrowUp, control.KeyArrowDown, control.KeyArrowLeft, control.KeyArrowRight)...); err != nil {
		return err
	}
	if err := ctx.Use(s.players); err != nil {
		return err
	}
	s.drive = ctx.RegSys(func() goke.System { return &driveSystem{drives: &s.drives} })
	s.follow = ctx.RegSys(func() goke.System { return &followSystem{players: s.players} })
	s.minimapCam = s.world.NewCamera()
	s.defineKinds()

	main := &mainScene{stage: s}
	minimap := &minimapScene{stage: s}
	stack, err := game.NewStack(main, minimap)
	if err != nil {
		return err
	}
	s.stack = stack
	comp := stack.Composition()
	comp.Show(main.Name())
	comp.Show(minimap.Name())
	return nil
}

// driveKeys binds up, down, left and right to Drive while held.
func driveKeys(up, down, left, right control.Key) []control.Binding {
	way := func(dx, dy float64) func(control.Context) (Drive, bool) {
		return func(control.Context) (Drive, bool) { return Drive{Dir: geom.NewVec(dx, dy)}, true }
	}
	return []control.Binding{
		control.Command(control.KeyHeld{Key: up}, "Drive up", way(0, -1)),
		control.Command(control.KeyHeld{Key: down}, "Drive down", way(0, 1)),
		control.Command(control.KeyHeld{Key: left}, "Drive left", way(-1, 0)),
		control.Command(control.KeyHeld{Key: right}, "Drive right", way(1, 0)),
	}
}

// block is the row a block spawns from: where it starts.
type block struct{ start cell.ID }

func (s *mainStage) defineKinds() {
	brd := s.board.Res.Logic.Board
	units := board.NewUnits[block](s.board, board.Shape{Size: BlockSize}, func(b block) geom.Vec { return brd.CellCenter(b.start) })
	profile := steering.Steering{MaxSpeed: BlockSpeed, Accel: BlockSpeed * 3, Brake: BlockSpeed * 6, TurnRate: 0.3}
	// each block is its player's: it takes that player's Drive alone
	s.redBlock = units.Define("red", unit.Mover{Domain: cell.Land}, profile, comp.Tagged(s.redPlayer.Owner()))
	s.blueBlock = units.Define("blue", unit.Mover{Domain: cell.Land}, profile, comp.Tagged(s.bluePlayer.Owner()))
}

func (s *mainStage) Restore(game.Persistence) (bool, error) { return false, nil }

// Spawn lays out the arena — walls round it and a few pillars and walls inside — and a block for
// each player in opposite corners.
func (s *mainStage) Spawn() error {
	brd := s.board.Res.Logic.Board
	cellAt := func(x, y uint32) cell.ID { c, _ := brd.CellIndex(x, y); return c }
	var cells []cell.Entry
	wall := func(x, y uint32) { cells = append(cells, cell.Entry{Kind: "wall", Cell: cellAt(x, y)}) }
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
	s.board.Seed(board.Layout{Default: "floor", Cells: cells})
	s.world.Seed(
		s.redBlock.Entry(block{start: cellAt(3, 3)}),
		s.blueBlock.Entry(block{start: cellAt(GridWidth-4, GridHeight-4)}),
	)
	return nil
}

func (s *mainStage) Update(ctx goke.RunCtx, d time.Duration) {
	ctx.Run(s.drive, d)
	ctx.Sync()
	s.world.RunPlan(ctx, d)
	s.collision.RunPlan(ctx, d)
	s.board.RunPlan(ctx, d)
	ctx.Run(s.follow, d)
	s.players.RunPlan(ctx, d)
	ctx.Sync()
}

// =========================== Systems ===========================

// driveSystem steers every block the way the Drive commands of the player who owns it add up to
// this tick, and brakes it to a stop when there were none.
type driveSystem struct {
	drives *control.Queue[Drive]
	want   map[control.PlayerID]geom.Vec

	query  *goke.Query
	owners goke.Comp[tag.Tags[owner.Family]]
	steer  goke.Comp[steering.Steering]
	course goke.Comp[steering.Course]
}

func (s *driveSystem) Init(si *goke.SysInit) {
	s.query = si.NewQueryBuilder(&s.owners, &s.steer, &s.course).Build()
	s.want = map[control.PlayerID]geom.Vec{}
}

func (s *driveSystem) Update(*goke.CmdBuf, time.Duration) {
	clear(s.want)
	s.drives.Drain(func(i control.Issued[Drive]) { s.want[i.Player] = s.want[i.Player].Add(i.Command.Dir) })
	for s.query.All(); s.query.Next(); {
		cur := s.query.Cursor()
		owners, steers, courses := s.owners.Slice(cur), s.steer.Slice(cur), s.course.Slice(cur)
		for i := range cur.IDs {
			st := steering.Helm{Steering: &steers[i], Course: &courses[i]}
			var dir geom.Vec
			for by, want := range s.want {
				if owner.Obeys(owners[i], by) {
					dir = dir.Add(want)
				}
			}
			if dir.X != 0 || dir.Y != 0 {
				st.Request(dir)
				st.RequestSpeed(st.MaxSpeed)
			} else {
				st.RequestSpeed(0)
			}
		}
	}
}

// followSystem centres each player's camera on the block it owns.
type followSystem struct {
	players *players.Plugin

	query  *goke.Query
	owners goke.Comp[tag.Tags[owner.Family]]
	base   goke.Comp[world.Base]
}

func (s *followSystem) Init(si *goke.SysInit) {
	s.query = si.NewQueryBuilder(&s.owners, &s.base).Build()
}

func (s *followSystem) Update(*goke.CmdBuf, time.Duration) {
	for s.query.All(); s.query.Next(); {
		cur := s.query.Cursor()
		owners, bases := s.owners.Slice(cur), s.base.Slice(cur)
		for i := range cur.IDs {
			for _, pl := range s.players.Players() {
				if owner.Obeys(owners[i], pl.ID) {
					c := bases[i].Pos.Center()
					pl.Camera.CenterOn(c.X, c.Y, 0)
				}
			}
		}
	}
}

// =========================== Scenes ===========================

var (
	colorFloor   = color.RGBA{R: 70, G: 80, B: 70, A: 255}
	colorWall    = color.RGBA{R: 30, G: 30, B: 35, A: 255}
	colorRed     = color.RGBA{R: 220, G: 80, B: 80, A: 255}
	colorBlue    = color.RGBA{R: 80, G: 130, B: 230, A: 255}
	colorDivider = color.RGBA{R: 240, G: 240, B: 240, A: 255}
)

// mainScene is the arena seen by both players, each in its half, with a line between the halves.
type mainScene struct {
	stage *mainStage
	right geom.AABB // the right half, as Viewports last laid it out
}

var _ game.Scene = (*mainScene)(nil)
var _ game.Viewer = (*mainScene)(nil)

func (m *mainScene) Name() string    { return "main" }
func (m *mainScene) Focusable() bool { return true }

func (m *mainScene) Layers() []render.Layer {
	s := m.stage
	worldAtlas := render.NewAtlas()
	worldAtlas.RegisterAt(s.redBlock.SpriteID(), BlockSize, render.Solid(colorRed))
	worldAtlas.RegisterAt(s.blueBlock.SpriteID(), BlockSize, render.Solid(colorBlue))
	worldAtlas.Close()
	s.world.WithRenderer(worldAtlas)

	floor, _ := s.board.CellKinds().Get("floor")
	wall, _ := s.board.CellKinds().Get("wall")
	boardAtlas := render.NewAtlas()
	boardAtlas.RegisterAt(floor.SpriteID, CellSize, render.Solid(colorFloor))
	boardAtlas.RegisterAt(wall.SpriteID, CellSize, render.Solid(colorWall))
	boardAtlas.Close()
	s.board.WithRenderer(boardAtlas)
	s.board.Res.Render.ShowGridLines = false

	return []render.Layer{s.picture(), divider{&m.right}}
}

// Viewports are the two players' halves; the right one is kept for the divider.
func (m *mainScene) Viewports(screen geom.AABB) []render.Viewport {
	vps := m.stage.players.Viewports(screen)
	if len(vps) > 1 {
		m.right = vps[1].Area
	}
	return vps
}

func (m *mainScene) HandleEvents(events *control.InputEvents, runtime game.Runtime, _ game.Composition) {
	m.stage.players.EventHandler().HandleEvents(events)
	for _, k := range events.KeyEvents {
		if k.Action != control.ActionPress {
			continue
		}
		switch k.Key {
		case control.KeyEscape:
			runtime.Quit()
		case control.KeySpace:
			runtime.TogglePause()
		}
	}
}

// divider draws the line between the halves, at the left edge of the right one, as the scene last
// laid them out.
type divider struct{ right *geom.AABB }

func (divider) Init(*goke.SysInit) {}

func (d divider) Draw(screen *render.Image) {
	if x := float32(d.right.TopLeft.X); x > 0 {
		render.StrokeLine(screen, x, 0, x, float32(screen.Bounds().Dy()), 2, colorDivider)
	}
}

// minimapScene shows the whole arena through a camera of its own, in a frame at the bottom of the
// screen; it never takes input.
type minimapScene struct {
	stage *mainStage
	area  geom.AABB
}

var _ game.Scene = (*minimapScene)(nil)
var _ game.Viewer = (*minimapScene)(nil)

func (m *minimapScene) Name() string    { return "minimap" }
func (m *minimapScene) Focusable() bool { return false }

func (m *minimapScene) HandleEvents(*control.InputEvents, game.Runtime, game.Composition) {}

// Layers are the picture the players' views draw, through the minimap's camera, and the frame.
func (m *minimapScene) Layers() []render.Layer {
	return []render.Layer{m.stage.picture(), frame{m}}
}

// Viewports is the minimap: the arena's proportions, MinimapWidth wide, at the bottom middle of
// the screen, the camera zoomed out until the whole arena fits.
func (m *minimapScene) Viewports(screen geom.AABB) []render.Viewport {
	const w, h = MinimapWidth, MinimapWidth * WorldHeight / WorldWidth
	x := math.Round((screen.TopLeft.X + screen.BottomRight.X - w) / 2)
	area := geom.NewAABBAt(geom.NewVec(x, screen.BottomRight.Y-h-10), w, h)
	if area.BottomRight.Sub(area.TopLeft) != m.area.BottomRight.Sub(m.area.TopLeft) {
		cam := m.stage.minimapCam
		cam.SetViewport(w, h)
		cam.ZoomOut(1e6, WorldWidth/2, WorldHeight/2)
		cam.CenterOn(WorldWidth/2, WorldHeight/2, 0)
	}
	m.area = area
	return []render.Viewport{{Camera: m.stage.minimapCam, Area: area}}
}

// frame outlines the minimap.
type frame struct{ m *minimapScene }

func (frame) Init(*goke.SysInit) {}

func (f frame) Draw(screen *render.Image) {
	a := f.m.area
	size := a.BottomRight.Sub(a.TopLeft)
	render.StrokeRect(screen, float32(a.TopLeft.X), float32(a.TopLeft.Y), float32(size.X), float32(size.Y), 2, colorDivider)
}
