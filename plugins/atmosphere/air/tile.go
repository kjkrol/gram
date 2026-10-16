package air

import (
	"image"

	"github.com/kjkrol/gram/render"
)

var tileShader = render.NewShaderWith("cloud tile", render.Files(shaders, "shaders/cloud_tile.wgsl"), nil)

// the tile's size in pixels: its finest level and the coarser beside it, each with a pixel round
// it of what lies past its far edge, so a shader reads across the wrap with the GPU's filter
const (
	tileTexels = 1024
	tileLevels = 6
	tileWidth  = tileTexels*3/2 + 4
	tileHeight = tileTexels + 2
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
		at := image.Rect(0, 0, size+2, size+2)
		if level > 0 {
			at = at.Add(image.Pt(tileTexels+2, tileTexels-2*size+2*(level-1)))
		}
		at = at.Add(corner)
		pixel := float32(CloudTile) / float32(size)
		samples := float32(min(1<<level, 4))
		side := float32(size + 2)
		for i, c := range [4][2]float32{{0, 0}, {1, 0}, {0, 1}, {1, 1}} {
			// a pixel more each way, the noise there what lies past the far edge
			from := (c[0]*side - 1) * pixel
			down := (c[1]*side - 1) * pixel
			verts[i] = render.Vertex{
				DstX:    float32(at.Min.X) + c[0]*side,
				DstY:    float32(at.Min.Y) + c[1]*side,
				ColorR:  1,
				ColorG:  1,
				ColorB:  1,
				ColorA:  1,
				Custom0: from,
				Custom1: down,
				Custom2: pixel,
				Custom3: samples,
			}
		}
		dst.SubImage(at).DrawTrianglesShader(verts, tileQuad, tileShader, &render.DrawTrianglesShaderOptions{})
	}
}

var tileQuad = []uint16{0, 1, 2, 1, 2, 3}
