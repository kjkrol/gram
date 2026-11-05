package main

import (
	"embed"
	"log"
	"time"

	"github.com/kjkrol/ceremony"
	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/ceremony/effect"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/game/stage"
	"github.com/kjkrol/gram/lod"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/cameras"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/driving"
	"github.com/kjkrol/gram/plugins/minimap"
	"github.com/kjkrol/gram/plugins/motion"
	"github.com/kjkrol/gram/plugins/motion/steering"
	"github.com/kjkrol/gram/plugins/navigation"
	"github.com/kjkrol/gram/plugins/players"
	"github.com/kjkrol/gram/plugins/selection"
	"github.com/kjkrol/gram/plugins/telemetry"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/ui"
)

const (
	TPS          = 60
	GridWidth    = 192
	GridHeight   = 128
	CellSize     = 32
	WorldWidth   = GridWidth * CellSize
	WorldHeight  = GridHeight * CellSize
	ScreenWidth  = 1280
	ScreenHeight = 800
	MaxEntCount  = 1024

	// how far round the camera the world lives: the whole board from its middle
	Sight = 5000
	// how far round the camera the minimap shows: half the board's shorter side, the circle full
	// of board wherever the camera is
	MinimapReach = 2048
	// the zoom the game starts at, L2: the forest a sheet, the armies points
	StartZoom = 0.7
	// seconds a sector lies at L2 before what stands for its parts folds
	FoldAfter = 2

	marchSpeed = CellSize * 1.5
)

var marching = steering.Steering{
	MaxSpeed: marchSpeed,
	Accel:    marchSpeed * 2,
	Brake:    marchSpeed * 4,
	V0:       marchSpeed / 2,
	TurnRate: 0.15,
}

//go:embed map.yaml
var mapFile embed.FS

type arena struct {
	world     *world.Plugin
	motion    *motion.Plugin
	collision *collision.Plugin
	board     *board.Plugin
	selection *selection.Plugin
	driving   *driving.Plugin
	nav       *navigation.Plugin
	cameras   *cameras.Plugin
	minimap   *minimap.Plugin
	telemetry *telemetry.Plugin
	players   *players.Plugin

	player *players.Player
}

func main() {
	props := game.Props{
		Title: "gram forest and armies",
		Description: "A forest and two armies, each one thing seen from afar and its parts " +
			"seen near. Zoomed out the forest is a sheet of crowns and each army a point " +
			"marching from region to region; zoomed in the forest's sectors plant their trees " +
			"and each army stands its soldiers round it, who take its march over, folding them " +
			"back in where they stand two seconds after the camera draws away. Soldiers fell " +
			"the trees they strike, and the forest grows back sparser; its crowns are " +
			"see-through, the soldiers under them showing. Select an army or its " +
			"soldiers and send them by a right click: two armies meeting far off stop and " +
			"fight blow by blow; H has the army selected charge the other, which unfolds " +
			"however far, and their soldiers fight it out, as they do when the camera comes " +
			"to a fight far off. = and - zoom, " +
			"Ctrl+= and Ctrl+- the sight, F10 the sectors by their level, ~ the telemetry; a " +
			"click on the minimap takes the camera there.",
		ScreenWidth:  ScreenWidth,
		ScreenHeight: ScreenHeight,
		Resizable:    true,
		TargetTPS:    TPS,
	}
	a := &arena{}
	mainStage := stage.New(ForestArmyStage).
		World(world.Config{
			Space: world.SpaceCfg{Width: WorldWidth, Height: WorldHeight},
			Entities: world.EntitiesCfg{
				MaxCount: MaxEntCount,
				MinSize:  soldierPx,
				MaxSize:  armyPx,
			},
			Sectors: world.SectorsCfg{Buckets: 2},
			LOD: lod.Ladder{
				Scales: [lod.Levels - 1]float32{2, 1, 0.5, 0},
				Sight:  Sight,
				Fold:   FoldAfter,
			},
			Heights: true,
		}).
		Plugins(a.makePlugins).
		Figures(a.defineFigures).
		Players(a.definePlayer).
		Effects(a.defineEffects).
		Rules(a.defineRules).
		Commands(a.defineCommands).
		Controls(a.bindKeys).
		Spawn(a.spawn).
		Scenes(a.defineScenes).
		Update(a.update)
	gram.Run(game.New(props, mainStage))
}

func (a *arena) makePlugins(w *world.Plugin) {
	a.world = w
	a.motion = motion.NewPlugin(a.world)
	a.collision = collision.NewPlugin(a.world, a.motion)
	grid := grid.DefaultGrids{}.Square(GridWidth, GridHeight, CellSize)
	a.board = board.NewPlugin(grid, &cell.SingleOccupancy{}, a.world).
		WithMotion(a.motion, cell.DefaultHiking).
		WithCollision(a.collision).
		WithLog(log.Default())
	a.selection = selection.NewPlugin(a.world)
	a.driving = driving.NewPlugin(a.world, a.motion, a.selection).WithGround(a.board)
	a.nav = navigation.NewPlugin(a.board, a.world, a.motion, a.selection, a.driving).
		WithCollision(a.collision)
	a.cameras = cameras.NewPlugin(a.world, cameras.Config{})
	a.minimap = minimap.NewPlugin(a.world, a.board.Write(), minimap.Config{Reach: MinimapReach})
	a.telemetry = telemetry.NewPlugin(a.world)
	a.players = players.NewPlugin(a.world, a.collision, a.cameras, a.board, a.selection,
		a.nav, a.driving, a.minimap)
	a.board.Write().ShowGrid(false)
	a.world.Write().ShowSectors(world.SectorBands)
}

func (a *arena) definePlayer() error {
	a.player = a.players.Local("player")
	return a.player.Bind(a.players.Defaults()...)
}

// enlisted is what the player's units are told as they are made, the armies the map lays and the
// soldiers they muster alike
func (a *arena) enlisted() []command.Command {
	return []command.Command{players.Give{To: a.player.ID}, selection.Allow{}}
}

func (a *arena) defineFigures() {
	a.board.Tiles(tiles()...)
	a.board.Surfaces(surface{})
	a.board.Figures(
		forest{},
		tree{},
		soldier{
			kind:    RedSoldierKind,
			role:    RedSoldierRole,
			side:    RedRole,
			foe:     BlueRole,
			plan:    RedSoldierPlan,
			foeArmy: BlueArmyName,
			color:   redColor,
			wounded: redWounded,
		},
		soldier{
			kind:    BlueSoldierKind,
			role:    BlueSoldierRole,
			side:    BlueRole,
			foe:     RedRole,
			plan:    BlueSoldierPlan,
			foeArmy: RedArmyName,
			color:   blueColor,
			wounded: blueWounded,
		},
		army{
			kind:     RedArmyKind,
			role:     RedArmyRole,
			plan:     RedArmyPlan,
			side:     RedRole,
			foe:      BlueRole,
			soldier:  RedSoldierKind,
			foeArmy:  BlueArmyName,
			soldiers: RedSoldiersGroup,
			color:    redColor,
			enlisted: a.enlisted,
		},
		army{
			kind:     BlueArmyKind,
			role:     BlueArmyRole,
			plan:     BlueArmyPlan,
			side:     BlueRole,
			foe:      RedRole,
			soldier:  BlueSoldierKind,
			foeArmy:  RedArmyName,
			soldiers: BlueSoldiersGroup,
			color:    blueColor,
			enlisted: a.enlisted,
		},
	)
}

// the states both sides' armies and soldiers share: an army's losses, the blows left of its
// battle, its charge; a soldier's wound, its swing and its charge
func (a *arena) defineEffects() {
	fx := a.world.Write().Effects()
	fx.Define(WoundedEf, effect.Spec{effect.Lasts(healing)})
	fx.Define(BlowEf, effect.Spec{effect.Lasts(swing)})
	fx.Define(ChargingEf, effect.Spec{})
	fx.Define(EngagedEf, effect.Spec{effect.Lasts(engage)})
	for k := 1; k < soldiers; k++ {
		fx.Define(lossEf(k), effect.Spec{})
	}
	for n := 1; n <= soldiers; n++ {
		fx.Define(fightEf(n), effect.Spec{})
	}
}

// the sides, which a foe's rules ask; the felling, which a tree's ask; the soldiers' being struck
// down, which a foe's strike asks, never the army standing among them
func (a *arena) defineRules() {
	a.world.Roles().Define(RedRole)
	a.world.Roles().Define(BlueRole)
	a.world.Roles().Define(FellerRole)
	a.world.Roles().Define(MortalRole)
}

// the selected army charges the other one
func (a *arena) defineCommands() {
	roles := a.world.Roles()
	charge := ceremony.Put(a.world.Write().Effects().Named(ChargingEf)).
		On(a.selection.Selected(roles.Named(RedArmyRole), roles.Named(BlueArmyRole)))
	a.world.Commands().Define(ChargeCmd, charge)
}

func (a *arena) bindKeys() error {
	return a.player.Bind(control.Give(control.KeyPress{Key: control.KeyH},
		"Charge the other army with the one selected", a.world.Commands().Named(ChargeCmd)))
}

func (a *arena) spawn() error {
	level, err := a.board.Lay(mapFile, "map.yaml")
	if err != nil {
		return err
	}
	if err := board.Spawn[forestRow](level, nil); err != nil {
		return err
	}
	return board.Spawn[armyRow](level, board.Sides{PlayerSide: a.enlisted()})
}

func (a *arena) defineScenes() []game.Scene {
	main := &mainScene{arena: a, status: &status{arena: a}}
	a.telemetry.With(a.nav.Read(0).Reporter(), a.world.Read(0).Sectors(),
		a.world.Read(0).Seen(), main.status)
	return []game.Scene{ui.NewScene(MainScene, main.screen()).Input(a.players.Handle)}
}

func (a *arena) update(ctx goke.RunCtx, d time.Duration) {
	a.world.RunPlan(ctx, d)
	a.motion.RunPlan(ctx, d)
	a.collision.RunPlan(ctx, d)
	a.board.RunPlan(ctx, d)
	a.nav.RunPlan(ctx, d)
	a.driving.RunPlan(ctx, d)
	a.selection.RunPlan(ctx, d)
	a.cameras.RunPlan(ctx, d)
	a.minimap.RunPlan(ctx, d)
	a.players.RunPlan(ctx, d)
	ctx.Sync()
}
