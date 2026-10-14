package main

import (
	"embed"
	"math"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony"
	"github.com/kjkrol/ceremony/act"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/ceremony/cond"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/game/stage"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/unit"
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
	UnitSize     = 20
	UnitSpeed    = CellSize * 3
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
				MinSize:  UnitSize,
				MaxSize:  UnitSize,
			},
		}).
		Plugins(a.makePlugins).
		Players(a.definePlayer).
		Effects(a.defineEffects).
		Rules(a.defineRoles).
		With(dialog.Conversations(a.defineConversations)).
		Kinds(a.defineCells, a.defineKinds).
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

func (a *arena) defineRoles() {
	effects := a.world.Effects()
	talking, talked := effects.Named(dialog.TalkingEf), effects.Named(dialog.TalkedEf)
	roles := a.world.Roles()
	roles.Define(TravellerRole)
	traveller := roles.Named(TravellerRole)
	roles.Define(HostRole,
		act.Branch[vision.Sighting]{
			Name: "talk",
			When: cond.Other.If(traveller).
				And(cond.Self.If(cond.Not(talking))).
				And(cond.Self.If(cond.Not(talked))),
			Then: act.Self.Do(dialog.Begin{}),
		},
		act.Branch[vision.Sighting]{
			Name: "part",
			When: cond.Self.If(vision.SeesNobody()),
			Then: act.Self.Take(talking),
		},
	)
}

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

func (a *arena) defineCells() {
	a.board.CellKinds().Define(MeadowCell, cell.Kind{Cost: 1, Allows: cell.Land})
}

type unitRow struct {
	Start cell.ID `yaml:"start"`
	Talks string  `yaml:"talks"`
}

func (a *arena) defineKinds() {
	brd := a.board.Board()
	units := board.NewUnits[unitRow](
		a.board,
		board.Shape{Size: UnitSize},
		func(u unitRow) geom.Vec { return brd.CellCenter(u.Start) },
	)
	walks := steering.Steering{
		MaxSpeed: UnitSpeed,
		Accel:    UnitSpeed * 3,
		Brake:    UnitSpeed * 6,
		TurnRate: 0.3,
	}
	roles := a.world.Roles()

	units.Define(
		TravellerKind,
		unit.Mover{Domain: cell.Land},
		walks,
		ceremony.Plays(roles.Named(TravellerRole)),
	)
	units.Define(HostKind, unit.Mover{Domain: cell.Land}, walks, ceremony.Plays(roles.Named(HostRole)),
		comp.Const(vision.Sight{Facing: geom.NewVec(-1, 0), Radius: CellSize * 2.5}),
		comp.Const(world.Eye{Angle: math.Pi / 3}), // looking west, the way the traveller comes
		comp.Load(func(u unitRow) dialog.Script { return a.dialog.Script(u.Talks) }),
		comp.Const(Regard{}),
	)
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
