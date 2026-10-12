package dialog

import (
	"bytes"
	"fmt"
	"hash/fnv"
	"io/fs"
	"slices"
	"strings"

	"github.com/kjkrol/gram/rule"
	"github.com/kjkrol/gram/rule/effect"
	"go.yaml.in/yaml/v3"
)

// Node is one place of a conversation: who speaks, what is said, and the answers offered. A file
// Load reads is a map of them by name.
type Node struct {
	Speaker string   `yaml:"speaker"`
	Say     []string `yaml:"say"`     // lines, one under another
	Choices []Choice `yaml:"choices"` // at most MaxChoices
}

// Choice is one answer: its words, when it is offered, what it changes in the game, where it leads
// and the commands it gives.
type Choice struct {
	Text string `yaml:"text"`
	// If is when the answer is offered: a condition the game defined (Plugin.Conditions) or the
	// name of an effect the speaker is under; "!" before it, when it does not hold. Empty: always.
	If   string   `yaml:"if"`
	Then []string `yaml:"then"` // changes the game defined (Plugin.Changes), made in order as it is chosen
	Next string   `yaml:"next"` // the node it leads to, or End
	Do   []string `yaml:"do"`   // commands of the Stage's register (world.Commands), given after the changes
}

// End is the Next of an answer that ends the conversation.
const End = "end"

// MaxChoices is how many answers a node offers at most: the buttons of Window.
const MaxChoices = 6

// node is a Node as the plugin runs it.
type node struct {
	name    string
	speaker string
	line    string
	choices []choice
}

// choice is a Choice as the plugin runs it.
type choice struct {
	text string
	when condition
	then []Change
	next string // "" for End
	do   []rule.Command
}

// condition is a Choice's If: the game's, or an effect on the speaker, or none.
type condition struct {
	game     checker
	effect   effect.Effect
	isEffect bool
	not      bool
}

// idOf is a node's name as a Talk and a Script keep it.
func idOf(name string) uint64 {
	h := fnv.New64a()
	h.Write([]byte(name))
	return h.Sum64()
}

// Define says the node called name, written in code: what Load does for every node of a file.
// Call it in the plugin's section (Conversations), after the changes and conditions the node's
// answers name; a name defined twice, an unknown one, or too many answers panics.
func (p *Plugin) Define(name string, n Node) {
	if err := p.define(name, n); err != nil {
		panic(err)
	}
}

// Load reads the nodes of the YAML file at path in fsys — a map of Nodes by name — and defines
// them; several files may be loaded, a character's each. Call it in the plugin's section
// (Conversations), after the changes and conditions the answers name.
func (p *Plugin) Load(fsys fs.FS, path string) error {
	data, err := fs.ReadFile(fsys, path)
	if err != nil {
		return fmt.Errorf("dialog: %w", err)
	}
	var nodes map[string]Node
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&nodes); err != nil {
		return fmt.Errorf("dialog: %s: %w", path, err)
	}
	names := make([]string, 0, len(nodes))
	for name := range nodes {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		if err := p.define(name, nodes[name]); err != nil {
			return fmt.Errorf("%w (in %s)", err, path)
		}
	}
	return nil
}

// define compiles n and keeps it as name.
func (p *Plugin) define(name string, n Node) error {
	if err := p.w.InSection(fmt.Sprintf("dialog node %q defined", name), ConversationsPart); err != nil {
		return fmt.Errorf("dialog: %w", err)
	}
	if name == "" || name == End {
		return fmt.Errorf("dialog: a node may not be called %q", name)
	}
	if _, ok := p.nodes[idOf(name)]; ok {
		return fmt.Errorf("dialog: the node %q is defined already", name)
	}
	if len(n.Choices) > MaxChoices {
		return fmt.Errorf("dialog: the node %q offers %d answers, at most %d", name, len(n.Choices), MaxChoices)
	}
	c := node{name: name, speaker: n.Speaker, line: strings.Join(n.Say, "\n")}
	for _, ch := range n.Choices {
		when, err := p.condition(ch.If)
		if err != nil {
			return fmt.Errorf("dialog: the node %q, the answer %q: %w", name, ch.Text, err)
		}
		next := ch.Next
		if next == End {
			next = ""
		} else if next == "" {
			return fmt.Errorf("dialog: the node %q, the answer %q leads nowhere: say next, or %q", name, ch.Text, End)
		}
		var then []Change
		for _, change := range ch.Then {
			found, ok := p.changes.byName[change]
			if !ok {
				return fmt.Errorf("dialog: the node %q, the answer %q: no change is defined as %q", name, ch.Text, change)
			}
			then = append(then, found)
		}
		var do []rule.Command
		for _, cmd := range ch.Do {
			found, ok := p.w.Commands().Lookup(cmd)
			if !ok {
				return fmt.Errorf("dialog: the node %q, the answer %q: no command is defined as %q", name, ch.Text, cmd)
			}
			do = append(do, found)
		}
		c.choices = append(c.choices, choice{text: ch.Text, when: when, then: then, next: next, do: do})
	}
	if p.nodes == nil {
		p.nodes = map[uint64]*node{}
	}
	p.nodes[idOf(name)] = &c
	return nil
}

// condition is an answer's If, read.
func (p *Plugin) condition(text string) (condition, error) {
	var c condition
	text = strings.TrimSpace(text)
	if rest, ok := strings.CutPrefix(text, "!"); ok {
		c.not, text = true, strings.TrimSpace(rest)
	}
	if text == "" {
		if c.not {
			return c, fmt.Errorf("%q says no condition", "!")
		}
		return c, nil
	}
	if cond, ok := p.conditions.byName[text]; ok {
		c.game = cond.c
		return c, nil
	}
	ef, ok := p.w.Effects().Lookup(text)
	if !ok {
		return c, fmt.Errorf("%q is neither a condition the game defined nor an effect", text)
	}
	c.effect, c.isEffect = ef, true
	return c, nil
}

// check panics for an answer leading to a node nobody defined: as the Stage is set up, every file
// loaded.
func (p *Plugin) check() {
	for _, n := range p.sorted() {
		for _, ch := range n.choices {
			if _, ok := p.nodes[idOf(ch.next)]; ch.next != "" && !ok {
				panic(fmt.Sprintf("dialog: the node %q, the answer %q leads to %q, which is not defined", n.name, ch.text, ch.next))
			}
		}
	}
}

// sorted is every node, by name.
func (p *Plugin) sorted() []*node {
	out := make([]*node, 0, len(p.nodes))
	for _, n := range p.nodes {
		out = append(out, n)
	}
	slices.SortFunc(out, func(a, b *node) int { return strings.Compare(a.name, b.name) })
	return out
}
