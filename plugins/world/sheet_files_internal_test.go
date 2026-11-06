package world

import (
	"image"
	"image/png"
	"io"
	"math/rand/v2"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/lod"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/render/gputest"
)

// noise is n bytes drawn from seed
func noise(n int, seed uint64) []byte {
	r := rand.New(rand.NewPCG(seed, 1))
	b := make([]byte, n)
	for i := range b {
		b[i] = byte(r.Uint32())
	}
	return b
}

// bytesOf is what img holds
func bytesOf(img *render.Image) []byte {
	b := img.Bounds()
	pix := make([]byte, 4*b.Dx()*b.Dy())
	img.ReadPixels(pix)
	return pix
}

// sheetsOf are the four images of s
func sheetsOf(s *sheets) [4]*render.Image {
	return [4]*render.Image{s.albedo, s.relief, s.albedoL5, s.reliefL5}
}

var sheetNames = [4]string{"albedo", "relief", "albedo at L5", "relief at L5"}

// filedSheets is sheets 128 by 125 texels over g, 8 world units a texel, kept in files of tiles
// tile a side as cfg says, read as they are made, their clock at
func filedSheets(g tiling, cfg SheetsCfg, tile int, at *time.Time) *sheets {
	s := new(sheets)
	*s = sheetsOver(g, 1.0/8, 128, 125)
	s.open()
	fileSheets(s, cfg, tile, at)
	return s
}

// fileSheets has s kept in files as cfg says, tiles tile a side, read now, its clock at
func fileSheets(s *sheets, cfg SheetsCfg, tile int, at *time.Time) {
	s.files = cfg
	s.store = newSheetStore(s, tile)
	s.store.now = func() time.Time { return *at }
	s.store.read(s)
}

// keptSheets is filedSheets of noise, its sectors' photographs those of photos developed, every
// tile written
func keptSheets(t *testing.T, g tiling, cfg SheetsCfg, tile int, photos ...photo) *sheets {
	t.Helper()
	at := time.Unix(1000, 0)
	s := filedSheets(g, cfg, tile, &at)
	s.albedo.WritePixels(noise(4*128*125, 1))
	s.relief.WritePixels(noise(4*128*125, 2))
	s.room.readyHalves()
	s.room.halve(s, appendRect(nil, halfOf(s.albedo.Bounds()), 0, 0))
	s.store.took(photos)
	for range len(s.store.dirty) {
		at = at.Add(sheetSettled)
		s.store.keep(s)
		s.store.writing.Wait()
	}
	if s.store.failed.Load() {
		t.Fatal("the tiles were not written")
	}
	return s
}

// filesIn are the names of the files in dir
func filesIn(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

func TestSheetFiles_KeepTheBytesOfATileAsTheyAre(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tile.png")
	var enc png.Encoder
	for _, tc := range []struct {
		name string
		pix  []byte
	}{
		{"translucent", noise(4*7*5, 3)},
		{"opaque", func() []byte {
			b := noise(4*7*5, 4)
			for i := 3; i < len(b); i += 4 {
				b[i] = 255
			}
			return b
		}()},
	} {
		if err := writePNG(path, tc.pix, 7, 5, &enc); err != nil {
			t.Fatal(err)
		}
		got, err := readPNG(path, 7, 5)
		if err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(got, tc.pix) {
			t.Errorf("%s: read back %v, want %v", tc.name, got, tc.pix)
		}
	}
	if _, err := readPNG(path, 5, 7); err == nil {
		t.Error("a tile of another size read")
	}
}

func TestSheetFiles_KeepTheManifestAsWritten(t *testing.T) {
	s := sheetsOver(oddHalves{}, 0.12, 123, 120)
	s.files = SheetsCfg{Dir: t.TempDir(), Version: "forests 2"}
	st := newSheetStore(&s, 64)
	sectors := []sheetSector{{I: 1, N: 7, Mix: 0xfedcba9876543210, Inked: true}, {I: 0}}
	if err := st.writeManifest(sectors); err != nil {
		t.Fatal(err)
	}
	m, err := st.manifest()
	if err != nil || m.sheetHead != st.head || !slices.Equal(m.Sectors, sectors) {
		t.Errorf("read %+v (%v), want %+v and %v", m, err, st.head, sectors)
	}
}

func TestSheetFiles_ReadEveryTileOfTheSheetsByteForByte(t *testing.T) {
	gputest.Need(t)
	cfg := SheetsCfg{Dir: t.TempDir(), Keep: true, Version: "1"}
	signed := signature{n: 3, mix: 0xfedcba9876543210}
	// sector 0's texels lie in every tile, cut at 65, sector 1's in the right two
	kept := keptSheets(t, oddHalves{}, cfg, 64, photo{sector: 0, signed: signed, full: true},
		photo{sector: 1})
	names := filesIn(t, cfg.Dir)
	if len(names) != 9 || !slices.Contains(names, sheetManifestFile) ||
		!slices.Contains(names, "relief-1-1.png") {
		t.Fatalf("files %v, want the manifest and the albedo and relief of 4 tiles", names)
	}
	at := time.Unix(2000, 0)
	got := filedSheets(oddHalves{}, cfg, 64, &at)
	if len(got.store.loads) != 4 {
		t.Fatalf("%d tiles read, want 4", len(got.store.loads))
	}
	got.upload()
	want := sheetsOf(kept)
	for k, img := range sheetsOf(got) {
		if !slices.Equal(bytesOf(img), bytesOf(want[k])) {
			t.Errorf("the %s read differs from the one written", sheetNames[k])
		}
	}
	if !got.holds(0) || got.signed[0] != signed || got.holds(1) ||
		got.painted[1] != got.version[1] || got.shots != 1 {
		t.Errorf("held %v and %v, signed %v, sector 1 painted %d of %d, %d holding something; "+
			"want the first held as signed, the second painted empty, one", got.holds(0),
			got.holds(1), got.signed[0], got.painted[1], got.version[1], got.shots)
	}
}

func TestSheetFiles_ClearWhatTheTilesHoldOfASectorNotRead(t *testing.T) {
	gputest.Need(t)
	cfg := SheetsCfg{Dir: t.TempDir(), Keep: true, Version: "1"}
	kept := keptSheets(t, oddHalves{}, cfg, 64, photo{sector: 0, full: true})
	at := time.Unix(2000, 0)
	got := filedSheets(oddHalves{}, cfg, 64, &at)
	got.upload()
	if !got.holds(0) || got.painted[1] == got.version[1] {
		t.Fatalf("held %v, sector 1 painted %v; want the first held, the second not",
			got.holds(0), got.painted[1] == got.version[1])
	}
	for k, img := range []*render.Image{got.albedo, got.relief} {
		pix, was := bytesOf(img), bytesOf(sheetsOf(kept)[k])
		for y := range 125 {
			for x := range 128 {
				i := 4 * (y*128 + x)
				want := was[i : i+4]
				if x >= 65 {
					want = []byte{0, 0, 0, 0}
				}
				if !slices.Equal(pix[i:i+4], want) {
					t.Fatalf("the %s at (%d, %d) is %v, want %v", sheetNames[k], x, y, pix[i:i+4],
						want)
				}
			}
		}
	}
}

func TestSheetFiles_IgnoreTheTilesOfOtherSheets(t *testing.T) {
	gputest.Need(t)
	cfg := SheetsCfg{Dir: t.TempDir(), Keep: true, Version: "1"}
	keptSheets(t, oddHalves{}, cfg, 64, photo{sector: 0, full: true})
	at := time.Unix(2000, 0)
	other := cfg
	other.Version = "2"
	coarser := new(sheets)
	*coarser = sheetsOver(oddHalves{}, 1.0/16, 128, 125)
	coarser.open()
	fileSheets(coarser, cfg, 64, &at)
	for name, s := range map[string]*sheets{
		"of another version":      filedSheets(oddHalves{}, other, 64, &at),
		"of another texel":        coarser,
		"cut in other tiles":      filedSheets(oddHalves{}, cfg, 32, &at),
		"cut in other sectors":    filedSheets(halves{}, cfg, 64, &at),
		"the tiles were cut from": filedSheets(oddHalves{}, cfg, 64, &at),
	} {
		read := len(s.store.loads) > 0 || s.holds(0)
		if want := name == "the tiles were cut from"; read != want {
			t.Errorf("sheets %s read %v, want %v", name, read, want)
		}
	}
}

func TestSheetFiles_LeaveTheSectorsOfATileUnreadToBePhotographed(t *testing.T) {
	gputest.Need(t)
	cfg := SheetsCfg{Dir: t.TempDir(), Keep: true, Version: "1"}
	// the halves cut at the tiles' seam: sector 0's texels in the left tiles, sector 1's the right
	kept := keptSheets(t, halves{}, cfg, 64, photo{sector: 0, full: true},
		photo{sector: 1, full: true})
	if err := os.WriteFile(filepath.Join(cfg.Dir, "relief-1-1.png"), []byte("not a png"),
		0o644); err != nil {
		t.Fatal(err)
	}
	at := time.Unix(2000, 0)
	got := filedSheets(halves{}, cfg, 64, &at)
	if !got.holds(0) || got.holds(1) || len(got.store.loads) != 2 {
		t.Fatalf("held %v and %v, %d tiles read; want the left held from its two", got.holds(0),
			got.holds(1), len(got.store.loads))
	}
	got.upload()
	pix, was := bytesOf(got.albedo), bytesOf(kept.albedo)
	for y := range 125 {
		for x := range 128 {
			i := 4 * (y*128 + x)
			want := was[i : i+4]
			if x >= 64 {
				want = []byte{0, 0, 0, 0}
			}
			if !slices.Equal(pix[i:i+4], want) {
				t.Fatalf("the albedo at (%d, %d) is %v, want %v", x, y, pix[i:i+4], want)
			}
		}
	}
}

func TestSheetFiles_WriteATileTwoSecondsAfterItsLastPhotographAndOneASecond(t *testing.T) {
	gputest.Need(t)
	cfg := SheetsCfg{Dir: filepath.Join(t.TempDir(), "sheets"), Keep: true, Version: "1"}
	at := time.Unix(1000, 0)
	s := filedSheets(oddHalves{}, cfg, 64, &at)
	start := at
	// sector 1's texels lie in the tiles (1, 0) and (1, 1)
	s.store.took([]photo{{sector: 1, full: true}})
	written := func(after time.Duration) []string {
		at = start.Add(after)
		s.store.keep(s)
		s.store.writing.Wait()
		return filesIn(t, cfg.Dir)
	}
	if got := written(sheetSettled - time.Millisecond); len(got) != 0 {
		t.Fatalf("written %v before the tile settled, want nothing", got)
	}
	if got := written(sheetSettled); !slices.Equal(got,
		[]string{"albedo-1-0.png", "relief-1-0.png", sheetManifestFile}) {
		t.Fatalf("written %v once settled, want the first tile and the manifest", got)
	}
	if m, err := s.store.manifest(); err != nil || len(m.Sectors) != 0 {
		t.Errorf("the manifest lists %v (%v), want none while a tile of it waits", m.Sectors, err)
	}
	if got := written(sheetSettled + sheetPace/2); len(got) != 3 {
		t.Fatalf("written %v half a second after, want no more", got)
	}
	if got := written(sheetSettled + sheetPace); len(got) != 5 {
		t.Fatalf("written %v a second after, want the second tile too", got)
	}
	if m, err := s.store.manifest(); err != nil || len(m.Sectors) != 1 || m.Sectors[0].I != 1 ||
		!m.Sectors[0].Inked {
		t.Errorf("the manifest lists %v (%v), want the sector inked", m.Sectors, err)
	}
	s.store.took([]photo{{sector: 1, full: true}})
	before := s.store.dirty[1]
	if written(sheetSettled + sheetPace + sheetSettled/2); !s.store.dirty[1].Equal(before) {
		t.Error("a tile photographed anew written before it settled")
	}
	if written(2*sheetSettled + sheetPace); !s.store.dirty[1].IsZero() {
		t.Error("a tile photographed anew not written once settled")
	}
}

func TestSheetFiles_WriteNothingNotKept(t *testing.T) {
	cfg := SheetsCfg{Dir: filepath.Join(t.TempDir(), "sheets"), Version: "1"}
	at := time.Unix(1000, 0)
	s := filedSheets(oddHalves{}, cfg, 64, &at)
	s.store.took([]photo{{sector: 0, full: true}, {sector: 1, full: true}})
	for range 8 {
		at = at.Add(sheetSettled)
		s.store.keep(s)
		s.store.writing.Wait()
	}
	if _, err := os.Stat(cfg.Dir); !os.IsNotExist(err) {
		t.Errorf("the folder of sheets not kept made (%v), want nothing written", err)
	}
}

// filedRig is a sheetRig of things kept in files as cfg says, tiles 64 a side, its clock at
func filedRig(t *testing.T, k standingKinds, cfg SheetsCfg, at *time.Time,
	things ...stood) *sheetRig {
	s := newSheetRig(t, k, halves{ladder: atL4}, things...)
	fileSheets(&s.r.sheet, cfg, 64, at)
	return s
}

func TestSheetFiles_PhotographASectorReadOnlyAsWhatStandsInItChanges(t *testing.T) {
	gputest.Need(t)
	k := newStandingKinds()
	dome := stood{box: box(600, 400, 88), z: Z{Altitude: 10, Height: 40}, sprite: k.dome,
		fixed: true}
	at := time.Unix(1000, 0)
	cfg := SheetsCfg{Dir: t.TempDir(), Keep: true, Version: "1"}
	first := filedRig(t, k, cfg, &at, dome)
	if f := first.publish(); len(f.sheet.photos) != 1 {
		t.Fatalf("%d photographs with no files, want the right half's", len(f.sheet.photos))
	}
	first.r.develop(nil)
	// the right half's texels lie in the tiles (1, 0) and (1, 1)
	for range 2 {
		at = at.Add(sheetSettled)
		first.r.develop(nil)
		first.r.sheet.store.writing.Wait()
	}
	cfg.Keep = false
	again := filedRig(t, k, cfg, &at, dome)
	if !again.r.sheet.holds(1) {
		t.Fatal("the right half not held as read")
	}
	if f := again.publish(); len(f.sheet.photos) != 0 || !f.sheet.held[1] || len(f.all) != 0 {
		t.Errorf("read, %d photographs, held %v, %d records; want none, held, none",
			len(f.sheet.photos), f.sheet.held, len(f.all))
	}
	again.r.sheet.upload()
	for j, img := range sheetsOf(&again.r.sheet) {
		if !slices.Equal(bytesOf(img), bytesOf(sheetsOf(&first.r.sheet)[j])) {
			t.Errorf("the %s read differs from the one photographed", sheetNames[j])
		}
	}
	raised := dome
	raised.z.Altitude = 30
	changed := filedRig(t, k, cfg, &at, raised)
	if f := changed.publish(); len(f.sheet.photos) != 1 || f.sheet.photos[0].sector != 1 {
		t.Errorf("raised, photographs %v, want the right half's", f.sheet.photos)
	}
}

func TestSheetFiles_WriteTheTilesAsTheSheetsAreDrawnWithNoPhotographLeft(t *testing.T) {
	gputest.Need(t)
	k := newStandingKinds()
	at := time.Unix(1000, 0)
	cfg := SheetsCfg{Dir: t.TempDir(), Keep: true, Version: "1"}
	s := filedRig(t, k, cfg, &at, stood{box: box(600, 400, 88), z: Z{Height: 40},
		sprite: k.dome, fixed: true})
	dst := render.NewImage(128, 125)
	// a frame: what was published drawn, a draw reading the sheet; how many photographs it had
	frame := func() int {
		n := len(s.publish().sheet.photos)
		dst.CopyFrom(s.r.sheet.albedo)
		s.r.sheet.store.writing.Wait()
		return n
	}
	if n := frame(); n != 1 {
		t.Fatalf("%d photographs, want the right half's", n)
	}
	// the right half's texels lie in the tiles (1, 0) and (1, 1), written a second apart
	for _, after := range []time.Duration{sheetSettled, sheetSettled + sheetPace} {
		at = time.Unix(1000, 0).Add(after)
		if n := frame(); n != 0 {
			t.Fatalf("%d photographs again, want none", n)
		}
	}
	if m, err := s.r.sheet.store.manifest(); err != nil || len(m.Sectors) != 1 {
		t.Errorf("the manifest lists %v (%v), want the right half once both its tiles are written",
			m.Sectors, err)
	}
}

func TestSheetFiles_DevelopAsBeforeWithoutFiles(t *testing.T) {
	gputest.Need(t)
	k := newStandingKinds()
	dome := stood{box: box(600, 400, 88), z: Z{Altitude: 10, Height: 40}, sprite: k.dome,
		fixed: true}
	bare := newSheetRig(t, k, halves{ladder: atL4}, dome)
	if bare.r.sheet.store != nil {
		t.Fatal("files kept with no folder")
	}
	at := time.Unix(1000, 0)
	filed := filedRig(t, k, SheetsCfg{Dir: t.TempDir()}, &at, dome)
	for _, s := range []*sheetRig{bare, filed} {
		s.publish()
		s.r.develop(nil)
	}
	for j, img := range sheetsOf(&filed.r.sheet) {
		if !slices.Equal(bytesOf(img), bytesOf(sheetsOf(&bare.r.sheet)[j])) {
			t.Errorf("the %s developed with files differs from the one without", sheetNames[j])
		}
	}
}

func TestSheetFiles_ReadThroughThePlugin(t *testing.T) {
	gputest.Need(t)
	cfg := Config{
		Space:    SpaceCfg{Width: 4096, Height: 512},
		Entities: EntitiesCfg{MaxCount: 8, MinSize: 5, MaxSize: 20},
		Sectors:  SectorsCfg{Buckets: 2},
		LOD:      lod.Ladder{Scales: [lod.Levels - 1]float32{0, 0, 0.5, 0.2, 0.1}},
		Sheets:   SheetsCfg{Dir: t.TempDir(), Keep: true, Version: "1"},
	}
	p := NewPlugin(cfg)
	p.Sheets()
	s := &p.renderer.sheet
	last := s.grid.Len() - 1
	at := time.Unix(1000, 0)
	s.store.now = func() time.Time { return at }
	s.albedo.WritePixels(noise(4*s.cols*s.rows, 1))
	s.store.took([]photo{{sector: last, full: true}})
	at = at.Add(sheetSettled)
	s.store.keep(s)
	s.store.writing.Wait()
	read := NewPlugin(cfg)
	read.Sheets()
	r := read.Read(0)
	if _, _, _, _, perUnit := r.Sheet(); !r.Painted(last) || r.Painted(0) || perUnit == 0 {
		t.Errorf("painted %v and %v, %v texels a unit; want the last sector read painted alone",
			r.Painted(last), r.Painted(0), perUnit)
	}
}

// oneTile is a world 512 a side, one sector
type oneTile struct{}

func (oneTile) Len() int  { return 1 }
func (oneTile) Cols() int { return 1 }
func (oneTile) Box(int) geom.AABB {
	return geom.NewAABB(geom.NewVec(0, 0), geom.NewVec(512, 512))
}
func (oneTile) Of(float32, float32) int   { return 0 }
func (oneTile) Neighbours(int, func(int)) {}

// canopy is a tile side texels a side of crowns 6 texels wide on a lattice of 8, jittered, half of
// it covered: its albedo and its relief
func canopy(side int) (albedo, relief []byte) {
	r := rand.New(rand.NewPCG(5, 1))
	albedo, relief = make([]byte, 4*side*side), make([]byte, 4*side*side)
	for cy := 0; cy < side; cy += 8 {
		for cx := 0; cx < side; cx += 8 {
			mx, my := cx+2+r.IntN(4), cy+2+r.IntN(4)
			g := byte(90 + r.IntN(80))
			for y := max(my-3, 0); y < min(my+3, side); y++ {
				for x := max(mx-3, 0); x < min(mx+3, side); x++ {
					dx, dy := x-mx, y-my
					if dx*dx+dy*dy > 9 {
						continue
					}
					i := 4 * (y*side + x)
					copy(albedo[i:], []byte{g / 3, g, g / 4, 255})
					top := uint16(0x5000 + 64*(9-dx*dx-dy*dy))
					copy(relief[i:], []byte{byte(top), byte(top >> 8), byte(128 + 14*dx),
						byte(128 + 14*dy)})
				}
			}
		}
	}
	return albedo, relief
}

// Benchmark_World_SheetFiles is a tile of the sheets' files of crowns, the GPU's work waited for:
// load put into the sheets at L4 and halved at L5, save read back from them and both its PNGs
// encoded.
func Benchmark_World_SheetFiles(b *testing.B) {
	gputest.Need(b)
	s := new(sheets)
	*s = sheetsOver(oneTile{}, 1, sheetTile, sheetTile)
	s.open()
	s.files = SheetsCfg{Dir: b.TempDir(), Keep: true}
	s.store = newSheetStore(s, sheetTile)
	albedo, relief := canopy(sheetTile)
	tile := tilePixels{rect: image.Rect(0, 0, sheetTile, sheetTile), albedo: albedo,
		relief: relief}
	dst := render.NewImage(1, 1)
	px := make([]byte, 4)
	load := func() {
		s.store.loads = append(s.store.loads, tile)
		s.upload()
		// read as a draw reads it, the sheet at L5 has the tile halved
		dst.CopyFrom(s.reliefL5)
		render.Submit()
		dst.ReadPixels(px)
	}
	save := func() {
		a, r := s.store.readBack(s, tile.rect)
		for _, pix := range [2][]byte{a, r} {
			img := &image.NRGBA{Pix: pix, Stride: 4 * sheetTile, Rect: tile.rect}
			if err := s.store.enc.Encode(io.Discard, img); err != nil {
				b.Fatal(err)
			}
		}
	}
	// the sheets hold the crowns whichever runs alone
	load()
	for _, sc := range []struct {
		name string
		run  func()
	}{{"load,tile", load}, {"save,tile", save}} {
		b.Run(sc.name, func(b *testing.B) {
			sc.run()
			b.ReportAllocs()
			for b.Loop() {
				sc.run()
			}
		})
	}
}
