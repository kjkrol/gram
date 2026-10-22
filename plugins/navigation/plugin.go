package navigation

import (
	"fmt"
	"github.com/kjkrol/gram/plugins/navigation/internal/routing"
	"github.com/kjkrol/gram/plugins/navigation/internal/spacing"
	"time"

	"github.com/kjkrol/ceremony/act"
	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/ceremony/host"
	"github.com/kjkrol/ceremony/tag"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/driving"
	"github.com/kjkrol/gram/plugins/motion"
	"github.com/kjkrol/gram/plugins/motion/steering"
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
	selected    tag.Tag[selection.Family]

	board  *board.BoardRead
	module *module

	moves   command.Queue[MoveTo]
	looks   command.Queue[LookAt]
	routes  command.Queue[Routes]
	onwards command.Queue[Onward]
	given   givenQueues
	finder  *spacing.Finder

	touches  host.PairRules[Touch]
	crowd    []act.Rule
	crowdSet bool

	pathRenderer *pathRenderer
	collision    *collision.Plugin
	spacing      Spacing
	handKeeping  handKeeping
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
		selected:    selectionPlugin.Tags().Selected,
	}
	p.read.p, p.write.p = p, p
	p.kept = plugin.Keep(&p.Res.Settings)
	p.Self = world.NewSelf(worldPlugin, p)
	drivingPlugin.WithKeeping(&p.handKeeping)
	p.pathRenderer = newPathRenderer(boardPlugin.Read(0).Board(), DefaultRouteStyle, p.selected).
		WithHeights(boardPlugin.Read(0).Heights).WithLook(worldPlugin.Read(0).Look).
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
	p.spacing = p.spacing.resolve(float64(p.worldPlugin.Res.Config.Entities.MaxSize), min(w, h))
	// routes are planned blind to the others: the keeping answers what a route runs into
	finder := spacing.NewFinder(brd, brd, read, spacing.OpenOccupancy{})
	finder.WithLevels(spacing.BoardLevels{Read: read})
	var keep spacing.Keeping
	if p.spacing == BodySpacing {
		keep = spacing.NewBodyKeeping(finder, p.worldPlugin.Write().Space(), read.Heights)
	} else {
		keep = spacing.NewCellKeeping(finder, p.boardPlugin.Write().Occupancy())
	}
	p.finder = finder
	p.pathRenderer.finder = finder
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
	navSys := newNavigationSystem(finder, brd, brd, finder.Occupancy).withKeeping(keep)
	navSys.BindSpace(p.worldPlugin.Write().Space())
	navSys.given, navSys.touches, navSys.tick = &p.given, &p.touches, p.worldPlugin.Read(0).Tick
	navSys.pace = p.worldPlugin.Pace(sector.Everywhere)

	moveCommandSystem := routing.NewSystem(finder, &p.moves, &p.looks, p.selected).WithKeeping(keep)
	moveCommandSystem.Onwards = &p.onwards
	p.handKeeping.keep, p.handKeeping.nav = keep, navSys
	if p.collision != nil {
		navSys.bumps = anyBump
		if p.spacing == BodySpacing {
			navSys.bumps = groundBump
		}
	}

	p.module = &module{navigationSystem: navSys, moveCommandSystem: moveCommandSystem, clock: p.worldPlugin.Write().Clock()}
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
	p.pathRenderer.style, p.pathRenderer.current = style, style
	return p
}
