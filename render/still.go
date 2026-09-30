package render

import (
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/render/gpu"
)

// Still is a picture composed once in world units and drawn frame after frame through a camera
// looking straight down: what stays as it is between frames — a flat board's tiles — composed anew
// only when it changes, kept on the GPU run by run as a Composer would draw its frame, and placed
// and lit there every frame under the uniforms of the frame it is drawn in.
type Still struct {
	c    Composer
	view worldView
	runs []stillRun
}

// stillRun is one call of a still: the triangles sampling one sheet, kept on the GPU.
type stillRun struct {
	sheet *Image
	kept  *gpu.Kept
}

// NewStill is a Still holding nothing yet.
func NewStill() *Still {
	s := &Still{}
	s.c.draw = func(_ *Image, verts []Vertex, indices []uint16, sheet *Image) {
		s.runs = append(s.runs, stillRun{sheet: sheet, kept: gpu.NewKept(verts, indices)})
	}
	return s
}

// Compose composes the still anew: compose hands the frame what lies on the world w by h units
// large, through a camera drawing a world unit a pixel, (0, 0) at the top-left.
func (s *Still) Compose(w, h float32, compose func(f *Frame, cam camera.Camera)) {
	s.view = worldView{w: w, h: h}
	s.c.frame.Reset(&s.view)
	compose(&s.c.frame, &s.view)
	s.c.sort()
	for _, r := range s.runs {
		r.kept.Release()
	}
	s.runs = s.runs[:0]
	s.c.paint(nil, Target{}) // into runs
}

// Len is how many pieces the still holds.
func (s *Still) Len() int { return s.c.frame.Len() }

// Draw draws the still onto screen zoom pixels a world unit, the world point (x, y) at the
// screen's top-left, its colours times light, under the uniforms u of the frame it is drawn in.
func (s *Still) Draw(screen *Image, u Uniforms, zoom, x, y float32, light Light) {
	if screen == nil || len(s.runs) == 0 {
		return
	}
	s.c.packed = append(s.c.packed[:0], composer.pack(u.m)...)
	dw := gpu.Draw{Target: screen.gpu(), Program: composer.program(), Uniforms: s.c.packed, Blend: gpu.SourceOver,
		Place: [4]float32{zoom, zoom, -x * zoom, -y * zoom}, Tint: [4]float32{light[0], light[1], light[2], 1}}
	for _, r := range s.runs {
		dw.Images[0] = r.sheet.gpu()
		gpu.DrawKept(&dw, r.kept)
	}
}

// worldView is the camera a Still is composed through: a world unit a pixel, straight down, the
// whole world w by h in view; it neither moves nor zooms.
type worldView struct{ w, h float32 }

var _ camera.Camera = (*worldView)(nil)

func (*worldView) Projection() camera.Projection                  { return camera.TopDown{} }
func (*worldView) Project(x, y, _ float32) (float32, float32)     { return x, y }
func (*worldView) Unproject(sx, sy, _ float32) (float32, float32) { return sx, sy }
func (*worldView) Depth(_, y, _ float32) float32                  { return y }
func (v *worldView) Viewport() (float32, float32)                 { return v.w, v.h }
func (*worldView) SetViewport(float32, float32)                   {}
func (*worldView) ToScreen(x, y float32) (float32, float32)       { return x, y }
func (*worldView) FromScreen(sx, sy float32) (float32, float32)   { return sx, sy }
func (v *worldView) Visible(box camera.AABB) bool                 { return v.Bounds().Intersects(box) }
func (v *worldView) Bounds() camera.AABB {
	return geom.NewAABBAt(geom.Vec{}, float64(v.w), float64(v.h))
}
func (*worldView) MoveTo(float64, float64)            {}
func (*worldView) CenterOn(float64, float64, float64) {}
func (*worldView) Translate(float64, float64)         {}
func (*worldView) Pan(float32, float32)               {}
func (*worldView) Zoom() float32                      { return 1 }
func (*worldView) ZoomIn(float32, float32, float32)   {}
func (*worldView) ZoomOut(float32, float32, float32)  {}
func (v *worldView) State() camera.State              { return camera.State{Viewport: v.Bounds(), Zoom: 1} }
func (*worldView) Persisted() []any                   { return nil }
func (*worldView) Restore()                           {}
func (*worldView) SetMinZoom(float32)                 {}
func (*worldView) SetMaxZoom(float32)                 {}

func (*worldView) ToScreenQuads(x0, y0, x1, y1 float32, dst []camera.Quad) []camera.Quad {
	return append(dst, camera.Quad{X0: x0, Y0: y0, X1: x1, Y1: y1, T0X: 0, T1X: 1, T0Y: 0, T1Y: 1})
}
