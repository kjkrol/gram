package celestial

import (
	"embed"
	"math"
	"time"

	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/render"
)

//go:embed shaders/*.wgsl
var shaders embed.FS

// starShader draws the real stars (shaders/stars.wgsl): an instance a star, its place, spread and
// brightness and its colour.
var starShader = render.NewMeshShaderWith("stars", render.Files(shaders, "shaders/stars.wgsl"),
	[]render.Uniform{{Name: "StarView", Size: 2}}).Instanced(2)

// How the real stars shine: how wide a star of magnitude 2 or fainter spreads, in pixels, the
// brighter spread wider at the same peak; how dim a star may be and still be drawn; how much they
// twinkle low in the sky, and how fast.
const (
	starSpread  = 0.7
	starFaint   = 0.01
	starTwinkle = 0.35
	starFlicker = 7.0
)

// StarField is the real stars (Stars) drawn on a sky: Place lays out those a camera sees, Draw
// draws them, each a soft dot of light in its colour — for a sky drawn over them to hide as it
// covers them. It twinkles by the wall clock, a look only.
type StarField struct {
	inst  []float32
	opts  render.DrawMeshOptions
	view  [2]float32
	begun time.Time
}

// Place lays out the stars of heavens seen through cam, a viewport w by h pixels, as bright as
// bright (0 none): only those over the horizon and on the screen, faint as they are, dimmer and
// twinkling low in the sky. It is how many it laid out.
func (f *StarField) Place(cam camera.Camera, w, h float32, heavens Heavens, bright float32) int {
	f.inst, f.view = f.inst[:0], [2]float32{w, h}
	v, ok := cam.(camera.Vanisher)
	if !ok || bright <= 0 {
		return 0
	}
	if f.begun.IsZero() {
		f.begun = time.Now()
	}
	t := time.Since(f.begun).Seconds()
	for i, st := range Stars() {
		way := heavens.OnSky(st.Dir)
		if way[2] < -0.01 {
			continue
		}
		x, y, ahead := v.Vanish(way[0], way[1], way[2])
		if !ahead || x < -4 || y < -4 || x > w+4 || y > h+4 {
			continue
		}
		low := 1 - smoothstep(0.02, 0.4, way[2])
		light := bright * starLight(st.Mag) * smoothstep(-0.01, 0.2, way[2]) *
			(1 + starTwinkle*low*float32(math.Sin(t*starFlicker+float64(i)*2.399)))
		if light < starFaint {
			continue
		}
		c := starColour(st.BV)
		f.inst = append(f.inst, x, y, starSpread*float32(math.Sqrt(float64(max(light, 1)))), min(light, 1), c[0], c[1], c[2], 0)
	}
	return len(f.inst) / 8
}

// Draw draws the stars Place laid out onto screen, over what is there.
func (f *StarField) Draw(screen *render.Image) {
	if len(f.inst) == 0 {
		return
	}
	if f.opts.Uniforms == nil {
		f.opts = render.DrawMeshOptions{Vertices: 6, Uniforms: map[string]any{}}
	}
	f.opts.Uniforms["StarView"] = f.view[:]
	f.opts.Instances = f.inst
	screen.DrawMesh(nil, starShader, &f.opts)
}

// starLight is how bright a star of magnitude mag shows, 1 at magnitude 2: its light's
// two-fifths power, so the faint ones still show on a screen, a sixth-magnitude star a quarter.
func starLight(mag float32) float32 { return float32(math.Pow(10, -0.16*float64(mag-2))) }

// starColours is the colour of a star by its colour index B−V, from blue-white to red, every 0.4
// from −0.4; starColour blends it, half towards white, as the eye sees them.
var starColours = []render.Light{
	{0.61, 0.69, 1}, {0.79, 0.85, 1}, {1, 0.97, 0.93}, {1, 0.89, 0.76}, {1, 0.82, 0.63}, {1, 0.72, 0.47}, {1, 0.64, 0.35},
}

func starColour(bv float32) render.Light {
	k := min(max((bv+0.4)/0.4, 0), float32(len(starColours)-1))
	i := min(int(k), len(starColours)-2)
	a, b, frac := starColours[i], starColours[i+1], k-float32(i)
	var c render.Light
	for n := range c {
		c[n] = 0.5 + 0.5*(a[n]+(b[n]-a[n])*frac)
	}
	return c
}

// smoothstep eases from 0 at a to 1 at b.
func smoothstep(a, b, x float32) float32 {
	t := min(max((x-a)/(b-a), 0), 1)
	return t * t * (3 - 2*t)
}
