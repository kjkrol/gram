package landscape

import (
	"fmt"
	"time"

	"github.com/kjkrol/goke/v3"

	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

// Plugin is the landscape of a board: made, it dresses the board's tiles — the sun's light on the
// relief and the terrain's shadows, the grounds blending, coasts, water glinting and running, the
// ways across the cells, the clouds' shadows, less of it all far off — as the Styles of its kinds
// say. A board without it draws its sprites in even light.
type Plugin struct {
	styles  map[board.Name]Style
	dresser *dresser
}

var _ plugin.Plugin = (*Plugin)(nil)

// NewPlugin dresses boardPlugin's tiles in worldPlugin's sun and weather; make it before the board
// draws first.
func NewPlugin(boardPlugin *board.Plugin, worldPlugin *world.Plugin) *Plugin {
	p := &Plugin{styles: map[board.Name]Style{}}
	p.dresser = newDresser(boardPlugin.Res.Logic.Board, worldPlugin.Sun, worldPlugin.Quasi3D(), p.styles)
	p.dresser.kinds = boardPlugin.CellKindDict()
	boardPlugin.SetDressing(p.dresser)
	return p
}

// Style sets how the kind named name looks beyond its sprite; a kind without one is plain ground.
func (p *Plugin) Style(name string, s Style) *Plugin {
	p.styles[board.Named(name)] = s
	return p
}

// StyleOf is how the kind named name looks, as Style set it.
func (p *Plugin) StyleOf(name string) Style { return p.styles[board.Named(name)] }

// WithShadows says whether, in a world with heights, the terrain casts shadows — the ground and
// what stands on it hiding the sun from what lies behind; on by default.
func (p *Plugin) WithShadows(on bool) *Plugin {
	p.dresser.shadows = on
	return p
}

// =================================================================
// plugin.Plugin contract
// =================================================================

func (p *Plugin) Name() string { return "gram.landscape" }

// Install is a no-op: the landscape dresses the board from NewPlugin on.
func (p *Plugin) Install(plugin.Installer) error { return nil }

// RunPlan is a no-op: the landscape has no per-tick work.
func (p *Plugin) RunPlan(goke.RunCtx, time.Duration) {}

// WithRenderer is a no-op: the landscape draws through the board's renderer.
func (p *Plugin) WithRenderer(render.AtlasSource) {}

// Renderer is nil: see WithRenderer.
func (p *Plugin) Renderer() render.Layer { return nil }

// EventHandler is nil: the landscape takes no input.
func (p *Plugin) EventHandler() control.EventHandler { return nil }

// Serializable is nil: the landscape keeps nothing a save needs.
func (p *Plugin) Serializable() plugin.Serializable { return nil }

// RegisterBehavior refuses every behavior: the landscape hosts none.
func (p *Plugin) RegisterBehavior(behaviors ...plugin.Behavior) error {
	for _, b := range behaviors {
		return fmt.Errorf("%w: %T in %s", plugin.ErrUnhostedBehavior, b, p.Name())
	}
	return nil
}
