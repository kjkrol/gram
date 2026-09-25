package render

import (
	"fmt"
	"slices"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
)

// Submitter is a WorldRenderer that can hand its quads, projected through cam, to a Sink with a
// depth each instead of drawing them, so that a Sorted layer draws several renderers back to front
// as one picture.
type Submitter interface {
	WorldRenderer
	Submit(sink *Sink, cam camera.Camera)
}

// Sink gathers projected quads with their depths for one frame of a Sorted layer.
type Sink struct {
	items []item
	verts []ebiten.Vertex
	order []int
}

// item is one quad: its depth, the sheet it samples and where its four vertices start.
type item struct {
	depth float32
	atlas *ebiten.Image
	first int
}

// Corners are a quad's screen corners: top-left, top-right, bottom-left, bottom-right.
type Corners [4][2]float32

// Quad submits the sprite id drawn over dst at depth, lit fully.
func (s *Sink) Quad(depth float32, atlas AtlasSource, id SpriteID, dst Corners) {
	s.Shaded(depth, atlas, id, dst, 1)
}

// Shaded is Quad with the colour scaled by shade (1 as drawn, 0.5 half as bright): a cliff face.
func (s *Sink) Shaded(depth float32, atlas AtlasSource, id SpriteID, dst Corners, shade float32) {
	u0, v0, u1, v1 := inset(atlas.UV(id))
	s.items = append(s.items, item{depth: depth, atlas: atlas.Atlas(), first: len(s.verts)})
	for i, c := range dst {
		su, sv := u0, v0
		if i%2 == 1 {
			su = u1
		}
		if i >= 2 {
			sv = v1
		}
		s.verts = append(s.verts, ebiten.Vertex{DstX: c[0], DstY: c[1], SrcX: su, SrcY: sv, ColorR: shade, ColorG: shade, ColorB: shade, ColorA: 1})
	}
}

func (s *Sink) reset() { s.items, s.verts, s.order = s.items[:0], s.verts[:0], s.order[:0] }

// Len is how many quads have been submitted since the frame began.
func (s *Sink) Len() int { return len(s.items) }

// Vertices are the submitted quads' vertices, four per quad in submission order — for tests.
func (s *Sink) Vertices() []ebiten.Vertex { return s.verts }

// sorted is the items' indices back to front; ties keep submission order.
func (s *Sink) sorted() []int {
	s.order = s.order[:0]
	for i := range s.items {
		s.order = append(s.order, i)
	}
	slices.SortStableFunc(s.order, func(a, b int) int {
		switch da, db := s.items[a].depth, s.items[b].depth; {
		case da < db:
			return -1
		case da > db:
			return 1
		}
		return 0
	})
	return s.order
}

// inset pulls a sprite's source rectangle in by half a texel, so the edge of a quad drawn at an
// angle never samples the neighbouring sprite of the sheet and shows a seam.
func inset(sx0, sy0, sx1, sy1 float32) (float32, float32, float32, float32) {
	return sx0 + 0.5, sy0 + 0.5, sx1 - 0.5, sy1 - 0.5
}

// Overlayer is a Submitter with something to draw over the sorted picture — grid lines, say —
// which Sorted calls after drawing it.
type Overlayer interface {
	Overlay(screen *ebiten.Image, cam camera.Camera)
}

// Sorted is a WorldRenderer drawing several Submitters as one picture, back to front by depth: the
// terrain and the entities of an isometric view, where a wall in front hides a unit behind it.
// It draws one DrawTriangles per run of quads sharing a sheet, so alternating sheets cost calls.
type Sorted struct {
	subs    []Submitter
	sink    Sink
	verts   []ebiten.Vertex
	indices []uint16
	opts    *ebiten.DrawTrianglesOptions
}

var _ WorldRenderer = (*Sorted)(nil)

// NewSorted takes the layers to sort, which must all be Submitters.
func NewSorted(layers ...Layer) *Sorted {
	s := &Sorted{opts: &ebiten.DrawTrianglesOptions{}}
	for _, l := range layers {
		sub, ok := l.(Submitter)
		if !ok {
			panic(fmt.Sprintf("render: %T cannot be sorted: it draws instead of submitting", l))
		}
		s.subs = append(s.subs, sub)
	}
	return s
}

func (s *Sorted) Init(si *goke.SysInit) {
	for _, sub := range s.subs {
		sub.Init(si)
	}
}

// DrawWorld gathers every submitter's quads through cam and draws them back to front; a nil
// screen only gathers.
func (s *Sorted) DrawWorld(screen *ebiten.Image, cam camera.Camera) {
	s.sink.reset()
	for _, sub := range s.subs {
		sub.Submit(&s.sink, cam)
	}
	if screen == nil {
		return
	}
	var atlas *ebiten.Image
	s.verts, s.indices = s.verts[:0], s.indices[:0]
	for _, i := range s.sink.sorted() {
		it := s.sink.items[i]
		if it.atlas != atlas || len(s.verts)+4 > chunkVertices {
			s.flush(screen, atlas)
			atlas = it.atlas
		}
		idx := uint16(len(s.verts))
		s.verts = append(s.verts, s.sink.verts[it.first:it.first+4]...)
		s.indices = append(s.indices, idx, idx+1, idx+2, idx+1, idx+2, idx+3)
	}
	s.flush(screen, atlas)
	for _, sub := range s.subs {
		if o, ok := sub.(Overlayer); ok {
			o.Overlay(screen, cam)
		}
	}
}

func (s *Sorted) flush(screen, atlas *ebiten.Image) {
	if len(s.verts) > 0 && atlas != nil {
		screen.DrawTriangles(s.verts, s.indices, atlas, s.opts)
	}
	s.verts, s.indices = s.verts[:0], s.indices[:0]
}

// Gathered is how many quads the last Draw gathered — for measuring without a screen.
func (s *Sorted) Gathered() int { return len(s.sink.items) }
