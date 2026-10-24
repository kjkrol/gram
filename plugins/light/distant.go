package light

import "github.com/kjkrol/gram/render"

// Distant is a light from far off, the same everywhere, the light of a sky or a scene's own
// (Plugin.WithDistant); render's (render.Distant).
type Distant = render.Distant

// Beam is a Distant ready to light many surfaces (render.Beam).
type Beam = render.Beam

// Umbra is where a shadow lies on the ground (render.Umbra).
type Umbra = render.Umbra

func white(l render.Light) render.Light {
	if l == (render.Light{}) {
		return render.Light{1, 1, 1}
	}
	return l
}
