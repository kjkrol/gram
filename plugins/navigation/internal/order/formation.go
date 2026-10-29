package order

// Formation is how a March lays its group out and paces it, kept by value on every order it
// gives: the shape the group walks in, how far apart, and whether its head waits for the rest.
type Formation struct {
	Shape Shape
	// Spacing is how far apart along the formation its members walk, in bodies, each with the
	// gap kept beside a body; 0 reads as 1.
	Spacing float32
	// Closed has the head wait while a member lags more than Slack+1 spacings behind the one
	// ahead of it; open, the formation may break up, each member still at the pace of that one.
	Closed bool
	Slack  uint8
}

// Shape is the form a formation takes, the zero Shape none.
type Shape uint8

const (
	NoFormation Shape = iota
	// Column is single file: each member behind the one nearer the goal along the way there.
	Column
)

// ColumnOf is a column whose members walk spacing bodies apart; closed, its head waits while one
// lags more than slack+1 spacings behind the one ahead of it.
func ColumnOf(spacing float32, closed bool, slack uint8) Formation {
	return Formation{Shape: Column, Spacing: spacing, Closed: closed, Slack: slack}
}

// Apart is how many bodies, each with its gap, a member keeps behind the one ahead of it.
func (f Formation) Apart() float32 {
	if f.Spacing <= 0 {
		return 1
	}
	return f.Spacing
}
