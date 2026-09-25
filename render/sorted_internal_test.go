package render

import (
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
)

type sheet struct{ img *ebiten.Image }

func (s sheet) Atlas() *ebiten.Image                   { return s.img }
func (sheet) UV(SpriteID) (sx0, sy0, sx1, sy1 float32) { return 0, 0, 32, 32 }

// depths is a Submitter that submits one unit quad per depth given, in order.
type depths struct {
	sheet  AtlasSource
	depths []float32
}

func (depths) Init(*goke.SysInit)                     {}
func (depths) DrawWorld(*ebiten.Image, camera.Camera) {}
func (d depths) Submit(sink *Sink, _ camera.Camera) {
	for _, z := range d.depths {
		sink.Quad(z, d.sheet, 0, Corners{{0, 0}, {1, 0}, {0, 1}, {1, 1}})
	}
}

func TestSorted_DrawsBackToFrontKeepingTiesInSubmissionOrder(t *testing.T) {
	terrain := depths{sheet: sheet{}, depths: []float32{3, 1, 2}}
	units := depths{sheet: sheet{}, depths: []float32{2, 0.5}}
	s := NewSorted(terrain, units)
	s.DrawWorld(nil, nil)

	var got []float32
	for _, i := range s.sink.sorted() {
		got = append(got, s.sink.items[i].depth)
	}
	want := []float32{0.5, 1, 2, 2, 3}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("drawn depths %v, want %v", got, want)
		}
	}
	// the two at depth 2: terrain's (submitted first) before the unit's
	order := s.sink.sorted()
	if s.sink.items[order[2]].first > s.sink.items[order[3]].first {
		t.Error("at equal depth the unit was drawn before the terrain it stands on")
	}
	if s.Gathered() != 5 {
		t.Errorf("Gathered = %d, want 5", s.Gathered())
	}
}

// onlyDraws shows the world without submitting quads.
type onlyDraws struct{}

func (onlyDraws) Init(*goke.SysInit)                     {}
func (onlyDraws) DrawWorld(*ebiten.Image, camera.Camera) {}

func TestSorted_RefusesARendererThatOnlyDraws(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("NewSorted took a WorldRenderer that does not submit")
		}
	}()
	NewSorted(onlyDraws{})
}

func TestSink_ShadedScalesTheColour(t *testing.T) {
	var s Sink
	s.Shaded(0, sheet{}, 0, Corners{{0, 0}, {2, 0}, {0, 2}, {2, 2}}, 0.5)
	if len(s.verts) != 4 || s.verts[3].DstX != 2 || s.verts[0].SrcX != 0.5 || s.verts[3].SrcX != 31.5 || s.verts[3].ColorR != 0.5 {
		t.Errorf("vertices %+v, want four corners sampling half a texel inside 0..32 at half brightness", s.verts)
	}
}
