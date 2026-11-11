package main

import (
	"encoding/json"
	"fmt"
	"maps"
	"math"
	"net/url"
	"slices"
	"strconv"
)

// the land cover a map takes of the European Environment Agency's services: CORINE Land Cover
// 2018 (© European Union, Copernicus Land Monitoring Service 2018, EEA) and the Natura 2000
// sites (© EEA), each asked by a query and kept beside the tiles
const (
	corineURL = "https://image.discomap.eea.europa.eu/arcgis/rest/services/Corine/CLC2018_WM/" +
		"MapServer/0/query"
	sitesURL = "https://bio.discomap.eea.europa.eu/arcgis/rest/services/ProtectedSites/" +
		"Natura2000Sites/MapServer/2/query"
	// how many shapes the services give an answer, at most
	coverPage = 1000
)

// what the land's cells are drawn as where the land cover says: the kinds by CORINE's classes,
// and the sites that make one kind another
type coverCfg struct {
	Source string           `yaml:"source"` // corine, or none
	Kinds  map[string][]int `yaml:"kinds"`
	Sites  []siteCfg        `yaml:"sites"`
}

// within the Natura 2000 site named, the cells of the kinds Of are of Kind
type siteCfg struct {
	Site string   `yaml:"site"`
	Of   []string `yaml:"of"`
	Kind string   `yaml:"kind"`
}

// kinds are the kinds the cover draws cells as, in order
func (c coverCfg) kinds() []string {
	out := slices.Collect(maps.Keys(c.Kinds))
	for _, s := range c.Sites {
		if !slices.Contains(out, s.Kind) {
			out = append(out, s.Kind)
		}
	}
	slices.Sort(out)
	return out
}

func (c coverCfg) validate() error {
	if c.Source != "" && c.Source != "corine" {
		return fmt.Errorf("cover: no source %q", c.Source)
	}
	seen := map[int]string{}
	for _, kind := range slices.Sorted(maps.Keys(c.Kinds)) {
		for _, class := range c.Kinds[kind] {
			if other, ok := seen[class]; ok {
				return fmt.Errorf("cover: the class %d is %s and %s", class, other, kind)
			}
			seen[class] = kind
		}
	}
	for _, s := range c.Sites {
		if s.Site == "" || s.Kind == "" || len(s.Of) == 0 {
			return fmt.Errorf("cover: a site wants {site, of, kind}")
		}
		for _, of := range s.Of {
			if _, ok := c.Kinds[of]; !ok {
				return fmt.Errorf("cover: the site %s turns %q, no kind of the cover's", s.Site, of)
			}
		}
	}
	return nil
}

// a shape of the land cover: its rings, longitude and latitude, the first its outline and the
// rest the holes in it
type shape [][][2]float64

// coverData is what a map takes of the land cover: the shapes of each class and of each site
type coverData struct {
	classes map[int][]shape
	sites   map[string][]shape
}

// landcover is the land cover within a box, south, west, north, east, and the sites named
type landcover func(box [4]float64, sites []string) (*coverData, error)

// newLandcover asks the agency's services, each answer kept in cache by its query
func newLandcover(cache string) landcover {
	return landcoverOf(&overpass{cache: cache, post: get})
}

// landcoverOf asks through o, each query a URL
func landcoverOf(o *overpass) landcover {
	return func(box [4]float64, sites []string) (*coverData, error) {
		data := &coverData{classes: map[int][]shape{}, sites: map[string][]shape{}}
		// the service's envelope is west, south, east, north
		frame := boxOf([4]float64{box[1], box[0], box[3], box[2]})
		for from := 0; ; from += coverPage {
			q := url.Values{
				"f":                 {"geojson"},
				"geometry":          {frame},
				"geometryType":      {"esriGeometryEnvelope"},
				"inSR":              {"4326"},
				"outSR":             {"4326"},
				"spatialRel":        {"esriSpatialRelIntersects"},
				"outFields":         {"Code_18"},
				"geometryPrecision": {"5"},
				"orderByFields":     {"OBJECTID"},
				"resultOffset":      {strconv.Itoa(from)},
				"resultRecordCount": {strconv.Itoa(coverPage)},
			}
			var page features
			if err := o.answer(corineURL+"?"+q.Encode(), &page); err != nil {
				return nil, err
			}
			for _, f := range page.Features {
				class, err := strconv.Atoi(f.Properties.Code)
				if err != nil {
					return nil, fmt.Errorf("cover: a shape of the class %q", f.Properties.Code)
				}
				shapes, err := f.shapes()
				if err != nil {
					return nil, err
				}
				data.classes[class] = append(data.classes[class], shapes...)
			}
			if len(page.Features) < coverPage {
				break
			}
		}
		for _, site := range sites {
			q := url.Values{
				"f":                 {"geojson"},
				"where":             {"SITECODE='" + site + "'"},
				"outSR":             {"4326"},
				"outFields":         {"SITECODE"},
				"geometryPrecision": {"5"},
			}
			var page features
			if err := o.answer(sitesURL+"?"+q.Encode(), &page); err != nil {
				return nil, err
			}
			if len(page.Features) == 0 {
				return nil, fmt.Errorf("cover: no Natura 2000 site %s", site)
			}
			for _, f := range page.Features {
				shapes, err := f.shapes()
				if err != nil {
					return nil, err
				}
				data.sites[site] = append(data.sites[site], shapes...)
			}
		}
		return data, nil
	}
}

// features is a GeoJSON answer of the services
type features struct {
	Features []feature `json:"features"`
}

type feature struct {
	Properties struct {
		Code string `json:"Code_18"`
	} `json:"properties"`
	Geometry struct {
		Type        string          `json:"type"`
		Coordinates json.RawMessage `json:"coordinates"`
	} `json:"geometry"`
}

func (f feature) shapes() ([]shape, error) {
	switch f.Geometry.Type {
	case "Polygon":
		var s shape
		err := json.Unmarshal(f.Geometry.Coordinates, &s)
		return []shape{s}, err
	case "MultiPolygon":
		var s []shape
		err := json.Unmarshal(f.Geometry.Coordinates, &s)
		return s, err
	}
	return nil, fmt.Errorf("cover: a shape of the geometry %q", f.Geometry.Type)
}

// cover lays the land cover over the chart's cells: the kind of each by its class and the sites
// it lies in, by the middle of the cell
func (w *chart) cover(data *coverData) {
	c := w.r.Cover
	w.covered = make([]string, w.cols*w.rows)
	for _, kind := range slices.Sorted(maps.Keys(c.Kinds)) {
		for _, class := range c.Kinds[kind] {
			for _, s := range data.classes[class] {
				w.fill(s, func(i int) { w.covered[i] = kind })
			}
		}
	}
	for _, site := range c.Sites {
		for _, s := range data.sites[site.Site] {
			w.fill(s, func(i int) {
				if slices.Contains(site.Of, w.covered[i]) {
					w.covered[i] = site.Kind
				}
			})
		}
	}
}

// fill calls at with every cell whose middle lies within s, a row at a time: between the
// crossings of the row's middle line with its rings, a hole's among them
func (w *chart) fill(s shape, at func(cell int)) {
	type point struct{ x, y float64 }
	var rings [][]point
	top, bottom := math.Inf(1), math.Inf(-1)
	for _, ring := range s {
		ps := make([]point, len(ring))
		for i, p := range ring {
			x, y := w.place(p[0], p[1])
			ps[i] = point{x, y}
			top, bottom = min(top, y), max(bottom, y)
		}
		rings = append(rings, ps)
	}
	var cross []float64
	for y := max(int(math.Floor(top)), 0); y < min(int(math.Ceil(bottom)), w.rows); y++ {
		line := float64(y) + 0.5
		cross = cross[:0]
		for _, ps := range rings {
			for i, a := range ps {
				b := ps[(i+1)%len(ps)]
				if (a.y <= line) != (b.y <= line) {
					cross = append(cross, a.x+(line-a.y)/(b.y-a.y)*(b.x-a.x))
				}
			}
		}
		slices.Sort(cross)
		for k := 0; k+1 < len(cross); k += 2 {
			from := max(int(math.Ceil(cross[k]-0.5)), 0)
			to := min(int(math.Ceil(cross[k+1]-0.5)), w.cols)
			for x := from; x < to; x++ {
				at(y*w.cols + x)
			}
		}
	}
}
