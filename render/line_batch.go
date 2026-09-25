package render

import (
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
)

// LineBatch batches straight lines in screen space into as few DrawTriangles calls as the 16-bit
// index space allows: each line is a quad of its width, filled with its colour.
type LineBatch struct {
	vertices []ebiten.Vertex
	indices  []uint16
	triOpts  *ebiten.DrawTrianglesOptions
	chunk    int
}

func NewLineBatch() *LineBatch { return &LineBatch{triOpts: &ebiten.DrawTrianglesOptions{}} }

// Reset empties the batch for a frame.
func (b *LineBatch) Reset() { b.vertices, b.indices, b.chunk = b.vertices[:0], b.indices[:0], 0 }

// Len is how many lines were appended since Reset.
func (b *LineBatch) Len() int { return len(b.vertices) / 4 }

// Append adds the line from (x0, y0) to (x1, y1) on screen, width pixels wide, in c.
func (b *LineBatch) Append(x0, y0, x1, y1, width float32, c color.Color) {
	dx, dy := x1-x0, y1-y0
	n := float32(math.Hypot(float64(dx), float64(dy)))
	if n == 0 {
		return
	}
	// half the width across the line
	px, py := -dy/n*width/2, dx/n*width/2
	// c is premultiplied, a vertex takes straight alpha
	r, g, bl, a := c.RGBA()
	if a == 0 {
		return
	}
	ca := float32(a) / 0xffff
	cr, cg, cb := float32(r)/float32(a), float32(g)/float32(a), float32(bl)/float32(a)
	if len(b.vertices)-b.chunk == chunkVertices {
		b.chunk = len(b.vertices)
	}
	idx := uint16(len(b.vertices) - b.chunk)
	for _, p := range [4][2]float32{{x0 + px, y0 + py}, {x1 + px, y1 + py}, {x0 - px, y0 - py}, {x1 - px, y1 - py}} {
		b.vertices = append(b.vertices, ebiten.Vertex{DstX: p[0], DstY: p[1], SrcX: 1, SrcY: 1, ColorR: cr, ColorG: cg, ColorB: cb, ColorA: ca})
	}
	b.indices = append(b.indices, idx, idx+1, idx+2, idx+1, idx+2, idx+3)
}

// Flush draws every line appended since Reset, one call per chunk of vertices.
func (b *LineBatch) Flush(screen *ebiten.Image) {
	if len(b.vertices) == 0 {
		return
	}
	src := whitePixel()
	for start := 0; start < len(b.vertices); start += chunkVertices {
		end := min(start+chunkVertices, len(b.vertices))
		screen.DrawTriangles(b.vertices[start:end], b.indices[start/4*6:end/4*6], src, b.triOpts)
	}
}

var white *ebiten.Image

// whitePixel is a white image the lines sample at (1, 1), clear of its edges.
func whitePixel() *ebiten.Image {
	if white == nil {
		white = ebiten.NewImage(3, 3)
		white.Fill(color.White)
	}
	return white
}
