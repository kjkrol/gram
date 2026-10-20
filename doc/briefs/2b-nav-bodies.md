# 2b. Nawigacja: ciała bez closures, `aside` ze scratchem, back-off po pomiarze

**Repo, gałąź:** gram, `n/bodies` od `perf/navigation` (po scaleniu 2a), worktree
`/home/karol/worktrees/gram/bodies`.
**Etap:** 2 (`doc/plans-gc.md` zadanie 3, kroki 2–4; benchmark z kroku 1 już jest).
**Skille:** `gram-plugin`, `gram-plugins` (navigation), `ceremony`, `gram-probe`.

## Cel

Per ciało per tick: closure w `bodies.go` (`CellsUnder`), `near()` i closures w
`body_keeping.go`, `Leg.cells()`/`held()` alokujące na każde wywołanie, `cellKeeping.aside`
budujące i sortujące slice z closure; zablokowana jednostka co 500 ms powtarza pełne A*,
które znów się nie udaje. Najpierw mechanika bez zmiany zachowania, potem decyzja o back-off
z profilu reżimu „pod murem”.

## Pliki

Wolno: `plugins/navigation/{bodies,body_keeping,keeping,navigation_system,bump}.go` i ich
testy, `bench/navigation_test.go`. Nie wolno: `internal/pathfind` (2a), board, ceremony,
`examples/`.

## Kroki

1. Profil przed w reżimie crowded 1 000 (po 2a). Zapisać top 20.
2. Mechanicznie: stan closures do pól i jeden method value bindowany w konstruktorze (jak
   collision); `Leg.cells()`/`held()` zwracają `[4]cell.ID`/`[4]spot` z licznikiem; `aside`
   na scratchu `[8]scored` w `cellKeeping`. Benchmark po każdym kroku; `contacts`/ścieżki
   identyczne (testy pakietu).
3. Back-off tylko, jeśli po kroku 2 nieudane A* dominuje w profilu crowded: moment ostatniej
   porażki przy jednostce, odstęp rosnący 500 ms → 1 s → 2 s do pułapu, zerowany, gdy cel albo
   zajętość komórek na drodze się zmieni (licznik zmian zajętości; jeśli go nie ma, dodać, nie
   heurystykę). To zmiana zachowania: zaznaczyć w raporcie i w doc.go pluginu.
4. Probe w kopii: navigation/spawn (tłum pod murem, mur zwolniony, jednostki ruszają bez
   zauważalnej zwłoki), navigation/square (wagon za szeroki na most utyka tak jak dziś);
   zrzuty `GRAM_SHOT` z `PROBE_SEED` przed i po.

## Poza zakresem

A*, `Neighbors` (2a). Zasada „nigdy nie widzi innych z wyprzedzeniem” zostaje.

## Raport

Liczby przed/po (`Navigation_Tick` 4 reżimy, alokacje), profil po, czy back-off wszedł i na
jakiej podstawie, proponowane wpisy do `CHANGELOG.md`, doc.go i skilla.
