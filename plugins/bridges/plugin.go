package bridges

import (
	"fmt"
	"github.com/kjkrol/gram/plugin"
	"log"
	"math"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/ground"
	"github.com/kjkrol/gram/plugins/bridges/site"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/ui"
)

// Tool is a bridge in a player's hand, for control.Take.
const Tool control.Tool = "bridge"

// Config is where bridges may stand (Longest 3 where Rules say none) and the kind of bridge a
// player's Tool builds, the first defined for none.
type Config struct {
	Rules  site.Rules
	Builds string
}

// Plugin builds bridges by one set of rules, a level's and a player's, as entities standing on
// the board.
type Plugin struct {
	*world.Self

	worldPlugin *world.Plugin
	board       *board.Plugin
	cfg         Config
	types       *Types
	shaper      ground.Shaper
	module      *module
	renderer    *renderer

	hands   map[command.Issuer]*hand
	pending []pending
	laid    []laid
	tick    uint64

	aims    command.Queue[Aim]
	marks   command.Queue[Mark]
	builds  command.Queue[Build]
	cancels command.Queue[Cancel]
}

var (
	_ plugin.Plugin         = (*Plugin)(nil)
	_ plugin.CommandHandler = (*Plugin)(nil)
	_ plugin.Populator      = (*Plugin)(nil)
)

// NewPlugin builds the bridges plugin over w and brd.
func NewPlugin(w *world.Plugin, brd *board.Plugin, cfg Config) *Plugin {
	cfg.Rules.Heights = w.HasHeights()
	if cfg.Rules.Longest <= 0 {
		cfg.Rules.Longest = 3
	}
	p := &Plugin{worldPlugin: w, board: brd, cfg: cfg, hands: map[command.Issuer]*hand{}}
	p.types = &Types{p: p}
	p.Self = world.NewSelf(w, p)
	return p
}

// WithShaper gives the plugin the ground a player's bridge levels its heads on; without it a
// bridge stands only where its heads lie flat at its deck already.
func (p *Plugin) WithShaper(s ground.Shaper) *Plugin {
	p.shaper = s
	return p
}

// Types is the plugin's register of the kinds of bridges.
func (p *Plugin) Types() *Types { return p.types }

// Spawn builds every bridge a level's file has, a line `{kind: <a kind of bridge>, from: [x, y],
// to: [x, y]}`, its deck at its first head's level. One the Rules refuse fails Populate with its
// line.
func (p *Plugin) Spawn(level *board.Level) error {
	g := newGround(p.board.Board(), level.Heights(), 0)
	gr := g.Grid()
	return board.Each(level, func(name string, r Row, at string) error {
		t := p.types.Named(name)
		way, spans, err := site.Line(gr, r.From, r.To, p.cfg.Rules.Longest)
		if err != nil {
			return err
		}
		b := site.Bridge{From: r.From, To: r.To, Way: way, Deck: g.level(r.From), Spans: spans}
		if p.cfg.Rules.Heights {
			b.Piers = site.Piers(gr, spans)
		}
		p.laid = append(p.laid, laid{t: t, plan: b, at: at, heights: level.Heights()})
		p.worldPlugin.Seed(p.parts(t, b, g)...)
		return nil
	})
}

// Populate holds the bridges a level's file laid to the Rules; one that may not stand fails with
// its line.
func (p *Plugin) Populate() error {
	laid := p.laid
	p.laid = nil
	var g *boardGround
	rules := p.cfg.Rules
	rules.Reshape = 1e-3 * float64(p.board.Board().Grid.CellSpan())
	for _, l := range laid {
		if g == nil {
			g = newGround(p.board.Board(), l.heights, 0)
		}
		g.allows = l.t.spec.Allows
		if _, err := site.Plan(g, rules, l.plan.From, l.plan.To, l.plan.Deck); err != nil {
			return fmt.Errorf("bridges: %s: %w", l.at, err)
		}
		g.laid = append(g.laid, l.plan)
	}
	return nil
}

type laid struct {
	t       Type
	plan    site.Bridge
	at      string
	heights func(geom.Vec) float64
}

func (p *Plugin) parts(t Type, b site.Bridge, g *boardGround) []kind.Entry {
	gr := g.Grid()
	step := gr.Step(b.Way)
	length := math.Hypot(step.X, step.Y)
	dir := geom.NewVec(step.X/length, step.Y/length)
	piece := func(c cell.ID, links cell.Links, d geom.Vec) kind.Entry {
		height := 0.0
		if p.worldPlugin.HasHeights() {
			height = b.Deck - g.level(c)
		}
		return t.deck.Entry(deckRow{At: c, Links: links, Height: height, Dir: d})
	}
	back := gr.Opposite(b.Way)
	out := []kind.Entry{
		piece(b.From, 1<<b.Way, dir),
		piece(b.To, 1<<back, geom.NewVec(-dir.X, -dir.Y)),
	}
	for _, s := range b.Spans {
		out = append(out, piece(s, 1<<b.Way|1<<back, dir))
	}
	if t.piers {
		for _, at := range b.Piers {
			row := pierRow{At: at, Bottom: g.at(at), Top: b.Deck - t.spec.Thickness}
			out = append(out, t.pier.Entry(row))
		}
	}
	return out
}

func (p *Plugin) rules() site.Rules {
	r := p.cfg.Rules
	if p.shaper == nil {
		r.Reshape = 1e-3 * float64(p.board.Board().Grid.CellSpan())
	}
	return r
}

func (p *Plugin) building() Type {
	if p.cfg.Builds != "" {
		return p.types.Named(p.cfg.Builds)
	}
	if len(p.types.names) == 0 {
		panic("bridges: a bridge in a player's hand, but no kind of bridge is defined")
	}
	return p.types.Named(p.types.names[0])
}

func (p *Plugin) ground(t Type) *boardGround {
	var at func(geom.Vec) float64
	if p.worldPlugin.HasHeights() {
		if h := p.board.Heights(); h != nil {
			at = h.At
		}
	}
	return newGround(p.board.Board(), at, t.spec.Allows)
}

func (p *Plugin) build(b pending) {
	g := p.ground(b.t)
	plan, err := site.Plan(g, p.rules(), b.plan.From, b.plan.To, b.plan.Deck)
	if err != nil {
		log.Printf("bridges: not built: %v", err)
		return
	}
	if p.cfg.Rules.Heights && p.shaper != nil {
		for _, h := range []cell.ID{plan.From, plan.To} {
			if !p.shaper.Flatten(h, plan.Deck) {
				x, y, _ := g.Grid().Coords(h)
				log.Printf("bridges: not built: the head at [%d, %d] could not be levelled "+
					"without breaking a road", x, y)
				return
			}
		}
	}
	p.worldPlugin.Spawn(p.parts(b.t, plan, g)...)
}

// Window is the window asking a player whether to build the bridge it marked out, shown while it
// asks.
func (p *Plugin) Window() *ui.Element {
	return ui.WindowOf(text{p: p, say: askText},
		ui.LabelOf(text{p: p, say: spansText}),
		ui.Columns(
			ui.Share(1, ui.Button("Build", Build{})),
			ui.Share(1, ui.Button("Cancel", Cancel{})),
		),
	).Modal()
}

// Reason is a label saying why the bridge in a player's hand may not stand where it points.
func (p *Plugin) Reason() *ui.Element { return ui.LabelOf(text{p: p, say: reasonText}) }

func (p *Plugin) Name() string { return "gram.bridges" }

func (p *Plugin) Install(ctx plugin.Installer) error {
	p.module = newModule(p)
	ctx.UseModule(p.module)
	return nil
}

// RunPlan carries out the players' commands at once and builds the bridges confirmed in the
// simulation.
func (p *Plugin) RunPlan(ctx goke.RunCtx, d time.Duration) { p.module.RunPlan(ctx, d) }

// WithRenderer builds the renderer of the bridges in the players' hands.
func (p *Plugin) WithRenderer(render.AtlasSource) { p.renderer = &renderer{p: p} }

// Renderer draws the bridges in the players' hands; nil before WithRenderer.
func (p *Plugin) Renderer() render.Layer {
	if p.renderer == nil {
		return nil
	}
	return p.renderer
}

func (p *Plugin) EventHandler() control.EventHandler { return nil }

// Serializable is nil: a bridge is its parts, saved as entities.
func (p *Plugin) Serializable() plugin.Serializable { return nil }

func (p *Plugin) String() string { return fmt.Sprintf("bridges %v", p.types.names) }
