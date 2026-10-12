package ui

import (
	"math"

	"github.com/kjkrol/aabbworld/geom"
)

// Mask is a shape an element is cut to inside its box: drawn and hit only within it.
type Mask interface {
	// points are the shape's corners in box, round its edge, a convex polygon.
	points(box geom.AABB) [][2]float32
	contains(box geom.AABB, p geom.Vec) bool
}

// Circle is the ellipse filling the box: a circle in a square one.
var Circle Mask = ellipse{}

type ellipse struct{}

const circleSides = 64

func (ellipse) points(box geom.AABB) [][2]float32 {
	w, h := size(box)
	cx, cy := box.TopLeft.X+w/2, box.TopLeft.Y+h/2
	pts := make([][2]float32, circleSides)
	for k := range pts {
		s, c := math.Sincos(2 * math.Pi * float64(k) / circleSides)
		pts[k] = [2]float32{float32(cx + c*w/2), float32(cy + s*h/2)}
	}
	return pts
}

func (ellipse) contains(box geom.AABB, p geom.Vec) bool {
	w, h := size(box)
	if w <= 0 || h <= 0 {
		return false
	}
	dx := (p.X-box.TopLeft.X)/w*2 - 1
	dy := (p.Y-box.TopLeft.Y)/h*2 - 1
	return dx*dx+dy*dy <= 1
}

// Polygon is the convex polygon of corners given in the box's own terms: (0, 0) its top-left,
// (1, 1) its bottom-right — a hexagon, a diamond.
func Polygon(corners ...geom.Vec) Mask { return polygon(corners) }

type polygon []geom.Vec

func (pg polygon) at(box geom.AABB) []geom.Vec {
	w, h := size(box)
	out := make([]geom.Vec, len(pg))
	for k, c := range pg {
		out[k] = geom.NewVec(box.TopLeft.X+c.X*w, box.TopLeft.Y+c.Y*h)
	}
	return out
}

func (pg polygon) points(box geom.AABB) [][2]float32 {
	at := pg.at(box)
	pts := make([][2]float32, len(at))
	for k, c := range at {
		pts[k] = [2]float32{float32(c.X), float32(c.Y)}
	}
	return pts
}

// contains holds where p lies on one side of every edge, whichever way round the corners go.
func (pg polygon) contains(box geom.AABB, p geom.Vec) bool {
	at := pg.at(box)
	if len(at) < 3 {
		return false
	}
	sign := 0.0
	for k, a := range at {
		b := at[(k+1)%len(at)]
		cross := (b.X-a.X)*(p.Y-a.Y) - (b.Y-a.Y)*(p.X-a.X)
		if cross == 0 {
			continue
		}
		if sign == 0 {
			sign = cross
		} else if (cross > 0) != (sign > 0) {
			return false
		}
	}
	return true
}

// outline is the box's shape: the mask's, else its four corners.
func outline(box geom.AABB, m Mask) [][2]float32 {
	if m != nil {
		return m.points(box)
	}
	x0, y0 := float32(box.TopLeft.X), float32(box.TopLeft.Y)
	x1, y1 := float32(box.BottomRight.X), float32(box.BottomRight.Y)
	return [][2]float32{{x0, y0}, {x1, y0}, {x1, y1}, {x0, y1}}
}
