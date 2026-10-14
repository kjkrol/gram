package render

import "time"

var spent struct{ update, draw, present time.Duration }

// SetSpent notes where a frame's time went on average over the last second: its tick, its
// drawing and its presenting.
func SetSpent(update, draw, present time.Duration) {
	spent.update, spent.draw, spent.present = update, draw, present
}

// Spent is where a frame's time went on average over the last second.
func Spent() (update, draw, present time.Duration) {
	return spent.update, spent.draw, spent.present
}
