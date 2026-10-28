package main

import (
	"embed"
	"flag"
	"log"
	"time"

	"github.com/kjkrol/ceremony"
	"github.com/kjkrol/ceremony/act"
	"github.com/kjkrol/ceremony/cond"
	"github.com/kjkrol/ceremony/effect"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/game/stage"
	"github.com/kjkrol/gram/plugins/atmosphere"
	"github.com/kjkrol/gram/plugins/atmosphere/calendar"
	"github.com/kjkrol/gram/plugins/atmosphere/climate"
	"github.com/kjkrol/gram/plugins/atmosphere/climate/weather"
	"github.com/kjkrol/gram/plugins/atmosphere/sky"
	"github.com/kjkrol/gram/plugins/atmosphere/weathering"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/unit"
	"github.com/kjkrol/gram/plugins/bridges"
	"github.com/kjkrol/gram/plugins/bridges/site"
	"github.com/kjkrol/gram/plugins/cameras"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/driving"
	"github.com/kjkrol/gram/plugins/gravity"
	"github.com/kjkrol/gram/plugins/light"
	"github.com/kjkrol/gram/plugins/minimap"
	"github.com/kjkrol/gram/plugins/motion"
	"github.com/kjkrol/gram/plugins/navigation"
	"github.com/kjkrol/gram/plugins/players"
	"github.com/kjkrol/gram/plugins/selection"
	"github.com/kjkrol/gram/plugins/telemetry"
	"github.com/kjkrol/gram/plugins/topography"
	"github.com/kjkrol/gram/plugins/vision"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/sector"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/ui"
)

const (
	TPS          = 60
	GridWidth    = 204
	GridHeight   = 106
	CellSize     = 32
	WorldWidth   = GridWidth * CellSize
	WorldHeight  = GridHeight * CellSize
	ScreenWidth  = 1024
	ScreenHeight = 768
	MaxEntCount  = 320
)

//go:embed map.yaml
var mapFile embed.FS

// A cell is 333 m of Madeira; the figures on it are drawn 50 times bigger than life.
var scale = world.Scale{Metres: 333.0 / CellSize, Figures: 50}

// figure is how tall a walker stands as drawn: what passes under a bridge.
var figure = scale.Figure(1.8)

type arena struct {
	world      *world.Plugin
	motion     *motion.Plugin
	board      *board.Plugin
	topography *topography.Plugin
	bridges    *bridges.Plugin
	nav        *navigation.Plugin
	driving    *driving.Plugin
	collision  *collision.Plugin
	gravity    *gravity.Plugin
	selection  *selection.Plugin
	players    *players.Plugin
	telemetry  *telemetry.Plugin
	cameras    *cameras.Plugin
	minimap    *minimap.Plugin
	light      *light.Plugin
	player     *players.Player
	rival      *players.Player
	vision     *vision.Plugin
	atmosphere *atmosphere.Plugin
	look       string
	hud        string
}

func main() {
	look := flag.String("look", NaturalLook,
		"the ground's look: natural (photographs) or ghibli (painted, the light in bands)")
	hud := flag.String("hud", BarHUD,
		"the hour, the date and the weather: bar (one, at the top) or widgets (by the corners)")
	flag.Parse()
	if *hud != BarHUD && *hud != WidgetsHUD {
		log.Fatalf("no HUD is named %q: bar or widgets", *hud)
	}
	props := game.Props{
		Title: "gram board topography",
		Description: "Madeira in relief, its ground textured as -look says (natural or ghibli), " +
			"the hour, the date and the weather shown as -hud says (bar: one at the top; " +
			"widgets: apart, by the corners). Select a unit: V follows it from behind in the " +
			"perspective (W, S, A and D walk it the way the screen lies, the mouse turns the " +
			"camera round it and tilts it, = and - draw it in and out), again rides inside " +
			"(the mouse looks round, A and D strafe); Shift sprints, Space jumps; C centres " +
			"over it, back in the view it was in. " +
			"The minimap turns with the camera; a click on it takes the camera there.",
		ScreenWidth: ScreenWidth, ScreenHeight: ScreenHeight, Resizable: true,
		TargetTPS: TPS,
	}
	a := &arena{look: *look, hud: *hud}
	mainStage := stage.New(BoardTopographyStage).
		World(world.Config{
			Scale: scale,
			Space: world.SpaceCfg{Width: WorldWidth, Height: WorldHeight},
			Entities: world.EntitiesCfg{
				MaxCount: MaxEntCount,
				MinSize:  2,
				MaxSize:  CellSize,
			},
			Sectors: world.SectorsCfg{
				Buckets: 2,
				Bands: []sector.Band{
					{Reach: scale.Units(16_000)},
					{Reach: scale.Units(32_000), Pace: 4},
				},
			},
			Heights: true,
		}).
		Plugins(a.makePlugins).
		Figures(a.defineFigures).
		Players(a.definePlayers).
		Effects(a.defineEffects).
		Rules(a.defineRules).
		Commands(a.defineCommands).
		Controls(a.bindKeys).
		Spawn(a.spawn).
		Scenes(a.defineScenes).
		Shows(MainScene, telemetry.SceneName).
		Update(a.update)
	gram.Run(game.New(props, mainStage))
}

func (a *arena) makePlugins(w *world.Plugin) {
	a.world = w
	a.motion = motion.NewPlugin(a.world)
	grid := grid.DefaultGrids{}.Square(GridWidth, GridHeight, CellSize)
	a.collision = collision.NewPlugin(a.world, a.motion)
	a.light = light.NewPlugin(a.world, light.Config{LampShadows: 2})
	a.board = board.NewPlugin(grid, &cell.SingleOccupancy{}, a.world).
		WithMotion(a.motion, madeira{cell.DefaultHiking}).
		WithCollision(a.collision).
		WithLog(log.Default())
	a.gravity = gravity.NewPlugin(a.world, a.motion, gravity.Config{}).WithSurfaces(a.board.Read(0).Footing)
	a.topography = topography.NewPlugin(a.world, a.board, a.light, topography.Config{
		Shaping: topography.Shaping{
			Step:    scale.Units(40),
			MaxStep: scale.Units(160),
		}})
	a.bridges = bridges.NewPlugin(a.world, a.board, bridges.Config{Rules: site.Rules{
		Longest:   3,
		Under:     figure + deckThickness + scale.Units(1),
		Clearance: deckThickness + scale.Units(1),
		Reshape:   scale.Units(200),
		High:      scale.Units(200),
	}}).WithShaper(a.topography.Write())
	a.selection = selection.NewPlugin(a.world)
	a.driving = driving.NewPlugin(a.world, a.motion, a.selection).WithGround(a.board)
	a.nav = navigation.NewPlugin(a.board, a.world, a.motion, a.selection, a.driving).
		WithCollision(a.collision).
		WithSpacing(navigation.BodySpacing)
	a.vision = vision.NewPlugin(a.world).WithBoard(a.board).WithFrequency(15)
	a.atmosphere = atmosphere.NewPlugin(a.world, a.light, atmosphere.Config{
		Calendar: calendar.Config{
			Start:  12 * time.Hour,
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
	a.atmosphere.WithWeathering(a.board, a.defineClimate())
	a.cameras = cameras.NewPlugin(a.world, cameras.Config{
		Turn:        true,
		Tilt:        true,
		Perspective: true,
		Grab:        true,
		Cell:        CellSize,
		HeightUnit:  1,
		Cage:        a.topography.Read(0),
		Levers:      []cameras.Lever{a.topography.Write(), a.light.Write(), a.atmosphere.Write()},
		Detail: camera.Detail{
			Sight:  scale.Units(30_000),
			Fog:    scale.Units(6_000),
			Coarse: 0.25,
		},
		Tactical: cameras.Knobs{Strategy: cameras.InLayers, Layers: true},
		Riding:   cameras.Knobs{Strategy: cameras.InLayers, Layers: true},
		Arm:      cameras.Arm{Pitch: 20, Distance: 3, Aim: 1, Tilts: true, Zooms: true},
	})
	a.minimap = minimap.NewPlugin(a.world, a.topography.Write(), minimap.Config{
		Reach:   scale.Units(12_000),
		Turning: true,
	})
	a.telemetry = telemetry.NewPlugin(a.world).
		With(a.atmosphere.Read(0).Reporter(), a.light.Read(0).Reporter(), a.topography.Read(0).Reporter(),
			a.cameras.Read(0).Reporter(), a.world.Read(0).Sectors(), a.world.Read(0).Seen())
	a.players = players.NewPlugin(a.world, a.collision, a.cameras, a.board, a.selection,
		a.nav, a.driving,
		a.atmosphere, a.topography, a.vision, a.bridges, a.minimap, a.light)
	a.topography.Write().AdaptCare(true)
	a.light.Write().AdaptShadows(true)
	a.atmosphere.Write().ShowClouds(atmosphere.AllBodies)
	a.atmosphere.CastClouds(atmosphere.CloudShadows{Ground: true, Figures: true, High: true})
	a.cameras.Write().ShowZones(true)
}

func (a *arena) definePlayers() error {
	a.player = a.players.Local("player")
	a.rival = a.players.Add("rival")
	return a.player.Bind(a.players.Defaults()...)
}

func (a *arena) defineEffects() {
	night := a.atmosphere.Read(0).Calendar().Config().DayRealTime / 2
	a.world.Write().Effects().Define(BloodMoonEf, effect.Spec{effect.Lasts(night),
		effect.Alter(func(m *sky.Moon) {
			m.Color, m.Face = render.Light{1, 0.25, 0.2}, render.Light{1, 0.3, 0.25}
			m.Strength *= 2
		})})
	a.world.Write().Effects().Define(WindwardRainEf, effect.Spec{effect.Lasts(2 * time.Second),
		effect.Alter(func(w *weathering.Wring) { w.Falls = 0.8 })})
	a.world.Write().Effects().Define(GatheringEf, effect.Spec{effect.Lasts(2 * time.Second),
		effect.Alter(func(w *weathering.Wring) { w.Gathers = 0.05 })})
	a.world.Write().Effects().Define(SilhouetteEf, effect.Spec{
		effect.Alter(func(s *light.Shadow) { s.Cast = light.Silhouette })})
}

func (a *arena) defineRules() {
	a.world.Roles().Define(MortalRole, act.Branch[unit.Standing]{
		Name: "drown",
		When: cond.Self.If(unit.Fallen()),
		Then: act.Self.Do(world.Despawn{}),
	})
	a.world.Roles().Define(LunarRole, act.Branch[sky.Moonrise]{
		Name: "a blood moon rises",
		When: cond.Self.If(sky.FullMoon()),
		Then: act.Self.Put(a.world.Write().Effects().Named(BloodMoonEf)),
	})
	a.atmosphere.Plays(a.world.Roles().Named(LunarRole))
	a.world.Roles().Define(UplandRole,
		act.Branch[weathering.Overhead]{
			Name: "rain on the windward slopes",
			When: cond.Self.If(weathering.Reaches(0.9)).And(cond.Self.If(weathering.Clouded(0.6))),
			Then: act.Self.Put(a.world.Write().Effects().Named(WindwardRainEf)),
		},
		act.Branch[weathering.Overhead]{
			Name: "clouds over the hills of an afternoon",
			When: cond.Self.If(weathering.Hours(12*time.Hour, 17*time.Hour)).
				And(cond.Self.If(weathering.Damp(0.3))).
				And(cond.Self.If(weathering.Reaches(0.7))),
			Then: act.Self.Put(a.world.Write().Effects().Named(GatheringEf)),
		})
}

func (a *arena) defineCommands() {
	bleed := ceremony.Toggle(a.world.Write().Effects().Named(BloodMoonEf)).On(a.atmosphere)
	a.world.Commands().Define(BleedCmd, bleed)
	outline := ceremony.Toggle(a.world.Write().Effects().Named(SilhouetteEf)).
		On(a.selection.Selected())
	a.world.Commands().Define(SilhouetteCmd, outline)
}

type unitRow struct {
	Start  cell.ID `yaml:"start"`
	Target cell.ID `yaml:"target"`
}

func (a *arena) defineFigures() {
	a.board.Tiles(tiles()...)
	a.board.Surfaces(surface{})
	a.board.Figures(
		walker{kind: UnitKind, color: playerColor, ordered: true},
		walker{kind: PlateauKind, color: playerColor, highland: true},
		walker{kind: RivalKind, color: rivalColor, ordered: true},
		hawk{},
		ford{},
		lantern{},
	)
	a.bridges.Figures(bridge{})
	a.atmosphere.Skies(earth{})
}

func (a *arena) bindKeys() error {
	return a.player.Bind(
		control.Give(
			control.KeyPress{Key: control.KeyM},
			"Blood moon, on or off",
			a.world.Commands().Named(BleedCmd),
		),
		control.Give(
			control.KeyPress{Key: control.KeyG},
			"The selected cast their silhouettes, or patches again",
			a.world.Commands().Named(SilhouetteCmd),
		),
	)
}

func (a *arena) spawn() error {
	level, err := a.board.Lay(mapFile, "map.yaml")
	if err != nil {
		return err
	}
	a.topography.Write().Seed(level.Heights())
	if err := board.Spawn[fordRow](level, nil); err != nil {
		return err
	}
	if err := board.Spawn[lanternRow](level, nil); err != nil {
		return err
	}
	if err := a.bridges.Write().Spawn(level); err != nil {
		return err
	}
	return board.Spawn[unitRow](level, board.Sides{
		PlayerSide:  {players.Give{To: a.player.ID}, selection.Allow{}},
		RivalSide:   {players.Give{To: a.rival.ID}, selection.Allow{}},
		ReserveSide: {players.Give{To: a.player.ID}, selection.Allow{}},
	})
}

func (a *arena) defineScenes(ctx game.Initializer) []game.Scene {
	main := &mainScene{arena: a}
	return []game.Scene{ui.NewScene(MainScene, main.screen()).
		Input(a.players.Handle).
		Issue(a.players.IssueAs(a.player))}
}

func (a *arena) update(ctx goke.RunCtx, d time.Duration) {
	a.bridges.RunPlan(ctx, d)
	a.world.RunPlan(ctx, d)
	a.light.RunPlan(ctx, d)
	a.motion.RunPlan(ctx, d)
	a.collision.RunPlan(ctx, d)
	a.gravity.RunPlan(ctx, d)
	a.board.RunPlan(ctx, d)
	a.nav.RunPlan(ctx, d)
	a.driving.RunPlan(ctx, d)
	a.vision.RunPlan(ctx, d)
	a.atmosphere.RunPlan(ctx, d)
	a.selection.RunPlan(ctx, d)
	a.topography.RunPlan(ctx, d)
	a.cameras.RunPlan(ctx, d)
	a.minimap.RunPlan(ctx, d)
	a.players.RunPlan(ctx, d)
	ctx.Sync()
}
