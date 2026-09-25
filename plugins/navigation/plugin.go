package navigation

import (
	"fmt"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/board"
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

	occupancy := p.boardPlugin.Occupancy()
	finder := newPathFinder(brd, brd, occupancy)
	p.finder = finder
	if p.pathRenderer != nil {
		p.pathRenderer.finder = finder
	}
	navSys := newNavigationSystem(finder, brd, brd, occupancy)
	navSys.BindSpace(p.worldPlugin.Space())

	moveCommandSystem := newMoveCommandSystem(finder, &p.moves, &p.looks, p.selected)
	if c := p.boardPlugin.Collision(); c != nil {
		if err := c.RegisterBehavior(bumped()); err != nil {
			return err
		}
	}

	p.module = &module{navigationSystem: navSys, moveCommandSystem: moveCommandSystem}
	ctx.UseModule(p.module)
	return nil
}

// RunPlan runs navigation and the move commands for this tick; call before world's RunPlan.
func (p *Plugin) RunPlan(ctx goke.RunCtx, d time.Duration) {
	p.module.RunPlan(ctx, d)
}

// WithRenderer draws the remaining route of every selected entity; call SetPathSprites first.
func (p *Plugin) WithRenderer(atlas render.AtlasSource) {
	p.pathRenderer = NewPathRenderer(p.board, atlas, p.pathSprites, p.selected)
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
