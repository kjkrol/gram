// Command trapdoor-demo is two levers and two strips of trapdoors across a meadow: wanderers
// nobody owns walk to and fro over both, and the player pulls a lever — 1 the west one, 2 the east
// — and its trapdoors open for a while: every unfortunate standing on one falls in, the player's
// own scouts too. The scouts are clicked about as anywhere; J hastens the selected ones for a
// while, to get off a strip in time. A lever is a command: open the group of cells that is its
// strip, for a while (rule.Cast, entity.Group); the haste a command for the selected
// (selection.Selected). All of it is defined here, in the game; the rules are a role's, played by
// the kinds.
package main

import (
	"image/color"
	"log"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/entity"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/entity/kind/comp"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/game/stage"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/unit"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/navigation"
	"github.com/kjkrol/gram/plugins/players"
	"github.com/kjkrol/gram/plugins/selection"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/steering"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/rule"
	"github.com/kjkrol/gram/rule/effect"
)

const (
	TPS          = 60
	GridWidth    = 24
	GridHeight   = 16
	CellSize     = 32
	ScreenWidth  = GridWidth * CellSize
	ScreenHeight = GridHeight * CellSize
	EntitySize   = 20
	UnitSpeed    = CellSize * 2
	MaxEntCount  = 16 // the units; cell entities do not count

	// the strips of trapdoors run from top to bottom of the meadow, two cells wide
	stripTop, stripBottom uint32 = 2, 13

	// leverHeld is how long the trapdoors stay open once their lever is pulled.
	leverHeld = 2 * time.Second
	// hasteHeld is how long the scouts go twice as fast.
	hasteHeld = 3 * time.Second
)

// rows are where the wanderers walk to and fro, each across both strips.
var rows = []uint32{3, 6, 9, 12}

// levers are the two levers: the key that pulls each and the first column of its strip.
var levers = []struct {
	name string
	key  control.Key
	left uint32
}{{"west", control.Key1, 7}, {"east", control.Key2, 15}}

// =========================== Game ===========================

// Demo is the trapdoor demo — exactly one Stage (arena below).
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
		Title:       "gram — trapdoors under a lever",
		ScreenWidth: ScreenWidth, ScreenHeight: ScreenHeight, Resizable: true,
		TargetTPS: TPS,
	}
}

func (d *Demo) Stages() (map[string]game.Stage, string) {
	return map[string]game.Stage{d.stage.Name(): d.stage}, d.stage.Name()
}

// =========================== Stage ===========================

// unit is the row every kind spawns from: where it starts and, for a wanderer, the other end of
// its walk.
type unitRow struct{ start, to cell.ID }

type arena struct {
	world     *world.Plugin
	board     *board.Plugin
	nav       *navigation.Plugin
	collision *collision.Plugin
	selection *selection.Plugin
	players   *players.Plugin
	player    *players.Player // the one at this keyboard: the scouts are its
	brd       *board.Board

	hasteSprite render.SpriteID
}

// newStage defines the game a section at a time, each building on those before it.
// newArena makes the arena — the collector of the stage's plugins, which every
// section builds on — and defines the stage on it, a section at a time.
func newArena() (*arena, game.Stage) {
	s := &arena{}
	return s, stage.New("trapdoor-demo").
		Plugins(s.usePlugins).
		Players(s.definePlayer).
		Effects(s.defineEffects).
		Rules(s.defineRules).
		Commands(s.defineCommands).
		Cells(s.defineCells).
		Kinds(s.defineKinds).
		Controls(s.bindKeys).
		Scenes(s.defineScenes).
		Layout(s.layOut).
		Units(s.placeUnits).
		Update(s.update)
}

func (s *arena) usePlugins(ctx game.Initializer) error {
	s.world = ctx.UseWorld(world.Config{
		Space:    world.SpaceCfg{Width: ScreenWidth, Height: ScreenHeight},
		Entities: world.EntitiesCfg{MaxCount: MaxEntCount, MinSize: EntitySize, MaxSize: EntitySize},
	})
	grid := grid.DefaultGrids{}.Square(GridWidth, GridHeight, CellSize)
	s.collision = collision.NewPlugin(s.world)
	s.board = board.NewPlugin(grid, &cell.SingleOccupancy{}, s.world).WithCollision(s.collision).WithLog(log.Default())
	s.brd = s.board.Res.Logic.Board
	s.selection = selection.NewPlugin(s.world)
	s.nav = navigation.NewPlugin(s.board, s.world, s.selection).WithCollision(s.collision)
	s.players = players.NewPlugin(s.world, s.board, s.selection, s.nav)
	for _, p := range []plugin.Plugin{s.collision, s.board, s.selection, s.nav, s.players} {
		if err := ctx.Use(p); err != nil {
			return err
		}
	}
	return nil
}

func (s *arena) definePlayer() error {
	s.player = s.players.Local("player")
	return s.player.Bind(s.players.Defaults()...)
}

func (s *arena) defineCells() {
	kinds := s.board.CellKinds()
	kinds.Define(GrassCell, cell.Kind{Cost: 1, Allows: cell.Land})
	kinds.Define(BoardsCell, cell.Kind{Cost: 1, Allows: cell.Land}) // a trapdoor shut
	kinds.Define(PitCell, cell.Kind{Cost: 1})                       // holds nobody
}

func (s *arena) defineEffects() {
	kinds := s.board.CellKinds() // the kinds are defined later: the alter resolves the pit as it runs
	fx := s.world.Effects()
	fx.Define(OpenEf, effect.Spec{effect.Alter(func(g *cell.Ground) { g.Kind = kinds.Named(PitCell) })})
	s.hasteSprite = s.world.Kinds().NewSprite()
	hasteSprite := s.hasteSprite
	fx.Define(HasteEf, effect.Spec{
		effect.Lasts(hasteHeld),
		effect.Alter(func(st *steering.Steering) { st.MaxSpeed, st.Accel = st.MaxSpeed*2, st.Accel*2 }),
		effect.Alter(func(a *world.Appearance) { a.SpriteID = hasteSprite }),
	})
}

func (s *arena) defineRules() {
	s.world.Roles().Define(MortalRole,
		rule.Then[unit.Standing]("fall in", rule.All, rule.If(unit.Standing.Fallen, rule.Order(world.Despawn{}))))
}

func (s *arena) defineCommands() {
	cmds := s.world.Commands()
	for _, l := range levers {
		cmds.Define(pullCmd(l.name), rule.Cast(s.world.Effects().Named(OpenEf)).On(entity.Group("trapdoors "+l.name)).For(leverHeld))
	}
	cmds.Define(HastenCmd, rule.Cast(s.world.Effects().Named(HasteEf)).On(s.selection.Selected()))
}

func (s *arena) bindKeys() error {
	for _, l := range levers {
		if err := s.player.Bind(control.Give(control.KeyPress{Key: l.key}, "Pull the "+l.name+" lever: its trapdoors open", s.world.Commands().Named(pullCmd(l.name)))); err != nil {
			return err
		}
	}
	return s.player.Bind(control.Give(control.KeyPress{Key: control.KeyJ}, "Hasten the selected scouts", s.world.Commands().Named(HastenCmd)))
}

func (s *arena) defineScenes() []game.Scene {
	main := &mainScene{arena: s}
	return []game.Scene{main}
}

func (s *arena) defineKinds() {
	units := board.NewUnits[unitRow](s.board, board.Shape{Size: EntitySize}, func(u unitRow) geom.Vec { return s.brd.CellCenter(u.start) })
	profile := steering.Steering{MaxSpeed: UnitSpeed, Accel: UnitSpeed * 2, V0: UnitSpeed / 2, TurnRate: 0.15}
	land := unit.Mover{Domain: cell.Land}
	units.Define(ScoutKind, land, profile, rule.Plays(s.world.Roles().Named(MortalRole)))
	// a wanderer walks to the other end of its row and back, a second's rest at each end
	units.Define(WandererKind, land, profile,
		comp.Load(func(u unitRow) navigation.MoveOrder { return navigation.Patrol(time.Second, u.to, u.start) }),
		rule.Plays(s.world.Roles().Named(MortalRole)))
}

func (s *arena) cellAt(x, y uint32) cell.ID { c, _ := s.brd.CellIndex(x, y); return c }

func (s *arena) layOut() {
	var cells []cell.Entry
	for _, l := range levers {
		for y := stripTop; y <= stripBottom; y++ {
			for x := l.left; x <= l.left+1; x++ {
				cells = append(cells, cell.Entry{Kind: BoardsCell, Cell: s.cellAt(x, y), Group: "trapdoors " + l.name})
			}
		}
	}
	s.board.Seed(board.Layout{Default: GrassCell, Cells: cells})
}

func (s *arena) placeUnits() {
	for i := range uint32(3) {
		s.world.Seed(kind.Named[unitRow](s.world.Kinds(), ScoutKind).Entry(unitRow{start: s.cellAt(3+2*i, GridHeight-2)}).Told(players.Give{To: s.player.ID}, selection.Allow{}))
	}
	for _, row := range rows {
		s.world.Seed(kind.Named[unitRow](s.world.Kinds(), WandererKind).Entry(unitRow{start: s.cellAt(2, row), to: s.cellAt(GridWidth-3, row)}))
	}
}

func (s *arena) update(ctx goke.RunCtx, d time.Duration) {
	s.world.RunPlan(ctx, d)
	s.collision.RunPlan(ctx, d)
	s.board.RunPlan(ctx, d)
	s.nav.RunPlan(ctx, d)
	s.selection.RunPlan(ctx, d)
	s.players.RunPlan(ctx, d)
	ctx.Sync()
}

// =========================== Scene ===========================

type mainScene struct {
	arena *arena
}

var _ game.Scene = (*mainScene)(nil)

func (m *mainScene) Name() string { return "main" }

func (m *mainScene) Layers() []render.Layer {
	s := m.arena

	worldAtlas := render.NewAtlas()
	worldAtlas.RegisterAt(kind.Named[unitRow](s.world.Kinds(), ScoutKind).SpriteID(), EntitySize, render.Solid(color.RGBA{R: 90, G: 140, B: 230, A: 255}))
	worldAtlas.RegisterAt(s.hasteSprite, EntitySize, render.Solid(color.RGBA{R: 170, G: 220, B: 255, A: 255}))
	worldAtlas.RegisterAt(kind.Named[unitRow](s.world.Kinds(), WandererKind).SpriteID(), EntitySize, render.Diamond(color.RGBA{R: 220, G: 150, B: 60, A: 255}))
	worldAtlas.Close()
	s.world.WithRenderer(worldAtlas)

	kinds := s.board.CellKinds()
	boardAtlas := render.NewAtlas()
	for name, c := range map[string]color.RGBA{
		GrassCell:  {R: 60, G: 95, B: 60, A: 255},
		BoardsCell: {R: 120, G: 90, B: 55, A: 255},
		PitCell:    {R: 15, G: 12, B: 20, A: 255},
	} {
		k, _ := kinds.Get(name)
		boardAtlas.RegisterAt(k.SpriteID, CellSize, render.Solid(c))
	}
	boardAtlas.Close()
	s.board.WithRenderer(boardAtlas)

	s.nav.WithRenderer(nil)
	s.selection.WithRenderer(nil)

	return []render.Layer{render.NewComposer(s.board.Renderer(), s.world.Renderer(), s.selection.Renderer(), s.nav.Renderer())}
}

// Viewports are where the world is shown: the local players' views.
func (m *mainScene) Viewports(screen geom.AABB) []render.Viewport {
	return m.arena.players.Viewports(screen)
}

func (m *mainScene) HandleEvents(events *control.InputEvents, runtime game.Runtime, composition game.Composition) {
	m.arena.players.Handle(events, runtime, composition)
}

func (m *mainScene) Focusable() bool { return true }
