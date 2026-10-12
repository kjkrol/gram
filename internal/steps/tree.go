package steps

import (
	"fmt"
	"hash/fnv"
	"strings"

	"github.com/kjkrol/goke/v3"
)

type tree struct {
	name  string
	nodes []laid
	signs []string
}

type laid struct {
	exec exec
	kids []int
}

func (t *tree) add(s exec, sign string) int {
	if len(t.nodes) == MaxSteps {
		panic(fmt.Sprintf("rule: %q has more than %d steps", t.name, MaxSteps))
	}
	t.nodes = append(t.nodes, laid{exec: s})
	t.signs = append(t.signs, sign)
	return len(t.nodes) - 1
}

func (t *tree) signature() string {
	var b strings.Builder
	for i, n := range t.nodes {
		fmt.Fprintf(&b, "%d:%s%v;", i, t.signs[i], n.kids)
	}
	return b.String()
}

func layOut(name string, root Step) *tree {
	t := &tree{name: name}
	root.lay(t)
	return t
}

type definition struct {
	name string
	root Step
	sign string
	load []goke.CompToken
}

// Define lays root out as the plan named name and returns its id, what a Mind carries. A name
// defined twice, or after the plans' system is set up, panics.
func (c *Plans) Define(name string, root Step) uint64 {
	if name == "" {
		panic("rule: a plan needs a name")
	}
	s := c.system
	if s.si != nil {
		panic(fmt.Sprintf("rule: the plan %q is defined after the game was set up", name))
	}
	root = named(name, root)
	t := layOut(name, root)
	d := definition{name: name, root: root, sign: t.signature()}
	for _, n := range t.nodes {
		if l, ok := n.exec.(interface{ loads() []goke.CompToken }); ok {
			d.load = append(d.load, l.loads()...)
		}
	}
	id := hash(name)
	if have, ok := s.defs[id]; ok {
		panic(fmt.Sprintf("rule: the plan %q is defined already (or shares its hash with %q)", name, have.name))
	}
	if s.defs == nil {
		s.defs = map[uint64]definition{}
	}
	s.defs[id] = d
	return id
}

func named(name string, body Step) Step {
	if n, ok := body.(composite); ok && n.name == "" && (n.sign == "first" || n.sign == "then") {
		n.name = name
		return n
	}
	return NewThen(name, body)
}

// PlanID is the id of the plan named name: the name hashed.
func PlanID(name string) uint64 { return hash(name) }

func hash(name string) uint64 {
	h := fnv.New64a()
	h.Write([]byte(name))
	return h.Sum64()
}
