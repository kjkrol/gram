package world

import (
	"image/color"
	"math"
	"testing"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/cameras/cameratest"
	"github.com/kjkrol/gram/plugins/world/sector"
	"github.com/kjkrol/gram/plugins/world/view"
	"github.com/kjkrol/gram/render"
)

// the insides the sectors view composes through cam, each by its left edge on the screen, in its
// colour
func insides(t *testing.T, p *Plugin, cam camera.Camera) map[int]color.RGBA {
	t.Helper()
	var f render.Frame
	f.Reset(cam)
	p.sectors.Compose(&f, cam)
	got := map[int]color.RGBA{}
	f.Each(func(_ render.At, v []render.Vertex) {
		a := v[0].ColorA
		if math.Abs(float64(a-sectorLine.Inside)) > 0.01 {
			return
		}
		of := func(c float32) uint8 { return uint8(math.Round(float64(c / a * 255))) }
		left := min(v[0].DstX, v[1].DstX, v[2].DstX, v[3].DstX)
		got[int(math.Round(float64(left)))] = color.RGBA{R: of(v[0].ColorR), G: of(v[0].ColorG),
			B: of(v[0].ColorB), A: 255}
	})
	return got
}

func TestSectorsView_LaysTheSectorsInSightInTheColoursOfTheirBandsOrOfHowTheyAreSeen(t *testing.T) {
	r := sectorWorld(t, twoBands...)
	cam := cameratest.NewFromSpace(4096, 512, 0)
	cam.SetViewport(1600, 512)
	cam.MoveTo(0, 0)
	r.p.write.NewView(cam)
	r.run()
	r.run()
	r.p.write.ShowSectors(SectorBands)
	// round the anchor at 100: two sectors in the first band, one in the second, the rest beyond
	beyond := bandColors[sector.MaxBands]
	want := map[int]color.RGBA{0: bandColors[0], 512: bandColors[0], 1024: bandColors[1],
		1536: beyond}
	if got := insides(t, r.p, cam); len(got) != len(want) || !sameColors(got, want) {
		t.Errorf("by band %v, want %v", got, want)
	}
	r.p.write.ShowSectors(SectorsSeen)
	exact, hidden := levelColors[view.Exact], levelColors[view.Hidden]
	want = map[int]color.RGBA{0: exact, 512: exact, 1024: exact, 1536: hidden}
	if got := insides(t, r.p, cam); len(got) != len(want) || !sameColors(got, want) {
		t.Errorf("as seen %v, want %v", got, want)
	}
	r.p.write.ShowSectors(NoSectors)
	var f render.Frame
	f.Reset(cam)
	if r.p.sectors.Compose(&f, cam); f.Len() != 0 {
		t.Errorf("hidden, %d pieces composed, want none", f.Len())
	}
}

func sameColors(got, want map[int]color.RGBA) bool {
	for x, w := range want {
		g, ok := got[x]
		if !ok {
			return false
		}
		for _, d := range [3]int{int(g.R) - int(w.R), int(g.G) - int(w.G), int(g.B) - int(w.B)} {
			if d < -2 || d > 2 {
				return false
			}
		}
	}
	return true
}

func TestSectorsView_LaysASectorAcrossTheSeamOnBothSides(t *testing.T) {
	p := NewPlugin(Config{
		Space:    SpaceCfg{Width: 4096, Height: 512, Edges: aabbworld.Torus},
		Entities: EntitiesCfg{MaxCount: 8, MinSize: 5, MaxSize: 20},
		Sectors:  SectorsCfg{Buckets: 2, Bands: twoBands},
	})
	cam := cameratest.NewFromSpace(4096, 512, aabbworld.Torus)
	cam.SetViewport(600, 512)
	cam.MoveTo(3800, 0)
	p.write.ShowSectors(SectorBands)
	// the last sector from 3584 shows from -216 on the screen, the first from 4096, at 296
	got := insides(t, p, cam)
	if _, ok := got[296]; !ok || len(got) != 2 {
		t.Errorf("insides by their left edges %v, want the last sector's and the first's at 296",
			got)
	}
}

func TestSectorsView_SwitchesFromTheBandsToHowTheyAreSeenToNothing(t *testing.T) {
	r := sectorWorld(t, twoBands...)
	for _, want := range []SectorsShown{SectorBands, SectorsSeen, NoSectors} {
		r.p.module.commands.Put(command.Nobody, SwitchSectors{Shown: want})
		r.run()
		if got := r.p.read.SectorsShown(); got != want {
			t.Errorf("shown %v, want %v", got, want)
		}
	}
}
