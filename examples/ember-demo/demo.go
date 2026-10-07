// The ember demo: procedural animation out of an entity's own state. Each ember's look is one
// material (ember.wgsl) — no frames: the renderer feeds it how fast the entity moves (red) and
// which way it heads (custom.w), so a standing ember breathes on the clock and a driven one
// streams a tail against its heading, wilder the faster it runs. Click an ember and drive it
// with WSAD; D douses the selected ones — under the doused effect an ember is a plain sooty
// sprite until it rekindles.
package main

import (
	"embed"
	"image/color"
	"log"
	"slices"
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
	EntitySize   = 28
	UnitSpeed    = CellSize * 3

	// dousedFor is how long D keeps an ember out.
	dousedFor = 3 * time.Second
)

//go:embed ember.wgsl
var emberFS embed.FS

// emberMaterial joins the composer's one shader as the package is set up, before it compiles.
var emberMaterial = render.RegisterMaterials(render.Files(emberFS, "ember.wgsl"), nil, "Ember")[0]

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
		Title:       "gram — embers animated by their own state; WSAD drives the selected, F douses",
		ScreenWidth: ScreenWidth, ScreenHeight: ScreenHeight, Resizable: true,
		TargetTPS: TPS,
	}
}

func (d *Demo) Stages() (map[string]game.Stage, string) {
	return map[string]game.Stage{d.stage.Name(): d.stage}, d.stage.Name()
}

// =========================== Stage ===========================

type unitRow struct {
	start cell.ID
	round [4]cell.ID
}

type arena struct {
	world     *world.Plugin
	board     *board.Plugin
	nav       *navigation.Plugin
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
	return s, stage.New("ember-demo").
		Plugins(s.usePlugins).
		Players(s.definePlayer).
		Effects(s.defineEffects).
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
		Entities: world.EntitiesCfg{MaxCount: 8, MinSize: EntitySize, MaxSize: EntitySize},
	})
	s.collision = collision.NewPlugin(s.world)
	grid := grid.DefaultGrids{}.Square(GridWidth, GridHeight, CellSize)
	s.board = board.NewPlugin(grid, &cell.SingleOccupancy{}, s.world).WithCollision(s.collision).WithLog(log.Default())
	s.selection = selection.NewPlugin(s.world)
	s.nav = navigation.NewPlugin(s.board, s.world, s.selection).WithCollision(s.collision)
	s.cameras = cameras.NewPlugin(s.world, cameras.TopDown(), camera.Config{})
	s.players = players.NewPlugin(s.world, s.cameras, s.board, s.selection, s.nav)
	s.nav.WithPlayers(s.players)
	for _, p := range []plugin.Plugin{s.collision, s.board, s.selection, s.nav, s.cameras, s.players} {
		if err := ctx.Use(p); err != nil {
			return err
		}
	}
	return nil
}

func (s *arena) definePlayer() error {
	s.player = s.players.Local("player", s.cameras.Main())
	taken := []control.Trigger{ // WSAD drives the selected ember, not the camera
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

func (s *arena) defineEffects() {
	s.world.Effects().Define(DousedEf, effect.Spec{
		effect.Described("Doused: the flame put out — soot until it rekindles."),
		effect.Lasts(dousedFor),
	})
}

func (s *arena) defineCommands() {
	s.world.Commands().Define(DouseCmd,
		rule.Cast(s.world.Effects().Named(DousedEf)).On(s.selection.Selected()))
}

func (s *arena) defineCells() {
	s.board.CellKinds().Define(GrassCell, cell.Kind{Cost: 1, Allows: cell.Land})
}

func (s *arena) defineKinds() {
	brd := s.board.Res.Logic.Board
	units := board.NewUnits[unitRow](s.board, board.Shape{Size: EntitySize}, func(u unitRow) geom.Vec { return brd.CellCenter(u.start) })
	profile := steering.Steering{MaxSpeed: UnitSpeed, Accel: UnitSpeed * 2, Brake: UnitSpeed * 4, V0: UnitSpeed / 2, TurnRate: 0.1}
	round := comp.Load(func(u unitRow) navigation.MoveOrder {
		return navigation.Patrol(2*time.Second, u.round[0], u.round[1], u.round[2], u.round[3])
	})
	units.Define(EmberKind, unit.Mover{Domain: cell.Land}, profile, round)
}

func (s *arena) bindKeys() error {
	return s.player.Bind(append(players.DriveBindings(),
		control.Give(control.KeyPress{Key: control.KeyF}, "Douse the selected embers for a while",
			s.world.Commands().Named(DouseCmd)))...)
}

func (s *arena) defineScenes() []game.Scene {
	return []game.Scene{&mainScene{arena: s}}
}

func (s *arena) layOut() {
	s.board.Seed(board.Layout{Default: GrassCell})
}

func (s *arena) placeUnits() {
	brd := s.board.Res.Logic.Board
	emberKind := kind.Named[unitRow](s.world.Kinds(), EmberKind)
	player := []any{players.Give{To: s.player.ID}, selection.Allow{}}
	corners := [4]cell.ID{
		brd.CellIndex(4, 4), brd.CellIndex(GridWidth-5, 4),
		brd.CellIndex(GridWidth-5, GridHeight-5), brd.CellIndex(4, GridHeight-5),
	}
	for i := range corners {
		var round [4]cell.ID
		for j := range round {
			round[j] = corners[(i+1+j)%4]
		}
		s.world.Seed(emberKind.Entry(unitRow{start: corners[i], round: round}).Told(player...))
	}
}

func (s *arena) update(ctx goke.RunCtx, d time.Duration) {
	s.world.RunPlan(ctx, d)
	s.collision.RunPlan(ctx, d)
	s.board.RunPlan(ctx, d)
	s.nav.RunPlan(ctx, d)
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

// The scene's colours: the meadow and a doused ember's soot.
var (
	grassColor = color.RGBA{R: 55, G: 80, B: 55, A: 255}
	sootColor  = color.RGBA{R: 70, G: 65, B: 60, A: 255}
)

func (m *mainScene) Layers() []render.Layer {
	s := m.arena
	emberKind := kind.Named[unitRow](s.world.Kinds(), EmberKind)
	doused := s.world.Effects().Named(DousedEf)

	worldAtlas := s.world.NewAtlas()
	worldAtlas.Add(emberKind, EntitySize, emberMaterial).
		Under(doused, render.Dot(5, sootColor))
	worldAtlas.Close()
	s.world.WithRenderer(worldAtlas)

	boardAtlas := s.board.NewAtlas(CellSize)
	boardAtlas.Add(GrassCell, render.Solid(grassColor))
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
