package camera_test

import (
	"testing"

	"github.com/kjkrol/gram/camera"
)

type (
	sighted struct {
		camera.Camera
		s float32
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

func (c sighted) Sight() float32 { return c.s }

func (c focused) Focus() (x, y, z float32) { return c.x, c.y, 0 }

func (c riding) Eye() (x, y, z float32, ok bool) { return 7, 8, 9, c.eye }

func TestSightOf_IsTheCamerasOwnElseOnce(t *testing.T) {
	if got := camera.SightOf(sighted{s: 2.5}); got != 2.5 {
		t.Errorf("a sighted camera sees %v times the world's sight, want 2.5", got)
	}
	if got := camera.SightOf(focused{}); got != 1 {
		t.Errorf("a camera without one sees %v times the world's sight, want 1", got)
	}
}

func TestViewpoint_IsTheEyeElseTheFocus(t *testing.T) {
	if x, y, ok := camera.Viewpoint(riding{focused{x: 1, y: 2}, true}); !ok || x != 7 || y != 8 {
		t.Errorf("riding: (%v, %v, %v), want the eye (7, 8)", x, y, ok)
	}
	if x, y, ok := camera.Viewpoint(riding{focused{x: 1, y: 2}, false}); !ok || x != 1 || y != 2 {
		t.Errorf("without an eye: (%v, %v, %v), want the focus (1, 2)", x, y, ok)
	}
	if _, _, ok := camera.Viewpoint(sighted{}); ok {
		t.Error("a camera knowing neither eye nor focus has a viewpoint")
	}
}
