package main

import (
	"image/color"
	"log"
	"slices"
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
	"github.com/kjkrol/gram/plugins/cameras"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/driving"
	"github.com/kjkrol/gram/plugins/navigation"
	"github.com/kjkrol/gram/plugins/players"
	"github.com/kjkrol/gram/plugins/selection"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/steering"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/rule"
	"github.com/kjkrol/gram/ui"
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
	MaxEntCount  = 16 // the units; the walls are cells, not entities

	saveBasePath = "board-navigation-demo"
)

// =========================== Game ===========================

// Demo is the board/navigation/selection demo — exactly one Stage (arena below).
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
		Title:       "gram board & navigation plugins demo",
		ScreenWidth: ScreenWidth, ScreenHeight: ScreenHeight, Resizable: true,
		TargetTPS: TPS,
	}
}

func (d *Demo) Stages() (map[string]game.Stage, string) {
	return map[string]game.Stage{d.stage.Name(): d.stage}, d.stage.Name()
}

// =========================== Stage ===========================

// arena wires the board/navigation/selection demo; its plugins are its own fields.
type arena struct {
	world     *world.Plugin
	board     *board.Plugin
	nav       *navigation.Plugin
	driving   *driving.Plugin
	collision *collision.Plugin
	selection *selection.Plugin
	players   *players.Plugin
	cameras   *cameras.Plugin
	player    *players.Player // the one at this keyboard: the units are its
}

// newStage defines the game a section at a time, each building on those before it.
// newArena makes the arena — the collector of the stage's plugins, which every
// section builds on — and defines the stage on it, a section at a time.
func newArena() (*arena, game.Stage) {
	s := &arena{}
	return s, stage.New("board-navigation-demo").
		Plugins(s.usePlugins).
		Players(s.definePlayer).
		Rules(s.defineRules).
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
		Space:    world.SpaceCfg{Width: ScreenWidth, Height: ScreenHeight},
		Entities: world.EntitiesCfg{MaxCount: MaxEntCount, MinSize: EntitySize, MaxSize: EntitySize},
	})
	grid := grid.DefaultGrids{}.Square(GridWidth, GridHeight, CellSize)
	s.collision = collision.NewPlugin(s.world)
	s.board = board.NewPlugin(grid, &cell.SingleOccupancy{}, s.world).WithCollision(s.collision).WithLog(log.Default())
	s.selection = selection.NewPlugin(s.world)
	s.driving = driving.NewPlugin(s.world, s.selection).WithGround(s.board)
	s.nav = navigation.NewPlugin(s.board, s.world, s.selection, s.driving).WithCollision(s.collision)
	s.cameras = cameras.NewPlugin(s.world)
	s.players = players.NewPlugin(s.world, s.cameras, s.board, s.selection, s.nav, s.driving).WithSaves(saveBasePath)
	for _, p := range []plugin.Plugin{s.collision, s.board, s.selection, s.nav, s.driving, s.cameras, s.players} {
		if err := ctx.Use(p); err != nil {
			return err
		}
	}
	return nil
}

func (s *arena) definePlayer() error {
	s.player = s.players.Local("player", s.cameras.New(cameras.TopDown(), camera.Config{}))
	return s.player.Bind(s.players.Defaults()...)
}

func (s *arena) defineRules() {
	s.world.Roles().Define(MortalRole,
		rule.Then[unit.Standing]("fall in", rule.All, rule.If(unit.Standing.Fallen, rule.Order(world.Despawn{}))))
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
	return []game.Scene{ui.NewScene("main", main.pictures, main.screen).Input(s.players.Handle)}
}

func (s *arena) defineCells() {
	kinds := s.board.CellKinds()
	kinds.Define(GrassCell, cell.Kind{Cost: 2, Allows: cell.Land})
	kinds.Define(WallCell, cell.Kind{Cost: 1, Solid: true})
	kinds.Define(RoadCell, cell.Kind{Cost: 1, Allows: cell.Land})
	kinds.Define(HoleCell, cell.Kind{Cost: 1}) // admits nobody and is not solid: whoever stands on it falls
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
		rule.Plays(s.world.Roles().Named(MortalRole)),
	}
	units.Define(RedKind, unit.Mover{Domain: cell.Land}, profile, own...)
	units.Define(BlueKind, unit.Mover{Domain: cell.Land}, profile, own...)
}

// cellAt is the cell at column x, row y.
func (s *arena) layOut() {
	brd := s.board.Res.Logic.Board
	// A wall down column 12 from row 2, a road round it along row 1 and down both flanks, and a
	// hole on each unit's straight line, so the planner has to go round.
	var cells []cell.Entry
	for y := uint32(2); y < GridHeight; y++ {
		cells = append(cells, cell.Entry{Kind: WallCell, Cell: brd.CellIndex(wallCol, y)})
	}
	cells = append(cells, cell.Entry{Kind: HoleCell, Cell: brd.CellIndex(6, 4)}, cell.Entry{Kind: HoleCell, Cell: brd.CellIndex(17, 12)})
	for x := roadLeft; x <= roadRight; x++ {
		cells = append(cells, cell.Entry{Kind: RoadCell, Cell: brd.CellIndex(x, roadTop)})
	}
	for y := roadTop + 1; y <= roadBottom; y++ {
		cells = append(cells, cell.Entry{Kind: RoadCell, Cell: brd.CellIndex(roadLeft, y)}, cell.Entry{Kind: RoadCell, Cell: brd.CellIndex(roadRight, y)})
	}
	s.board.Seed(board.Layout{Default: GrassCell, Cells: cells})
}

func (s *arena) placeUnits() {
	redKind := kind.Named[unitRow](s.world.Kinds(), RedKind)
	blueKind := kind.Named[unitRow](s.world.Kinds(), BlueKind)
	brd := s.board.Res.Logic.Board
	player := []any{players.Give{To: s.player.ID}, selection.Allow{Selected: true}}
	s.world.Seed(
		redKind.Entry(unitRow{start: brd.CellIndex(2, 4), target: brd.CellIndex(GridWidth-3, 4)}).Told(player...),
		blueKind.Entry(unitRow{start: brd.CellIndex(2, 12), target: brd.CellIndex(GridWidth-3, 12)}).Told(player...),
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

// =========================== Scene ===========================

type mainScene struct {
	arena   *arena
	keys    players.SceneKeys
	picture *render.Composer // the world, as the scene shows it
}

// The scene's colours: the two bands and the ground.
var (
	redColor   = color.RGBA{R: 220, G: 90, B: 90, A: 255}
	blueColor  = color.RGBA{R: 90, G: 140, B: 220, A: 255}
	grassColor = color.RGBA{R: 60, G: 95, B: 60, A: 255}
	wallColor  = color.RGBA{R: 40, G: 40, B: 40, A: 255}
	roadColor  = color.RGBA{R: 150, G: 130, B: 80, A: 255}
	holeColor  = color.RGBA{R: 10, G: 10, B: 30, A: 255}
)

// pictures dresses the world and hands its picture.
func (m *mainScene) pictures() []render.Picture {
	s := m.arena
	redKind := kind.Named[unitRow](s.world.Kinds(), RedKind)
	blueKind := kind.Named[unitRow](s.world.Kinds(), BlueKind)

	worldAtlas := render.NewAtlas()
	worldAtlas.Add(redKind, EntitySize, render.Solid(redColor))
	worldAtlas.Add(blueKind, EntitySize, render.Solid(blueColor))
	worldAtlas.Close()
	s.world.WithRenderer(worldAtlas)

	boardAtlas := s.board.NewAtlas(CellSize)
	boardAtlas.Add(GrassCell, render.Solid(grassColor))
	boardAtlas.Add(WallCell, render.Solid(wallColor))
	boardAtlas.Add(RoadCell, render.Solid(roadColor))
	boardAtlas.Add(HoleCell, render.Solid(holeColor))
	boardAtlas.Close()
	s.board.WithRenderer(boardAtlas)

	s.nav.WithRenderer(nil)

	s.selection.WithRenderer(nil)

	m.picture = render.NewComposer(s.board.Renderer(), s.world.Renderer(), s.selection.Renderer(), s.nav.Renderer())
	return []render.Picture{m.picture}
}

// screen is the world through the player's camera.
func (m *mainScene) screen() *ui.Element {
	s := m.arena
	return ui.Image(render.NewFeed(s.player.Camera, m.picture)).Input(s.players.Through(s.player))
}

const (
	wallCol     = 12
	shortcutRow = 8

	roadLeft, roadRight uint32 = 2, GridWidth - 3
	roadTop, roadBottom uint32 = 1, 13
)

// buildShortcut lays a road along shortcutRow from flank to flank, through the wall.
func buildShortcut(brd *board.Board, kinds cell.Kinds) {
	road := kinds.Named(RoadCell).Kind()
	for x := roadLeft + 1; x < roadRight; x++ {
		c := brd.CellIndex(x, shortcutRow)
		brd.Set(c, road)
	}
}
