package ui

import (
	"testing"

	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/render/gputest"
	"github.com/kjkrol/uid"
)

func drawingOf(s *Scene) *drawing { return s.Drawers()[0].(*drawing) }

// frame publishes what d's scene draws, to be drawn next
func frame(d *drawing) {
	d.Publish()
	d.Swap()
}

func wordsOf(e *Element) string {
	w, _ := e.content.(*label).words(e)
	return w
}

func TestScene_DrawsTheWordsTheControlsAndWhatIsHiddenAsPublished(t *testing.T) {
	words := &changing{words: "one"}
	on := false
	text, box, note := LabelOf(words), Checkbox("box", func() bool { return on }, nil),
		Label("note")
	d := drawingOf(NewScene("main", Rows(Fit(text), Fit(box), Fit(note))))
	words.words = "two"
	if wordsOf(text) != "two" {
		t.Fatalf("before any publish the label says %q, want what it reads now", wordsOf(text))
	}
	frame(d)
	words.words, on = "three", true
	note.Hide()
	if wordsOf(text) != "two" || box.content.(*checkbox).ticked(box) || note.shut() {
		t.Errorf("changed after the frame was published, the label says %q, the box ticked %v, "+
			"the note hidden %v; want them as published till the next", wordsOf(text),
			box.content.(*checkbox).ticked(box), note.shut())
	}
	frame(d)
	if wordsOf(text) != "three" || !box.content.(*checkbox).ticked(box) || !note.shut() {
		t.Errorf("published again, the label says %q, the box ticked %v, the note hidden %v",
			wordsOf(text), box.content.(*checkbox).ticked(box), note.shut())
	}
}

func TestScene_PublishesAPinnedLabelsWordsForEachEntityAndWhoShowsIt(t *testing.T) {
	name := LabelOf(named{1: "bob", 2: "ann"}).On(nobody)
	root, _, _ := halves(name)
	d := drawingOf(NewScene("main", root))
	d.pins = []*Element{name}
	b := d.published.Back()
	b.reset()
	b.spots[name] = []spot{{id: 1}, {id: 2}}
	d.showing(b, root)
	d.read(b, root, nil, false)
	d.Swap()
	for id, want := range map[uid.UID64]string{1: "bob", 2: "ann", 3: ""} {
		name.drawnFor(id, true)
		if got := wordsOf(name); got != want {
			t.Errorf("drawn for entity %d the label says %q, want %q", id, got, want)
		}
	}
	vs := views(root)
	if !vs[0].shows(1) || vs[0].shows(2) || !vs[1].shows(2) || vs[1].shows(1) {
		t.Errorf("the left half shows 1 %v, 2 %v, the right 2 %v, 1 %v; want each its own",
			vs[0].shows(1), vs[0].shows(2), vs[1].shows(2), vs[1].shows(1))
	}
}

func TestPin_GoToItMovesTheCameraAtThePublishAfterTheFrameDrawn(t *testing.T) {
	red := &looking{}
	plague := Window("plague").On(nobody).OffScreen(GoToIt)
	root := Layers(Image(&shifted{}).Input(red), plague)
	d := drawingOf(NewScene("main", root))
	frame(d)
	standing(root, plague, spot{id: 7, placed: true, x: 5000, y: 5000})
	if len(red.looked) != 0 {
		t.Fatalf("laid, the camera was moved at once to %v, want it moved as the game publishes",
			red.looked)
	}
	frame(d)
	frame(d)
	if len(red.looked) != 1 || red.looked[0] != 7 {
		t.Errorf("the camera was asked to look at %v, want entity 7 once", red.looked)
	}
}

func TestScene_DrawsAsTheBarrierLaidIt(t *testing.T) {
	gputest.Need(t)
	s := NewScene("main", TopLeft(Label("x")))
	d := drawingOf(s)
	d.Lay(200, 100)
	screen := render.NewImage(400, 300)
	defer screen.Deallocate()
	d.Draw(screen)
	if got := s.Laid(); got != box(0, 0, 200, 100) {
		t.Errorf("laid on the barrier, the scene drawn is laid over %v, want 200 by 100", got)
	}
	d.Draw(screen)
	if got := s.Laid(); got != box(0, 0, 400, 300) {
		t.Errorf("drawn with no barrier, the scene is laid over %v, want the screen's 400 by 300",
			got)
	}
}

func TestScene_ATreePutInAfterThePublishIsPublishedAsTheBarrierLaysIt(t *testing.T) {
	s := NewScene("main", TopLeft(Label("old")))
	d := drawingOf(s)
	frame(d)
	words := &changing{words: "put in"}
	text := LabelOf(words)
	s.Replace(TopLeft(text))
	d.Lay(200, 100)
	words.words = "ticked"
	if got := wordsOf(text); got != "put in" {
		t.Errorf("put in after the publish and laid, the label says %q, want %q: read on the "+
			"barrier, not as it is drawn", got, "put in")
	}
	frame(d)
	d.Lay(200, 100)
	if got := wordsOf(text); got != "ticked" {
		t.Errorf("published again, the label says %q, want %q", got, "ticked")
	}
}
