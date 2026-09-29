package main

import (
	"image"
	"image/png"
	"os"
	"reflect"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/internal/engine"
	"github.com/kjkrol/gram/plugins/topography"
)

// shooter runs the demo in a window and saves what it draws, a view at a time: the isometric
// start, zoomed out, the perspective (Tab), zoomed out, the view from above, and the three views
// again with the ground traced on the GPU (G) — into the directory GRAM_SHOTS names, for a look
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
	cam := s.d.stage.world.Camera()
	for _, q := range s.d.stage.topography.Queues() {
		if q.Accepts() == reflect.TypeOf(c) {
			switch v := c.(type) {
			case topography.View:
				v.Camera = cam
				q.Put(control.Nobody, v)
			case topography.Heightfield:
				q.Put(control.Nobody, v)
			}
		}
	}
}

func (s *shooter) Update() error {
	s.frame++
	cam := s.d.stage.world.Camera()
	s.shot = ""
	switch s.frame {
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
		s.cmd(topography.Heightfield{})
	case 360:
		s.shot = "6-tab-tab-G"
	case 361:
		s.cmd(topography.View{})
	case 420:
		s.shot = "7-tab3-G"
	case 421:
		s.cmd(topography.View{})
	case 480:
		s.shot = "8-tab4-G"
	case 500:
		return ebiten.Termination
	}
	return s.e.Update()
}

func (s *shooter) Draw(screen *ebiten.Image) {
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

func (s *shooter) Layout(w, h int) (int, int) { return s.e.Layout(w, h) }

func TestShots(t *testing.T) {
	dir := os.Getenv("GRAM_SHOTS")
	if dir == "" {
		t.Skip("set GRAM_SHOTS to a directory to save what the demo draws")
	}
	d := NewDemo()
	e := engine.NewEngine(d)
	if err := e.Init(); err != nil {
		t.Fatal(err)
	}
	ebiten.SetWindowSize(1024, 768)
	if err := ebiten.RunGame(&shooter{dir: dir, e: e, d: d, t: t}); err != nil {
		t.Fatal(err)
	}
}
