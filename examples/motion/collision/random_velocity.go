package main

import (
	"math/rand/v2"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugins/world"
)

type randomVelocity struct {
	rng, deadZone, minSpeed int32
}

func newRandomVelocity(rng, deadZone, minSpeed int32) *randomVelocity {
	return &randomVelocity{rng: rng, deadZone: deadZone, minSpeed: minSpeed}
}

func (m *randomVelocity) initialVelocity(index int) world.Velocity {
	dx := rand.Int32N(2*m.rng+1) - m.rng
	dy := rand.Int32N(2*m.rng+1) - m.rng

	if dx >= 0 && dx < m.deadZone {
		dx = m.minSpeed
	} else if dx < 0 && dx > -m.deadZone {
		dx = -m.minSpeed
	}

	var vel world.Velocity
	vel.SetDelta(geom.NewVec(float32(dx), float32(dy)))
	return vel
}
