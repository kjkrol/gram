package main

import (
	"embed"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/game/stage"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/cameras"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/dialog"
	"github.com/kjkrol/gram/plugins/driving"
	"github.com/kjkrol/gram/plugins/navigation"
	"github.com/kjkrol/gram/plugins/players"
	"github.com/kjkrol/gram/plugins/selection"
	"github.com/kjkrol/gram/plugins/vision"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/steering"
	"github.com/kjkrol/gram/ui"
	"github.com/kjkrol/uid"
)

//go:embed dialogs
var dialogs embed.FS

const (
	TPS          = 60
	GridWidth    = 24
	GridHeight   = 15
	CellSize     = 32
	ScreenWidth  = GridWidth * CellSize
	ScreenHeight = GridHeight * CellSize
	unitSize     = 20
	unitSpeed    = CellSize * 3
	MaxEntCount  = 4
)

//go:embed map.yaml
var mapFile embed.FS

type arena struct {
	world     *world.Plugin
	collision *collision.Plugin
	board     *board.Plugin
	selection *selection.Plugin
	driving   *driving.Plugin
	nav       *navigation.Plugin
	vision    *vision.Plugin
	dialog    *dialog.Plugin
	cameras   *cameras.Plugin
	players   *players.Plugin
	player    *players.Player
}

func main() {
	props := game.Props{
		Title: "gram dialog demo",
		Description: "Right click: walk up to a host and answer it. " +
			"Point at a host with the traveller selected to see what it makes of him.",
		ScreenWidth: ScreenWidth, ScreenHeight: ScreenHeight, Resizable: true,
		TargetTPS: TPS,
	}
	a := &arena{}
	mainStage := stage.New(DialogStage).
		World(world.Config{
			Space: world.SpaceCfg{Width: ScreenWidth, Height: ScreenHeight},
			Entities: world.EntitiesCfg{
				MaxCount: MaxEntCount,
				MinSize:  unitSize,
				MaxSize:  unitSize,
			},
		}).
		Plugins(a.makePlugins).
		Figures(a.defineFigures).
		Players(a.definePlayer).
		Effects(a.defineEffects).
		With(dialog.Conversations(a.defineConversations)).
		Spawn(a.spawn).
		Scenes(a.defineScenes).
		Update(a.update)
	gram.Run(game.New(props, mainStage))
}

func (a *arena) makePlugins(w *world.Plugin) {
	a.world = w
	a.collision = collision.NewPlugin(a.world)
	grid := grid.DefaultGrids{}.Square(GridWidth, GridHeight, CellSize)
	a.board = board.NewPlugin(grid, &cell.SingleOccupancy{}, a.world).WithCollision(a.collision)
	a.selection = selection.NewPlugin(a.world)
	a.driving = driving.NewPlugin(a.world, a.selection).WithGround(a.board)
	a.nav = navigation.NewPlugin(a.board, a.world, a.selection, a.driving).
		WithCollision(a.collision)
	a.vision = vision.NewPlugin(a.world)
	a.dialog = dialog.NewPlugin(a.world, dialog.Config{})
	a.cameras = cameras.NewPlugin(a.world)
	a.players = players.NewPlugin(
		a.world,
		a.collision, a.cameras,
		a.board,
		a.selection,
		a.nav,
		a.driving,
		a.vision,
		a.dialog,
	)
}

func (a *arena) definePlayer() error {
	a.player = a.players.Local("player")
	return a.player.Bind(a.players.Defaults()...)
}

func (a *arena) defineEffects() { a.dialog.DefineEffects() }

func (a *arena) defineConversations(d *dialog.Plugin) error {
	like := func(by int8) dialog.Change {
		return dialog.Alter(func(r *Regard, listener uid.UID64) { r.Like(listener, by) })
	}
	changes := d.Changes()
	changes.Define(WarmChange, like(40))
	changes.Define(PleaseChange, like(20))
	changes.Define(ForgiveChange, like(50))
	changes.Define(IrkChange, like(-20))
	changes.Define(OffendChange, like(-60))
	conditions := d.Conditions()
	conditions.Define(FriendCond, dialog.When((*Regard).Friendly))
	conditions.Define(EnemyCond, dialog.When((*Regard).Hostile))
	for _, file := range []string{"dialogs/miller.yaml", "dialogs/smith.yaml"} {
		if err := d.Load(dialogs, file); err != nil {
			return err
		}
	}
	return nil
}

type unitRow struct {
	Start cell.ID `yaml:"start"`
	Talks string  `yaml:"talks"`
}

func units(b *board.Plugin) *board.Units[unitRow] {
	return board.NewUnits[unitRow](b, board.Shape{Size: unitSize},
		func(u unitRow) geom.Vec { return b.Board().CellCenter(u.Start) })
}

var walks = steering.Steering{
	MaxSpeed: unitSpeed,
	Accel:    unitSpeed * 3,
	Brake:    unitSpeed * 6,
	TurnRate: 0.3,
}

// the traveller's role before the host's rule that names it
func (a *arena) defineFigures() {
	a.board.Tiles(meadow{})
	a.board.Figures(traveller{}, host{talks: a.dialog})
}

func (a *arena) spawn() error {
	level, err := a.board.Lay(mapFile, "map.yaml")
	if err != nil {
		return err
	}
	return board.Spawn[unitRow](level, board.Sides{
		PlayerSide: {players.Give{To: a.player.ID}, selection.Allow{Selected: true}},
	})
}

func (a *arena) defineScenes() []game.Scene {
	m := &mainScene{arena: a}
	return []game.Scene{ui.NewScene(MainScene, m.screen()).
		Input(a.players.Handle).
		Issue(a.players.IssueAs(a.player))}
}

func (a *arena) update(ctx goke.RunCtx, d time.Duration) {
	a.world.RunPlan(ctx, d)
	a.collision.RunPlan(ctx, d)
	a.board.RunPlan(ctx, d)
	a.nav.RunPlan(ctx, d)
	a.driving.RunPlan(ctx, d)
	a.vision.RunPlan(ctx, d)
	a.dialog.RunPlan(ctx, d)
	a.selection.RunPlan(ctx, d)
	a.cameras.RunPlan(ctx, d)
	a.players.RunPlan(ctx, d)
	ctx.Sync()
}
