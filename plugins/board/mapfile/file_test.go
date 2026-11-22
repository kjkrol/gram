package mapfile_test

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugins/board/mapfile"
)

const manifest = `
size: [100, 100]
seed: 3
corner: 6
heights: {file: heights.png, pixel: 50, lowest: -10, step: 0.01}
features: features.geojson
roads:
  residential: {lanes: 2, lane: 3.5, walk: 2.5, kerb: 0.15, verge: 1.5}
  track: {lanes: 1, lane: 3}
mosaic: {apart: 3}
`

func feature(tags, kind, coordinates string) string {
	return `{"type":"Feature","properties":{` + tags + `},"geometry":{"type":"` + kind +
		`","coordinates":` + coordinates + `}}`
}

func road(coordinates string) string {
	return feature(`"highway":"residential"`, "LineString", coordinates)
}

func files(t *testing.T, features ...string) fstest.MapFS {
	t.Helper()
	img := image.NewGray16(image.Rect(0, 0, 3, 3))
	// 0 m along the north edge, 20 m in the south-east corner
	img.SetGray16(0, 0, color.Gray16{Y: 1000})
	img.SetGray16(1, 0, color.Gray16{Y: 1000})
	img.SetGray16(2, 0, color.Gray16{Y: 1000})
	img.SetGray16(2, 2, color.Gray16{Y: 3000})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	all := `{"type":"FeatureCollection","features":[` + strings.Join(features, ",") + "]}"
	return fstest.MapFS{
		"lv/level.yaml":       {Data: []byte(manifest)},
		"lv/heights.png":      {Data: buf.Bytes()},
		"lv/features.geojson": {Data: []byte(all)},
	}
}

func load(t *testing.T, features ...string) *mapfile.File {
	t.Helper()
	l, err := mapfile.Load(files(t, features...), "lv/level.yaml")
	if err != nil {
		t.Fatal(err)
	}
	return l
}

func near(a, b geom.Vec) bool {
	d := a.Sub(b)
	return d.X*d.X+d.Y*d.Y < 1e-4
}

func TestHeightsAreTheImageEvenBetweenThePixels(t *testing.T) {
	l := load(t)
	for _, c := range []struct {
		x, y, want float32
	}{
		{0, 0, 0},
		{100, 100, 20},
		{75, 100, 5},
		{50, 50, -10},
		{25, 0, 0},
		{-40, 300, -10},
	} {
		if got := l.HeightAt(c.x, c.y); got < c.want-1e-3 || got > c.want+1e-3 {
			t.Errorf("the height at (%g, %g) is %g, want %g", c.x, c.y, got, c.want)
		}
	}
}

func TestAFeaturesPlacesAreTurnedToTheWorlds(t *testing.T) {
	l := load(t,
		feature(`"amenity":"bench","seats":3`, "Point", `[10,90]`),
		feature(`"highway":"residential"`, "LineString", `[[0,0],[0,0],[30,10]]`),
		feature(`"natural":"water"`, "Polygon",
			`[[[20,20],[60,20],[60,50],[20,20]],[[40,25],[50,25],[50,30],[40,25]]]`),
	)
	if len(l.Features) != 3 {
		t.Fatalf("%d features, want 3", len(l.Features))
	}
	bench, line, pond := l.Features[0], l.Features[1], l.Features[2]
	// north is up in the file, down the world
	if bench.Shape != mapfile.Point || !near(bench.Path[0], geom.NewVec(10, 10)) {
		t.Errorf("the bench is %+v", bench)
	}
	if bench.Tags["amenity"] != "bench" || bench.Tags["seats"] != "3" {
		t.Errorf("the bench's tags are %v", bench.Tags)
	}
	if line.Shape != mapfile.Line || len(line.Path) != 2 || !near(line.Path[1], geom.NewVec(30, 90)) {
		t.Errorf("the line is %+v, want its doubled point once", line)
	}
	if pond.Shape != mapfile.Area || len(pond.Path) != 3 || len(pond.Holes) != 1 ||
		len(pond.Holes[0]) != 3 {
		t.Errorf("the pond is %+v, want an outline and a hole of three points each", pond)
	}
}

func TestLinesSharingAPointMeetThere(t *testing.T) {
	// a road through (50, 50) south to north in two lines joined at (50, 30), a side road
	// ending on a point within it
	l := load(t,
		road(`[[50,0],[50,30]]`),
		road(`[[50,30],[50,50],[50,100]]`),
		road(`[[100,50],[50,50]]`),
		feature(`"natural":"water"`, "LineString", `[[50,50],[0,50]]`),
	)
	roads, meetings := l.Network()
	if len(roads) != 3 {
		t.Fatalf("%d roads, want 3: the two lines joined, cut at the side road", len(roads))
	}
	long := 0
	for _, r := range roads {
		if len(r.Path) == 3 {
			long++
		}
	}
	if long != 1 {
		t.Errorf("%d roads of three points, want the one joined at (50, 30)", long)
	}
	if len(meetings) != 1 || !near(meetings[0].At, geom.NewVec(50, 50)) {
		t.Fatalf("the meetings are %+v, want one at (50, 50)", meetings)
	}
	// the road to the file's y of 100 leaves up the world
	want := []geom.Vec{geom.NewVec(0, -1), geom.NewVec(1, 0), geom.NewVec(0, 1)}
	for k, w := range meetings[0].Ways {
		if !near(w, want[k]) {
			t.Errorf("way %d is %v, want %v", k, w, want[k])
		}
	}
}

func TestRoadsOfTwoHighwaysAreNotJoined(t *testing.T) {
	l := load(t,
		road(`[[0,50],[50,50]]`),
		feature(`"highway":"track"`, "LineString", `[[50,50],[100,50]]`),
	)
	roads, meetings := l.Network()
	if len(roads) != 2 || len(meetings) != 0 {
		t.Errorf("%d roads and %d meetings, want 2 and none", len(roads), len(meetings))
	}
}

func TestAPartOfTheManifestIsItsPlugins(t *testing.T) {
	l := load(t)
	var own struct {
		Apart float32 `yaml:"apart"`
	}
	if ok, err := l.Part("mosaic", &own); !ok || err != nil || own.Apart != 3 {
		t.Errorf("the mosaic's part is %+v (%v, %v), want apart 3", own, ok, err)
	}
	if ok, _ := l.Part("forest", &own); ok {
		t.Error("a part the manifest has not was found")
	}
	if l.Sections["track"].Lanes != 1 || l.Corner != 6 || l.Seed != 3 {
		t.Errorf("the level is %+v", l)
	}
}

func TestALevelSaysWhatIsWrongWithIt(t *testing.T) {
	for _, c := range []struct {
		name, was, is, says string
	}{
		{"an image too small", "size: [100, 100]", "size: [101, 100]", "do not cover"},
		{"no pixel", "pixel: 50, ", "", "not set"},
		{"no heights' file", "heights.png", "none.png", "none.png"},
		{"no size", "size: [100, 100]", "", "no size"},
	} {
		fsys := files(t)
		changed := strings.Replace(manifest, c.was, c.is, 1)
		if changed == manifest {
			t.Fatalf("%s: nothing replaced", c.name)
		}
		fsys["lv/level.yaml"].Data = []byte(changed)
		_, err := mapfile.Load(fsys, "lv/level.yaml")
		if err == nil || !strings.Contains(err.Error(), c.says) {
			t.Errorf("%s: the error is %v, want one saying %q", c.name, err, c.says)
		}
	}
	fsys := files(t)
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewGray(image.Rect(0, 0, 3, 3))); err != nil {
		t.Fatal(err)
	}
	fsys["lv/heights.png"].Data = buf.Bytes()
	if _, err := mapfile.Load(fsys, "lv/level.yaml"); err == nil {
		t.Error("an image of 8 bits was taken for heights")
	}
	fsys = files(t, feature("", "MultiPolygon", `[]`))
	if _, err := mapfile.Load(fsys, "lv/level.yaml"); err == nil {
		t.Error("a geometry the level does not read was taken")
	}
}

func TestTheLevelInFilesIsTheGroundItDescribes(t *testing.T) {
	l, err := mapfile.Load(os.DirFS("testdata"), "level/level.yaml")
	if err != nil {
		t.Fatal(err)
	}
	// the hills the image was made of, a pixel every 4 m and a step a millimetre
	for _, at := range [][2]float64{{0, 0}, {384, 384}, {40, 300}, {301.5, 77.25}} {
		want := float32(20*math.Sin(at[0]/80)*math.Cos(at[1]/95) + 5*math.Sin((at[0]+at[1])/40))
		got := l.HeightAt(float32(at[0]), float32(at[1]))
		if d := got - want; d < -0.05 || d > 0.05 {
			t.Errorf("the height at %v is %g, the hills' %g", at, got, want)
		}
	}
	if len(l.Features) != 5 {
		t.Errorf("%d features, want the four lines and the pond", len(l.Features))
	}
	// Main Street cut at the crossing, North Road's two lines joined, Mill Lane
	roads, meetings := l.Network()
	if len(roads) != 4 {
		t.Fatalf("%d roads, want 4", len(roads))
	}
	if len(meetings) != 1 || !near(meetings[0].At, geom.NewVec(192, 192)) {
		t.Fatalf("the meetings are %+v, want one at (192, 192)", meetings)
	}
	// north, east, Mill Lane to the south-east, west
	want := []geom.Vec{
		geom.NewVec(0, -1),
		geom.NewVec(1, 0),
		geom.NewVec(0.5, 0.866),
		geom.NewVec(-1, 0),
	}
	if len(meetings[0].Ways) != len(want) {
		t.Fatalf("%d ways, want %d", len(meetings[0].Ways), len(want))
	}
	for k, w := range meetings[0].Ways {
		if !near(w, want[k]) {
			t.Errorf("way %d is %v, want %v", k, w, want[k])
		}
	}
}
