package act

import (
	"fmt"
	"hash/fnv"
	"strings"
	"sync"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugins/world/entity/kind/comp"
)

// tree is a tree laid out for running: its nodes by index, the root first.
type tree struct {
	name  string
	nodes []laid
	signs []string
}

// laid is one node of a laid-out tree: how it runs, and the nodes under it.
type laid struct {
	step step
	kids []int
}

// add lays s out as the tree's next node, sign saying what it is.
func (t *tree) add(s step, sign string) int {
	if len(t.nodes) == MaxNodes {
		panic(fmt.Sprintf("act: tree %q has more than %d nodes", t.name, MaxNodes))
	}
	t.nodes = append(t.nodes, laid{step: s})
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
func layOut(name string, root Node) *tree {
	t := &tree{name: name}
	root.lay(t)
	return t
}

// definition is a tree as registered: its root, to lay out for each world, and its signature.
type definition struct {
	name string
	root Node
	sign string
	load []goke.CompToken // the components of its asks, for the saves
}

// registry is every tree registered, by its name hashed.
var registry = struct {
	sync.Mutex
	trees map[uint64]definition
}{trees: map[uint64]definition{}}

// Tree is the component a kind gives its entities to behave by the tree under root,
// whose name — First's or Then's — is what a save knows it by. One name is one tree: registered
// again, it must be alike.
func Tree(root Node) comp.Template[Mind] {
	name := nameOf(root)
	if name == "" {
		panic("act: a tree's root needs a name: Named(name), When(name) or On(name)")
	}
	t := layOut(name, root)
	d := definition{name: name, root: root, sign: t.signature()}
	for _, n := range t.nodes {
		if l, ok := n.step.(interface{ loads() []goke.CompToken }); ok {
			d.load = append(d.load, l.loads()...)
		}
	}
	id := hash(name)
	registry.Lock()
	defer registry.Unlock()
	if have, ok := registry.trees[id]; ok {
		if have.name != name || have.sign != d.sign {
			panic(fmt.Sprintf("act: two different trees are named %q (or share its hash with %q)", name, have.name))
		}
	} else {
		registry.trees[id] = d
	}
	return comp.Const(Mind{Tree: id})
}

// nameOf is a root's name: a composite's.
func nameOf(root Node) string {
	if c, ok := bare(root).(composite); ok {
		return c.name
	}
	return ""
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
