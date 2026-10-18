package camera_test

import (
	"testing"

	"github.com/kjkrol/gram/camera"
)

type (
	detailed struct {
		camera.Camera
		d camera.Detail
	}
	focused struct {
		camera.Camera
		x, y float32
	}
	// an eye of its own while riding, else the point it observes
	riding struct {
		focused
		eye bool
	}
)

func (c detailed) Detail() camera.Detail { return c.d }

func (c focused) Focus() (x, y, z float32) { return c.x, c.y, 0 }

func (c riding) Eye() (x, y, z float32, ok bool) { return 7, 8, 9, c.eye }

func TestDetail_WidenedSeesFartherAndWithoutEndStaysSo(t *testing.T) {
	if got := (camera.Detail{Sight: 100, Fog: 10}).Widened(2); got.Sight != 200 || got.Fog != 10 {
		t.Errorf("widened twice: %+v, want a sight of 200 and the fog kept", got)
	}
	if got := (camera.Detail{}).Widened(2); got.Sight != 0 {
		t.Errorf("a sight without end widened is %v, want it without end still", got.Sight)
	}
}

func TestDetailOf_IsTheCamerasOwnOrNone(t *testing.T) {
	d := camera.Detail{Sight: 5, Coarse: 2}
	if got := camera.DetailOf(detailed{d: d}); got != d {
		t.Errorf("a detailed camera's detail is %+v, want %+v", got, d)
	}
	if got := camera.DetailOf(focused{}); got != (camera.Detail{}) {
		t.Errorf("a camera without one has %+v, want none", got)
	}
}

func TestViewpoint_IsTheEyeElseTheFocus(t *testing.T) {
	if x, y, ok := camera.Viewpoint(riding{focused{x: 1, y: 2}, true}); !ok || x != 7 || y != 8 {
		t.Errorf("riding: (%v, %v, %v), want the eye (7, 8)", x, y, ok)
	}
	if x, y, ok := camera.Viewpoint(riding{focused{x: 1, y: 2}, false}); !ok || x != 1 || y != 2 {
		t.Errorf("without an eye: (%v, %v, %v), want the focus (1, 2)", x, y, ok)
	}
	if _, _, ok := camera.Viewpoint(detailed{}); ok {
		t.Error("a camera knowing neither eye nor focus has a viewpoint")
	}
}
