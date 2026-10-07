package topography_test

import (
	"math"
	"reflect"
	"strings"
	"testing"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/cameras"
	"github.com/kjkrol/gram/plugins/players"
	"github.com/kjkrol/gram/plugins/topography"
	icameras "github.com/kjkrol/gram/plugins/topography/internal/cameras"
	"github.com/kjkrol/gram/plugins/topography/internal/topotest"
	"github.com/kjkrol/gram/plugins/world"
)

// From above the camera is flat — heights not drawn, screen x and y the world's — and View turns
// it isometric and back, keeping the ground point in the middle of the screen and a cell as wide.
func TestPlugin_ViewSwitchesBetweenAboveAndIsometric(t *testing.T) {
	// a world wide enough that no zoom floor holds the view back
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 2048, Height: 2048},
		Entities: world.EntitiesCfg{MaxCount: 4, MinSize: 1, MaxSize: 20},
		Heights:  true,
	})
	b := board.NewPlugin(grid.DefaultGrids{}.Square(64, 64, 32), &cell.MultipleOccupancy{}, w)
	p := topography.NewPlugin(w, b, topography.Config{Cell: 32, TileW: 64, HeightUnit: 1})
	cam := cameras.NewPlugin(w, p.Views(), camera.Config{ViewportWidth: 128, ViewportHeight: 64}).Main()
	if cam.Projection().Sorts() {
		t.Fatal("a game not begun Isometric looks isometrically")
	}
	x0, y0 := cam.Project(32, 32, 0)
	x1, y1 := cam.Project(32, 32, 10)
	if x0 != x1 || y0 != y1 {
		t.Errorf("from above a height is drawn %v, %v away from the ground, want not at all", x1-x0, y1-y0)
	}
	cam.CenterOn(1024, 1024, 0)
	cellBefore, _ := cam.Project(1024, 1024, 0)
	cellBefore2, _ := cam.Project(1056, 1024, 0)
	icameras.Switch(cam)
	if !cam.Projection().Sorts() {
		t.Fatal("after View the camera does not look isometrically")
	}
	vw, vh := cam.Viewport()
	if x, y := cam.Unproject(vw/2, vh/2, 0); math.Abs(float64(x-1024)) > 0.5 || math.Abs(float64(y-1024)) > 0.5 {
		t.Errorf("after View the middle of the screen is over (%v, %v), want (1024, 1024) still", x, y)
	}
	// a cell's diamond is as wide as the cell was: TileW of zoom to Cell of zoom
	left, _ := cam.Project(1024, 1056, 0)
	right, _ := cam.Project(1056, 1024, 0)
	if width := right - left; math.Abs(float64(width-(cellBefore2-cellBefore))) > 0.5 {
		t.Errorf("after View a cell spans %v pixels, want the %v it did from above", width, cellBefore2-cellBefore)
	}
	icameras.Switch(cam)
	if cam.Projection().Sorts() {
		t.Error("View again does not look from above")
	}
	_ = p
}

func TestPlugin_RefusesAWrappingWorld(t *testing.T) {
	defer func() {
		if r, _ := recover().(string); !strings.Contains(r, "wraps") {
			t.Errorf("NewPlugin over a torus: %q, want a refusal", r)
		}
	}()
	w := topotest.NewWorld(aabbworld.Torus)
	b, _ := topotest.LevelBoard(w)
	topography.NewPlugin(w, b, topography.Config{Cell: 32})
}

// Riding inside a unit, the mouse looks round, V and Tab leave, and Q, E, R and F are not bound;
// outside, Q and E turn and R and F tilt, Tab switches the view and V, over or behind the unit
// the camera follows, takes it closer. The keys that drive the unit are the players', bound to
// nothing here.
func TestDefaultBindings_RidingKeysHoldInsideOnly(t *testing.T) {
	w := topotest.NewWorld(0)
	b, _ := topotest.LevelBoard(w)
	p := topography.NewPlugin(w, b, topography.Config{Cell: 32, HeightUnit: 1, Isometric: true, Perspective: true})
	holding := func(how camera.How) map[string]control.Binding {
		out := map[string]control.Binding{}
		for _, bd := range p.DefaultBindings() {
			if bd.Holds(how) {
				out[players.Written(bd.Trigger)] = bd
			}
		}
		return out
	}
	riding, loose, over := holding(camera.Inside), holding(camera.Loose), holding(camera.Centred)
	for _, key := range []string{"W (held)", "S (held)", "A (held)", "D (held)", "Up (held)"} {
		if _, ok := riding[key]; ok {
			t.Errorf("riding, %s is the topography's, want the players'", key)
		}
		if _, ok := loose[key]; ok {
			t.Errorf("loose, %s is the topography's: the camera's WASD would be taken", key)
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
	cam := topotest.Camera(w, p)
	if bd, ok := riding["mouse"]; !ok {
		t.Error("riding, there is no binding of the mouse")
	} else {
		if _, built := bd.Build(control.Context{Camera: cam, Delta: geom.NewVec(3, -2)}); built {
			t.Error("riding, the mouse looks round with mouse look off")
		}
		cam.(camera.MouseLooker).SetMouseLook(true)
		if cmd, _ := bd.Build(control.Context{Camera: cam, Delta: geom.NewVec(3, -2)}); cmd != (topography.Look{Camera: cam, Dx: 3, Dy: -2}) {
			t.Errorf("riding, mouse look on, a move of (3, -2) issues %+v, want Look{Dx: 3, Dy: -2}", cmd)
		}
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
	if cmd, _ := riding["V"].Build(control.Context{}); reflect.TypeOf(cmd) != reflect.TypeFor[topography.Ride]() {
		t.Errorf("riding, V issues %T, want Ride: it leaves", cmd)
	}
	if cmd, _ := over["V"].Build(control.Context{}); reflect.TypeOf(cmd) != reflect.TypeFor[topography.Ride]() {
		t.Errorf("over the unit, V issues %T, want Ride: it goes behind", cmd)
	}
	if _, ok := loose["V"]; ok {
		t.Error("loose, V is bound: there is nothing to ride")
	}
}
