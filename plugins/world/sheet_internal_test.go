package world

import (
	"testing"

	"github.com/kjkrol/gram/lod"
)

func TestSheets_HoldAPhotographTillWhatStandsInItsSectorChanges(t *testing.T) {
	p := NewPlugin(Config{
		Space:    SpaceCfg{Width: 4096, Height: 512},
		Entities: EntitiesCfg{MaxCount: 8, MinSize: 5, MaxSize: 20},
		Sectors:  SectorsCfg{Buckets: 2},
		LOD:      lod.Ladder{Scales: [lod.Levels - 1]float32{0, 0, 0.5, 0.2, 0.1}},
	})
	s, read := &p.renderer.sheet, p.Read(0)
	if len(s.version) != p.module.sectors.grid.Len() || s.perFrame != sheetPerFrame {
		t.Fatalf("versions of %d sectors, %d photographed a frame; want of the grid's %d, %d",
			len(s.version), s.perFrame, p.module.sectors.grid.Len(), sheetPerFrame)
	}
	s.paint(1)
	if !read.Painted(1) || read.Painted(0) {
		t.Errorf("painted %v, %v; want the sector photographed alone", read.Painted(1),
			read.Painted(0))
	}
	if _, _, _, _, perUnit := read.Sheet(); perUnit != 0 {
		t.Errorf("%v texels a unit read before a renderer asked for the sheets, want 0", perUnit)
	}
	p.Sheets()
	if _, _, _, _, perUnit := read.Sheet(); perUnit != s.perUnit || perUnit == 0 {
		t.Errorf("%v texels a unit read with a photograph held, want %v", perUnit, s.perUnit)
	}
	s.bump(1)
	if read.Painted(1) {
		t.Error("painted after what stands in it changed, want photographed anew first")
	}
	s.paint(1)
	if !read.Painted(1) || s.shots != 1 {
		t.Errorf("painted %v, %d sectors holding a photograph; want painted, one", read.Painted(1),
			s.shots)
	}
	if read.Painted(-1) || read.Painted(len(s.version)) {
		t.Error("a sector off the grid painted")
	}
}
