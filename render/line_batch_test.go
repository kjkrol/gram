package render

import (
	"image/color"
	"math"
	"testing"
)

func TestLineBatch_ALineIsAQuadOfItsWidthInItsColour(t *testing.T) {
	b := NewLineBatch()
	b.Append(10, 20, 30, 20, 2, color.RGBA{R: 255, A: 255})
	if b.Len() != 1 || len(b.indices) != 6 {
		t.Fatalf("%d lines, %d indices; want one quad", b.Len(), len(b.indices))
	}
	v := b.vertices
	if v[0].DstY-v[2].DstY != 2 && v[2].DstY-v[0].DstY != 2 {
		t.Errorf("the quad spans y %v to %v, want 2 across the line", v[0].DstY, v[2].DstY)
	}
	if v[0].DstX != 10 || v[1].DstX != 30 {
		t.Errorf("the quad runs x %v to %v, want 10 to 30", v[0].DstX, v[1].DstX)
	}
	if v[0].ColorR != 1 || v[0].ColorG != 0 || v[0].ColorA != 1 {
		t.Errorf("colour %v %v %v %v, want opaque red", v[0].ColorR, v[0].ColorG, v[0].ColorB, v[0].ColorA)
	}
}

func TestLineBatch_TakesTheColourStraightFromItsPremultipliedForm(t *testing.T) {
	b := NewLineBatch()
	b.Append(0, 0, 10, 0, 1, color.RGBA{R: 20, G: 20, B: 20, A: 120})
	if v := b.vertices[0]; math.Abs(float64(v.ColorR)-20.0/120) > 1e-6 || math.Abs(float64(v.ColorA)-120.0/255) > 1e-6 {
		t.Errorf("colour %v alpha %v, want %v alpha %v", v.ColorR, v.ColorA, 20.0/120, 120.0/255)
	}
}

func TestLineBatch_ADiagonalKeepsItsWidthAcross(t *testing.T) {
	b := NewLineBatch()
	b.Append(0, 0, 10, 10, 1, color.White)
	v := b.vertices
	if w := math.Hypot(float64(v[0].DstX-v[2].DstX), float64(v[0].DstY-v[2].DstY)); math.Abs(w-1) > 1e-6 {
		t.Errorf("the diagonal is %v wide, want 1", w)
	}
}

func TestLineBatch_SkipsAPointAndRestartsIndicesPerChunk(t *testing.T) {
	b := NewLineBatch()
	b.Append(5, 5, 5, 5, 1, color.White)
	if b.Len() != 0 {
		t.Fatalf("a line of no length made %d quads", b.Len())
	}
	lines := chunkVertices/4 + 3
	for i := range lines {
		x := float32(i)
		b.Append(x, 0, x, 1, 1, color.White)
	}
	if first := b.indices[chunkVertices/4*6]; first != 0 {
		t.Errorf("the first index past the chunk is %d, want 0", first)
	}
	b.Reset()
	if b.Len() != 0 {
		t.Errorf("%d lines after Reset", b.Len())
	}
}
