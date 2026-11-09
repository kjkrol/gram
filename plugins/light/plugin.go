package light

import (
	"fmt"
	"math"
	"slices"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/internal/f32"
	"github.com/kjkrol/gram/lod"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/uid"
)

// MaxLamps is how many lamps at most light a picture: the nearest its camera.
const MaxLamps = render.MaxLamps

// cells a side the shade of level ground is baked in, without a relief to say
const flatCells = 256

// Config is how the light begins.
type Config struct {
	// LampShadows is how many lamps nearest the middle of a picture cast shadows; 0 none.
	LampShadows int
}

// Relief is the ground's heights as the light lays its shadows on them, the topography's
// (Plugin.OverRelief): a lattice over a square grid, false off one, the height anywhere, and a
// count of the heights' changes.
type Relief interface {
	Lattice() (cols, rows int, cell float32, heights []float32, ok bool)
	GroundAt(p geom.Vec) float32
	Version() uint64
}

// ShadowPolicy is how an entity casts its live shadow at distance world units from the eye (0
// through a camera without one), its Shadow declaring declared: Patch, Silhouette or NoShadow
// (render.ShadowPolicy).
type ShadowPolicy = render.ShadowPolicy

// Plugin is the light over a world: a Distant light some entity shines with (the sun), and the
// lamps, the Sources that reach; what lights the entities and the ground, and what shade each
// takes (Shadow). It bakes the shadows of the relief and of what stands Baked on the ground for
// every ground to read (render.GroundShade) and lays the live ones over whatever ground is drawn.
type Plugin struct {
	*world.Self

	w   *world.Plugin
	cfg Config

	Res Resources

	own       *Distant // a scene's distant light, shone by the plugin's own entity
	distant   Distant
	distantOf uid.UID64
	lit       bool // a distant light shines, distantOf's
	lamps     []Lamp
	shadow    goke.OptComp[Shadow]

	policy ShadowPolicy
	kept   *plugin.Kept[Settings]

	sources *goke.Query
	source  goke.Comp[Source]
	shining goke.OptComp[Shining]
	base    goke.OptComp[world.Base]
	z       goke.OptComp[world.Z]

	relief    Relief
	reliefAt  uint64 // one more than the relief's version the lattice holds; 0 none
	lat       render.Lattice
	groundAt  func(x, y float32) float32 // ground, bound once
	blocks    *goke.Query
	blockBase goke.Comp[world.Base]
	blockZ    goke.Comp[world.Z]
	blockOf   goke.Comp[Shadow]
	blockLie  goke.OptComp[world.Lie]

	lights  render.Buffered[render.Lights]
	casts   []cast                // each sector's blocks as last published
	swapped bool                  // a frame's light published
	drawnFn func() *render.Lights // drawn, bound once

	coarses      command.Queue[CoarseShadows]
	adapts       command.Queue[AdaptShadows]
	lampSwitches command.Queue[LampShadows]
	module       *module
	read         Read
	write        Write
}

var _ plugin.Plugin = (*Plugin)(nil)
var _ plugin.CommandHandler = (*Plugin)(nil)
var _ plugin.Tunable = (*Plugin)(nil)
var _ plugin.Restorer = (*Plugin)(nil)
var _ plugin.KindTemplater = (*Plugin)(nil)
var _ plugin.Publisher = (*Plugin)(nil)
var _ world.Lighting = lighting{}

// NewPlugin makes the light over w as cfg says: the world's entities are lit by it from then on.
// Make it before the board and the topography, which draw by it.
func NewPlugin(w *world.Plugin, cfg Config) *Plugin {
	p := &Plugin{w: w, cfg: cfg, distant: White}
	p.read.p, p.write.p = p, p
	p.groundAt, p.drawnFn = p.ground, p.drawn
	p.Res.Settings.LampShadows = cfg.LampShadows
	p.kept = plugin.Keep(&p.Res.Settings)
	p.Self = world.NewSelf(w, p, comp.Const(Source{Strength: 1, Ambient: 1}),
		comp.Const(Shining{}))
	w.Write().SetLighting(lighting{p})
	w.Write().SetLights(p.drawnFn)
	return p
}

// White is the light of a world with no distant light: every surface lit as it is, no shadow.
var White = Distant{Dir: [3]float32{0, 0, 1}, Ambient: 1}

// WithDistant has the plugin's own entity shine the distant light d over the world, a scene's
// sun where no sky gives one: set once by a scene, or each tick before RunPlan by a game whose
// sun moves. A sky's beside it panics.
func (p *Plugin) WithDistant(d Distant) *Plugin {
	p.own = &d
	if p.sources == nil {
		p.distant, p.lit = d, true
	}
	return p
}

// OverRelief has the shadows fall on the relief r, the topography's, in place of level ground.
func (p *Plugin) OverRelief(r Relief) *Plugin {
	p.relief, p.reliefAt = r, 0
	return p
}

// WithShadowPolicy has the entities cast their live shadows as policy says; nil, as each
// declares. A Baked shadow is the ground's, asked of no policy.
func (p *Plugin) WithShadowPolicy(policy ShadowPolicy) *Plugin {
	p.policy = policy
	return p
}

// KindTemplates is what the light asks of the kinds a game defines: of every entity, how it
// casts its shadow and which shade it takes, a Patch taking all unless its kind says.
func (p *Plugin) KindTemplates(r *kind.Roster) {
	r.Default(world.EntityTemplate, comp.Const(Shadow{Cast: Patch}))
}

func (p *Plugin) Name() string                       { return "gram.light" }
func (p *Plugin) EventHandler() control.EventHandler { return nil }

// Serializable is the light's Settings, kept whole.
func (p *Plugin) Serializable() plugin.Serializable { return p.kept }

// Restore takes the Settings loaded and bakes as they say.
func (p *Plugin) Restore() {
	p.kept.Restore()
}

func (p *Plugin) Install(ctx plugin.Installer) error {
	p.module = &module{p: p, pace: p.w.Pace(lod.Steady)}
	ctx.UseModule(p.module)
	p.w.Write().Clock().Replayed(p.refresh)
	return nil
}

// RunPlan carries the light's commands out and has the plugin's own entity shine the scene's
// sun; call it after the world's.
func (p *Plugin) RunPlan(ctx goke.RunCtx, d time.Duration) {
	p.module.RunPlan(ctx, d)
	p.coarses.Drain(func(i command.Issued[CoarseShadows]) { p.write.SetCoarseShadows(i.Command.On) })
	p.lampSwitches.Drain(func(i command.Issued[LampShadows]) {
		p.write.SetLampShadows(i.Command.Count)
	})
	p.adapts.Drain(func(i command.Issued[AdaptShadows]) { p.write.AdaptShadows(i.Command.On) })
}

// refresh takes the sources as they stand as the clock's Replay ends: the one distant light and
// every lamp; before the sources are queried, the scene's own alone
func (p *Plugin) refresh() {
	if p.sources == nil {
		if p.own != nil {
			p.distant, p.lit = *p.own, true
		}
		return
	}
	p.lamps = p.lamps[:0]
	p.distant, p.distantOf, p.lit = White, 0, false
	for p.sources.Paced(0); p.sources.Next(); {
		cur := p.sources.Cursor()
		sources, shines := p.source.Slice(cur), p.shining.Slice(cur)
		bases, zs := p.base.Slice(cur), p.z.Slice(cur)
		for i, id := range cur.IDs {
			s := sources[i]
			if s.Reach <= 0 {
				if shines == nil || shines[i].Light == (Distant{}) {
					continue
				}
				if p.lit {
					panic(fmt.Sprintf("light: two distant lights shine, the entities %v and %v",
						p.distantOf, id))
				}
				p.distant, p.distantOf, p.lit = shone(shines[i].Light, s), id, true
				continue
			}
			if bases == nil || s.Strength <= 0 {
				continue
			}
			c := bases[i].Pos.Center()
			var alt float32
			if zs != nil {
				alt = zs[i].Altitude + zs[i].Height/2
			}
			col := white(s.Color)
			p.lamps = append(p.lamps, Lamp{X: c.X, Y: c.Y, Z: alt,
				Reach: s.Reach, Color: render.Light{col[0] * s.Strength, col[1] * s.Strength,
					col[2] * s.Strength}})
		}
	}
}

// shone is the distant light as its source scales it
func shone(sun Distant, s Source) Distant {
	col, tint := white(sun.Color), white(s.Color)
	sun.Color = render.Light{col[0] * tint[0], col[1] * tint[1], col[2] * tint[2]}
	sun.Strength *= s.Strength
	sun.Ambient *= s.Ambient
	return sun
}

// Publish hands the frame drawn next the light as it stands: the distant light and the lamps,
// how it is set, the blocks the ground bakes the shadows of, a set a sector, the ground they are
// baked over and the relief under the live shadows.
func (p *Plugin) Publish() {
	b := p.lights.Back()
	b.Sun, b.Lamps = p.distant, append(b.Lamps[:0], p.lamps...)
	b.LampShadows, b.Coarse, b.Policy = p.Res.Settings.LampShadows, p.Res.Settings.Coarse, p.policy
	standing := p.standing(b)
	b.Ground, b.GroundAt = render.Lattice{}, nil
	if lat, ok := p.lattice(standing); ok {
		b.Ground = lat
	}
	if p.relief != nil {
		b.GroundAt = p.groundAt
	}
}

// Swap has the light published last drawn.
func (p *Plugin) Swap() {
	p.lights.Swap()
	p.swapped = true
}

// drawn is the light the frame drawn is lit by, nil before any was published
func (p *Plugin) drawn() *render.Lights {
	if !p.swapped {
		return nil
	}
	return p.lights.Front()
}

// the ground under a point, the relief's
func (p *Plugin) ground(x, y float32) float32 {
	return p.relief.GroundAt(geom.NewVec(x, y))
}

// lattice is the ground the shade is baked over: the relief's, or level ground under the world
// while a block stands on it (standing); false with nothing to bake
func (p *Plugin) lattice(standing bool) (render.Lattice, bool) {
	if p.distant.Strength <= 0 {
		return render.Lattice{}, false
	}
	if p.relief == nil {
		if !standing {
			return render.Lattice{}, false
		}
		space := p.w.Res.Config.Space
		w, h := float32(space.Width), float32(space.Height)
		cell := max(w, h) / flatCells
		p.lat = render.Lattice{
			Cols: int(f32.Ceil(w/cell)) + 1,
			Rows: int(f32.Ceil(h/cell)) + 1,
			Cell: cell,
		}
		return p.lat, true
	}
	if v := p.relief.Version() + 1; p.reliefAt != v {
		p.reliefAt = v
		cols, rows, cell, heights, ok := p.relief.Lattice()
		if !ok {
			p.lat = render.Lattice{}
			return p.lat, false
		}
		low, high := f32.Inf(1), f32.Inf(-1)
		for _, h := range heights {
			low, high = min(low, h), max(high, h)
		}
		if len(heights) == 0 {
			low, high = 0, 0
		}
		p.lat = render.Lattice{Cols: cols, Rows: rows, Cell: cell, Low: low, Span: max(high-low, 1),
			Version: p.relief.Version()}
	}
	return p.lat, p.lat.Cols > 0
}

// a sector's blocks: the sum of their hashes as last published and as gathered now, the version
// they are published at, their joint rectangle (origin and size) and their highest top
type cast struct {
	sum, now, version uint64
	box               [4]float32
	top               float32
}

// standing puts every Baked block on the ground into b's set of the sector its middle lies in, one
// lying along (a bridge's handle) over its Lie from its Altitude down its Height, anything else
// over its box from its Altitude up, and raises the version of each set standing other than at the
// last publish; it reports whether any block stands
func (p *Plugin) standing(b *render.Lights) bool {
	r := p.w.Read(0)
	n := max(r.SectorsLen(), 1)
	if len(b.Casts) != n {
		b.Casts = slices.Grow(b.Casts[:0], n)[:n]
	}
	for len(p.casts) < n {
		p.casts = append(p.casts, cast{version: 1})
	}
	for i := range b.Casts {
		b.Casts[i].Blocks = b.Casts[i].Blocks[:0]
		p.casts[i].now = 0
	}
	stands := p.blocks != nil && p.gather(b, r, n)
	for i := range b.Casts {
		c, was := &b.Casts[i], &p.casts[i]
		if was.now != was.sum {
			was.sum = was.now
			was.version++
			was.box, was.top = bounds(c.Blocks)
		}
		c.Box, c.Top, c.Version = was.box, was.top, was.version
	}
	return stands
}

// gather puts every Baked block into b's set of the sector its middle lies in, n of them, summing
// their hashes; it reports whether any block stands
func (p *Plugin) gather(b *render.Lights, r *world.Read, n int) bool {
	stands, casts, sets := false, p.casts, b.Casts
	for p.blocks.All(); p.blocks.Next(); {
		cur := p.blocks.Cursor()
		bases, zs, lies := p.blockBase.Slice(cur), p.blockZ.Slice(cur), p.blockLie.Slice(cur)
		for i, s := range p.blockOf.Slice(cur) {
			if s.Cast != Baked {
				continue
			}
			box, alt := bases[i].Pos.AABB, zs[i].Altitude
			bl := render.Block{
				X:      box.TopLeft.X + box.Size.X/2,
				Y:      box.TopLeft.Y + box.Size.Y/2,
				UX:     1,
				Along:  box.Size.X / 2,
				Wide:   box.Size.Y / 2,
				Bottom: alt,
				Top:    alt + zs[i].Height,
			}
			if lies != nil && lies[i].Length > 0 {
				bl = lying(bases[i].Pos.Center(), bases[i].Vel.Dir, lies[i], alt-zs[i].Height, alt)
			}
			k := 0
			if n > 1 {
				k = r.SectorOf(bl.X, bl.Y)
			}
			casts[k].now += hashed(bl)
			sets[k].Blocks = append(sets[k].Blocks, bl)
			stands = true
		}
	}
	return stands
}

// bounds is the joint rectangle of bs (origin and size) and the highest of their tops; zero for
// none
func bounds(bs []render.Block) (box [4]float32, top float32) {
	if len(bs) == 0 {
		return box, 0
	}
	lo := [2]float32{f32.Inf(1), f32.Inf(1)}
	hi := [2]float32{f32.Inf(-1), f32.Inf(-1)}
	top = f32.Inf(-1)
	for _, b := range bs {
		ux, uy := f32.Abs(b.UX), f32.Abs(b.UY)
		hx, hy := ux*b.Along+uy*b.Wide, uy*b.Along+ux*b.Wide
		lo = [2]float32{min(lo[0], b.X-hx), min(lo[1], b.Y-hy)}
		hi = [2]float32{max(hi[0], b.X+hx), max(hi[1], b.Y+hy)}
		top = max(top, b.Top)
	}
	return [4]float32{lo[0], lo[1], hi[0] - lo[0], hi[1] - lo[1]}, top
}

// hashed mixes the block's eight numbers into one; a set's blocks are summed in any order
func hashed(b render.Block) uint64 {
	xy := uint64(math.Float32bits(b.X)) | uint64(math.Float32bits(b.Y))<<32
	u := uint64(math.Float32bits(b.UX)) | uint64(math.Float32bits(b.UY))<<32
	s := uint64(math.Float32bits(b.Along)) | uint64(math.Float32bits(b.Wide))<<32
	z := uint64(math.Float32bits(b.Bottom)) | uint64(math.Float32bits(b.Top))<<32
	return mixed(xy*0x9e3779b97f4a7c15 ^ u*0xc2b2ae3d27d4eb4f ^ s*0x165667b19e3779f9 ^
		z*0xd6e8feb86659fd93)
}

// splitmix64's finaliser
func mixed(h uint64) uint64 {
	h = (h ^ h>>30) * 0xbf58476d1ce4e5b9
	h = (h ^ h>>27) * 0x94d049bb133111eb
	return h ^ h>>31
}

func lying(m, dir geom.Vec, lie world.Lie, bottom, top float32) render.Block {
	c := lie.Corners(m, dir)
	length := f32.Hypot(c[1].X-c[0].X, c[1].Y-c[0].Y)
	width := f32.Hypot(c[2].X-c[0].X, c[2].Y-c[0].Y)
	b := render.Block{
		X:      (c[0].X + c[3].X) / 2,
		Y:      (c[0].Y + c[3].Y) / 2,
		UX:     1,
		Along:  length / 2,
		Wide:   width / 2,
		Bottom: bottom,
		Top:    top,
	}
	if length > 0 {
		b.UX, b.UY = (c[1].X-c[0].X)/length, (c[1].Y-c[0].Y)/length
	}
	return b
}
