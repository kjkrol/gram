// Command wire-demo is three commands on one meadow, each a sentence saying what it does, whom
// it is for and who sets it off (rule.Cast, Toggle; entity.Named, entity.Group). "Open the west
// trapdoors" is the west lever's: 1 gives it, and so does a scout pulling the lever beside it (U).
// "Open the east trapdoors" is the pressure plate's in the yard, given while someone stands on it;
// the trapdoors stay open two seconds after. "Flip the gate" is the player's alone: G opens it and
// it stays open until G again. Two strips of trapdoors cross the meadow, each a group of cells, and
// a fence shuts the yard off from it, its gate a group too. A plate stood on and a lever pulled
// only Trigger: which command that sets off, the command says with By. Wanderers nobody owns walk
// to and fro over both strips; the player's scouts and porters start in the yard. Everyone plays
// mortal and falls in where nothing holds them; the scouts play hasty too, and J hastens the
// selected ones (selection.Selected), never the porters. All of it is defined here, in the game.
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
	MaxEntCount  = 16 // the units; cell entities and wires do not count

	// the strips of trapdoors run down the meadow, two cells wide from their left column
	stripTop, stripBottom uint32 = 1, 11
	westLeft, eastLeft    uint32 = 7, 15
	// the fence runs across under the meadow, its gate two cells from gateLeft; the yard under it
	// holds the plate, the scouts on its first row and the porters on its second
	fenceRow, gateLeft uint32 = 13, 11
	yardRow, plateCol  uint32 = 14, GridWidth - 4
	// the west lever stands at the yard's left end, on the scouts' row
	leverCol uint32 = 1

	// pulse is how long a wire stays on once its lever is pulled or its plate let go.
	pulse = 2 * time.Second
	// pulling is how long a scout pulls the lever beside it once told to.
	pulling = time.Second / 4
	// hasteHeld is how long the scouts go twice as fast.
	hasteHeld = 3 * time.Second
)

// rows are where the wanderers walk to and fro, each across both strips.
var rows = []uint32{2, 5, 8, 11}

// =========================== Game ===========================

// Demo is the wire demo — exactly one Stage (mainStage below).
type Demo struct{ stage *mainStage }

var _ game.Game = (*Demo)(nil)

func NewDemo() *Demo { return &Demo{stage: newStage()} }

func (d *Demo) Props() game.Props {
	return game.Props{
		Title:       "gram — wires: a lever, a plate and a switch",
		ScreenWidth: ScreenWidth, ScreenHeight: ScreenHeight, Resizable: true,
		TargetTPS: TPS,
	}
}

func (d *Demo) Stages() (map[string]game.Stage, string) {
	return map[string]game.Stage{d.stage.Name(): d.stage}, d.stage.Name()
}

// =========================== Stage ===========================

// unitRow is the row every kind spawns from: where it starts and, for a wanderer, the other end of
// its walk.
type unitRow struct{ start, to cell.ID }

type mainStage struct {
	game.Stage // defined a section at a time: newStage

	world     *world.Plugin
	board     *board.Plugin
	nav       *navigation.Plugin
	collision *collision.Plugin
	selection *selection.Plugin
	players   *players.Plugin
	player    *players.Player // the one at this keyboard: the scouts and the porters are its
	brd       *board.Board

	hasteSprite render.SpriteID
}

// newStage defines the game a section at a time, each building on those before it.
func newStage() *mainStage {
	s := &mainStage{}
	s.Stage = stage.New("wire-demo").
		Plugins(s.usePlugins).
		Players(s.definePlayer).
		Cells(s.defineCells).
		Effects(s.defineEffects).
		Rules(s.defineRoles).
		Commands(s.defineCommands).
		Kinds(s.defineKinds).
		Controls(s.bindKeys).
		Scenes(s.defineScenes).
		Layout(s.layOut).
		Units(s.placeUnits).
		Update(s.update)
	return s
}

func (s *mainStage) usePlugins(ctx game.Initializer) error {
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

func (s *mainStage) definePlayer() error {
	s.player = s.players.Local("player")
	return s.player.Bind(s.players.Defaults()...)
}

func (s *mainStage) defineCells() {
	s.board.CellKinds().Create(
		cell.Kind{Name: cell.Named(GrassCell), Cost: 1, Allows: cell.Land},
		cell.Kind{Name: cell.Named(BoardsCell), Cost: 1, Allows: cell.Land}, // a trapdoor shut
		cell.Kind{Name: cell.Named(PitCell), Cost: 1},                       // holds nobody
		cell.Kind{Name: cell.Named(PlateCell), Cost: 1, Allows: cell.Land},
		cell.Kind{Name: cell.Named(LeverCell), Cost: 1, Allows: cell.Land},
		cell.Kind{Name: cell.Named(FenceCell), Cost: 1, Solid: true},
		cell.Kind{Name: cell.Named(GateCell), Cost: 1, Solid: true},          // the gate shut
		cell.Kind{Name: cell.Named(GatewayCell), Cost: 1, Allows: cell.Land}, // the gate open
	)
}

func (s *mainStage) defineEffects() {
	pit, _ := s.board.CellKinds().Get(PitCell)
	gateway, _ := s.board.CellKinds().Get(GatewayCell)
	fx := s.world.Effects()
	fx.Define(OpenEf, effect.Spec{effect.Lasts(pulse), effect.Alter(func(g *cell.Ground) { g.Kind = pit })})
	fx.Define(AjarEf, effect.Spec{effect.Alter(func(g *cell.Ground) { g.Kind = gateway })})
	s.hasteSprite = s.world.Kinds().NewSprite()
	hasteSprite := s.hasteSprite
	fx.Define(HasteEf, effect.Spec{
		effect.Lasts(hasteHeld),
		effect.Alter(func(st *steering.Steering) { st.MaxSpeed, st.Accel = st.MaxSpeed*2, st.Accel*2 }),
		effect.Alter(func(a *world.Appearance) { a.SpriteID = hasteSprite }),
	})
	fx.Define(PullEf, effect.Spec{effect.Lasts(pulling)})
}

func (s *mainStage) defineRoles() {
	s.world.Roles().Define(PlateRole,
		rule.Then[cell.Now]("press", rule.All, rule.If(cell.Now.Stood, rule.Trigger())))
	s.world.Roles().Define(LeverRole) // does nothing of its own: a handy unit beside it pulls it
	s.world.Roles().Define(HastyRole)
	s.world.Roles().Define(HandyRole,
		rule.Then[unit.Standing]("pull the lever beside", rule.All,
			rule.Under(s.world.Effects().Named(PullEf), rule.Around(1, rule.Playing(s.world.Roles().Named(LeverRole), rule.Trigger())))))
	s.world.Roles().Define(MortalRole,
		rule.Then[unit.Standing]("fall in", rule.All, rule.If(unit.Standing.Fallen, rule.Order(world.Despawn{}))))
	// the cells laid as a plate and as a lever play those roles
	s.board.Plays(PlateCell, s.world.Roles().Named(PlateRole))
	s.board.Plays(LeverCell, s.world.Roles().Named(LeverRole))
}

func (s *mainStage) defineCommands() {
	fx, roles, cmds := s.world.Effects(), s.world.Roles(), s.world.Commands()
	cmds.Define(OpenWestCmd, rule.Cast(fx.Named(OpenEf)).On(entity.Group("west trapdoors")).By(entity.Named("west lever")))
	cmds.Define(OpenEastCmd, rule.Cast(fx.Named(OpenEf)).On(entity.Group("east trapdoors")).By(entity.Named("plate")))
	cmds.Define(FlipTheGateCmd, rule.Toggle(fx.Named(AjarEf)).On(entity.Group("gate")))
	cmds.Define(HastenCmd, rule.Cast(fx.Named(HasteEf)).On(s.selection.Selected(roles.Named(HastyRole))))
	cmds.Define(ReachCmd, rule.Cast(fx.Named(PullEf)).On(s.selection.Selected(roles.Named(HandyRole))))
}

func (s *mainStage) bindKeys() error {
	return s.player.Bind(
		control.Give(control.KeyPress{Key: control.Key1}, "Pull the west lever: its trapdoors open for a while", s.world.Commands().Named(OpenWestCmd)),
		control.Give(control.KeyPress{Key: control.KeyG}, "Flip the gate's switch: open, or shut", s.world.Commands().Named(FlipTheGateCmd)),
		control.Give(control.KeyPress{Key: control.KeyJ}, "Hasten the selected scouts", s.world.Commands().Named(HastenCmd)),
		control.Give(control.KeyPress{Key: control.KeyU}, "Pull the lever beside the selected scouts", s.world.Commands().Named(ReachCmd)),
	)
}

func (s *mainStage) defineScenes() []game.Scene {
	main := &mainScene{stage: s}
	return []game.Scene{main}
}

func (s *mainStage) defineKinds() {
	units := board.NewUnits[unitRow](s.board, board.Shape{Size: EntitySize}, func(u unitRow) geom.Vec { return s.brd.CellCenter(u.start) })
	profile := steering.Steering{MaxSpeed: UnitSpeed, Accel: UnitSpeed * 2, V0: UnitSpeed / 2, TurnRate: 0.15}
	laden := steering.Steering{MaxSpeed: UnitSpeed * 3 / 4, Accel: UnitSpeed, V0: UnitSpeed / 4, TurnRate: 0.1}
	land := unit.Mover{Domain: cell.Land}
	units.Define(ScoutKind, land, profile, rule.Plays(s.world.Roles().Named(HastyRole), s.world.Roles().Named(HandyRole), s.world.Roles().Named(MortalRole)))
	units.Define(PorterKind, land, laden, rule.Plays(s.world.Roles().Named(MortalRole)))
	// a wanderer walks to the other end of its row and back, a second's rest at each end
	units.Define(WandererKind, land, profile, rule.Plays(s.world.Roles().Named(MortalRole)),
		comp.Load(func(u unitRow) navigation.MoveOrder { return navigation.Patrol(time.Second, u.to, u.start) }))
}

func (s *mainStage) cellAt(x, y uint32) cell.ID { c, _ := s.brd.CellIndex(x, y); return c }

func (s *mainStage) layOut() {
	var cells []cell.Entry
	strips := []struct {
		group string
		left  uint32
	}{{"west trapdoors", westLeft}, {"east trapdoors", eastLeft}}
	for _, strip := range strips {
		for y := stripTop; y <= stripBottom; y++ {
			for x := strip.left; x <= strip.left+1; x++ {
				cells = append(cells, cell.Entry{Kind: BoardsCell, Cell: s.cellAt(x, y), Group: strip.group})
			}
		}
	}
	for x := range uint32(GridWidth) {
		if x == gateLeft || x == gateLeft+1 {
			cells = append(cells, cell.Entry{Kind: GateCell, Cell: s.cellAt(x, fenceRow), Group: "gate"})
		} else {
			cells = append(cells, cell.Entry{Kind: FenceCell, Cell: s.cellAt(x, fenceRow)})
		}
	}
	cells = append(cells, cell.Entry{Kind: PlateCell, Cell: s.cellAt(plateCol, yardRow), Name: "plate"})
	cells = append(cells, cell.Entry{Kind: LeverCell, Cell: s.cellAt(leverCol, yardRow), Name: "west lever"})
	s.board.Seed(board.Layout{Default: GrassCell, Cells: cells})
}

func (s *mainStage) placeUnits() {
	mine := []any{players.Give{To: s.player.ID}, selection.Allow{}}
	for i := range uint32(3) {
		s.world.Seed(kind.Named[unitRow](s.world.Kinds(), ScoutKind).Entry(unitRow{start: s.cellAt(3+2*i, yardRow)}).Told(mine...))
	}
	for i := range uint32(2) {
		s.world.Seed(kind.Named[unitRow](s.world.Kinds(), PorterKind).Entry(unitRow{start: s.cellAt(4+2*i, yardRow+1)}).Told(mine...))
	}
	for _, row := range rows {
		s.world.Seed(kind.Named[unitRow](s.world.Kinds(), WandererKind).Entry(unitRow{start: s.cellAt(2, row), to: s.cellAt(GridWidth-3, row)}))
	}
}

func (s *mainStage) update(ctx goke.RunCtx, d time.Duration) {
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
	stage *mainStage
}

var _ game.Scene = (*mainScene)(nil)

func (m *mainScene) Name() string { return "main" }

func (m *mainScene) Layers() []render.Layer {
	s := m.stage

	worldAtlas := render.NewAtlas()
	worldAtlas.RegisterAt(kind.Named[unitRow](s.world.Kinds(), ScoutKind).SpriteID(), EntitySize, render.Solid(color.RGBA{R: 90, G: 140, B: 230, A: 255}))
	worldAtlas.RegisterAt(s.hasteSprite, EntitySize, render.Solid(color.RGBA{R: 170, G: 220, B: 255, A: 255}))
	worldAtlas.RegisterAt(kind.Named[unitRow](s.world.Kinds(), PorterKind).SpriteID(), EntitySize, render.Solid(color.RGBA{R: 150, G: 110, B: 200, A: 255}))
	worldAtlas.RegisterAt(kind.Named[unitRow](s.world.Kinds(), WandererKind).SpriteID(), EntitySize, render.Diamond(color.RGBA{R: 220, G: 150, B: 60, A: 255}))
	worldAtlas.Close()
	s.world.WithRenderer(worldAtlas)

	kinds := s.board.CellKinds()
	boardAtlas := render.NewAtlas()
	for name, c := range map[string]color.RGBA{
		GrassCell:   {R: 60, G: 95, B: 60, A: 255},
		BoardsCell:  {R: 120, G: 90, B: 55, A: 255},
		PitCell:     {R: 15, G: 12, B: 20, A: 255},
		PlateCell:   {R: 160, G: 160, B: 170, A: 255},
		LeverCell:   {R: 200, G: 170, B: 60, A: 255},
		FenceCell:   {R: 85, G: 60, B: 40, A: 255},
		GateCell:    {R: 70, G: 75, B: 90, A: 255},
		GatewayCell: {R: 150, G: 130, B: 95, A: 255},
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
	return m.stage.players.Viewports(screen)
}

func (m *mainScene) HandleEvents(events *control.InputEvents, runtime game.Runtime, composition game.Composition) {
	m.stage.players.Handle(events, runtime, composition)
}

func (m *mainScene) Focusable() bool { return true }
