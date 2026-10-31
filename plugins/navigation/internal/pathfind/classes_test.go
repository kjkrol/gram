package pathfind

import (
	"slices"
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugins/board/cell"
)

func box(side float32) geom.Vec { return geom.NewVec(side, side) }

func TestClasses_EachBodyOnceByItsLimitsAndTheDomainsApart(t *testing.T) {
	walker := Body{Step: 0.5, Height: 2, Size: box(4), Weight: 80}
	wagon := Body{Step: 0.5, Height: 3, Size: geom.NewVec(12, 8), Weight: 900}
	child := Body{Step: 0.5, Height: 1, Size: box(4), Weight: 30}
	climber := Body{Step: 1, Height: 2, Size: box(4), Weight: 80}
	hawk := Body{Height: 1, Size: box(6)}
	var c Classes
	c.Set([]Kind{
		{cell.Land, wagon},
		{cell.Land, walker},
		{cell.Air, hawk},
		{cell.Land, Body{Feet: 7, Step: 0.5, Height: 2, Size: box(4), Weight: 80}},
		{cell.Land, child},
		{cell.Land, climber},
		{cell.Air, hawk},
	})
	if got, want := c.Of(cell.Land), []Body{child, climber, walker, wagon}; !slices.Equal(got,
		want) {
		t.Errorf("the land's classes %v, want %v: each body once, its feet left out, by size, "+
			"height, weight, the step from the highest", got, want)
	}
	if got := c.Of(cell.Air); !slices.Equal(got, []Body{hawk}) {
		t.Errorf("the air's classes %v, want the hawk's alone", got)
	}
	if got := c.Of(cell.Water); len(got) != 0 {
		t.Errorf("the water's classes %v, want none: no kind moves in it", got)
	}
}

func TestClasses_ABodyIsPlannedAsItsOwnClassOrTheNearestHoldingIt(t *testing.T) {
	small := Body{Step: 1, Height: 2, Size: box(4), Weight: 80}
	wide := Body{Step: 1, Height: 2, Size: box(6), Weight: 80}
	heavy := Body{Step: 1, Height: 2, Size: box(5), Weight: 1000}
	var c Classes
	c.Set([]Kind{{cell.Land, small}, {cell.Land, wide}, {cell.Land, heavy}})
	for _, tc := range []struct {
		name string
		body Body
		want Body
		ok   bool
	}{
		{"its own class, on its feet", Body{Feet: 3, Step: 1, Height: 2, Size: box(4),
			Weight: 80}, Body{Feet: 3, Step: 1, Height: 2, Size: box(4), Weight: 80}, true},
		{"a little wider, the nearest holding it", Body{Step: 1, Height: 2, Size: box(5),
			Weight: 80}, wide, true},
		{"a little heavier and narrower", Body{Step: 1, Height: 2, Size: box(4),
			Weight: 900}, heavy, true},
		{"stepping higher, held by its own size", Body{Step: 2, Height: 2, Size: box(4),
			Weight: 80}, small, true},
		{"stepping lower, held by none", Body{Step: 0.5, Height: 2, Size: box(4), Weight: 80},
			Body{Step: 0.5, Height: 2, Size: box(4), Weight: 80}, false},
		{"taller than all, held by none", Body{Step: 1, Height: 3, Size: box(4)},
			Body{Step: 1, Height: 3, Size: box(4)}, false},
	} {
		_, class, ok := c.Class(cell.Land, tc.body)
		planned := c.Planned(cell.Land, tc.body)
		if ok != tc.ok || planned != tc.want || ok && class != (Body{Step: tc.want.Step,
			Height: tc.want.Height, Size: tc.want.Size, Weight: tc.want.Weight}) {
			t.Errorf("%s: class %v %v, planned as %v; want %v planned as %v", tc.name, class, ok,
				planned, tc.ok, tc.want)
		}
	}
	if _, _, ok := c.Class(cell.Air, small); ok || c.Planned(cell.Air, small) != small {
		t.Error("a body of a domain no kind moves in has a class")
	}
}

// twelve bodies of one domain merged to eight, the four nearest pairs: each class no laxer than
// the bodies it is the class of, one of them two bodies' larger limits together
func TestClasses_PastEightTheNearestMergeIntoAClassNoLaxerThanEither(t *testing.T) {
	at := func(weight float32) Body {
		return Body{Step: 1, Height: 2, Size: box(4), Weight: weight}
	}
	lower := at(10)
	lower.Step = 0.9
	wider := at(158)
	wider.Size = box(4.1)
	bodies := []Body{at(10), lower, at(20), at(40), at(41), at(80), at(160), wider, at(320),
		at(640), at(1280), at(1300)}
	var kinds []Kind
	for _, b := range bodies {
		kinds = append(kinds, Kind{cell.Land, b})
	}
	var c Classes
	c.Set(kinds)
	classes := c.Of(cell.Land)
	both := at(160)
	both.Size = wider.Size
	want := []Body{lower, at(20), at(41), at(80), at(320), at(640), at(1300), both}
	if !slices.Equal(classes, want) {
		t.Errorf("the classes %v, want %v", classes, want)
	}
	for _, b := range bodies {
		if k, class, ok := c.Class(cell.Land, b); !ok || !holds(class, b) {
			t.Errorf("%v: class %d %v %v; want one no laxer than it", b, k, class, ok)
		}
	}
}

func TestClasses_SetAgainLeavesNoneOfTheLast(t *testing.T) {
	var c Classes
	c.Set([]Kind{{cell.Land, Body{Size: box(4)}}, {cell.Water, Body{Size: box(8)}}})
	c.Set([]Kind{{cell.Land, Body{Size: box(6)}}})
	if got := c.Of(cell.Land); !slices.Equal(got, []Body{{Size: box(6)}}) ||
		len(c.Of(cell.Water)) != 0 {
		t.Errorf("set again: the land's %v, the water's %v; want the last set's alone", got,
			c.Of(cell.Water))
	}
}
