package game

import "fmt"

// Defined is a game of props and stages and nothing else of its own: what New makes.
type Defined struct {
	props  Props
	stages map[string]Stage
	first  string
}

var _ Game = (*Defined)(nil)

// New is the game of props and stages, starting with the first: a game keeping nothing of its own
// beside them. None, or a name two stages share, panics.
func New(props Props, stages ...Stage) *Defined {
	if len(stages) == 0 {
		panic("game: a game needs a stage")
	}
	g := &Defined{props: props, stages: make(map[string]Stage, len(stages)), first: stages[0].Name()}
	for _, s := range stages {
		if _, ok := g.stages[s.Name()]; ok {
			panic(fmt.Sprintf("game: two stages are called %q", s.Name()))
		}
		g.stages[s.Name()] = s
	}
	return g
}

// Starting has the game start with the stage called name; an unknown name panics.
func (g *Defined) Starting(name string) *Defined {
	if _, ok := g.stages[name]; !ok {
		panic(fmt.Sprintf("game: no stage is called %q", name))
	}
	g.first = name
	return g
}

func (g *Defined) Props() Props { return g.props }

func (g *Defined) Stages() (map[string]Stage, string) { return g.stages, g.first }
