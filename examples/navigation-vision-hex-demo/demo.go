// Command navigation-vision-hex-demo puts sight on units navigating a hex board: their cones stop
// at the wall and fade in the forest, read from the hex cells themselves; a hawk flies over
// both and sees through the forest.
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
	"github.com/kjkrol/gram/plugins/topography"
	"github.com/kjkrol/gram/plugins/vision"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/kind"
	"github.com/kjkrol/gram/plugins/world/kind/comp"
	"github.com/kjkrol/gram/plugins/world/steering"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/uid"
)

// The board is a parallelogram of pointy-top hexes in axial (q, r) coordinates — see
// navigation-hex-demo.
const (
	TPS        = 60
	GridWidth  = 20
	GridHeight = 12
	HexSize    = 24
	EntitySize = 22
	UnitSpeed  = HexSize * 3
	// MaxEntCount is the units; the wall and the forests are cells, not entities.
	MaxEntCount = 32

	hexSprite   = 2 * HexSize
	sightRadius = 200
	sightHalf   = math.Pi / 5
)

var (
	ScreenWidth  = int(math.Ceil(HexSize*(math.Sqrt(3)*(GridWidth-1)+math.Sqrt(3)/2*(GridHeight-1)) + 2*HexSize))
	ScreenHeight = int(math.Ceil(HexSize * (1.5*(GridHeight-1) + 2)))
)

// =========================== Game ===========================

// Demo is the hex board + navigation + vision demo — exactly one Stage (mainStage below).
type Demo struct{ stage *mainStage }

var _ game.Game = (*Demo)(nil)

func NewDemo() *Demo { return &Demo{stage: &mainStage{}} }

func (d *Demo) Props() game.Props {
	return game.Props{
		Title:       "gram — sight across a hex board: walls cut, forests dim, a hawk flies over",
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
	world      *world.Plugin
	board      *board.Plugin
	topography *topography.Plugin
	nav        *navigation.Plugin
	collision  *collision.Plugin
	selection  *selection.Plugin
	players    *players.Plugin
	shortcuts  *players.Shortcuts
	vision     *vision.Plugin
	unitTag    plugin.Tag[units]
	kinds      []kind.Of[unit]
	hawk       kind.Of[unit]
	noticed    map[[2]uid.UID64]bool
	stack      game.Scenes
}

var _ game.Stage = (*mainStage)(nil)

func (s *mainStage) Name() string { return "board-navigation-vision-hex-demo" }

func (s *mainStage) Stack() game.Scenes { return s.stack }

func (s *mainStage) Init(ctx game.Initializer) error {
	s.world = ctx.UseWorld(world.Config{
		Space:    world.SpaceCfg{Width: uint32(ScreenWidth), Height: uint32(ScreenHeight)},
		Entities: world.EntitiesCfg{MaxCount: MaxEntCount, MinSize: EntitySize, MaxSize: EntitySize},
		Heights:  true, // heights: the hawk looks over the wall, the forest and the hill
	})

	s.collision = collision.NewPlugin(s.world)
	if err := ctx.Use(s.collision); err != nil {
		return err
	}

	grid := board.DefaultGrids{}.Hex(GridWidth, GridHeight, HexSize)
	s.board = board.NewPlugin(grid, &board.SingleOccupancy{}, s.world).WithCollision(s.collision)
	s.topography = topography.NewPlugin(s.world, s.board, topography.Config{Cell: HexSize}) // the hills in relief, seen from above
	s.board.CellKindDict().Create(
		board.CellKind{Name: board.Named("grass"), Cost: 2, Allows: board.Land | board.Air}.Costing(board.Air, 1),
		board.CellKind{Name: board.Named("wall"), Cost: 1, Solid: true, Allows: board.Air, Veil: 1, Height: 10},
		board.CellKind{Name: board.Named("forest"), Cost: 3, Allows: board.Land | board.Air, Veil: 0.6, Height: 8}.Costing(board.Air, 1),
		board.CellKind{Name: board.Named("road"), Cost: 1, Allows: board.Land | board.Air},
		board.CellKind{Name: board.Named("hill"), Cost: 2, Allows: board.Land | board.Air}.Costing(board.Air, 1),
	)
	if err := ctx.Use(s.board); err != nil {
		return err
	}
	if err := ctx.Use(s.topography); err != nil {
		return err
	}

	s.selection = selection.NewPlugin(s.world)
	if err := ctx.Use(s.selection); err != nil {
		return err
	}

	s.nav = navigation.NewPlugin(s.board, s.world, s.selection).WithCollision(s.collision)
	if err := ctx.Use(s.nav); err != nil {
		return err
	}

	s.players = players.NewPlugin(s.world, s.selection, s.nav, s.topography)
	if err := s.players.Local("player").Bind(s.players.Defaults()...); err != nil {
		return err
	}
	if err := ctx.Use(s.players); err != nil {
		return err
	}

	s.noticed = map[[2]uid.UID64]bool{}
	s.unitTag = s.world.Kinds().DefineTag[units]("unit")
	s.vision = vision.NewPlugin(s.world).WithBoard(s.board)
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
	// the scene's own keys, labelled for the shortcuts list: K opens it, Esc closes it
	main.keys = players.SceneKeys{
		{Key: ebiten.KeyK, Label: "Shortcuts; Esc closes them", Do: func(rt game.Runtime, c game.Composition) { s.shortcuts.Open(rt, c) }},
		{Key: ebiten.KeyEscape, Shift: true, Label: "Quit", Do: func(rt game.Runtime, _ game.Composition) { rt.Quit() }},
		{Key: ebiten.KeyB, Label: "Toggle the grid", Do: func(game.Runtime, game.Composition) { s.board.Res.Render.ToggleShowGridLines() }},
	}
	s.shortcuts = s.players.Shortcuts(main.keys)
	stack, err := game.NewStack(main, s.shortcuts)
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
	scout := steering.Steering{MaxSpeed: UnitSpeed, Accel: UnitSpeed * 2, Brake: UnitSpeed * 4, V0: UnitSpeed / 2, TurnRate: 0.15}
	for _, name := range []string{"red", "blue", "yellow"} {
		s.kinds = append(s.kinds, units.Define(name, board.Mover{Domain: board.Land}, scout, order,
			comp.Tagged(s.selection.Tags().Selectable, s.selection.Tags().Selected),
			sight(1.5), comp.Const(vision.SightOutline{}), comp.Tagged(s.unitTag)))
	}
	// The hawk flies 40 above the ground on the Air plane: walls and walkers pass under it, and its
	// eye looks over the wall, the forest and the hill that stop a walker's.
	flyer := steering.Steering{MaxSpeed: UnitSpeed * 1.5, Accel: UnitSpeed * 2, Brake: UnitSpeed * 4, V0: UnitSpeed / 2, TurnRate: 0.1}
	s.hawk = units.Define("hawk", board.Mover{Domain: board.Air, Lift: 40}, flyer, order,
		comp.Tagged(s.selection.Tags().Selectable),
		sight(1), comp.Const(vision.SightOutline{}), comp.Tagged(s.unitTag))
}

// Spawn says who is there when the game starts fresh.
func (s *mainStage) Spawn() error {
	brd := s.board.Res.Logic.Board
	cell := func(x, y uint32) board.CellID { c, _ := brd.CellIndex(x, y); return c }

	// A wall down the q = wallCol column with a gap at r = gapRow, a forest either side of the
	// gap, and a road along r = 0 with both flanks.
	var cells []board.CellEntry
	for r := uint32(1); r < GridHeight; r++ {
		if r == gapRow {
			continue
		}
		cells = append(cells, board.CellEntry{Kind: "wall", Cell: cell(wallCol, r)})
	}
	for _, f := range [][2]uint32{{5, 4}, {13, 8}} {
		for dr := uint32(0); dr < 3; dr++ {
			for dq := uint32(0); dq < 3; dq++ {
				cells = append(cells, board.CellEntry{Kind: "forest", Cell: cell(f[0]+dq, f[1]+dr)})
			}
		}
	}
	// A hill in the first unit's way: its cone climbs the slope and stops, the hawk's passes over.
	for dr := uint32(2); dr <= 4; dr++ {
		for dq := uint32(8); dq <= 10; dq++ {
			if dq != wallCol {
				cells = append(cells, board.CellEntry{Kind: "hill", Cell: cell(dq, dr)})
			}
		}
	}
	for q := roadLeft; q <= roadRight; q++ {
		cells = append(cells, board.CellEntry{Kind: "road", Cell: cell(q, roadTop)})
	}
	for r := roadTop + 1; r <= roadBottom; r++ {
		cells = append(cells, board.CellEntry{Kind: "road", Cell: cell(roadLeft, r)}, board.CellEntry{Kind: "road", Cell: cell(roadRight, r)})
	}
	hills := map[board.CellID]bool{}
	for _, e := range cells {
		hills[e.Cell] = e.Kind == "hill"
	}
	heights := topography.MeanOfCells(s.board.Res.Logic.Board, func(c board.CellID) float64 {
		if hills[c] {
			return hillHeight
		}
		return 0
	})
	s.board.Seed(board.Layout{Default: "grass", Cells: cells})
	s.topography.Seed(heights)

	s.world.Seed(
		s.kinds[0].Entry(unit{start: cell(3, 3), target: cell(GridWidth-4, 3)}),
		s.kinds[1].Entry(unit{start: cell(3, 9), target: cell(GridWidth-4, 9)}),
		s.kinds[2].Entry(unit{start: cell(GridWidth-4, gapRow), target: cell(3, gapRow)}),
		// The hawk crosses the wall and the second forest head-on.
		s.hawk.Entry(unit{start: cell(1, 9), target: cell(GridWidth-2, 9)}),
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
	s.topography.RunPlan(ctx, d)
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

type mainScene struct {
	stage *mainStage
	keys  players.SceneKeys
}

var _ game.Scene = (*mainScene)(nil)

func (m *mainScene) Name() string { return "main" }

func (m *mainScene) Layers() []render.Layer {
	s := m.stage

	worldAtlas := render.NewAtlas()
	for i, k := range s.kinds {
		worldAtlas.RegisterAt(k.SpriteID(), EntitySize, render.Diamond(unitColors[i]))
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
		boardAtlas.RegisterAt(k.SpriteID, hexSprite, render.Hexagon(c))
	}
	boardAtlas.Close()
	s.board.WithRenderer(boardAtlas)

	pathAtlas, pathSprites := navigation.RegisterDefaultPathSprites(hexSprite, 2, color.RGBA{R: 255, G: 140, B: 0, A: 255})
	s.nav.SetPathSprites(pathSprites)
	s.nav.WithRenderer(pathAtlas)

	s.vision.WithRenderer(nil)
	s.selection.WithRenderer(nil)

	return []render.Layer{render.NewComposer(s.board.Renderer(), s.world.Renderer(), s.vision.Renderer(), s.selection.Renderer(), s.nav.Renderer())}
}

// Viewports are where the world is shown: the local players' views.
func (m *mainScene) Viewports(screen geom.AABB) []render.Viewport {
	return m.stage.players.Viewports(screen)
}

func (m *mainScene) HandleEvents(events *control.InputEvents, runtime game.Runtime, composition game.Composition) {
	m.stage.players.EventHandler().HandleEvents(events)
	m.keys.Handle(events, runtime, composition)
}

func (m *mainScene) Focusable() bool { return true }

const (
	wallCol = 10
	gapRow  = 6

	roadLeft, roadRight uint32 = 2, GridWidth - 3
	roadTop, roadBottom uint32 = 0, GridHeight - 1
)

// hillHeight is how high the hill stands over the grass.
const hillHeight = 12
