package cameras_test

import (
	"math"
	"reflect"
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/plugins/cameras"
	"github.com/kjkrol/gram/plugins/cameras/cameratest"
	"github.com/kjkrol/gram/plugins/players"
	"github.com/kjkrol/gram/plugins/world"
)

func reliefWorld() *world.Plugin {
	return world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 256, Height: 256},
		Entities: world.EntitiesCfg{MaxCount: 4, MinSize: 1, MaxSize: 20},
		Heights:  true,
	})
}

func boundIn(p *cameras.Plugin, how camera.How) map[string]control.Binding {
	out := map[string]control.Binding{}
	for _, bd := range p.DefaultBindings() {
		if bd.Holds(how) {
			out[players.Written(bd.Trigger)] = bd
		}
	}
	return out
}

func TestDefaultBindings_RidingKeysHoldInsideOnly(t *testing.T) {
	w := reliefWorld()
	p := cameras.NewPlugin(w, cameras.Config{Turn: true, Tilt: true, Perspective: true, Cell: 32})
	riding, loose, over := boundIn(p, camera.Inside), boundIn(p, camera.Loose),
		boundIn(p, camera.Centred)
	for _, key := range []string{"W (held)", "S (held)", "A (held)", "D (held)"} {
		if _, ok := riding[key]; ok {
			t.Errorf("riding, %s pans the camera, want it the players' to drive with", key)
		}
		if _, ok := loose[key]; !ok {
			t.Errorf("loose, %s does not pan the camera", key)
		}
	}
	for _, key := range []string{"Q (held)", "E (held)", "R (held)", "F (held)"} {
		if _, ok := riding[key]; ok {
			t.Errorf("riding, %s is bound, want nothing: the mouse looks round", key)
		}
		if _, ok := loose[key]; !ok {
			t.Errorf("loose, %s is not bound", key)
		}
	}
	cam := p.Write().New(cameras.Isometrically, camera.Config{})
	if bd, ok := riding["mouse"]; !ok {
		t.Error("riding, there is no binding of the mouse")
	} else if cmd, _ := bd.Build(control.Context{Camera: cam, Delta: geom.NewVec(3, -2)}); cmd !=
		(cameras.Look{Camera: cam, Dx: 3, Dy: -2}) {
		t.Errorf("riding, a move of (3, -2) issues %+v, want Look{Dx: 3, Dy: -2}", cmd)
	}
	if _, ok := loose["mouse"]; ok {
		t.Error("loose, the mouse looks round")
	}
	if _, ok := riding["Tab"]; !ok {
		t.Error("riding, Tab is not bound")
	}
	if _, ok := loose["Tab"]; !ok {
		t.Error("loose, Tab is not bound")
	}
	ride := reflect.TypeFor[cameras.Ride]()
	if cmd, _ := riding["V"].Build(control.Context{}); reflect.TypeOf(cmd) != ride {
		t.Errorf("riding, V issues %T, want Ride: it leaves", cmd)
	}
	if cmd, _ := over["V"].Build(control.Context{}); reflect.TypeOf(cmd) != ride {
		t.Errorf("over the unit, V issues %T, want Ride: it goes behind", cmd)
	}
	if _, ok := loose["V"]; ok {
		t.Error("loose, V is bound: there is nothing to ride")
	}
}

func TestDefaultBindings_APlainConfigHasNoViewKeys(t *testing.T) {
	p := cameras.NewPlugin(reliefWorld(), cameras.Config{})
	for _, key := range []string{"Q (held)", "E (held)", "R (held)", "F (held)", "Tab", "V"} {
		for _, how := range []camera.How{camera.Loose, camera.Centred, camera.Inside} {
			if _, ok := boundIn(p, how)[key]; ok {
				t.Errorf("a plain Config binds %s, want no view keys", key)
			}
		}
	}
}

func TestNewPlugin_RefusesSeeingInReliefWithoutACell(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("a Config turning and tilting without a Cell is taken, want a refusal")
		}
	}()
	cameras.NewPlugin(reliefWorld(), cameras.Config{Turn: true, Tilt: true})
}

func TestDefaultBindings_TabSwitchesTheProjectionOnlyWithThePerspective(t *testing.T) {
	flat := cameras.NewPlugin(reliefWorld(), cameras.Config{Turn: true, Tilt: true, Cell: 32})
	for _, how := range []camera.How{camera.Loose, camera.Centred, camera.Inside} {
		if _, ok := boundIn(flat, how)["Tab"]; ok {
			t.Errorf("without the perspective Tab is bound %v, want nothing to switch to", how)
		}
	}
	p := cameras.NewPlugin(reliefWorld(), cameras.Config{Perspective: true, Cell: 32})
	view := reflect.TypeFor[cameras.View]()
	if cmd, _ := boundIn(p, camera.Loose)["Tab"].Build(control.Context{}); reflect.TypeOf(cmd) !=
		view {
		t.Errorf("with the perspective Tab issues %T, want View", cmd)
	}
}

func TestDefaultBindings_RLooksDownMoreSteeplyAndFFlatter(t *testing.T) {
	p := cameras.NewPlugin(reliefWorld(), cameras.Config{Tilt: true, Cell: 32})
	loose := boundIn(p, camera.Loose)
	keys := map[string]float32{"R (held)": cameras.TiltStep, "F (held)": -cameras.TiltStep}
	for key, want := range keys {
		if cmd, _ := loose[key].Build(control.Context{}); cmd != (cameras.Tilt{Angle: want}) {
			t.Errorf("%s issues %+v, want Tilt{Angle: %v}", key, cmd, want)
		}
	}
}

func TestNew_FromAboveInReliefIsTheMapsCamera(t *testing.T) {
	p := cameras.NewPlugin(reliefWorld(), cameras.Config{Turn: true, Tilt: true, Cell: 32})
	stick, plain := p.Write().New(cameras.FromAbove, camera.Config{}), cameratest.NewFromSpace(256,
		256, 0)
	same := func(when string) {
		t.Helper()
		if stick.Zoom() != plain.Zoom() {
			t.Errorf("%s: zoom %v, want the map camera's %v", when, stick.Zoom(), plain.Zoom())
		}
		for _, q := range [][3]float32{{0, 0, 0}, {100, 40, 0}, {13.5, 250, 30}} {
			sx, sy := stick.Project(q[0], q[1], q[2])
			px, py := plain.Project(q[0], q[1], q[2])
			if math.Abs(float64(sx-px)) > 1e-3 || math.Abs(float64(sy-py)) > 1e-3 {
				t.Errorf("%s: %v is drawn at (%v, %v), want the map camera's (%v, %v)",
					when, q, sx, sy, px, py)
			}
			x, y := stick.Unproject(sx, sy, q[2])
			if math.Abs(float64(x-q[0])) > 1e-3 || math.Abs(float64(y-q[1])) > 1e-3 {
				t.Errorf("%s: (%v, %v) is seen at (%v, %v), want %v", when, sx, sy, x, y, q)
			}
		}
		sb, pb := stick.Bounds(), plain.Bounds()
		for _, d := range []float64{sb.TopLeft.X - pb.TopLeft.X, sb.TopLeft.Y - pb.TopLeft.Y,
			sb.BottomRight.X - pb.BottomRight.X, sb.BottomRight.Y - pb.BottomRight.Y} {
			if math.Abs(d) > 1e-3 {
				t.Errorf("%s: Bounds %v, want the map camera's %v", when, sb, pb)
				break
			}
		}
	}
	for _, cam := range []camera.Camera{stick, plain} {
		cam.SetViewport(128, 64)
		cam.MoveTo(10, 20)
	}
	same("moved")
	for _, cam := range []camera.Camera{stick, plain} {
		cam.ZoomIn(2, 60, 50)
	}
	same("zoomed in")
	for _, cam := range []camera.Camera{stick, plain} {
		cam.Pan(7, -3)
	}
	same("panned")
}

func TestNew_IsometricallyTheZoomIsWhatTheDiamondSpans(t *testing.T) {
	p := cameras.NewPlugin(reliefWorld(), cameras.Config{Turn: true, Cell: 32})
	cam := p.Write().New(cameras.Isometrically, camera.Config{})
	if want := float32(64 / (math.Sqrt2 * 32)); math.Abs(float64(cam.Zoom()-want)) > 1e-6 {
		t.Errorf("a 64-wide diamond of a 32 cell begins at zoom %v, want %v: screen pixels a "+
			"world unit", cam.Zoom(), want)
	}
	if cam := p.Write().New(cameras.Isometrically, camera.Config{Zoom: 3}); cam.Zoom() != 3 {
		t.Errorf("camera.Config.Zoom 3 begins at zoom %v, want 3", cam.Zoom())
	}
}
