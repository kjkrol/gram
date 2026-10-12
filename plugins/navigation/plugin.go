package navigation

import (
	"fmt"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/entity/kind/comp"
	"github.com/kjkrol/gram/entity/tag"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/driving"
	"github.com/kjkrol/gram/plugins/selection"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/steering"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/rule"
)

// Plugin moves entities along a MoveOrder's path across a board, re-pathing when terrain changes,
// and defines the MoveTo command.
type Plugin struct {
	*world.Self

	boardPlugin *board.Plugin
	worldPlugin *world.Plugin
	selected    tag.Tag[selection.Family]

	board  *board.Board
	module *module

	moves  control.Queue[MoveTo]
	looks  control.Queue[LookAt]
	routes control.Queue[Routes]
	given  givenQueues
	finder *pathFinder

	touches  plugin.PairRules[Touch]
	crowd    []rule.Rule
	crowdSet bool

	routeStyle   RouteStyle
	routesShown  bool
	pathRenderer *pathRenderer
	collision    *collision.Plugin
	spacing      Spacing
	handKeeping  handKeeping
}

var _ plugin.Plugin = (*Plugin)(nil)

// NewPlugin builds a navigation plugin over a board; hand it to the players plugin for its MoveTo
// command and default bindings.
func NewPlugin(boardPlugin *board.Plugin, worldPlugin *world.Plugin, selectionPlugin *selection.Plugin, drivingPlugin *driving.Plugin) *Plugin {
	kind.Require[steering.Steering](&worldPlugin.Roster().Unit, "navigation", "the profile it is steered by")
	worldPlugin.Roster().Unit.Default(comp.Const(LastOrder{}))
	p := &Plugin{boardPlugin: boardPlugin, worldPlugin: worldPlugin, selected: selectionPlugin.Tags().Selected}
	p.Self = world.NewSelf(worldPlugin, p)
	drivingPlugin.WithKeeping(&p.handKeeping)
	return p
}

func (p *Plugin) Name() string { return "gram.navigation" }

func (p *Plugin) Install(ctx plugin.Installer) error {
	brd := p.boardPlugin.Board()
	p.board = brd

	w, h := brd.CellBounds()
	p.spacing = p.spacing.resolve(float64(p.worldPlugin.Res.Config.Entities.MaxSize), min(w, h))
	// routes are planned blind to the others: the keeping answers what a route runs into
	finder := newPathFinder(brd, brd, p.boardPlugin, openOccupancy{})
	if p.worldPlugin.HasHeights() {
		finder.WithLevels(boardLevels{p.boardPlugin})
	}
	var keep keeping
	if p.spacing == BodySpacing {
		keep = newBodyKeeping(finder, p.worldPlugin.Space(), p.boardPlugin.Heights)
	} else {
		keep = newCellKeeping(finder, p.boardPlugin.Occupancy())
	}
	p.finder = finder
	if p.pathRenderer != nil {
		p.pathRenderer.finder = finder
	}
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
	navSys := newNavigationSystem(finder, brd, brd, finder.occupancy).withKeeping(keep)
	navSys.BindSpace(p.worldPlugin.Space())
	navSys.given, navSys.touches, navSys.tick = &p.given, &p.touches, p.worldPlugin.Tick

	moveCommandSystem := newMoveCommandSystem(finder, &p.moves, &p.looks, p.selected).withKeeping(keep)
	p.handKeeping.keep, p.handKeeping.nav = keep, navSys
	if p.collision != nil {
		navSys.bumps = anyBump
		if p.spacing == BodySpacing {
			navSys.bumps = groundBump
		}
	}

	p.module = &module{navigationSystem: navSys, moveCommandSystem: moveCommandSystem, clock: p.worldPlugin.Clock()}
	ctx.UseModule(p.module)
	return nil
}

// RunPlan takes the orders at once and steers the units in the simulation; call it after the
// board's RunPlan.
func (p *Plugin) RunPlan(ctx goke.RunCtx, d time.Duration) {
	p.routes.Drain(func(control.Issued[Routes]) { p.ShowRoutes(!p.routesShown) })
	p.module.RunPlan(ctx, d)
}

// WithSpacing sets how units keep out of each other's way, AutoSpacing by default. Call before
// Use.
func (p *Plugin) WithSpacing(s Spacing) *Plugin {
	p.spacing = s
	return p
}

// Spacing is how units keep out of each other's way: as asked until Use, then as decided.
func (p *Plugin) Spacing() Spacing { return p.spacing }

// WithCollision has navigation learn what the units strike. Call before Use.
func (p *Plugin) WithCollision(c *collision.Plugin) *Plugin {
	if c == nil {
		panic("navigation: WithCollision needs the collision plugin")
	}
	p.collision = c
	return p
}

// WithRenderer builds the renderer of the selected units' goals and routes; the atlas is unused.
func (p *Plugin) WithRenderer(render.AtlasSource) {
	p.pathRenderer = newPathRenderer(p.board, p.routeStyle, p.selected).WithHeights(p.boardPlugin.Heights).WithLook(p.worldPlugin.Look)
	p.pathRenderer.BindSpace(p.worldPlugin.Space())
	p.pathRenderer.finder = p.finder
	p.pathRenderer.ShowRoutes(p.routesShown)
}

// Renderer is the renderer of routes, or nil unless WithRenderer was called.
func (p *Plugin) Renderer() render.Layer {
	if p.pathRenderer == nil {
		return nil
	}
	return p.pathRenderer
}

// EventHandler returns nil.
func (p *Plugin) EventHandler() control.EventHandler { return nil }

// Serializable returns nil: navigation has nothing to persist.
func (p *Plugin) Serializable() plugin.Serializable { return nil }

// WithCrowd sets the rules of Touch the units get on among others by, in place of the plugin's
// own; none leaves them to navigation's last word alone. Call before Use.
func (p *Plugin) WithCrowd(rules ...rule.Rule) *Plugin {
	p.crowd, p.crowdSet = rules, true
	return p
}

// WithRouteStyle sets how routes and goals are drawn, in place of DefaultRouteStyle. Call before
// Use.
func (p *Plugin) WithRouteStyle(style RouteStyle) *Plugin {
	p.routeStyle = style
	return p
}

// ShowRoutes has the selected units' routes drawn, or their goals alone.
func (p *Plugin) ShowRoutes(shown bool) {
	p.routesShown = shown
	if p.pathRenderer != nil {
		p.pathRenderer.ShowRoutes(shown)
	}
}

// RoutesShown reports whether the routes are drawn.
func (p *Plugin) RoutesShown() bool { return p.routesShown }
