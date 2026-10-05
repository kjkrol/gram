// Command board is the island on the simple map: a flat world whose board draws itself — every
// kind in its colour from the board's own atlas, the streams, rivers, roads and bridges as plain
// bands — and prices a step by its kind alone. Units walk from stop to stop over the roads, slower
// off them, with sight cones (Shift+C shows them) and routes (Shift+P); a day goes by over the flat map (plugins/atmosphere): the tiles and
// the units tinted by the hour, dark at night and warm at dawn, the clouds' shadows drifting over
// the whole screen, rain and snow falling, snow lying and the shores freezing in winter. Space
// pauses the game, ] and [ set its tempo, P freezes the light, Shift+W changes the weather; WASD,
// the wheel, a middle drag or the cursor at an edge move the camera; K lists every key.
package main

import (
	"image/color"
	"log"
	"math"
	"slices"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/entity/kind/comp"
	"github.com/kjkrol/gram/examples/island"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/game/stage"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/atmosphere"
	"github.com/kjkrol/gram/plugins/atmosphere/calendar"
	"github.com/kjkrol/gram/plugins/atmosphere/climate"
	"github.com/kjkrol/gram/plugins/atmosphere/weathering"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/unit"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/navigation"
	"github.com/kjkrol/gram/plugins/players"
	"github.com/kjkrol/gram/plugins/selection"
	"github.com/kjkrol/gram/plugins/vision"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/steering"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/rule"
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
	MaxEntCount  = 4 * island.Stops

	saveBasePath = "board"
)

// =========================== Game ===========================

// Demo is the board demo — exactly one Stage (mainStage below).
type Demo struct{ stage *mainStage }

var _ game.Game = (*Demo)(nil)

func NewDemo() *Demo { return &Demo{stage: newStage()} }

func (d *Demo) Props() game.Props {
	return game.Props{
		Title:       "gram — the island on the simple map",
		ScreenWidth: ScreenWidth, ScreenHeight: ScreenHeight, Resizable: true,
		TargetTPS: TPS,
	}
}

func (d *Demo) Stages() (map[string]game.Stage, string) {
	return map[string]game.Stage{d.stage.Name(): d.stage}, d.stage.Name()
}

// =========================== Stage ===========================

type mainStage struct {
	game.Stage // defined a section at a time: newStage

	world      *world.Plugin
	board      *board.Plugin
	nav        *navigation.Plugin
	collision  *collision.Plugin
	selection  *selection.Plugin
	players    *players.Plugin
	player     *players.Player // the one at this keyboard: the units are its
	vision     *vision.Plugin
	atmosphere *atmosphere.Plugin
	weather    weathering.Config // how the weather lies on the island
	stops      []cell.ID         // where the units start, as the layout says
}

// newStage defines the game a section at a time, each building on those before it.
func newStage() *mainStage {
	s := &mainStage{}
	s.Stage = stage.New("board").
		Plugins(s.usePlugins).
		Players(s.definePlayer).
		Cells(s.defineCells).
		Effects(s.defineEffects).
		Rules(s.defineRules).
		Kinds(s.defineKinds).
		Looks(s.defineLooks).
		Scenes(s.defineScenes).
		Restore(s.restore).
		Layout(s.layOut).
		Units(s.placeUnits).
		Update(s.update)
	return s
}

func (s *mainStage) usePlugins(ctx game.Initializer) error {
	s.world = ctx.UseWorld(world.Config{
		Space:    world.SpaceCfg{Width: WorldWidth, Height: WorldHeight},
		Entities: world.EntitiesCfg{MaxCount: MaxEntCount, MinSize: EntitySize, MaxSize: EntitySize},
		Camera:   camera.Config{ViewportWidth: ScreenWidth, ViewportHeight: ScreenHeight},
	})
	s.world.Camera().CenterOn(WorldWidth/2, WorldHeight/2, 0)

	// the simple map: the board's own flat look, the kinds in their colours, the ways as plain bands
	grid := grid.DefaultGrids{}.Square(island.GridWidth, island.GridHeight, CellSize)
	s.collision = collision.NewPlugin(s.world)
	s.board = board.NewPlugin(grid, &cell.SingleOccupancy{}, s.world).WithCollision(s.collision).WithLog(log.Default())
	s.selection = selection.NewPlugin(s.world)
	s.nav = navigation.NewPlugin(s.board, s.world, s.selection).WithCollision(s.collision)
	s.vision = vision.NewPlugin(s.world).WithBoard(s.board)
	// A temperate island whose weather is thrown anew every run.
	s.atmosphere = atmosphere.NewPlugin(s.world, atmosphere.Config{Calendar: calendar.Config{Season: calendar.Autumn}, Climate: climate.Config{Zone: climate.Temperate, Seed: uint64(time.Now().UnixNano())}})
	s.players = players.NewPlugin(s.world, s.board, s.selection, s.nav, s.atmosphere, s.vision).WithSaves(saveBasePath)
	for _, p := range []plugin.Plugin{s.collision, s.board, s.selection, s.nav, s.vision, s.atmosphere, s.players} {
		if err := ctx.Use(p); err != nil {
			return err
		}
	}
	s.atmosphere.WithBoard(s.board) // a flat board: its tiles and the units lit by the hour, leaning in the wind
	return nil
}

func (s *mainStage) definePlayer() error {
	s.player = s.players.Local("player")
	return s.player.Bind(s.players.Defaults()...)
}

func (s *mainStage) defineCells() {
	s.board.CellKinds().Create(island.Kinds(0)...)
	s.weather = s.defineClimate()
}

func (s *mainStage) defineEffects() { s.atmosphere.WithWeathering(s.board, s.weather) }

func (s *mainStage) defineRules() {
	s.world.Roles().Define(MortalRole,
		rule.Then[unit.Standing]("drown", rule.All, rule.If(unit.Standing.Fallen, rule.Order(world.Despawn{}))))
}

func (s *mainStage) defineLooks() error { return s.vision.Draw(render.Show(s.selection.IsSelected)) }

func (s *mainStage) defineScenes(ctx game.Initializer) []game.Scene {
	main := &mainScene{stage: s, tps: ctx.TPS()}
	return []game.Scene{main}
}

func (s *mainStage) restore(p game.Persistence) (bool, error) {
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
	log.Print("loaded the saved island")
	return true, nil
}

// unit is the row the unit kind spawns from: where it starts and where it heads.
type unitRow struct{ start, target cell.ID }

func (s *mainStage) defineKinds() {
	brd := s.board.Res.Logic.Board
	units := board.NewUnits[unitRow](s.board, board.Shape{Size: EntitySize}, func(u unitRow) geom.Vec { return brd.CellCenter(u.start) })
	order := comp.Load(func(u unitRow) navigation.MoveOrder { return navigation.MoveOrder{Target: u.target} })
	units.Define(UnitKind, unit.Mover{Domain: cell.Land}, steering.Steering{MaxSpeed: UnitSpeed, Accel: UnitSpeed * 2, Brake: UnitSpeed * 4, V0: UnitSpeed / 2, TurnRate: 0.15},
		order,
		comp.Const(vision.Sight{Facing: geom.NewVec(1, 0), Radius: sightRadius, Ahead: true}), comp.Const(world.Eye{Angle: 2 * sightHalf}), comp.Const(vision.SightOutline{}),
		rule.Plays(s.world.Roles().Named(MortalRole)),
	)
}

func (s *mainStage) layOut() {
	layout, _, stops := island.Layout(s.board.Res.Logic.Board)
	s.stops = stops
	s.board.Seed(layout)
}

func (s *mainStage) placeUnits() {
	entries := make([]kind.Entry, 0, len(s.stops))
	for i, from := range s.stops {
		entries = append(entries, kind.Named[unitRow](s.world.Kinds(), UnitKind).Entry(unitRow{start: from, target: s.stops[(i+len(s.stops)/2)%len(s.stops)]}).Told(players.Give{To: s.player.ID}, selection.Allow{Selected: true}))
	}
	s.world.Seed(entries...)
}

func (s *mainStage) update(ctx goke.RunCtx, d time.Duration) {
	s.world.RunPlan(ctx, d)
	s.collision.RunPlan(ctx, d)
	s.board.RunPlan(ctx, d)
	s.nav.RunPlan(ctx, d)
	s.vision.RunPlan(ctx, d)
	s.atmosphere.RunPlan(ctx, d)
	s.selection.RunPlan(ctx, d)
	s.players.RunPlan(ctx, d)
	ctx.Sync()
}

// =========================== Scene ===========================

type mainScene struct {
	stage *mainStage
	keys  players.SceneKeys
	tps   *game.TPS
}

var _ game.Scene = (*mainScene)(nil)

func (m *mainScene) Name() string { return "main" }

func (m *mainScene) Layers() []render.Layer {
	s := m.stage

	worldAtlas := render.NewAtlas()
	worldAtlas.RegisterAt(kind.Named[unitRow](s.world.Kinds(), UnitKind).SpriteID(), EntitySize, render.Solid(color.RGBA{R: 230, G: 80, B: 80, A: 255}))
	worldAtlas.Close()
	s.world.WithRenderer(worldAtlas)

	s.board.WithRenderer(nil) // the board's own atlas: every kind in its colour
	s.board.Res.Render.ShowGridLines = false

	s.nav.WithRenderer(nil)
	s.vision.WithRenderer(nil)
	s.selection.WithRenderer(nil)

	count := func() int { return s.world.Res.Telemetry.Count }
	// the tiles and the bands, the units, the clouds' shadows over them all, then the cones, the
	// overlays and the rain
	layers := []render.Layer{render.NewComposer(s.atmosphere.Renderer(), s.board.Renderer(), s.world.Renderer(), s.atmosphere.Clouds(), s.vision.Renderer(), s.selection.Renderer(), s.nav.Renderer(), s.atmosphere.Precipitation())}
	return append(layers, render.NewTelemetryRenderer(&m.tps.Ticks, count).With(s.world.Clock().Reporter(), s.atmosphere.Reporter()), s.world.Clock().HUD())
}

// Viewports are where the world is shown: the local players' views.
func (m *mainScene) Viewports(screen geom.AABB) []render.Viewport {
	return m.stage.players.Viewports(screen)
}

func (m *mainScene) HandleEvents(events *control.InputEvents, runtime game.Runtime, composition game.Composition) {
	m.stage.players.Handle(events, runtime, composition)
}

func (m *mainScene) Focusable() bool { return true }
