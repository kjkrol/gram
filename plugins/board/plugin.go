package board

import (
	"fmt"
	"math"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugin/host"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/entity/kind"
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

// Plugin wires a Board into a Game; it depends on world, and hands collision its solid ground.
type Plugin struct {
	Res Resources

	occupancy Occupancy
	renderer  *Renderer
	kinds     *cellKindDict
	seeded    *Layout
	mapping   Map

	worldPlugin *world.Plugin
	module      *module
	standing    host.EachHost[Standing]
	workers     int // how many goroutines at most share a frame's tiles: 0 all the CPUs, 1 none
}

var _ plugin.Plugin = (*Plugin)(nil)
var _ plugin.Populator = (*Plugin)(nil)

// NewPlugin builds a board over grid with the given occupancy cap, slowing worldPlugin's entities.
// It is drawn and priced by the simple map until WithMap sets another.
func NewPlugin(grid Grid, occupancy Occupancy, worldPlugin *world.Plugin) *Plugin {
	terrain := NewTerrainMap()
	kind.Require[Cell](&worldPlugin.Roster().Unit, "board", "the cell it starts in")
	kind.Require[Mover](&worldPlugin.Roster().Unit, "board", "the domains it moves in")
	p := &Plugin{
		occupancy:   occupancy,
		worldPlugin: worldPlugin,
		kinds:       newCellKindDict(worldPlugin.HasHeights()),
	}
	brd := NewBoard(grid, terrain)
	p.Res.Logic.Board = brd
	brd.heights = worldPlugin.HasHeights()
	p.mapping = newSimpleMap(brd)
	brd.mapping = p.mapping
	if ws, ok := p.Res.Logic.Board.Grid.(wrapSetter); ok {
		edges := worldPlugin.Res.Config.Space.Edges
		ws.SetWrap(edges.WrapsX(), edges.WrapsY())
	}
	if err := worldPlugin.Hook(terrainSpeed(p.Res.Logic.Board)); err != nil {
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
		cells:    newCellSystem(p.Res.Logic.Board),
		standing: newStandingSystem(p.Res.Logic.Board, &p.standing),
		clock:    p.worldPlugin.Clock(),
	}
	p.module.standing.commands = p.worldPlugin.Commands()
	ctx.UseModule(p.module)
	return nil
}

// RunPlan, in the simulation, notices what effects did to the cells and reports where everyone
// stands; call it after collision's RunPlan.
func (p *Plugin) RunPlan(ctx goke.RunCtx, d time.Duration) { p.module.RunPlan(ctx, d) }

// WithRenderer builds the board renderer, drawing each cell's CellKind.SpriteID from atlas — or,
// given nil, from the board's own atlas of the kinds' Colors and drawn sprites (DefaultAtlas).
func (p *Plugin) WithRenderer(atlas render.AtlasSource) {
	if atlas == nil {
		atlas = p.DefaultAtlas()
	}
	p.Res.Render = &RenderState{ShowGridLines: true}
	p.renderer = newRenderer(p.Res.Logic.Board, atlas, p.Res.Render, p.Map)
	p.renderer.space = p.worldPlugin.Res.Config.Space
	p.renderer.Workers(p.workers)
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
func (p *Plugin) Atlas() render.AtlasSource {
	if p.renderer == nil {
		return nil
	}
	return p.renderer.atlas
}

// DefaultAtlas is an atlas of every kind in the dictionary, a cell's size each: its drawn sprite
// (CellKindDict.Draw) or its Color, grey for a kind of no colour. Call it once the kinds are
// created.
func (p *Plugin) DefaultAtlas() render.AtlasSource {
	w, h := p.Res.Logic.Board.CellBounds()
	size := int(math.Ceil(max(w, h)))
	atlas := render.NewAtlas()
	for _, k := range p.kinds.All() {
		if draw, ok := p.kinds.drawers[k.Name]; ok {
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

// Hook hosts rules (rule.On) of Standing, fired every step for every entity on the board; hook
// them before Use.
func (p *Plugin) Hook(rules ...plugin.Rule) error {
	for _, b := range rules {
		if err := p.standing.Add(b); err != nil {
			return fmt.Errorf("%w in %s — it takes a rule of Standing", err, p.Name())
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

// Top is the Map's: the height of c's corners as drawn and of its ground, for whoever lays
// something on the tiles.
func (p *Plugin) Top(c CellID) (corners [4]float32, level float32) { return p.mapping.Top(c) }

// Climb is the Map's: how many times as long the step from one cell to its neighbour takes
// whoever moves in d as on the flat.
func (p *Plugin) Climb(from, to CellID, d Domain) float64 { return p.mapping.Climb(from, to, d) }

// Least is the Map's: the smallest Climb for d.
func (p *Plugin) Least(d Domain) float64 { return p.mapping.Least(d) }

// Slope is the Map's: how many times as long moving at at towards dir takes whoever moves in d.
func (p *Plugin) Slope(at, dir geom.Vec, d Domain) float64 { return p.mapping.Slope(at, dir, d) }

// CellEntity is cell c's own entity, carrying its [Plot], [Ground], [Way] and [Crossing] for as
// long as the board lives, so an effect cast on it is an effect on the cell's terrain; false off
// the board or before Setup.
func (p *Plugin) CellEntity(c CellID) (uid.UID64, bool) { return p.Res.Logic.Board.CellEntity(c) }

// Occupancy returns the occupancy tracker this plugin was built with.
func (p *Plugin) Occupancy() Occupancy { return p.occupancy }

// CellKindDict returns this Plugin's registered CellKinds.
func (p *Plugin) CellKindDict() CellKindDict { return p.kinds }

// Seed sets the terrain applied when this Stage starts fresh — see Populate.
func (p *Plugin) Seed(layout Layout) { p.seeded = &layout }

// Populate applies the seeded Layout, kinds, ways and crossings, changing nothing and erroring on
// an unknown kind name.
func (p *Plugin) Populate() error {
	if p.seeded == nil {
		return nil
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

	ways := make([]Way, len(p.seeded.Ways))
	for i, e := range p.seeded.Ways {
		kind, err := resolve(e.Kind)
		if err != nil {
			return err
		}
		ways[i] = Way{Kind: kind, Width: e.Width, Links: e.Links, Fade: e.Fade, Mix: e.Mix}
	}
	crossings := make([]Crossing, len(p.seeded.Crossings))
	for i, e := range p.seeded.Crossings {
		kind, err := resolve(e.Kind)
		if err != nil {
			return err
		}
		crossings[i] = Crossing{Way{Kind: kind, Width: e.Width, Links: e.Links, Fade: e.Fade, Mix: e.Mix}}
	}

	brd := p.Res.Logic.Board
	if p.seeded.Default != "" {
		brd.SetAll(def)
	}
	for i, e := range p.seeded.Cells {
		brd.Set(e.Cell, cells[i])
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
