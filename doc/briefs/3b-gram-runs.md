# 3b. gram: motion, world i collision na przebiegach aabbworld

**Repo, gałąź:** gram, `c/runs` od `main` po podbiciu aabbworld w `go.mod` (worktree
`/home/karol/worktrees/gram/runs`). Zależy od wydania 3a.
**Etap:** 3 (`doc/plans-perf.md` „Etap 3”, kroki 3–4).
**Skille:** `goke`, `gram-plugin`, `gram-plugins` (world, motion, collision), `gram-probe`.

## Cel

`motion.moveSystem.rebuild` zbiera przebiegi z `held` (`Base` bez `sector.Off`) zamiast
kopiować Item; `world.indexSystem` i `unindexed` (`module.go`) znikają albo sprowadzają się
do ponownego zebrania przebiegów; `collision.rebuild` bez kopii poza sweepami; `collision.moved`
bez `Seek`. Pozycja żyje raz na tick.

## Pliki

Wolno: `plugins/motion/move_system.go` (+ testy), `plugins/world/module.go` (indeksowanie),
`plugins/collision/collision_system.go`, `plugins/collision/physics.go` (tylko `moved`/weto),
testy tych pakietów, `bench/world_test.go`, `bench/collision_test.go`. Nie wolno: aabbworld
(wydane), goke, `Base` jako typ (ponad sto plików), ceremony, `examples/`.

## Kroki

1. Pomiar przed: `Benchmark_World_Tick` (1 000, 5 000) i `Benchmark_Collision_Tick` z
   profilem; zapisać udział `rebuild`, `remember`, `Seek`.
2. Przebiegi zbierane po każdym `Sync` goke (swap-and-pop przy migracji przenosi encje między
   slotami); strażnik `TableVer` ze `ChunkSnapshot`: przebiegi na nowo, gdy wersja
   którejkolwiek tabeli się zmieniła. `Base` ma pudełko pod offsetem 0 i Caps pod 100: jeden
   przebieg na chunk z zapytania po `Base`. Sweepy (pociski) jako lista Item za przebiegami.
3. Weto z `moved` (`response.Worse`: ruch na gorszy grunt odrzucony, `stale`) musi zostać:
   jako sprawdzenie w `Touch` silnika przed pchnięciem albo jako przejście po `Tick` po liście
   pchniętych cofające pudełko; wybrać to, co zachowa dzisiejsze testy kolizji i probe
   mechanisms/platformer bez zmiany zachowania.
4. Pomiar po: oba benchmarki z profilem (oczekiwanie: `rebuild` do ułamka procenta, `seek`
   znika z profilu kolizji). Probe w kopii: motion/collision, motion/bullet (sweepy),
   navigation/spawn (spawn i despawn w biegu: `Sync` między ruchem a kolizją), rules/mechanisms,
   motion/platformer; każde pod `timeout`, bez paniki; zrzuty z `PROBE_SEED` przed i po tam,
   gdzie demo jest deterministyczne.

## Poza zakresem

Wersje per chunk (4b), `Mut[T]`. Zmiana w tym, co faza szeroka zwraca.

## Raport

Liczby przed/po, profile, decyzja o wecie i dlaczego, proponowane wpisy do `CHANGELOG.md` i
skilla `gram-plugins`.
