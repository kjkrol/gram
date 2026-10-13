package world

import (
	"fmt"
	"time"

	"github.com/kjkrol/ceremony"
	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/ceremony/cond"
	"github.com/kjkrol/ceremony/effect"
	"github.com/kjkrol/ceremony/tag"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/uid"
)

type effectCommands struct {
	queue    command.Queue[ceremony.Command]
	triggers command.Queue[ceremony.Trigger]
	sourced  []ceremony.Command
	known    []ceremony.Command
	checked  bool
	effects  *effect.Effects
	world    func() uid.UID64
	commands *command.Carrier
	flips    []flip
	found    []uid.UID64

	labelled *goke.Query
	label    goke.Comp[ceremony.Label]
	plays    *goke.Query
	playsC   goke.Comp[tag.Tags[cond.Roles]]
}

// a step's commands of one effect on one entity, netted before they are carried out
type flip struct {
	id            uid.UID64
	e             effect.Effect
	was, on, cast bool
	lasts         time.Duration
}

func (c *effectCommands) take(cmds ...ceremony.Command) error {
	for _, cmd := range cmds {
		if cmd.Whom == nil {
			return fmt.Errorf("world: %v names nobody: say whom with On", cmd)
		}
		if !cmd.Source.Nobody() {
			c.sourced = append(c.sourced, cmd)
		}
		c.known = append(c.known, cmd)
	}
	return nil
}

// RolesOf is the roles id plays, a bit each.
func (c *effectCommands) RolesOf(id uid.UID64) uint64 {
	if c.plays == nil || !c.plays.Seek(id) {
		return 0
	}
	return uint64(*c.playsC.At(c.plays.Cursor()))
}

func (c *effectCommands) system() goke.System {
	return goke.SystemFn{
		OnInit: func(si *goke.SysInit) {
			c.labelled = si.NewQueryBuilder(&c.label).Build()
			c.plays = si.NewQueryBuilder(&c.playsC).Build()
		},
		OnUpdate: func(cb *goke.CmdBuf, _ time.Duration) {
			if !c.checked {
				c.checked = true
				c.check()
			}
			c.triggers.Drain(func(i command.Issued[ceremony.Trigger]) {
				if !i.ByEntity || !c.labelled.Seek(i.Entity) {
					return
				}
				who := *c.label.At(c.labelled.Cursor())
				for _, cmd := range c.sourced {
					if !cmd.Source.Holds(who) {
						continue
					}
					if _, mine := cmd.Whom.(ceremony.Whom); mine {
						c.carry(cmd)
					} else {
						c.commands.PutFrom(i.Entity, cmd)
					}
				}
			})
			c.queue.Drain(func(i command.Issued[ceremony.Command]) { c.carry(i.Command) })
			for _, f := range c.flips {
				switch {
				case f.on && (f.cast || !f.was) && f.lasts > 0:
					c.effects.CastFor(cb, f.id, f.e, f.lasts)
				case f.on && (f.cast || !f.was):
					c.effects.Cast(cb, f.id, f.e)
				case !f.on && f.was:
					c.effects.Dispel(f.id, f.e)
				}
			}
			c.flips = c.flips[:0]
		},
	}
}

func (c *effectCommands) carry(cmd ceremony.Command) {
	whom, ok := cmd.Whom.(ceremony.Whom)
	if !ok || cmd.Effect == (effect.Effect{}) {
		return
	}
	c.found = c.found[:0]
	if whom.IsWorld() {
		c.found = append(c.found, c.world())
	}
	c.found = append(c.found, whom.IDs()...)
	if !whom.IsWorld() && len(whom.IDs()) == 0 {
		c.each(func(id uid.UID64, l ceremony.Label) {
			if whom.Holds(l) {
				c.found = append(c.found, id)
			}
		})
	}
	on := cmd.Verb == ceremony.Puts
	if cmd.Verb == ceremony.Toggles {
		on = true
		for _, id := range c.found {
			if c.flipOf(id, cmd.Effect).on {
				on = false
				break
			}
		}
	}
	for _, id := range c.found {
		f := c.flipOf(id, cmd.Effect)
		f.on, f.cast, f.lasts = on, on, cmd.Lasts
	}
}

func (c *effectCommands) flipOf(id uid.UID64, e effect.Effect) *flip {
	for i := range c.flips {
		if c.flips[i].id == id && c.flips[i].e == e {
			return &c.flips[i]
		}
	}
	on := c.effects.Has(id, e)
	c.flips = append(c.flips, flip{id: id, e: e, was: on, on: on})
	return &c.flips[len(c.flips)-1]
}

func (c *effectCommands) each(fn func(id uid.UID64, l ceremony.Label)) {
	for c.labelled.All(); c.labelled.Next(); {
		cur := c.labelled.Cursor()
		for i, l := range c.label.Slice(cur) {
			fn(cur.IDs[i], l)
		}
	}
}

func (c *effectCommands) check() {
	names := map[uint64]int{}
	var labels []ceremony.Label
	c.each(func(_ uid.UID64, l ceremony.Label) {
		labels = append(labels, l)
		if l.Name != 0 {
			names[l.Name]++
		}
	})
	said := func(w ceremony.Whom, cmd ceremony.Command) {
		w.Each(func(what string, borne func(ceremony.Label) bool) {
			n := 0
			for _, l := range labels {
				if borne(l) {
					n++
				}
			}
			if n == 0 {
				panic(fmt.Sprintf("world: %v says %s, which nobody is called", cmd, what))
			}
		})
	}
	for _, cmd := range c.known {
		if whom, ok := cmd.Whom.(ceremony.Whom); ok {
			said(whom, cmd)
		}
		said(cmd.Source, cmd)
	}
	for _, n := range names {
		if n > 1 {
			panic(fmt.Sprintf("world: %d entities bear one name; a name is one entity's, a group many's", n))
		}
	}
}
