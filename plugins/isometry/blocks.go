package isometry

import (
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/render"
)

var _ board.Look = blocks{}

// Shades of a block's faces against its top: the side facing down-right and the one facing
// down-left, as if lit from the upper left; a level top is drawn at shadeLevel, so a slope rising
// towards the light can be brighter and one falling away darker (shadePerUnit per world unit of
// rise across the cell).
const (
	shadeRight   = 0.72
	shadeLeft    = 0.55
	shadeLevel   = 0.92
	shadePerUnit = 0.012
)

// blocks is how the board's cells stand in the isometric view: a top sloped between its corners
// and raised by its kind's Height, at the depth of its centre, and the two faces towards the viewer
// wherever it stands above the neighbour's top — a wall over grass, a raised edge over the sea.
type blocks struct{}

func (blocks) Cell(f *render.Frame, cam camera.Camera, t *board.Tile) {
	top, level := t.Top()
	sprite := t.Sprite()
	x0, y0, x1, y1 := t.X0, t.Y0, t.X1, t.Y1
	depth := cam.Depth((x0+x1)/2, (y0+y1)/2, level)
	// the face along x = x1 shows down to the top of the neighbour across it, likewise y = y1
	if n := t.Beside(1, 0); top[1] > n[0] || top[3] > n[2] {
		f.Sprite(render.Ground, depth, t.Atlas, sprite, face(cam, x1, y0, x1, y1, top[1], top[3], n[0], n[2]), shadeRight)
	}
	if n := t.Beside(0, 1); top[2] > n[0] || top[3] > n[1] {
		f.Sprite(render.Ground, depth, t.Atlas, sprite, face(cam, x0, y1, x1, y1, top[2], top[3], n[0], n[1]), shadeLeft)
	}
	corners := sloped(cam, x0, y0, x1, y1, top)
	if t.Outlined {
		f.Tile(render.Ground, depth, t.Atlas, sprite, corners, slopeShade(top))
		return
	}
	f.Sprite(render.Ground, depth, t.Atlas, sprite, corners, slopeShade(top))
}

// slopeShade lights a top from the upper left: level at shadeLevel, brighter where it rises
// towards the light (up and to the left), darker where it falls away.
func slopeShade(top [4]float32) float32 {
	rise := (top[1] + top[3] - top[0] - top[2]) / 2 // along x, towards the right
	rise += (top[2] + top[3] - top[0] - top[1]) / 2 // along y, downwards
	return min(max(shadeLevel-shadePerUnit*rise, 0.45), 1)
}

// sloped projects the four corners of a world box, each at its own height.
func sloped(cam camera.Camera, x0, y0, x1, y1 float32, z [4]float32) render.Corners {
	var out render.Corners
	for i, p := range [4][2]float32{{x0, y0}, {x1, y0}, {x0, y1}, {x1, y1}} {
		out[i][0], out[i][1] = cam.Project(p[0], p[1], z[i])
	}
	return out
}

// face projects a wall from the edge (ax, ay)-(bx, by): tops topA and topB down to feet footA and
// footB.
func face(cam camera.Camera, ax, ay, bx, by, topA, topB, footA, footB float32) render.Corners {
	var out render.Corners
	out[0][0], out[0][1] = cam.Project(ax, ay, topA)
	out[1][0], out[1][1] = cam.Project(bx, by, topB)
	out[2][0], out[2][1] = cam.Project(ax, ay, footA)
	out[3][0], out[3][1] = cam.Project(bx, by, footB)
	return out
}
