package render

import "github.com/kjkrol/gram/render/gpu"

// Paint draws f's pieces onto dst in the order they came, through the composer's shader in no
// light and no weather: for what is painted once into an image of its own, its corners in dst's
// pixels. A sub-image of dst clips it.
func Paint(dst *Image, f *Frame) {
	var (
		verts   []Vertex
		indices []uint16
		sheet   AtlasSource
	)
	uniforms := composer.pack(nil)
	flush := func() {
		if len(verts) > 0 && sheet != nil {
			gpu.Triangles(&gpu.Draw{Target: dst.gpu(), Program: composer.program(), Images: [4]gpu.Image{sheet.Atlas().gpu()}, Uniforms: uniforms, Blend: gpu.SourceOver}, verts, indices)
		}
		verts, indices = verts[:0], indices[:0]
	}
	for _, it := range f.items {
		if it.atlas == nil || it.shape == fan {
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
			indices = appendQuad(indices, base, it.shape)
		}
	}
	flush()
}
