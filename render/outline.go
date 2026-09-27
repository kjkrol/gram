package render

import (
	_ "embed"
	"math"
)

//go:embed outline.kage
var outlineKage []byte

// outlineMaterial draws a tile's outline over all laid on it (outline.kage).
var outlineMaterial = RegisterMaterials(outlineKage, "Outline")[0]

// OutlineOn outlines the sprite m along its own edges, over all drawn on it since: a Tile's
// outline that what lies on the tile does not cover. m is a Sprite, or a SpriteRect's pieces
// outlined along the whole rectangle's edges.
func (f *Frame) OutlineOn(m Mark) {
	var dst Corners
	switch {
	case !m.rect || len(m.quads) == 0:
		for k := range dst {
			v := f.verts[m.first+k]
			dst[k] = [2]float32{v.DstX, v.DstY}
		}
	default:
		// the whole rectangle on screen, from the part of it the first piece shows
		q := m.quads[0]
		w, h := (q.X1-q.X0)/(q.T1X-q.T0X), (q.Y1-q.Y0)/(q.T1Y-q.T0Y)
		left, top := q.X0-q.T0X*w, q.Y0-q.T0Y*h
		dst = Corners{{left, top}, {left + w, top}, {left, top + h}, {left + w, top + h}}
	}
	o := Overlay{Material: outlineMaterial, Red: [4]float32{1, 1, 1, 1}}
	edges := [4]edge{newEdge(dst[0], dst[2]), newEdge(dst[1], dst[3]), newEdge(dst[0], dst[1]), newEdge(dst[2], dst[3])}
	for k, p := range dst {
		for e := range edges {
			o.Custom[k][e] = edges[e].outline(p)
		}
	}
	f.OverlayOn(m, &o)
}

// edge is the line through a and b, its normal worked out once for the corners measured from it.
type edge struct {
	a        [2]float32
	nx, ny   float32 // of length 1
	straight bool    // false where a and b are one point
}

func newEdge(a, b [2]float32) edge {
	ex, ey := b[0]-a[0], b[1]-a[1]
	n := float32(math.Hypot(float64(ex), float64(ey)))
	if n == 0 {
		return edge{a: a}
	}
	return edge{a: a, nx: ey / n, ny: -ex / n, straight: true}
}

// outline is as the package's outline: minus 1 minus p's distance to the edge's line, in pixels.
func (e edge) outline(p [2]float32) float32 {
	if !e.straight {
		return 0
	}
	d := (p[0]-e.a[0])*e.nx + (p[1]-e.a[1])*e.ny
	if d < 0 {
		d = -d
	}
	return -1 - d
}
