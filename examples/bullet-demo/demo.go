// Command bullet-demo is a soldier on WSAD who shoots: Space fires a round the way the soldier
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
	"github.com/kjkrol/gram/clock"
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

// Demo is the bullet demo — exactly one Stage (mainStage below).
type Demo struct{ stage *mainStage }

var _ game.Game = (*Demo)(nil)

func NewDemo() *Demo { return &Demo{stage: newStage()} }

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

type mainStage struct {
	game.Stage // defined a section at a time: newStage

	world     *world.Plugin
	board     *board.Plugin
	nav       *navigation.Plugin
	collision *collision.Plugin
	selection *selection.Plugin
	bullet    *bullet.Plugin
	players   *players.Plugin
	player    *players.Player // the one at this keyboard: the soldier is its
	wild      *players.Player // whose the wanderers are
	shortcuts *players.Shortcuts
	brd       *board.Board
	effects   *effect.Effects

	wounded, fuse, bang     effect.Effect
	paleSprite, sparkSprite render.SpriteID
	mortal, shot, thrown    *rule.Part // who can be wounded; a round, a grenade
	round, grenade          bullet.Ammo

	soldier, wanderer kind.Of[unitRow]
}

// newStage defines the game a section at a time, each building on those before it.
func newStage() *mainStage {
	s := &mainStage{}
	s.Stage = stage.New("bullet-demo").
		Plugins(s.usePlugins).
		Players(s.definePlayers).
		Cells(s.defineCells).
		Effects(s.defineEffects).
		Rules(s.defineRules).
		Kinds(s.defineKinds).
		Controls(s.bindKeys).
		Looks(s.defineLooks).
		Scenes(s.defineScenes).
		Layout(s.layOut).
		Units(s.placeUnits).
		Update(s.update)
	return s
}

// usePlugins makes a world with heights: the walls have a height, a round flies at the soldier's
// eye and a grenade in an arc, though nothing is drawn in relief.
func (s *mainStage) usePlugins(ctx game.Initializer) error {
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
	s.players = players.NewPlugin(s.world, s.selection, s.nav, s.bullet)
	for _, p := range []plugin.Plugin{s.collision, s.board, s.selection, s.nav, s.bullet, s.players} {
		if err := ctx.Use(p); err != nil {
			return err
		}
	}
	return nil
}

// definePlayers makes the player at the keyboard and the wild, whose the wanderers are, and binds
// the plugins' default keys but the camera's W, S, A and D, which drive the soldier here, and the
// clock's Space, which shoots.
func (s *mainStage) definePlayers() error {
	s.player = s.players.Local("player")
	s.wild = s.players.Add("wild")
	taken := []control.Trigger{
		control.KeyHeld{Key: control.KeyW}, control.KeyHeld{Key: control.KeyS},
		control.KeyHeld{Key: control.KeyA}, control.KeyHeld{Key: control.KeyD},
		control.KeyPress{Key: control.KeySpace},
	}
	var bindings []control.Binding
	for _, b := range s.players.Defaults() {
		if !slices.Contains(taken, b.Trigger) {
			bindings = append(bindings, b)
		}
	}
	return s.player.Bind(bindings...)
}

func (s *mainStage) defineCells() {
	s.board.CellKinds().Create(
		cell.Kind{Name: cell.Named("grass"), Cost: 2, Allows: cell.Land},
		cell.Kind{Name: cell.Named("road"), Cost: 1, Allows: cell.Land},
		cell.Kind{Name: cell.Named("water"), Cost: 1, Allows: cell.Water},
		cell.Kind{Name: cell.Named("wall"), Cost: 1, Solid: true, Height: 30},
		cell.Kind{Name: cell.Named("low wall"), Cost: 1, Solid: true, Height: 10},
	)
}

// defineEffects says the game's states: wounded turns a unit pale and slow; a grenade landed has
// a fuse, which bangs when it is up.
func (s *mainStage) defineEffects() {
	s.paleSprite, s.sparkSprite = s.world.Kinds().NewSprite(), s.world.Kinds().NewSprite()
	s.effects.Define("wounded", effect.Spec{
		effect.Lasts(woundLasts),
		effect.Alter(func(a *world.Appearance) { a.SpriteID = s.paleSprite }),
		effect.Alter(func(st *steering.Steering) { st.MaxSpeed /= 2 }),
	})
	s.wounded = s.effects.Named("wounded")
	s.effects.Define("bang", effect.Spec{effect.Lasts(time.Second / TPS)})
	s.bang = s.effects.Named("bang")
	s.effects.Define("fuse", effect.Spec{effect.Lasts(fuseLength), effect.Then(s.bang)})
	s.fuse = s.effects.Named("fuse")
}

// defineRules says who can be wounded and what each shot does to them: a round striking a mortal
// wounds it, or takes it if wounded already; a grenade landing lights its fuse, bangs as the fuse
// is up, and its blast takes whoever stands within a cell and wounds the rest within its radius.
func (s *mainStage) defineRules() {
	s.world.Roles().Define("mortal")
	s.mortal = s.world.Roles().Named("mortal")
	s.world.Roles().Define("round",
		rule.Then[collision.Meeting]("shot", rule.Other(s.mortal),
			rule.ForOther(rule.OneOf(rule.Under(s.wounded, rule.Order(world.Despawn{})), rule.Apply(s.wounded)))))
	s.shot = s.world.Roles().Named("round")
	s.world.Roles().Define("grenade",
		rule.Then[bullet.Landing]("fuse", rule.All, rule.Apply(s.fuse)),
		rule.Then[bullet.Blast]("blast", rule.Other(s.mortal), rule.ForOther(rule.OneOf(
			rule.If(func(b bullet.Blast) bool { return b.Distance < blastKills }, rule.Order(world.Despawn{})),
			rule.Apply(s.wounded),
		))),
		rule.Then[bullet.Resting]("bang", rule.Self(s.bang.Mark()), rule.Order(bullet.Burst{Radius: blastRadius})))
	s.thrown = s.world.Roles().Named("grenade")
}

// bindKeys gives the player the game's own keys: W, S, A and D drive the soldier, Space shoots a
// round the way it faces, G throws a grenade at the cursor; the pause goes to P.
func (s *mainStage) bindKeys() error {
	return s.player.Bind(append(navigation.DriveBindings(),
		control.Give(control.KeyPress{Key: control.KeySpace}, "Shoot a round the way the soldier faces", bullet.Shoot{Ammo: s.round}),
		control.Command(control.KeyPress{Key: control.KeyG}, "Throw a grenade at the cursor",
			func(c control.Context) (bullet.Shoot, bool) {
				return bullet.Shoot{Ammo: s.grenade, At: c.World(c.Cursor), Targeted: true}, true
			}),
		control.Give(control.KeyPress{Key: control.KeyP}, "Pause the game", clock.Pause{}),
	)...)
}

// defineLooks has a grenade with its fuse burning show a spark.
func (s *mainStage) defineLooks() error {
	return s.world.Draw(render.Over(render.Appearance{SpriteID: s.sparkSprite}, s.fuse.Mark().In))
}

func (s *mainStage) defineScenes() []game.Scene {
	main := &mainScene{stage: s}
	main.keys = players.SceneKeys{
		{Key: control.KeyK, Label: "Shortcuts; Esc closes them", Do: func(rt game.Runtime, c game.Composition) { s.shortcuts.Open(rt, c) }},
		{Key: control.KeyEscape, Shift: true, Label: "Quit", Do: func(rt game.Runtime, _ game.Composition) { rt.Quit() }},
		{Key: control.KeyB, Label: "Toggle the grid", Do: func(game.Runtime, game.Composition) { s.board.Res.Render.ToggleShowGridLines() }},
	}
	s.shortcuts = s.players.Shortcuts(main.keys)
	return []game.Scene{main, s.shortcuts}
}

// defineKinds says what this game's units are: the soldier, the player's, selected from the
// start, facing east, looking from 16 up; the wanderers the wild's, each on its round.
func (s *mainStage) defineKinds() {
	units := board.NewUnits[unitRow](s.board, board.Shape{Size: EntitySize, Height: EntitySize}, func(u unitRow) geom.Vec { return s.brd.CellCenter(u.start) })
	profile := steering.Steering{MaxSpeed: UnitSpeed, Accel: UnitSpeed * 2, Brake: UnitSpeed * 4, V0: UnitSpeed / 2, TurnRate: 0.15}
	// The ammo: a round flies ten cells a second over ten cells, spent as it lands; a grenade is
	// thrown in an arc and lies where it comes down.
	shots := bullet.NewShots(s.world)
	shots.Define("round", bullet.Body{Size: 4, Speed: 10 * CellSize, Range: 10 * CellSize}, rule.Plays(s.shot))
	s.round = shots.Named("round")
	shots.Define("grenade", bullet.Body{Size: 8, Speed: 5 * CellSize, Range: 9 * CellSize, Gravity: 240, Lands: true}, rule.Plays(s.thrown))
	s.grenade = shots.Named("grenade")
	mortal := rule.Plays(s.mortal)
	units.Define("soldier", unit.Mover{Domain: cell.Land}, profile, mortal,
		comp.Const(world.Velocity{Dir: geom.NewVec(1, 0)}), comp.Const(world.Eye{Height: 16}))
	s.soldier = units.Named("soldier")
	units.Define("wanderer", unit.Mover{Domain: cell.Land}, profile, mortal,
		comp.Load(func(u unitRow) navigation.MoveOrder { return navigation.Patrol(time.Second, u.to, u.start) }))
	s.wanderer = units.Named("wanderer")
}

// layOut lays the road, the pond and the two walls.
func (s *mainStage) layOut() {
	at := s.cellAt
	var cells []cell.Entry
	for x := uint32(1); x < GridWidth-1; x++ {
		cells = append(cells, cell.Entry{Kind: "road", Cell: at(x, roadRow)})
	}
	for y := uint32(12); y <= 14; y++ {
		for x := uint32(2); x <= 4; x++ {
			cells = append(cells, cell.Entry{Kind: "water", Cell: at(x, y)})
		}
	}
	for y := uint32(2); y <= 6; y++ {
		cells = append(cells, cell.Entry{Kind: "wall", Cell: at(wallColumn, y)})
	}
	for y := uint32(10); y <= 14; y++ {
		cells = append(cells, cell.Entry{Kind: "low wall", Cell: at(wallColumn, y)})
	}
	s.board.Seed(board.Layout{Default: "grass", Cells: cells})
}

// placeUnits puts the soldier, the player's and selected, and the wild's wanderers in place: one
// on the road in the line of fire, one behind the low wall, one behind the high one.
func (s *mainStage) placeUnits() {
	at := s.cellAt
	s.world.Seed(
		s.soldier.Entry(unitRow{start: at(3, roadRow)}).Told(players.Give{To: s.player.ID}, selection.Allow{Selected: true}),
		s.wanderer.Entry(unitRow{start: at(8, roadRow), to: at(13, roadRow)}).Told(players.Give{To: s.wild.ID}),
		s.wanderer.Entry(unitRow{start: at(9, 12), to: at(14, 12)}).Told(players.Give{To: s.wild.ID}),
		s.wanderer.Entry(unitRow{start: at(9, 4), to: at(12, 4)}).Told(players.Give{To: s.wild.ID}),
	)
}

func (s *mainStage) cellAt(x, y uint32) cell.ID { c, _ := s.brd.CellIndex(x, y); return c }

// Update runs the plugins: bullet first, so the shots' steps are tested by collision in the
// same step, then the world and whatever reads it.
func (s *mainStage) update(ctx goke.RunCtx, d time.Duration) {
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
	stage *mainStage
	keys  players.SceneKeys
}

var _ game.Scene = (*mainScene)(nil)

func (m *mainScene) Name() string { return "main" }

func (m *mainScene) Layers() []render.Layer {
	s := m.stage

	worldAtlas := render.NewAtlas()
	worldAtlas.RegisterAt(s.soldier.SpriteID(), EntitySize, render.Diamond(color.RGBA{R: 90, G: 140, B: 230, A: 255}))
	worldAtlas.RegisterAt(s.wanderer.SpriteID(), EntitySize, render.Solid(color.RGBA{R: 220, G: 90, B: 90, A: 255}))
	worldAtlas.RegisterAt(s.paleSprite, EntitySize, render.Solid(color.RGBA{R: 235, G: 200, B: 200, A: 255}))
	worldAtlas.RegisterAt(s.round.SpriteID(), 4, render.Solid(color.RGBA{R: 255, G: 230, B: 80, A: 255}))
	worldAtlas.RegisterAt(s.grenade.SpriteID(), 8, render.Solid(color.RGBA{R: 50, G: 80, B: 50, A: 255}))
	worldAtlas.RegisterAt(s.sparkSprite, 8, render.Diamond(color.RGBA{R: 255, G: 250, B: 200, A: 255}))
	worldAtlas.Close()
	s.world.WithRenderer(worldAtlas)

	kinds := s.board.CellKinds()
	boardAtlas := render.NewAtlas()
	for name, c := range map[string]color.RGBA{
		"grass":    {R: 60, G: 95, B: 60, A: 255},
		"road":     {R: 150, G: 130, B: 80, A: 255},
		"water":    {R: 40, G: 90, B: 170, A: 255},
		"wall":     {R: 90, G: 90, B: 100, A: 255},
		"low wall": {R: 150, G: 150, B: 160, A: 255},
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
	m.stage.players.EventHandler().HandleEvents(events)
	m.keys.Handle(events, runtime, composition)
}

func (m *mainScene) Focusable() bool { return true }
