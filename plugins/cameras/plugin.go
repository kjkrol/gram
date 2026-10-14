package cameras

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"log"
	"time"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/ceremony"
	"github.com/kjkrol/ceremony/command"
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
	*world.Self

	worldPlugin *world.Plugin
	cameras     []camera.Camera
	saved       [][]byte
	loaded      bool
	starts      []start
	pans        command.Queue[Pan]
	zooms       command.Queue[Zoom]
	follows     command.Queue[Follow]
	lookAts     command.Queue[LookAt]
	zones       command.Queue[Zones]
	zonesView   *zonesView
	module      *module
}

var _ plugin.Plugin = (*Plugin)(nil)
var _ plugin.CommandHandler = (*Plugin)(nil)
var _ plugin.Restorer = (*Plugin)(nil)

// NewPlugin keeps and moves the cameras of worldPlugin's world; it makes none itself.
func NewPlugin(worldPlugin *world.Plugin) *Plugin {
	p := &Plugin{worldPlugin: worldPlugin}
	p.Self = world.NewSelf(worldPlugin, p)
	p.zonesView = &zonesView{}
	return p
}

// New is a camera over the world made by make and configured by cfg, made where a scene shows
// it and sized by whoever shows it. It is saved with the game: one made after a Load is as the
// camera made in its place was saved.
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
		p.starts = append(p.starts, start{cam: f, whom: ceremony.Named(cfg.Follow)})
	}
	return cam
}

type start struct {
	cam  camera.Fastenable
	whom ceremony.Whom
}

// Cameras are every camera made, in order.
func (p *Plugin) Cameras() []camera.Camera { return p.cameras }

func (p *Plugin) Name() string { return "gram.cameras" }

func (p *Plugin) Install(ctx plugin.Installer) error {
	p.module = &module{p: p}
	ctx.UseModule(p.module)
	return nil
}

// RunPlan carries out Pan, Zoom and Follow and keeps every camera fastened Centred over its
// entity; call it after the world's.
func (p *Plugin) RunPlan(ctx goke.RunCtx, d time.Duration) {
	p.switchZones()
	p.module.RunPlan(ctx, d)
}

// Renderer lays the scroll zones over every picture while Zones shows them (Shift+H), for a
// scene's composer.
func (p *Plugin) Renderer() render.Layer { return p.zonesView }

// EventHandler is nil: a player's bindings issue the commands.
func (p *Plugin) EventHandler() control.EventHandler { return nil }

// Serializable is every camera's state, its window, zoom and fastening, in the order made.
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
