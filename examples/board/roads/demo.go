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
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/ground"
	"github.com/kjkrol/gram/plugins/bridges"
	"github.com/kjkrol/gram/plugins/cameras"
	"github.com/kjkrol/gram/plugins/light"
	"github.com/kjkrol/gram/plugins/motion"
	"github.com/kjkrol/gram/plugins/players"
	"github.com/kjkrol/gram/plugins/roads"
	"github.com/kjkrol/gram/plugins/roads/site"
	"github.com/kjkrol/gram/plugins/telemetry"
	"github.com/kjkrol/gram/plugins/topography"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/ui"
)

const (
	TPS          = 60
	GridWidth    = 24
	GridHeight   = 16
	CellSize     = 32
	ScreenWidth  = GridWidth * CellSize
	ScreenHeight = GridHeight * CellSize
	MaxEntCount  = 128
	RoadWidth    = 6
	StartGold    = 500
	GoldPerMetre = 0.25
	GoldPerCubic = 0.05
)

//go:embed map.yaml
var mapFile embed.FS

type arena struct {
	world      *world.Plugin
	motion     *motion.Plugin
	light      *light.Plugin
	board      *board.Plugin
	topography *topography.Plugin
	bridges    *bridges.Plugin
	roads      *roads.Plugin
	cameras    *cameras.Plugin
	players    *players.Plugin

	player   *players.Player
	treasury *treasury
}

func main() {
	props := game.Props{
		Title: "gram board roads",
		Description: "A valley with two villages either side of a river, an old bridge between " +
			"them, a ford below and a hill to the north-west. Shift+R takes a road in hand: a " +
			"left click marks where it begins, every next one a point it runs to by the " +
			"cheapest way the rules allow (a click on its last point lifts that again), and a " +
			"left drag with Shift lays one piece toward a neighbour. The road in hand is green " +
			"where it may lie and yellow where not, the ground it levels outlined, and the " +
			"window says what it takes and what it costs before it is built. The treasury " +
			"pays: once it is empty the builders refuse. I brings the winter, whose frozen " +
			"ground is levelled for no road; a road levelled sets the roadworks going for half " +
			"a minute. Shift+B builds another bridge. Whether the tool levels the ground is set " +
			"on F10.",
		ScreenWidth: ScreenWidth, ScreenHeight: ScreenHeight, Resizable: true,
		TargetTPS: TPS,
	}
	a := &arena{treasury: &treasury{}}
	mainStage := stage.New(BoardRoadsStage).
		World(world.Config{
			Space: world.SpaceCfg{Width: ScreenWidth, Height: ScreenHeight},
			Entities: world.EntitiesCfg{
				MaxCount: MaxEntCount,
				MinSize:  pierWidth,
				MaxSize:  CellSize,
			},
			Heights: true,
		}).
		Plugins(a.makePlugins).
		Keeps(a.treasury).
		Figures(a.defineFigures).
		Players(a.definePlayer).
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
	a.light = light.NewPlugin(a.world, light.Config{})
	grid := grid.DefaultGrids{}.Square(GridWidth, GridHeight, CellSize)
	a.board = board.NewPlugin(grid, &cell.SingleOccupancy{}, a.world).
		WithMotion(a.motion, cell.DefaultHiking).
		WithLog(log.Default())
	a.topography = topography.NewPlugin(a.world, a.board, a.light, topography.Config{})
	a.bridges = bridges.NewPlugin(a.world, a.board, bridges.Config{Rules: bridging}).
		WithShaper(a.topography.Write())
	a.roads = roads.NewPlugin(a.world, a.board, roads.Config{
		Kind:  RoadCell,
		Width: RoadWidth,
		Rules: site.Rules{
			Cut:       6,
			Fill:      6,
			Earth:     2000,
			Spread:    1,
			Slope:     ground.RoadSlope,
			Crossings: true,
			Decks:     true,
			Fords:     true,
			Longest:   60,
			Ends:      true,
			Metre:     1,
			Cubic:     0.5,
			Climb:     4,
		},
		Tariff:    tariff,
		Levelling: true,
	}).WithShaper(a.topography.Write())
	a.cameras = cameras.NewPlugin(a.world, cameras.Config{
		Cell: CellSize,
		Cage: a.topography.Read(0),
	})
	a.players = players.NewPlugin(a.world, a.cameras, a.board, a.topography, a.bridges, a.roads,
		a.light)
}

func tariff(e site.Estimate) float32 {
	return e.Length*GoldPerMetre + (e.Cut+e.Fill)*GoldPerCubic
}

func (a *arena) definePlayer() error {
	a.player = a.players.Local("player")
	return a.player.Bind(a.players.Defaults()...)
}

func (a *arena) defineFigures() {
	a.board.Tiles(tiles()...)
	a.board.Surfaces(surface{})
	a.board.Figures(settlement{})
	a.bridges.Figures(bridge{})
}

func (a *arena) defineEffects() {
	fx := a.world.Write().Effects()
	fx.Define(BrokeEf, effect.Spec{})
	fx.Define(WinterEf, effect.Spec{})
	fx.Define(RoadworksEf, effect.Spec{effect.Lasts(30 * time.Second)})
}

func (a *arena) defineRules() {
	fx := a.world.Write().Effects()
	a.world.Roles().Define(BuilderRole,
		act.Branch[roads.Estimated]{
			Name: "the treasury is empty",
			When: cond.Self.If(fx.Named(BrokeEf)),
			Then: act.Self.Do(roads.Refuse{Why: "the treasury is empty"}),
		},
		act.Branch[roads.Estimated]{
			Name: "frozen ground",
			When: cond.Self.If(roads.Levels()).And(cond.On(a.world).If(fx.Named(WinterEf))),
			Then: act.Self.Do(roads.Refuse{Why: "the ground is frozen"}),
		},
		act.Branch[roads.Built]{
			Name: "roadworks",
			When: cond.Self.If(roads.Levelled()),
			Then: act.Self.Do(ceremony.Put(fx.Named(RoadworksEf)).On(ceremony.World)),
		},
	)
	a.roads.Plays(a.world.Roles().Named(BuilderRole))
}

func (a *arena) defineCommands() {
	winter := ceremony.Toggle(a.world.Write().Effects().Named(WinterEf)).On(ceremony.World)
	a.world.Commands().Define(WinterCmd, winter)
}

func (a *arena) bindKeys() error {
	return a.player.Bind(control.Give(
		control.KeyPress{Key: control.KeyI},
		"Winter, or the thaw",
		a.world.Commands().Named(WinterCmd),
	))
}

func (a *arena) spawn() error {
	a.treasury.Gold = StartGold
	level, err := a.board.Lay(mapFile, "map.yaml")
	if err != nil {
		return err
	}
	a.topography.Write().Seed(level.Heights())
	if err := a.bridges.Write().Spawn(level); err != nil {
		return err
	}
	return board.Spawn[settlementRow](level, nil)
}

func (a *arena) defineScenes() []game.Scene {
	main := &mainScene{arena: a}
	return []game.Scene{ui.NewScene(MainScene, main.screen()).
		Input(a.players.Handle).
		Issue(a.players.IssueAs(a.player))}
}

func (a *arena) update(ctx goke.RunCtx, d time.Duration) {
	a.pay()
	a.bridges.RunPlan(ctx, d)
	a.world.RunPlan(ctx, d)
	a.light.RunPlan(ctx, d)
	a.motion.RunPlan(ctx, d)
	a.board.RunPlan(ctx, d)
	a.topography.RunPlan(ctx, d)
	a.roads.RunPlan(ctx, d)
	a.cameras.RunPlan(ctx, d)
	a.players.RunPlan(ctx, d)
	ctx.Sync()
}

// BrokeEf follows the treasury every tick rather than once as it runs dry: a game saved before
// the effect landed loads with an empty treasury and without it
func (a *arena) pay() {
	for _, r := range a.roads.Read(0).Built() {
		if r.Issuer == a.player.ID {
			a.treasury.Gold -= r.Price
		}
	}
	// no step carries the command under the pause: given every tick, it would pile up meanwhile
	if a.world.Read(0).Clock().State().Paused {
		return
	}
	broke := a.world.Write().Effects().Named(BrokeEf)
	empty := a.treasury.Gold <= 0
	if empty == a.world.Read(0).Effects().Has(a.roads.Entity(), broke) {
		return
	}
	cmd := ceremony.Take(broke)
	if empty {
		cmd = ceremony.Put(broke)
	}
	a.world.Write().Carrier().Put(a.player.ID, cmd.On(ceremony.Named(a.roads.Name())))
}
