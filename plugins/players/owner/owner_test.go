package owner_test

import (
	"testing"

	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/plugins/players/owner"
	"github.com/kjkrol/gram/plugins/world/entity/tag"
)

// A player's tag is its id less one; nobody and past the family's size own nothing.
func TestOf_IsTheIdLessOneAndRefusesNobody(t *testing.T) {
	if owner.Of(1) != 0 || owner.Of(64) != 63 {
		t.Errorf("Of(1), Of(64) = %v, %v; want 0, 63", owner.Of(1), owner.Of(64))
	}
	for _, id := range []control.PlayerID{control.Nobody, 65} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("Of(%d) did not refuse", id)
				}
			}()
			owner.Of(id)
		}()
	}
}

// An owned entity obeys its owners alone, one shared among players each of them, an ownerless one
// nobody alone.
func TestObeys_OwnersAloneAndNobodyTheOwnerless(t *testing.T) {
	mine := tag.Tags[owner.Family](0).With(owner.Of(1))
	shared := mine.With(owner.Of(2))
	for _, c := range []struct {
		owners tag.Tags[owner.Family]
		by     control.PlayerID
		want   bool
	}{
		{mine, 1, true}, {mine, 2, false}, {mine, control.Nobody, false}, {mine, 200, false},
		{shared, 1, true}, {shared, 2, true}, {shared, 3, false},
		{0, control.Nobody, true}, {0, 1, false},
	} {
		if got := owner.Obeys(c.owners, c.by); got != c.want {
			t.Errorf("owners %b obey player %d: %v, want %v", c.owners, c.by, got, c.want)
		}
	}
}

// Two entities are allies when they share an owner, or when nobody owns either.
func TestAllies_ShareAnOwnerOrAreBothNobodys(t *testing.T) {
	one := tag.Tags[owner.Family](0).With(owner.Of(1))
	two := tag.Tags[owner.Family](0).With(owner.Of(2))
	both := one.With(owner.Of(2))
	for _, c := range []struct {
		a, b tag.Tags[owner.Family]
		want bool
	}{{one, one, true}, {one, two, false}, {both, two, true}, {0, 0, true}, {0, one, false}} {
		if got := owner.Allies(c.a, c.b); got != c.want {
			t.Errorf("Allies(%b, %b) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}
