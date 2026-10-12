package pathfind

// Ended is the level of its goal the last route Find found ended on.
func (p *Finder) Ended() Level {
	if p.ended == deck {
		return Deck
	}
	return Ground
}
