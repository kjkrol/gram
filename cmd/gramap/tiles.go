package main

import (
	"fmt"
	"image"
	"image/png"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
)

// the Terrain Tiles on AWS: heights in metres as R·256 + G + B/256 − 32768
const tileURL = "https://s3.amazonaws.com/elevation-tiles-prod/terrarium/%d/%d/%d.png"

const tilePx int = 256

type elevation func(lon, lat float64) (float64, error)

type terrarium struct {
	zoom  int
	cache string
	fetch func(url string) (io.ReadCloser, error)
	tiles map[[2]int]image.Image
}

func newTerrarium(zoom int, cache string) *terrarium {
	return &terrarium{zoom: zoom, cache: cache, fetch: get, tiles: map[[2]int]image.Image{}}
}

func (t *terrarium) at(lon, lat float64) (float64, error) {
	n := float64(tilePx << t.zoom)
	x := (lon + 180) / 360 * n
	phi := lat * math.Pi / 180
	y := (1 - math.Log(math.Tan(phi)+1/math.Cos(phi))/math.Pi) / 2 * n
	x, y = x-0.5, y-0.5
	x0, y0 := math.Floor(x), math.Floor(y)
	u, v := x-x0, y-y0
	var h [4]float64
	for k, d := range [4][2]float64{{0, 0}, {1, 0}, {0, 1}, {1, 1}} {
		px, err := t.pixel(int(x0+d[0]), int(y0+d[1]))
		if err != nil {
			return 0, err
		}
		h[k] = px
	}
	return (h[0]*(1-u)+h[1]*u)*(1-v) + (h[2]*(1-u)+h[3]*u)*v, nil
}

func (t *terrarium) pixel(x, y int) (float64, error) {
	n := tilePx << t.zoom
	x = (x%n + n) % n
	y = min(max(y, 0), n-1)
	img, err := t.tile(x/tilePx, y/tilePx)
	if err != nil {
		return 0, err
	}
	b := img.Bounds()
	r, g, bl, _ := img.At(b.Min.X+x%tilePx, b.Min.Y+y%tilePx).RGBA()
	return float64(r>>8)*256 + float64(g>>8) + float64(bl>>8)/256 - 32768, nil
}

func (t *terrarium) tile(x, y int) (image.Image, error) {
	if img, ok := t.tiles[[2]int{x, y}]; ok {
		return img, nil
	}
	path := filepath.Join(t.cache, fmt.Sprint(t.zoom), fmt.Sprint(x), fmt.Sprintf("%d.png", y))
	if _, err := os.Stat(path); err != nil {
		if err := t.download(fmt.Sprintf(tileURL, t.zoom, x, y), path); err != nil {
			return nil, err
		}
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("tile %d/%d/%d: %w", t.zoom, x, y, err)
	}
	t.tiles[[2]int{x, y}] = img
	return img, nil
}

func (t *terrarium) download(url, path string) error {
	body, err := t.fetch(url)
	if err != nil {
		return err
	}
	defer body.Close()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".part"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, body); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func get(url string) (io.ReadCloser, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("%s: %s", url, resp.Status)
	}
	return resp.Body, nil
}

// the coarsest zoom whose pixels at lat are no wider than half a cell
func zoomFor(cell, lat float64) int {
	equator := 2 * math.Pi * earthRadius / float64(tilePx) // metres a pixel at zoom 0
	z := math.Ceil(math.Log2(equator * math.Cos(lat*math.Pi/180) / (cell / 2)))
	return int(min(max(z, 0), 14))
}

const earthRadius = 6371000.0

// x metres east and y metres south of centre, in a projection keeping distances near it
func lonLatOf(centre [2]float64, x, y float64) (lon, lat float64) {
	lat0 := centre[1] * math.Pi / 180
	lon = centre[0] + x/(earthRadius*math.Cos(lat0))*180/math.Pi
	lat = centre[1] - y/earthRadius*180/math.Pi
	return lon, lat
}

func metresOf(centre [2]float64, lon, lat float64) (x, y float64) {
	lat0 := centre[1] * math.Pi / 180
	x = (lon - centre[0]) * math.Pi / 180 * earthRadius * math.Cos(lat0)
	y = (centre[1] - lat) * math.Pi / 180 * earthRadius
	return x, y
}
