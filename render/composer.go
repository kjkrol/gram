package render

import (
	"fmt"
	"slices"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/render/gpu"
)

// Composer is a Picture drawing its Sources as one picture per viewport: every source hands
// its items to a Frame, the Composer orders them — by depth below the Marks when the camera's
// projection sorts — and draws each run of items sampling one sheet in one call; a Direct source
// draws its part itself where its tier comes.
type Composer struct {
	sources []Source
	directs []Direct // by tier
	frame   Frame
	white   whiteSheet

	verts   []Vertex
	indices []uint16
	// the shader's uniforms, kept and written over so a frame allocates none: the composer's own —
	// the way towards the eye, the clock, the world units a pixel spans — and every one a source
	// set (Frame.Uniform), zeroed in a frame that did not; boxed, by name, for a Direct source
	uniforms map[string][]float32
	boxed    map[string]any
	packed   []byte // the uniforms laid out for this frame's draws
	depth    *Depth // the frame's meshes', handed to the Direct sources
	start    time.Time
	// draw issues one call; tests count them instead.
	draw func(screen *Image, verts []Vertex, indices []uint16, sheet *Image)
}

var _ Picture = (*Composer)(nil)

// NewComposer takes the layers to compose, which must all be Sources; a nil layer — a plugin with
// no renderer — is left out.
func NewComposer(layers ...Layer) *Composer {
	c := &Composer{start: time.Now(), uniforms: map[string][]float32{}, boxed: map[string]any{}, depth: NewDepth()}
	c.uniform("Toward", 3)
	c.uniform("Clock", 1)
	c.uniform("Pixel", 1)
	for _, l := range layers {
		if l == nil {
			continue
		}
		src, ok := l.(Source)
		if !ok {
			panic(fmt.Sprintf("render: %T cannot be composed: it is no Source", l))
		}
		c.sources = append(c.sources, src)
		if d, ok := l.(Direct); ok {
			c.directs = append(c.directs, d)
		}
	}
	slices.SortStableFunc(c.directs, func(a, b Direct) int { return int(a.Tier()) - int(b.Tier()) })
	c.draw = c.drawTriangles
	return c
}

func (c *Composer) Init(si *goke.SysInit) {
	for _, s := range c.sources {
		s.Init(si)
	}
}

// DrawWorld composes the frame through cam and draws it; a nil screen only composes and settles
// the shader's uniforms (Uniforms), for a test.
func (c *Composer) DrawWorld(screen *Image, cam camera.Camera) {
	c.compose(cam)
	if screen == nil {
		c.setUniforms()
		return
	}
	c.render(screen)
}

// Composed is how many pieces the last frame held — for measuring without a screen.
func (c *Composer) Composed() int { return c.frame.Len() }

// Uniforms is what the last frame handed the shader, by name — for a test of a material's source.
func (c *Composer) Uniforms() map[string][]float32 { return c.uniforms }

func (c *Composer) compose(cam camera.Camera) {
	c.frame.Reset(cam)
	c.frame.time = float32(c.clock().Seconds())
	for _, s := range c.sources {
		s.Compose(&c.frame, cam)
	}
	c.sort()
}

// Clocked is a Source that keeps a game time — the world's, going by its tactical clock — for the
// frame's animations to go by instead of the composer's own clock; false while it has none.
type Clocked interface {
	Clock() (time.Duration, bool)
}

// clock is the time the frame's animations go by: the first Clocked source's, else the composer's.
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

// sort orders the frame: through a projection that sorts the items below Marks back to front by
// depth, ties by tier, then the rest by tier; otherwise by tier alone; always the order given last.
func (c *Composer) sort() {
	f := &c.frame
	for i := range f.items {
		f.order = append(f.order, int32(i))
	}
	byDepth := f.cam.Projection().Sorts()
	cmp := func(a, b int32) int {
		x, y := &f.items[a], &f.items[b]
		if byDepth {
			mx, my := x.tier >= Marks, y.tier >= Marks
			switch {
			case mx != my:
				if mx {
					return 1
				}
				return -1
			case !mx && x.depth != y.depth:
				if x.depth < y.depth {
					return -1
				}
				return 1
			}
		}
		return int(x.tier) - int(y.tier)
	}
	// sources mostly hand their items in order already: a walk costs less than a sort
	if !slices.IsSortedFunc(f.order, cmp) {
		slices.SortStableFunc(f.order, cmp)
	}
}

// uniform is the slice the uniform name is handed to the shader in, made once, n long.
func (c *Composer) uniform(name string, n int) []float32 {
	u, ok := c.uniforms[name]
	if !ok || len(u) != n {
		u = make([]float32, n)
		c.uniforms[name] = u
		c.boxed[name] = u
	}
	return u
}

// setUniforms hands the shader the composer's own uniforms and the frame's, every other it has
// ever been handed zeroed.
func (c *Composer) setUniforms() {
	f := &c.frame
	for _, u := range c.uniforms {
		clear(u)
	}
	toward := f.cam.Projection().Toward()
	copy(c.uniform("Toward", 3), toward[:])
	c.uniform("Clock", 1)[0] = f.time
	pixel := float32(1) // world units a pixel spans: what is finer than a few of them is not drawn
	if z := f.cam.Zoom(); z > 0 {
		pixel = 1 / z
	}
	c.uniform("Pixel", 1)[0] = pixel
	for _, u := range f.uniforms {
		copy(c.uniform(u.name, int(u.n)), u.v[:u.n])
	}
}

// render draws the ordered items under the frame's uniforms, the Direct sources where their tiers
// come.
func (c *Composer) render(screen *Image) {
	c.setUniforms()
	c.packed = append(c.packed[:0], composer.pack(c.boxed)...)
	if screen != nil {
		screen.ClearDepth(c.depth)
	}
	c.paint(screen, Target{Screen: screen, Depth: c.depth})
}

// paint draws the ordered items, one call per run sharing a sheet; a plain colour joins the run it
// falls in and samples that sheet's white texel. A Direct source draws before the first item of
// its tier or over, after all before it.
func (c *Composer) paint(screen *Image, target Target) {
	f := &c.frame
	var sheet AtlasSource
	c.verts, c.indices = c.verts[:0], c.indices[:0]
	direct := 0
	for _, i := range f.order {
		it := &f.items[i]
		for direct < len(c.directs) && c.directs[direct].Tier() <= it.tier {
			c.flush(screen, sheet)
			c.directs[direct].Draw(target, f.cam, Uniforms{c.boxed})
			direct++
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
			c.append(f.verts[it.first:it.first+it.count], it.atlas == nil, sheet)
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
			c.append(f.verts[k:k+4], it.atlas == nil, sheet)
			c.indices = appendQuad(c.indices, base, it.shape)
		}
	}
	c.flush(screen, sheet)
	for ; direct < len(c.directs); direct++ {
		c.directs[direct].Draw(target, f.cam, Uniforms{c.boxed})
	}
}

// fan adds a fan item, whole, to the call being gathered.
func (c *Composer) fan(screen *Image, sheet AtlasSource, it *item) {
	if len(c.verts)+int(it.count) > chunkVertices {
		c.flush(screen, sheet)
	}
	base := uint16(len(c.verts))
	c.append(c.frame.verts[it.first:it.first+it.count], it.atlas == nil, sheet)
	for k := uint16(1); k+1 < uint16(it.count); k++ {
		c.indices = append(c.indices, base, base+k, base+k+1)
	}
}

// append copies verts into the call being gathered; a plain colour samples sheet's white texel.
func (c *Composer) append(verts []Vertex, plain bool, sheet AtlasSource) {
	c.verts = append(c.verts, verts...)
	if !plain {
		return
	}
	wx, wy := sheet.White()
	for k := len(c.verts) - len(verts); k < len(c.verts); k++ {
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
	gpu.Triangles(&gpu.Draw{Target: screen.gpu(), Program: composer.program(), Images: [4]gpu.Image{sheet.gpu()}, Uniforms: c.packed, Blend: gpu.SourceOver}, verts, indices)
}

// chunkVertices is how many vertices one call may index: a multiple of four under 65536.
const chunkVertices = 65532

// whiteSheet is a sheet of nothing but white, for colours drawn before any sprite.
type whiteSheet struct{ img *Image }

func (w *whiteSheet) Atlas() *Image {
	if w.img == nil {
		w.img = NewImage(3, 3)
		w.img.WritePixels(white9)
	}
	return w.img
}

// white9 is nine white pixels.
var white9 = []byte{255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255}

func (w *whiteSheet) UV(SpriteID) (sx0, sy0, sx1, sy1 float32) { return 0, 0, 3, 3 }
func (w *whiteSheet) White() (u, v float32)                    { return 1.5, 1.5 }

// appendQuad adds the two triangles of the quad whose vertices begin at base, meeting along the
// diagonal its shape says.
func appendQuad(indices []uint16, base uint16, s shape) []uint16 {
	if s == folded {
		return append(indices, base, base+1, base+3, base, base+2, base+3)
	}
	return append(indices, base, base+1, base+2, base+1, base+2, base+3)
}
