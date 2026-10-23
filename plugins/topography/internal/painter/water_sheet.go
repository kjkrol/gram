package painter

import "image"

// waterLayer is where the water's layer q lies in the sheet's water: a quadrant of it
func (s *groundSheet) waterLayer(q int) image.Rectangle {
	w, h := s.water.Bounds().Dx()/2, s.water.Bounds().Dy()/2
	x, y := (q%2)*w, (q/2)*h
	return image.Rect(x, y, x+w, y+h)
}
