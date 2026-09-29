package render

import (
	"image/color"
	"math"
)

// Solid returns a SpriteDrawer filling the whole sprite with c.
func Solid(c color.RGBA) SpriteDrawer {
	return func(dst *Canvas, size int) {
		dst.FillRect(0, 0, float32(size), float32(size), c)
	}
}

// Border returns a SpriteDrawer outlining the sprite with c, transparent inside.
func Border(c color.RGBA) SpriteDrawer {
	return func(dst *Canvas, size int) {
		for y := 0; y < size; y++ {
			for x := 0; x < size; x++ {
				if x <= 1 || x >= size-2 || y <= 1 || y >= size-2 {
					dst.Set(x, y, c)
				}
			}
		}
	}
}

// Diamond returns a SpriteDrawer filling a diamond shape with c, transparent outside.
func Diamond(c color.RGBA) SpriteDrawer {
	return func(dst *Canvas, size int) {
		cx, cy := size/2, size/2
		for y := 0; y < size; y++ {
			for x := 0; x < size; x++ {
				if abs(x-cx)+abs(y-cy) <= cx-1 {
					dst.Set(x, y, c)
				}
			}
		}
	}
}

// Cross returns a SpriteDrawer filling a cross shape with c, transparent outside.
func Cross(c color.RGBA) SpriteDrawer {
	return func(dst *Canvas, size int) {
		for y := 0; y < size; y++ {
			for x := 0; x < size; x++ {
				inH := y >= size/4 && y < size*3/4
				inV := x >= size/4 && x < size*3/4
				if inH || inV {
					dst.Set(x, y, c)
				}
			}
		}
	}
}

// Hexagon returns a SpriteDrawer filling a pointy-top hexagon with c, transparent at the corners;
// drawn over a cell of width √3·r and height 2·r it is regular.
func Hexagon(c color.RGBA) SpriteDrawer {
	return func(dst *Canvas, size int) {
		half := float64(size) / 2
		for y := 0; y < size; y++ {
			for x := 0; x < size; x++ {
				du := math.Abs(float64(x)+0.5-half) / half
				dv := math.Abs(float64(y)+0.5-half) / half
				if du <= 1 && dv <= 1-du/2 {
					dst.Set(x, y, c)
				}
			}
		}
	}
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
