package relief

import (
	"math"
	"math/rand/v2"
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/grid"
)

func TestRelief_AlongIsTheGroundAlongARay(t *testing.T) {
	g := grid.DefaultGrids{}.Square(12, 9, 32)
	r := New(board.NewBoard(g))
	rnd := rand.New(rand.NewPCG(3, 5))
	r.SetHeights(func(geom.Vec) float64 { return 40 * rnd.Float64() })
	for trial := range 200 {
		origin := geom.NewVec(384*rnd.Float64(), 288*rnd.Float64())
		a := 2 * math.Pi * rnd.Float64()
		if trial%10 == 0 {
			a = float64(trial/10%4) * math.Pi / 2 // along the grid's lines
		}
		dir := geom.NewVec(math.Cos(a), math.Sin(a))
		length := 30 + 300*rnd.Float64()
		ds, alts := []float64{0}, []float64{r.GroundAt(origin)}
		r.Along(origin, dir, length, func(d, alt float64) bool {
			if d < ds[len(ds)-1] {
				t.Fatalf("trial %d: the profile goes back from %.4f to %.4f",
					trial, ds[len(ds)-1], d)
			}
			ds, alts = append(ds, d), append(alts, alt)
			return true
		})
		if last := ds[len(ds)-1]; last != length {
			t.Fatalf("trial %d: the profile ends at %.4f, want %.4f", trial, last, length)
		}
		k := 1
		for i := range 4000 {
			d := length * float64(i) / 3999
			for k < len(ds)-1 && ds[k] < d {
				k++
			}
			want := r.GroundAt(geom.NewVec(origin.X+dir.X*d, origin.Y+dir.Y*d))
			got := alts[k]
			if span := ds[k] - ds[k-1]; span > 0 {
				got = alts[k-1] + (d-ds[k-1])/span*(alts[k]-alts[k-1])
			}
			if math.Abs(got-want) > 1e-6 {
				t.Fatalf("trial %d: %.3f along the ground is %.6f, the profile says %.6f",
					trial, d, want, got)
			}
		}
	}
}
