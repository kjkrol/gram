package ui

import (
	"image/color"
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/render/gputest"
)

type set struct {
	on bool
	v  float64
	i  int
}

func (set) Command() {}

func catching(s *Scene) (*Scene, *[]command.Command) {
	var issued []command.Command
	s.Issue(func(cmd command.Command) error { issued = append(issued, cmd); return nil })
	return s, &issued
}

func release(at geom.Vec) *control.InputEvents {
	return &control.InputEvents{MousePos: at, ClickQueue: []control.ClickEvent{
		{Button: control.MouseButtonLeft, Action: control.ActionRelease, Pos: at}}}
}

func TestCheckbox_AClickGivesTheOtherState(t *testing.T) {
	on := true
	box := Checkbox("shadows", func() bool { return on }, func(v bool) command.Command { return set{on: v} })
	s, issued := catching(built(NewScene("main", TopLeft(box))))
	w, h := box.needs()
	lw, _ := box.children[0].needs()
	if w <= lw || h == 0 {
		t.Fatalf("a checkbox needs %gx%g, want wider than its label's %g", w, h, lw)
	}
	s.HandleEvents(click(geom.NewVec(5, 5)), nil, nil)
	if len(*issued) != 1 || (*issued)[0] != (set{on: false}) {
		t.Fatalf("issued %v, want the box turned off", *issued)
	}
}

func TestSlider_DraggedGivesEachNewNumberAndTheWheelSteps(t *testing.T) {
	v := 0.0
	sl := Slider(0, 10, 1, func() float64 { return v }, func(n float64) command.Command { return set{v: n} })
	s, issued := catching(built(NewScene("main", TopLeft(sl).Size(sliderLength, sliderHeight))))
	// the knob runs from a radius in to a radius before the end: 160-18 pixels for 10 steps
	s.HandleEvents(click(geom.NewVec(9, 9)), nil, nil)
	if len(*issued) != 0 || s.held != sl {
		t.Fatalf("pressed at the start: issued %v, held %v; want nothing, the slider held", *issued, s.held)
	}
	s.HandleEvents(&control.InputEvents{MousePos: geom.NewVec(9+142*0.5, 9)}, nil, nil)
	s.HandleEvents(&control.InputEvents{MousePos: geom.NewVec(9+142*0.5+3, 9)}, nil, nil)
	s.HandleEvents(release(geom.NewVec(9+142, 300)), nil, nil)
	if want := []command.Command{set{v: 5}, set{v: 10}}; len(*issued) != 2 ||
		(*issued)[0] != want[0] || (*issued)[1] != want[1] || s.held != nil {
		t.Fatalf("dragged to the middle and let go at the end: issued %v, want %v, the slider let go",
			*issued, want)
	}
	v = 10
	*issued = nil
	ev := &control.InputEvents{MousePos: geom.NewVec(80, 9), ScrollDelta: -1}
	s.HandleEvents(ev, nil, nil)
	if len(*issued) != 1 || (*issued)[0] != (set{v: 9}) {
		t.Fatalf("the wheel down: issued %v, want 9", *issued)
	}
}

func TestDropdown_OpensItsOptionsOverAllAndTheOneClickedIsChosen(t *testing.T) {
	i := 0
	dd := Dropdown([]string{"none", "bases", "solids"}, func() int { return i },
		func(k int) command.Command { return set{i: k} })
	under := Button("under", order{"under"})
	s, issued := catching(built(NewScene("main", Layers(
		TopLeft(under).Size(300, 300),
		TopLeft(dd).Size(100, 30)))))
	s.HandleEvents(click(geom.NewVec(50, 15)), nil, nil)
	if s.overlay == nil || len(*issued) != 0 {
		t.Fatalf("clicked: open %v, issued %v; want the options open and nothing issued", s.overlay != nil, *issued)
	}
	s.Lay(screen)
	ob := s.overlay.Box()
	if ob.TopLeft.Y != 30 || ob.BottomRight.X-ob.TopLeft.X < 100 {
		t.Fatalf("the options lie at %v, want under the field, as wide at least", ob)
	}
	third := s.overlay.children[0].children[2]
	at := third.Box().TopLeft.Add(geom.NewVec(5, 5))
	s.HandleEvents(click(at), nil, nil)
	if len(*issued) != 1 || (*issued)[0] != (set{i: 2}) || s.overlay != nil {
		t.Fatalf("clicked the third option: issued %v, open %v; want solids chosen and the options closed", *issued, s.overlay != nil)
	}
	s.HandleEvents(click(geom.NewVec(50, 15)), nil, nil)
	s.HandleEvents(click(geom.NewVec(250, 250)), nil, nil)
	if s.overlay != nil || len(*issued) != 1 {
		t.Fatalf("clicked outside the options: open %v, issued %v; want them closed, nothing under reached", s.overlay != nil, *issued)
	}
}

func TestWidgets_DrawTheirMarksInTheAccent(t *testing.T) {
	gputest.Need(t)
	th := DefaultTheme()
	th.Accent = color.RGBA{G: 255, A: 255}
	th.Border = color.RGBA{R: 255, A: 255}
	box := Checkbox("x", func() bool { return true }, nil)
	sl := Slider(0, 1, 0, func() float64 { return 1 }, nil)
	s := NewScene("main", Layers(TopLeft(box).Size(100, 20), BottomLeft(sl).Size(100, 20))).Theme(th)
	s.Drawers()
	dst := render.NewImage(100, 60)
	dst.Clear()
	s.Lay(box_(0, 0, 100, 60))
	s.root.paint(dst)
	if p := pixel(dst, 4, 12); p[1] != 255 {
		t.Errorf("inside the ticked box is %v, want the accent", p)
	}
	if p := pixel(dst, 90, 50); p[1] != 255 {
		t.Errorf("the knob at the end is %v, want the accent", p)
	}
	if p := pixel(dst, 0, 10); p[0] != 255 {
		t.Errorf("the box's edge is %v, want the border", p)
	}
}

func box_(x, y, w, h float32) geom.AABB { return box(x, y, w, h) }

func TestLive_ADeadElementTakesNoPressAndComesBackLive(t *testing.T) {
	live := false
	sl := Slider(0, 10, 1, func() float64 { return 0 },
		func(n float64) command.Command { return set{v: n} }).Live(func() bool { return live })
	s, issued := catching(built(NewScene("main", TopLeft(sl).Size(sliderLength, sliderHeight))))
	s.HandleEvents(click(geom.NewVec(80, 9)), nil, nil)
	if len(*issued) != 0 || s.held != nil || sl.Hits(geom.NewVec(80, 9)) {
		t.Fatalf("dead, the slider took a press: issued %v, held %v", *issued, s.held)
	}
	if a, want := sl.look().Accent.A, DefaultTheme().Accent.A/2; a != want {
		t.Errorf("dead, the slider's marks are %v opaque, want half the theme's, %v", a, want)
	}
	live = true
	s.HandleEvents(click(geom.NewVec(80, 9)), nil, nil)
	if s.held != sl || !sl.Hits(geom.NewVec(80, 9)) {
		t.Fatalf("live again, the slider took no press: held %v", s.held)
	}
	if a, want := sl.look().Accent.A, DefaultTheme().Accent.A; a != want {
		t.Errorf("live, the slider's marks are %v opaque, want the theme's %v", a, want)
	}
}
