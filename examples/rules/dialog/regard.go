package main

import "github.com/kjkrol/uid"

// Regard is what a host makes of those it talked with: how much it likes each, from -100 to
// 100, eight at most. Full, it forgets the one it feels least about.
type Regard struct {
	Of    [8]uid.UID64
	Likes [8]int8
	Used  [8]bool // entity 0 is an entity: no zero for nobody
}

const (
	friendFrom = 30
	enemyFrom  = -30
)

// Like adds by to how much the host likes who.
func (r *Regard) Like(who uid.UID64, by int8) {
	i := r.keep(who)
	r.Likes[i] = int8(max(-100, min(100, int(r.Likes[i])+int(by))))
}

// Liking is how much the host likes who; 0 for one it does not remember.
func (r *Regard) Liking(who uid.UID64) int8 {
	if i := r.slot(who); i >= 0 {
		return r.Likes[i]
	}
	return 0
}

// Friendly reports whether the host is a friend to who.
func (r *Regard) Friendly(who uid.UID64) bool { return r.Liking(who) >= friendFrom }

// Hostile reports whether the host is an enemy to who.
func (r *Regard) Hostile(who uid.UID64) bool { return r.Liking(who) <= enemyFrom }

// Stance is the host's regard for who in a word: Friend, Neutral or Enemy.
func (r *Regard) Stance(who uid.UID64) string {
	switch {
	case r.Friendly(who):
		return "Friend"
	case r.Hostile(who):
		return "Enemy"
	}
	return "Neutral"
}

func (r *Regard) slot(who uid.UID64) int {
	for i := range r.Of {
		if r.Used[i] && r.Of[i] == who {
			return i
		}
	}
	return -1
}

func (r *Regard) keep(who uid.UID64) int {
	if i := r.slot(who); i >= 0 {
		return i
	}
	at := -1
	for i := range r.Of {
		if !r.Used[i] {
			at = i
			break
		}
		if at < 0 || abs(r.Likes[i]) < abs(r.Likes[at]) {
			at = i
		}
	}
	r.Of[at], r.Likes[at], r.Used[at] = who, 0, true
	return at
}

func abs(v int8) int {
	if v < 0 {
		return -int(v)
	}
	return int(v)
}
