// Command navigation-vision-demo puts sight on navigated units in a Quasi3D world: their cones stop
// at the wall, fade in the forest and climb the hill; a hawk 40 up looks over all three.
package main

import (
	"image/color"
	"log"
	"math"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/navigation"
	"github.com/kjkrol/gram/plugins/players"
	"github.com/kjkrol/gram/plugins/selection"
	"github.com/kjkrol/gram/plugins/vision"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/kind"
	"github.com/kjkrol/gram/plugins/world/kind/comp"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/uid"
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
	MaxEntCount  = 80 // units plus the terrain bodies of the wall and the forests

	sightRadius = 200
	sightHalf   = math.Pi / 5
)

// =========================== Game ===========================

// Demo is the board + navigation + vision demo — exactly one Stage (mainStage below).
type Demo struct{ stage *mainStage }

var _ game.Game = (*Demo)(nil)

func NewDemo() *Demo { return &Demo{stage: &mainStage{}} }

func (d *Demo) Props() game.Props {
	return game.Props{
		Title:       "gram — sight across a board: walls cut, forests dim, a hawk flies over",
		ScreenWidth: ScreenWidth, ScreenHeight: ScreenHeight, Resizable: true,
		TargetTPS: TPS,
	}
}

func (d *Demo) Stages() (map[string]game.Stage, string) {
	return map[string]game.Stage{d.stage.Name(): d.stage}, d.stage.Name()
}

// =========================== Stage ===========================

// units is the demo's tag family; unit marks its units, so a sighting of one can be told from a
// sighting of terrain.
type units struct{}

type mainStage struct {
	world     *world.Plugin
	board     *board.Plugin
	nav       *navigation.Plugin
	collision *collision.Plugin
	selection *selection.Plugin
	players   *players.Plugin
	vision    *vision.Plugin
	unitTag   plugin.Tag[units]
	kinds     []kind.Of[unit]
	hawk      kind.Of[unit]
	noticed   map[[2]uid.UID64]bool
	stack     game.Scenes
}

var _ game.Stage = (*mainStage)(nil)

func (s *mainStage) Name() string { return "board-navigation-vision-demo" }

func (s *mainStage) Stack() game.Scenes { return s.stack }

func (s *mainStage) Init(ctx game.Initializer) error {
	s.world = ctx.UseWorld(world.Config{
		Space:    world.SpaceCfg{Width: ScreenWidth, Height: ScreenHeight},
		Entities: world.EntitiesCfg{MaxCount: MaxEntCount, MinSize: EntitySize, MaxSize: EntitySize},
		Quasi3D:  true, // heights: the hawk looks over the wall, the forest and the hill
	})

	s.collision = collision.NewPlugin(s.world)
	if err := ctx.Use(s.collision); err != nil {
		return err
	}

	grid := board.DefaultGrids{}.Square(GridWidth, GridHeight, CellSize)
	s.board = board.NewPlugin(grid, &board.SingleOccupancy{}, s.world).WithCollision(s.collision)
	s.board.CellKindDict().Create(
		board.CellKind{Name: board.Named("grass"), Cost: 2, Allows: board.Land | board.Air}.Costing(board.Air, 1),
		board.CellKind{Name: board.Named("wall"), Cost: 1, Solid: true, Allows: board.Air, Height: 10},
		board.CellKind{Name: board.Named("forest"), Cost: 3, Allows: board.Land | board.Air, Veil: 0.6, Height: 8}.Costing(board.Air, 1),
		board.CellKind{Name: board.Named("road"), Cost: 1, Allows: board.Land | board.Air},
		board.CellKind{Name: board.Named("hill"), Cost: 2, Allows: board.Land | board.Air}.Costing(board.Air, 1),
	)
	if err := ctx.Use(s.board); err != nil {
		return err
	}

	s.selection = selection.NewPlugin(s.world)
	if err := ctx.Use(s.selection); err != nil {
		return err
	}

	s.nav = navigation.NewPlugin(s.board, s.world, s.selection)
	if err := ctx.Use(s.nav); err != nil {
		return err
	}

	s.players = players.NewPlugin(s.world, s.selection, s.nav)
	if err := s.players.Local("player").Bind(s.players.Defaults()...); err != nil {
		return err
	}
	if err := ctx.Use(s.players); err != nil {
		return err
	}

	s.noticed = map[[2]uid.UID64]bool{}
	s.unitTag = s.world.Kinds().DefineTag[units]("unit")
	s.vision = vision.NewPlugin(s.world)
	if err := s.vision.RegisterBehavior(
		vision.Between(plugin.Any, plugin.Any, faceTravel),
		vision.Between(s.unitTag, s.unitTag, s.noticedEachOther),
	); err != nil {
		return err
	}
	if err := ctx.Use(s.vision); err != nil {
		return err
	}

	s.defineKinds()

	main := &mainScene{stage: s}
	stack, err := game.NewStack(main)
	if err != nil {
		return err
	}
	s.stack = stack
	comp := stack.Composition()
	comp.Show(main.Name())
	return ctx.Track(comp)
}

func (s *mainStage) Restore(game.Persistence) (bool, error) { return false, nil }

// unit is the row every unit kind spawns from: where it starts and where it heads.
type unit struct{ start, target board.CellID }

var unitColors = []color.RGBA{
	{R: 220, G: 90, B: 90, A: 255},
	{R: 90, G: 140, B: 220, A: 255},
	{R: 230, G: 200, B: 80, A: 255},
}

var hawkColor = color.RGBA{R: 120, G: 130, B: 60, A: 255}

// defineKinds says what this game's entities are: one kind per colour, all scouts, and a hawk.
func (s *mainStage) defineKinds() {
	brd := s.board.Res.Logic.Board
	// Every unit is 2 tall; the eye is a fact of the kind, the altitude the board's to write.
	units := board.NewUnits[unit](s.board, board.Shape{Size: EntitySize, Height: 2}, func(u unit) geom.Vec { return brd.CellCenter(u.start) })
	order := comp.Load(func(u unit) navigation.MoveOrder { return navigation.MoveOrder{Target: u.target} })
	sight := func(eye float64) comp.Comp {
		return comp.Const(vision.Sight{Facing: geom.NewVec(1, 0), HalfAngle: sightHalf, Radius: sightRadius, Eye: eye})
	}
	scout := world.Steering{MaxSpeed: UnitSpeed, Accel: UnitSpeed * 2, Brake: UnitSpeed * 4, V0: UnitSpeed / 2, TurnRate: 0.15}
	for _, name := range []string{"red", "blue", "yellow"} {
		s.kinds = append(s.kinds, units.Define(name, board.Mover{Domain: board.Land}, scout, order,
			comp.Tagged(s.selection.Tags().Selectable, s.selection.Tags().Selected),
			sight(1.5), comp.Const(vision.SightOutline{}), comp.Tagged(s.unitTag)))
	}
	// The hawk flies 40 above the ground on the Air plane: walls and walkers pass under it, and its
	// eye looks over the wall, the forest and the hill that stop a walker's.
	flyer := world.Steering{MaxSpeed: UnitSpeed * 1.5, Accel: UnitSpeed * 2, Brake: UnitSpeed * 4, V0: UnitSpeed / 2, TurnRate: 0.1}
	s.hawk = units.Define("hawk", board.Mover{Domain: board.Air, Lift: 40}, flyer, order,
		comp.Tagged(s.selection.Tags().Selectable),
		sight(1), comp.Const(vision.SightOutline{}), comp.Tagged(s.unitTag))
}

// Spawn says who is there when the game starts fresh.
func (s *mainStage) Spawn() error {
	brd := s.board.Res.Logic.Board
	cell := func(x, y uint32) board.CellID { c, _ := brd.CellIndex(x, y); return c }

	// A wall down column 12 with a gap at row 8, a forest either side of the gap, and a road
	// along row 1 with both flanks.
	var cells []board.CellEntry
	for y := uint32(2); y < GridHeight; y++ {
		if y == gapRow {
			continue
		}
		cells = append(cells, board.CellEntry{Kind: "wall", Cell: cell(wallCol, y)})
	}
	for _, f := range [][2]uint32{{6, 6}, {17, 10}} {
		for dy := uint32(0); dy < 3; dy++ {
			for dx := uint32(0); dx < 4; dx++ {
				cells = append(cells, board.CellEntry{Kind: "forest", Cell: cell(f[0]+dx, f[1]+dy)})
			}
		}
	}
	// A hill in the first unit's way: its cone climbs the slope and stops, the hawk's passes over.
	for dy := uint32(3); dy <= 5; dy++ {
		for dx := uint32(7); dx <= 9; dx++ {
			cells = append(cells, board.CellEntry{Kind: "hill", Cell: cell(dx, dy)})
		}
	}
	for x := roadLeft; x <= roadRight; x++ {
		cells = append(cells, board.CellEntry{Kind: "road", Cell: cell(x, roadTop)})
	}
	for y := roadTop + 1; y <= roadBottom; y++ {
		cells = append(cells, board.CellEntry{Kind: "road", Cell: cell(roadLeft, y)}, board.CellEntry{Kind: "road", Cell: cell(roadRight, y)})
	}
	hills := map[board.CellID]bool{}
	for _, e := range cells {
		hills[e.Cell] = e.Kind == "hill"
	}
	heights := board.MeanOfCells(s.board.Res.Logic.Board, func(c board.CellID) float64 {
		if hills[c] {
			return hillHeight
		}
		return 0
	})
	s.board.Seed(board.Layout{Default: "grass", Cells: cells, Heights: heights})

	s.world.Seed(
		s.kinds[0].Entry(unit{start: cell(2, 4), target: cell(GridWidth-3, 4)}),
		s.kinds[1].Entry(unit{start: cell(2, 12), target: cell(GridWidth-3, 12)}),
		s.kinds[2].Entry(unit{start: cell(GridWidth-3, gapRow), target: cell(2, gapRow)}),
		// The hawk crosses the wall and the second forest head-on.
		s.hawk.Entry(unit{start: cell(1, 11), target: cell(GridWidth-2, 11)}),
	)
	return nil
}

func (s *mainStage) Update(ctx goke.RunCtx, d time.Duration) {
	s.world.RunPlan(ctx, d)
	s.collision.RunPlan(ctx, d)
	s.board.RunPlan(ctx, d)
	s.nav.RunPlan(ctx, d)
	s.vision.RunPlan(ctx, d)
	s.selection.RunPlan(ctx, d)
	s.players.RunPlan(ctx, d)
	ctx.Sync()
}

// faceTravel points each unit's Sight where it is heading, turning in place included; a unit that
// stops keeps its last heading, so its Sight stays where it looked.
func faceTravel(_ plugin.Tick, s vision.Sighting) {
	if d := s.Base.Vel.Dir; d.X != 0 || d.Y != 0 {
		s.Sight.Facing = d
	}
}

// noticedEachOther logs the first time one unit sees another.
func (s *mainStage) noticedEachOther(_ plugin.Tick, sighting vision.Sighting) {
	for _, seen := range sighting.Seen {
		pair := [2]uid.UID64{sighting.Self, seen.ID}
		if !s.noticed[pair] {
			s.noticed[pair] = true
			log.Printf("unit %d sees unit %d at %.0f", sighting.Self, seen.ID, seen.Dist)
		}
	}
}

// =========================== Scene ===========================

type mainScene struct{ stage *mainStage }

var _ game.Scene = (*mainScene)(nil)

func (m *mainScene) Name() string { return "main" }

func (m *mainScene) Layers() []render.Layer {
	s := m.stage

	worldAtlas := render.NewAtlas()
	for i, k := range s.kinds {
		worldAtlas.RegisterAt(k.SpriteID(), EntitySize, render.Solid(unitColors[i]))
	}
	worldAtlas.RegisterAt(s.hawk.SpriteID(), EntitySize, render.Diamond(hawkColor))
	worldAtlas.Close()
	s.world.WithRenderer(worldAtlas)

	kinds := s.board.CellKindDict()
	boardAtlas := render.NewAtlas()
	for name, c := range map[string]color.RGBA{
		"grass":  {R: 60, G: 95, B: 60, A: 255},
		"wall":   {R: 40, G: 40, B: 40, A: 255},
		"forest": {R: 25, G: 60, B: 30, A: 255},
		"road":   {R: 150, G: 130, B: 80, A: 255},
		"hill":   {R: 110, G: 100, B: 70, A: 255},
	} {
		k, _ := kinds.Get(name)
		boardAtlas.RegisterAt(k.SpriteID, CellSize, render.Solid(c))
	}
	boardAtlas.Close()
	s.board.WithRenderer(boardAtlas)

	pathAtlas, pathSprites := navigation.RegisterDefaultPathSprites(CellSize, 2, color.RGBA{R: 255, G: 140, B: 0, A: 255})
	s.nav.SetPathSprites(pathSprites)
	s.nav.WithRenderer(pathAtlas)

	s.vision.WithRenderer(nil)
	s.selection.WithRenderer(nil)

	return []render.Layer{s.board.Renderer(), s.vision.Renderer(), s.world.Renderer(), s.selection.Renderer(), s.nav.Renderer()}
}

// Viewports are where the world is shown: the local players' views.
func (m *mainScene) Viewports(screen geom.AABB) []render.Viewport {
	return m.stage.players.Viewports(screen)
}

func (m *mainScene) HandleEvents(events *control.InputEvents, runtime game.Runtime, _ game.Composition) {
	s := m.stage
	s.players.EventHandler().HandleEvents(events)
	for _, k := range events.KeyEvents {
		if k.Action != control.ActionPress {
			continue
		}
		switch k.Key {
		case ebiten.KeyEscape:
			runtime.Quit()
		case ebiten.KeySpace:
			runtime.TogglePause()
		case ebiten.KeyB:
			s.board.Res.Render.ToggleShowGridLines()
		}
	}
}

func (m *mainScene) Focusable() bool { return true }

const (
	wallCol = 12
	gapRow  = 8

	roadLeft, roadRight uint32 = 2, GridWidth - 3
	roadTop, roadBottom uint32 = 1, 13
)

// hillHeight is how high the hill stands over the grass.
const hillHeight = 12
