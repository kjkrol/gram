package order

import "testing"

func TestTouch_TheRulesOfTheCrowd(t *testing.T) {
	moving := Touch{Moving: true, LastGoal: true}
	for _, c := range []struct {
		name                                       string
		touch                                      Touch
		pushed, reached, clears, blocks, goalTaken bool
	}{
		{"an idle ally pushed", Touch{OtherMoving: true, Ally: true}, true, false, false, false, false},
		{"an idle stranger pushed", Touch{OtherMoving: true}, false, false, false, false, false},
		{"one of the group arrived, pushed by its own", Touch{OtherMoving: true, Ally: true, Groupmate: true}, false, false, false, false, false},
		{"pushed by one giving way", Touch{OtherMoving: true, OtherGivingWay: true, Ally: true}, false, false, false, false, false},
		{"on the move at one of its group arrived", with(moving, func(t *Touch) { t.Ally, t.Groupmate = true, true }), false, true, false, false, false},
		{"on the move, not at its last goal, at one of its group", with(moving, func(t *Touch) { t.Ally, t.Groupmate, t.LastGoal = true, true, false }), false, false, false, true, false},
		{"on the move at an idle ally with room", with(moving, func(t *Touch) { t.Ally, t.Room = true, true }), false, false, true, false, false},
		{"on the move at an idle ally with no room", with(moving, func(t *Touch) { t.Ally = true }), false, false, false, true, false},
		{"on the move at an idle ally with no room on its goal", with(moving, func(t *Touch) { t.Ally, t.OnMyGoal = true, true }), false, false, false, true, true},
		{"on the move at an idle ally with room on its goal", with(moving, func(t *Touch) { t.Ally, t.OnMyGoal, t.Room = true, true, true }), false, false, true, false, false},
		{"on the move at a stranger on its goal", with(moving, func(t *Touch) { t.OnMyGoal = true }), false, false, false, true, true},
		{"on the move at one on the move", with(moving, func(t *Touch) { t.OtherMoving, t.Ally = true, true }), false, false, false, true, false},
	} {
		got := [5]bool{c.touch.pushedByAlly(), c.touch.reachedTheGroup(), c.touch.clearsTheWay(), c.touch.blocks(), c.touch.goalTaken()}
		if want := [5]bool{c.pushed, c.reached, c.clears, c.blocks, c.goalTaken}; got != want {
			t.Errorf("%s: pushed, reached, clears, blocks, goal taken %v, want %v", c.name, got, want)
		}
	}
	head := Touch{Self: 1, Other: 2, Moving: true, OtherMoving: true, HeadOn: true}
	if !head.waitsFirst() || (Touch{Self: 2, Other: 1, Moving: true, OtherMoving: true, HeadOn: true}).waitsFirst() {
		t.Error("of two head on the lower id does not wait first, or the other does too")
	}
	if head.WaitedOut = true; head.waitsFirst() {
		t.Error("the first waits on after its Hold ran out")
	}
}

func with(t Touch, fn func(*Touch)) Touch {
	fn(&t)
	return t
}
