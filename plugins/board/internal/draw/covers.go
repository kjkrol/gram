package draw

import (
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/entity/tag"
	"github.com/kjkrol/gram/plugins/board/look"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/rule/effect"
)

// Cover is what lies on the cells under an effect: the sprite laid over each while the effect's
// marker is on it.
type Cover struct {
	Mark   tag.Tag[effect.States]
	Sprite render.SpriteID
}

// coverSoft is how far either side of its line a cover fades in, as a share of the weight.
const coverSoft = 0.2

// around are the grid's directions (Grid.Toward) by where the neighbour lies, down then across,
// -1 the cell itself.
var around = [3][3]int{{4, 0, 5}, {2, -1, 3}, {6, 1, 7}}

// Cover has sprite laid over the cells under the effect marked mark.
func (d *Bands) Cover(mark tag.Tag[effect.States], sprite render.SpriteID) {
	d.covers = append(d.covers, Cover{Mark: mark, Sprite: sprite})
}

// covered lays every cover over t along the line the cells under its effect draw, not along the
// cells' edges: on a square grid; on any other over the whole of a cell under it.
func (d *Bands) covered(f *render.Frame, cam camera.Camera, t *look.Tile, depth float32) {
	if len(d.covers) == 0 {
		return
	}
	var near [3][3]tag.Tags[effect.States]
	mine := d.board.States(t.ID)
	square := d.square()
	for j := range 3 {
		for i := range 3 {
			near[j][i] = mine
			if dir := around[j][i]; dir >= 0 && square {
				if n, ok := d.board.Toward(t.ID, dir); ok {
					near[j][i] = d.board.States(n)
				}
			}
		}
	}
	for _, c := range d.covers {
		var in [3][3]bool
		any := false
		for j := range 3 {
			for i := range 3 {
				in[j][i] = near[j][i].Has(c.Mark)
				any = any || in[j][i]
			}
		}
		if !any {
			continue
		}
		for _, q := range Quarters(Weigh(in), coverSoft) {
			var dst render.Corners
			w, h := t.X1-t.X0, t.Y1-t.Y0
			for k, p := range [4][2]float32{{q.Part[0], q.Part[1]}, {q.Part[2], q.Part[1]}, {q.Part[0], q.Part[3]}, {q.Part[2], q.Part[3]}} {
				dst[k][0], dst[k][1] = cam.Project(t.X0+p[0]*w, t.Y0+p[1]*h, 0)
			}
			f.SpriteBlendPart(render.Ground, depth, t.Atlas, c.Sprite, q.Part, dst, t.Light(), q.Weight, coverSoft)
		}
	}
}

// square reports whether the board's grid is the square one, where covers blend.
func (d *Bands) square() bool {
	s, ok := d.board.Shape()
	return ok && !s.Hex
}

// Weigh is the share of the cells round a tile that are in, at each of its points across and
// down: 0 its left or top edge, 1 its middle, 2 its right or bottom edge — the tile's own at its
// middle. The share at a corner or a side is the same from every cell meeting there, so a line
// runs on from tile to tile.
func Weigh(in [3][3]bool) [3][3]float32 {
	span := [3][2]int{{0, 1}, {1, 1}, {1, 2}}
	var weight [3][3]float32
	for j := range 3 {
		for i := range 3 {
			n, all := 0, 0
			for cy := span[j][0]; cy <= span[j][1]; cy++ {
				for cx := span[i][0]; cx <= span[i][1]; cx++ {
					all++
					if in[cy][cx] {
						n++
					}
				}
			}
			weight[j][i] = float32(n) / float32(all)
		}
	}
	return weight
}

// Quarter is a quarter of a tile a cover is drawn over: Part its left, top, right and bottom as
// shares of the tile, Weight the cover's at its corners — top-left, top-right, bottom-left,
// bottom-right.
type Quarter struct {
	Part   [4]float32
	Weight [4]float32
}

// Quarters are the quarters of a tile weighed so on which the cover shows anywhere, fading in
// over soft: none where no weight comes within soft of a half.
func Quarters(weight [3][3]float32, soft float32) []Quarter {
	var out []Quarter
	for qj := range 2 {
		for qi := range 2 {
			w := [4]float32{weight[qj][qi], weight[qj][qi+1], weight[qj+1][qi], weight[qj+1][qi+1]}
			if max(w[0], w[1], w[2], w[3]) <= 0.5-soft {
				continue
			}
			x, y := float32(qi)/2, float32(qj)/2
			out = append(out, Quarter{Part: [4]float32{x, y, x + 0.5, y + 0.5}, Weight: w})
		}
	}
	return out
}
