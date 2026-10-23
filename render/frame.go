package render

import (
	"image/color"
	"math"
	"slices"

	"github.com/kjkrol/gram/camera"
)

// Source is what draws a part of the world by handing its pieces to a Frame.
type Source interface {
	Drawer
	Compose(f *Frame, cam camera.Camera)
}

// Overlying is a Source whose pieces lie over the figures' ground: the figures themselves, what
// falls or floats over them and the marks. A Laid picture composes and draws it in the pass over
// the layers of the ground, the rest in the ground's.
type Overlying interface{ Overlies() }

// Corners are a quad's screen corners: top-left, top-right, bottom-left, bottom-right.
type Corners [4][2]float32

// Fade is how many pixels each side of a Soft quad takes to fade out; 0 leaves a side hard.
type Fade struct{ Left, Right, Top, Bottom float32 }

// A vertex's Custom0..3 hold, per edge (left, right, top, bottom), 1 plus its distance to that
// edge in units of the edge's fade; 0 leaves the edge alone.

type shape uint8

const (
	quad shape = iota // four corners, two triangles meeting along top-right to bottom-left
	fan               // a fan round the first vertex
)

type item struct {
	at    At
	atlas AtlasSource // nil for a plain colour, drawn on whatever sheet's white texel
	first int32
	count int32 // 0 for a Direct's place among the pieces
	shape shape
	// the source drawing itself at this place, one more than its index; 0 a piece
	direct int32
	own    bool // the vertices hold heights of their own, not the At's
}

// Frame gathers what the sources of a Composer draw for one viewport, in screen pixels, each
// piece where it lies (At), and where the Direct sources draw themselves.
type Frame struct {
	cam   camera.Camera
	items []item
	verts []Vertex
	order []int32
	quads []camera.Quad
	count int

	lastFirst int
	lastRect  bool
	lastAt    At
	lastAtlas AtlasSource

	uniforms []uniform
	images   []frameImage
	time     float32
	ground   bool    // a source reads the ground's depth this frame
	overs    int     // the pieces lying over the figures
	source   int32   // the source composing, one more than its index; 0 none
	opaque   []int32 // the sources drawing in the opaque pass, one more than their index
}

type uniform struct {
	name string
	n    uint8
	v    [4]float32
}

type frameImage struct {
	name string
	img  *Image
}

// Len is how many pieces the frame holds: quads and fans.
func (f *Frame) Len() int { return f.count }

// Reset starts the frame over, drawn through cam.
func (f *Frame) Reset(cam camera.Camera) {
	f.cam, f.items, f.verts, f.order, f.count = cam, f.items[:0], f.verts[:0], f.order[:0], 0
	f.uniforms, f.ground, f.opaque, f.overs = f.uniforms[:0], false, f.opaque[:0], 0
	f.images = f.images[:0]
}

// DrawOpaque has the composing Direct draw in the opaque pass, the ground's: tested against the
// depth and writing it, before anything seen through.
func (f *Frame) DrawOpaque() { f.opaque = append(f.opaque, f.source) }

// DrawAt has the composing Direct draw among what is seen through, back to front, at at.
func (f *Frame) DrawAt(at At) {
	f.items = append(f.items, item{at: at, first: int32(len(f.verts)), direct: f.source})
}

// Places calls fn with every pass a Direct composing into f asked to draw in, and where.
func (f *Frame) Places(fn func(p Pass, at At)) {
	for range f.opaque {
		fn(Opaque, At{})
	}
	for _, it := range f.items {
		if it.count == 0 {
			fn(SeeThrough, it.at)
		}
	}
}

// ReadGround has the ground's depth kept apart this frame, for the composing source to read it
// (Target.Ground) once the ground is drawn.
func (f *Frame) ReadGround() {
	f.ground = true
}

// Uniform sets the shader's uniform name to v, a float or a vector of two to four, for this
// frame; what was not set is zero. The names Toward, Clock, Pixel and Fog are the composer's own.
func (f *Frame) Uniform(name string, v ...float32) {
	n := min(len(v), 4)
	for i := range f.uniforms {
		if f.uniforms[i].name == name {
			f.uniforms[i].n = uint8(n)
			copy(f.uniforms[i].v[:], v[:n])
			return
		}
	}
	u := uniform{name: name, n: uint8(n)}
	copy(u.v[:], v[:n])
	f.uniforms = append(f.uniforms, u)
}

// Image hands every shader drawing this frame the image registered as name (RegisterImage); a
// name not registered panics.
func (f *Frame) Image(name string, img *Image) {
	if !slices.Contains(images, name) {
		panic("render: no image is registered as " + name)
	}
	for i := range f.images {
		if f.images[i].name == name {
			f.images[i].img = img
			return
		}
	}
	f.images = append(f.images, frameImage{name: name, img: img})
}

// Uniformed reports whether a source composing this frame has set the uniform name already.
func (f *Frame) Uniformed(name string) bool {
	for i := range f.uniforms {
		if f.uniforms[i].name == name {
			return true
		}
	}
	return false
}

// Time is the frame's clock, in seconds, for what moves by itself.
func (f *Frame) Time() float32 { return f.time }

// Each calls fn with every piece in the order it came: where it lies and its vertices, the
// frame's own.
func (f *Frame) Each(fn func(at At, verts []Vertex)) {
	for _, it := range f.items {
		if it.shape == fan {
			fn(it.at, f.verts[it.first:it.first+it.count])
			continue
		}
		for k := it.first; k < it.first+it.count; k += 4 {
			fn(it.at, f.verts[k:k+4])
		}
	}
}

func (f *Frame) add(at At, atlas AtlasSource, s shape, count int) {
	f.addOwn(at, atlas, s, count, false)
}

func (f *Frame) addOwn(at At, atlas AtlasSource, s shape, count int, own bool) {
	f.count++
	if at.over {
		f.ground, f.overs = true, f.overs+1
	}
	if n := len(f.items); n > 0 && s != fan {
		last := &f.items[n-1]
		if last.count > 0 && last.shape == s && last.at == at && last.atlas == atlas &&
			last.own == own {
			last.count += int32(count)
			return
		}
	}
	f.items = append(f.items, item{
		at:    at,
		atlas: atlas,
		first: int32(len(f.verts)) - int32(count),
		count: int32(count),
		shape: s,
		own:   own,
	})
}

func vertex(x, y, sx, sy float32, c [4]float32) Vertex {
	return Vertex{DstX: x, DstY: y, SrcX: sx, SrcY: sy, ColorR: c[0], ColorG: c[1], ColorB: c[2], ColorA: c[3]}
}

// a colour as gram takes it, straight alpha, premultiplied for the GPU
func premultiplied(c color.RGBA) [4]float32 {
	a := float32(c.A) / 0xff
	return [4]float32{float32(c.R) / 0xff * a, float32(c.G) / 0xff * a, float32(c.B) / 0xff * a, a}
}

// Light is a colour of light, red, green and blue, a colour drawn in it is scaled by; 1 each is
// white.
type Light [3]float32

// Taking is which shade a drawn thing takes, bits of it: the relief's shadow, the baked blocks',
// the clouds', and which light, the lamps'. One taking none shines as it is drawn.
type Taking uint8

const (
	TakesRelief Taking = 1 << iota
	TakesBlocks
	TakesClouds
	TakesLamps
	// Shines takes nothing: the thing is drawn as it is.
	Shines Taking = 1 << 7
	// TakesAll takes every shade and the lamps' light, as a zero Taking does.
	TakesAll = TakesRelief | TakesBlocks | TakesClouds | TakesLamps
)

// Illumination is the light a thing is drawn in: Light on a surface facing up, the sun's and the
// lamps', Lamps the lamps' alone, and which shade it Takes.
type Illumination struct {
	Light, Lamps Light
	// Direct is the share of Light that is the distant light's, 0 to 1: what a shadow takes away.
	Direct float32
	Takes  Taking
}

// WhiteLight is a thing drawn as it is, taking every shade: a world without a light plugin.
var WhiteLight = Illumination{Light: Light{1, 1, 1}, Takes: TakesAll}

// Shade is the light a piece is drawn in at its corners, top-left, top-right, bottom-left,
// bottom-right, blended across it.
type Shade [4]Light

// Even is a white Shade of v at every corner; Even(1) draws a sprite as it is.
func Even(v float32) Shade { return Lit(Light{v, v, v}) }

// Lit is a Shade of l at every corner.
func Lit(l Light) Shade { return Shade{l, l, l, l} }

func lit(l Light) [4]float32 { return [4]float32{l[0], l[1], l[2], 1} }

// Sprite draws sprite id of atlas over the screen corners dst, as bright as shade says.
func (f *Frame) Sprite(at At, atlas AtlasSource, id SpriteID, dst Corners, shade Shade) {
	f.lastFirst, f.lastRect, f.lastAt = len(f.verts), false, at
	f.lastAtlas = atlas
	u0, v0, u1, v1 := inset(atlas.UV(id))
	f.verts = append(f.verts,
		vertex(dst[0][0], dst[0][1], u0, v0, lit(shade[0])), vertex(dst[1][0], dst[1][1], u1, v0, lit(shade[1])),
		vertex(dst[2][0], dst[2][1], u0, v1, lit(shade[2])), vertex(dst[3][0], dst[3][1], u1, v1, lit(shade[3])))
	f.add(at, atlas, quad, 4)
}

// Glaze is Sprite laid over what lies under it as much as opacity says at each of its corners, 0
// to 1.
func (f *Frame) Glaze(
	at At, atlas AtlasSource, id SpriteID, dst Corners, shade Shade, opacity [4]float32,
) {
	f.Sprite(at, atlas, id, dst, shade)
	v := f.verts[len(f.verts)-4:]
	for i := range v {
		o := min(max(opacity[i], 0), 1)
		v[i].ColorR, v[i].ColorG, v[i].ColorB, v[i].ColorA = v[i].ColorR*o, v[i].ColorG*o, v[i].ColorB*o, o
	}
}

// SpritePart is Sprite drawing the part src (x0, y0, x1, y1 in the sheet's pixels) of atlas's
// sheet rather than a sprite of it.
func (f *Frame) SpritePart(at At, atlas AtlasSource, src [4]float32, dst Corners, shade Shade) {
	f.lastFirst, f.lastRect, f.lastAt = len(f.verts), false, at
	f.lastAtlas = atlas
	u0, v0, u1, v1 := src[0], src[1], src[2], src[3]
	f.verts = append(f.verts,
		vertex(dst[0][0], dst[0][1], u0, v0, lit(shade[0])), vertex(dst[1][0], dst[1][1], u1, v0, lit(shade[1])),
		vertex(dst[2][0], dst[2][1], u0, v1, lit(shade[2])), vertex(dst[3][0], dst[3][1], u1, v1, lit(shade[3])))
	f.add(at, atlas, quad, 4)
}

// SpriteBlend draws sprite id over dst where weight, blended between its corners, is over a half,
// fading in over soft either side of it (0 to a half).
func (f *Frame) SpriteBlend(
	at At, atlas AtlasSource, id SpriteID, dst Corners, shade Shade, weight [4]float32,
	soft float32,
) {
	f.Sprite(at, atlas, id, dst, shade)
	v := f.verts[len(f.verts)-4:]
	for i := range v {
		v[i].ColorA, v[i].Custom3 = weight[i], blendMark+min(max(soft, 0.01), 0.5)
	}
}

// a blended sprite's last custom starts at blendMark, a fade stays under softCap: the shaders tell
// them apart at half the mark
const (
	blendMark = 100000
	softCap   = 10000
)

// SpriteRect draws sprite id over the world rectangle (x0, y0)-(x1, y1) through the frame's
// camera, split where it crosses a wrap seam.
func (f *Frame) SpriteRect(
	at At, atlas AtlasSource, id SpriteID, x0, y0, x1, y1 float32, shade Shade,
) {
	f.SpriteRectUV(at, atlas, id, x0, y0, x1, y1, 0, 0, 1, 1, shade)
}

// SpriteRectUV is SpriteRect showing only the part u0..u1, v0..v1 of the sprite, 0 to 1 across it.
func (f *Frame) SpriteRectUV(
	at At, atlas AtlasSource, id SpriteID, x0, y0, x1, y1, u0, v0, u1, v1 float32, shade Shade,
) {
	f.lastFirst, f.lastRect, f.lastAt = len(f.verts), true, at
	f.lastAtlas = atlas
	f.quads = spritePieces(f.cam, atlas, id, x0, y0, x1, y1, u0, v0, u1, v1, f.quads[:0], func(q camera.Quad, a0, b0, a1, b1 float32) {
		f.verts = append(f.verts,
			vertex(q.X0, q.Y0, a0, b0, lit(shade.at(q.T0X, q.T0Y))), vertex(q.X1, q.Y0, a1, b0, lit(shade.at(q.T1X, q.T0Y))),
			vertex(q.X0, q.Y1, a0, b1, lit(shade.at(q.T0X, q.T1Y))), vertex(q.X1, q.Y1, a1, b1, lit(shade.at(q.T1X, q.T1Y))))
		f.add(at, atlas, quad, 4)
	})
}

// World is where each corner of a piece lies in the world, in the order of its Corners on screen.
type World [4][2]float32

// Box is the World of the world box (x0, y0)-(x1, y1).
func Box(x0, y0, x1, y1 float32) World { return World{{x0, y0}, {x1, y0}, {x0, y1}, {x1, y1}} }

// Overlay is a quad laid over a sprite for a material to work out in the shader: where the
// sprite's corners lie in the world, and what the material reads at each corner, blended across
// the quad.
type Overlay struct {
	Material MaterialID
	World    World
	Red      [4]float32
	Fraction [4]float32 // 0 to 1
	Custom   [4][4]float32
	// Blended, over a SpriteBlend, takes the sprite's weight and mark into Custom's third and
	// fourth.
	Blended bool
}

// Overlay lays o over the last sprite added, every piece of a SpriteRect's.
func (f *Frame) Overlay(o *Overlay) {
	if f.lastAtlas == nil {
		return
	}
	wu, wv := f.lastAtlas.White()
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
			vx := Vertex{DstX: f.verts[first+k].DstX, DstY: f.verts[first+k].DstY, SrcX: wu, SrcY: wv,
				ColorR: blend(o.Red, u, v), ColorG: blend(xs, u, v), ColorB: blend(ys, u, v), ColorA: blend(mark, u, v),
				Custom0: custom(0, u, v), Custom1: custom(1, u, v), Custom2: custom(2, u, v), Custom3: custom(3, u, v)}
			if s := f.verts[first+k]; o.Blended && s.Custom3 > blendMark/2 {
				vx.Custom2, vx.Custom3 = s.ColorA, s.Custom3
			}
			f.verts = append(f.verts, vx)
		}
		f.add(f.lastAt, f.lastAtlas, quad, 4)
	}
	if !f.lastRect || whole(f.quads) {
		for k := range 4 {
			s := f.verts[f.lastFirst+k]
			c := o.Custom[k]
			vx := Vertex{DstX: s.DstX, DstY: s.DstY, SrcX: wu, SrcY: wv, ColorR: o.Red[k], ColorG: w[k][0], ColorB: w[k][1],
				ColorA: mark[k], Custom0: c[0], Custom1: c[1], Custom2: c[2], Custom3: c[3]}
			if o.Blended && s.Custom3 > blendMark/2 {
				vx.Custom2, vx.Custom3 = s.ColorA, s.Custom3
			}
			f.verts = append(f.verts, vx)
		}
		f.add(f.lastAt, f.lastAtlas, quad, 4)
		return
	}
	for k, q := range f.quads {
		piece(f.lastFirst+4*k, q.T0X, q.T0Y, q.T1X, q.T1Y)
	}
}

func whole(quads []camera.Quad) bool {
	return len(quads) == 0 || len(quads) == 1 && quads[0].T0X == 0 && quads[0].T0Y == 0 && quads[0].T1X == 1 && quads[0].T1Y == 1
}

// an overlay's alpha: overlayMark + twice its material + its fraction, over any colour's 1
const overlayMark = 2

func (s Shade) at(u, v float32) Light {
	switch {
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

func blend(corners [4]float32, u, v float32) float32 {
	top := corners[0] + (corners[1]-corners[0])*u
	bottom := corners[2] + (corners[3]-corners[2])*u
	return top + (bottom-top)*v
}

// Line draws the line from (x0, y0) to (x1, y1) on screen, width pixels wide, in c, its sides
// fading over a pixel.
func (f *Frame) Line(at At, x0, y0, x1, y1, width float32, c color.RGBA) {
	if f.line(x0, y0, x1, y1, width, c) {
		f.add(at, nil, quad, 4)
	}
}

// Edge is Line whose ends lie at the heights z0 and z1 in the world, each tested against the depth
// where it lies, the line drawn among the pieces at at.
func (f *Frame) Edge(at At, x0, y0, z0, x1, y1, z1, width float32, c color.RGBA) {
	n := len(f.verts)
	if !f.line(x0, y0, x1, y1, width, c) {
		return
	}
	for k, z := range [4]float32{z0, z1, z0, z1} {
		f.verts[n+k].DstZ = z
	}
	f.addOwn(at, nil, quad, 4, true)
}

func (f *Frame) line(x0, y0, x1, y1, width float32, c color.RGBA) bool {
	dx, dy := x1-x0, y1-y0
	n := float32(math.Hypot(float64(dx), float64(dy)))
	if n == 0 || width <= 0 {
		return false
	}
	full := width + 1 // half a pixel of fade either side
	px, py := -dy/n*full/2, dx/n*full/2
	col := premultiplied(c)
	for i, p := range [4][2]float32{{x0 + px, y0 + py}, {x1 + px, y1 + py}, {x0 - px, y0 - py}, {x1 - px, y1 - py}} {
		v := vertex(p[0], p[1], 0, 0, col)
		if i < 2 {
			v.Custom0, v.Custom1 = 1, 1+full
		} else {
			v.Custom0, v.Custom1 = 1+full, 1
		}
		f.verts = append(f.verts, v)
	}
	return true
}

// SpriteFan fills the polygon pts, which every point of sees whole from pts[0], with the part of
// sprite id under it when the sprite is laid over the screen box x0, y0, x1, y1, as opaque as
// opacity says, 0 to 1.
func (f *Frame) SpriteFan(at At, atlas AtlasSource, id SpriteID, pts [][2]float32,
	box [4]float32, opacity float32) {
	if len(pts) < 3 || box[2] == box[0] || box[3] == box[1] {
		return
	}
	sx0, sy0, sx1, sy1 := inset(atlas.UV(id))
	o := min(max(opacity, 0), 1)
	col := [4]float32{o, o, o, o}
	for _, p := range pts {
		u := min(max((p[0]-box[0])/(box[2]-box[0]), 0), 1)
		v := min(max((p[1]-box[1])/(box[3]-box[1]), 0), 1)
		f.verts = append(f.verts, vertex(p[0], p[1], sx0+u*(sx1-sx0), sy0+v*(sy1-sy0), col))
	}
	f.add(at, atlas, fan, len(pts))
}

// Fog has the last plain sprite added turn to the colour of the Fog uniform as much as amount
// says at each corner, 0 to 1; a rectangle's (SpriteRect) as much as its corners on average,
// every piece it lies in.
func (f *Frame) Fog(amount [4]float32) {
	if amount == ([4]float32{}) || len(f.verts) < f.lastFirst+4 {
		return
	}
	if f.lastRect {
		even := (amount[0] + amount[1] + amount[2] + amount[3]) / 4
		for at := f.lastFirst; at+4 <= len(f.verts); at += 4 {
			f.fog(f.verts[at:at+4], [4]float32{even, even, even, even})
		}
		return
	}
	f.fog(f.verts[f.lastFirst:f.lastFirst+4], amount)
}

func (f *Frame) fog(v []Vertex, amount [4]float32) {
	for i := range v {
		if v[i].ColorA != 1 {
			return
		}
	}
	for i := range v {
		v[i].ColorA = 1 + fogSpan*min(max(amount[i], 0), 1)
	}
}

// a fogged sprite's alpha stays under an overlay's mark, which the shader tells apart at 1.5
const fogSpan = 0.49

// Material lays o on its own, the screen quad dst for o's material to work out.
func (f *Frame) Material(at At, dst Corners, o *Overlay) {
	w := o.World
	for k, p := range dst {
		c := o.Custom[k]
		f.verts = append(f.verts, Vertex{DstX: p[0], DstY: p[1], ColorR: o.Red[k], ColorG: w[k][0], ColorB: w[k][1],
			ColorA: overlayMark + 2*float32(o.Material) + min(max(o.Fraction[k], 0), 1), Custom0: c[0], Custom1: c[1], Custom2: c[2], Custom3: c[3]})
	}
	f.add(at, nil, quad, 4)
}

// Quad fills dst in a colour per corner, blended across.
func (f *Frame) Quad(at At, dst Corners, c [4]color.RGBA) {
	for i, p := range dst {
		f.verts = append(f.verts, vertex(p[0], p[1], 0, 0, premultiplied(c[i])))
	}
	f.add(at, nil, quad, 4)
}

// Soft fills the quad dst in c, fading towards each side over the pixels fade gives it.
func (f *Frame) Soft(at At, dst Corners, c color.RGBA, fade Fade) {
	col := premultiplied(c)
	customs := fadeCustoms(dst, fade)
	for i, p := range dst {
		v := vertex(p[0], p[1], 0, 0, col)
		v.Custom0, v.Custom1, v.Custom2, v.Custom3 = customs[i][0], customs[i][1], customs[i][2], customs[i][3]
		f.verts = append(f.verts, v)
	}
	f.add(at, nil, quad, 4)
}

func fadeCustoms(dst Corners, fade Fade) [4][4]float32 {
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
		return min(1+dist/w, softCap)
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

// half a texel in, so a quad's edge never samples the neighbouring sprite of the sheet
func inset(sx0, sy0, sx1, sy1 float32) (float32, float32, float32, float32) {
	return sx0 + 0.5, sy0 + 0.5, sx1 - 0.5, sy1 - 0.5
}

// Branch readies dst to gather a part of f's picture on another goroutine, for Append to take
// back.
func (f *Frame) Branch(dst *Frame) {
	dst.Reset(f.cam)
	dst.time = f.time
}

// Append takes into f all that o holds, after what f holds already, in o's order.
func (f *Frame) Append(o *Frame) {
	base := int32(len(f.verts))
	f.verts = append(f.verts, o.verts...)
	for _, it := range o.items {
		it.first += base
		f.items = append(f.items, it)
	}
	f.count, f.overs = f.count+o.count, f.overs+o.overs
	f.ground = f.ground || o.ground
	f.opaque = append(f.opaque, o.opaque...)
	for _, u := range o.uniforms {
		f.Uniform(u.name, u.v[:u.n]...)
	}
	if o.count > 0 {
		f.lastFirst, f.lastRect, f.lastAt = o.lastFirst+int(base), o.lastRect, o.lastAt
		f.lastAtlas = o.lastAtlas
		f.quads = append(f.quads[:0], o.quads...)
	}
}
