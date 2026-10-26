# s3. aabbworld i gram: pole wysokości jako dane stożka

**Repo, gałęzie:** aabbworld `s/heights` od `perf/sight-simd` (worktree
`/home/karol/worktrees/aabbworld/heights`); gram `s/heights` od `perf/sight-simd` (worktree
`/home/karol/worktrees/gram/heights`, w jego korzeniu `go.work` od integratora wskazujący ten
worktree aabbworld: nie commitować go, jest wyłączony w `.git/info/exclude`).
**Etap:** `doc/plans-perf.md` gram „SIMD w widzeniu", część C.
**Skille gram:** `gram-plugin`, `gram-plugins` (vision, topography, board), `gram-probe`.
**Lektura aabbworld:** `doc.go` (Sight, Numbers), `view.go` (preludium: `Cone.Heights`,
`Cone.Bend`, `HeightField`), `internal/raycast/elevation.go` (`walkProfile`, `elevation.at`,
`castElevated`, `ray.ground`), `raycast.go` (`collector.take`: `foot`, `standing`).
**Lektura gram:** `plugins/topography/internal/relief/along.go` (`Along`, `folded`,
`cellCorners`, `entered`), `relief.go` (`Lattice`, `GroundAt`, `foldAxis`),
`plugins/vision/scan_system.go` (`ground`, `cone`, `profiling`, `sunkGround`),
`plugins/vision/heights_test.go`, `bench/vision_relief_test.go`.

## Cel

W topography profil gruntu to 53% skanu: `relief.Along` w gram, wołany per promień przez
callback, oddaje 2–3 punkty na komórkę przez kolejny callback, z konwersjami float32↔float64.
Preludium dało aabbworld `Cone.Heights *HeightField` (narożniki, `Cols`×`Rows`, `Cell`,
`WrapX`/`WrapY`; komórka to dwa trójkąty złożone wzdłuż przekątnej o mniejszej różnicy
wysokości; poza siatką 0) i `Cone.Bend` (grunt opada o Bend·d²). Teraz aabbworld ma liczyć profil
z tych danych u siebie, bez callbacków, dając te same punkty co `relief.Along`, a gram ma je
podawać.

## Pliki

aabbworld, wolno: `internal/raycast/elevation.go`, nowe `internal/raycast/heightfield.go` (walk
DDA po komórkach: krawędzie, fałdy, uskok na krawędzi siatki, `Bend`; stopa kandydata `foot` z
pola; `elevation.at` z pola), opcjonalnie `heightfield_simd.go`/`_scalar.go`, testy pakietu,
`bench/view_test.go` (podbenchmark `View_Elevated` z `Heights` obok `Profile`). Nie wolno:
`sweep.go`, `slab.go` (A), `raycast.go` poza odczytem `Cone.Heights`/`Bend` w `Scan` i `take`
(`foot` z pola, gdy `Ground == nil`), `internal/spatial/*` (B), `doc.go`, `CHANGELOG.md`,
`BENCHMARKS.md`, `go.mod`.

gram, wolno: `plugins/vision/scan_system.go` (`ground()`, `cone()`, pola `scanner`),
`plugins/vision/module.go` (jeśli trzeba przepuścić `Lattice`), nowy test w `plugins/vision`,
`bench/vision_relief_test.go` (reżim `heights`). Nie wolno: `Update`/`isDue`/`jobs` w
`scan_system.go` (część D), `plugins/vision/plugin.go` poza jedną opcją, jeśli jest konieczna,
`plugins/topography/*` poza `Lattice()` już istniejącym, `examples/`, dokumenty, skille,
`go.mod`.

## Kroki

1. aabbworld: `walkProfile` z `HeightField`, gdy `Cone.Heights != nil`: DDA po komórkach jak
   `relief.Along` (punkty: fałda komórki, gdy leży na promieniu; wyjście z komórki; drugi punkt na
   krawędzi, gdy wysokość po drugiej stronie jest inna, czyli uskok na krawędzi siatki; koniec w
   `length`), w float32 na wejściu, suma `t` we float64 jak w gram; `Bend` odejmowany od każdej
   wysokości (`alt − Bend·d²`), także w `at` i `foot`. `Ground`/`Profile` zostają dla gruntów
   bez siatki. Test: dla losowych pól i promieni profil z `HeightField` równa się profilowi z
   `ProfileField` zbudowanego tą samą regułą w teście (oracle w teście aabbworld, bez gram).
2. gram: `ScanSystem.ground()`: jeśli grunt ma `Lattice()` (interfejs lokalny w vision:
   `interface{ Lattice() (cols, rows int, cell float32, heights []float32, ok bool) }`), scanner
   dostaje `*aabbworld.HeightField` nad tym samym `[]float32` (bez kopii; relief go trzyma) i
   `cone()` ustawia `Heights` i `Bend` zamiast `Profile`, `Ground` i `sunkGround`. Grunty bez
   siatki (hex) idą jak dotąd.
3. gram, test krzyżowy w `plugins/vision`: ten sam relief, ci sami obserwatorzy, skan przez
   `Profile` (jak dziś) i przez `Heights`: identyczne `Sighted` (zbiór i porządek), `Depths` w
   tolerancji 1e-3·promień, także ze `world.Scale` (Bend ≠ 0). Pokrycie: fałda przecięta,
   krawędź planszy, torus (`WrapX`).
4. Test dymny kosztu: `go test ./bench -run xxx -bench Vision_Relief -benchtime=300ms` przed i
   po (bez raportowania jako wyniku): reżim `profile` ma zmaleć; `Benchmark_Vision_Relief` dostaje
   reżim porównawczy, jeśli oba sposoby mają zostać mierzalne.
5. Lockstep ×8 promieni po polu (SIMD): tylko gdy skalarny profil już działa i test dymny
   pokazuje, że DDA jest dalej dominujący; `archsimd` nie ma gatherów (narożniki ładowane
   skalarnie), więc bez wyraźnego zysku zostawić skalarnie i opisać w raporcie, co by trzeba.

## Poza zakresem

Mip wysokości i pomijanie gruntu, który nie podniesie horyzontu (osobny etap). Osłona
(`Cover.Walk`). Kto jest skanowany (D). Zmiany w `relief.Along` samym.

## Zasady

Sekwencja przed każdym commitem w obu repo: `go build ./... && go vet ./... && gofmt -l . &&
go test ./...`, w gram także z `GOEXPERIMENT=simd` i z `GOWORK=off`? Nie: gram w tym worktree
buduje się tylko przez `go.work` (potrzebuje aabbworld z tej części); `GOWORK=off` sprawdzi
integrator po tagu. Commity konwencjonalne (`feat:` w aabbworld, `perf:` w gram), trailer
`Claude-Session: https://claude.ai/code/session_014GbD5V5FZgxcEYK1zPRzjF`, bez `Co-Authored-By`.
Bez pushu. Liczby mierzy integrator. W gram żadne pliki demo nie są ruszane; sonda
(`gram-probe`) na kopii, jeśli potrzebna.

## Raport

Które punkty profilu różnią się od `relief.Along` i dlaczego (ma nie być żadnych); co liczy
aabbworld, co zostało w gram; czy lockstep wszedł; lista plików w obu repo; polecenia
benchmarków (`View_Elevated`, `Vision_Relief` reżimy) i oczekiwany kierunek; propozycje wpisów do
CHANGELOG obu repo i do skilla `gram-plugins` (vision); szwy z A/B (scratch, `take`) i D
(`scan_system.go`).
