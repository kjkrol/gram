package board

import (
	"fmt"
	"log"
	"math"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/entity/kind/comp"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/ground"
	"github.com/kjkrol/gram/plugins/board/internal/moments"
	"github.com/kjkrol/gram/plugins/board/internal/occupancy"
	"github.com/kjkrol/gram/plugins/board/internal/terrain"
	"github.com/kjkrol/gram/plugins/board/look"
	"github.com/kjkrol/gram/plugins/board/unit"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/steering"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/rule"
	"github.com/kjkrol/uid"
)

// Resources is board's single published Resources — Logic is what
// board-modifying code needs; Render is nil unless WithRenderer was called.
type Resources struct {
	Logic struct {
		Board *Board
	}
	Render *look.RenderState
}

// Plugin wires a Board into a Game; it depends on world, and hands collision its solid ground.
type Plugin struct {
	Res Resources

	occupancy cell.Occupancy
	renderer  *look.Renderer
	atlas     render.AtlasSource // the renderer's
	kinds     *terrain.Kinds
	seeded    *Layout
	mapping   Map

	worldPlugin *world.Plugin
	module      *module
	rules       *moments.Rules
	workers     int // how many goroutines at most share a frame's tiles: 0 all the CPUs, 1 none
}

var _ plugin.Plugin = (*Plugin)(nil)
var _ plugin.Populator = (*Plugin)(nil)

// NewPlugin builds a board over g with the given occupancy cap, slowing worldPlugin's entities.
// It is drawn and priced by the simple map until WithMap sets another.
func NewPlugin(g grid.Grid, occupancy cell.Occupancy, worldPlugin *world.Plugin) *Plugin {
	kind.Require[unit.At](&worldPlugin.Roster().Unit, "board", "the cell it starts in")
	kind.Require[unit.Mover](&worldPlugin.Roster().Unit, "board", "the domains it moves in")
	p := &Plugin{
		occupancy:   occupancy,
		worldPlugin: worldPlugin,
		kinds:       terrain.NewKinds(worldPlugin.HasHeights()),
	}
	brd := NewBoard(g)
	p.Res.Logic.Board = brd
	brd.setHeights(worldPlugin.HasHeights())
	p.mapping = newSimpleMap(brd)
	brd.mapping = p.mapping
	if ws, ok := p.Res.Logic.Board.Grid.(interface{ SetWrap(x, y bool) }); ok {
		edges := worldPlugin.Res.Config.Space.Edges
		ws.SetWrap(edges.WrapsX(), edges.WrapsY())
	}
	slope := func(at, dir geom.Vec, d cell.Domain) float64 { return brd.Map().Slope(at, dir, d) }
	p.rules = moments.New(brd.Grid, brd.cells, worldPlugin.Tick, slope)
	worldPlugin.Roster().Unit.Default(comp.Const(steering.Pace{Share: 1}))
	return p
}

// =================================================================
// plugin.Plugin contract
// =================================================================

func (p *Plugin) Name() string { return "gram.board" }

// Install wires the cell entities, the occupancy's upkeep and the rules.
func (p *Plugin) Install(ctx plugin.Installer) error {
	p.module = &module{
		cells:     p.Res.Logic.Board.cells.System(),
		release:   occupancy.ReleaseSystem(p.occupancy),
		standing:  p.rules.StandingSystem(),
		cellRules: p.rules.CellSystem(),
		clock:     p.worldPlugin.Clock(),
	}
	ctx.UseModule(p.module)
	return nil
}

// RunPlan, in the simulation, notices what effects did to the cells and reports where everyone
// stands; call it after collision's RunPlan.
func (p *Plugin) RunPlan(ctx goke.RunCtx, d time.Duration) { p.module.RunPlan(ctx, d) }

// WithRenderer builds the board renderer, drawing each cell's kind's SpriteID from atlas — or,
// given nil, from the board's own atlas of the kinds' Colors and drawn sprites.
func (p *Plugin) WithRenderer(atlas render.AtlasSource) {
	if atlas == nil {
		atlas = p.defaultAtlas()
	}
	p.atlas = atlas
	p.renderer = look.NewRenderer(p.Res.Logic.Board, atlas, mapRef{p}, p.worldPlugin.Res.Config.Space)
	p.Res.Render = p.renderer.State()
	p.Res.Render.ShowGridLines = true
	p.renderer.Workers(p.workers)
}

// WithLog has the board write a line to l, once for each, for a unit fallen where its domain may
// not be — in a hole, a walker in the water; call before Use.
func (p *Plugin) WithLog(l *log.Logger) *Plugin {
	p.rules.Log = l
	return p
}

// WithWorkers sets how many goroutines at most share a frame's tiles when the Map's Dressing
// dresses them in parallel (Parallel): 0, as it starts, as many as there are CPUs; 1 none.
func (p *Plugin) WithWorkers(n int) *Plugin {
	p.workers = n
	if p.renderer != nil {
		p.renderer.Workers(n)
	}
	return p
}

// Atlas is the sprite sheet the board's renderer draws the cells from, WithRenderer's; nil before
// it.
func (p *Plugin) Atlas() render.AtlasSource { return p.atlas }

// defaultAtlas is an atlas of every kind in the dictionary, a cell's size each: its drawn sprite
// (cell.Kinds.Draw) or its Color, grey for a kind of no colour. Call it once the kinds are
// created.
func (p *Plugin) defaultAtlas() render.AtlasSource {
	w, h := p.Res.Logic.Board.CellBounds()
	size := int(math.Ceil(max(w, h)))
	atlas := render.NewAtlas()
	for _, k := range p.kinds.All() {
		if draw, ok := p.kinds.Drawer(k.Name); ok {
			atlas.RegisterAt(k.SpriteID, size, draw)
			continue
		}
		c := k.Color
		if c.A == 0 {
			c.R, c.G, c.B, c.A = 128, 128, 128, 255
		}
		atlas.RegisterAt(k.SpriteID, size, render.Solid(c))
	}
	atlas.Close()
	return atlas
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

// Hook hosts rules (rule.On) of a unit.Standing, fired every step for every entity on the board,
// and of a cell.Now, fired every step for every cell; hook them before Use.
func (p *Plugin) Hook(rules ...rule.Rule) error {
	for _, r := range rules {
		if err := p.rules.Hook(r); err != nil {
			return fmt.Errorf("%w in %s — it takes a rule of unit.Standing or cell.Now", err, p.Name())
		}
	}
	return nil
}

// =================================================================
// board-specific
// =================================================================

// WithMap has the board drawn and priced by m — a topography's — in place of the simple map.
// Call before Use.
func (p *Plugin) WithMap(m Map) *Plugin {
	p.mapping = m
	p.Res.Logic.Board.mapping = m
	return p
}

// Map is what the board is drawn and priced by: the simple map unless WithMap set another.
func (p *Plugin) Map() Map { return p.mapping }

// Climb is the Map's: how many times as long the step from one cell to its neighbour takes
// whoever moves in d as on the flat.
func (p *Plugin) Climb(from, to cell.ID, d cell.Domain) float64 { return p.mapping.Climb(from, to, d) }

// Least is the Map's: the smallest Climb for d.
func (p *Plugin) Least(d cell.Domain) float64 { return p.mapping.Least(d) }

// CellEntity is cell c's own entity, carrying its [Plot], [Ground], [Way] and [Crossing] for as
// long as the board lives, so an effect cast on it is an effect on the cell's terrain; false off
// the board or before Setup.
func (p *Plugin) CellEntity(c cell.ID) (uid.UID64, bool) { return p.Res.Logic.Board.CellEntity(c) }

// Occupancy returns the occupancy tracker this plugin was built with.
func (p *Plugin) Occupancy() cell.Occupancy { return p.occupancy }

// CellKinds are this Plugin's registered kinds of cells.
func (p *Plugin) CellKinds() cell.Kinds { return p.kinds }

// Seed sets the terrain applied when this Stage starts fresh — see Populate.
func (p *Plugin) Seed(layout Layout) { p.seeded = &layout }

// Populate applies the seeded Layout — kinds, the cells' tags, ways and crossings — changing nothing
// and erroring on an unknown kind name.
func (p *Plugin) Populate() error {
	if p.seeded == nil {
		return nil
	}
	resolve := func(name string) (cell.Kind, error) {
		kind, ok := p.kinds.Get(name)
		if !ok {
			return cell.Kind{}, fmt.Errorf("board: unknown cell kind %q", name)
		}
		return kind, nil
	}

	var def cell.Kind
	if p.seeded.Default != "" {
		kind, err := resolve(p.seeded.Default)
		if err != nil {
			return err
		}
		def = kind
	}
	cells := make([]cell.Kind, len(p.seeded.Cells))
	for i, e := range p.seeded.Cells {
		if e.Kind == "" {
			continue // the Default kept
		}
		kind, err := resolve(e.Kind)
		if err != nil {
			return err
		}
		cells[i] = kind
	}

	ways := make([]cell.Way, len(p.seeded.Ways))
	for i, e := range p.seeded.Ways {
		kind, err := resolve(e.Kind)
		if err != nil {
			return err
		}
		ways[i] = cell.Way{Kind: kind, Width: e.Width, Links: e.Links, Fade: e.Fade, Mix: e.Mix}
	}
	crossings := make([]cell.Crossing, len(p.seeded.Crossings))
	for i, e := range p.seeded.Crossings {
		kind, err := resolve(e.Kind)
		if err != nil {
			return err
		}
		crossings[i] = cell.Crossing{Way: cell.Way{Kind: kind, Width: e.Width, Links: e.Links, Fade: e.Fade, Mix: e.Mix}}
	}

	brd := p.Res.Logic.Board
	if p.seeded.Default != "" {
		brd.SetAll(def)
	}
	for i, e := range p.seeded.Cells {
		if e.Kind != "" {
			brd.Set(e.Cell, cells[i])
		}
		if e.Tags != 0 {
			brd.cells.Tag(e.Cell, e.Tags)
		}
		if e.Roles != 0 {
			brd.cells.Cast(e.Cell, e.Roles)
		}
		if e.Wired != nil {
			brd.cells.Wire(e.Cell, e.Wired)
		}
	}
	for i, e := range p.seeded.Ways {
		brd.SetWay(e.Cell, ways[i])
	}
	for i, e := range p.seeded.Crossings {
		brd.SetCrossing(e.Cell, crossings[i])
	}
	p.seeded = nil
	return nil
}

// =================================================================
// the ground the others meet
// =================================================================

// Heights is the board's ground heights: its Map's, nil on a flat map.
func (p *Plugin) Heights() ground.Heights { return p.mapping.Heights() }

// Cover is what stands on the board and holds sight back — walls, forests — walked along a ray;
// it is a ground.Readied too.
func (p *Plugin) Cover() ground.Cover { return p.Res.Logic.Board.field }

// WithCollision makes the board's Solid cells the solid ground c pushes colliders out of, and the
// ground a kind does not take (Overhang) what c never pushes one over; call before Use.
func (p *Plugin) WithCollision(c *collision.Plugin) *Plugin {
	c.WithField(p.Res.Logic.Board.field)
	return p
}
