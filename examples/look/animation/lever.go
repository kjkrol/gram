package main

import (
	_ "embed"
	"time"

	"github.com/kjkrol/ceremony/cond"
	"github.com/kjkrol/ceremony/effect"
	"github.com/kjkrol/ceremony/plan"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

// four frames of 12 pixels in a row, the arm from the left (up) to the right (pulled down), the
// lever filling its frame; the first and the last from "Lever and Button" (OpenGameArt, CC0), the
// two between drawn after them
//
//go:embed lever.png
var leverPNG []byte

const (
	leverFrames = 4
	leverPixels = 12
	leverSize   = 16
	leverSwing  = 400 * time.Millisecond
	leverRests  = 2 * time.Second
)

type standingRow struct {
	At cell.ID `yaml:"at"`
}

var leverSheet = render.DecodeSheet("lever.png", leverPNG, leverPixels, leverPixels)

// lever swings down and up by itself, its arm drawn along the effects of the swing.
type lever struct{}

var (
	_ board.Figure   = lever{}
	_ board.Stateful = lever{}
)

// pulled down for good, and the swings each way, the pull ending down
func (lever) Effects(b *board.Plugin) {
	fx := b.World().Effects()
	fx.Define(LeverPulledEf, effect.Spec{})
	fx.Define(LeverPullingEf, effect.Spec{
		effect.Lasts(leverSwing),
		effect.Then(fx.Named(LeverPulledEf)),
	})
	fx.Define(LeverPushingEf, effect.Spec{effect.Lasts(leverSwing)})
}

func (lever) Define(b *board.Plugin) {
	fx := b.World().Effects()
	swings := plan.New(SwingsPlan, plan.Loop[any]{
		Name: "swing",
		When: cond.Self.If(cond.Always),
		Then: plan.Steps(
			plan.Self.Put(fx.Named(LeverPullingEf)),
			plan.Self.Wait(leverRests),
			plan.Self.Take(fx.Named(LeverPulledEf)),
			plan.Self.Put(fx.Named(LeverPushingEf)),
			plan.Self.Wait(leverRests),
		),
	})
	board.NewEntities(b, board.Shape{Size: leverSize},
		func(r standingRow) cell.ID { return r.At }).Define(LeverKind, swings)
}

func (lever) Look(a *world.Atlas, w *world.Plugin) {
	fx := w.Effects()
	a.Add(kind.Named[standingRow](w.Kinds(), LeverKind), leverSize, leverSheet.At(0, 0)).
		Under(fx.Named(LeverPulledEf), leverSheet.At(leverFrames-1, 0)).
		Under(fx.Named(LeverPullingEf), render.Phased(leverFrames, leverFrame)).
		Under(fx.Named(LeverPushingEf), render.Phased(leverFrames, leverFrameBack))
}

func leverFrame(i int) render.SpriteDrawer     { return leverSheet.At(i, 0) }
func leverFrameBack(i int) render.SpriteDrawer { return leverSheet.At(leverFrames-1-i, 0) }
