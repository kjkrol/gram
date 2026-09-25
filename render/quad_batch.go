package render

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/kjkrol/gram/camera"
)

// QuadBatch batches textured quads from an AtlasSource into as few DrawTriangles calls as the
// 16-bit index space allows, transformed through the camera.Camera given to Reset.
type QuadBatch struct {
	atlas    AtlasSource
	camera   camera.Camera
	vertices []ebiten.Vertex
	indices  []uint16
	quads    []camera.Quad
	triOpts  *ebiten.DrawTrianglesOptions
	// chunk is where the vertices of the current DrawTriangles call begin.
	chunk int
}

// chunkVertices is how many vertices one DrawTriangles call may index: a multiple of four under 65536.
const chunkVertices = 65532

func NewQuadBatch(atlas AtlasSource) *QuadBatch {
	return &QuadBatch{atlas: atlas, triOpts: &ebiten.DrawTrianglesOptions{}}
}

// Reset empties the batch for a frame drawn through cam.
func (b *QuadBatch) Reset(cam camera.Camera) {
	b.vertices, b.indices, b.chunk, b.camera = b.vertices[:0], b.indices[:0], 0, cam
}

// AppendQuadUV appends a quad for the world box (x0,y0)-(x1,y1), sampling id's UV sub-rect.
func (b *QuadBatch) AppendQuadUV(x0, y0, x1, y1 float32, id SpriteID, u0, v0, u1, v1 float32) {
	sx0, sy0, sx1, sy1 := b.atlas.UV(id)
	spriteW, spriteH := sx1-sx0, sy1-sy0

	b.quads = b.camera.ToScreenQuads(x0, y0, x1, y1, b.quads[:0])
	for _, q := range b.quads {
		pu0, pu1 := u0+q.T0X*(u1-u0), u0+q.T1X*(u1-u0)
		pv0, pv1 := v0+q.T0Y*(v1-v0), v0+q.T1Y*(v1-v0)
		fsx0, fsy0 := sx0+pu0*spriteW, sy0+pv0*spriteH
		fsx1, fsy1 := sx0+pu1*spriteW, sy0+pv1*spriteH

		if len(b.vertices)-b.chunk == chunkVertices {
			b.chunk = len(b.vertices)
		}
		idx := uint16(len(b.vertices) - b.chunk)
		b.vertices = append(b.vertices,
			ebiten.Vertex{DstX: q.X0, DstY: q.Y0, SrcX: fsx0, SrcY: fsy0, ColorR: 1, ColorG: 1, ColorB: 1, ColorA: 1},
			ebiten.Vertex{DstX: q.X1, DstY: q.Y0, SrcX: fsx1, SrcY: fsy0, ColorR: 1, ColorG: 1, ColorB: 1, ColorA: 1},
			ebiten.Vertex{DstX: q.X0, DstY: q.Y1, SrcX: fsx0, SrcY: fsy1, ColorR: 1, ColorG: 1, ColorB: 1, ColorA: 1},
			ebiten.Vertex{DstX: q.X1, DstY: q.Y1, SrcX: fsx1, SrcY: fsy1, ColorR: 1, ColorG: 1, ColorB: 1, ColorA: 1},
		)
		b.indices = append(b.indices, idx, idx+1, idx+2, idx+1, idx+2, idx+3)
	}
}

// AppendQuad is AppendQuadUV over the sprite's full region — the common case (no slicing).
func (b *QuadBatch) AppendQuad(x0, y0, x1, y1 float32, id SpriteID) {
	b.AppendQuadUV(x0, y0, x1, y1, id, 0, 0, 1, 1)
}

// AppendCorners appends the sprite over four screen corners already projected — top-left,
// top-right, bottom-left, bottom-right — for a quad that is no rectangle on screen.
func (b *QuadBatch) AppendCorners(dst Corners, id SpriteID) {
	sx0, sy0, sx1, sy1 := inset(b.atlas.UV(id))
	if len(b.vertices)-b.chunk == chunkVertices {
		b.chunk = len(b.vertices)
	}
	idx := uint16(len(b.vertices) - b.chunk)
	b.vertices = append(b.vertices,
		ebiten.Vertex{DstX: dst[0][0], DstY: dst[0][1], SrcX: sx0, SrcY: sy0, ColorR: 1, ColorG: 1, ColorB: 1, ColorA: 1},
		ebiten.Vertex{DstX: dst[1][0], DstY: dst[1][1], SrcX: sx1, SrcY: sy0, ColorR: 1, ColorG: 1, ColorB: 1, ColorA: 1},
		ebiten.Vertex{DstX: dst[2][0], DstY: dst[2][1], SrcX: sx0, SrcY: sy1, ColorR: 1, ColorG: 1, ColorB: 1, ColorA: 1},
		ebiten.Vertex{DstX: dst[3][0], DstY: dst[3][1], SrcX: sx1, SrcY: sy1, ColorR: 1, ColorG: 1, ColorB: 1, ColorA: 1},
	)
	b.indices = append(b.indices, idx, idx+1, idx+2, idx+1, idx+2, idx+3)
}

// ProjectCorners is the four corners of the world box (x0, y0)-(x1, y1) at height z through cam.
func ProjectCorners(cam camera.Camera, x0, y0, x1, y1, z float32) Corners {
	var out Corners
	for i, p := range [4][2]float32{{x0, y0}, {x1, y0}, {x0, y1}, {x1, y1}} {
		out[i][0], out[i][1] = cam.Project(p[0], p[1], z)
	}
	return out
}

// Billboard is a sprite w x h world units large standing upright at the world point (x, y, z):
// on screen, a rectangle whose bottom edge is centred on the projected point.
func Billboard(cam camera.Camera, x, y, z, w, h float32) Corners {
	sx, sy := cam.Project(x, y, z)
	zoom := cam.Zoom()
	hw, hh := w*zoom/2, h*zoom
	return Corners{{sx - hw, sy - hh}, {sx + hw, sy - hh}, {sx - hw, sy}, {sx + hw, sy}}
}

// Flush draws every quad appended since Reset, one call per chunk of vertices.
func (b *QuadBatch) Flush(screen *ebiten.Image) {
	for start := 0; start < len(b.vertices); start += chunkVertices {
		end := min(start+chunkVertices, len(b.vertices))
		screen.DrawTriangles(b.vertices[start:end], b.indices[start/4*6:end/4*6], b.atlas.Atlas(), b.triOpts)
	}
}
