package lod

import "github.com/kjkrol/uid"

// Seed is the number the cell'th place of the aggregate whole draws from, the same at every
// unfold, so its parts stand where they stood; what has changed since is the aggregate's own.
func Seed(whole uid.UID64, cell uint64) uint64 {
	return mixed(mixed(uint64(whole)+golden) + cell*golden)
}

// splitmix64's step and finalizer
const golden = 0x9e3779b97f4a7c15

func mixed(x uint64) uint64 {
	x = (x ^ x>>30) * 0xbf58476d1ce4e5b9
	x = (x ^ x>>27) * 0x94d049bb133111eb
	return x ^ x>>31
}
