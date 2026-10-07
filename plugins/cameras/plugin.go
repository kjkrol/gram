package cameras

import (
	"time"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/control"
	icamera "github.com/kjkrol/gram/internal/camera"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

// Maker makes a camera over a width x height world with the given edges, configured by cfg: the
// view a game looks at its world through.
type Maker func(width, height uint32, edges aabbworld.Edges, cfg camera.Config) camera.Camera

// TopDown is the plain camera from above over a flat world: it wraps on a wrapping axis and holds
// inside the world on any other.
func TopDown() Maker { return icamera.NewFromSpaceWithConfig }

// Plugin makes a world's cameras and moves them: Pan, Zoom and Follow, and a camera fastened
// Centred over an entity kept over it as it goes.
type Plugin struct {
	*world.Self // its own entity: its knobs, the roles it plays, the effects it is under

	worldPlugin *world.Plugin
	cameras     []camera.Camera // every camera made, in order
	unsized     []camera.Camera // made with no viewport before the screen was known
	screenW     int             // the window's size, once Install has learnt it
	screenH     int
	pans        control.Queue[Pan]
	zooms       control.Queue[Zoom]
	follows     control.Queue[Follow]
	looks       control.Queue[MouseLook]
	module      *module
}

var _ plugin.Plugin = (*Plugin)(nil)
var _ plugin.CommandHandler = (*Plugin)(nil)
var _ plugin.Restorer = (*Plugin)(nil)

// NewPlugin keeps and moves the cameras of worldPlugin's world; it makes none itself.
func NewPlugin(worldPlugin *world.Plugin) *Plugin {
	return &Plugin{Self: world.NewSelf(worldPlugin, "gram.cameras"), worldPlugin: worldPlugin}
}

// New is a camera over the world made by make and configured by cfg — each camera its own: a
// player's, made as it is given to the player, a minimap's — saved with the game. A zero viewport
// sees the window's size at zoom 1. In a world in relief the cameras are the view plugin's
// (topography.Plugin.Views): what it draws asks for their lines of sight.
func (p *Plugin) New(make Maker, cfg camera.Config) camera.Camera {
	space := p.worldPlugin.Res.Config.Space
	cam := make(space.Width, space.Height, space.Edges, cfg)
	p.cameras = append(p.cameras, cam)
	if cfg.ViewportWidth == 0 && cfg.ViewportHeight == 0 {
		if p.screenW > 0 && p.screenH > 0 {
			cam.SetViewport(float32(p.screenW), float32(p.screenH))
		} else {
			p.unsized = append(p.unsized, cam)
		}
	}
	return cam
}

// Cameras are every camera made, in order.
func (p *Plugin) Cameras() []camera.Camera { return p.cameras }

// =================================================================
// plugin.Plugin contract
// =================================================================

func (p *Plugin) Name() string { return "gram.cameras" }

// Install learns the window's size, when the engine knows it, and sizes the cameras made with no
// viewport to it — those made so far, and those made later.
func (p *Plugin) Install(ctx plugin.Installer) error {
	if s, ok := ctx.(plugin.Screen); ok {
		if w, h := s.Screen(); w > 0 && h > 0 {
			p.screenW, p.screenH = w, h
			for _, cam := range p.unsized {
				cam.SetViewport(float32(w), float32(h))
			}
			p.unsized = nil
		}
	}
	p.module = &module{p: p}
	ctx.UseModule(p.module)
	return nil
}

// RunPlan carries out Pan, Zoom and Follow and keeps every camera fastened Centred over its
// entity; call it after the world's.
func (p *Plugin) RunPlan(ctx goke.RunCtx, d time.Duration) { p.module.RunPlan(ctx, d) }

// WithRenderer is a no-op — cameras draw nothing.
func (p *Plugin) WithRenderer(render.AtlasSource) {}

// Renderer is nil — cameras draw nothing.
func (p *Plugin) Renderer() render.Layer { return nil }

// EventHandler is nil — a player's bindings (Keys) issue the commands.
func (p *Plugin) EventHandler() control.EventHandler { return nil }

// Serializable is every camera's window and zoom, in the order made.
func (p *Plugin) Serializable() plugin.Serializable { return persisted{p} }

// Restore rebuilds the cameras after a load has written their state.
func (p *Plugin) Restore() {
	for _, cam := range p.cameras {
		cam.Restore()
	}
}

// persisted is what of the cameras a game saves.
type persisted struct{ p *Plugin }

func (s persisted) Persisted() []any {
	var out []any
	for _, cam := range s.p.cameras {
		out = append(out, cam.Persisted()...)
	}
	return out
}
