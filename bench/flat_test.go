package bench_test

import (
	"image/color"
	"testing"

	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/look"
	"github.com/kjkrol/gram/plugins/cameras/cameratest"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/render/gputest"
)

// the flat board's frames: 96x64 cells of 32, as the island
const flatCols, flatRows, flatCell = 96, 64, 32

// Benchmark_Frame_Flat draws a flat board of three kinds of ground, roads across it and its grid
// through the board's renderer on a headless GPU into a 1440p screen, read back every frame so
// the GPU is waited for: from above, from above with a cell changed every frame, and in
// isometry.
func Benchmark_Frame_Flat(b *testing.B) {
	gputest.Need(b)
	for _, sc := range []struct {
		name    string
		iso     bool
		changed bool
	}{
		{"above", false, false},
		{"above,changed", false, true},
		{"iso", true, false},
	} {
		b.Run(sc.name, func(b *testing.B) {
			g := grid.DefaultGrids{}.Square(flatCols, flatRows, flatCell)
			brd, brdW := board.NewBoard(g)
			kinds := flatKinds(brdW, g)
			space := world.SpaceCfg{Width: flatCols * flatCell, Height: flatRows * flatCell}
			r := look.NewRenderer(brd, flatAtlas(kinds), brd.Map(), space)
			*r.State() = look.RenderState{ShowGridLines: true,
				GridLine: color.RGBA{A: 90}, GridWidth: 1}
			var cam camera.Camera = cameratestAbove(space)
			if sc.iso {
				cam = cameratest.NewFromAbove(space.Width, space.Height)
				cameratest.Switch(cam)
			}
			cam.SetViewport(frameW, frameH)
			cam.CenterOn(float32(space.Width)/2, float32(space.Height)/2, 0)
			screen := render.NewImage(frameW, frameH)
			buf := make([]byte, 4*frameW*frameH)
			composer := render.NewComposer(r)
			// the first frame builds the shaders and lays the board, as a game's first does
			composer.DrawWorld(screen, cam)
			render.Submit()
			screen.ReadPixels(buf)
			k := 0
			b.ReportAllocs()
			for b.Loop() {
				if sc.changed {
					brdW.Set(g.At(uint32(k%flatCols), 7), kinds[k%2])
					k++
				}
				screen.Clear()
				composer.DrawWorld(screen, cam)
				render.Submit()
				screen.ReadPixels(buf)
			}
		})
	}
}

func cameratestAbove(space world.SpaceCfg) camera.Camera {
	return cameratest.NewFromSpace(space.Width, space.Height, space.Edges)
}

// flatKinds lays grass with lakes and sand round them and roads every 12 cells across both ways
func flatKinds(brdW *board.BoardWrite, g grid.Grid) []cell.Kind {
	grass := cell.Kind{Name: cell.Named("grass"), SpriteID: 1, Pace: 1, Allows: cell.Land}
	sand := cell.Kind{Name: cell.Named("sand"), SpriteID: 2, Pace: 1, Allows: cell.Land}
	water := cell.Kind{Name: cell.Named("water"), SpriteID: 3, Pace: 1, Allows: cell.Water}
	road := cell.Kind{Name: cell.Named("road"), SpriteID: 4, Pace: 1, Allows: cell.Land}
	brdW.SetAll(grass)
	for y := range uint32(flatRows) {
		for x := range uint32(flatCols) {
			dx, dy := int(x%24)-12, int(y%20)-10
			switch d := dx*dx + dy*dy; {
			case d < 16:
				brdW.Set(g.At(x, y), water)
			case d < 30:
				brdW.Set(g.At(x, y), sand)
			}
			if x%12 == 6 || y%12 == 6 {
				brdW.SetWay(g.At(x, y), cell.Way{Kind: road, Width: 10, Links: 0xff})
			}
		}
	}
	return []cell.Kind{grass, sand, water, road}
}

// flatAtlas paints each kind's sprite at the cell's size, a speckled fill so it reads as art
func flatAtlas(kinds []cell.Kind) render.AtlasSource {
	colours := []color.RGBA{{70, 150, 60, 255}, {210, 190, 120, 255}, {40, 80, 190, 255},
		{140, 110, 80, 255}}
	atlas := render.NewAtlas()
	for i, k := range kinds {
		c := colours[i]
		atlas.Add(k.SpriteID, flatCell, func(dst *render.Canvas, size int) {
			for y := range size {
				for x := range size {
					v := uint8((x*7 + y*13) % 9 * 4)
					dst.SetRGBA(x, y, color.RGBA{c.R - v, c.G - v, c.B - v, 255})
				}
			}
		})
	}
	atlas.Close()
	return atlas
}
