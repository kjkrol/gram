package world

import (
	"unsafe"

	"github.com/kjkrol/uid"
)

// standers are the things standing for good as Publish last walked them, kept between frames:
// by chunk what a sheet makes of each, by sector the signature of those living by their middle
// in it and photographed there, and their photographs' instances. A chunk is walked anew as it
// comes, goes, grows, shrinks or has its table moved, every chunk as the world's Restills move:
// the frames between read what is kept.
type standers struct {
	chunks   map[*uid.UID64]*standerChunk
	sig      []signature
	restills uint64
	frame    uint32 // the publish under way, to find the chunks gone
	rebuilds int
}

// a chunk of things standing for good as last walked: how many its ids were and its table's
// version then, the publish it was last met in, and what a sheet makes of each of its things
type standerChunk struct {
	n     int
	ver   uint32
	frame uint32
	slots []stander
}

// stander is what a sheet makes of a thing standing for good: live, whether it is read as the
// frame finds it (it lives in a cell of the tiers or crosses a seam, so a view may draw it alive
// by a sector drawn finer); else the sector its middle lies in, one more than it where it is
// photographed there (0 where its kind declares a look at L4 or it has no look to photograph),
// its sign and the instance of its photograph
type stander struct {
	live   bool
	sector int32
	sheet  int32
	h      uint64
	shot   shot
}

// begin readies a publish, every chunk of n sectors' standers to be met or found gone; it
// reports whether every chunk is to be walked anew, the Restills having moved to restills
func (k *standers) begin(n int, restills uint64) (all bool) {
	k.frame++
	if k.chunks == nil {
		k.chunks = map[*uid.UID64]*standerChunk{}
	}
	if len(k.sig) != n {
		k.sig, all = make([]signature, n), true
	}
	if restills != k.restills {
		k.restills, all = restills, true
	}
	if all {
		for _, c := range k.chunks {
			c.n = -1
		}
	}
	return all
}

// meet is the chunk whose ids are ids, its table's version ver, as last walked, and whether it is
// to be walked anew, the chunk's slots then to be filled by the caller through sign
func (k *standers) meet(ids []uid.UID64, ver uint32) (c *standerChunk, anew bool) {
	key := unsafe.SliceData(ids)
	c = k.chunks[key]
	if c == nil {
		c = &standerChunk{}
		k.chunks[key] = c
	}
	c.frame = k.frame
	if c.n == len(ids) && c.ver == ver {
		return c, false
	}
	k.unsign(c)
	c.n, c.ver, c.slots = len(ids), ver, c.slots[:0]
	k.rebuilds++
	return c, true
}

// sign keeps st as the next of c's things and signs it into its sector where a sheet
// photographs it there
func (k *standers) sign(c *standerChunk, st stander) {
	c.slots = append(c.slots, st)
	if !st.live && st.sheet > 0 {
		k.sig[st.sector].n++
		k.sig[st.sector].mix ^= st.h
	}
}

// unsign takes c's things out of their sectors' signatures
func (k *standers) unsign(c *standerChunk) {
	for i := range c.slots {
		if st := &c.slots[i]; !st.live && st.sheet > 0 {
			k.sig[st.sector].n--
			k.sig[st.sector].mix ^= st.h
		}
	}
}

// settle forgets the chunks the publish did not meet
func (k *standers) settle() {
	for key, c := range k.chunks {
		if c.frame != k.frame {
			k.unsign(c)
			delete(k.chunks, key)
		}
	}
}

// shots appends to dst the instance of each thing photographed in sector i living by its middle,
// span the most they reach up or down from zero at least
func (k *standers) shots(i int, dst []float32, span *float32) []float32 {
	for _, c := range k.chunks {
		for j := range c.slots {
			if st := &c.slots[j]; !st.live && st.sheet > 0 && int(st.sector) == i {
				dst = appendShot(dst, &st.shot, span)
			}
		}
	}
	return dst
}
