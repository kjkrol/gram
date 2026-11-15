package main

import (
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
)

// the buildings of a map, each OpenStreetMap's own: the kind of cell under them, how high a
// storey is and the kinds of buildings, a building of the first whose words it answers
type buildingsCfg struct {
	Cells  string  `yaml:"cells"`
	Storey float64 `yaml:"storey"` // metres
	// the longest a building is either way, metres: a longer one is written in parts
	Longest float64       `yaml:"longest"`
	Kinds   []buildingCfg `yaml:"kinds"`
}

// a kind of building and whose it is: of every tag named one of its values, Floors storeys or
// more, a footprint of Over square metres or more and under Under; a word left out is no bound
type buildingCfg struct {
	Kind   string               `yaml:"kind"`
	Tags   map[string]cellKinds `yaml:"tags"`
	Floors int                  `yaml:"floors"`
	Over   float64              `yaml:"over"`
	Under  float64              `yaml:"under"`
}

// the streets of a map, drawn as cells of a kind where OpenStreetMap's ways of the classes run
func (b buildingsCfg) validate() error {
	if b.Cells == "" {
		if len(b.Kinds) > 0 {
			return fmt.Errorf("buildings: kinds without the cells they stand on")
		}
		return nil
	}
	if b.Storey <= 0 || len(b.Kinds) == 0 {
		return fmt.Errorf("buildings: want a storey's metres and the kinds")
	}
	for i, k := range b.Kinds {
		if k.Kind == "" {
			return fmt.Errorf("buildings: kind %d has no name", i+1)
		}
	}
	return nil
}

// names are the kinds of buildings the recipe may write, each once
func (b buildingsCfg) names() []string {
	var out []string
	for _, k := range b.Kinds {
		if !slices.Contains(out, k.Kind) {
			out = append(out, k.Kind)
		}
	}
	return out
}

// osmBuilding is a building as OpenStreetMap has it: its tags and its outline, longitude and
// latitude
type osmBuilding struct {
	id   int64
	tags map[string]string
	ring [][2]float64
}

// built is a building of the map: its kind, its middle, its length and width, world units, how
// it is turned, degrees from the east towards the south, and its storeys, 0 for unknown
type built struct {
	kind   string
	mid    [2]float64
	size   [2]float64
	turn   float64
	floors int
}

// stand lays the buildings OpenStreetMap has on the chart: the cells under each, of the
// buildings' kind, and each as the rectangle of its outline's area lying as its narrowest
// frame lies; one whose middle is off the land is left out
func (w *chart) stand(all []osmBuilding) {
	cfg := w.r.Buildings
	for _, b := range all {
		ring := make([][2]float64, len(b.ring))
		for i, p := range b.ring {
			x, y := w.place(p[0], p[1])
			ring[i] = [2]float64{x * w.size, y * w.size}
		}
		mid, size, turn, ok := framed(ring)
		if !ok {
			continue
		}
		cx, cy := int(math.Floor(mid[0]/w.size)), int(math.Floor(mid[1]/w.size))
		if cx < 0 || cy < 0 || cx >= w.cols || cy >= w.rows || !w.land[cy*w.cols+cx] {
			continue
		}
		floors := storeys(b.tags, cfg.Storey)
		area := size[0] * size[1] * w.perUnit * w.perUnit
		name, ok := cfg.kindOf(b.tags, floors, area)
		if !ok {
			continue
		}
		w.soils[cy*w.cols+cx] = cfg.Cells
		w.fill(shape{b.ring}, func(i int) {
			if w.land[i] {
				w.soils[i] = cfg.Cells
			}
		})
		// cut along and across where it is longer than the longest
		along, across := 1, 1
		if cfg.Longest > 0 {
			along = max(1, int(math.Ceil(size[0]*w.perUnit/cfg.Longest)))
			across = max(1, int(math.Ceil(size[1]*w.perUnit/cfg.Longest)))
		}
		sin, cos := math.Sincos(turn * math.Pi / 180)
		for k := range along * across {
			a := (float64(k%along)+0.5)/float64(along)*size[0] - size[0]/2
			c := (float64(k/along)+0.5)/float64(across)*size[1] - size[1]/2
			w.buildings = append(w.buildings, built{
				kind:   name,
				mid:    [2]float64{mid[0] + cos*a - sin*c, mid[1] + sin*a + cos*c},
				size:   [2]float64{size[0] / float64(along), size[1] / float64(across)},
				turn:   turn,
				floors: floors,
			})
		}
	}
	slices.SortStableFunc(w.buildings, func(a, b built) int {
		if a.mid[1] != b.mid[1] {
			return cmpFloat(a.mid[1], b.mid[1])
		}
		return cmpFloat(a.mid[0], b.mid[0])
	})
}

// kindOf is the first kind of the recipe's a building of tags, floors storeys and area square
// metres is
func (b buildingsCfg) kindOf(tags map[string]string, floors int, area float64) (string, bool) {
next:
	for _, k := range b.Kinds {
		for tag, values := range k.Tags {
			if !slices.Contains(values, tags[tag]) {
				continue next
			}
		}
		if floors < k.Floors || area < k.Over || k.Under > 0 && area >= k.Under {
			continue
		}
		return k.Kind, true
	}
	return "", false
}

// storeys is how many storeys a building of tags has: its levels, or its height in storeys of
// storey metres; 0 where OpenStreetMap says neither
func storeys(tags map[string]string, storey float64) int {
	if v, err := strconv.ParseFloat(tags["building:levels"], 64); err == nil && v >= 1 {
		return int(math.Round(v))
	}
	h := strings.TrimSpace(strings.TrimSuffix(tags["height"], "m"))
	if v, err := strconv.ParseFloat(h, 64); err == nil && v > 0 && storey > 0 {
		return max(1, int(math.Round(v/storey)))
	}
	return 0
}

// framed is the rectangle a building of the outline ring stands as: the narrowest frame round
// the outline, one side along an edge of it, drawn in to the outline's own area; its middle, its
// length and width and how its length is turned, degrees. An outline of no area has none.
func framed(ring [][2]float64) (mid, size [2]float64, turn float64, ok bool) {
	area := 0.0
	for i := range ring {
		a, b := ring[i], ring[(i+1)%len(ring)]
		area += a[0]*b[1] - b[0]*a[1]
	}
	area = math.Abs(area) / 2
	if len(ring) < 3 || area <= 0 {
		return mid, size, 0, false
	}
	least := math.Inf(1)
	for i := range ring {
		a, b := ring[i], ring[(i+1)%len(ring)]
		l := math.Hypot(b[0]-a[0], b[1]-a[1])
		if l == 0 {
			continue
		}
		ux, uy := (b[0]-a[0])/l, (b[1]-a[1])/l
		lo, hi := [2]float64{math.Inf(1), math.Inf(1)}, [2]float64{math.Inf(-1), math.Inf(-1)}
		for _, p := range ring {
			s, t := p[0]*ux+p[1]*uy, -p[0]*uy+p[1]*ux
			lo = [2]float64{min(lo[0], s), min(lo[1], t)}
			hi = [2]float64{max(hi[0], s), max(hi[1], t)}
		}
		along, across := hi[0]-lo[0], hi[1]-lo[1]
		if along*across >= least {
			continue
		}
		least = along * across
		s, t := (lo[0]+hi[0])/2, (lo[1]+hi[1])/2
		mid = [2]float64{s*ux - t*uy, s*uy + t*ux}
		size = [2]float64{along, across}
		turn = math.Atan2(uy, ux) * 180 / math.Pi
		if across > along {
			size = [2]float64{across, along}
			turn += 90
		}
	}
	if math.IsInf(least, 1) || least <= 0 {
		return mid, size, 0, false
	}
	shrink := math.Sqrt(area / least)
	size = [2]float64{size[0] * shrink, size[1] * shrink}
	turn = math.Mod(math.Mod(turn, 180)+180, 180)
	return mid, size, turn, true
}

// pave draws the cells OpenStreetMap's streets run through as the streets' kind
func (w *chart) writeBuildings(b *strings.Builder) {
	for _, s := range w.buildings {
		fmt.Fprintf(b, "    - {kind: %s, mid: [%s, %s], size: [%s, %s], turn: %s", s.kind,
			tenth(s.mid[0]), tenth(s.mid[1]), tenth(s.size[0]), tenth(s.size[1]), tenth(s.turn))
		if s.floors > 0 {
			fmt.Fprintf(b, ", floors: %d", s.floors)
		}
		b.WriteString("}\n")
	}
}

func tenth(v float64) string {
	return strconv.FormatFloat(math.Round(v*10)/10, 'f', -1, 64)
}
