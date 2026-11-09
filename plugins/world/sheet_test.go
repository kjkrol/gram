package world_test

import (
	"testing"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/lod"
	"github.com/kjkrol/gram/plugins/world"
)

// the expanse's world: a torus 16384 a side, buckets of 64 by its 100 000 boxes 5 a side, so
// sectors 512 a side, L4 met by any scale under L3's threshold
func expanseConfig(texel float32) world.Config {
	return world.Config{
		Space: world.SpaceCfg{Width: 1 << 14, Height: 1 << 14, Edges: aabbworld.Torus},
		Entities: world.EntitiesCfg{
			MaxCount: 100_000,
			MinSize:  5,
			MaxSize:  5,
		},
		Sectors: world.SectorsCfg{Buckets: 8},
		LOD: lod.Ladder{
			Scales: [lod.Levels - 1]float32{0, 0, 0.5},
			Sight:  1_000,
			Fog:    300,
			Texel:  texel,
		},
	}
}

// the topography's Madeira: 640 by 320 cells 32 a side, buckets of 256 by its 512 figures up to a
// cell, so sectors 512 a side, L4 from l4 pixels a unit
func topographyConfig(l4 float32) world.Config {
	return world.Config{
		Space: world.SpaceCfg{Width: 640 * 32, Height: 320 * 32},
		Entities: world.EntitiesCfg{
			MaxCount: 512,
			MinSize:  2,
			MaxSize:  32,
		},
		Heights: true,
		Sectors: world.SectorsCfg{Buckets: 2},
		LOD: lod.Ladder{
			Scales: [lod.Levels - 1]float32{0, 0, 1, 0.5, l4},
			Sight:  30_000,
			Fog:    6_000,
		},
	}
}

func TestSheets_MakesTheImagesOverTheWorldInTexelsOfTheLadderAtMost4096ASide(t *testing.T) {
	for _, c := range []struct {
		name       string
		cfg        world.Config
		cols, rows int
		perUnit    float32
	}{
		{"a sector's side over 64 without L4's threshold", expanseConfig(0), 2048, 2048, 1.0 / 8},
		{"a texel of 16", expanseConfig(16), 1024, 1024, 1.0 / 16},
		{"a texel of 2 raised to 4", expanseConfig(2), 4096, 4096, 1.0 / 4},
		{"a pixel at L4's threshold", topographyConfig(0.125), 2560, 1280, 1.0 / 8},
		{"a pixel at L4's threshold raised", topographyConfig(0.25), 4096, 2048, 1.0 / 5},
	} {
		t.Run(c.name, func(t *testing.T) {
			w := world.NewPlugin(c.cfg)
			if err := w.Install(&installCtx{ecs: goke.New()}); err != nil {
				t.Fatal(err)
			}
			if albedo, relief, _, _, _ := w.Read(0).Sheet(); albedo != nil || relief != nil {
				t.Error("sheets no renderer asked for, want none")
			}
			w.Sheets()
			albedo, relief, albedoL5, reliefL5, held := w.Read(0).Sheet()
			if albedo == nil || relief == nil || albedoL5 == nil || reliefL5 == nil {
				t.Fatal("no sheets in a world cut into sectors")
			}
			for _, img := range []struct {
				name       string
				cols, rows int
				dx, dy     int
			}{
				{"albedo", c.cols, c.rows, albedo.Bounds().Dx(), albedo.Bounds().Dy()},
				{"relief", c.cols, c.rows, relief.Bounds().Dx(), relief.Bounds().Dy()},
				{"albedo at L5", c.cols / 2, c.rows / 2, albedoL5.Bounds().Dx(),
					albedoL5.Bounds().Dy()},
				{"relief at L5", c.cols / 2, c.rows / 2, reliefL5.Bounds().Dx(),
					reliefL5.Bounds().Dy()},
			} {
				if img.dx != img.cols || img.dy != img.rows {
					t.Errorf("the %s %dx%d texels, want %dx%d", img.name, img.dx, img.dy,
						img.cols, img.rows)
				}
			}
			if perUnit := w.SheetTexels(); perUnit != c.perUnit || held != 0 {
				t.Errorf("%v texels a world unit, read as %v holding no photograph; want %v, 0",
					perUnit, held, c.perUnit)
			}
			if w.Read(0).Painted(0) != 0 {
				t.Error("a sector painted before any photograph, want none")
			}
		})
	}
}

func TestSheets_NoneWithoutSectors(t *testing.T) {
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 400, Height: 400},
		Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 10, MaxSize: 10},
	})
	if err := w.Install(&installCtx{ecs: goke.New()}); err != nil {
		t.Fatal(err)
	}
	w.Sheets()
	if albedo, _, _, _, perUnit := w.Read(0).Sheet(); albedo != nil || perUnit != 0 {
		t.Errorf("a sheet %v, %v a world unit in a world of no sectors; want none", albedo,
			perUnit)
	}
	if w.Read(0).Painted(0) != 0 {
		t.Error("a sector painted in a world of no sectors")
	}
}
