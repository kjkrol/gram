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

func (s *arena) makePlugins(w *world.Plugin) {
	s.world = w
	s.collision = collision.NewPlugin(s.world)
	grid := grid.DefaultGrids{}.Square(GridWidth, GridHeight, CellSize)
	s.board = board.NewPlugin(grid, &cell.SingleOccupancy{}, s.world).WithCollision(s.collision)
	s.selection = selection.NewPlugin(s.world)
	s.driving = driving.NewPlugin(s.world, s.selection).WithGround(s.board)
	s.nav = navigation.NewPlugin(s.board, s.world, s.selection, s.driving).
		WithCollision(s.collision)
	s.vision = vision.NewPlugin(s.world)
	s.dialog = dialog.NewPlugin(s.world, dialog.Config{})
	s.cameras = cameras.NewPlugin(s.world)
	s.players = players.NewPlugin(
		s.world,
		s.cameras,
		s.board,
		s.selection,
		s.nav,
		s.driving,
		s.vision,
		s.dialog,
	)
}

func (s *arena) definePlayer() error {
	s.player = s.players.Local("player")
	return s.player.Bind(s.players.Defaults()...)
}

func (s *arena) defineEffects() { s.dialog.DefineEffects() }

func (s *arena) defineRoles() {
	effects := s.world.Effects()
	talking, talked := effects.Named(dialog.TalkingEf), effects.Named(dialog.TalkedEf)
	roles := s.world.Roles()
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

func (s *arena) defineConversations(d *dialog.Plugin) error {
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

func (s *arena) defineCells() {
	s.board.CellKinds().Define(MeadowCell, cell.Kind{Cost: 1, Allows: cell.Land})
}

type unitRow struct {
	Start cell.ID `yaml:"start"`
	Talks string  `yaml:"talks"`
}

func (s *arena) defineKinds() {
	brd := s.board.Board()
	units := board.NewUnits[unitRow](
		s.board,
		board.Shape{Size: UnitSize},
		func(u unitRow) geom.Vec { return brd.CellCenter(u.Start) },
	)
	walks := steering.Steering{
		MaxSpeed: UnitSpeed,
		Accel:    UnitSpeed * 3,
		Brake:    UnitSpeed * 6,
		TurnRate: 0.3,
	}
	roles := s.world.Roles()

	units.Define(
		TravellerKind,
		unit.Mover{Domain: cell.Land},
		walks,
		ceremony.Plays(roles.Named(TravellerRole)),
	)
	units.Define(HostKind, unit.Mover{Domain: cell.Land}, walks, ceremony.Plays(roles.Named(HostRole)),
		comp.Const(vision.Sight{Facing: geom.NewVec(-1, 0), Radius: CellSize * 2.5}),
		comp.Const(world.Eye{Angle: math.Pi / 3}), // looking west, the way the traveller comes
		comp.Load(func(u unitRow) dialog.Script { return s.dialog.Script(u.Talks) }),
		comp.Const(Regard{}),
	)
}

func (s *arena) spawn() error {
	level, err := s.board.Lay(mapFile, "map.yaml")
	if err != nil {
		return err
	}
	return board.Spawn[unitRow](level, board.Sides{
		PlayerSide: {players.Give{To: s.player.ID}, selection.Allow{Selected: true}},
	})
}

func (s *arena) defineScenes() []game.Scene {
	m := &mainScene{arena: s}
	return []game.Scene{ui.NewScene(MainScene, m.screen()).
		Input(s.players.Handle).
		Issue(s.players.IssueAs(s.player))}
}

func (s *arena) update(ctx goke.RunCtx, d time.Duration) {
	s.world.RunPlan(ctx, d)
	s.collision.RunPlan(ctx, d)
	s.board.RunPlan(ctx, d)
	s.nav.RunPlan(ctx, d)
	s.driving.RunPlan(ctx, d)
	s.vision.RunPlan(ctx, d)
	s.dialog.RunPlan(ctx, d)
	s.selection.RunPlan(ctx, d)
	s.cameras.RunPlan(ctx, d)
	s.players.RunPlan(ctx, d)
	ctx.Sync()
}
