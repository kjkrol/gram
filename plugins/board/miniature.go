package board

import (
	"image/color"
	"math"

	"github.com/kjkrol/gram/plugins/board/look"
	"github.com/kjkrol/gram/plugins/minimap"
	"github.com/kjkrol/gram/render"
)

var _ minimap.Ground = (*Write)(nil)

// the board in miniature: its pixels a world unit, at most miniMax a side
const (
	miniPx  = 1
	miniMax = 2048
)

type miniature struct {
	tiles *look.Renderer
	img   *render.Image
	px    float32
	at    uint64
	light render.Light
}

// Miniature is the board drawn flat from above, perUnit pixels a world unit, for a minimap to lay
// under its marks, as the frame drawn holds it; nil before the board's atlas is closed. It is
// drawn anew as the board changes and as the sun's light on it does.
func (w *Write) Miniature() (img *render.Image, perUnit float32) {
	p := w.p
	if p.atlas == nil {
		return nil, 0
	}
	m := &p.mini
	brd := p.board.drawnBoard()
	// in the sun's light on level ground as it stands, the lamps' left out
	light := render.Light{1, 1, 1}
	if l := p.worldPlugin.Read(0).Lights(); l != nil {
		light = l.Sun.Light(0, 0, 1)
	}
	if m.img != nil && m.at == brd.Changes()+1 && seenAlike(m.light, light) {
		return m.img, m.px
	}
	space := p.worldPlugin.Res.Config.Space
	ww, wh := float32(space.Width), float32(space.Height)
	if m.img == nil {
		m.tiles = look.NewRenderer(brd, p.atlas, drawnMap{brd}, space)
		m.px = min(miniPx, miniMax/ww, miniMax/wh)
		m.img = render.NewImage(int(math.Ceil(float64(ww*m.px))), int(math.Ceil(float64(wh*m.px))))
	}
	m.img.Fill(color.Transparent)
	u := render.UniformsOf(map[string]any{"Pixel": []float32{1 / m.px}})
	m.tiles.Whole(m.img, m.px, light, u)
	m.at, m.light = brd.Changes()+1, light
	return m.img, m.px
}

// seenAlike reports whether lights a and b differ by less than a shade of a channel
func seenAlike(a, b render.Light) bool {
	for k := range a {
		if math.Abs(float64(a[k]-b[k])) >= 1.0/255 {
			return false
		}
	}
	return true
}
