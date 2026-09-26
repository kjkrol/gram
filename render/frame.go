package render

import (
	"image/color"
	"math"
	"slices"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/kjkrol/gram/camera"
)

// Tier is which part of the picture an item belongs to, drawn in order of value; the gaps leave a
// game room for tiers of its own.
type Tier int

const (
	Backdrop Tier = 0   // what lies behind the world: the sky
	Ground   Tier = 100 // the tiles and what the ground is made of
	Objects  Tier = 200 // what stands: entities, tall terrain
	Overlays Tier = 300 // what lies on the world: routes, sight
	Air      Tier = 350 // what fills the air before the eye: rain, snow
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
	quad   shape = iota // four corners, two triangles meeting along top-right to bottom-left
	fan                 // a fan round the first vertex
	folded              // a quad whose triangles meet along top-left to bottom-right
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
	// the last sprite added: where its vertices begin, whether it is a SpriteRect's pieces, where
	// in the picture it lies and on what sheet
	lastFirst int
	lastRect  bool
	lastTier  Tier
	lastDepth float32
	lastAtlas AtlasSource
	lastShape shape
	// the world's light and weather, set by whoever lights it, for what glints, reflects the sky,
	// lies in the clouds' shadow and sways; the composer's clock
	daylight Daylight
	weather  Weather
	time     float32
}

// Camera is the camera the frame is drawn through.
func (f *Frame) Camera() camera.Camera { return f.cam }

// Len is how many pieces the frame holds: quads and fans.
func (f *Frame) Len() int { return f.count }

// Reset starts the frame over, drawn through cam; a Composer does it every frame.
func (f *Frame) Reset(cam camera.Camera) {
	f.cam, f.items, f.verts, f.order, f.count = cam, f.items[:0], f.verts[:0], f.order[:0], 0
	f.daylight, f.weather = Daylight{}, Weather{}
}

// Weather is the air over the world as a frame needs it: the wind, world units a second along x
// and y, how far it has carried the clouds and how much of the sky they cover, 0 to 1.
type Weather struct {
	Wind, Drift [2]float32
	Clouds      float32
}

// Weather sets the frame's weather; the source that draws the ground says so, and a frame without
// it is a calm, clear day.
func (f *Frame) Weather(w Weather) { f.weather = w }

// Wind is the frame's wind, for what sways in it.
func (f *Frame) Wind() [2]float32 { return f.weather.Wind }

// Clouds is how much of the frame's sky the clouds cover, 0 to 1: under a clear one nothing need
// be laid for their shadows.
func (f *Frame) Clouds() float32 { return f.weather.Clouds }

// Time is the composer's clock, in seconds, for what moves by itself: the waves, the clouds,
// what sways.
func (f *Frame) Time() float32 { return f.time }

// Daylight is the world's light as what glints and reflects the sky needs it: the way towards the
// sun, its strength and colour, the colour of the sky, and the light every surface gets from it.
type Daylight struct {
	Dir      [3]float32
	Strength float32
	Sun, Sky Light
	Ambient  Light
}

// Daylight sets the frame's light; the source that lights the world says so, and a frame without
// it glints nowhere and reflects black.
func (f *Frame) Daylight(d Daylight) { f.daylight = d }

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
	if n := len(f.items); n > 0 && s != fan {
		last := &f.items[n-1]
		if last.shape == s && last.tier == tier && last.depth == depth && last.atlas == atlas {
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

// Light is a colour of light — red, green and blue — a colour drawn in it is scaled by: 1 each is
// white, as drawn, 0.5 each half as bright.
type Light [3]float32

// Shade is the light a piece is drawn in at its corners — top-left, top-right, bottom-left,
// bottom-right — blended across it.
type Shade [4]Light

// Even is a white Shade of v at every corner; Even(1) draws a sprite as it is.
func Even(v float32) Shade { return Lit(Light{v, v, v}) }

// Lit is a Shade of l at every corner.
func Lit(l Light) Shade { return Shade{l, l, l, l} }

// lit is the vertex colour for brightness v.
func lit(l Light) [4]float32 { return [4]float32{l[0], l[1], l[2], 1} }

// Sprite draws sprite id of atlas over the screen corners dst, as bright as shade says.
func (f *Frame) Sprite(tier Tier, depth float32, atlas AtlasSource, id SpriteID, dst Corners, shade Shade) {
	f.lastFirst, f.lastRect, f.lastTier, f.lastDepth, f.lastAtlas, f.lastShape = len(f.verts), false, tier, depth, atlas, quad
	u0, v0, u1, v1 := inset(atlas.UV(id))
	f.verts = append(f.verts,
		vertex(dst[0][0], dst[0][1], u0, v0, lit(shade[0])), vertex(dst[1][0], dst[1][1], u1, v0, lit(shade[1])),
		vertex(dst[2][0], dst[2][1], u0, v1, lit(shade[2])), vertex(dst[3][0], dst[3][1], u1, v1, lit(shade[3])))
	f.add(tier, depth, atlas, quad, 4)
}

// Glaze is Sprite laid over what lies under it as much as opacity says at each of its corners, 0
// to 1, blended across it: one look turning into another along a piece.
func (f *Frame) Glaze(tier Tier, depth float32, atlas AtlasSource, id SpriteID, dst Corners, shade Shade, opacity [4]float32) {
	f.Sprite(tier, depth, atlas, id, dst, shade)
	v := f.verts[len(f.verts)-4:]
	for i := range v {
		o := min(max(opacity[i], 0), 1)
		v[i].ColorR, v[i].ColorG, v[i].ColorB, v[i].ColorA = v[i].ColorR*o, v[i].ColorG*o, v[i].ColorB*o, o
	}
}

// Fold has the last sprite added — a Sprite, Tile or SpritePart whose corners stand at heights z —
// meet its two triangles along the diagonal whose corners stand nearer in height: a corner
// standing apart from the other three bends the quad towards it rather than standing up as a fin.
// What is laid over it later folds with it.
func (f *Frame) Fold(z [4]float32) {
	n := len(f.items)
	if n == 0 || f.lastRect || f.lastShape != quad || abs32(z[0]-z[3]) >= abs32(z[1]-z[2]) {
		return
	}
	last := &f.items[n-1]
	if last.shape != quad || int(last.first+last.count) != f.lastFirst+4 {
		return
	}
	last.count -= 4
	if last.count == 0 {
		f.items = f.items[:n-1]
	}
	f.items = append(f.items, item{tier: f.lastTier, depth: f.lastDepth, atlas: f.lastAtlas, first: int32(f.lastFirst), count: 4, shape: folded})
	f.lastShape = folded
}

// SpritePart is Sprite drawing the part src — x0, y0, x1, y1 in the sheet's pixels — of atlas's
// sheet rather than a sprite of it.
func (f *Frame) SpritePart(tier Tier, depth float32, atlas AtlasSource, src [4]float32, dst Corners, shade Shade) {
	f.lastFirst, f.lastRect, f.lastTier, f.lastDepth, f.lastAtlas, f.lastShape = len(f.verts), false, tier, depth, atlas, quad
	u0, v0, u1, v1 := src[0], src[1], src[2], src[3]
	f.verts = append(f.verts,
		vertex(dst[0][0], dst[0][1], u0, v0, lit(shade[0])), vertex(dst[1][0], dst[1][1], u1, v0, lit(shade[1])),
		vertex(dst[2][0], dst[2][1], u0, v1, lit(shade[2])), vertex(dst[3][0], dst[3][1], u1, v1, lit(shade[3])))
	f.add(tier, depth, atlas, quad, 4)
}

// SpriteBlend draws sprite id over dst where weight, blended between its corners, is over a half,
// fading in over soft either side of it (0 to a half): the look of one ground running into another
// along the line the corners' weights draw, not along the edges of the quad.
func (f *Frame) SpriteBlend(tier Tier, depth float32, atlas AtlasSource, id SpriteID, dst Corners, shade Shade, weight [4]float32, soft float32) {
	f.Sprite(tier, depth, atlas, id, dst, shade)
	v := f.verts[len(f.verts)-4:]
	for i := range v {
		v[i].ColorA, v[i].Custom3 = weight[i], blendMark+min(max(soft, 0.01), 0.5)
	}
}

// blendMark is what a blended sprite's last custom starts from, over any fade's, how soft its
// edge is above it.
const blendMark = 10

// Tile is Sprite outlined along its four edges, half a pixel inside each: tiles side by side show a
// grid a pixel wide at no cost of its own.
func (f *Frame) Tile(tier Tier, depth float32, atlas AtlasSource, id SpriteID, dst Corners, shade Shade) {
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
func (f *Frame) TileRect(tier Tier, depth float32, atlas AtlasSource, id SpriteID, x0, y0, x1, y1 float32, shade Shade) {
	first := len(f.verts)
	f.SpriteRectUV(tier, depth, atlas, id, x0, y0, x1, y1, 0, 0, 1, 1, shade)
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
// camera, split where it crosses a wrap seam, as bright as shade says at the rectangle's corners.
func (f *Frame) SpriteRect(tier Tier, depth float32, atlas AtlasSource, id SpriteID, x0, y0, x1, y1 float32, shade Shade) {
	f.SpriteRectUV(tier, depth, atlas, id, x0, y0, x1, y1, 0, 0, 1, 1, shade)
}

// SpriteRectUV is SpriteRect showing only the part u0..u1, v0..v1 of the sprite, 0 to 1 across it.
func (f *Frame) SpriteRectUV(tier Tier, depth float32, atlas AtlasSource, id SpriteID, x0, y0, x1, y1, u0, v0, u1, v1 float32, shade Shade) {
	f.lastFirst, f.lastRect, f.lastTier, f.lastDepth, f.lastAtlas, f.lastShape = len(f.verts), true, tier, depth, atlas, quad
	sx0, sy0, sx1, sy1 := inset(atlas.UV(id))
	w, h := sx1-sx0, sy1-sy0
	f.quads = f.cam.ToScreenQuads(x0, y0, x1, y1, f.quads[:0])
	for _, q := range f.quads {
		pu0, pu1 := u0+q.T0X*(u1-u0), u0+q.T1X*(u1-u0)
		pv0, pv1 := v0+q.T0Y*(v1-v0), v0+q.T1Y*(v1-v0)
		a0, b0, a1, b1 := sx0+pu0*w, sy0+pv0*h, sx0+pu1*w, sy0+pv1*h
		// a piece of the rectangle takes the shade the rectangle has where the piece's corners are
		f.verts = append(f.verts,
			vertex(q.X0, q.Y0, a0, b0, lit(shade.at(q.T0X, q.T0Y))), vertex(q.X1, q.Y0, a1, b0, lit(shade.at(q.T1X, q.T0Y))),
			vertex(q.X0, q.Y1, a0, b1, lit(shade.at(q.T0X, q.T1Y))), vertex(q.X1, q.Y1, a1, b1, lit(shade.at(q.T1X, q.T1Y))))
		f.add(tier, depth, atlas, quad, 4)
	}
}

// World is where each corner of a piece lies in the world — top-left, top-right, bottom-left,
// bottom-right, as its Corners on screen — for what the shader works out where it lies.
type World [4][2]float32

// Box is the World of the world box (x0, y0)-(x1, y1).
func Box(x0, y0, x1, y1 float32) World { return World{{x0, y0}, {x1, y0}, {x0, y1}, {x1, y1}} }

// Overlay is a quad laid over a sprite for a material to work out in the shader (see
// RegisterMaterials): where the sprite's corners lie in the world, and what the material reads at
// each corner — Red, a Fraction 0 to 1, and four Custom values — blended across the quad. Under
// has the overlay take how faint the sprite under it is into its red and how it fades or blends
// into its custom, as a shadow over it must; Blended, over a sprite drawn with SpriteBlend, takes
// the sprite's weight and mark into its custom's third and fourth.
type Overlay struct {
	Material MaterialID
	World    World
	Red      [4]float32
	Fraction [4]float32
	Custom   [4][4]float32
	Under    bool
	Blended  bool
}

// Overlay lays o over the last sprite added — a Sprite or Tile, or every piece of the last
// SpriteRect or TileRect — on its sheet's white texel.
func (f *Frame) Overlay(o *Overlay) {
	f.overlay(Mark{first: f.lastFirst, rect: f.lastRect, tier: f.lastTier, depth: f.lastDepth, atlas: f.lastAtlas, shape: f.lastShape, quads: f.quads}, o)
}

// Mark is a sprite already in a frame (Frame.Last), for an overlay laid over it later.
type Mark struct {
	first int
	rect  bool
	tier  Tier
	depth float32
	atlas AtlasSource
	shape shape
	quads []camera.Quad // a SpriteRect's pieces; none when it is one whole
}

// Last is the last sprite added, for an overlay laid over it later (OverlayOn).
func (f *Frame) Last() Mark {
	m := Mark{first: f.lastFirst, rect: f.lastRect, tier: f.lastTier, depth: f.lastDepth, atlas: f.lastAtlas, shape: f.lastShape}
	if f.lastRect && !whole(f.quads) {
		m.quads = slices.Clone(f.quads)
	}
	return m
}

// OverlayOn is Overlay laid over the sprite m, over all drawn on it since at its tier and depth.
func (f *Frame) OverlayOn(m Mark, o *Overlay) { f.overlay(m, o) }

// whole reports whether a SpriteRect's pieces are one, the whole of it.
func whole(quads []camera.Quad) bool {
	return len(quads) == 0 || len(quads) == 1 && quads[0].T0X == 0 && quads[0].T0Y == 0 && quads[0].T1X == 1 && quads[0].T1Y == 1
}

func (f *Frame) overlay(m Mark, o *Overlay) {
	if m.atlas == nil {
		return
	}
	wu, wv := m.atlas.White()
	w := o.World
	xs, ys := [4]float32{w[0][0], w[1][0], w[2][0], w[3][0]}, [4]float32{w[0][1], w[1][1], w[2][1], w[3][1]}
	mark := [4]float32{}
	for k := range mark {
		mark[k] = overlayMark + 2*float32(o.Material) + min(max(o.Fraction[k], 0), 1)
	}
	custom := func(i int, u, v float32) float32 {
		return blend([4]float32{o.Custom[0][i], o.Custom[1][i], o.Custom[2][i], o.Custom[3][i]}, u, v)
	}
	piece := func(first int, u0, v0, u1, v1 float32) {
		for k, uv := range [4][2]float32{{u0, v0}, {u1, v0}, {u0, v1}, {u1, v1}} {
			u, v := uv[0], uv[1]
			vx := ebiten.Vertex{DstX: f.verts[first+k].DstX, DstY: f.verts[first+k].DstY, SrcX: wu, SrcY: wv,
				ColorR: blend(o.Red, u, v), ColorG: blend(xs, u, v), ColorB: blend(ys, u, v), ColorA: blend(mark, u, v),
				Custom0: custom(0, u, v), Custom1: custom(1, u, v), Custom2: custom(2, u, v), Custom3: custom(3, u, v)}
			s := f.verts[first+k]
			if o.Under {
				vx.ColorR, vx.Custom0, vx.Custom1, vx.Custom2, vx.Custom3 = s.ColorA, s.Custom0, s.Custom1, s.Custom2, s.Custom3
			}
			if o.Blended && s.Custom3 > blendMark-4.5 {
				vx.Custom2, vx.Custom3 = s.ColorA, s.Custom3
			}
			f.verts = append(f.verts, vx)
		}
		f.add(m.tier, m.depth, m.atlas, m.shape, 4)
	}
	if !m.rect || whole(m.quads) {
		// the whole quad: each corner takes its own, nothing blended
		for k := range 4 {
			s := f.verts[m.first+k]
			c := o.Custom[k]
			vx := ebiten.Vertex{DstX: s.DstX, DstY: s.DstY, SrcX: wu, SrcY: wv, ColorR: o.Red[k], ColorG: w[k][0], ColorB: w[k][1],
				ColorA: mark[k], Custom0: c[0], Custom1: c[1], Custom2: c[2], Custom3: c[3]}
			if o.Under {
				vx.ColorR, vx.Custom0, vx.Custom1, vx.Custom2, vx.Custom3 = s.ColorA, s.Custom0, s.Custom1, s.Custom2, s.Custom3
			}
			if o.Blended && s.Custom3 > blendMark-4.5 {
				vx.Custom2, vx.Custom3 = s.ColorA, s.Custom3
			}
			f.verts = append(f.verts, vx)
		}
		f.add(m.tier, m.depth, m.atlas, m.shape, 4)
		return
	}
	for k, q := range m.quads {
		piece(m.first+4*k, q.T0X, q.T0Y, q.T1X, q.T1Y)
	}
}

// overlayMark is what an overlay's alpha starts from, telling the shader it is one — no colour's
// is over 1 — twice its material's number and its fraction above it.
const overlayMark = 2

// at is the light at (u, v) across the piece, 0 to 1 each way, blended from its corners.
func (s Shade) at(u, v float32) Light {
	switch { // a corner is its own light: an unsplit piece needs no blending
	case u == 0 && v == 0:
		return s[0]
	case u == 1 && v == 0:
		return s[1]
	case u == 0 && v == 1:
		return s[2]
	case u == 1 && v == 1:
		return s[3]
	}
	var l Light
	for c := range l {
		l[c] = blend([4]float32{s[0][c], s[1][c], s[2][c], s[3][c]}, u, v)
	}
	return l
}

// blend is the value at (u, v) across a piece, 0 to 1 each way, from its corners'.
func blend(corners [4]float32, u, v float32) float32 {
	top := corners[0] + (corners[1]-corners[0])*u
	bottom := corners[2] + (corners[3]-corners[2])*u
	return top + (bottom-top)*v
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
	customs := fadeCustoms(dst, fade)
	for i, p := range dst {
		v := vertex(p[0], p[1], 0, 0, col)
		v.Custom0, v.Custom1, v.Custom2, v.Custom3 = customs[i][0], customs[i][1], customs[i][2], customs[i][3]
		f.verts = append(f.verts, v)
	}
	f.add(tier, depth, nil, quad, 4)
}

// fadeCustoms is what each corner of dst holds for the shader to fade the quad out towards the
// sides fade names — left, right, top, bottom — over so many pixels each.
func fadeCustoms(dst Corners, fade Fade) [4][4]float32 {
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
	var out [4][4]float32
	for i := range dst {
		left, top := i%2 == 0, i < 2
		out[i] = [4]float32{side(fade.Left, width[i], left), side(fade.Right, width[i], !left), side(fade.Top, height[i], top), side(fade.Bottom, height[i], !top)}
	}
	return out
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
