package main

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

// the Overpass API and a mirror of it: OpenStreetMap's data by a query, © OpenStreetMap
// contributors, under the ODbL (https://www.openstreetmap.org/copyright)
var overpassURLs = []string{
	"https://overpass-api.de/api/interpreter",
	"https://overpass.kumi.systems/api/interpreter",
}

// the classes of OpenStreetMap's highways a recipe may name, the greatest first
var osmClasses = []string{"motorway", "trunk", "primary", "secondary", "tertiary", "unclassified",
	"residential"}

// osmWay is a stretch of road as OpenStreetMap has it: its class (a link's is its road's), what
// it runs through or over and its line, longitude and latitude
type osmWay struct {
	id     int64
	class  int // an index into osmClasses
	tunnel bool
	bridge bool
	line   [][2]float64
	// its lanes in all, whether it is driven one way, along its line, and whether it is a
	// roundabout's ring
	lanes      int
	oneway     bool
	roundabout bool
}

// osmData is what a map takes of OpenStreetMap: the roads and the middles of the buildings, or
// the buildings whole and the streets' lines
type osmData struct {
	ways       []osmWay
	buildings  [][2]float64
	footprints []osmBuilding
	greens     *osmGreens
	coast      *osmCoast
}

// overpass asks the Overpass API, each answer kept in cache by its query
type overpass struct {
	cache string
	post  func(query string) (io.ReadCloser, error)
}

func newOverpass(cache string) *overpass { return &overpass{cache: cache, post: ask} }

// source is what the recipe r takes of the API: asked of no classes, the town's own, its
// buildings whole; else the roads of the classes and the buildings' middles
func (o *overpass) source(r recipe) source {
	return func(box [4]float64, classes []string, buildings bool) (*osmData, error) {
		data := &osmData{}
		var err error
		if len(classes) == 0 {
			if r.Buildings.Cells != "" {
				if data.footprints, err = o.footprints(box); err != nil {
					return nil, err
				}
			}
			if len(r.Greens.Trees) > 0 {
				if data.greens, err = o.greens(box); err != nil {
					return nil, err
				}
			}
			if r.Coast.Source != "" {
				data.coast, err = o.coast(box)
			}
			return data, err
		}
		if data.ways, err = o.roads(box, classes); err != nil {
			return nil, err
		}
		if buildings {
			data.buildings, err = o.buildings(box)
		}
		return data, err
	}
}

// roads are the ways of classes within the box south, west, north, east
func (o *overpass) roads(box [4]float64, classes []string) ([]osmWay, error) {
	q := fmt.Sprintf("[out:json][timeout:180][bbox:%s];way[highway~\"^(%s)(_link)?$\"];"+
		"out tags geom;", boxOf(box), strings.Join(classes, "|"))
	var answer struct {
		Elements []struct {
			ID       int64             `json:"id"`
			Tags     map[string]string `json:"tags"`
			Geometry []struct {
				Lat float64 `json:"lat"`
				Lon float64 `json:"lon"`
			} `json:"geometry"`
		} `json:"elements"`
	}
	if err := o.answer(q, &answer); err != nil {
		return nil, err
	}
	var ways []osmWay
	for _, e := range answer.Elements {
		class := slices.Index(osmClasses, strings.TrimSuffix(e.Tags["highway"], "_link"))
		if class < 0 || len(e.Geometry) < 2 {
			continue
		}
		w := osmWay{
			id:     e.ID,
			class:  class,
			tunnel: e.Tags["tunnel"] == "yes",
			bridge: e.Tags["bridge"] != "" && e.Tags["bridge"] != "no",
		}
		for _, p := range e.Geometry {
			w.line = append(w.line, [2]float64{p.Lon, p.Lat})
		}
		w.roundabout = e.Tags["junction"] == "roundabout" || e.Tags["junction"] == "circular"
		w.oneway = e.Tags["oneway"] == "yes" || e.Tags["oneway"] == "-1" || w.roundabout
		if e.Tags["oneway"] == "-1" {
			slices.Reverse(w.line)
		}
		if w.lanes, _ = strconv.Atoi(e.Tags["lanes"]); w.lanes <= 0 {
			w.lanes = 2
			if w.oneway {
				w.lanes = 1
			}
		}
		ways = append(ways, w)
	}
	slices.SortStableFunc(ways, func(a, b osmWay) int {
		if a.class != b.class {
			return a.class - b.class
		}
		return int(a.id - b.id)
	})
	return ways, nil
}

// buildings are the middles of the buildings within the box south, west, north, east
func (o *overpass) buildings(box [4]float64) ([][2]float64, error) {
	q := fmt.Sprintf("[out:json][timeout:180][bbox:%s];way[building];out ids center;",
		boxOf(box))
	var answer struct {
		Elements []struct {
			Center struct {
				Lat float64 `json:"lat"`
				Lon float64 `json:"lon"`
			} `json:"center"`
		} `json:"elements"`
	}
	if err := o.answer(q, &answer); err != nil {
		return nil, err
	}
	out := make([][2]float64, 0, len(answer.Elements))
	for _, e := range answer.Elements {
		out = append(out, [2]float64{e.Center.Lon, e.Center.Lat})
	}
	return out, nil
}

// an element of an answer with its tags and its line
type osmElement struct {
	ID       int64             `json:"id"`
	Tags     map[string]string `json:"tags"`
	Geometry []struct {
		Lat float64 `json:"lat"`
		Lon float64 `json:"lon"`
	} `json:"geometry"`
}

func (e osmElement) line() [][2]float64 {
	out := make([][2]float64, 0, len(e.Geometry))
	for _, p := range e.Geometry {
		out = append(out, [2]float64{p.Lon, p.Lat})
	}
	return out
}

// footprints are the buildings within the box south, west, north, east, each with its outline
func (o *overpass) footprints(box [4]float64) ([]osmBuilding, error) {
	q := fmt.Sprintf("[out:json][timeout:180][bbox:%s];way[building];out tags geom;",
		boxOf(box))
	var answer struct {
		Elements []osmElement `json:"elements"`
	}
	if err := o.answer(q, &answer); err != nil {
		return nil, err
	}
	out := make([]osmBuilding, 0, len(answer.Elements))
	for _, e := range answer.Elements {
		ring := e.line()
		if n := len(ring); n > 1 && ring[0] == ring[n-1] {
			ring = ring[:n-1]
		}
		if len(ring) >= 3 {
			out = append(out, osmBuilding{id: e.ID, tags: e.Tags, ring: ring})
		}
	}
	slices.SortFunc(out, func(a, b osmBuilding) int { return int(a.id - b.id) })
	return out, nil
}

// greens are the trees, the rows of trees and the parks within the box south, west, north, east
func (o *overpass) greens(box [4]float64) (*osmGreens, error) {
	q := fmt.Sprintf("[out:json][timeout:180][bbox:%s];(node[natural=tree];"+
		"way[natural=tree_row];way[leisure~\"^(park|garden)$\"];"+
		"way[landuse~\"^(grass|cemetery|recreation_ground|village_green)$\"];);"+
		"out tags geom;", boxOf(box))
	var answer struct {
		Elements []struct {
			osmElement
			Type string  `json:"type"`
			Lat  float64 `json:"lat"`
			Lon  float64 `json:"lon"`
		} `json:"elements"`
	}
	if err := o.answer(q, &answer); err != nil {
		return nil, err
	}
	slices.SortFunc(answer.Elements, func(a, b struct {
		osmElement
		Type string  `json:"type"`
		Lat  float64 `json:"lat"`
		Lon  float64 `json:"lon"`
	}) int {
		return int(a.ID - b.ID)
	})
	g := &osmGreens{}
	for _, e := range answer.Elements {
		switch {
		case e.Type == "node":
			g.trees = append(g.trees, [2]float64{e.Lon, e.Lat})
		case e.Tags["natural"] == "tree_row" && len(e.Geometry) >= 2:
			g.rows = append(g.rows, e.line())
		case len(e.Geometry) >= 4:
			g.parks = append(g.parks, e.line())
		}
	}
	return g, nil
}

func boxOf(box [4]float64) string {
	parts := make([]string, len(box))
	for i, v := range box {
		parts[i] = strconv.FormatFloat(v, 'f', 4, 64)
	}
	return strings.Join(parts, ",")
}

func (o *overpass) answer(query string, into any) error {
	sum := sha1.Sum([]byte(query))
	path := filepath.Join(o.cache, hex.EncodeToString(sum[:])+".json")
	if _, err := os.Stat(path); err != nil {
		if err := o.download(query, path); err != nil {
			return err
		}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, into); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return nil
}

func (o *overpass) download(query, path string) error {
	body, err := o.post(query)
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

// ask asks each of the API's servers in turn, a busy one passed over
func ask(query string) (io.ReadCloser, error) {
	var why error
	for range 2 {
		for _, at := range overpassURLs {
			body, err := askAt(at, query)
			if err == nil {
				return body, nil
			}
			why = err
		}
	}
	return nil, why
}

func askAt(at, query string) (io.ReadCloser, error) {
	req, err := http.NewRequest(http.MethodPost, at,
		strings.NewReader(url.Values{"data": {query}}.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "gramap")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("%s: %s", at, resp.Status)
	}
	return resp.Body, nil
}
