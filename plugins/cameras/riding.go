package cameras

import (
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/internal/f32"
	"github.com/kjkrol/gram/plugins/cameras/internal/stick"
	"github.com/kjkrol/gram/plugins/motion/steering"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/uid"
)

var _ goke.System = (*riding)(nil)
var _ stick.Entities = (*riding)(nil)

// the cameras in relief kept behind and inside the world's entities
type riding struct {
	keeper *stick.Keeper

	query  *goke.Query
	base   goke.Comp[world.Base]
	z      goke.OptComp[world.Z]
	eye    goke.OptComp[world.Eye]
	driven goke.OptComp[steering.Driven]
}

func (r *riding) Init(si *goke.SysInit) {
	r.query = si.NewQueryBuilder(&r.base).
		Optional(&r.z).
		Optional(&r.eye).
		Optional(&r.driven).
		Build()
}

func (r *riding) Update(_ *goke.CmdBuf, d time.Duration) { r.keeper.Update(d, r) }

func (r *riding) Of(id uid.UID64) (stick.Target, bool) {
	if !r.query.Seek(id) {
		return stick.Target{}, false
	}
	cur := r.query.Cursor()
	base := r.base.At(cur)
	box := base.Pos.AABB
	var z world.Z
	if at := r.z.At(cur); at != nil {
		z = *at
	}
	t := stick.Target{
		X:     (box.TopLeft.X + box.BottomRight.X) / 2,
		Y:     (box.TopLeft.Y + box.BottomRight.Y) / 2,
		Alt:   z.Altitude,
		Level: z.Top(),
	}
	if n := f32.Hypot(base.Vel.Dir.X, base.Vel.Dir.Y); n > 0 {
		t.Dir = geom.NewVec(base.Vel.Dir.X/n, base.Vel.Dir.Y/n)
	}
	if eye := r.eye.At(cur); eye != nil {
		t.Level, t.Across, t.Tilt, t.Eyed = eye.Level(z), eye.Angle, eye.Tilt, true
	}
	if d := r.driven.At(cur); d != nil {
		t.Turning = d.Turn != 0
	}
	return t, true
}

func (r *riding) Drive(id uid.UID64, look geom.Vec, flown bool, climb float32) {
	if !r.query.Seek(id) {
		return
	}
	if d := r.driven.At(r.query.Cursor()); d != nil {
		d.Look, d.Flown, d.Climb = look, flown, climb
	}
}

func (r *riding) Tilt(id uid.UID64, tilt float32) {
	if !r.query.Seek(id) {
		return
	}
	if eye := r.eye.At(r.query.Cursor()); eye != nil {
		eye.Tilt = tilt
	}
}
