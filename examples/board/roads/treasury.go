package main

// treasury is the game's gold, a resource kept with the game: the roads built are paid out of it,
// and the builders refuse once it is empty.
type treasury struct{ Gold float32 }
