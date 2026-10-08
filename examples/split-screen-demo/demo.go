// Command split-screen-demo is two players at one keyboard: red drives its block with WSAD, blue
// with the arrows, each through a camera of its own in its half of the screen, and a minimap at
// the bottom shows the whole arena through a camera nobody drives.
package main

import (
	"image/color"
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
	"github.com/kjkrol/gram/plugins/driving"
	"github.com/kjkrol/gram/plugins/navigation"
	"github.com/kjkrol/gram/plugins/players"
	"github.com/kjkrol/gram/plugins/selection"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/steering"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/ui"
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
// own keys (driving.Compass).
type arena struct {
	world      *world.Plugin
	collision  *collision.Plugin
	board      *board.Plugin
	selection  *selection.Plugin
	nav        *navigation.Plugin
	driving    *driving.Plugin
	cameras    *cameras.Plugin
	players    *players.Plugin
	redPlayer  *players.Player
	bluePlayer *players.Player
	minimapCam camera.Camera
	picture    *render.Composer // the arena, as every half and the minimap show it
}

// newStage defines the game a section at a time, each building on those before it.
// newArena makes the arena — the collector of the stage's plugins, which every
// section builds on — and defines the stage on it, a section at a time.
func newArena() (*arena, game.Stage) {
	s := &arena{}
	return s, stage.New(SplitScreenStage).
		Plugins(s.usePlugins).
		Players(s.definePlayers).
		Kinds(s.defineCells, s.defineKinds).
		Controls(s.bindKeys).
		Spawn(s.spawnCells, s.spawnUnits).
		Scenes(s.defineScenes).
		Update(s.update)
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
	s.driving = driving.NewPlugin(s.world, s.selection).WithGround(s.board)
	s.nav = navigation.NewPlugin(s.board, s.world, s.selection, s.driving).WithCollision(s.collision)
	s.cameras = cameras.NewPlugin(s.world)
	s.players = players.NewPlugin(s.world, s.cameras, s.board, s.selection, s.nav, s.driving)
	for _, p := range []plugin.Plugin{s.collision, s.board, s.selection, s.nav, s.driving, s.cameras, s.players} {
		if err := ctx.Use(p); err != nil {
			return err
		}
	}
	s.minimapCam = s.cameras.New(cameras.TopDown(), camera.Config{ViewportWidth: MinimapWidth, ViewportHeight: MinimapWidth * WorldHeight / WorldWidth}) // the arena whole, as the minimap shows it
	return nil
}

func (s *arena) definePlayers() {
	half := camera.Config{ViewportWidth: ScreenWidth / 2, ViewportHeight: ScreenHeight} // each its own half of the screen
	s.redPlayer = s.players.Local("red", s.cameras.New(cameras.TopDown(), half))
	s.bluePlayer = s.players.Local("blue", s.cameras.New(cameras.TopDown(), half))
}

func (s *arena) defineCells() {
	kinds := s.board.CellKinds()
	kinds.Define(FloorCell, cell.Kind{Cost: 1, Allows: cell.Land})
	kinds.Define(WallCell, cell.Kind{Cost: 1, Solid: true})
}

// bindKeys gives each player its own keys to drive its block the way they say — WSAD the red,
// the arrows the blue — and to follow it or let its camera go: C the red, Enter the blue.
func (s *arena) bindKeys() error {
	red := driving.Compass{Up: control.KeyW, Down: control.KeyS, Left: control.KeyA, Right: control.KeyD, In: camera.Outside}
	if err := s.redPlayer.Bind(append(red.Bindings(), s.selection.FollowKey(control.KeyC))...); err != nil {
		return err
	}
	// the keyboard is one: the game's keys and the world's (Space pauses) are bound once
	if err := s.redPlayer.Bind(append(players.GameBindings(), s.world.DefaultBindings()...)...); err != nil {
		return err
	}
	blue := driving.Compass{Up: control.KeyArrowUp, Down: control.KeyArrowDown, Left: control.KeyArrowLeft, Right: control.KeyArrowRight, In: camera.Outside}
	return s.bluePlayer.Bind(append(blue.Bindings(), s.selection.FollowKey(control.KeyEnter))...)
}

func (s *arena) defineScenes() []game.Scene {
	return []game.Scene{ui.NewScene(MainScene, s.pictures, s.screen).Input(s.players.Handle)}
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

func (s *arena) spawnCells() {
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

func (s *arena) spawnUnits() {
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
	s.driving.RunPlan(ctx, d)
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

// pictures dresses the arena — its blocks and its cells — and hands its one picture, which the
// players' halves and the minimap all show.
func (s *arena) pictures() []render.Picture {
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

	s.picture = render.NewComposer(s.board.Renderer(), s.world.Renderer())
	return []render.Picture{s.picture}
}

// screen is each player's half, a line between, and the minimap at the bottom over them, the arena
// whole.
func (s *arena) screen() *ui.Element {
	s.minimapCam.ZoomOut(1e6, WorldWidth/2, WorldHeight/2)
	s.minimapCam.CenterOn(WorldWidth/2, WorldHeight/2, 0)
	red := render.NewFeed(s.redPlayer.Camera, s.picture)
	blue := render.NewFeed(s.bluePlayer.Camera, s.picture)
	minimap := render.NewFeed(s.minimapCam, s.picture)
	return ui.Layers( // from the bottom up: each covers those before it
		ui.Columns(
			ui.Share(1, ui.Image(red).Input(s.players.Through(s.redPlayer))),   // the left half
			ui.Fixed(2, ui.Blank().Fill(dividerColor)),                         // the line between
			ui.Share(1, ui.Image(blue).Input(s.players.Through(s.bluePlayer))), // the right half
		),
		ui.BottomMiddle(ui.Image(minimap).Border(dividerColor, 2)).Size(MinimapWidth, MinimapWidth*WorldHeight/WorldWidth).Margin(10),
	)
}
