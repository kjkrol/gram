package navigation

import (
	"fmt"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/selection"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/kind"
	"github.com/kjkrol/gram/render"
)

// Plugin moves entities along a MoveOrder's path across a board, re-pathing when terrain changes,
// and defines the MoveTo command; WithRenderer draws the remaining route.
type Plugin struct {
	boardPlugin *board.Plugin
	worldPlugin *world.Plugin
	selected    plugin.Tag[selection.Family]

	board  *board.Board
	module *module

	moves  control.Queue[MoveTo]
	looks  control.Queue[LookAt]
	finder *pathFinder

	pathSprites  PathSprites
	pathRenderer *PathRenderer
	collision    *collision.Plugin
	spacing      Spacing // as asked; Install decides AutoSpacing
}

var _ plugin.Plugin = (*Plugin)(nil)

// NewPlugin builds a navigation plugin over a board; hand it to the players plugin for its MoveTo
// command and default bindings. Entities move as their Steering profile says.
func NewPlugin(boardPlugin *board.Plugin, worldPlugin *world.Plugin, selectionPlugin *selection.Plugin) *Plugin {
	kind.Require[world.Steering](&worldPlugin.Roster().Unit, "navigation", "the profile it is steered by")
	return &Plugin{boardPlugin: boardPlugin, worldPlugin: worldPlugin, selected: selectionPlugin.Tags().Selected}
}

// =================================================================
// plugin.Plugin contract
// =================================================================

func (p *Plugin) Name() string { return "gram.navigation" }

func (p *Plugin) Install(ctx plugin.Installer) error {
	brd := p.boardPlugin.Res.Logic.Board
	p.board = brd

	w, h := brd.CellBounds()
	p.spacing = p.spacing.resolve(float64(p.worldPlugin.Res.Config.Entities.MaxSize), min(w, h))
	var keep keeping
	var finder *pathFinder
	if p.spacing == BodySpacing {
		finder = newPathFinder(brd, brd, p.boardPlugin, openOccupancy{})
		keep = newBodyKeeping(finder, p.worldPlugin.Space(), p.worldPlugin.Ground)
	} else {
		finder = newPathFinder(brd, brd, p.boardPlugin, p.boardPlugin.Occupancy())
		keep = newCellKeeping(finder)
	}
	p.finder = finder
	if p.pathRenderer != nil {
		p.pathRenderer.finder = finder
	}
	navSys := newNavigationSystem(finder, brd, brd, finder.occupancy).withKeeping(keep)
	navSys.BindSpace(p.worldPlugin.Space())

	moveCommandSystem := newMoveCommandSystem(finder, &p.moves, &p.looks, p.selected).withKeeping(keep)
	if p.collision != nil {
		answer := bumped()
		if p.spacing == BodySpacing {
			answer = struckBy()
		}
		if err := p.collision.RegisterBehavior(answer); err != nil {
			return err
		}
	}

	p.module = &module{navigationSystem: navSys, moveCommandSystem: moveCommandSystem, driveSystem: &driveSystem{nav: navSys}, clock: p.worldPlugin.Clock()}
	ctx.UseModule(p.module)
	return nil
}

// RunPlan takes the orders at once and hands the driving to the simulation; call it after board's
// RunPlan.
func (p *Plugin) RunPlan(ctx goke.RunCtx, d time.Duration) {
	p.module.RunPlan(ctx, d)
}

// WithSpacing sets how units keep out of each other's way; AutoSpacing, the default, decides by
// how large the world's boxes are against a cell. Call before Use.
func (p *Plugin) WithSpacing(s Spacing) *Plugin {
	p.spacing = s
	return p
}

// Spacing is how units keep out of each other's way: as WithSpacing asked until Use, then as
// decided.
func (p *Plugin) Spacing() Spacing { return p.spacing }

// WithCollision marks an entity under orders that strikes someone, or the solid ground, Bumped,
// so it looks for a way round; call before Use.
func (p *Plugin) WithCollision(c *collision.Plugin) *Plugin {
	if c == nil {
		panic("navigation: WithCollision needs the collision plugin")
	}
	p.collision = c
	return p
}

// WithRenderer draws the remaining route of every selected entity; call SetPathSprites first.
func (p *Plugin) WithRenderer(atlas render.AtlasSource) {
	p.pathRenderer = NewPathRenderer(p.board, atlas, p.pathSprites, p.selected).WithTops(p.boardPlugin.Top)
	p.pathRenderer.BindSpace(p.worldPlugin.Space())
	p.pathRenderer.finder = p.finder
}

// Renderer returns this plugin's own render.Renderer, or nil unless WithRenderer was called.
func (p *Plugin) Renderer() render.Layer {
	if p.pathRenderer == nil {
		return nil
	}
	return p.pathRenderer
}

// EventHandler returns nil — a player's bindings (DefaultBindings) issue the MoveTo commands.
func (p *Plugin) EventHandler() control.EventHandler { return nil }

// Serializable is a no-op — navigation has nothing to persist.
func (p *Plugin) Serializable() plugin.Serializable { return nil }

// RegisterBehavior reports ErrUnhostedBehavior — navigation hosts no behaviors.
func (p *Plugin) RegisterBehavior(behaviors ...plugin.Behavior) error {
	for _, b := range behaviors {
		return fmt.Errorf("%w: %T in %s", plugin.ErrUnhostedBehavior, b, p.Name())
	}
	return nil
}

// =================================================================
// navigation-specific
// =================================================================

// SetPathSprites sets the sprite set WithRenderer's PathRenderer draws — call before UsePlugin.
func (p *Plugin) SetPathSprites(sprites PathSprites) *Plugin {
	p.pathSprites = sprites
	return p
}
