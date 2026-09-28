package render

import (
	"fmt"
	"image/color"
	"slices"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
)

// Composer is a WorldRenderer drawing its Sources as one picture per viewport: every source hands
// its items to a Frame, the Composer orders them — by depth below the Marks when the camera's
// projection sorts — and draws each run of items sampling one sheet in one call.
type Composer struct {
	sources []Source
	frame   Frame
	white   whiteSheet

	verts   []ebiten.Vertex
	indices []uint16
	opts    *ebiten.DrawTrianglesShaderOptions
	// the shader's uniforms, kept and written over so a frame allocates none: the sun, the eye, the
	// time and the sun's strength, and the colours of the sun, the sky and the light from it
	sun, toward, glint           []float32
	sunColor, skyColor, ambience []float32
	wind, drift, weather         []float32
	start                        time.Time
	// draw issues one call; tests count them instead.
	draw func(screen *ebiten.Image, verts []ebiten.Vertex, indices []uint16, sheet *ebiten.Image)
}

var _ WorldRenderer = (*Composer)(nil)

// NewComposer takes the layers to compose, which must all be Sources.
func NewComposer(layers ...Layer) *Composer {
	c := &Composer{opts: &ebiten.DrawTrianglesShaderOptions{}, start: time.Now(),
		sun: make([]float32, 3), toward: make([]float32, 3), glint: make([]float32, 4),
		sunColor: make([]float32, 3), skyColor: make([]float32, 3), ambience: make([]float32, 3),
		wind: make([]float32, 2), drift: make([]float32, 2), weather: make([]float32, 4)}
	c.opts.Uniforms = map[string]any{"Sun": c.sun, "Toward": c.toward, "Glint": c.glint,
		"SunColor": c.sunColor, "SkyColor": c.skyColor, "Ambience": c.ambience,
		"Wind": c.wind, "Drift": c.drift, "Weather": c.weather}
	for _, l := range layers {
		src, ok := l.(Source)
		if !ok {
			panic(fmt.Sprintf("render: %T cannot be composed: it is no Source", l))
		}
		c.sources = append(c.sources, src)
	}
	c.draw = c.drawTriangles
	return c
}

func (c *Composer) Init(si *goke.SysInit) {
	for _, s := range c.sources {
		s.Init(si)
	}
}

// DrawWorld composes the frame through cam and draws it; a nil screen only composes.
func (c *Composer) DrawWorld(screen *ebiten.Image, cam camera.Camera) {
	c.compose(cam)
	if screen != nil {
		c.render(screen)
	}
}

// Composed is how many pieces the last frame held — for measuring without a screen.
func (c *Composer) Composed() int { return c.frame.Len() }

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

// render draws the ordered items, one call per run sharing a sheet; a plain colour joins the run
// it falls in and samples that sheet's white texel.
func (c *Composer) render(screen *ebiten.Image) {
	f := &c.frame
	toward := f.cam.Projection().Toward()
	day := &f.daylight
	copy(c.sun, day.Dir[:])
	copy(c.toward, toward[:])
	c.glint[0], c.glint[1] = f.time, day.Strength
	c.glint[2] = 1 // world units a pixel spans: what is finer than a few of them is not drawn
	if z := f.cam.Zoom(); z > 0 {
		c.glint[2] = 1 / z
	}
	copy(c.sunColor, day.Sun[:])
	copy(c.skyColor, day.Sky[:])
	copy(c.ambience, day.Ambient[:])
	air := &f.weather
	copy(c.wind, air.Wind[:])
	copy(c.drift, air.Drift[:])
	c.weather[0] = air.Clouds
	var sheet AtlasSource
	c.verts, c.indices = c.verts[:0], c.indices[:0]
	for _, i := range f.order {
		it := &f.items[i]
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
}

// fan adds a fan item, whole, to the call being gathered.
func (c *Composer) fan(screen *ebiten.Image, sheet AtlasSource, it *item) {
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
func (c *Composer) append(verts []ebiten.Vertex, plain bool, sheet AtlasSource) {
	c.verts = append(c.verts, verts...)
	if !plain {
		return
	}
	wx, wy := sheet.White()
	for k := len(c.verts) - len(verts); k < len(c.verts); k++ {
		c.verts[k].SrcX, c.verts[k].SrcY = wx, wy
	}
}

func (c *Composer) flush(screen *ebiten.Image, sheet AtlasSource) {
	if len(c.verts) > 0 && sheet != nil {
		c.draw(screen, c.verts, c.indices, sheet.Atlas())
	}
	c.verts, c.indices = c.verts[:0], c.indices[:0]
}

func (c *Composer) drawTriangles(screen *ebiten.Image, verts []ebiten.Vertex, indices []uint16, sheet *ebiten.Image) {
	c.opts.Images[0] = sheet
	screen.DrawTrianglesShader(verts, indices, shader(), c.opts)
}

// chunkVertices is how many vertices one call may index: a multiple of four under 65536.
const chunkVertices = 65532

// whiteSheet is a sheet of nothing but white, for colours drawn before any sprite.
type whiteSheet struct{ img *ebiten.Image }

func (w *whiteSheet) Atlas() *ebiten.Image {
	if w.img == nil {
		w.img = ebiten.NewImage(3, 3)
		w.img.Fill(color.White)
	}
	return w.img
}
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
