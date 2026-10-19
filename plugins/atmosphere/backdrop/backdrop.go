package backdrop

import (
	"embed"
	"image/color"
	"math"
	"sync"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/atmosphere/air"
	"github.com/kjkrol/gram/plugins/atmosphere/celestial"
	"github.com/kjkrol/gram/plugins/light"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

var _ render.Direct = (*Renderer)(nil)

//go:embed shaders/*.wgsl
var shaders embed.FS

// the sky's own uniforms, beside its parts'
var own = []render.Uniform{
	{Name: "SkyView", Size: 2},
	{Name: "SkyPieces", Size: 1},
	{Name: "SunAt", Size: 2},
	{Name: "SunRadius", Size: 1},
	{Name: "SunDisc", Size: 4},
	{Name: "SunHalo", Size: 3},
	{Name: "SunWay", Size: 3},
	{Name: "SunTint", Size: 3},
	{Name: "CloudsOn", Size: 1},
	{Name: "MoonAt", Size: 2},
	{Name: "MoonRadius", Size: 1},
	{Name: "MoonLight", Size: 3},
	{Name: "MoonNorth", Size: 2},
	{Name: "MoonFace", Size: 4},
	{Name: "StarsOn", Size: 1},
	{Name: "OverStars", Size: 1},
	{Name: "StarX", Size: 3},
	{Name: "StarY", Size: 3},
	{Name: "StarZ", Size: 3},
	{Name: "CloudBodies", Size: 1},
	{Name: "CloudsFlat", Size: 2},
}

var heaps = render.NewMeshShaderWith("cumulus", render.Files(shaders, "shaders/cumulus.wgsl"),
	[]render.Uniform{
		{Name: "HeapsView", Size: 2},
		{Name: "HeapsPixel", Size: 1},
		{Name: "HeapsLayers", Size: 2},
		{Name: "HeapsBefore", Size: 1},
	})

// how many pixels of the screen a pixel of the heaps spans each way: they are soft, worked out a
// sixteenth as often, the sky reading them enlarged
const heapsCoarse = 4

// the skies' shaders by their source, so a Stage entered again compiles none anew
var (
	skyShadersMu sync.Mutex
	skyShaders   = map[string]*render.Shader{}
)

// Renderer is the sky behind the world, a render.Direct behind all of it: through a
// perspective the sky of the day with the sun, the moon, the stars and the clouds on it, through
// any other camera the viewport filled in the sky's colour wherever the ground does not cover it.
type Renderer struct {
	space world.SpaceCfg
	sun   func() light.Distant
	air   func() air.Weather
	// nil: the sun where the light comes from
	heavens func() celestial.Heavens
	// nil: both
	shown func() (stars, moon bool)
	// nil: every layer flat
	clouds func() Clouds

	dome     Dome
	shader   *render.Shader
	values   [4]Values // by piece
	tri      *render.Indices
	moon     *render.Image
	stars    celestial.StarField
	opts     render.DrawMeshOptions
	uniforms map[string][]float32 // boxed once in the options
	masses   air.DomeImage
	heaps    *render.Image // the clouds as heaps, coarser than the screen
	heapOpts render.DrawMeshOptions
}

type skyPlan struct {
	None, Flat bool
	View       [2]float32
	Field      camera.RayField
	Eye        [3]float32
	SunAt      [2]float32
	SunRadius  float32
	SunDisc    [4]float32
	SunHalo    render.Light
	SunTint    render.Light
	Clouds     bool
	Seeing     float32
	// MoonLight and MoonNorth are in the face's frame: x right, y down, z towards the eye
	MoonAt     [2]float32
	MoonRadius float32
	MoonLight  [3]float32
	MoonNorth  [2]float32
	MoonFace   [4]float32
	Stars      float32
	StarTurn   [3][3]float32
	Heavens    celestial.Heavens
}

// New is the Earth's sky behind a world of space, under sun and the weather air give.
func New(space world.SpaceCfg, sun func() light.Distant, weather func() air.Weather) *Renderer {
	b := &Renderer{space: space, sun: sun, air: weather, uniforms: map[string][]float32{}}
	b.opts.Uniforms = map[string]any{}
	b.Dress(Earth())
	return b
}

// Dress has the sky drawn with d's parts. A uniform or a name of the shaders two parts declare, or
// one of the library's, panics naming them; a part's WGSL is compiled the first time it is drawn.
func (b *Renderer) Dress(d Dome) {
	src, uniforms := assemble(&d)
	skyShadersMu.Lock()
	shader, ok := skyShaders[src]
	if !ok {
		shader = render.NewMeshShaderWith("sky", []byte(src), uniforms)
		skyShaders[src] = shader
	}
	skyShadersMu.Unlock()
	b.dome, b.shader, b.moon = d, shader, nil
	for i, p := range b.dome.pieces() {
		b.values[i] = Values{b: b, part: p.name, reads: p.part.Reads}
	}
}

// Dome is the parts the sky is drawn with.
func (b *Renderer) Dome() Dome { return b.dome }

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

// Clouds is how the sky shows the clouds: none if Hidden, else each layer (by air.Layer) flat or,
// in Bodies, heaped, bodies seen from under them, dearer than flat. Before, another draws the low
// heaps before the ground (aloft), and the sky only those past the highest ground.
type Clouds struct {
	Hidden bool
	Bodies [2]bool
	Before bool
}

// WithClouds has the sky show the clouds as clouds says.
func (b *Renderer) WithClouds(clouds func() Clouds) *Renderer {
	b.clouds = clouds
	return b
}

func (b *Renderer) look() Clouds {
	if b.clouds == nil {
		return Clouds{}
	}
	return b.clouds()
}

func (*Renderer) Init(*goke.SysInit) {}

// Compose hands the frame the sun, the weather over cam's eye and the air masses, and has the
// sky drawn Direct, behind the whole world.
func (b *Renderer) Compose(f *render.Frame, cam camera.Camera) {
	day := b.sun()
	day.Frame(f)
	x, y := air.Under(cam)
	w := b.air()
	w.Frame(f, day, x, y)
	if w.Dome != nil {
		f.Image("AirMasses", b.masses.Of(w.Dome))
	}
	f.DrawAt(render.Furthest)
}

// the clouds' cover where the way dir from the eye at (x, y, z) passes their layers, those under
// the eye left out; none while they are not shown
func coverAlong(weather air.Weather, shown bool, x, y, z float32, dir [3]float32) float32 {
	if !shown {
		return 0
	}
	clear := float32(1)
	for _, l := range []air.Layer{air.Low, air.High} {
		base := weather.Base(l)
		if z >= base {
			continue
		}
		at := [2]float32{x, y}
		if dir[2] > 1e-3 {
			t := (base - z) / dir[2]
			at = [2]float32{x + dir[0]*t, y + dir[1]*t}
		}
		clear *= 1 - weather.Share(weather.At(at[0], at[1]), l).Cover
	}
	return 1 - clear
}

// Draw draws the sky into the target's screen through cam.
func (b *Renderer) Draw(t render.Target, cam camera.Camera, u render.Uniforms) {
	p := b.plan(cam)
	if t.Screen == nil || p.None {
		return
	}
	if p.Flat && p.SunRadius == 0 {
		var horizon render.Light
		copy(horizon[:], u.Get("Fog"))
		fillBehind(t, rgba(horizon))
		return
	}
	if b.tri == nil {
		b.tri = render.NewIndices([]uint32{0, 1, 2})
	}
	u.Into(b.opts.Uniforms)
	if p.MoonRadius > 0 && b.moon == nil && b.dome.Moon.Image != nil {
		b.moon = b.dome.Moon.Image()
	}
	b.opts.Images[1] = b.moon
	f := p.Field
	b.set("SkyView", p.View[:]...)
	b.set("SunAt", p.SunAt[:]...)
	b.set("SunRadius", p.SunRadius)
	b.set("SunDisc", p.SunDisc[:]...)
	b.set("SunHalo", p.SunHalo[:]...)
	b.set("SunWay", p.Heavens.Sun[:]...)
	b.set("SunTint", p.SunTint[:]...)
	b.set("MoonAt", p.MoonAt[:]...)
	b.set("MoonRadius", p.MoonRadius)
	b.set("MoonLight", p.MoonLight[:]...)
	b.set("MoonNorth", p.MoonNorth[:]...)
	b.set("MoonFace", p.MoonFace[:]...)
	b.set("StarsOn", p.Stars)
	b.set("OverStars", 0)
	if stars := b.dome.Stars.Catalogue; len(stars) > 0 && p.Stars > 0 &&
		b.stars.Place(cam, p.View[0], p.View[1], p.Heavens, stars, p.Stars) > 0 {
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
	b.set("Visibility", p.Seeing)
	for i, piece := range b.dome.pieces() {
		if len(piece.part.Source) > 0 && piece.part.Feed != nil {
			piece.part.Feed(&b.values[i])
		}
	}
	b.opts.Depth = t.Depth // at the furthest depth: only where the world left nothing
	b.set("CloudBodies", 0)
	b.opts.Images[2] = nil
	look := b.look()
	b.set("CloudsFlat", flag(!look.Bodies[air.Low]), flag(!look.Bodies[air.High]))
	if p.Clouds && (look.Bodies[air.Low] || look.Bodies[air.High]) {
		b.drawHeaps(p, look)
		b.set("CloudBodies", 1)
		b.opts.Images[2] = b.heaps
	}
	if !p.Clouds {
		b.set("SkyPieces", 0)
		b.opts.Vertices = 0
		t.Screen.DrawMesh(b.tri, b.shader, &b.opts)
		return
	}
	nx, ny := int(math.Ceil(float64(p.View[0]/skyPiece))), int(math.Ceil(float64(p.View[1]/skyPiece)))
	b.set("SkyPieces", float32(nx))
	b.opts.Vertices = 6 * nx * ny
	t.Screen.DrawMesh(nil, b.shader, &b.opts)
}

// pixels; shaders/backdrop.wgsl has the same
const skyPiece = 16

// the layers of clouds as heaps into an image of their own, coarser than the view, for the sky to
// read
func (b *Renderer) drawHeaps(p skyPlan, look Clouds) {
	w, h := int(p.View[0]+heapsCoarse-1)/heapsCoarse, int(p.View[1]+heapsCoarse-1)/heapsCoarse
	if b.heaps == nil || b.heaps.Bounds().Dx() != w || b.heaps.Bounds().Dy() != h {
		if b.heaps != nil {
			b.heaps.Deallocate()
		}
		b.heaps = render.NewImage(w, h)
	}
	b.set("HeapsView", p.View[:]...)
	b.set("HeapsPixel", heapsCoarse)
	b.set("HeapsLayers", flag(look.Bodies[air.Low]), flag(look.Bodies[air.High]))
	b.set("HeapsBefore", flag(look.Before))
	b.heapOpts.Uniforms, b.heapOpts.Vertices = b.opts.Uniforms, 3
	b.heaps.Clear()
	b.heaps.DrawMesh(nil, heaps, &b.heapOpts)
}

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
	p := skyPlan{View: [2]float32{w, h}}
	heavens, sunColor := celestial.Heavens{Sun: day.Dir}, day.Color
	if b.heavens != nil {
		heavens = b.heavens()
		sunColor = heavens.SunColor
	}
	p.Heavens, p.SunTint = heavens, sunColor
	if p.SunTint == (render.Light{}) {
		p.SunTint = render.Light{1, 1, 1}
	}
	field, ex, ey, ez, eyed := lines(cam)
	if !eyed {
		ex, ey = air.Under(cam)
	}
	shown := !b.look().Hidden
	p.SunAt, p.SunRadius, p.SunDisc, p.SunHalo = b.sunOn(cam, w, h, heavens.Sun, p.SunTint,
		coverAlong(weather, shown, ex, ey, ez, heavens.Sun))
	if !eyed {
		p.Flat = true
		return p
	}
	if b.heavens != nil {
		b.nightOn(&p, cam, field, w, h, heavens,
			coverAlong(weather, shown, ex, ey, ez, heavens.Moon))
	}
	p.Field, p.Eye = field, [3]float32{ex, ey, ez}
	p.Clouds = shown && weather.Most().Cover > 0 && ez < weather.Base(air.High)
	p.Seeing = float32(weather.Visibility(ex, ey))
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
func (b *Renderer) sunOn(cam camera.Camera, w, h float32, dir [3]float32, color render.Light,
	clouds float32) (at [2]float32, radius float32, disc [4]float32, halo render.Light) {
	v, ok := cam.(camera.Vanisher)
	if !ok || !celestial.Up(dir[2]) || len(b.dome.Sun.Source) == 0 {
		return at, 0, disc, halo
	}
	ax, ay, ahead := v.Vanish(dir[0], dir[1], dir[2])
	if !ahead {
		return at, 0, disc, halo
	}
	r := discRadius(cam, h) * size(b.dome.Sun.Size)
	if ax < -sunGlow*r || ax > w+sunGlow*r || ay < -sunGlow*r || ay > h+sunGlow*r {
		return at, 0, disc, halo
	}
	clear := 1 - min(max(clouds, 0), 1)
	if clear <= 0 {
		return at, 0, disc, halo
	}
	halo = color
	low := 1 - celestial.Risen(dir[2])
	for k := range 3 {
		disc[k] = 1 + (halo[k]-1)*low
	}
	disc[3] = clear
	return [2]float32{ax, ay}, r, disc, halo
}

func (b *Renderer) nightOn(p *skyPlan, cam camera.Camera, field camera.RayField, w, h float32,
	heavens celestial.Heavens, clouds float32) {
	clear := 1 - min(max(clouds, 0), 1)
	night := celestial.Night(heavens.Sun[2])
	moon, sun := heavens.Moon, heavens.Sun
	stars, moonOn := true, true
	if b.shown != nil {
		stars, moonOn = b.shown()
	}
	m := b.dome.Moon
	v, ok := cam.(camera.Vanisher)
	if ok && moonOn && len(m.Source) > 0 && celestial.Up(moon[2]) && clear > 0 {
		ax, ay, ahead := v.Vanish(moon[0], moon[1], moon[2])
		if ahead && ax > -w && ax < 2*w && ay > -h && ay < 2*h {
			// the way to the sun as the moon's face sees it
			c := dot3(sun, moon)
			across := [3]float32{sun[0] - moon[0]*c, sun[1] - moon[1]*c, sun[2] - moon[2]*c}
			right, down := unit3(field.DDX), unit3(field.DDY)
			light := unit3([3]float32{dot3(across, right), dot3(across, down), -c})
			p.MoonAt, p.MoonLight = [2]float32{ax, ay}, light
			p.MoonRadius = discRadius(cam, h) * size(m.Size)
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
			face := m.Color
			if face == (render.Light{}) {
				face = render.Light{1, 1, 1}
			}
			for k := range 3 {
				p.MoonFace[k] = face[k] * tint[k]
			}
			p.MoonFace[3] = clear * (m.ByDay + (1-m.ByDay)*night)
		}
	}
	if stars {
		p.Stars = night
	}
	p.StarTurn = heavens.Sphere
}

// a disc's Size, 0 as the Earth's
func size(s float32) float32 {
	if s == 0 {
		return 1
	}
	return s
}

func dot3(a, b [3]float32) float32 { return a[0]*b[0] + a[1]*b[1] + a[2]*b[2] }

func unit3(a [3]float32) [3]float32 {
	n := float32(math.Sqrt(float64(dot3(a, a))))
	if n == 0 {
		return a
	}
	return [3]float32{a[0] / n, a[1] / n, a[2] / n}
}

func channel(v float32) uint8 { return uint8(min(max(v, 0), 1)*255 + 0.5) }
