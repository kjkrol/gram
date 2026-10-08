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
	"github.com/kjkrol/gram/plugins/topography"
	"github.com/kjkrol/gram/plugins/vision"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/steering"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/rule"
	"github.com/kjkrol/gram/rule/effect"
	"github.com/kjkrol/gram/ui"
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

// Demo is the island demo — exactly one Stage (arena below).
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
		Title:       "gram — the island in relief",
		ScreenWidth: ScreenWidth, ScreenHeight: ScreenHeight, Resizable: true,
		TargetTPS: TPS,
	}
}

func (d *Demo) Stages() (map[string]game.Stage, string) {
	return map[string]game.Stage{d.stage.Name(): d.stage}, d.stage.Name()
}

// =========================== Stage ===========================

type arena struct {
	world      *world.Plugin
	board      *board.Plugin
	topography *topography.Plugin
	nav        *navigation.Plugin
	driving    *driving.Plugin
	collision  *collision.Plugin
	selection  *selection.Plugin
	players    *players.Plugin
	cameras    *cameras.Plugin
	player     *players.Player
	rival      *players.Player
	vision     *vision.Plugin
	atmosphere *atmosphere.Plugin
	stops      []cell.ID
}

// newArena makes the arena — the collector of the stage's plugins, which every
// section builds on — and defines the stage on it, a section at a time.
func newArena() (*arena, game.Stage) {
	s := &arena{}
	return s, stage.New(BoardTopographyStage).
		Plugins(s.usePlugins).
		Players(s.definePlayers).
		Effects(s.defineEffects).
		Rules(s.defineRules).
		Commands(s.defineCommands).
		Kinds(s.defineCells, s.defineKinds).
		Controls(s.bindKeys).
		Restore(s.restore).
		Spawn(s.spawnGround, s.spawnUnits).
		Scenes(s.defineScenes).
		Update(s.update)
}

func (s *arena) usePlugins(ctx game.Initializer) error {
	s.world = ctx.UseWorld(world.Config{
		Scale:    scale,
		Space:    world.SpaceCfg{Width: WorldWidth, Height: WorldHeight},
		Entities: world.EntitiesCfg{MaxCount: MaxEntCount, MinSize: EntitySize, MaxSize: EntitySize},
		Heights:  true,
	})
	grid := grid.DefaultGrids{}.Square(island.GridWidth, island.GridHeight, CellSize)
	s.collision = collision.NewPlugin(s.world)
	s.board = board.NewPlugin(grid, &cell.SingleOccupancy{}, s.world).WithCollision(s.collision).WithLog(log.Default())
	s.topography = island.Style(topography.NewPlugin(s.world, s.board, topography.Config{
		Cell:        CellSize,
		HeightUnit:  1,
		Perspective: true,
		Shaping:     topography.Shaping{Step: scale.Units(5 * island.Metres), MaxStep: scale.Units(20 * island.Metres)}}))
	s.selection = selection.NewPlugin(s.world)
	s.driving = driving.NewPlugin(s.world, s.selection).WithGround(s.board)
	s.nav = navigation.NewPlugin(s.board, s.world, s.selection, s.driving).WithCollision(s.collision).WithSpacing(navigation.BodySpacing)
	s.vision = vision.NewPlugin(s.world).WithBoard(s.board).WithGroundStep(scale.Units(50)).
		WithViews(render.Show(s.selection.IsSelected)) // only the selected ones' cones
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
	s.cameras = cameras.NewPlugin(s.world)
	s.players = players.NewPlugin(s.world, s.cameras, s.board, s.selection, s.nav, s.driving, s.atmosphere, s.topography, s.vision).WithSaves(saveBasePath)
	for _, p := range []plugin.Plugin{s.collision, s.board, s.selection, s.topography, s.nav, s.driving, s.vision, s.atmosphere, s.cameras, s.players} {
		if err := ctx.Use(p); err != nil {
			return err
		}
	}
	return nil
}

func (s *arena) definePlayers() error {
	s.player = s.players.Local("player")
	s.rival = s.players.Add("rival")
	return s.player.Bind(s.players.Defaults()...)
}

func (s *arena) defineCells() {
	island.Define(s.board.CellKinds(), scale.Units(20))
	s.defineWinterCells()
}

func (s *arena) defineEffects() {
	s.atmosphere.WithWeathering(s.board, s.defineClimate())
	night := s.atmosphere.Calendar().Config().Day / 2
	s.world.Effects().Define(BloodMoonEf, effect.Spec{effect.Lasts(night),
		effect.Alter(func(m *sky.Moon) {
			m.Color, m.Face = render.Light{1, 0.25, 0.2}, render.Light{1, 0.3, 0.25}
			m.Strength *= 2
		})})
}

func (s *arena) defineRules() {
	s.world.Roles().Define(MortalRole,
		rule.Then[unit.Standing]("drown", rule.All, rule.If(unit.Standing.Fallen, rule.Order(world.Despawn{}))))
	bloodMoon := s.world.Effects().Named(BloodMoonEf)
	s.world.Roles().Define(LunarRole,
		rule.Then[sky.Moonrise]("a blood moon rises", rule.All, rule.If(sky.Moonrise.Full, rule.Apply(bloodMoon))))
	s.atmosphere.Plays(s.world.Roles().Named(LunarRole))
}

func (s *arena) defineCommands() {
	s.world.Commands().Define(BleedCmd, rule.Toggle(s.world.Effects().Named(BloodMoonEf)).On(s.atmosphere))
}

func (s *arena) bindKeys() error {
	return s.player.Bind(
		control.Give(control.KeyPress{Key: control.KeyM}, "Blood moon, on or off", s.world.Commands().Named(BleedCmd)),
		cameras.MouseLookKey(control.KeyO), // first person looks round with the mouse only once asked
	)
}

func (s *arena) defineScenes(ctx game.Initializer) []game.Scene {
	main := &mainScene{arena: s, tps: ctx.TPS()}
	return []game.Scene{ui.NewScene(MainScene, main.screen()).Input(s.players.Handle)}
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
	log.Print("loaded saved island")
	return true, nil
}

type unitRow struct{ start, target cell.ID }

func (s *arena) defineKinds() {
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

func (s *arena) spawnGround() {
	layout, heights, stops := island.Layout(s.board.Res.Logic.Board)
	s.stops = stops
	s.board.Seed(layout)
	metres := scale.Units(island.Metres)
	s.topography.Seed(func(p geom.Vec) float64 { return heights(p) * metres })
}

func (s *arena) spawnUnits() {
	unitKind := kind.Named[unitRow](s.world.Kinds(), UnitKind)
	rivalKind := kind.Named[unitRow](s.world.Kinds(), RivalKind)
	hawkKind := kind.Named[unitRow](s.world.Kinds(), HawkKind)
	plateauKind := kind.Named[unitRow](s.world.Kinds(), PlateauKind)
	entries := make([]kind.Entry, 0, len(s.stops)+1)
	for i, from := range s.stops {
		walkers, whose := unitKind, []any{players.Give{To: s.player.ID}, selection.Allow{Selected: true}}
		if i%2 == 0 {
			walkers, whose = rivalKind, []any{players.Give{To: s.rival.ID}, selection.Allow{}}
		}
		entries = append(entries, walkers.Entry(unitRow{start: from, target: s.stops[(i+len(s.stops)/2)%len(s.stops)]}).Told(whose...))
	}
	entries = append(entries, hawkKind.Entry(unitRow{start: s.stops[0], target: s.stops[len(s.stops)/2]}).Told(players.Give{To: s.player.ID}, selection.Allow{}))
	for _, c := range island.Plateau(s.board.Res.Logic.Board)[:PlateauUnits] {
		entries = append(entries, plateauKind.Entry(unitRow{start: c, target: c}).Told(players.Give{To: s.player.ID}, selection.Allow{}))
	}
	s.world.Seed(entries...)
}

func (s *arena) update(ctx goke.RunCtx, d time.Duration) {
	s.world.RunPlan(ctx, d)
	s.collision.RunPlan(ctx, d)
	s.board.RunPlan(ctx, d)
	s.nav.RunPlan(ctx, d)
	s.driving.RunPlan(ctx, d)
	s.vision.RunPlan(ctx, d)
	s.atmosphere.RunPlan(ctx, d)
	s.selection.RunPlan(ctx, d)
	s.topography.RunPlan(ctx, d)
	s.cameras.RunPlan(ctx, d)
	s.players.RunPlan(ctx, d)
	ctx.Sync()
}

// =========================== Scene ===========================

type mainScene struct {
	arena *arena
	keys  players.SceneKeys
	tps   *game.TPS
}

// The scene's colours: the player's walkers and giants, the rival's, the hawk.
var (
	playerColor = color.RGBA{R: 230, G: 80, B: 80, A: 255}
	rivalColor  = color.RGBA{R: 70, G: 110, B: 230, A: 255}
	hawkColor   = color.RGBA{R: 120, G: 130, B: 60, A: 255}
)

// picture dresses the units, the hawk and the island and hands the world's picture.
func (m *mainScene) picture() render.Picture {
	s := m.arena
	unitKind := kind.Named[unitRow](s.world.Kinds(), UnitKind)
	plateauKind := kind.Named[unitRow](s.world.Kinds(), PlateauKind)
	rivalKind := kind.Named[unitRow](s.world.Kinds(), RivalKind)
	hawkKind := kind.Named[unitRow](s.world.Kinds(), HawkKind)

	worldAtlas := render.NewAtlas()
	worldAtlas.Add(unitKind, spritePx, render.Solid(playerColor))
	worldAtlas.Add(plateauKind, spritePx, render.Solid(playerColor))
	worldAtlas.Add(rivalKind, spritePx, render.Solid(rivalColor))
	worldAtlas.Add(hawkKind, spritePx, render.Diamond(hawkColor))
	worldAtlas.Close()
	s.world.WithRenderer(worldAtlas)

	s.board.WithRenderer(nil)
	s.board.Res.Render.ShowGridLines = true
	s.nav.WithRenderer(nil)
	s.vision.WithRenderer(nil)
	s.selection.WithRenderer(nil)

	return render.NewComposer(
		s.atmosphere.Renderer(), s.board.Renderer(), s.topography.Renderer(),
		s.world.Renderer(), s.vision.Renderer(), s.selection.Renderer(),
		s.nav.Renderer(), s.atmosphere.Precipitation())
}

// screen is the island through a camera of its own, the player's view, the telemetry and the clock over it.
func (m *mainScene) screen() *ui.Element {
	s := m.arena
	count := func() int { return s.world.Res.Telemetry.Count }
	return ui.Layers( // from the bottom up: each covers those before it
		ui.Image(render.NewFeed(s.cameras.New(s.topography.Views(topography.Isometrically), camera.Config{}), m.picture())).Input(s.players.Through(s.player)),
		ui.Layer(render.NewTelemetryRenderer(&m.tps.Ticks, count).With(s.world.Clock().Reporter(), s.atmosphere.Reporter())),
		ui.Layer(s.world.Clock().HUD()),
	)
}
