package mapfile

import (
	"fmt"
	"io/fs"
	"path"

	"github.com/kjkrol/aabbworld/geom"
	"go.yaml.in/yaml/v3"
)

// File is a level read: its ground and what lies on it, whole or in tiles read as asked for.
type File struct {
	// Width and Height are the level's size, metres.
	Width  float32
	Height float32
	// Seed is what a reader scatters from where it must.
	Seed uint64
	// Heights is the ground's heights of a level in one piece; nil, a level in tiles or a
	// ground level at zero.
	Heights *Heights
	// Features is what lies on a level in one piece; a level in tiles gives its own by Within.
	Features []Feature
	// Sections is how a road of each highway lies across, Corner the radius the kerbs round
	// a junction's corners at.
	Sections map[string]Section
	Corner   float32
	parts    map[string]yaml.Node
	tiles    *tiling
}

// Section is a road across: its lanes, and a pavement and a verge either side.
type Section struct {
	Lanes int     `yaml:"lanes"`
	Lane  float32 `yaml:"lane"`
	Walk  float32 `yaml:"walk"`
	Kerb  float32 `yaml:"kerb"`
	Verge float32 `yaml:"verge"`
}

type manifest struct {
	Size    [2]float32 `yaml:"size"`
	Seed    uint64     `yaml:"seed"`
	Corner  float32    `yaml:"corner"`
	Heights struct {
		File   string  `yaml:"file"`
		Pixel  float32 `yaml:"pixel"`
		Lowest float32 `yaml:"lowest"`
		Step   float32 `yaml:"step"`
	} `yaml:"heights"`
	Features string             `yaml:"features"`
	Roads    map[string]Section `yaml:"roads"`
	Tiles    *struct {
		Side   float32 `yaml:"side"`
		Margin float32 `yaml:"margin"`
		Tiers  []int   `yaml:"tiers"`
	} `yaml:"tiles"`
	Parts map[string]yaml.Node `yaml:",inline"`
}

// DefaultCorner is the radius of a junction's corners where the manifest sets none.
const DefaultCorner = 8

// Load is the level whose manifest is name in fsys, the files it names beside it; of a level
// in tiles nothing but the manifest is read yet.
func Load(fsys fs.FS, name string) (*File, error) {
	raw, err := fs.ReadFile(fsys, name)
	if err != nil {
		return nil, err
	}
	var m manifest
	if err := yaml.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("board: %s: %w", name, err)
	}
	if m.Size[0] <= 0 || m.Size[1] <= 0 {
		return nil, fmt.Errorf("board: %s: no size", name)
	}
	l := &File{
		Width:    m.Size[0],
		Height:   m.Size[1],
		Seed:     m.Seed,
		Sections: m.Roads,
		Corner:   m.Corner,
		parts:    m.Parts,
	}
	if l.Corner == 0 {
		l.Corner = DefaultCorner
	}
	dir := path.Dir(name)
	if m.Tiles != nil {
		l.tiles, err = newTiling(fsys, dir, name, &m)
		return l, err
	}
	if m.Heights.File != "" {
		file := path.Join(dir, m.Heights.File)
		l.Heights, err = readHeights(fsys, file, m.Heights.Pixel, m.Heights.Lowest, m.Heights.Step)
		if err != nil {
			return nil, err
		}
		if !l.Heights.covers(l.Width, l.Height) {
			return nil, fmt.Errorf(
				"board: %s: %dx%d pixels of %g do not cover %gx%g",
				file, l.Heights.Columns, l.Heights.Rows, l.Heights.Pixel, l.Width, l.Height,
			)
		}
	}
	if m.Features != "" {
		file := path.Join(dir, m.Features)
		if l.Features, err = readFeatures(fsys, file, l.Height); err != nil {
			return nil, err
		}
	}
	return l, nil
}

// Part decodes the manifest's part of a name, a reader's own, into what into points at, and
// says whether the manifest has one.
func (l *File) Part(name string, into any) (bool, error) {
	node, ok := l.parts[name]
	if !ok {
		return false, nil
	}
	if err := node.Decode(into); err != nil {
		return true, fmt.Errorf("board: the manifest's %s: %w", name, err)
	}
	return true, nil
}

// Grounded is whether the level has heights of its own.
func (l *File) Grounded() bool {
	return l.Heights != nil || l.tiles != nil && l.tiles.pixel > 0
}

// HeightAt is the ground's height at a place of the world.
func (l *File) HeightAt(x, y float32) float32 { return l.HeightAtTier(x, y, 0) }

// HeightAtTier is the ground's height at a place as a tier of the tiles has it; a level in
// one piece has one ground.
func (l *File) HeightAtTier(x, y float32, tier int) float32 {
	switch {
	case l.tiles != nil:
		return l.tiles.heightAt(x, y, tier, l.Width, l.Height)
	case l.Heights != nil:
		return l.Heights.At(x, y)
	}
	return 0
}

// Within is what lies on the level at a tier of the tiles where it may touch a box: all of a
// level in one piece, the features of the tiles the box touches of one in tiles, each whole
// one once.
func (l *File) Within(box geom.AABB, tier int) ([]Feature, error) {
	if l.tiles == nil {
		return l.Features, nil
	}
	return l.tiles.within(box, tier, l.Width, l.Height)
}

// Whole reads all of a level in tiles at its finest, so nothing is left to fail later.
func (l *File) Whole() error {
	if l.tiles == nil {
		return nil
	}
	all, err := l.Within(geom.NewAABB(geom.NewVec(0, 0), geom.NewVec(l.Width, l.Height)), 0)
	if err != nil {
		return err
	}
	l.Features = all
	return l.tiles.heightsOf(0, l.Width, l.Height)
}
