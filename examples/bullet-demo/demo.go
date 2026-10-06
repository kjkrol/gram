// Command bullet-demo is a soldier on WSAD who shoots: F fires a round the way the soldier
// faces — four across, ten cells a second, flying over the low wall and into the high one — and
// whoever stands on its path is wounded, pale and slow for a while, gone if wounded again. G
// throws a grenade where the cursor is, in an arc over the high wall: it lies with its fuse
// burning, a spark on it, and bursts, wounding everyone within two cells and a half and taking
// whoever stands within one. The wanderers walk their rounds on the far side. Everything the
// shots do is rules and effects; the plugin only flies them.
package main

import (
	"image/color"
	"slices"
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
	"github.com/kjkrol/gram/plugins/bullet"
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
	MaxEntCount  = 64 // the units and the shots in the air; cell entities do not count

	wallColumn uint32 = 7 // the high wall north of the road, the low one south of it
	roadRow    uint32 = 8

	fuseLength  = 2 * time.Second
	woundLasts  = 5 * time.Second
	blastRadius = 2.5 * CellSize // a burst wounds within it
	blastKills  = CellSize       // and takes whoever stands within this
)

// =========================== Game ===========================

// Demo is the bullet demo — exactly one Stage (arena below).
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
		Title:       "gram — a soldier shoots",
		ScreenWidth: ScreenWidth, ScreenHeight: ScreenHeight, Resizable: true,
		TargetTPS: TPS,
	}
}

func (d *Demo) Stages() (map[string]game.Stage, string) {
	return map[string]game.Stage{d.stage.Name(): d.stage}, d.stage.Name()
}

// =========================== Stage ===========================

// unitRow is the row every unit spawns from: where it starts and, a wanderer, where it walks to.
type unitRow struct{ start, to cell.ID }

type arena struct {
	world     *world.Plugin
	board     *board.Plugin
	nav       *navigation.Plugin
	collision *collision.Plugin
	selection *selection.Plugin
	bullet    *bullet.Plugin
	players   *players.Plugin
	player    *players.Player // the one at this keyboard: the soldier is its
	wild      *players.Player // whose the wanderers are
	brd       *board.Board
	effects   *effect.Effects

	ammo *bullet.Shots // the kinds of shots, by name
}

// newStage defines the game a section at a time, each building on those before it.
// newArena makes the arena — the collector of the stage's plugins, which every
// section builds on — and defines the stage on it, a section at a time.
func newArena() (*arena, game.Stage) {
	s := &arena{}
	return s, stage.New("bullet-demo").
		Plugins(s.usePlugins).
		Players(s.definePlayers).
		Effects(s.defineEffects).
		Rules(s.defineRules).
		Cells(s.defineCells).
		Kinds(s.defineKinds).
		Controls(s.bindKeys).
		Looks(s.defineLooks).
		Scenes(s.defineScenes).
		Layout(s.layOut).
		Units(s.placeUnits).
		Update(s.update)
}

func (s *arena) usePlugins(ctx game.Initializer) error {
	s.world = ctx.UseWorld(world.Config{
		Space:    world.SpaceCfg{Width: ScreenWidth, Height: ScreenHeight},
		Entities: world.EntitiesCfg{MaxCount: MaxEntCount, MinSize: 4, MaxSize: EntitySize},
		Heights:  true,
	})
	grid := grid.DefaultGrids{}.Square(GridWidth, GridHeight, CellSize)
	s.effects = s.world.Effects()
	s.collision = collision.NewPlugin(s.world)
	s.board = board.NewPlugin(grid, &cell.SingleOccupancy{}, s.world).WithCollision(s.collision)
	s.brd = s.board.Res.Logic.Board
	s.selection = selection.NewPlugin(s.world)
	s.nav = navigation.NewPlugin(s.board, s.world, s.selection).WithCollision(s.collision)
	s.bullet = bullet.NewPlugin(s.world, s.selection).WithGround(s.board.Heights)
	s.players = players.NewPlugin(s.world, s.board, s.selection, s.nav, s.bullet)
	for _, p := range []plugin.Plugin{s.collision, s.board, s.selection, s.nav, s.bullet, s.players} {
		if err := ctx.Use(p); err != nil {
			return err
		}
	}
	return nil
}

func (s *arena) definePlayers() error {
	s.player = s.players.Local("player")
	s.wild = s.players.Add("wild")
	taken := []control.Trigger{
		control.KeyHeld{Key: control.KeyW}, control.KeyHeld{Key: control.KeyS},
		control.KeyHeld{Key: control.KeyA}, control.KeyHeld{Key: control.KeyD},
	}
	var bindings []control.Binding
	for _, b := range s.players.Defaults() {
		if !slices.Contains(taken, b.Trigger) {
			bindings = append(bindings, b)
		}
	}
	return s.player.Bind(bindings...)
}

func (s *arena) defineCells() {
	kinds := s.board.CellKinds()
	kinds.Define(GrassCell, cell.Kind{Cost: 2, Allows: cell.Land})
	kinds.Define(RoadCell, cell.Kind{Cost: 1, Allows: cell.Land})
	kinds.Define(WaterCell, cell.Kind{Cost: 1, Allows: cell.Water})
	kinds.Define(WallCell, cell.Kind{Cost: 1, Solid: true, Height: 30})
	kinds.Define(LowWallCell, cell.Kind{Cost: 1, Solid: true, Height: 10})
}

func (s *arena) defineEffects() {
	s.effects.Define(WoundedEf, effect.Spec{ // how the wounded look is the scene's: Under in its Layers
		effect.Lasts(woundLasts),
		effect.Alter(func(st *steering.Steering) { st.MaxSpeed /= 2 }),
	})
	s.effects.Define(BangEf, effect.Spec{effect.Lasts(time.Second / TPS)})
	s.effects.Define(FuseEf, effect.Spec{effect.Lasts(fuseLength), effect.Then(s.effects.Named(BangEf))})
}

func (s *arena) defineRules() {
	s.world.Roles().Define(MortalRole)
	s.world.Roles().Define(RoundRole,
		rule.Then[collision.Meeting]("shot", rule.Other(s.world.Roles().Named(MortalRole)),
			rule.ForOther(rule.OneOf(rule.Under(s.effects.Named(WoundedEf), rule.Order(world.Despawn{})), rule.Apply(s.effects.Named(WoundedEf))))))
	s.world.Roles().Define(GrenadeRole,
		rule.Then[bullet.Landing]("fuse", rule.All, rule.Apply(s.effects.Named(FuseEf))),
		rule.Then[bullet.Blast]("blast", rule.Other(s.world.Roles().Named(MortalRole)), rule.ForOther(rule.OneOf(
			rule.If(func(b bullet.Blast) bool { return b.Distance < blastKills }, rule.Order(world.Despawn{})),
			rule.Apply(s.effects.Named(WoundedEf)),
		))),
		rule.Then[bullet.Resting]("bang", rule.Self(s.effects.Named(BangEf).Mark()), rule.Order(bullet.Burst{Radius: blastRadius})))
}

func (s *arena) bindKeys() error {
	return s.player.Bind(append(navigation.DriveBindings(),
		control.Give(control.KeyPress{Key: control.KeyF}, "Fire a round the way the soldier faces", bullet.Shoot{Ammo: s.ammo.Named(RoundKind)}),
		control.Command(control.KeyPress{Key: control.KeyG}, "Throw a grenade at the cursor",
			func(c control.Context) (bullet.Shoot, bool) {
				return bullet.Shoot{Ammo: s.ammo.Named(GrenadeKind), At: c.World(c.Cursor), Targeted: true}, true
			}),
	)...)
}

func (s *arena) defineLooks() error {
	return s.world.Draw(s.ammo.Facing(RoundKind, 16)) // a round drawn the way it flies
}

func (s *arena) defineScenes() []game.Scene {
	main := &mainScene{arena: s}
	return []game.Scene{main}
}

func (s *arena) defineKinds() {
	units := board.NewUnits[unitRow](s.board, board.Shape{Size: EntitySize, Height: EntitySize}, func(u unitRow) geom.Vec { return s.brd.CellCenter(u.start) })
	profile := steering.Steering{MaxSpeed: UnitSpeed, Accel: UnitSpeed * 2, Brake: UnitSpeed * 4, V0: UnitSpeed / 2, TurnRate: 0.15}
	// The ammo: a round flies ten cells a second over ten cells, spent as it lands; a grenade is
	// thrown in an arc and lies where it comes down.
	s.ammo = bullet.NewShots(s.world)
	s.ammo.Define(RoundKind, bullet.Body{Size: 4, Speed: 10 * CellSize, Range: 10 * CellSize}, rule.Plays(s.world.Roles().Named(RoundRole)))
	s.ammo.Define(GrenadeKind, bullet.Body{Size: 8, Speed: 5 * CellSize, Range: 9 * CellSize, Gravity: 240, Lands: true}, rule.Plays(s.world.Roles().Named(GrenadeRole)))
	mortal := rule.Plays(s.world.Roles().Named(MortalRole))
	units.Define(SoldierKind, unit.Mover{Domain: cell.Land}, profile, mortal,
		comp.Const(world.Velocity{Dir: geom.NewVec(1, 0)}), comp.Const(world.Eye{Height: 16}))
	units.Define(WandererKind, unit.Mover{Domain: cell.Land}, profile, mortal,
		comp.Load(func(u unitRow) navigation.MoveOrder { return navigation.Patrol(time.Second, u.to, u.start) }))
}

func (s *arena) layOut() {
	at := s.cellAt
	var cells []cell.Entry
	for x := uint32(1); x < GridWidth-1; x++ {
		cells = append(cells, cell.Entry{Kind: RoadCell, Cell: at(x, roadRow)})
	}
	for y := uint32(12); y <= 14; y++ {
		for x := uint32(2); x <= 4; x++ {
			cells = append(cells, cell.Entry{Kind: WaterCell, Cell: at(x, y)})
		}
	}
	for y := uint32(2); y <= 6; y++ {
		cells = append(cells, cell.Entry{Kind: WallCell, Cell: at(wallColumn, y)})
	}
	for y := uint32(10); y <= 14; y++ {
		cells = append(cells, cell.Entry{Kind: LowWallCell, Cell: at(wallColumn, y)})
	}
	s.board.Seed(board.Layout{Default: GrassCell, Cells: cells})
}

func (s *arena) placeUnits() {
	soldierKind := kind.Named[unitRow](s.world.Kinds(), SoldierKind)
	wandererKind := kind.Named[unitRow](s.world.Kinds(), WandererKind)
	at := s.cellAt
	s.world.Seed(
		soldierKind.Entry(unitRow{start: at(3, roadRow)}).Told(players.Give{To: s.player.ID}, selection.Allow{Selected: true}),
		wandererKind.Entry(unitRow{start: at(8, roadRow), to: at(13, roadRow)}).Told(players.Give{To: s.wild.ID}),
		wandererKind.Entry(unitRow{start: at(9, 12), to: at(14, 12)}).Told(players.Give{To: s.wild.ID}),
		wandererKind.Entry(unitRow{start: at(9, 4), to: at(12, 4)}).Told(players.Give{To: s.wild.ID}),
	)
}

func (s *arena) cellAt(x, y uint32) cell.ID { c := s.brd.CellIndex(x, y); return c }

func (s *arena) update(ctx goke.RunCtx, d time.Duration) {
	s.bullet.RunPlan(ctx, d)
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

// The scene's colours: the units, the shots, the wounds' pale and the burst's spark, the ground.
var (
	soldierColor  = color.RGBA{R: 90, G: 140, B: 230, A: 255}
	wandererColor = color.RGBA{R: 220, G: 90, B: 90, A: 255}
	paleColor     = color.RGBA{R: 235, G: 200, B: 200, A: 255}
	roundColor    = color.RGBA{R: 255, G: 230, B: 80, A: 255}
	grenadeColor  = color.RGBA{R: 50, G: 80, B: 50, A: 255}
	sparkColor    = color.RGBA{R: 255, G: 250, B: 200, A: 255}

	cellColors = map[string]color.RGBA{
		GrassCell:   {R: 60, G: 95, B: 60, A: 255},
		RoadCell:    {R: 150, G: 130, B: 80, A: 255},
		WaterCell:   {R: 40, G: 90, B: 170, A: 255},
		WallCell:    {R: 90, G: 90, B: 100, A: 255},
		LowWallCell: {R: 150, G: 150, B: 160, A: 255},
	}
)

func (m *mainScene) Layers() []render.Layer {
	s := m.arena
	soldierKind := kind.Named[unitRow](s.world.Kinds(), SoldierKind)
	wandererKind := kind.Named[unitRow](s.world.Kinds(), WandererKind)

	wounded := s.effects.Named(WoundedEf)
	fuse := s.effects.Named(FuseEf)

	worldAtlas := render.NewAtlas()
	worldAtlas.Add(soldierKind, EntitySize, render.Diamond(soldierColor)).
		Under(wounded, render.Diamond(paleColor)) // the soldier gone pale
	worldAtlas.Add(wandererKind, EntitySize, render.Solid(wandererColor)).
		Under(wounded, render.Solid(paleColor)) // the wanderer too
	worldAtlas.Add(s.ammo.Named(RoundKind), 8, render.Dot(2, roundColor)).
		Facing(func(angleDeg float64) render.SpriteDrawer { return render.Arrow(angleDeg, 2, roundColor) })
	worldAtlas.Add(s.ammo.Named(GrenadeKind), 8, render.Dot(3, grenadeColor)).
		Under(fuse, func(dst *render.Canvas, size int) { // the grenade with its fuse sparking
			render.Dot(3, grenadeColor)(dst, size)
			render.Diamond(sparkColor)(dst, size)
		})
	worldAtlas.Close()
	s.world.WithRenderer(worldAtlas)

	boardAtlas := s.board.NewAtlas(CellSize)
	for name, c := range cellColors {
		boardAtlas.Add(name, render.Solid(c))
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
