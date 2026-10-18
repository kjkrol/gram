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
	if !ok || !isPan || pan.Dx <= 0 || pan.Dy >= 0 {
		t.Errorf("in the top-right corner the edge builds %+v (%v), want a Pan right and up", cmd, ok)
	}
	ctx.Cursor = geom.NewVec(985, 400)
	cmd, _ = keys[0].Build(ctx)
	if pan := cmd.(cameras.Pan); pan.Dx <= 0 || pan.Dy != 0 {
		t.Errorf("at the right side the edge builds %+v, want a Pan right alone", pan)
	}
}

func TestKeys_CornersTurnAsQAndEDoInsteadOfScrolling(t *testing.T) {
	keys := cameras.Keys{Edge: true, CornersTurn: true}.Bindings()
	if len(keys) != 2 {
		t.Fatalf("%d bindings, want the edge's scroll and its turn", len(keys))
	}
	built := func(x, y float64) (pans, turns []any) {
		ctx := control.Context{Screen: geom.NewVec(1000, 800), Cursor: geom.NewVec(x, y)}
		for _, k := range keys {
			if cmd, ok := k.Build(ctx); ok {
				switch cmd.(type) {
				case cameras.Pan:
					pans = append(pans, cmd)
				case cameras.Turn:
					turns = append(turns, cmd)
				}
			}
		}
		return pans, turns
	}
	for _, c := range []struct {
		name string
		x, y float64
		want float32
	}{
		{"top-left", 15, 15, -cameras.CornerStep},
		{"bottom-left", 15, 785, -cameras.CornerStep},
		{"top-right", 985, 15, cameras.CornerStep},
		{"bottom-right", 985, 785, cameras.CornerStep},
	} {
		pans, turns := built(c.x, c.y)
		if len(pans) != 0 || len(turns) != 1 || turns[0].(cameras.Turn).Angle != c.want {
			t.Errorf("in the %s corner the edge builds pans %v and turns %v, want one Turn by %v",
				c.name, pans, turns, c.want)
		}
	}
	if pans, turns := built(985, 400); len(pans) != 1 || len(turns) != 0 {
		t.Errorf("at the right side the edge builds pans %v and turns %v, want a Pan alone",
			pans, turns)
	}
}

func TestDefaultBindings_CornersTurnWhereTheConfigTurns(t *testing.T) {
	turning := func(cfg cameras.Config) bool {
		ctx := control.Context{Screen: geom.NewVec(1000, 800), Cursor: geom.NewVec(15, 15)}
		for _, b := range cameras.NewPlugin(reliefWorld(), cfg).DefaultBindings() {
			if b.Trigger != (control.CursorInCorner{}) {
				continue
			}
			if cmd, ok := b.Build(ctx); ok {
				if _, is := cmd.(cameras.Turn); is {
					return true
				}
			}
		}
		return false
	}
	if !turning(cameras.Config{Turn: true, Cell: 32}) {
		t.Error("a Config turning has no corner turning the camera")
	}
	if turning(cameras.Config{Tilt: true, Cell: 32}) || turning(cameras.Config{}) {
		t.Error("a Config not turning turns the camera in a corner")
	}
}
