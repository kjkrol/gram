# s2. aabbworld: kolumny siatki i zapytanie stożkiem po osiem kawałków

**Repo, gałąź:** aabbworld, `s/cone-query` od `perf/sight-simd` (worktree
`/home/karol/worktrees/aabbworld/cone-query`). Moduł buduje się sam, bez `go.work`.
**Etap:** `doc/plans-perf.md` gram „SIMD w widzeniu", część B.
**Lektura:** `doc.go` (Sight, SIMD, Package dependencies), `internal/spatial/doc.go`,
`grid.go` (`list`, `Query`, `gridCell`, `starts`), `pair_sweep_simd.go` (wzór jądra i kolumn
`sweepCols`), `internal/raycast/raycast.go` (`gather`, `collector.take`, `searchAreas`),
`shadow.go` (`wedge.excludes`, `subtendedArc`), `space.go` (`Space.Query`: `WrapAABB`,
fragmenty), `view.go` (adapter `sight`, preludium: `QueryCone` odpowiada kołem).

## Cel

Gather wzroku woła `Query` obszarów wokół obserwatora i dla każdego zwróconego kawałka robi w
`take` najbliższy obraz, odległość, klin (osiem iloczynów wektorowych) i dwa `atan2`. Ma
dostawać tylko to, co naprawdę sięga stożka: `Grid.QueryCone` filtruje promień (odległość
pudełka od początku ≤ promień), klin (`wedge.excludes`: całe pudełko poza jedną krawędzią) i
komórkę właściciela po osiem kawałków naraz na kolumnach float32, a `atan2` liczy się tylko dla
przeżywających. Kolumny (`minX`, `minY`, `maxX`, `maxY`, caps) powstają w `list()` razem z
komórkami, raz na tick, i są wspólnym fundamentem dla `Query` (tam zysk mały, nie celem) i
gathera.

## Pliki

Wolno: `internal/spatial/grid.go`, nowe `internal/spatial/cone_query.go`, `cone_query_simd.go`
(`//go:build goexperiment.simd && amd64`), `cone_query_scalar.go`, testy pakietu;
`internal/raycast/raycast.go` (tylko `gather`/`collector`: przejście na `QueryCone`, klin i
promień już sprawdzone), `internal/raycast/raycast_test.go` (fake `QueryCone` ścisły), `view.go`
(tylko ciało `sight.QueryCone`), `space.go` (opcjonalnie publiczne `Space.QueryCone`, jeśli
wyjdzie naturalnie; inaczej nie), `bench/view_test.go` i `bench/broad_phase_test.go` (nowe
podbenchmarki). Nie wolno: `internal/raycast/sweep.go`, `slab.go`, `elevation.go` (A i C),
`pair_sweep*.go` poza współużyciem `wide`, `doc.go`, `BENCHMARKS.md`, `CHANGELOG.md`, `README.md`,
`go.mod`.

## Kroki

1. Kolumny w `Grid`: równolegle do `cells` (ten sam indeks), budowane w `list()`; pamięć
   trzymana między tickami jak `cells`. Koszt `list()` zmierzyć dymnie: `Grid_RebuildPairsQuery`
   i `BroadPhase` nie mogą drgnąć więcej niż o szum (to też sprawdzi integrator).
2. `Grid.QueryCone(origin, dir geom.Vec, halfAngle, radius float32, want Capability, fn)`:
   komórki w kole wokół stożka (można ciaśniej: prostokąt wycinka), w komórce po osiem kawałków:
   maska promienia (kwadrat odległości pudełko–punkt), maska klina (dla półkąta < π/2; szerszy
   klin nic nie odrzuca, jak `newWedge`), maska caps, test właściciela jak w `Query` (całkowity,
   po `cellOf` z kolumn); ogon skalarnie tą samą drogą. Semantyka: nic w stożku nie ginie; coś tuż
   poza może przejść.
3. Torus: `sight.QueryCone` wywołuje `Grid.QueryCone` dla obszaru zawiniętego i każdego fragmentu
   (jak `Space.Query`), z początkiem stożka przesuniętym o świat dla fragmentu; `collector.take`
   dalej robi `nearestImage` i dedup.
4. `gather` na `QueryCone` zamiast `searchAreas` + `Query`; w `take` zostają: dedup, `EntryAABB`,
   `nearestImage`, `boxDistance` (potrzebny do `dist`), `Transparency`/`Elevation`/`Ground`,
   `subtendedArc`. Klin i promień sprawdzone po raz drugi skalarnie są dozwolone tylko jako
   asercja w testach, nie w kodzie.
5. Testy: (a) `QueryCone` przeciw brute force na losowych scenach: zbiór zwrócony ⊇ zbiór pudełek
   przecinających stożek (koło ∩ klin) i ⊆ zbiór pudełek w kole; identyczny zbiór w obu buildach;
   (b) na torusie przez szew; (c) wszystkie testy raycast i `View_*` dają ten sam `Entities` co
   przed zmianą (gather przez `Query`): test porównawczy z fakiem bez filtra.

## Poza zakresem

Slab i sweep kątów (A). `HeightField` (C). SIMD w `Query` dla cullingu kamer (nie warto; wolno
tylko jeśli wypadnie za darmo z tych samych kolumn, z testem równoważności). Dokumenty.

## Zasady

Sekwencja przed każdym commitem: `go build ./... && go vet ./... && gofmt -l . && go test ./...`
oraz `GOEXPERIMENT=simd go test ./...`. Commity konwencjonalne (`perf:`, `feat:` dla publicznego
`QueryCone`), trailer `Claude-Session: https://claude.ai/code/session_014GbD5V5FZgxcEYK1zPRzjF`,
bez `Co-Authored-By`. Bez pushu. Liczby mierzy integrator.

## Raport

Co filtruje SIMD, a co zostało w `take`; koszt kolumn w `list()` (dymnie); lista plików;
polecenia benchmarków (`View_Visible`, `View_Both`, `View_Elevated`, `BroadPhase`,
`Grid_RebuildPairsQuery`) i które wiersze mają się ruszyć; propozycja wpisu do CHANGELOG i zdań
do `doc.go`; szwy z A (scratch) i C (`gather` z `Ground`/`Heights`).
