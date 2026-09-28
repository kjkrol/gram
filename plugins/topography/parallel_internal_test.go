package topography

import (
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/camera"
	icamera "github.com/kjkrol/gram/internal/camera"
	"github.com/kjkrol/gram/plugins/atmosphere/air"
	"github.com/kjkrol/gram/plugins/atmosphere/sky"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/render"
)

// piece is one piece of a frame as a test compares it: its tier, depth and vertices.
type piece struct {
	tier  render.Tier
	depth float32
	verts []ebiten.Vertex
}

// piecesOf composes r through cam and gives every piece of the frame in order.
func piecesOf(r *board.Renderer, cam camera.Camera) (pieces []piece, n int) {
	var f render.Frame
	f.Reset(cam)
	r.Compose(&f, cam)
	f.Each(func(tier render.Tier, depth float32, v []ebiten.Vertex) {
		pieces = append(pieces, piece{tier, depth, append([]ebiten.Vertex(nil), v...)})
	})
	return pieces, f.Len()
}

// islet is a 16 by 16 board of a hill of earth spreading over a sea lying under it, a stream
// running down the hill into the sea, under a sun and broken clouds: every way the dresser has of
// dressing a tile at once. Its dressers are made anew each: what one has read the other has not.
func islet() (*board.Board, func() *dresser) {
	st := map[board.Name]Style{}
	grid := board.DefaultGrids{}.Square(16, 16, 32)
	brd := board.NewBoard(grid, board.NewTerrainMap())
	sea := styled(st, board.CellKind{Name: board.Named("sea"), Cost: 1, Allows: board.Water, SpriteID: 1}, Style{Shine: 0.9, Under: true})
	earth := styled(st, board.CellKind{Name: board.Named("earth"), Cost: 1, Allows: board.Land, SpriteID: 2}, Style{Spread: 0.3})
	wood := styled(st, board.CellKind{Name: board.Named("wood"), Cost: 1, Allows: board.Land, SpriteID: 3, Height: 12, Sway: 1}, Style{})
	stream := styled(st, board.CellKind{Name: board.Named("stream"), Cost: 2, Allows: board.Water, SpriteID: 4}, Style{Shine: 0.9, Flow: 30})
	brd.SetAll(sea)
	grid.EachCell(func(c board.CellID) {
		x, y, _ := grid.Coords(c)
		dx, dy := float64(x)-7.5, float64(y)-7.5
		switch d := dx*dx + dy*dy; {
		case d < 9 && (x+y)%3 == 0:
			brd.Set(c, wood)
		case d < 30:
			brd.Set(c, earth)
		}
	})
	for y := uint32(7); y < 14; y++ {
		c, _ := grid.CellIndex(8, y)
		links := board.Links(1<<0 | 1<<1) // north and south
		brd.SetWay(c, board.Way{Kind: stream, Width: 8, Links: links, Fade: float32(y-7) / 8})
	}
	reliefFor(brd).SetHeights(func(p geom.Vec) float64 {
		dx, dy := p.X/32-8, p.Y/32-8
		return max(0, 40-2*(dx*dx+dy*dy))
	})
	weather := air.Weather{Clouds: 0.6, Wind: [2]float32{1, 0.5}}
	return brd, func() *dresser {
		return newDresser(brd, reliefFor(brd), testSky{sun: sky.Sun{Dir: [3]float32{1, 0.5, 0.3}, Strength: 0.7, Ambient: 0.3}, air: weather}, true, st)
	}
}

// The tiles dressed on several goroutines at once make the picture one goroutine makes: every piece
// the same, in the same order — from above, isometrically, whole and a part of the island with the
// shadows cast from beyond the screen, and through a perspective with the eye among the tiles.
func TestDresser_WorkersDressWhatOneGoroutineDoes(t *testing.T) {
	brd, dresserOf := islet()
	d, dw := dresserOf(), dresserOf()
	one := board.NewRenderer(brd, flatAtlas{}, testMap{look: boardLook{d: d}, d: d})
	one.Workers(1)
	many := board.NewRenderer(brd, flatAtlas{}, testMap{look: boardLook{d: dw}, d: dw})
	many.Workers(4)
	persp := newCamera(testProjection, 512, 512, 0, camera.Config{ViewportWidth: 400, ViewportHeight: 300}, 0, true, nil, nil, 0)
	persp.enterInside([3]float32{300, 300, 20}, 0.4, 0)
	persp.Tilt(0.2)
	iso := newCamera(testProjection, 512, 512, 0, camera.Config{ViewportWidth: 400, ViewportHeight: 300}, 0, true, nil, nil, 0)
	iso.ZoomOut(2, 200, 150)
	part := newCamera(testProjection, 512, 512, 0, camera.Config{ViewportWidth: 400, ViewportHeight: 300}, 0, true, nil, nil, 0)
	part.CenterOn(120, 300, 0) // the hill's shadows fall here from cells off the screen
	for name, cam := range map[string]camera.Camera{"above": icamera.NewFromSpace(512, 512, 0), "iso": iso, "part": part, "persp": persp} {
		want, wantN := piecesOf(one, cam)
		got, gotN := piecesOf(many, cam)
		if len(dw.workers) < 2 {
			t.Fatalf("%s: %d workers dressed the tiles, want several", name, len(dw.workers))
		}
		if len(got) != len(want) || gotN != wantN || len(want) < 128 {
			t.Fatalf("%s: %d pieces (%d counted) from several goroutines, %d (%d) from one", name, len(got), gotN, len(want), wantN)
		}
		for i := range want {
			if got[i].tier != want[i].tier || got[i].depth != want[i].depth || len(got[i].verts) != len(want[i].verts) {
				t.Fatalf("%s: piece %d is %v at %v with %d vertices, want %v at %v with %d", name, i, got[i].tier, got[i].depth, len(got[i].verts), want[i].tier, want[i].depth, len(want[i].verts))
			}
			for k := range want[i].verts {
				if got[i].verts[k] != want[i].verts[k] {
					t.Fatalf("%s: piece %d vertex %d is %+v, want %+v", name, i, k, got[i].verts[k], want[i].verts[k])
				}
			}
		}
	}
}
