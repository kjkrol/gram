package main

import (
	_ "embed"
	"math"
	"time"

	"github.com/kjkrol/ceremony"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/ceremony/effect"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/collision"
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
	leverSize   = 16 // the box, what is drawn: the smallest entity of the world
	leverSwing  = 400 * time.Millisecond
)

// lever is a post the hero walks up to and pulls by touch, its arm drawn down and up along the
// effects of the swing.
type lever struct{ sheet render.Sheet }

var (
	_ board.Figure   = (*lever)(nil)
	_ board.Stateful = (*lever)(nil)
	_ board.Ruled    = (*lever)(nil)
)

func newLever() *lever {
	return &lever{sheet: render.DecodeSheet("lever.png", leverPNG, leverPixels, leverPixels)}
}

// the lever's positions: pulled down for good, and the swings each way, the pull ending down
func (l *lever) Effects(b *board.Plugin) {
	fx := b.World().Effects()
	fx.Define(LeverPulledEf, effect.Spec{})
	fx.Define(LeverPullingEf, effect.Spec{
		effect.Lasts(leverSwing),
		effect.Then(fx.Named(LeverPulledEf)),
	})
	fx.Define(LeverPushingEf, effect.Spec{effect.Lasts(leverSwing)})
}

func (l *lever) Rules(b *board.Plugin) { b.World().Roles().Define(LeverRole) }

func (l *lever) Define(b *board.Plugin) {
	board.NewEntities(b, board.Shape{Size: leverSize}, standingAt).Define(LeverKind,
		comp.Const(collision.Collider{}),
		comp.Const(collision.Physics{Mass: math.Inf(1)}), // a post: the hero walks up to it
		ceremony.Plays(b.World().Roles().Named(LeverRole)))
}

func (l *lever) Look(a *world.Atlas, w *world.Plugin) {
	fx := w.Effects()
	a.Add(kind.Named[standingRow](w.Kinds(), LeverKind), leverSize, l.sheet.At(0, 0)).
		Under(fx.Named(LeverPulledEf), l.sheet.At(leverFrames-1, 0)).
		Under(fx.Named(LeverPullingEf), render.Phased(leverFrames, l.frame)).
		Under(fx.Named(LeverPushingEf), render.Phased(leverFrames, l.frameBack))
}

func (l *lever) frame(i int) render.SpriteDrawer     { return l.sheet.At(i, 0) }
func (l *lever) frameBack(i int) render.SpriteDrawer { return l.sheet.At(leverFrames-1-i, 0) }
