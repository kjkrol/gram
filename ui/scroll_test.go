package ui

import (
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/gram/control"
)

func TestScroll_ShowsItsContentThroughItsBoxScrolledByTheWheelAndTheBar(t *testing.T) {
	pressed := 0
	var rows []Part
	for range 10 {
		rows = append(rows, Fixed(30, Button("row", Show{Name: "x"})))
	}
	last := Checkbox("last", func() bool { return false },
		func(bool) command.Command { pressed++; return set{} })
	rows = append(rows, Fixed(30, last))
	content := Rows(rows...)
	sc := Scroll(content)
	s := built(NewScene("main", TopLeft(sc).Size(212, 100)))
	// 330 pixels of content through 100: the last row lies past the box, unseen and unhit
	if box := last.Box(); box.TopLeft.Y != 300 {
		t.Fatalf("the last row laid at y %v, want 300: the content as tall as it needs", box.TopLeft.Y)
	}
	s.HandleEvents(click(geom.NewVec(10, 50)), nil, nil)
	if t2 := topmost(s.root, geom.NewVec(10, 310), nil); t2.hit != nil {
		t.Errorf("a point under the box hit %v, want nothing: the content is clipped", t2.hit)
	}
	for range 5 {
		s.HandleEvents(&control.InputEvents{MousePos: geom.NewVec(10, 50), ScrollDelta: -1}, nil, nil)
		s.Lay(geom.NewAABB(geom.NewVec(0, 0), geom.NewVec(400, 300)))
	}
	if box := last.Box(); box.TopLeft.Y != 70 {
		t.Fatalf("five notches down, the last row lies at y %v, want 70: 48 a notch, stopped at the end", box.TopLeft.Y)
	}
	s.HandleEvents(click(geom.NewVec(10, 85)), nil, nil)
	if pressed != 1 {
		t.Errorf("the last row scrolled into the box pressed %d times, want once", pressed)
	}
	// the thumb dragged back to the top
	s.HandleEvents(click(geom.NewVec(205, 95)), nil, nil)
	if s.held != sc {
		t.Fatalf("pressed on the bar: held %v, want the scroll", s.held)
	}
	s.HandleEvents(release(geom.NewVec(205, -50)), nil, nil)
	s.Lay(geom.NewAABB(geom.NewVec(0, 0), geom.NewVec(400, 300)))
	if box := last.Box(); box.TopLeft.Y != 300 || s.held != nil {
		t.Errorf("dragged to the top and let go, the last row lies at y %v held %v, want 300, nothing held",
			box.TopLeft.Y, s.held)
	}
}

func TestScroll_AContentThatFitsShowsWholeWithNoBar(t *testing.T) {
	content := Rows(Fixed(30, Label("a")), Fixed(30, Label("b")))
	sc := Scroll(content)
	built(NewScene("main", TopLeft(sc).Size(212, 100)))
	cw, _ := content.needs()
	if w, h := sc.needs(); w != cw+scrollBar || h != 60 {
		t.Errorf("the scroll needs %v by %v, want its content's %v and the bar wide, 60 tall", w, h, cw)
	}
	if box := content.Box(); box.TopLeft.Y != 0 || box.BottomRight.X != 212-scrollBar {
		t.Errorf("the content laid at %v, want from the top, the bar's width short", box)
	}
}
