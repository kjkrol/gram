package air

import (
	"image"

	"github.com/kjkrol/gram/render"
)

var tileShader = render.NewShaderWith("cloud tile", render.Files(shaders, "shaders/cloud_tile.wgsl"), nil)

// the tile's size in pixels: its finest level and the coarser beside it
const (
	tileTexels = 1024
	tileLevels = 6
	tileWidth  = tileTexels * 3 / 2
	tileHeight = tileTexels
)

var tile *render.Image

// Tile is a tile of the clouds' noise for the shaders to look the clouds up in, the frame's image
// CloudTile, baked once, as a draw first reads it.
func Tile() *render.Image {
	if tile == nil {
		tile = render.NewImage(tileWidth, tileHeight)
		tile.Defer(bakeTile)
	}
	return tile
}

func bakeTile(dst *render.Image) {
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

var tileQuad = []uint16{0, 1, 2, 1, 2, 3}
