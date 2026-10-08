package cameras

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"log"
	"time"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/entity"
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
	saved       [][]byte        // each camera's state, by the order made, as saved or loaded
	loaded      bool            // saved came from a Load: a camera made since takes its own
	starts      []start         // cameras waiting for the entity their config fastens them to
	pans        control.Queue[Pan]
	zooms       control.Queue[Zoom]
	follows     control.Queue[Follow]
	lookAts     control.Queue[LookAt]
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

// New is a camera over the world made by make and configured by cfg — each camera its own, made
// where a scene shows it (render.NewFeed), sized by whoever shows it — saved with the game: one
// made after a Load is as the camera made in its place was saved. In a world in relief the
// cameras are the view plugin's (topography.Plugin.Views): what it draws asks for their lines of
// sight.
func (p *Plugin) New(make Maker, cfg camera.Config) camera.Camera {
	space := p.worldPlugin.Res.Config.Space
	cam := make(space.Width, space.Height, space.Edges, cfg)
	at := len(p.cameras)
	p.cameras = append(p.cameras, cam)
	if p.loaded && at < len(p.saved) {
		p.restore(at)
		return cam
	}
	if cfg.Follow != "" {
		f, ok := cam.(camera.Fastenable)
		if !ok {
			panic(fmt.Sprintf("cameras: a %T cannot be fastened, so cannot follow %q", cam, cfg.Follow))
		}
		p.starts = append(p.starts, start{cam: f, whom: entity.Named(cfg.Follow)})
	}
	return cam
}

// start is a camera to fasten Centred over the entity called whom once it is in the world.
type start struct {
	cam  camera.Fastenable
	whom entity.Whom
}

// Cameras are every camera made, in order.
func (p *Plugin) Cameras() []camera.Camera { return p.cameras }

// =================================================================
// plugin.Plugin contract
// =================================================================

func (p *Plugin) Name() string { return "gram.cameras" }

func (p *Plugin) Install(ctx plugin.Installer) error {
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

// Serializable is every camera's state — its window, zoom and fastening — in the order made.
func (p *Plugin) Serializable() plugin.Serializable { return persisted{p} }

// Restore gives the cameras made so far the state a Load wrote; those made later take theirs as
// they are made.
func (p *Plugin) Restore() {
	p.loaded = true
	for at := range p.cameras {
		if at < len(p.saved) {
			p.restore(at)
		}
	}
}

// restore writes the camera made at as it was saved, and rebuilds it.
func (p *Plugin) restore(at int) {
	cam := p.cameras[at]
	dec := gob.NewDecoder(bytes.NewReader(p.saved[at]))
	for _, t := range cam.Persisted() {
		if err := dec.Decode(t); err != nil {
			log.Printf("cameras: camera %d is not as it was saved: %v", at, err)
			return
		}
	}
	cam.Restore()
}

// persisted is what of the cameras a game saves: each camera's state on its own, so a camera made
// after a Load finds its own.
type persisted struct{ p *Plugin }

func (s persisted) Persisted() []any {
	p := s.p
	p.saved = p.saved[:0]
	for _, cam := range p.cameras {
		var buf bytes.Buffer
		enc := gob.NewEncoder(&buf)
		for _, t := range cam.Persisted() {
			if err := enc.Encode(t); err != nil {
				panic(fmt.Sprintf("cameras: a %T cannot be saved: %v", cam, err))
			}
		}
		p.saved = append(p.saved, buf.Bytes())
	}
	return []any{&p.saved}
}
