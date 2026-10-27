package cameras_test

import (
	"reflect"
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/internal/f32"
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
	if n := len(cameras.DefaultKeys().Bindings()); n != 11 {
		t.Errorf("the default keys bind %d, want 11: the zoom and the sight keys among them", n)
	}
}

func TestDefaultBindings_NoKeySetsWhatTheSettingsScreenSets(t *testing.T) {
	cfg := cameras.Config{Turn: true, Tilt: true, Perspective: true, Cell: 32}
	for _, b := range cameras.NewPlugin(reliefWorld(), cfg).DefaultBindings() {
		switch b.Type() {
		case reflect.TypeFor[cameras.Zones](), reflect.TypeFor[cameras.Adapt](),
			reflect.TypeFor[cameras.Picture](), reflect.TypeFor[cameras.Resolution]():
			t.Errorf("%v is bound to %v, want the settings screen alone", b.Type(), b.Trigger)
		}
	}
}

func TestKeys_SightKeysWithCtrlSeeFartherAndNearer(t *testing.T) {
	keys := cameras.Keys{Farther: control.KeyEqual, Nearer: control.KeyMinus}.Bindings()
	if len(keys) != 2 {
		t.Fatalf("%d bindings, want the two sight keys", len(keys))
	}
	for i, want := range []float32{cameras.SightStep, 1 / cameras.SightStep} {
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
	built := func(x, y float32) (pans, turns []any) {
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
		x, y float32
		want float32
	}{
		{"top-left", 15, 15, cameras.CornerStep},
		{"bottom-left", 15, 785, -cameras.CornerStep},
		{"top-right", 985, 15, -cameras.CornerStep},
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
	// the cursor on the last shown pixel, half a camera pixel short of the width
	half := control.Context{Resolution: 0.5, Screen: geom.NewVec(500, 400),
		Cursor: geom.NewVec(499.5, 200)}
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

func TestKeys_TheEdgeScrollsAQuarterAsFastAtTheZonesInnerBoundAsAtTheWindowsEdge(t *testing.T) {
	keys := cameras.Keys{Edge: true}.Bindings()
	ctx := control.Context{Screen: geom.NewVec(1000, 800)}
	speeds := map[float32]float32{}
	for _, x := range []float32{0, 30, 59} {
		ctx.Cursor = geom.NewVec(x, 400)
		cmd, ok := keys[0].Build(ctx)
		pan, isPan := cmd.(cameras.Pan)
		if !ok || !isPan || pan.Dx >= 0 || pan.Dy != 0 {
			t.Fatalf("at x %v the edge builds %+v (%v), want a Pan left", x, cmd, ok)
		}
		speeds[x] = -pan.Dx
	}
	if speeds[0] != cameras.DefaultScrollSpeed || speeds[30] != cameras.DefaultScrollSpeed*0.625 ||
		f32.Abs(float32(speeds[59]-cameras.DefaultScrollSpeed*0.2625)) > 1e-5 {
		t.Errorf("the speeds by the cursor's x are %v, want the whole at the edge, five eighths "+
			"halfway in and a shade over a quarter at the inner bound", speeds)
	}
}

func TestDefaultBindings_TheEdgeScrollsAsTheSettingsSayAtTheSpeedSet(t *testing.T) {
	p := cameras.NewPlugin(reliefWorld(), cameras.Config{})
	var edge control.Binding
	for _, b := range p.DefaultBindings() {
		if b.Trigger == (control.CursorAtEdge{}) {
			edge = b
		}
	}
	if edge.Trigger == nil {
		t.Fatal("no binding on the edge")
	}
	ctx := control.Context{Screen: geom.NewVec(1000, 800), Cursor: geom.NewVec(999, 400)}
	if cmd, ok := edge.Build(ctx); !ok || cmd.(cameras.Pan).Dx != cameras.DefaultScrollSpeed {
		t.Errorf("at the edge the default builds %+v (%v), want a Pan right at the default speed",
			cmd, ok)
	}
	p.Write().SetEdgeSpeed(16)
	if cmd, ok := edge.Build(ctx); !ok || cmd.(cameras.Pan).Dx != 16 {
		t.Errorf("at a speed of 16 the edge builds %+v (%v), want a Pan right by 16", cmd, ok)
	}
	p.Write().ScrollFromEdges(false)
	if cmd, ok := edge.Build(ctx); ok {
		t.Errorf("the edges still, the edge builds %+v, want nothing", cmd)
	}
	if !p.Read(0).EdgesScroll() == false || p.Read(0).EdgeSpeed() != 16 {
		t.Errorf("the settings say scrolling %v at %v, want still at 16", p.Read(0).EdgesScroll(),
			p.Read(0).EdgeSpeed())
	}
}

func TestKeys_TheGrabDragsTheWorldPastTheSlopAndYields(t *testing.T) {
	held := control.ButtonHeld{Button: control.MouseButtonRight}
	keys := cameras.Keys{Grab: true}.Bindings()
	if len(keys) != 1 || keys[0].Trigger != held || !keys[0].Yields() ||
		!keys[0].Holds(camera.Outside) || keys[0].Holds(camera.Inside) {
		t.Fatalf("the grab's bindings are %+v, want one on the right button held, yielding, "+
			"outside", keys)
	}
	ctx := control.Context{Start: geom.NewVec(100, 100), Cursor: geom.NewVec(103, 100),
		Delta: geom.NewVec(3, 0)}
	if cmd, ok := keys[0].Build(ctx); ok {
		t.Errorf("within a click's slop the grab builds %+v, want nothing", cmd)
	}
	ctx.Cursor = geom.NewVec(106, 100)
	pan := func() cameras.Pan {
		t.Helper()
		cmd, ok := keys[0].Build(ctx)
		if !ok {
			t.Fatalf("at %v the grab builds nothing", ctx.Cursor)
		}
		return cmd.(cameras.Pan)
	}
	if got := pan(); got.Dx != -6 || got.Dy != 0 {
		t.Errorf("the first pass past the slop builds %+v, want a Pan by the whole way, -6", got)
	}
	ctx.Cursor = geom.NewVec(110, 100)
	ctx.Delta = geom.NewVec(4, 0)
	if got := pan(); got.Dx != -4 || got.Dy != 0 {
		t.Errorf("a later pass builds %+v, want a Pan by the delta, -4", got)
	}
	if cameras.DefaultKeys().Grab {
		t.Error("the default keys grab the world, want a game's choice")
	}
	grabs := func(cfg cameras.Config) bool {
		for _, b := range cameras.NewPlugin(reliefWorld(), cfg).DefaultBindings() {
			if b.Trigger == held {
				return true
			}
		}
		return false
	}
	if grabs(cameras.Config{}) || !grabs(cameras.Config{Grab: true}) {
		t.Error("the default bindings grab the world where the Config says nothing, or not where " +
			"it asks")
	}
}
