package mosaic

import (
	"fmt"
	"io/fs"
	"math"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugins/board/mapfile"
)

// Load is the ground of the level whose manifest is name in fsys, in one piece or in tiles:
// its heights from images, its roads the lines tagged highway, meeting where they share a point.
func Load(fsys fs.FS, name string) (Map, error) {
	l, err := mapfile.Load(fsys, name)
	if err != nil {
		return Map{}, err
	}
	if err := l.Whole(); err != nil {
		return Map{}, err
	}
	var own struct {
		Apart float32 `yaml:"apart"`
		Rungs int     `yaml:"rungs"`
	}
	if _, err := l.Part("mosaic", &own); err != nil {
		return Map{}, err
	}
	m := Map{Seed: l.Seed, Apart: own.Apart, Rungs: own.Rungs}
	if l.Grounded() {
		m.Ground = func(at geom.Vec) float32 { return l.HeightAt(at.X, at.Y) }
	}
	roads, meetings := l.Network()
	for i, r := range roads {
		s, ok := l.Sections[r.Highway]
		if !ok {
			return Map{}, fmt.Errorf("mosaic: the level has no road for highway=%s", r.Highway)
		}
		section := Section{Lanes: s.Lanes, Lane: s.Lane, Walk: s.Walk, Kerb: s.Kerb, Verge: s.Verge}
		if i > 0 && section != m.Section {
			return Map{}, fmt.Errorf("mosaic: a level's roads are of one section yet")
		}
		m.Section = section
		m.Roads = append(m.Roads, Road{Path: r.Path})
	}
	for _, at := range meetings {
		for k, way := range at.Ways {
			next := at.Ways[(k+1)%len(at.Ways)]
			gap := math.Atan2(float64(next.Y), float64(next.X)) -
				math.Atan2(float64(way.Y), float64(way.X))
			if gap <= 0 {
				gap += 2 * math.Pi
			}
			if gap > math.Pi+1e-3 {
				return Map{}, fmt.Errorf(
					"mosaic: the roads meeting at (%g, %g) leave more than a straight angle open",
					at.At.X, at.At.Y,
				)
			}
		}
		m.Junctions = append(m.Junctions, Junction{At: at.At, Ways: at.Ways, Radius: l.Corner})
	}
	return m, nil
}
