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
	make        Maker
	cfg         camera.Config
	cameras     []camera.Camera // every camera made, in order: the first is Main
	pans        control.Queue[Pan]
	zooms       control.Queue[Zoom]
	follows     control.Queue[Follow]
	looks       control.Queue[MouseLook]
	module      *module
}

var _ plugin.Plugin = (*Plugin)(nil)
var _ plugin.CommandHandler = (*Plugin)(nil)
var _ plugin.Restorer = (*Plugin)(nil)

// NewPlugin makes the cameras of worldPlugin's world with make, each configured by cfg — the zero
// Config sees the window's size at zoom 1 — and makes the main one at once.
func NewPlugin(worldPlugin *world.Plugin, make Maker, cfg camera.Config) *Plugin {
	p := &Plugin{Self: world.NewSelf(worldPlugin, "gram.cameras"), worldPlugin: worldPlugin, make: make, cfg: cfg}
	p.New()
	return p
}

// Main is the camera made first: the one a game with a single view looks through.
func (p *Plugin) Main() camera.Camera { return p.cameras[0] }

// New is another camera over the world, configured as the others, saved with the game: a second
// player's, a minimap's.
func (p *Plugin) New() camera.Camera {
	space := p.worldPlugin.Res.Config.Space
	cam := p.make(space.Width, space.Height, space.Edges, p.cfg)
	p.cameras = append(p.cameras, cam)
	return cam
}

// Cameras are every camera made, in order.
func (p *Plugin) Cameras() []camera.Camera { return p.cameras }

// =================================================================
// plugin.Plugin contract
// =================================================================

func (p *Plugin) Name() string { return "gram.cameras" }

// Install sizes the cameras made with no viewport to the window, when the engine knows it.
func (p *Plugin) Install(ctx plugin.Installer) error {
	if s, ok := ctx.(plugin.Screen); ok && p.cfg.ViewportWidth == 0 && p.cfg.ViewportHeight == 0 {
		if w, h := s.Screen(); w > 0 && h > 0 {
			p.cfg.ViewportWidth, p.cfg.ViewportHeight = uint32(w), uint32(h)
			for _, cam := range p.cameras {
				cam.SetViewport(float32(w), float32(h))
			}
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
