package board

import (
	"image/color"
	"math"

	"github.com/kjkrol/gram/plugins/board/look"
	"github.com/kjkrol/gram/plugins/minimap"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

var _ minimap.Ground = (*Write)(nil)

// the board in miniature: its pixels a world unit, at most miniMax a side
const (
	miniPx  = 1
	miniMax = 2048
)

type miniature struct {
	still *render.Still
	tiles *look.Renderer
	img   *render.Image
	px    float32
	at    uint64
}

// flat is the board's map seen from above with the flat look
type flat struct{ look.Map }

func (flat) Look() look.Look { return look.FlatLook() }

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
		m.still = render.NewStill()
		m.tiles = look.NewRenderer(brd, p.atlas, flat{brd.Map()}, world.SpaceCfg{})
		m.px = min(miniPx, miniMax/ww, miniMax/wh)
		m.img = render.NewImage(int(math.Ceil(float64(ww*m.px))), int(math.Ceil(float64(wh*m.px))))
	}
	m.still.Compose(ww, wh, m.tiles.Compose)
	m.img.Fill(color.Transparent)
	u := render.UniformsOf(map[string]any{"Pixel": []float32{1 / m.px}})
	// in the sun's light on level ground as it stands, the lamps' left out
	light := render.Light{1, 1, 1}
	if l := p.worldPlugin.Read(0).Lighting(); l != nil {
		lit := l.At(0, 0, 0)
		for k := range light {
			light[k] = lit.Light[k] - lit.Lamps[k]
		}
	}
	m.still.Draw(render.Target{Screen: m.img}, nil, u, m.px, 0, 0, light)
	m.at = brd.Changes() + 1
	return m.img, m.px
}
