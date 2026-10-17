package board

import (
	"image/color"

	"github.com/kjkrol/gram/plugin/section"
	"github.com/kjkrol/gram/plugins/board/grid"
)

// Surface is how the board looks beyond its tiles: its Look sets on the board's atlas the line
// of the grid and the width of the cells held, the defaults standing in the rest.
type Surface interface {
	Look(a *Atlas, b *Plugin)
}

// Surfaces notes the board's surfaces, each Look in the order noted after the tiles', a later
// setting taking the place of an earlier; none draws the defaults. Called in the Stage's Figures
// link.
func (p *Plugin) Surfaces(surfaces ...Surface) {
	if err := p.worldPlugin.InSection("surfaces noted", section.Plugins); err != nil {
		panic("board: " + err.Error())
	}
	p.surfaces = append(p.surfaces, surfaces...)
}

// Line is a line of the board's surface: its colour and its width in screen pixels, 0 for none.
type Line struct {
	Color color.RGBA
	Width float32
}

// DefaultGridLine is the line the grid g is drawn in when no surface says otherwise.
func DefaultGridLine(g grid.Grid) Line {
	if shape, ok := grid.ShapeOf(g); ok && !shape.Hex {
		return Line{Color: color.RGBA{A: 115}, Width: 1}
	}
	return Line{Color: color.RGBA{R: 20, G: 20, B: 20, A: 184}, Width: 1}
}

// DefaultHoldWidth is how many pixels wide a cell held is outlined when no surface says
// otherwise.
const DefaultHoldWidth = 2
