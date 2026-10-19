# Plany: alokacje per tick i per klatka

Sześć zadań, każde dla osobnej sesji (Opus 5.5) w osobnym worktree. Plik roboczy; nie jest
częścią dokumentacji biblioteki. Odwołania do linii sprawdzone na `51da9307` (the light demo);
plugin `light` (`19174ed0`) wszedł po pierwszej wersji planów i jest uwzględniony w zadaniach
5 i 6.

## Wspólne dla każdego zadania

- Przeczytaj `CLAUDE.md` i skille warstwy, w której pracujesz (nazwane w każdym zadaniu).
- Nie commituj. Nie dotykaj plików dem pod `examples/`, nawet żeby coś sprawdzić: kopia
  (`gram-probe`, § Checking a change).
- Komentarze tylko tam, gdzie kod sam nie powie (`CLAUDE.md`, "Docs, benchmarks and commits");
  linie do 100 kolumn; jedna wartość w linii.
- Standardowa weryfikacja: `go build ./... && go vet ./... && gofmt -l . && go test ./...`.
- Zmiana wydajnościowa jest mierzona przed i po, naprzemiennie, na kopii drzewa sprzed zmiany i
  na drzewie roboczym (`BENCHMARKS.md`, "metoda"; maszyna dryfuje ~10% między biegami).
  `go test ./bench -run xxx -bench <nazwa> -benchmem` dla benchmarku; `GRAM_FPS_LOG=1` i
  `GODEBUG=gctrace=1` dla dema.
- Raport na końcu: co zmierzono przed, co po, w jakim reżimie; co zmieniono; czego nie
  zrobiono i dlaczego.

Kolejność i zależności: zadania 1, 2, 4, 5, 6 są niezależne. Zadanie 3 zmienia te same pliki
nawigacji co zadanie 2 (`keeping.go`, `spots.go`, `path_finder.go`, `internal/pathfind/finder.go`),
więc rusza po tym, jak 2 wejdzie do `main`, albo rebase'uje się na nie. Zadanie 5 dotyka
`module.go` w wielu pluginach; to inne pliki niż w 1-4, konflikty małe.

---

## Zadanie 1. Overlay wizji liczony co klatkę

Zrobione w etapie 1 planu `plans-perf.md` (2026-10-19, `83d06226`): `Field.Version`/`Changes`.

**Skille:** `gram-plugin`, `gram-plugins` (vision, board), `gram-render`.

**Problem.** `plugins/vision/views.go`, `views.copy` (ok. linia 250): cache próbki podłoża
sprawdza `at := [2]uint64{versionOf(ground), versionOf(cover)}` i wymaga `at[1] != 0`.
`versionOf` (ok. linia 331) zwraca 0, gdy wartość nie ma `Version() uint64`. Cover daje
`board.Plugin.Cover()` (`plugins/board/plugin.go:481`), czyli `*field.Field`
(`plugins/board/internal/field/field.go`), który `Version()` nie ma. Skutek: co klatkę z
obserwatorem na ekranie idzie próbkowanie `nx*ny` wysokości, `Walk` po każdym wierszu, dwa
`WritePixels`, w pięciu demach: board/flat, board/topography, navigation/square,
navigation/hex, rules/dialog.

**Co wiadomo.** `Field.Ready()` (field.go, ok. linia 77) już porównuje
`f.cells.Changes()` i `f.cells.Version()`; `terrain.Cells` ma obie metody
(`plugins/board/internal/terrain/cells.go:346-349`). `board.Board` też je wystawia
(`plugins/board/board.go:162-165`). `versionOf` łączy `Version()<<1|1` z `Changes()<<33`.

**Kroki.**
1. Dodać `Field.Version() uint64` i `Field.Changes() uint64` przepuszczające `cells`. Sprawdzić,
   czy cover zależy od czegoś poza komórkami (veil kindu; kindy nie zmieniają się w biegu, ale
   upewnić się, że np. mosty albo pokrycie `Covering` nie wpływają na `Walk` bez zmiany wersji
   komórek). Jeśli wpływają, wersja musi to obejmować.
2. Sprawdzić stronę `ground`: `board.Plugin.Heights()` → `mapping.Heights()`. Dla mapy płaskiej
   nil (wersja 1, ok). Dla reliefu `topography/internal/relief/relief.go:89` ma `Version()`.
   Upewnić się, że to, co trafia do `views.copy`, naprawdę jest tym typem, a nie opakowaniem
   bez `Version()` (sprawdzić `WithHeights`/`mapRef` w `plugins/board/map.go`).
3. Test w `plugins/vision/views_internal_test.go`: cover z `Version()`, dwie klatki, druga nie
   przelicza (licznik `WritePixels` albo sprawdzenie, że `v.groundAt` się nie zmienia i bufor
   `v.pix` nie jest zapisywany ponownie). Test, że zmiana wersji przelicza. Test przez `Field`
   (pakiet `field`): wersja rośnie po zmianie komórki.
4. Pomiar: kopia dema board/topography z obserwatorem na ekranie (`gram-probe`), `GRAM_FPS_LOG=1`
   przed i po, naprzemiennie; zapisać, ile ms z klatki ubyło.

**Poza zakresem.** Reszta alokacji wizji (closures w `copy`: `x`, `y`, `enc` zostają, bo po
poprawce wykonują się tylko przy zmianie).

---

## Zadanie 2. `Grid.Neighbors` bez alokacji

**Skille:** `gram-plugin`, `gram-plugins` (board, navigation).

**Problem.** `plugins/board/internal/grids/square.go:85` i `hex.go:67`: `Neighbors` robi
`make([]cell.ID, 0, 8)` (6 dla hex) przy każdym wywołaniu; wywołanie idzie przez interfejs
`grid.Grid` (`plugins/board/grid/grid.go:11`), więc slice zawsze ucieka. Woła je m.in. rozwijanie
węzła A* (`plugins/navigation/internal/pathfind/finder.go:235` i `:269`), czyli alokacja na
każdy węzeł każdej ścieżki.

**Wszystkie wywołania (14, poza testami):**
`plugins/board/internal/moments/around.go:113`, `plugins/board/network/route.go:35`,
`plugins/board/water/water.go:110,180`, `plugins/navigation/spots.go:255`,
`plugins/navigation/path_finder.go:76`, `plugins/navigation/keeping.go:256`,
`plugins/navigation/internal/pathfind/finder.go:235,269`,
`plugins/atmosphere/weathering/weathering.go:265,277,311`,
`plugins/topography/internal/relief/shaping.go:143`, `cmd/gramap/build.go:287`.

**Co wiadomo.** `grid.Grid` ma już `Directions() int` i `Toward` (używane w
`plugins/board/grid/link.go`), więc iteracja po kierunkach bez slice'a jest możliwa od ręki.

**Kroki.**
1. Zdecydować kształt: (a) `NeighborsInto(c cell.ID, dst []cell.ID) []cell.ID` obok
   `Neighbors` w interfejsie, albo (b) iteracja `for i := range g.Directions() { n, ok :=
   g.Toward(c, i) }` w gorących miejscach i `Neighbors` zostawiony tam, gdzie wywołanie jest
   rzadkie (gramap, shaping, weathering). Sprawdzić, czy `Toward` przez interfejs nie kosztuje
   więcej niż jedna alokacja na wywołanie (benchmark w `plugins/board/internal/grids`).
   Preferowane (b) w A* i `aside`, (a) jeśli jakieś miejsce potrzebuje listy (sort w
   `keeping.go:aside`, `slices.Contains` w `path_finder.go:76`).
2. Przerobić gorące miejsca: `finder.go:235,269`, `keeping.go:256`, `spots.go:255`,
   `path_finder.go:76` (tu zamiast listy wystarczy `grid.Way(from, c)` albo pętla po
   kierunkach). Pozostałe wywołania zamienić tylko, jeśli zmieniono interfejs.
3. Test: `go test ./plugins/board/... ./plugins/navigation/...`; testy gridów dla
   `NeighborsInto`, jeśli powstał (zawijanie, brzegi, to samo co `Neighbors`).
4. Pomiar: benchmark A* (jeśli jest w `bench/`; jeśli nie, dodać prosty `Benchmark_Navigation_Path`
   na siatce 200x200 z przeszkodami, `ReportAllocs`), przed i po. Demo navigation/square z
   `GODEBUG=gctrace=1` w kopii, jednostki wysłane wokół muru.

**Poza zakresem.** `aside`, closures i back-off (zadanie 3).

---

## Zadanie 3. Nawigacja: zmierzyć, potem odchudzić

**Skille:** `gram-plugin`, `gram-plugins` (navigation), `ceremony` (co jest regułą, co
przejściem pluginu), `gram-probe`. Rusza po zadaniu 2.

**Problem.** Per ciało per tick: closure w `plugins/navigation/bodies.go:69` (`CellsUnder`),
`near()` i closures w `body_keeping.go:323,371,401,420`. `Leg.cells()`/`held()`
(`navigation_system.go:157-170`) alokują na każde wywołanie. `cellKeeping.aside`
(`keeping.go:247`) buduje `var out []scored` i sortuje z closure, a wołane jest przez
`stepAside` dla jednostki zablokowanej co tick. Zablokowana jednostka co `bumpInterval`
(500 ms, `navigation_system.go:236`) powtarza pełne A*, które znów się nie udaje.

**Kroki.**
1. **Najpierw benchmark**, bo go nie ma: `bench/navigation_test.go`,
   `Benchmark_Navigation_Tick` w dwóch reżimach: (a) N jednostek w drodze bez przeszkód,
   (b) N jednostek stłoczonych przy murze z celem za nim (ciągłe `stepAside` i nieudane A*).
   N = 100, 1000. `ReportAllocs`. Wzór na instalację headless: `bench/headless_test.go`.
   Zapisać wynik jako punkt wyjścia.
2. Mechaniczne, bez zmiany zachowania: stan closures do pól i jeden method value bindowany w
   konstruktorze (tak jak collision); `Leg.cells()`/`held()` zwracają `[4]cell.ID`/`[4]spot` z
   licznikiem; `aside` używa scratcha `[8]scored` w `cellKeeping`. Benchmark po każdym kroku.
3. **Decyzja o back-off dopiero po pomiarze z 1(b).** Jeśli nieudane A* dominuje: zapisać przy
   jednostce moment ostatniej porażki i rosnąć z odstępem (500 ms, 1 s, 2 s, do pułapu), zerując
   go, gdy cel albo zajętość komórek na drodze się zmieni (`Changes()` zajętości, jeśli jest; jeśli
   nie ma, to jest brakujący licznik, nie heurystyka). To zmiana zachowania: probe na kopii
   navigation/spawn (tłum pod murem, potem mur zwolniony, jednostki muszą ruszyć bez
   zauważalnej zwłoki) i navigation/square (wagon za szeroki na most nie może utknąć inaczej
   niż dziś).
4. Wyniki benchmarku do `BENCHMARKS.md` (sekcja nawigacji, z metodą).

**Poza zakresem.** `Neighbors` (zadanie 2). Jakakolwiek zmiana, jak jednostki omijają się
nawzajem; zasada "nigdy nie widzi innych z wyprzedzeniem" zostaje.

---

## Zadanie 4. `gpu.Present` i `SetBindGroup` bez alokacji na klatkę

Zrobione w etapie 1 (2026-10-19, `1a2b3c89`, `ec0a8228`): arena offsetów resetowana po
`Submit` (GLES trzyma wycinek), deskryptory i bufory komend jako pola, `presented` jako własna
kopia.

**Skille:** `gram-render`.

**Problem.** `render/gpu/ops.go`: `Present` (linia 132) robi `uniforms := make([]byte,
uniformBlock)` (171), `&wgpu.RenderPassDescriptor{...ColorAttachments: []...{{` (187),
`[]uint32{0, 0}` (194). `render/gpu/device.go`: `trySubmit` (225) buduje
`[]*wgpu.CommandBuffer` (247-249); `begin` (281) nowy deskryptor przebiegu (286). Przy każdym
rysowaniu `[]uint32{uint32(dOff), uint32(uOff)}` w `kept.go:93`, `mesh.go:267,277,317`.
`program.go:403` robi `append(place[:], tint[:]...)` w pętli. Łącznie rzędu 5 KB na klatkę plus
jeden mały slice na draw.

**Kroki.**
1. **Zanim cokolwiek zostanie zreużyte: sprawdzić w gogpu/wgpu (`go doc`, źródło w module
   cache), czy `BeginRenderPass`, `SetBindGroup` i `Submit` kopiują przekazane slice'y, czy
   trzymają referencje po powrocie.** Reużycie między klatkami jest bezpieczne tylko, jeśli
   kopiują albo kończą z nimi przed powrotem. Zapisać ustalenie w raporcie z linią źródła.
2. Na `device`: stałe pola `offsets [2]uint32`, `cmds [2]*wgpu.CommandBuffer`, deskryptory
   przebiegów i ich tablice załączników, bufor uniformów `Present`; wypełniane przed użyciem.
   Każde `[]uint32{dOff, uOff}` → `d.offsets[:]` po przypisaniu.
3. Test: `go test ./render/...` (headless; `gpu.Headless`, skip bez GPU).
   `Benchmark_World_Draw` (`bench/world_test.go:66`) z `-benchmem` przed i po; cel: zero alokacji
   w `Present` i `trySubmit`.
4. Kontrola obrazu: `GRAM_SHOT` z kopii board/topography i motion/collision przed i po; obrazy
   identyczne (pomijając panel telemetrii i pogodę).

**Poza zakresem.** Uniformy w `topography/internal/terrain/terrain.go` (zadanie 5). Zmiany w
`render.Composer`.

---

## Zadanie 5. Drobne alokacje per klatka i per tick

Zrobione w etapie 1 (2026-10-19): 1 `e5581a5a`, 2 `f14e7759`, 3 `8e1fe910`, 4 `28390e26`,
5 `0a319825`, 7 i 8 `f1cf2e7c`; 6 zostaje (goke nie ma zdjęcia kilku komponentów naraz).

**Skille:** `gram-plugin`, `gram-render` (dla fontu), `gram-plugins` (telemetry, world, bullet).

Jedno przejście po małych, niezależnych od siebie miejscach. Każde z osobnym pomiarem tylko tam,
gdzie jest benchmark; reszta sprawdzana `-gcflags=-m` i testami pakietu.

1. **`clock.Simulate` wiązane raz.** `clock/clock.go:101` dopisuje blok do slice'a, więc
   closure ucieka. Wywołania z nową closure co tick: `plugins/{board,navigation,topography,
   vision,collision,bullet,bridges,driving,gravity,dialog}/module.go`,
   `plugins/atmosphere/plugin.go:281`; `plugins/world/module.go:130` przekazuje method value
   `w.simulate`. Wzór: `plugins/motion/module.go:59` (`m.step` związany w konstruktorze).
   Po tym `Benchmark_World_Tick` powinien dawać 0 allocs/op (dziś 1 alloc, ok. 15 B na tick w
   stanie ustalonym); poprawić zdanie o "0 allocs" w `BENCHMARKS.md`, jeśli po zmianie
   nadal nie jest prawdziwe.
2. **Tekst.** `render/font.go:100` i `:157`: `strings.Split(s, "\n")` na każdą etykietę co
   klatkę; `pieces` rośnie (166). Iterować po `strings.Cut`/indeksie `\n` bez slice'a; `pieces`
   jako scratch na renderze fontu. Sprawdzić `DrawText`/`DrawPieces` w `render/gpu/ops.go:90`
   (`make` na wierzchołki i uniformy przy każdym wywołaniu, 95-106).
3. **Telemetria.** `plugins/telemetry/plugin.go:40,121` `fmt.Sprintf` co klatkę,
   `scene.go:70` `strings.Builder` co klatkę. Odświeżać tekst nie częściej niż 4 razy na
   sekundę i składać przez `strconv.Append*` do bufora trzymanego na scenie.
4. **Wejście.** `internal/engine/desktop_adapter.go:46` `a.pending = nil` wyrzuca bufor co
   klatkę; dwa bufory zamieniane miejscami (`a.pending, a.taken = a.taken[:0], a.pending`).
   Upewnić się, że nikt nie trzyma poprzedniego slice'a po pobraniu.
5. **Spawn.** `plugins/world/spawn_system.go:107` nowy `spawnGroup{rows: []any{row}}` na
   każde przyjęcie; grupy reużywane (`groups[:0]` i wewnętrzne slice'y zachowane).
6. **Pociski.** `plugins/bullet/flight_system.go:124-125` zdejmuje `Sweep` i `Collider`
   dwoma `RemoveCompOne`, czyli dwa przejścia archetypu na pocisk. Sprawdzić w goke (skill
   `goke`, `scripts/api.sh`), czy bufor komend ma zdjęcie kilku komponentów naraz; jeśli ma,
   użyć; jeśli nie, zostawić i odnotować w raporcie.
7. **Uniformy boxowane.** Po wejściu pluginu `light` topography już tego nie robi
   (`terrain.go:125` trzyma `uniforms`/`boxed` raz); boxowanie zostało w
   `plugins/light/internal/bake/bake.go:211,212,222` (`[]float32{...}` do `map[string]any`
   przy każdym wypieku cienia; wypiek idzie pasami, gdy słońce obróci się o `shadeLeap`, coś
   się ruszy albo wersja reliefu wzrośnie, więc w demie ze słońcem krążącym to jest co klatkę).
   Wzór: `plugins/light/internal/cast/cast.go:117` (`set`: slice raz, potem `copy`) i
   `render/sprites.go`. Przejrzeć `-gcflags=-m` także `grid_lines`, chmury, heksy, atmosferę;
   Opus wskazał je, grep tego nie potwierdził.
8. **Nazwy uniformów lamp co klatkę.** `plugins/light/plugin.go:307-308`:
   `fmt.Sprintf("Lamp%dAt", k)` i `"Lamp%dColor"` na każdą lampę w kadrze w każdej klatce.
   Nazwy są stałe dla `k < MaxLamps`: tablica `[MaxLamps][2]string` zbudowana raz tam, gdzie
   deklaracje uniformów już je formatują (`plugin.go:31-32`). `plugin.go:575` (`Sprintf` linii
   telemetrii) wchodzi pod punkt 3.

**Weryfikacja.** Testy pakietów; `Benchmark_World_Tick` dla 1; `GRAM_SHOT` z kopii
board/topography (ta sama klatka przed i po, bez panelu) dla 2, 3, 7 i z kopii look/light
(lampy, cień wieży; `P` zatrzymuje światło, żeby klatki były porównywalne) dla 7 i 8;
motion/bullet w kopii pod `timeout` dla 6. Raport z listą: zrobione / pominięte i dlaczego.

---

## Zadanie 6. Pomiar: ile GC naprawdę kosztuje, i brakujące benchmarki

**Skille:** `gram-probe`, `goke` (dla benchmarków headless), `gram-game` (co dema robią).

To zadanie nic nie optymalizuje. Daje liczby, bez których zadania 2-5 są sprzątaniem na
wiarę.

**Kroki.**
1. **gctrace na demach**, w kopiach, po 60 s każde, `GODEBUG=gctrace=1 GRAM_FPS_LOG=1`:
   board/topography (kamera na reliefie, obserwator w kadrze, pogoda), look/light (słońce
   krążące, czyli wypiek cienia pasami; lampy w nocy, Shift+L), navigation/square
   (jednostki wysłane wokół muru, potem stłoczone przy nim), motion/collision, motion/expanse,
   motion/bullet (seria strzałów i granatów), look/animation (dużo etykiet? jeśli nie,
   rules/dialog dla tekstu). Z logu: odstęp między cyklami GC, rozmiar sterty, czas CPU
   markowania; z FPS log: ile ms klatki idzie gdzie. Tabela: demo, GC/s, MB sterty, ms/klatkę.
2. **Brakujące benchmarki w `bench/`** (pakiet `bench_test`, `ReportAllocs`, wzór instalacji
   `bench/headless_test.go`): pociski (N w locie, lądowania), tekst i telemetria (klatka panelu
   z 20 liniami), pełna klatka z UI (scena z kilkoma elementami ui nad światem).
   `Benchmark_Navigation_Tick` robi zadanie 3; nie dublować.
3. **`Benchmark_Climate_Step`** istnieje (`bench/climate_test.go`), ale nie ma wyniku w
   `BENCHMARKS.md`: uruchomić 5 razy, wpisać z metodą jak pozostałe.
4. **Pokrętła runtime, tylko pomiar**: to samo demo z `GOGC=200` i `GOGC=400` (zmienna
   środowiska, bez zmian w kodzie) obok domyślnego; tabela jak w 1. Nie wprowadzać
   `SetMemoryLimit` ani `SetGCPercent` do kodu; raport ma powiedzieć, czy jest po co.

**Wyjście.** Sekcja w `BENCHMARKS.md` (nowe benchmarki z wynikami i metodą) i raport z tabelą z
1 i 4 z wnioskiem: które z zadań 2-5 mają pokrycie w liczbach, a które nie.
