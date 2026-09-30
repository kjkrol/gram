package celestial

import (
	"bytes"
	_ "embed"
	"image"
	"image/draw"
	"image/png"
	"math"

	"github.com/kjkrol/gram/render"
)

// moonInclination is how far the moon's path leans from the sun's.
const moonInclination = 5.14 * math.Pi / 180

// MoonAt is the way towards the moon from p at ofYear and time of day t, moon round from new (0 to
// 1, 0.5 full): along the ecliptic as far ahead of the sun as it is round, off it by up to its
// inclination, turned with the sphere — rising and setting with the stars, drifting east among
// them some 13° a day.
func (p Place) MoonAt(ofYear, t, moon float32) [3]float32 {
	long := 2 * math.Pi * float64(ofYear+moon)
	lat := moonInclination * math.Sin(long)
	se, ce := math.Sincos(AxialTilt)
	x := math.Cos(lat) * math.Cos(long)
	y := ce*math.Cos(lat)*math.Sin(long) - se*math.Sin(lat)
	z := se*math.Cos(lat)*math.Sin(long) + ce*math.Sin(lat)
	return onSky(p.Sphere(SiderealTime(ofYear, t)), [3]float32{float32(x), float32(y), float32(z)})
}

// Phase is how much of the moon's face is lit with the moon moon round from new: 0 new, 1 full.
func Phase(moon float32) float32 {
	return 0.5 * (1 - float32(math.Cos(2*math.Pi*float64(moon))))
}

// moonPNG is the moon's face as it turns to the Earth, north up, grey: the near side of NASA's
// Scientific Visualization Studio's CGI Moon Kit colour map (Lunar Reconnaissance Orbiter LROC
// data), seen square on as a ball, the limb carried on past the disc's edge.
//
//go:embed moon.png
var moonPNG []byte

// MoonFace is the moon's face on the GPU, north up, for a sky to draw the moon's disc with; call it
// once the GPU is there.
func MoonFace() *render.Image {
	src, err := png.Decode(bytes.NewReader(moonPNG))
	if err != nil {
		panic("celestial: the moon's face: " + err.Error())
	}
	rgba := image.NewRGBA(src.Bounds())
	draw.Draw(rgba, rgba.Bounds(), src, src.Bounds().Min, draw.Src)
	img := render.NewImage(rgba.Bounds().Dx(), rgba.Bounds().Dy())
	img.WritePixels(rgba.Pix)
	return img
}
