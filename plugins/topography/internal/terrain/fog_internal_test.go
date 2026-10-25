package terrain

import (
	"testing"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/cameras/cameratest"
	"github.com/kjkrol/gram/plugins/world/view"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/render/gputest"
)

// straight down seeing 150 from the world's corner, fogged from 100 on
type fogging struct{ downward }

func (fogging) Detail() camera.Detail    { return camera.Detail{Sight: 150, Fog: 50} }
func (fogging) Focus() (x, y, z float32) { return 0, 0, 0 }
func (fogging) Visible(camera.AABB) bool { return true }

// one sector over the whole world
type one struct{}

func (one) Len() int  { return 1 }
func (one) Cols() int { return 1 }
func (one) Box(int) geom.AABB {
	return geom.NewAABB(geom.NewVec(0, 0), geom.NewVec(512, 512))
}
func (one) Of(float32, float32) int { return 0 }
func (one) Off(int) bool            { return false }

func TestTerrain_TurnsTheGroundPastTheSightIntoTheFog(t *testing.T) {
	gputest.Need(t)
	cam := fogging{downward{cameratest.NewFromSpace(64, 64, 0)}}
	v := view.NewSeen(cam, one{})
	space, err := aabbworld.NewSpace(aabbworld.Config{Width: 512, Height: 512, BucketSize: 128})
	if err != nil {
		t.Fatal(err)
	}
	views := []*view.View{v}
	view.NewSystem(space, &views, 512, 512).Update(nil, 0)
	r := New(bumped{}, &grey{}, noAir{}, Config{Seen: func(camera.Camera) *view.View { return v }})
	screen := render.NewImage(64, 64)
	u := render.UniformsOf(map[string]any{"LightWay": []float32{0.3, 0.2, 1},
		"LightStrength": []float32{1}, "LightColor": []float32{1, 1, 1},
		"Ambience": []float32{0.2, 0.2, 0.2}, "Fog": []float32{0, 0, 1}})
	depth := render.NewDepth()
	screen.ClearDepth(depth)
	r.Draw(render.Target{Screen: screen, Depth: depth}, cam, u)
	pix := make([]byte, 4*64*64)
	screen.ReadPixels(pix)
	at := func(x, y int) []byte { return pix[4*(y*64+x) : 4*(y*64+x)+3] }
	// a pixel 8 world units: (5, 5) is 57 off, short of the fog; (60, 60) 679, past the sight
	if near := at(5, 5); near[2] > near[0]+10 {
		t.Errorf("the ground near shows %v, want it unfogged", near)
	}
	if far := at(60, 60); far[0] > 5 || far[2] < 250 {
		t.Errorf("the ground past the sight shows %v, want the fog's blue", far)
	}
}
