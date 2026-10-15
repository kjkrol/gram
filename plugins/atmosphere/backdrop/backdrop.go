package backdrop

import (
	"embed"
	"image/color"
	"math"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/atmosphere/air"
	"github.com/kjkrol/gram/plugins/atmosphere/celestial"
	"github.com/kjkrol/gram/plugins/atmosphere/sky"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

var _ render.Direct = (*Renderer)(nil)

//go:embed shaders/*.wgsl
var shaders embed.FS

var skyShader = render.NewMeshShaderWith("sky", render.Files(shaders, "shaders/backdrop.wgsl"), []render.Uniform{
	{Name: "SkyView", Size: 2}, {Name: "SkyPieces", Size: 1}, {Name: "SkyHorizon", Size: 3}, {Name: "SkyOverhead", Size: 3},
	{Name: "SunAt", Size: 2}, {Name: "SunRadius", Size: 1}, {Name: "SunDisc", Size: 4}, {Name: "SunHalo", Size: 3}, {Name: "CloudsOn", Size: 1},
	{Name: "MoonAt", Size: 2}, {Name: "MoonRadius", Size: 1}, {Name: "MoonLight", Size: 3}, {Name: "MoonNorth", Size: 2}, {Name: "MoonFace", Size: 4},
	{Name: "StarsOn", Size: 1}, {Name: "ScatteredOn", Size: 1}, {Name: "OverStars", Size: 1},
	{Name: "StarX", Size: 3}, {Name: "StarY", Size: 3}, {Name: "StarZ", Size: 3},
})

// Renderer is the sky behind the world, a render.Direct behind all of it: through a
// perspective the sky of the day with the sun, the moon, the stars and the clouds on it, through
// any other camera the viewport filled in the sky's colour wherever the ground does not cover it.
type Renderer struct {
	space world.SpaceCfg
	scale world.Scale
	sun   func() sky.Sun
	air   func() air.Weather
	// nil: the sun where the light comes from
	heavens func() celestial.Heavens
	// nil: both
	shown func() (stars, moon bool)

	tri      *render.Indices
	tile     *render.Image
	moon     *render.Image
	stars    celestial.StarField
	opts     render.DrawMeshOptions
	uniforms map[string][]float32 // boxed once in the options
}

type skyPlan struct {
	None, Flat          bool
	Horizon, Overhead   render.Light
	View                [2]float32
	Field               camera.RayField
	Eye                 [3]float32
	SunAt               [2]float32
	SunRadius           float32
	SunDisc             [4]float32
	SunHalo             render.Light
	Clouds              bool
	CloudHeight, Seeing float32
	// MoonLight and MoonNorth are in the face's frame: x right, y down, z towards the eye
	MoonAt     [2]float32
	MoonRadius float32
	MoonLight  [3]float32
	MoonNorth  [2]float32
	MoonFace   [4]float32
	Stars      float32
	RealStars  bool
	StarTurn   [3][3]float32
	Heavens    celestial.Heavens
}

// New is the sky behind a world of space and scale, under sun and the weather air give.
func New(space world.SpaceCfg, scale world.Scale, sun func() sky.Sun, weather func() air.Weather) *Renderer {
	b := &Renderer{space: space, scale: scale, sun: sun, air: weather, uniforms: map[string][]float32{}}
	b.opts.Uniforms = map[string]any{}
	return b
}

// WithHeavens has the sky show the sun, the moon and the stars where heavens says they stand.
func (b *Renderer) WithHeavens(heavens func() celestial.Heavens) *Renderer {
	b.heavens = heavens
	return b
}

// WithShown has the sky show the stars and the moon only while shown says they go on.
func (b *Renderer) WithShown(shown func() (stars, moon bool)) *Renderer {
	b.shown = shown
	return b
}

func (*Renderer) Init(*goke.SysInit) {}

// Compose has the sky drawn Direct, behind the whole world.
func (*Renderer) Compose(f *render.Frame, _ camera.Camera) { f.DrawAt(render.Furthest) }

// Draw draws the sky into the target's screen through cam.
func (b *Renderer) Draw(t render.Target, cam camera.Camera, u render.Uniforms) {
	p := b.plan(cam)
	if t.Screen == nil || p.None {
		return
	}
	if p.Flat && p.SunRadius == 0 {
		fillBehind(t, rgba(p.Horizon))
		return
	}
	if b.tri == nil {
		b.tri = render.NewIndices([]uint32{0, 1, 2})
	}
	u.Into(b.opts.Uniforms)
	if p.MoonRadius > 0 && b.moon == nil {
		b.moon = celestial.MoonFace()
	}
	b.opts.Images[1] = b.moon
	f := p.Field
	b.set("SkyView", p.View[:]...)
	b.set("SkyHorizon", p.Horizon[:]...)
	b.set("SkyOverhead", p.Overhead[:]...)
	b.set("SunAt", p.SunAt[:]...)
	b.set("SunRadius", p.SunRadius)
	b.set("SunDisc", p.SunDisc[:]...)
	b.set("SunHalo", p.SunHalo[:]...)
	b.set("MoonAt", p.MoonAt[:]...)
	b.set("MoonRadius", p.MoonRadius)
	b.set("MoonLight", p.MoonLight[:]...)
	b.set("MoonNorth", p.MoonNorth[:]...)
	b.set("MoonFace", p.MoonFace[:]...)
	b.set("StarsOn", p.Stars)
	b.set("ScatteredOn", flag(!p.RealStars))
	b.set("OverStars", 0)
	if p.RealStars && p.Stars > 0 && b.stars.Place(cam, p.View[0], p.View[1], p.Heavens, p.Stars) > 0 {
		// the stars on black first, the sky over them hiding what it covers
		fillBehind(t, color.RGBA{A: 255})
		b.stars.Draw(t)
		b.set("OverStars", 1)
	}
	b.set("StarX", p.StarTurn[0][:]...)
	b.set("StarY", p.StarTurn[1][:]...)
	b.set("StarZ", p.StarTurn[2][:]...)
	b.set("CloudsOn", flag(p.Clouds))
	b.set("EyeAt", p.Eye[:]...)
	b.set("LookDir", f.Dir[:]...)
	b.set("LookDX", f.DDX[:]...)
	b.set("LookDY", f.DDY[:]...)
	b.set("CloudHeight", p.CloudHeight)
	b.set("Visibility", p.Seeing)
	b.opts.Depth = t.Depth // at the furthest depth: only where the world left nothing
	if !p.Clouds {
		b.set("SkyPieces", 0)
		b.opts.Vertices = 0
		t.Screen.DrawMesh(b.tri, skyShader, &b.opts)
		return
	}
	if b.tile == nil {
		b.tile = render.NewImage(air.TileWidth, air.TileHeight)
		air.BakeTile(b.tile)
	}
	b.opts.Images[0] = b.tile
	nx, ny := int(math.Ceil(float64(p.View[0]/skyPiece))), int(math.Ceil(float64(p.View[1]/skyPiece)))
	b.set("SkyPieces", float32(nx))
	b.opts.Vertices = 6 * nx * ny
	t.Screen.DrawMesh(nil, skyShader, &b.opts)
}

// pixels; shaders/backdrop.wgsl has the same
const skyPiece = 16

// the sky's colour where the world left nothing, everywhere without a depth
func fillBehind(t render.Target, c color.Color) {
	if t.Depth == nil {
		t.Screen.Fill(c)
		return
	}
	t.Screen.FillBehind(c, t.Depth)
}

func (b *Renderer) set(name string, v ...float32) {
	s, ok := b.uniforms[name]
	if !ok || len(s) != len(v) {
		s = make([]float32, len(v))
		b.uniforms[name] = s
	}
	copy(s, v)
	b.opts.Uniforms[name] = s
}

func flag(on bool) float32 {
	if on {
		return 1
	}
	return 0
}

func (b *Renderer) plan(cam camera.Camera) skyPlan {
	w, h := cam.Viewport()
	if b.covered(cam, w, h) {
		return skyPlan{None: true}
	}
	day := b.sun()
	weather := b.air()
	p := skyPlan{Horizon: air.Overcast(day.SkyLight(), weather.Clouds), View: [2]float32{w, h}}
	heavens, sunColor := celestial.Heavens{Sun: day.Dir}, day.Color
	if b.heavens != nil {
		heavens = b.heavens()
		sunColor = sky.SunColorAt(heavens.Sun[2])
	}
	p.Heavens = heavens
	p.SunAt, p.SunRadius, p.SunDisc, p.SunHalo = b.sunOn(cam, w, h, heavens.Sun, sunColor, weather.Clouds)
	field, ex, ey, ez, eyed := lines(cam)
	if !eyed {
		p.Flat, p.Overhead = true, p.Horizon
		return p
	}
	if b.heavens != nil {
		b.nightOn(&p, cam, field, w, h, heavens, weather.Clouds)
	}
	p.Field, p.Eye = field, [3]float32{ex, ey, ez}
	p.Overhead = air.Overhead(day.SkyLight(), weather.Clouds)
	p.CloudHeight = air.Base(b.scale)
	p.Clouds = weather.Clouds > 0 && ez < p.CloudHeight
	p.Seeing = float32(air.Visibility(b.scale, weather))
	return p
}

// false for any camera but a perspective, whose rays all start at the eye
func lines(cam camera.Camera) (field camera.RayField, ex, ey, ez float32, ok bool) {
	e, eyed := cam.(camera.Eyed)
	r, rays := cam.(camera.Rays)
	if !eyed || !rays {
		return field, 0, 0, 0, false
	}
	ex, ey, ez, ok = e.Eye()
	if !ok {
		return field, 0, 0, 0, false
	}
	field, ok = r.Rays()
	return field, ex, ey, ez, ok && field.DX == [3]float32{} && field.DY == [3]float32{}
}

func rgba(l render.Light) color.RGBA {
	return color.RGBA{R: channel(l[0]), G: channel(l[1]), B: channel(l[2]), A: 255}
}

func (b *Renderer) covered(cam camera.Camera, w, h float32) bool {
	space := b.space
	if space.Edges.WrapsX() && space.Edges.WrapsY() && cam.Projection().Wraps() {
		return true
	}
	ww, wh := float32(space.Width), float32(space.Height)
	for _, p := range [4][2]float32{{0, 0}, {w, 0}, {0, h}, {w, h}} {
		x, y := cam.Unproject(p[0], p[1], 0)
		if x < 0 || y < 0 || x > ww || y > wh {
			return false
		}
	}
	return true
}

// the discs are larger than the real sun's, a fortieth of the screen through a 45° field of view
const (
	sunRadius = 1.0 / 40
	discAngle = 0.41421356 / 20 // tan(22.5°)/20
	sunGlow   = 16
	sunBelow  = 0.1
)

func discRadius(cam camera.Camera, h float32) float32 {
	if r, ok := cam.(camera.Rays); ok {
		if f, ok := r.Rays(); ok {
			if px := float32(math.Sqrt(float64(dot3(f.DDY, f.DDY)))); px > 0 {
				return discAngle / px
			}
		}
	}
	return h * sunRadius
}

// radius 0 where no sun shows
func (b *Renderer) sunOn(cam camera.Camera, w, h float32, dir [3]float32, color render.Light, clouds float32) (at [2]float32, radius float32, disc [4]float32, halo render.Light) {
	v, ok := cam.(camera.Vanisher)
	if !ok || dir[2] <= -sunBelow {
		return at, 0, disc, halo
	}
	ax, ay, ahead := v.Vanish(dir[0], dir[1], dir[2])
	if !ahead {
		return at, 0, disc, halo
	}
	r := discRadius(cam, h)
	if ax < -sunGlow*r || ax > w+sunGlow*r || ay < -sunGlow*r || ay > h+sunGlow*r {
		return at, 0, disc, halo
	}
	clear := 1 - min(max(clouds, 0), 1)
	if clear <= 0 {
		return at, 0, disc, halo
	}
	halo = color
	if halo == (render.Light{}) {
		halo = render.Light{1, 1, 1}
	}
	low := 1 - smoothstep(0, 0.2, dir[2])
	for k := range 3 {
		disc[k] = 1 + (halo[k]-1)*low
	}
	disc[3] = clear
	return [2]float32{ax, ay}, r, disc, halo
}

func (b *Renderer) nightOn(p *skyPlan, cam camera.Camera, field camera.RayField, w, h float32, heavens celestial.Heavens, clouds float32) {
	clear := 1 - min(max(clouds, 0), 1)
	night := 1 - smoothstep(-0.2, -0.05, heavens.Sun[2])
	moon, sun := heavens.Moon, heavens.Sun
	stars, moonOn := true, true
	if b.shown != nil {
		stars, moonOn = b.shown()
	}
	if v, ok := cam.(camera.Vanisher); ok && moonOn && moon[2] > -sunBelow && clear > 0 {
		if ax, ay, ahead := v.Vanish(moon[0], moon[1], moon[2]); ahead && ax > -w && ax < 2*w && ay > -h && ay < 2*h {
			// the way to the sun as the moon's face sees it
			c := dot3(sun, moon)
			across := [3]float32{sun[0] - moon[0]*c, sun[1] - moon[1]*c, sun[2] - moon[2]*c}
			right, down := unit3(field.DDX), unit3(field.DDY)
			light := unit3([3]float32{dot3(across, right), dot3(across, down), -c})
			p.MoonAt, p.MoonRadius, p.MoonLight = [2]float32{ax, ay}, discRadius(cam, h), light
			pole := heavens.Pole
			k := dot3(pole, moon)
			up := [3]float32{pole[0] - moon[0]*k, pole[1] - moon[1]*k, pole[2] - moon[2]*k}
			if nx, ny := dot3(up, right), dot3(up, down); nx != 0 || ny != 0 {
				n := float32(math.Hypot(float64(nx), float64(ny)))
				p.MoonNorth = [2]float32{nx / n, ny / n}
			} else {
				p.MoonNorth = [2]float32{0, -1}
			}
			tint := heavens.MoonTint
			if tint == ([3]float32{}) {
				tint = [3]float32{1, 1, 1}
			}
			p.MoonFace = [4]float32{moonFace[0] * tint[0], moonFace[1] * tint[1], moonFace[2] * tint[2], clear * (moonByDay + (1-moonByDay)*night)}
		}
	}
	if stars {
		p.Stars = night
	}
	p.RealStars = heavens.Stars == celestial.RealStars
	p.StarTurn = heavens.Sphere
}

var moonFace = render.Light{0.92, 0.93, 0.96}

const moonByDay = 0.3

func dot3(a, b [3]float32) float32 { return a[0]*b[0] + a[1]*b[1] + a[2]*b[2] }

func unit3(a [3]float32) [3]float32 {
	n := float32(math.Sqrt(float64(dot3(a, a))))
	if n == 0 {
		return a
	}
	return [3]float32{a[0] / n, a[1] / n, a[2] / n}
}

func smoothstep(a, b, x float32) float32 {
	t := min(max((x-a)/(b-a), 0), 1)
	return t * t * (3 - 2*t)
}

func channel(v float32) uint8 { return uint8(min(max(v, 0), 1)*255 + 0.5) }
