package cameras_test

import (
	"github.com/kjkrol/aabbworld/geom"
	"testing"

	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/plugins/cameras"
)

func TestKeys_ZoomKeysBuildAZoomInAndOut(t *testing.T) {
	keys := cameras.Keys{ZoomIn: control.KeyEqual, ZoomOut: control.KeyMinus}.Bindings()
	if len(keys) != 2 {
		t.Fatalf("%d bindings, want the two zoom keys", len(keys))
	}
	for i, want := range []float32{cameras.ZoomStep, 1 / cameras.ZoomStep} {
		cmd, ok := keys[i].Build(control.Context{})
		z, isZoom := cmd.(cameras.Zoom)
		if !ok || !isZoom || z.Factor != want {
			t.Errorf("key %d gives %v (%v), want a Zoom by %v", i, cmd, ok, want)
		}
	}
	if n := len(cameras.DefaultKeys().Bindings()); n != 9 {
		t.Errorf("the default keys bind %d, want 9: the zoom keys among them", n)
	}
}

func TestKeys_TheCursorInACornerBuildsADiagonalPanFromTheEdge(t *testing.T) {
	keys := cameras.Keys{Edge: true}.Bindings()
	if len(keys) != 1 {
		t.Fatalf("%d bindings, want the edge alone", len(keys))
	}
	ctx := control.Context{Screen: geom.NewVec(1000, 800), Cursor: geom.NewVec(985, 100)}
	cmd, ok := keys[0].Build(ctx)
	pan, isPan := cmd.(cameras.Pan)
	if !ok || !isPan || !pan.Edge || !pan.Corner || pan.Dx <= 0 || pan.Dy >= 0 {
		t.Errorf("in the top-right corner the edge builds %+v (%v), want a Pan right and up", cmd, ok)
	}
	ctx.Cursor = geom.NewVec(985, 400)
	cmd, _ = keys[0].Build(ctx)
	if pan := cmd.(cameras.Pan); pan.Corner || pan.Dx <= 0 || pan.Dy != 0 {
		t.Errorf("at the right side the edge builds %+v, want a Pan right alone, no corner", pan)
	}
}
