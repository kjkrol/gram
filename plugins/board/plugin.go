package board

import (
	"fmt"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugin/host"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/kind"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/uid"
)

// Resources is board's single published Resources — Logic is what
// board-modifying code needs; Render is nil unless WithRenderer was called.
type Resources struct {
	Logic struct {
		Board *Board
	}
	Render *RenderState
}

// Plugin wires a Board into a Game; it depends on world alone.
type Plugin struct {
	Res Resources

	occupancy Occupancy
	renderer  *Renderer
	kinds     *cellKindDict
	seeded    *Layout
	shaping   shaping

	worldPlugin *world.Plugin
	module      *module
	standing    host.EachHost[Standing]
}

var _ plugin.Plugin = (*Plugin)(nil)
var _ plugin.Populator = (*Plugin)(nil)
var _ plugin.CommandHandler = (*Plugin)(nil)

// NewPlugin builds a board over grid with the given occupancy cap, slowing worldPlugin's entities.
func NewPlugin(grid Grid, occupancy Occupancy, worldPlugin *world.Plugin) *Plugin {
	terrain := NewTerrainMap()
	kind.Require[Cell](&worldPlugin.Roster().Unit, "board", "the cell it starts in")
	kind.Require[Mover](&worldPlugin.Roster().Unit, "board", "the domains it moves in")
	p := &Plugin{
		occupancy:   occupancy,
		worldPlugin: worldPlugin,
		kinds:       newCellKindDict(worldPlugin.Quasi3D()),
	}
	w, h := grid.CellBounds()
	p.shaping.cfg = Shaping{Step: min(w, h) / 4}
	brd := NewBoard(grid, terrain)
	p.Res.Logic.Board = brd
	brd.quasi3D = worldPlugin.Quasi3D()
	if brd.quasi3D {
		worldPlugin.SetGround(brd)
	}
	worldPlugin.SetCover(brd)
	worldPlugin.SetField(brd)
	if ws, ok := p.Res.Logic.Board.Grid.(wrapSetter); ok {
		edges := worldPlugin.Res.Config.Space.Edges
		ws.SetWrap(edges.WrapsX(), edges.WrapsY())
	}
	if err := worldPlugin.RegisterBehavior(terrainSpeed(p.Res.Logic.Board)); err != nil {
		panic(err)
	}
	return p
}

// =================================================================
// plugin.Plugin contract
// =================================================================

func (p *Plugin) Name() string { return "gram.board" }

// Install wires the cell entities and the standing report.
func (p *Plugin) Install(ctx plugin.Installer) error {
	p.module = &module{
		cells:    newCellSystem(p.Res.Logic.Board, &p.shaping),
		standing: newStandingSystem(p.Res.Logic.Board, &p.standing),
	}
	if p.worldPlugin.Quasi3D() {
		p.module.altitude = newAltitudeSystem(p.Res.Logic.Board)
	}
	ctx.UseModule(p.module)
	return nil
}

// RunPlan shapes the ground, notices what effects did to the cells and reports where everyone
// stands; call it after collision's RunPlan.
func (p *Plugin) RunPlan(ctx goke.RunCtx, d time.Duration) { p.module.RunPlan(ctx, d) }

// WithRenderer builds the board renderer, drawing each cell's CellKind.SpriteID from atlas.
func (p *Plugin) WithRenderer(atlas render.AtlasSource) {
	p.Res.Render = &RenderState{ShowGridLines: true}
	p.renderer = newRenderer(p.Res.Logic.Board, atlas, p.Res.Render)
}

// Renderer returns this plugin's own render.Renderer, or nil unless WithRenderer was called.
func (p *Plugin) Renderer() render.Layer {
	if p.renderer == nil {
		return nil
	}
	return p.renderer
}

// EventHandler always returns nil — board has no input handling of its own; see plugins/navigation.
func (p *Plugin) EventHandler() control.EventHandler { return nil }

// Serializable is nil — the terrain is the cells' entities, saved with the ECS.
func (p *Plugin) Serializable() plugin.Serializable { return nil }

// RegisterBehavior hosts an Each or Every of Standing, run every tick for every entity on the board;
// register before Use.
func (p *Plugin) RegisterBehavior(behaviors ...plugin.Behavior) error {
	for _, b := range behaviors {
		if err := p.standing.Add(b); err != nil {
			return fmt.Errorf("%w in %s — it takes Each for Standing", err, p.Name())
		}
	}
	return nil
}

// =================================================================
// board-specific
// =================================================================

// CellEntity is cell c's own entity, carrying its [Plot], [Ground] and [Relief] for as long as the
// board lives, so an effect cast on it is an effect on the cell's terrain; false off the board or
// before Setup.
func (p *Plugin) CellEntity(c CellID) (uid.UID64, bool) { return p.Res.Logic.Board.CellEntity(c) }

// WithShaping sets how the Raise, Lower and Level commands move the ground; by default a Step of
// a quarter of a cell's shorter side and any slope. Call before Use.
func (p *Plugin) WithShaping(s Shaping) *Plugin {
	p.shaping.cfg = s
	return p
}

// Queues are where Raise, Lower and Level land in a Quasi3D world; none in a flat one.
func (p *Plugin) Queues() []control.CommandQueue {
	if !p.worldPlugin.Quasi3D() {
		return nil
	}
	return []control.CommandQueue{&p.shaping.raise, &p.shaping.lower, &p.shaping.level}
}

// DefaultBindings in a Quasi3D world: = raises and - lowers the ground under the cursor, a left
// drag with L held levels it to where the drag began.
func (p *Plugin) DefaultBindings() []control.Binding {
	if !p.worldPlugin.Quasi3D() {
		return nil
	}
	at := func(c control.Context) geom.Vec { return c.World(c.Cursor) }
	return []control.Binding{
		control.Command(control.KeyPress{Key: ebiten.KeyEqual}, "Raise the ground", func(c control.Context) (Raise, bool) { return Raise{At: at(c)}, true }),
		control.Command(control.KeyPress{Key: ebiten.KeyMinus}, "Lower the ground", func(c control.Context) (Lower, bool) { return Lower{At: at(c)}, true }),
		control.Command(control.Drag{Button: ebiten.MouseButtonLeft, Mods: control.Mods{}.Holding(ebiten.KeyL)}, "Level the ground",
			func(c control.Context) (Level, bool) {
				return Level{From: c.World(c.Start), To: c.World(c.Cursor)}, true
			}),
	}
}

// Occupancy returns the occupancy tracker this plugin was built with.
func (p *Plugin) Occupancy() Occupancy { return p.occupancy }

// CellKindDict returns this Plugin's registered CellKinds.
func (p *Plugin) CellKindDict() CellKindDict { return p.kinds }

// Seed sets the terrain applied when this Stage starts fresh — see Populate.
func (p *Plugin) Seed(layout Layout) { p.seeded = &layout }

// Populate applies the seeded Layout, kinds and heights, changing nothing and erroring on an
// unknown kind name.
func (p *Plugin) Populate() error {
	if p.seeded == nil {
		return nil
	}
	if p.seeded.Heights != nil && !p.worldPlugin.Quasi3D() {
		return fmt.Errorf("board: a Layout with Heights in a flat world; set world.Config.Quasi3D")
	}
	resolve := func(name string) (CellKind, error) {
		kind, ok := p.kinds.Get(name)
		if !ok {
			return CellKind{}, fmt.Errorf("board: unknown CellKind %q", name)
		}
		return kind, nil
	}

	var def CellKind
	if p.seeded.Default != "" {
		kind, err := resolve(p.seeded.Default)
		if err != nil {
			return err
		}
		def = kind
	}
	cells := make([]CellKind, len(p.seeded.Cells))
	for i, e := range p.seeded.Cells {
		kind, err := resolve(e.Kind)
		if err != nil {
			return err
		}
		cells[i] = kind
	}

	brd := p.Res.Logic.Board
	if p.seeded.Default != "" {
		brd.SetAll(def)
	}
	for i, e := range p.seeded.Cells {
		brd.Set(e.Cell, cells[i])
	}
	if p.seeded.Heights != nil {
		brd.SetHeights(p.seeded.Heights)
	}
	p.seeded = nil
	return nil
}
