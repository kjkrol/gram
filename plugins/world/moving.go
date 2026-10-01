package world

import "github.com/kjkrol/uid"

// Moving is what a rule hosted by the VelocitySystem gets, every tick, for every entity:
// scale Base.Vel.Value to slow or stop it, after Steering has written the base speed.
type Moving struct {
	ID   uid.UID64
	Base *Base
}

// Who is the entity moving: whose moment it is, for a rule.
func (m Moving) Who() uid.UID64 { return m.ID }
