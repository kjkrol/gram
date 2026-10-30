package navigation

import (
	"cmp"
	"fmt"
	"math"
	"time"

	"github.com/kjkrol/ceremony/act"
	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/ceremony/host"
	"github.com/kjkrol/ceremony/tag"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/internal/f32"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/driving"
	"github.com/kjkrol/gram/plugins/motion"
	"github.com/kjkrol/gram/plugins/motion/steering"
	"github.com/kjkrol/gram/plugins/navigation/internal/routes"
	"github.com/kjkrol/gram/plugins/navigation/internal/routing"
	"github.com/kjkrol/gram/plugins/navigation/internal/spacing"
	"github.com/kjkrol/gram/plugins/navigation/internal/traffic"
	"github.com/kjkrol/gram/plugins/selection"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/sector"
)

// Plugin moves entities along a MoveOrder's path across a board, re-pathing when terrain changes,
// and defines the MoveTo command.
type Plugin struct {
	*world.Self

	boardPlugin *board.Plugin
	worldPlugin *world.Plugin
	sel         *selection.Plugin
	selected    tag.Tag[selection.Family]

	board  *board.BoardRead
	module *module

	moves   command.Queue[MoveTo]
	marches command.Queue[March]
	looks   command.Queue[LookAt]
	routes  command.Queue[Routes]
	onwards command.Queue[Onward]
	given   traffic.GivenQueues
	finder  *spacing.Finder

	coordinator *routing.Coordinator

	touches  host.PairRules[Touch]
	crowd    []act.Rule
	crowdSet bool

	pathRenderer *routes.Renderer
	collision    *collision.Plugin
	spacing      Spacing
	routers      int
	hierarchy    Hierarchy
	handKeeping  traffic.HandKeeping
	read         Read
	write        Write
	kept         *plugin.Kept[Settings]

	Res Resources
}

var _ plugin.Plugin = (*Plugin)(nil)
var _ plugin.KindTemplater = (*Plugin)(nil)
var _ plugin.Tunable = (*Plugin)(nil)
var _ plugin.Restorer = (*Plugin)(nil)

// NewPlugin builds a navigation plugin over a board, its units moved by m; hand it to the players
// plugin for its MoveTo command and default bindings.
func NewPlugin(
	boardPlugin *board.Plugin,
	worldPlugin *world.Plugin,
	m *motion.Plugin,
	selectionPlugin *selection.Plugin,
	drivingPlugin *driving.Plugin,
) *Plugin {
	if m == nil {
		panic("navigation: a unit sent moves by motion, and none was given")
	}
	p := &Plugin{
		boardPlugin: boardPlugin,
		worldPlugin: worldPlugin,
		sel:         selectionPlugin,
		selected:    selectionPlugin.Tags().Selected,
	}
	p.read.p, p.write.p = p, p
	p.kept = plugin.Keep(&p.Res.Settings)
	p.Self = world.NewSelf(worldPlugin, p)
	drivingPlugin.WithKeeping(&p.handKeeping)
	p.pathRenderer = routes.NewRenderer(boardPlugin.Read(0).Board(), DefaultRouteStyle, p.selected).
		WithHeights(boardPlugin.Read(0).Heights).
		WithDrawnHeights(boardPlugin.Read(0).DrawnHeights).WithLook(worldPlugin.Read(0).Look).
		WithStyles(worldPlugin.Read(0).Styles)
	p.pathRenderer.BindSpace(worldPlugin.Write().Space())
	return p
}

// KindTemplates is what navigation asks of the kinds a game defines: of a unit, the profile it is
// steered by, and the order it was given last.
func (p *Plugin) KindTemplates(r *kind.Roster) {
	kind.Require[steering.Steering](r, motion.UnitTemplate, "navigation",
		"the profile it is steered by")
	r.Default(motion.UnitTemplate, comp.Const(LastOrder{}))
}

func (p *Plugin) Name() string { return "gram.navigation" }

func (p *Plugin) Install(ctx plugin.Installer) error {
	read := p.boardPlugin.Read(0)
	brd := read.Board()
	p.board = brd

	w, h := brd.CellBounds()
	p.spacing = p.spacing.resolve(float32(p.worldPlugin.Res.Config.Entities.MaxSize), min(w, h))
	// routes are planned blind to the others: the keeping answers what a route runs into; the
	// routing plans the routes, the traffic searches but round a unit, each with its own
	keeping := func(answers *spacing.Answers) (*spacing.Finder, spacing.Keeping) {
		finder := spacing.NewFinder(brd, brd, read, spacing.OpenOccupancy{})
		finder.WithLevels(spacing.BoardLevels{Read: read})
		if p.spacing == BodySpacing {
			k := spacing.NewBodyKeeping(finder, p.worldPlugin.Write().Space(), read.Heights)
			if answers != nil {
				k.WithAnswers(answers)
			}
			return finder, k
		}
		k := spacing.NewCellKeeping(finder, p.boardPlugin.Write().Occupancy())
		if answers != nil {
			k.WithAnswers(answers)
		}
		return finder, k
	}
	routeFinder, routeKeep := keeping(nil)
	finder, keep := keeping(nil)
	coordinator := routing.NewCoordinator(ctx.Pool(), p.boardPlugin.Read)
	answers := &spacing.Answers{}
	for range cmp.Or(p.routers, DefaultRouters) {
		coordinator.Router(keeping(answers))
	}
	if p.hierarchy = p.hierarchy.resolve(p.worldPlugin, brd); !p.hierarchy.Off {
		coordinator.Hierarchy(int(p.hierarchy.Side), float64(p.hierarchy.From))
	}
	p.finder, p.coordinator = finder, coordinator
	p.pathRenderer.Finder = finder
	rules := p.crowd
	if !p.crowdSet {
		rules = crowd()
	}
	for _, r := range rules {
		if err := p.touches.Add(r); err != nil {
			return fmt.Errorf("%w in %s — it takes a rule of Touch", err, p.Name())
		}
	}
	ctx.Hosts(&p.touches)
	navSys := traffic.NewSystem(finder, brd, brd, finder.Occupancy).WithKeeping(keep)
	navSys.BindSpace(p.worldPlugin.Write().Space())
	navSys.Given, navSys.Touches, navSys.Tick = &p.given, &p.touches, p.worldPlugin.Read(0).Tick
	navSys.Pace = p.worldPlugin.Pace(sector.Everywhere)
	navSys.Routing, navSys.Formations = coordinator, coordinator

	moveCommandSystem := routing.NewSystem(routeFinder, &p.moves, &p.looks, p.selected).
		WithKeeping(routeKeep)
	moveCommandSystem.Onwards, moveCommandSystem.Marches = &p.onwards, &p.marches
	p.handKeeping.Keep, p.handKeeping.Nav = keep, navSys
	if p.collision != nil {
		navSys.Bumps = traffic.AnyBump
		if p.spacing == BodySpacing {
			navSys.Bumps = traffic.GroundBump
		}
	}

	p.module = &module{
		navigationSystem:  navSys,
		moveCommandSystem: moveCommandSystem,
		clock:             p.worldPlugin.Write().Clock(),
	}
	p.module.step = p.module.simulate
	ctx.UseModule(p.module)
	ctx.Draws(p.pathRenderer)
	return nil
}

// RunPlan takes the orders at once and steers the units in the simulation; call it after the
// board's RunPlan.
func (p *Plugin) RunPlan(ctx goke.RunCtx, d time.Duration) {
	p.routes.Drain(func(i command.Issued[Routes]) { p.write.ShowRoutes(i.Command.Shown) })
	p.module.RunPlan(ctx, d)
}

// WithSpacing sets how units keep out of each other's way, AutoSpacing by default. Call before
// Use.
func (p *Plugin) WithSpacing(s Spacing) *Plugin {
	p.spacing = s
	return p
}

// DefaultRouters is how many routers search the units' routes side by side.
const DefaultRouters = 4

// WithRouters sets how many routers search the units' routes side by side, each with a search's
// memory of its own on the board; 0, DefaultRouters. Call before Use.
func (p *Plugin) WithRouters(k int) *Plugin {
	p.routers = max(k, 0)
	return p
}

// Hierarchy is how the long routes are planned: climbed through a hierarchy of square regions
// of the board's cells kept for each class of units, not searched cell by cell. The zero value is
// the default.
type Hierarchy struct {
	// Side is a region's side in cells; zero, the side of the world's sectors where they span
	// whole cells of a square grid, else 16.
	Side uint16
	// From is how many cells apart a route's ends lie at least for it to be climbed; zero,
	// MaxPathLength, past which a route is followed a Path at a time.
	From uint16
	// Off has every route searched, no hierarchy built.
	Off bool
}

// the side of a hierarchy's regions, in cells, where the world's sectors give none
const hierarchySide = 16

// resolve is h with its zero Side and From as the plugin takes them over the world w and the
// board b
func (h Hierarchy) resolve(w *world.Plugin, b *board.BoardRead) Hierarchy {
	if h.Off {
		return h
	}
	if h.From == 0 {
		h.From = MaxPathLength
	}
	if h.Side == 0 {
		h.Side = sectorSide(w, b)
	}
	return h
}

// sectorSide is the side of w's sectors in b's cells, where they span whole cells of a square
// grid; else hierarchySide
func sectorSide(w *world.Plugin, b *board.BoardRead) uint16 {
	sectors := w.Res.Config.Sectors
	shape, square := b.Shape()
	if len(sectors.Bands) == 0 || !square || shape.Hex || shape.Cell <= 0 {
		return hierarchySide
	}
	// a sector spans Buckets of the space's buckets a side, four where none are given
	side := float32(cmp.Or(sectors.Buckets, 4)) * float32(w.Write().Space().BucketSize) /
		shape.Cell
	if side < 1 || side > math.MaxUint16 || side != f32.Floor(side) {
		return hierarchySide
	}
	return uint16(side)
}

// WithHierarchy sets how the long routes are planned, Hierarchy's zero value by default. Call
// before Use.
func (p *Plugin) WithHierarchy(h Hierarchy) *Plugin {
	p.hierarchy = h
	return p
}

// WithCollision has navigation learn what the units strike. Call before Use.
func (p *Plugin) WithCollision(c *collision.Plugin) *Plugin {
	if c == nil {
		panic("navigation: WithCollision needs the collision plugin")
	}
	p.collision = c
	return p
}

// EventHandler returns nil.
func (p *Plugin) EventHandler() control.EventHandler { return nil }

// Serializable is navigation's Settings, kept whole.
func (p *Plugin) Serializable() plugin.Serializable { return p.kept }

// Restore takes the Settings loaded and draws as they say.
func (p *Plugin) Restore() {
	p.kept.Restore()
	p.write.ShowRoutes(p.Res.Settings.Routes)
}

// WithCrowd sets the rules of Touch the units get on among others by, in place of the plugin's
// own; none leaves them to navigation's last word alone. Call before Use.
func (p *Plugin) WithCrowd(rules ...act.Rule) *Plugin {
	p.crowd, p.crowdSet = rules, true
	return p
}

// WithRouteStyle sets how the routes and goals of the kinds without a RouteStyle of their own
// are drawn, in place of DefaultRouteStyle.
func (p *Plugin) WithRouteStyle(style RouteStyle) *Plugin {
	p.pathRenderer.Style, p.pathRenderer.Current = style, style
	return p
}
