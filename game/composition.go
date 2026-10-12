package game

import "github.com/kjkrol/gram/plugin"

// Composition tracks which Scenes in a Stack are visible, in what order, and which is active.
type Composition interface {
	// Show makes name visible, on top.
	Show(name string)

	// Hide makes name no longer visible.
	Hide(name string)

	// BringToFront moves an already-visible name to the top.
	BringToFront(name string)

	// Order returns the currently visible names, bottom-to-top.
	Order() []string

	// Active returns the topmost focusable Scene's name, or "" if none is.
	Active() string

	plugin.Serializable
}

type composition struct {
	stack  Scenes
	order  []string
	active string
}

var _ Composition = (*composition)(nil)
var _ plugin.Restorer = (*composition)(nil)

func newComposition(stack Scenes) Composition {
	return &composition{stack: stack}
}

func (c *composition) Show(name string) {
	if _, ok := c.stack.Get(name); !ok {
		return
	}
	if c.indexOf(name) >= 0 {
		return
	}
	c.order = append(c.order, name)
	c.promote(name)
}

func (c *composition) Hide(name string) {
	i := c.indexOf(name)
	if i < 0 {
		return
	}
	c.order = append(c.order[:i], c.order[i+1:]...)
	if c.active == name {
		c.active = c.topFocusable()
	}
}

func (c *composition) BringToFront(name string) {
	i := c.indexOf(name)
	if i < 0 {
		return
	}
	c.order = append(c.order[:i], c.order[i+1:]...)
	c.order = append(c.order, name)
	c.promote(name)
}

func (c *composition) Order() []string { return c.order }

func (c *composition) Active() string { return c.active }

func (c *composition) Persisted() []any { return []any{&c.order} }

func (c *composition) Restore() { c.active = c.topFocusable() }

func (c *composition) promote(name string) {
	if sc, ok := c.stack.Get(name); ok && sc.Focusable() {
		c.active = name
	}
}

func (c *composition) topFocusable() string {
	for i := len(c.order) - 1; i >= 0; i-- {
		if sc, ok := c.stack.Get(c.order[i]); ok && sc.Focusable() {
			return c.order[i]
		}
	}
	return ""
}

func (c *composition) indexOf(name string) int {
	for i, n := range c.order {
		if n == name {
			return i
		}
	}
	return -1
}
