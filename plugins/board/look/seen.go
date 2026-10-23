package look

import (
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugins/world/view"
	"github.com/kjkrol/gram/render"
)

// seenSectors is a frame's sectors as the shaders read them: a pixel a sector, red its
// view.Level, and where they lie
type seenSectors struct {
	img        *render.Image
	pix, drawn []byte
	cols, rows int
	side       [2]float32
	fogging    [4]float32
	on         bool
}

// see takes the levels v sees its sectors at; nothing without a View of sectors
func (s *seenSectors) see(v *view.View) {
	s.on = v != nil && v.Grid() != nil && v.Grid().Len() > 0
	if !s.on {
		return
	}
	g := v.Grid()
	s.cols = max(g.Cols(), 1)
	s.rows = (g.Len() + s.cols - 1) / s.cols
	box := g.Box(0)
	s.side = [2]float32{float32(box.BottomRight.X - box.TopLeft.X),
		float32(box.BottomRight.Y - box.TopLeft.Y)}
	s.fogging = v.Fogging()
	n := 4 * s.cols * s.rows
	if len(s.pix) != n {
		s.pix, s.drawn = make([]byte, n), nil
		s.img = render.NewImage(s.cols, s.rows)
	}
	clear(s.pix)
	v.Sectors(func(i int, _ geom.AABB, l view.Level) { s.pix[4*i], s.pix[4*i+3] = byte(l), 255 })
	if string(s.pix) != string(s.drawn) {
		s.img.WritePixels(s.pix)
		s.drawn = append(s.drawn[:0], s.pix...)
	}
}

// uniforms are the sectors' as the shaders take them under the prefix: whether there are any,
// how many across and down and how large, and the fog before the sight's end
func (s *seenSectors) uniforms(set func(name string, v ...float32), prefix string) *render.Image {
	set(prefix+"Seen", flag(s.on))
	set(prefix+"Sectors", float32(s.cols), float32(s.rows), s.side[0], s.side[1])
	if prefix == "Tile" {
		set("TileFogging", s.fogging[:]...)
	}
	if !s.on {
		return nil
	}
	return s.img
}
