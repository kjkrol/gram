// Command effect-demo is an ice witch: an entity under orders that turns the ground round her
// feet into snow and the water into ice — the terrain is hers to write, as far as her power
// reaches, and it thaws some seconds after she has gone. She is fast on her own snow; a walker
// can cross the lake on her trail while it lasts — slipping, so it brakes badly and may not stop
// before ice that melts ahead of it — and a boat, whose brakes are weak, sails onto the ice it saw
// coming and is frozen in — pale, still, immovable — until the ice melts. Everything temporary
// here is an effect.
package main

import (
	"github.com/kjkrol/aabbworld/geom"
	"image/color"
	"log"
	"math"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/effects"
	"github.com/kjkrol/gram/plugins/navigation"
	"github.com/kjkrol/gram/plugins/players"
	"github.com/kjkrol/gram/plugins/selection"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/kind"
	"github.com/kjkrol/gram/plugins/world/kind/comp"
	"github.com/kjkrol/gram/render"
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
	MaxEntCount  = 16 // the units; cell entities do not count

	lakeLeft, lakeRight uint32 = 8, 15
	lakeTop, lakeBottom uint32 = 4, 11

	// Frost is the witch's own way of moving: snow and ice price it low.
	Frost = board.Domain(1 << 3)
	// thawAfter is how long the witch's frost holds where she stood — five seconds at most; her
	// Power widens the area, not the time.
	thawAfter = 5 * time.Second
)

// chill is the demo's tag family; frozen is who is stuck in the ice.
type chill struct{}

// =========================== Game ===========================

// Demo is the effect demo — exactly one Stage (mainStage below).
type Demo struct{ stage *mainStage }

var _ game.Game = (*Demo)(nil)

func NewDemo() *Demo { return &Demo{stage: &mainStage{}} }

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

// witch tags who leaves winter behind; Power is how many rings of cells round her freeze.
type witch struct{ Power int }

// unit is the row every kind spawns from: where it starts and, if ordered, where it heads.
type unit struct {
	start, target board.CellID
	ordered       bool
}

type mainStage struct {
	world     *world.Plugin
	board     *board.Plugin
	nav       *navigation.Plugin
	collision *collision.Plugin
	selection *selection.Plugin
	players   *players.Plugin
	brd       *board.Board
	effects   *effects.Plugin
	snow, ice board.CellKind

	frost, frozen, slip effects.ID
	frozenTag           plugin.Tag[chill]
	paleSprite          render.SpriteID

	witch, walker, boat kind.Of[unit]
	stack               game.Scenes
}

var _ game.Stage = (*mainStage)(nil)

func (s *mainStage) Name() string { return "effect-demo" }

func (s *mainStage) Stack() game.Scenes { return s.stack }

func (s *mainStage) Init(ctx game.Initializer) error {
	s.world = ctx.UseWorld(world.Config{
		Space:    world.SpaceCfg{Width: ScreenWidth, Height: ScreenHeight},
		Entities: world.EntitiesCfg{MaxCount: MaxEntCount, MinSize: EntitySize, MaxSize: EntitySize},
	})

	s.collision = collision.NewPlugin(s.world)
	if err := ctx.Use(s.collision); err != nil {
		return err
	}

	grid := board.DefaultGrids{}.Square(GridWidth, GridHeight, CellSize)
	s.effects = effects.NewPlugin(s.world)
	// A frozen boat holds its cell, so the planner goes round.
	s.board = board.NewPlugin(grid, &board.SingleOccupancy{}, s.world)
	s.brd = s.board.Res.Logic.Board
	s.board.CellKindDict().Create(
		board.CellKind{Name: board.Named("grass"), Cost: 2, Allows: board.Land},
		board.CellKind{Name: board.Named("road"), Cost: 1, Allows: board.Land},
		board.CellKind{Name: board.Named("water"), Cost: 1, Allows: board.Water},
		board.CellKind{Name: board.Named("snow"), Cost: 3, Allows: board.Land | Frost}.Costing(Frost, 0.5),
		board.CellKind{Name: board.Named("ice"), Cost: 2, Allows: board.Land | Frost}.Costing(Frost, 0.5),
	)
	s.snow, _ = s.board.CellKindDict().Get("snow")
	s.ice, _ = s.board.CellKindDict().Get("ice")

	// The whole of the game's logic: two reactions to where things stand, registered before Use.
	if err := s.board.RegisterBehavior(
		board.Each[witch](s.freeze),
		board.Each[board.Mover](s.onGround),
	); err != nil {
		return err
	}
	if err := ctx.Use(s.board); err != nil {
		return err
	}

	// Two effects: frost on a cell's ground, for a while; frozen on whoever is caught in the ice,
	// until the ice is gone. What frozen means for movement is a speed modifier of the game's.
	s.frozenTag = s.world.Kinds().DefineTag[chill]("frozen")
	s.paleSprite = s.world.Kinds().NewSprite()
	s.frost = s.effects.Define("frost", effects.Spec{
		effects.Lasts(thawAfter),
		effects.Alter(func(g *board.Ground) { g.Kind = s.frozenKind(g.Kind) }),
	})
	s.frozen = s.effects.Define("frozen", effects.Spec{
		effects.Grant(s.frozenTag),
		effects.Alter(func(a *world.Appearance) { a.SpriteID = s.paleSprite }),
		effects.Alter(func(p *collision.Physics) { p.Mass = math.Inf(1) }), // stuck fast: nobody shoves it
	})
	s.slip = s.effects.Define("slip", effects.Spec{
		effects.Alter(func(st *world.Steering) { st.Brake = st.Accel / 8 }), // ice: brakes barely bite
	})
	frozen := s.frozenTag
	if err := s.world.RegisterBehavior(world.Each[plugin.Tags[chill]](func(_ plugin.Tick, marks *plugin.Tags[chill], m world.Moving) {
		if marks.Has(frozen) {
			m.Base.Vel.Value = 0 // frozen fast: whoever carries the tag does not move
		}
	})); err != nil {
		return err
	}
	if err := ctx.Use(s.effects); err != nil {
		return err
	}

	s.selection = selection.NewPlugin(s.world)
	if err := ctx.Use(s.selection); err != nil {
		return err
	}

	s.nav = navigation.NewPlugin(s.board, s.world, s.selection).WithCollision(s.collision)
	if err := ctx.Use(s.nav); err != nil {
		return err
	}

	s.players = players.NewPlugin(s.world, s.selection, s.nav)
	if err := s.players.Local("player").Bind(s.players.Defaults()...); err != nil {
		return err
	}
	if err := ctx.Use(s.players); err != nil {
		return err
	}
	s.defineKinds()

	main := &mainScene{stage: s}
	stack, err := game.NewStack(main)
	if err != nil {
		return err
	}
	s.stack = stack
	comp := stack.Composition()
	comp.Show(main.Name())
	return ctx.Track(comp)
}

func (s *mainStage) Restore(game.Persistence) (bool, error) { return false, nil }

// frozenKind is what the witch's frost makes of a kind of ground.
func (s *mainStage) frozenKind(k board.CellKind) board.CellKind {
	switch k.Name.String() {
	case "grass", "road":
		return s.snow
	case "water":
		return s.ice
	}
	return k
}

// freeze casts frost on every cell round the witch, as many rings of neighbours out as her
// Power; a cell already frozen has its time refreshed.
func (s *mainStage) freeze(t plugin.Tick, w *witch, st board.Standing) {
	ring := map[board.CellID]struct{}{}
	s.brd.CellsUnder(st.Box, func(c board.CellID) { ring[c] = struct{}{} })
	for range w.Power {
		next := map[board.CellID]struct{}{}
		for c := range ring {
			for _, n := range s.brd.Neighbors(c) {
				next[n] = struct{}{}
			}
		}
		for c := range next {
			ring[c] = struct{}{}
		}
	}
	for c := range ring {
		if s.frozenKind(s.brd.Kind(c)) == s.brd.Kind(c) && !s.brd.Kind(c).Admits(Frost) {
			continue // nothing here freezes
		}
		if id, ok := s.board.CellEntity(c); ok {
			s.effects.Cast(t.CmdBuf, id, s.frost)
		}
	}
}

// onGround decides for whoever stands on the board by what is under it: caught in ice — a boat
// whose water froze — it is frozen until the ice is gone; on ice it may walk, it slips; where its
// domain may not be and there is no ice — a walker whose ice melted — it has fallen in and is gone.
func (s *mainStage) onGround(t plugin.Tick, m *board.Mover, st board.Standing) {
	fell := st.Fell(m.Domain)
	onIce := st.Kind == s.ice
	frozen, slipping := s.effects.Has(st.ID, s.frozen), s.effects.Has(st.ID, s.slip)
	switch {
	case fell && onIce && !frozen:
		log.Printf("entity %d froze into the ice at cell %d", st.ID, st.Cell)
		s.effects.Cast(t.CmdBuf, st.ID, s.frozen)
	case fell && !onIce:
		log.Printf("entity %d fell into the %s at cell %d", st.ID, st.Kind.Name, st.Cell)
		s.world.Despawn(t.CmdBuf, st.ID)
	case !fell && frozen:
		log.Printf("entity %d is free of the ice", st.ID)
		s.effects.Dispel(st.ID, s.frozen)
	}
	switch {
	case !fell && onIce && !slipping:
		s.effects.Cast(t.CmdBuf, st.ID, s.slip)
	case !onIce && slipping:
		s.effects.Dispel(st.ID, s.slip)
	}
}

// defineKinds says what this game's entities are: the witch walks on land and water, the walker
// on land, the boat on water.
func (s *mainStage) defineKinds() {
	units := board.NewUnits[unit](s.board, board.Shape{Size: EntitySize}, func(u unit) geom.Vec { return s.brd.CellCenter(u.start) })
	profile := func(brake float64) world.Steering {
		return world.Steering{MaxSpeed: UnitSpeed, Accel: UnitSpeed * 2, Brake: brake, V0: UnitSpeed / 2, TurnRate: 0.15}
	}
	sel := comp.Tagged(s.selection.Tags().Selectable)
	order := comp.Load(func(u unit) navigation.MoveOrder { return navigation.MoveOrder{Target: u.target} })
	s.witch = units.Define("witch", board.Mover{Domain: board.Land | board.Water | Frost}, profile(UnitSpeed*4), sel, order, comp.Const(witch{Power: 1}))
	s.walker = units.Define("walker", board.Mover{Domain: board.Land}, profile(UnitSpeed*4), sel)
	s.boat = units.Define("boat", board.Mover{Domain: board.Water}, profile(UnitSpeed/4), sel, order)
}

// Spawn lays the lake and the road and puts the three of them in place.
func (s *mainStage) Spawn() error {
	cell := func(x, y uint32) board.CellID { c, _ := s.brd.CellIndex(x, y); return c }
	var cells []board.CellEntry
	for y := lakeTop; y <= lakeBottom; y++ {
		for x := lakeLeft; x <= lakeRight; x++ {
			cells = append(cells, board.CellEntry{Kind: "water", Cell: cell(x, y)})
		}
	}
	for x := uint32(1); x < GridWidth-1; x++ {
		cells = append(cells, board.CellEntry{Kind: "road", Cell: cell(x, 1)}, board.CellEntry{Kind: "road", Cell: cell(x, GridHeight-2)})
	}
	s.board.Seed(board.Layout{Default: "grass", Cells: cells})

	s.world.Seed(
		s.witch.Entry(unit{start: cell(2, 8), target: cell(GridWidth-3, 8)}),
		s.walker.Entry(unit{start: cell(2, 10)}),
		s.boat.Entry(unit{start: cell(lakeRight, 8), target: cell(lakeLeft, 8)}), // head-on into the witch's trail
	)
	return nil
}

func (s *mainStage) Update(ctx goke.RunCtx, d time.Duration) {
	s.world.RunPlan(ctx, d)
	s.collision.RunPlan(ctx, d)
	s.effects.RunPlan(ctx, d)
	s.board.RunPlan(ctx, d)
	s.nav.RunPlan(ctx, d)
	s.selection.RunPlan(ctx, d)
	s.players.RunPlan(ctx, d)
	ctx.Sync()
}

// =========================== Scene ===========================

type mainScene struct{ stage *mainStage }

var _ game.Scene = (*mainScene)(nil)

func (m *mainScene) Name() string { return "main" }

func (m *mainScene) Layers() []render.Layer {
	s := m.stage

	worldAtlas := render.NewAtlas()
	worldAtlas.RegisterAt(s.witch.SpriteID(), EntitySize, render.Diamond(color.RGBA{R: 200, G: 230, B: 255, A: 255}))
	worldAtlas.RegisterAt(s.walker.SpriteID(), EntitySize, render.Solid(color.RGBA{R: 220, G: 90, B: 90, A: 255}))
	worldAtlas.RegisterAt(s.boat.SpriteID(), EntitySize, render.Solid(color.RGBA{R: 140, G: 90, B: 40, A: 255}))
	worldAtlas.RegisterAt(s.paleSprite, EntitySize, render.Solid(color.RGBA{R: 190, G: 220, B: 245, A: 255}))
	worldAtlas.Close()
	s.world.WithRenderer(worldAtlas)

	kinds := s.board.CellKindDict()
	boardAtlas := render.NewAtlas()
	for name, c := range map[string]color.RGBA{
		"grass": {R: 60, G: 95, B: 60, A: 255},
		"road":  {R: 150, G: 130, B: 80, A: 255},
		"water": {R: 40, G: 90, B: 170, A: 255},
		"snow":  {R: 235, G: 240, B: 245, A: 255},
		"ice":   {R: 170, G: 215, B: 240, A: 255},
	} {
		k, _ := kinds.Get(name)
		boardAtlas.RegisterAt(k.SpriteID, CellSize, render.Solid(c))
	}
	boardAtlas.Close()
	s.board.WithRenderer(boardAtlas)

	pathAtlas, pathSprites := navigation.RegisterDefaultPathSprites(CellSize, 2, color.RGBA{R: 255, G: 140, B: 0, A: 255})
	s.nav.SetPathSprites(pathSprites)
	s.nav.WithRenderer(pathAtlas)
	s.selection.WithRenderer(nil)

	return []render.Layer{s.board.Renderer(), s.world.Renderer(), s.selection.Renderer(), s.nav.Renderer()}
}

// Viewports are where the world is shown: the local players' views.
func (m *mainScene) Viewports(screen geom.AABB) []render.Viewport {
	return m.stage.players.Viewports(screen)
}

func (m *mainScene) HandleEvents(events *control.InputEvents, runtime game.Runtime, _ game.Composition) {
	s := m.stage
	s.players.EventHandler().HandleEvents(events)
	for _, k := range events.KeyEvents {
		if k.Action != control.ActionPress {
			continue
		}
		switch k.Key {
		case ebiten.KeyEscape:
			runtime.Quit()
		case ebiten.KeySpace:
			runtime.TogglePause()
		case ebiten.KeyB:
			s.board.Res.Render.ToggleShowGridLines()
		}
	}
}

func (m *mainScene) Focusable() bool { return true }
