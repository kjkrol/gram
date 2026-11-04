package world_test

import (
	"testing"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/lod"
	"github.com/kjkrol/gram/plugins/world"
)

// the expanse's world: a torus 16384 a side, buckets of 64 by its 100 000 boxes 5 a side, so
// sectors 512 a side
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
			Scales: [lod.Levels - 1]float32{0.5},
			Sight:  1_000,
			Fog:    300,
			Texel:  texel,
		},
	}
}

// the topography's Madeira: 640 by 320 cells 32 a side, buckets of 256 by its 512 figures up to a
// cell, so sectors 512 a side
func topographyConfig() world.Config {
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
			Scales: [lod.Levels - 1]float32{0.5, 0.25, 0.12},
			Sight:  30_000,
			Fog:    6_000,
		},
	}
}

func TestAggregates_MakesTheImageOverTheWorldInTexelsOfTheLadderAtMost2048ASide(t *testing.T) {
	for _, c := range []struct {
		name       string
		cfg        world.Config
		bucket     uint32
		cols, rows int
		perUnit    float32
	}{
		{"expanse", expanseConfig(0), 64, 2048, 2048, 1.0 / 8},
		{"expanse at a texel of 16", expanseConfig(16), 64, 1024, 1024, 1.0 / 16},
		{"expanse at a texel of 4", expanseConfig(4), 64, 2048, 2048, 1.0 / 8},
		{"topography", topographyConfig(), 256, 2048, 1024, 1.0 / 10},
	} {
		t.Run(c.name, func(t *testing.T) {
			w := world.NewPlugin(c.cfg)
			if b := w.Write().Space().BucketSize; b != c.bucket {
				t.Fatalf("a bucket of %d, want %d", b, c.bucket)
			}
			if err := w.Install(&installCtx{ecs: goke.New()}); err != nil {
				t.Fatal(err)
			}
			if img, _ := w.Read(0).Aggregate(); img != nil {
				t.Error("an aggregate no renderer asked for, want none")
			}
			w.Aggregates()
			img, held := w.Read(0).Aggregate()
			if img == nil {
				t.Fatal("no aggregate in a world cut into sectors")
			}
			b, perUnit := img.Bounds(), w.AggregateTexels()
			if b.Dx() != c.cols || b.Dy() != c.rows || perUnit != c.perUnit || held != 0 {
				t.Errorf("an aggregate of %dx%d texels, %v a world unit, read as %v holding no "+
					"colour; want %dx%d, %v, 0", b.Dx(), b.Dy(), perUnit, held, c.cols, c.rows,
					c.perUnit)
			}
		})
	}
}

func TestAggregate_NoneWithoutSectors(t *testing.T) {
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 400, Height: 400},
		Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 10, MaxSize: 10},
	})
	if err := w.Install(&installCtx{ecs: goke.New()}); err != nil {
		t.Fatal(err)
	}
	w.Aggregates()
	if img, perUnit := w.Read(0).Aggregate(); img != nil || perUnit != 0 {
		t.Errorf("an aggregate %v, %v a world unit in a world of no sectors; want none", img,
			perUnit)
	}
}
