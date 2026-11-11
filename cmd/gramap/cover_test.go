package main

import (
	"io"
	"strings"
	"testing"
)

func TestLandcover_ReadsTheShapesOfTheClassesAndOfTheSites(t *testing.T) {
	asked := 0
	o := &overpass{cache: t.TempDir(), post: func(q string) (io.ReadCloser, error) {
		asked++
		answer := `{"type": "FeatureCollection", "features": [
			{"properties": {"Code_18": "311"}, "geometry": {"type": "Polygon", "coordinates":
			 [[[1, 1], [2, 1], [2, 2], [1, 1]], [[1.5, 1.2], [1.8, 1.2], [1.8, 1.4], [1.5, 1.2]]]}},
			{"properties": {"Code_18": "322"}, "geometry": {"type": "MultiPolygon", "coordinates":
			 [[[[3, 3], [4, 3], [4, 4], [3, 3]]], [[[5, 5], [6, 5], [6, 6], [5, 5]]]]}}]}`
		switch {
		case strings.Contains(q, "Corine") && strings.Contains(q, "2.0000%2C1.0000%2C4.0000"):
		case strings.Contains(q, "Natura2000") && strings.Contains(q, "PTMAD0001"):
			answer = `{"features": [{"properties": {}, "geometry": {"type": "Polygon",
				"coordinates": [[[1, 1], [2, 1], [2, 2], [1, 1]]]}}]}`
		default:
			t.Errorf("asked %q, want the classes within the box west first, or the site", q)
		}
		return io.NopCloser(strings.NewReader(answer)), nil
	}}
	for range 2 {
		data, err := landcoverOf(o)([4]float64{1, 2, 3, 4}, []string{"PTMAD0001"})
		if err != nil {
			t.Fatal(err)
		}
		if got := data.classes[311]; len(got) != 1 || len(got[0]) != 2 {
			t.Errorf("the class 311 is %v, want one shape with a hole", got)
		}
		if got := data.classes[322]; len(got) != 2 {
			t.Errorf("the class 322 is %v, want the two shapes of its multipolygon", got)
		}
		if got := data.sites["PTMAD0001"]; len(got) != 1 {
			t.Errorf("the site is %v, want its one shape", got)
		}
	}
	if asked != 2 {
		t.Errorf("asked %d times, want each query asked once and kept", asked)
	}
}

// covered is an island 4 km by 2 km, flat, its west half broad-leaved woods with a clearing in
// them and its east half heath, the north of both a Natura 2000 site
func covered(t *testing.T) (recipe, elevation, landcover) {
	t.Helper()
	r, err := parseRecipe([]byte(`
area: {centre: [10, 40], cells: [60, 40], cell: 100, units: 32}
ground: {rock: {slope: 2}, forest: {from: 0, to: 100, share: 1}}
rivers: {brook: 1000, stream: 2000, river: 3000}
cover:
  source: corine
  kinds: {woodland: [311, 313], heath: [322]}
  sites: [{site: S1, of: [woodland], kind: laurisilva}]
fields:
  - {cells: [laurisilva, woodland], kind: woods, grain: 16, least: 4}
`), "covered.yaml")
	if err != nil {
		t.Fatal(err)
	}
	height := func(lon, lat float64) (float64, error) {
		if x, y := metresOf(r.Area.Centre, lon, lat); x < -2000 || x > 2000 || y < -1000 ||
			y > 1000 {
			return -20, nil
		}
		return 40, nil
	}
	at := func(x, y float64) [2]float64 {
		lon, lat := lonLatOf(r.Area.Centre, x, y)
		return [2]float64{lon, lat}
	}
	box := func(x0, y0, x1, y1 float64) [][2]float64 {
		return [][2]float64{at(x0, y0), at(x1, y0), at(x1, y1), at(x0, y1), at(x0, y0)}
	}
	land := func([4]float64, []string) (*coverData, error) {
		return &coverData{
			classes: map[int][]shape{
				311: {{box(-2000, -1000, 0, 1000), box(-1500, 200, -1000, 700)}},
				322: {{box(0, -1000, 2000, 1000)}},
			},
			sites: map[string][]shape{"S1": {{box(-2000, -1000, 2000, 0)}}},
		}, nil
	}
	return r, height, land
}

func TestBuild_DrawsTheLandAsItsCoverSays(t *testing.T) {
	r, height, land := covered(t)
	c, err := build(r, height, nil, land, false)
	if err != nil {
		t.Fatal(err)
	}
	kind := func(east, south float64) string {
		lon, lat := lonLatOf(r.Area.Centre, east, south)
		x, y := c.place(lon, lat)
		return c.kindAt(int(x), int(y))
	}
	for name, tc := range map[string]struct {
		east, south float64
		want        string
	}{
		"the woods within the site": {-1000, -500, "laurisilva"},
		"the woods off the site":    {-500, 500, "woodland"},
		"the clearing in the woods": {-1250, 450, "earth"},
		"the heath within the site": {1000, -500, "heath"},
		"the heath off the site":    {1000, 500, "heath"},
		"the sea":                   {2500, 0, "water"},
	} {
		if got := kind(tc.east, tc.south); got != tc.want {
			t.Errorf("%s is %s, want %s", name, got, tc.want)
		}
	}
	text := string(c.text("covered.yaml", ""))
	for _, want := range []string{`"l": laurisilva`, `"w": woodland`, `"h": heath`,
		"kind: woods", "CORINE Land Cover 2018"} {
		if !strings.Contains(text, want) {
			t.Errorf("the map lacks %q", want)
		}
	}
	if strings.Contains(text, "forest") {
		t.Error("the map has a forest laid by noise beside the land cover's")
	}
}

func TestBuild_AFlatMapLaysItsForestByNoise(t *testing.T) {
	r, height, land := covered(t)
	c, err := build(r.asFlat(), height, nil, land, true)
	if err != nil {
		t.Fatal(err)
	}
	if text := string(c.text("covered.yaml", "")); !strings.Contains(text, "forest") ||
		strings.Contains(text, "laurisilva") {
		t.Error("a flat map takes the land cover, want its forest by noise as before")
	}
}

func TestRecipe_RefusesACoverItCannotLay(t *testing.T) {
	for name, cover := range map[string]string{
		"an unknown source":    `{source: guess}`,
		"a class of two kinds": `{source: corine, kinds: {a: [311], b: [311]}}`,
		"a site turning no kind": `{source: corine, kinds: {a: [311]},
			sites: [{site: S, kind: b}]}`,
		"a site of a kind unsaid": `{source: corine, kinds: {a: [311]},
			sites: [{site: S, of: [c], kind: b}]}`,
	} {
		_, err := parseRecipe([]byte("area: {centre: [10, 40], cells: [10, 10], cell: 100}\n"+
			"cover: "+cover+"\n"), "bad.yaml")
		if err == nil {
			t.Errorf("%s: the recipe is taken", name)
		}
	}
}
