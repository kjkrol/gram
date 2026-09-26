package isometry

import (
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/render"
)

var _ board.Look = blocks{}

// blocks is how the board's cells stand in the isometric view: a top sloped between its corners
// and raised by its kind's Height, at the depth of its centre, and the faces turned towards the
// viewer, however the view is turned, wherever it stands above the neighbour's top — a wall over grass, a raised edge over the sea —
// all lit by the world's sun as the board says.
type blocks struct{}

func (blocks) Cell(f *render.Frame, cam camera.Camera, t *board.Tile) {
	top, level := t.Top()
	sprite := t.Sprite()
	x0, y0, x1, y1 := t.X0, t.Y0, t.X1, t.Y1
	depth := cam.Depth((x0+x1)/2, (y0+y1)/2, level)
	// what sways leans its top with the wind, as far as it stands high
	var dx, dy float32
	if amount, rise := t.Sway(); amount > 0 {
		lx, ly := render.Sway(f.Time(), f.Wind(), (x0+x1)/2, (y0+y1)/2, amount)
		dx, dy = lx*rise, ly*rise
	}
	// a face turned towards the eye shows down to the top of the neighbour across it
	toward := cam.Projection().Toward()
	switch {
	case toward[0] > 0:
		if n := t.Beside(1, 0); top[1] > n[0] || top[3] > n[2] {
			f.Sprite(render.Ground, depth, t.Atlas, sprite, face(cam, x1, y0, x1, y1, top[1], top[3], n[0], n[2], dx, dy), render.Lit(t.FaceLight(1, 0)))
		}
	case toward[0] < 0:
		if n := t.Beside(-1, 0); top[2] > n[3] || top[0] > n[1] {
			f.Sprite(render.Ground, depth, t.Atlas, sprite, face(cam, x0, y1, x0, y0, top[2], top[0], n[3], n[1], dx, dy), render.Lit(t.FaceLight(-1, 0)))
		}
	}
	switch {
	case toward[1] > 0:
		if n := t.Beside(0, 1); top[2] > n[0] || top[3] > n[1] {
			f.Sprite(render.Ground, depth, t.Atlas, sprite, face(cam, x0, y1, x1, y1, top[2], top[3], n[0], n[1], dx, dy), render.Lit(t.FaceLight(0, 1)))
		}
	case toward[1] < 0:
		if n := t.Beside(0, -1); top[1] > n[3] || top[0] > n[2] {
			f.Sprite(render.Ground, depth, t.Atlas, sprite, face(cam, x1, y0, x0, y0, top[1], top[0], n[3], n[2], dx, dy), render.Lit(t.FaceLight(0, -1)))
		}
	}
	corners := sloped(cam, x0+dx, y0+dy, x1+dx, y1+dy, top)
	if t.Outlined {
		f.Tile(render.Ground, depth, t.Atlas, sprite, corners, t.Light())
	} else {
		f.Sprite(render.Ground, depth, t.Atlas, sprite, corners, t.Light())
	}
	f.Overcast(x0, y0, x1, y1)
	if shine, lit, ok := t.Shine(); ok {
		f.Glint(x0, y0, x1, y1, shine, lit, t.Shore())
	}
}

// sloped projects the four corners of a world box, each at its own height.
func sloped(cam camera.Camera, x0, y0, x1, y1 float32, z [4]float32) render.Corners {
	var out render.Corners
	for i, p := range [4][2]float32{{x0, y0}, {x1, y0}, {x0, y1}, {x1, y1}} {
		out[i][0], out[i][1] = cam.Project(p[0], p[1], z[i])
	}
	return out
}

// face projects a wall from the edge (ax, ay)-(bx, by): tops topA and topB, leant dx, dy, down to
// feet footA and footB.
func face(cam camera.Camera, ax, ay, bx, by, topA, topB, footA, footB, dx, dy float32) render.Corners {
	var out render.Corners
	out[0][0], out[0][1] = cam.Project(ax+dx, ay+dy, topA)
	out[1][0], out[1][1] = cam.Project(bx+dx, by+dy, topB)
	out[2][0], out[2][1] = cam.Project(ax, ay, footA)
	out[3][0], out[3][1] = cam.Project(bx, by, footB)
	return out
}
