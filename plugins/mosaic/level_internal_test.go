package mosaic

import (
	"strings"
	"testing"
	"testing/fstest"

	"github.com/kjkrol/aabbworld/geom"
)

const testManifest = `
size: [100, 100]
corner: 6
features: features.geojson
roads:
  residential: {lanes: 2, lane: 3.5, walk: 2.5, kerb: 0.15, verge: 1.5}
  service: {lanes: 1, lane: 3, walk: 2.5, kerb: 0.15, verge: 1.5}
mosaic: {apart: 3, rungs: 4}
`

func testLoad(lines ...string) (Map, error) {
	var all []string
	for _, l := range lines {
		highway, coordinates, _ := strings.Cut(l, " ")
		all = append(all, `{"type":"Feature","properties":{"highway":"`+highway+
			`"},"geometry":{"type":"LineString","coordinates":`+coordinates+`}}`)
	}
	return Load(fstest.MapFS{
		"lv/level.yaml": {Data: []byte(testManifest)},
		"lv/features.geojson": {Data: []byte(
			`{"type":"FeatureCollection","features":[` + strings.Join(all, ",") + "]}")},
	}, "lv/level.yaml")
}

func TestALevelsMeetingsAreTheGroundsJunctions(t *testing.T) {
	m, err := testLoad(
		"residential [[50,0],[50,50],[50,100]]",
		"residential [[100,50],[50,50]]",
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Roads) != 3 || len(m.Junctions) != 1 {
		t.Fatalf("%d roads and %d junctions, want 3 and 1", len(m.Roads), len(m.Junctions))
	}
	j := m.Junctions[0]
	if j.At != geom.NewVec(50, 50) || j.Radius != 6 || len(j.Ways) != 3 {
		t.Errorf("the junction is %+v", j)
	}
	if m.Section.Lanes != 2 || m.Apart != 3 || m.Rungs != 4 {
		t.Errorf("the map's section is %+v, apart %g, rungs %d", m.Section, m.Apart, m.Rungs)
	}
	build(m, 100, 100)
}

func TestALevelSaysWhatTheGroundCannotLay(t *testing.T) {
	for _, c := range []struct {
		name  string
		lines []string
	}{
		{"a highway the level names no road for", []string{"track [[0,0],[10,10]]"}},
		{"roads of two sections", []string{
			"residential [[0,0],[10,10]]",
			"service [[50,0],[60,10]]",
		}},
		{"roads leaving more than a straight angle open", []string{
			"residential [[50,50],[100,50]]",
			"residential [[50,50],[100,60]]",
			"residential [[50,50],[100,40]]",
		}},
	} {
		if _, err := testLoad(c.lines...); err == nil {
			t.Errorf("%s were laid", c.name)
		}
	}
}
