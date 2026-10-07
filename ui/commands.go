package ui

// Show shows the scene's elements called Name.
type Show struct{ Name string }

// Hide hides the scene's elements called Name.
type Hide struct{ Name string }

// Toggle shows the scene's elements called Name where any is hidden, else hides them.
type Toggle struct{ Name string }

// carry carries out cmd when it is one of the scene's own, reporting whether it was.
func (s *Scene) carry(cmd any) bool {
	switch c := cmd.(type) {
	case Show:
		s.Show(c.Name)
	case Hide:
		s.Hide(c.Name)
	case Toggle:
		s.Toggle(c.Name)
	default:
		return false
	}
	return true
}
