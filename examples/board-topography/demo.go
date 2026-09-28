// Command board-topography is the island in relief: a Quasi3D world whose board is drawn and
// priced by a topography — a range of peaks up to 250 and a plateau 118 up, rock on the heights,
// sand on the beaches, sea cliffs in the north, earth between, streams and rivers running down to
// the sea and falling over the cliffs, roads from stop to stop over bridges, slower up the slopes
// and routed round them; seen isometrically, Transport Tycoon's way, from above or in perspective
// — Tab goes round — the units billboards, a hawk 40 up whose cone looks over everything a walker's stops at. A
// day goes by (plugins/atmosphere): long shadows morning and evening, dark nights; Space pauses
// the game, ] and [ set its tempo — the clock bottom-left shows it, and when the engine holds it
// back — P freezes the light, Shift+] and Shift+[ move the frozen light half an hour. The year is the Earth's, beginning in mid-spring; the weather goes by: clouds'
// shadows drift over the island, rain falls — snow in winter, lying until spring, ice along the
// shores — the sea roughens with the wind; Shift+W changes it. WASD, the wheel, a middle drag or
// the cursor at an edge move the camera; Q and E turn it, R raises its head and F bows it; V rides
// in the selected unit, first person — W walks, S stops, A and D turn, the mouse looks round, up at
// the sun and down at the feet, V or Tab leave, K lists these keys while riding; = and -
// shape the ground under the cursor, an
// L-drag levels it. K lists every key. The kinds are drawn from the board's own atlas of their
// colours.
package main

import (
	"image/color"
	"log"
	"math"
	"os"
	"slices"
	"strconv"
	"time"

	"github.com/kjkrol/aabbworld/geom"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/examples/island"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/atmosphere"
	"github.com/kjkrol/gram/plugins/atmosphere/calendar"
	"github.com/kjkrol/gram/plugins/atmosphere/climate"
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
	"github.com/kjkrol/gram/render"
)

const (
	TPS          = 60
	CellSize     = island.CellSize
	WorldWidth   = island.GridWidth * CellSize
	WorldHeight  = island.GridHeight * CellSize
	ScreenWidth  = 1024
	ScreenHeight = 768
	EntitySize   = 22
	UnitSpeed    = CellSize * 3
	sightRadius  = 220
	sightHalf    = math.Pi / 5
	// MaxEntCount is the units and the hawk, with room to spare.
	MaxEntCount = 4 * island.Stops

	saveBasePath = "board-topography"
)

type State struct{ Saves int }

// =========================== Game ===========================

// Demo is the island demo — exactly one Stage (mainStage below).
type Demo struct{ stage *mainStage }

var _ game.Game = (*Demo)(nil)

func NewDemo() *Demo { return &Demo{stage: &mainStage{}} }

func (d *Demo) Props() game.Props {
	return game.Props{
		Title:       "gram — the island in relief",
		ScreenWidth: ScreenWidth, ScreenHeight: ScreenHeight, Resizable: true,
		TargetTPS: TPS,
	}
}

func (d *Demo) Stages() (map[string]game.Stage, string) {
	return map[string]game.Stage{d.stage.Name(): d.stage}, d.stage.Name()
}

// =========================== Stage ===========================

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
	atmosphere *atmosphere.Plugin
	unit       kind.Of[unit]
	hawk       kind.Of[unit]
	stack      game.Scenes
	state      *State
}

var _ game.Stage = (*mainStage)(nil)

func (s *mainStage) Name() string { return "board-topography" }

func (s *mainStage) Stack() game.Scenes { return s.stack }

func (s *mainStage) Init(ctx game.Initializer) error {
	s.world = ctx.UseWorld(world.Config{
		Space:    world.SpaceCfg{Width: WorldWidth, Height: WorldHeight},
		Entities: world.EntitiesCfg{MaxCount: MaxEntCount, MinSize: EntitySize, MaxSize: EntitySize},
		Camera:   camera.Config{ViewportWidth: ScreenWidth, ViewportHeight: ScreenHeight},
		Quasi3D:  true,
	})
	// Start over the island's middle rather than the world's corner.
	s.world.Camera().CenterOn(WorldWidth/2, WorldHeight/2, 0)

	s.collision = collision.NewPlugin(s.world)
	if err := ctx.Use(s.collision); err != nil {
		return err
	}

	grid := board.DefaultGrids{}.Square(island.GridWidth, island.GridHeight, CellSize)
	s.board = board.NewPlugin(grid, &board.SingleOccupancy{}, s.world)
	s.board.CellKindDict().Create(island.Kinds(true)...)
	// the island in relief: its heights, the views of it (Tab), = and - shaping the ground under
	// the cursor and an L-drag levelling it; how the kinds look beyond their sprites — the sea
	// glinting under the land's blended grounds, the running water running
	s.topography = island.Style(topography.NewPlugin(s.world, s.board, topography.Config{Cell: CellSize, HeightUnit: 1, Isometric: true, Perspective: true, Shaping: topography.Shaping{Step: 5, MaxStep: 20}}))
	weather := s.defineClimate() // the snowy kinds and ice, and how the weather lies on the island
	if err := s.board.RegisterBehavior(board.Each[board.Mover](s.drown)); err != nil {
		return err
	}
	if err := ctx.Use(s.board); err != nil {
		return err
	}
	s.selection = selection.NewPlugin(s.world)
	if err := ctx.Use(s.selection); err != nil {
		return err
	}
	if err := ctx.Use(s.topography.WithSelection(s.selection)); err != nil {
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

	// A temperate island — central Europe, southern Scandinavia before the warming — whose weather is
	// thrown anew every run; the same seed would give the same weather. The noon sun stands over the
	// north-west, as it does by default: beyond the sea as the view looks at it, so the water throws
	// it back towards the eye. The weather lies on the island as defineClimate says.
	seed := uint64(time.Now().UnixNano())
	if measure { // TEMP-MEASURE: the same weather in every run, GRAM_SEED or 7
		seed = 7
		if v, err := strconv.ParseUint(os.Getenv("GRAM_SEED"), 10, 64); err == nil {
			seed = v
		}
	}
	s.atmosphere = atmosphere.NewPlugin(s.world, atmosphere.Config{Calendar: calendar.Config{Season: calendar.Spring, Year: calendar.EarthYear}, Climate: climate.Config{Zone: climate.Temperate, Seed: seed}}).WithWeathering(s.board, weather)
	if err := ctx.Use(s.atmosphere); err != nil {
		return err
	}

	s.players = players.NewPlugin(s.world, s.selection, s.nav, s.atmosphere, s.topography)
	if err := s.players.Local("player").Bind(s.players.Defaults()...); err != nil {
		return err
	}
	if err := ctx.Use(s.players); err != nil {
		return err
	}

	s.state = &State{}

	s.defineKinds()

	main := &mainScene{stage: s, tps: ctx.TPS()}
	// the scene's own keys, labelled for the shortcuts list: K opens it, Esc closes it
	main.keys = players.SceneKeys{
		{Key: ebiten.KeyK, Label: "Shortcuts; Esc closes them", Do: func(rt game.Runtime, c game.Composition) { s.shortcuts.Open(rt, c) }},
		{Key: ebiten.KeyEscape, Shift: true, Label: "Quit", Do: func(rt game.Runtime, _ game.Composition) { rt.Quit() }},
		{Key: ebiten.KeyB, Label: "Toggle the grid", Do: func(game.Runtime, game.Composition) { s.board.Res.Render.ToggleShowGridLines() }},
		{Key: ebiten.KeyF5, Label: "Save the game", Do: func(rt game.Runtime, _ game.Composition) {
			s.state.Saves++
			if err := rt.Persistence().Save(saveBasePath, "", s.state); err != nil {
				log.Printf("save: %v", err)
				return
			}
			log.Printf("saved (save #%d)", s.state.Saves)
		}},
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
	// Every unit stands 8 tall with its eye at 6: from a slope's edge an eye sees the rim, not the
	// valley below. The board writes where a unit stands in height.
	units := board.NewUnits[unit](s.board, board.Shape{Size: EntitySize, Height: 8}, func(u unit) geom.Vec { return brd.CellCenter(u.start) })
	order := comp.Load(func(u unit) navigation.MoveOrder { return navigation.MoveOrder{Target: u.target} })
	sight := func(eye float64) comp.Comp {
		return comp.Const(vision.Sight{Facing: geom.NewVec(1, 0), HalfAngle: sightHalf, Radius: sightRadius, Eye: eye})
	}
	s.unit = units.Define("unit", board.Mover{Domain: board.Land}, world.Steering{MaxSpeed: UnitSpeed, Accel: UnitSpeed * 2, Brake: UnitSpeed * 4, V0: UnitSpeed / 2, TurnRate: 0.15},
		order, comp.Tagged(s.selection.Tags().Selectable, s.selection.Tags().Selected),
		sight(6), comp.Const(vision.SightOutline{}),
	)
	// The hawk flies 40 above the ground on the Air plane: its eye looks over the ridges a walker's
	// cone climbs and stops at, and it flies over them as over the flat.
	s.hawk = units.Define("hawk", board.Mover{Domain: board.Air, Lift: 40}, world.Steering{MaxSpeed: UnitSpeed * 1.5, Accel: UnitSpeed * 2, Brake: UnitSpeed * 4, V0: UnitSpeed / 2, TurnRate: 0.1},
		order, comp.Tagged(s.selection.Tags().Selectable),
		sight(1), comp.Const(vision.SightOutline{}),
	)
}

// Spawn lays the island out and puts a unit at every stop, bound for the one across the range; the
// way it takes goes round what is steep.
func (s *mainStage) Spawn() error {
	layout, heights, stops := island.Layout(s.board.Res.Logic.Board)
	s.board.Seed(layout)
	s.topography.Seed(heights)

	entries := make([]kind.Entry, 0, len(stops)+1)
	for i, from := range stops {
		entries = append(entries, s.unit.Entry(unit{start: from, target: stops[(i+len(stops)/2)%len(stops)]}))
	}
	// The hawk crosses the island from the first stop to the one across the range.
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

// TEMP-MEASURE: GRAM_FULLSCREEN=1 opens full screen at once, centred, the weather of GRAM_SEED
// (7), at GRAM_ZOOM (1) and GRAM_TEMPO (1), and logs the FPS, the clock and the clouds once a
// second; GRAM_NO_GRID=1 leaves the grid out.
var (
	measure   = os.Getenv("GRAM_FULLSCREEN") == "1"
	measured  time.Time
	fpsSum    float64
	fpsFrames int
)

func (s *mainStage) Update(ctx goke.RunCtx, d time.Duration) {
	if measure {
		if measured.IsZero() {
			ebiten.SetFullscreen(true)
			measured = time.Now()
			if os.Getenv("GRAM_NO_GRID") == "1" {
				s.board.Res.Render.ShowGridLines = false
			}
			if z, err := strconv.ParseFloat(os.Getenv("GRAM_ZOOM"), 64); err == nil && z > 0 && z != 1 {
				s.world.Camera().ZoomOut(float32(1/z), WorldWidth/2, WorldHeight/2)
			}
			s.world.Camera().CenterOn(WorldWidth/2, WorldHeight/2, 0)
			if t, err := strconv.ParseFloat(os.Getenv("GRAM_TEMPO"), 64); err == nil && t > 0 {
				if err := s.world.Clock().SetTempo(float32(t)); err != nil {
					log.Print(err)
				}
			}
		}
		if time.Since(measured) > time.Second {
			fps := ebiten.ActualFPS()
			if fps > 0 {
				fpsSum, fpsFrames = fpsSum+fps, fpsFrames+1
			}
			w, h := ebiten.Monitor().Size()
			log.Printf("MEASURE fps=%.1f tps=%.1f avg=%.1f screen=%dx%d zoom=%.2f clock=%q clouds=%.2f", fps, ebiten.ActualTPS(), fpsSum/float64(max(fpsFrames, 1)), w, h, s.world.Camera().Zoom(), s.world.Clock().Written(), s.world.Weather().Clouds)
			measured = time.Now()
		}
	}
	s.world.RunPlan(ctx, d)
	s.collision.RunPlan(ctx, d)
	s.board.RunPlan(ctx, d)
	s.nav.RunPlan(ctx, d)
	s.vision.RunPlan(ctx, d)
	s.atmosphere.RunPlan(ctx, d)
	s.selection.RunPlan(ctx, d)
	s.topography.RunPlan(ctx, d)
	s.players.RunPlan(ctx, d)
	ctx.Sync()
}

// =========================== Scene ===========================

type mainScene struct {
	stage *mainStage
	keys  players.SceneKeys
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

	s.board.WithRenderer(nil)               // the board's own atlas: every kind in its colour, the snowy ones and the ice too
	s.board.Res.Render.ShowGridLines = true // B toggles it; the grid shows the relief best

	pathAtlas, pathSprites := navigation.RegisterDefaultPathSprites(CellSize, 2, color.RGBA{R: 255, G: 140, B: 0, A: 255})
	s.nav.SetPathSprites(pathSprites)
	s.nav.WithRenderer(pathAtlas)
	s.vision.WithRenderer(nil)
	s.selection.WithRenderer(nil)

	count := func() int { return s.world.Res.Telemetry.Count }
	// The terrain and the entities are one picture sorted by depth; the cones and the overlays go on top.
	layers := []render.Layer{render.NewComposer(s.atmosphere.Renderer(), s.board.Renderer(), s.world.Renderer(), s.vision.Renderer(), s.selection.Renderer(), s.nav.Renderer(), s.atmosphere.Precipitation())}
	return append(layers, render.NewTelemetryRenderer(&m.tps.Ticks, count, &m.none).With(s.world.Clock().Reporter(), s.atmosphere.Reporter()), s.world.Clock().HUD())
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
