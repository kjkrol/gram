package topography

import (
	"cmp"
	"slices"

	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/atmosphere/air"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/render"
)

var _ board.Look = blocks{}

// blocks is how the board's cells stand in the isometric view: a top sloped between its corners
// and raised by its kind's Height, at the depth of its centre, and the faces turned towards the
// viewer, however the view is turned, wherever it stands above the neighbour's top — a wall over
// grass, a raised edge over the sea — all lit by the world's sun as the dresser says.
type blocks struct{ d *dresser }

func (b blocks) Cell(f *render.Frame, cam camera.Camera, t *board.Tile) {
	c := b.d.topOf(t.ID)
	top, level := c.z, c.alt
	sprite := t.Sprite()
	x0, y0, x1, y1 := t.X0, t.Y0, t.X1, t.Y1
	depth := cam.Depth((x0+x1)/2, (y0+y1)/2, level)
	if !b.d.inFront(cam, render.Box(x0, y0, x1, y1), top) {
		b.near(f, cam, t, top, depth) // the eye among its corners, or all of it behind the eye
		return
	}
	// what sways leans its top with the wind, as far as it stands high
	var dx, dy float32
	if amount, rise := t.Sway(); amount > 0 {
		lx, ly := b.d.weather.Sway(f.Time(), (x0+x1)/2, (y0+y1)/2, amount)
		dx, dy = lx*rise, ly*rise
	}
	// a face turned towards the eye shows down to the top of the neighbour across it
	toward := cam.Projection().Toward()
	switch {
	case toward[0] > 0:
		if n := b.d.beside(t, 1, 0); top[1] > n[0] || top[3] > n[2] {
			f.Sprite(render.Ground, depth, t.Atlas, sprite, face(cam, x1, y0, x1, y1, top[1], top[3], n[0], n[2], dx, dy), render.Lit(t.FaceLight(1, 0)))
			hazed(f, cam, b.d.weather, [4][3]float32{{x1, y0, top[1]}, {x1, y1, top[3]}, {x1, y0, n[0]}, {x1, y1, n[2]}})
		}
	case toward[0] < 0:
		if n := b.d.beside(t, -1, 0); top[2] > n[3] || top[0] > n[1] {
			f.Sprite(render.Ground, depth, t.Atlas, sprite, face(cam, x0, y1, x0, y0, top[2], top[0], n[3], n[1], dx, dy), render.Lit(t.FaceLight(-1, 0)))
			hazed(f, cam, b.d.weather, [4][3]float32{{x0, y1, top[2]}, {x0, y0, top[0]}, {x0, y1, n[3]}, {x0, y0, n[1]}})
		}
	}
	switch {
	case toward[1] > 0:
		if n := b.d.beside(t, 0, 1); top[2] > n[0] || top[3] > n[1] {
			f.Sprite(render.Ground, depth, t.Atlas, sprite, face(cam, x0, y1, x1, y1, top[2], top[3], n[0], n[1], dx, dy), render.Lit(t.FaceLight(0, 1)))
			hazed(f, cam, b.d.weather, [4][3]float32{{x0, y1, top[2]}, {x1, y1, top[3]}, {x0, y1, n[0]}, {x1, y1, n[1]}})
		}
	case toward[1] < 0:
		if n := b.d.beside(t, 0, -1); top[1] > n[3] || top[0] > n[2] {
			f.Sprite(render.Ground, depth, t.Atlas, sprite, face(cam, x1, y0, x0, y0, top[1], top[0], n[3], n[2], dx, dy), render.Lit(t.FaceLight(0, -1)))
			hazed(f, cam, b.d.weather, [4][3]float32{{x1, y0, top[1]}, {x0, y0, top[0]}, {x1, y0, n[3]}, {x0, y0, n[2]}})
		}
	}
	corners := sloped(cam, x0+dx, y0+dy, x1+dx, y1+dy, top)
	if t.Outlined && !t.Covered() {
		f.Tile(render.Ground, depth, t.Atlas, t.Base(), corners, t.Light())
	} else {
		f.Sprite(render.Ground, depth, t.Atlas, t.Base(), corners, t.Light())
	}
	hazed(f, cam, b.d.weather, [4][3]float32{{x0, y0, top[0]}, {x1, y0, top[1]}, {x0, y1, top[2]}, {x1, y1, top[3]}})
	f.Fold(top)
	t.Dress(f, cam, x0, y0, x1, y1, depth)
}

// nearPieces is how many pieces a side a tile the eye stands among is drawn in, and farPieces one
// the eye's plane only grazes far off to a side, all but off the screen.
const (
	nearPieces = 16
	farPieces  = 4
)

// near draws the top of a tile some of whose corners lie beside or behind the eye — the one it
// stands in, those round it — as pieces, the farthest first, leaving out every piece not wholly in
// front of the eye or off the screen, each with its water and the clouds' shadow; then what is laid
// over the tile, as far as it lies in front. A projection has no point for what lies behind the
// eye: a corner there, drawn, would throw the whole top across the screen. A tile wholly behind
// the eye is not drawn at all; its faces are left out, as near as they stand.
func (b blocks) near(f *render.Frame, cam camera.Camera, t *board.Tile, top [4]float32, depth float32) {
	d := b.d.tileOf(t)
	x0, y0, x1, y1 := t.X0, t.Y0, t.X1, t.Y1
	behind := 0
	for k, p := range render.Box(x0, y0, x1, y1) {
		if camera.ScaleAt(cam, p[0], p[1], top[k]) == 0 {
			behind++
		}
	}
	if behind == 4 {
		return // the top lies between its corners: all of it behind the eye
	}
	ex, ey, ez, eyed := float32(0), float32(0), float32(0), false
	if e, ok := cam.(camera.Eyed); ok {
		ex, ey, ez, eyed = e.Eye()
	}
	n := farPieces
	if w := max(x1-x0, y1-y0); !eyed || max(ex-(x0+x1)/2, (x0+x1)/2-ex) < 2*w && max(ey-(y0+y1)/2, (y0+y1)/2-ey) < 2*w {
		n = nearPieces
	}
	h0, h1, h2, h3 := float64(top[0]), float64(top[1]), float64(top[2]), float64(top[3])
	at := func(u, v float32) float32 { return float32(drawnAt(h0, h1, h2, h3, float64(u), float64(v))) }
	sw, sh := cam.Viewport()
	pieces := b.d.pieces[:0]
	for j := range n {
		for i := range n {
			u0, v0, u1, v1 := float32(i)/float32(n), float32(j)/float32(n), float32(i+1)/float32(n), float32(j+1)/float32(n)
			w := render.Box(x0+(x1-x0)*u0, y0+(y1-y0)*v0, x0+(x1-x0)*u1, y0+(y1-y0)*v1)
			z := [4]float32{at(u0, v0), at(u1, v0), at(u0, v1), at(u1, v1)}
			if !b.d.inFront(cam, w, z) {
				continue
			}
			var c render.Corners
			for k, p := range w {
				c[k][0], c[k][1] = cam.Project(p[0], p[1], z[k])
			}
			if max(c[0][0], c[1][0], c[2][0], c[3][0]) < 0 || min(c[0][0], c[1][0], c[2][0], c[3][0]) > sw ||
				max(c[0][1], c[1][1], c[2][1], c[3][1]) < 0 || min(c[0][1], c[1][1], c[2][1], c[3][1]) > sh {
				continue // off the screen
			}
			mx, my, mz := (w[0][0]+w[3][0])/2-ex, (w[0][1]+w[3][1])/2-ey, (z[0]+z[3])/2-ez
			pieces = append(pieces, nearPiece{u: [2]float32{u0, u1}, v: [2]float32{v0, v1}, w: w, z: z, c: c, dist: mx*mx + my*my + mz*mz})
		}
	}
	slices.SortFunc(pieces, func(a, b nearPiece) int { return cmp.Compare(b.dist, a.dist) }) // the farthest first
	b.d.pieces = pieces
	sx0, sy0, sx1, sy1 := t.Atlas.UV(t.Base())
	light := t.Light()
	var clouds [4]float32
	if b.d.weather.Clouds > 0 {
		clouds = d.clouds()
	}
	for _, p := range pieces {
		src := [4]float32{sx0 + (sx1-sx0)*p.u[0], sy0 + (sy1-sy0)*p.v[0], sx0 + (sx1-sx0)*p.u[1], sy0 + (sy1-sy0)*p.v[1]}
		var shade render.Shade
		for k, uv := range [4][2]float32{{p.u[0], p.v[0]}, {p.u[1], p.v[0]}, {p.u[0], p.v[1]}, {p.u[1], p.v[1]}} {
			shade[k] = mixLight(light, uv[0], uv[1])
		}
		f.SpritePart(render.Ground, depth, t.Atlas, src, p.c, shade)
		mark := f.Last()
		hazed(f, cam, b.d.weather, [4][3]float32{{p.w[0][0], p.w[0][1], p.z[0]}, {p.w[1][0], p.w[1][1], p.z[1]}, {p.w[2][0], p.w[2][1], p.z[2]}, {p.w[3][0], p.w[3][1], p.z[3]}})
		f.Fold(p.z)
		d.DrawSurface(f, p.w[0][0], p.w[0][1], p.w[3][0], p.w[3][1])
		if b.d.weather.Clouds > 0 {
			var cloud [4]float32
			for k, uv := range [4][2]float32{{p.u[0], p.v[0]}, {p.u[1], p.v[0]}, {p.u[0], p.v[1]}, {p.u[1], p.v[1]}} {
				cloud[k] = mix4(clouds, uv[0], uv[1])
			}
			b.d.weather.OvercastOn(f, mark, p.w, cloud)
		}
	}
	d.DrawBlends(f, cam, depth)
	d.DrawWay(f, cam, depth)
}

// nearPiece is a piece of a tile the eye stands among: its share of the tile across (u) and down
// (v), its corners in the world at heights z and on the screen, and how far it lies from the eye,
// squared.
type nearPiece struct {
	u, v [2]float32
	w    render.World
	z    [4]float32
	c    render.Corners
	dist float32
}

// hazed has the sprite just added turn to the sky as far off as its corners at pts lie in the
// weather's air (air.Weather.Haze): through a perspective, in air that is not clear without end.
func hazed(f *render.Frame, cam camera.Camera, weather air.Weather, pts [4][3]float32) {
	var h [4]float32
	for k, p := range pts {
		if h[k] = weather.Haze(cam, p[0], p[1], p[2]); h[k] == 0 && k == 0 {
			return // no eye, or no end to the air: nothing far off hazes
		}
	}
	f.Fog(h)
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
