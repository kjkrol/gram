package main

import (
	"embed"
	"log"
	"time"

	"github.com/kjkrol/ceremony"
	"github.com/kjkrol/ceremony/act"
	"github.com/kjkrol/ceremony/cond"
	"github.com/kjkrol/ceremony/effect"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/game/stage"
	"github.com/kjkrol/gram/plugins/atmosphere"
	"github.com/kjkrol/gram/plugins/atmosphere/calendar"
	"github.com/kjkrol/gram/plugins/atmosphere/climate"
	"github.com/kjkrol/gram/plugins/atmosphere/climate/weather"
	"github.com/kjkrol/gram/plugins/atmosphere/sky"
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
	player     *players.Player
	rival      *players.Player
	vision     *vision.Plugin
	atmosphere *atmosphere.Plugin
}

func main() {
	props := game.Props{
		Title: "gram board topography",
		Description: "Madeira in relief. Select a unit, C follows it, V rides it: inside, the " +
			"mouse looks round, W and S walk, A and D strafe, Shift sprints, Space jumps.",
		ScreenWidth: ScreenWidth, ScreenHeight: ScreenHeight, Resizable: true,
		TargetTPS: TPS,
	}
	a := &arena{}
	mainStage := stage.New(BoardTopographyStage).
		World(world.Config{
			Scale: scale,
			Space: world.SpaceCfg{Width: WorldWidth, Height: WorldHeight},
			Entities: world.EntitiesCfg{
				MaxCount: MaxEntCount,
				MinSize:  2,
				MaxSize:  CellSize,
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
	grid := grid.DefaultGrids{}.Square(GridWidth, GridHeight, CellSize)
	a.collision = collision.NewPlugin(a.world)
	a.board = board.NewPlugin(grid, &cell.SingleOccupancy{}, a.world).
		WithCollision(a.collision).
		WithLog(log.Default())
	a.gravity = gravity.NewPlugin(a.world, gravity.Config{}).WithSurfaces(a.board.Surfaces)
	a.topography = topography.NewPlugin(a.world, a.board, topography.Config{
		Cell:        CellSize,
		HeightUnit:  1,
		Perspective: true,
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
	}}).WithShaper(a.topography)
	a.selection = selection.NewPlugin(a.world)
	a.driving = driving.NewPlugin(a.world, a.selection).WithGround(a.board)
	a.nav = navigation.NewPlugin(a.board, a.world, a.selection, a.driving).
		WithCollision(a.collision).
		WithSpacing(navigation.BodySpacing)
	a.vision = vision.NewPlugin(a.world).WithBoard(a.board).WithFrequency(15)
	a.atmosphere = atmosphere.NewPlugin(a.world, atmosphere.Config{
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
	a.atmosphere.WithWeathering(a.board, a.defineClimate())
	a.cameras = cameras.NewPlugin(a.world)
	a.telemetry = telemetry.NewPlugin(a.world).With(a.atmosphere.Reporter())
	a.players = players.NewPlugin(a.world, a.collision, a.cameras, a.board, a.selection,
		a.nav, a.driving,
		a.atmosphere, a.topography, a.vision, a.bridges)
}

func (a *arena) definePlayers() error {
	a.player = a.players.Local("player")
	a.rival = a.players.Add("rival")
	return a.player.Bind(a.players.Defaults()...)
}

func (a *arena) defineEffects() {
	night := a.atmosphere.Calendar().Config().DayRealTime / 2
	a.world.Effects().Define(BloodMoonEf, effect.Spec{effect.Lasts(night),
		effect.Alter(func(m *sky.Moon) {
			m.Color, m.Face = render.Light{1, 0.25, 0.2}, render.Light{1, 0.3, 0.25}
			m.Strength *= 2
		})})
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
		Then: act.Self.Put(a.world.Effects().Named(BloodMoonEf)),
	})
	a.atmosphere.Plays(a.world.Roles().Named(LunarRole))
}

func (a *arena) defineCommands() {
	bleed := ceremony.Toggle(a.world.Effects().Named(BloodMoonEf)).On(a.atmosphere)
	a.world.Commands().Define(BleedCmd, bleed)
}

type unitRow struct {
	Start  cell.ID `yaml:"start"`
	Target cell.ID `yaml:"target"`
}

func (a *arena) defineFigures() {
	a.board.Tiles(tiles()...)
	a.board.Figures(
		walker{kind: UnitKind, color: playerColor, ordered: true},
		walker{kind: PlateauKind, color: playerColor},
		walker{kind: RivalKind, color: rivalColor, ordered: true},
		hawk{},
		ford{},
	)
	a.bridges.Figures(bridge{})
}

func (a *arena) bindKeys() error {
	return a.player.Bind(control.Give(
		control.KeyPress{Key: control.KeyM},
		"Blood moon, on or off",
		a.world.Commands().Named(BleedCmd),
	))
}

func (a *arena) spawn() error {
	level, err := a.board.Lay(mapFile, "map.yaml")
	if err != nil {
		return err
	}
	a.topography.Seed(level.Heights())
	if err := board.Spawn[fordRow](level, nil); err != nil {
		return err
	}
	if err := a.bridges.Spawn(level); err != nil {
		return err
	}
	return board.Spawn[unitRow](level, board.Sides{
		PlayerSide:  {players.Give{To: a.player.ID}, selection.Allow{Selected: true}},
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
	a.players.RunPlan(ctx, d)
	ctx.Sync()
}
