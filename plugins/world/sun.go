package world

import (
	"math"

	"github.com/kjkrol/gram/render"
)

// Sun is the world's light: Dir points from the ground towards the sun (x and y along the world,
// z up), Strength is how much it lights a surface facing it square on, Ambient how much of the
// sky's light every surface gets anyway. Color is the colour of the sun's light and Sky the colour
// of the sky — the light it gives, what water reflects, what shows beyond the world; zero is
// white. It lights the ground of a world with heights; a flat world is drawn as its sprites are.
type Sun struct {
	Dir      [3]float32
	Strength float32
	Ambient  float32
	Color    render.Light
	Sky      render.Light
}

// DefaultSun stands high over the world's south-east, towards the viewer of the isometric view,
// so the faces it shows are lit and the slopes turned away from it darken; its light and sky are
// white.
var DefaultSun = Sun{Dir: [3]float32{0.522, 0.282, 0.805}, Strength: 0.708, Ambient: 0.35}

// Light is the light the sun casts on a surface whose normal is (nx, ny, nz): Ambient of the sky's,
// and Strength of its own by how square on the surface faces it; none of it from behind.
func (s Sun) Light(nx, ny, nz float32) render.Light { return s.Shaded(nx, ny, nz, 1) }

// Shaded is Light where only lit of the sun, 0 to 1, reaches the surface: the rest is in shadow
// and gets the sky's light alone.
func (s Sun) Shaded(nx, ny, nz, lit float32) render.Light { return s.Lamp().Shaded(nx, ny, nz, lit) }

// Lamp is the sun made ready to light many surfaces: its way as a unit vector, its light and the
// sky's already scaled.
func (s Sun) Lamp() Lamp {
	var l Lamp
	if d := float32(math.Sqrt(float64(s.Dir[0]*s.Dir[0] + s.Dir[1]*s.Dir[1] + s.Dir[2]*s.Dir[2]))); d > 0 {
		l.dir = [3]float32{s.Dir[0] / d, s.Dir[1] / d, s.Dir[2] / d}
	}
	sun, sky := white(s.Color), white(s.Sky)
	for c := range l.sun {
		l.sun[c], l.sky[c] = s.Strength*sun[c], s.Ambient*sky[c]
	}
	return l
}

// Lamp is a Sun ready to light surfaces: Shaded as the Sun's, without working the sun out again.
type Lamp struct {
	dir      [3]float32
	sun, sky render.Light
}

// Shaded is the light on a surface whose normal is (nx, ny, nz), lit of the sun reaching it.
func (l Lamp) Shaded(nx, ny, nz, lit float32) render.Light {
	direct := float32(0)
	if n := float32(math.Sqrt(float64(nx*nx + ny*ny + nz*nz))); n > 0 {
		direct = max((nx*l.dir[0]+ny*l.dir[1]+nz*l.dir[2])/n, 0) * lit
	}
	return render.Light{l.sky[0] + direct*l.sun[0], l.sky[1] + direct*l.sun[1], l.sky[2] + direct*l.sun[2]}
}

// Daylight is the sun as a render.Frame needs it for what glints and reflects the sky.
func (s Sun) Daylight() render.Daylight {
	sky := white(s.Sky)
	return render.Daylight{Dir: s.Dir, Strength: s.Strength, Sun: white(s.Color), Sky: sky,
		Ambient: render.Light{s.Ambient * sky[0], s.Ambient * sky[1], s.Ambient * sky[2]}}
}

// white is l, or white for the zero light.
func white(l render.Light) render.Light {
	if l == (render.Light{}) {
		return render.Light{1, 1, 1}
	}
	return l
}
