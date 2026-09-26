package render

import "github.com/hajimehoshi/ebiten/v2"

// Paint draws f's pieces onto dst in the order they came, through the composer's shader in no
// light and no weather: for what is painted once into an image of its own, its corners in dst's
// pixels. A sub-image of dst clips it.
func Paint(dst *ebiten.Image, f *Frame) {
	var (
		verts   []ebiten.Vertex
		indices []uint16
		sheet   AtlasSource
	)
	opts := &ebiten.DrawTrianglesShaderOptions{}
	flush := func() {
		if len(verts) > 0 && sheet != nil {
			opts.Images[0] = sheet.Atlas()
			dst.DrawTrianglesShader(verts, indices, shader(), opts)
		}
		verts, indices = verts[:0], indices[:0]
	}
	for _, it := range f.items {
		if it.atlas == nil || it.shape != quad {
			continue // only sprites are painted
		}
		if it.atlas != sheet {
			flush()
			sheet = it.atlas
		}
		for k := it.first; k < it.first+it.count; k += 4 {
			if len(verts)+4 > chunkVertices {
				flush()
			}
			base := uint16(len(verts))
			verts = append(verts, f.verts[k:k+4]...)
			indices = append(indices, base, base+1, base+2, base+1, base+2, base+3)
		}
	}
	flush()
}
