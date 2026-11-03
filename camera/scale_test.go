package camera_test

import (
	"testing"

	"github.com/kjkrol/gram/camera"
)

type zoomed struct {
	camera.Camera
	z float32
}

func (c zoomed) Zoom() float32 { return c.z }

type perspective struct{ zoomed }

// as far behind the eye as before it, the same scale
func (perspective) ScaleAt(_, y, _ float32) float32 { return 100 / max(y, -y, 1) }

type resolved struct {
	zoomed
	res float32
}

func (c resolved) Resolution() float32       { return c.res }
func (c resolved) SetResolution(res float32) {}

func TestScaleAt_IsTheScalersOwnElseTheZoom(t *testing.T) {
	if s := camera.ScaleAt(zoomed{z: 2}, 5, 50, 0); s != 2 {
		t.Errorf("through a camera drawing every point alike the scale is %v, want its zoom 2", s)
	}
	p := perspective{zoomed{z: 2}}
	near, far := camera.ScaleAt(p, 0, 10, 0), camera.ScaleAt(p, 0, 100, 0)
	if near != 10 || far != 1 {
		t.Errorf("through a perspective the scale is %v near and %v far, want 10 and 1", near, far)
	}
}

func TestScaleShown_IsTheScaleInThePixelsShown(t *testing.T) {
	if s := camera.ScaleShown(resolved{zoomed{z: 2}, 0.5}, 0, 0, 0); s != 4 {
		t.Errorf("a zoom of 2 drawn at half the pixels shows %v, want 4", s)
	}
	if s := camera.ScaleShown(zoomed{z: 2}, 0, 0, 0); s != 2 {
		t.Errorf("a camera without a resolution shows %v, want its zoom 2", s)
	}
}
