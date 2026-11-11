package underground

import (
	"fmt"
	"log"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/ground"
	"github.com/kjkrol/gram/plugins/underground/site"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/ui"
)

// Config is where structures may lie under the ground and the kind of tunnel a player's Tool
// digs, the first defined for none.
type Config struct {
	Rules site.Rules
	Digs  string
}

// Resources is the state of the plugin, saved with the game whole: the tunnels ordered and not
// yet dug, and the players' settings.
type Resources struct {
	Orders   []Order
	Settings Settings
}

// Settings is what the plugin lets the players adjust, saved with the game: whether the courses
// of what lies under the ground and the units in it are marked over the world, as they are
// unless hidden.
type Settings struct{ HideCourses bool }

// Order is a tunnel its player confirmed (Dig), dug at the next step of the simulation: the name
// of its kind (Types.Named) and its plan.
type Order struct {
	Kind string
	Plan site.Tunnel
}

// Plugin digs tunnels and chambers by one set of rules, a level's and a player's, as entities
// lying under the board's cells.
type Plugin struct {
	*world.Self
	Res Resources

	worldPlugin *world.Plugin
	board       *board.Plugin
	cfg         Config
	types       *Types
	shaper      ground.Shaper
	module      *module
	tubes       *tubes
	marker      *marker
	kept        *plugin.Kept[Resources]

	hands map[command.Issuer]*hand
	laid  []laid
	tick  uint64

	aims    command.Queue[Aim]
	marks   command.Queue[Mark]
	digs    command.Queue[Dig]
	cancels command.Queue[Cancel]
	courses command.Queue[Courses]
	write   Write
}

var (
	_ plugin.Plugin         = (*Plugin)(nil)
	_ plugin.CommandHandler = (*Plugin)(nil)
	_ plugin.Populator      = (*Plugin)(nil)
	_ plugin.Restorer       = (*Plugin)(nil)
	_ plugin.Tunable        = (*Plugin)(nil)
)

// NewPlugin builds the plugin of what lies under the ground over w and brd, a world with heights.
func NewPlugin(w *world.Plugin, brd *board.Plugin, cfg Config) *Plugin {
	if !w.Res.Config.Heights {
		panic("underground: the world has no heights (world.Config.Heights): nothing lies " +
			"under a flat one")
	}
	p := &Plugin{worldPlugin: w, board: brd, cfg: cfg, hands: map[command.Issuer]*hand{}}
	p.kept = plugin.Keep(&p.Res)
	p.write.p = p
	p.types = &Types{p: p}
	p.tubes = &tubes{p: p}
	p.marker = &marker{p: p}
	p.Self = world.NewSelf(w, p)
	return p
}

// WithShaper gives the plugin the ground a player's tunnel levels its heads on; without it a
// tunnel opens only where its heads' ground lies flat already.
func (p *Plugin) WithShaper(s ground.Shaper) *Plugin {
	p.shaper = s
	return p
}

// Types is the plugin's register of the kinds of structures.
func (p *Plugin) Types() *Types { return p.types }

// Populate holds the structures a level's file laid to the Rules; one that may not lie fails with
// its line.
func (p *Plugin) Populate() error {
	laid := p.laid
	p.laid = nil
	var g *boardGround
	rules := p.cfg.Rules
	rules.Reshape = 1e-3 * p.board.Read(0).Board().Grid.CellSpan()
	for _, l := range laid {
		if g == nil {
			g = newGround(p.board.Read(0).Board(), l.heights, 0)
		}
		g.allows = l.t.spec.Allows
		if err := l.check(g, rules); err != nil {
			return fmt.Errorf("underground: %s: %w", l.at, err)
		}
		l.note(g)
	}
	return nil
}

// laid is a structure a level's file laid: a tunnel or, room, a chamber
type laid struct {
	t       Type
	tunnel  site.Tunnel
	via     []cell.ID
	chamber site.Chamber
	depth   float32
	at      string
	heights func(geom.Vec) float32
}

func (l *laid) check(g *boardGround, r site.Rules) error {
	if l.t.room {
		_, err := site.Room(g, r, l.t.spec.Clear, l.depth, l.chamber.From, l.chamber.To)
		return err
	}
	r.Width = l.t.spec.Width
	_, err := site.Plan(g, r, l.t.spec.Clear, l.tunnel.From, l.tunnel.To, l.via...)
	return err
}

// note has g read the structure as lying, for the plans after it
func (l *laid) note(g *boardGround) {
	spec := l.t.spec
	if l.t.room {
		for _, c := range l.chamber.Cells {
			g.plan(c, cell.Floor{Allows: spec.Allows, Height: l.chamber.Floor - g.level(c),
				Clear: spec.Clear, Storey: l.chamber.Storey})
		}
		return
	}
	for k, c := range l.tunnel.Cells {
		g.plan(c, cell.Floor{Allows: spec.Allows, Height: l.tunnel.Floors[k] - g.level(c),
			Clear: spec.Clear, Width: spec.Width, Storey: l.tunnel.Storey})
	}
}

// tunnel is the tunnel t of the kind ty under g: its handle and its pieces
func (p *Plugin) tunnel(ty Type, t site.Tunnel, g *boardGround) (kind.Entry, []kind.Entry) {
	gr := g.Grid()
	run := append(append([]cell.ID{t.From}, t.Cells...), t.To)
	var out []kind.Entry
	lo, hi := gr.CellCenter(t.Cells[0]), gr.CellCenter(t.Cells[0])
	for k, c := range t.Cells {
		var links cell.Links
		for _, n := range [2]cell.ID{run[k], run[k+2]} {
			if way, ok := grid.Way(gr, c, n); ok {
				links |= 1 << way
			}
		}
		out = append(out, ty.bore.Entry(boreRow{At: c, Links: links,
			Height: t.Floors[k] - g.level(c), Storey: t.Storey}))
		m := gr.CellCenter(c)
		lo = geom.NewVec(min(lo.X, m.X), min(lo.Y, m.Y))
		hi = geom.NewVec(max(hi.X, m.X), max(hi.Y, m.Y))
	}
	half := gr.CellSpan() / 2
	whole := wholeRow{
		At:    geom.NewVec(lo.X-half, lo.Y-half),
		Size:  geom.NewVec(hi.X-lo.X+2*half, hi.Y-lo.Y+2*half),
		Floor: min(t.Ends[0], t.Ends[1]),
	}
	return ty.whole.Entry(whole), out
}

// chamber is the chamber ch of the kind ty under g: its handle and its pieces
func (p *Plugin) chamber(ty Type, ch site.Chamber, g *boardGround) (kind.Entry, []kind.Entry) {
	gr := g.Grid()
	var out []kind.Entry
	lo, hi := gr.CellCenter(ch.Cells[0]), gr.CellCenter(ch.Cells[0])
	for _, c := range ch.Cells {
		out = append(out, ty.bore.Entry(boreRow{At: c, Height: ch.Floor - g.level(c),
			Storey: ch.Storey, Wide: true}))
		m := gr.CellCenter(c)
		lo = geom.NewVec(min(lo.X, m.X), min(lo.Y, m.Y))
		hi = geom.NewVec(max(hi.X, m.X), max(hi.Y, m.Y))
	}
	half := gr.CellSpan() / 2
	whole := wholeRow{
		At:    geom.NewVec(lo.X-half, lo.Y-half),
		Size:  geom.NewVec(hi.X-lo.X+2*half, hi.Y-lo.Y+2*half),
		Floor: ch.Floor,
	}
	return ty.whole.Entry(whole), out
}

// rules are the Rules a structure of the kind t lies by
func (p *Plugin) rules(t Type) site.Rules {
	r := p.cfg.Rules
	r.Width = t.spec.Width
	if p.shaper == nil {
		r.Reshape = 1e-3 * p.board.Read(0).Board().Grid.CellSpan()
	}
	return r
}

func (p *Plugin) digging() Type {
	if p.cfg.Digs != "" {
		return p.types.Named(p.cfg.Digs)
	}
	for _, name := range p.types.names {
		if !p.types.rooms[name] {
			return p.types.Named(name)
		}
	}
	panic("underground: a tunnel in a player's hand, but no kind of tunnel is defined")
}

func (p *Plugin) ground(t Type) *boardGround {
	var at func(geom.Vec) float32
	if h := p.board.Read(0).Heights(); h != nil {
		at = h.At
	}
	return newGround(p.board.Read(0).Board(), at, t.spec.Allows)
}

func (p *Plugin) dig(o Order) {
	t := p.types.Named(o.Kind)
	g := p.ground(t)
	plan, err := site.Plan(g, p.rules(t), t.spec.Clear, o.Plan.From, o.Plan.To, turns(g.Grid(),
		o.Plan)...)
	if err != nil {
		log.Printf("underground: not dug: %v", err)
		return
	}
	if p.shaper != nil {
		for i, h := range [2]cell.ID{plan.From, plan.To} {
			if b := g.brd.Built(h); b.Levelled() {
				continue
			}
			if !p.shaper.Flatten(h, plan.Ends[i]) {
				x, y, _ := g.Grid().Coords(h)
				log.Printf("underground: not dug: the head at [%d, %d] could not be levelled "+
					"without breaking a road", x, y)
				return
			}
		}
	}
	whole, pieces := p.tunnel(t, plan, g)
	p.worldPlugin.Write().SpawnWhole(whole, pieces...)
}

// turns are the cells t turns at, which lead a plan through its cells again
func turns(g grid.Grid, t site.Tunnel) []cell.ID {
	run := append(append([]cell.ID{t.From}, t.Cells...), t.To)
	var via []cell.ID
	for k := 1; k+1 < len(run); k++ {
		in, _ := grid.Way(g, run[k-1], run[k])
		out, _ := grid.Way(g, run[k], run[k+1])
		if in != out {
			via = append(via, run[k])
		}
	}
	return via
}

func (p *Plugin) Name() string { return "gram.underground" }

func (p *Plugin) Install(ctx plugin.Installer) error {
	p.module = newModule(p)
	ctx.UseModule(p.module)
	ctx.Draws(p.tubes, p.marker)
	return nil
}

// Window is the window asking a player whether to dig the tunnel it marked out, shown while it
// asks.
func (p *Plugin) Window() *ui.Element {
	return ui.WindowOf(text{p: p, say: askText},
		ui.LabelOf(text{p: p, say: cellsText}),
		ui.Columns(
			ui.Share(1, ui.Button("Dig", Dig{})),
			ui.Share(1, ui.Button("Cancel", Cancel{})),
		),
	).Modal()
}

// Reason is a label saying why the tunnel in a player's hand may not lie where it points.
func (p *Plugin) Reason() *ui.Element {
	return ui.LabelOf(text{p: p, say: reasonText}).Passing()
}

// RunPlan carries out the players' commands at once and digs the tunnels confirmed in the
// simulation.
func (p *Plugin) RunPlan(ctx goke.RunCtx, d time.Duration) { p.module.RunPlan(ctx, d) }

func (p *Plugin) EventHandler() control.EventHandler { return nil }

// Serializable is the plugin's Resources, kept whole; a structure dug is its parts, saved as
// entities.
func (p *Plugin) Serializable() plugin.Serializable { return p.kept }

// Restore takes the Resources loaded.
func (p *Plugin) Restore() { p.kept.Restore() }

func (p *Plugin) String() string { return fmt.Sprintf("underground %v", p.types.names) }
