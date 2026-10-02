package air

import (
	"image"

	"github.com/kjkrol/gram/render"
)

// tileShader bakes a tile of the clouds' noise (shaders/cloud_tile.wgsl).
var tileShader = render.NewShaderWith("cloud tile", render.Files(shaders, "shaders/cloud_tile.wgsl"), nil)

// The tile of the clouds' noise BakeTile bakes, TileWidth by TileHeight pixels: its first level
// tileTexels across over CloudTile world units, tileLevels levels each half as wide, the first on
// the left, the rest stacked down its right; shaders/cloud_noise.wgsl has the same.
const (
	tileTexels = 1024
	tileLevels = 6
	TileWidth  = tileTexels * 3 / 2
	TileHeight = tileTexels
)

// BakeTile bakes a tile of the clouds' noise into dst, TileWidth by TileHeight pixels from its
// corner, every level of it: the shreds in red, the heaps in green, each pixel the noise averaged
// over the world square it stands for. A shader looks the clouds up in it (cloudTileSpot,
// cloudTileLevel) rather than work their noise out at every pixel, seeing them from afar as
// evened out as a pixel sees them.
func BakeTile(dst *render.Image) {
	corner := dst.Bounds().Min
	verts := make([]render.Vertex, 4)
	for level := range tileLevels {
		size := tileTexels >> level
		at := image.Rect(0, 0, size, size)
		if level > 0 {
			at = at.Add(image.Pt(tileTexels, tileTexels-2*tileTexels>>level))
		}
		at = at.Add(corner)
		pixel := float32(CloudTile) / float32(size)
		samples := float32(min(1<<level, 4))
		for i, c := range [4][2]float32{{0, 0}, {1, 0}, {0, 1}, {1, 1}} {
			verts[i] = render.Vertex{DstX: float32(at.Min.X) + c[0]*float32(size), DstY: float32(at.Min.Y) + c[1]*float32(size),
				ColorR: 1, ColorG: 1, ColorB: 1, ColorA: 1,
				Custom0: c[0] * CloudTile, Custom1: c[1] * CloudTile, Custom2: pixel, Custom3: samples}
		}
		dst.SubImage(at).DrawTrianglesShader(verts, tileQuad, tileShader, &render.DrawTrianglesShaderOptions{})
	}
}

// tileQuad is a rectangle's two triangles over its four corners.
var tileQuad = []uint16{0, 1, 2, 1, 2, 3}
