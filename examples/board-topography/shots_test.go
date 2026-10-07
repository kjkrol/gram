package main

import (
	"image"
	"image/png"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/examples/island"
	"github.com/kjkrol/gram/internal/engine"
	"github.com/kjkrol/gram/plugins/cameras"
	"github.com/kjkrol/gram/plugins/navigation"
	"github.com/kjkrol/gram/plugins/selection"
	"github.com/kjkrol/gram/plugins/topography"
	"github.com/kjkrol/gram/plugins/vision"
	"github.com/kjkrol/gram/render"
)

// shooter runs the demo without a window, a frame every 60th of a second on a GPU of its own, and
// saves what it draws, a view at a time: the isometric
// start, zoomed out, the perspective (Tab), zoomed out, the view from above, the views after them
// and first person, the views of sight and the routes shown (Shift+C, Shift+P) — into the directory GRAM_SHOTS names, for a look
// at what the GPU makes of a frame; the test is skipped without it.
type shooter struct {
	dir   string
	e     *engine.Engine
	d     *Demo
	t     *testing.T
	frame int
	shot  string
}

func (s *shooter) cmd(c any) {
	cam := s.d.a.player.Camera
	for _, q := range s.d.a.topography.Queues() {
		if q.Accepts() == reflect.TypeOf(c) {
			switch v := c.(type) {
			case topography.View:
				v.Camera = cam
				q.Put(s.d.a.player.ID, v)
			case topography.Ride:
				v.Camera = cam
				q.Put(s.d.a.player.ID, v)
			case topography.Look:
				v.Camera = cam
				q.Put(s.d.a.player.ID, v)
			}
		}
	}
}

// selectOne selects the one walker standing at the second stop, before anyone has moved: the
// camera follows exactly one selected (selection.FollowKey).
func (s *shooter) selectOne() {
	brd := s.d.a.board.Res.Logic.Board
	_, _, stops := island.Layout(brd)
	at := brd.CellCenter(stops[1])
	box := geom.NewAABB(geom.NewVec(at.X-CellSize/4, at.Y-CellSize/4), geom.NewVec(at.X+CellSize/4, at.Y+CellSize/4))
	for _, q := range s.d.a.selection.Queues() {
		if q.Accepts() == reflect.TypeFor[selection.Select]() {
			q.Put(s.d.a.player.ID, selection.Select{Box: box})
		}
	}
}

// follow fastens the player's camera over its one selected unit (C).
func (s *shooter) follow() {
	id, ok := s.d.a.selection.Chosen(s.d.a.player.ID)
	if err := s.d.a.players.Issue(s.d.a.player, cameras.Follow{Camera: s.d.a.player.Camera, Entity: id, On: ok}); err != nil {
		s.t.Fatal(err)
	}
}

// showViews shows every view of sight (Shift+C) and the routes (Shift+P), drawn over the ground.
func (s *shooter) showViews() {
	for _, q := range s.d.a.vision.Queues() {
		if q.Accepts() == reflect.TypeFor[vision.Cones]() {
			q.Put(control.Nobody, vision.Cones{})
		}
	}
	for _, q := range s.d.a.nav.Queues() {
		if q.Accepts() == reflect.TypeFor[navigation.Routes]() {
			q.Put(control.Nobody, navigation.Routes{})
		}
	}
}

func (s *shooter) Update() error {
	s.frame++
	cam := s.d.a.player.Camera
	s.shot = ""
	switch s.frame {
	case 2:
		s.selectOne()
		s.showViews()
	case 90:
		s.shot = "1-start"
	case 91:
		cam.ZoomOut(3, 512, 384)
	case 120:
		s.shot = "2-iso-far"
	case 121:
		s.cmd(topography.View{})
	case 180:
		s.shot = "3-tab"
	case 181:
		cam.ZoomOut(3, 512, 384)
	case 240:
		s.shot = "4-tab-far"
	case 241:
		s.cmd(topography.View{})
	case 300:
		s.shot = "5-tab-tab"
	case 301:
		s.cmd(topography.View{})
	case 360:
		s.shot = "6-tab3"
	case 361:
		s.cmd(topography.View{})
	case 420:
		s.shot = "7-tab4"
	case 421:
		s.follow() // over the selected unit
	case 422:
		s.cmd(topography.Ride{}) // behind it
	case 423:
		s.cmd(topography.Ride{}) // first person, in it
	case 424:
		s.cmd(topography.Look{Dy: -80}) // the head raised: most lines of sight go up
	case 480:
		s.shot = "8-first-person"
	case 500:
		return engine.Termination
	}
	return s.e.Update()
}

func (s *shooter) Draw(screen *render.Image) {
	s.e.Draw(screen)
	if s.shot == "" {
		return
	}
	img := image.NewRGBA(screen.Bounds())
	screen.ReadPixels(img.Pix)
	f, err := os.Create(s.dir + "/" + s.shot + ".png")
	if err != nil {
		s.t.Error(err)
		return
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		s.t.Error(err)
	}
	s.t.Logf("shot %s", s.shot)
}

func TestShots(t *testing.T) {
	dir := os.Getenv("GRAM_SHOTS")
	if dir == "" {
		t.Skip("set GRAM_SHOTS to a directory to save what the demo draws")
	}
	needGPU(t)
	d := NewDemo()
	e := engine.NewEngine(d)
	if err := e.Init(); err != nil {
		t.Fatal(err)
	}
	s := &shooter{dir: dir, e: e, d: d, t: t}
	w, h := e.Layout(1024, 768)
	screen := render.NewImage(w, h)
	for {
		if err := s.Update(); err != nil {
			if err == engine.Termination {
				return
			}
			t.Fatal(err)
		}
		screen.Clear()
		s.Draw(screen)
		time.Sleep(time.Second / 60)
	}
}
