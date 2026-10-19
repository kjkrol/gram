package clock

import (
	"fmt"
	"slices"
)

func (c *Clock) Paused() bool     { return c.state.Paused }
func (c *Clock) Tempo() float32   { return c.state.Tempo }
func (c *Clock) Written() string  { return c.written() }
func (c *Clock) SetPaused(p bool) { c.state.Paused = p; c.tell() }

// SetExactTempo is SetTempo refusing a tempo not in the list, for the tests that set exact ones
func (c *Clock) SetExactTempo(tempo float32) error {
	if !slices.Contains(c.cfg.Tempos, tempo) {
		return fmt.Errorf("clock: tempo %g is not one of %v", tempo, c.cfg.Tempos)
	}
	c.SetTempo(tempo)
	return nil
}
