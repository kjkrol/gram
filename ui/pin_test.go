package ui

import (
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/entity"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/rule"
	"github.com/kjkrol/gram/rule/effect"
	"github.com/kjkrol/uid"
)

// shifted is a picture of the world moved: the world point (x, y) lies at (x-left, y-top) in it.
type shifted struct {
	left, top float32
	w, h      int
}

func (v *shifted) Resize(w, h int)   { v.w, v.h = w, h }
func (*shifted) Draw() *render.Image { return nil }
func (v *shifted) ToPixels(x, y, _ float32) (px, py, depth float32, visible bool) {
	px, py = x-v.left, y-v.top
	return px, py, 0, px >= 0 && py >= 0 && px < float32(v.w) && py < float32(v.h)
}

// nobody names no entity: what pins an element in a test that hands it its spots itself.
var nobody = entity.Named("nobody")

// looking is the player over a picture: it notes the entities it was asked to look at.
type looking struct {
	player
	looked []uid.UID64
}

func (l *looking) LookAt(id uid.UID64) { l.looked = append(l.looked, id) }

// standing lays root over the screen and stands e for spots, as a frame would.
func standing(root, e *Element, spots ...spot) {
	root.lay(screen)
	e.pin.stand(e, spots, views(root), screen)
}

func TestPin_StandsAboveItsEntityInThePictureThatShowsIt(t *testing.T) {
	label := Label("frozen").Size(40, 20).On(nobody)
	root := Layers(Columns(Share(1, Label("left")), Share(1, Image(&shifted{left: 1000}))), label)
	standing(root, label, spot{id: 1, placed: true, x: 1100, y: 50})
	if got := label.pin.instances[0].box; got != box(600+100-20, 50-20, 40, 20) {
		t.Fatalf("stood at %v, want above (700, 50) in the right half", got)
	}
}

func TestPin_PointsAtItsEntityFromTheEdgeWhileItIsOutOfSight(t *testing.T) {
	label := Label("there").Size(40, 20).On(nobody).OffScreen(PointAtIt)
	view := Image(&shifted{})
	root := Layers(view, label)
	standing(root, label, spot{id: 1, placed: true, x: 5000, y: 300})
	in := label.pin.instances[0]
	if in.box.BottomRight.X > 1200 || in.box.TopLeft.X < 1000 || len(in.arrow) != 3 {
		t.Fatalf("stood at %v with arrow %v, want at the right edge pointing right", in.box, in.arrow)
	}
	if tip := in.arrow[0]; tip.X <= in.box.BottomRight.X {
		t.Errorf("the arrow's tip %v does not point past the box towards the entity", tip)
	}
}

func TestPin_GoToItMovesTheCameraOnceAsTheEntityAppears(t *testing.T) {
	red := &looking{}
	plague := Window("plague").On(nobody).OffScreen(GoToIt)
	root := Layers(Image(&shifted{}).Input(red), plague)
	far := spot{id: 7, placed: true, x: 5000, y: 5000}
	standing(root, plague, far)
	standing(root, plague, far)
	if len(red.looked) != 1 || red.looked[0] != 7 {
		t.Fatalf("the camera was asked to look at %v, want entity 7 once", red.looked)
	}
}

func TestPin_ShowItsButtonMovesTheCameraOntoTheEntity(t *testing.T) {
	red := &looking{}
	panel := Window("far off").On(nobody).OffScreen(ShowIt)
	s := built(NewScene("main", nil, func() *Element { return Layers(Image(&shifted{}).Input(red), panel) }))
	panel.pin.stand(panel, []spot{{id: 9, placed: true, x: 5000, y: 5000}}, views(s.root), screen)
	in := panel.pin.instances[0]
	if !in.show {
		t.Fatal("ShowIt shows no button for an entity out of sight")
	}
	var button geom.AABB
	panel.pin.each(panel, panel.parent, func(*instance) { button = panel.pin.show.Box() })
	s.HandleEvents(click(button.TopLeft.Add(geom.NewVec(2, 2))), nil, nil)
	if len(red.looked) != 1 || red.looked[0] != 9 {
		t.Fatalf("Show asked the camera to look at %v, want entity 9", red.looked)
	}
}

func TestPin_AModalElementIsShownForOneEntityAtATime(t *testing.T) {
	decision := Center(Window("decide")).Modal().On(nobody)
	root := Layers(Image(&shifted{}), decision)
	standing(root, decision, spot{id: 1}, spot{id: 2})
	if n := len(decision.pin.instances); n != 1 || decision.pin.instances[0].id != 1 {
		t.Fatalf("%d shown, want the first alone", n)
	}
}

func TestPin_AnEntityWithNoPlaceHasItsElementWhereItsParentLaysIt(t *testing.T) {
	inner := Window("the world")
	decision := Center(inner).Size(200, 100).On(nobody)
	root := Layers(Image(&shifted{}), decision)
	standing(root, decision, spot{id: 1})
	decision.pin.each(decision, decision.parent, func(*instance) {})
	if inner.Box() != box(500, 250, 200, 100) {
		t.Fatalf("laid at %v, want in the middle of the screen", inner.Box())
	}
}

func TestPin_AButtonGivesCommandsForItAboutTheEntityItIsShownFor(t *testing.T) {
	var issued []any
	greeting := effect.Effect{}
	lift := rule.Lift(greeting).On(It)
	answer := Button("hello", lift).Named("answer")
	window := Window("host", answer).On(nobody)
	s := built(NewScene("main", nil, func() *Element { return Layers(Image(&shifted{}), window) }).
		Issue(func(cmd any) error { issued = append(issued, cmd); return nil }))
	window.pin.stand(window, []spot{{id: 7, placed: true, x: 300, y: 300}}, views(s.root), screen)
	window.pin.each(window, window.parent, func(*instance) {})
	b := answer.Box()
	s.HandleEvents(click(b.TopLeft.Add(geom.NewVec(2, 2))), nil, nil)
	if len(issued) != 1 {
		t.Fatalf("issued %v, want the button's command", issued)
	}
	got := issued[0].(rule.Command).Whom.(entity.Whom).IDs()
	if len(got) != 1 || got[0] != 7 {
		t.Fatalf("the command is for %v, want entity 7", got)
	}
}

func TestScene_ACommandForItOutsideAPinIsNotGiven(t *testing.T) {
	var issued []any
	s := built(NewScene("main", nil, func() *Element {
		return Center(Button("lost", rule.Lift(effect.Effect{}).On(It))).Size(100, 40)
	}).Issue(func(cmd any) error { issued = append(issued, cmd); return nil }))
	s.HandleEvents(click(geom.NewVec(600, 300)), nil, nil)
	if len(issued) != 0 {
		t.Fatalf("issued %v, want nothing: no entity is pinned", issued)
	}
}
