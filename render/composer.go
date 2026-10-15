package render

import (
	"slices"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/render/gpu"
)

// Composer is a Picture drawing its Sources as one picture: the opaque ground first, writing the
// depth, then the world back to front by where its pieces lie (At), each tested against what lies
// nearer, then the marks on top in the order they came; a run sampling one sheet in one call.
type Composer struct {
	sources []Source
	directs []Direct // by source, nil for one that is no Direct
	frame   Frame
	white   whiteSheet
	// how the frame's runs meet the depth: none in a frame without a ground; the ground's alone
	// for what lies over the figures, and the one bound to the run being gathered
	depth  *Depth
	ground *Depth
	bound  *Depth
	test   gpu.DepthTest
	sight  gpu.Sight
	seen   bool // the sight holds the camera's
	// the pieces' vertices carry their heights, for a sight to make depths of: a frame's with a
	// ground, a still's always
	heights bool

	verts   []Vertex
	indices []uint16
	// kept and written over so a frame allocates none; boxed for a Direct source
	uniforms map[string][]float32
	boxed    map[string]any
	packed   []byte
	depths   []depths // a pair for each size of target drawn into, the latest last
	start    time.Time
	draw     func(screen *Image, verts []Vertex, indices []uint16, sheet *Image)
	inited   bool
}

var _ Picture = (*Composer)(nil)

// NewComposer composes sources into one picture; a nil source is left out.
func NewComposer(sources ...Source) *Composer {
	c := &Composer{start: time.Now(), uniforms: map[string][]float32{}, boxed: map[string]any{}}
	c.uniform("Toward", 3)
	c.uniform("Clock", 1)
	c.uniform("Pixel", 1)
	for _, src := range sources {
		if src == nil {
			continue
		}
		c.sources = append(c.sources, src)
		d, _ := src.(Direct)
		c.directs = append(c.directs, d)
	}
	c.draw = c.drawTriangles
	return c
}

// Init readies the sources once, however many scenes show the picture.
func (c *Composer) Init(si *goke.SysInit) {
	if c.inited {
		return
	}
	c.inited = true
	for _, s := range c.sources {
		s.Init(si)
	}
}

// DrawWorld composes the frame through cam and draws it; a nil screen only composes.
func (c *Composer) DrawWorld(screen *Image, cam camera.Camera) {
	c.compose(cam)
	if screen == nil {
		c.setUniforms()
		return
	}
	c.render(screen)
}

// Composed is how many pieces the last frame held.
func (c *Composer) Composed() int { return c.frame.Len() }

// Uniforms is what the last frame handed the shader, by name.
func (c *Composer) Uniforms() map[string][]float32 { return c.uniforms }

func (c *Composer) compose(cam camera.Camera) {
	c.frame.Reset(cam)
	c.frame.time = float32(c.clock().Seconds())
	for i, s := range c.sources {
		c.frame.source = int32(i + 1)
		s.Compose(&c.frame, cam)
	}
	c.frame.source = 0
	c.sort()
}

// Clocked is a Source that keeps a game time for the frame's animations to go by instead of the
// composer's own clock; false while it has none.
type Clocked interface {
	Clock() (time.Duration, bool)
}

func (c *Composer) clock() time.Duration {
	for _, s := range c.sources {
		if k, ok := s.(Clocked); ok {
			if t, has := k.Clock(); has {
				return t
			}
		}
	}
	return time.Since(c.start)
}

// sort orders the world back to front by key, as it came at one key, what lies over the figures
// after it alike, and the marks last by their levels
func (c *Composer) sort() {
	f := &c.frame
	for i := range f.items {
		if !f.items[i].at.top && !f.items[i].at.over {
			f.order = append(f.order, int32(i))
		}
	}
	world := len(f.order)
	if f.overs > 0 {
		for i := range f.items {
			if f.items[i].at.over && !f.items[i].at.top {
				f.order = append(f.order, int32(i))
			}
		}
	}
	over := len(f.order)
	for i := range f.items {
		if f.items[i].at.top {
			f.order = append(f.order, int32(i))
		}
	}
	cmp := func(a, b int32) int {
		x, y := f.items[a].at.key, f.items[b].at.key
		switch {
		case x < y:
			return -1
		case x > y:
			return 1
		}
		return 0
	}
	// sources mostly hand their pieces in order already: a walk costs less than a sort
	for _, run := range [3][]int32{f.order[:world], f.order[world:over], f.order[over:]} {
		if !slices.IsSortedFunc(run, cmp) {
			slices.SortStableFunc(run, cmp)
		}
	}
}

// inOrder has the frame drawn in the order its pieces came
func (c *Composer) inOrder() {
	f := &c.frame
	for i := range f.items {
		f.order = append(f.order, int32(i))
	}
}

func (c *Composer) uniform(name string, n int) []float32 {
	u, ok := c.uniforms[name]
	if !ok || len(u) != n {
		u = make([]float32, n)
		c.uniforms[name] = u
		c.boxed[name] = u
	}
	return u
}

func (c *Composer) setUniforms() {
	f := &c.frame
	for _, u := range c.uniforms {
		clear(u)
	}
	toward := f.cam.Projection().Toward()
	copy(c.uniform("Toward", 3), toward[:])
	c.uniform("Clock", 1)[0] = f.time
	pixel := float32(1) // world units a pixel spans
	if z := f.cam.Zoom(); z > 0 {
		pixel = 1 / z
	}
	c.uniform("Pixel", 1)[0] = pixel
	for _, u := range f.uniforms {
		copy(c.uniform(u.name, int(u.n)), u.v[:u.n])
	}
}

// depths are the depth buffers of one size of target: the frame's, and the ground's alone, read
// by what lies on it
type depths struct {
	w, h         int
	main, ground *Depth
}

// the sizes a composer keeps buffers for: split screens and a minimap share one
const keptDepths = 4

func (c *Composer) depthsFor(screen *Image) *depths {
	w, h := screen.tex.Size()
	for i := range c.depths {
		if c.depths[i].w == w && c.depths[i].h == h {
			return &c.depths[i]
		}
	}
	if len(c.depths) == keptDepths {
		c.depths[0].main.Release()
		c.depths[0].ground.Release()
		c.depths = append(c.depths[:0], c.depths[1:]...)
	}
	c.depths = append(c.depths, depths{w: w, h: h, main: NewDepth(), ground: NewDepth().Readable()})
	return &c.depths[len(c.depths)-1]
}

func (c *Composer) render(screen *Image) {
	c.setUniforms()
	c.packed = append(c.packed[:0], composer.pack(c.boxed)...)
	f := &c.frame
	t := Target{Screen: screen, Pass: Opaque}
	c.depth, c.ground, c.seen = nil, nil, false
	if len(f.opaque) > 0 {
		var d *depths
		if screen != nil {
			d = c.depthsFor(screen)
			t.Depth = d.main
			if f.ground {
				// the ground drawn into it, then copied: the main one is never read
				t.Depth = d.ground
			}
			screen.ClearDepth(t.Depth)
		}
		for _, i := range f.opaque {
			if i > 0 && c.directs[i-1] != nil {
				c.directs[i-1].Draw(t, f.cam, Uniforms{c.boxed})
			}
		}
		if d != nil {
			t.Depth = d.main
			if f.ground {
				screen.CopyDepth(d.ground, d.main)
				t.Ground, c.ground = d.ground, d.ground
			}
			c.depth, c.seen = d.main, c.sightOf(f.cam)
		}
	}
	c.heights = c.seen
	t.Pass = SeeThrough
	c.paint(screen, t)
}

// sightOf has the sight say how cam turns the pieces' heights into depths, false for a camera that
// cannot say its lines of sight
func (c *Composer) sightOf(cam camera.Camera) bool {
	rays, ok := cam.(camera.Rays)
	if !ok {
		return false
	}
	f, ok := rays.Rays()
	if !ok {
		return false
	}
	w, h := cam.Viewport()
	tr, ok := camera.SceneTransform(f, w, h)
	if !ok {
		return false
	}
	m := tr.M
	c.sight = gpu.Sight{Origin: f.Origin, DX: f.DX, DY: f.DY, Dir: f.Dir, DDX: f.DDX, DDY: f.DDY,
		Z: [4]float32{m[2], m[6], m[10], m[14]}, W: [4]float32{m[3], m[7], m[11], m[15]},
		Eye: tr.Eye, Bend: tr.Bend}
	return true
}

func (c *Composer) paint(screen *Image, target Target) {
	f := &c.frame
	var sheet AtlasSource
	c.verts, c.indices = c.verts[:0], c.indices[:0]
	c.bound, c.test = c.depth, gpu.Tested
	for _, i := range f.order {
		it := &f.items[i]
		if it.direct > 0 {
			c.flush(screen, sheet)
			if int(it.direct) <= len(c.directs) && c.directs[it.direct-1] != nil {
				c.directs[it.direct-1].Draw(target, f.cam, Uniforms{c.boxed})
			}
			continue
		}
		bound, test := c.depth, gpu.Tested
		switch {
		case it.at.top:
			test = gpu.Untested
		case it.at.over && c.ground != nil:
			bound = c.ground
		}
		if bound != c.bound || test != c.test {
			c.flush(screen, sheet)
			c.bound, c.test = bound, test
		}
		switch {
		case it.atlas != nil && it.atlas != sheet:
			c.flush(screen, sheet)
			sheet = it.atlas
		case it.atlas == nil && sheet == nil:
			sheet = &c.white
		}
		if it.shape == fan {
			c.fan(screen, sheet, it)
			continue
		}
		if len(c.verts)+int(it.count) > chunkVertices {
			c.flush(screen, sheet)
		}
		// the whole run of quads at once where the call has room for it, else quad by quad
		if int(it.count) <= chunkVertices {
			base := uint16(len(c.verts))
			c.append(f.verts[it.first:it.first+it.count], it, sheet)
			for k := int32(0); k < it.count; k += 4 {
				c.indices = appendQuad(c.indices, base+uint16(k), it.shape)
			}
			continue
		}
		for k := it.first; k < it.first+it.count; k += 4 {
			if len(c.verts)+4 > chunkVertices {
				c.flush(screen, sheet)
			}
			base := uint16(len(c.verts))
			c.append(f.verts[k:k+4], it, sheet)
			c.indices = appendQuad(c.indices, base, it.shape)
		}
	}
	c.flush(screen, sheet)
}

func (c *Composer) fan(screen *Image, sheet AtlasSource, it *item) {
	if len(c.verts)+int(it.count) > chunkVertices {
		c.flush(screen, sheet)
	}
	base := uint16(len(c.verts))
	c.append(c.frame.verts[it.first:it.first+it.count], it, sheet)
	for k := uint16(1); k+1 < uint16(it.count); k++ {
		c.indices = append(c.indices, base, base+k, base+k+1)
	}
}

// append takes the piece's vertices at its height, a plain colour's onto the sheet's white
func (c *Composer) append(verts []Vertex, it *item, sheet AtlasSource) {
	from := len(c.verts)
	c.verts = append(c.verts, verts...)
	switch {
	case c.heights && !it.own:
		for k := from; k < len(c.verts); k++ {
			c.verts[k].DstZ = it.at.z
		}
	case !c.heights && it.own:
		// with no depth to meet the vertex's z is the depth itself: one at a height falls outside
		for k := from; k < len(c.verts); k++ {
			c.verts[k].DstZ = 0
		}
	}
	if it.atlas != nil {
		return
	}
	wx, wy := sheet.White()
	for k := from; k < len(c.verts); k++ {
		c.verts[k].SrcX, c.verts[k].SrcY = wx, wy
	}
}

func (c *Composer) flush(screen *Image, sheet AtlasSource) {
	if len(c.verts) > 0 && sheet != nil {
		c.draw(screen, c.verts, c.indices, sheet.Atlas())
	}
	c.verts, c.indices = c.verts[:0], c.indices[:0]
}

func (c *Composer) drawTriangles(screen *Image, verts []Vertex, indices []uint16, sheet *Image) {
	dw := gpu.Draw{
		Target:   screen.gpu(),
		Program:  composer.program(),
		Images:   [4]gpu.Image{sheet.gpu()},
		Uniforms: c.packed,
		Blend:    gpu.SourceOver,
	}
	if c.bound != nil {
		dw.Depth, dw.Test = c.bound.d, c.test
	}
	if c.seen {
		dw.Sight = &c.sight
	}
	gpu.Triangles(&dw, verts, indices)
}

// a multiple of four under 65536, what 16-bit indices reach
const chunkVertices = 65532

type whiteSheet struct{ img *Image }

func (w *whiteSheet) Atlas() *Image {
	if w.img == nil {
		w.img = NewImage(3, 3)
		w.img.WritePixels(white9)
	}
	return w.img
}

var white9 = []byte{255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255}

func (w *whiteSheet) UV(SpriteID) (sx0, sy0, sx1, sy1 float32) { return 0, 0, 3, 3 }
func (w *whiteSheet) White() (u, v float32)                    { return 1.5, 1.5 }

func appendQuad(indices []uint16, base uint16, s shape) []uint16 {
	if s == folded {
		return append(indices, base, base+1, base+3, base, base+2, base+3)
	}
	return append(indices, base, base+1, base+2, base+1, base+2, base+3)
}
