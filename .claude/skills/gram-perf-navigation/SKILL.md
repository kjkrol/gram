---
name: gram-perf-navigation
description: Wydajność nawigacji grama — A* (astar na tablicach po id, kopiec binarny z sitem z dziurą, jeden słownik; 4-arny z SIMD próbowany i odrzucony, bo zmienił remisy i dynamikę tłumu), trasy do trzymanych buforów i odpowiedzi pamiętane po wersji planszy, cele grupy bez A*, pula routerów, keepingi bez alokacji; wycena kroków w udziałach i jej niezmiennik (udział na płaskim raz na komórkę i wersję planszy, strategia na krok tylko nad reliefem, h = odległość, nic poniżej długości); benchmarki Navigation_Tick, Navigation_Path, Routing_Path, Routing_Routes, PathFinder_Terrain (map, cells, relief); skill astara czytany ścieżką. Czytaj przy zmianie plannera, routingu, keepingów, Mobility, Kind.Pace albo astara, gdy zatłoczony tick albo trasa zwolniły, i przed planowaniem HPA* lub szukania dwukierunkowego.
---

# gram-perf-navigation

Protokół pomiaru i pułapki są w `gram-perf`; tu mapa tego, co w nawigacji zrobiono i na czym to
stoi. Liczby: `BENCHMARKS.md` („Navigation", „The routing alone", profile); historie:
`doc/plans-perf.md` (Etap 2, Fale 3–5, „Udziały tempa"); jak nawigacja działa: `gram-plugins`
(wpis `navigation`). Solver: `/home/karol/CODE/astar/.claude/skills/astar/SKILL.md` (sesja w tym
repo go ładuje; stąd czyta się go ścieżką) i `BENCHMARKS.md` astara.

## Gdzie idzie czas

`Navigation_Tick` (open i crowded, 100 i 1 000 piechurów na planszy 64×64), `Navigation_Path`
(jedna trasa przez 200×200 co tick, od drugiego ticku odpowiedź pamiętana), `Routing_Path` i
`Routing_Routes` (`plugins/navigation`: sam routing, 1 trasa albo 32 naraz na jednym wątku i na
puli), `PathFinder_Terrain` (`pathfind`: jedna trasa przez 128×128 z ćwiercią ścian; `map`
czyta seed, `cells` encje komórek, `relief` wzgórza po ordynale ze strategią na krok). W
zatłoczonym ticku A* to trzy czwarte (solver astara, `leveledFrom`, `price`), pod nim odczyty
komórek siatki (`dxdy`, `NeighborCost`, `Distance`); w otwartym A* to dwie piąte, resztę zjada
ruch jednostek po trasach (`traffic`).

## Co zrobiono i na czym stoi

- **Etap 2 i fale 3–5** (2026-10-20…22): A* do trzymanych buforów (`astar.SolveInto`, słowniki
  slice'owe ze stemplami, `Wipe`), odpowiedzi pamiętane po pytaniu i wersji planszy
  (`keptAnswers`), cele grupy bez A* (`bodyKeeping.place`), odroczenia nieudanych szukań
  (`MoveOrder.Retry`), keepingi bez alokacji, routing na puli. `Navigation_Tick` open 1 000
  9,56 → 6,32 → 4,55 ms, crowded 1 000 306 ms → poniżej 10; `Navigation_Path` 94 → 18 ms,
  14 740 → 19 alokacji; `Routing_Routes` na puli 3,2× szybciej niż na jednym wątku.
  Niezmiennik: co zmienia to, co szukanie czyta, liczy się w `Board.Version` przed następnym
  szukaniem; trasa raz zaplanowana jest śledzona, planowana na nowo tylko gdy pęknie.
- **astar na tablicach** (2026-10-28, astar v1.2.0, `perf/arrays`): węzły w tablicach po
  `int32` bez wskaźników, kopiec binarny kluczy i id pisany ręcznie z sitem z dziurą, jeden
  słownik (zamknięty jako `^id`); ścieżki co do kroku jak v1.1.3. Kroki osobno: układ
  −26…−31%, jeden słownik −10…−14%, sito z dziurą −4…−8%; v1.1.3 → v1.2.0 razem (najniższy z
  sześciu, na zmianę, `-cpu=1`): pełne szukanie −41…−43% (1024×1024 208 → 123 ms, słowniki
  mapowe −43…−45%), krótkie (waga 0,1) −76…−89%, `SolveInto` −40…−43%, Octile −38…−40%
  (1024×1024 413 → 249 ms); `Solve` alokuje raz (ścieżkę) zamiast dwa, `SolveInto` nic.
  **Próbowane i odrzucone**: kopiec 4-arny z wyborem najmniejszego dziecka jedną instrukcją w
  buildzie SIMD dał −10% na Octile astara wobec kopca binarnego (1024×1024 227 → 204 ms), ale
  zmienia, która z kilku najtańszych ścieżek wychodzi, a w `Navigation_Tick/crowded,1000` (ten
  sam gram, tylko astar inny) jednostki przeplanowały 55% więcej: 7,2 → 11,2 ms na tick.
  Reguła „żaden reżim nie traci"
  obejmuje reżimy wołającego. Niezmiennik: sito porównuje dokładnie jak `container/heap`, więc
  remisy, golden astara i `routes.golden` grama zostają; zmiana arności łamie go i przepisuje
  oba goldeny z powodem — i dynamikę tłumów.
- **Wycena kroków w udziałach** (2026-10-28, `feat/pace`, decyzje użytkownika): waga przejścia
  to udział tempa maksymalnego w [0, 1], 0 odmowa (węzeł zamknięty), 1 pełne tempo i nic
  szybszego; `cell.Mobility.Pace(Step)` gry składa, biblioteka zbiera (rodzaj na poziomie kroku,
  rise/run między środkami komórek z `Map.Rise`, 0 dla `Graded`); planner liczy koszt =
  długość / udział, h = odległość w siatce (dopuszczalna, bo żaden krok nie kosztuje mniej niż
  długość; `Least` zniknęło); tempo i jazda czytają tę samą strategię tą samą miarą stoku (do
  sąsiada w kierunku marszu, nie w punkcie), więc co planner dopuścił, jednostka idzie, a
  odcięcie nie zatrzymuje jej w połowie kroku. **Niezmiennik kosztu**: udział gruntu na płaskim
  czytany raz na komórkę i domenę, trzymany między szukaniami po wersji planszy (`flatStep`,
  jak odpowiedzi pamiętane: co zmienia to, co szukanie czyta, liczy się w `Board.Version`), na
  szukanie w `read.rate` mnożony jak dawniej; nad reliefem strategia wołana na każdy
  niestopniowany krok, rodzaj gruntu trzymany po ordynale (`kinds`, alokowane przy pierwszym
  szukaniu ze stokiem); bramka ustępowania nad płaską planszą nie woła strategii
  (`Sloped()`). `PathFinder_Terrain/relief` mierzy koszt strategii na krok; `Finder.Searches()`
  liczy szukania (sonda testów `TestKept_*`: szukanie nie czyta już terenu na drugie pytanie).
  Pierwszy pomiar bez tej pamięci: zatłoczony tick +6% (łańcuch `Flat` na komórkę na szukanie),
  bramka ustępowania z dwoma `PaceOf` na kandydata +15% w open; z pamięcią i skrótem, razem z
  astarem v1.2.0 (baza `main` v0.6.0 z astarem v1.1.3, `BENCHTIME=200x`, najniższy z trzech
  rund):
  `Navigation_Tick` crowded 1 000 8,83 → 7,93 ms (−10%), crowded 100 −11%, open 100 −8%, open
  1 000 3,22 → 3,21 (w szumie: tam rządzi ruch po trasach), `Navigation_Path` −2,5%;
  `Routing_Path` 15,8 → 15,2 ms, `Routing_Routes` 1 wątek 255 → 211 ms (−17%), pula 87,8 → 75,8
  (−14%); `PathFinder_Terrain` map 6,88 → 5,50 ms, cells 6,78 → 5,27 (−20…−22%, `-cpu=1` tak
  samo); nowy szczebel `relief` ok. 9 ms: strategia (`PaceOf`, `exp`) na każdy krok to niemal
  drugie tyle co samo szukanie. Tempo (`Board_Altitude`, `Gravity_Standing`): +9…+10% przy
  1 000 piechurów na reliefie, ok. 40 ns na jednostkę na krok (`Map.Rise` do sąsiada w kierunku
  marszu i `exp`), w szumie przy 100; cena strategii, przyjęta.
- **Ustępowanie**: `YieldShare` (1/3) jako stosunek udziału kroku do udziału na płaskim, nie
  bezwzględny próg: płaski las wciąż wchodzony, klif nie; niezależne od strategii.

## Co dalej (poza etapem, osobne planowanie)

- **HPA***: graf regionów nad komórkami; sektory świata mogą dać podział na siatce kwadratowej
  (na heksach klastry z samej siatki); nowe są wejścia na granicach, koszty między wejściami
  per domena i klasa jednostki, unieważnianie po `CellVersion`; trasy ograniczenie
  nieoptymalne.
- **Szukanie dwukierunkowe**: cena zależna od kierunku (rodzaj docelowy, znak stoku) wymaga
  wyceny wstecznej w lustrze; warunek stopu to najlepsza znaleziona trasa ≤ mniejsze z
  najmniejszych F frontów; zysk co najwyżej ~2× przy drogim gruncie, żaden przy ciasnej
  heurystyce; `NearestFree`/`FindAround` nie skorzystają. Najpierw licznik rozwiniętych węzłów
  na długiej trasie przez Maderę.
- **`Best(d)`**: planszy bez dróg, gdzie najlepszy kafel ma udział 0,4, planner mógłby liczyć
  h = 2,5 × odległość; na Maderze drogi dają 1, więc nic do wzięcia dziś.

## Próbowano i porzucono

Pasy w nawigacji (jednostki uczą się uderzając; reguła domu), kopiec 4-arny skalarny bez
kernela (+1…+2%), arytmetyka następników i porównania słownikowe w SIMD (callback heurystyki na
następnik, brak gather; `BENCHMARKS.md` astara, „Tried and dropped"). Nie proponować bez nowego
powodu.
