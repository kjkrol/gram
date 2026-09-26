package render

import (
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/kjkrol/gram/camera"
)

// Tier is which part of the picture an item belongs to, drawn in order of value; the gaps leave a
// game room for tiers of its own.
type Tier int

const (
	Ground   Tier = 100 // the tiles and what the ground is made of
	Objects  Tier = 200 // what stands: entities, tall terrain
	Overlays Tier = 300 // what lies on the world: routes, sight
	Marks    Tier = 400 // what must always show: the selection, the box being dragged
)

// Source is a world layer that hands its items to a Frame instead of drawing them; a Composer
// draws the items of all its sources as one picture.
type Source interface {
	Layer
	Compose(f *Frame, cam camera.Camera)
}

// Corners are a quad's screen corners: top-left, top-right, bottom-left, bottom-right.
type Corners [4][2]float32

// Fade is how many pixels each side of a Soft quad takes to fade out; 0 leaves a side hard.
type Fade struct{ Left, Right, Top, Bottom float32 }

// A vertex's Custom0..3 hold, per edge — left, right, top, bottom — 1 plus its distance to that
// edge in units of the edge's fade, or minus 1 minus its distance in pixels to an edge outlined;
// 0 leaves the edge alone, so a plain vertex needs no custom values at all.

// shape is how an item's vertices make triangles.
type shape uint8

const (
	quad shape = iota // four corners, two triangles
	fan               // a fan round the first vertex
)

// item is a run of things to draw sharing a place in the picture and a sheet (nil for a plain
// colour, which takes the white texel of whatever sheet it is drawn with): quads that came one
// after another, or one fan.
type item struct {
	tier  Tier
	depth float32
	atlas AtlasSource
	first int32
	count int32
	shape shape
}

// Frame gathers what the sources of a Composer draw for one viewport, in screen pixels, each item
// keyed by its tier, its depth and the order it came in.
type Frame struct {
	cam   camera.Camera
	items []item
	verts []ebiten.Vertex
	order []int32
	quads []camera.Quad
	count int // pieces: quads and fans
}

// Camera is the camera the frame is drawn through.
func (f *Frame) Camera() camera.Camera { return f.cam }

// Len is how many pieces the frame holds: quads and fans.
func (f *Frame) Len() int { return f.count }

// Reset starts the frame over, drawn through cam; a Composer does it every frame.
func (f *Frame) Reset(cam camera.Camera) {
	f.cam, f.items, f.verts, f.order, f.count = cam, f.items[:0], f.verts[:0], f.order[:0], 0
}

// Each calls fn with every piece in the order it came, its tier, depth and vertices — for tests
// and tools; the vertices are the frame's own.
func (f *Frame) Each(fn func(tier Tier, depth float32, verts []ebiten.Vertex)) {
	for _, it := range f.items {
		if it.shape == fan {
			fn(it.tier, it.depth, f.verts[it.first:it.first+it.count])
			continue
		}
		for k := it.first; k < it.first+it.count; k += 4 {
			fn(it.tier, it.depth, f.verts[k:k+4])
		}
	}
}

// add closes the piece of count vertices just appended: into the last item when it is a quad in
// the same place on the same sheet, else as an item of its own.
func (f *Frame) add(tier Tier, depth float32, atlas AtlasSource, s shape, count int) {
	f.count++
	if n := len(f.items); n > 0 && s == quad {
		last := &f.items[n-1]
		if last.shape == quad && last.tier == tier && last.depth == depth && last.atlas == atlas {
			last.count += int32(count)
			return
		}
	}
	f.items = append(f.items, item{tier: tier, depth: depth, atlas: atlas, first: int32(len(f.verts)) - int32(count), count: int32(count), shape: s})
}

// vertex is a point of an item with no fade, coloured c (premultiplied).
func vertex(x, y, sx, sy float32, c [4]float32) ebiten.Vertex {
	return ebiten.Vertex{DstX: x, DstY: y, SrcX: sx, SrcY: sy, ColorR: c[0], ColorG: c[1], ColorB: c[2], ColorA: c[3]}
}

// premultiplied is c as a vertex takes it; color.RGBA is premultiplied already.
func premultiplied(c color.RGBA) [4]float32 {
	return [4]float32{float32(c.R) / 0xff, float32(c.G) / 0xff, float32(c.B) / 0xff, float32(c.A) / 0xff}
}

// Sprite draws sprite id of atlas over the screen corners dst, its colour scaled by shade (1 as
// drawn, 0.5 half as bright).
func (f *Frame) Sprite(tier Tier, depth float32, atlas AtlasSource, id SpriteID, dst Corners, shade float32) {
	u0, v0, u1, v1 := inset(atlas.UV(id))
	c := [4]float32{shade, shade, shade, 1}
	f.verts = append(f.verts,
		vertex(dst[0][0], dst[0][1], u0, v0, c), vertex(dst[1][0], dst[1][1], u1, v0, c),
		vertex(dst[2][0], dst[2][1], u0, v1, c), vertex(dst[3][0], dst[3][1], u1, v1, c))
	f.add(tier, depth, atlas, quad, 4)
}

// Tile is Sprite outlined along its four edges, half a pixel inside each: tiles side by side show a
// grid a pixel wide at no cost of its own.
func (f *Frame) Tile(tier Tier, depth float32, atlas AtlasSource, id SpriteID, dst Corners, shade float32) {
	f.Sprite(tier, depth, atlas, id, dst, shade)
	v := f.verts[len(f.verts)-4:]
	for i, p := range dst {
		v[i].Custom0 = outline(p, dst[0], dst[2]) // left
		v[i].Custom1 = outline(p, dst[1], dst[3]) // right
		v[i].Custom2 = outline(p, dst[0], dst[1]) // top
		v[i].Custom3 = outline(p, dst[2], dst[3]) // bottom
	}
}

// TileRect is SpriteRect outlined along the world rectangle's edges; where a wrap seam splits it,
// the pieces are outlined only along the rectangle's own edges.
func (f *Frame) TileRect(tier Tier, depth float32, atlas AtlasSource, id SpriteID, x0, y0, x1, y1 float32) {
	first := len(f.verts)
	f.SpriteRect(tier, depth, atlas, id, x0, y0, x1, y1)
	for k, q := range f.quads {
		// the whole rectangle on screen, from the part of it this piece shows
		w, h := (q.X1-q.X0)/(q.T1X-q.T0X), (q.Y1-q.Y0)/(q.T1Y-q.T0Y)
		left, top := q.X0-q.T0X*w, q.Y0-q.T0Y*h
		v := f.verts[first+4*k : first+4*k+4]
		for i := range v {
			v[i].Custom0 = -1 - (v[i].DstX - left)
			v[i].Custom1 = -1 - (left + w - v[i].DstX)
			v[i].Custom2 = -1 - (v[i].DstY - top)
			v[i].Custom3 = -1 - (top + h - v[i].DstY)
		}
	}
}

// outline is the custom value of p for an outline along the edge from a to b: minus 1 minus its
// distance to the edge's line, in pixels.
func outline(p, a, b [2]float32) float32 {
	ex, ey := b[0]-a[0], b[1]-a[1]
	n := float32(math.Hypot(float64(ex), float64(ey)))
	if n == 0 {
		return 0
	}
	d := ((p[0]-a[0])*ey - (p[1]-a[1])*ex) / n
	return -1 - float32(math.Abs(float64(d)))
}

// SpriteRect draws sprite id over the world rectangle (x0, y0)-(x1, y1) through the frame's
// camera, split where it crosses a wrap seam.
func (f *Frame) SpriteRect(tier Tier, depth float32, atlas AtlasSource, id SpriteID, x0, y0, x1, y1 float32) {
	f.SpriteRectUV(tier, depth, atlas, id, x0, y0, x1, y1, 0, 0, 1, 1)
}

// SpriteRectUV is SpriteRect showing only the part u0..u1, v0..v1 of the sprite, 0 to 1 across it.
func (f *Frame) SpriteRectUV(tier Tier, depth float32, atlas AtlasSource, id SpriteID, x0, y0, x1, y1, u0, v0, u1, v1 float32) {
	sx0, sy0, sx1, sy1 := inset(atlas.UV(id))
	w, h := sx1-sx0, sy1-sy0
	c := [4]float32{1, 1, 1, 1}
	f.quads = f.cam.ToScreenQuads(x0, y0, x1, y1, f.quads[:0])
	for _, q := range f.quads {
		pu0, pu1 := u0+q.T0X*(u1-u0), u0+q.T1X*(u1-u0)
		pv0, pv1 := v0+q.T0Y*(v1-v0), v0+q.T1Y*(v1-v0)
		a0, b0, a1, b1 := sx0+pu0*w, sy0+pv0*h, sx0+pu1*w, sy0+pv1*h
		f.verts = append(f.verts,
			vertex(q.X0, q.Y0, a0, b0, c), vertex(q.X1, q.Y0, a1, b0, c),
			vertex(q.X0, q.Y1, a0, b1, c), vertex(q.X1, q.Y1, a1, b1, c))
		f.add(tier, depth, atlas, quad, 4)
	}
}

// Line draws the line from (x0, y0) to (x1, y1) on screen, width pixels wide, in c; its sides fade
// over a pixel instead of stepping.
func (f *Frame) Line(tier Tier, depth float32, x0, y0, x1, y1, width float32, c color.RGBA) {
	dx, dy := x1-x0, y1-y0
	n := float32(math.Hypot(float64(dx), float64(dy)))
	if n == 0 || width <= 0 {
		return
	}
	full := width + 1 // half a pixel of fade either side
	px, py := -dy/n*full/2, dx/n*full/2
	col := premultiplied(c)
	for i, p := range [4][2]float32{{x0 + px, y0 + py}, {x1 + px, y1 + py}, {x0 - px, y0 - py}, {x1 - px, y1 - py}} {
		v := vertex(p[0], p[1], 0, 0, col)
		// 1 plus the distance in pixels to either side
		if i < 2 {
			v.Custom0, v.Custom1 = 1, 1+full
		} else {
			v.Custom0, v.Custom1 = 1+full, 1
		}
		f.verts = append(f.verts, v)
	}
	f.add(tier, depth, nil, quad, 4)
}

// Fan fills in c the polygon pts, which every point of sees whole from pts[0]: a cone of sight.
func (f *Frame) Fan(tier Tier, depth float32, pts [][2]float32, c color.RGBA) {
	if len(pts) < 3 {
		return
	}
	col := premultiplied(c)
	for _, p := range pts {
		f.verts = append(f.verts, vertex(p[0], p[1], 0, 0, col))
	}
	f.add(tier, depth, nil, fan, len(pts))
}

// Soft fills the quad dst in c, fading towards each side over the pixels fade gives it.
func (f *Frame) Soft(tier Tier, depth float32, dst Corners, c color.RGBA, fade Fade) {
	col := premultiplied(c)
	// the distance from each corner across to the opposite side, along the quad's edges
	across := func(a, b [2]float32) float32 {
		return float32(math.Hypot(float64(b[0]-a[0]), float64(b[1]-a[1])))
	}
	width := [4]float32{across(dst[0], dst[1]), across(dst[0], dst[1]), across(dst[2], dst[3]), across(dst[2], dst[3])}
	height := [4]float32{across(dst[0], dst[2]), across(dst[1], dst[3]), across(dst[0], dst[2]), across(dst[1], dst[3])}
	side := func(w, dist float32, near bool) float32 {
		switch {
		case w <= 0:
			return 0
		case near:
			return 1
		}
		return 1 + dist/w
	}
	for i, p := range dst {
		v := vertex(p[0], p[1], 0, 0, col)
		left, top := i%2 == 0, i < 2
		v.Custom0 = side(fade.Left, width[i], left)
		v.Custom1 = side(fade.Right, width[i], !left)
		v.Custom2 = side(fade.Top, height[i], top)
		v.Custom3 = side(fade.Bottom, height[i], !top)
		f.verts = append(f.verts, v)
	}
	f.add(tier, depth, nil, quad, 4)
}

// ProjectCorners is the four corners of the world box (x0, y0)-(x1, y1) at height z through cam.
func ProjectCorners(cam camera.Camera, x0, y0, x1, y1, z float32) Corners {
	var out Corners
	for i, p := range [4][2]float32{{x0, y0}, {x1, y0}, {x0, y1}, {x1, y1}} {
		out[i][0], out[i][1] = cam.Project(p[0], p[1], z)
	}
	return out
}

// inset pulls a sprite's source rectangle in by half a texel, so the edge of a quad drawn at an
// angle or a fraction of a pixel never samples the neighbouring sprite of the sheet.
func inset(sx0, sy0, sx1, sy1 float32) (float32, float32, float32, float32) {
	return sx0 + 0.5, sy0 + 0.5, sx1 - 0.5, sy1 - 0.5
}
