package landscape

import (
	_ "embed"

	"github.com/kjkrol/gram/render"
)

//go:embed overcast.kage
var overcastKage []byte

// cloudShadow is the material of the clouds' shadows over the ground (overcast.kage).
var cloudShadow = render.RegisterMaterials(overcastKage, "CloudShadow")[0]

// Overcast lays over the last sprite added to f, whose corners lie at w in the world, the shadows
// of the clouds of the frame's weather drifting over it — worked out per pixel, as faint as the
// sprite and fading or blending as it does; under a clear sky nothing.
func Overcast(f *render.Frame, w render.World) {
	if f.Clouds() <= 0 {
		return
	}
	f.Overlay(&render.Overlay{Material: cloudShadow, World: w, Under: true})
}

// OvercastOn is Overcast over the sprite m drawn earlier, and over all drawn on it since.
func OvercastOn(f *render.Frame, m render.Mark, w render.World) {
	if f.Clouds() <= 0 {
		return
	}
	f.OverlayOn(m, &render.Overlay{Material: cloudShadow, World: w, Under: true})
}
