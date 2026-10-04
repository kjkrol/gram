package render_test

import (
	"testing"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/entity/tag"
	"github.com/kjkrol/gram/render"
)

// place is what the made-up renderer reads of every entity itself: its place in the order given
// and the way it goes. ghost marks one drawn as a ghost, mood is what one feels, marks its tags.
type (
	place struct{ K, Heading int }
	ghost struct{}
	mood  struct{ Angry bool }
	fam   struct{}
)

const hit tag.Tag[fam] = 3

// one is one entity of a drawing test: which way it goes and what it carries.
type one struct {
	heading int
	ghost   bool
	mood    *mood
	hit     bool
}

// draw runs rules over the entities, each starting from sprite 100 plus its place, as a renderer
// reading place itself does, and returns each one's layers and whether it is shown.
func draw(t *testing.T, rules []render.Rule, entities ...one) ([][]render.Appearance, []bool) {
	t.Helper()
	var r render.Rules
	if err := r.Add(rules...); err != nil {
		t.Fatal(err)
	}
	var places goke.Comp[place]
	var ghosts goke.Comp[ghost]
	var moods goke.Comp[mood]
	var marks goke.Comp[tag.Tags[fam]]
	render.Own(&r, &places)
	layers, shown := make([][]render.Appearance, len(entities)), make([]bool, len(entities))
	goke.New().Setup(goke.SystemFn{OnInit: func(si *goke.SysInit) {
		for k, e := range entities {
			comps := []goke.Addable{&places}
			if e.ghost {
				comps = append(comps, &ghosts)
			}
			if e.mood != nil {
				comps = append(comps, &moods)
			}
			if e.hit {
				comps = append(comps, &marks)
			}
			f := si.NewFactory(comps...)
			f.Create(1)
			f.Next()
			places.Slice(&f.Cursor)[0] = place{K: k, Heading: e.heading}
			if e.mood != nil {
				moods.Slice(&f.Cursor)[0] = *e.mood
			}
			if e.hit {
				marks.Slice(&f.Cursor)[0] = tag.Tags[fam](0).With(hit)
			}
		}
		qb := si.NewQueryBuilder(&places)
		r.Bind(qb)
		q := qb.Build()
		for q.All(); q.Next(); {
			cur := q.Cursor()
			ps := places.Slice(cur)
			chunk, seen := make([][]render.Appearance, len(ps)), make([]bool, len(ps))
			for i, p := range ps {
				chunk[i] = []render.Appearance{{SpriteID: render.SpriteID(100 + p.K)}}
			}
			r.Run(cur, chunk, seen)
			for i, p := range ps {
				layers[p.K], shown[p.K] = chunk[i], seen[i]
			}
		}
	}})
	return layers, shown
}

const eastward, other, spook, crown render.SpriteID = 7, 3, 9, 11

// facing picks the sprite from the way the entity goes, read from the renderer's own column.
func facing() render.Rule {
	return render.With(func(a render.Appearance, p place) render.Appearance {
		a.SpriteID = other
		if p.Heading == 1 {
			a.SpriteID = eastward
		}
		return a
	})
}

func TestWith_ReadsAComponentTheRendererReadsItself(t *testing.T) {
	got, _ := draw(t, []render.Rule{facing()}, one{heading: 1}, one{})
	if got[0][0].SpriteID != eastward || got[1][0].SpriteID != other {
		t.Errorf("sprites %v and %v, want %v heading east and %v the other way", got[0][0].SpriteID, got[1][0].SpriteID, eastward, other)
	}
}

// As draws one carrying the component as something else, after the facing has turned it; the
// others keep theirs.
func TestAs_DrawsACarrierAsAnotherSpriteAfterTheRulesBefore(t *testing.T) {
	got, _ := draw(t, []render.Rule{facing(), render.As[ghost](render.Appearance{SpriteID: spook})},
		one{heading: 1, ghost: true}, one{heading: 1})
	if len(got[0]) != 1 || got[0][0].SpriteID != spook {
		t.Errorf("the ghost drawn as %v, want %v alone", got[0], spook)
	}
	if got[1][0].SpriteID != eastward {
		t.Errorf("the other drawn as %v, want %v", got[1][0].SpriteID, eastward)
	}
}

func TestOver_LaysASpriteOnACarrierAlone(t *testing.T) {
	got, _ := draw(t, []render.Rule{render.Over[ghost](render.Appearance{SpriteID: crown})}, one{ghost: true}, one{})
	if len(got[0]) != 2 || got[0][0].SpriteID != 100 || got[0][1].SpriteID != crown {
		t.Errorf("the carrier drawn as %v, want its own sprite under %v", got[0], crown)
	}
	if len(got[1]) != 1 {
		t.Errorf("the other drawn as %v, want its own sprite alone", got[1])
	}
}

// A condition narrows a rule to the carriers it holds for: a tag's In to those carrying the tag.
func TestOver_OnlyWhereItsConditionHolds(t *testing.T) {
	got, _ := draw(t, []render.Rule{render.Over(render.Appearance{SpriteID: crown}, hit.In)}, one{hit: true}, one{})
	if len(got[0]) != 2 || len(got[1]) != 1 {
		t.Errorf("drawn as %v and %v, want the crown on the one hit alone", got[0], got[1])
	}
}

// With reads the component every frame: the sprite follows its value.
func TestWith_ReworksTheSpriteByTheComponentsValue(t *testing.T) {
	angry := render.With(func(a render.Appearance, m mood) render.Appearance {
		if m.Angry {
			a.SpriteID += 50
		}
		return a
	})
	got, _ := draw(t, []render.Rule{angry}, one{mood: &mood{Angry: true}}, one{mood: &mood{}}, one{})
	if got[0][0].SpriteID != 150 || got[1][0].SpriteID != 101 || got[2][0].SpriteID != 102 {
		t.Errorf("sprites %v, %v, %v; want the angry one reworked (150), the calm one and the one with no mood left alone (101, 102)",
			got[0][0].SpriteID, got[1][0].SpriteID, got[2][0].SpriteID)
	}
}

// With a condition reworks the sprite where it holds alone.
func TestWith_ReworksOnlyWhereItsConditionHolds(t *testing.T) {
	bump := render.With(func(a render.Appearance, _ mood) render.Appearance { a.SpriteID += 50; return a },
		func(m mood) bool { return m.Angry })
	got, _ := draw(t, []render.Rule{bump}, one{mood: &mood{Angry: true}}, one{mood: &mood{}})
	if got[0][0].SpriteID != 150 || got[1][0].SpriteID != 101 {
		t.Errorf("sprites %v, %v; want the angry one reworked (150), the calm one left alone (101)", got[0][0].SpriteID, got[1][0].SpriteID)
	}
}

const frozenEast, frozenOther render.SpriteID = 17, 13

// Swap draws one its condition holds for as the twin of the sprite it would be drawn with — after
// the facing, a twin a way faced — one with no twin as it is, and the others as they are.
func TestSwap_DrawsTheTwinOfTheSpriteWhereItsConditionHolds(t *testing.T) {
	twins := map[render.SpriteID]render.SpriteID{eastward: frozenEast, other: frozenOther}
	got, _ := draw(t, []render.Rule{facing(), render.Swap(twins, hit.In)},
		one{heading: 1, hit: true}, one{hit: true}, one{heading: 1})
	if got[0][0].SpriteID != frozenEast || got[1][0].SpriteID != frozenOther {
		t.Errorf("the ones hit drawn as %v and %v, want their twins %v (east) and %v", got[0][0].SpriteID, got[1][0].SpriteID, frozenEast, frozenOther)
	}
	if got[2][0].SpriteID != eastward {
		t.Errorf("the one not hit drawn as %v, want %v, its own", got[2][0].SpriteID, eastward)
	}
	if got, _ = draw(t, []render.Rule{render.Swap(twins, hit.In)}, one{hit: true}); got[0][0].SpriteID != 100 {
		t.Errorf("one hit with no twin for its sprite drawn as %v, want 100, as it is", got[0][0].SpriteID)
	}
}

// Two states compose in the order given: the second Swap finds the first's twin where its table
// lists it, else leaves the first's look.
func TestSwap_TwoStatesComposeInTheOrderGiven(t *testing.T) {
	frozen := map[render.SpriteID]render.SpriteID{100: 30, 101: 31}
	wounded := map[render.SpriteID]render.SpriteID{30: 50, 102: 42} // the frozen first's wounded twin; the third's
	angry := func(m mood) bool { return m.Angry }
	got, _ := draw(t, []render.Rule{render.Swap(frozen, hit.In), render.Swap(wounded, angry)},
		one{hit: true, mood: &mood{Angry: true}}, one{hit: true, mood: &mood{Angry: true}}, one{mood: &mood{Angry: true}})
	if got[0][0].SpriteID != 50 || got[1][0].SpriteID != 31 || got[2][0].SpriteID != 42 {
		t.Errorf("drawn as %v, %v, %v; want 50 (frozen, then its wounded twin), 31 (frozen, no wounded twin of it) and 42 (wounded alone)",
			got[0][0].SpriteID, got[1][0].SpriteID, got[2][0].SpriteID)
	}
}

// Without a Show rule every entity is shown; with one, those it holds for alone.
func TestShow_DrawsOnlyThoseItHoldsFor(t *testing.T) {
	if _, shown := draw(t, nil, one{ghost: true}, one{}); !shown[0] || !shown[1] {
		t.Errorf("shown %v with no Show rule, want both", shown)
	}
	if _, shown := draw(t, []render.Rule{render.Show[ghost]()}, one{ghost: true}, one{}); !shown[0] || shown[1] {
		t.Errorf("shown %v, want the ghost alone", shown)
	}
	if _, shown := draw(t, []render.Rule{render.Show(hit.In)}, one{hit: true}, one{}); !shown[0] || shown[1] {
		t.Errorf("shown %v, want the one hit alone", shown)
	}
}

func TestRules_RefuseOnesAddedOnceBound(t *testing.T) {
	var r render.Rules
	goke.New().Setup(goke.SystemFn{OnInit: func(si *goke.SysInit) {
		var places goke.Comp[place]
		r.Bind(si.NewQueryBuilder(&places))
	}})
	if err := r.Add(facing()); err == nil {
		t.Error("a rule added after Bind taken, want it refused")
	}
}
