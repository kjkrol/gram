package main

import (
	"image/color"
	"log"
	"math"
	"slices"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
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

// The board is a parallelogram of pointy-top hexes in axial (q, r) coordinates: every row
// shifts right by half a hex, so the world is as wide as the last row reaches and the triangles
// either side hold no cells.
const (
	TPS        = 60
	GridWidth  = 20
	GridHeight = 12
	HexSize    = 24 // circumradius
	EntitySize = 22
	UnitSpeed  = HexSize * 3
	// MaxEntCount is the units; the walls are cells, not entities.
	MaxEntCount = 16

	saveBasePath = "board-navigation-hex-demo"
)

var (
	ScreenWidth  = int(math.Ceil(HexSize*(math.Sqrt(3)*(GridWidth-1)+math.Sqrt(3)/2*(GridHeight-1)) + 2*HexSize))
	ScreenHeight = int(math.Ceil(HexSize * (1.5*(GridHeight-1) + 2)))
)

// =========================== Game ===========================

// Demo is the navigation demo on a hex board — exactly one Stage (arena below).
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
		Title:       "gram navigation on a hex board",
		ScreenWidth: ScreenWidth, ScreenHeight: ScreenHeight, Resizable: true,
		TargetTPS: TPS,
	}
}

func (d *Demo) Stages() (map[string]game.Stage, string) {
	return map[string]game.Stage{d.stage.Name(): d.stage}, d.stage.Name()
}

// =========================== Stage ===========================

// arena wires the hex board, navigation and selection; its plugins are its own fields.
type arena struct {
	world     *world.Plugin
	board     *board.Plugin
	nav       *navigation.Plugin
	collision *collision.Plugin
	selection *selection.Plugin
	players   *players.Plugin
	player    *players.Player // the one at this keyboard: the units are its
}

// newStage defines the game a section at a time, each building on those before it.
// newArena makes the arena — the collector of the stage's plugins, which every
// section builds on — and defines the stage on it, a section at a time.
func newArena() (*arena, game.Stage) {
	s := &arena{}
	return s, stage.New("board-navigation-hex-demo").
		Plugins(s.usePlugins).
		Players(s.definePlayer).
		Cells(s.defineCells).
		Kinds(s.defineKinds).
		Scenes(s.defineScenes).
		Restore(s.restore).
		Layout(s.layOut).
		Units(s.placeUnits).
		Update(s.update)
}

func (s *arena) usePlugins(ctx game.Initializer) error {
	s.world = ctx.UseWorld(world.Config{
		Space:    world.SpaceCfg{Width: uint32(ScreenWidth), Height: uint32(ScreenHeight)},
		Entities: world.EntitiesCfg{MaxCount: MaxEntCount, MinSize: EntitySize, MaxSize: EntitySize},
	})
	grid := grid.DefaultGrids{}.Hex(GridWidth, GridHeight, HexSize)
	s.collision = collision.NewPlugin(s.world)
	s.board = board.NewPlugin(grid, &cell.SingleOccupancy{}, s.world).WithCollision(s.collision)
	s.selection = selection.NewPlugin(s.world)
	s.nav = navigation.NewPlugin(s.board, s.world, s.selection).WithCollision(s.collision)
	s.players = players.NewPlugin(s.world, s.board, s.selection, s.nav).WithSaves(saveBasePath)
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

func (s *arena) defineScenes() []game.Scene {
	main := &mainScene{arena: s}
	// the scene's own keys, labelled for the shortcuts list: K opens it, Esc closes it
	main.keys = players.SceneKeys{
		{Key: control.KeyR, Label: "Build a road through the wall", Do: func(game.Runtime, game.Composition) {
			buildShortcut(s.board.Res.Logic.Board, s.board.CellKinds())
			log.Print("built a road through the wall — in-flight units re-path onto it as soon as they deviate")
		}},
	}
	s.players.OwnKeys(main.keys)
	return []game.Scene{main}
}

func (s *arena) defineCells() {
	kinds := s.board.CellKinds()
	kinds.Define(GrassCell, cell.Kind{Cost: 2, Allows: cell.Land})
	kinds.Define(WallCell, cell.Kind{Cost: 1, Solid: true})
	kinds.Define(RoadCell, cell.Kind{Cost: 1, Allows: cell.Land})
}

func (s *arena) restore(p game.Persistence) (bool, error) {
	saves, err := p.List(saveBasePath)
	if err != nil {
		return false, err
	}
	if !slices.Contains(saves, "") {
		return false, nil
	}
	if err := p.Load(saveBasePath, ""); err != nil {
		return false, err
	}
	log.Print("loaded saved board")
	return true, nil
}

// unit is the row the "red"/"blue" kinds spawn from: where the unit starts and where it heads.
type unitRow struct{ start, target cell.ID }

func (s *arena) defineKinds() {
	brd := s.board.Res.Logic.Board
	units := board.NewUnits[unitRow](s.board, board.Shape{Size: EntitySize}, func(u unitRow) geom.Vec { return brd.CellCenter(u.start) })
	profile := steering.Steering{MaxSpeed: UnitSpeed, Accel: UnitSpeed * 2, Brake: UnitSpeed * 4, V0: UnitSpeed / 2, TurnRate: 0.15}
	own := []comp.Comp{
		comp.Load(func(u unitRow) navigation.MoveOrder { return navigation.MoveOrder{Target: u.target} }),
	}
	units.Define(RedKind, unit.Mover{Domain: cell.Land}, profile, own...)
	units.Define(BlueKind, unit.Mover{Domain: cell.Land}, profile, own...)
}

// cellAt is the cell at column x, row y.
func (s *arena) layOut() {
	brd := s.board.Res.Logic.Board
	// A wall down the q = wallCol column from r = 1 to the bottom, and a road round it: along
	// r = 0 and down both flanks (which slant with the rows, as every hex column does).
	var cells []cell.Entry
	for r := uint32(1); r < GridHeight; r++ {
		cells = append(cells, cell.Entry{Kind: WallCell, Cell: brd.CellIndex(wallCol, r)})
	}
	for q := roadLeft; q <= roadRight; q++ {
		cells = append(cells, cell.Entry{Kind: RoadCell, Cell: brd.CellIndex(q, roadTop)})
	}
	for r := roadTop + 1; r <= roadBottom; r++ {
		cells = append(cells, cell.Entry{Kind: RoadCell, Cell: brd.CellIndex(roadLeft, r)}, cell.Entry{Kind: RoadCell, Cell: brd.CellIndex(roadRight, r)})
	}
	s.board.Seed(board.Layout{Default: GrassCell, Cells: cells})
}

func (s *arena) placeUnits() {
	redKind := kind.Named[unitRow](s.world.Kinds(), RedKind)
	blueKind := kind.Named[unitRow](s.world.Kinds(), BlueKind)
	brd := s.board.Res.Logic.Board
	player := []any{players.Give{To: s.player.ID}, selection.Allow{Selected: true}}
	s.world.Seed(
		redKind.Entry(unitRow{start: brd.CellIndex(3, 3), target: brd.CellIndex(GridWidth-4, 3)}).Told(player...),
		blueKind.Entry(unitRow{start: brd.CellIndex(3, 9), target: brd.CellIndex(GridWidth-4, 9)}).Told(player...),
	)
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
	keys  players.SceneKeys
}

var _ game.Scene = (*mainScene)(nil)

func (m *mainScene) Name() string { return "main" }

// The scene's colours: the two bands and the ground.
var (
	redColor   = color.RGBA{R: 220, G: 90, B: 90, A: 255}
	blueColor  = color.RGBA{R: 90, G: 140, B: 220, A: 255}
	grassColor = color.RGBA{R: 60, G: 95, B: 60, A: 255}
	wallColor  = color.RGBA{R: 40, G: 40, B: 40, A: 255}
	roadColor  = color.RGBA{R: 150, G: 130, B: 80, A: 255}
)

func (m *mainScene) Layers() []render.Layer {
	s := m.arena
	redKind := kind.Named[unitRow](s.world.Kinds(), RedKind)
	blueKind := kind.Named[unitRow](s.world.Kinds(), BlueKind)

	worldAtlas := render.NewAtlas()
	worldAtlas.Add(redKind, EntitySize, render.Diamond(redColor))
	worldAtlas.Add(blueKind, EntitySize, render.Diamond(blueColor))
	worldAtlas.Close()
	s.world.WithRenderer(worldAtlas)

	boardAtlas := s.board.NewAtlas(hexSprite)
	boardAtlas.Add(GrassCell, render.Hexagon(grassColor))
	boardAtlas.Add(WallCell, render.Hexagon(wallColor))
	boardAtlas.Add(RoadCell, render.Hexagon(roadColor))
	boardAtlas.Close()
	s.board.WithRenderer(boardAtlas)

	s.nav.WithRenderer(nil)

	s.selection.WithRenderer(nil)

	return []render.Layer{render.NewComposer(s.board.Renderer(), s.world.Renderer(), s.selection.Renderer(), s.nav.Renderer())}
}

// Viewports are where the world is shown: the local players' views.
func (m *mainScene) Viewports(screen geom.AABB) []render.Viewport {
	return m.arena.players.Viewports(screen)
}

func (m *mainScene) HandleEvents(events *control.InputEvents, runtime game.Runtime, composition game.Composition) {
	m.arena.players.Handle(events, runtime, composition)
}

func (m *mainScene) Focusable() bool { return true }

const (
	// hexSprite is the texture side for a hex sprite: the hex's height, so nothing is upscaled.
	hexSprite = 2 * HexSize

	wallCol     = 10
	shortcutRow = 6

	roadLeft, roadRight uint32 = 2, GridWidth - 3
	roadTop, roadBottom uint32 = 0, GridHeight - 1
)

// buildShortcut lays a road along shortcutRow from flank to flank, through the wall.
func buildShortcut(brd *board.Board, kinds cell.Kinds) {
	road := kinds.Named(RoadCell).Kind()
	for q := roadLeft + 1; q < roadRight; q++ {
		c := brd.CellIndex(q, shortcutRow)
		brd.Set(c, road)
	}
}
