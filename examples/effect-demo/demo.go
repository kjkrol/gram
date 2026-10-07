package main

import (
	"image/color"
	"log"
	"math"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
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
	"github.com/kjkrol/gram/plugins/cameras"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/driving"
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

type arena struct {
	world     *world.Plugin
	board     *board.Plugin
	nav       *navigation.Plugin
	driving   *driving.Plugin
	collision *collision.Plugin
	selection *selection.Plugin
	players   *players.Plugin
	cameras   *cameras.Plugin
	player    *players.Player
}

// newArena makes the arena — the collector of the stage's plugins, which every
// section builds on — and defines the stage on it, a section at a time.
func newArena() (*arena, game.Stage) {
	s := &arena{}
	return s, stage.New("effect-demo").
		Plugins(s.usePlugins).
		Players(s.definePlayer).
		Effects(s.defineEffects).
		Rules(s.defineRoles).
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
	s.collision = collision.NewPlugin(s.world)
	grid := grid.DefaultGrids{}.Square(GridWidth, GridHeight, CellSize)
	s.board = board.NewPlugin(grid, &cell.SingleOccupancy{}, s.world).WithCollision(s.collision).WithLog(log.Default())
	s.selection = selection.NewPlugin(s.world)
	s.driving = driving.NewPlugin(s.world, s.selection).WithGround(s.board)
	s.nav = navigation.NewPlugin(s.board, s.world, s.selection, s.driving).WithCollision(s.collision)
	s.cameras = cameras.NewPlugin(s.world)
	s.players = players.NewPlugin(s.world, s.cameras, s.board, s.selection, s.nav, s.driving)
	for _, p := range []plugin.Plugin{s.collision, s.board, s.selection, s.nav, s.driving, s.cameras, s.players} {
		if err := ctx.Use(p); err != nil {
			return err
		}
	}
	return nil
}

func (s *arena) definePlayer() error {
	s.player = s.players.Local("player", s.cameras.New(cameras.TopDown(), camera.Config{}))
	return s.player.Bind(s.players.Defaults()...)
}

func (s *arena) defineCommands() {
	s.world.Commands().Define(FreezeCmd,
		rule.Cast(s.world.Effects().Named(FrozenEf)).On(s.selection.Pointed()).For(3*time.Second))
}

func (s *arena) bindKeys() error {
	return s.player.Bind(control.Give(control.KeyPress{Key: control.KeyF}, "Freeze the one pointed at",
		s.world.Commands().Named(FreezeCmd)))
}

func (s *arena) defineCells() {
	kinds := s.board.CellKinds()
	kinds.Define(GrassCell, cell.Kind{Cost: 2, Allows: cell.Land})
	kinds.Define(WaterCell, cell.Kind{Cost: 1, Allows: cell.Water})
}

func (s *arena) defineEffects() {
	effects := s.world.Effects()
	effects.Define(FrostEf, effect.Spec{
		effect.Described("Land under snow: slower, and the witch's own ground."),
		effect.Lasts(5 * time.Second),
		effect.Alter(func(g *cell.Ground) {
			g.Kind.Allows |= Frost
			g.Kind.Cost++
			g.Kind = g.Kind.Costing(Frost, 0.5)
		}),
	})
	effects.Define(IcedEf, effect.Spec{
		effect.Described("Water under ice: walked over, not sailed."),
		effect.Lasts(5 * time.Second),
		effect.Alter(func(g *cell.Ground) {
			g.Kind.Allows = cell.Land | Frost
			g.Kind.Cost = 2
			g.Kind = g.Kind.Costing(Frost, 0.5)
		}),
	})
	effects.Define(FrozenEf, effect.Spec{
		effect.Described("Frozen stiff: stood dead still, an immovable block."),
		effect.Alter(func(p *collision.Physics) { p.Mass = math.Inf(1) }),
		effect.Alter(func(st *steering.Steering) { st.Halted = true }),
	})
	effects.Define(SlipEf, effect.Spec{
		effect.Described("On the ice: hardly any braking, whoever moves slides on."),
		effect.Alter(func(st *steering.Steering) { st.Brake = st.Accel / 8 }),
	})
}

func (s *arena) defineRoles() {
	effects := s.world.Effects()
	frost, iced := effects.Named(FrostEf), effects.Named(IcedEf)
	frozen, slip := effects.Named(FrozenEf), effects.Named(SlipEf)

	roles := s.world.Roles()
	roles.Define(LakeRole) // the lake's cells play it, in the Layout: where the witch's winter is ice
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

func (s *arena) defineKinds() {
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

func (s *arena) defineScenes() []game.Scene {
	main := &mainScene{arena: s}
	return []game.Scene{main}
}

// The lake: where the water lies, and where the boat sails.
const (
	lakeLeft, lakeRight uint32 = 8, 15
	lakeTop, lakeBottom uint32 = 4, 11
)

func (s *arena) layOut() {
	brd := s.board.Res.Logic.Board
	water := s.board.CellKinds().Named(WaterCell)
	lake := s.world.Roles().Named(LakeRole)
	var cells []cell.Entry
	for y := lakeTop; y <= lakeBottom; y++ {
		for x := lakeLeft; x <= lakeRight; x++ {
			cells = append(cells, water.Entry(brd.CellIndex(x, y)).Plays(lake))
		}
	}
	s.board.Seed(board.Layout{Default: GrassCell, Cells: cells})
}
func (s *arena) placeUnits() {
	witchKind := kind.Named[unitRow](s.world.Kinds(), WitchKind)
	walkerKind := kind.Named[unitRow](s.world.Kinds(), WalkerKind)
	boatKind := kind.Named[unitRow](s.world.Kinds(), BoatKind)
	brd := s.board.Res.Logic.Board
	player := []any{players.Give{To: s.player.ID}, selection.Allow{}}
	s.world.Seed(
		witchKind.Entry(unitRow{start: brd.CellIndex(2, 8), target: brd.CellIndex(GridWidth-3, 8)}).Told(player...),
		walkerKind.Entry(unitRow{start: brd.CellIndex(2, 10)}).Told(player...),
		boatKind.Entry(unitRow{start: brd.CellIndex(lakeRight, 8), target: brd.CellIndex(lakeLeft, 8)}).Told(player...),
	)
}

func (s *arena) update(ctx goke.RunCtx, d time.Duration) {
	s.world.RunPlan(ctx, d)
	s.collision.RunPlan(ctx, d)
	s.board.RunPlan(ctx, d)
	s.nav.RunPlan(ctx, d)
	s.driving.RunPlan(ctx, d)
	s.selection.RunPlan(ctx, d)
	s.cameras.RunPlan(ctx, d)
	s.players.RunPlan(ctx, d)
	ctx.Sync()
}

// =========================== Scene ===========================

type mainScene struct {
	arena *arena
}

var _ game.Scene = (*mainScene)(nil)

func (m *mainScene) Name() string { return "main" }

// The scene's colours: the units, each with its look under frozen, and the ground with what
// covers it.
var (
	witchColor        = color.RGBA{R: 200, G: 230, B: 255, A: 255}
	witchFrozenColor  = color.RGBA{R: 240, G: 248, B: 255, A: 255}
	walkerColor       = color.RGBA{R: 220, G: 90, B: 90, A: 255}
	walkerFrozenColor = color.RGBA{R: 235, G: 175, B: 175, A: 255}
	boatColor         = color.RGBA{R: 140, G: 90, B: 40, A: 255}
	boatRimColor      = color.RGBA{R: 190, G: 220, B: 245, A: 255}
	grassColor        = color.RGBA{R: 60, G: 95, B: 60, A: 255}
	snowColor         = color.RGBA{R: 235, G: 240, B: 245, A: 255}
	waterColor        = color.RGBA{R: 40, G: 90, B: 170, A: 255}
	iceColor          = color.RGBA{R: 170, G: 215, B: 240, A: 255}
)

func (m *mainScene) Layers() []render.Layer {
	s := m.arena

	witchKind := kind.Named[unitRow](s.world.Kinds(), WitchKind)
	walkerKind := kind.Named[unitRow](s.world.Kinds(), WalkerKind)
	boatKind := kind.Named[unitRow](s.world.Kinds(), BoatKind)
	effects := s.world.Effects()
	frozen := effects.Named(FrozenEf)

	worldAtlas := render.NewAtlas()
	worldAtlas.Add(witchKind, EntitySize, render.Diamond(witchColor)).
		Under(frozen, render.Diamond(witchFrozenColor)) // the witch gone white
	worldAtlas.Add(walkerKind, EntitySize, render.Solid(walkerColor)).
		Under(frozen, render.Solid(walkerFrozenColor)) // the walker rimed
	worldAtlas.Add(boatKind, EntitySize, render.Solid(boatColor)).
		Under(frozen, func(dst *render.Canvas, size int) { // the boat in a rim of ice
			render.Solid(boatColor)(dst, size)
			render.Border(boatRimColor)(dst, size)
		})
	worldAtlas.Close()
	s.world.WithRenderer(worldAtlas)

	boardAtlas := s.board.NewAtlas(CellSize)
	boardAtlas.Add(GrassCell, render.Solid(grassColor)).
		Under(effects.Named(FrostEf), render.Solid(snowColor)) // snow over the land
	boardAtlas.Add(WaterCell, render.Solid(waterColor)).
		Under(effects.Named(IcedEf), render.Solid(iceColor)) // ice over the lake
	boardAtlas.Close()
	s.board.WithRenderer(boardAtlas)

	s.nav.WithRenderer(nil)
	s.selection.WithRenderer(nil)

	return []render.Layer{render.NewComposer(s.board.Renderer(), s.world.Renderer(), s.selection.Renderer(), s.nav.Renderer())}
}

func (m *mainScene) Viewports(screen geom.AABB) []render.Viewport {
	return m.arena.players.Viewports(screen)
}

func (m *mainScene) HandleEvents(events *control.InputEvents, runtime game.Runtime, composition game.Composition) {
	m.arena.players.Handle(events, runtime, composition)
}

func (m *mainScene) Focusable() bool { return true }
