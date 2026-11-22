package mapfile

import (
	"errors"
	"fmt"
	"io/fs"
	"math"
	"path"
	"sync"

	"github.com/kjkrol/aabbworld/geom"
)

// Overview is the tier of the tiles that is drawn from far off: tiles eight times the side,
// heights every eighth pixel.
const Overview = 3

type tileKey struct{ tier, col, row int }

// tiling is a level in tiles: what is read of it so far, a tile absent being the sea
type tiling struct {
	fsys                fs.FS
	dir                 string
	side, margin        float32
	tiers               []int
	pixel, lowest, step float32
	mu                  sync.Mutex
	heights             map[tileKey]*Heights
	features            map[tileKey][]Feature
}

func newTiling(fsys fs.FS, dir, name string, m *manifest) (*tiling, error) {
	t := &tiling{
		fsys: fsys, dir: dir, side: m.Tiles.Side, margin: m.Tiles.Margin, tiers: m.Tiles.Tiers,
		pixel: m.Heights.Pixel, lowest: m.Heights.Lowest, step: m.Heights.Step,
		heights: map[tileKey]*Heights{}, features: map[tileKey][]Feature{},
	}
	if t.side <= 0 || len(t.tiers) == 0 {
		return nil, fmt.Errorf("board: %s: its tiles have no side or no tier", name)
	}
	for _, tier := range t.tiers {
		info, err := fs.Stat(fsys, path.Join(dir, fmt.Sprint(tier)))
		if err != nil || !info.IsDir() {
			return nil, fmt.Errorf("board: %s: no folder of the tier %d", name, tier)
		}
	}
	return t, nil
}

func (t *tiling) has(tier int) bool {
	for _, k := range t.tiers {
		if k == tier {
			return true
		}
	}
	return false
}

func (t *tiling) sideOf(tier int) float32 { return t.side * float32(int(1)<<tier) }

func (t *tiling) across(tier int, width, height float32) (cols, rows int) {
	side := float64(t.sideOf(tier))
	return int(math.Ceil(float64(width) / side)), int(math.Ceil(float64(height) / side))
}

func (t *tiling) file(k tileKey, name string) string {
	return path.Join(t.dir, fmt.Sprint(k.tier), fmt.Sprintf("%d_%d", k.col, k.row), name)
}

// tile is a tile's heights, nil where the tile is the sea
func (t *tiling) tile(k tileKey) (*Heights, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if h, ok := t.heights[k]; ok {
		return h, nil
	}
	pixel := t.pixel * float32(int(1)<<k.tier)
	h, err := readHeights(t.fsys, t.file(k, "heights.png"), pixel, t.lowest, t.step)
	if errors.Is(err, fs.ErrNotExist) {
		h, err = nil, nil
	}
	if err != nil {
		return nil, err
	}
	t.heights[k] = h
	return h, nil
}

func (t *tiling) heightAt(x, y float32, tier int, width, height float32) float32 {
	if t.pixel <= 0 {
		return 0
	}
	if !t.has(tier) {
		panic(fmt.Sprintf("board: the level has no tier %d", tier))
	}
	x, y = min(max(x, 0), width), min(max(y, 0), height)
	side := t.sideOf(tier)
	cols, rows := t.across(tier, width, height)
	k := tileKey{tier, min(int(x/side), cols-1), min(int(y/side), rows-1)}
	h, err := t.tile(k)
	if err != nil {
		panic(err)
	}
	if h == nil {
		return 0
	}
	return h.At(x-float32(k.col)*side, y-float32(k.row)*side)
}

// heightsOf reads every tile's heights of a tier
func (t *tiling) heightsOf(tier int, width, height float32) error {
	if t.pixel <= 0 {
		return nil
	}
	cols, rows := t.across(tier, width, height)
	for row := range rows {
		for col := range cols {
			if _, err := t.tile(tileKey{tier, col, row}); err != nil {
				return err
			}
		}
	}
	return nil
}

func (t *tiling) lying(k tileKey, height float32) ([]Feature, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if f, ok := t.features[k]; ok {
		return f, nil
	}
	f, err := readFeatures(t.fsys, t.file(k, "features.geojson"), height)
	if errors.Is(err, fs.ErrNotExist) {
		f, err = nil, nil
	}
	if err != nil {
		return nil, err
	}
	t.features[k] = f
	return f, nil
}

func (t *tiling) within(box geom.AABB, tier int, width, height float32) ([]Feature, error) {
	if !t.has(tier) {
		return nil, fmt.Errorf("board: the level has no tier %d", tier)
	}
	side := t.sideOf(tier)
	cols, rows := t.across(tier, width, height)
	c0, c1 := max(int(box.TopLeft.X/side), 0), min(int(box.BottomRight.X/side), cols-1)
	r0, r1 := max(int(box.TopLeft.Y/side), 0), min(int(box.BottomRight.Y/side), rows-1)
	var out []Feature
	seen := map[string]bool{}
	for row := r0; row <= r1; row++ {
		for col := c0; col <= c1; col++ {
			all, err := t.lying(tileKey{tier, col, row}, height)
			if err != nil {
				return nil, err
			}
			for _, f := range all {
				// a clipped area's pieces are each its tile's own
				if f.Tags[clippedTag] == "" {
					if seen[f.ID] {
						continue
					}
					seen[f.ID] = true
				}
				out = append(out, f)
			}
		}
	}
	return out, nil
}
