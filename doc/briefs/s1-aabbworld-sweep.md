# s1. aabbworld: sweep kątów wzroku po osiem naraz

**Repo, gałąź:** aabbworld, `s/sweep` od `perf/sight-simd` (worktree
`/home/karol/worktrees/aabbworld/sweep`). Moduł buduje się sam, bez `go.work`.
**Etap:** `doc/plans-perf.md` gram „SIMD w widzeniu", część A.
**Lektura:** `doc.go` (sekcje Sight, Numbers, SIMD), `collide/doc.go`, `BENCHMARKS.md`
(„Wide pair sweep": jak zmierzono i opisano sweep par), `internal/spatial/pair_sweep_simd.go`
(wzór jądra: kolumny float32, broadcast, `Float32x8`, maski, `ClearAVXUpperBits`),
`internal/raycast/sweep.go`, `slab.go`, `elevation.go`, `raycast.go`.

## Cel

Skan stożka (`raycast.Scan`) liczy per kąt `hitDistance` dla każdego aktywnego kandydata, kąt po
kącie, skalarnie, we float64. Między dwoma eventami (`walk`: wejście albo wyjście kandydata)
zbiór aktywnych jest stały, więc dla każdego aktywnego kandydata można policzyć `near`/`far`
dla ośmiu kolejnych kątów naraz (`Float32x8`: wektor `dir.x`/`dir.y` ośmiu promieni, pudełko
rozgłoszone, `1/dir`, min/max/merge dla znaków i pasów z `dir == 0`). Porządek przecięć
(`insertCrossing`, `spend`), osłona (`castCover`, callback) i wysokości (`castElevated`) zostają
skalarne i czytają slab z bufora scratch. Wyniki: ten sam zbiór encji i te same trafienia co
skalarnie, głębokości w tolerancji 1e-3·promień (decyzja użytkownika: float32, osiem pasów).

## Pliki

Wolno: `internal/raycast/sweep.go`, `slab.go`, `elevation.go` (tylko czytanie slabu z bufora),
nowe `internal/raycast/slab_simd.go` (`//go:build goexperiment.simd && amd64`), `slab_scalar.go`
(`const wide = false`), testy pakietu, `bench/view_test.go` (nowy podbenchmark, jeśli trzeba).
Nie wolno: `internal/raycast/raycast.go` (gather i `collector` to część B), `internal/spatial/*`
(część B), `view.go`, `space.go`, `doc.go`, `BENCHMARKS.md`, `CHANGELOG.md`, `README.md`,
`go.mod`.

## Kroki

1. Profil przed, do raportu (nie do BENCHMARKS): `go test ./bench -run xxx -bench
   'Benchmark_View_(Visible|Both|Elevated|Cover)$' -cpuprofile` i top 25: udziały gathera
   (`collector.take`, `subtendedArc`, `Query`), sweepu (`castFlat`, `hitDistance`,
   `insertCrossing`), osłony (`castCover`), wysokości (`castElevated`, `ray.ground`).
2. Bufor slabu w `scratch`: `near`, `far`, `ok` dla bloku do ośmiu kątów × aktywni kandydaci,
   wypełniany jądrem SIMD (gdy `wide` i blok ma ≥ 2 kąty; inaczej skalarnie tą samą drogą).
   `walk` grupuje kąty między eventami w bloki; `castFlat`/`castElevated` dostają slab z bufora
   zamiast liczyć `hitDistance`. Promienie są już zaokrąglane do float32 (`round32`), więc
   `dir` we float32 to te same promienie; różnica jest w arytmetyce slabu.
3. Bramka jak w sweepie par: `var wide = archsimd.X86.AVX2()` w pliku simd, `const wide = false`
   w skalarnym; `ClearAVXUpperBits` przed callbackami i na wyjściu z jądra.
4. Testy: (a) równoważność na losowych scenach i stożkach (jak
   `TestPairSweep_WideFindsWhatNarrowFinds`: ten sam zbiór `Entities`, te same `hit`/`id` w
   `Depths`, głębokości w 1e-3·promień), w obu
   buildach (`GOEXPERIMENT=simd` i bez); (b) przypadki brzegowe slabu: promień wzdłuż osi
   (`dir.x == 0`), początek w pudełku, pudełko za plecami, promień stykający się z krawędzią;
   (c) istniejące testy raycast bez zmian oczekiwań, poza tolerancją tam, gdzie porównują
   głębokość dokładnie.
5. Test dymny kosztu (krótko, `-benchtime=200ms`, bez raportowania liczb jako wyniku): jądro nie
   może pogorszyć `View_Visible`/`View_Both`; gdy blok ma mało kątów, skalar.

## Poza zakresem

Gather, `QueryCone`, kolumny siatki (B). Profil gruntu z `HeightField` (C). Zmiany API
publicznego. Dokumenty i CHANGELOG (integrator).

## Zasady

Sekwencja przed każdym commitem: `go build ./... && go vet ./... && gofmt -l . && go test ./...`
oraz `GOEXPERIMENT=simd go test ./...`. Commity konwencjonalne (`perf:`, `test:`), trailer
`Claude-Session: https://claude.ai/code/session_014GbD5V5FZgxcEYK1zPRzjF`, bez `Co-Authored-By`.
Bez pushu. Liczby mierzy integrator na bezczynnej maszynie.

## Raport

Profil przed (udziały); co jest w SIMD, a co zostało skalarne i dlaczego; lista plików; dokładne
polecenia benchmarków do zmierzenia (`go test ./bench -run xxx -bench ... -count N`) i które
wiersze mają się ruszyć; propozycja wpisu do CHANGELOG aabbworld i zdania do `doc.go` (sekcja
SIMD); otwarte szwy z częściami B i C.
