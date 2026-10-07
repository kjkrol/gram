package ui

import (
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/game"
)

// order is a command a test's button gives.
type order struct{ what string }

// player notes the area it was shown over and the clicks that reached it.
type player struct {
	area   geom.AABB
	clicks []geom.Vec
	wheel  float64
}

func (p *player) Over(area geom.AABB) { p.area = area }

func (p *player) handle(ev *control.InputEvents, _ game.Runtime, _ game.Composition) {
	for _, c := range ev.ClickQueue {
		p.clicks = append(p.clicks, c.Pos)
	}
	p.wheel += ev.ScrollDelta
}

// built lays the scene's screen as a frame would.
func built(s *Scene) *Scene {
	s.Layers()
	s.root.lay(screen)
	return s
}

func click(at geom.Vec) *control.InputEvents {
	return &control.InputEvents{MousePos: at, ClickQueue: []control.ClickEvent{
		{Button: control.MouseButtonLeft, Action: control.ActionPress, Pos: at}}}
}

func TestInput_AClickOnTheWorldReachesThePlayerAndOneOnAPanelDoesNot(t *testing.T) {
	red := &player{}
	s := built(NewScene("main", nil, func() *Element {
		return Layers(Image(&sized{}).Input(red), TopLeft(Panel(Label("hud"))).Size(200, 100))
	}).Input(red.handle))
	if red.area != screen {
		t.Fatalf("the player was told the area %v, want the screen", red.area)
	}
	s.HandleEvents(click(geom.NewVec(600, 300)), nil, nil)
	s.HandleEvents(click(geom.NewVec(50, 50)), nil, nil)
	if len(red.clicks) != 1 || red.clicks[0] != geom.NewVec(600, 300) {
		t.Fatalf("the player got clicks %v, want only the one on the world", red.clicks)
	}
}

func TestInput_AButtonGivesItsCommandThroughIssue(t *testing.T) {
	var issued []any
	s := built(NewScene("main", nil, func() *Element {
		return Center(Button("go", order{"go"})).Size(100, 40)
	}).Issue(func(cmd any) error { issued = append(issued, cmd); return nil }))
	s.HandleEvents(click(geom.NewVec(600, 300)), nil, nil)
	if len(issued) != 1 || issued[0] != (order{"go"}) {
		t.Fatalf("issued %v, want the button's order", issued)
	}
}

func TestInput_AButtonOfTheScenesOwnShowsAndHidesByName(t *testing.T) {
	panel := Panel(Label("panel")).Named("panel").Hidden()
	s := built(NewScene("main", nil, func() *Element {
		return Layers(TopLeft(Button("open", Toggle{"panel"})).Size(100, 40), panel)
	}))
	s.HandleEvents(click(geom.NewVec(20, 20)), nil, nil)
	if panel.hidden {
		t.Fatal("the button left the panel hidden")
	}
}

func TestInput_AModalWindowHoldsTheClicksAndTheWheelOutsideIt(t *testing.T) {
	red := &player{}
	var issued []any
	s := built(NewScene("main", nil, func() *Element {
		return Layers(
			Image(&sized{}).Input(red),
			TopLeft(Button("under", order{"under"})).Size(100, 40),
			Center(Window("decide", Button("yes", order{"yes"}))).Size(300, 200).Named("decision").Modal(),
		)
	}).Input(red.handle).Issue(func(cmd any) error { issued = append(issued, cmd); return nil }))

	outside := click(geom.NewVec(20, 20))
	outside.ScrollDelta = 1
	s.HandleEvents(outside, nil, nil)
	if len(issued) != 0 || len(red.clicks) != 0 || red.wheel != 0 {
		t.Fatalf("outside the modal window: issued %v, clicks %v, wheel %v; want nothing", issued, red.clicks, red.wheel)
	}

	s.Hide("decision")
	s.HandleEvents(click(geom.NewVec(20, 20)), nil, nil)
	if len(issued) != 1 || issued[0] != (order{"under"}) {
		t.Fatalf("with the window hidden: issued %v, want the button under it", issued)
	}
}

func TestInput_TheScenesKeysGiveTheirCommands(t *testing.T) {
	panel := Label("panel").Named("panel").Hidden()
	s := built(NewScene("main", nil, func() *Element { return Layers(panel) }).
		Keys(control.Give(control.KeyPress{Key: control.KeyP}, "Panel", Toggle{"panel"})))
	ev := &control.InputEvents{KeyEvents: []control.KeyEvent{{Key: control.KeyP, Action: control.ActionPress}}}
	s.HandleEvents(ev, nil, nil)
	if panel.hidden {
		t.Fatal("P left the panel hidden")
	}
}
