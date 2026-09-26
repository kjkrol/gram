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
func (s Sun) Light(nx, ny, nz float32) float32 {
	n := float32(math.Sqrt(float64(nx*nx + ny*ny + nz*nz)))
	d := float32(math.Sqrt(float64(s.Dir[0]*s.Dir[0] + s.Dir[1]*s.Dir[1] + s.Dir[2]*s.Dir[2])))
	if n == 0 || d == 0 {
		return s.Ambient
	}
	facing := (nx*s.Dir[0] + ny*s.Dir[1] + nz*s.Dir[2]) / (n * d)
	return s.Ambient + s.Strength*max(facing, 0)
}
