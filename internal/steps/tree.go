package steps

import (
	"fmt"
	"hash/fnv"
	"strings"
	"sync"

	"github.com/kjkrol/goke/v3"
)

// tree is a plan or a rule laid out for running: its steps by index, the root first.
type tree struct {
	name  string
	nodes []laid
	signs []string
}

// laid is one node of a laid-out tree: how it runs, and the nodes under it.
type laid struct {
	exec exec
	kids []int
}

// add lays s out as the tree's next node, sign saying what it is.
func (t *tree) add(s exec, sign string) int {
	if len(t.nodes) == MaxSteps {
		panic(fmt.Sprintf("rule: %q has more than %d steps", t.name, MaxSteps))
	}
	t.nodes = append(t.nodes, laid{exec: s})
	t.signs = append(t.signs, sign)
	return len(t.nodes) - 1
}

// signature is what the tree is, node by node with the nodes under each: two trees under one name
// must be alike.
func (t *tree) signature() string {
	var b strings.Builder
	for i, n := range t.nodes {
		fmt.Fprintf(&b, "%d:%s%v;", i, t.signs[i], n.kids)
	}
	return b.String()
}

// layOut lays root out as a tree named name.
func layOut(name string, root Step) *tree {
	t := &tree{name: name}
	root.lay(t)
	return t
}

// definition is a tree as registered: its root, to lay out for each world, and its signature.
type definition struct {
	name string
	root Step
	sign string
	load []goke.CompToken // the components of its asks, for the saves
}

// registry is every tree registered, by its name hashed.
var registry = struct {
	sync.Mutex
	trees map[uint64]definition
}{trees: map[uint64]definition{}}

// Register lays root out as the plan named name — what a save knows it by — and returns its id,
// what a Mind carries. One name is one plan: registered again, it must be alike.
func Register(name string, root Step) uint64 {
	if name == "" {
		panic("rule: a plan needs a name")
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
	registry.Lock()
	defer registry.Unlock()
	if have, ok := registry.trees[id]; ok {
		if have.name != name || have.sign != d.sign {
			panic(fmt.Sprintf("rule: two different plans are named %q (or share its hash with %q)", name, have.name))
		}
	} else {
		registry.trees[id] = d
	}
	return id
}

// named is body under name: a OneOf or a Steps of no name takes it, anything else is put in a
// Steps of that name.
func named(name string, body Step) Step {
	if n, ok := body.(composite); ok && n.name == "" && (n.sign == "first" || n.sign == "then") {
		n.name = name
		return n
	}
	return NewThen(name, body)
}

// hash is name hashed, as a Mind keeps it.
func hash(name string) uint64 {
	h := fnv.New64a()
	h.Write([]byte(name))
	return h.Sum64()
}

// definitions is every tree registered so far.
func definitions() map[uint64]definition {
	registry.Lock()
	defer registry.Unlock()
	out := make(map[uint64]definition, len(registry.trees))
	for id, d := range registry.trees {
		out[id] = d
	}
	return out
}
