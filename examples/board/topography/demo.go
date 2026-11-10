package main

import (
	"embed"
	"flag"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/ceremony"
	"github.com/kjkrol/ceremony/act"
	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/ceremony/cond"
	"github.com/kjkrol/ceremony/effect"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/game/stage"
	"github.com/kjkrol/gram/lod"
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
	"github.com/kjkrol/gram/plugins/forest"
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
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/ui"
)

const (
	TPS          = 60
	GridWidth    = 640
	GridHeight   = 320
	CellSize     = 32
	WorldWidth   = GridWidth * CellSize
	WorldHeight  = GridHeight * CellSize
	ScreenWidth  = 1024
	ScreenHeight = 768
	// the trees and houses unfolded round the camera and what walks among them
	MaxEntCount = 40_000

	// a sector's side in cells, gramap's fields' grain: Buckets of the space's buckets, which
	// MaxEntCount makes 64 world units a side
	sectorCells = 16
	Buckets     = sectorCells * CellSize / 64
	// seconds a sector lies at L4 before the trees, the houses and the caravans' men fold
	FoldAfter = 3
	// the side of a field's own box at the middle of its rectangle
	fieldPx = 16
	// the zoom the game starts at, raised by the camera to the one fitting the world in the window,
	// 0.18 in 1024 by 768, 0.28 in 1920 by 1080 and 0.38 in 2560 by 1440: L4 in a window up to
	// about 2700 pixels wide at 16:9, the forests and the settlements in the sheet, the caravans
	// and the travellers far off
	StartZoom = 0.15
)

//go:embed map.yaml
var mapFile embed.FS

// A cell is 100 m of Madeira; the figures on it are drawn 15 times bigger than life.
var scale = world.Scale{Metres: 100.0 / CellSize, Figures: 15}

// figure is how tall a walker stands as drawn: what passes under a bridge.
var figure = scale.Figure(1.8)

type arena struct {
	world      *world.Plugin
	motion     *motion.Plugin
	collision  *collision.Plugin
	light      *light.Plugin
	board      *board.Plugin
	gravity    *gravity.Plugin
	topography *topography.Plugin
	bridges    *bridges.Plugin
	forest     *forest.Plugin
	selection  *selection.Plugin
	driving    *driving.Plugin
	nav        *navigation.Plugin
	vision     *vision.Plugin
	atmosphere *atmosphere.Plugin
	cameras    *cameras.Plugin
	minimap    *minimap.Plugin
	telemetry  *telemetry.Plugin
	players    *players.Plugin

	player *players.Player
	rival  *players.Player
	look   string
	hud    string
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
			"widgets: apart, by the corners). Its forests and settlements are photographed " +
			"into a sheet from afar, kept between runs, and plant their trees and stand their " +
			"houses as the camera comes near; caravans, a wagon and three porters, and " +
			"travellers go to and fro along the roads, far off an icon of their kind a sector " +
			"with their number over it (F10: points, the numbers off); a band of thirty " +
			"walkers stands on the plateau, one icon far off, and a lookout at its eastern " +
			"edge faces the peaks. Select a unit: V " +
			"follows it from behind in the perspective (W, S, A and D walk it the way the " +
			"screen lies, the mouse turns the camera round it and tilts it, = and - draw it " +
			"in and out), again rides inside " +
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
			Sectors: world.SectorsCfg{Buckets: Buckets},
			LOD: lod.Ladder{
				// L2 from a tree of 12 pixels, L3 down to 8, L4, the sheet, under that
				Scales: [lod.Levels - 1]float32{0, 0, 0.6, 0.4, 0.12, 0},
				Sight:  scale.Units(30_000),
				Fog:    scale.Units(6_000),
				Fold:   FoldAfter,
			},
			Heights:  true,
			Far:      world.Banners,
			Counts:   true,
			Canopies: true,
			Sheets:   sheets(),
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

// sheets is where the sheets of what stands are kept between runs, in the user's cache: none
// where there is no cache
func sheets() world.SheetsCfg {
	cfg := world.SheetsCfg{Keep: true, Version: SheetsVersion}
	if dir, err := os.UserCacheDir(); err == nil {
		cfg.Dir = filepath.Join(dir, "gram", "topography", "sheets")
	}
	return cfg
}

// routes is where the hierarchies the long routes are climbed through are kept between runs, in
// the user's cache: none where there is no cache
func routes() navigation.Cache {
	cfg := navigation.Cache{Keep: true, Version: RoutesVersion}
	if dir, err := os.UserCacheDir(); err == nil {
		cfg.Dir = filepath.Join(dir, "gram", "topography", "hierarchy")
	}
	return cfg
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
	a.gravity = gravity.NewPlugin(a.world, a.motion, gravity.Config{}).
		WithSurfaces(a.board.Read(0).Footing)
	a.topography = topography.NewPlugin(a.world, a.board, a.light, topography.Config{
		Shaping: topography.Shaping{
			Step:    scale.Units(12),
			MaxStep: scale.Units(48),
		}})
	a.bridges = bridges.NewPlugin(a.world, a.board, bridges.Config{Rules: site.Rules{
		Longest:   4,
		Under:     figure + deckThickness + scale.Units(1),
		Clearance: deckThickness + scale.Units(1),
		Reshape:   scale.Units(60),
		High:      scale.Units(200),
	}}).WithShaper(a.topography.Write())
	a.forest = forest.NewPlugin(a.world, a.board)
	a.selection = selection.NewPlugin(a.world)
	a.driving = driving.NewPlugin(a.world, a.motion, a.selection).WithGround(a.board)
	a.nav = navigation.NewPlugin(a.board, a.world, a.motion, a.selection, a.driving).
		WithCollision(a.collision).
		WithSpacing(navigation.BodySpacing).
		WithHierarchy(navigation.Hierarchy{From: 32}).
		WithCache(routes())
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
		Levers:      []cameras.Lever{a.light.Write(), a.atmosphere.Write()},
		Tactical:    cameras.Knobs{Strategy: cameras.InLayers, Layers: true},
		Riding:      cameras.Knobs{Strategy: cameras.InLayers, Layers: true},
		Arm:         cameras.Arm{Pitch: 20, Distance: 3, Aim: 1, Tilts: true, Zooms: true},
	})
	a.minimap = minimap.NewPlugin(a.world, a.topography.Write(), minimap.Config{
		Reach:   scale.Units(12_000),
		Turning: true,
	})
	a.telemetry = telemetry.NewPlugin(a.world).
		With(a.atmosphere.Read(0).Reporter(), a.light.Read(0).Reporter(), a.topography.Read(0).Reporter(),
			a.cameras.Read(0).Reporter(), a.nav.Read(0).Reporter(), a.world.Read(0).Sectors(),
			a.world.Read(0).Seen())
	a.players = players.NewPlugin(a.world, a.collision, a.cameras, a.board, a.selection,
		a.nav, a.driving,
		a.atmosphere, a.topography, a.vision, a.bridges, a.minimap, a.light)
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
	a.world.Roles().Define(CarrierRole, act.Branch[world.Folding]{
		Name: "fold in",
		When: cond.Self.If(cond.Always),
		Then: act.Self.Do(world.Despawn{}),
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

// fieldRow is a field of the map over the cells of a kind: the rectangle round them from cell
// From to cell To, and how many of them it holds.
type fieldRow struct {
	From  cell.ID `yaml:"from"`
	To    cell.ID `yaml:"to"`
	Cells int     `yaml:"cells"`
}

// overField is what a field aggregate of a fieldRow is: its own box at the middle of the
// rectangle, the rectangle its Field
func overField(b *board.Plugin) []comp.Comp {
	corners := func(r fieldRow) (geom.Vec, geom.Vec) {
		bd := b.Read(0).Board()
		return bd.CellCenter(r.From), bd.CellCenter(r.To)
	}
	return []comp.Comp{
		comp.Load(func(r fieldRow) world.Position {
			from, to := corners(r)
			corner := geom.NewVec((from.X+to.X-fieldPx)/2, (from.Y+to.Y-fieldPx)/2)
			return world.Position{AABB: plane.NewAABB(corner, fieldPx, fieldPx)}
		}),
		comp.Load(func(r fieldRow) world.Field {
			from, to := corners(r)
			return world.Field{Size: geom.NewVec(to.X-from.X+CellSize, to.Y-from.Y+CellSize)}
		}),
		comp.Const(world.Aggregate{}),
	}
}

// unseen is the look of what is drawn for it by its parts
func unseen(*render.Canvas, int) {}

// carried is what the wagon and the porters of a caravan are told as they are made: whose they
// are; the caravan alone is picked, through any of them
func (a *arena) carried() []command.Command {
	return []command.Command{players.Give{To: a.player.ID}}
}

// enlisted is what a band's walkers are told as they are made: whose they are, and that each is
// picked itself where it stands
func (a *arena) enlisted() []command.Command {
	return []command.Command{players.Give{To: a.player.ID}, selection.Allow{}}
}

func (a *arena) defineFigures() {
	a.board.Tiles(tiles()...)
	a.board.Surfaces(surface{})
	a.board.Figures(
		walker{kind: UnitKind, color: playerColor, ordered: true},
		walker{kind: PlateauKind, color: playerColor, highland: true, member: MemberRole},
		walker{kind: LookoutKind, color: playerColor, highland: true},
		walker{
			kind:    RivalKind,
			color:   rivalColor,
			ordered: true,
			banner:  world.BannerStyle{Shape: render.BadgeDiamond},
		},
		hawk{},
		ford{},
		lantern{},
		house{},
		lamp{},
		settlement{},
		wagon{},
		porter{color: playerColor},
		caravan{color: playerColor, carried: a.carried},
		traveller{color: playerColor},
		band{color: playerColor, joined: a.enlisted},
	)
	a.bridges.Figures(bridge{})
	a.forest.Figures(woods{})
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
	if err := board.Spawn[fieldRow](level, nil); err != nil {
		return err
	}
	if err := a.bridges.Write().Spawn(level); err != nil {
		return err
	}
	caravans := board.Sides{ReserveSide: append(a.carried(), selection.Allow{})}
	if err := board.Spawn[caravanRow](level, caravans); err != nil {
		return err
	}
	sides := board.Sides{
		PlayerSide:  a.enlisted(),
		RivalSide:   {players.Give{To: a.rival.ID}, selection.Allow{}},
		ReserveSide: a.enlisted(),
	}
	if err := board.Spawn[travellerRow](level, sides); err != nil {
		return err
	}
	if err := board.Spawn[bandRow](level, sides); err != nil {
		return err
	}
	return board.Spawn[unitRow](level, sides)
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
