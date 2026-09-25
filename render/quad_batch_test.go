package render

import (
	"github.com/kjkrol/aabbworld"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/kjkrol/gram/camera"
)

type fakeAtlasSource struct{}

func (fakeAtlasSource) Atlas() *ebiten.Image { return nil }
func (fakeAtlasSource) UV(SpriteID) (float32, float32, float32, float32) {
	return 0, 0, 1, 1
}

func TestQuadBatch_AppendQuad_ConsistentAcrossWrapSeam(t *testing.T) {
	cam := camera.NewFromSpace(1024, 1024, aabbworld.Torus)
	cam.Translate(1000, 0)

	batch := NewQuadBatch(fakeAtlasSource{})
	batch.Reset(cam)
	batch.AppendQuad(998, 0, 1010, 10, 0)

	if len(batch.vertices) != 8 {
		t.Fatalf("len(vertices) = %d, want 8 (two split quads)", len(batch.vertices))
	}
	firstWidth := batch.vertices[1].DstX - batch.vertices[0].DstX
	secondWidth := batch.vertices[5].DstX - batch.vertices[4].DstX
	if firstWidth+secondWidth != 12 {
		t.Errorf("total quad width straddling the wrap seam = %v, want 12 (must not stretch or drop part of it)", firstWidth+secondWidth)
	}
}

func TestQuadBatch_IndicesRestartPerChunk(t *testing.T) {
	cam := camera.NewFromSpace(100000, 100, 0)
	batch := NewQuadBatch(fakeAtlasSource{})
	batch.Reset(cam)
	quads := chunkVertices/4 + 5
	for i := range quads {
		x := float32(i)
		batch.AppendQuad(x, 0, x+1, 1, 0)
	}
	if len(batch.vertices) != quads*4 || len(batch.indices) != quads*6 {
		t.Fatalf("%d vertices and %d indices for %d quads", len(batch.vertices), len(batch.indices), quads)
	}
	first := batch.indices[chunkVertices/4*6]
	if first != 0 {
		t.Errorf("the first index past the chunk is %d, want 0 (relative to its own DrawTriangles call)", first)
	}
	for i, idx := range batch.indices {
		if int(idx) >= chunkVertices {
			t.Fatalf("index %d = %d reaches past a chunk", i, idx)
		}
	}
}

func TestQuadBatch_AppendCornersTakesTheScreenPointsAsGiven(t *testing.T) {
	batch := NewQuadBatch(fakeAtlasSource{})
	batch.Reset(camera.NewFromSpace(1024, 1024, 0))
	batch.AppendCorners(Corners{{10, 0}, {20, 5}, {0, 15}, {10, 20}}, 0)
	sx0, _, sx1, _ := fakeAtlasSource{}.UV(0)
	if len(batch.vertices) != 4 || batch.vertices[1].DstX != 20 || batch.vertices[2].DstY != 15 || batch.vertices[0].SrcX != sx0+0.5 || batch.vertices[3].SrcX != sx1-0.5 {
		t.Errorf("vertices %+v, want the four corners as given sampling half a texel inside the sprite", batch.vertices)
	}
}

func TestBillboard_StandsOnTheProjectedPoint(t *testing.T) {
	cam := camera.NewFromSpaceWithConfig(640, 640, 0, camera.Config{ViewportWidth: 400, ViewportHeight: 300, Projection: camera.Isometric{Cell: 32}})
	sx, sy := cam.Project(100, 100, 5)
	c := Billboard(cam, 100, 100, 5, 20, 30)
	if c[2][1] != sy || c[3][1] != sy || c[0][1] != sy-30 || c[1][0]-c[0][0] != 20 || (c[0][0]+c[1][0])/2 != sx {
		t.Errorf("billboard %v, want 20 wide and 30 tall with its bottom edge centred on (%v, %v)", c, sx, sy)
	}
	if p := ProjectCorners(cam, 0, 0, 32, 32, 0); p[0][0] != p[3][0] || p[1][1] != p[2][1] {
		t.Errorf("a cell projects to %v, want a diamond: top over bottom, left level with right", p)
	}
}
