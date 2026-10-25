package board

import (
	"fmt"
	"log"
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
	"github.com/kjkrol/gram/internal/f32"
	"github.com/kjkrol/gram/internal/parallel"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugin/section"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
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
)

// Resources is board's published state; Render is nil until the atlas is closed.
type Resources struct {
	Render   *look.RenderState
	Settings Settings
}

// Plugin wires a Board into a Game; it depends on world, and hands collision its solid ground.
type Plugin struct {
	*world.Self

	Res Resources

	board     *board
	reads     parallel.Reads[*Read]
	read      Read
	write     Write
	occupancy cell.Occupancy
	renderer  *look.Renderer
	atlas     render.AtlasSource
	kinds     *terrain.Kinds
	seeded    *Layout
	level     *Level
	simple    *simpleMap
	covers    map[tag.Tag[effect.States]]render.SpriteID

	worldPlugin *world.Plugin
	motion      *motion.Plugin
	module      *module
	rules       *moments.Rules
	grids       command.Queue[Grid]
	holds       command.Queue[Holds]
	holdsView   *occupancy.View
	kept        *plugin.Kept[Settings]
	bridges     Bridges
	cellOf      map[string]func(row any) cell.ID
	tiles       []Tile
	surfaces    []Surface
	where       map[cell.ID]string // the file and line a level laid the entity on a cell by
	mini        miniature
}

// Bridges is what a bridge keeps to besides running straight and level from a head to a head:
// its deck Clearance over the ground under the middle of every cell between at least.
type Bridges struct{ Clearance float32 }

var _ plugin.Publisher = (*Plugin)(nil)
var _ plugin.Plugin = (*Plugin)(nil)
var _ plugin.Populator = (*Plugin)(nil)
var _ plugin.CommandHandler = (*Plugin)(nil)
var _ plugin.KindTemplater = (*Plugin)(nil)

// NewPlugin builds a board over g with the given occupancy, slowing worldPlugin's entities; a
// world without heights panics.
func NewPlugin(g grid.Grid, occupancy cell.Occupancy, worldPlugin *world.Plugin) *Plugin {
	if !worldPlugin.Read(0).HasHeights() {
		panic("board: a board stands in a world with heights; set world.Config.Heights")
	}
	p := &Plugin{
		occupancy:   occupancy,
		worldPlugin: worldPlugin,
		kinds:       terrain.NewKinds(),
	}
	p.Res.Settings.Grid = true
	p.kept = plugin.Keep(&p.Res.Settings)
	p.Self = world.NewSelf(worldPlugin, p)
	p.kinds.Guard = func(name string) {
		if err := worldPlugin.InSection(fmt.Sprintf("cell kind %q defined", name), section.Kinds); err != nil {
			panic("board: " + err.Error())
		}
	}
	p.kinds.Played = worldPlugin.Kinds().Play
	brd := newBoard(g)
	p.board, p.simple = brd, brd.mapping.(*simpleMap)
	p.read, p.write = Read{p: p, board: &brd.read}, Write{p: p}
	p.reads = parallel.Reads[*Read]{&p.read}
	if ws, ok := brd.Grid.(interface{ SetWrap(x, y bool) }); ok {
		edges := worldPlugin.Res.Config.Space.Edges
		ws.SetWrap(edges.WrapsX(), edges.WrapsY())
	}
	slope := func(at, dir geom.Vec, d cell.Domain) float64 { return brd.mapping.Slope(at, dir, d) }
	p.rules = moments.New(brd.Grid, brd.cells, worldPlugin.Read(0).Tick, slope, brd.read.altitude)
	if t := worldPlugin.Kinds().DefineTag[unit.States](unit.EnteredName); t != unit.Entered {
		panic(fmt.Sprintf("board: the units' markers have tags of their own before %q",
			unit.EnteredName))
	}
	if t := worldPlugin.Kinds().DefineTag[unit.States](unit.AirborneName); t != unit.Airborne {
		panic(fmt.Sprintf("board: the units' markers have tags of their own before %q",
			unit.AirborneName))
	}
	p.keepBridges()
	p.renderer = look.NewRenderer(brd.drawnBoard(), nil, drawnMap{brd.drawnBoard()},
		worldPlugin.Res.Config.Space).
		WithSeen(worldPlugin.Read(0).ViewFor).
		WithLights(worldPlugin.Read(0).Lights)
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
	v := occupancy.NewView(p.board.Grid, p.occupancy, p.holdAt, p.worldPlugin.Read(0).Views)
	v.Width(DefaultHoldWidth)
	return v
}

func (p *Plugin) Name() string { return "gram.board" }

func (p *Plugin) Install(ctx plugin.Installer) error {
	p.module = &module{
		cells:     p.board.cells.System(p.worldPlugin.Roster(), CellTemplate, p.mattering),
		built:     p.board.cells.BuiltSystem(p.worldPlugin.Write().Space),
		roster:    p.worldPlugin.Roster(),
		template:  CellTemplate,
		keep:      occupancy.KeepSystem(p.occupancy, p.board.read.footing),
		standing:  p.rules.StandingSystem(),
		cellRules: p.rules.CellSystem(),
		clock:     p.worldPlugin.Write().Clock(),
	}
	p.module.step = p.module.simulate
	ctx.UseModule(p.module)
	ctx.Hosts(p.rules.Hosts()...)
	ctx.Draws(p.renderer, p.holdsView)
	ctx.Looks(p.dress)
	p.reads = parallel.NewReads(&p.read, ctx.Pool())
	return nil
}

// Read reads the board's state on worker w of the game's pool, 0 the game's own goroutine; the
// other workers' reads come with Use.
func (p *Plugin) Read(w int) *Read { return p.reads[w] }

// Write changes the board's state, on the game's goroutine.
func (p *Plugin) Write() *Write { return &p.write }

// dress declares the tiles' looks on the board's atlas, every sprite the cell's size across, and
// closes it, unless a game closed its own
func (p *Plugin) dress() {
	if p.atlas == nil {
		w, h := p.board.Grid.CellBounds()
		p.NewAtlas(int(f32.Ceil(max(w, h)))).Looks().Close()
	}
}

// RunPlan runs the board's step; call it after collision's RunPlan.
// Publish hands the frame drawn next the board's cells as they stand.
func (p *Plugin) Publish() { p.board.drawn.publish() }

// Swap has the cells published last drawn.
func (p *Plugin) Swap() { p.board.drawn.swap() }

func (p *Plugin) RunPlan(ctx goke.RunCtx, d time.Duration) {
	p.grids.Drain(func(i command.Issued[Grid]) { p.write.ShowGrid(i.Command.Shown) })
	p.holds.Drain(func(i command.Issued[Holds]) { p.write.ShowHolds(i.Command.Shown) })
	p.module.RunPlan(ctx, d)
}

func (p *Plugin) holdAt(c cell.ID, at geom.Vec, deck bool) float32 {
	if deck {
		return p.board.read.altitude(c) + p.board.read.Built(c).Floor.Height
	}
	if h := p.board.mapping.Heights(); h != nil {
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
	brd := p.board
	brd.cells.SetBridges(terrain.Bridges{
		Clearance: p.bridges.Clearance,
		Level:     brd.read.altitude,
		Ground: func(at geom.Vec) float32 {
			if h := brd.mapping.Heights(); h != nil {
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

// EventHandler is nil: the board handles no input of its own.
func (p *Plugin) EventHandler() control.EventHandler { return nil }

// Serializable is the board's Settings, kept whole; the terrain is the cells' entities, saved
// with the ECS.
func (p *Plugin) Serializable() plugin.Serializable { return p.kept }

// Restore takes the Settings loaded and shows what they say.
func (p *Plugin) Restore() {
	p.kept.Restore()
	p.write.ShowGrid(p.Res.Settings.Grid)
	p.write.ShowHolds(p.Res.Settings.Holds)
}

// WithMap has the board drawn and priced by m in place of the simple map. Call before Use.
func (p *Plugin) WithMap(m Map) *Plugin {
	p.board.mapping = m
	return p
}

// the marks of the effects the terrain changes under: those its cells show, those granting tags
// and those turning a knob of its own; not those turning only another's, as the air's
func (p *Plugin) mattering() tag.Tags[effect.States] {
	fx := p.worldPlugin.Write().Effects()
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

	brd := p.board
	if p.seeded.Default != "" {
		brd.write.SetAll(def)
	}
	for i, e := range p.seeded.Cells {
		if e.Kind != "" {
			brd.write.Set(e.Cell, cells[i])
		}
		if e.Name != "" || e.Group != "" {
			brd.cells.Label(e.Cell, ceremony.LabelOf(e.Name, e.Group))
		}
	}
	p.cast()
	for i, e := range p.seeded.Ways {
		brd.write.SetWay(e.Cell, ways[i])
	}
	p.seeded = nil
	return nil
}

func (p *Plugin) cast() {
	brd := p.board
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

// WithMotion has m move the board's units; a board without it has none. Call before Use.
func (p *Plugin) WithMotion(m *motion.Plugin) *Plugin {
	p.motion = m
	return p
}

// WithCollision makes the board's Solid cells the solid ground c pushes colliders out of; call
// before Use.
func (p *Plugin) WithCollision(c *collision.Plugin) *Plugin {
	c.WithField(p.board.field)
	return p
}
