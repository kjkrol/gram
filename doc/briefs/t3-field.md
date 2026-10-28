# Etap 3: plan grupy — pole przepływu (2026-10-28)

Gałąź `feat/field` od `main` (po scaleniu etapu 2), worktree `/home/karol/worktrees/gram/field`.
Plan: `doc/plans-perf.md` „Kolejka i pole grupy" (etap 3) i „: wynik" etapów 1–2; zasady
wspólne: `doc/briefs/README.md`. Skille przed kodem: `gram-perf-navigation` (niezmiennik wyceny:
udział na płaskim raz na komórkę i wersję, strategia na krok nad reliefem; odpowiedzi
pamiętane po wersji), skill astara `/home/karol/CODE/astar/.claude/skills/astar/SKILL.md`,
`gram-plugin` (układ pakietu), `gram-plugins` (navigation). **Bez pomiarów czasu** w tej części.

## Cel

Jedno liczenie na rozkaz grupy zamiast N szukań i replanów: pole przepływu od komórki celu
(koszt do celu i najlepszy następny krok każdego węzła {komórka, poziom}), z którego trasę
każdego członka czyta się marszem po polu, przy pierwszej prośbie i przy każdym replanie;
zmiana wersji planszy to jedno przeliczenie. Keepingi, dotyk i reguły tłumu nietknięte;
jednostki dalej uczą się o sobie uderzając. Etap 4 posortuje członków po koszcie z pola, więc
pole odsłania `Cost`.

## Kontrakty (nazwy zamknięte przez użytkownika: `pathfind.Field`, `spacing.Fields`,
`fieldParty`, `Benchmark_Routing_Group`)

- **Klucz pola**: komórka celu grupy (`grid.CellAt(o.At)`), `o.Level`, domena, klasa ciała
  `pathfind.Body{Step, Height, Size}` z **wyzerowanym `Feet`** (wysokość członka psułaby
  wspólność klucza; własne `Feet` służy tylko poziomowi startu marszu). Numer `Group` decyduje
  tylko, czy partia jest dość liczna: `fieldParty = 4` żyjących rozkazów z tym `Group`
  (liczone w przebiegu koordynatora, który i tak odwiedza każdy rozkaz; mniejsza partia zostaje
  przy A*, więc `routes.golden` i zatłoczony tick bez zmian). **`CellKeeping.Orders` musi
  ustawiać `At`** jak `BodyKeeping.Orders` (`cmd.At` albo środek `cmd.Cell`), także w
  `Goal` przy `Append` — dziś wydaje rozkaz bez `At`.
- **`pathfind.Field`** (`internal/pathfind/field.go`, nowy): `cost []float32`, `next []uint8`
  po slotach `2·ordinal+poziom`; `Cost(c, l) (float32, bool)` (`Either` tańszy poziom; fałsz
  nieosiągnięte), `Next(c, l) (cell.ID, Level, bool)`, `Reached(c) bool`, `Goal() cell.ID`.
- **`Finder.FillField(f, goal, end, domain, body, seeds []cell.ID, margin float32) (reached
  int)`**: Dijkstra od źródeł = poziomów komórki celu dopuszczonych przez `end`, po węzłach
  {komórka, poziom}, z **przejściami w lustrze**: dla osiadłego `t` i sąsiada `n` (każdy
  kierunek, każdy poziom) krok naprzód `n → t` sprawdzany i wyceniany dokładnie jak
  `leveledFrom` z `n` (`CanEnter`, `crosses`, `cut && !along`, `passes`, `fits`, `admits`,
  `price(n, t)`: stok od `n` do `t`): wyłuskać ciało pętli `leveledFrom` do funkcji `step(from
  node, rf, rn *read, level, c corner, cut) (float64, bool)` i użyć jej po obu stronach, **ta
  sama kolejność sprawdzeń** (golden `TestRoutes_AsPinned` pilnuje); bez osobnego lustra dla
  `plainFrom` (bez poziomów `leveledFrom` redukuje się do niego). `p.begin(domain)` raz na
  budowę; `p.blocked = nil`, `p.entity = 0` (pole tylko dla ślepego findera). Kopiec leniwy
  (`keys []float32`, `ids []int32`, duplikaty wpychane, nieaktualne pomijane; remisy jak w
  astarze: `!(key < keys[i])` w górę, ściśle `<` w dół), slice'y w Finderze wielokrotnego użytku.
  Ziarna stemplowane po ordynale; stop, gdy wszystkie osiadły i koszt frontu > `najdalsze ·
  margin` (`fieldMargin = 1.5`), albo front pusty. `fieldCells = 1<<18`: większa plansza nie
  dostaje pola. Zwraca liczbę ziaren osiadłych. `Finder.Fills() uint64` jak `Searches()`.
- **`Finder.Walk(f, entity, domain, body, from, target, end, dst []cell.ID) (int, bool)`**,
  `Decks()` jak po `Find`: start na poziomie z prawdziwego `Feet` (`levelOf`); nieosiągnięta →
  `(0, false)`; marsz po `next` do `len(dst)` (kawałek 64 kroków jak dziś: następna prośba
  idzie dalej od końca kawałka), bity `Decks`; gdy komórka == `f.Goal()` albo
  `grid.Distance(cur, target) <= fieldLastLeg` (2): ostatni odcinek do własnego celu członka
  przez `findFrom(entity, domain, body, start node, to, end, dst[n:])` wyłuskane z `Find`
  (A* ze znanego poziomu, kilka rozwinięć); `!ok` → `(0, false)` (wołający spada do A*).
  `Path.From = from`, `Steps` bez `from`, jak `FindPath`.
- **`spacing.Fields`** (`internal/spacing/fields.go`, nowy): `fieldAsked{goal, end, domain,
  body}` jak `asked`; wpisy `fieldEntry{key, field, seeds, state free|expected|building|done,
  used}`, `keptFields = 8` (LRU, tablice odzyskiwane); `mu`, `worked sync.Cond`; związane z
  finderem jak `avoiding.bind` (tylko ślepy finder nad planszą z `Version()`; inaczej żadnych
  pól). `Key(m bodies.Member, o *order.MoveOrder) (fieldAsked, bool)` (fałsz gdy `Group == 0`,
  `At` zerowe albo poza siatką). `Sweep()` raz na Update na gorutynie wołającej: zmiana
  `board.Version()` zwalnia wszystkie wpisy, stempel `used` rośnie. `Expect(q, seed)`: znajdź
  albo załóż wpis (LRU wolnych; brak wolnego → bez wpisu: A* dla tego klucza w tym Update),
  dopisz ziarno, stan `expected`. `Take(q) (*pathfind.Field, build bool)`: `done` → (f,
  false); `building` → czekaj na `worked` → (f, false); `expected` → `building`, (f, true);
  brak → (nil, false). `Give(f)`: `done`, `Broadcast`. **Nic nie zwalnia ani nie przelicza
  wpisu w trakcie paczki** (marsze czytają tablice w miejscu). `Finder.walkField(f, m, from,
  o) (order.Path, bool)` w `path_finder.go` jak `FindPath`.
- **`bodies.Member.Field *pathfind.Field`** („the field of its group's goal this ask, nil for
  none"). `avoiding.find` (`keeping.go`): przed odpowiedziami pamiętanymi, gdy `!around &&
  m.Field != nil`: `walkField`; nieudany marsz spada do dzisiejszego A*. `FindAround` (należne
  `Avoid`) nietknięte: pole jest ślepe na nie. `Answers` nie pamięta marszów.
- **Koordynator** (`routing/coordinator.go`): `fields spacing.Fields` związane w `Router` z
  finderem pierwszego routera; `parties map[uint32]uint16` czyszczone co Update; przebieg 1
  liczy `parties[o.Group]++` dla `Group != 0`; gdy jakaś prośba kwalifikuje się (`parties[g] >=
  fieldParty`): `Sweep()`, potem przebieg 2 po samych prośbach (`Expect(key, at)`); w `route(w,
  i)` po zbudowaniu `m`: `Key` → `Take` → gdy `build`: `FillField` finderem routera (ziarna
  wpisu) i `Give`; `m.Field = f`. Sondy testów: `Coordinator.Fills()`, `Searches()`.
- **Spadki** do dzisiejszego kodu: `Group == 0`/`At` zerowe (ustępowanie, patrzenie, rozkazy
  własne, benchmarki bez `At`); partia < 4; `Avoids > 0` należne → `FindAround` jak dziś;
  plansza > `fieldCells`; komórka nieosiągnięta (członek poza regionem po budowie); ostatni
  odcinek nieznaleziony; więcej kluczy niż `keptFields` w Update; finder widzący innych. Leg
  aktywny przy prośbie nie zachodzi (`traffic` pyta tylko bez lega).
- **Zapis**: nic nowego (klucz z `At`, `Level`, `Group`, domeny i ciała; po wczytaniu pierwsza
  prośba buduje pole; liczność partii z żyjących rozkazów).

## Pliki

Wolno: `plugins/navigation/internal/pathfind/{finder.go,field.go (nowy),field_test.go (nowy),
path_finder_bench_test.go,helpers_test.go}`, `internal/spacing/{fields.go (nowy),keeping.go
(avoiding.find, CellKeeping.Orders),path_finder.go}`, `internal/bodies/member.go`,
`internal/routing/{coordinator.go,system.go}`, testy pakietu `navigation` (nowy plik
`group_test.go`, `routing_bench_test.go`), `doc.go` pakietów pathfind, spacing, routing (po
zdaniu). Szwy nie do ruszenia: `traffic`, keepingi poza `avoiding.find` i `Orders`, `Touch`,
reguły, `order.Path` (64 kroki), astar (jego API bez zmian), board, dema. `doc/`, skille,
`BENCHMARKS.md`, `CHANGELOG.md`, `go.mod`: integratora.

## Kroki

1. pathfind: wyłuskanie `step` i `findFrom` (golden `TestRoutes_AsPinned` w pakiecie
   `navigation` musi zostać bez zmian; sprawdzić po tym kroku), potem `Field`, kopiec,
   `FillField`, `Walk`, `Fills()`; testy: `TestField_EveryCellCostsWhatASearchFromItCosts`
   (24×24: skały, las, piasek, dwie drogi z linkami; `FillField` z ziarnami = każda dopuszczona
   komórka, margines 1; dla każdej komórki `Find(c → goal)` i suma `Price` wzdłuż trasy ==
   `Cost(c)` w 1e-4 względnie, albo obie nieosiągalne; to samo dla `Walk`),
   `…OverARelief` (wycena `rolling` z `path_finder_bench_test.go`: Tobler asymetryczny łapie
   lustro odwrócone; także na hexie), `TestField_AWalkArrivesOnTheGoalsLevel` (dolina z
   pokładem z `levels_test.go`: `Either/Ground/Deck`, bity `Decks` jak `Find`),
   `…ACellTheFieldDidNotReachWalksNowhere`, `…TheLastLegReachesTheMembersOwnCell` (cel 3
   komórki od celu grupy: każdy krok sąsiedni i dopuszczony przez `Steps`),
   `…AFillAllocatesNothingTwice` (`AllocsPerRun` drugiego wypełnienia = 0); rung
   `BenchmarkPathFinder_Field/{map,relief}` (jedno `FillField` 128×128 z dalekim ziarnem).
2. spacing: `At` w `CellKeeping.Orders`; `Fields`, `fieldAsked`, `walkField`; gałąź w
   `avoiding.find`; `Member.Field`.
3. routing: liczenie partii, `Sweep/Expect/Take/Give`, `m.Field`, `Fills()/Searches()`.
4. Testy pakietu `navigation` (`group_test.go`; rig: `newRoadWorld` z `rw.nav.moves.Add` jednego
   `MoveTo` dla zaznaczonych, albo rig routingu piszący rozkazy z `Group` i `At` jak
   `routing_bench_test.go`): `TestGroup_OneFieldServesEveryMembersRoute` (8 członków, jeden
   MoveTo: `Fills == 1`, każdy rozkaz ma `Path` ze swojej komórki, `Searches <= 8`: tylko
   ostatnie odcinki), `TestGroup_ABumpRefillsTheRouteWithoutASearch` (`Path{}` + `AskRoute`
   jednego członka, którego cel to komórka celu: `Fills` i `Searches` bez zmian),
   `TestGroup_AChangedBoardRebuildsTheFieldOnce` (ściana w poprzek tras: jedno wypełnienie
   więcej), `TestGroup_FewerThanAPartyKeepTheSearch` (3 członków: `Fills == 0`),
   `TestGroup_AMemberOffTheFieldSearches`, trasy identyczne na puli i bez niej (jak
   `TestCoordinator_AsksOnAPoolGetTheRoutesOfOneGoroutine`), `TestRoutes_AsPinned` bez zmian.
   `Benchmark_Routing_Group` w `routing_bench_test.go`: 32 jednostki `Routing_Routes` do
   JEDNEGO celu jako grupa (`Target` cel, `At` jego środek, `Group 1`), `workers=1|all`,
   `board.Touch(0)` co iterację (jedna Dijkstra + 32 marsze; na bazie 32 A*). Rung
   `Benchmark_Navigation_Tick/grouped,entities=1000` w `bench/navigation_test.go` (tłum z
   jednym `MoveTo` dla wszystkich zaznaczonych): **tylko dodać**, nie mierzyć.
5. Sekwencja i `-race` na `plugins/navigation/...` (pula gorutyn czyta planszę jak przy
   szukaniu; tablice publikowane pod `mu`); `golines` na dotkniętych plikach.
6. Commity na `feat/field` (`perf(navigation): …` / `feat(navigation): a group's routes read
   off one flow field from its goal`), trailer sesji, bez `Co-Authored-By`.

## Poza zakresem

HPA*; zmiany keepingów, dotyku, reguł; kolumna (etap 4); `Reads` czyszczące `flats` co Update
(zauważone, nie ruszać); pomiary (integrator: `Routing_Path`, `Routing_Routes`,
`Routing_Group`, `PathFinder_Terrain`, `PathFinder_Field`, `Navigation_Tick`); dema i sondy.

## Raport

Commity; co zmieniono poza briefem; czego nie zrobiono; ryzyka (lustro, pamięć na pole: 10
B/komórkę, remisy Dijkstra kontra A* po replanie); proponowane wpisy do `CHANGELOG.md` (temat
v0.9.0), skilli `gram-perf-navigation` (pole obok odpowiedzi pamiętanych; „cele grupy") i
`gram-plugins`; dokładne polecenia benchmarków (`go test ./plugins/navigation -run xxx -bench
'Routing_Path|Routing_Routes|Routing_Group' -benchtime=…`, `go test
./plugins/navigation/internal/pathfind -run xxx -bench 'PathFinder_(Terrain|Field)'`, `go test
./bench -run xxx -bench 'Navigation_Tick' -benchtime=200x -count=2`).
