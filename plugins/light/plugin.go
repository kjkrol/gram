package light

import (
	"fmt"
	"math"
	"slices"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/light/internal/bake"
	"github.com/kjkrol/gram/plugins/light/internal/cast"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/uid"
)

// MaxLamps is how many lamps at most light a picture: the nearest its camera.
const MaxLamps = 8

// the lamps' uniforms declared, and the names each lamp's place and colour go to the frame by
var lampUniforms, lampNames = func() ([]render.Uniform, [MaxLamps][2]string) {
	u := []render.Uniform{{Name: "LampCount", Size: 1}}
	var names [MaxLamps][2]string
	for k := range MaxLamps {
		names[k] = [2]string{fmt.Sprintf("Lamp%dAt", k), fmt.Sprintf("Lamp%dColor", k)}
		u = append(u, render.Uniform{Name: names[k][0], Size: 4},
			render.Uniform{Name: names[k][1], Size: 3})
	}
	return u, names
}()

var _ = render.RegisterMaterials(render.Files(shaders, "shaders/lamps.wgsl"), lampUniforms)

// how far over the ground the live shadows lie among what lies on it: under the board's grid
const castLift = 0.05

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
	GroundAt(p geom.Vec) float64
	Version() uint64
}

// ShadowPolicy is how an entity casts its live shadow at distance world units from the eye (0
// through a camera without one), its Shadow declaring declared: Patch, Silhouette or NoShadow.
type ShadowPolicy func(declared Casting, distance float32) Casting

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
	near      []Lamp // the lamps lighting the picture begun, nearest its camera first
	shadow    goke.OptComp[Shadow]

	policy ShadowPolicy
	kept   *plugin.Kept[Settings]

	sources *goke.Query
	source  goke.Comp[Source]
	shining goke.OptComp[Shining]
	base    goke.OptComp[world.Base]
	z       goke.OptComp[world.Z]
	fresh   bool // the sources read this tick

	relief    Relief
	reliefAt  uint64 // one more than the relief's version the lattice holds; 0 none
	lat       bake.Lattice
	baker     *bake.Baker
	caster    cast.Caster
	blocks    *goke.Query
	blockBase goke.Comp[world.Base]
	blockZ    goke.Comp[world.Z]
	blockOf   goke.Comp[Shadow]
	blockLie  goke.OptComp[world.Lie]
	found     []bake.Block
	// the picture begun: its camera's eye, if any, its way across, and the casts gathered for it
	eye   [3]float32
	eyed  bool
	right [2]float32
	casts []float32

	coarses      command.Queue[CoarseShadows]
	adapts       command.Queue[AdaptShadows]
	lampSwitches command.Queue[LampShadows]
	module       *module
}

var _ plugin.Plugin = (*Plugin)(nil)
var _ plugin.CommandHandler = (*Plugin)(nil)
var _ plugin.Tunable = (*Plugin)(nil)
var _ plugin.Restorer = (*Plugin)(nil)
var _ plugin.KindTemplater = (*Plugin)(nil)
var _ world.Lighting = (*Plugin)(nil)
var _ render.Direct = (*Plugin)(nil)

// NewPlugin makes the light over w as cfg says: the world's entities are lit by it from then on.
// Make it before the board and the topography, which draw by it.
func NewPlugin(w *world.Plugin, cfg Config) *Plugin {
	p := &Plugin{w: w, cfg: cfg, distant: White, baker: bake.New()}
	p.Res.Settings.LampShadows = cfg.LampShadows
	p.kept = plugin.Keep(&p.Res.Settings)
	p.Self = world.NewSelf(w, p, comp.Const(Source{Strength: 1, Ambient: 1}),
		comp.Const(Shining{}))
	w.SetLighting(p)
	return p
}

// White is the light of a world with no distant light: every surface lit as it is, no shadow.
var White = Distant{Dir: [3]float32{0, 0, 1}, Ambient: 1}

// WithDistant has the plugin's own entity shine the distant light d over the world, a scene's
// sun where no sky gives one: set once by a scene, or each tick before RunPlan by a game whose
// sun moves. A sky's beside it panics.
func (p *Plugin) WithDistant(d Distant) *Plugin {
	p.own = &d
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

// Distant is the distant light this tick, the sun's; White without one.
func (p *Plugin) Distant() Distant {
	p.read()
	return p.distant
}

// Lamps are the lamps shining this tick.
func (p *Plugin) Lamps() []Lamp {
	p.read()
	return p.lamps
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
	p.baker.Coarse(p.Res.Settings.Coarse)
}

func (p *Plugin) Install(ctx plugin.Installer) error {
	p.module = &module{p: p}
	ctx.UseModule(p.module)
	ctx.Draws(p)
	return nil
}

// RunPlan carries the light's commands out and has the plugin's own entity shine the scene's
// sun; call it after the world's.
func (p *Plugin) RunPlan(ctx goke.RunCtx, d time.Duration) {
	p.module.RunPlan(ctx, d)
	p.coarses.Drain(func(i command.Issued[CoarseShadows]) { p.SetCoarseShadows(i.Command.On) })
	p.lampSwitches.Drain(func(i command.Issued[LampShadows]) { p.SetLampShadows(i.Command.Count) })
	p.adapts.Drain(func(i command.Issued[AdaptShadows]) { p.AdaptShadows(i.Command.On) })
	p.fresh = false
}

// read takes the sources as they stand: the one distant light and every lamp; before Install,
// the scene's own alone
func (p *Plugin) read() {
	if p.fresh {
		return
	}
	if p.sources == nil {
		if p.own != nil {
			p.distant, p.lit = *p.own, true
		}
		return
	}
	p.fresh = true
	p.lamps = p.lamps[:0]
	p.distant, p.distantOf, p.lit = White, 0, false
	for p.sources.All(); p.sources.Next(); {
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
				alt = float32(zs[i].Altitude + zs[i].Height/2)
			}
			col := white(s.Color)
			p.lamps = append(p.lamps, Lamp{X: float32(c.X), Y: float32(c.Y), Z: alt,
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

// Join adds the light's columns to the world renderer's query.
func (p *Plugin) Join(qb *goke.QueryBuilder) { qb.Optional(&p.shadow) }

// Init is nothing: the plugin's queries are its module's.
func (p *Plugin) Init(*goke.SysInit) {}

// Begin readies the light for a picture through cam: the distant light's uniforms, and the
// lamps nearest the camera, MaxLamps at most, theirs.
func (p *Plugin) Begin(f *render.Frame, cam camera.Camera) {
	p.read()
	if !f.Uniformed("LightWay") {
		p.distant.Frame(f)
	}
	p.casts = p.casts[:0]
	p.eyed, p.right = false, [2]float32{1, 0}
	if e, ok := cam.(camera.Eyed); ok {
		var x, y, z float32
		if x, y, z, p.eyed = e.Eye(); p.eyed {
			p.eye = [3]float32{x, y, z}
		}
	}
	if r, ok := cam.(camera.Rays); ok {
		if field, ok := r.Rays(); ok {
			right := field.DX
			if field.DDX != ([3]float32{}) || field.DDY != ([3]float32{}) {
				right = field.DDX
			}
			p.right = [2]float32{right[0], right[1]}
		}
	}
	x, y, ok := camera.Viewpoint(cam)
	if !ok {
		b := cam.Bounds()
		x, y = float32(b.TopLeft.X+b.BottomRight.X)/2, float32(b.TopLeft.Y+b.BottomRight.Y)/2
	}
	p.near = append(p.near[:0], p.lamps...)
	slices.SortFunc(p.near, func(a, b Lamp) int {
		da := (a.X-x)*(a.X-x) + (a.Y-y)*(a.Y-y)
		db := (b.X-x)*(b.X-x) + (b.Y-y)*(b.Y-y)
		switch {
		case da < db:
			return -1
		case da > db:
			return 1
		}
		return 0
	})
	if len(p.near) > MaxLamps {
		p.near = p.near[:MaxLamps]
	}
	f.Uniform("LampCount", float32(len(p.near)))
	for k, l := range p.near {
		f.Uniform(lampNames[k][0], l.X, l.Y, l.Z, l.Reach)
		f.Uniform(lampNames[k][1], l.Color[0], l.Color[1], l.Color[2])
	}
}

// Casting are the lamps of the picture begun that cast shadows: the nearest its camera, as many
// as LampShadows says.
func (p *Plugin) Casting() []Lamp { return p.near[:min(len(p.near), p.Res.Settings.LampShadows)] }

// Of is the light on the entity i of the chunk at cur, standing in box as z says and drawn as a,
// taking what its Shadow says; the entity's live shadow is gathered for the picture begun.
func (p *Plugin) Of(cur *goke.Cursor, i int, box plane.AABB, z world.Z,
	a render.Appearance) render.Illumination {
	takes, casting := render.TakesAll, Patch
	if s := p.shadow.Slice(cur); s != nil {
		takes, casting = taken(s[i].Take), s[i].Cast
	}
	p.cast(box.AABB, z, a, casting)
	return p.on(float32(box.TopLeft.X+box.Size.X/2), float32(box.TopLeft.Y+box.Size.Y/2),
		float32(z.Altitude), takes)
}

// At is the light on the ground at the world point, taking every shade.
func (p *Plugin) At(x, y, z float32) render.Illumination { return p.on(x, y, z, render.TakesAll) }

func (p *Plugin) on(x, y, z float32, takes render.Taking) render.Illumination {
	p.read()
	if takes == 0 { // shines: drawn as it is
		return render.Illumination{Light: render.Light{1, 1, 1}}
	}
	beam := p.distant.Beam()
	lit := render.Illumination{Light: beam.Shaded(0, 0, 1, 1), Takes: takes}
	if sum := lit.Light[0] + lit.Light[1] + lit.Light[2]; sum > 0 {
		sky := beam.Shaded(0, 0, 1, 0)
		lit.Direct = 1 - (sky[0]+sky[1]+sky[2])/sum
	}
	if takes&render.TakesLamps == 0 {
		return lit
	}
	for _, l := range p.near {
		if c, ok := l.light(x, y, z); ok {
			for k := range lit.Lamps {
				lit.Lamps[k] += c[k]
			}
		}
	}
	if sum := lit.Light[0] + lit.Light[1] + lit.Light[2]; sum > 0 {
		lit.Direct *= sum / (sum + lit.Lamps[0] + lit.Lamps[1] + lit.Lamps[2])
	}
	for k := range lit.Light {
		lit.Light[k] += lit.Lamps[k]
	}
	return lit
}

// cast gathers the live shadow of what stands in box as z says, drawn as a, cast as declared
// says through the policy: the distant light's, and on level ground the casting lamps' too
func (p *Plugin) cast(box geom.AABB, z world.Z, a render.Appearance, declared Casting) {
	if declared == Baked || declared == NoShadow {
		return
	}
	casting := declared
	if p.policy != nil {
		casting = p.policy(declared, p.distance(box, z))
	}
	if casting == Baked || casting == NoShadow {
		return
	}
	shape := casting == Silhouette
	if u, ok := p.distant.ShadowOf(box, z, p.groundAt()); ok {
		if shape {
			u, _ = p.distant.SilhouetteOf(box, z, p.groundAt())
		}
		p.gather(u, a, shape)
	}
	if p.relief != nil {
		return
	}
	for _, l := range p.Casting() {
		if u, ok := l.ShadowOf(box, z, shape); ok {
			p.gather(u, a, shape)
		}
	}
}

// gather adds the cast u, in the shape of the sprite a where shape says, seen from the side the
// sprite faces: its left on the left of the eye's way across
func (p *Plugin) gather(u Umbra, a render.Appearance, shape bool) {
	var u0, v0, u1, v1 float32
	if atlas := p.w.Atlas(); shape && atlas != nil {
		u0, v0, u1, v1 = atlas.UV(a.SpriteID)
		if -u.UY*p.right[0]+u.UX*p.right[1] < 0 {
			u0, u1 = u1, u0
		}
	}
	p.casts = append(p.casts, u.X, u.Y, u.UX, u.UY, u.Along, u.Wide, u.Fade, u.Veil,
		u0, v0, u1, v1)
}

// the ground under a point, nil for level ground
func (p *Plugin) groundAt() func(x, y float32) float32 {
	if p.relief == nil {
		return nil
	}
	return func(x, y float32) float32 {
		return float32(p.relief.GroundAt(geom.NewVec(float64(x), float64(y))))
	}
}

// how far from the eye the box's middle stands, 0 through a camera without one
func (p *Plugin) distance(box geom.AABB, z world.Z) float32 {
	if !p.eyed {
		return 0
	}
	dx := float32(box.TopLeft.X+box.BottomRight.X)/2 - p.eye[0]
	dy := float32(box.TopLeft.Y+box.BottomRight.Y)/2 - p.eye[1]
	dz := float32(z.Altitude) - p.eye[2]
	return sqrt32(dx*dx + dy*dy + dz*dz)
}

// Compose plans the shade baked this frame and hands the frame it (render.GroundShade), the bake
// drawn in the opaque pass before the ground reads it, and lays the live shadows gathered for the
// picture over the ground's depth.
func (p *Plugin) Compose(f *render.Frame, _ camera.Camera) {
	p.read()
	blocks := p.standing()
	if lat, ok := p.lattice(len(blocks) > 0); ok {
		if shade, shown := p.baker.Plan(lat, p.distant.Dir, blocks); shown {
			f.GroundShade(shade)
		}
		if p.baker.Pending() {
			f.DrawOpaque()
		}
	} else {
		p.baker.Drop()
	}
	if len(p.casts) > 0 {
		f.ReadGround()
		f.DrawAt(f.OnGround(castLift))
	}
}

// Draw bakes the shade planned in the opaque pass, and lays the casts after the ground is drawn.
func (p *Plugin) Draw(t render.Target, cam camera.Camera, u render.Uniforms) {
	switch t.Pass {
	case render.Opaque:
		p.baker.Bake(u)
	case render.SeeThrough:
		p.caster.Draw(t, cam, p.w.Atlas(), p.casts, p.lat.Low, p.lat.Low+p.lat.Span)
	}
}

// lattice is the ground the shade is baked over: the relief's, or level ground under the world
// while a block stands on it (standing); false with nothing to bake
func (p *Plugin) lattice(standing bool) (bake.Lattice, bool) {
	if p.distant.Strength <= 0 {
		return bake.Lattice{}, false
	}
	if p.relief == nil {
		if !standing {
			return bake.Lattice{}, false
		}
		space := p.w.Res.Config.Space
		w, h := float64(space.Width), float64(space.Height)
		cell := max(w, h) / flatCells
		p.lat = bake.Lattice{
			Cols: int(math.Ceil(w/cell)) + 1,
			Rows: int(math.Ceil(h/cell)) + 1,
			Cell: float32(cell),
		}
		return p.lat, true
	}
	if v := p.relief.Version() + 1; p.reliefAt != v {
		p.reliefAt = v
		cols, rows, cell, heights, ok := p.relief.Lattice()
		if !ok {
			p.lat = bake.Lattice{}
			return p.lat, false
		}
		low, high := float32(math.Inf(1)), float32(math.Inf(-1))
		for _, h := range heights {
			low, high = min(low, h), max(high, h)
		}
		if len(heights) == 0 {
			low, high = 0, 0
		}
		p.lat = bake.Lattice{Cols: cols, Rows: rows, Cell: cell, Low: low, Span: max(high-low, 1),
			Version: p.relief.Version()}
	}
	return p.lat, p.lat.Cols > 0
}

// standing is every Baked block on the ground: a deck over its Lie from its Altitude down its
// Height, anything else over its box from its Altitude up
func (p *Plugin) standing() []bake.Block {
	p.found = p.found[:0]
	if p.blocks == nil {
		return p.found
	}
	for p.blocks.All(); p.blocks.Next(); {
		cur := p.blocks.Cursor()
		bases, zs, lies := p.blockBase.Slice(cur), p.blockZ.Slice(cur), p.blockLie.Slice(cur)
		for i, s := range p.blockOf.Slice(cur) {
			if s.Cast != Baked {
				continue
			}
			box, alt := bases[i].Pos.AABB, float32(zs[i].Altitude)
			if lies != nil && lies[i].Length > 0 {
				p.found = append(p.found, lying(bases[i].Pos.Center(), bases[i].Vel.Dir, lies[i],
					alt-float32(zs[i].Height), alt))
				continue
			}
			p.found = append(p.found, bake.Block{
				X:      float32(box.TopLeft.X + box.Size.X/2),
				Y:      float32(box.TopLeft.Y + box.Size.Y/2),
				UX:     1,
				Along:  float32(box.Size.X / 2),
				Wide:   float32(box.Size.Y / 2),
				Bottom: alt,
				Top:    alt + float32(zs[i].Height),
			})
		}
	}
	return p.found
}

func lying(m, dir geom.Vec, lie world.Lie, bottom, top float32) bake.Block {
	c := lie.Corners(m, dir)
	length := math.Hypot(c[1].X-c[0].X, c[1].Y-c[0].Y)
	width := math.Hypot(c[2].X-c[0].X, c[2].Y-c[0].Y)
	b := bake.Block{
		X:      float32((c[0].X + c[3].X) / 2),
		Y:      float32((c[0].Y + c[3].Y) / 2),
		UX:     1,
		Along:  float32(length / 2),
		Wide:   float32(width / 2),
		Bottom: bottom,
		Top:    top,
	}
	if length > 0 {
		b.UX, b.UY = float32((c[1].X-c[0].X)/length), float32((c[1].Y-c[0].Y)/length)
	}
	return b
}

// SetCoarseShadows has the shadows baked half as fine a side, a quarter of the GPU's work, or
// fine again (Settings.Coarse).
func (p *Plugin) SetCoarseShadows(on bool) {
	p.Res.Settings.Coarse = on
	p.baker.Coarse(on)
}

// ShadowsCoarse reports whether the shadows are baked coarse.
func (p *Plugin) ShadowsCoarse() bool { return p.Res.Settings.Coarse }

// AdaptShadows has the shadows follow the cameras' frame budget, or not
// (Settings.AdaptiveShadows).
func (p *Plugin) AdaptShadows(on bool) { p.Res.Settings.AdaptiveShadows = on }

// ShadowsAdaptive reports whether the shadows follow the frame budget.
func (p *Plugin) ShadowsAdaptive() bool { return p.Res.Settings.AdaptiveShadows }

// Following is whether the shadows follow the frame budget (cameras.Lever).
func (p *Plugin) Following() bool { return p.ShadowsAdaptive() }

// Coarser drops the lamps' shadows, or bakes the shadows coarse once they are gone, true where
// it could (cameras.Lever).
func (p *Plugin) Coarser() bool {
	switch {
	case p.Res.Settings.LampShadows > 0:
		p.SetLampShadows(0)
	case !p.ShadowsCoarse():
		p.SetCoarseShadows(true)
	default:
		return false
	}
	return true
}

// Finer bakes the shadows fine, or brings the lamps' shadows back once they are, as many as the
// Config says, true where it could (cameras.Lever).
func (p *Plugin) Finer() bool {
	switch {
	case p.ShadowsCoarse():
		p.SetCoarseShadows(false)
	case p.Res.Settings.LampShadows < p.cfg.LampShadows:
		p.SetLampShadows(p.cfg.LampShadows)
	default:
		return false
	}
	return true
}

// SetLampShadows has the n lamps nearest a picture's camera cast shadows, 0 none
// (Settings.LampShadows).
func (p *Plugin) SetLampShadows(n int) { p.Res.Settings.LampShadows = n }

// LampShadows is how many lamps nearest a picture's camera cast shadows.
func (p *Plugin) LampShadows() int { return p.Res.Settings.LampShadows }

// Reporter is the light's part of the telemetry panel: the distant light's strength, the lamps
// and how many cast shadows.
func (p *Plugin) Reporter() render.Reporter { return reporter{p} }

type reporter struct{ p *Plugin }

func (reporter) Init(*goke.SysInit) {}

func (r reporter) Report(line func(label, value string)) {
	r.p.read()
	line("Light", fmt.Sprintf("sun %.2f\nlamps %d\nshadows of %d", r.p.distant.Strength,
		len(r.p.lamps), min(len(r.p.lamps), r.p.Res.Settings.LampShadows)))
}

func sqrt32(v float32) float32 { return float32(math.Sqrt(float64(v))) }

// taken is the shade t takes: all for a zero one, none for Shines
func taken(t render.Taking) render.Taking {
	switch {
	case t == 0:
		return render.TakesAll
	case t&render.Shines != 0:
		return 0
	}
	return t
}
