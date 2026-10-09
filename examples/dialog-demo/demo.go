// Command dialog-demo is conversations written as data: walk the traveller (right click) up to a
// host, who looks west, the way the traveller comes, sees it near and talks in a window above it.
// The miller and the smith each say lines of their own, read from dialogs/*.yaml; the answers lead
// on through the conversation and move what the host makes of the traveller, which it remembers:
// point at a host with the traveller selected and it says so under it — Friend, Neutral or Enemy —
// and meets the traveller again accordingly. Walk away mid-word and the conversation is over. One
// window serves both hosts.
package main

import (
	"embed"
	"image/color"
	"math"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/entity/kind/comp"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/game/stage"
	"github.com/kjkrol/gram/plugin"
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
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/rule"
	"github.com/kjkrol/gram/ui"
	"github.com/kjkrol/uid"
)

// dialogs are the hosts' conversations, a file each.
//
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

	// hostSight is how far the host sees the traveller come: two cells and a half, in a cone as
	// wide as vision allows (vision.MaxHalfAngleMilli).
	hostSight = CellSize * 2.5
	hostCone  = math.Pi / 3
)

// =========================== Game ===========================

// NewDemo is the dialog demo, one Stage.
func NewDemo() game.Game {
	_, g := newDemo()
	return g
}

// =========================== Stage ===========================

// arena is a meadow with the traveller the player walks and two hosts standing in it.
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
	scene     *ui.Scene
}

// newDemo makes the arena — the collector of the stage's plugins, which every section builds on —
// and the game defined on it: its props and its Stage, a section at a time.
func newDemo() (*arena, game.Game) {
	s := &arena{}
	return s, game.New(game.Props{
		Title:       "gram dialog demo — right click: walk up to a host; point at one to see what it makes of you",
		ScreenWidth: ScreenWidth, ScreenHeight: ScreenHeight, Resizable: true,
		TargetTPS: TPS,
	},
		stage.New(DialogStage).
			World(world.Config{
				Space:    world.SpaceCfg{Width: ScreenWidth, Height: ScreenHeight},
				Entities: world.EntitiesCfg{MaxCount: MaxEntCount, MinSize: UnitSize, MaxSize: UnitSize},
			}, s.makePlugins).
			Players(s.definePlayer).
			Effects(s.defineEffects).
			Rules(s.defineRoles).
			With(dialog.Conversations(s.defineConversations)).
			Kinds(s.defineCells, s.defineKinds).
			Spawn(s.spawnCells, s.spawnUnits).
			Scenes(s.defineScenes).
			Update(s.update))
}

// makePlugins makes the Stage's plugins over its world w — anew each time the Stage is entered —
// in the order they are used.
func (s *arena) makePlugins(w *world.Plugin) []plugin.Plugin {
	s.world = w
	s.collision = collision.NewPlugin(s.world)
	grid := grid.DefaultGrids{}.Square(GridWidth, GridHeight, CellSize)
	s.board = board.NewPlugin(grid, &cell.SingleOccupancy{}, s.world).WithCollision(s.collision)
	s.selection = selection.NewPlugin(s.world)
	s.driving = driving.NewPlugin(s.world, s.selection).WithGround(s.board)
	s.nav = navigation.NewPlugin(s.board, s.world, s.selection, s.driving).WithCollision(s.collision)
	s.vision = vision.NewPlugin(s.world)
	s.dialog = dialog.NewPlugin(s.world, dialog.Config{})
	s.cameras = cameras.NewPlugin(s.world)
	s.players = players.NewPlugin(s.world, s.cameras, s.board, s.selection, s.nav, s.driving, s.vision, s.dialog)
	return []plugin.Plugin{s.collision, s.board, s.selection, s.nav, s.driving, s.vision, s.dialog, s.cameras, s.players}
}

func (s *arena) definePlayer() error {
	s.player = s.players.Local("player")
	return s.player.Bind(s.players.Defaults()...)
}

// defineEffects are the conversations' handles, the dialog plugin's own.
func (s *arena) defineEffects() { s.dialog.DefineEffects() }

// defineRoles: the host, seeing the traveller near, not talking and not just done talking, begins
// a conversation with it; seeing nobody, it ends the one it holds.
func (s *arena) defineRoles() {
	effects := s.world.Effects()
	talking, talked := effects.Named(dialog.TalkingEf), effects.Named(dialog.TalkedEf)
	roles := s.world.Roles()
	roles.Define(TravellerRole)
	traveller := roles.Named(TravellerRole)
	roles.Define(HostRole,
		rule.Then[vision.Sighting]("talk", rule.Other(traveller),
			rule.If(rule.Not(vision.Sighting.Nobody), rule.Unless(talking, rule.Unless(talked, rule.Order(dialog.Begin{}))))),
		rule.Then[vision.Sighting]("part", rule.Other(traveller),
			rule.If(vision.Sighting.Nobody, rule.Dispel(talking))),
	)
}

// defineConversations says what the hosts' answers change in this game — their Regard — and when
// they are offered, then loads the conversations naming them, a file each: what they say is data.
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

// unitRow is the row a unit spawns from: the cell it starts on and, for a host, the node its
// conversations begin at.
type unitRow struct {
	start cell.ID
	talks string
}

func (s *arena) defineKinds() {
	brd := s.board.Res.Logic.Board
	units := board.NewUnits[unitRow](s.board, board.Shape{Size: UnitSize}, func(u unitRow) geom.Vec { return brd.CellCenter(u.start) })
	walks := steering.Steering{MaxSpeed: UnitSpeed, Accel: UnitSpeed * 3, Brake: UnitSpeed * 6, TurnRate: 0.3}
	roles := s.world.Roles()
	units.Define(TravellerKind, unit.Mover{Domain: cell.Land}, walks, rule.Plays(roles.Named(TravellerRole)))
	units.Define(HostKind, unit.Mover{Domain: cell.Land}, walks, rule.Plays(roles.Named(HostRole)),
		comp.Const(vision.Sight{Facing: geom.NewVec(-1, 0), Radius: hostSight}),
		comp.Const(world.Eye{Angle: hostCone}), // looking west, the way the traveller comes
		// where its conversations begin: each host's lines its own
		comp.Load(func(u unitRow) dialog.Script { return s.dialog.Script(u.talks) }),
		comp.Const(Regard{}), // what it makes of whom it talked with: this game's model
	)
}

func (s *arena) defineScenes() []game.Scene {
	s.scene = ui.NewScene(MainScene, s.screen()).
		Input(s.players.Handle).
		Issue(s.players.IssueAs(s.player))
	return []game.Scene{s.scene}
}

func (s *arena) spawnCells() {
	s.board.Seed(board.Layout{Default: MeadowCell})
}

func (s *arena) spawnUnits() {
	traveller := kind.Named[unitRow](s.world.Kinds(), TravellerKind)
	host := kind.Named[unitRow](s.world.Kinds(), HostKind)
	brd := s.board.Res.Logic.Board
	s.world.Seed(
		traveller.Entry(unitRow{start: brd.CellIndex(3, 7)}).Told(players.Give{To: s.player.ID}, selection.Allow{Selected: true}),
		host.Entry(unitRow{start: brd.CellIndex(16, 4), talks: MillerGreetNode}),
		host.Entry(unitRow{start: brd.CellIndex(16, 10), talks: SmithGreetNode}),
	)
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

// =========================== Scene ===========================

// The scene's colours: the meadow, the traveller and the host.
var (
	meadowColor    = color.RGBA{R: 70, G: 110, B: 60, A: 255}
	travellerColor = color.RGBA{R: 230, G: 200, B: 90, A: 255}
	hostColor      = color.RGBA{R: 120, G: 160, B: 230, A: 255}
)

// picture dresses the meadow and the two and hands the world's picture.
func (s *arena) picture() render.Picture {
	worldAtlas := render.NewAtlas()
	worldAtlas.Add(kind.Named[unitRow](s.world.Kinds(), TravellerKind), UnitSize, render.Diamond(travellerColor))
	worldAtlas.Add(kind.Named[unitRow](s.world.Kinds(), HostKind), UnitSize, render.Solid(hostColor))
	worldAtlas.Close()
	s.world.WithRenderer(worldAtlas)

	boardAtlas := s.board.NewAtlas(CellSize)
	boardAtlas.Add(MeadowCell, render.Solid(meadowColor))
	boardAtlas.Close()
	s.board.WithRenderer(boardAtlas)

	s.nav.WithRenderer(nil)
	s.selection.WithRenderer(nil)
	return render.NewComposer(s.board.Renderer(), s.world.Renderer(), s.selection.Renderer(), s.nav.Renderer())
}

// screen is the meadow through a camera of its own, the player's view; above every host that
// talks, its conversation, and under the one the cursor points at, what it makes of the traveller.
func (s *arena) screen() *ui.Element {
	return ui.Layers( // from the bottom up: each covers those before it
		ui.Image(render.NewFeed(s.cameras.New(cameras.TopDown(), camera.Config{}), s.picture())).Input(s.players.Through(s.player)),
		s.dialog.Window().Above().Offset(0, -6),
		ui.LabelOf(ui.Of(func(r *Regard) (string, bool) { // what the host makes of the traveller
			chosen, ok := s.selection.Chosen(s.player.ID)
			return r.Stance(chosen), ok
		})).Where(ui.Tagged(s.selection.Tags().Hovered)).Below().Offset(0, 4),
	)
}
