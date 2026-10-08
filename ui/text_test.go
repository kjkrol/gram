package ui

import (
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/uid"
)

// named says each pinned entity's name, and nothing for one it does not know.
type named map[uid.UID64]string

func (n named) Text(of uid.UID64, pinned bool) (string, bool) {
	s, ok := n[of]
	return s, ok && pinned
}

// silent says nothing.
type silent struct{}

func (silent) Text(uid.UID64, bool) (string, bool) { return "", false }

func TestText_APinnedLabelSaysWhatItsEntityIs(t *testing.T) {
	label := LabelOf(named{1: "Ann", 2: "Bartholomew"}).On(nobody)
	root := Layers(Image(&shifted{}), label)
	standing(root, label,
		spot{id: 1, placed: true, x: 100, y: 100},
		spot{id: 2, placed: true, x: 400, y: 100},
		spot{id: 3, placed: true, x: 700, y: 100})
	widths := map[uid.UID64]float64{}
	label.pin.each(label, label.parent, func(in *instance) {
		w, _ := size(label.Box())
		widths[in.id] = w
	})
	if widths[1] == 0 || widths[2] <= widths[1] {
		t.Fatalf("widths %v: want Ann's narrower than Bartholomew's", widths)
	}
	if widths[3] != 0 {
		t.Errorf("an entity the label says nothing for has it %v wide, want left out", widths[3])
	}
}

func TestText_AnElementLeftOutTakesNoRoomAndIsNotHit(t *testing.T) {
	var issued []any
	next := Button("next", order{"next"})
	s := built(NewScene("main", TopLeft(Rows(Fit(ButtonOf(silent{}, order{"lost"})), Fit(next)))).
		Issue(func(cmd any) error { issued = append(issued, cmd); return nil }))
	if top := next.Box().TopLeft.Y; top != 0 {
		t.Fatalf("the button after one left out stands at %v, want at the top", top)
	}
	s.HandleEvents(click(geom.NewVec(2, 2)), nil, nil)
	if len(issued) != 1 || issued[0] != (order{"next"}) {
		t.Fatalf("issued %v, want the second button's command alone", issued)
	}
}

// answer is a command of a plugin's own for the entity the element giving it is pinned to.
type answer struct{ to uid.UID64 }

func (a answer) About(of uid.UID64) any { a.to = of; return a }

func TestPin_AButtonGivesAnAboutCommandForTheEntityItIsShownFor(t *testing.T) {
	var issued []any
	hi := Button("hi", answer{}).Named("hi")
	window := Window("host", hi).On(nobody)
	s := built(NewScene("main", Layers(Image(&shifted{}), window)).
		Issue(func(cmd any) error { issued = append(issued, cmd); return nil }))
	window.pin.stand(window, []spot{{id: 7, placed: true, x: 300, y: 300}}, views(s.root), screen)
	window.pin.each(window, window.parent, func(*instance) {})
	s.HandleEvents(click(hi.Box().TopLeft.Add(geom.NewVec(2, 2))), nil, nil)
	if len(issued) != 1 || issued[0] != (answer{to: 7}) {
		t.Fatalf("issued %v, want the answer to entity 7", issued)
	}
}

func TestScene_AnAboutCommandOutsideAPinIsNotGiven(t *testing.T) {
	var issued []any
	s := built(NewScene("main", Center(Button("lost", answer{})).Size(100, 40)).Issue(func(cmd any) error { issued = append(issued, cmd); return nil }))
	s.HandleEvents(click(geom.NewVec(600, 300)), nil, nil)
	if len(issued) != 0 {
		t.Fatalf("issued %v, want nothing: no entity is pinned", issued)
	}
}
