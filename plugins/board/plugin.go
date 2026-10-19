package board

import (
	"fmt"
	"log"
	"math"
	"reflect"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony"
	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/ceremony/effect"
	"github.com/kjkrol/ceremony/tag"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/entity/kind"
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
	"github.com/kjkrol/gram/plugins/light"
	"github.com/kjkrol/gram/plugins/motion"
	"github.com/kjkrol/gram/plugins/motion/steering"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/uid"
)

// Resources is board's published state; Render is nil until the atlas is closed.
type Resources struct {
	Logic struct {
		Board *Board
	}
	Render *look.RenderState
}

// Plugin wires a Board into a Game; it depends on world, and hands collision its solid ground.
type Plugin struct {
	*world.Self

	Res Resources

	occupancy cell.Occupancy
	renderer  *look.Renderer
	atlas     render.AtlasSource
	kinds     *terrain.Kinds
	seeded    *Layout
	level     *Level
	mapping   Map
	simple    *simpleMap
	covers    map[tag.Tag[effect.States]]render.SpriteID

	worldPlugin *world.Plugin
	motion      *motion.Plugin
	module      *module
	rules       *moments.Rules
	workers     int // 0 all the CPUs, 1 none
	grids       command.Queue[Grid]
	holds       command.Queue[Holds]
	holdsView   *occupancy.View
	holdsShown  bool
	gridShown   bool
	bridges     Bridges
	cellOf      map[string]func(row any) cell.ID
	tiles       []Tile
	surfaces    []Surface
	where       map[cell.ID]string // the file and line a level laid the entity on a cell by
	mini        miniature
}

// Bridges is what a bridge keeps to besides running straight and level from a head to a head:
// its deck Clearance over the ground under the middle of every cell between at least.
type Bridges struct{ Clearance float64 }

var _ plugin.Plugin = (*Plugin)(nil)
var _ plugin.Populator = (*Plugin)(nil)
var _ plugin.CommandHandler = (*Plugin)(nil)
var _ plugin.KindTemplater = (*Plugin)(nil)

// NewPlugin builds a board over g with the given occupancy, slowing worldPlugin's entities; a
// world without heights panics.
func NewPlugin(g grid.Grid, occupancy cell.Occupancy, worldPlugin *world.Plugin) *Plugin {
	if !worldPlugin.HasHeights() {
		panic("board: a board stands in a world with heights; set world.Config.Heights")
	}
	p := &Plugin{
		occupancy:   occupancy,
		worldPlugin: worldPlugin,
		kinds:       terrain.NewKinds(),
		gridShown:   true,
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
	p.simple = newSimpleMap(brd)
	p.mapping = p.simple
	brd.mapping = p.mapping
	if ws, ok := p.Board().Grid.(interface{ SetWrap(x, y bool) }); ok {
		edges := worldPlugin.Res.Config.Space.Edges
		ws.SetWrap(edges.WrapsX(), edges.WrapsY())
	}
	slope := func(at, dir geom.Vec, d cell.Domain) float64 { return brd.Map().Slope(at, dir, d) }
	p.rules = moments.New(brd.Grid, brd.cells, worldPlugin.Tick, slope, brd.altitude)
	if t := worldPlugin.Kinds().DefineTag[unit.States](unit.EnteredName); t != unit.Entered {
		panic(fmt.Sprintf("board: the units' markers have tags of their own before %q",
			unit.EnteredName))
	}
	if t := worldPlugin.Kinds().DefineTag[unit.States](unit.AirborneName); t != unit.Airborne {
		panic(fmt.Sprintf("board: the units' markers have tags of their own before %q",
			unit.AirborneName))
	}
	p.keepBridges()
	p.renderer = look.NewRenderer(brd, nil, mapRef{p}, worldPlugin.Res.Config.Space).
		WithSeen(worldPlugin.ViewFor).
		WithLighting(worldPlugin.Lighting)
	p.holdsView = p.newHoldsView()
	return p
}

// KindTemplates is what the board asks of the kinds a game defines: of a unit, where it starts
// and the domains it moves in; of a kind standing on it, lying as it is and casting a Baked
// shadow.
func (p *Plugin) KindTemplates(r *kind.Roster) {
	kind.Require[unit.At](r, motion.UnitTemplate, "board", "the cell it starts in")
	kind.Require[unit.Mover](r, motion.UnitTemplate, "board", "the domains it moves in")
	r.Default(motion.UnitTemplate, comp.Const(steering.Pace{Share: 1}))
	r.Default(motion.UnitTemplate, comp.Marks[unit.States]())
	r.Default(StandingTemplate, comp.Const(world.Lie{}))
	r.Default(StandingTemplate, comp.Const(light.Shadow{Cast: light.Baked}))
}

func (p *Plugin) newHoldsView() *occupancy.View {
	v := occupancy.NewView(p.Board().Grid, p.occupancy, p.holdAt)
	v.Width(DefaultHoldWidth)
	return v
}

func (p *Plugin) Name() string { return "gram.board" }

func (p *Plugin) Install(ctx plugin.Installer) error {
	p.module = &module{
		cells:     p.Board().cells.System(p.worldPlugin.Roster(), CellTemplate, p.mattering),
		built:     p.Board().cells.BuiltSystem(p.worldPlugin.Space),
		roster:    p.worldPlugin.Roster(),
		template:  CellTemplate,
		keep:      occupancy.KeepSystem(p.occupancy, p.Board().footing),
		standing:  p.rules.StandingSystem(),
		cellRules: p.rules.CellSystem(),
		clock:     p.worldPlugin.Clock(),
	}
	ctx.UseModule(p.module)
	ctx.Hosts(p.rules.Hosts()...)
	ctx.Draws(p.renderer, p.holdsView)
	ctx.Looks(p.dress)
	return nil
}

// dress declares the tiles' looks on the board's atlas, every sprite the cell's size across, and
// closes it, unless a game closed its own
func (p *Plugin) dress() {
	if p.atlas == nil {
		w, h := p.Board().Grid.CellBounds()
		p.NewAtlas(int(math.Ceil(max(w, h)))).Looks().Close()
	}
}

// RunPlan runs the board's step; call it after collision's RunPlan.
func (p *Plugin) RunPlan(ctx goke.RunCtx, d time.Duration) {
	p.grids.Drain(func(command.Issued[Grid]) { p.ShowGrid(!p.gridShown) })
	p.holds.Drain(func(command.Issued[Holds]) { p.ShowHolds(!p.holdsShown) })
	p.module.RunPlan(ctx, d)
}

// SetAtlas takes atlas as the board's looks for the board's renderer; a *Atlas's Close calls it,
// a foreign render.AtlasSource is given by hand. nil panics.
func (p *Plugin) SetAtlas(atlas render.AtlasSource) {
	if atlas == nil {
		panic("board: SetAtlas needs the game's atlas of the cell kinds (board.Plugin.NewAtlas)")
	}
	p.atlas = atlas
	p.renderer.SetAtlas(atlas)
	p.renderer.Shade(nil)
	if ba, ok := atlas.(*Atlas); ok {
		p.renderer.Shade(ba.shaded)
	}
	line, holds := DefaultGridLine(p.Board().Grid), float32(DefaultHoldWidth)
	if ba, ok := atlas.(*Atlas); ok {
		line, holds = ba.grid, ba.holds
	}
	p.Res.Render = p.renderer.State()
	p.Res.Render.ShowGridLines = p.gridShown
	p.Res.Render.GridLine = line.Color
	p.Res.Render.GridWidth = line.Width
	p.holdsView.Width(holds)
}

// ShowGrid draws the grid over the board, or not, as the Grid command (B) switches it: a look,
// not saved; shown from the start.
func (p *Plugin) ShowGrid(shown bool) {
	p.gridShown = shown
	if p.Res.Render != nil {
		p.Res.Render.ShowGridLines = shown
	}
}

// GridShown reports whether the grid is drawn.
func (p *Plugin) GridShown() bool { return p.gridShown }

// ShowHolds shows the cells the occupancy holds, each an outline in its holder's colour, or
// hides them: a look, not saved.
func (p *Plugin) ShowHolds(shown bool) {
	p.holdsShown = shown
	p.holdsView.Show(shown)
}

func (p *Plugin) holdAt(c cell.ID, at geom.Vec, deck bool) float64 {
	if deck {
		return p.Board().altitude(c) + p.Board().Built(c).Floor.Height
	}
	if h := p.Heights(); h != nil {
		return h.At(at)
	}
	return 0
}

// WithLog has the board write a line to l for a unit fallen where its domain may not be; call
// before Use.
func (p *Plugin) WithLog(l *log.Logger) *Plugin {
	p.rules.Log = l
	return p
}

// WithBridges has the board's bridges keep to b; zero has the deck no lower than the ground.
func (p *Plugin) WithBridges(b Bridges) *Plugin {
	p.bridges = b
	p.keepBridges()
	return p
}

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

// WithWorkers sets how many goroutines at most share a frame's tiles under a Parallel Dressing:
// 0 as many as there are CPUs, 1 none.
func (p *Plugin) WithWorkers(n int) *Plugin {
	p.workers = n
	p.renderer.Workers(n)
	return p
}

// Atlas is the sprite sheet the board's renderer draws the cells from; nil before it is closed.
func (p *Plugin) Atlas() render.AtlasSource { return p.atlas }

// EventHandler is nil: the board handles no input of its own.
func (p *Plugin) EventHandler() control.EventHandler { return nil }

// Serializable is nil: the terrain is the cells' entities, saved with the ECS.
func (p *Plugin) Serializable() plugin.Serializable { return nil }

// WithMap has the board drawn and priced by m in place of the simple map. Call before Use.
func (p *Plugin) WithMap(m Map) *Plugin {
	p.mapping = m
	p.Board().mapping = m
	return p
}

// Map is what the board is drawn and priced by: the simple map unless WithMap set another.
func (p *Plugin) Map() Map { return p.mapping }

// Climb is the Map's Climb.
func (p *Plugin) Climb(from, to cell.ID, d cell.Domain) float64 { return p.mapping.Climb(from, to, d) }

// Least is the Map's Least.
func (p *Plugin) Least(d cell.Domain) float64 { return p.mapping.Least(d) }

// CellEntity is cell c's own entity, so an effect cast on it is an effect on the cell's
// terrain; false off the board or before Setup.
func (p *Plugin) CellEntity(c cell.ID) (uid.UID64, bool) { return p.Board().CellEntity(c) }

// Occupancy is the occupancy the plugin was built with.
func (p *Plugin) Occupancy() cell.Occupancy { return p.occupancy }

// the marks of the effects the terrain changes under: those its cells show, those granting tags
// and those turning a knob of its own; not those turning only another's, as the air's
func (p *Plugin) mattering() tag.Tags[effect.States] {
	fx := p.worldPlugin.Effects()
	var matters tag.Tags[effect.States]
	for k := range fx.Count() {
		d := fx.Definition(uint8(k))
		mark := fx.Named(d.Name).Mark()
		foreign := !d.Shows && len(d.Alters) > 0
		for _, g := range d.Grants {
			// every effect grants its own marker
			own := g.Family == reflect.TypeFor[tag.Tags[effect.States]]() && g.Bits == 1<<mark
			foreign = foreign && own
		}
		for _, a := range d.Alters {
			foreign = foreign && !ownKnobs[a.Comp]
		}
		if !foreign {
			matters = matters.With(mark)
		}
	}
	return matters
}

// the knobs of a cell the terrain reads
var ownKnobs = map[reflect.Type]bool{
	reflect.TypeFor[cell.Ground](): true,
	reflect.TypeFor[cell.Floor]():  true,
	reflect.TypeFor[cell.Wall]():   true,
	reflect.TypeFor[cell.Rail]():   true,
}

// Covering is the slot of the board's atlas laid over the cells under the effect e, issued the
// first time it is asked for. Add what it shows to the atlas before it is closed; Slot.Under
// does both.
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

// World is the world the board lies over: its registers, for a figure defined over the board.
func (p *Plugin) World() *world.Plugin { return p.worldPlugin }

// CellKinds are this Plugin's registered kinds of cells.
func (p *Plugin) CellKinds() cell.Kinds { return p.kinds }

// Seed sets the terrain applied when the Stage starts fresh.
func (p *Plugin) Seed(layout Layout) {
	if err := p.worldPlugin.InSection("the board's layout seeded", section.Spawn); err != nil {
		panic("board: " + err.Error())
	}
	p.seeded = &layout
}

// Populate applies the seeded Layout; an unknown kind name, or an entity of the level laid that
// no Spawn took, is an error and changes nothing.
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
			continue
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
			brd.cells.Label(e.Cell, ceremony.LabelOf(e.Name, e.Group))
		}
	}
	p.cast()
	for i, e := range p.seeded.Ways {
		brd.SetWay(e.Cell, ways[i])
	}
	p.seeded = nil
	return nil
}

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

// Heights is the board's ground heights, its Map's; nil on a flat map.
func (p *Plugin) Heights() ground.Heights { return p.mapping.Heights() }

// Footing is where whoever moves over the board stands, the ground or a deck; nil on a flat
// map.
func (p *Plugin) Footing() ground.Surfaces {
	s, _ := p.mapping.Heights().(ground.Surfaces)
	return s
}

// Cover is what stands on the board and holds sight back; it is a ground.Readied too.
func (p *Plugin) Cover() ground.Cover { return p.Board().field }

// WithMotion has m move the board's units; a board without it has none. Call before Use.
func (p *Plugin) WithMotion(m *motion.Plugin) *Plugin {
	p.motion = m
	return p
}

// WithCollision makes the board's Solid cells the solid ground c pushes colliders out of; call
// before Use.
func (p *Plugin) WithCollision(c *collision.Plugin) *Plugin {
	c.WithField(p.Board().field)
	return p
}
