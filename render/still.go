package render

import (
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/render/gpu"
)

// Still is a picture composed once in world units, in the order its pieces come, kept on the GPU,
// and drawn frame after frame as a camera looking straight down shows it, placed and lit.
type Still struct {
	c    Composer
	view worldView
	runs []stillRun
}

type stillRun struct {
	sheet *Image
	kept  *gpu.Kept
}

// NewStill is a Still holding nothing yet.
func NewStill() *Still {
	s := &Still{}
	s.c.heights = true
	s.c.draw = func(_ *Image, verts []Vertex, indices []uint16, sheet *Image) {
		s.runs = append(s.runs, stillRun{sheet: sheet, kept: gpu.NewKept(verts, indices)})
	}
	return s
}

// Compose composes the still anew: compose hands the frame what lies on the world w by h units
// large, through a camera drawing a world unit a pixel.
func (s *Still) Compose(w, h float32, compose func(f *Frame, cam camera.Camera)) {
	s.view = worldView{w: w, h: h}
	s.c.frame.Reset(&s.view)
	compose(&s.c.frame, &s.view)
	s.c.inOrder()
	for _, r := range s.runs {
		r.kept.Release()
	}
	s.runs = s.runs[:0]
	s.c.paint(nil, Target{})
}

// Len is how many pieces the still holds.
func (s *Still) Len() int { return s.c.frame.Len() }

// Draw draws the still onto the target's screen zoom pixels a world unit, the world point (x, y)
// at the screen's top-left, its colours times light, under the uniforms u of the frame it is drawn
// in; into the target's depth at its heights through cam, opaque, where the target has one.
func (s *Still) Draw(t Target, cam camera.Camera, u Uniforms, zoom, x, y float32, light Light) {
	if t.Screen == nil || len(s.runs) == 0 {
		return
	}
	s.c.packed = append(s.c.packed[:0], composer.pack(u.m)...)
	dw := gpu.Draw{
		Target:   t.Screen.gpu(),
		Program:  composer.program(),
		Uniforms: s.c.packed,
		Blend:    gpu.SourceOver,
		Place:    [4]float32{zoom, zoom, -x * zoom, -y * zoom},
		Tint:     [4]float32{light[0], light[1], light[2], 1},
	}
	dw.Sight = &level
	if t.Depth != nil && s.c.sightOf(cam) {
		dw.Depth, dw.Test, dw.Sight = t.Depth.d, gpu.Written, &s.c.sight
	}
	for _, r := range s.runs {
		dw.Images[0] = r.sheet.gpu()
		gpu.DrawKept(&dw, r.kept)
	}
}

// every height at depth 0: a still's, drawn where there is no depth to meet
var level = gpu.Sight{Dir: [3]float32{0, 0, -1}, W: [4]float32{0, 0, 0, 1}}

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
