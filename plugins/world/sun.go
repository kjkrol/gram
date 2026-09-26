package world

import "math"

// Sun is the world's light: Dir points from the ground towards the sun (x and y along the world,
// z up), Strength is how much it lights a surface facing it square on, Ambient how much every
// surface gets anyway. It lights the ground of a world with heights; a flat world is drawn as its
// sprites are.
type Sun struct {
	Dir      [3]float32
	Strength float32
	Ambient  float32
}

// DefaultSun stands high over the world's south-east, towards the viewer of the isometric view,
// so the faces it shows are lit and the slopes turned away from it darken.
var DefaultSun = Sun{Dir: [3]float32{0.522, 0.282, 0.805}, Strength: 0.708, Ambient: 0.35}

// Light is how bright the sun makes a surface whose normal is (nx, ny, nz): Ambient, and Strength
// by how square on the surface faces the sun; none of it from behind.
func (s Sun) Light(nx, ny, nz float32) float32 { return s.Shaded(nx, ny, nz, 1) }

// Glint is the sun thrown back by a surface of normal (nx, ny, nz) and shine towards an eye that
// looks from toward, where lit of the sun reaches it: bright only where the surface faces halfway
// between the sun and the eye.
func (s Sun) Glint(nx, ny, nz float32, toward [3]float32, shine, lit float32) float32 {
	if shine <= 0 || lit <= 0 || s.Strength <= 0 {
		return 0
	}
	d := float32(math.Sqrt(float64(s.Dir[0]*s.Dir[0] + s.Dir[1]*s.Dir[1] + s.Dir[2]*s.Dir[2])))
	n := float32(math.Sqrt(float64(nx*nx + ny*ny + nz*nz)))
	if d == 0 || n == 0 {
		return 0
	}
	hx, hy, hz := s.Dir[0]/d+toward[0], s.Dir[1]/d+toward[1], s.Dir[2]/d+toward[2]
	h := float32(math.Sqrt(float64(hx*hx + hy*hy + hz*hz)))
	if h == 0 {
		return 0
	}
	facing := (nx*hx + ny*hy + nz*hz) / (n * h)
	if facing <= 0 {
		return 0
	}
	return shine * s.Strength * lit * float32(math.Pow(float64(facing), glintSharpness))
}

// glintSharpness is how narrowly a glint gathers round the perfect reflection.
const glintSharpness = 48

// Shaded is Light where only lit of the sun, 0 to 1, reaches the surface: the rest is in shadow
// and gets the Ambient alone.
func (s Sun) Shaded(nx, ny, nz, lit float32) float32 {
	n := float32(math.Sqrt(float64(nx*nx + ny*ny + nz*nz)))
	d := float32(math.Sqrt(float64(s.Dir[0]*s.Dir[0] + s.Dir[1]*s.Dir[1] + s.Dir[2]*s.Dir[2])))
	if n == 0 || d == 0 {
		return s.Ambient
	}
	facing := (nx*s.Dir[0] + ny*s.Dir[1] + nz*s.Dir[2]) / (n * d)
	return s.Ambient + s.Strength*max(facing, 0)*lit
}
