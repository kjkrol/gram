package rule

import (
	"errors"
	"testing"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/entity/tag"
	"github.com/kjkrol/uid"
)

// roles is the family of tags this test names; hunter and hunted are two of its bits.
type roles struct{}

const (
	hunter tag.Tag[roles] = iota
	hunted
)

type hostBody struct{ N int }

// sighting is a made-up host's description of a pair: who looks at whom.
type sighting struct{ from, to uid.UID64 }

// hostOf builds a host with rules registered, a hunter, a hunted, and what each carries.
func hostOf(t *testing.T, rules ...Rule) (h *PairHost[sighting], hunterMarks, huntedMarks Marks, pair sighting) {
	t.Helper()
	h = &PairHost[sighting]{}
	for _, b := range rules {
		if err := h.Add(b); err != nil {
			t.Fatalf("Add: %v", err)
		}
	}

	goke.New().Setup(goke.SystemFn{OnInit: func(si *goke.SysInit) {
		var bodies goke.Comp[hostBody]
		var tags goke.Comp[tag.Tags[roles]]
		f := si.NewFactory(&bodies, &tags)
		f.Create(2)
		f.Next()
		pair = sighting{from: f.IDs[0], to: f.IDs[1]}
		tags.Slice(&f.Cursor)[0] = tag.Tags[roles](0).With(hunter)
		tags.Slice(&f.Cursor)[1] = tag.Tags[roles](0).With(hunted)

		var walked, sought goke.Comp[hostBody]
		walk, seek := si.NewQueryBuilder(&walked), si.NewQueryBuilder(&sought)
		h.Bind(walk, seek)
		walking, seeking := walk.Build(), seek.Build()

		for walking.All(); walking.Next(); {
			for i, id := range walking.Cursor().IDs {
				if id == pair.from {
					hunterMarks = h.InChunk(0, walking.Cursor(), i)
				}
			}
		}
		if !seeking.Seek(pair.to) {
			t.Fatal("the hunted entity could not be sought")
		}
		huntedMarks = h.At(1, seeking.Cursor())
	}})
	return h, hunterMarks, huntedMarks, pair
}

func recording(into *[]sighting) Rule {
	return pairRule(hunter, hunted, func(_ Tick, s sighting) { *into = append(*into, s) })
}

func TestPairHost_Dispatch_KeepsTheDirection(t *testing.T) {
	var got []sighting
	h, hunterMarks, huntedMarks, pair := hostOf(t, recording(&got))

	h.Dispatch(Tick{}, hunterMarks, huntedMarks, pair)
	h.Dispatch(Tick{}, huntedMarks, hunterMarks, sighting{from: pair.to, to: pair.from})

	if len(got) != 1 || got[0] != pair {
		t.Errorf("the rule ran for %+v, want just the hunter looking at the hunted: %+v", got, pair)
	}
}

// A pair with no direction is handed over whichever way round the tags fit.
func TestPairHost_DispatchEitherWay_FindsTheFit(t *testing.T) {
	var got []sighting
	h, hunterMarks, huntedMarks, pair := hostOf(t, recording(&got))
	reversed := sighting{from: pair.to, to: pair.from}

	h.DispatchEitherWay(Tick{}, huntedMarks, hunterMarks, reversed, pair)

	if len(got) != 1 || got[0] != pair {
		t.Errorf("the rule ran for %+v, want it handed the pair with the hunter as Self: %+v", got, pair)
	}
}

// The same tag on both sides matches either way round, and runs once.
func TestPairHost_DispatchEitherWay_SameTagRunsOnce(t *testing.T) {
	var got []sighting
	h, hunterMarks, _, pair := hostOf(t, pairRule(hunter, hunter, func(_ Tick, s sighting) { got = append(got, s) }))

	h.DispatchEitherWay(Tick{}, hunterMarks, hunterMarks, pair, sighting{from: pair.to, to: pair.from})

	if len(got) != 1 {
		t.Errorf("the rule ran %d times, want once", len(got))
	}
}

func TestPairHost_Any_TakesWhateverIsThere(t *testing.T) {
	var got []sighting
	h, hunterMarks, _, pair := hostOf(t, pairRule(tag.Any, hunter, func(_ Tick, s sighting) { got = append(got, s) }))

	h.Dispatch(Tick{}, Marks{}, hunterMarks, pair)
	h.Dispatch(Tick{}, hunterMarks, Marks{}, pair)

	if len(got) != 1 {
		t.Errorf("the rule ran %d times, want once: Any on the left, a hunter on the right", len(got))
	}
}

func TestPairHost_Add_RefusesARuleMadeForAnotherHost(t *testing.T) {
	var h PairHost[sighting]
	stranger := pairRule(hunter, hunted, func(Tick, string) {})

	if err := h.Add(stranger); !errors.Is(err, ErrUnhosted) {
		t.Errorf("Add = %v, want ErrUnhosted", err)
	}
}

// Once the host's queries exist a rule could no longer join them.
func TestPairHost_Add_RefusesOneThatComesAfterBind(t *testing.T) {
	var got []sighting
	h, _, _, _ := hostOf(t)

	if err := h.Add(recording(&got)); !errors.Is(err, ErrHostBuilt) {
		t.Errorf("Add after Bind = %v, want ErrHostBuilt", err)
	}
}

// group is a made-up host's description of one Self against many Others.
type group struct{ others []int }

func TestPairHost_DispatchGrouped_HandsOverTheOthersThatFit(t *testing.T) {
	_, hunterMarks, huntedMarks, _ := hostOf(t, recording(new([]sighting)))
	var got []group
	var h PairHost[group]
	if err := h.Add(pairRule(hunter, hunted, func(_ Tick, g group) {
		got = append(got, group{others: append([]int(nil), g.others...)})
	})); err != nil {
		t.Fatalf("Add: %v", err)
	}
	build := func(matched []int) group { return group{others: matched} }

	h.DispatchGrouped(Tick{}, hunterMarks, []Marks{huntedMarks, {}, huntedMarks}, build)
	h.DispatchGrouped(Tick{}, hunterMarks, nil, build)
	h.DispatchGrouped(Tick{}, huntedMarks, []Marks{huntedMarks}, build)

	if len(got) != 2 {
		t.Fatalf("the rule ran %d times, want twice — once with prey in view, once with none", len(got))
	}
	if len(got[0].others) != 2 || got[0].others[0] != 0 || got[0].others[1] != 2 {
		t.Errorf("first group = %v, want others 0 and 2 — the one in between carries no tag", got[0].others)
	}
	if len(got[1].others) != 0 {
		t.Errorf("second group = %v, want it empty", got[1].others)
	}
}

// The host reads every family its rules name, so a payload can ask about any of their tags.
func TestCarries_AnswersForTheFamiliesTheHostNames(t *testing.T) {
	h, hunterMarks, huntedMarks, _ := hostOf(t, pairRule(hunter, tag.Any, func(Tick, sighting) {}))

	if !huntedMarks.Carries(hunted) || hunterMarks.Carries(hunted) {
		t.Errorf("Carries(hunted) = %v for the hunted and %v for the hunter, want true and false",
			huntedMarks.Carries(hunted), hunterMarks.Carries(hunted))
	}
	if (Marks{}).Carries(hunted) {
		t.Error("empty Marks carry a tag")
	}
	type other struct{}
	defer func() {
		if recover() == nil {
			t.Error("Carries about a family no rule names returned quietly, want a panic naming the fix")
		}
	}()
	hunterMarks.Carries(tag.Tag[other](0))
	_ = h
}

func TestTags_WithAndWithout(t *testing.T) {
	var s tag.Tags[roles]
	s = s.With(hunter, hunted)
	if !s.Has(hunter) || !s.Has(hunted) {
		t.Fatalf("With set %b, want both bits", s)
	}
	s = s.Without(hunter)
	if s.Has(hunter) || !s.Has(hunted) {
		t.Errorf("Without left %b, want only hunted", s)
	}
}
