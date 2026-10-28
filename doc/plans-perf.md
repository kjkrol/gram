# Plan: wydajność ticku i klatki, skonsolidowany

Zastępuje kolejność z `plans-gc.md`; zadania stamtąd wchodzą tu jako etapy albo ich części i są
cytowane po numerze (`plans-gc Z4`), bo ich kroki z numerami linii nadal obowiązują. Plik
roboczy, nie część dokumentacji biblioteki. Odwołania do linii sprawdzone na gram `51da9307`,
goke `a6b663b` (v3.5.0), aabbworld `4889c6e` (v1.12.0).

Źródła: `doc/benchmarks-history.md`, profil CPU `Benchmark_World_Tick/entities=5000` z
2026-10-18 (poniżej), przegląd systemów i renderera z tej samej sesji, `plans-gc.md`.

## Co wiadomo na starcie

Tick świata przy 5 000 encji, 259 µs na maszynie pomiarowej, 1 alloc, bez śladu GC w profilu:

| Gdzie | Udział |
|:--|--:|
| `motion.moveSystem.rebuild`: kopia Base do `aabbworld.Item`, `Grid.Rebuild`, `remember` | 35% |
| `motion.clampStep`, w tym `math.Hypot` (`math.archHypot` 17% samo) | 26% |
| `aabbworld.Space.Move`: `Surface.Translate`, `move`, `fold`, `wrap` | 25% |
| iteracja goke, `Velocity.Delta`, `Position.MaxStep` | ok. 10% |

`Grid.index()` w tym benchmarku nie biegnie ani razu, bo nikt nie pyta przestrzeni; w grze
odpala je odświeżenie widoku raz na tick, ok. 20 µs (`doc/benchmarks-history.md`, "Drawing").
Wniosek: tick nie jest problemem GC, tylko trzech miejsc CPU. Klatka ma inne centra kosztu
(`doc/benchmarks-history.md`: wyspa 1,8-4 ms, cienie i brzegi liczone na nowo 4-5 ms, encje
0,4-0,65 ms) i tam liczby per miejsce dopiero trzeba zebrać (etap 0).

Decyzje projektowe przyjęte z góry, żeby nie wracać do nich w etapach:
- `Position` zostaje autorytatywny na CPU. Jedenaście systemów czyta go losowo do struktur CPU
  (sektory, zajętość, nawigacja, relief, wzrok, kolizje, renderer), a `plane.AABB` to osiem
  float64, których WGSL nie ma. GPU dostaje co najwyżej lustro float32.
- Systemy przeplatane regułami ceremony (`RunWhere` w środku chunka) nie idą na GPU.
- Nie ma GPU dla samej arytmetyki ruchu: to mikrosekundy, dispatch kosztuje tyle samo.

## Wspólne dla każdego etapu

Obowiązuje sekcja "Wspólne dla każdego zadania" z `plans-gc.md` (skille warstwy, bez commitów,
dema tylko w kopii, pomiar przed i po naprzemiennie, raport). Do tego:
- Profil CPU obok benchmarku: `go test ./bench -run xxx -bench <nazwa> -cpuprofile <plik>`,
  `go tool pprof -top`. Zmiana w gorącej pętli bez profilu przed i po nie jest zmierzona.
- Moduły: `goke` i `ceremony` są w `/home/karol/CODE/go.work`, `aabbworld` nie jest. Etap
  dotykający aabbworld dopisuje `./aabbworld` do `go.work` na czas pracy i mówi to w raporcie;
  tag wydania i podbicie wersji w `gram/go.mod` robi właściciel po przeglądzie, sesja tego nie
  robi. To samo dla goke: zmiana w rdzeniu kończy się raportem i propozycją wpisu do
  `CHANGELOG.md`, nie tagiem.
- Etapy oznaczone "równolegle" mogą iść w osobnych sesjach i worktree naraz; pozostałe czekają
  na to, co wymienione w "Po".

Kolejność etapów: 0 → 1 (sześć części równolegle) → 2 i 3 (równolegle) → 4 → 5 → 6.
Etapy 5 i 6 mają bramkę: ruszają tylko, gdy liczby z etapu 0 i z etapów wcześniejszych je
uzasadnią.

---

## Etap 0. Pomiar: liczby, bez których reszta jest na wiarę

**Skille:** `gram-probe`, `goke`, `gram-game`. To `plans-gc Z6` poszerzone o profile CPU.
Nic nie optymalizuje.

**Kroki.**
1. `plans-gc Z6` w całości: gctrace i `GRAM_FPS_LOG` na demach (60 s, w kopiach), brakujące
   benchmarki (pociski, tekst i telemetria, klatka z UI), wynik `Benchmark_Climate_Step`,
   `GOGC=200/400` jako sam pomiar. `Benchmark_Navigation_Tick` z `plans-gc Z3` krok 1 powstaje
   tutaj, w obu reżimach (w drodze; stłoczeni pod murem), żeby etap 2 zaczynał od liczby.
2. Profile CPU, każdy jako tabela "funkcja, udział" w raporcie, po pięć najcięższych liści:
   `Benchmark_World_Tick/5000` (punkt odniesienia dla etapu 1), `Benchmark_Collision_Tick`
   (8 388 pudełek: ile w `Grid.Pairs` i `listItems`, ile w solverze, ile w `rebuild`
   i `moved`/`seek`), `Benchmark_Vision_Relief` (ile w raycaście, ile w kopiowaniu wyników),
   `Benchmark_World_Draw` przy kamerze na całości (ile w `rules.Run`, `view.Level`, fog,
   a ile w `lay`/instancje `Sprites`; to bramka etapu 5), `Benchmark_Board_Island`
   (ile w cieniach i brzegach "kept", a ile w składaniu kafli).
3. Z `Benchmark_World_Draw` osobno: ile encji z 5 000 zmienia w typowym ticku `Base`
   (porusza się), a ile stoi. Do policzenia w benchmarku: porównanie `Pos` przed i po ticku.
   To liczba, od której zależy sens śledzenia zmian per chunk (etap 4 i 5).

**Wyjście.** Sekcja w `BENCHMARKS.md` z nowymi benchmarkami i metodą; raport z tabelami z 1 i 2
oraz liczbą z 3; na końcu lista: które części etapu 1 i które etapy 2-6 mają pokrycie w
liczbach.

---

## Etap 1. Poprawki na godzinę, równolegle, bez zmian architektury

Sześć niezależnych części, każda w osobnej sesji. Żadna nie zmienia kontraktu między
pluginami ani API biblioteki.

### 1A. Overlay wizji liczony co klatkę

`plans-gc Z1` bez zmian: `Field.Version()`/`Changes()`, sprawdzenie strony `ground`, testy,
pomiar `GRAM_FPS_LOG` na kopii board/topography z obserwatorem w kadrze. Największy
pojedynczy zysk per klatka w pięciu demach i w istocie naprawa błędu, więc idzie pierwsze.

### 1B. `clampStep` bez `Hypot` w typowym kroku

**Skille:** `gram-plugin`, `gram-plugins` (motion).

**Problem.** `plugins/motion/move_system.go`, `clampStep` (koniec pliku): `math.Hypot` na
każdą poruszającą się encję co tick, 26% ticku świata, choć krok prawie nigdy nie przekracza
limitu. `Hypot` jest wolne (obsługa nadmiaru i NaN), a tu potrzebne jest tylko porównanie.

**Kroki.**
1. Porównać kwadraty: `sq := x*x + y*y; if sq <= max*max || sq == 0 { return step }`; `Sqrt`
   dopiero przy skalowaniu, i to raz. Zachowanie identyczne dla skończonych wartości; test
   z krokiem równym limitowi, większym i zerowym.
2. Przy okazji sprawdzić `-gcflags=-m`, czy `Velocity.Delta` i `Position.MaxStep` inlinują się
   w `Update` (profil pokazuje je jako osobne ramki inline, ok. 4%); jeśli nie, dlaczego.
3. Pomiar: `Benchmark_World_Tick` z profilem przed i po. Oczekiwanie: `archHypot` znika
   z listy, tick spada o rząd 20%.

### 1C. Szybka ścieżka w `Surface.Translate` (aabbworld)

**Skille:** `goke` (metoda pomiaru), README i `BENCHMARKS.md` aabbworld. Praca w
`/home/karol/CODE/aabbworld`, `go.work` tymczasowo (patrz "Wspólne").

**Problem.** `internal/plane/surface.go:58` `Translate` woła per oś `move` (:73), a ta
`fold` (:85) i `wrap` (`vector_math.go:30`). Cztery ramki na oś na każdą poruszającą się
encję: 25% ticku świata w gram. `wrap` ma już szybki powrót dla wartości w zakresie, ale
dociera się do niego przez trzy wywołania.

**Kroki.**
1. W `Translate`, przed `move`: jeśli `lo >= 0 && lo+length <= world`, to nowy `lo`, `hi`,
   `overhang = 0` bez wywołań, dla każdego rodzaju krawędzi (w granicach wynik jest ten sam
   dla wrap, open i stop). Dopiero poza granicami stara ścieżka.
2. Sprawdzić `-gcflags=-m`, czy `move`/`fold`/`wrap` inlinują się do `Translate` po zmianie;
   jeśli budżet inlinera nie pozwala, zostawić szybką ścieżkę i odnotować.
3. Testy aabbworld (`engine_wall_test.go`, `space_wrap_test.go`, `edges_test.go`) bez zmian
   wyniku; dodać przypadek pudełka dotykającego krawędzi dokładnie (`lo+length == world`).
4. Pomiar: benchmark aabbworld dla `Translate` (dodać, jeśli nie ma) i `Benchmark_World_Tick`
   w gram z `go.work`. Raport z propozycją wpisu do `CHANGELOG.md` aabbworld; tag robi
   właściciel.

### 1D. `clock.Simulate` wiązane raz

`plans-gc Z5` punkt 1 bez zmian. Po nim `Benchmark_World_Tick` ma dawać 0 allocs/op; poprawić
zdanie w `BENCHMARKS.md`, jeśli nadal nie daje.

### 1E. `gpu.Present` i `SetBindGroup` bez alokacji na klatkę

`plans-gc Z4` bez zmian, z naciskiem na krok 1: ustalenie z linią źródła w gogpu/wgpu, czy
`BeginRenderPass`, `SetBindGroup` i `Submit` kopiują przekazane wycinki. To ustalenie jest
potrzebne etapowi 5, więc trafia do raportu osobnym akapitem, także gdy sama zmiana okaże się
mała.

### 1F. Drobne alokacje per klatka i per tick

`plans-gc Z5` punkty 2-5, 7, 8. Punkt 6 (dwa `RemoveCompOne` na pocisk) rozstrzygnięty:
goke v3.5.0 nie ma zdjęcia kilku komponentów naraz; `orch/scheduler.go:107-127` wykonuje
każdą komendę osobno przez `mutator.RemoveComp`, więc pocisk przechodzi dwa archetypy.
Zostaje jak jest; pozycja wraca w etapie 4 jako uwaga dla goke. Punkty bez benchmarku
wchodzą tylko, gdy etap 0 pokazał je w gctrace albo są oczywiście per klatka (Sprintf nazw
lamp, boxowanie uniformów przy wypieku cienia).

---

## Etap 2. Nawigacja

`plans-gc Z2`, potem `plans-gc Z3` od kroku 2 (benchmark z kroku 1 zrobił etap 0). Bez
zmian w treści, z jedną poprawką metody: każdy krok mierzony także profilem CPU, bo
alokacje w `Neighbors` i closures to jedno, a koszt samego A* przy nieudanych ścieżkach to
drugie, i decyzja o back-off (Z3 krok 3) ma wynikać z profilu reżimu "pod murem", nie z
liczby alokacji. Równolegle z etapem 3: inne pliki.

---

## Etap 3. aabbworld czyta kolumny chunków zamiast kopii Item

**Skille:** `goke` (model chunków, `ChunkSnapshot`), `gram-plugin`, `gram-plugins` (world,
motion, collision). Dwa repozytoria: aabbworld (API) i gram (adaptacja). Po etapie 1B i 1C,
żeby zysk mierzyć wobec nowego punktu odniesienia.

**Problem.** `motion.moveSystem.rebuild` (`plugins/motion/move_system.go`) kopiuje co tick
każdy `Base` (104 B) do `aabbworld.Item` (80 B) i woła `remember` per identyfikator: 35%
ticku. `collision.rebuild` (`collision_system.go:378`) robi to drugi raz, gdy zmienią się
Caps albo trwa sweep; `world.indexSystem` trzeci, po spawnie. Silnik kolizji pcha pudełka
w kopii, a `collision.moved` (:686) przepisuje je do `Base` przez `Seek` na każdą pchniętą
encję. Pozycja żyje dwa razy na tick, a między nimi jest kopiowanie w obie strony.

**Kształt.** `Space.Rebuild` przyjmuje przebiegi zamiast Item: przebieg to `ids []uid.UID64`,
wskaźnik bazowy kolumny, stride, offset pudełka (`plane.AABB`), offset `Capability`,
długość. `Grid.index` (`internal/spatial/grid.go:82`) i `Pairs` (:216) czytają pudełko spod
`base + i*stride + boxOff`, bez funkcji wywoływanych per element (między modułami nie
zainlinują się). Pozycja `at int32` zostaje płaska przez sumy prefiksowe długości przebiegów;
`gridCell` dostaje kopię identyfikatora, żeby `Query` (:185) nie szukało przebiegu.
W gram `Base` ma pudełko pod offsetem 0 i Caps pod 100, więc jeden przebieg na chunk z
zapytania po `Base`. Dla sweepów (pudełko rozciągnięte, którego nie ma w kolumnie) zostaje
mała lista Item doklejana za przebiegami; to pociski, dziesiątki.

**Kroki.**
1. aabbworld: nowe wejście `Rebuild` obok starego (stare zostaje dla testów i dla listy
   sweepów), `Grid` z przebiegami, `Items()` dla silnika kolizji jako widok po `at`. Silnik
   (`internal/collide/engine.go:62` `Tick`) pisze pchnięte pudełka wprost pod adres
   przebiegu, czyli do `Base.Pos`. Testy fazy szerokiej (`broad_phase_test.go`) w obu
   wejściach muszą dawać te same pary.
2. Kontrakt do spisania w godoc `Rebuild`: pamięć przebiegów musi stać do ostatniego `Query`
   lub `Tick` przed następnym `Rebuild`. W gram to oznacza: `Rebuild` po każdym `Sync` goke,
   bo swap-and-pop przy migracji przenosi encje między slotami. Strażnik: `TableVer` ze
   `ChunkSnapshot`; przebiegi zbiera się ponownie, gdy wersja którejkolwiek tabeli się zmieniła.
   Zebranie przebiegów jest liniowe w liczbie chunków, więc po każdym `Sync` jest tanie.
3. gram motion: `rebuild` zbiera przebiegi z `held` (`Base` bez `sector.Off`), nie Item.
   gram world: `indexSystem` i `unindexed` (`module.go:202`) znikają albo sprowadzają się do
   ponownego zebrania przebiegów. gram collision: `rebuild` bez kopii poza sweepami; `moved`
   bez `Seek`. Weto z `moved` (`response.Worse`: ruch na gorszy grunt odrzucony, `stale`)
   musi zostać: albo jako sprawdzenie w `Touch` silnika przed pchnięciem, albo jako przejście
   po `Tick` po liście pchniętych, które cofa pudełko. Wybrać to, co zachowa dzisiejsze testy
   kolizji i probe mechanisms/platformer bez zmiany zachowania.
4. Pomiar: `Benchmark_World_Tick` i `Benchmark_Collision_Tick` z profilem przed i po;
   oczekiwanie: `rebuild` schodzi do ułamka procenta, `seek` znika z profilu kolizji.
   Probe na kopiach motion/collision, motion/bullet (sweepy), navigation/spawn (spawn i
   despawn w biegu, czyli `Sync` między ruchem a kolizją).

**Poza zakresem.** Jakakolwiek zmiana w tym, co faza szeroka zwraca. Wersje per chunk
(etap 4): tu wystarczy `TableVer`.

---

## Etap 4. goke: wersje zmian per chunk

**Skille:** `goke` (`scripts/api.sh`, release flow). Praca w `/home/karol/CODE/goke`.
Po etapie 3, bo przebiegi z etapu 3 są konsumentem, który pokaże, czy wersje są potrzebne
poza GPU (zapis gry, sieć).

**Co jest.** `Table.version` (`internal/colstore/table.go`) rośnie tylko przy usunięciu
i relokacji; `ChunkSnapshot` (`internal/bulk`, `internal/query/matcher.go:212`) niesie
`ChunkPtr` i `TableVer`; o zapisach danych goke nie wie nic; `Cursor` (`iter/cursor.go`) ma
`Base`, `Offsets`, `IDs`.

**Kształt.**
1. `vers []uint32` w `chunk.Pack` (`internal/chunk/pack.go`) równolegle do `chunks`;
   `AddChunks`, `trimTrailing` (:134), `SwapChunks` (:161) utrzymują. Nie w pamięci chunka:
   kolumna identyfikatorów zostaje pod offsetem 0 (MassMigrate na tym polega).
2. `Cursor.Ver *uint32`, ustawiany w `Table.FillCursorNext` (:145) i w `Seek`.
3. Akcesor `Mut[T]` jak `Comp[T]` (`comp.go:26-29`), którego `Slice`/`At` robią inkrement
   przez `cur.Ver`. `Comp[T]` zostaje odczytem. Licznik monotoniczny per chunk, bez zegara
   globalnego; konsument pamięta ostatnią widzianą wartość.
4. `ChunkSnapshot.Ver`. Reguła dla konsumenta: zmieniła się `TableVer`, to cała tabela od
   nowa; nie zmieniła się, to porównanie `Ver` per chunk.
5. Tryb testowy (build tag albo opcja ECS): w `Sync` skrót bajtów chunka porównywany z
   poprzednim; zmiana bez `Mut` to panika z nazwą archetypu. Go nie ma wycinków tylko do
   odczytu, więc to jedyna egzekucja kontraktu.

**Koszt do potwierdzenia benchmarkami goke:** jeden zapis wskaźnika na chunk na zapytanie,
jeden inkrement na `Slice` przez `Mut`, zero w ciele pętli. `bench/` goke przed i po, jak
w `BENCHMARKS.md` goke.

**Uwaga do CmdBuf (z 1F).** Zdjęcie kilku komponentów jedną komendą to osobna, planowana
zmiana (zbiorcze operacje strukturalne); tu tylko odnotować w raporcie, nie robić.

**W gram, po wydaniu:** podbicie goke, przejście systemów piszących `Base` na `Mut`
(motion, collision, navigation, bullet, spawn). Potem liczba z etapu 0 krok 3 wraca: jeśli
ruch dotyka co tick każdego chunka z `Base` (bo każda encja ma `Vel` w `Base`), wersje
mówią "wszystko brudne" i nic nie dają rendererowi. Decyzja do podjęcia wtedy, nie teraz:
tag `Still` nadawany przez velocity, gdy prędkość spada do zera, i wykluczany z zapytania
ruchu (migracja przy każdym zatrzymaniu i ruszeniu; `doc/benchmarks-history.md` "Markers" mówi,
ile to kosztuje), albo rozbicie `Base` tak, by stojąca encja nie miała `Velocity` (duża zmiana,
`Base` jest w ponad stu plikach). Bez tej decyzji etap 5 nie ma sensu.

---

## Etap 5. Lustro chunków na GPU dla renderera encji. Bramka

**Skille:** `gram-render`, `gram-plugins` (world), `goke`. Po etapach 1E, 3, 4 i po
decyzji z końca etapu 4.

**Bramka.** Z profilu `Benchmark_World_Draw` (etap 0): jeśli większość klatki encji to
`rules.Run`, `view.Level` i fog per encja, a nie składanie instancji w `Sprites` (`lay`,
`Rect`, `Turned`), lustro nic nie da, bo reguły wyglądu (animacje czasem) zmieniają warstwy
bez zapisu do żadnego komponentu i chunk "czysty" nadal trzeba przejść. Wtedy etap zamyka
się raportem. Jeśli składanie instancji dominuje, dalej.

**Co już jest.** Ścieżka `DirectLook` w `plugins/world/internal/look/flat.go:40`
(`Begin`, `DrawSprites`) i `render/sprites.go:119` (`Draw`): sprite'y jednym batchem
instancjonowanym, rysowanym w kolejności Composera przez `Frame.DrawAt(lowest)`
(`render/frame.go:99`), projekcja i kopie przy szwach torusa w vertex shaderze (`ViewProj`,
`Wrap`, `Vertices = 6 * kopie`). Wysyłka przez pierścień staging
(`render/gpu/staging.go`, cztery w locie). Czyli GPU już projektuje; CPU składa instancje
per sprite per klatka.

**Kształt.** Instancje trzymane na GPU per chunk w slabach (kilka dużych buforów; chunk to
offset w slabie), odświeżane tylko dla chunków, których `Ver` lub `TableVer` się zmieniły,
z konwersją `Base` do float32 przy wysyłce przez ten sam pierścień staging. Rysowanie jak
dziś, jednym batchem w `DrawAt`. Warstwy i widoczność z reguł wyglądu zostają per klatka,
ale jako mała kolumna (indeks sprite'a, flaga) obok trwałych instancji, nie jako pełne
instancje.

**Kroki.**
1. Projekt na stronie: co jest trwałe (pudełko, kierunek, wysokość), co per klatka (warstwa,
   flaga, fog), jak batch z trwałymi instancjami wpisuje się w `Frame.DrawAt` i w sort
   Composera (sprite'y nieprzezroczyste przez test głębi; z alfa w dzisiejszej kolejności).
2. Lustro jako typ w `render/gpu` albo w `plugins/world`: rejestr chunków po `ChunkPtr`
   z wersją, alokator offsetów w slabie, wysyłka brudnych przez staging, zwalnianie po
   zniknięciu chunka. Zero zależności goke od gogpu: wejściem są `ChunkSnapshot` i przebiegi
   z etapu 3.
3. Pomiar: `Benchmark_World_Draw` w trzech kadrach, przed i po, z profilem; scena z 5 000
   encji, z których porusza się 10% (liczba z etapu 0 krok 3 jako wzór). `GRAM_SHOT` z kopii
   motion/collision i board/topography: obrazy identyczne.

---

## Etap 6. Compute na GPU: wzrok nad reliefem. Bramka

**Skille:** `gram-plugins` (vision, board, topography), `gram-render`, `ceremony`
(co widzą reguły i kiedy). Po etapie 5 albo zamiast niego, gdy 5 zamknął się na bramce.

**Bramka.** Z etapu 0: `Benchmark_Vision_Relief` w reżimie, w którym gra naprawdę biegnie
(dziś "profil, 15 razy na sekundę" to 0,81 ms, "co 50 m" 7,28 ms). Jeśli produkcyjny reżim
mieści się poniżej milisekundy, etap zamyka się raportem. Jeśli nie, dalej.

**Kształt.** Raycast stożków po polu wysokości w compute shaderze gogpu/wgpu (storage
buffer z reliefem, bufor obserwatorów, bufor wyników: widziani i zarysy), readback do
pierścienia buforów, wyniki czytane tick później. Reguły o widzeniu (`Sighted`, listy
widzianych) działają więc z opóźnieniem jednego ticku. To zmiana semantyki i musi być
nazwana w dokumentacji pluginu, a probe rules/vision i rules/guard muszą pokazać, że
myśliwy i strażnicy zachowują się tak samo przy 60 TPS.

**Kroki.** Projekt na stronie (układ buforów, kiedy relief wysyłany od nowa: `Version()`
reliefu), shader obok istniejących w `render/shaders`, system wzroku z dwoma ścieżkami
(CPU jak dziś; GPU za flagą), pomiar w obu, probe. Faza szeroka kolizji na GPU nie jest
planowana: pary są potrzebne w tym samym ticku, a solver pyta reguł o każdą parę; wraca
tylko, jeśli profil z etapu 0 pokaże `Grid.Pairs` jako większość ticku kolizji.

---

## Co nie wchodzi

- Przeniesienie `Position` na GPU jako komponentu rezydentnego (patrz decyzje na początku).
- GPU dla ruchu, sterowania, grawitacji, wysokości: mikrosekundy.
- Zmiana `Base` na float32: pół biblioteki i aabbworld; lustro konwertuje przy wysyłce.
- Jakakolwiek zmiana w tym, co faza szeroka aabbworld zwraca, ani w tym, jak jednostki
  omijają się w nawigacji.

---

## Etap 0: wynik (2026-10-19, `51da9307`, gałąź `perf/plans-perf`)

**Co powstało.** `bench/navigation_test.go` (`Benchmark_Navigation_Tick`, open i crowded, 100
i 1 000), `bench/bullet_test.go` (`Benchmark_Bullet_Flight`, straight i thrown, 1, 5, 20 strzałów
na tick), `bench/text_test.go` (`Benchmark_Text_Panel`, `Benchmark_Frame_UI`, headless GPU).
W `BENCHMARKS.md` sekcje Navigation, Shots, Text and a frame with UI, Climate, Where the time
goes (profile jedenastu benchmarków), The demos (gctrace, klatki, GOGC, udział poruszających
się) i noty "Stale" przy Shadows, Shores i Island. Skrypty pomiaru dem (`run_demo.sh`,
`gcstats.sh`, `fpsstats.sh`, `medians.py`) i sondy (`probes/`) weszły pod skill `gram-probe`,
sekcja "Measuring a demo's frames"; profile i logi dem zostały w scratchpadzie sesji.

**Poprawki przesłanek planu.**
1. Plansza nie jest już kosztem CPU klatki. Od pluginu light (`19174ed0`) `Compose` tylko
   planuje, malowanie to przebieg GPU; `Benchmark_Board_Shadows/Shores/Island` dają ~0,1 µs.
   Liczby "wyspa 1,8-4 ms, cienie i brzegi 4-5 ms" z początku planu są nieaktualne. Punkt 3
   kandydatów ("światło, cienie i brzegi") skreślony; koszt rysowania planszy mierzą dema
   (topography: drawing 0,88 ms na klatkę).
2. GC nie jest kosztem klatki w żadnym demie: 0,17-0,45 cyklu na sekundę, skumulowany udział
   CPU 0%, `GOGC=200/400` bez wpływu na tick i rysowanie, tylko wyższy szczyt sterty. Etap 1F
   i `plans-gc` Z4/Z5 to porządek, nie wydajność; zostają bez oczekiwania zysku w milisekundach.
3. Największy koszt na tick to nawigacja: 1 000 jednostek 10,8 ms w otwartym polu i 270 ms w
   tłoku; A* 55-81%, pod nim odczyty komórek przez mapy (`Cells.Kind`/`KindAt`,
   `Square.Toward`, `boardLevels.Deck`). Etap 2 ma zacząć od ścieżki A* (`Price`, `KindAt`,
   `Toward`), nie od alokacji, i idzie przed etapem 3.
4. Udział poruszających się encji na tick: dema planszy 5%, expanse 5% ze 100 000 (pasma),
   dema ruchu 100%. Śledzenie zmian per chunk (etap 4 i 5) ma sens w demach z planszą, nie w
   motion. Scena `World_Draw` ma 100% z konstrukcji.
5. Bramka etapu 5 otwarta: w `World_Draw` składanie instancji (`Flat.Sprite`/`lay`,
   `VisitWrapImages`, `Sprites.Rect`) to 59% klatki, praca per encja 22%.
6. Bramka etapu 6 zamknięta w reżimie produkcyjnym: profil przy 15 Hz to 0,68 ms; "co 50 m"
   6,6 ms. Etap 6 zostaje warunkowy na gęstsze próbkowanie.
7. Tekst: panel 261 µs i 486 alokacji; 28% to wywołania Vulkana przez goffi, 28% glify.
8. Kolizje: solver 30%, przemiatanie par 18%, gram's `rest` 19% (w tym `met` 13% flat), `Seek`
   w `moved` 3%. Faza szeroka nie dominuje, więc kolizje na GPU zostają wykluczone; nowy, tani
   kandydat: `collisionSystem.rest` (do etapu 1 albo 3).

**Pokrycie liczbami.** 1A: brak benchmarku overlayu; pomiar przed i po w demie topography
(drawing 0,88 ms). 1B: tak (`archHypot` 20% ticku). 1C: tak (`Translate` 27%). 1D: tak (1 alloc
na tick). 1E: tak (`Frame_UI`, `Text_Panel`, `cgocall` 20-28%). 1F: częściowo (telemetria przez
`Text_Panel`; lampy bez liczby). 2: tak. 3: tak (`rebuild` 34%, `Seek` 3%). 4: tak (udziały
poruszających się). 5: tak, bramka otwarta. 6: tak, bramka zamknięta.

---

## Etap 1: wynik (2026-10-19, gałąź `perf/plans-perf`, HEAD po scaleniach)

Wykonanie równoległe w pięciu worktree, scalenie fast-forward w kolejności W2, W4, W1, W5, W3,
pomiar po każdym scaleniu naprzemiennie z kopią stanu sprzed. Liczby to mediany z 12 (tick,
klatki) lub 8 (kolizje) biegów.

| Część | Przed | Po | Uwagi |
|:--|--:|--:|:--|
| 1B+1D tick świata 5 000 / 1 000 | 215 / 44,5 µs, 1 alloc | 169 / 35,2 µs, 0 alloc | `archHypot` znika; `clampStep` dopiero teraz inlinuje się (z `Hypot` przekraczał budżet) |
| 1D kolizje, alokacje | 3 na tick | 0 | dwie closures `Simulate` i method value `RolesOf` w `world.module.tick` |
| 1G kolizje 8 388 pudełek | 4,06 ms | 3,35 ms | `rest` 20,7% → 5,9%; `contacts/tick` identyczne w sześciu scenach; 10×10 przy 40% 2,95 → 2,64 ms |
| 1A kopia widoków, ciepła, płaska / relief | 925 µs / 1,57 ms | 16 / 21 ns | `Benchmark_Views_Copy` (nowy, `plugins/vision`); zmiana komórki bez zmian 2,1 / 2,8 ms |
| 1E+1F panel tekstu | 273 µs, 486 alloc, 149 KB | 240 µs, 452 alloc, 20 KB | reszta alokacji to boxowanie argumentów `vkCmd*` w HAL gogpu (~36 na draw) i `fmt` benchmarku |
| 1E+1F klatka z UI | 532 µs, 703 alloc, 160 KB | 472 µs, 646 alloc, 28 KB | `World_Draw` bez zmian (0 alloc) |
| 1E+1F motion/collision, alokacje na klatkę | 152 | 119 | sonda `MemStats`, vsync włączony |
| 1C aabbworld `Translate` inside, euklides / torus | 13 / 16 ns | 8 / 9 ns | warunek w samym `Translate`, `move` nie inlinuje się (koszt 138 > 80) |
| 1C tick świata 5 000 / 1 000 z lokalnym aabbworld | 168 / 35,2 µs | 128 / 27,0 µs | łącznie z 215 µs: −40%; `rebuild` ma teraz 54% |

**Pomiar 1A w demie nie zadziałał jako dowód.** Sonda wybierała obserwatora i włączała kamerę
podążającą (C), ale „drawing” w `GRAM_FPS_LOG` został przy 0,9 ms, a FPS bez vsync dryfował
±15% między parami. Jednostki gracza w topography są zaznaczone od spawnu, więc overlay był
rysowany już w etapie 0, a jego koszt leżał poza trzema licznikami klatki. Stąd benchmark
`Benchmark_Views_Copy`, który mierzy dokładnie usuniętą pracę; zrzuty klatki 600 i 1500 z
`PROBE_SEED=7` różnią się 65 i 144 pikselami z 786 tys. Demo motion/collision losuje prędkości
bez ziarna, więc jego zrzuty nie są porównywalne między biegami; porównywalna jest sonda alokacji.

**Kontrakt wgpu v0.34.5 dla etapu 5** (natywny backend): `BeginRenderPass` kopiuje deskryptor,
`Submit` nie trzyma wycinka, `WriteBuffer` kopiuje; `SetBindGroup` oddaje wycinek offsetów
backendowi i GLES trzyma go do `Submit`, więc gram ma arenę offsetów resetowaną po `Submit`
i stały `zeroOffsets` dla Present. Pułapka `presented[k]` (aliasowanie z reużytym buforem)
rozwiązana własną kopią. Każde wywołanie natywne to jeden `runtime.cgocall` przez goffi plus
`__errno_location`; HAL Vulkana boxuje argumenty każdego `vkCmd*`, ok. 36 alokacji na draw: to
dolna granica alokacji klatki z tekstem, poza zasięgiem gram.

**aabbworld.** 1C wydane jako v1.13.0 (`surface.go`, `space_translate_test.go`,
`bench/plane_test.go` z `across`/`inside`, `BENCHMARKS.md`, `CHANGELOG.md`); gram podbity do
v1.13.0 w `go.mod` na `perf/plans-perf`, `/home/karol/CODE/go.work` bez aabbworld.

**Pominięte i uwagi.** Cache „raz na klatkę” odczytu tekstu etykiety w ui (4 odczyty na klatkę)
nie zrobiony, telemetria odświeża tekst co 250 ms, co zeruje ten koszt tam. `DrawImageIn`
nadal robi własny wycinek uniformów (jedna linia, jeśli chcieć). `cast.go` `set` boxuje wycinek
przy każdym wywołaniu (24 B). plans-gc Z1, Z4, Z5 zrobione; Z2, Z3 to etap 2; Z6 to etap 0.
Nowy punkt odniesienia dla etapu 3: tick świata 5 000 encji 169 µs (128 µs z aabbworld v1.13.0),
z czego kopia Base do Item 54%.

---

## Etap R. Rozdzielczość obrazu świata i staranność cieniowania per pierścień

**Skille:** `gram-render`, `gram-plugin`, `gram-plugins` (cameras, topography), `gram-probe`.
Dodany 2026-10-19 po pomiarze board/topography na pełnym ekranie 2560×1440: 38–45 FPS
niezależnie od vsync, liczniki CPU 5,8 ms z 26 ms klatki, reszta GPU; koszt rośnie z liczbą
pikseli (1024×768: 230 FPS), z `Resizable: false` świat w 1024×768 skalowany przy prezentacji
daje 146 FPS; board/flat w tej rozdzielczości 94 FPS, więc ~15 ms z 26 to przebiegi reliefu.

**Decyzje.** Rozdzielczość tylko obrazu świata (UI natywne), konfiguracja inicjalizuje zmienną w
biegu, komenda ją stopniuje (Alt+=/−), adaptacja włączana komendą (Alt+A), domyślnie wyłączona,
budżet 60 FPS w konfiguracji. Staranność cieniowania terenu per pierścień sektorów od sektora
pod środkiem ekranu (Alt+Shift+=/−): rzeki i morze zostają, tracą detal; poziom 0 zachowuje
normalną, traci cień gruntu i lampy; hex poza zakresem. Słowo „Resolution”, bo `camera.Scaler`
istnieje; krok mnożnikowy 0,85 do 0,4; odległość pierścienia do bliższego z sektora pod
środkiem i sektora pod okiem; rysowanie od dalekich do bliskich; kamera żyje w pikselach obrazu,
konwersja tylko na granicy UI (wejście, piny); ręczny krok rozdzielczości wyłącza adaptację.

**Części.** 0: `Benchmark_Frame_Topography` (headless GPU, 1440p, readback jako synchronizacja).
1: `camera.Resolved` w kamerach topdown, ortho, persp, view; `render.Feed` rysuje w rozdzielczości
kamery i pokazuje w rozmiarze elementu; `ui.picture` skaluje liniowo, także z maską. 2:
`control.Context.Resolution`, `Pixels`, `Shown`, `Edges`; gracze skalują kursor, deltę i starty;
prędkości przewijania i strefy krawędzi w pikselach pokazywanych. 3: `cameras.Config{Resolution,
Adaptive, FrameBudget}`, komendy `Resolution{Camera, Factor}` i `Adapt{}`, adapter z histerezą
(0,92–0,98 budżetu, 2 s pod, 5 s nad, wypalenie 30 s po cofnięciu), reporter „Picture”. 4:
`view.Sectors.Cols`, `runs` z care per sektor, trzy shadery z `const CARE`, `LevelWater` w
`sea.wgsl`, `topography.Config.Care`, komenda `Care{Step}` po drabince 1, 2, 3, wszystko,
reporter „Ground”. 5: integracja, pomiar, dokumenty.

## Etap R: wynik (2026-10-19, gałąź `perf/plans-perf`, `b9f2cffe` + dokumenty)

Wykonanie: część 0 przez integratora, WA (części 1–3) i WB (część 4) równolegle w worktree,
scalenie fast-forward, pomiar na bezczynnej maszynie. Liczby w `doc/benchmarks-history.md`,
sekcja „The island's frame”.

| Co | Wynik |
|:--|--:|
| Wyspa 1440p, izo, bez readbacku (7,7 ms) | 8,0 ms; bez wody 6,7; 0,7 piksela 3,8; care R=1 6,8 |
| Demo 2560×1440 bez vsync, baza | 45 FPS |
| rozdzielczość 0,72 / 0,52 | 70 / 106 FPS |
| staranność R=2 / R=1 | 48 / 53 FPS |
| 0,72 + R=2 | 71 FPS |
| perspektywa, R=1 | 51 wobec 46 |
| adaptacja, vsync, ekran 100 Hz, budżet 60 | 45 → 57 w 3 s, próbny krok w górę co 30 s |

**Wnioski.** Rozdzielczość jest dźwignią pierwszego rzędu i skaluje się liniowo z pikselami;
staranność daje 7–18% w zależności od ilości wody w kadrze, najwięcej w perspektywie, i jest
tańsza wizualnie niż utrata ostrości na całym obrazie. Oba mechanizmy się składają. Zrzuty
klatki 600 przy 0,52 i w perspektywie przy R=1: układ bez zmian, panel ostry, daleka woda płaska
bez glintu, rzeki widoczne wszędzie.

**Uwagi.** Adapter spoczywa w paśmie 0,92–0,98 budżetu, więc przy budżecie 60 zadowala go 55–59
FPS; ostrzejszy próg to jedna stała w `adapt.go`. Benchmark wyspy nie uruchamiał `RunPlan` świata,
więc z sektorami rysowałby tylko skirt; naprawione w części 4. Pierścienie są rysowane od
bliskich do dalekich (pełna staranność pierwsza), żeby prime dalszych odrzucał grunt zasłonięty
wzgórzami; zmierzone wobec kolejności odwrotnej w 2560×1440: perspektywa R=1 49/51 → 49/52,
R=2 47/49 → 48/50, izo bez zmian, czyli w granicach szumu. `zoneRound` stref krawędzi zostaje
w pikselach kamery (kosmetyka). Hex: no-op.

## Etap R2. Rozdzielczość pierścieniowa obrazu świata jako druga strategia adaptera

**Skille:** `gram-render`, `gram-plugin`, `gram-plugins` (cameras), `gram-probe`. Dodany
2026-10-19 po etapie R: użytkownik oczekiwał, że rozdzielczość będzie spadać pierścieniami od
środka obrazu (pełna w centrum uwagi, niższa ku brzegom) i chce tego jako drugiej strategii
adaptera. Composer składa wierzchołki w pikselach kamery dla jednego rozmiaru obrazu, więc obraz
o różnej gęstości to kilka obrazów złożonych jeden na drugim.

**Decyzje.** Trzy warstwy: cały kadr, środkowe 60%, środkowe 35%, każda to ta sama kamera z
viewportem przyciętym do środka i swoją rozdzielczością, składane od najgrubszej do
najdokładniejszej z miękką krawędzią (20 px); UI natywne. Przycięcie to stan kamery
(`camera.Cropping`), nie dekorator; perspektywa trzyma ogniskową nieprzyciętego kadru i podmienia
tylko `proj.focal`, bez `look()` i `Pick`. Pierścienie to ustawienie zapisywane z grą
(`Settings.Rings`, panel F10) obok `Settings.Adaptive`; jedna komenda `Adapt{On, Rings}`; Alt+A
cyklicznie: idzie za budżetem, idzie w pierścieniach, stoi w całości. Preset 0,6 / 0,8 / 1,0,
podłogi drabinki 0,4 / 0,6 / 0,7; adapter i ręczne kroki Alt+=/− obniżają najpierw zewnętrzny
pierścień, potem środkowy, środek ostatni, w górę odwrotnie; ręczny krok zatrzymuje adapter nie
ruszając pierścieni. Rozdzielczość kamery jest rozdzielczością środka, więc konwersje wejścia i
pinezek się nie zmieniają. Warstwy nakładają się (zewnętrzna rysuje cały kadr): preset to 0,71
pikseli pełnej klatki (jak jednolite 0,85), podłoga 0,35 (jednolita 0,16); tryb kupuje detal
środka przy podobnym budżecie, nie klatkarz ponad tryb jednolity.

**Części.** A0: `camera.Ring`, `Ringed`, `Cropping`, `RingsOf`, `IsCropped`. A1–A2: `Crop`,
`Uncrop`, `Cropped` w kamerach topdown, ortho, persp i view. B1: `DrawImageOptions.Fade`, blit
przez shader `imageSrc0Linear * color * faded(custom)`. B2: `render.Feed` rysuje warstwy
(przycięcie, obraz per pierścień, blit z piórem) i przywraca kamerę przed minimapą. B3:
`keptDepths` 4 → 8. B4: obrazy chmur (aloft, backdrop) trzymane per rozmiar celu. B5: strefy
krawędzi tylko na nieprzyciętej kamerze. C: `Settings.Rings`, `Config.Rings`, `Adapt{On, Rings}`,
drabinka `stepRings`, reporter „Picture: 0.60/0.80/1.00 of the pixels, ringed, adaptive”. D:
`Benchmark_Frame_Topography` `iso,ringed` i `persp,ringed`, pomiar demo 2560×1440, dokumenty.

## Etap R2: wynik (2026-10-19, gałąź `perf/ringed-picture`, `76c18e9b` + dokumenty)

Wykonanie: A0 i B3 przez integratora, WA (A1, A2, B1, B2, B4) i WB (B5, C) równolegle w
worktree, cherry-pick na gałąź, pomiar na bezczynnej maszynie, baza `a25a4af2` i drzewo
na przemian. Liczby w `doc/benchmarks-history.md`, „The island's frame”, akapit „In rings”.

| Co | Wynik |
|:--|--:|
| Wyspa 1440p, izo / w pierścieniach (0,575 px), bez readbacku | 7,3 / 5,9 ms |
| Wyspa 1440p, persp / w pierścieniach | 11,1 / 7,9 ms |
| Demo 2560×1440 bez vsync, baza / drzewo, wszystkie piksele | 58 / 58 FPS |
| jednolite 0,85 (0,72 px) / preset pierścieni (0,71 px) | 70 / 67 |
| jednolite 0,72 (0,52 px) / zewnętrzny na podłodze (0,51 px) | 88 / 85 |
| perspektywa: 0,85 / preset; 0,72 / podłoga | 67 / 64; 83 / 80 |

**Wnioski.** Zgodnie z rachunkiem pikseli pierścienie nie biją trybu jednolitego na klatkach:
przy tym samym udziale rysowanych pikseli kosztują ok. 3 FPS (4%) na trzy składania CPU
(„drawing” 1,2 → 2,1 ms) i dwa blity, a dają pełny detal w środku kadru. Zrzuty klatki 600 w
izo i perspektywie: brak szwów przy 60% i 35%, glint wody różni się nieco w paśmie pióra.
Adapter w pierścieniach nie był mierzony: baza 58 FPS leży w jego paśmie spoczynku (55–59 z
budżetu 60), więc nie miałby czego robić.

**Uwagi.** WB wykrył, że `persisted.Persisted()` kamer nigdy nie zapisywał `Settings` (strefy,
adaptacja): naprawione, starszy zapis zostawia je jak są. Perspektywa trzyma `crop` jako wektor
(w/W, h/H), bo parzystość przycięć daje różne ułamki w osiach. Aloft i backdrop trzymają po 8
rozmiarów (dwa widoki w pierścieniach w podzielonym ekranie). B6 (dziura w warstwach
zewnętrznych, 0,71 → 0,505 px) niewykonane: zysk ok. 10 FPS przy presecie, do rozważenia, gdy
tryb pierścieniowy okaże się używany. Alokacje na klatkę wyspy rosną z 446 do 1037 (trzy
składania); to boxing gogpu per draw, nie nowe alokacje w gram.

## Etap S (2026-10-19, gałąź `perf/ringed-picture`): ekran ustawień w sekcjach

Nie pomiar, a obsługa etapów R i R2: ekran F10 w sekcjach wspólnych dla pluginów (`Section` na
ustawieniu, `control.Picture…`, `ui.Subheading`, wycentrowany); strategia obrazu jako dropdown
(`Settings.Strategy`: still, the whole picture, in rings) z pokrętłami obu strategii (budżet
klatki; udziały i rozdzielczości pierścieni), wszystkie ręczne, gdy adapter stoi; staranność
jako suwak do „every sector”; nakładki „Show the rings” (obrys przyciętej warstwy) i „Show the
care” (granice pasm staranności po kratownicy); telemetria „Picture: … following a budget of 60
FPS” / „…, still”. Zrzuty w `BENCHMARKS.md` nie; sprawdzone zrzutem demo.

## Etap T. Grunt w rozdzielczości kamery, figury w każdym pikselu; pięć poziomów staranności

**Skille:** `gram-render`, `gram-plugin`, `gram-plugins` (cameras, topography), `gram-probe`.
Dodany 2026-10-20 po rozmowie o leverach: to figury, obrysy i znaczniki zdradzają rozmycie, nie
grunt, więc obniżona rozdzielczość może dotyczyć samego gruntu; staranność może spadać dłuższą
drabinką.

**Decyzje.** Dwa przebiegi na klatkę, każde źródło składane raz w swoim: grunt z kamerą w jej
rozdzielczości do mniejszego obrazu z głębią Readable, potem to, co nad figurami (figury,
chmury, deszcz, obrysy, pudła, strefy, pierścienie: źródła deklarują `render.Overlying`) z
kamerą przyciętą do całego kadru w 1 wprost na cel, testowane głębią gruntu rozciągniętą przez
`gpu.ScaleDepth` (wspólny shader z `CopyDepth`, skala w `place`); przebieg górny dziedziczy
uniformy i obrazy gruntu. `render.Grounded{DrawGround, DrawOver}` opcjonalne na composerze.
Przełącznik `Settings.SoftFigures` (zero = ostre, stare zapisy nie wyłączają), „Figures at every
pixel”, `SharpFigures{On}`, `camera.Figured`; w pierścieniach bez zmian (następny etap: jedno
składanie, warstwy per widok). Drabinka: `cares = 5`, `care = max(4 − (d−1)/R, 0)`; 4 wszystko,
3 bez brzegu, piany, wody płynącej i glintu ujścia, 2 płaskie lustro, 1 bez cienia gruntu i
lamp, 0 bez siatki, cienia chmur i lustra; mgła zawsze. Reporter „9 · 3 · 2 · 1 · 0 (full to
least)”, widok staranności kolor per poziom, suwak i klawisze bez zmian (R).

**Części.** A: topografia (drabinka, reporter, widok). B1: `gpu.ScaleDepth`, composer w dwóch
przebiegach, Feed `drawSharp`. B2: plugin kamer (ustawienie, komenda, `camera.Figured` w
kamerach), `Overlies()` w siedmiu źródłach, bench `iso,0.7,feed`, `iso,0.7,sharp`,
`persp,0.7`, `persp,0.7,sharp`. Integracja, pomiar, dokumenty.

## Etap T: wynik (2026-10-20, gałąź `perf/sharp-figures`, baza `e5ec6dbc`)

Wykonanie: interfejsy przez integratora, A (topografia), B1 (gpu, render) i B2 (plugin kamer,
źródła, bench) równolegle w worktree, cherry-pick, pomiar na bezczynnej maszynie (pierwszy
pomiar był skażony równoległym pomiarem innej sesji: baza raz 15, raz 24 ms). Liczby w
`doc/benchmarks-history.md`, „The island's frame”, akapit „The ground at the camera's resolution…”.

| Co | Wynik |
|:--|--:|
| Wyspa 1440p, izo / persp, pełna rozdzielczość, baza i drzewo | 7,9 / 11,6 ms bez zmian |
| izo 0,7 przez Feed: figury miękkie / ostre | 4,7 / 6,4 ms |
| persp 0,7 przez Feed: miękkie / ostre | 6,7 / 8,5 ms |
| care R=1: trzy / pięć poziomów | 6,9 / 7,3 ms |
| Demo 2560×1440, wszystkie piksele, baza / drzewo | 57 / 57 FPS |
| 0,72: baza / ostre / miękkie | 86 / 76 / 86 |
| 0,52: baza / ostre / miękkie | 128 / 108 / 128 |
| perspektywa 0,72: baza / ostre / miękkie | 82 / 73 / 82 |

**Wnioski.** Przebieg nad gruntem kosztuje stałe 1,5–1,8 ms na klatkę niezależnie od
rozdzielczości gruntu: chmury aloft rysują się w rozmiarze ekranu (znane ryzyko), reszta to
niewiele pikseli. Przy 0,52 obraz biegnie 108 FPS z ostrymi krawędziami jednostek wobec 128 z
rozmytymi i 57 w pełnej rozdzielczości: lever działa, a adapter może schodzić niżej. Zrzuty
klatki 600 przy 0,52: miara krawędzi w wycinku z jednostkami 9,9 (ostre) wobec 8,8 (miękkie),
grunt rozmyty tak samo. Pięć poziomów staranności: +0,4 ms przy R=1 (dwa draw calle więcej),
telemetria w perspektywie „care within 1 sectors: 10 · 9 · 8 · 3 · 0”, daleka woda płaska dwa
sektory wcześniej.

**Uwagi.** Następny krok dla tego levera: chmury w przebiegu górnym w rozmiarze gruntu
(wskazówka `Target.Coarse`) zdjęłyby większość stałego kosztu. Pierścienie w tym etapie bez
ostrych figur (przełącznik nie działa w pierścieniach). Checkout użytkownika w czasie etapu
miał niezacommitowaną pracę nad detalem gruntu w `mesh.wgsl`, `terrain.go`, `plugin.go`:
integracja poszła w osobnym worktree, scalenie tych dwóch linii da drobne konflikty w tych
plikach.

## Etap U. Obraz po stanie kamery: profile taktyczny/jazda, warstwy per widok, jedno składanie

**Skille:** `gram-render`, `gram-plugin`, `gram-plugins` (cameras), `gram-game` (ekran
ustawień), `gram-probe`. Dodany 2026-10-20 jako „następny etap” zapowiedziany w T: wątek z
19.10 (20:47–21:34) o tym, że pierścienie (foveacja) mają sens w pierwszej osobie, a w
widokach taktycznych oko skanuje cały kadr, więc tam lepszy jest podział grunt/figury, a
„daleko” w perspektywie to pas pod horyzontem. Pierwszy etap robiony w częściach przez osobne
agenty (CLAUDE.md „A stage in parts”).

**Decyzje.** Nazwy: pierścienie obrazu to warstwy (`camera.Layer`, `Layered`, `InLayers`,
`Layers{On}`, `LayerShares`, `LayerResolutions`, `ShowLayers`, `LayerFeather`); pierścienie
staranności sektorów zostają. Układ warstw (`camera.Layout`): `Inset` (każda w środku kadru,
wcięta z czterech stron) w izometrii, z góry i w jeździe; `Footed` (każda `Share` kadru wszerz
i wzwyż, wyśrodkowana w poziomie, stojąca na dolnej krawędzi; grubsza warstwa widać wokół jak
stół: blat u góry i dwie nogi) w perspektywie taktycznej, z decyzji użytkownika; `Arrange` daje
oknom całe piksele, parzyste marginesy wciętych. Przycięcie poza środkiem: `Cropping.Crop(win
Window, s)`, `Window()`; perspektywa trzyma punkt główny zamiast `viewportSize/2`, orto
przesuwa `pan`, z góry `effective`; `TransformOf` idzie za `RayField`, więc Directy nic nie
wiedzą. Profile: `Settings{Zones, FrameBudget, ShowLayers, Tactical, Riding Knobs}`, `Knobs{
Strategy, Resolution, SoftFigures, Layers, Inner, Middle, OuterRes, MiddleRes}`, profil kamery
po jej stanie (`ProfileOf`: jazda gdy `camera.HowOf == Inside`), układ Footed gdy taktyczny i
w perspektywie (`Eyed`); plugin ubiera kamerę (`dress`) przy zmianie stanu po `module.RunPlan`;
komendy niosą `Profile`; adapter kroczy każdą kamerą po strategii jej profilu; `Knobs.filled()`
strzeże zerowych floatów per pole (gob zeruje całą strukturę ze starego zapisu). Ekran F10:
sekcje `Picture/Tactical`, `…/Layers`, `Picture/Riding`, `…/Layers`, nazwy elementów ze
ścieżką. Jedno składanie: `render.Laid{Lay(cam, apart); DrawLayer(layer, cam, at Window);
DrawOver(screen, cam)}` zamiast `Grounded`; kawałki składane raz w pikselach całego kadru i
rysowane w każdej warstwie z `gpu.Draw.Place` (skala i przesunięcie okna; `quad.wgsl` liczy
głębię po umieszczeniu, z sight przyciętej kamery), Directy rysują przez przyciętą kamerę;
`gpu.PlaceDepth` (dawny `ScaleDepth` z przesunięciem w `place.zw` i nożycami) składa głębie
warstw w głębię ekranu dla przebiegu górnego, więc ostre figury działają także nad warstwami;
`Sprites.Draw` bez `Reset` (reset w `Begin`), by druga warstwa miała figury. Start dema
board/topography: taktyczny `Uniform` (figury ostre, grunt za budżetem), jazda `InLayers`.

**Części.** Preludium (integrator, gałąź `perf/picture-profiles`): scalenie T (konflikt w
`mesh.wgsl`: ziarno i kołysanie przy `CARE >= 1u`), zmiana nazw, kontrakt `Layout`/`Window`/
`Arrange`. A (`u/profiles`): plugin kamer, `control`, ekran ustawień, demo. B (`u/footed`):
`camera.Cropping` z oknem, trzy kamery, okna i pióro feed, obrys warstw po gaszonych bokach.
C (`u/laid`): `PlaceDepth`, composer `Laid`, `Sprites`/`Flat`, ścieżki feed, bench
`persp,footed`, `iso,layered,sharp`, `persp,footed,sharp`. Integracja B, C, A; obrys warstw
bez bramki `IsCropped` (składany z `Arrange`); pomiar; dokumenty.

## Etap U: wynik (2026-10-20, gałąź `perf/picture-profiles`, baza `b6ecc82c`)

Wykonanie: preludium przez integratora (scalenie T z konfliktem w `mesh.wgsl`, zmiana nazw,
kontrakt `Layout`/`Window`/`Arrange`), części A (`u/profiles`), B (`u/footed`) i C (`u/laid`)
przez trzy agenty Opus 5.5 równolegle w worktree poza `go.work`, scalenie B, A, C na gałąź
etapu (konflikty tylko w `render/feed.go` i dwóch testach: okna B pod ścieżkami C), obrys
warstw bez bramki `IsCropped` i poprawka tras nawigacji (C wykrył: `routes.Lines` zrzucał
odcinki przy pierwszym rysowaniu i trzymał prostokąty w pikselach kamery składającej, więc
warstwy wewnętrzne nie miały tras) przez integratora, pomiar na bezczynnej maszynie. Liczby w
`doc/benchmarks-history.md`, „The island's frame”, akapit „The picture in layers from one
composition…”.

| Co | Wynik |
|:--|--:|
| Wyspa 1440p bez readbacku, izo w warstwach: trzy składania / jedno | 8,0 / 7,0 ms |
| perspektywa w warstwach wciętych: baza / drzewo | 10,4 / 9,4 ms |
| perspektywa w warstwach footed | 8,9 ms |
| izo / perspektywa footed, ostre figury nad warstwami | 10,2 / 11,6 ms |
| izo, persp, 0,7 przez feed (miękkie i ostre) | w granicach dryfu bazy |
| Demo 2560×1440 bez vsync, cały obraz: baza / drzewo | 63 / 63 FPS |
| warstwy presetu, figury miękkie: baza / drzewo (drawing 2,0 / 1,7 ms) | 70 / 70 FPS |
| warstwy presetu, figury ostre (drzewo) | 64 FPS |

**Wnioski.** Jedno składanie zdejmuje ok. 1 ms z klatki wyspy w warstwach i 0,3 ms z „drawing”
dema (Directy nadal rysują per warstwa), czego demo związane GPU nie zamienia na klatki;
footed w perspektywie jest o pół milisekundy tańszy od wciętego (mniej kadru rysowanego dwa
razy). Ostre figury nad warstwami kosztują 2,7–3,1 ms na wyspie (przebieg górny w każdym
pikselu i trzy głębie umieszczane) i 6 FPS w demie. Reszta w granicach dryfu. Profile działają
w demie: start taktyczny `Uniform` z ostrymi figurami, jazda (V) w pierścieniach `InLayers`.

**Uwagi.** Następne kroki tego levera: chmury aloft w przebiegu górnym w rozmiarze gruntu
(stały koszt przebiegu), dziura w zewnętrznych warstwach pod wewnętrznymi (B6 z R2, teraz
tańsza: jedno składanie, głębie są już per warstwa), Directy rysowane raz w pikselach kadru i
umieszczane jak kawałki (wymaga `Place` w `DrawMesh`). Adapter w warstwach nadal nie mierzony
(demo spoczywa w paśmie budżetu). Worktree części usunięte po scaleniu; gałęzie `u/*` zostają
do przeglądu.

## Wykonanie w sesjach: etapy 2–6 i dźwignie po U (plan 2026-10-20, `main` `21e9e106`)

Reguła: `CLAUDE.md` „A stage in parts”, wariant w sesjach. Integrator (ta albo dowolna
późniejsza sesja, Fable albo Opus, uruchomiona w `/home/karol/CODE/goke`, gdzie leży pamięć
projektu) pisze briefy w `doc/briefs/`, użytkownik otwiera na każdy brief osobną sesję Claude
Code, sesja pracuje i commituje na swojej gałęzi, integrator scala do gałęzi etapu, mierzy,
pisze dokumenty i scala etap do `main` (`--no-ff`). W goke i aabbworld scala, taguje i podbija
`gram/go.mod` użytkownik; to ścieżka krytyczna fali 2.

**Co odpada.** „Operacje strukturalne hurtem przez CmdBuf” w goke są zrobione (3.x: `Editor`,
`ValueEditor`, `Remover` jako odroczone partie przez `Query.BeginMigrate`/`Commit`, kompaktowanie
po zebraniu); zostaje tylko uwaga z etapu 4 (`RemoveComp` wielu komponentów jednym poleceniem
poza `Editor`), do raportu, nie do roboty.

**Podział na sesje** (litera gałęzi: `n` nawigacja, `c` przebiegi chunków, `v` wersje,
`r` dźwignie obrazu, `m` lustro; gałąź części `<litera>/<część>`, worktree gram pod
`/home/karol/worktrees/gram/<część>`; goke i aabbworld na własnych gałęziach w swoich repo):

| Fala | Brief | Repo, gałąź | Zależy od | Etap, gałąź etapu w gram |
|:--|:--|:--|:--|:--|
| 1 | `2a-nav-astar` | gram `n/astar` | — | 2, `perf/navigation` |
| 1 | `3a-aabbworld-runs` | aabbworld `feat/runs` | — | 3, `perf/chunk-runs` |
| 1 | `4a-goke-versions` | goke `feat/chunk-versions` | — | 4, `perf/chunk-versions` |
| 1 | `r1-aloft-over` | gram `r/aloft` | — | R, `perf/layer-levers` |
| 1 | `r2-layers-placed` | gram `r/placed` | — (szew z r1 w `composer.go`) | R, `perf/layer-levers` |
| 2 | `2b-nav-bodies` | gram `n/bodies` | 2a scalone do `perf/navigation` | 2 |
| 2 | `3b-gram-runs` | gram `c/runs` | aabbworld wydane, `go.mod` podbity | 3 |
| 2 | `4b-gram-mut` | gram `v/mut` | goke wydane; po 3b (te same pliki motion/collision) | 4 |
| 3 | decyzja `Still` (integrator, po pomiarze 4b) → `5-sprite-mirror` | gram `m/mirror` | 3b, 4b, decyzja | 5, `perf/sprite-mirror` |
| — | etap 6 | — | bramka zamknięta (etap 0, pkt 6) | tylko gdy wzrok zgęstnieje |

Kolejność uzasadniona: fala 1 to pięć niezależnych zbiorów plików w trzech repozytoriach
(nawigacja; aabbworld; goke; aloft z przebiegiem górnym; feed/composer/gpu), więc idą naraz;
3a i 4a najwcześniej, bo ich wydania blokują falę 2; 2b po 2a (te same pliki nawigacji);
4b po 3b, bo oba przepisują `rebuild`/`moved` w motion i collision; 5 dopiero po decyzji o
znaczniku `Still` (etap 4, akapit „W gram, po wydaniu”), bez której wersje per chunk mówią
„wszystko brudne”.

**Integracja po każdej fali** (integrator): scalenie części na gałąź etapu, pełna sekwencja,
każde demo z kamerami pod `timeout`, pomiar bazy i drzewa naprzemiennie na bezczynnej maszynie
(`gram-probe` § Checking a change; benchmarki nazwane w briefach), `BENCHMARKS.md`,
`CHANGELOG.md`, skille, sekcja „Etap N: wynik” tutaj, `--no-ff` do `main`, worktree usunięte,
gałęzie części zostawione do przeglądu.

**Stan** (integrator aktualizuje, 2026-10-21 00:30): fale 1 i 2 zrobione, w `main`: 2a, 2b, odstęp
po nieudanym wyszukiwaniu, 3b, R1, dziura z R2, kopia instancji jednym ruchem. Etapy 4 i 5
zamknięte pomiarem (niżej, „Fala 2: wynik”); gram `m/mirror` zostaje jako ślad. aabbworld v1.14.1 (PR #19, wyścig w
leniwej mapie id z v1.14.0) wydane i podbite w `go.mod`, `-race` wzroku czysty; goke PR #55 i gałąź
`feat/chunk-versions` usunięte. Otwarte: decyzja użytkownika o „dosłownym” odstępie (Detour w
tłoku), etap 6 za zamkniętą bramką.

## Fala 1: wynik (2026-10-20)

Pomiar integratora szeregowo na bezczynnej maszynie (load 0,9–2,2), baza = `main` `e797a753`
(aabbworld `main` v1.13.0, goke `main` v3.5.0) i drzewo na przemian; liczby agentów brane pod
obciążeniem 4–34 tylko orientacyjnie.

| Część | Co | Baza | Drzewo |
|:--|:--|--:|--:|
| 2a | `Navigation_Tick` open 100 / 1 000 | 799 µs / 9,56 ms | 425 µs / 6,32 ms |
| 2a | `Navigation_Tick` crowded 100 / 1 000 | 2,85 / 306 ms | 1,16 / 104 ms |
| 2a | `Navigation_Path` 200×200 (baza agenta) | 94 ms, 14 740 alok. | 18,4 ms, 19 alok. |
| R2 dziura | wyspa izo / footed w warstwach | 15,6 / 17,1 ms | 13,9 / 15,4 ms |
| R2 dziura | to samo z ostrymi figurami | 18,2 / 19,3 ms | 16,6 / 17,9 ms |
| R1 | chmury w przebiegu górnym, 1440p, grunt 0,6 (agent, w procesie) | 6,1 ms | 3,7 ms |
| 3a | `Column_RebuildQueryTick` bez ticku: items / runs | 248 µs | 204 µs |
| 3a | to samo z tickiem kolizji, spread / crowd | 1,29 / 4,31 ms | 1,24 / 4,11 ms |
| 3a | stare wejście: `BroadPhase` tick, minima | 1,78–8,37 ms | +1–3% |
| 4a | `Mut` wobec `Comp`: przejście 65 536 / `SeekH` 512 | 40,0 µs / 2,5 µs | 40,8 µs / 2,9 µs |
| 4a | `Matcher_*` (bez `Mut`) | — | ±3% |

**Wnioski.** Nawigacja: 2,9× w tłoku, 1,5× w otwartym polu, trasy identyczne (golden). Dziura:
1,3–1,8 ms z każdej klatki w warstwach, obraz bez zmian. Przebiegi aabbworld zdejmują kopię
(−18% w samym przebudowaniu), ale z tickiem kolizji zysk w aabbworld to 4–5%: prawdziwa liczba
przyjdzie z 3b (`World_Tick`, gdzie kopia `Base` → `Item` to 54%). Stare wejście aabbworld kosztuje
1–3% więcej (wskaźnik w `Entry`), mediany pod szumem do +13%. Wersje goke: zero w ciele pętli,
+2% na przejściu przez `Mut`, +16% na `SeekH` z `Mut.At`; `WithMutCheck` 7,9 ns na encję na `Sync`.

**Decyzja: Directy umieszczane (R2, drugi commit) nie wchodzą.** Dają 0,25–0,4 ms CPU w demie,
ale na wyspie kosztują 0,1–0,4 ms GPU (warstwy wewnętrzne rysują sektory całego kadru), wnoszą
~670 linii mechanizmu (nagrywanie i odtwarzanie rysowań, `gpu.Record`) z kryterium opartym na
skanie tekstu WGSL (`Program.ReadsPosition`), a dla ośmiu Directów i tak zostaje rysowanie per
warstwa. Wariant „bez przebiegu nieprzezroczystego” dał w demie −0,06/−0,13 ms. Zysk nie
równoważy złożoności (zob. pamięć: „uproszczenie ma zmniejszać ślad”); commit zostaje na
`r/placed` do decyzji użytkownika.

**Co zastano i do zgłoszenia.** aabbworld: `internal/raycast` `TestProfile_SeesWhatAFineStepSees`
pada 1–2 razy na 40 biegów także na `main`; `Scan` w świecie zawijanym alokuje 3 razy na wywołanie
(`raycast.splitAxis`). goke: zapisy przez `Column`, `OptColumn`, `OptComp` nie są liczone (ceremony
pisze efekty przez `OptColumn`: brief 4b), równoległe zapisy przez `Mut` jednego archetypu
ścigają się na liczniku chunka (gram i ceremony nie używają `RunParallel`); `RemoveCompOne` k razy
na jednej encji to k migracji przez archetypy pośrednie (luka poza `Editor`, nie robiona). astar:
słowniki na mapach to ~12% ticku w tłoku, a `expandCurrent` gubi bufor, który `append` urósł
(obejście `WithSuccessorCapacity`); propozycje dla właściciela astar.

## Fala 2: wynik (2026-10-21)

Części 2b, odstęp, 3b i 4b+5 (lustro) przez agenty Opus 5.5; od połowy fali agenty nie mierzą
(decyzja użytkownika: pomiar równoległy jest bezcelowy), integrator mierzy wszystko po kolei na
bezczynnej maszynie (load 1,0–1,5), baza i drzewo naprzemiennie, mediany z 5–6.

| Część | Co | Przed | Po |
|:--|:--|--:|--:|
| 2b | `Navigation_Tick` open 1 000 | 5,67 ms | 4,55 ms |
| 2b | pozostałe trzy reżimy | — | w szumie; alokacje 2,5–3× mniej |
| odstęp | wszystkie reżimy wobec 2b | — | w szumie (±4%) |
| 3b | `World_Tick` 1 000 / 5 000 | 23,3 / 110,9 µs | 13,9 / 58,0 µs |
| 3b | `Collision_Tick`, `Collision_Swept` | — | −1% do −10% |
| kopia instancji | `World_TickDraw`, cały kadr | 0,95–1,01 ms | 0,85–0,90 ms |
| lustro (4b+5) | `World_TickDraw`, 10% / 100% w ruchu, cały kadr | 0,90 / 0,91 ms | 1,07 / 1,10 ms |
| lustro | nikt się nie rusza | 0,95 ms | 0,60 ms |

**Wnioski.** 3b to największy zysk ticku: kopia `Base` do `Item` (52% profilu) znika, tick świata
prawie o połowę krótszy. 2b daje −20% tylko w otwartym polu z 1 000 jednostek; tłok to udane
wyszukiwania Detour (85%), których 2b nie dotyka. Odstęp jest neutralny dla czasu i dla tego, co
widzi gracz (logi i zrzuty identyczne), zmienia kształt zapisu.

**Etapy 4 i 5 zamknięte.** Wersje per chunk w goke dają zysk tylko przez lustro sprite'ów, a lustro
wygrywa jedynie tam, gdzie chunki stoją czyste. W realnych scenach brudzi się każdy chunk w kadrze
co tick (motion/collision, motion/expanse, 10% w ruchu rozrzuconych: 100% chunków), więc lustro jest
o ok. 20% wolniejsze od dzisiejszej ścieżki; na planszy (board/topography) w ogóle nie działa, bo
figury to billboardy. Znacznik `Still` nie pomógłby: w demach, gdzie lustro działa, ruszają się
wszyscy w kadrze. Koszt, którego zysk nie broni: ok. 520 linii lustra, ok. 140 linii liczników w
ośmiu systemach, obowiązek liczenia dla każdego przyszłego pisarza `Base`, `WithMutCheck`
niewłączalny w testach ze światem (zegar i ceremony piszą przez `Comp`/`OptColumn`). Wzięta jest
tylko niezależna od goke kopia instancji jednym ruchem (`fa2fc440`, z `m/mirror` `e1fb48b9`).
Uwagi agenta do API goke, gdyby wersje kiedyś wróciły: `Mut` bez odczytu bez liczenia
(`Mut.Read`), check per kolumna zamiast hashu całego chunka, `ChunkSnapshot.ArchID` typu z
pakietu internal.

**Znalezione.** aabbworld v1.14.0 zostawił mapę id i listę wpisów pierwszemu `Scan`; wzrok gram
skanuje po jednym `Query` na kilku gorutynach, więc `go test -race ./plugins/vision/` pada na
`main` od podbicia (16 wyścigów). Poprawka w aabbworld PR #19 (mapa budowana z komórkami przy
pierwszym `Query`, test łapie 20 wyścigów bez niej), do wydania jako v1.14.1. 3b zostawia lukę:
czytelnik w niesymulowanym `RunPlan` po cudzym niesymulowanym `Sync` (np. selekcja po `MoveTo`
nawigacji) może widzieć nieświeże przebiegi do następnego przebudowania; pełna naprawa to
opakowanie `rc` w `internal/engine/stage_runtime.go` jak w `Clock.Replay`.

## Fala 3 (plan 2026-10-21, gałąź `perf/wave-3`)

Dziewięć części naraz (agenty Opus 5.5, bez pomiarów czasu), briefy w `doc/briefs/w3.md`: A
Detour bez zmiany zachowania, B odstęp „dosłowny” (alternatywa A, wejdzie lepsza), C osiadanie
po `Sync` poza symulacją (poprawność), D słowniki astar, E alokacje kroków ceremony, F `Scan` bez
alokacji i niestabilny test w aabbworld, G tańszy ruch (aabbworld + gram), H bez `unindexed`
(porządek), I shadery bez podwójnego `@builtin(position)` (Metal). Integrator po kolei: C, I, H,
A wobec B, D, E, F, G; pomiar szeregowy na bezczynnej maszynie, baza = `perf/wave-3` przed
częścią; ulepszenie przyjęte, gdy mediana ≥3% lepsza w reżimie celu i zakresy biegów się nie
nakładają, a żaden inny reżim nie gorszy; porządek i poprawność na testach. astar, ceremony,
aabbworld: gałęzie `perf/wave-3`, PR-y integratora bez scalania; użytkownik scala, taguje i
podbija w gram; do tego czasu `/home/karol/CODE/go.work` używa lokalnych `./astar` i
`./aabbworld` (przed falą: tylko `./ceremony`, `./goke`, `./gram`).

## Fala 3: wynik (2026-10-21)

Dziewięć części naraz bez pomiarów czasu (agenty Opus 5.5; w połowie wstrzymane dema i sondy,
bo obciążenie doszło do 60), potem integrator po kolei: weryfikacja, pomiar szeregowo na
bezczynnej maszynie (5 rund, baza = gałąź integracji przed częścią), sprawdzenia deterministyczne
(sondy, zrzuty), scalenie albo odrzucenie. Reguła: ulepszenie przyjęte, gdy mediana ≥3% lepsza w
reżimie celu i zakresy biegów się nie nakładają, a żaden reżim nie gorszy; poprawność i porządek
na testach.

| Część | Wynik | Liczby |
|:--|:--|:--|
| C osiadanie po `Sync` poza zegarem | przyjęta (poprawność) | test odtwarza lukę; C+H+I: tick świata i nawigacji w szumie (open 1 000 +2,8%, pod progiem) |
| I shadery na Metal | przyjęta (poprawność) | test pada na bazie na trzech plikach; zrzuty 4 scen 0 pikseli różnicy, próby kontrolne różne |
| H bez `unindexed` | przyjęta (porządek) | logi sond spawn i bullet identyczne, x1 i x4 |
| A odpowiedzi wyszukiwań trzymane | przyjęta | tłok 1 000 94,9 → 40,3 ms, tłok 100 −24%, open 1 000 −24%, open 100 −7%; sondy square i spawn identyczne tick w tick, zrzuty 0 pikseli |
| B odstęp „dosłowny” | odrzucona (zmiana zachowania) | tłok 1 000 17,5 ms sama, 10,4 ms na A; po 21 s 247 jednostek do celu zamiast 539: decyzja użytkownika |
| D słowniki astar ze znacznikiem | przyjęta | nawigacja −5 do −7%, trasa −4,6%. Po przeglądzie PR astar znacznik to bajt w osobnej tablicy: 9 B na slot zamiast 16 B (`{node, uint32}` przez wyrównanie); astar wobec `main` waga 0,1 −1 do −59%, wagi 1,0 i 0,5 −4 do +5%, żaden przypadek gorszy wg reguły (16 B: do +10%); gram `Navigation_Tick` i `Navigation_Path` jak przy 16 B (szum) |
| E kroki ceremony bez alokacji | przyjęta (po decyzji użytkownika) | alokacje open 1 000 2 599 → 90 na tick, czas ±2%; sondy guard, dialog, spawn identyczne |
| F `Scan` bez alokacji, niestabilny test | przyjęta (po decyzji użytkownika) | torus 3 → 0 alokacji, czas −2%; test 40 razy bez porażki |
| G ruch przebiegu jednym wywołaniem | przyjęta | `World_Tick` 1 000 / 5 000 −24 / −26%; skróty pozycji w 4 demach identyczne |

**Decyzja użytkownika (2026-10-21, po przeglądzie):** mniej alokacji przy czasie nie gorszym to
też ulepszenie („pewnie oddadzą nam ten czas z nawiązką później, gdy włączy się GC”); E i część
alokacyjna F przyjęte.

**Stan.** Gałęzie `perf/wave-3`: gram (C, I, H, A, D, G), astar (D), aabbworld (G, F), ceremony (E). PR-y integratora bez scalania; użytkownik scala, taguje i podbija w
gram. Do tego czasu `/home/karol/CODE/go.work` używa `./astar` i `./aabbworld` (przed falą:
`./ceremony`, `./goke`, `./gram`). Gałąź odrzucona zostaje: `n/detour-wait`.

## Fala 4 (plan 2026-10-21, ta sama gałąź `perf/wave-3`)

Na życzenie użytkownika: pozostałe pomysły alokacyjne, brief `doc/briefs/w4.md`: E2 trzy kroki
ceremony, D2 ścieżka astar do bufora (astar + gram), K boksowanie uniformów i `DrawImageIn` w
gram. Ta sama procedura; reguła akceptacji rozszerzona decyzją użytkownika: mniej alokacji przy
czasie nie gorszym to ulepszenie. Przyjęte części wchodzą do tych samych PR-ów (`perf/wave-3`).

## Fala 4: wynik (2026-10-21)

Trzy części, ta sama procedura, reguła: czas nie gorszy i mniej alokacji, albo czas lepszy.
Wszystkie trzy przyjęte, w tych samych PR-ach.

| Część | Czas | Alokacje | Sprawdzenia |
|:--|:--|:--|:--|
| D2 ścieżka astar do bufora (`SolveInto`) | nawigacja i trasa w szumie | tłok 1 000 1 406 → 13 na tick, open 1 000 90 → 8, trasa 16 → 4; astar `Solve` 11–18 → 1–4 | golden tras, `-race` |
| E2 kroki ceremony na drugiej encji, na miejscach, warunek drugiej | benchmarki reguł, planów, efektów w szumie (te kroki odpalają tylko dema) | 2 → 0 na odpalenie | sonda effect: log i zrzut identyczne; bullet z ziarnem: skróty identyczne |
| K uniformy pakowane raz, blity bez bufora | klatka wyspy, UI, tekst, `World_Draw`, cienie w szumie | wyspa −1 na klatkę; niebo −28, aloft −22, krople −15, plansza −23 (licznik agenta) | zrzuty light (różni tylko tekst czasu w telemetrii) i hex identyczne; sky i flat w szumie bazy |

Uwaga: `render/gpu` `TestDevice_ARedrawnFrameAllocatesNothingOfItsOwn` padł raz w pełnym `go test
./...` (8 alokacji), a 10 powtórzeń osobno na bazie, na K i po scaleniu oraz dwa kolejne pełne
przebiegi przeszły: test liczy alokacje całego procesu, więc pod obciążeniem bywa niestabilny.

## Fala 5 (plan 2026-10-22, ta sama gałąź `perf/wave-3`): pula wątków, Read/Write, routing i traffic

**Cel.** Wyszukiwania tras na kilku rdzeniach. Pierwszy szkic (wstępne wypełnianie pamięci
Detour przed pętlą i `SeekTo` w goke) odrzucony 2026-10-22; z niego zostało `Query.Fork` (goke
v3.6.0).

**Układ.**
- Pula (`internal/parallel`): stałe gorutyny przez całą grę, wołający jako wątek 0; `Begin(task,
  idle)`, `Add(n)`, `Wait`; zadania z licznika atomowego, bez alokacji; `idle(w)` raz na wątku,
  który nie znajduje zadania. Rozmiar z `game.Props.Workers`, pluginom przez
  `plugin.Installer.Pool`.
- Każdy plugin wydaje swój stan przez `Read(w)`, bezpieczny na wątku `w` (forki zapytań, bufory
  na wątek, żadnych leniwych zapisów przy odczycie), i `Write()` na wątku gry.
- Nawigacja to `routing` (polecenia, prośby o trasę, liczone na puli) i `traffic` (jazda po
  trasie, zajętość, tłum, szeregowo).

**Etapy** (po każdym raport i akceptacja użytkownika):
1. Pula; vision i renderer planszy na niej.
2. `Read`/`Write`: 2a plansza, 2b świat (aabbworld `SpaceRead`, ceremony `EffectsRead`), 2c
   pozostałe pluginy.
3. 3a: podział nawigacji na pakiety, zachowanie identyczne. 3b: prośby o trasę w `MoveOrder`
   (`Ask`), odpowiadane w tym samym kroku (przebieg 1 `traffic` zbiera prośby, routing odpowiada,
   przebieg 2 kończy krok tych jednostek); cele grupy z pierścieni od celu, bez A*; osobne
   `Finder`y routingu i traffic.
4. Routing na puli: K routerów (`WithRouters`, domyślnie 4), każdy z własnym `Finder`em;
   paczka ma min(K, prośby, P) pasów, pierwszy router przesuwa się co tick; pas na wątku `w`
   czyta planszę przez `Read(w)` i zapisuje wynik przez fork zapytania tego wątku; wspólna
   pamięć odpowiedzi ze stanem „w toku” (`spacing.Answers`).
5. astar `Solver.Wipe` (v1.1.3): wątek bez zadań czyści znaczniki routerów nieużytych w ticku,
   gdy zostało im mniej niż 64 wyszukiwania do zawinięcia.
6. Dokumenty.

## Fala 5: wynik

Pomiary naprzemiennie, baza to `perf/wave-3` przed etapem; etapy 1–3a po 5 albo 2 rundy, od 3b
po jednym przebiegu.

| Etap | Zmiana | Wynik |
|:--|:--|:--|
| 1 | pula | `Vision_Scan` 9 → 0 alokacji na tick, `Vision_ScanWithOutlines` 100 −7,7%, reszta w szumie |
| 2a | plansza | w szumie; regresja `Board_Altitude` (+9–11%, grunt liczony dwa razy na encję) usunięta przed scaleniem |
| 2b | świat, aabbworld, ceremony | w szumie: indeks przestrzeni budowany przed paczką vision (`SpaceRead.Ready`, bez tego `Vision_Scan` 500 +78%), `ready()` w aabbworld inline'owane |
| 2c | pozostałe pluginy | w szumie |
| 3a | pakiety nawigacji | open 1000 −3,3%, reszta w szumie; logi sond identyczne |
| 3b | prośby w tym samym kroku, cele grupy bez A* | zachowanie inne od pierwszego rozkazu grupy; crowded 1000 przy `12x` 38,2 → 8,3 ms (okno trafia na szczyt bazy), na 600 tickach 12,8 → 8,7 ms; `Navigation_Path` 14,9 → 0,7 ms, bo mierzy już pamięć odpowiedzi, nie A* |
| — | zajętość bez alokacji na krok | open 1000 (`400x`) 20 → 14 alokacji na tick, czas bez zmian |
| 4 | routing na puli | logi sond identyczne z routingiem szeregowym; crowded 1000 8,27 → 8,45 ms (w oknie mało próśb) |
| 5 | czyszczenie znaczników | crowded 1000 8,78 → 9,00 ms, maszyna obciążona, w szumie |

**Otwarte.**
- Zmierzone 2026-10-23 samym routingiem (`plugins/navigation`, `Routing_Path`,
  `Routing_Routes`, każde wyszukiwanie od nowa): A* trasy przez planszę 200×200 15,2 ms, tyle co
  `Navigation_Path` przed falą 5; 32 trasy naraz 256 ms na jednym wątku, 77,8 ms na puli (4
  routery, 3,3×). Tablice routera powstają przy jego pierwszym wyszukiwaniu, potem 0 alokacji.
- `Navigation_Path` w `bench` zostaje: mierzy cały tick z pamięcią odpowiedzi, A* mierzy `Routing_Path`.
- Na później: wielu piszących do `Space` (wkład na wątek, sklejenie przez koordynatora, jedno
  `RebuildRuns`); płaska plansza drogą GPU topografii (bez składania kafli w rendererze planszy).

## Płaska plansza na GPU (2026-10-23, gałąź `perf/flat-gpu`)

Rozpoznanie: droga „płaska plansza przez topografię z zerową rzeźbą” odrzucona (sprite'y pieczone
najwyżej 16 px na komórkę, bez materiałów i okryć, panika na zawijanym świecie, hex słabszym
potokiem, niezerowe `Heights` zmienia footing, mosty, światło i A*). Zamiast niej mapa kafli w
shaderze renderera planszy: CPU trzyma obraz komórek (blok 2×2 piksele na komórkę) i tablicę
prostokątów atlasu, przy zmianie komórki przepisuje jej blok i sąsiadów.

| Etap | Zmiana | Commit |
|:--|:--|:--|
| 0 | `Benchmark_Frame_Flat` (1440p, odczyt co klatkę), zrzuty wzorcowe 5 dem | `af067e6a` |
| 1 | usunięta martwa ścieżka równoległa (`look.Parallel`, `ParallelLook`, `Workers`, `UsePool`, `Reads`, `board.WithWorkers`) | `af067e6a` |
| 2–3 | mapa kafli: sprite'y, światło per piksel, mgła, głębia ziemi, drogi, okrycia, kołysanie, materiały (`look.Laid`, `look.Swaying`) | `212ba395` |
| 4 | sektory widoku na ich poziomach, także siatka | `6d3a02ad` |
| 5 | renderer rysuje tylko mapą kafli; usunięte `Still` planszy, `evenLight`, `look/shade.go`; drogi i okrycia jako instancje | `7142fcc2` |
| 6 | dokumenty | — |

Pomiar (jeden przebieg, z odczytem ~10,5 ms): z góry 11,5 → 13,4 ms, z góry ze zmianą komórki co
klatkę 17,1 → 13,7 ms (4,8 MB → 55 KB na klatkę), izo 17,6 → 13,8 ms. Drogi liczone per piksel
(każdy piksel szuka pasów wokół) kosztowały ~15 ms więcej; jako instancje mieszczą się w liczbach
wyżej.

Odstępstwa od planu: składanie CPU zostaje dla minimapy planszy (`render.Still`), wierzchów heksów
topografii (`FlatLook` + `Still`, malarz bez odpowiednika na GPU) i kamer bez promieni, więc
`render.Still`, `look.FlatLook` i `draw.Bands`/`Covers` zostają. Na heksach kolejność nakładania
okryć i kołysania na sąsiadów jest przybliżona (24 piksele różnicy na planszy testowej).

### Płaska plansza na GPU: dokończenie (część A, 2026-10-23)

| Etap | Zmiana | Commit |
|:--|:--|:--|
| A1 | materiały rysowane raz (bez kawałków CPU co klatkę), okrycia każdej komórki zaraz po jej kaflu w pełnym przebiegu (heksy, kołysanie) | `126f2b7e` |
| A2 | `look.Renderer.Whole`: cała plansza z góry do obrazu; minimapa planszy przez nią zamiast `Still` | `a40c0fba` |
| A3 | wierzchy heksów topografii: kafle przez `Whole`, połysk stojącej wody (`look.Overlaid`), drogi malarza jako instancje GPU (`lanes.wgsl`) | `a71b4c94` |
| A4 | usunięte składanie kafli na CPU: `compose`, `look.Tile`, `FlatLook`, `EvenLit`, metody CPU Dressingu, `render.Still`, `gpu.Kept`, `draw.Weigh/Quarters`; testy porównawcze zamienione na sprawdzanie kolorów w punktach | `a255183f` |
| A5 | dokumenty | — |

Zgodność sprawdzona póki droga CPU istniała: heksy z okryciami i drogami co do piksela poza
krawędzią widoku (tam CPU gubił komórkę), kołysanie z okryciami 0 pikseli, `Whole` wobec `Still`
0 pikseli, wierzchy heksów malarza 3 piksele na krawędzi pudełka. Dema (przyspieszone, raz):
navigation/hex i rules/effect identyczne, board/atlas i look/light różnią się tylko panelem
telemetrii. Pomiar raz, naprzemiennie z `7142fcc2`: z góry +0,7 ms, izo +0,7 ms, zmiana komórki
bez różnicy.

Wypiek malarza na planszy kwadratowej (`bake.go`, `render.Paint`) przeszedł na GPU w części C
niżej.

### Wypiek malarza na GPU (część C, 2026-10-23)

Wariant (b): CPU zapisuje tylko dane komórek, kształty liczą shadery.

| Etap | Zmiana | Commit |
|:--|:--|:--|
| C1 | komórki z nakładką materiału nigdy „proste”: bierze je pełny przebieg (uniform `TileFull`), `overlayOf` zniknął z przebiegu prostego | `1ceec73e` |
| C2 | mapa komórek malarza (`painter/cells.go`): blok 4×2 piksele na komórkę (sprite, flagi, `Spread`, połysk, przepływ, detal, droga, wysokości rogów), obraz prostokątów sprite'ów; zapis zmienionych komórek albo całości | `3fc31b88` |
| C3 | albedo bez dróg na GPU (`paint_ground.wgsl`): baza, przejścia ćwiartkami z wagami po dwóch trójkątach | `4c9cf2d2` |
| C4 | drogi na GPU (`paint_ways.wgsl`): kawałki budowane w shaderze wierzchołków (pień, dołączenia, łuki 6/4/2, zanik, mieszanie, wejście pod wodę, maska lądu, prędkość wody), do 46 kawałków; wierzchy heksów też z niego | `932aad8c` |
| C5 | warstwy wody (przepływ, połysk, ujście) i detale rodzajów na GPU, jeden `bake` maluje wszystkie | `53e14c7f` |
| C6 | brzegi na GPU (`paint_coast.wgsl`, `Painter.Shores`), teren kopiuje obraz do ćwiartki siatki; `Wet` i `CellDetails` z reguły | `b623c0d0` |
| C7 | usunięty wypiek CPU (kawałki, płótna, `render.Paint` w malarzu, szukanie brzegów, niebo malarza); testy sprawdzają piksele wypieczonych obrazów | `1e573806` |
| — | `Benchmark_Frame_Topography` czeka na GPU po pierwszej klatce | `6ee8d4f4` |
| C8 | dokumenty | — |

Zgodność sprawdzona, póki wypiek CPU istniał: albedo, woda, detale i brzegi planszy z morzem,
rzekami, drogami i detalami co do piksela, całość i po zmianie; drogi na wierzchach heksów 11
pikseli na 208 000. Bilans części C: −740 linii Go, +950 WGSL, +150 testów.

Pomiar (Intel UHD 620, bardzo słabe GPU — na typowym przeniesienie na GPU wypada lepiej; decyzja
użytkownika: zostaje). `Benchmark_Frame_Flat` po C1 naprzemiennie z `a255183f`: z góry
15,3 → 14,0 ms, zmiana komórki 15,5 → 14,2, izo 15,2 → 14,1. `Benchmark_Frame_Topography`
(poprawiony, 2 rundy naprzemiennie z `a255183f`): izo bez zmian (16,3/16,1 → 16,3/16,0 ms),
perspektywa 20,4/20,2 → 20,4/21,3, izo pod słońcem 18,0/17,9 → 19,8/18,2 — w rozrzucie maszyny.
Sam wypiek (sonda, mediany, CPU w `b623c0d0`): zmiana komórki +8,5 → +6 ms, cały wypiek
(pierwsza klatka, zmiana stylu, ponad 1/8 komórek do przemalowania) +25 → +110 ms.

Uwagi: naga źle kompilowała `counts[k] += 1` na tablicy lokalnej i funkcję zwracającą strukturę
w poszukiwaniu brzegu — oba miejsca przepisane bez tego. `CellDetails` i `Wet` liczone z reguły
są zachowawcze (kilka komórek więcej „ubranych”). Dema po C7: navigation/square i navigation/hex
identyczne z C6, board/topography i look/sky ta sama scena.

`look.Dressing.Sheet`, który we wszystkich implementacjach zwracał już sam atlas, usunięty w B1.

### Szybszy cały wypiek (B0, 2026-10-23)

Sonda (czas każdego przebiegu, wyspa 96×64, UHD 620): przebieg gruntu ~19 ms na warstwę × 5
(także ujścia, gdzie nic nie kładzie, i rodzajów bez detali), drogi ~1,3 ms na warstwę, CPU ~2 ms.

| Dźwignia | Zmiana | Commit |
|:--|:--|:--|
| — | `Benchmark_Painter_Bake` (wyspa jak w `bench/`: cały wypiek, komórka) | `9939c39a` |
| L1 | warstwa ujść bez przebiegu gruntu | `70133303` |
| L2 | warstwa rodzajów tylko, gdy jakiś rodzaj ma `Detail` | `e7a91e6a` |
| L3 | komórki 3×3, baza, „top”, maska przejść i przepływ w rogach liczone w wierzchołkach (raz na komórkę), piksele tylko wagi ćwiartek i sprite'y: warstwa 19 → 4 ms | `2faab01e` |
| L4 | drogi rysowane w `6 × najwięcej kawałków` wierzchołkach (`piecesOf`, reguła `pieceOf`) zamiast 276: 1,3 → 0,6 ms na warstwę | `88c65be2` |

Każda dźwignia sprawdzona na obrazach wypieku (wyspa, plansza z detalami, plansza dróg; całość i
po zmianie): 0 pikseli różnicy; drogi z liczbą z Go = drogi w 46 kawałkach (test). Pomiar 2
rundy naprzemiennie z `9939c39a`: cały wypiek 111 / 153 → 22,8 / 24,8 ms, komórka
6,5 / 9,4 → 7,1 / 6,4 ms (tu rządzi część CPU i małe przebiegi). Demo board/topography: ta sama
scena. Niezrobione z L4: wczesne wyjście wierzchołka z nadmiarowym kawałkiem w shaderze (drogi to
już ~10% wypieku, zysk ≲1 ms).

## aabbworld v1.17.0: liczby świata w float32 (2026-10-25, gałąź `feat/aabbworld-float32`)

To, co „Co nie wchodzi” odkładało („Zmiana `Base` na float32”), przyszło z aabbworld v1.17.0:
geometria w float32, pudełko 32 B. W gram każda liczba geometrii świata i ekranu jest float32
(reguła w `CLAUDE.md`), także wysokości, z których buduje się mapę; float64 zostaje w kosztach,
ułamkach czasu, wartościach ustawień i w rachunkach sumujących (osłona planszy i profil reliefu
wzdłuż promienia, odpływ wody, sztywnik kamery ortho). `world.Base` 104 → 52 B; rekord publikacji
trzyma pudełko, kierunek, prędkość i `Z` bez konwersji. Pomiary (baza i drzewo na przemian,
niższy z dwóch przebiegów): tick świata 5 000 encji 43,6 → 37,3 µs, kolizje 4–14% mniej,
`Sectors_Expanse` 13% mniej, publikacja 7–12% mniej, nawigacja 3–6% mniej, wzrok 0–5% więcej.
Zapisy sprzed zmiany się nie wczytują (goke odrzuca inny rozmiar komponentu).

**Domknięcie (2026-10-25).** Po migracji padały dwa testy nawigacji. Tłum (cztery stojące
jednostki stykające się przez 99 ticków): solver aabbworld dzielił nałożenie o krok float32 po
połowie, a połowa tak małej liczby nie ruszała żadnego pudełka, więc nałożenie krążyło po słupku co
tick; poprawka w aabbworld (kjkrol/aabbworld#23: styk w granicy zaokrąglenia to nie penetracja).
Pas bez obejścia (blokujący dopychany 6,52 przy progu 6): pięć przyczyn w gram, każda osobno —
masa domyślna to pole pudełka (`DefaultMass` znika), impuls obniża prędkość steeringu
(`Course.Speed`) w tej samej proporcji, także od ścian, normalna kontaktu spoczynkowego to
ściana, a nie linia między środkami, nawigacja nie wjeżdża w nikogo, kogo dotyka
(`bodies.Member.Touching`, `clearOf` w `BodyKeeping.Steer`), i rozsunięcie pary dzielone
według wagi przez udział, który handler podaje aabbworld w `Touch` (zmiana łamiąca, aabbworld
dalej nie zna masy). Wynik: pas 6,52 → 0,09; tłum bez reguł zatrzymany na 1,07 (dwie równe
wagi); w board/topography 36 jednostek w jedno miejsce 1800 → 790 kontaktów pchających w 100 s,
stojące przesunięte 14,2 → 9,0. Koszt: `Collision_Tick` 1–4% więcej (impuls czyta `Course`,
udział liczony na parę), `Navigation_Tick` bez zmian. aabbworld v1.18.0 w `go.mod` (tag
2026-10-25). Otwarte: stojący
tej samej wagi dalej bierze połowę nałożenia z każdego uderzenia; zerowanie tego wymagałoby
kotwicy dla stojącego na hamulcu (strona statyczna, gdy `WantSpeed == 0`).

## SIMD w widzeniu (plan 2026-10-26, gałąź `perf/sight-simd` w gram i w aabbworld, litera `s`)

Analiza (2026-10-25): SIMD pracuje tylko w sweepie par kolizji, bo jego pętla to czysta
arytmetyka na pudełkach leżących obok siebie, bez callbacków. Culling kamer (`view.System`,
`Space.Query`) to ok. 2% ticku i jeden test bitu na encję przy publikacji: SIMD niczego tam nie
da. Skan wzroku (`raycast.Scan`) to gather (Query obszarów, per kandydat klin, odległość i dwa
`atan2`), sweep kątów (per kąt slab `hitDistance` dla aktywnych kandydatów) i z wysokościami
profil gruntu per promień przez callback do gram (`relief.Along`): w topography 1,66 ms na tick
przy 37 obserwatorach, z czego relief 53% (gram), raycast 24%, osłona 14–18% (gram). Grunt nie
zwektoryzuje się przez callback: potrzebuje danych po stronie aabbworld. `simd/archsimd` nie ma
instrukcji gather, więc lockstep promieni po polu wysokości ładuje narożniki skalarnie.

Decyzje użytkownika (2026-10-26): cztery części naraz; slab we `Float32x8` (głębokości w
tolerancji ok. 1e-3 promienia, ten sam zbiór encji).

**Preludium** (integrator, w obu repo): aabbworld `perf/sight-simd` od `main` v1.18.0:
`Cone.Heights *HeightField` (narożniki, Cols×Rows, Cell, WrapX/Y; komórka to dwa trójkąty
złożone wzdłuż przekątnej o mniejszej różnicy wysokości, poza siatką 0), `Cone.Bend` (krzywizna:
grunt opada o Bend·d²), `QueryableSpace.QueryCone` z adapterem `sight` odpowiadającym kołem wokół
stożka. gram `perf/sight-simd` od `main` v0.5.0: ten plan i briefy `doc/briefs/s*.md`.

**Części** (agenci Opus 5.5, worktree `/home/karol/worktrees/<repo>/<część>`, gałąź
`s/<część>` od preludium; pomiary tylko integratora):
- **A `s/sweep`** (aabbworld, brief `s1`): profil skanu (gather, sweep, osłona) i `castFlat`
  po 8 kątów naraz na AVX2 między eventami aktywnych kandydatów: slab w SIMD do scratch,
  porządek przecięć i osłona skalarnie; fallback skalarny; test równoważności jak
  `TestPairSweep_WideFindsWhatNarrowFinds`.
- **B `s/cone-query`** (aabbworld, brief `s2`): kolumny float32 kawałków budowane z komórkami
  w `list()`, `Grid.QueryCone` ścisły (promień, klin, komórka właściciela) po 8 kawałków, gather
  wzroku na `QueryCone`, `atan2` tylko dla przeżywających.
- **C `s/heights`** (aabbworld i gram, brief `s3`): profil z `HeightField` liczony w aabbworld
  bez callbacków (krawędzie i fałdy komórek jak `relief.Along`, `Bend` zamiast `sunkGround`),
  gram podaje `Lattice()` reliefu; test krzyżowy Profile vs Heights na tym samym reliefie;
  lockstep ×8 promieni tylko jeśli test dymny pokaże zysk bez gatherów.
- **D `s/watched`** (gram, brief `s4`): skan tylko czytanych obserwatorów: reguły `Sighting`
  roli, `SightOutline`, znacznik `vision.Watched{}`; `WithScanAll()` przywraca dziś; `Sighted`
  nieskanowanego czyszczone raz.

**Scalanie i pomiar** (integrator): aabbworld A, B, C na `perf/sight-simd`; gram C, D. Baza
`main` obu repo (aabbworld v1.18.0, gram v0.5.0) i drzewo na przemian, bezczynna maszyna:
aabbworld `View_Visible`, `View_Both`, `View_Elevated`, `View_Cover`, `BroadPhase`; gram
`Vision_Scan`, `Vision_Relief` (4 reżimy), `Views_Copy`, `World_Tick`; sonda gram-probe
board/topography (37 obserwatorów, `GRAM_FPS_LOG`), dema vision pod `timeout`. Po akceptacji
użytkownika squash commit w obu repo.

## SIMD w widzeniu: wynik (2026-10-26)

Cztery części poszły równolegle u agentów Opus 5.5 (A `s/sweep`, B `s/cone-query`, C
`s/heights`, D `s/watched`), bez pomiarów po ich stronie; scalone na `perf/sight-simd` w obu repo
(aabbworld A, C, B; gram C, D), jeden konflikt (dwa `export_test.go`), pełna sekwencja zielona w
obu buildach, `-race` czysty, siedem dem wzroku chodzi. Pomiar: baza (aabbworld v1.18.0, gram
v0.5.0) i drzewo na przemian, goke i ceremony z cache po obu stronach, najniższy z serii.

**aabbworld** (`BENCHMARKS.md` „Sight in SIMD"): skan pyta siatkę o pudełka w zasięgu stożka
(`Grid.QueryCone`, po osiem kawałków na kolumnach float32 budowanych z komórkami), zbieranie
kandydatów było połową skanu; przed sweepem kładzie slaby każdego kandydata dla wszystkich jego
kątów, cztery promienie naraz we float64. `View_Visible` 1 000 encji 29,0 → 19,0 µs (build
zwykły 27,3 → 17,8: ścisły filtr stożka daje prawie tyle samo bez SIMD), `View_Both` 33,2 →
23,2, `View_Elevated` raster 113 → 104, osłona z encjami 52,5 → 37,8; `BroadPhase` i
`Grid_RebuildPairsQuery` w szumie. Odstępstwo od decyzji: slab we float32×8 zmieniał zbiór
widzianych encji (ściana pudełka na linii komórki, remis odległości rozstrzygany odwrotnie), a
pomiar pokazał, że osiem pasów float32 daje wobec czterech float64 ±1%, więc zostaje float64×4,
bit w bit ze skalarem. Profil przed zmianą: slab to 4–10% skanu, gather 45–55%, promienie
(`cos`+`sin`) 16–27%, sortowanie kątów 10–16%; po: jeden `math.Sincos` na promień w obu
buildach.

**Grunt jako dane** (C): `Cone.Heights` z narożnikami reliefu (`Lattice()`, bez kopii) i `Bend`
zamiast callbacków `Along`/`Ground`/`sunkGround`; profil liczony w aabbworld daje te same punkty
co `relief.Along` (oracle, 2 000 losowych pól; 60 scen i prawdziwy relief topography bit w bit).
`Vision_Relief/profile` 1,73 → 1,54 ms, 15 Hz 761 → 660 µs; `along` (stara droga na drzewie)
1,81 ms. Lockstep ×8 promieni po polu niezrobiony: `lattice.walk` to 52% ticku `profile` (68%
szczebla `field` w aabbworld), z czego arytmetyka (`folded`, `point`, `clamp01`) ok. 25% ticku,
a loady narożników i appendy (`crossedX/Y`, `take`) ok. 14%; bez gatherów w `archsimd` zysk to
rząd 25% × (1 − 1/1,75) ≈ 10% ticku za przepisanie `walk` z części A. Do decyzji osobno.

**Tylko czytani** (D): skan dla obserwatora z regułą `Sighting` jego roli, `SightOutline` albo
`vision.Watched{}`; `WithScanAll()` skanuje wszystkich; `Sighted` nieczytanego czyszczony raz.
Decyzje użytkownika (2026-10-26): model zostaje domyślny; log (`WithLog`) nie jest czytelnikiem,
bo to telemetria (navigation/square i hex nic już nie logują: nikt nie czyta ich zwiadowców);
agent pytał o dopasowanie reguły lustrzanym `PairRules` (kopia reguły ustawiająca flagę), bo
ceremony v0.3.1 nie ma takiego pytania, więc integrator dodał w ceremony
`PairRules.For(marks)` (gałąź `feat/pair-rules-for`, `replace` w go.mod gram do taga) i lustro
znikło. W board/topography 37 skanów reliefu na tick znika (nikt ich nie czyta; widoki na GPU
skanu nie czytają). Mapa czytelników dem w briefie `s4`: rules/vision i rules/dialog przez
reguły, board/flat przez outline. `vision.NewScanSystem` (eksportowane, nieużywane) usunięte.

**gram razem** (bezczynna maszyna, 6 przebiegów, najniższy): `Vision_Scan` 100 obs. 79 → 73 µs
(szeregowo 129 → 106), 500 obs. 295 → 249 (596 → 484); z outline 2–11% mniej; `World_Tick` i
`Board_Terrain` w szumie. `Vision_Relief/profile` 1,76 → 1,21 ms, 15 Hz 774 → 495 µs (z czego E:
1,43 → 1,17 i 637 → 487). **Regresja wyjaśniona**: grunt próbkowany callbackami (`step=50m` 6,60 →
6,99 ms, `step=cell` 1,41 → 1,44) jest o 2–6% wolniejszy tylko w buildzie SIMD ze skanem
równoległym; na jednym CPU (`-cpu=1`) jest o 2–4% szybszy, w buildzie zwykłym w granicach 2%, a
profil CPU bazy i drzewa jest niemal identyczny: jądra AVX2 (slaby, filtr stożka) na wszystkich
wątkach obniżają zegar dla skalarnych callbacków, które w tych reżimach stanowią ponad 90% pracy.
Nie jest to praca do odjęcia; ewentualne wyjście to jądra 128-bitowe (bez licencji AVX2), wpis w
roadmap. Topography tej ścieżki już nie używa (ma siatkę); dotyczy plansz hex i gruntów własnych
gry.

**Porządki integratora**: martwy kod po B (`searchAreas`, `axisSpans`, `splitAxis`, `clampRect`,
`wedge`, `boxDistance`, `candidate.dist`, `QueryableSpace.Query`) usunięty; komentarz
`QueryCone` doprecyzowany (kopie przez szew mogą się powtórzyć); publiczne `Space.QueryCone` i
`SpaceRead.QueryCone`; `go.work` w `.gitignore` gram (regułę `!/*.*` przebijało); dokumenty obu
repo. Zależności: `replace` w go.mod gram na `../aabbworld` (`perf/sight-simd`) i `../ceremony`
(`feat/pair-rules-for`) do czasu tagów (decyzja użytkownika); CI czerwone do tego czasu.

**Część E `s/lockstep`** (brief `s5`, decyzja użytkownika 2026-10-26): `walk` kładzie grunt bloku do
czterech kolejnych kątów przed rzutem, promień na pas `Float64x4`; DDA, fałda, punkt wyjścia i
reguły `take` wektorowo z maskami, narożniki ładowane per pas rundę wcześniej (archsimd nie ma
gather), punkty zbierane per pas po rundzie; `GOAMD64=v3` łączy w skalarze `a + b·c` w FMA, więc
sonda `fused` wykrywa to przy starcie i jądro używa `MulAdd` w tych samych czterech miejscach.
Punkty każdego promienia bit w bit jak skalarnie (4 000 losowych przypadków, 10 mutacji złapanych),
pasy zajęte w 95–99% rund. Wynik: `View_Elevated/heights=field` 1 000 encji 242 → 173 µs (−28%), 100
encji 84 → 59 (−30%), szczebel `profile` bez zmian (kontrola); gram `Vision_Relief/profile` 1,43 →
1,17 ms (−18%), 15 Hz 637 → 487 (−24%), `along` bez zmian.

## Udziały tempa: wycena kroków, astar na tablicach (plan 2026-10-28, gałęzie `perf/arrays` w astar i `feat/pace` w gram)

Analiza (2026-10-28): 36 jednostek z płaskowyżu posłanych na Pico do Arieiro dzieliło się i
zacinało; zamiast diagnozy użytkownik wybrał przebudowę wyceny kroków, bo była rozproszona i
nieprecyzyjna. Zakres wag był nigdzie niezdefiniowany (jedno zdanie w komentarzu
`cell.Kind.Cost`), zero znaczyło co innego w plannerze, w tempie i w `Costs[i]`, nic nie pilnowało
zakresu; przeliczniki koszt × długość × stok były zaszyte w plannerze (`pathfind/finder.go`
`price`) i w tempie (`moments/standing.go` `share`), a krzywa stoku siedziała w topografii
(`relief.Climbing`); zejście było szybsze niż marsz po płaskim (czynnik 0,7, `Pace.Share` 1,43),
więc `MaxSpeed` nie było maksimum, a heurystyka A* była luźna o 30% wszędzie. astar nie
dokumentował kontraktu i trzymał węzły na wskaźnikach.

Decyzje użytkownika (2026-10-28): waga przejścia to udział tempa maksymalnego w [0, 1] (0 odmowa,
1 pełne tempo, nic szybszego; neutralne 1, czynniki się mnożą, bez normalizacji po liczbie
czynników); kontrakt `cell.Mobility{Pace(Step)}` w `board/cell`, biblioteka zbiera wymiary,
strategia składa; domyślna `cell.Hiking` (Tobler obcięty na płaskim); podanie obok motion
(`WithMotion(m, mobility)`); kafle mówią udziałami; dema hex i square jawnie domyślna,
topografia własna `madeira` z odcięciami 1:1 i 2:1 w obie strony; stare zapisy bez obsługi;
astar: układ danych, potem SIMD z bramką pomiaru; wszystko w jednej sesji, po kolei; HPA* i
szukanie dwukierunkowe poza etapem.

**Jedna miara stoku.** Planner wycenia krok po rise/run między środkami komórek; tempo liczy
stok tak samo (komórka pod środkiem jednostki i sąsiad w kierunku marszu, `grid.Nearest` +
`Toward`), nie w punkcie: co planner dopuścił, tempo dopuszcza, więc odcięcie nie zatrzyma
jednostki w połowie kroku. `Map.SlopeAt` i `relief.SlopeAt` znikają; zostaje `Map.Rise`.

**astar** (`perf/arrays`): golden ścieżek sześciu scen z v1.1.3, testy optymalności (Dijkstra),
ponownego otwarcia, kroków zerowych, `Iter`; węzły w tablicach po `int32` (bez wskaźników),
kopiec kluczy i id pisany ręcznie z sitem z dziurą (porównania jak `container/heap`: te same
remisy), jeden słownik (zamknięty jako `^id`); kopiec 4-arny z wyborem dziecka w SIMD
(`GOEXPERIMENT=simd`, AVX2) próbowany z bramką pomiaru; dokumentacja z kontraktem A* w jego
słowach, README z przykładem udziałów, `BENCHMARKS.md`, skill po polsku, CI z jobem SIMD na
Go 1.27.

**gram** (`feat/pace`): `cell.Mobility`, `Step`, `PaceOf`, `Hiking`/`DefaultHiking`; `Kind.Pace`,
`Paces`, `Paced`, `PaceFor`, `Floor.Pace`, `bridges.Spec.Pace`; `Define` odmawia rodzaju bez
udziału i udziału > 1; `Map.Rise` zamiast `Climb/Least/Slope`; `board.Read.Mobility/Pace/Flat/
Sloped`; planner: koszt = długość / udział, h = odległość, udział na płaskim raz na komórkę,
strategia na krok tylko nad reliefem (`kinds` po ordynale), `Steps` odmawia udziału 0,
`YieldShare` jako stosunek do udziału na płaskim; tempo przez `PaceOf` z rise do sąsiada;
jazda odmawia kroku o udziale 0 (pod grawitacją też); `topography.Config.Climbing` i pakiet
`topography/relief` usunięte; przebieg mechaniczny literałów (136 plików); `routes.golden`
przepisany; dema: `madeira` (bit `HighlandDomain` jednostek płaskowyżu), `hillside` w
custom-map, hex i square jawnie, effect odwrócony (śnieg 1/3, lód 1/2, wiedźma 1).

**Pomiar** (integrator): baza `main` `b374438a` (worktree w scratchpadzie, `GOWORK=off`: tagi,
astar v1.1.3) i drzewo (własny `go.work` z gram i astar, goke/ceremony/aabbworld z tagów), na
zmianę, `-count=2` × 3 rundy; `./bench`: `Navigation_Tick`, `Navigation_Path`, `Board_Altitude`,
`Gravity_Standing` (`BENCHTIME=200x`); `plugins/navigation`: `Routing_Path`, `Routing_Routes`;
`pathfind`: `PathFinder_Terrain` (`map`, `cells` i nowy `relief`: wzgórza po ordynale, strategia
na krok). Dema z jednostkami pod `timeout 20s`.

## Udziały tempa: wynik (2026-10-28)

Trzy części poszły w jednej sesji, po kolei (decyzja użytkownika), bez agentów i worktree: astar
na gałęzi `perf/arrays` od v1.1.3, gram na `feat/pace` od `main` v0.6.0. Sekwencja standardowa
zielona w obu buildach w obu repo, `-race` na board i navigation czysty, 18 dem chodzi pod
`timeout`. Pomiar: kopia bazy w scratchpadzie (`GOWORK=off`: tagi, astar v1.1.3) i drzewo
(własny `go.work` z gram i astar; goke, ceremony i aabbworld z tagów po obu stronach, bo lokalne
goke było brudne) na zmianę, bezczynna maszyna.

**astar** (`BENCHMARKS.md` astara; najniższy z sześciu, `-cpu=1`, build zwykły): pełne szukanie
(`CostWeight_1.0`, `IndexedSliceDict`) 256×256 11,7 → 6,72 ms, 1024×1024 208 → 123, 2048×2048
937 → 547 (−41…−43%; słowniki mapowe −43…−45%), krótkie (waga 0,1) −76…−89%, `SolveInto`
−40…−43% (1 → 0 alokacji), Octile −38…−40% (1024×1024 413 → 249 ms); `Solve` alokuje raz
(ścieżkę) zamiast dwa. Kroki osobno: układ danych −26…−31% (waga 0,1 −70…−85%), jeden słownik
−10…−14% (mapowe −31%), sito z dziurą −4…−8%; ścieżki co do kroku jak v1.1.3 (golden sześciu
scen; `routes.golden` grama przepisany tylko z powodu wyceny). **Odrzucone**: kopiec 4-arny
skalarny (+1…+2% na pełnych szukaniach) i z wyborem dziecka w SIMD (−10% na Octile), bo inny z
równie tanich wariantów trasy wychodzi pierwszy i w `Navigation_Tick/crowded,1000` grama
jednostki przeplanowały 55% więcej (7,2 → 11,2 ms na tick); zostaje kopiec binarny, pliki
4-arnego w scratchpadzie sesji.

**gram** (`BENCHMARKS.md` „Navigation", „The routing alone", „Standing"; baza `main` v0.6.0 z
astarem v1.1.3, drzewo z astarem v1.2.0, najniższy z trzech rund): `Navigation_Tick` crowded
1 000 8,83 → 7,93 ms (−10%), crowded 100 −11%, open 100 −8%, open 1 000 w szumie (tam rządzi
ruch po trasach); `Navigation_Path` −2,5%; `Routing_Path` 15,8 → 15,2 ms, `Routing_Routes` 255 →
211 ms na jednym wątku (−17%), 87,8 → 75,8 na puli (−14%); `PathFinder_Terrain` −20…−22% (cells
6,78 → 5,27 ms, `-cpu=1` tak samo), nowy szczebel `relief` ok. 9 ms: strategia na każdy krok
(`PaceOf`, `exp`) to niemal drugie tyle co samo szukanie — pole do HPA*, nie do SIMD. Tempo:
`Board_Altitude` i `Gravity_Standing` +9…+10% przy 1 000 piechurów na reliefie (367 → 404 µs,
ok. 40 ns na jednostkę na krok: `Map.Rise` do sąsiada w kierunku marszu i `exp` krzywej Toblera
tam, gdzie v0.6.0 czytało stok w punkcie z czynnikiem 1), w szumie przy 100; przyjęte jako cena
strategii. Pierwszy pomiar pokazał, że planner czytał łańcuch `Flat` na komórkę w każdym
szukaniu (+6% crowded) i że bramka ustępowania wołała `PaceOf` dwa razy na kandydata także nad
płaską planszą (+15% open): stąd udział na płaskim trzymany po wersji planszy (`flatStep`) i
skrót `Sloped()`.

**Pułapka pomiaru znaleziona**: linia loga świata (`maxEntities=… → bucket=…`) wchodzi między
nazwę wyniku a liczby, a `grep '^Benchmark'` w `measure.sh` gubił każdy wynik po pierwszym z
`-count=N`: „3 rundy × `-count=2`" grama było trzema próbkami, nie sześcioma (astar, bez loga,
miał sześć). `measure.sh` zapisuje teraz całe wyjście, `compare.py` przypisuje gołą linię liczb
ostatniej nazwie; liczby tego etapu są z trzech próbek na stronę, tempo zmierzone ponownie po
poprawce z sześciu.

**Odstępstwa i co dalej**: stare zapisy bez obsługi (decyzja); długie linie sprzed etapu
nietknięte; `bench-save` całego zestawu astara nienagrany (pomiary na zmianę zamiast niego);
`go.mod` grama czeka na tag astara v1.2.0 (API astara bez zmian i te same ścieżki, więc CI grama
na v1.1.3 zielone, tylko wolniejsze). Dalej: tag v1.2.0 (użytkownik) → `chore: astar v1.2.0`;
scalenie
`feat/pace` `--no-ff`; osobne planowanie HPA* i szukania dwukierunkowego
(`gram-perf-navigation`, „Co dalej"); zacięcie 36 jednostek na płaskowyżu sprawdzić na nowym
kodzie.

## Kolejka i pole grupy: zacięcie względem tempa, podążanie, jedno szukanie na rozkaz (plan 2026-10-28, gałęzie `fix/stall-pace`, `feat/trail`, `feat/field`)

Analiza (2026-10-28, v0.7.0, sondy headless board/topography): 30 górali posłanych z
płaskowyżu (66–71, 48–52) na Pico do Arieiro (115, 61) traci rozkazy na krawędzi płaskowyżu
(77–81, 53–56) między 50 a 77 s gry, nikt nie dochodzi; trasa jest znajdowana od razu (62 kroki,
`Ask 0`, `Fails 0`). Cztery przyczyny naraz: (1) keeping ciał uznaje za zacięcie brak postępu
ćwierci ciała (0,72 jednostki) na sekundę bezwzględnie, a prawne zejście z krawędzi (udział
0,049: skała × Tobler) to ~1,2 jednostki/s, więc jednostka idąca zgodnie z prawem jest
„zacięta", a pięć zacięć gasi rozkaz; (2) przebieg stania wycenia udział po sąsiedzie w
kierunku głowy, a w tłoku głowa obraca się ku stromym sąsiadom (udział 0,03–0,047), choć krok
trasy jest wyceniony na 0,28; (3) współtowarzysz rozkazu idący albo czekający przede mną daje
słowo `Blocks` (`Moving` znaczy „ma rozkaz") i `Detour`, obejście kogoś, kto się porusza, z
powrotem w grupę; (4) każda jednostka pyta o własną trasę (30 A*), każde `Bumped` szuka od
zera. Eksperyment z progiem 10× niższym: 4 liderów doszło, 26 straciło rozkazy w kolejce.
Pasy (jednostka widząca ramki i prędkości innych) były próbowane i porzucone na słowo
użytkownika; reguła domu zostaje: jednostka uczy się o innych uderzając.

Decyzje użytkownika (2026-10-28): schody, każdy stopień osobno mierzalny: (1) poprawka:
zacięcie względem prawnego tempa i tempo po stawianym kroku (`fix/stall-pace`, v0.7.1; pole
`unit.At.Next`, stary zapis nie wczyta się); (2) kolejka: słowo przechodnie `WalksMyWay`
(„idzie przede mną w moją stronę albo czeka w kolejce za kimś, kto idzie"), komenda `Trail`,
reguła domyślna dla współtowarzysza (`feat/trail`); (3) plan grupy: pole przepływu
(`pathfind.Field`, Dijkstra od celu z przejściami w lustrze, marsz po polu zamiast szukania,
`spacing.Fields` w koordynatorze; `feat/field`); (4) kolumna nad polem jako formacja z
porządkiem z kosztu do celu — własny plan po etapie 3; formacje kształtne poza planem. Stok po
prowadzonym kroku tylko w ćwierci obrotu od głowy. Wykonanie w jednej sesji, po kolei; nazwy
jak w planie sesji.

**Etap 1** (`fix/stall-pace`): `bodies.Member` niesie `Share` (z opcjonalnej kolumny
`steering.Pace` zapytania traffic, 1 bez niej) i `MaxSpeed`; `BodyKeeping.Watch` żąda postępu
`min(ćwierć ciała, MaxSpeed·Share·StallAfter/4)` (na otwartym gruncie bez zmian: czapka), udział
0 nie liczy; keeping komórek bez zmian (odmowa to komórka trzymana, nie powolność).
`unit.At{Cell, Next}`: `Next` pisze traffic (`head` zeruje, `tail` wpisuje punkt trasy po
rezerwacji i następny krok po `Index++`, `arrive`, ręka, `moveTo`), jazda (`open` oddaje
komórkę przed; `Next` przy jeździe naprzód i w tył, zerowany przy zdjęciu ręki) i zajętość
(pisarz zapasowy); przebieg stania wycenia stok `rise(c, Next)`, gdy `Next` jest sąsiadem w
ćwierci obrotu od kierunku marszu (`grid.Way`, iloczyn > 0), inaczej po głowie jak dotąd; rodzaj
kroku zostaje gruntem pod jednostką.

**Etap 2** (`feat/trail`): `Touch` dostaje `Ahead`, `Along`, `SameWay`, `OtherTrailing`; słowa
`WalksMyWay`, `Groupmate`; komenda `Trail{Of}` (`host.Aimed`), stan rozkazu `Trailing`,
`TrailFor`, `Trailed`, `TrailPace`; `Body` niesie `Share`, `MaxSpeed`, `Trailing`; keeping ciał
ogranicza prędkość chcianą do prędkości prowadzonego wzdłuż własnego kierunku (`along/Share`),
`Watch` i `Blocked` nie liczą zacięcia przez czas, jakiego prowadzony prawnie potrzebuje na
komórkę (najwyżej `maxTrailWait`); reguła domyślna `WalksMyWay && Groupmate → Trail` przed
`Blocks → Detour`.

**Etap 3** (`feat/field`): klucz pola (komórka celu z `At`, `Level`, domena, klasa ciała bez
`Feet`), partia ≥ `fieldParty` liczona z żyjących rozkazów, `Sweep/Expect/Take/Give` w
koordynatorze, `FillField` (Dijkstra, krok `n → t` sprawdzany i wyceniany jak `leveledFrom`,
kopiec leniwy, stop po osiadaniu ziaren z marginesem), `Walk` (kawałek 64 kroków, ostatni
odcinek do własnego celu przez `findFrom`), spadki do A* (mała partia, `Avoid`, komórka
nieosiągnięta, duża plansza); `CellKeeping.Orders` ustawia `At`.

**Pomiar** (po każdym etapie, kopia bazowa przeciw drzewu, na zmianę, bezczynna maszyna):
`Navigation_Tick`, `Navigation_Path`, `Board_Altitude`, `Gravity_Standing` (etapy 1–2; w szumie),
`Routing_Path`, `Routing_Routes`, nowe `Routing_Group` i `PathFinder_Field` (etap 3: jedna
Dijkstra + 32 marsze wobec 32 A*, oczekiwane ≥ −70%); sonda płaskowyżu po każdym etapie (30
górali jednym `MoveTo`: dojścia, czas ostatniego, max `Stalls`, szukania i pola).

## Kolejka i pole grupy: wynik (2026-10-28)

**Etap 1** (`fix/stall-pace`, v0.7.1): keeping ciał żąda postępu `min(ćwierć ciała,
MaxSpeed·Share·StallAfter/4)`, `Share` z opcjonalnej kolumny `steering.Pace` zapytania traffic
(1 bez niej), udział 0 nie liczy; keeping komórek bez zmian. `unit.At.Next` pisze traffic (0 na
początku `head`, punkt trasy po rezerwacji, następny krok po `Index++`, 0 w `arrive`, pod ręką i
w `moveTo`), jazda (komórka przed; 0 przy własnej komórce, bo w jej obrębie prowadzi głowa) i
zajętość (0); przebieg stania wycenia stok kroku prowadzonego, gdy ten jest sąsiadem w ćwierci
obrotu od kierunku marszu, a **krok w obrębie własnej komórki na płasko**: pierwsza sonda
pokazała lidera pełznącego 100 s w komórce celu po udziale najstromszego boku (0,016 zamiast
0,308), bo na szczycie każdy sąsiad jest stromy, a marsz do miejsca w komórce planner wycenia bez
stoku. Testy: tempo po kroku (tabela sześciu przypadków), odmowa jazdy (brakujący dotąd test),
zacięcie po prawnym tempie (goły keeping, cztery przypadki), pełzanie po pasie 0,01 bez zacięcia
(pada na `main`, przechodzi na gałęzi; rig `fieldUnit.paced`), `Next` w marszu i po dojściu.
Sonda płaskowyżu (30 górali jednym `MoveTo`, x4, kopia `main` przeciw drzewu): baza 0 z 30,
wszystkie rozkazy gasną w 50–77 s; drzewo 3 z 30 dochodzi w 228–233 s gry z udziałem równym
udziałowi kroku plannera (0,071 przy stoku 0,42, zero zacięć), 27 traci rozkazy w 51–71 s w
kolejce za idącymi (`held by`, `Blocks → Detour`, `stalls 5`): to etap 2, jak zakładano. Pomiar
(kopia `main` v0.7.0 w scratchpadzie przeciw drzewu, `GOWORK=off` po obu stronach,
`BENCHTIME=200x`, 3 rundy × `-count=2`, zwykły i `-cpu=1`): `Navigation_Tick` cztery rungi i
`Navigation_Path` w szumie (±1,5%); przebieg stania +1,5…+2,3% min przy 1 000 piechurów
(`Board_Altitude` 398 → 405 µs: odczyt `At.Next`, `At` 8 → 16 B), w szumie przy 100; przyjęte.
Dema z nawigacją i jazdą pod `timeout 20s` zdrowe. Stary zapis nie wczyta się (`At` większe).

**Etap 2** (`feat/trail`, v0.8.0; wykonany przez agenta Opus 5.5 z briefu `doc/briefs/t2-trail.md`
w worktree, integrowany sondą): cztery rundy, bo sonda płaskowyżu obalała projekt trzy razy.
(1) Brief jak w planie: słowa `Ahead`/`Along`/`SameWay`/`OtherTrailing` po głowie, `Trail` z
pamięcią 0,5 s, czekanie ograniczone rozpiętością komórki przez tempo prowadzonego i 3 s;
agent dołożył podawanie `TrailPace` w dół kolejki (bez tego trzeci w kolumnie zacina się, bo
drugi na otwartym gruncie ma pełne tempo). Sonda: 3 z 30, jak po etapie 1 — w zbitej grupie
styki są `Resting`, dotyk dla nich nie pada, trail gaśnie po 0,5 s; jednostki podążały za
sobą nawzajem (109↔103↔120); granica 3 s zabijała lejek, gdzie czeka się na całą kolejkę.
(2) `Ahead` po odległości od punktu rozkazu, odnowienie trailu przez styk (także spoczynkowy)
z żywym prowadzonym (`Body.Alive`: idzie albo podąża), granica tylko żywotnością
(`maxTrailWait` 30 s jako bezpiecznik; martwa kolejka rozwija się ogniwo po ogniwie co 0,5 s).
Sonda: 3 z 30 — jednostka u wylotu lejka podążała za sąsiadką obok, o włos bliższą punktu, i
stała z jej prędkością (0); a gdy jedna się poddała, `ReachedTheGroup → Stop` zatrzymało
kaskadowo 13 współtowarzyszy w 2 s, 35 komórek od celu. (3) `Body.Way` (kierunek następnego
kroku trasy): `Ahead` to stożek ćwierć obrotu wzdłuż niego, `SameWay` zgodność kierunków
tras, `Along` ruch wzdłuż niego (bez trasy po głowie); `InTheWay` wymaga `Ahead` (uderzony od
tyłu albo z boku nie obchodzi uderzającego); `ReachedTheGroup` wymaga `NearMyGoal` (dwie
komórki od celu): grupa dołącza tylko przy celu. Sonda: 23 z 30; siedmiu ginie w drugim lejku
pod krawędzią, gdzie prowadzony idzie komórkę przed, bez styku, trail gaśnie i powolny marsz w
tłoku (0,3–0,7 jednostki/s przy udziale 0,08) liczy zacięcia. (4) Trail trzyma się także, gdy
prowadzony jest na szerokość ciała przed jednostką wzdłuż jej drogi (`withinReach`). Sonda:
**30 z 30**, ostatni dochodzi w 290 s gry (baza v0.7.1: 0 z 30, rozkazy gasną w 50–77 s).
(5) Pomiar `-cpu=1` wykazał `Navigation_Tick/crowded,100` +28% i `open,1000` +12%; bisekcja po
commitach wskazała rundę 3, profil A* (więcej szukań), nie nowy kod traffic: obcy uderzeni od
tyłu przestali uskakiwać, korek w szczelinie benchmarku trwał dłużej i zablokowani szukali
częściej. `InTheWay` wymaga `Ahead` tylko od współtowarzysza (`Ahead || !Groupmate`): tick
zatłoczony wraca do bazy (kopia: 1068 wobec 1076 µs), płaskowyż bez zmian (wszyscy to jedna
grupa). Trzy nieziarnowane przebiegi sondy (20:30–20:38, drzewa ef612380 i b95d9801) pokazały
całą grupę idącą w miejscu od startu (v 9,6, komórka bez zmian, zero zacięć, bez styków);
ziarna 1–7 i późniejsze przebiegi bez ziarna na bazie i drzewie normalne; niewyjaśnione,
nieprzypisane etapowi (etap nie dotyka ruchu jednostki bez dotyku). Testy: słowa
(`touch_test`), `bearing`/`against` (traffic), kolumna trzech za pełzającym liderem bez
zacięć, sojusznik innej grupy nie podąża, kolejka pod komórkami idzie za pełzającym jedyną
drogą i obchodzi zatrzymanego, ślepa uliczka rozwija się, grupa dołącza tylko przy celu,
obcy od tyłu blokuje jak dawniej, współtowarzysz nie, tabela zacięć rozszerzona. (6) `Way`
liczony w `gather` na każde ciało (`CellCenter`, unwrap, hypot) kosztował open 1 000 +3% min:
zapisuje go `tail` w rozkazie z wektorów, które i tak ma. Pomiar końcowy (`38bb2879` przeciw
kopii `main` v0.7.1, `GOWORK=off` po obu stronach, `BENCHTIME=200x`, 3 rundy × `-count=2`,
`-cpu=1` i zwykły): `Navigation_Tick` cztery rungi i `Navigation_Path` w szumie (najgorzej open
1 000 +1,9% min w przebiegu zwykłym, +0,7% `-cpu=1`). Sonda końcowa: 30 z 30 z ziarnem 7 i bez
ziarna, ostatni w 291 s gry. Sześć dem z nawigacją pod `timeout 20s` zdrowe. Stary zapis nie
wczyta się (rozkaz większy).

**Etap 3** (`feat/field`, v0.9.0; agent Opus 5.5 z briefu `doc/briefs/t3-field.md`, siedem
commitów: wyłuskanie `step`/`findFrom` (golden tras bez zmian), `pathfind.Field` z `FillField`,
`Walk`, `Fills()`, `spacing.Fields` z `walkField` i gałęzią w `avoiding.find`, `Member.Field`,
liczność partii w rozkazie (`MoveOrder.Party`), `At` w `CellKeeping.Orders`, testy grupy,
`Benchmark_Routing_Group` i rung `grouped`). Odstępstwa agenta od briefu, przyjęte:
`findFrom(start, to, end, dst)` bez drugiego `begin`; pole tylko dla findera `WithLevels`
(wtyczka zawsze planuje po poziomach); bez ostatniego odcinka, gdy cel członka to cel grupy;
węzły nieosiadłe przy stopie liczone jako nieosiągnięte (`Cost` dokładny, gdzie odpowiada);
`Expect` nie przelicza wypełnionego wpisu (inaczej pole liczyłoby się co Update); `Key` odmawia
rozkazowi ustępowania z kolejką celów; `Fields.Fill(f, finder)` między `Take` a `Give`.
Poprawka po pomiarze (`dd24d2c2`): liczenie partii mapą po żywych rozkazach w każdym Update z
prośbą grupy kosztowało +1,7…+3,0% min na rungach `Navigation_Tick` bez pola i `grouped,1000`
+5…+7,8%; liczność wpisuje się przy wykonaniu MoveTo (`MoveOrder.Party`, zachowany przez
`Advance` i ustąpienie), mapa i drugi przebieg odpadły; ponowne wypełnienie po zmianie planszy
bierze miejsce poprzedniego (bez alokacji); ustąpienie zachowuje `Party` (`1b6afb7b`); marsz
jako odpowiedź pamiętana (`eddf3324`, odwraca brief: najpierw odpowiedzi pamiętane, po
chybieniu marsz oddany pod tym samym pytaniem, po nieudanym marszu szukanie). Niezmiennik
przeszedł na trzech planszach (2 362
porównań kosztu komórki pola z
trasą `Find`, 1e-4). Sonda płaskowyżu: 30 z 30 z ziarnem 7 i bez (jak v0.8.0; remisy Dijkstry
nie zmieniły wyniku). Pomiar (`-cpu=1`, baza v0.8.0 z plikami benchmarków gałęzi, na zmianę,
najniższy z 6): `Routing_Group` 347 → 55,2 ms na jednym wątku i 344 → 54,6 na puli (−84%;
wypełnienie to praca jednego routera, marsze mikrosekundy, więc pula nic tu nie daje),
`Routing_Path` +0,1%, `Routing_Routes` +1…+2,5% (w szumie), `PathFinder_Terrain` −2,3…+1,8%
(wyłuskanie `step` bez kosztu), `PathFinder_Field` map 19,6 ms, relief 31,2 ms: ok. 3 × jedno
szukanie tej planszy (Dijkstra bez heurystyki po całej planszy). W tłumie grupy
(`grouped,1000`) pole nie wygrywa: liczniki agenta na 200 ticków — szukań wokół nauczonych
komórek (`FindAround`, A* jak dawniej) 3 615 wobec 3 613 na bazie; z marszami niepamiętanymi
147 marszów (każdy z ostatnim odcinkiem: cele w pierścieniach) i 104 zwykłe szukania wobec 71,
`grouped,1000` +6,5…+13,9% min (w korku to samo pytanie wraca, baza odpowiada z pamięci); po
poprawce `eddf3324` 15 marszów i 102 zwykłe szukania — zysk pola to trasy początkowe i replany
bez nauczonych komórek, nie korek. Tick (baza v0.8.0 z plikami benchmarków gałęzi, `-cpu=1`,
na zmianę, najniższy z 6): rungi bez pola w szumie (+0,2…+1,7% min; przed `Party` +1,7…+3,0%),
`grouped,1000` +3,9% min (księgowość pól i marsze z ostatnimi odcinkami w korku, który i tak
szuka wokół nauczonych komórek; przyjęte jako cena nowego reżimu, bo stare reżimy nie tracą).
Sondy na cichej maszynie: po trzy przebiegi z ziarnem 7 na v0.8.0 i na drzewie, wszystkie 30 z
30; jeden wcześniejszy przebieg drzewa 26 z 30 i jeden „w miejscu" przypadły na maszynę
obciążoną po benchmarkach — sonda nie jest deterministyczna przy jednym ziarnie (liczba kroków
symulacji na Update idzie za czasem rzeczywistym; pułapka w `gram-probe`). Stary zapis nie
wczyta się (rozkaz o 2 B większy). Sześć dem z nawigacją pod `timeout 20s` zdrowe.
