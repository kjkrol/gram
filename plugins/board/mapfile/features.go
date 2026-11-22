package mapfile

import (
	"encoding/json"
	"fmt"
	"io/fs"

	"github.com/kjkrol/aabbworld/geom"
)

// Shape is what a feature's places make.
type Shape uint8

// The shapes: a place, a line through places, an area within an outline.
const (
	Point Shape = iota
	Line
	Area
)

// Feature is a thing lying on the ground: what it is by its tags, named as OpenStreetMap's,
// and where, in the world's places.
type Feature struct {
	// ID is the feature's own name in its file, empty where it has none.
	ID    string
	Tags  map[string]string
	Shape Shape
	// Path is the place, the line's points or the area's outline, Holes the outlines cut out
	// of an area.
	Path  []geom.Vec
	Holes [][]geom.Vec
}

type collection struct {
	Features []struct {
		ID         any            `json:"id"`
		Properties map[string]any `json:"properties"`
		Geometry   struct {
			Type        string          `json:"type"`
			Coordinates json.RawMessage `json:"coordinates"`
		} `json:"geometry"`
	} `json:"features"`
}

func readFeatures(fsys fs.FS, file string, height float32) ([]Feature, error) {
	raw, err := fs.ReadFile(fsys, file)
	if err != nil {
		return nil, err
	}
	var c collection
	if err := json.Unmarshal(raw, &c); err != nil {
		return nil, fmt.Errorf("board: %s: %w", file, err)
	}
	// north is up in the file, down the world
	turn := func(at []float64) (geom.Vec, error) {
		if len(at) < 2 {
			return geom.Vec{}, fmt.Errorf("a point of one number")
		}
		return geom.NewVec(float32(at[0]), height-float32(at[1])), nil
	}
	path := func(all [][]float64) ([]geom.Vec, error) {
		var out []geom.Vec
		for _, at := range all {
			v, err := turn(at)
			if err != nil {
				return nil, err
			}
			if len(out) == 0 || nodeOf(v) != nodeOf(out[len(out)-1]) {
				out = append(out, v)
			}
		}
		return out, nil
	}
	var out []Feature
	for i, f := range c.Features {
		ft := Feature{Tags: map[string]string{}}
		if f.ID != nil {
			ft.ID = fmt.Sprint(f.ID)
		}
		for k, v := range f.Properties {
			if v != nil {
				ft.Tags[k] = fmt.Sprint(v)
			}
		}
		bad := func(err error) error {
			return fmt.Errorf("board: %s: feature %d: %w", file, i, err)
		}
		switch f.Geometry.Type {
		case "Point":
			var at []float64
			if err := json.Unmarshal(f.Geometry.Coordinates, &at); err != nil {
				return nil, bad(err)
			}
			v, err := turn(at)
			if err != nil {
				return nil, bad(err)
			}
			ft.Shape, ft.Path = Point, []geom.Vec{v}
		case "LineString":
			var all [][]float64
			if err := json.Unmarshal(f.Geometry.Coordinates, &all); err != nil {
				return nil, bad(err)
			}
			if ft.Path, err = path(all); err != nil {
				return nil, bad(err)
			}
			if len(ft.Path) < 2 {
				continue
			}
			ft.Shape = Line
		case "Polygon":
			var rings [][][]float64
			if err := json.Unmarshal(f.Geometry.Coordinates, &rings); err != nil {
				return nil, bad(err)
			}
			for k, ring := range rings {
				p, err := path(ring)
				if err != nil {
					return nil, bad(err)
				}
				// a ring is written closed: its last point its first
				if n := len(p); n > 1 && nodeOf(p[0]) == nodeOf(p[n-1]) {
					p = p[:n-1]
				}
				if len(p) < 3 {
					if k == 0 {
						break
					}
					continue
				}
				if k == 0 {
					ft.Path = p
				} else {
					ft.Holes = append(ft.Holes, p)
				}
			}
			if len(ft.Path) < 3 {
				continue
			}
			ft.Shape = Area
		default:
			return nil, bad(fmt.Errorf("a geometry of type %q", f.Geometry.Type))
		}
		out = append(out, ft)
	}
	return out, nil
}
