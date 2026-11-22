package mapfile_test

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugins/board/mapfile"
)

func roadsAndBig(tags map[string]string, extent float32) bool {
	return tags["highway"] != "" || extent >= 64
}

// cut is the level in testdata cut into tiles 128 m a side, and the level itself
func cut(t *testing.T) (whole, tiled *mapfile.File, dir string) {
	t.Helper()
	dir = t.TempDir()
	c := mapfile.Cutting{Tile: 128, Margin: 32, Overview: roadsAndBig}
	if err := mapfile.Cut(os.DirFS("testdata"), "level/level.yaml", dir, c); err != nil {
		t.Fatal(err)
	}
	var err error
	if whole, err = mapfile.Load(os.DirFS("testdata"), "level/level.yaml"); err != nil {
		t.Fatal(err)
	}
	if tiled, err = mapfile.Load(os.DirFS(dir), "level.yaml"); err != nil {
		t.Fatal(err)
	}
	return whole, tiled, dir
}

// written is a level of 384 m written into a folder: flat with these features, or with the
// heights given, a pixel every 4 m from 10 m under zero in steps of a centimetre
func written(t *testing.T, heights *image.Gray16, features ...string) string {
	t.Helper()
	dir := t.TempDir()
	manifest := "size: [384, 384]\nfeatures: features.geojson\n"
	if heights != nil {
		manifest += "heights: {file: heights.png, pixel: 4, lowest: -10, step: 0.01}\n"
		var buf bytes.Buffer
		if err := png.Encode(&buf, heights); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "heights.png"), buf.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	all := `{"type":"FeatureCollection","features":[` + strings.Join(features, ",") + "]}"
	for name, text := range map[string]string{"level.yaml": manifest, "features.geojson": all} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func cutOf(t *testing.T, source string) (*mapfile.File, string) {
	t.Helper()
	dir := t.TempDir()
	c := mapfile.Cutting{Tile: 128, Margin: 32, Overview: roadsAndBig}
	if err := mapfile.Cut(os.DirFS(source), "level.yaml", dir, c); err != nil {
		t.Fatal(err)
	}
	l, err := mapfile.Load(os.DirFS(dir), "level.yaml")
	if err != nil {
		t.Fatal(err)
	}
	return l, dir
}

func everywhere() geom.AABB {
	return geom.NewAABB(geom.NewVec(0, 0), geom.NewVec(384, 384))
}

// told is a road as its ends and length say it, whichever way it runs
func told(r mapfile.Road) string {
	var long float64
	for k := 1; k < len(r.Path); k++ {
		d := r.Path[k].Sub(r.Path[k-1])
		long += float64(d.X*d.X + d.Y*d.Y)
	}
	ends := []string{fmt.Sprint(r.Path[0]), fmt.Sprint(r.Path[len(r.Path)-1])}
	sort.Strings(ends)
	return fmt.Sprintf("%s %v %.1f %d", r.Highway, ends, long, len(r.Path))
}

func TestALevelCutAndReadIsTheLevel(t *testing.T) {
	whole, tiled, _ := cut(t)
	// across the level and either side of every seam between tiles
	for _, x := range []float32{0, 3, 77.7, 127.99, 128, 128.01, 200, 255.99, 256, 256.01, 384} {
		for _, y := range []float32{0, 41.5, 127.99, 128, 128.01, 190, 256, 256.01, 300, 384} {
			if got, want := tiled.HeightAt(x, y), whole.HeightAt(x, y); got != want {
				t.Fatalf("the height at (%g, %g) is %g in tiles, %g whole", x, y, got, want)
			}
		}
	}
	if err := tiled.Whole(); err != nil {
		t.Fatal(err)
	}
	if len(tiled.Features) != len(whole.Features) {
		t.Errorf("%d features in tiles, %d whole", len(tiled.Features), len(whole.Features))
	}
	roads, meetings := whole.Network()
	cutRoads, cutMeetings := tiled.Network()
	var want, got []string
	for _, r := range roads {
		want = append(want, told(r))
	}
	for _, r := range cutRoads {
		got = append(got, told(r))
	}
	sort.Strings(want)
	sort.Strings(got)
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("the roads in tiles are\n%v\nwhole\n%v", got, want)
	}
	if fmt.Sprint(cutMeetings) != fmt.Sprint(meetings) {
		t.Errorf("the meetings in tiles are %v, whole %v", cutMeetings, meetings)
	}
}

func TestTheOverviewIsTheLevelEveryEighthPixel(t *testing.T) {
	whole, tiled, _ := cut(t)
	// a pixel of the overview every 32 m
	for x := float32(0); x <= 384; x += 32 {
		for y := float32(0); y <= 384; y += 32 {
			got, want := tiled.HeightAtTier(x, y, mapfile.Overview), whole.HeightAt(x, y)
			if got != want {
				t.Fatalf("the overview at (%g, %g) is %g, the level %g", x, y, got, want)
			}
		}
	}
	// and even between them, where the level has its own pixels
	if tiled.HeightAtTier(16, 16, mapfile.Overview) == whole.HeightAt(16, 16) {
		t.Error("the overview holds the level's own pixels between its own")
	}
	far, err := tiled.Within(everywhere(), mapfile.Overview)
	if err != nil {
		t.Fatal(err)
	}
	if len(far) != 4 {
		t.Errorf("%d features in the overview, want the four lines of the roads", len(far))
	}
	for _, f := range far {
		if f.Tags["highway"] == "" {
			t.Errorf("the overview holds %v, 30 m across", f.Tags)
		}
	}
}

func TestAFeatureByASeamIsBothTilesAndComesOnce(t *testing.T) {
	// a bench 2 m past the seam at 128 and a wall across it
	tiled, dir := cutOf(t, written(t, nil,
		`{"type":"Feature","id":"bench","properties":{"amenity":"bench"},`+
			`"geometry":{"type":"Point","coordinates":[130,300]}}`,
		`{"type":"Feature","properties":{"barrier":"wall"},`+
			`"geometry":{"type":"LineString","coordinates":[[100,330],[150,330]]}}`,
	))
	for _, tile := range []string{"0/0_0", "0/1_0"} {
		raw, err := os.ReadFile(filepath.Join(dir, tile, "features.geojson"))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Contains(raw, []byte(`"bench"`)) || !bytes.Contains(raw, []byte(`"f1"`)) {
			t.Errorf("the tile %s holds\n%s\nwant the bench and the wall, named f1", tile, raw)
		}
	}
	// asked of the first tile alone, within the margin of the seam
	west := geom.NewAABB(geom.NewVec(0, 0), geom.NewVec(120, 120))
	got, err := tiled.Within(west, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Errorf("%d features by the first tile, want the bench and the wall", len(got))
	}
	if got, _ = tiled.Within(everywhere(), 0); len(got) != 2 {
		t.Errorf("%d features over the level, want each once", len(got))
	}
}

func area(path []geom.Vec, box geom.AABB) float64 {
	var sum float64
	for k, v := range path {
		w := path[(k+1)%len(path)]
		ax := float64(min(max(v.X, box.TopLeft.X), box.BottomRight.X))
		ay := float64(min(max(v.Y, box.TopLeft.Y), box.BottomRight.Y))
		bx := float64(min(max(w.X, box.TopLeft.X), box.BottomRight.X))
		by := float64(min(max(w.Y, box.TopLeft.Y), box.BottomRight.Y))
		sum += ax*by - bx*ay
	}
	if sum < 0 {
		sum = -sum
	}
	return sum / 2
}

func TestAnAreaWiderThanATileIsCutToItsTiles(t *testing.T) {
	// a field 300 m by 160 m and a pond 30 m across
	tiled, _ := cutOf(t, written(t, nil,
		`{"type":"Feature","id":"field","properties":{"landuse":"meadow"},"geometry":{"type":`+
			`"Polygon","coordinates":[[[40,40],[340,40],[340,200],[40,200],[40,40]]]}}`,
		`{"type":"Feature","id":"pond","properties":{"natural":"water"},"geometry":{"type":`+
			`"Polygon","coordinates":[[[110,300],[140,300],[140,330],[110,330],[110,300]]]}}`,
	))
	all, err := tiled.Within(everywhere(), 0)
	if err != nil {
		t.Fatal(err)
	}
	ponds, pieces := 0, 0
	for _, f := range all {
		switch f.ID {
		case "pond":
			ponds++
			if f.Tags["clipped"] != "" || len(f.Path) != 4 {
				t.Errorf("the pond is %+v, want it whole", f)
			}
		case "field":
			pieces++
			if f.Tags["clipped"] != "true" || f.Tags["landuse"] != "meadow" {
				t.Errorf("a piece of the field is tagged %v", f.Tags)
			}
		}
	}
	if ponds != 1 || pieces != 6 {
		t.Errorf("%d ponds and %d pieces of the field, want 1 and a piece a tile it lies in", ponds,
			pieces)
	}
	// each tile's piece within the tile: together the field
	var sum float64
	for row := range 3 {
		for col := range 3 {
			lo := geom.NewVec(float32(col)*128, float32(row)*128)
			box := geom.NewAABB(lo, geom.NewVec(lo.X+128, lo.Y+128))
			in, err := tiled.Within(geom.NewAABB(
				geom.NewVec(lo.X+1, lo.Y+1), geom.NewVec(lo.X+127, lo.Y+127)), 0)
			if err != nil {
				t.Fatal(err)
			}
			for _, f := range in {
				if f.ID == "field" {
					sum += area(f.Path, box)
				}
			}
		}
	}
	if sum < 300*160-1 || sum > 300*160+1 {
		t.Errorf("the pieces within their tiles cover %g m², the field %d", sum, 300*160)
	}
}

func TestATileOfTheSeaIsNotWritten(t *testing.T) {
	// 5 m under zero but for a hill of 10 m in the first tile
	img := image.NewGray16(image.Rect(0, 0, 97, 97))
	for j := range 97 {
		for i := range 97 {
			v := uint16(500)
			if i >= 5 && i <= 25 && j >= 5 && j <= 25 {
				v = 2000
			}
			img.SetGray16(i, j, color.Gray16{Y: v})
		}
	}
	tiled, dir := cutOf(t, written(t, img))
	tiles, err := os.ReadDir(filepath.Join(dir, "0"))
	if err != nil {
		t.Fatal(err)
	}
	if len(tiles) != 1 || tiles[0].Name() != "0_0" {
		t.Errorf("the tiles written are %v, want the hill's alone", tiles)
	}
	if h := tiled.HeightAt(60, 60); h != 10 {
		t.Errorf("the hill is %g high, want 10", h)
	}
	if h := tiled.HeightAt(300, 300); h != 0 {
		t.Errorf("the sea is %g high, want 0", h)
	}
}

func TestACutSaysWhatItCannotDo(t *testing.T) {
	source := os.DirFS("testdata")
	for _, side := range []float32{100, 0, 2} {
		c := mapfile.Cutting{Tile: side, Margin: 32}
		if err := mapfile.Cut(source, "level/level.yaml", t.TempDir(), c); err == nil {
			t.Errorf("a level was cut into tiles %g a side", side)
		}
	}
	_, _, dir := cut(t)
	c := mapfile.Cutting{Tile: 128, Margin: 32}
	if err := mapfile.Cut(os.DirFS(dir), "level.yaml", t.TempDir(), c); err == nil {
		t.Error("a level in tiles was cut again")
	}
	if err := os.RemoveAll(filepath.Join(dir, "3")); err != nil {
		t.Fatal(err)
	}
	if _, err := mapfile.Load(os.DirFS(dir), "level.yaml"); err == nil {
		t.Error("a level was read whose manifest names a tier it has not")
	}
}
