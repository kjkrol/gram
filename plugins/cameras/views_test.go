package cameras_test

import (
	"reflect"
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/plugins/cameras"
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
	cam := p.New(cameras.Isometrically, camera.Config{})
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
