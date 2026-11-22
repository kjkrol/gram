package mapfile

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image/png"
	"io/fs"
	"math"
	"os"
	"path/filepath"

	"github.com/kjkrol/aabbworld/geom"
	"go.yaml.in/yaml/v3"
)

// clippedTag marks a piece of an area wider than a tile, cut to its tile and the margin
const clippedTag = "clipped"

// Cutting is how a level is cut into tiles.
type Cutting struct {
	// Tile is a tile's side in metres, a power of two; Margin how far past a tile a feature
	// may lie and still be the tile's.
	Tile   float32
	Margin float32
	// Overview says whether a feature of these tags, extent metres across, is drawn from far
	// off: it lies in the overview tier too.
	Overview func(tags map[string]string, extent float32) bool
}

// Cut writes the level whose manifest is name in fsys, one in one piece, into the folder out
// as tiles of two tiers: the level as it is and its overview.
func Cut(fsys fs.FS, name, out string, c Cutting) error {
	l, err := Load(fsys, name)
	if err != nil {
		return err
	}
	if l.tiles != nil {
		return fmt.Errorf("board: %s is in tiles already", name)
	}
	if k := math.Log2(float64(c.Tile)); c.Tile <= 0 || k != math.Trunc(k) {
		return fmt.Errorf("board: a tile's side of %g is no power of two", c.Tile)
	}
	if h := l.Heights; h != nil {
		if n := float64(c.Tile / h.Pixel); n < 1 || n != math.Trunc(n) {
			return fmt.Errorf("board: a tile of %g is no whole number of pixels of %g",
				c.Tile, h.Pixel)
		}
	}
	for i := range l.Features {
		if l.Features[i].ID == "" {
			l.Features[i].ID = fmt.Sprintf("f%d", i)
		}
	}
	t := &tiling{side: c.Tile, margin: c.Margin, tiers: []int{0, Overview}}
	for _, tier := range t.tiers {
		if err := os.MkdirAll(filepath.Join(out, fmt.Sprint(tier)), 0o755); err != nil {
			return err
		}
		cols, rows := t.across(tier, l.Width, l.Height)
		for row := range rows {
			for col := range cols {
				if err := l.cutTile(t, tileKey{tier, col, row}, out, c); err != nil {
					return err
				}
			}
		}
	}
	raw, err := fs.ReadFile(fsys, name)
	if err != nil {
		return err
	}
	text, err := tiledManifest(raw, t)
	if err != nil {
		return fmt.Errorf("board: %s: %w", name, err)
	}
	return os.WriteFile(filepath.Join(out, filepath.Base(name)), text, 0o644)
}

func (l *File) cutTile(t *tiling, k tileKey, out string, c Cutting) error {
	side := t.sideOf(k.tier)
	lo := geom.NewVec(float32(k.col)*side, float32(k.row)*side)
	hi := geom.NewVec(min(lo.X+side, l.Width), min(lo.Y+side, l.Height))
	grown := geom.NewAABB(
		geom.NewVec(lo.X-t.margin, lo.Y-t.margin),
		geom.NewVec(hi.X+t.margin, hi.Y+t.margin),
	)
	var lying []Feature
	for _, f := range l.Features {
		extent := extentOf(f.Path)
		if k.tier > 0 && (c.Overview == nil || !c.Overview(f.Tags, extent)) {
			continue
		}
		if piece, ok := pieceIn(f, extent, side, grown); ok {
			lying = append(lying, piece)
		}
	}
	var high *Heights
	if h := l.Heights; h != nil {
		every := 1 << k.tier
		per := int(math.Round(float64(side / h.Pixel)))
		i0, j0 := k.col*per, k.row*per
		i1, j1 := min(i0+per, h.Columns-1), min(j0+per, h.Rows-1)
		if i1-i0 >= every && j1-j0 >= every {
			high = h.cut(i0, j0, i1, j1, every)
		}
	}
	if len(lying) == 0 && (high == nil || !high.land()) {
		return nil
	}
	dir := filepath.Join(out, fmt.Sprint(k.tier), fmt.Sprintf("%d_%d", k.col, k.row))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if high != nil {
		var buf bytes.Buffer
		if err := png.Encode(&buf, high.image()); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dir, "heights.png"), buf.Bytes(), 0o644); err != nil {
			return err
		}
	}
	if len(lying) == 0 {
		return nil
	}
	return os.WriteFile(filepath.Join(dir, "features.geojson"), geoJSON(lying, l.Height), 0o644)
}

func extentOf(path []geom.Vec) float32 {
	lo, hi := path[0], path[0]
	for _, v := range path {
		lo = geom.NewVec(min(lo.X, v.X), min(lo.Y, v.Y))
		hi = geom.NewVec(max(hi.X, v.X), max(hi.Y, v.Y))
	}
	return max(hi.X-lo.X, hi.Y-lo.Y)
}

// pieceIn is the feature as a tile holds it, if it does: whole where it touches the tile's
// grown box, an area wider than a tile cut to that box
func pieceIn(f Feature, extent, side float32, box geom.AABB) (Feature, bool) {
	switch f.Shape {
	case Point:
		return f, inside(f.Path[0], box)
	case Line:
		for k := 1; k < len(f.Path); k++ {
			if crosses(f.Path[k-1], f.Path[k], box) {
				return f, true
			}
		}
		return f, false
	}
	if extent <= side {
		lo, hi := f.Path[0], f.Path[0]
		for _, v := range f.Path {
			lo = geom.NewVec(min(lo.X, v.X), min(lo.Y, v.Y))
			hi = geom.NewVec(max(hi.X, v.X), max(hi.Y, v.Y))
		}
		return f, lo.X <= box.BottomRight.X && hi.X >= box.TopLeft.X &&
			lo.Y <= box.BottomRight.Y && hi.Y >= box.TopLeft.Y
	}
	piece := Feature{ID: f.ID, Shape: Area, Tags: map[string]string{clippedTag: "true"}}
	for name, v := range f.Tags {
		piece.Tags[name] = v
	}
	if piece.Path = clip(f.Path, box); len(piece.Path) < 3 {
		return f, false
	}
	for _, hole := range f.Holes {
		if cut := clip(hole, box); len(cut) >= 3 {
			piece.Holes = append(piece.Holes, cut)
		}
	}
	return piece, true
}

func inside(v geom.Vec, box geom.AABB) bool {
	return v.X >= box.TopLeft.X && v.X <= box.BottomRight.X &&
		v.Y >= box.TopLeft.Y && v.Y <= box.BottomRight.Y
}

// crosses is whether the stretch from a to b touches the box (Liang and Barsky's)
func crosses(a, b geom.Vec, box geom.AABB) bool {
	t0, t1 := 0.0, 1.0
	dx, dy := float64(b.X-a.X), float64(b.Y-a.Y)
	for _, e := range [4][2]float64{
		{-dx, float64(a.X - box.TopLeft.X)},
		{dx, float64(box.BottomRight.X - a.X)},
		{-dy, float64(a.Y - box.TopLeft.Y)},
		{dy, float64(box.BottomRight.Y - a.Y)},
	} {
		p, q := e[0], e[1]
		if p == 0 {
			if q < 0 {
				return false
			}
			continue
		}
		if r := q / p; p < 0 {
			t0 = max(t0, r)
		} else {
			t1 = min(t1, r)
		}
		if t0 > t1 {
			return false
		}
	}
	return true
}

// clip is an outline cut to a box (Sutherland and Hodgman's)
func clip(outline []geom.Vec, box geom.AABB) []geom.Vec {
	type edge struct {
		in func(geom.Vec) bool
		at func(a, b geom.Vec) geom.Vec
	}
	onX := func(x float32) func(a, b geom.Vec) geom.Vec {
		return func(a, b geom.Vec) geom.Vec {
			return geom.NewVec(x, a.Y+(b.Y-a.Y)*(x-a.X)/(b.X-a.X))
		}
	}
	onY := func(y float32) func(a, b geom.Vec) geom.Vec {
		return func(a, b geom.Vec) geom.Vec {
			return geom.NewVec(a.X+(b.X-a.X)*(y-a.Y)/(b.Y-a.Y), y)
		}
	}
	lo, hi := box.TopLeft, box.BottomRight
	out := outline
	for _, e := range []edge{
		{func(v geom.Vec) bool { return v.X >= lo.X }, onX(lo.X)},
		{func(v geom.Vec) bool { return v.X <= hi.X }, onX(hi.X)},
		{func(v geom.Vec) bool { return v.Y >= lo.Y }, onY(lo.Y)},
		{func(v geom.Vec) bool { return v.Y <= hi.Y }, onY(hi.Y)},
	} {
		in := out
		out = nil
		for k, v := range in {
			prev := in[(k+len(in)-1)%len(in)]
			switch {
			case e.in(v) && e.in(prev):
				out = append(out, v)
			case e.in(v):
				out = append(out, e.at(prev, v), v)
			case e.in(prev):
				out = append(out, e.at(prev, v))
			}
		}
		if len(out) == 0 {
			return nil
		}
	}
	return out
}

// geoJSON writes features as a collection, their places turned back north up
func geoJSON(features []Feature, height float32) []byte {
	at := func(v geom.Vec) [2]float64 {
		return [2]float64{round(v.X), round(height - v.Y)}
	}
	ring := func(path []geom.Vec, closed bool) [][2]float64 {
		out := make([][2]float64, 0, len(path)+1)
		for _, v := range path {
			out = append(out, at(v))
		}
		if closed {
			out = append(out, at(path[0]))
		}
		return out
	}
	var b bytes.Buffer
	b.WriteString("{\n\"type\": \"FeatureCollection\",\n\"features\": [\n")
	for i, f := range features {
		var kind string
		var places any
		switch f.Shape {
		case Point:
			kind, places = "Point", at(f.Path[0])
		case Line:
			kind, places = "LineString", ring(f.Path, false)
		default:
			rings := [][][2]float64{ring(f.Path, true)}
			for _, hole := range f.Holes {
				rings = append(rings, ring(hole, true))
			}
			kind, places = "Polygon", rings
		}
		line, err := json.Marshal(map[string]any{
			"type":       "Feature",
			"id":         f.ID,
			"properties": f.Tags,
			"geometry":   map[string]any{"type": kind, "coordinates": places},
		})
		if err != nil {
			panic(err)
		}
		b.Write(line)
		if i < len(features)-1 {
			b.WriteByte(',')
		}
		b.WriteByte('\n')
	}
	b.WriteString("]\n}\n")
	return b.Bytes()
}

// a place to a millimetre
func round(v float32) float64 { return math.Round(float64(v)*1000) / 1000 }

// tiledManifest is a manifest with the files it names taken out and its tiles said
func tiledManifest(raw []byte, t *tiling) ([]byte, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return nil, err
	}
	if len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
		return nil, fmt.Errorf("no mapping")
	}
	top := doc.Content[0]
	without := func(m *yaml.Node, key string) {
		for i := 0; i+1 < len(m.Content); i += 2 {
			if m.Content[i].Value == key {
				m.Content = append(m.Content[:i], m.Content[i+2:]...)
				return
			}
		}
	}
	without(top, "features")
	without(top, "tiles")
	for i := 0; i+1 < len(top.Content); i += 2 {
		if top.Content[i].Value == "heights" {
			without(top.Content[i+1], "file")
		}
	}
	var tiles yaml.Node
	if err := tiles.Encode(map[string]any{
		"side":   t.side,
		"margin": t.margin,
		"tiers":  t.tiers,
	}); err != nil {
		return nil, err
	}
	tiles.Style = yaml.FlowStyle
	for _, c := range tiles.Content {
		c.Style = yaml.FlowStyle
	}
	key := &yaml.Node{Kind: yaml.ScalarNode, Value: "tiles",
		HeadComment: "cut into tiles: never edited by hand, cut anew from the level in one piece"}
	top.Content = append(top.Content, key, &tiles)
	var b bytes.Buffer
	enc := yaml.NewEncoder(&b)
	enc.SetIndent(2)
	if err := enc.Encode(&doc); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}
