package clock

import (
	"fmt"
	"slices"
)

// Paused, Tempo and Written read the clock, SetPaused and SetTempo set it, for the tests; a game
// goes through the commands.
func (c *Clock) Paused() bool     { return c.state.Paused }
func (c *Clock) Tempo() float32   { return c.state.Tempo }
func (c *Clock) Written() string  { return c.written() }
func (c *Clock) SetPaused(p bool) { c.state.Paused = p; c.tell() }

func (c *Clock) SetTempo(tempo float32) error {
	if !slices.Contains(c.cfg.Tempos, tempo) {
		return fmt.Errorf("clock: tempo %g is not one of %v", tempo, c.cfg.Tempos)
	}
	c.state.Tempo, c.slowed = tempo, false
	c.tell()
	return nil
}
