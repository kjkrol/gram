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
}

// Miniature is the board drawn flat from above, perUnit pixels a world unit, for a minimap to lay
// under its marks; nil before the board's atlas is closed. It is drawn anew as the board changes.
func (w *Write) Miniature() (img *render.Image, perUnit float32) {
	p := w.p
	if p.atlas == nil {
		return nil, 0
	}
	m := &p.mini
	brd := &p.board.read
	if m.img != nil && m.at == brd.Changes()+1 {
		return m.img, m.px
	}
	space := p.worldPlugin.Res.Config.Space
	ww, wh := float32(space.Width), float32(space.Height)
	if m.img == nil {
		m.tiles = look.NewRenderer(brd, p.atlas, brd.Map(), space)
		m.px = min(miniPx, miniMax/ww, miniMax/wh)
		m.img = render.NewImage(int(math.Ceil(float64(ww*m.px))), int(math.Ceil(float64(wh*m.px))))
	}
	m.img.Fill(color.Transparent)
	u := render.UniformsOf(map[string]any{"Pixel": []float32{1 / m.px}})
	// in the sun's light on level ground as it stands, the lamps' left out
	light := render.Light{1, 1, 1}
	if l := p.worldPlugin.Read(0).Lights(); l != nil {
		light = l.Sun.Light(0, 0, 1)
	}
	m.tiles.Whole(m.img, m.px, light, u)
	m.at = brd.Changes() + 1
	return m.img, m.px
}
