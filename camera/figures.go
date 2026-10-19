package camera

// Figured is a camera whose figures, all that lies over the ground, whoever shows it draws at
// every shown pixel while its ground is drawn at its Resolution.
type Figured interface {
	SetSharpFigures(on bool)
	SharpFigures() bool
}

// SharpFiguresOf reports whether cam's figures are drawn at every pixel; false for a camera
// without the choice.
func SharpFiguresOf(cam Camera) bool {
	if f, ok := cam.(Figured); ok {
		return f.SharpFigures()
	}
	return false
}
