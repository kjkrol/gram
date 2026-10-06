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
	"github.com/kjkrol/gram/plugins/atmosphere/air"
	"github.com/kjkrol/gram/plugins/atmosphere/calendar"
	"github.com/kjkrol/gram/plugins/atmosphere/climate"
	"github.com/kjkrol/gram/plugins/atmosphere/climate/weather"
	"github.com/kjkrol/gram/plugins/atmosphere/sky"
	"github.com/kjkrol/gram/plugins/atmosphere/weathering"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/unit"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/navigation"
	"github.com/kjkrol/gram/plugins/players"
	"github.com/kjkrol/gram/plugins/selection"
	"github.com/kjkrol/gram/plugins/topography"
	"github.com/kjkrol/gram/plugins/vision"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/steering"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/rule"
	"github.com/kjkrol/gram/rule/effect"
)

const (
	TPS          = 60
	CellSize     = island.CellSize
	WorldWidth   = island.GridWidth * CellSize
	WorldHeight  = island.GridHeight * CellSize
	ScreenWidth  = 1024
	ScreenHeight = 768
	EntitySize   = 3
	spritePx     = 22
	UnitSpeed    = CellSize * 3 / 4
	Sprint       = 4
	PlateauUnits = 20
	MaxEntCount  = 4*island.Stops + PlateauUnits
	saveBasePath = "board-topography"
)

var scale = world.Scale{Metres: 100.0 / CellSize}

// =========================== Game ===========================

// Demo is the island demo — exactly one Stage (mainStage below).
type Demo struct{ stage *mainStage }

var _ game.Game = (*Demo)(nil)

func NewDemo() *Demo { return &Demo{stage: newStage()} }

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
	game.Stage

	world      *world.Plugin
	board      *board.Plugin
	topography *topography.Plugin
	nav        *navigation.Plugin
	collision  *collision.Plugin
	selection  *selection.Plugin
	players    *players.Plugin
	player     *players.Player
	rival      *players.Player
	vision     *vision.Plugin
	atmosphere *atmosphere.Plugin
	weather    weathering.Config
	stops      []cell.ID
}

func newStage() *mainStage {
	s := &mainStage{}
	s.Stage = stage.New("board-topography").
		Plugins(s.usePlugins).
		Players(s.definePlayers).
		Cells(s.defineCells).
		Effects(s.defineEffects).
		Rules(s.defineRules).
		Commands(s.defineCommands).
		Kinds(s.defineKinds).
		Controls(s.bindKeys).
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
		Scale:    scale,
		Space:    world.SpaceCfg{Width: WorldWidth, Height: WorldHeight},
		Entities: world.EntitiesCfg{MaxCount: MaxEntCount, MinSize: EntitySize, MaxSize: EntitySize},
		Camera:   camera.Config{ViewportWidth: ScreenWidth, ViewportHeight: ScreenHeight},
		Heights:  true,
	})
	s.world.Camera().CenterOn(WorldWidth/2, WorldHeight/2, 0)
	grid := grid.DefaultGrids{}.Square(island.GridWidth, island.GridHeight, CellSize)
	s.collision = collision.NewPlugin(s.world)
	s.board = board.NewPlugin(grid, &cell.SingleOccupancy{}, s.world).WithCollision(s.collision).WithLog(log.Default())
	s.topography = island.Style(topography.NewPlugin(s.world, s.board, topography.Config{
		Cell:        CellSize,
		HeightUnit:  1,
		Isometric:   true,
		Perspective: true,
		Shaping:     topography.Shaping{Step: scale.Units(5 * island.Metres), MaxStep: scale.Units(20 * island.Metres)}}))
	s.selection = selection.NewPlugin(s.world)
	s.topography.WithSelection(s.selection)
	s.nav = navigation.NewPlugin(s.board, s.world, s.selection).WithCollision(s.collision).WithSpacing(navigation.BodySpacing)
	s.vision = vision.NewPlugin(s.world).WithBoard(s.board).WithGroundStep(scale.Units(50))
	s.atmosphere = atmosphere.NewPlugin(s.world, atmosphere.Config{
		Calendar: calendar.Config{
			Start:  18 * time.Hour,
			Season: calendar.Summer,
			Year:   calendar.EarthYear,
		},
		Climate: climate.Config{
			Zone:     climate.Mediterranean,
			Weathers: weathers,
			Start:    weather.Clear,
			Seed:     uint64(time.Now().UnixNano()),
		},
		Running: &atmosphere.Running{
			Day:        true,
			Weather:    true,
			Wind:       true,
			Clouds:     true,
			Falls:      true,
			Weathering: true,
			Stars:      true,
			Moon:       true,
		},
	})
	s.topography.WithAtmosphere(s.atmosphere)
	s.players = players.NewPlugin(s.world, s.board, s.selection, s.nav, s.atmosphere, s.topography, s.vision).WithSaves(saveBasePath)
	for _, p := range []plugin.Plugin{s.collision, s.board, s.selection, s.topography, s.nav, s.vision, s.atmosphere, s.players} {
		if err := ctx.Use(p); err != nil {
			return err
		}
	}
	return nil
}

func (s *mainStage) definePlayers() error {
	s.player = s.players.Local("player")
	s.rival = s.players.Add("rival")
	return s.player.Bind(s.players.Defaults()...)
}

func (s *mainStage) defineCells() {
	s.board.CellKinds().Create(island.Kinds(scale.Units(20))...)
	s.weather = s.defineClimate()
}

func (s *mainStage) defineEffects() {
	s.atmosphere.WithWeathering(s.board, s.weather)
	night := s.atmosphere.Calendar().Config().Day / 2
	s.world.Effects().Define(BloodMoonEf, effect.Spec{effect.Lasts(night),
		effect.Alter(func(m *sky.Moon) {
			m.Color, m.Face = render.Light{1, 0.25, 0.2}, render.Light{1, 0.3, 0.25}
			m.Strength *= 2
		})})
}

func (s *mainStage) defineRules() {
	s.world.Roles().Define(MortalRole,
		rule.Then[unit.Standing]("drown", rule.All, rule.If(unit.Standing.Fallen, rule.Order(world.Despawn{}))))
	bloodMoon := s.world.Effects().Named(BloodMoonEf)
	s.world.Roles().Define(LunarRole,
		rule.Then[sky.Moonrise]("a blood moon rises", rule.All, rule.If(sky.Moonrise.Full, rule.Apply(bloodMoon))))
	s.atmosphere.Plays(s.world.Roles().Named(LunarRole))
}

func (s *mainStage) defineCommands() {
	s.world.Commands().Define(BleedCmd, rule.Toggle(s.world.Effects().Named(BloodMoonEf)).On(s.atmosphere))
}

func (s *mainStage) bindKeys() error {
	return s.player.Bind(control.Give(control.KeyPress{Key: control.KeyM}, "Blood moon, on or off", s.world.Commands().Named(BleedCmd)))
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
	log.Print("loaded saved island")
	return true, nil
}

type unitRow struct{ start, target cell.ID }

func (s *mainStage) defineKinds() {
	brd := s.board.Res.Logic.Board
	units := board.NewUnits[unitRow](s.board, board.Shape{Size: EntitySize, Height: scale.Units(20)}, func(u unitRow) geom.Vec { return brd.CellCenter(u.start) })
	order := comp.Load(func(u unitRow) navigation.MoveOrder { return navigation.MoveOrder{Target: u.target} })
	sight := comp.Const(vision.Sight{Facing: geom.NewVec(1, 0), Radius: 960, Ahead: true})
	eye := comp.Const(world.Eye{Angle: 72 * math.Pi / 180})
	walker := steering.Steering{MaxSpeed: UnitSpeed, Sprint: Sprint, Accel: UnitSpeed * 2, Brake: UnitSpeed * 4, V0: UnitSpeed / 2, TurnRate: 0.15}
	mortal := rule.Plays(s.world.Roles().Named(MortalRole))
	units.Define(UnitKind, unit.Mover{Domain: cell.Land}, walker,
		order,
		sight, eye, mortal,
	)
	units.Define(PlateauKind, unit.Mover{Domain: cell.Land}, walker,
		sight, eye, mortal,
	)
	units.Define(RivalKind, unit.Mover{Domain: cell.Land}, walker,
		order,
		sight, eye, mortal,
	)
	units.Define(HawkKind,
		unit.Mover{
			Domain:    cell.Air,
			Lift:      scale.Units(300),
			Clearance: scale.Units(20),
			Ceiling:   scale.Units(air.CloudBase - 100),
		},
		steering.Steering{
			MaxSpeed: UnitSpeed * 1.5,
			Sprint:   Sprint,
			Accel:    UnitSpeed * 2,
			Brake:    UnitSpeed * 4,
			V0:       UnitSpeed / 2,
			TurnRate: 0.1},
		order,
		sight, eye,
	)
}

func (s *mainStage) layOut() {
	layout, heights, stops := island.Layout(s.board.Res.Logic.Board)
	s.stops = stops
	s.board.Seed(layout)
	metres := scale.Units(island.Metres)
	s.topography.Seed(func(p geom.Vec) float64 { return heights(p) * metres })
}

func (s *mainStage) placeUnits() {
	entries := make([]kind.Entry, 0, len(s.stops)+1)
	for i, from := range s.stops {
		walkers, whose := kind.Named[unitRow](s.world.Kinds(), UnitKind), []any{players.Give{To: s.player.ID}, selection.Allow{Selected: true}}
		if i%2 == 0 {
			walkers, whose = kind.Named[unitRow](s.world.Kinds(), RivalKind), []any{players.Give{To: s.rival.ID}, selection.Allow{}}
		}
		entries = append(entries, walkers.Entry(unitRow{start: from, target: s.stops[(i+len(s.stops)/2)%len(s.stops)]}).Told(whose...))
	}
	entries = append(entries, kind.Named[unitRow](s.world.Kinds(), HawkKind).Entry(unitRow{start: s.stops[0], target: s.stops[len(s.stops)/2]}).Told(players.Give{To: s.player.ID}, selection.Allow{}))
	for _, c := range island.Plateau(s.board.Res.Logic.Board)[:PlateauUnits] {
		entries = append(entries, kind.Named[unitRow](s.world.Kinds(), PlateauKind).Entry(unitRow{start: c, target: c}).Told(players.Give{To: s.player.ID}, selection.Allow{}))
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
	s.topography.RunPlan(ctx, d)
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
	playerColor := render.Solid(color.RGBA{R: 230, G: 80, B: 80, A: 255})
	worldAtlas.RegisterAt(kind.Named[unitRow](s.world.Kinds(), UnitKind).SpriteID(), spritePx, playerColor)
	worldAtlas.RegisterAt(kind.Named[unitRow](s.world.Kinds(), PlateauKind).SpriteID(), spritePx, playerColor)
	worldAtlas.RegisterAt(kind.Named[unitRow](s.world.Kinds(), RivalKind).SpriteID(), spritePx, render.Solid(color.RGBA{R: 70, G: 110, B: 230, A: 255}))
	worldAtlas.RegisterAt(kind.Named[unitRow](s.world.Kinds(), HawkKind).SpriteID(), spritePx, render.Diamond(color.RGBA{R: 120, G: 130, B: 60, A: 255}))
	worldAtlas.Close()
	s.world.WithRenderer(worldAtlas)

	s.board.WithRenderer(nil)
	s.board.Res.Render.ShowGridLines = true
	s.nav.WithRenderer(nil)
	s.vision.WithRenderer(nil)
	s.selection.WithRenderer(nil)

	count := func() int { return s.world.Res.Telemetry.Count }
	layers := []render.Layer{render.NewComposer(
		s.atmosphere.Renderer(), s.board.Renderer(), s.topography.Renderer(),
		s.world.Renderer(), s.vision.Renderer(), s.selection.Renderer(),
		s.nav.Renderer(), s.atmosphere.Precipitation())}
	return append(layers, render.NewTelemetryRenderer(&m.tps.Ticks, count).With(s.world.Clock().Reporter(), s.atmosphere.Reporter()), s.world.Clock().HUD())
}

func (m *mainScene) Viewports(screen geom.AABB) []render.Viewport {
	return m.stage.players.Viewports(screen)
}

func (m *mainScene) HandleEvents(events *control.InputEvents, runtime game.Runtime, composition game.Composition) {
	m.stage.players.Handle(events, runtime, composition)
}

func (m *mainScene) Focusable() bool { return true }
