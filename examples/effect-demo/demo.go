package main

import (
	"image/color"
	"log"
	"math"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
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
	EntitySize   = 22
	UnitSpeed    = CellSize * 2
	MaxEntCount  = 16

	Frost = cell.Domain(1 << 3)
)

// =========================== Game ===========================

type Demo struct{ stage *mainStage }

var _ game.Game = (*Demo)(nil)

// TODO: zmienilbym sposob opisywania stage. Juz teraz korzystamy z DSL.
// IMO lepiej pluginy zgrupowac w arene, z ktorej korzystalby stage.
// wowczas tutaj byloby
// definicja areny (ktora jest "zbierakiem na pluginy")
// stage: stage.New("effect-demo").
// Plugins(a.usePlugins).
// Players(a.definePlayer).
// Cells(a.defineCells).
// Effects(a.defineEffects).
// Rules(a.defineRoles).
// itd
func NewDemo() *Demo { return &Demo{stage: newStage()} }

func (d *Demo) Props() game.Props {
	return game.Props{
		Title:       "gram — an ice witch writes the terrain",
		ScreenWidth: ScreenWidth, ScreenHeight: ScreenHeight, Resizable: true,
		TargetTPS: TPS,
	}
}

func (d *Demo) Stages() (map[string]game.Stage, string) {
	return map[string]game.Stage{d.stage.Name(): d.stage}, d.stage.Name()
}

// =========================== Stage ===========================

type unitRow struct {
	start, target cell.ID
	ordered       bool
}

type mainStage struct {
	game.Stage

	world     *world.Plugin
	board     *board.Plugin
	nav       *navigation.Plugin
	collision *collision.Plugin
	selection *selection.Plugin
	players   *players.Plugin
	player    *players.Player
}

func newStage() *mainStage {
	s := &mainStage{}
	s.Stage = stage.New("effect-demo").
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
	s.collision = collision.NewPlugin(s.world)
	grid := grid.DefaultGrids{}.Square(GridWidth, GridHeight, CellSize)
	s.board = board.NewPlugin(grid, &cell.SingleOccupancy{}, s.world).WithCollision(s.collision).WithLog(log.Default())
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

func (s *mainStage) defineCommands() {
	s.world.Commands().Define(FreezeCmd,
		rule.Cast(s.world.Effects().Named(FrozenEf)).On(s.selection.Pointed()).For(3*time.Second))
}

func (s *mainStage) bindKeys() error {
	return s.player.Bind(control.Give(control.KeyPress{Key: control.KeyF}, "Freeze the one pointed at",
		s.world.Commands().Named(FreezeCmd)))
}

func (s *mainStage) defineCells() {
	s.board.CellKinds().Create(
		cell.Kind{Name: cell.Named(GrassCell), Cost: 2, Allows: cell.Land},
		cell.Kind{Name: cell.Named(WaterCell), Cost: 1, Allows: cell.Water},
	)
}

func (s *mainStage) defineEffects() {
	effects := s.world.Effects()
	// TODO: kazdy efekt powinien miec pole description, ktore mozna wyswietlic w UI. Wtedy mozna by bylo wyswietlic tooltip z opisem efektu.
	// poza tym dzieki temun te komentarze moglyby byc usuniete, bo opis bylby w samym efekcie.
	effects.Define(FrostEf, effect.Spec{ // land under snow: slower, the witch's own
		effect.Lasts(5 * time.Second),
		effect.Alter(func(g *cell.Ground) {
			g.Kind.Allows |= Frost
			g.Kind.Cost++
			g.Kind = g.Kind.Costing(Frost, 0.5)
		}),
	})
	effects.Define(IcedEf, effect.Spec{ // water under ice: walked over, not sailed
		effect.Lasts(5 * time.Second),
		effect.Alter(func(g *cell.Ground) {
			g.Kind.Allows = cell.Land | Frost
			g.Kind.Cost = 2
			g.Kind = g.Kind.Costing(Frost, 0.5)
		}),
	})
	effects.Define(FrozenEf, effect.Spec{
		effect.Alter(func(p *collision.Physics) { p.Mass = math.Inf(1) }),
		effect.Alter(func(st *steering.Steering) { st.Halted = true }),
	})
	effects.Define(SlipEf, effect.Spec{
		effect.Alter(func(st *steering.Steering) { st.Brake = st.Accel / 8 }),
	})
}

func (s *mainStage) defineRoles() {
	effects := s.world.Effects()
	frost, iced := effects.Named(FrostEf), effects.Named(IcedEf)
	frozen, slip := effects.Named(FrozenEf), effects.Named(SlipEf)

	roles := s.world.Roles()
	roles.Define(LakeRole)
	s.board.Plays(WaterCell, roles.Named(LakeRole)) // TODO: mowielm by nie zakladać,
	//rule przez pluginy (po to usuwalismy hooks z plugins!).
	// Role powinny przyjąc wyłącznie określone kafle wody - te które tworzą jezioro, a nie wszystkie.
	roles.Define(WitchRole,
		rule.Then[unit.Standing]("freeze", rule.All, rule.Around(1, rule.OneOf(
			rule.Playing(roles.Named(LakeRole), rule.Apply(iced)),
			rule.Apply(frost),
		))),
	)
	roles.Define(MortalRole,
		rule.Then[unit.Standing]("fallen in", rule.All,
			rule.If(unit.Standing.Fallen, rule.OneOf(
				rule.If(unit.Over(iced), rule.Keep(frozen)),
				rule.Order(world.Despawn{}),
			))),
		rule.Then[unit.Standing]("on the ice", rule.All,
			rule.If(rule.Not(unit.Standing.Fallen), rule.If(unit.Over(iced), rule.Keep(slip)))),
	)
}

func (s *mainStage) defineKinds() {
	brd := s.board.Res.Logic.Board
	units := board.NewUnits[unitRow](s.board, board.Shape{Size: EntitySize}, func(u unitRow) geom.Vec { return brd.CellCenter(u.start) })
	profile := func(brake float64) steering.Steering {
		return steering.Steering{MaxSpeed: UnitSpeed, Accel: UnitSpeed * 2, Brake: brake, V0: UnitSpeed / 2, TurnRate: 0.15}
	}
	order := comp.Load(func(u unitRow) navigation.MoveOrder { return navigation.MoveOrder{Target: u.target} })
	roles := s.world.Roles()
	units.Define(WitchKind, unit.Mover{Domain: cell.Land | cell.Water | Frost}, profile(UnitSpeed*4), order, rule.Plays(roles.Named(WitchRole), roles.Named(MortalRole)))
	units.Define(WalkerKind, unit.Mover{Domain: cell.Land}, profile(UnitSpeed*4), rule.Plays(roles.Named(MortalRole)))
	units.Define(BoatKind, unit.Mover{Domain: cell.Water}, profile(UnitSpeed/4), order, rule.Plays(roles.Named(MortalRole)))
}

func (s *mainStage) defineScenes() []game.Scene {
	main := &mainScene{stage: s}
	return []game.Scene{main}
}

// The lake: where the water lies, and where the boat sails.
const (
	lakeLeft, lakeRight uint32 = 8, 15
	lakeTop, lakeBottom uint32 = 4, 11
)

func (s *mainStage) layOut() {
	var cells []cell.Entry
	for y := lakeTop; y <= lakeBottom; y++ {
		for x := lakeLeft; x <= lakeRight; x++ {
			cells = append(cells, cell.Entry{Kind: WaterCell, Cell: s.cellAt(x, y)})
		}
	}
	s.board.Seed(board.Layout{Default: GrassCell, Cells: cells})
}
func (s *mainStage) placeUnits() {
	player := []any{players.Give{To: s.player.ID}, selection.Allow{}}
	s.world.Seed(
		kind.Named[unitRow](s.world.Kinds(), WitchKind).Entry(unitRow{start: s.cellAt(2, 8), target: s.cellAt(GridWidth-3, 8)}).Told(player...),
		kind.Named[unitRow](s.world.Kinds(), WalkerKind).Entry(unitRow{start: s.cellAt(2, 10)}).Told(player...),
		kind.Named[unitRow](s.world.Kinds(), BoatKind).Entry(unitRow{start: s.cellAt(lakeRight, 8), target: s.cellAt(lakeLeft, 8)}).Told(player...),
	)
}

func (s *mainStage) cellAt(x, y uint32) cell.ID {
	c, _ := s.board.Res.Logic.Board.CellIndex(x, y)
	return c
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
	worldAtlas.RegisterAt(kind.Named[unitRow](s.world.Kinds(), WitchKind).SpriteID(), EntitySize, render.Diamond(color.RGBA{R: 200, G: 230, B: 255, A: 255}))
	worldAtlas.RegisterAt(kind.Named[unitRow](s.world.Kinds(), WalkerKind).SpriteID(), EntitySize, render.Solid(color.RGBA{R: 220, G: 90, B: 90, A: 255}))
	worldAtlas.RegisterAt(kind.Named[unitRow](s.world.Kinds(), BoatKind).SpriteID(), EntitySize, render.Solid(color.RGBA{R: 140, G: 90, B: 40, A: 255}))
	effects := s.world.Effects()
	for _, l := range []struct {
		effect string
		of     render.SpriteID
		draw   render.SpriteDrawer
	}{
		{FrozenEf, kind.Named[unitRow](s.world.Kinds(), WitchKind).SpriteID(), render.Diamond(color.RGBA{R: 240, G: 248, B: 255, A: 255})},
		{FrozenEf, kind.Named[unitRow](s.world.Kinds(), WalkerKind).SpriteID(), render.Solid(color.RGBA{R: 235, G: 175, B: 175, A: 255})},
		{FrozenEf, kind.Named[unitRow](s.world.Kinds(), BoatKind).SpriteID(), func(dst *render.Canvas, size int) {
			render.Solid(color.RGBA{R: 140, G: 90, B: 40, A: 255})(dst, size)
			render.Border(color.RGBA{R: 190, G: 220, B: 245, A: 255})(dst, size)
		}},
	} {
		worldAtlas.RegisterAt(effects.Named(l.effect).Look(l.of), EntitySize, l.draw)
	}
	worldAtlas.Close()
	s.world.WithRenderer(worldAtlas)

	// TODO: uwazam ze kafle trzeba definiowac, a potem wypelniac analogicznie do jednostek. Pozwolilobyto przypisywac wybranym kaflom efekty.
	kinds := s.board.CellKinds()
	boardAtlas := render.NewAtlas()
	for name, c := range map[string]color.RGBA{
		GrassCell: {R: 60, G: 95, B: 60, A: 255},
		WaterCell: {R: 40, G: 90, B: 170, A: 255},
	} {
		k, _ := kinds.Get(name)
		boardAtlas.RegisterAt(k.SpriteID, CellSize, render.Solid(c))
	}
	for name, c := range map[string]color.RGBA{
		FrostEf: {R: 235, G: 240, B: 245, A: 255},
		IcedEf:  {R: 170, G: 215, B: 240, A: 255},
	} {
		boardAtlas.RegisterAt(s.board.Covering(effects.Named(name)), CellSize, render.Solid(c))
	}
	boardAtlas.Close()
	s.board.WithRenderer(boardAtlas)

	s.nav.WithRenderer(nil)
	s.selection.WithRenderer(nil)

	return []render.Layer{render.NewComposer(s.board.Renderer(), s.world.Renderer(), s.selection.Renderer(), s.nav.Renderer())}
}

func (m *mainScene) Viewports(screen geom.AABB) []render.Viewport {
	return m.stage.players.Viewports(screen)
}

func (m *mainScene) HandleEvents(events *control.InputEvents, runtime game.Runtime, composition game.Composition) {
	m.stage.players.Handle(events, runtime, composition)
}

func (m *mainScene) Focusable() bool { return true }
