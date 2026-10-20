# 2a. Nawigacja: ścieżka A* bez map i bez alokacji

**Repo, gałąź:** gram, `n/astar` od `main` (worktree `/home/karol/worktrees/gram/astar`).
**Etap:** 2 (`doc/plans-perf.md` „Etap 2”, `doc/plans-gc.md` zadanie 2 i wynik etapu 0 pkt 3).
**Skille:** `gram-plugin`, `gram-plugins` (board, navigation), `gram-probe`.

## Cel

Największy koszt ticku to nawigacja: 1 000 jednostek 10,8 ms w otwartym polu, 270 ms w tłoku;
A* 55–81% tego, a pod nim odczyty komórek przez mapy (`Cells.Kind`/`KindAt`, `Square.Toward`,
`boardLevels.Deck`) i `Neighbors` alokujący slice na każdy węzeł. Zejść z tego, nie zmieniając
ani jednej wybranej ścieżki.

## Pliki

Wolno: `plugins/navigation/internal/pathfind/*`, `plugins/navigation/path_finder.go`,
`plugins/navigation/spots.go`, `plugins/navigation/keeping.go` (tylko wywołania `Neighbors`),
`plugins/board/internal/grids/{square,hex}.go`, `plugins/board/grid/grid.go` (tylko jeśli
dochodzi `NeighborsInto`/iteracja po kierunkach), `bench/navigation_test.go` (nowy benchmark
samej ścieżki), testy tych pakietów. Nie wolno: `bodies.go`, `body_keeping.go`,
`navigation_system.go` (to 2b), ceremony, `examples/`.

## Kroki

1. Profil przed: `go test ./bench -run xxx -bench Benchmark_Navigation_Tick -cpuprofile`
   dla obu reżimów (open, crowded; 100 i 1 000); zapisać top 20. Dodać
   `Benchmark_Navigation_Path`: sama ścieżka na siatce 200×200 z przeszkodami, `ReportAllocs`.
2. Odczyty komórek na ścieżce A* (`Price`, `KindAt`, `Toward`, `Deck`) bez map: tablica po
   indeksie komórki albo odczyt z kolumny, zdecydowane po profilu; każdy krok mierzony.
3. `Neighbors` bez alokacji w A* i `aside`: wariant (b) z `plans-gc` zadanie 2 (iteracja po
   `Directions()`/`Toward`), (a) `NeighborsInto` tylko tam, gdzie lista jest potrzebna; benchmark
   `Toward` przez interfejs w `plugins/board/internal/grids`.
4. Test równoważności: dla stu losowych par start–cel na siatkach kwadratowej i hex ścieżka
   przed i po jest identyczna (test porównujący z wersją sprzed zmiany, np. przez zachowaną
   kopię funkcji ceny albo zrzut ścieżek do testdata).
5. Pomiar po: oba benchmarki, profil; demo navigation/square i navigation/spawn w kopii pod
   `timeout`, jednostki wysłane wokół muru, bez paniki.

## Poza zakresem

Closures per ciało, `aside` scratch, back-off (2b). Jakakolwiek zmiana w tym, jak jednostki
omijają się nawzajem.

## Raport

Liczby przed/po dla `Navigation_Tick` (4 reżimy) i `Navigation_Path`; profil po; lista
plików; proponowany wpis do `CHANGELOG.md` i do skilla `gram-plugins` (navigation, board).
