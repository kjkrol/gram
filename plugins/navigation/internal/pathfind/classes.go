package pathfind

import (
	"cmp"
	"math"
	"slices"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/internal/f32"
	"github.com/kjkrol/gram/plugins/board/cell"
)

// MaxClasses is how many classes of bodies the routes of one domain are planned as at most.
const MaxClasses = 8

// Kind is a kind of unit as its routes are planned: the domain it moves in and its body.
type Kind struct {
	Domain cell.Domain
	Body   Body
}

// Classes are the bodies the units' routes are planned as, each domain's apart: the bodies of
// the kinds moving in it, their feet left out, at most MaxClasses. The zero value has none.
type Classes struct {
	domains []domainClasses
}

type domainClasses struct {
	domain cell.Domain
	bodies []Body // by their limits, the laxest first
}

// Set has c the classes of kinds: each domain's bodies once, by their limits, the two whose
// merging makes them least stricter merged into a class no laxer than either till MaxClasses
// are left.
func (c *Classes) Set(kinds []Kind) {
	c.domains = nil
	for _, k := range kinds {
		body := k.Body
		body.Feet = 0
		i := slices.IndexFunc(c.domains, func(d domainClasses) bool { return d.domain == k.Domain })
		if i < 0 {
			i = len(c.domains)
			c.domains = append(c.domains, domainClasses{domain: k.Domain})
		}
		if d := &c.domains[i]; !slices.Contains(d.bodies, body) {
			d.bodies = append(d.bodies, body)
		}
	}
	for i := range c.domains {
		d := &c.domains[i]
		slices.SortFunc(d.bodies, byLimits)
		for len(d.bodies) > MaxClasses {
			d.bodies = mergeNearest(d.bodies)
		}
	}
}

// Of are domain's classes, by their limits, the laxest first; none where no kind moves in it.
func (c *Classes) Of(domain cell.Domain) []Body {
	for i := range c.domains {
		if c.domains[i].domain == domain {
			return c.domains[i].bodies
		}
	}
	return nil
}

// Class is the class body is planned as in domain, its index among Of(domain) and its body: the
// class that is its body, else the nearest of those whose every limit is as strict; false where
// none is.
func (c *Classes) Class(domain cell.Domain, body Body) (int, Body, bool) {
	body.Feet = 0
	classes := c.Of(domain)
	for i, class := range classes {
		if class == body {
			return i, class, true
		}
	}
	k, nearest := -1, math.Inf(1)
	for i, class := range classes {
		if holds(class, body) {
			if d := apart(class, body); d < nearest {
				k, nearest = i, d
			}
		}
	}
	if k < 0 {
		return 0, Body{}, false
	}
	return k, classes[k], true
}

// Planned is body as its routes are planned in domain: its class's body on its own feet, itself
// where no class holds it.
func (c *Classes) Planned(domain cell.Domain, body Body) Body {
	if _, class, ok := c.Class(domain, body); ok {
		class.Feet = body.Feet
		return class
	}
	return body
}

// byLimits orders bodies by their limits, a linear extension of holds: across, down, height,
// weight, the step from the highest down
func byLimits(a, b Body) int {
	return cmp.Or(cmp.Compare(a.Size.X, b.Size.X), cmp.Compare(a.Size.Y, b.Size.Y),
		cmp.Compare(a.Height, b.Height), cmp.Compare(a.Weight, b.Weight),
		cmp.Compare(b.Step, a.Step))
}

// holds says whether every limit of class is at least as strict as body's: no narrower either
// way, no lower, no lighter, stepping up no higher
func holds(class, body Body) bool {
	return class.Size.X >= body.Size.X && class.Size.Y >= body.Size.Y &&
		class.Height >= body.Height && class.Weight >= body.Weight && class.Step <= body.Step
}

// merged is the class of a and b, no laxer than either: the larger of each limit, the lower step
func merged(a, b Body) Body {
	return Body{
		Step:   min(a.Step, b.Step),
		Height: max(a.Height, b.Height),
		Size:   geom.NewVec(max(a.Size.X, b.Size.X), max(a.Size.Y, b.Size.Y)),
		Weight: max(a.Weight, b.Weight),
	}
}

// apart is how far two bodies' limits lie apart: each limit's difference over the larger of the
// two, summed
func apart(a, b Body) float64 {
	return share(a.Size.X, b.Size.X) + share(a.Size.Y, b.Size.Y) + share(a.Height, b.Height) +
		share(a.Weight, b.Weight) + share(a.Step, b.Step)
}

func share(a, b float32) float64 {
	if a == b {
		return 0
	}
	return float64(f32.Abs(a-b) / max(f32.Abs(a), f32.Abs(b)))
}

// mergeNearest merges the two of bodies, ordered by their limits, whose class is the least
// stricter than they are, the first such pair in their order, and keeps the order
func mergeNearest(bodies []Body) []Body {
	a, b, least := 0, 1, math.Inf(1)
	for i := range bodies {
		for j := i + 1; j < len(bodies); j++ {
			m := merged(bodies[i], bodies[j])
			if loss := apart(bodies[i], m) + apart(bodies[j], m); loss < least {
				a, b, least = i, j, loss
			}
		}
	}
	m := merged(bodies[a], bodies[b])
	bodies = slices.Delete(bodies, b, b+1)
	bodies[a] = m
	slices.SortFunc(bodies, byLimits)
	return slices.Compact(bodies)
}
