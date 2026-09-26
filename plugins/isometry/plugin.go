package isometry

import (
	"fmt"
	"time"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

// Config is the isometric view: a Cell-sized square of the world is a TileW x TileH diamond, and a
// height lifts a point HeightUnit screen units per world unit; Headroom is how far above the
// ground a camera looks for sprites. Zero TileW, TileH, HeightUnit and Headroom are 64, 32, 1 and
// 64; Cell must be set.
type Config struct {
	Cell, TileW, TileH float32
	HeightUnit         float32
	Headroom           float32
}

// Plugin is the isometric view of a world: made, it has the world make isometric cameras and lay
// its entities as billboards; WithBoard lays the board's cells as blocks. It installs nothing of
// its own.
type Plugin struct {
	projection projection
}

var _ plugin.Plugin = (*Plugin)(nil)

// NewPlugin puts worldPlugin in the isometric view of cfg: its cameras, its own anew, and how its
// entities lie on the screen. Make it right after the world, before anything asks for a camera; a
// world that wraps cannot be seen this way.
func NewPlugin(worldPlugin *world.Plugin, cfg Config) *Plugin {
	if edges := worldPlugin.Res.Config.Space.Edges; edges.WrapsX() || edges.WrapsY() {
		panic("isometry: a world that wraps cannot be seen isometrically")
	}
	p := &Plugin{projection: projection{Cell: cfg.Cell, TileW: cfg.TileW, TileH: cfg.TileH, HeightUnit: cfg.HeightUnit, Headroom: cfg.Headroom}.withDefaults()}
	worldPlugin.SetCameras(func(width, height uint32, edges aabbworld.Edges, c camera.Config) camera.Camera {
		return newCamera(p.projection, width, height, edges, c)
	})
	worldPlugin.SetLook(billboards{})
	return p
}

// WithBoard lays boardPlugin's cells as blocks: each top sloped between its corners and raised by
// its kind's Height, with the faces towards the viewer where it stands above its neighbour.
func (p *Plugin) WithBoard(boardPlugin *board.Plugin) *Plugin {
	boardPlugin.SetLook(blocks{})
	return p
}

// =================================================================
// plugin.Plugin contract
// =================================================================

func (p *Plugin) Name() string { return "gram.isometry" }

// Install is empty: the view is in place from NewPlugin on.
func (p *Plugin) Install(plugin.Installer) error { return nil }

// RunPlan is empty: a view has nothing to tick.
func (p *Plugin) RunPlan(goke.RunCtx, time.Duration) {}

// WithRenderer is a no-op: the view draws through the world's and the board's renderers.
func (p *Plugin) WithRenderer(render.AtlasSource) {}

// Renderer is nil: see WithRenderer.
func (p *Plugin) Renderer() render.Layer { return nil }

// EventHandler is nil: the view reads no input.
func (p *Plugin) EventHandler() control.EventHandler { return nil }

// Serializable is nil: the cameras save themselves with the world.
func (p *Plugin) Serializable() plugin.Serializable { return nil }

// RegisterBehavior refuses every behavior: the view hosts none.
func (p *Plugin) RegisterBehavior(behaviors ...plugin.Behavior) error {
	for _, b := range behaviors {
		return fmt.Errorf("%w: %T in %s", plugin.ErrUnhostedBehavior, b, p.Name())
	}
	return nil
}
