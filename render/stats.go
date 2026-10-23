package render

import "time"

var spent struct{ update, draw, present time.Duration }

var rates struct{ fps float64 }

// SetRates notes the frames the engine drew in the last second.
func SetRates(fps float64) { rates.fps = fps }

// ActualFPS is how many frames the engine drew in the last second.
func ActualFPS() float64 { return rates.fps }

// SetSpent notes where a frame's time went on average over the last second: its tick, its
// drawing and its presenting.
func SetSpent(update, draw, present time.Duration) {
	spent.update, spent.draw, spent.present = update, draw, present
}

// Spent is where a frame's time went on average over the last second.
func Spent() (update, draw, present time.Duration) {
	return spent.update, spent.draw, spent.present
}
