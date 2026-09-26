// Command island-isometric-demo is the island of island-demo in a Quasi3D world seen through an
// isometric camera, Transport Tycoon's way: hills 20 and mountains 40 up with sloping sides,
// forests standing 8 tall, units drawn upright on the ground and a hawk 40 up whose cone looks
// over everything a walker's stops at. A day goes by (plugins/sky): long shadows morning and
// evening, dark nights; P stops it, ] and [ hurry it on and hold it back — or, stopped, move it
// half an hour on or back. Scroll with the wheel, drag with the middle button or push the cursor to
// an edge to move the camera.
package main

import (
	"image/color"
	"log"
	"math"
	"slices"
	"time"

	"github.com/kjkrol/aabbworld/geom"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/isometry"
	"github.com/kjkrol/gram/plugins/navigation"
	"github.com/kjkrol/gram/plugins/players"
	"github.com/kjkrol/gram/plugins/selection"
	"github.com/kjkrol/gram/plugins/sky"
	"github.com/kjkrol/gram/plugins/vision"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/kind"
	"github.com/kjkrol/gram/plugins/world/kind/comp"
	"github.com/kjkrol/gram/render"
)

const (
	TPS          = 60
	GridWidth    = 96
	GridHeight   = 64
	CellSize     = 32
	WorldWidth   = GridWidth * CellSize
	WorldHeight  = GridHeight * CellSize
	ScreenWidth  = 1024
	ScreenHeight = 768
	EntitySize   = 22
	UnitSpeed    = CellSize * 3
	UnitCount    = 6
	sightRadius  = 220
	sightHalf    = math.Pi / 5
	// MaxEntCount is the units; the forests are cells, not entities.
	MaxEntCount = 4 * UnitCount

	saveBasePath = "island-isometric-demo"
)

type State struct{ Saves int }

// =========================== Game ===========================

// Demo is the island demo — exactly one Stage (mainStage below).
type Demo struct{ stage *mainStage }

var _ game.Game = (*Demo)(nil)

func NewDemo() *Demo { return &Demo{stage: &mainStage{}} }

func (d *Demo) Props() game.Props {
	return game.Props{
		Title:       "gram — an island in isometric relief",
		ScreenWidth: ScreenWidth, ScreenHeight: ScreenHeight, Resizable: true,
		TargetTPS: TPS,
	}
}

func (d *Demo) Stages() (map[string]game.Stage, string) {
	return map[string]game.Stage{d.stage.Name(): d.stage}, d.stage.Name()
}

// =========================== Stage ===========================

type mainStage struct {
	world     *world.Plugin
	isometry  *isometry.Plugin
	board     *board.Plugin
	nav       *navigation.Plugin
	collision *collision.Plugin
	selection *selection.Plugin
	players   *players.Plugin
	vision    *vision.Plugin
	sky       *sky.Plugin
	unit      kind.Of[unit]
	hawk      kind.Of[unit]
	stack     game.Scenes
	state     *State
}

var _ game.Stage = (*mainStage)(nil)

func (s *mainStage) Name() string { return "island-isometric-demo" }

func (s *mainStage) Stack() game.Scenes { return s.stack }

func (s *mainStage) Init(ctx game.Initializer) error {
	s.world = ctx.UseWorld(world.Config{
		Space:    world.SpaceCfg{Width: WorldWidth, Height: WorldHeight},
		Entities: world.EntitiesCfg{MaxCount: MaxEntCount, MinSize: EntitySize, MaxSize: EntitySize},
		Camera:   camera.Config{ViewportWidth: ScreenWidth, ViewportHeight: ScreenHeight},
		Quasi3D:  true,
	})
	s.isometry = isometry.NewPlugin(s.world, isometry.Config{Cell: CellSize, HeightUnit: 1})
	// Start over the island's middle rather than the world's corner.
	s.world.Camera().CenterOn(WorldWidth/2, WorldHeight/2, 0)

	s.collision = collision.NewPlugin(s.world)
	if err := ctx.Use(s.collision); err != nil {
		return err
	}

	grid := board.DefaultGrids{}.Square(GridWidth, GridHeight, CellSize)
	s.board = board.NewPlugin(grid, &board.SingleOccupancy{}, s.world).
		WithShaping(board.Shaping{Step: 5, MaxStep: 20}) // = and - under the cursor, L-drag levels
	s.board.CellKindDict().Create(
		board.CellKind{Name: board.Named("water"), Cost: 1, Allows: board.Water | board.Air, Shine: 0.9},
		board.CellKind{Name: board.Named("field"), Cost: 1.5, Allows: board.Land | board.Air}.Costing(board.Air, 1),
		board.CellKind{Name: board.Named("forest"), Cost: 3, Allows: board.Land | board.Air, Veil: 0.6, Height: 8}.Costing(board.Air, 1),
		board.CellKind{Name: board.Named("hills"), Cost: 4, Allows: board.Land | board.Air}.Costing(board.Air, 1),
		board.CellKind{Name: board.Named("mountain"), Cost: 8, Allows: board.Land | board.Air}.Costing(board.Air, 1),
		board.CellKind{Name: board.Named("road"), Cost: 1, Allows: board.Land | board.Air},
	)
	if err := s.board.RegisterBehavior(board.Each[board.Mover](s.drown)); err != nil {
		return err
	}
	if err := ctx.Use(s.board); err != nil {
		return err
	}
	if err := ctx.Use(s.isometry.WithBoard(s.board)); err != nil {
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

	s.vision = vision.NewPlugin(s.world)
	if err := s.vision.RegisterBehavior(vision.Between(plugin.Any, plugin.Any, faceTravel)); err != nil {
		return err
	}
	if err := ctx.Use(s.vision); err != nil {
		return err
	}

	s.sky = sky.NewPlugin(s.world, sky.Config{})
	if err := ctx.Use(s.sky); err != nil {
		return err
	}

	s.players = players.NewPlugin(s.world, s.selection, s.nav, s.board, s.sky)
	if err := s.players.Local("player").Bind(s.players.Defaults()...); err != nil {
		return err
	}
	if err := ctx.Use(s.players); err != nil {
		return err
	}

	s.state = &State{}

	s.defineKinds()

	main := &mainScene{stage: s, tps: ctx.TPS()}
	stack, err := game.NewStack(main)
	if err != nil {
		return err
	}
	s.stack = stack
	comp := stack.Composition()
	comp.Show(main.Name())
	return ctx.Track(comp)
}

func (s *mainStage) Restore(p game.Persistence) (bool, error) {
	saves, err := p.List(saveBasePath)
	if err != nil {
		return false, err
	}
	if !slices.Contains(saves, "") {
		return false, nil
	}
	if err := p.Load(saveBasePath, "", s.state); err != nil {
		return false, err
	}
	log.Printf("loaded saved island (save #%d)", s.state.Saves)
	return true, nil
}

// unit is the row the unit kind spawns from: where it starts and where it heads.
type unit struct{ start, target board.CellID }

// defineKinds says what this game's entities are, fresh or restored.
func (s *mainStage) defineKinds() {
	brd := s.board.Res.Logic.Board
	// Every unit stands 8 tall with its eye at 6, in proportion to hills of 20 and mountains of 40:
	// the smoothed edge of a plateau is a slope a unit or two below its top, and an eye lower than
	// that sees the rim, not the valley. The board writes where a unit stands in height.
	units := board.NewUnits[unit](s.board, board.Shape{Size: EntitySize, Height: 8}, func(u unit) geom.Vec { return brd.CellCenter(u.start) })
	order := comp.Load(func(u unit) navigation.MoveOrder { return navigation.MoveOrder{Target: u.target} })
	sight := func(eye float64) comp.Comp {
		return comp.Const(vision.Sight{Facing: geom.NewVec(1, 0), HalfAngle: sightHalf, Radius: sightRadius, Eye: eye})
	}
	s.unit = units.Define("unit", board.Mover{Domain: board.Land}, world.Steering{MaxSpeed: UnitSpeed, Accel: UnitSpeed * 2, Brake: UnitSpeed * 4, V0: UnitSpeed / 2, TurnRate: 0.15},
		order, comp.Tagged(s.selection.Tags().Selectable, s.selection.Tags().Selected),
		sight(6), comp.Const(vision.SightOutline{}),
	)
	// The hawk flies 40 above the ground on the Air plane: its eye looks over the hills and the
	// forests a walker's cone climbs and stops at.
	s.hawk = units.Define("hawk", board.Mover{Domain: board.Air, Lift: 40}, world.Steering{MaxSpeed: UnitSpeed * 1.5, Accel: UnitSpeed * 2, Brake: UnitSpeed * 4, V0: UnitSpeed / 2, TurnRate: 0.1},
		order, comp.Tagged(s.selection.Tags().Selectable),
		sight(1), comp.Const(vision.SightOutline{}),
	)
}

// Spawn lays the island out and puts a unit at every road stop, bound for the opposite one.
func (s *mainStage) Spawn() error {
	layout, stops := islandLayout(s.board.Res.Logic.Board)
	s.board.Seed(layout)

	entries := make([]kind.Entry, 0, len(stops)+1)
	for i, from := range stops {
		entries = append(entries, s.unit.Entry(unit{start: from, target: stops[(i+len(stops)/2)%len(stops)]}))
	}
	// The hawk crosses the island from the first stop to the one across the mountains.
	entries = append(entries, s.hawk.Entry(unit{start: stops[0], target: stops[len(stops)/2]}))
	s.world.Seed(entries...)
	return nil
}

// faceTravel points each unit's Sight where it is heading, turning in place included; a unit that
// stops keeps its last heading, so its Sight stays where it looked.
func faceTravel(_ plugin.Tick, s vision.Sighting) {
	if d := s.Base.Vel.Dir; d.X != 0 || d.Y != 0 {
		s.Sight.Facing = d
	}
}

// drown despawns a unit standing where its domain may not — pushed into the sea, say.
func (s *mainStage) drown(t plugin.Tick, m *board.Mover, st board.Standing) {
	if st.Fell(m.Domain) {
		log.Printf("unit %d drowned in the %s at cell %d", st.ID, st.Kind.Name, st.Cell)
		s.world.Despawn(t.CmdBuf, st.ID)
	}
}

func (s *mainStage) Update(ctx goke.RunCtx, d time.Duration) {
	s.world.RunPlan(ctx, d)
	s.collision.RunPlan(ctx, d)
	s.board.RunPlan(ctx, d)
	s.nav.RunPlan(ctx, d)
	s.vision.RunPlan(ctx, d)
	s.sky.RunPlan(ctx, d)
	s.selection.RunPlan(ctx, d)
	s.players.RunPlan(ctx, d)
	ctx.Sync()
}

// =========================== Scene ===========================

type mainScene struct {
	stage *mainStage
	tps   *game.TPS
	none  int
}

var _ game.Scene = (*mainScene)(nil)

func (m *mainScene) Name() string { return "main" }

func (m *mainScene) Layers() []render.Layer {
	s := m.stage

	worldAtlas := render.NewAtlas()
	worldAtlas.RegisterAt(s.unit.SpriteID(), EntitySize, render.Solid(color.RGBA{R: 230, G: 80, B: 80, A: 255}))
	worldAtlas.RegisterAt(s.hawk.SpriteID(), EntitySize, render.Diamond(color.RGBA{R: 120, G: 130, B: 60, A: 255}))
	worldAtlas.Close()
	s.world.WithRenderer(worldAtlas)

	kinds := s.board.CellKindDict()
	boardAtlas := render.NewAtlas()
	for name, c := range map[string]color.RGBA{
		"water":    {R: 40, G: 90, B: 170, A: 255},
		"field":    {R: 120, G: 170, B: 80, A: 255},
		"forest":   {R: 30, G: 90, B: 45, A: 255},
		"hills":    {R: 150, G: 140, B: 70, A: 255},
		"mountain": {R: 120, G: 120, B: 125, A: 255},
		"road":     {R: 190, G: 170, B: 120, A: 255},
	} {
		k, _ := kinds.Get(name)
		boardAtlas.RegisterAt(k.SpriteID, CellSize, render.Solid(c))
	}
	boardAtlas.Close()
	s.board.WithRenderer(boardAtlas)
	s.board.Res.Render.ShowGridLines = true // B toggles it; the grid shows the relief best

	pathAtlas, pathSprites := navigation.RegisterDefaultPathSprites(CellSize, 2, color.RGBA{R: 255, G: 140, B: 0, A: 255})
	s.nav.SetPathSprites(pathSprites)
	s.nav.WithRenderer(pathAtlas)
	s.vision.WithRenderer(nil)
	s.selection.WithRenderer(nil)

	count := func() int { return s.world.Res.Telemetry.Count }
	// The terrain and the entities are one picture sorted by depth; the cones and the overlays go on top.
	layers := []render.Layer{render.NewComposer(s.board.Renderer(), s.world.Renderer(), s.vision.Renderer(), s.selection.Renderer(), s.nav.Renderer())}
	return append(layers, render.NewTelemetryRenderer(&m.tps.Ticks, count, &m.none).With(s.sky.Reporter()))
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
		case ebiten.KeyF5:
			s.state.Saves++
			if err := runtime.Persistence().Save(saveBasePath, "", s.state); err != nil {
				log.Printf("save: %v", err)
				continue
			}
			log.Printf("saved (save #%d)", s.state.Saves)
		}
	}
}

func (m *mainScene) Focusable() bool { return true }
