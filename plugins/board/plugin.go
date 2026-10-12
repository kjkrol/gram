package board

import (
	"fmt"
	"log"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/entity"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/entity/kind/comp"
	"github.com/kjkrol/gram/entity/tag"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugin/section"
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
	"github.com/kjkrol/gram/rule/effect"
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
	*world.Self // its own entity: its knobs, the roles it plays, the effects it is under

	Res Resources

	occupancy cell.Occupancy
	renderer  *look.Renderer
	atlas     render.AtlasSource // the renderer's
	kinds     *terrain.Kinds
	seeded    *Layout
	level     *Level // the level laid last, whose entities Spawn takes
	mapping   Map
	simple    *simpleMap // the board's own map, which lays the covers
	covers    map[tag.Tag[effect.States]]render.SpriteID

	worldPlugin *world.Plugin
	module      *module
	rules       *moments.Rules
	workers     int                 // how many goroutines at most share a frame's tiles: 0 all the CPUs, 1 none
	grids       control.Queue[Grid] // the Grid commands given, until the next tick
	bridges     Bridges
	// the cell an entity of each kind NewEntities defines stands on, read off its row, and the
	// file and line of a level the entity on a cell was laid by
	cellOf map[string]func(row any) cell.ID
	where  map[cell.ID]string
}

// Bridges is what a bridge — the cells whose Graded Floors run as bands joined along their links —
// keeps to besides running straight and level from a head to a head (a band from the middle of a
// flat tile at the deck into the bridge, its Links the way in alone): its deck Clearance over the
// ground under the middle of every cell between at least. A level laying one that breaks them
// fails with its file and line as the game begins; one built later is not built, said in the log.
type Bridges struct{ Clearance float64 }

var _ plugin.Plugin = (*Plugin)(nil)
var _ plugin.Populator = (*Plugin)(nil)
var _ plugin.CommandHandler = (*Plugin)(nil)

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
	p.Self = world.NewSelf(worldPlugin, p)
	p.kinds.Guard = func(name string) {
		if err := worldPlugin.InSection(fmt.Sprintf("cell kind %q defined", name), section.Kinds); err != nil {
			panic("board: " + err.Error())
		}
	}
	p.kinds.Played = worldPlugin.Kinds().Play
	brd := NewBoard(g)
	p.Res.Logic.Board = brd
	brd.setHeights(worldPlugin.HasHeights())
	p.simple = newSimpleMap(brd)
	p.mapping = p.simple
	brd.mapping = p.mapping
	if ws, ok := p.Board().Grid.(interface{ SetWrap(x, y bool) }); ok {
		edges := worldPlugin.Res.Config.Space.Edges
		ws.SetWrap(edges.WrapsX(), edges.WrapsY())
	}
	slope := func(at, dir geom.Vec, d cell.Domain) float64 { return brd.Map().Slope(at, dir, d) }
	p.rules = moments.New(brd.Grid, brd.cells, worldPlugin.Tick, slope, brd.altitude)
	worldPlugin.Roster().Unit.Default(comp.Const(steering.Pace{Share: 1}))
	if t := worldPlugin.Kinds().DefineTag[unit.States](unit.EnteredName); t != unit.Entered {
		panic(fmt.Sprintf("board: the units' markers have tags of their own before %q",
			unit.EnteredName))
	}
	if t := worldPlugin.Kinds().DefineTag[unit.States](unit.AirborneName); t != unit.Airborne {
		panic(fmt.Sprintf("board: the units' markers have tags of their own before %q",
			unit.AirborneName))
	}
	worldPlugin.Roster().Unit.Default(comp.Marks[unit.States]())
	p.keepBridges()
	return p
}

// =================================================================
// plugin.Plugin contract
// =================================================================

func (p *Plugin) Name() string { return "gram.board" }

// Install wires the cell entities, the occupancy's upkeep and the rules.
func (p *Plugin) Install(ctx plugin.Installer) error {
	p.module = &module{
		cells:     p.Board().cells.System(&p.worldPlugin.Roster().Cell),
		built:     p.Board().cells.BuiltSystem(p.worldPlugin.HasHeights(), p.worldPlugin.Space),
		template:  &p.worldPlugin.Roster().Cell,
		release:   occupancy.ReleaseSystem(p.occupancy),
		standing:  p.rules.StandingSystem(),
		cellRules: p.rules.CellSystem(),
		clock:     p.worldPlugin.Clock(),
	}
	ctx.UseModule(p.module)
	ctx.Hosts(p.rules.Hosts()...)
	return nil
}

// RunPlan, in the simulation, notices what effects did to the cells and reports where everyone
// stands; call it after collision's RunPlan.
func (p *Plugin) RunPlan(ctx goke.RunCtx, d time.Duration) {
	p.grids.Drain(func(control.Issued[Grid]) {
		if p.Res.Render != nil {
			p.Res.Render.ToggleShowGridLines()
		}
	})
	p.module.RunPlan(ctx, d)
}

// WithRenderer builds the board renderer, drawing each cell's kind's SpriteID from atlas, the
// game's look of every kind (NewAtlas); a board has no look of its own, and nil panics.
func (p *Plugin) WithRenderer(atlas render.AtlasSource) {
	if atlas == nil {
		panic("board: WithRenderer needs the game's atlas of the cell kinds (board.Plugin.NewAtlas)")
	}
	p.atlas = atlas
	p.renderer = look.NewRenderer(p.Board(), atlas, mapRef{p}, p.worldPlugin.Res.Config.Space)
	if ba, ok := atlas.(*Atlas); ok {
		p.renderer.Shade(ba.shaded)
	}
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

// WithBridges has the board's bridges keep to b; zero, as it starts, has the deck no lower than
// the ground.
func (p *Plugin) WithBridges(b Bridges) *Plugin {
	p.bridges = b
	p.keepBridges()
	return p
}

// keepBridges hands the terrain the rules its bridges keep to and how it reads the ground.
func (p *Plugin) keepBridges() {
	brd := p.Board()
	brd.cells.SetBridges(terrain.Bridges{
		Clearance: p.bridges.Clearance,
		Level:     brd.altitude,
		Ground: func(at geom.Vec) float64 {
			if h := p.Heights(); h != nil {
				return h.At(at)
			}
			return 0
		},
		Where: func(c cell.ID) string { return p.where[c] },
		Log: func(format string, args ...any) {
			if p.rules.Log != nil {
				p.rules.Log.Printf(format, args...)
				return
			}
			log.Printf(format, args...)
		},
	})
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

// =================================================================
// board-specific
// =================================================================

// WithMap has the board drawn and priced by m — a topography's — in place of the simple map.
// Call before Use.
func (p *Plugin) WithMap(m Map) *Plugin {
	p.mapping = m
	p.Board().mapping = m
	return p
}

// Map is what the board is drawn and priced by: the simple map unless WithMap set another.
func (p *Plugin) Map() Map { return p.mapping }

// Climb is the Map's: how many times as long the step from one cell to its neighbour takes
// whoever moves in d as on the flat.
func (p *Plugin) Climb(from, to cell.ID, d cell.Domain) float64 { return p.mapping.Climb(from, to, d) }

// Least is the Map's: the smallest Climb for d.
func (p *Plugin) Least(d cell.Domain) float64 { return p.mapping.Least(d) }

// CellEntity is cell c's own entity, carrying its [Plot], [Ground] and [Way] for as
// long as the board lives, so an effect cast on it is an effect on the cell's terrain; false off
// the board or before Setup.
func (p *Plugin) CellEntity(c cell.ID) (uid.UID64, bool) { return p.Board().CellEntity(c) }

// Occupancy returns the occupancy tracker this plugin was built with.
func (p *Plugin) Occupancy() cell.Occupancy { return p.occupancy }

// Covering is the slot of the board's atlas laid over the cells under the effect e, along the line
// those cells draw, not along their edges — snow on the ground, ice on the water: issued the
// first time it is asked for, the same after. Add what it shows to the board's atlas, before
// WithRenderer — Slot.Under does both. The simple map lays it; a cell stays the kind it is.
func (p *Plugin) Covering(e effect.Effect) render.SpriteID {
	if id, ok := p.covers[e.Mark()]; ok {
		return id
	}
	if p.covers == nil {
		p.covers = map[tag.Tag[effect.States]]render.SpriteID{}
	}
	id := p.kinds.NewSprite()
	p.covers[e.Mark()] = id
	p.simple.dressing.Cover(e.Mark(), id)
	e.Shows()
	return id
}

// Board is the board: its terrain, read and written.
func (p *Plugin) Board() *Board { return p.Res.Logic.Board }

// CellKinds are this Plugin's registered kinds of cells.
func (p *Plugin) CellKinds() cell.Kinds { return p.kinds }

// Seed sets the terrain applied when this Stage starts fresh — see Populate.
func (p *Plugin) Seed(layout Layout) {
	if err := p.worldPlugin.InSection("the board's layout seeded", section.Spawn); err != nil {
		panic("board: " + err.Error())
	}
	p.seeded = &layout
}

// Populate applies the seeded Layout — kinds, the cells' roles, names and groups, ways —
// changing nothing and erroring on an unknown kind name, or on an entity of the level laid that
// no Spawn took.
func (p *Plugin) Populate() error {
	if level := p.level; level != nil {
		p.level = nil
		if err := level.unspawned(); err != nil {
			return err
		}
	}
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

	brd := p.Board()
	if p.seeded.Default != "" {
		brd.SetAll(def)
	}
	for i, e := range p.seeded.Cells {
		if e.Kind != "" {
			brd.Set(e.Cell, cells[i])
		}
		if e.Name != "" || e.Group != "" {
			brd.cells.Label(e.Cell, entity.LabelOf(e.Name, e.Group))
		}
	}
	p.cast()
	for i, e := range p.seeded.Ways {
		brd.SetWay(e.Cell, ways[i])
	}
	p.seeded = nil
	return nil
}

// cast gives every cell the roles it plays, as the Layout lays it: its kind's — the entry's, or
// the Default's where the entry keeps it — and the entry's own on top.
func (p *Plugin) cast() {
	brd := p.Board()
	def := p.kinds.Plays(p.seeded.Default)
	if def != 0 {
		brd.Grid.EachCell(func(c cell.ID) { brd.cells.Cast(c, def) })
	}
	for _, e := range p.seeded.Cells {
		roles := def
		if e.Kind != "" {
			roles = p.kinds.Plays(e.Kind)
		}
		roles |= e.Roles
		if roles != 0 || e.Kind != "" {
			brd.cells.Cast(e.Cell, roles)
		}
	}
}

// =================================================================
// the ground the others meet
// =================================================================

// Heights is the board's ground heights: its Map's, nil on a flat map.
func (p *Plugin) Heights() ground.Heights { return p.mapping.Heights() }

// Surfaces is where whoever moves over the board stands — the ground or a deck — when its Map's
// heights know; nil on a flat map.
func (p *Plugin) Surfaces() ground.Surfaces {
	s, _ := p.mapping.Heights().(ground.Surfaces)
	return s
}

// Cover is what stands on the board and holds sight back — walls, forests — walked along a ray;
// it is a ground.Readied too.
func (p *Plugin) Cover() ground.Cover { return p.Board().field }

// WithCollision makes the board's Solid cells the solid ground c pushes colliders out of, and the
// ground a kind does not take (Overhang) what c never pushes one over; call before Use.
func (p *Plugin) WithCollision(c *collision.Plugin) *Plugin {
	c.WithField(p.Board().field)
	return p
}
