package mosaic

import (
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/world"
)

// Plugin is the ground of a Stage laid as a mosaic and drawn by the world's levels.
type Plugin struct {
	*world.Self

	worldPlugin *world.Plugin
	cfg         Config
	ground      *ground
	renderer    *renderer
	read        Read
}

var _ plugin.Plugin = (*Plugin)(nil)

// NewPlugin is the ground m says over w's world, which must have heights.
func NewPlugin(w *world.Plugin, m Map, cfg Config) *Plugin {
	if !w.Res.Config.Heights {
		panic("mosaic: the world has no heights (world.Config.Heights): a ground in relief")
	}
	p := &Plugin{worldPlugin: w, cfg: cfg.filled()}
	space := w.Res.Config.Space
	p.ground = newGround(m, float32(space.Width), float32(space.Height), p.cfg)
	p.renderer = &renderer{p: p, shown: map[int]*laid{}, next: map[int]*laid{},
		staged: map[int]*laid{}}
	p.read.p = p
	p.Self = world.NewSelf(w, p)
	w.Write().SetGround(func(at geom.Vec) float32 { return p.read.HeightAt(at.X, at.Y) })
	return p
}

// SetLook has the ground drawn in look's colours.
func (p *Plugin) SetLook(look Look) { p.ground.paint(look) }

// Read is the ground as its readers see it; one view serves every reader.
func (p *Plugin) Read(int) *Read { return &p.read }

func (p *Plugin) Name() string { return "gram.mosaic" }

func (p *Plugin) Install(ctx plugin.Installer) error {
	ctx.Draws(p.renderer)
	return nil
}

func (p *Plugin) RunPlan(goke.RunCtx, time.Duration) {}

func (p *Plugin) EventHandler() control.EventHandler { return nil }

func (p *Plugin) Serializable() plugin.Serializable { return nil }
