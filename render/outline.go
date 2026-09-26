package render

import _ "embed"

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
	for k, p := range dst {
		o.Custom[k] = [4]float32{outline(p, dst[0], dst[2]), outline(p, dst[1], dst[3]), outline(p, dst[0], dst[1]), outline(p, dst[2], dst[3])}
	}
	f.OverlayOn(m, &o)
}
