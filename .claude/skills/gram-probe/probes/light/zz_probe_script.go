package main

import (
	"log"

	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/gram/plugins/light"
)

// the lamps' shadows switched off at the fifth second, the command put through the carrier
// (a setting on the settings screen has no key)
func (p *probe) play() {
	if p.tick != 300 {
		return
	}
	p.a.world.Carrier().Put(command.Nobody, light.LampShadows{Count: 0})
	log.Printf("P:t%d LampShadows{0}", p.tick)
}
