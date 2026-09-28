package camera

// Rays is a camera that can say every one of its lines of sight at once — a RayField — for a
// shader tracing the world per pixel; false for a camera that cannot at the moment.
type Rays interface {
	Rays() (RayField, bool)
}

// RayField is every line of sight of a camera at once: the ray through the screen point (sx, sy)
// starts at Origin + sx·DX + sy·DY and runs along Dir + sx·DDX + sy·DDY, not a unit vector. A
// perspective's DX and DY are zero, every ray starting at its eye; a parallel projection's DDX and
// DDY are zero, every ray running one way. Bend is how far below the eye's level the ground d off
// along it is drawn, per d² — a perspective's curve of the Earth; 0 flat.
type RayField struct {
	Origin, DX, DY [3]float32
	Dir, DDX, DDY  [3]float32
	Bend           float32
}

// At is the ray through the screen point (sx, sy): where it starts and the way it runs, not a unit
// vector.
func (f RayField) At(sx, sy float32) (origin, dir [3]float32) {
	for k := range 3 {
		origin[k] = f.Origin[k] + sx*f.DX[k] + sy*f.DY[k]
		dir[k] = f.Dir[k] + sx*f.DDX[k] + sy*f.DDY[k]
	}
	return origin, dir
}
