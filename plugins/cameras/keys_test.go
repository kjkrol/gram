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
	if n := len(cameras.DefaultKeys().Bindings()); n != 13 {
		t.Errorf("the default keys bind %d, want 13: the zoom, the sight and the resolution keys "+
			"among them", n)
	}
}

func TestKeys_ResolutionKeysWithAltDrawFinerAndCoarser(t *testing.T) {
	keys := cameras.Keys{Finer: control.KeyEqual, Coarser: control.KeyMinus}.Bindings()
	if len(keys) != 2 {
		t.Fatalf("%d bindings, want the two resolution keys", len(keys))
	}
	for i, want := range []float32{1 / cameras.ResolutionStep, cameras.ResolutionStep} {
		press, isPress := keys[i].Trigger.(control.KeyPress)
		cmd, ok := keys[i].Build(control.Context{})
		r, isResolution := cmd.(cameras.Resolution)
		if !isPress || !press.Mods.Alt || press.Mods.Ctrl || !ok || !isResolution ||
			r.Factor != want {
			t.Errorf("key %d is %+v giving %v (%v), want Alt held and a Resolution by %v", i,
				keys[i].Trigger, cmd, ok, want)
		}
	}
	adapts := 0
	for _, b := range cameras.NewPlugin(reliefWorld(), cameras.Config{}).DefaultBindings() {
		if b.Trigger != (control.KeyPress{Key: control.KeyA, Mods: control.Mods{Alt: true}}) {
			continue
		}
		if cmd, ok := b.Build(control.Context{}); ok {
			if _, is := cmd.(cameras.Adapt); is {
				adapts++
			}
		}
	}
	if adapts != 1 {
		t.Errorf("the default bindings give Adapt on Alt+A %d times, want once", adapts)
	}
}

func TestKeys_SightKeysWithCtrlSeeFartherAndNearer(t *testing.T) {
	keys := cameras.Keys{Farther: control.KeyEqual, Nearer: control.KeyMinus}.Bindings()
	if len(keys) != 2 {
		t.Fatalf("%d bindings, want the two sight keys", len(keys))
	}
	for i, want := range []float64{cameras.SightStep, 1 / cameras.SightStep} {
		press, isPress := keys[i].Trigger.(control.KeyPress)
		cmd, ok := keys[i].Build(control.Context{})
		s, isSight := cmd.(cameras.Sight)
		if !isPress || !press.Mods.Ctrl || !ok || !isSight || s.Factor != want {
			t.Errorf("key %d is %+v giving %v (%v), want Ctrl held and a Sight by %v", i,
				keys[i].Trigger, cmd, ok, want)
		}
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

func TestKeys_AtAResolutionTheScrollAndTheLookAreTheSameOnTheScreen(t *testing.T) {
	keys := cameras.Keys{Up: control.KeyW, Edge: true}.Bindings()
	if len(keys) != 2 {
		t.Fatalf("%d bindings, want the key and the edge", len(keys))
	}
	half := control.Context{Resolution: 0.5, Screen: geom.NewVec(500, 400),
		Cursor: geom.NewVec(495, 200)}
	cmd, ok := keys[0].Build(half)
	if pan, isPan := cmd.(cameras.Pan); !ok || !isPan || pan.Dy != -cameras.DefaultScrollSpeed/2 {
		t.Errorf("W at 0.5 builds %+v (%v), want a Pan up by half the speed: the camera's pixels",
			cmd, ok)
	}
	cmd, ok = keys[1].Build(half)
	if pan, isPan := cmd.(cameras.Pan); !ok || !isPan || pan.Dx != cameras.DefaultScrollSpeed/2 {
		t.Errorf("the edge at 0.5 builds %+v (%v), want a Pan right by half the speed", cmd, ok)
	}
	p := cameras.NewPlugin(reliefWorld(), cameras.Config{Perspective: true, Cell: 32})
	for _, b := range p.DefaultBindings() {
		if b.Trigger != (control.CursorMove{}) {
			continue
		}
		cmd, ok := b.Build(control.Context{Resolution: 0.5, Delta: geom.NewVec(3, 4)})
		if look, isLook := cmd.(cameras.Look); !ok || !isLook || look.Dx != 6 || look.Dy != 8 {
			t.Errorf("a move of (3, 4) camera pixels at 0.5 builds %+v (%v), want a Look by (6, 8): "+
				"the shown ones", cmd, ok)
		}
		return
	}
	t.Error("no binding on the mouse looks round")
}
