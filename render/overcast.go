package render

// Overcast is the colour of a sky of colour sky under clouds covering this much of it, 0 to 1:
// greyer and darker the more it is covered. The composer's shader sees the sky water reflects the
// same way.
func Overcast(sky Light, clouds float32) Light {
	grey := (0.3*sky[0] + 0.5*sky[1] + 0.2*sky[2]) * overcastGrey
	k := min(max(clouds, 0), 1) * overcastSky
	return Light{sky[0] + (grey-sky[0])*k, sky[1] + (grey-sky[1])*k, sky[2] + (grey-sky[2])*k}
}

// How clouds grey the sky: how bright its grey is to the clear sky's brightness, and how far a sky
// all covered goes to that grey.
const (
	overcastGrey = 0.85
	overcastSky  = 0.8
)
