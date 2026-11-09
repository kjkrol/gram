package world

import (
	"testing"
)

func TestSheets_PhotographASectorAFieldIsBareInOnceItsPreviewIsKept(t *testing.T) {
	for _, tc := range []struct {
		name  string
		parts []shot
	}{
		{"parts", []shot{{700, 400, 10, 10}, {800, 400, 10, 10}}},
		{"none", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			k := newStandingKinds()
			s := newSheetRig(t, k, halves{ladder: atL4},
				stood{box: box(600, 400, 88), z: Z{Height: 40}, sprite: k.dome, fixed: true})
			sheet := &s.r.sheet
			at := place{whole: 7, sector: 1}
			// bare in the left half too, which the view draws short of L4
			sheet.bare = append(sheet.bare, at, place{whole: 7, sector: 0})
			for range 2 {
				if f := s.publish(); len(f.sheet.photos) != 0 || !sheet.pending[1] ||
					sheet.pending[0] {
					t.Fatalf("photographs %v, pending %v; want none, the right half waiting",
						f.sheet.photos, sheet.pending)
				}
			}
			sheet.fold(at, tc.parts)
			f := s.publish()
			want := (1 + len(tc.parts)) * photoFloats
			if len(f.sheet.photos) != 1 || f.sheet.photos[0].sector != 1 ||
				f.sheet.photos[0].to-f.sheet.photos[0].from != want || sheet.pending[1] {
				t.Fatalf("photographs %v, pending %v; want the right half's of the thing and "+
					"%d parts, none pending", f.sheet.photos, sheet.pending, len(tc.parts))
			}
			f.sheet.developed = true
			if f := s.publish(); len(f.sheet.photos) != 0 {
				t.Errorf("photographs %v once the preview is kept, want none", f.sheet.photos)
			}
		})
	}
}
