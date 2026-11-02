package roads

import (
	"fmt"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/ceremony/host"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/ground"
	"github.com/kjkrol/gram/plugins/roads/site"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/ui"
)

// Tool is a road in a player's hand, for control.Take.
const Tool control.Tool = "road"

// Config is what the game allows a road (Rules), the kind of road the Tool lays (Kind, a kind
// of cell defined on the board, Graded or a track) and how wide (a fifth of a cell for none),
// the Tariff pricing a plan (nil shows its quantities alone), whether the tool starts levelling
// the ground (Levelling) and whether taking it holds the game in the tactical pause (InPause).
type Config struct {
	Kind      string
	Width     float32
	Rules     site.Rules
	Tariff    func(e site.Estimate) float32
	Levelling bool
	// MaxStep is the topography's Shaping.MaxStep, so a plan levels the ground as the shaping
	// has it follow; 0 has nothing follow.
	MaxStep float32
	InPause bool
}

// Settings is what roads lets the players adjust, saved with the game: whether the tool levels
// the ground under the roads it lays.
type Settings struct{ Levelling bool }

// Resources is the state of roads, saved with the game whole: its Settings, the roads ordered and
// not yet built, and the receipts of those built that Read.Built has not handed out yet.
type Resources struct {
	Settings Settings
	Orders   []Order
	Receipts []Receipt
}

// Order is a road its player confirmed (Build), laid at the next step of the simulation: whose
// hand laid it out, what it takes and what it costs.
type Order struct {
	Issuer   command.Issuer
	Estimate site.Estimate
	Price    float32
}

// Receipt is a road built: whose hand laid it, what it took and what it cost.
type Receipt struct {
	Issuer   command.Issuer
	Estimate site.Estimate
	Price    float32
}

// Plugin lays roads by one set of rules, package site's, with a road in a player's hand.
type Plugin struct {
	*world.Self
	Res Resources

	worldPlugin *world.Plugin
	board       *board.Plugin
	cfg         Config
	shaper      ground.Shaper
	module      *module
	renderer    *renderer
	kept        *plugin.Kept[Resources]
	read        Read
	newRouter   routing
	router      router
	levelled    bool // the levelling the router was made for
	plan        planning
	ground      boardGround

	estimated host.StepRules[Estimated]
	built     host.StepRules[Built]

	hands   []*hand
	tick    uint64
	paused  uint64 // the tick the tool last asked for the tactical pause
	leg     []cell.ID
	outline []geom.Vec
	heights []float32

	aims     command.Queue[Aim]
	marks    command.Queue[Mark]
	pieces   command.Queue[Piece]
	builds   command.Queue[Build]
	cancels  command.Queue[Cancel]
	refusals command.Queue[Refuse]
	levels   command.Queue[Levelling]
}

// router routes a road's legs one by one: site's Planner, or a test's in its place
type router interface {
	Route(from, to cell.ID, dst []cell.ID) ([]cell.ID, error)
}

// routing makes the router of a road over g, planning plans one: site.NewPlanner and site.Plan,
// or what a test plans with in their place
type (
	routing  func(g site.Ground, r site.Rules, graded, levelling bool) router
	planning func(g site.Ground, r site.Rules, cells []cell.ID, kind string, width float32,
		graded, levelling bool, step float32, extra []cell.Links) (site.Estimate, error)
)

func planner(g site.Ground, r site.Rules, graded, levelling bool) router {
	return site.NewPlanner(g, r, graded, levelling, nil)
}

var (
	_ plugin.Plugin         = (*Plugin)(nil)
	_ plugin.CommandHandler = (*Plugin)(nil)
	_ plugin.Tunable        = (*Plugin)(nil)
	_ plugin.Restorer       = (*Plugin)(nil)
)

// NewPlugin lays roads over w and brd as cfg says.
func NewPlugin(w *world.Plugin, brd *board.Plugin, cfg Config) *Plugin {
	p := &Plugin{worldPlugin: w, board: brd, cfg: cfg, newRouter: planner, plan: site.Plan}
	p.Res.Settings.Levelling = cfg.Levelling
	p.kept = plugin.Keep(&p.Res)
	p.read = Read{p: p}
	p.ground = boardGround{p: p, standing: map[cell.ID]struct{}{}}
	p.renderer = &renderer{p: p}
	p.Self = world.NewSelf(w, p)
	return p
}

// WithShaper gives the plugin the ground a road's levelling grades; without it a road lies only
// where the ground fits it already.
func (p *Plugin) WithShaper(s ground.Shaper) *Plugin {
	p.shaper = s
	return p
}

// Settings is the one setting, the ground levelled under the roads laid (plugin.Tunable).
func (p *Plugin) Settings() []control.Setting {
	return []control.Setting{control.Switch{
		Label: "Level the ground under roads",
		Get:   p.read.Levelling,
		Set:   func(on bool) command.Command { return Levelling{On: on} },
	}}
}

// Window is the window asking a player whether to build the road it planned, with what it takes
// and what it costs, shown while it asks.
func (p *Plugin) Window() *ui.Element {
	return ui.WindowOf(text{p: p, say: askText},
		ui.LabelOf(text{p: p, say: estimateText}),
		ui.Columns(
			ui.Share(1, ui.Button("Build", Build{})),
			ui.Share(1, ui.Button("Cancel", Cancel{})),
		),
	).Modal()
}

// Reason is a label saying why the road in a player's hand may not lie or be built as planned.
func (p *Plugin) Reason() *ui.Element {
	return ui.LabelOf(text{p: p, say: reasonText}).Passing()
}

func (p *Plugin) Name() string { return "gram.roads" }

// Install registers what the plugin draws and the hosts of its moments.
func (p *Plugin) Install(ctx plugin.Installer) error {
	p.module = newModule(p)
	ctx.UseModule(p.module)
	ctx.Hosts(&p.estimated, &p.built)
	ctx.Draws(p.renderer)
	return nil
}

// RunPlan carries out the players' commands at once, the rules of Estimated with them, and has
// the roads confirmed built in the next step of the simulation. Call it after the board's and
// the topography's RunPlan.
func (p *Plugin) RunPlan(ctx goke.RunCtx, d time.Duration) { p.module.RunPlan(ctx, d) }

// EventHandler is nil: a player's bindings issue the plugin's commands.
func (p *Plugin) EventHandler() control.EventHandler { return nil }

// Serializable is the plugin's Resources, kept whole; the roads laid are the board's cells.
func (p *Plugin) Serializable() plugin.Serializable { return p.kept }

// Restore takes the Resources loaded.
func (p *Plugin) Restore() { p.kept.Restore() }

func (p *Plugin) kind() cell.Kind {
	k, ok := p.board.Read(0).CellKinds().Get(p.cfg.Kind)
	if !ok {
		panic(fmt.Sprintf("roads: a road in a player's hand, but its kind %q is no kind of cell "+
			"defined on the board", p.cfg.Kind))
	}
	return k
}

func (p *Plugin) width() float32 {
	if p.cfg.Width > 0 {
		return p.cfg.Width
	}
	return p.board.Read(0).Board().CellSpan() / 5
}

// levelling reports whether the plans level the ground: the players' setting, where there is
// ground to level
func (p *Plugin) levelling() bool { return p.Res.Settings.Levelling && p.shaper != nil }

// routerOf is what routes the legs of the roads in hand over g, made anew as the levelling is
// switched: the rules and the kind of road stay
func (p *Plugin) routerOf(g *boardGround) router {
	if levelling := p.levelling(); p.router == nil || p.levelled != levelling {
		p.router, p.levelled = p.newRouter(g, p.cfg.Rules, p.kind().Graded, levelling), levelling
	}
	return p.router
}
