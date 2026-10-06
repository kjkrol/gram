package render

import (
	"github.com/kjkrol/gram/camera"
)

// spritesShader draws a Sprites' run (shaders/sprites.wgsl).
var spritesShader = NewMeshShaderWith("sprites", Files(shaderFiles, "shaders/sprites.wgsl"), []Uniform{{Name: "ViewSize", Size: 2}}).Instanced(3)

// Sprites is sprites a Direct source draws on the GPU as instances — each a rectangle of the screen,
// a part of an atlas's sprite over it, a light — in the order they came, one call a run sharing an
// atlas: what Frame.SpriteRectUV would lay, piece for piece, without the frame.
type Sprites struct {
	runs      []spriteRun
	instances []float32 // three vec4s a piece: its rectangle on screen, on the atlas, its light and the angle it is turned by
	quads     []camera.Quad
	opts      DrawMeshOptions
	view      []float32
}

// spriteRun is the pieces from first, count of them, sampling one atlas.
type spriteRun struct {
	atlas        AtlasSource
	first, count int
}

// Rect adds sprite id of atlas over the world rectangle (x0, y0)-(x1, y1) through cam, the part
// u0..u1, v0..v1 of it, split where it crosses a wrap seam, in light — as Frame.SpriteRectUV lays
// it.
func (s *Sprites) Rect(cam camera.Camera, atlas AtlasSource, id SpriteID, x0, y0, x1, y1, u0, v0, u1, v1 float32, light Light) {
	s.quads = spritePieces(cam, atlas, id, x0, y0, x1, y1, u0, v0, u1, v1, s.quads[:0], func(q camera.Quad, a0, b0, a1, b1 float32) {
		if n := len(s.runs); n == 0 || s.runs[n-1].atlas != atlas {
			s.runs = append(s.runs, spriteRun{atlas: atlas, first: len(s.instances) / 12})
		}
		s.runs[len(s.runs)-1].count++
		s.instances = append(s.instances, q.X0, q.Y0, q.X1, q.Y1, a0, b0, a1, b1, light[0], light[1], light[2], 0)
	})
}

// Turned adds the whole of sprite id of atlas over the world rectangle (x0, y0)-(x1, y1)
// through cam, turned by angle radians about its middle, in light — one piece per wrap image,
// never split across a seam.
func (s *Sprites) Turned(cam camera.Camera, atlas AtlasSource, id SpriteID, x0, y0, x1, y1, angle float32, light Light) {
	sx0, sy0, sx1, sy1 := atlas.UV(id)
	s.quads = cam.ToScreenQuads(x0, y0, x1, y1, s.quads[:0])
	for _, q := range s.quads {
		if n := len(s.runs); n == 0 || s.runs[n-1].atlas != atlas {
			s.runs = append(s.runs, spriteRun{atlas: atlas, first: len(s.instances) / 12})
		}
		s.runs[len(s.runs)-1].count++
		s.instances = append(s.instances, q.X0, q.Y0, q.X1, q.Y1, sx0, sy0, sx1, sy1, light[0], light[1], light[2], angle)
	}
}

// Len is how many pieces are gathered.
func (s *Sprites) Len() int { return len(s.instances) / 12 }

// Reset drops what was gathered.
func (s *Sprites) Reset() {
	s.runs, s.instances = s.runs[:0], s.instances[:0]
}

// Draw draws the gathered sprites into the target's screen, a viewport cam's size, and starts
// gathering anew.
func (s *Sprites) Draw(t Target, cam camera.Camera) {
	defer s.Reset()
	if t.Screen == nil || len(s.runs) == 0 {
		return
	}
	w, h := cam.Viewport()
	if s.view == nil {
		s.view = make([]float32, 2)
		s.opts.Uniforms = map[string]any{"ViewSize": s.view}
	}
	s.view[0], s.view[1] = w, h
	s.opts.Vertices = 6
	for _, r := range s.runs {
		s.opts.Images[0] = r.atlas.Atlas()
		s.opts.Instances = s.instances[12*r.first : 12*(r.first+r.count)]
		t.Screen.DrawMesh(nil, spritesShader, &s.opts)
	}
}

// spritePieces calls piece for every piece of sprite id of atlas over the world rectangle (x0,
// y0)-(x1, y1) through cam, the part u0..u1, v0..v1 of it — where on screen, and where on the atlas
// its corners lie — split where it crosses a wrap seam; quads is its scratch, given back.
func spritePieces(cam camera.Camera, atlas AtlasSource, id SpriteID, x0, y0, x1, y1, u0, v0, u1, v1 float32, quads []camera.Quad, piece func(q camera.Quad, a0, b0, a1, b1 float32)) []camera.Quad {
	sx0, sy0, sx1, sy1 := inset(atlas.UV(id))
	w, h := sx1-sx0, sy1-sy0
	quads = cam.ToScreenQuads(x0, y0, x1, y1, quads)
	for _, q := range quads {
		pu0, pu1 := u0+q.T0X*(u1-u0), u0+q.T1X*(u1-u0)
		pv0, pv1 := v0+q.T0Y*(v1-v0), v0+q.T1Y*(v1-v0)
		piece(q, sx0+pu0*w, sy0+pv0*h, sx0+pu1*w, sy0+pv1*h)
	}
	return quads
}
