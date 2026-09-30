package trigger

import "github.com/kjkrol/gram/plugins/world"

// centre is the middle of an entity's box.
func centre(p *world.Position) (float64, float64) {
	box := p.AABB.AABB
	return (float64(box.TopLeft.X) + float64(box.BottomRight.X)) / 2,
		(float64(box.TopLeft.Y) + float64(box.BottomRight.Y)) / 2
}
