package camera_test

import (
	"testing"

	"github.com/kjkrol/gram/camera"
)

// zoomed is a camera drawing every point at Zoom z.
type zoomed struct {
	camera.Camera
	z float32
}

func (c zoomed) Zoom() float32 { return c.z }

// perspective is a camera drawing a world unit smaller the further y is, and nothing behind y 0.
type perspective struct{ zoomed }

func (perspective) ScaleAt(_, y, _ float32) float32 {
	if y <= 0 {
		return 0
	}
	return 100 / y
}

func TestScaleAt_IsTheScalersOwnElseTheZoom(t *testing.T) {
	if s := camera.ScaleAt(zoomed{z: 2}, 5, 50, 0); s != 2 {
		t.Errorf("through a camera drawing every point alike the scale is %v, want its zoom 2", s)
	}
	p := perspective{zoomed{z: 2}}
	if near, far := camera.ScaleAt(p, 0, 10, 0), camera.ScaleAt(p, 0, 100, 0); near != 10 || far != 1 {
		t.Errorf("through a perspective the scale is %v near and %v far, want 10 and 1", near, far)
	}
	if s := camera.ScaleAt(p, 0, -5, 0); s != 0 {
		t.Errorf("behind the eye the scale is %v, want 0", s)
	}
}
