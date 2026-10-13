package bullet

import (
	"math"
	"time"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony/host"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugins/board/ground"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/uid"
)

// landBack lays a shot short of the first touch, so its box overlaps nothing.
const landBack = 0.01

const edgeEps = 1e-6

// rangeEps absorbs a step's rounding short of the Range.
const rangeEps = 1e-3

var _ goke.System = (*flightSystem)(nil)

type flightSystem struct {
	w        *world.Plugin
	space    *aabbworld.Space
	landings *host.Rules[Landing]
	restings *host.Rules[Resting]
	groundOf func() ground.Heights
	tickOf   host.TickSource

	flights *goke.Query
	base    goke.Comp[world.Base]
	flight  goke.Comp[Flight]
	body    goke.Comp[Body]
	coll    goke.OptComp[collision.Collider]
	sweep   goke.OptComp[collision.Sweep]
	z       goke.OptComp[world.Z]

	resting *goke.Query // a host a query
	rbase   goke.Comp[world.Base]
	rflight goke.Comp[Flight]
	rbody   goke.Comp[Body]

	sweepID, collID, outsideID goke.CompID

	walking struct {
		ids     []uid.UID64
		flights []Flight
		bodies  []Body
		ends    []bool
	}
	landingAt func(i int) Landing
	landsAt   func(i int) bool
	restingAt func(i int) Resting
	restsAt   func(i int) bool
}

func newFlightSystem(w *world.Plugin, landings *host.Rules[Landing], restings *host.Rules[Resting], groundOf func() ground.Heights) *flightSystem {
	s := &flightSystem{w: w, space: w.Space(), landings: landings, restings: restings, groundOf: groundOf, tickOf: w.Tick}
	s.landingAt, s.landsAt, s.restingAt, s.restsAt = s.landing, s.lands, s.rest, s.rests
	return s
}

func (s *flightSystem) Init(si *goke.SysInit) {
	s.sweepID, s.collID, s.outsideID = si.RegComp[collision.Sweep](), si.RegComp[collision.Collider](), si.RegComp[world.Outside]()
	qb := si.NewQueryBuilder(&s.base, &s.flight, &s.body).Optional(&s.coll).Optional(&s.sweep).Optional(&s.z)
	s.landings.Bind(qb)
	s.flights = qb.Build()
	rb := si.NewQueryBuilder(&s.rbase, &s.rflight, &s.rbody)
	s.restings.Bind(rb)
	s.resting = rb.Build()
}

func (s *flightSystem) Update(cb *goke.CmdBuf, dt time.Duration) {
	t := s.tickOf.Of(cb, dt)
	if !s.restings.Empty() {
		for s.resting.All(); s.resting.Next(); {
			cur := s.resting.Cursor()
			s.walking.ids, s.walking.flights, s.walking.bodies = cur.IDs, s.rflight.Slice(cur), s.rbody.Slice(cur)
			s.restings.RunWhere(t, cur, s.restsAt, s.restingAt)
		}
	}
	for s.flights.All(); s.flights.Next(); {
		cur := s.flights.Cursor()
		bases, flights, bodies := s.base.Slice(cur), s.flight.Slice(cur), s.body.Slice(cur)
		colls, sweeps, zs := s.coll.Slice(cur), s.sweep.Slice(cur), s.z.Slice(cur)
		s.walking.ids, s.walking.flights, s.walking.ends = cur.IDs, flights, s.walking.ends[:0]
		for i, id := range cur.IDs {
			f := &flights[i]
			if f.Landed && !bodies[i].Lands {
				s.walking.ends = append(s.walking.ends, false)
				s.w.Despawn(cb, id) // it lay a step for the effects its landing cast
				continue
			}
			var sw *collision.Sweep
			if sweeps != nil {
				sw = &sweeps[i]
			}
			var z *world.Z
			if zs != nil {
				z = &zs[i]
			}
			var contacts []collision.Contact
			if colls != nil {
				contacts = colls[i].Contacts()
			}
			ends := !f.Landed && (s.struck(f, &bases[i], sw, contacts) || f.Ending != Flying)
			s.walking.ends = append(s.walking.ends, ends)
			switch {
			case ends:
				f.Landed = true
				cb.RemoveCompOne(id, s.sweepID)
				cb.RemoveCompOne(id, s.collID)
			case !f.Landed:
				s.fly(f, bodies[i], &bases[i], sw, z, dt.Seconds())
			}
		}
		s.landings.RunWhere(t, cur, s.landsAt, s.landingAt)
		for i, id := range cur.IDs {
			if s.walking.ends[i] && flights[i].Ending == Left {
				cb.AddOne(id, s.outsideID, world.Outside{})
			}
		}
	}
}

func (s *flightSystem) struck(f *Flight, base *world.Base, sw *collision.Sweep, contacts []collision.Contact) bool {
	k := -1
	for i := range contacts {
		if k < 0 || contacts[i].Along < contacts[k].Along {
			k = i
		}
	}
	if k < 0 {
		return false
	}
	c := contacts[k]
	from := f.At
	if sw != nil {
		from = sw.From
	}
	hit := geom.NewVec(from.X+(f.At.X-from.X)*c.Along-f.Dir.X*landBack, from.Y+(f.At.Y-from.Y)*c.Along-f.Dir.Y*landBack)
	s.place(f, base, hit)
	if c.Terrain {
		f.Ending, f.Cell = Wall, c.Cell
	} else {
		f.Ending, f.Other = Struck, c.Other
	}
	return true
}

func (s *flightSystem) fly(f *Flight, body Body, base *world.Base, sw *collision.Sweep, z *world.Z, dt float64) {
	run := min(body.Speed*dt, f.Range-f.Flown)
	base.Vel.Dir = f.Dir // for a Turning look; Value stays 0, the flight moves it
	if sw != nil {
		sw.From = f.At
	}
	thrown := z != nil && body.Gravity > 0
	if thrown {
		f.Climb -= body.Gravity * dt
		z.Altitude += f.Climb * dt
	}
	want := geom.NewVec(f.At.X+f.Dir.X*run, f.At.Y+f.Dir.Y*run)
	in := s.place(f, base, want)
	f.Flown += run
	switch {
	case !in:
		f.Ending = Left
	case math.Abs(f.At.X-want.X) > edgeEps || math.Abs(f.At.Y-want.Y) > edgeEps:
		f.Ending = Edge
	case thrown && z.Altitude <= groundAt(s.groundOf, f.At):
		z.Altitude = groundAt(s.groundOf, f.At)
		f.Ending = Grounded
	case f.Flown >= f.Range-rangeEps:
		f.Ending = Spent
		if body.Gravity > 0 {
			f.Ending = Grounded
			if z != nil {
				z.Altitude = groundAt(s.groundOf, f.At)
			}
		}
	}
}

// place is false once the box has left by an open edge.
func (s *flightSystem) place(f *Flight, base *world.Base, p geom.Vec) bool {
	in := s.space.MoveTo(&base.Pos.AABB, geom.NewVec(p.X-base.Pos.Size.X/2, p.Y-base.Pos.Size.Y/2))
	f.At = base.Pos.Center()
	return in
}

func (s *flightSystem) landing(i int) Landing {
	f := &s.walking.flights[i]
	return Landing{ID: s.walking.ids[i], At: f.At, Struck: f.Ending == Struck, Other: f.Other, Wall: f.Ending == Wall,
		Cell: f.Cell, Grounded: f.Ending == Grounded || f.Ending == Spent, Left: f.Ending == Left}
}

func (s *flightSystem) lands(i int) bool { return s.walking.ends[i] }

func (s *flightSystem) rest(i int) Resting {
	return Resting{ID: s.walking.ids[i], At: s.walking.flights[i].At}
}

func (s *flightSystem) rests(i int) bool {
	return s.walking.flights[i].Landed && s.walking.bodies[i].Lands
}
