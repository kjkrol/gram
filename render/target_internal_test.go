package render

import (
	"slices"
	"testing"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/internal/gpu/gputest"
)

// a Direct over the figures noting how coarse the ground under each of its draws was
type coarseNoted struct{ coarse []float32 }

func (*coarseNoted) Init(*goke.SysInit)                {}
func (*coarseNoted) Overlies()                         {}
func (*coarseNoted) Compose(f *Frame, _ camera.Camera) { f.DrawAt(Nearest) }
func (n *coarseNoted) Draw(t Target, _ camera.Camera, _ Uniforms) {
	n.coarse = append(n.coarse, t.Coarse)
}

func TestComposer_TheOverPassSaysHowCoarseTheGroundUnderItIs(t *testing.T) {
	gputest.Need(t)
	var log []string
	over := &coarseNoted{}
	c := NewComposer(&staged{name: "ground", opaque: true, log: &log}, over)
	cam := topDown()
	cam.SetViewport(64, 64)
	c.Lay(cam, true)
	c.DrawLayer(NewImage(16, 16), cam, camera.Window{W: 64, H: 64}, camera.Window{})
	c.DrawLayer(NewImage(32, 32), cam, camera.Window{X: 16, Y: 16, W: 32, H: 32}, camera.Window{})
	c.DrawOver(NewImage(64, 64), cam)
	c.Lay(cam, true)
	c.DrawLayer(NewImage(32, 32), cam, camera.Window{W: 64, H: 64}, camera.Window{})
	c.DrawOver(NewImage(64, 64), cam)
	c.DrawWorld(NewImage(64, 64), cam)
	if want := []float32{4, 2, 0}; !slices.Equal(over.coarse, want) {
		t.Errorf("drawn over the grounds laid the Direct was handed a coarse of %v, want %v: the "+
			"coarsest layer's, a half's, and none drawn whole", over.coarse, want)
	}
}
