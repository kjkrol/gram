package main

import (
	"log"
	"os"
	"strconv"

	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/plugins/cameras"
)

// PROBE_PICTURE at the second second: "whole" draws the tactical picture whole at PROBE_RES of
// the pixels (1), "layers" in the preset's layers, both still (no following of the budget);
// unset leaves the demo as it starts. PROBE_TAB=1 presses Tab a tick later (the perspective
// from a free camera).
func (p *probe) play() {
	switch p.tick {
	case 120:
		pic := os.Getenv("PROBE_PICTURE")
		if pic == "" {
			return
		}
		res := float32(1)
		if v, err := strconv.ParseFloat(os.Getenv("PROBE_RES"), 32); err == nil {
			res = float32(v)
		}
		put := func(c command.Command) { p.a.world.Write().Carrier().Put(command.Nobody, c) }
		put(cameras.Adapt{Profile: cameras.Tactical, Strategy: cameras.Still})
		put(cameras.Layers{Profile: cameras.Tactical, On: pic == "layers"})
		if pic == "whole" {
			put(cameras.Picture{Profile: cameras.Tactical, Resolution: res})
		}
		log.Printf("P:t%d picture %s res %.2f", p.tick, pic, res)
	case 130:
		if os.Getenv("PROBE_TAB") != "1" {
			return
		}
		ev := &control.InputEvents{}
		ev.AddKeyEvent(control.KeyTab, control.ActionPress)
		ev.AddKeyEvent(control.KeyTab, control.ActionRelease)
		p.a.players.EventHandler().HandleEvents(ev)
		log.Printf("P:t%d Tab", p.tick)
	}
}
