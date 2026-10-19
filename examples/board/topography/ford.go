package main

import (
	"math"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/light"
	"github.com/kjkrol/gram/plugins/minimap"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

type fordRow struct {
	At  cell.ID `yaml:"at"`
	Way [2]int  `yaml:"way"`
}

// ford is stepping stones across a river: a floor waded at a cost, lying the way the map says.
type ford struct{}

var _ board.Figure = ford{}

func (ford) Define(b *board.Plugin) {
	fords := board.NewEntities(b, board.Shape{Size: CellSize, Height: scale.Units(2)},
		func(r fordRow) cell.ID { return r.At })
	fords.Define(FordKind,
		comp.Load(func(r fordRow) world.Velocity {
			return world.Velocity{Dir: geom.NewVec(float64(r.Way[0]), float64(r.Way[1]))}
		}),
		comp.Const(cell.Floor{Allows: cell.Land, Cost: 6.25}),
		comp.Const(light.Shadow{Cast: light.NoShadow}))
}

func (ford) Look(a *world.Atlas, w *world.Plugin) {
	a.Add(kind.Named[fordRow](w.Kinds(), FordKind), CellSize, stones(0)).Facing(8, stones).Lying().
		Styled(minimap.Mark{Color: stoneColor, Size: 3})
}

func stones(angle float64) render.SpriteDrawer {
	return func(dst *render.Canvas, size int) {
		x0, y0, x1, y1 := across(angle, size)
		for i := range 4 {
			t := (float32(i) + 0.5) / 4
			dst.FillCircle(x0+(x1-x0)*t, y0+(y1-y0)*t, float32(size)*0.09, stoneColor)
		}
	}
}

// the ends of a line through the middle of a sprite at angle, degrees, 0 east, against the clock
func across(angle float64, size int) (x0, y0, x1, y1 float32) {
	sin, cos := math.Sincos(angle * math.Pi / 180)
	half := float64(size) / 2
	reach := half / max(math.Abs(cos), math.Abs(sin))
	dx, dy := float32(cos*reach), float32(-sin*reach)
	m := float32(half)
	return m - dx, m - dy, m + dx, m + dy
}
