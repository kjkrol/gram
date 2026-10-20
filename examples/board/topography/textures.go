package main

import (
	"embed"
	"image"
	"image/color"
	"image/png"
	"log"
	"math"

	"github.com/kjkrol/gram/plugins/topography/painter"
)

// Poly Haven's, CC0: forrest_ground_01 (earth), coast_sand_01 (sand), rocky_terrain_02 (rock),
// forest_leaves_02 (forest), snow_02 (snow), gravel_road (road), each 512 px of a 1k diffuse.
//
//go:embed textures/*.png
var textures embed.FS

// details is the texture over each kind's cells in the look named look: photographs for the
// natural look, tiles painted here for the Ghibli one.
func details(look string) map[string]painter.Detail {
	var paint map[string]painter.Detail
	switch look {
	case NaturalLook:
		paint = map[string]painter.Detail{
			EarthCell:  {Image: texture("earth"), Across: 2},
			SandCell:   {Image: texture("sand"), Across: 2},
			RockCell:   {Image: texture("rock"), Across: 2},
			ForestCell: {Image: texture("forest"), Across: 3},
			RoadCell:   {Image: texture("road"), Across: 1},
		}
		snow := painter.Detail{Image: texture("snow"), Across: 2}
		for _, name := range snowable() {
			paint[snowyCell(name)] = snow
		}
	case GhibliLook:
		paint = map[string]painter.Detail{
			EarthCell:  {Image: paintedEarth(), Across: 2},
			SandCell:   {Image: paintedSand(), Across: 2},
			RockCell:   {Image: paintedRock(), Across: 2},
			ForestCell: {Image: paintedForest(), Across: 3},
			RoadCell:   {Image: paintedRoad(), Across: 1},
		}
		snow := painter.Detail{Image: paintedSnow(), Across: 2}
		for _, name := range snowable() {
			paint[snowyCell(name)] = snow
		}
	default:
		log.Fatalf("no look is named %q: natural or ghibli", look)
	}
	return paint
}

// shading is how the sun lights the relief in the look named look.
func shading(look string) painter.Shading {
	if look == GhibliLook {
		return painter.Shading{
			Bands:      3,
			ShadowTint: color.RGBA{R: 175, G: 190, B: 235, A: 255},
			Warmth:     0.35,
		}
	}
	return painter.Shading{}
}

func texture(name string) image.Image {
	f, err := textures.Open("textures/" + name + ".png")
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		log.Fatalf("textures/%s.png: %v", name, err)
	}
	return img
}

// The painted tiles are paintedPx square and seamless: their noise repeats with the tile.
const paintedPx = 256

// a tile painted by tone, a grey round a half at each point 0..1 across it, with the lights a
// little warmer and the shades a little cooler as gouache has them
func painted(tone func(x, y float64) float64) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, paintedPx, paintedPx))
	for y := range paintedPx {
		for x := range paintedPx {
			v := tone((float64(x)+0.5)/paintedPx, (float64(y)+0.5)/paintedPx)
			v = min(max(v, 0), 1)
			warm := (v - 0.5) * 0.12
			img.SetRGBA(x, y, color.RGBA{
				R: byteOf(v + warm),
				G: byteOf(v),
				B: byteOf(v - warm),
				A: 255,
			})
		}
	}
	return img
}

func byteOf(v float64) uint8 { return uint8(min(max(v, 0), 1)*255 + 0.5) }

func paintedEarth() *image.RGBA {
	return painted(func(x, y float64) float64 {
		return 0.5 + 0.16*(fbm(x, y, 3, 3, 1)-0.5) + 0.05*(fbm(x, y, 12, 2, 2)-0.5)
	})
}

func paintedSand() *image.RGBA {
	return painted(func(x, y float64) float64 {
		ripple := math.Sin(2 * math.Pi * (3*y + 0.4*fbm(x, y, 2, 2, 3)))
		return 0.5 + 0.05*ripple + 0.04*(fbm(x, y, 8, 2, 4)-0.5)
	})
}

// plates round seeds on a 5 by 5 lattice, each its own tone, their cracks dark
func paintedRock() *image.RGBA {
	const n = 5
	return painted(func(x, y float64) float64 {
		px, py := x*n, y*n
		ix, iy := math.Floor(px), math.Floor(py)
		near, next, tone := 9.0, 9.0, 0.5
		for dy := -1.0; dy <= 1; dy++ {
			for dx := -1.0; dx <= 1; dx++ {
				cx, cy := ix+dx, iy+dy
				sx := cx + 0.2 + 0.6*hash(wrap(cx, n), wrap(cy, n), 5)
				sy := cy + 0.2 + 0.6*hash(wrap(cx, n), wrap(cy, n), 6)
				d := math.Hypot(px-sx, py-sy)
				if d < near {
					near, next = d, near
					tone = 0.42 + 0.2*hash(wrap(cx, n), wrap(cy, n), 7)
				} else if d < next {
					next = d
				}
			}
		}
		crack := 1 - smooth(0, 0.07, next-near)
		return tone - 0.22*crack + 0.03*(fbm(x, y, 10, 2, 8)-0.5)
	})
}

// crowns on a jittered 4 by 4 lattice, each lit from the top
func paintedForest() *image.RGBA {
	const n = 4
	return painted(func(x, y float64) float64 {
		px, py := x*n, y*n
		ix, iy := math.Floor(px), math.Floor(py)
		tone := 0.36 + 0.04*(fbm(x, y, 6, 2, 9)-0.5)
		for dy := -1.0; dy <= 1; dy++ {
			for dx := -1.0; dx <= 1; dx++ {
				cx, cy := ix+dx, iy+dy
				sx := cx + 0.25 + 0.5*hash(wrap(cx, n), wrap(cy, n), 10)
				sy := cy + 0.25 + 0.5*hash(wrap(cx, n), wrap(cy, n), 11)
				r := 0.38 + 0.12*hash(wrap(cx, n), wrap(cy, n), 12)
				d := math.Hypot(px-sx, py-sy)
				if d >= r {
					continue
				}
				lit := 0.5 + 0.5*(sy-py)/r
				crown := 0.5 + 0.22*lit*(1-d/r) - 0.08*(d/r)
				tone = max(tone, crown)
			}
		}
		return tone
	})
}

func paintedSnow() *image.RGBA {
	return painted(func(x, y float64) float64 {
		return 0.5 + 0.05*(fbm(x, y, 4, 2, 13)-0.5)
	})
}

// flat, a pebble here and there
func paintedRoad() *image.RGBA {
	const n = 12
	return painted(func(x, y float64) float64 {
		tone := 0.5 + 0.03*(fbm(x, y, 16, 2, 14)-0.5)
		px, py := x*n, y*n
		ix, iy := math.Floor(px), math.Floor(py)
		if hash(wrap(ix, n), wrap(iy, n), 15) < 0.3 {
			sx := ix + 0.3 + 0.4*hash(wrap(ix, n), wrap(iy, n), 16)
			sy := iy + 0.3 + 0.4*hash(wrap(ix, n), wrap(iy, n), 17)
			if math.Hypot(px-sx, py-sy) < 0.18 {
				tone += 0.1 - 0.2*hash(wrap(ix, n), wrap(iy, n), 18)
			}
		}
		return tone
	})
}

// fbm is value noise over the tile repeating every 1/period of it, 0 to 1, octaves octaves deep,
// seeded by seed.
func fbm(x, y float64, period, octaves int, seed float64) float64 {
	sum, weight, all := 0.0, 1.0, 0.0
	for o := range octaves {
		p := period << o
		sum += weight * pnoise(x*float64(p), y*float64(p), p, seed+float64(o))
		all += weight
		weight /= 2
	}
	return sum / all
}

func pnoise(x, y float64, period int, seed float64) float64 {
	ix, iy := math.Floor(x), math.Floor(y)
	fx, fy := smooth(0, 1, x-ix), smooth(0, 1, y-iy)
	at := func(dx, dy float64) float64 {
		return hash(wrap(ix+dx, period), wrap(iy+dy, period), seed)
	}
	top := at(0, 0) + (at(1, 0)-at(0, 0))*fx
	bottom := at(0, 1) + (at(1, 1)-at(0, 1))*fx
	return top + (bottom-top)*fy
}

func wrap(v float64, period int) float64 {
	p := float64(period)
	return v - p*math.Floor(v/p)
}

func hash(x, y, seed float64) float64 {
	v := math.Sin(x*127.1+y*311.7+seed*74.7) * 43758.5453
	return v - math.Floor(v)
}

func smooth(a, b, v float64) float64 {
	t := min(max((v-a)/(b-a), 0), 1)
	return t * t * (3 - 2*t)
}
