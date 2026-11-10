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

Dziewięć części naraz (agenty Opus 5.5, bez pomiarów czasu): A
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

Na życzenie użytkownika: pozostałe pomysły alokacyjne: E2 trzy kroki
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
stożka. gram `perf/sight-simd` od `main` v0.5.0: ten plan.

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

**Etap 4** (`feat/column`, plan 2026-10-28 po etapie 3; decyzje użytkownika: nowa komenda
nośna dla wielu formacji, nie pole w `MoveTo`, zwartość i luz kolumny jako parametry
rozkazu): `March{Cell, To, At, Level, In Formation}`, `Formation{Shape, Spacing, Closed,
Slack}`, `ColumnOf`; rozkaz niesie formację, miejsce w niej, poprzednika i odstęp; koordynator
układa formację z kosztu pola co Update, keeping trzyma tempo poprzednika, czoło zwarte czeka
na ogon (runda 2 po sondzie płaskowyżu). Pomiar:
`Navigation_Tick` (nowy rung `column,1000` wobec `grouped,1000` drzewa; baza ma 4 rungi),
`Routing_Group`; sonda: 30 górali jednym `March`, kolumna zwarta i rwana, wobec `MoveTo`.

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

**Etap 2** (`feat/trail`, v0.8.0; wykonany przez agenta Opus 5.5
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

**Etap 3** (`feat/field`, v0.9.0; agent Opus 5.5, siedem
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

**Etap 4** (`feat/column`, v0.10.0; agent Opus 5.5, dwie
rundy: `b07dc611`, `c6aae102`, `95effb44`, `76f88d45` i poprawka `19daf936`). Decyzje
użytkownika: nowa komenda nośna dla wielu formacji (nie pole `MoveTo`), zwartość i luz kolumny
jako parametry rozkazu. Weszło: `March{Cell, To, At, Level, In}` (ta sama droga co `MoveTo` w
`carryOut`, każdy rozkaz z `Formation`), `Formation{Shape, Spacing, Closed, Slack}`,
`ColumnOf`; rozkaz niesie `Place` (1 = czoło; id encji zaczynają się od 0, więc `AheadOf == 0`
nie mogło znaczyć czoła), `AheadOf`, `Lag` w jednostkach świata (nie w koszcie: odstęp w
koszcie to odstęp czasu i kurczy się na wolnym gruncie, przy udziale 0,01 z 12 px do 0,12;
koszt dzielony przez stok pola na droższym gruncie z dwóch, ciągły między komórkami, nie
większy od odległości w prostej), na czole `Gap`, `Narrowest`, `Closing`, `Patience`, na
każdym `Kept` i `HeadWaits`; koordynator układa formacje co Update przed ruchem (`Form` z
`traffic.Update`, członkowie zbierani w `gather`), formacja czyta pole od partii 2 (`byField`),
brakujące pole wypełnia sama; keepingi tempują za poprzednikiem w zasięgu dwóch `Room` i w
stożku ćwierci obrotu, z hamowaniem do zatrzymania `Room` przed nim
(`√(v² + 2·Brake·Share·(Lag − Room))`; prosty przełącznik przy jednym `Room` dawał styk: 12 px
drogi hamowania przy 96 px/s to dokładnie jeden `Room`); czoło zwarte czeka, póki najszersza
luka między kolejnymi członkami (nie odstęp ogona od czoła: zwarta kolumna n ma go ok.
(n−1)·`Room`, więc każda dłuższa niż `Slack`+2 czekałaby wiecznie) maleje, i `Patience` (1 s)
dłużej, nigdy gdy ktoś odmawia mu komórki; `HeadWaits` zwalnia członków z zacięć na czas
czekania czoła; członek trzymany liczy się dotykowi jak podążający; nikt nie podąża za kimś za
sobą w formacji (bez pętli podążań); ustąpienie zachowuje formację; kształt nieznany idzie jak
`MoveTo` (raz w logu); fakt `InFormation` niezrobiony (komenda na członka na tick). Runda 1
przeszła testy (korytarz pod komórkami, dwie kolumny pod ciałami), ale sonda płaskowyżu dała
kolumnę zwartą `Slack` 1 → 1 z 30 (wszystkie rozkazy stracone do 620 s), `Slack` 4 → 0 z 30
(grupa stojąca w ujściu bez zacięć), rwaną → 14 z 30, przy `MoveTo` 30 z 30 w 291 s: cztery
mechanizmy opisane w briefie (runda 2): `Lag` ciał w styku 6,6–7,3 przez skok kosztu na
granicy komórek, poza zasięgiem tempa `Room` + hamowanie ≈ 6,4; `Closing` jako żywotność
marudera (flaga podążania w węźle wzajemnych podążań 108 → 114 → 96 → 102 → 108) nie kończące
czekania czoła; stojący członek formacji pękający łańcuch podążania z etapu 2 (`Blocks →
Detour` wokół własnego czoła); brak zwolnienia z zacięć poza zasięgiem `pacer`. Same kontrakty
rundy 2 dały 20 z 30; porządek podążań w formacji 30 z 30 w 399 s, ale 340 inwersji na 435 par
(przód kolumny zamrożony za kimś obok, tył wyprzedzał); stożek ćwierci obrotu dał wynik
końcowy. Sonda (ziarno 7, cicha maszyna, agent dwa razy i integrator dwa razy, co do sekundy
te same): kolumna zwarta `Slack` 1 30 z 30, ostatni 339,7 s, zacięć 0, czoło czekało 10,8 s, 71
inwersji porządku dojścia na 435 par (ujście przetasowuje kolumnę, za nim porządek wraca);
rwana 30 z 30 w 329,7 s, zacięć najwyżej 1, 38 inwersji; `MoveTo` 30 z 30 w 291,3 s, zacięć do
5 (jak v0.9.0). Test rundy 1 „czoło czeka, aż ogon nadrobi" zmieniony: ogon idący tempem czoła
nic nie zamyka, więc czoło idzie dalej (`TestColumn_AClosedHeadsWaitOnAStoppedTailEnds`,
`…WaitsWhileItsGapCloses`). Ryzyka z raportu agenta: kolumna przetasowana w ujściu; za
zakrętem poprzednik wychodzi ze stożka, więc członek dochodzi do rogu, nim znów go tempuje;
członek trzymany jest podążającym także dla obcych; mieszane rozmiary ciał to osobne kolumny
(klucz pola z klasą ciała); szew torusa psuje „przed" na jeden tick; dwie grupy przez szczelinę
jednej komórki pod komórkami chaotyczne jak przy `MoveTo` (z 64 w ośmiu wariantach rundy 1:
`MoveTo` 7 straconych, zwarte 12, rwane 18). Stary zapis nie wczyta się (rozkaz większy).
Pomiar (baza v0.9.0 z własnym plikiem benchmarków, cztery rungi; na zmianę, najniższy z 6 przy
`BENCHTIME=200x`, zwykły i `-cpu=1`): rung `column,1000` 2,84 ms zwykły i 2,77 ms `-cpu=1`, 0
alokacji, wobec `grouped,1000` tego samego drzewa 4,78 i 8,17 ms (tłum tempowany za czołem
mniej napiera na szczelinę, więc mniej szuka wokół nauczonych komórek; bez profilu); stare
rungi w szumie (−1,0…+3,4% min; +3,4% `open,1000` zwykłe to −1,0% przy `-cpu=1`);
`Routing_Group` (w `plugins/navigation`, nie w `bench/`: `measure.sh` go nie widzi) bez zmian
(+0,0% i −1,1% min, `-cpu=1`, 10x). Sześć dem z nawigacją pod `timeout 20s` zdrowe.

## HPA*: planner hierarchiczny w astar, przejezdność planszy i adapter (plan 2026-10-29, gałęzie `feat/hierarchy` w astar, `feat/passage` i `feat/hpa` w gramie, litera `h`)

Analiza (2026-10-29, v0.10.0, trzech agentów Explore i trzech Plan; plik planu sesji):
(1) długie trasy zalewają planszę: na Maderze udziały poza drogami 0,13–0,4 czynią `h =
odległość` 2,5–7,5× za małą, więc ślepe szukanie rozwija większość wyspy (~6 ms, 0,75 µs na
rozwinięcie nad reliefem); `Routing_Path` (200×200, ≥195 kroków) rozwija ~33 tys. węzłów, 13,2
ms; (2) kawałkowanie mnoży koszt: `Path` niesie 64 kroki, traffic pyta od końca kawałka, a
klucz odpowiedzi pamiętanej ma `from`, więc trasa 195 kroków to trzy pełne szukania (~2×
pracy jednego); (3) wiatr gasi pamięć: kołysanie lasu to `effect.Alter` na `cell.Ground`, każde
przejście wiatru podbija `CellVersion` każdej komórki lasu i `Version`, a po `Version` lecą
odpowiedzi pamiętane, pola i udziały płaskie (śnieg tak samo bez zmiany cen; lód zmienia
przejezdność naprawdę); (4) cel nieosiągalny zalewa do 64 razy (`NearestFree`,
`BodyKeeping.Lost`); (5) sektory świata to podział pod pasma tempa (bok z gęstości encji,
bez publicznego dostępu, kwadratowe), a nawigacja nie ma żadnej struktury ponad komórką;
(6) astar v1.2.0: jeden pakiet, `Solver[T]` bez odczytu G, wielu celów i licznika rozwinięć.

Decyzje użytkownika (2026-10-29): korzeń astar staje się plannerem hierarchicznym — jeden
pakiet z `Solver` (bez zmian: jeden poziom i silnik szukania abstrakcyjnego), `Dijkstra[T]`
(jeden-do-wielu) i `Hierarchy[T]` (adaptacyjnie wiele poziomów: bok poziomu 1 od wołającego,
wyżej bloki f×f = 4, poziomów tyle, by najwyższy miał ≤ 16 klastrów; Madera przy boku 16: dwa
poziomy), przykład `examples/hpa` z mapami generowanymi 256²/1024²/4096², nic pod Maderę;
plansza rozdziela zmiany przejezdne od reszty (licznik `Passage`), nawigacja pamięta po nim;
klastry własne nawigacji po komórkach, domyślny bok = bok sektora w komórkach, gdy całkowity
na kwadracie, inaczej 16, hex 16 w osiach, pokrętło `WithHierarchy{Side, From, Off}`;
doprecyzowanie bez szukania (drzewa od wejść zapisane przy budowie, A* tylko abstrakcyjny;
0,5 ms na kawałek odrzucone); schody: astar v1.3.0 i gram etap A równolegle, gram etap B po
tagu; wykonanie i pomiary agentami Opus, integrator analizuje, scala i zleca poprawki.

**Etap astar** (`feat/hierarchy`, v1.3.0, części `h/dijkstra` → `h/hierarchy` ∥ `h/example`):
`Dijkstra[T]{NewDijkstra, Fill(sources, transitions, within,
targets), Cost, Parent, Settled, Wipe}` nad kopcem i słownikiem solvera; `Hierarchy[T]`
(`Graph{Forward, Backward, Heuristic}`, `Layout{Coords, Size, Indexer, Capacity}`; `WithSide`,
`WithFactor`, `WithTop`, `WithSpan`, `WithTrees`; `Stale`, `Build(budget)`, `Ready`, `Find`
leniwe do `dst`, `Reaches`, `Cost`, liczniki): wejścia na parę klastrów przerzedzone co `span`
miarą heurystyki plus lokalne minima kosztu przejścia (drogi) plus końce biegów; krawędzie
skierowane; poziom k nad grafem poziomu k−1; determinizm (porządek id i współrzędnych, remisy
kopca); testy skali i golden kosztów; przykład i benchmarki. Tag v1.3.0: użytkownik.

**Etap A** (`feat/passage`, v0.11.0): rzut przejezdny
komórki (`Allows`, `Solid`, `Graded`, `Pace`, `Paces`, droga: rodzaj, szerokość, linki,
`Built`) haszowany na ordinal; `BoardRead.Passage()` i `CellPassage(c)` (nie startują od nowa;
`Touch` zawsze liczy się; `Set`/`SetWay`/`SetAll` i przebieg efektów stemplują tylko przy
zmianie rzutu; wczytanie stempluje wszystkie); nawigacja (`flatStep`, odpowiedzi pamiętane,
pola, `terrainSeen`) po `Passage`; `navigation.Counts` i `Read.Counts()` (`Searches`, `Walks`,
`Fills`, `Climbs`, `Builds`, `Expansions`) z linią w panelu. Pomiar (brief `m-passage.md`,
agent mierzący): benchmarki nawigacji i planszy w szumie; sonda Madery jako punkt odniesienia
(trzy jednostki graczy i 30 górali: szukania na rozkaz, rozwinięcia, czas `Routing.Update`,
kadencja `Version`/`Passage` pod wiatrem).

**Etap B** (`feat/hpa`, v0.12.0, po tagu astara i podbiciu `go.mod`; części `h/graph` ∥
`h/route` od preludium kontraktów; briefy `h4-graph.md`, `h5-route.md`): `pathfind.Finder` jako
`astar.Graph[node]` (Forward `leveledFrom`, Backward `leveledInto` z `into` pola) i
`Layout[node]` (`Coords` komórki; sloty poza siatką bez klastra); `Key{Domain, Body bez Feet}`;
`Climb`, `Reaches`, `Climbs()`; próg jako czysta funkcja pytania `grid.Distance(from, to) ≥ From`
(64 = `MaxPathLength`), tylko szukania ślepe i gotowa hierarchia; `FindAround`, kandydaci
`NearestFree`, `rings`, `Detour`, podgląd: jak dziś (golden i `fieldMatchesFind` nietknięte z
konstrukcji); `spacing.Hierarchies` jak `Fields` bez Cond (`Sweep` na gorutynie wołającej: skan
`CellPassage` gdy `Passage` ruszył → `Stale`, `Build(budget)` klastrów na Update, klucz
dokładny póki nie `Ready`; `Expect` deterministyczne LRU 4; `Take` bez blokady);
`Member.Hierarchy`; `find`: pole → wspinaczka → `FindPath`; `NearestFree` i `Lost`: `Reaches` po
pierwszym nieudanym `Find`; pokrętło i domyślny bok z sektorów (jak minimapa czyta); demo
topografii `From: 32`.

**Pomiar** (agent mierzący, kopia bazowa v0.11.0 z plikami benchmarków gałęzi, na zmianę,
najniższy z 6, 200x/10x, zwykły i `-cpu=1`, profil `Routing_Path` po obu stronach):
`Routing_Path` 13,2 → ~1 ms (≥ −50%, 0 alokacji), `Routing_Routes` 189/64,7 → ~15/5 ms, nowy
`Routing_Long` (≥ 3 kawałki, ~30 → ~2 ms), `Routing_Group` ±3%, `PathFinder_Terrain` bez zmian +
rung `levels`, nowe `PathFinder_Hierarchy/{build,find,rebuild}×{map,relief}`, `Navigation_Tick`
6 rungów i `Navigation_Path` w szumie; sonda Madery: wspinaczek tyle, co kawałków, szukań 0 na
długą trasę, dojścia 30/30 ≤ 340 s, zero przebudów po przejściu wiatru.

## HPA*: wynik (2026-10-29)

**Etap A** (`feat/passage`, v0.11.0; agent Opus 5.5, cztery
commity: `6f7d1f23` przejezdność komórki, `dbd2737f` nawigacja po niej, `635153b2` liczniki,
`e302126e` komentarz). Weszło: rzut przejezdny komórki haszowany na ordinal (`maphash.Comparable`,
16 B na komórkę) — szerszy niż brief, i słusznie: `price` wycenia krok skośny poza drogą po
`Bare`, czyli po gruncie pod drogą, więc rzut liczy grunt i drogę osobno (udziały, domeny,
`Solid`, `Graded`; nazwa, szerokość i linki drogi; `Built` w całości czytany ze store'u, bo
`Read.Built` jest bramkowane `t.built` ustawianym dopiero po dotknięciu: pierwszy pokład na
pustej planszy nie liczyłby się); `Touch` liczy się zawsze; `Passage` startuje od nowa przy
wczytaniu jak `Version` (nowa plansza, nowy etap); odpowiedzi pamiętane, pola, `flatStep` i
`terrainSeen` po `Passage` (`entities.go` i `path_finder.go` bez zmian: oba przebiegi wołają
`touch`); `navigation.Counts`, `Read.Counts()`, `Read.Reporter()` (demo topografii pokazuje
linię; integrator), `Finder.Expansions` (`plainFrom`/`leveledFrom`); zdanie przy `cell.Mobility`
(udział tylko z `Step`). Testy: 26 przypadków rzutu pole po polu, pokład na pustej planszy,
flats po przejezdności (test w `pathfind`, bo routery czyszczą flats co Update), liczniki
rozwinięć, kołysanie bez utraty odpowiedzi i pola, lód z utratą; golden, `-race`, SIMD zielone.
Ryzyka: `Mobility` gry czytająca pole spoza `Step`; krok czyta sąsiadów (narożniki, pasma),
więc klaster etapu B ocenia nieaktualność po komórkach i pierścieniu; koszt przeniesiony na
zapisy (3 seeki i hasz na dotkniętą komórkę, `made`/`SetAll` O(komórek)). Pomiar (agent
mierzący Opus; baza v0.10.0, na zmianę, `GOWORK=off`, 200x
3×2 zwykły i `-cpu=1`, routing i pathfind ręcznie 10x): nic nie traci ≥ 3% mediany z powodu
etapu; hasz przejezdności (`passageOf`, `maphash`) widać w profilach tylko w układaniu planszy
benchmarku (10–30 ms), nigdy na ścieżce mierzonej; `Routing_Path` +2,1% mediany z 30 próbek
na stronę (po stronie drzewa w każdym przebiegu, pod progiem); `Routing_Group`, `PathFinder_*`,
`Navigation_Tick` 1 000 w szumie. **Pułapki pomiaru znalezione**: rungi 2–60 µs przy 200x
trwają ~10 ms i mówią tylko szum (powtórzone z `BENCHTIME=1s` wszystkie w −3,5…+1%); baza
biegnie pierwsza i jej pierwsza runda w oknie boostu (~25–30 s) wychodzi 10–20% szybciej
(zamiana stron odwraca znak); układ pamięci pod transparent huge pages (`[always]`) daje
dwumodalność (`Routing_Routes/workers=1` 230–245 wobec 257–277 ms po obu stronach,
`crowded,100` przy 2000x +28% z THP, −0,2% z `GODEBUG=disablethp=1`). Sonda Madery (burza na
start, ziarno 7, po dwa przebiegi na stronę): dojścia co do ticku te same (Poiso→Faial 118 s,
Funchal→Santana 170 s, Machico→Câmara de Lobos 210 s, 30 górali w 292 s); liczniki drzewa
identyczne w obu przebiegach: 54 szukania, 51 marszów, 1 pole, 18 300 rozwinięć w 300 s;
długa trasa to trzy kawałki i trzy szukania (Funchal→Santana 64+58+41 kroków; Machico 2912 i
994 rozwinięć na kawałek), siedem rozkazów mapy na drugim ticku to 8 139 rozwinięć; jedyne
przejście wiatru (43,8 → 15 przy 1,07 s, kołysanie 1 788 komórek lasu) podbiło `Version` o 1 i
`Passage` o 0 (mosty na pierwszym ticku: `Passage` +65); ticki z szukaniem w medianie równe
cichym (±0,1 ms). Sześć dem z nawigacją: w etapie B (demo topografii zbudowane z linią
liczników).

**Etap astar** (`feat/hierarchy` w astar, do v1.3.0; agenci Opus):
`Dijkstra[T]` (jeden-do-wielu, `within` pytane przed słownikiem, więc słownik
lokalny klastra), `Hierarchy[T]` (poziomy adaptacyjnie, wejścia co `span` z podwajaniem w górę,
lokalne minima kosztu przejścia, krawędzie zdominowane pominięte, budowa na tablicach lokalnych
klastra, kaskada `Stale`), `Search[T]` na gorutynę (zapytania nic nie piszą w hierarchii,
etykiety i bazy liczy `Build`), poziom szukania po odległości (słowo użytkownika: para bliska
przez granicę wysokiego poziomu robiła objazd do jego wejść; testy padały na starym kodzie),
`examples/hpa`, mapy generowane i testy skali (4096² za `ASTAR_SCALE=4096`). Pomiar (jeden
przebieg na stronę): `Find` 1024² 1,35 ms wobec `Solver` 358 ms (266×), budowa 256² 168 ms,
`Solver_Octile` wobec v1.2.0 −2,1% (wcześniejsze +5–7% to boost pierwszej rundy bazy).
Scalenie do `main` i tag: użytkownik.

**Etap B** (`feat/hpa`, v0.12.0; agenci Opus z briefów `h4-graph.md`, `h5-route.md`, rundy h4b,
h4c, h5 2–3): `pathfind.Hierarchy` nad `astar.Hierarchy[node]` (jeden finder buduje, każdy pyta
własnym `Search`; graf tylko po węzłach, na których klasa stoi — rzeka na granicy klastra
łączyła brzegi w `Reaches`), `spacing.Hierarchies` (4 klasy LRU, `Expect` deterministyczne,
`Sweep` co tick: `Stale` przy ruchu `Passage`, 2 klastry budowy na tick; partie bez hierarchii),
`find`: odpowiedź pamiętana → pole → wspinaczka → szukanie; `Lost` z `Reaches`; routing co tick;
`WithHierarchy{Side, From, Off}`, bok z sektorów świata; demo topografii `From: 32`. Pomiar
(jeden przebieg na stronę, słowo użytkownika 2026-10-29: scenariusze raz albo dwa, nie serie;
cały pomiar kilka minut): `Routing_Path` 12,25 → 3,88 ms (−68%), `Routing_Long` 27,5 → 6,0 ms
(−78%), `Navigation_Tick` 1 000 w szumie (crowded +4–6% z THP, −2,9% bez), wypełnianie pola po
h4c jak v0.11.0 (+0,4% map, −1,6% relief; przed h4c +5–17%: ramka `stepsInto` na sąsiada i
pola findera przesunięte o 16 B), `Routing_Group` +0,2%/−1,7%; tick budowy ~3 ms; sonda Madery:
klasa gotowa w 27 tickach (198 klastrów), długie rozkazy wspinane po pierwszym kawałku, Machico
217 s wobec 210 s (trasa wspinaczki o kilka procent dłuższa), reszta dojść jak w etapie A.

**API astara przed tagiem** (2026-10-29, `h/api` od `feat/hpa`, agent Opus): API
astara przebudowane przed tagiem na jedno publiczne
`Solver`/`Search`/`Path` (`Solver` nad `Graph`, z `Layout` HPA*; `Search` na gorutynę;
`Changed`/`Update`/`Ready`; silniki nieeksportowane), wersja 1.3.0 na słowo użytkownika; gram na
pseudowersji `aae2aec` do tagu. Szukania findera to `NewSolver(...).Search()` (`Path` pisze start
i tyle kroków, ile mieści bufor, poziomowy jeszcze cel; `Ended` dalej koniec trasy), `Wipe` i
bezczynne czyszczenie znaczników koordynatora zniknęły (astar czyści kawałek na szukanie);
hierarchia to `Solver` z `Layout` (`WithClusterSide`, `WithNear` dwa boki), klastry poziomu 1
liczone z własnych współrzędnych, `Stale` przez `Changed`, `Build` pętlą `Update(1)` (`Update(0)`
buduje teraz wszystko), prośby `SearchVia`, wspinaczki ze `Stats().Climbs`; `Path` nie odmawia par
bliskich, więc `Climb` odmawia sam pary bliższej niż dwa boki klastra (inaczej astar szukałby jej
A* w słowniku każdego `Search` hierarchii). Trasy, wspinaczki, `Decks`, `Ended`, `Reaches` i
liczniki co do bitu jak na `feat/hpa` (kwadrat, relief, heks, torus, wąwóz z pokładem, po 200
par). Sprawdzenie kosztu (jeden przebieg na stronę, `-cpu=1`, baza 4273e957): wszystko w szumie,
`Routing_Path` −0,9%, `Routing_Long` −0,9%, `Routing_Group` +1,6%/0,0%, `PathFinder_Terrain`
−0,6…+2,5%, `PathFinder_Field` −2,5%/−0,6% (wypełnienie pola nie woła astara); B/op
`Routing_Long` przy 5x +184 KB to słowniki A* routerów, które astar robi przy pierwszym szukaniu
`Search`, nie przy budowie: przy 40x i 400x pętla alokuje tyle samo w sumie, zero na op po
rozgrzaniu po obu stronach, a cały proces o 2,3 MB mniej.

## LOD: jedna drabina poziomów dla ticku i obrazu (plan 2026-11-03, gałąź `feat/lod`, litera `l`, v0.18.0)

Analiza (2026-11-03, v0.17.2, agent Explore po całym drzewie): poziom szczegółu zapada dziś w
czterech niezależnych drabinach i w kilkunastu miejscach: (1) pasma sektorów świata po
odległości euklidesowej od `Focus` kamer, z marginesem pół sektora, znaczniki
`Band2..Band5`/`Off`, polityki `Near`/`Paced`/`Everywhere` dokładane przez plugin do własnego
zapytania (`world.Plugin.Pace`), a systemy bez polityki (lampy światła, standing i occupancy
planszy, altitude, driving, selection, dialog, routing, momenty) widzą wszystko, także `Off`;
(2) poziomy widoku `Exact/Coarse/Fogged/Hidden` po `camera.Detail{Sight, Fog, Coarse}`,
odległość od `Viewpoint` do najbliższego punktu bez zawijania, skala `ScaleAt` w pikselach
kamery (adapter rozdzielczości mnoży `Zoom`, więc spadek FPS przesuwa decyzję ticku), bez
histerezy, konsumowane osobno przez renderer świata (bez `Lay` i `Upright` przy Coarse), teren
(siatka co drugi róg), heksy (wierzchy), kafle planszy (shadery), wzrok, boxy; (3) staranność
gruntu po pierścieniach Czebyszewa od sektora `Focus` i sektora oka, pięć wariantów shadera,
własne F10 i dźwignia; (4) drabinki za budżetem klatki w kamerach (rozdzielczość), topografii
(care), świetle (cienie lamp, wypiek), atmosferze (chmury), każda z własnym `AdaptX`. Punkt
odniesienia, metryka, zawijanie i skala liczone po pięć razy inaczej; żadnego culla małych
figur; „płachta” planszy istnieje (`Miniature()`: albedo malarza 4×2 px na komórkę, wierzchy
heksów), ale rysuje nią tylko minimapa.

Decyzje użytkownika (2026-11-03): jedna drabina poziomów dla ticku i obrazu, na sztywno pięć
(`lod.L0..L4`, numery w kodzie, słowa w dokumentacji: L0 pełnia, L1 bez detalu, L2 kształt, L3
kolor, L4 nic: ani tykany, ani rysowany; `Off` i `Hidden` to jeden szczebel); poziom sektora z
odległości od kamery wyrażonej skalą ekranu (`ScaleAt` w pikselach pokazywanych, nie kamery:
oddalona kamera obniża poziom także pod kotwicą) i obcięty `Sight`; encja nosi jeden znacznik,
poziom ticku: najlepszy po obserwatorach (serwer bez kamery: L0 wszędzie), a każda kamera trzyma
w widoku poziom własny i renderer wybiera po nim; każdy system podaje tempo na każdy szczebel
(tablica, 0 stop), żaden bez; każdy rysowany rodzaj ma wersję na szczebel, domyślnie
wyprowadzoną przez render (średni kolor, płachta), zadeklarowaną, gdy autor chce; słowo `lod`,
pakiet korzenia importowany przez `camera`, `world` i `render` (`render` pluginów nie widzi);
logika w `world` (siatka, obserwatorzy, znaczniki, sen, widoki: biegnie w planie świata między
zegarem a widokami), obraz w `render`; mgła zostaje ciągłym zanikaniem, nie szczeblem; budżet
klatki tyka nie dotyka. Skutki przyjęte: kolizja i wzrok biegną do szczebla, który gra wybierze
(`Paces`), a nie „tylko pierwsze pasmo”; sektor poza kadrem tyka po skali w swoim najbliższym
punkcie (to, co za plecami, żyje), a w obrazie jest L4.

**Preludium** (integrator, na `feat/lod`, stare zachowanie za nowymi kontraktami):
`lod` (`Level uint8`, `L0..L4`, `Levels = 5`; `Ladder{Scales [Levels-1]float32, Sight, Fog}`:
progi skali malejąco w pikselach pokazywanych na jednostkę świata, `At(scale, dist) Level`:
najmniejsze k ze `scale ≥ Scales[k]`, L4 pod ostatnim albo za `Sight`; `Widened(f)`;
`Paces [Levels]uint8`: kroków na krok, `Paces[L0] == 1`, `Paces[L4] == 0`, sprawdzane w
`Check()`; gotowe `lod.Exact` {1,0,0,0,0}, `lod.Steady` {1,1,1,1,0}, `lod.Slowing(n...)`);
`camera.ScaleShown(cam, x, y, z)`: `ScaleAt / ResolutionOf`, a za okiem (0) skala punktu o tej
odległości przed okiem (kamera perspektywiczna daje ją z ogniskowej); `sector`: znaczniki
`L1..L4` w miejsce `Band2..Band5`/`Off` (`Off = L4` jako alias przejściowy znika z częścią 1),
`Observers interface{ Observers(each func(camera.Camera)) }` w miejsce `Anchors`,
`Keen{Level lod.Level}` (komponent, zapisywany);
`world.Config.Sectors` → `SectorsCfg{Buckets}` i `world.Config.LOD lod.Ladder` (drabina
świata; `Band.Reach`/`Pace` znikają: tempo jest systemu, zasięg skalą); `world.Plugin.Pace(p
lod.Paces) goke.Opt` (dziś `Policy`; trzy polityki jako gotowe tablice); `view.Level` typu
`lod.Level` z dzisiejszego liczenia (Exact→L0, Coarse i Fogged→L1, Hidden→L4), `View.Fog` bez
zmian, `Exact/Coarse/Fogged/Hidden` znikają, obraz poziomów kafli planszy z remapem do
dzisiejszych liczb shaderów do części 4; `cameras.Config.Detail` znika: kamera ma
`Sight` jako mnożnik drabiny świata (`camera.Sighted`, `SightOf`; zapisany jako mnożnik, stare
zapisy się łamią), Ctrl+=/− jak dziś, `world.Read.Sight(cam)` odległością dla aloft i minimapy;
za okiem `perspCamera.ScaleAt` daje skalę punktu o tej odległości przed okiem; dema expanse i
topography przepisane na `LOD: lod.Ladder{...}` i `Sectors{Buckets}`; cała sekwencja zielona.

**Część 1 `l/levels`** (world, cameras; po preludium): nadzorca sektorów liczy co tick poziom
sektora na obserwatora `Ladder.At(ScaleShown(najbliższy punkt, z zawijaniem), odległość od
Viewpoint)`, poziom ticku = minimum po obserwatorach (bez obserwatorów L0) i po encjach keen:
`sector.Keen{Level}` na encji (zapisywany) trzyma jej sektor i osiem sąsiednich na jawie
najwyżej na tym poziomie, póki żyje (pocisk w locie z `Keen{L2}`: kolizja biegnie, cele
budzą się przed nim, lot także poza Sight); do drobniejszego
od razu, do grubszego z histerezą (próg skali ×0,9 i pół sektora za `Sight`), znaczniki
`L1..L3` ruszane jak dziś, `L4` śpi jak dziś `Off` (listy snu, `restart`, budzenie);
stopniowanie jako niezmiennik (to, co dawały pierścienie care), tylko wśród rysowanych:
sąsiednie sektory oba pod L4 różnią się najwyżej o jeden poziom, L4 graniczy z czymkolwiek
(inaczej fala budziłaby trzy pierścienie za Sight); transformata odległości Czebyszewa z
zawijaniem po poziomach z histerezą co tick, wynik nigdy nie wraca do nich, a reporter liczy
podniesienia, by gra widziała progi za gęste dla boku sektora; z góry kamera widzi każdy punkt
kadru w jednej skali, więc w Sight jest jeden poziom i Sight jest jedynym pokrętłem ticku
(użytkownik 2026-11-03: expanse przepisany z Sight ~1000 j. zamiast pasm 750/2000); widok
kamery bierze poziom swojego obserwatora, L4 także poza kadrem, zbiera encje sektorów pod L4
(`view.nearest` z zawijaniem: torus expanse); `Pace(Paces)` buduje `Never[L4]` zawsze i na
L1..L3 `Never`/`Every(n)`/nic; sprawdzenie, że największe `n` razy krok nie przerzuca
najszybszej encji przez pół komórki (dziś `checkBands`); nakładka „Sectors drawn” i reporter
„Sectors” pokazują L0..L4; `Unbanded` zostaje (wszystko L0 w ticku, obraz po drabinie);
kamery podają `Observers` (bez `Whole` i minimapy); minimapa sprawdza zasięg przeciw `Sight`.
Testy: poziom z skali i odległości (kamera z góry: zoom; perspektywa: odległość i za okiem),
minimum po dwu obserwatorach, keen podnosi sektor i sąsiadów i puszcza po despawnie,
histereza w obie strony, sen i budzenie po L4, zawijanie, widok
poza kadrem L4 z tickiem żywym, stopniowanie (progi ciasne na małym sektorze dają schody po
jednym, licznik wyrównań), zapis i odczyt znaczników. Nie dotyka: renderera świata,
`render`, pluginów poza world i cameras.

**Część 2 `l/paces`** (każdy plugin z systemem po encjach; po preludium, równolegle z 1):
każde zapytanie o encje świata z `world.Plugin.Pace(paces)`: motion `Slowing` jak dziś
(krok rośnie o n), efekty i plany świata `Slowing`, gravity fall `Slowing`, plane `Exact`,
kolizja i wzrok `Paces` z L0..L2 po 1 (bitwa nie staje przy całej mapie w kadrze) i stop od
L3, pocisk `Steady` pełnym krokiem (nigdy `Slowing`: przeleciałby przez cel; grunt sprawdza
sam, trafia przez kontakty kolizji) i strzał spawnowany z `sector.Keen{L2}` na czas lotu,
traffic `Steady`; dziś bez polityki, dostają co najmniej `Steady` (nic na L4):
lampy światła (uśpiona lampa gaśnie), standing i occupancy planszy, altitude, driving,
selection, dialog, give, routing, strzelanie, znaki minimapy; komórki nie mają `Base`, więc
znaczników nie noszą i reguł komórek się nie tyka; momenty świata nie mają zapytania. Dzisiejsze
`Paced` (tempo pasma 4 z konfiguracji świata) to `Slowing(2, 4, 8)` w preludium. Testy: na L4
żaden system nie
czyta encji (test po pluginie przez `Install → Setup → RunPlan`), `Dt` na L1..L3 równe n
kroków w `Slowing`. Nie dotyka: tego, co system robi, tylko gdzie i jak często.

**Część 3 `l/looks`** (render, world atlas i renderer, topography billboards; po preludium,
równolegle z 1 i 2): wersja na szczebel w atlasie: `Slot.At(level lod.Level, look L)` deklaruje
wygląd od tego szczebla w dal aż do następnej deklaracji, L4 zawsze nic; bez deklaracji render
wyprowadza przy `Close`: L0 i L1 sprite (na L1 renderer gubi krawędzie `Lay` i `Upright`, jak
dziś Coarse), L2 plama średniego koloru sprite'a (jeden sprite jednolity na rodzaj, pod każdym
efektem ten sam), L3 nic (figura pod pikselem znika); renderer świata wybiera wersję po
`view.Level` w środku pudła, `FoggyLook` jak dziś; billboardy topografii to samo w reliefie;
`Styled` i `Under` działają na każdej wersji. Testy: tablica wersji po `Close` (zadeklarowana,
wyprowadzona, L4 pusta), renderer rysuje wersję szczebla (sprite ID w instancji), średni kolor
policzony z alfą. Nie dotyka: terenu, kafli, nieba.

**Część 4 `l/ground`** (topography terrain i hexes, board look i shadery; po 1 i 3): grunt po
poziomie sektora z widoku zamiast pierścieni: L0 pełnia (dzisiejsza care 4), L1 bez detalu
(care 3: bez brzegu, piany, wody biegnącej), L2 kształt (care 1 na siatce co drugi róg, bez
fal i cieni lamp), L3 kolor: wariant „płachta” na siatce zgrubnej z albedo malarza
(`Miniature`) jako jedynym kolorem (tekstura, nie osobny wachlarz: krawędź L2|L3 bez
pęknięcia; wachlarz z mniejszą liczbą trójkątów to następny etap, jeśli liczby go uzasadnią),
heksy jeden kwadrat z `Miniature` na sektor, L4 nic; spódnica do horyzontu zawsze jak L3;
kafle płaskiej planszy: obraz poziomów niesie `lod.Level`, pokrycia,
drogi i siatka na L0, kafel jednolitym kolorem od L2, nic na L4; pierścienie care,
`Config.Care`, `SetCare`, `AdaptCare`, dźwignia topografii (`Following/Coarser/Finer`), F10
„Full care out to” i nakładka care znikają bez następcy w topografii (zasada: jeden poziom na
sektor dla wszystkiego, nic nie przesuwa go dla siebie); mosty, drogi i to, co stoi, po
wersjach z części 3. Testy: wariant po
poziomie (golden obrazów terenu na każdym szczeblu i kafli), płachta na L3 zgodna z albedo,
nic na L4, hex i kwadrat tak samo. Nie dotyka: atmosfery (chmury czytają `Ladder.Sight` jak
dziś `Detail.Sight`), światła.

**Część 5 `l/budget`** (cameras, world view; po 1): jedno przesunięcie poziomu obrazu za
budżetem klatki w adapterze kamer: `Settings.Detail` (F10 Picture: „Detail follows the frame
budget”, domyślnie tak) i krok `Shift` 0..2 trzymany przez adapter; pod budżetem najpierw
poziom obrazu każdej kamery o jeden grubiej (widok liczy `min(L4, poziom + Shift)`, tick nie),
potem rozdzielczość; drobniej w odwrotnej kolejności, z dzisiejszym oknem i paleniem prób;
`Config.Levers` i `cameras.Lever` zostają dla światła i atmosfery do czasu, gdy i one czytają
poziom (następny etap); reporter „Picture” pokazuje `Shift`. Testy: adapter przesuwa poziom
przed rozdzielczością i cofa po, widok z przesunięciem, znaczniki ticku nietknięte.

Słowa szczebli jako kontrakt (`lod/doc.go`, tabela): co L0..L4 znaczy dla figury (sprite z
krawędziami i bryłą; sprite płaski; plama koloru; nic; nic), kafla (pokrycia, drogi, siatka;
kafel i drogi; kolor; kolor; nic), gruntu (pełnia; bez brzegu i wody biegnącej; siatka
zgrubna bez fal; płachta; nic), tego, co stoi (jak figura), chmur i cieni (następny etap).
Każdy plugin realizuje swoją kolumnę; żaden nie liczy poziomu ani nie przesuwa go dla siebie.

Kolejność scalania: preludium → 2 → 1 → 3 → 4 → 5; szwy: `view.Level` (1↔3↔4↔5), `Pace`
(1↔2).

**Pomiar** (integrator, po scaleniu, kopia bazowa v0.17.2 na zmianę, bezczynna maszyna,
5–10 min): `World_Tick` z sektorami i `World_Draw` (szum: ta sama praca, inne nazwy),
`Frame_Topography` (±3%: wariant z poziomu zamiast z pierścienia), sonda expanse (klatka i
GC: płachta na L3 zamiast setek sektorów siatki ma dać mniej instancji) i topography; zrzuty
klatki 120 w czterech ustawieniach kamery (blisko, średnio, cała wyspa, perspektywa znad
jednostki) bazowa przeciw drzewu: różnice tylko tam, gdzie poziom się zmienił. Deterministycznie
w częściach: liczniki poziomów z reportera, zrzuty golden.

**Dokumenty** (integrator): `lod/doc.go` (słowa szczebli), `doc.go` korzenia (grafika pakietów:
`lod` pod `camera`), `plugins/world/doc.go` i `view/doc.go`, `plugins/cameras/doc.go`,
`render/doc.go`; skille `gram-plugin` (`Paces` obowiązkowe), `gram-render` (wersje, `Slot.At`),
`gram-plugins` (wpis world: poziomy zamiast pasm; topography: grunt po poziomie; cameras:
przesunięcie), `gram-game`
(`LOD` w konfiguracji świata), `gram-perf-tick` i `gram-perf-picture` (niezmienniki);
`CLAUDE.md` reguła domu: „jedna drabina LOD: poziom sektora liczy świat, ten sam dla
wszystkiego w nim, tempo na szczebel podaje każdy system, wersję na szczebel każdy rysowany
rodzaj po tabeli słów, nikt nie liczy własnej odległości ani nie przesuwa poziomu dla siebie”;
`CHANGELOG.md` v0.18.0 (`feat!`: `Sectors{Bands}` → `LOD`, `Detail` → `Sight`, `Policy` →
`Paces`, znaczniki w zapisach); `README` jeśli minimal dotyka. Roadmapa: „fewer rays or
sampling far off (with the LOD)” zostaje, bo wzrok nie bierze poziomu do skanu.

**Co nie wchodzi** (następny etap, po tym): chmury i cienie po poziomie (bryły do L1, płachty
od L2; cienie lamp na L0, wypiek zgrubny od L1), a z nimi koniec `AdaptShadows`, `AdaptClouds`,
`Config.Levers` i `cameras.Lever` (zostaje samo przesunięcie z części 5); wzrok próbkujący
rzadziej na L1+; wersja L3 figur jako piksel w płachcie (dziś nic); mipmapy atlasu jako wersje
pośrednie; rozmycie na granicy wariantów gruntu (poziom na wierzchołek, pas przy krawędzi
sektora mieszający dwa warianty jak pióro warstw; dziś żadnego nie ma, granice care są ostre),
gdy zrzuty etapu pokażą, że granice poziomów gryzą wzrok.

## LOD: wynik (2026-11-03, gałąź `feat/lod`, baza `43f0f4b2` v0.17.2)

Wykonanie: preludium agentem Opus w klonie (`89ebb2a4`, `8b181c45` Sight expanse w jednostkach
świata), części agentami Opus w worktree'ach od `8b181c45`: `l/paces` (`2d30aeba`, `df506d46`:
`lod.Contact` dla kolizji, wzroku i grawitacji bocznej), `l/levels` (`fcc31666`), `l/looks`
(`d6b00107`, `299a3181`: encja na brzegu kadru czyta poziom środka wciągniętego w kadr),
`l/budget` od `fcc31666` (`9ed05672`, `46c41e02`: przesunięcie najwyżej do L3), `l/ground` od
scalenia 1–3 (`aa5c04f0`, `a56946b3`: siatka terenu tylko na L0); scalenia `--no-ff` w
kolejności 1, 2, 3, 5, 4 (`44405491`, `e1a618b5`, `2a915e36`, `1ab0e323`, `597a6691`), bez
konfliktów (szwy rozdzielne). Sekwencja zielona z SIMD, 23 dema po 5 s zdrowe.

Odstępstwa od planu, przyjęte: `camera.InFront`/`Fronted` zamiast skali 0 za okiem (pięciu
konsumentów czytało 0 jako „za okiem”); `Grid.Nearest` eksportowany; `aloft.Renderer.Sight`
setter; `view.Level` zniknął w całości (`lod.Level`); stopniowanie tylko wśród rysowanych (L4
graniczy z czymkolwiek: fala budziłaby trzy pierścienie za Sight), transformata Czebyszewa trzema
przebiegami, wynik nie wraca do poziomów z histerezą; stan obserwatora po kamerze, nie po
indeksie; komórki bez `Base` nie noszą znaczników (reguły komórek nietknięte), momenty świata bez
zapytania; lookupy po tożsamości (`Seek`, partner rozmowy, zaznaczony poza Sight) zostają `All()`;
wersja wyglądu to para (sprite, drawn), bo sprite 0 istnieje; blob dyskiem po pokryciu; L0 i
wyglądy czasowe odrzucone jako wersje; płachta L3 to wariant CARE 0 z albedo jako jedynym
kolorem, oświetlony ambientem i rozproszonym słońcem (inaczej świeciłby nocą obok L2); heksy na
L3 jeden kwadrat na sektor; kafle materiałowe na L2+ w kolorze środka komórki, nadal
animowanym (kontrakt materiału nie ma koloru bazowego); dźwignie wracają pierwsze także w dół
(`f7068bcc`, decyzja użytkownika), przesunięcie tylko w świecie z sektorami; selekcja omija
śpiących (uśpiona jednostka zostaje zaznaczona do przebudzenia: widoczna zmiana, do oceny).

Sprawdzenia deterministyczne (części): expanse na starcie `L0 16, L4 1008`, zoom przez próg
przemarkowuje ~1400 encji naraz i z powrotem; topografia `L0 736, L4 64`, perspektywa z drabiną
`{0.5, 0.25, 0.12, 0}` daje schody L0..L3 i podniesienia (6–58); testy L4 „nic nie czyta” w
dziewięciu pluginach; zrzuty klatki 120 bazy przeciw drzewu: 0 pikseli różnicy przy wszystkich
sektorach L0 (topografia domyślna, mostek z bliska, board/flat, expanse), różnice tylko tam,
gdzie poziom się zmienił (blob mostu na L2, brak mostu na L3 z cieniem wypieczonym, 129 622 px
na widoku wyspy po zdjęciu siatki z L1/L2); adapter: shift 1 → 2 → rozdzielczość pod budżetem
1 ms, nic przy `Detail{Follows: false}`.

Pomiar (20:30–20:36, 6 min maszyny; baza `43f0f4b2` jako worktree z `GOWORK=off`, drzewo tak
samo, na zmianę, 2 rundy, `-count=1`, `BENCHTIME=1s`; skrypt i logi w scratchpadzie sesji):
`Sectors_Expanse/banded` (`-cpu=1`) 1,54 → 1,52 ms (−1,5% min, 0% mediana; 1 → 0 alokacji),
`unbanded` 12,2 → 16,0 ms (+31%, 0 → 1 alokacji): profil po obu stronach pokazuje w drzewie 31%
ticku w `spatial.Grid.index`/`Grid.add` aabbworlda, indeksie przestrzeni budowanym pod zapytanie
widoku prawdziwej kamery, którego bench bazy nie miał (kotwice jako punkty, bez widoku; demo
zawsze go płaciło); `World_Draw` +2,3…+4,3% (wersja na warstwę i poziom środka w granicach
widoku), `World_Publish` −0,8…+3,6% (szum, 0 alokacji); `Frame_Topography` iso/persp/layered/
footed −0,1…−1,0% (bez zmian); nowe `iso,level=0..3` 15,3 / 13,5 / 12,0 / 9,0 ms przy readbacku
9,25 ms: każdy poziom gruntu oszczędza 1,5–2,7 ms, płachta L3 niemal cały przebieg gruntu; FPS
dem bez vsync 20 s: expanse 792 → 840 (tick 285 → 210 µs), topografia 155 → 159. Zrzuty i
liczniki: części (wyżej).

Następny etap: chmury i cienie po poziomie (bryły do L1, płachty od L2; cienie lamp na L0, wypiek
zgrubny od L1) i z nimi koniec `AdaptShadows`, `AdaptClouds`, `Config.Levers`, `cameras.Lever`
(zostaje samo przesunięcie); wzrok rzadziej na L1+; przemarkowanie rozłożone na ticki (zoom z
góry przez próg znakuje wszystkie żywe encje naraz); rozmycie na granicy wariantów gruntu, gdy
zrzuty pokażą, że gryzie; wachlarz płachty, jeśli liczba trójkątów zacznie się liczyć;
**generalizacja figur** (użytkownik 2026-11-03, agregacja domyślna): na L2 plama jako punkt
(pozycja, kolor, rozmiar ≥ 1 px) pisany do instancji przy publikacji z pominięciem `Look`
(dziś ~85 ns na pudło w `World_Draw`), z akumulacją alfy pod pikselem zamiast pomijania; na L3
agregacja do obrazu sektora (kolor i pokrycie, teksel na kilka jednostek, splatting punktów do
małego celu albo suma przy publikacji), jeden quad na sektor jak płachta albedo, domyślna dla
każdej figury bez własnej wersji L3 w miejsce dzisiejszego „nic”; bez nowych encji i stanu
(wygląd to rysowanie; typizacja, ikona z licznikiem, zostaje grą przez `At(L3, ikona)`);
rzedzenie po stałym haszu id (`f` ze skali, Töpfer `f ∝ skala` albo `f ∝ skala²`, ważne encje
poza losowaniem) jako narzędzie gry z większą liczbą encji niż pikseli; **agregaty**
(użytkownik 2026-11-03; modelowanie wielorozdzielcze: agregacja i dezagregacja, jak X4, Mount &
Blade, OneSAF): agregat (las, armia) jest encją zwyczajnego rodzaju z własnymi efektami i
regułami ceremony, składniki (drzewa, żołnierze) są encjami tylko na L0–L1 i są jedyną prawdą
tam, agregat jedyną prawdą od L2; silnik daje moment przejścia poziomu na encji („rozwiń” przy
zejściu do L1, „zwiń” przy wejściu na L2, z mocniejszą histerezą), relację części, kontrakt
zwiń/rozwiń z testem odwracalności (zachowanie ilości), `Keen` agregatu w interakcji (rozwinięcie
łańcuchowe); gra pisze oba zestawy reguł i oba kierunki (rozwinięcie deterministyczne z ziarna i
komórki plus delta zmian); rozjazd wierności przyjęty, kalibracja reguły abstrakcyjnej; dt wynika
z tego, co na poziomie istnieje (żołnierz co krok, armia co 4–8 z planów), `Paces` zostaje osi
systemu, nie poziomu; pytanie etapu: tempo na rodzaj, gdy reguła abstrakcyjna potrzebuje częściej
niż gospodarz planów; szwy w perspektywie (2026-11-03): prawda na agregat, nie na sektor
(rozwinięci składniki mogą stać w sektorze L2: istnieją, plamy, tempo L2), agregaty punktowe
(armia, zwijana cała) i polowe (las, zwijany po sektorze: drzewa komórki istnieją przy L1 jej
sektora), interakcja rozwija grubszą stronę (`Keen` celu agregatu niesie L1, nie L2), wersja L2
z tego samego materiału co L1 (impostory), granica wariantów to szew obrazu; **świat bez
obserwatora** na poziomie z drabiny (`Ladder.Unobserved`,
domyślnie L2, bench i testy ustawiają L0 wprost) zamiast dzisiejszego L0; demo: las z drzewami i
armia z żołnierzami zwijane przy oddaleniu; selekcja a śpiący
(uśpiona jednostka zostaje zaznaczona do przebudzenia);
kafle materiałowe na L2+ nadal animowane (kolor bazowy w kontrakcie materiału).

## Generalizacja figur: plama bez `Look`, kolor w grunt, rzedzenie, świat bez obserwatora (plan 2026-11-03, gałąź `feat/lod-figures`, litera `g`, v0.19.0)

Analiza (2026-11-03, po etapie LOD): daleka encja kosztuje CPU, nie GPU: `World_Draw` ~85 ns na
pudło, trzy piąte w `Look` (sprite, kopie zawijania, `Frame.At`, atlas), reszta w kompozycji;
sto tysięcy plam na L2 to ~8,5 ms CPU, podczas gdy GPU rysuje sto tysięcy punktów za nic. Na L3
figura znika (`Nothing`), choć słowo szczebla to „kolor”; płachta gruntu na L3 i obraz minimapy
(`Miniature`, znaki rodzajów) już są agregacją w obraz. Pominięcie „co dziewiąta encja” odpada:
reprezentant migocze na granicach wiader, tłum traci gęstość, ważne encje giną losowo; właściwa
operacja z kartografii (generalizacja: pominięcie, uproszczenie, agregacja, typizacja; prawo
Töpfera `n_f = n_a·√(M_a/M_f)`) i z gier (dithering roślinności po haszu, ikony w RTS, klastry
map) to agregacja, a pominięcie tylko stabilnym haszem. Świat bez obserwatora jest dziś L0
wszędzie, czyli najdroższy; serwer chce odwrotnie.

Decyzje użytkownika (2026-11-03): agregacja domyślna dla każdej figury bez własnej wersji L3
(nic nie znika, wszystko staje się kolorem); bez nowych encji i stanu (wygląd to rysowanie;
typizacja, ikona z licznikiem, zostaje grą przez `At(L3, ikona)`); rzedzenie po haszu
narzędziem gry; poziom świata bez obserwatora z drabiny, domyślnie L2, bench i testy L0 wprost;
`Paces` zostaje osią systemu (dt wynika z tego, co na poziomie istnieje). Agregaty-encje (las,
armia) to etap następny, stojący na tym.

**Preludium** (agent Opus w klonie, od `main` po scaleniu `feat/lod`, inaczej od `feat/lod`):
`lod.Ladder.Unobserved lod.Fallback` (`type Fallback uint8`, zero = L2: `UnobservedL2`, dalej
`UnobservedL0`, `UnobservedL1`, `UnobservedL3`, `UnobservedL4`; metoda `Level()`), świat bez
obserwatora tyka na tym poziomie zamiast L0 (`sectors.ticked`), `Unbanded` bez zmian; bench i
testy, które liczą na L0 bez kamery (`bench/world_test.go`, `board_test.go`, `sectors_test.go`
nie: ma kamerę; testy pluginów budujące świat z sektorami bez obserwatora), ustawiają
`UnobservedL0` wprost; `lod/doc.go` tabela: figura L3 „kolor w płachcie gruntu”;
`world.Read.Aggregate() (img *render.Image, perUnit float32)` jako kontrakt (w preludium obraz
pusty o rozmiarze świata w tekselach `Config.LOD.Texel` jednostek, zero = bok sektora/64),
`lod.Ladder.Keep [Levels]float32` (udział encji publikowanych na poziomie, zero = 1, L4 bez
znaczenia) z `Check` (0..1); `Slot.Always()` w atlasie (rodzaj poza rzedzeniem); dema bez
zmian poza `Unobserved` gdzie trzeba; sekwencja zielona.

**Część A `g/points`** (world publisher i renderer, `render/internal/instanced`, `plugins/world/
internal/look`; po preludium): na L2 figura jest **punktem bez `Look`**: przy kompozycji
encja na L2 nie przechodzi `Sprite/Lie/Lay/Upright` ani `Cast`/`Lit`, lecz pisze jedną
instancję wprost (środek pudła przez kamerę raz, z kopiami zawijania policzonymi raz na klatkę
dla kamery, rozmiar `max(pudło w px, 1 px)`, sprite bloba, kolor i alfa: alfa = pokrycie
piksela przez pudło pod 1 px, by subpikselowe plamy sumowały się w gęstość zamiast migotać),
cel: ≤ 20 ns na encję L2 w `World_Draw` (nowe rungi `level=2`); billboardy topografii na L2 to
ten sam punkt (plama nie stoi); mgła jak dziś; **rzedzenie**: przy publikacji encja sektora na
poziomie k wchodzi do rekordów, gdy `hash(id)·2⁻⁶⁴ < Keep[k]` (hasz stały po id: ta sama encja
znika pierwsza, bez migotania), chyba że nosi `sector.Keen`, jest rodzaju `Always()` albo ma
zadeklarowaną wersję na tym poziomie; pominięta nie kosztuje rekordu. Testy: instancja punktu
(pozycja, rozmiar ≥ 1 px, alfa pokrycia) z `export_test`; zero alokacji; rzedzenie
deterministyczne (ten sam zbiór przy tym samym `Keep`, nadzbiór przy większym), wyjątki;
zrzuty: L0 i L1 piksel w piksel z bazą, L2 różnice tylko w plamach (rozmiar i alfa).

**Część B `g/aggregate`** (world publisher: obraz; topography terrain i hexes; board look;
po A, bo publisher): na L3 figura jest **kolorem w gruncie**: publisher sumuje do obrazu
`Aggregate` (RGBA, teksel `Config.LOD.Texel`) kolor bloba i pokrycie każdej encji sektora na L3
widzianego przez którąkolwiek kamerę (teksele tych sektorów zerowane co klatkę; encje
z zadeklarowaną wersją L3 rysowane nią zamiast sumy; w świecie bez sektorów nic), obraz
publikowany raz na klatkę jak znaki minimapy, przesyłany tylko w prostokątach sektorów L3;
grunt czyta go: wariant płachty terenu (L3) miesza kolor agregatu po pokryciu nad albedo
(piąty slot obrazu albo pakowanie w slot `Kinds` tam, gdzie płachta go nie czyta: agent
decyduje po `render.Direct.Images`), kwadrat heksów na L3 tak samo, kafle płaskiej planszy od
L3 (`tiles_plain` z próbką agregatu), spódnica nie; `lod/doc.go` wiersz figury L3 prawdziwy.
Testy: obraz po publikacji (encja L3 daje teksel koloru i pokrycia, L2 nic, sektor zerowany po
odejściu), golden `gputest` płachty z agregatem (plama koloru nad albedo), kafle; zrzuty: L3 w
expanse (plamy pudeł w kolorze gruntu) i topografia widok wyspy.

Kolejność: preludium → A → B. Szwy: `publisher.go` (A↔B, więc sekwencyjnie), `Read.Aggregate`
(preludium↔B), shadery gruntu (B sam).

**Pomiar** (integrator, ≤10 min, baza = `main` po v0.18.0 na zmianę z drzewem): `World_Draw`
rungi istniejące (±3%) i nowe `level=2`/`level=3` (same, cel ≤ 20 ns/encja L2, L3 koszt sumy
≤ 10 ns/encja); `Sectors_Expanse` (`-cpu=1`, `UnobservedL0` w benchu: bez zmian); `World_Publish`
(rzedzenie `Keep` 1: ±2%); `Frame_Topography/iso,level=3` (próbka agregatu w płachcie ≤ +0,3 ms);
FPS expanse oddalony do L2 w całym kadrze (reżim najcięższy: dziś ~100 tys. plam przez `Look`),
baza przeciw drzewu 20 s; zrzuty jak w częściach.

**Dokumenty** (integrator): `lod/doc.go`, `plugins/world/doc.go` (publikacja: punkty, agregat,
rzedzenie), `plugins/topography/doc.go`, `plugins/board/doc.go`, skille `gram-render`
(punkt bez `Look`, agregat), `gram-plugins` (world, topography, board), `gram-game`
(`Unobserved`, `Keep`, `Always`), `gram-perf-picture`/`gram-perf-tick` (niezmienniki: L2 bez
`Look`, obraz agregatu raz na klatkę, świat bez obserwatora L2), `CLAUDE.md` (reguła LOD: zdanie
o agregacji domyślnej i świecie bez obserwatora), `CHANGELOG.md` v0.19.0 (`feat`; `Unobserved`
zmienia zachowanie świata bez kamery), `BENCHMARKS.md`, wynik tu.

**Co nie wchodzi**: agregaty-encje (las, armia: moment przejścia poziomu na encji, relacja części,
zwiń/rozwiń, `Keen` L1 celu, demo), chmury i cienie po poziomie z końcem dźwigni, przemarkowanie
rozłożone, rozmycie granic, wachlarz płachty, selekcja a śpiący, kolor bazowy materiału.

## Generalizacja figur: wynik (2026-11-04, gałąź `feat/lod-figures`, baza `2fe2cbed` v0.18.0)

Wykonanie: preludium agentem Opus w klonie (`85d2c9c9`: `Unobserved`, `Keep`, `Texel`, kontrakt
`Read.Aggregate`, `Slot.Always`), części agentami Opus w worktree'ach: `g/points` od preludium
(`a3dcf1ee`, scalone `b3ea71ee`), `g/aggregate` od tego scalenia (`52a13136` suma;
`d70b6713` `world.Plugin.Aggregates()`: świat sumuje tylko dla gruntu, który o to poprosił;
`23f719f1` grunt nie próbkuje pustego obrazu; scalone `327e9692`, `0f933e29`). Sekwencja
zielona z SIMD, 24 dema po 5 s zdrowe.

Odstępstwa od planu, przyjęte: punkt L2 kosztuje ~40 ns na encję zamiast ≤ 20 (odczyt poziomu i
rekordów; rekord zwarty dla L2 to następny krok); suma L3 ~90 ns na encję zamiast ≤ 10 (pudło
20 na tekselu 16 to ~5 tekseli, sam dodatek ~25 ns na teksel, Go rozlewa rejestry w pętli
pokrycia; stały punkt i szybszy floor nie pomogły; do spróbowania: jeden teksel pod środkiem);
**sumowanie tylko dla czytelnika** (decyzja integratora: przesunięcie budżetu do L3 ma klatkę
tanić, nie drożyć): renderer gruntu woła raz `Aggregates()` w konstruktorze (topografia,
plansza), świat bez czytelnika (ekspansja) nie ma obrazu, nie sumuje i rysuje na L3 nic, jak
baza; obraz w kaflach 64×64 tekseli (nie po sektorach: przy `Texel` innym niż bok/64 sektory
nie leżą na siatce tekseli), podwójnie buforowany, istnieją tylko kafle zapisane, przesyłane raz
na klatkę w `Compose` świata te, których sygnatura się zmieniła; obraz klatki `"Aggregate"`
(siódmy z ośmiu) zamiast piątego slotu; `Read.Aggregate` daje skalę 0, gdy obraz pusty, a trzy
shadery gałęzią omijają próbkę; materiały i rodzaje z własną wersją L3 albo `Nothing(L3)`
rysowane po swojemu; encja widziana na L3 przez jedną kamerę, a drobniej przez drugą, jest
sumowana i zapisana; `Grid.Of` obcina bez float64; `Read.Aggregate` czyta przez wskaźnik
(kopia struktury ścigała się z publikacją).

Sprawdzenia deterministyczne (części): zrzuty klatki 120 bazy przeciw drzewu: L0 i L1 piksel w
piksel; ekspansja 0 px różnicy na każdym zoomie (bez gruntu); topografia zoom 0,3 (63 sektory
L3) 14 px do 6, zoom 0,08 (247 sektorów L3) 2015 px do 130 w 22 skupiskach, każde na figurze
(jednostki przy wsi, odcinki mostów jako brązowe kropki nad rzekami, gdzie baza nie rysowała
nic); kafle planszy i płachty heksów tylko testami GPU (żadne demo z sektorami ich nie ćwiczy);
publikacja bez alokacji w stanie ustalonym, ruchome figury alokują w `gpu.put` (~4 KiB na kafel).

Pomiar (integrator, baza `main` 2fe2cbed na zmianę z drzewem, 2 rundy × `-count=1`, ~2 min,
plus płachta L3 osobno i FPS, razem ~7 min):

| Bench | Baza | Drzewo | Zmiana |
|:---|---:|---:|---:|
| `World_Draw` rungi dotychczasowe | | | −4…−6% |
| `World_Publish` rungi dotychczasowe | | | +2…+6% (część A: hasz i poziom na encję; B ≤ 1%) |
| `World_Draw/5000, level=2` | — | 199 µs | ~40 ns na punkt (przez `Look` ~85) |
| `World_Publish/5000, level=2, keep=0.25` | — | 99 µs | 65 µs bez rzedzenia: hasz ~7 ns na encję, rysowanie −150 µs |
| `World_Publish/5000, level=3` | — | 460 µs | ~90 ns na pudło; `unread` 65 µs |
| `World_Draw/5000, level=3` | — | 0,3 µs | nic do rysowania; `unread` 78 µs |
| `Sectors_Expanse` banded / unbanded | 1,52 / 15,8 ms | 1,52 / 15,4 ms | −1 / −3% |
| `Frame_Topography/iso,level=3` | 9,02 ms | 9,30 ms | +3,1% (przed `23f719f1` +6,3%) |
| FPS ekspansji na L2 (drabina `{0.5, 0.4, 0.3, 0}`, zoom 0,35, 16 sektorów L2, 2 biegi) | 836 / 849 | 919 / 886 | rysowanie 580 → 480–490 µs, tick 200 µs bez zmian |

Co zostało: rekord zwarty dla L2 (cel 20 ns), tańszy hasz rzedzenia (dziś droższy niż rekord,
opłaca się tylko przez rysowanie), jeden teksel pod środkiem albo pętla pokrycia bez rozlewu,
alokacja `gpu.put` przy ruchomych kaflach, demo z sektorami na płaskiej planszy i heksach
(ćwiczące kafle i płachty heksów), kolor bazowy materiału, selekcja a śpiący; etap `b` niżej.

## Agregaty: nawigacja po poziomie, kontrakt zwiń/rozwiń, las i armia (plan 2026-11-03, gałąź `feat/aggregates`, litera `b`, v0.20.0, po etapie `g`; hpastar v1.5.0)

Analiza (2026-11-03, agent Plan przeciw kodowi): poziom ticku sektora leży w `sectors.level`
i w znacznikach encji (`sector.L1..L4`), `world.Read` nie daje go w punkcie; regiony poziomu 1
hierarchii HPA* to na kwadratowej planszy te same kwadraty co sektory (`Hierarchy.Side` zero →
`sectorSide`), gdzie indziej bok 16; `From` (64 komórki) decyduje o wspinaczce po długości;
hpastar doprecyzowuje leniwie po liczbie stanów, nie po predykacie regionów, a trasa
abstrakcyjna (`q.path`) i koszty skoków są wewnętrzne; `tail` traffic rezerwuje `Leg` co krok z
czytaniem terenu i zajętości, `head` łamie trasę, gdy komórka nie dopuszcza; udział tempa
planszy daje 0 na litej komórce, kolizja odpycha od gruntu (`solidField`) na `lod.Contact`;
`MoveOrder` zapisywany cały, hierarchia odbudowywana z budżetem (nie zapisywana).

Decyzje użytkownika (2026-11-03): świat ma poziomy abstrakcji zależne od zoomu także w
zachowaniu (drzewo / płachta drzew / las; żołnierze / armia); poziom LOD związany z hierarchią
HPA*: L0–L1 komórki, L2 graf regionów poziomu 1 (sektory), L3 wyższy poziom hierarchii (w tej
części L2); nawigacja po poziomie jest **częścią pierwszą** etapu agregatów; agregat zwyczajną
encją z regułami ceremony, składniki tylko na L0–L1 i tam jedyna prawda, agregat jedyna prawda
od L2, prawda na agregat nie na sektor, agregaty punktowe i polowe, interakcja rozwija grubszą
stronę (`Keen` celu L1), `Paces` osią systemu, nic z gry w ceremony. Założenia planu przyjęte z
zatwierdzeniem: od L2 grunt jest kształtem (kolizja nie odpycha od gruntu w chunkach `L2`/`L3`,
udział planszy nigdy nie odmawia), hpastar dostaje `Search.Abstract`. **Pierwsze założenie
odwrócone 2026-11-04** (uwaga użytkownika: w widoku z boku podłoga to komórki lite trzymane
polem kolizji, a grawitacja w poprzek biegnie na L2, więc bez pola bohater spadłby pod planszę):
nie po poziomie chunka, lecz po ciele. Kolizja działa jak dziś dla wszystkiego; z kolizji wypada
tylko ciało ze znakiem drogi abstrakcyjnej `sector.Abstract{}` (zerowy komponent świata obok
`sector.Keen`, zapisywany; nie `unit.States`, bo plansza importuje kolizję), kładziony przez
traffic na czas odcinka abstrakcyjnego i zdejmowany przy zejściu do L1 albo u celu: kolizja
wyklucza je z zapytania jak śpiących na L4 (ani ściany, ani ciała go nie pchają, ono nic nie
pcha), plansza daje mu udział tempa 1. Spotkanie dwu ciał abstrakcyjnych to moment świata
`world.Met{A, B}` w części 2 (odbiorca: reguły gry, `Keen{L1}`). Cały etap jednym PR, v0.20.0
(decyzja użytkownika 2026-11-04).

**Preludium** (`feat/aggregates` od `main` po `g`): hpastar `feat/abstract`: `(*Search[T])
Abstract(from, to T, dst []T, costs []float64) (int, bool)` (po `q.path` stany poziomu 1 na
granicach klastrów bez `intra1`, koszty skoków z `startCost`/`intra`/`edgesOut`/`goalCost`;
test: podciąg `Path`, Σ kosztów równa), tag użytkownika, `go.mod`; gram: `world.Read.LevelAt(x,
y) lod.Level` (L0 bez sektorów i przy `Unbanded`), `bodies.Member.Level` z własnych znaczników
(`Optional` `L2`/`L3`, `Present` raz na chunk), `order.Abstract{Cells [16]cell.ID, Costs
[16]float32, From, Length, Index}` w `MoveOrder` (zapis za darmo), `Counts` `Abstracts`,
`Snaps`, `Dropped`.

**Część 1 `b/navigation`**: routing: ask abstrakcyjny gdy `Member.Level >= L2` i hierarchia
włączona, bez pola i bez `FindPath` (hierarchia nie `Ready` → ask czeka, nie liczy się do
`TargetWaitTimeout`), `asked.abstract` rozdziela odpowiedzi pamiętane, `Finder.Abstract` z
preambułą `Climb`, `From` podłogą L0–L1, para za bliska jednym odcinkiem z udziałem 1; traffic:
`headAbstract` (zwolnienie `Leg`, `Path.Length = 0`, `At.Cell` bez zajętości, ask gdy trzeba),
`tailAbstract` (prosto na środek `Cells[Index]`, `MaxSpeed · min(1, |odcinek| / (Costs · span))`,
`passed` → `Index++`, bez `Trail`, kolejki, pola, szyku, `keep.Steer`); zejście pod L2 eager:
`Finder.Snap` (`breadthFirst` 64, bez trasy) na dopuszczoną komórkę, `space.Move`, `Enter`,
świeży ask (odtwarza prefiks: ta sama hierarchia, te same remisy), bez wolnej → `Lost`; wejście
na L2 leniwe (kawałek `Path` do końca); grupy na L2+ jako jednostki (`party` false, `Place 0`,
bez `Trail`); heksy/bok 16: poziom z własnego znacznika, L2 → poziom 1; znak drogi
abstrakcyjnej `sector.Abstract` kładziony w `headAbstract`, zdejmowany przy snapie, u celu i gdy
rozkaz ginie; kolizja `Exclude[sector.Abstract]` w zapytaniu ciał (jak `Exclude[sector.L4]`),
plansza `share` 1 dla ciała ze znakiem. Testy przez rig `newNavWorldIn` z sektorami i `observers`: ruch po wejściach bez
`Searches`, `Leg`, `Enter`; czas skoku; snap przy zejściu; zwolnienie `Leg` przy wejściu; ask
czeka na hierarchię; odpowiedzi rozdzielone; grupa jako jednostki; kolizja i plansza po
znaczniku; `routes.golden` nietknięte. Benche: `Navigation_Tick/abstract,1000`,
`Routing_Abstract`. Sonda topografii: `Counts`, poziomy, `Abstract Length/Index`, `Builds`.

**Część 2 `b/contract`** (świat, `lod`): `world.Aggregate{}` na szablonie rodzaju-agregatu,
`world.PartOf{Whole}` na składnikach (zapisywane); momenty świata `Unfold{ID, At}` (sektor
agregatu schodzi do L1) i `Fold{ID}` (wchodzi na L2 i trwa `Ladder.Fold` sekund, zero 3 s),
słowo `Parts(n)`, reguły gry do nich (rozwinięcie `Spawn` składników ze stanu, zwinięcie suma w
efekty i `Despawn`); `lod.Seed(whole, cell)` dla rozwinięcia deterministycznego, różnice w
efektach agregatu; test odwracalności zwiń ∘ rozwiń przez `Install → Setup → RunPlan`; `Keen{L1}`
celu-agregatu (bullet, `MoveTo`); `order.Abstract` lidera ↔ agregat przy zwijaniu i rozwijaniu.

**Część 3 `b/field`**: `world.Field` jako kształt agregatu nad komórkami (gęstość na komórkę),
zwijany po sektorze (`Unfold{ID, Sector}`/`Fold{ID, Sector}`), drzewa komórek sektora na L1,
wersja L2 lasu płachtą z tego samego materiału (`Slot.At(L2, …)`), L3 kolor z `g`.

**Część 4 `b/demo`**: `examples/board/forest-army`: las polowy i armia punktowa na płaskiej
planszy z sektorami i pełną drabiną; armia po grafie abstrakcyjnym na L2, rozwinięta w żołnierzy
na L1 z `Abstract` lidera; bitwa abstrakcyjna (ubytki regułą agregatu) i szczegółowa.

Kolejność: preludium → 1 → 2 → 3 → 4. Pomiar ≤10 min: `Navigation_Tick` (± i `abstract,1000`),
`Routing_Long` i `Routing_Abstract`, `Collision_Tick`, `Board_Altitude`, sonda topografii z
kamerą na L2 (jednostki abstrakcyjne, `Snaps` przy zbliżeniu), demo przy zoomie przez próg
zwijania. Dokumenty: `navigation/doc.go` „# Levels”, `world/doc.go`, `lod/doc.go` (wiersz
zachowania), skille navigation/world/collision/board/`gram-perf-navigation`/`gram-game`/
`gram-plugin`, hpastar skill, `CLAUDE.md` reguła LOD, `CHANGELOG.md` v0.20.0, `BENCHMARKS.md`.

**Co nie wchodzi**: L3 na wyższym poziomie hierarchii (parametr `level` w hpastar), rysowanie
tras abstrakcyjnych, ponowne sprawdzanie tras abstrakcyjnych po zmianie przejezdności, pola i
szyki na L2+, chmury i cienie po poziomie, przemarkowanie rozłożone, rozmycie granic, selekcja a
śpiący.

## Agregaty: wynik (2026-11-04, gałąź `feat/aggregates`, baza `aa9c5ce0` v0.19.1, hpastar v1.5.0)

Wykonanie: hpastar `Search.Abstract` agentem na `feat/abstract` (PR #8, scalone, tag v1.5.0 na
słowo użytkownika); preludium agentem w klonie (`ac1af1de`: `sector.Abstract`, `Read.LevelAt`,
`Member.Level`, `order.Abstract`, `Counts`); części agentami w worktree'ach, scalane `--no-ff`
w kolejności: `b/navigation` (`5dbcae9e`, `d3c1bb5d`, `bb03f035`; poprawka zejścia `9c2fc266`
na słowo użytkownika: komórka w udziale kosztu doprecyzowanej trasy odcinka, nie rzut prostej),
`b/contract` (`27eb34a4`, `da5bdf23`, `0a4acb16`), `b/field` (`5a6bd41c`, `e476d12a`),
`b/seams` (`d2864e5f`: `Keen{L1}` na agregacie wskazanym przez `To`, `Inherit`), `b/fieldlook`
(`5fe010a8`, `11490237`, `88e77a73`), `b/demo` (`d9708a14`, `e11beeff`, `5b9c0401`), runda
poprawek `b/fixes` po odrzuceniu dema przez użytkownika (`2f0e74a6`, `c1d94ec3`, `b0b54cf4`,
`f7a2fe60`, `8dfd7ab0`, `9ac30fe4`, `d8c9d555`, `ecf75820`, `df2143ae`), `b/publish` (koszt pól
zdjęty ze zwykłej ścieżki publikacji). Sekwencja zielona z SIMD, 25 dem po 5 s zdrowe.

Decyzje w trakcie (użytkownik): nie po poziomie chunka, lecz po ciele (`sector.Abstract`);
jeden PR; zejście na trasę doprecyzowaną („L2 to termodynamika, L0 dynamika”: zejście realizuje
stan gruby w drobnym, wejście sumuje drobny w gruby; reguła LOD w `CLAUDE.md`); rozwinięty
agregat niewidoczny i stojący, niezależnie od poziomu obrazu; spotkanie na L2 bez `Keen` (bitwa
ścierania), atak rozkazem rozwija przez `Keen` celu; minimapa w świecie ograniczonym trzymana w
jego brzegach.

Odstępstwa od planu, przyjęte: `Search.Abstract` pisze do buforów wołającego i daje `(0, true)`
dla końców zbyt bliskich (jeden prosty odcinek); `Member.Level` z własnych znaczników przez
`bodies.Levels`; pierwsza trasa abstrakcyjna przez zwykły ask po poziomie i klasie (jednostki
bez klasy zostają na komórkach); ciało ze znakiem nie wypada z przestrzeni (zniknęłoby z wzroku i
selekcji), lecz dostaje puste kontakty (`still()`); `Inherit{From ceremony.Whom}` (encja 0 jest
prawdziwa), zero = własna całość; `world.SpawnPart{Entry, At}` i moment `Folding` (bez nich
reguły zwijania nie istnieją); `world.Field{Size}` konieczny (pudło encji nie przekracza
`Entities.MaxSize`, sektor jest większy; armia na granicy sektorów nie jest polem); odcinki pola
jako encje świata `stretch` per (agregat, sektor), zapisywane, temat momentu `Unfold`; pole
rysowane po kawałkach sektorów (każdy na poziomie swojego sektora; nic nad sektorem rozwiniętym
ani zrzucającym składniki); `world.Unfolded`; `occupancy.PlaceSystem` stawia składniki na
dopuszczalnych komórkach; `world.Keen`/`Calm` dla reguł; `Query.Seek` goke ignoruje `Exclude`
(dwa miejsca czytają kolumnę opcjonalną). Szarża żołnierzy w demie planem co 1,5 s, klawisz H;
szarża gaśnie 20 s po ostatnim ciosie, bo rozwinięty agregat nie może się zwinąć, dopóki rozkazy
w niego celują (otwarte: `Fold` z chwilą utraty ostatniego składnika albo `MoveTo` w najbliższego
z grupy).

Sprawdzenia deterministyczne (części i runda poprawek, sondy): topografia oddalona do L2
(212 sektorów): 7 jednostek po bramkach, `Snaps 0`, zejście po zbliżeniu na komórki trasy 1–8
pól od rzutu prostej, po reliefie, `Searches` 7 → 15 (doprecyzowania); forest-army: marsz
`abs 2/16`, rozwinięcie w 12 żołnierzy bez snapu i bez „fell into” (przed rundą 4–5 na scenę),
zwinięcie w centroidzie, ścieranie na L2 12/8 → 4/0 w 9 s, szarża przy kamerze daleko 12 → 4 u
obrońcy, zapis i odczyt w obu stanach; zrzut przy przesunięciu poziomu 1: baneru nie ma między
żołnierzami; róg planszy: okrąg minimapy pełny.

Pomiar (integrator, baza `main` v0.19.1 na zmianę z drzewem, 2 rundy × `-count=1`, ~2,5 min):

| Bench | Baza | Drzewo | Zmiana |
|:---|---:|---:|---:|
| `Navigation_Tick/open,1000` | 2,35 ms | 2,39 ms | +1,9% |
| `Navigation_Tick/abstract,1000` | — | 0,63 ms | nowy (tryb daleki po trasie doprecyzowanej; pomiar końcowy) |
| `Navigation_Tick/crowded,1000` / `grouped` / `column` | 7,62 / 4,60 / 2,38 ms | 7,69 / 4,64 / 2,37 ms | +1,0 / +0,8 / −0,3% |
| `Routing_Long` / `Routing_Abstract` (sanity części, 200 ms) | 3,8 ms | 1,9 ms | bez szukania |
| `Collision_Tick` (6 rungów) | | | −0,2…+1,7% |
| `Board_Altitude` (6 rungów) | | | −0,1…+1,6% |
| `World_Tick` 1000 / 5000 | 8,70 / 35,5 µs | 8,87 / 36,4 µs | +1,9 / +2,5% |
| `World_Publish/49 of 5000` | 9,2 µs | 12,4 µs, po `b/publish` 8,8 µs | −3,8% (`seenAny` znów inline, bez rozlewu) |
| `World_Publish/5000 of 5000` | 61,5 µs | 62,0 µs | +0,9% |

Drugie podejście po przeglądzie dema przez użytkownika (2026-11-04, wieczór): (a) baner armii
na L2 klatkował, bo motion szedł co 4 ticki; użytkownik: czas ma biec normalnie tam, gdzie widz
patrzy najbliżej, choćby to był L2, a rzadsze ticki należą się sektorom dalszym; wobec stałych
okresów goke (wypiekanych na archetyp) tempo dostało własne znaczniki `sector.Far1..Far3`
(poziom minus najdrobniejszy widziany przez obserwatorów, `Keen` nie liczy się, bez obserwatora
podstawą jest `Unobserved`, więc serwer tyka pełnym krokiem, a oszczędność dają agregaty);
`b/far` (`eb5ffdaa`, `8b637af5`; sonda: baner co klatkę o 0,70, wcześniej co 4 klatki o 2,78;
`Sectors_Expanse/banded` bez zmian). (b) Cel i trasa armii wysłanej w las niewidoczne, a cięciwy
od bramki do bramki odrzucone: na L2 jednostka idzie tą samą trasą doprecyzowaną co na L0 w
trybie dalekim bez zajętości, kolejki, szyku i kolizji, z prędkością z wyceny plannera na krok;
zejście zakotwicza trasę w komórce pod jednostką, snap tylko poza nią; `order.Abstract`,
`Finder.Abstract`, `Along`, `Dropped`, `Routing_Abstract` odeszły, `Inherit` kopiuje `Path`;
trasy i cele to znaki `OnTop` (reguła Z w `CLAUDE.md`); kawałek pola na `PieceLift 0,5`;
`b/road` (`5c052212`, `02733c0d`; sonda topografii na L2: 1670 z 1680 ticków jednostek na
komórkach trasy, zejście bez snapu; `abstract,1000` 0,93 → 0,54 ms w sanity, routing rozwija
3× mniej węzłów). hpastar `Search.Abstract` zostaje w bibliotece nieużywane przez grama. Pomiar
końcowy drzewa przeciw `main` (2 rundy × `-count=1`): `Navigation_Tick` rungi dotychczasowe
−1…+1,7% mediany (crowded 100 +8,6% mediany przy +1,8% minimum: szum rungu 0,47 ms),
`abstract,1000` 0,63 ms, `Sectors_Expanse/banded` −6,4% (znaczniki `Far`: encje przy kamerze bez
znacznika tempa), `World_Publish` 49 −4,7%, 5000 +0,4%, `World_Tick` +0,1…+1,1%,
`Collision_Tick` +1,0…+1,6%.

Co zostało: pozycje roadmapy „Aggregates, what is left” (parametr poziomu hierarchii, czekanie
na pierwszą budowę, prawy klik w encję jako `MoveTo{To}`, składnik stojący na planszy, `Fold` z
utratą ostatniego składnika, `Stop` w trybie dalekim, przemarkowanie `Far` jednym ruchem
archetypu), etap minimapy po drabinie (decyzja użytkownika: poziom kamery +1, figury najwyżej
L2, grunt L3, rzadkie odświeżanie), etap `f` (koszt dalekiej figury).

## Daleka figura taniej: poziom w rekordzie, suma bez rozlewu, kafle bez alokacji (plan 2026-11-04, gałąź `perf/far-figures`, litera `f`, po etapie `b`)

Z wyniku etapu `g` (decyzja użytkownika 2026-11-04: po agregatach, bo las i armia zmienią
populację L2–L3 najbardziej, a to, co zostanie, to światy wielu niezależnych pudeł jak
ekspansja). Trzy chybienia celów `g` i ich przyczyny (`publisher.go`, `renderer.go`,
`aggregate.go`, `render/internal/gpu`):

- **Odczyt poziomu na encję, dwa razy.** Punkt L2 kosztuje ~40 ns w `World_Draw/…,level=2`
  (cel 20): `Compose` szuka sektora pod środkiem pudła w widoku kamery (`s.level`) dla każdego
  rekordu. Rzedzenie `Keep` kosztuje ~7 ns na encję w kadrze (`World_Publish/5000, keep=0.25`:
  65 → 99 µs): `kept` po haszu (~1 ns) robi ten sam odczyt dla trzech czwartych encji, do tego
  `v.Contains(id)` na widok. Hasz nie jest winny.
- **Suma L3 ~90 ns na pudło** (`World_Publish/5000, level=3`; cel 10 był nierealny dla sumy
  ważonej pokryciem): pudło 20 na tekselu 16 trafia w ~5 tekseli, sam dodatek ~25 ns na teksel,
  Go rozlewa rejestry w pętli pokrycia; stały punkt i szybszy floor nic nie dały.
- **Kafel zmieniony alokuje** ~4 KiB w `gpu.put` na rysowanie (figury w ruchu na L3); scena
  nieruchoma nie alokuje.

Jeden agent Opus, jeden worktree `/home/karol/worktrees/gram/far-figures` na `f/far-figures`
od `main` po `b`; części sekwencyjnie w jednym briefie (pliki wspólne), każda z własnym
commitem; profil przed każdą decyzją, nie domysł.

**Część 1 `poziom`**: profil `World_Draw/5000, level=2` i `World_Publish/5000, keep=0.25`
(gdzie leży odczyt: `Grid.Of`, tablica poziomów widoku, `Contains`), potem jedno z dwojga, po
profilu: (a) poziom w rekordzie: publisher pisze poziom, który wyznaczył rzedząc lub sumując,
dla widoku, który encję widzi, z wartością „kilka” gdy widzi ją więcej widoków (rekord jest
jeden na publikację, poziom jest kamery; przy jednej kamerze zawsze trafiony), `Compose` czyta
rekord i szuka tylko przy „kilka” lub bez poziomu; (b) odczyt tańszy: sektor liczony raz na
przebieg encji jednego chunka, gdy kolejne pudła leżą w tym samym sektorze (sprawdzenie granic
tańsze niż `Of`), `Contains` raz na chunk, gdzie widok obejmuje cały chunk. Cel: punkt L2 ≤ 25
ns w `World_Draw`, `keep=0.25` ≤ 75 µs; rungi zwykłe `World_Publish` ±2%. Testy: te same
rekordy co dziś (zrzuty L2 piksel w piksel z bazą; `sees` bajt w bajt), dwie kamery na różnych
poziomach nad jedną encją.

**Część 2 `suma`**: pętla pokrycia bez rozlewu (udziały wiersza policzone raz, cztery kanały
jednym słowem `uint32` z nasyceniem, indeks kafla raz na wiersz) albo, jeśli profil każe, pudło do
dwóch tekseli boku w jeden teksel pod środkiem z wagą pola (pokrycie zostaje dla większych: ślad
dużej figury ma zostać śladem). Cel: ≤ 50 ns na pudło w `World_Publish/5000, level=3`; obraz
agregatu w testach ten sam co dziś dla pudeł ponad dwa teksele, dla mniejszych suma kanałów na
sektor równa w 1/255.

**Część 3 `kafle`**: bufor pośredni `gpu.put` wielokrotnego użytku (jeden na rozmiar kafla,
`render/internal/gpu`, skill `gram-render`: pułapki wgpu przy zapisie w punkcie), zero alokacji
na klatkę przy figurach w ruchu na L3; test GPU liczący alokacje (`testing.AllocsPerRun` wokół
`upload`).

**Pomiar** (integrator, ≤5 min, baza `main` po `b` na zmianę z drzewem, 2 rundy × `-count=1`):
`World_Draw` i `World_Publish` wszystkie rungi (cele wyżej, zwykłe ±2%), `Frame_Topography/
iso,level=3` ±2%, sonda topografii z figurami w ruchu przy zoomie L3: `gctrace` 20 s, bajty na
klatkę baza przeciw drzewu.

**Dokumenty**: `BENCHMARKS.md` (tabela poziomów `World_*`, zdanie o alokacji), skill
`gram-perf-picture` (niezmienniki: poziom czytany raz na encję, suma bez rozlewu), `CHANGELOG.md`
(`perf`, v0.21.0 albo łatka po `b`, jak zdecyduje użytkownik), pozycja „A far figure's cost”
zdjęta z `doc/roadmap.md`, wynik tu.

**Co nie wchodzi**: tablice zwarte publishera (rekord SoA, SIMD sumy), demo z sektorami na
płaskiej planszy i heksach, kolor bazowy materiału, selekcja a śpiący.

## Trzy skale drabiny: przenumerowanie poziomów (plan 2026-11-05, gałąź `refactor/levels`, litera `e`, po etapie `b`, przed etapem `p`)

Decyzja użytkownika (2026-11-05): drabina ma trzy skale po dwa szczeble, dokładny i zgrubny, i
pominięcie na końcu; skala bliska (igły świerku, pory skóry) wchodzi już teraz jako systematyka,
choć żadne demo jej nie używa; stare L0 staje się L2. Skala bliska tyka i rysuje się tylko wtedy,
gdy ktoś tam wejrzy (kamera w perspektywie, FPP/TPP), i tylko w grze, która jawnie ją włącza
progami; w grze strategicznej nie istnieje. Osobny PR przed `p`, bez zmiany zachowania ani obrazu
dzisiejszych dem: te same dema wyglądają i tykają tak samo, zmieniają się numery i słowa. Jeden
agent w worktree `/home/karol/worktrees/gram/levels` na `e/levels`; dokumenty integratora.

| nowy | stary | skala | słowo | tick | obraz |
|---|---|---|---|---|---|
| L0 | — | bliska, dokładna (`lod.Close`) | close | systemy detalu (`Within(Close)`), pełny krok | wersja `At(L0, …)`, inaczej sprite L2 |
| L1 | — | bliska, zgrubna | close, no detail | systemy detalu, rung 1 od L0 | wersja `At(L1, …)`, inaczej sprite L2 |
| L2 | L0 | zwykła, dokładna (`lod.Standard`) | full | pełny, rung 0 gdy najdrobniejsze widziane | dzisiejsze L0 |
| L3 | L1 | zwykła, zgrubna | no detail | rung +1 | dzisiejsze L1 |
| L4 | L2 | strategiczna, dokładna (`lod.Strategic`) | shape | rung +2, agregaty | dzisiejsze L2 |
| L5 | L3 | strategiczna, zgrubna | colour | rung +3 | dzisiejsze L3 |
| L6 | L4 | pominięcie (`lod.Omitted`) | nothing | nic | nic |

Rungi liczą się od faktycznie najdrobniejszego poziomu, jaki widzi jakiś obserwator (zasada z
2026-11-04 bez wyjątku): gdy kamera FPP patrzy na igły (L0), sektor zwykły obok jest o dwa rungi
grubszy i motion z `Slowing(2, 4, 8)` idzie tam co 4 kroki. To konsekwencja zasady, nie błąd:
gra, która tego nie chce, nie włącza skali bliskiej albo daje systemom `Steady`. Systemy należące
do skali bliskiej (mimika, igły) filtrują encje `Within(lod.Close)` i idą `Exact`: tykają tylko
tam, gdzie ktoś patrzy z bliska, i wcale, gdy nikt.

**Kontrakt** (`lod/lod.go`, `lod/doc.go`):
- `Level` L0..L6, `Levels = 7`; nazwane pierwsze szczeble skal: `Close = L0`, `Standard = L2`,
  `Strategic = L4`, `Omitted = L6` (nie `Nothing`: zajęte przez `Slot.Nothing`; nie `Far`: zajęte
  przez `sector.Far*` i `Config.Far`). Słowo szczebla nieparzystego: „zgrubny" swojej skali.
- `Ladder.Scales [Levels-1]float32` (6 progów). Zero w progach skali bliskiej (`Scales[L0]`,
  `Scales[L1]`) znaczy „skala nieużywana" i `At` je pomija; zero od `Standard` wzwyż znaczy jak
  dziś „spełnia go każda skala". Zerowa drabina to więc `Standard` wszędzie bez końca (dziś L0
  wszędzie): zachowanie dem bez zmian. `Check`: progi malejące wśród ustawionych, z pominięciem
  zer bliskich. `Hold`/`Hysteresis` bez zmian.
- Tempo: `rung = level − finest` jak dziś, `finest` najdrobniejszy widziany przez kogokolwiek (L0
  przy kamerze z bliska). `type Paces [Levels]uint8` (7): `p[k]` co ile kroków system idzie o k
  rungów grubiej, rung 6 tylko na `Omitted`, więc `p[6] == 0`; `Exact {1,0,…}`, `Steady
  {1,1,1,1,1,1,0}`, `Contact {1,1,1,0,0,0,0}`, `Slowing(1..5 n)` (ostatnie n powtarzane:
  `Slowing(2, 4, 8)` daje rungi 1..5 = 2, 4, 8, 8, 8), `Check` jak dziś z nowymi granicami. W grze
  bez skali bliskiej `finest ≥ L2`, więc rungi i tempa są co do kroku dzisiejsze. Znaczniki tempa
  `sector.Far1..Far5` (rung 1..5), `sectors.far`, `Pace(paces)` → `Every[Far_k](p[k])`,
  `Never[L6]`.
- Filtr skali: `world.Plugin.Within(first lod.Level) goke.Opt`: encje w sektorach na poziomach
  `first` i `first+1` (jedna skala: `Within(Close)` = L0–L1), jako `Exclude` znaczników
  pozostałych poziomów (goke nie ma OR); system detalu łączy `Within(Close)` z `Pace(Exact)`.
  Godoc i akapit w `lod/doc.go`; test w `plugins/world`: system z `Within(Close)` nie dotyka encji
  sektora L2, dotyka L0 i L1.
- `Keep [Levels]float32` (7). `Unobserved`: `Fallback` zero = `UnobservedL4` (dziś L2), stałe
  `UnobservedL0..L6`, `Level()` panikuje na nieznane. `Fold` godoc: „na `Strategic` lub grubiej".
  `Texel` i `Seed` bez zmian.
- Znaczniki poziomu: `sector.L0..L6`, każdy poziom ze znacznikiem (dziś L0 bez: `Within`
  potrzebuje wykluczać każdy poziom, w tym `Standard`), rejestrowane i zapisywane jak dziś;
  `markOf(level)`; `sectors.move` kładzie i zdejmuje jak dziś. Koszt: archetyp na (rodzaj ×
  poziom) już istnieje dla L1+; `Sectors_Expanse` ±3 % to sprawdza. Stare zapisy przestają się
  wczytywać (typy komponentów zmieniają nazwy; precedens: `MoveOrder` w etapie `b`; wpis w
  CHANGELOG).
- Atlas: `versionsOf`: L0..L3 własny sprite (bliskie spadają na zwykły), L4 plama, L5 nic (do
  preludium `p`), L6 nic; `mayDeclare`: każdy poziom poza `Standard` (własny wygląd) i `Omitted`,
  więc `At(L0, close)` i `At(L1, …)` działają od razu; `Nothing(level)` tak samo.
- Widok kamery i budżet klatki: „nigdy poza L3" → „nigdy poza L5"; „nigdy sektor widziany na L0"
  → „nigdy sektor widziany na `Standard` lub bliżej" (grep `L3`/`L0` w `plugins/cameras` i
  `plugins/world/view`, godoc).
- Teren: `skirtLevel = L5`, `byLevel` indeksowane poziomem, ale shadery kompilowane dla
  `max(level, Standard)` (4 shadery jak dziś: bliskie rysują się shaderem `Standard`); w
  `mesh.wgsl` progi `LEVEL <= 1u` → `<= 3u`, `<= 2u` → `<= 4u`, `== 3u` → `== 5u`, `== 0u` →
  `<= 2u`. Heksy i płaska plansza analogicznie (grep `lod.L` w `plugins/board/look`,
  `topography/internal/hexes`; shadery kafli dekodują poziom sektora: sprawdzić kodowanie).
- Reszta mechanicznie: `lod.L0..L4` → `L2..L6` w 50 plikach (`git grep -nE 'lod\.L[0-4]\b'`;
  tam, gdzie kod mówi o roli, użyć nazwy roli, nie numeru: `lod.L4` jako „pominięcie" →
  `Omitted`, `lod.L2` jako „agregaty, graf abstrakcyjny" → `Strategic`, `lod.L0` jako „pełny" →
  `Standard`), `sector.Keen{Level: L1}` → `L3`, `bodies.Member.Level >= L2` → `>= Strategic`,
  `Exclude[sector.L4]` → `L6`, telemetria (wiersze „Sectors L0 …"), benche: rungi `level=2|3` →
  `level=4|5`, drabiny dem: forest-army `{0, 0, 2, 1, 0.5, 0}`, topography `{0, 0, 0.5, 0.25,
  0.12, 0}`, expanse `{0, 0, 0.5, 0, 0, 0}`.

**Testy**: istniejące z przesuniętymi oczekiwaniami; nowe w `lod`: zerowa drabina daje `Standard`
dla każdej skali i odległości; drabina z progami bliskimi daje L0/L1 przy dużej skali; `Check`
przyjmuje zera bliskie i odrzuca próg bliski niemalejący; `Paces.Check` i `Slowing` z nowymi
granicami; w `plugins/world`: kamera widząca sektor na L0 (bliska) tyka go rungiem 0 i bez `Far`,
sektor L2 obok ma `Far2`, L5 `Far5`; bez skali bliskiej (zerowe progi) kamera najbliżej na L2:
sektor L4 ma `Far2` jak dziś L2 przy L0; `Unobserved` zero to L4; `Within(Close)`; atlas: `At(L0,
look)` wybierane na L0, bez deklaracji na L0 własny sprite; zapis i odczyt znaczników L0..L6.
Sprawdzenie deterministyczne: `GRAM_SHOT` dem topography (start i zoom do dawnego L2/L3),
forest-army, flat, expanse piksel w piksel z bazą (`gram-probe`), `routes.golden` nietknięte.

**Pomiar** (integrator, ≤5 min): `World_Publish|Draw`, `Frame_Topography`, `Sectors_Expanse`,
`Navigation_Tick` ±2 %: zmiana ma kosztować zero.

**Dokumenty** (integrator, w stage): `lod/doc.go` (tabela siedmiu wierszy, akapit o skalach i
rungach), `CLAUDE.md` (reguła LOD: numery i nazwy skal), skille (`gram-plugins`, `gram-plugin`,
`gram-render`, `gram-perf-*`, `gram-game`), README (słowa poziomów, jeśli są), `CHANGELOG.md` pod
v0.20.0 (Changed: poziomy przenumerowane, trzy skale, `Paces` o siedmiu rungach, znacznik na
każdym poziomie; Added: skala bliska deklarowana `At(L0|L1, …)` i włączana progami,
`Plugin.Within`; stare zapisy nie wczytują się), `BENCHMARKS.md` (nazwy rungów), `doc/roadmap.md`
(punkt: skala bliska w którymś demie), wynik tu. Po scaleniu plan `p` (płachty) jest przepisywany
na nowe numery, zanim briefy pójdą do agentów.

### Wynik etapu `e` (2026-11-05, `refactor/levels` `937c2ac7`)

**Wykonanie**: jeden agent, cztery commity na `e/levels` (drabina siedmiu poziomów w trzech
skalach ze znacznikiem na każdym poziomie; `Plugin.Within`; `doc.go` pakietów; nazwy rungów
benchów), scalone `--no-ff`; worktree usunięty. Sekwencja zielona (90 pakietów testowanych, GPU
Intel UHD 620), `make build`, 25 dem po 5 s.

**Odstępstwa od planu, przyjęte**: tabela `lod/doc.go` ma poziom i słowo w jednej kolumnie (pięć
kolumn nie mieści się w 100 znakach); panel telemetrii pokazuje dwa dodatkowe wiersze L0 i L1
(zerowe w grach bez skali bliskiej); świeżo zespawnowana encja przed pierwszym przebiegiem
znakowania przechodzi przez każdy filtr `Within` najwyżej jeden tick (filtr z wykluczeń nie
widzi encji bez znaku); `islandWith` w benchach planszy ustawia progi od `Standard`; słownik
`sector` wymienia `Far1..Far5`. **Stare zapisy wczytują się** (goke dopasowuje komponenty po
nazwie, a `L1..L4` i `Far1..Far3` nadal istnieją), ale zapisany poziom `sector.Keen` (i
`order.Sought.Keen`) czyta się o dwa poziomy drobniej; dotyczy wyłącznie zapisów zrobionych
między PR #53 a tą zmianą (zapisy z v0.19 nie wczytują się od etapu `b`, `MoveOrder`), więc nic
nie robimy.

**Sprawdzenia deterministyczne**: zrzuty ośmiu widoków (expanse start; flat start; forest-army
start L4, oddalona L5, zbliżona L2 z 639 rozwiniętymi encjami; topography start L2, oddalona
×16 z 247 sektorami L4, perspektywa L3/L4/L5) piksel w piksel równe bazie `main` `7d6095f5`
(0 różnych pikseli, minimapa i panel ukryte, światło zamrożone, ziarno 12345); `routes.golden`
nietknięte.

**Pomiar** (baza `7d6095f5` przeciw drzewu, oba z `GOEXPERIMENT=simd` i tym samym go.work
rodzeństwa, 2 rundy, bezczynna maszyna, potem runda w odwrotnej kolejności): `Frame_Topography`
19 rungów −1.1…+0.5 % (`readback` +3.0 %, rundy się pokrywają), `Sectors_Expanse` ±0.4 %,
`Navigation_Tick` +0.4…+2.8 % (`grouped` +6.0 % i `abstract` +5.2 % z pokrywającymi się rundami,
nie sprawdzone w odwrotnej kolejności), `World_Publish` i `World_Draw`: ruchy do +12 % w
kolejności baza→drzewo zmieniają znak w kolejności odwrotnej (pułapka boostu), więc to szum;
jedno trwa w obu kolejnościach: `World_Draw/…,level=5` +20…25 ns na rungu ~330 ns (49 i 5000
encji; na L5 nic nie jest rysowane, więc to stały koszt przebiegu, najpewniej pętla po siedmiu
poziomach zamiast pięciu). Alokacje bez zmian. Pliki pomiaru w scratchpadzie sesji
(`levels/measure/`).

**Co zostało**: skala bliska w którymś demie (roadmapa); etap `p` (płachty) od `main` po
scaleniu tego PR, plan przepisany na nowe numery.

## Płachty: fotografia tego, co stoi, L5 w pół, banery i liczby (plan 2026-11-05, gałąź `feat/sheets`, litera `p`, po etapie `e`, v0.20.0)

Numery poziomów w nowej numeracji po etapie `e` (L2–L3 zwykła, L4–L5 strategiczna, L6
pominięcie). Fakty niżej sprawdzono na `main` `7d6095f5` (przed `e`); po `e` (`7496b324`) nazwy
trzymają, numery wierszy mogą się różnić o kilka linii, a warunek plamy w `Compose` brzmi
`level == lod.Strategic` (`renderer.go:179`). Wykonanie: agenci Opus 5.5, preludium w klonie na
`feat/sheets`, części w worktree'ach; brief agenta = jego część tej sekcji plus „Model wykonania".

### Kontekst

Agregaty (etap `b`) mają służyć Maderze. Dziś figura bez własnej wersji L4 jest na L4 plamą
(`render.Blob`), a na L5 jej kolor sumuje się do obrazu `world.AggregateImage`, który grunt kładzie
pod sobą. Użytkownik (2026-11-05): zamiast sumy ma być **płachta**: bryła (albedo, wysokość,
normalna) sfotografowana na GPU z wyglądu drobniejszego poziomu tego, co **stoi** w sektorze,
rysowana przez grunt jako relief oświetlany światłem klatki, odświeżana w biegu (mapa jest
przekształcalna), z L5 jako gorszą (zmniejszoną) wersją L4, nigdy liczoną po zwykłych encjach;
fallback tam, gdzie gra nie podała własnej wersji (`Slot.At`). Granice sektorów: poziom na kawałek,
długa encja nie dostaje całości drobno, części zszywają się. Luźne jednostki na L4: baner (tylko
gdzie gra podała wersję L4) albo miniatury, osobno liczby; per gra, w Maderze na F10.

### Decyzje użytkownika (2026-11-05, nie do rewizji)

- Płachta jest bryłą: zmiana położenia słońca działa na niej normalnie. Fotografia na GPU.
- L5 = gorsza wersja L4 (zmniejszona płachta). Suma koloru z etapu g odchodzi.
- Poziom encji zostaje na kawałek (sektor środka). Kawałek przecięty granicą należy do
  drobniejszego sektora; kawałek na L4 stykający się z sektorem L3 jest rysowany wersją L3
  (pierścień jednego kawałka); dalsze są w płachcie.
- Baner tylko tam, gdzie gra zadeklarowała wersję L4 rodzaju; tryb Banner/Points per gra w
  Config, w Maderze w ustawieniach; osobny przełącznik liczby; klik w baner zaznacza jego
  jednostki.
- Jeden tag v0.20.0 po trzech etapach (`p` płachty, `d` Madera z agregatami, `i` minimapa po
  drabinie; litery `m` i `n` są zajęte przez sesje fali 1 w `doc/plans-perf.md`). Wpisy pod
  „v0.20.0 — unreleased".
- Scalenia części idą na gałąź etapu i są wypychane; etap kończy się PR-em zgniecionym do jednego
  commitu (użytkownik 2026-11-05 wieczorem: „już wolę PR, niż jakieś szczątki do review"). Integrator
  koordynuje, nie koduje.

### Fakty sprawdzone (stan `main` `7d6095f5`)

- **Suma dziś**: `plugins/world/aggregate.go` (`AggregateImage = "Aggregate"` rejestrowane w
  `init`, `newAggregate`, `add`, `upload` przez `SubImage.WritePixels`, kafle 64, `aggregateSide
  2048`), `Plugin.Aggregates()` (:21), `Read.Aggregate()` (`rw.go:113`); publisher `recorded` →
  `fate` (`publisher.go:356-409`: L5 → `sum`), `keepAll` (`k.l3`), pola `spread`/`pieceAt`
  (:254-303, L5 → `aggregate.add`); atlas `level()` liczy `MeanColour` tylko dla rodzaju bez
  żadnej deklaracji (`atlas.go:528`), `versionsOf` daje plamę na L4 i nic na L5 (:579),
  `declares(id, level)` (:341), `blobColour` (:348), `isAlways`. Klienci: `board/plugin.go:136`
  (`WithAggregate`, `look/renderer.go:274`, `look/tiles.go:526` `summedOver` w
  `tiles_common.wgsl:163`), `topography/plugin.go:236,242` (`terrain.go:124,205-216`
  `AggregatePx`, `surface.wgsl aggregateAt`; `hexes.go:199,206` `drawSheets`), `bench/
  world_test.go:144`, testy `aggregate_test.go`, `aggregate_internal_test.go`, `field_internal_test.go:52`,
  `export_test.go` (`AggregateTexels`).
- **Rekordy**: `figures{all []figure, layers []layer}`, `figure{id, box, z Z, from, n, piece,
  shadow}` (`publisher.go:21-46`); wersja na poziom dobierana w `Compose` przez `levels.at(sprite,
  level)` (`renderer.go:165-190`), L4 przez `s.point`. Poziom encji: `renderer.level(v, box, mid)`
  (`renderer.go:318`) = `v.Level(mid)`.
- **Sektory**: `sector.Grid{Of, Box, Cover(box, dst) []Part{Sector, Box}, Side, Neighbours}`
  (`sector/grid.go`); nadzorca `sectors{graded, level, finest, far}` (`sectors.go`), `seenBy(cam,
  i)` (:376), `seenSectors` implementuje `view.Sectors` (`view/seen.go:15`: `Len, Cols, Box, Of,
  Level(cam,i), Ladder`); `view.View.Sectors(each(i, box, level))` pomija L6. Znaczniki
  `sector.L3..L6, Far1..3, Keen, Abstract` (`sector/sector.go`), rejestrowane w `sectors.Init`
  (:101-105), zapisywane w `module.go:178-186` (`goke.LoadComp`). Bok sektora = `Buckets ·
  BucketSize` (:86); Madera: 16 komórek = 512 jednostek, 800 sektorów.
- **Z**: `entity.Z{Altitude, Height, Step}`, `Altitude` to **bezwzględna** wysokość spodu (plansza
  pisze ją jednostkom z reliefu, `board/units.go:110`; filar mostu `Z{Altitude: Bottom, Height:
  Top−Bottom}`, `bridges/types.go:104`; pokład z `NewEntities` ma `Z{Height: Thickness}` a jego
  `Altitude` pisze plansza jak encjom stojącym).
- **Encje stojące**: `board.NewEntities[P](brd, shape, at)` → `Define` daje `Position`, `cell.On`,
  `world.Z{Height}` i `StandingTemplate` (`board/entities.go:24-48`, `templates.go:10`, domyślne
  `world.Lie{}`, `light.Shadow{Cast: Baked}`); pokłady mostów przez `NewEntities` (:78), filary
  przez gołe `kind.Define` (:104). `SpawnPart` nie dotyka szablonu.
- **Ustawienia świata**: `Settings{Sectors, Unbanded}` (`commands.go:92`), `plugin.Keep` w
  `plugin.go:82`, `Restore` (:247) nakłada je przez `write`; wzór komendy ustawienia:
  `SwitchSectors` → kolejka `module.go:81` → `switchSectors` (`sectors_view.go:172`) →
  `write.ShowSectors` (`rw.go:254`); `Settings()` (`commands.go:109`) z `control.Choice`/`Switch`.
- **Zaznaczenie**: `NewSelectionSystem(selects, space, tags, look func() world.Look)`
  (`selection_system.go:60`, wołane z `selection/plugin.go:71`), `pick(at, screen, cam)` (:218),
  `Select{IDs, Box, Screen, Camera, Additive}` → `applySelection` (:274); brak uchwytu `Read`.
- **Render**: obrazy klatki: `render.RegisterImage(name)` przed pierwszym shaderem, **8 nazw,
  7 w użyciu** (`GroundHeights, GroundShade, GroundShadeWas, Aggregate, CloudTile, AirMasses,
  GroundDetail`); Direct wiąże obraz klatki per draw przez `opts.Uniforms[name]`; draw ma 4 własne
  sloty `opts.Images`. **Brak mipmap** (`gpu/texture.go:53` `MipLevelCount: 1`, samplery bez
  `MipmapFilter`, wszystkie odczyty `textureSampleLevel(…, 0.0)`), brak compute. Render-to-texture:
  `render.NewImage`, `SubImage`, `Clear`, `WritePixels`, `ReadPixels`, `DrawImage`, `DrawMesh(idx,
  shader, &DrawMeshOptions{Depth, WriteDepth, Vertices, Ranges, Instances, Images [4], Uniforms,
  Untested})`, `render.NewMeshShaderWith(label, src, uniforms).Instanced(n)`. Tekst tylko
  ekranowy (`render.Font.Draw(dst, s, x, y, c)`, `DefaultFont`), żadnego tekstu w świecie.
- **Malarz** (`painter/bake_gpu.go`): wypiek do `render.Image` w `Painter.Surface` wołanym
  z `terrain.Renderer.Draw → prepare → refresh` (leniwie, gdy `seen`/`styled` się zmieniły);
  shadery `paint_*.wgsl` `Instanced(1)`, 6 wierzchołków na instancję, `PaintMasked` dla komórek
  zmienionych (`toPaint`), `paintMask`; sprite'y atlasu przez `opts.Images[0] = atlas.Atlas()`;
  testy `gputest.Need` + `ReadPixels`, pomocnik `differ`; `Benchmark_Painter_Bake` leży w
  pakiecie malarza (`measure.sh` go nie widzi).
- **Teren** (`terrain/sectors.go`, `terrain.go`): bez buforów wierzchołków (narożnik z
  `vertex_index`, `mesh.wgsl:19-45`), jeden bufor indeksów: `whole` wszystkich sektorów, potem
  `coarse` (co drugi narożnik, pierścień jednej komórki wachlarzami do pełnych narożników
  brzegu, :69-117), potem `skirt`; `runs` wybiera `coarse` od L4, `eachLevel` grupuje w
  `IndexRange`, jeden `DrawMesh(r.mesh, byLevel[level], &r.opts)` na poziom; shader na poziom z
  `const LEVEL` (`leveled`); `opts.Images = {lattice, albedo, water, kinds}`; siatka wysokości
  `r.lattice` RGBA8 w ćwiartkach (wysokości 16 bit `(h−low)/span·65535`, normalne, flagi,
  brzegi), `encode` (:480), `Low`/`Span` w uniformach; `mesh.wgsl:70-79`: L4 bez fal, cienia
  wypieczonego i lamp, L5 bez chmur, kołysania, ziarna i wody, suma nad albedo (:130).
  `Config.Aggregate func() (*render.Image, float32)` (:124). Światło: `lightWay()`,
  `groundShadeOf` (tylko `LEVEL <= 1`), `cloudsShadeAt` (`LEVEL <= 2`), `lampLightAt`, mgła
  `U.Sight`. Test wzorcowy: `terrain/draw_internal_test.go:52` (`gputest.Need`, `render.Target`,
  `cameratest.NewFromSpace`, `ReadPixels`), `atLevel` (:127).
- **Światło**: wypiek cienia gruntu czyta obraz klatki `GroundHeights` (`compose/bake.go:188`),
  nie siatkę pluginu; `light.Relief{Lattice, GroundAt, Version}` daje tylko metadane.
- **Dema**: topography: rodzaje `unit, rival, hawk, plateau` (jednostki), `bridge, ford,
  lantern` (stojące), żaden nie deklaruje `At`; drabina `Scales{0.5, 0.25, 0.12, 0}`, Sight 30 km,
  Fog 6 km, 640×320 komórek po 32 jednostki (jedn. 3.125 m), `Buckets 2`; `map.yaml`: 21 mostów,
  7 brodów, 1 latarnia, 37 jednostek; brak własnych `Tunable`. forest-army: las `At(L4, canopy)
  .At(L5, Solid(MeanColour))`, armia `At(L4, banner)`, `Scales{2, 1, 0.5, 0}`, Fold 2.
- **Testy/rigi**: `sectors_internal_test.go` (`sectorWorld`, `sectorRig`, `watchers`, `ticked`,
  `drawn`), `aggregate_internal_test.go` (`summable`, `newSumming`, `texel`, `written`),
  `coarse_internal_test.go` (`halves`, `farther`, `readied`), `publisher_internal_test.go`
  (`thinnable`, `published`, `keepingL2`). Benche: `World_Publish`/`World_Draw` rungi
  `leveledViews` („5000 of 5000 in view, level=2|3", „…level=3, unread", „…level=2, keep=0.25"),
  `World_Frame`, `Frame_Topography` („iso,level=0..3", …), `Sectors_Expanse` (banded/unbanded),
  `Text_Panel`.
- Gałąź `feat/sheets` nie istnieje; drzewo czyste; brak worktree'ów; `p/*` wolne.

### Projekt (co buduje etap)

**Obrazy płachty** (świat, `plugins/world/sheet.go`): nad całym światem, teksel `Ladder.Texel`
(zero: `1/Scales[L4]` jednostek, czyli jednostki na piksel na progu L4; podniesiony tak, by
płachta miała ≤ 4096 tekseli na bok — Madera: 2048×1024 dałoby 3,2 teksela na komórkę 100 m, 4096
daje 6,4); dwa obrazy RGBA8 na poziom:
- `Sheet` (albedo premultiplied; alfa 0 = teksel pusty, nic tam nie stoi),
- `SheetRelief` (RG: **wysokość bezwzględna szczytu** jako f16 przez `pack2x16float`, bajty w R i
  G; BA: normalna xy `(n+1)/2`), świat nie zna gruntu i nie musi: pisze szczyty swoich encji;
oraz ich połówki na L5 (`SheetL3`, `SheetReliefL3`: przebieg zmniejszający na GPU, albedo przez
`DrawImage` w pół, relief własnym shaderem: średnia wysokości tekseli niepustych i średnia
normalnych). Nazwy obrazów klatki: `world.SheetImage = "Sheet"`, `world.SheetReliefImage =
"SheetRelief"` zastępują `Aggregate` (8 z 8 nazw w użyciu; to samo dwie nazwy niosą obrazy L4 albo
L5 zależnie od draw, bo wiązanie jest per draw przez `opts.Uniforms`). Teren nie potrzebuje
nazw: ma własny shader płachty z 4 slotami `opts.Images`.

**Fotografia sektora** (świat, GPU, jak wypiek malarza): do **bufora sektorowego** (dwa
`render.NewImage` rozmiaru sektora w tekselach, albedo i relief, z własną głębią `render.NewDepth`;
głębia w `render` jest zawsze rozmiaru całej tekstury celu, więc bufor na całą płachtę kosztowałby
64 MB i nie da się jej czyścić prostokątem), dwa przebiegi `DrawMesh` (jeden cel na draw: najpierw
albedo, potem relief) z tą samą głębią, **oba `Primed`** (`fs_prime` kładzie głębię i musi
`discard`ować te same teksele z alfą 0 co `fs_main`; kolor pisany tylko gdy głębia równa, więc raz
na teksel, bez mieszania `SourceOver` niższej encji z wyższą), głębia = wysokość szczytu
znormalizowana do [0, 1] plus `instance_index · 1e-6` przeciw remisom (dwa pokłady obok); potem
`sheet.SubImage(rect).CopyFrom(bufor)` do obu obrazów L4 (`CopyFrom` kopiuje alfę bez mieszania).
W tekselu wygrywa najwyższa encja (drzewo nad domem). Instancje: encje `world.Fixed` sektora, których rodzaj **nie zadeklarował**
L4 (`declares(sprite, L4)` fałsz) i których poziom rysowania jest ≥ L4, oraz **zrzuty części
agregatu** zwiniętego bez L4 (migawka pudeł, `Z` i sprite'ów L3 części, brana w `wholes.fold`
zanim powie `Fold`, trzymana per rozciąg do `Unfold`). Instancja (`Instanced(3)`: trzy `vec4` na
instancję jako `@location(0..2)`, jak `compose/cast.go` z `Instanced(4)`) niesie pudło, prostokąt UV
sprite'a L3 (wersja `levels.at(sprite, L3)` z efektów `Under` tak, jak widzi je publisher),
`Altitude`, `Height`, flagę `Lying`; sprite'y przez `opts.Images[0] = atlas.Atlas()`. Shader: 6
wierzchołków na pudło; alfa sprite'a mówi, czy
teksel należy do encji; wysokość szczytu `Altitude + Height·(1 − r²/2)` (r = odległość od środka
sprite'a w jego połowie rozmiaru; `Lying`: `Altitude + Height`, płasko), normalna z gradientu
kopuły (`Lying`: w górę); albedo = teksel sprite'a. Teksel bez encji: bufor wyczyszczony (`Clear()`
całego bufora przed przebiegami, darmowy `LoadOpClear`). **Wersja treści** sektora (`uint32` na
sektor w CPU): rośnie, gdy zmieni się **sygnatura treści** sektora: liczba encji `Fixed` i xor po
nich z `mixed(id) ^ mixed(sprite L3 pierwszej warstwy)` liczone w tej publikacji względem
poprzedniej (publisher i tak idzie po każdej widzianej encji i czyta jej `Appearance`; sprite w
sygnaturze sprawia, że zmiana wyglądu pod efektem też odświeża płachtę; spawn i despawn świata
znają tylko id, a części zwinięte omijają `despawn`, więc hak tam byłby droższy i dziurawy; goke
nie ma śledzenia zmian komponentów ani relacji między encjami, a dodanie go kosztowałoby barierę na
każdym zapisie), przy `fold`/`unfold` rozciągu w nim i przez hak `neighbourChanged(i)` (część
2). Sektor encji: `grid.Of(mid.X, mid.Y)` (publisher dostaje `Of` jak dziś `covers`). **Kiedy**: w `Compose` publishera, dla sektorów, które
jakiś widok rysuje na L4+ (`v.Sectors`) i których wersja namalowana ≠ wersja treści, najwyżej
`sheetPerFrame` sektorów na klatkę (start 8; profil `Frame_Sheet` decyduje), potem przebieg
zmniejszający dla ich prostokątów w L5. Płachta nie jest zapisywana (odtwarzana z treści; wersje
startują od 1, namalowane od 0).

**Publisher**: encja `Fixed` w sektorze, który jest namalowany w aktualnej wersji, nie dostaje
rekordu na L4 ani L5 (dopóki sektor nie jest namalowany, rekord jak dziś: plama); suma L5 i
`k.l3` odchodzą; jednostka (nie-`Fixed`) na L5 to rekord plamy jak na L4 (miniatura), rzedzona
`Keep[L5]`; pola (`spread`) na L5 rysują wersję zadeklarowaną albo nic (pole bez L5 nie sumuje
się już). `Read.Sheet() (albedo, relief, albedoL3, reliefL3 *render.Image, perUnit float32)`
(nil, 0 bez sektorów albo zanim ktoś poprosił `Sheets()`).

**Grunt rysuje płachtę** (teren, `terrain/photo.go`, typ `photoMesh`; nazwa `sheet` jest zajęta
w testach terenu i w `render.Sheet` arkusza sprite'ów): na L4 i L5 sektor namalowany dostaje,
**ponad** dzisiejszą zgrubną siatką (która rysuje się jak dziś), dodatkową siatkę płachty: regularna
krata na sektor
(krok w tekselach = najmniejsza potęga dwójki, przy której sektor ma ≤ 32×32 czworokątów na L4;
na L5 z obrazów L5 i ≤ 8×8), wierzchołki bez bufora jak w `mesh.wgsl`, wysokość wierzchołka z
`SheetRelief` tam, gdzie teksel ma alfę (albedo) > 0, inaczej wysokość siatki terenu minus
`sheetSink` (pod gruntem, niewidoczna); fragment z alfą 0 odrzucony; albedo z `Sheet`, normalna z
`SheetRelief` (między wierzchołkami z teksela: relief pod krokiem siatki widać w oświetleniu),
oświetlenie jak grunt tego poziomu (`lightWay`, `cloudsShadeAt` na L4, mgła, `U.Sight`), bez
cienia wypieczonego i lamp (L4 ich nie czyta). Shader przez `leveled` (konkatenacja z `const
LEVEL`), więc deklaruje w swojej liście uniformów wszystko, co czytają pożyczone pliki
(`lattice.wgsl`: `Cell, Corners, Low, Span` i slot 0; `light.wgsl`: `Cell, Corners`); własne
`DrawMeshOptions` (własne `Images`; `Uniforms` wspólne z gruntem, `Layout.Put` ignoruje nieznane
nazwy), `Primed` z `fs_prime` odrzucającym alfę 0 albo `Primed: false, WriteDepth: true` z
`discard` w `fs_main` (porównanie głębi `GreaterEqual`, odwrócone; brak kontraktu early-z). Siatka płachty rysowana tylko dla sektorów, które
mają co najmniej jedną encję w płachcie (lista z `Read.Sheet`-owego `Painted(i) bool`; pusty
sektor nic nie kosztuje). Dzięki temu płachta nie zszywa się z siatką terenu — leży na niej jako
bryły; szwy między poziomami załatwia część 2 na encjach. Płaska plansza (`look/tiles.go`) i heksy
(`hexes.go`) kładą albedo płachty (L4 i L5, odpowiedni obraz) nad kaflami tam, gdzie kładły sumę,
płasko, bez normalnej (świat płaski; hex to osobny temat).

**Szwy L3|L4** (część 2): poziom rysowania encji `Fixed` = najdrobniejszy z poziomów sektorów,
które pokrywa `Grid.Cover(box.Grown(size))`; liczony w publisherze tylko dla encji `Fixed` w
sektorach L4+ sąsiadujących z drobniejszym; taka encja rysuje się żywo wersją drobniejszego
poziomu i **nie wchodzi do płachty** (fotografia pomija encje z poziomem rysowania < L4); zmiana
poziomu sąsiada przez L3|L4 podnosi wersję treści sektora. Znak ticku zostaje po środku.

**Banery** (część 3): `Config.Far FarLook` (`Points` zero, `Banner`), `Config.Counts bool`,
`Settings{…, Far FarLook, Counts Shown}` z **zerem znaczącym „jak Config"** (`plugin.Keep`
dekoduje stary zapis w zerowy struct i nadpisuje nim wartości z Config, więc zero musi być
neutralne: `FarLook{AsConfig 0, Points 1, Banner 2}` w ustawieniu, `Shown{AsConfig 0, On 1, Off 2}`
dla liczb; `Read.FarLook()`/`Read.Counts()` rozstrzygają), komendy `ShowFar{Look}`,
`ShowCounts{On}`, ustawienia w F10; `Read.Banners(cam, dst) []Banner{Kind kind.ID, Sector int,
Center geom.Vec, IDs []uid.UID64, Count int}`; klik w baner zaznacza `IDs`; liczby na **szkle**:
renderer świata implementuje `render.Glass` (`DrawGlass(dst, cam)`, wołane przez feed po wszystkim,
jak `cameras/zones.go`), tekst `render.DefaultFont().Draw(dst, s, x, y, c)` przy
`cam.Project(x, y, z)` centroidu, przesunięty o `dst.Bounds().Min`.

**Siatka L5 i albedo zmniejszone** (część 4): `coarse` sparametryzowane krokiem (2 na L4, 4 na L5,
pierścień jak dziś), albedo malarza w pół i w ćwierć (`DrawImage`) po każdym wypieku, teren
wiąże w `opts.Images[1]` obraz poziomu (pełny L2–L3, pół L4, ćwierć L5).

### Model wykonania

Jak w etapie b (`CLAUDE.md` „A stage in parts"): integrator planuje, briefuje, scala, mierzy
(≤10 min), pisze dokumenty; agenci Opus 5.5 (`Agent`, `model: opus`): preludium w klonie na
`feat/sheets`, części w worktree'ach `/home/karol/worktrees/gram/<część>` na `p/<część>` od
commitu preludium (`git worktree add /home/karol/worktrees/gram/<część> -b p/<część>
feat/sheets`; tam `GOWORK=off` niepotrzebne, `go.mod` wystarcza). **Wspólne reguły dla każdego
agenta** (do wklejenia w brief): czytaj skille warstw (`gram-plugin`, `gram-plugins`,
`gram-render`, `gram-perf-picture`, `gram-perf-tick`, przy testach `gram-probe`); trzymaj się
plików części; `go build ./... && go vet ./... && gofmt -l . && go test ./...` przed każdym
commitem (`GOEXPERIMENT=simd`); commity z prefiksami konwencjonalnymi i trailerem
`Claude-Session: https://claude.ai/code/session_014GbD5V5FZgxcEYK1zPRzjF`, bez `Co-Authored-By`;
żadnych zmian w `doc/`, `.claude/`, `BENCHMARKS.md`, `CHANGELOG.md`, `go.mod`, README; godoc ≤ 2
zdania, komentarze tylko gdzie kod nie powie; linie ≤ 100 kolumn; nic nie mierzyć (benche
pisać i podać komendy, integrator mierzy); demonstracje tylko na kopii dema (`gram-probe`,
`run_demo.sh`), nigdy na plikach dema; nic nie wypychać; raport końcowy: co zrobiono, odstępstwa
od briefu z powodem, komendy benchów, co zostało.

### Preludium (agent A, `feat/sheets`): kontrakty

Pliki: `plugins/world/{fixed.go (nowy), commands.go, rw.go, config.go, plugin.go, module.go,
sheet.go (nowy: typy i puste wiązania), aggregate.go (usunięcie), renderer.go, publisher.go
(tylko usunięcie sumy), doc.go (jedno zdanie), atlas.go (test)}`, `lod/lod.go` (godoc `Texel`),
`plugins/board/{templates.go, plugin.go, look/renderer.go, look/tiles.go, look/shaders/
tiles_common.wgsl}`, `plugins/topography/{plugin.go, internal/terrain/terrain.go, internal/
terrain/shaders/surface.wgsl, internal/terrain/shaders/mesh.wgsl, internal/hexes/hexes.go,
internal/hexes/shaders/sheet.wgsl}`, `plugins/bridges/types.go`, `bench/world_test.go`, testy.

1. `world.Fixed{}` (`fixed.go`): zerowy komponent-znak encji stojącej, rejestrowany w świecie
   (`si.RegComp`), zapisywany (`goke.LoadComp[Fixed]` w `module.go` za `Unfolded`), w słowniku
   `doc.go`. `board.StandingTemplate` daje go domyślnie (obok `world.Lie{}`), więc `NewEntities`
   i pokłady go noszą; filary mostów dostają `world.Fixed{}` w `Define` (`types.go:104`).
   `SpawnPart` i `NewUnits` nic nie dodają. Test: encja z `NewEntities` ma `Fixed`, jednostka nie,
   zapis/odczyt zachowuje.
2. **Suma odchodzi**: usunąć `aggregate.go`, `Plugin.Aggregates()`, `Read.Aggregate`,
   `AggregateImage`, `export_test AggregateTexels`, testy sumy (`aggregate_test.go`,
   `aggregate_internal_test.go`; `field_internal_test.go:52` przepisać bez sumy), rungi
   `World_Publish|Draw` „level=3, unread" (zostaje „level=3": rekordy plam), `worldLeveled(…,
   read)` bez `read`; publisher: `k.l3`, `recorded`→`fate` bez `sum` (L5 jak L4: rekord), `spread`
   na L5 bez `add`; atlas: `MeanColour` w `level()` i `a.colours` odchodzą (plama L4 liczy swój
   kolor jak dziś przez `render.Blob`), `blobColour` zostaje (część 3 bierze z niego kolor liczby);
   **`versionsOf` daje plamę także na L5** (`of[L5] = of[L4]` gdy niezadeklarowane; dziś L5 to
   zero = nic, `atlas.go:579-598`) i **`Compose` posyła L5 do `s.point` jak L4**
   (`renderer.go:179` warunek `level == lod.L4` → `>= lod.L4` dla niezadeklarowanych): jednostki na
   L5 stają się miniaturami już w preludium, co zastępuje sumę na oko. Testy publishera/renderera
   na L5: rekord plamy, `Keep[L5]` rzedzi.
   Klienci: `board/plugin.go:136` `WithAggregate` → `WithSheet(worldPlugin.Read(0).Sheet)` i
   `worldPlugin.Sheets()`; `look/tiles.go aggregate()` → `sheet()` wiążące `opts.Uniforms[world.
   SheetImage]` (L4: obraz L4, L5: obraz L5; `TileSheetPx`) — `summedOver` czyta `Sheet` zamiast
   `Aggregate` (na razie obraz jest pusty, więc kafle bez zmian na oko); topografia analogicznie
   (`terrain.Config.Sheet func() (…)`, `AggregatePx` → `SheetPx`, `aggregateAt` → `sheetAt` czyta
   `Sheet` na L4 **i** L5; heksy tak samo). Rejestracja `RegisterImage(SheetImage)` i
   `RegisterImage(SheetReliefImage)` w `init` `sheet.go`.
3. `sheet.go`: typ `sheets` (nie `sheet`: `render.Sheet` to arkusz sprite'ów) z polami obrazów
   (`albedo, relief, albedoL3, reliefL3 *render.Image`, `perUnit`, `cols, rows`, `version, painted
   []uint32` na sektor, `perFrame int`), `newSheets(cfg, grid)` (teksel jak w „Projekt": `cfg.LOD.Texel`, zero → `1/Scales[L4]`, podniesiony do ≤ 4096
   tekseli na bok; obrazy tworzone leniwie przy `Sheets()`), `Plugin.Sheets()` (odpowiednik
   dawnego `Aggregates()`), `Read.Sheet()` (nil, 0 gdy brak), `Read.Painted(i int) bool`,
   `sheet.bump(i)` (wersja treści), hak `sheet.neighbourChanged(i)` (= `bump`). Fotografii jeszcze
   nie ma: `Painted` zawsze fałsz, obrazy puste (`Clear`). `lod.Ladder.Texel` godoc: „teksel
   płachty tego, co stoi, w jednostkach świata; zero: jednostki na piksel na progu L4; świat
   podnosi go tak, by płachta miała ≤ 4096 tekseli na bok". `Config.LOD.Check` bez zmian.
4. `Config.Far FarLook` (`type FarLook uint8`: `AsConfig 0` tylko w ustawieniu, `Points 1`,
   `Banner 2`; w Config zero znaczy `Points`), `Config.Counts bool`; `Settings{Sectors, Unbanded,
   Far FarLook, Counts Shown}` (zero = jak Config, patrz „Projekt/Banery": stary zapis nie może
   nadpisać Config zerami), `Read.FarLook() FarLook` i `Read.Counts() bool` rozstrzygają; komendy `ShowFar{Look FarLook}`, `ShowCounts{On bool}` (`commands.go`,
   `var _ command.Command`, kolejki w `module.go` i `Queues()`, obsługa jak `switchSectors` →
   `write.ShowFar`, `write.ShowCounts`); `Settings()` dodaje `control.Choice{Label: "Far units",
   Options: {"points", "banners"}}` i `control.Switch{Label: "Counts"}` w `Section:
   control.Graphics` (sprawdzić, jak inne pluginy nazywają sekcję); `Read.FarLook()`,
   `Read.Counts()`. Testy: komenda zmienia ustawienie, zapis/odczyt je zachowuje, `Settings()`
   ma 4 pozycje ze sektorami.
5. `type Banner struct{ Kind kind.ID; Sector int; Center geom.Vec; IDs []uid.UID64; Count int }`,
   `Read.Banners(cam camera.Camera, dst []Banner) []Banner` (w preludium zawsze puste; godoc mówi,
   że wypełnia je publikacja w trybie Banner).
6. Test atlasu: wersja każdego poziomu (plama L4, `At(L4)`, `At(L5)`) jest rysowana w pudle encji
   i na jej `Z` — utrwala ślad, na którym stoi zszywanie (`atlas_test.go` albo `renderer` test:
   `Published()`/`Drawn` z `export_test.go`).
7. Sekwencja zielona; dema topography, flat, forest-army, atlas, navigation/square 5 s na kopii;
   zrzut topography L5 różni się od bazy tylko tym, że figury są plamami, nie kolorem w gruncie
   (nazwać). Commity: `refactor: the L5 colour sum goes, the sheet's images and settings stand in
   its place` (można 2–3 commity tematyczne).

### Część 1 `p/sheet` (agent B, po preludium): fotografia, L5 w pół, grunt z reliefem płachty

Pliki: `plugins/world/{sheet.go, sheet_gpu.go (nowy), shaders/sheet_photo.wgsl, shaders/
sheet_half.wgsl (nowe), publisher.go, renderer.go (Compose: fotografie i rekordy), wholes.go
(migawka części przy fold), rw.go, doc.go (słownik), export_test.go}`, `plugins/topography/
internal/terrain/{terrain.go, photo.go (nowy: krata płachty), shaders/photo.wgsl (nowy),
shaders/surface.wgsl}`, `plugins/board/look/{tiles.go, shaders/
tiles_common.wgsl}` (albedo L4 i L5 nad kaflami, jeśli preludium nie domknęło), `plugins/
topography/internal/hexes/*` (albedo płachty nad quadem sektora), `bench/world_test.go`, `bench/
frame_test.go`, testy. Nie dotyka: `atlas.go` poza odczytami, `sectors.go` świata, `commands.go`,
`selection`, `bridges`, `painter`.

Kroki:
1. **Migawka części** (`wholes.go`): w `fold` (:430), zanim `Fold` zostanie powiedziany, dla
   rozciągu, którego całość nie deklaruje L4 (`wa := s.w.p.renderer.atlas.(*Atlas)`,
   `wa.declares(sprite całości, L4)`; sprite całości przez `Base.TypeID → ikinds.Kind.SpriteID`
   albo `OptComp[render.Appearance]` w `s.all`), zebrać `snapshot{box, z Z, sprite
   render.SpriteID}` części (pudło z `partBase`; dodać `partZ goke.OptComp[Z]` i `partLook
   goke.OptComp[render.Appearance]` do zapytania `s.parts`, oba `Optional`; sprite L3 przez
   `wa.levels.at(sprite, lod.L3)`), `sheets.folded[stretch] = snaps`, `sheets.bump(sector)`; w `unfold`/`open` usunąć i `bump`. Pola (`Field`) po sektorze:
   migawka na sektor. Test (`wholes` rig z `field_internal_test`/`aggregate` kontraktu): po `Fold`
   migawka ma tyle wpisów, ile części, po `Unfold` jest pusta; z deklaracją L4 nic.
2. **Fotografia na GPU** (`sheet_gpu.go`, `sheet_photo.wgsl`): shader `NewMeshShaderWith("world's
   sheet", …, uniforms{SheetOrigin, SheetPx, SheetPass, SheetSpan}).Instanced(3)` (instancja:
   pudło, UV sprite'a, `Altitude, Height, Lying, 0`); `opts.Images[0] = atlas.Atlas()`; dwa
   przebiegi do bufora sektorowego (`SheetPass` 0 albedo, 1 relief) z jedną głębią, oba `Primed`
   z `fs_prime` odrzucającym alfę 0, głębia znormalizowana przez `SheetSpan` (zakres wysokości
   świata z `Low/Span`? świat go nie zna: wziąć `max |Altitude + Height|` po instancjach sektora
   jako skalę, to tylko porządek), bias `instance_index · 1e-6`; `CopyFrom` do `SubImage` sektora
   w obu obrazach L4. Kodowanie: wysokość f16 w RG (`pack2x16float` — naga v0.20.5 ma je w WGSL,
   SPIR-V i MSL; bajty `/255`), normalna w BA; odczyt bajtów po stronie terenu przez `textureLoad`
   jak `lattice.wgsl corner()`. Kopuła i `Lying` jak w „Projekt". Zbieranie instancji: w `Compose` publishera po rekordach, z
   `figures.all` (encje `Fixed` bez L4, poziom rysowania ≥ L4 — część 2 doda wyjątek pierścienia,
   tu: wszystkie `Fixed` sektora) i z `sheet.folded`; sektory do fotografii: `v.Sectors` każdego
   widoku, poziom ≥ L4, `painted[i] != version[i]`, ≤ `perFrame` (8); po przebiegu `painted[i] =
   version[i]`; potem przebieg `sheet_half.wgsl` dla prostokątów L5 tych sektorów (relief: średnia
   po tekselach z alfą, normalna średnia renormalizowana; albedo: `DrawImage` w pół albo ten sam
   shader; `DrawImage` w pół: `GeoM` liczy w pikselach tekstury, nie podobrazu — przesunąć o
   `Bounds().Min` celu; `FilterLinear` w skali 0.5 trafia w rogi tekseli, więc daje dokładną
   średnią 2×2). Wersja treści z liczby i xor identyfikatorów `Fixed` na sektor (patrz „Projekt"),
   `OptComp[Fixed]` w zapytaniu publishera wzorem `keen` (`renderer.go:32`, `publisher.go:52-54`,
   `s.keen.Slice(cur) != nil`).
3. **Publisher**: `fate`: encja `Fixed` (opcjonalny komponent w zapytaniu) z poziomem ≥ L4 w
   sektorze `painted[i] == version[i]` → bez rekordu; inaczej jak dziś. Jednostki na L5: rekord
   plamy (jak L4) — upewnić się, że `Compose` na L5 idzie przez `s.point` jak L4 (dziś L5 bez
   wersji nie rysuje). `Keep[L5]` rzedzi. `Read.Painted(i)`.
4. **Teren** (`terrain/photo.go`, `shaders/photo.wgsl`, typ `photoMesh`): krata płachty na
   sektor (indeksy raz, jak `lay`; krok z „Projekt"), rysowana po siatce poziomu w `Draw`
   (`terrain.go:199-217`, `eachLevel` :227) dla sektorów L4/L5 z `Painted(i)` (lista `Ranges`),
   własne `DrawMeshOptions{Images: {lattice, Sheet lub SheetL3, SheetRelief lub SheetReliefL3,
   albedo malarza}, Uniforms: r.opts.Uniforms}`, uniformy `SheetPx`, `SheetStep`, `SheetSink` plus
   te, których wymagają pożyczone pliki (patrz „Projekt"); shader: wierzchołek → teksel →
   wysokość z reliefu albo lattice−sink; fragment: alfa 0 → `discard`, kolor albedo · światło
   (`lightWay` z normalną reliefu, `cloudsShadeAt` na L4, mgła jak `mesh.wgsl` tego poziomu),
   `Depth` wspólna z gruntem; dwa shadery przez `leveled` (L4, L5) w `TestTerrainShaders_Compile`. `aggregateAt`/`sheetAt` w `surface.wgsl` odchodzi z `mesh.wgsl` dla
   terenu (płachta rysuje się własną kratą; płaska plansza i heksy kładą albedo płasko jak w
   preludium).
5. Testy GPU (`gputest.Need`, `ReadPixels`): (a) świat: dwie encje `Fixed` jednego koloru w
   sektorze widzianym na L4 → po `Compose` teksele albedo w pudłach mają ten kolor (±1/255), poza
   pudłami alfa 0; wysokość w środku pudła = `Altitude + Height` (±0.5 % f16), normalna w górę w
   środku, pochylona na zboczu kopuły; `Lying` płasko; rodzaj z `At(L4)` nie trafia do płachty;
   `Fold` agregatu bez L4 maluje części, `Unfold` czyści; wersja rośnie przy spawn, nie rośnie bez
   zmiany; `Painted` fałsz do pierwszej fotografii, potem prawda; L5 teksel = średnia 2×2;
   encja `Fixed` w sektorze namalowanym nie ma rekordu L4 (`Published()`), jednostka na L5 ma
   rekord. (b) teren: `draw_internal_test` wzorzec: sektor na L4 z płachtą o jednym pudle →
   piksele w pudle mają albedo płachty, poza nim grunt; dwa kierunki `LightWay` dają jaśniejszą
   stronę kopuły od światła; kamera pochylona: szczyt kopuły przesunięty względem jej podstawy
   (paralaksa) — `TestTerrainShaders_Compile` obejmuje nowy shader na L4 i L5.
6. Benche: `Benchmark_World_Sheet` w `bench/world_test.go` („sector,64": fotografia jednego
   sektora z 64 encjami `Fixed` włącznie z czekaniem na GPU; „frame,8": klatka z 8 sektorami do
   fotografii), `World_Publish|Draw level=3` zostają (teraz plamy), `Frame_Topography` nowy rung
   „iso,level=2,sheet" (grunt L4 z płachtą w 50 % sektorów). Cele podane w „Pomiar".
7. Sonda topografii na kopii: `GRAM_SHOT` z kamerą oddaloną do L4: latarnia i mosty w płachcie
   (różnica względem bazy nazwana), log `Painted` per klatka: ile sektorów sfotografowano,
   stabilizuje się do 0 po kilku klatkach.

### Część 2 `p/seams` (agent C, po części 1): szwy na granicach, mosty

Pliki: `plugins/world/{publisher.go (poziom rysowania Fixed), sheet.go (wykluczenie, hak
sąsiada), renderer.go}`, `plugins/bridges/doc.go`, testy. Nie dotyka: `sectors.go` (znakowanie
ticku bez zmian), terenu, atlasu.

1. **Poziom rysowania** encji `Fixed`: maska sąsiedztwa na publikację: dla każdego widoku
   sektory L4+ mające sąsiada (`Grid.Neighbours`) drobniejszego; dla encji `Fixed` w takim
   sektorze poziom = min po `Grid.Cover(box.Grown(size), dst)` (`size` = większy bok pudła);
   inaczej jak dziś (`v.Level(mid)`). Encja z poziomem < L4 rysuje się żywo (rekord) i nie wchodzi
   do zbioru fotografii (filtr w zbieraniu instancji części 1: `drawLevel >= L4`).
2. **Wersja treści**: gdy zbiór sektorów „L4+ z drobniejszym sąsiadem" zmienia się między
   publikacjami (sektor wchodzi albo wychodzi ze zbioru), `sheet.neighbourChanged(i)`.
3. **Mosty**: pokłady i filary są `Fixed` (preludium), więc na szwie L3|L4 pokład żywy i pokład w
   płachcie leżą na jednej wysokości (`Altitude + Height` w f16) i zmienia się tylko detal;
   plugin mostów nic nie deklaruje; `bridges/doc.go` jedno zdanie.
4. Testy (`plugins/world`, rig `sectorWorld` + płachta): encja `Fixed` przecięta granicą L3|L4
   ma rekord L3 i nie ma jej w płachcie; sąsiadka w L4 przy granicy (pudło powiększone sięga
   L3) rekord L3; dalsza w L4 żaden rekord, jest w płachcie; po przejściu sąsiada na L4 (kamera
   dalej) wszystkie trzy w płachcie, wersja treści sektora wzrosła dokładnie raz; jednostka
   (nie-`Fixed`) przy granicy jak dziś; `routes.golden` nietknięte; zrzut forest-army piksel w
   piksel z bazą (las i armia mają własne L4). Sonda topografii: most przez granicę L3|L4 w
   perspektywie, zrzut; wysokość pokładu odczytana z reliefu płachty równa `Z.Top()` kawałków
   żywych ±0.5 %.

### Część 3 `p/far` (agent D, po części 1, równolegle z 4): banery, miniatury, liczby, klik

Pliki: `plugins/world/{publisher.go, renderer.go, rw.go, doc.go, banners.go (nowy)}`,
`plugins/selection/{selection_system.go, plugin.go, doc.go}`, testy. `render` tylko gdy szkło nie
ma wejścia dla pluginu (wtedy stop i raport, nie improwizacja).

1. **Banner** (`banners.go`): w trybie `Banner` jednostki (nie-`Fixed`, nie `Unfolded`
   agregaty) jednego rodzaju w sektorze L4+ widoku, których atlas **zadeklarował** L4 (`declares`),
   dają zamiast rekordów jeden rekord w centroidzie pudeł, wersja L4 rodzaju, pudło rozmiaru
   sprite'a; `Banner{Kind, Sector, Center, IDs, Count}` do `Read.Banners(cam)` (bufor podwójny
   jak rekordy). Rodzaj bez deklaracji L4 zostaje miniaturami także w trybie Banner. `Keep`
   rzedzi miniatury, nie banery. **Points**: dziś.
2. **Counts**: liczba jednostek rodzaju w sektorze (banner: `Count`; points: skupisko ≥ 2 przy
   centroidzie) rysowana na szkle: renderer świata (już `Direct` + `Overlying`,
   `renderer.go:21-22`) implementuje `render.Glass` (`DrawGlass(dst *render.Image, cam
   camera.Camera)`, wzór `cameras/zones.go:84-113`), tekst `render.DefaultFont().Draw(dst, s, x,
   y, c)` w kolorze rodzaju (`blobColour`) przy `cam.Project(x, y, z)` centroidu (piksele okna
   kamery), przesunięty o `dst.Bounds().Min`; liczby z ostatniej publikacji (bufor jak rekordy).
3. **Klik**: `NewSelectionSystem` dostaje `banners func(cam camera.Camera, dst []world.Banner)
   []world.Banner` (z
   `worldPlugin.Read(0).Banners` w `selection/plugin.go`); `pick`: gdy pod kursorem leży pudło
   banera (ekranowo, jak `drawnIn`), `Select{IDs: banner.IDs}` (`Allow` respektowane); poza
   banerem jak dziś.
4. Testy (`plugins/world`, rig `published`/`thinnable` z `At(L4)`): dziesięć jednostek rodzaju
   z L4 w sektorze L4 w trybie Banner → jeden rekord, `Banners` ma `Count 10` i 10 `IDs`; rodzaj
   bez L4 → dziesięć plam; `ShowFar{Points}` → dziesięć rekordów; `ShowCounts{On}` → znak liczby
   (w `Published()` albo osobny odczyt); zapis/odczyt zachowują ustawienia. `plugins/selection`:
   klik w baner zaznacza dziesięć; klik obok nic. Sonda forest-army (kopia): 20 żołnierzy luzem na
   L4 (rozstawieni sondą), zrzut w obu trybach i z liczbami.

### Część 4 `p/terrain` (agent E, po preludium, równolegle z 1–3): siatka L5, albedo w pół

Pliki: `plugins/topography/internal/terrain/{sectors.go, terrain.go, shaders/mesh.wgsl}`,
`plugins/topography/internal/painter/{bake.go, bake_gpu.go}`, testy. Nie dotyka: `surface.wgsl`,
`sheet.*`, świata.

1. `coarse(c, dst, step)` z krokiem 2 (L4) i 4 (L5): wnętrze co `step` narożników, pierścień
   jednej komórki wachlarzami do pełnych narożników brzegu (jak dziś; krok 4 → wachlarz łączy 4
   zewnętrzne z 1 wewnętrznym), sektor za mały na krok wraca do mniejszego; `sectorSpans{whole,
   coarse, coarser}`, `lay` układa trzy przebiegi, `runs` wybiera po poziomie (L4 `coarse`, L5
   `coarser`), horyzont bez zmian.
2. Albedo malarza w pół i w ćwierć (`groundSheet.half, quarter *render.Image` odnawiane przez
   `DrawImage` po każdym `bake`, tylko prostokąty zmienione przy wypieku częściowym, całość przy
   pełnym); `Painter.Surface` zwraca też je (albo `Surfaces(level)`); teren wiąże
   `opts.Images[1]` po poziomie w `Draw`; `mesh.wgsl` bez zmian poza tym, że na L4/L5 czyta już
   mniejszy obraz (ta sama UV).
3. Testy: `sectors_internal_test` krok 4: każda krawędź wnętrza dzielona ma wierzchołek po obu
   stronach (brak pęknięć na granicy 4|2|1), liczba trójkątów jak wzór; test GPU malarza: pół =
   średnia 2×2 pełnego (±1/255); test terenu: zrzut L5 bez częstotliwości komórki (energia widma
   wzdłuż wiersza na okresie komórki ≥ 4× niższa niż na L3). `Benchmark_Painter_Bake` ±5 %
   (zmniejszanie w cenie), `Frame_Topography iso,level=3` ≤ dziś.

### Kolejność, szwy, scalanie

Preludium → (1 ∥ 4) → 2 → 3 (3 może iść równolegle z 2: pliki rozłączne poza `publisher.go` —
jeśli oba dotykają `fate`, 3 startuje po scaleniu 2). Szwy: `world.Fixed`, wersja treści i
`Painted` (prelude↔1↔2), `Read.Banners` (prelude↔3↔selection), `Read.Sheet`/nazwy obrazów
(prelude↔1↔board/topography), `opts.Images[1]` po poziomie w `terrain.Draw` (1↔4: 4 ustawia
albedo poziomu, 1 dodaje osobny draw płachty po nim — konflikt tekstowy w `Draw`, integrator
scala). Integrator po każdym scaleniu: sekwencja, dema 5 s, push `feat/sheets`, worktree
usunięty.

### Pomiar (integrator, ≤10 min, baza `main` `7496b324` po `e` na zmianę z drzewem, `measure.sh`)

| Bench | Cel |
|---|---|
| `World_Publish|Draw …level=3` | ≤ 60 % bazy (plamy zamiast sumy; baza miała sumę ~94 ns/encję) |
| `World_Publish|Draw` pozostałe rungi | ±2 % |
| `World_Sheet/sector,64` | ≤ 0.2 ms (z czekaniem na GPU) |
| `World_Sheet/frame,8` | ≤ 1.5 ms |
| `Frame_Topography iso,level=2` | ±3 %; `iso,level=2,sheet` ≤ level=2 + 0.3 ms |
| `Frame_Topography iso,level=3` | ≤ baza |
| `Painter_Bake` (pakiet malarza) | ±5 % |
| `Sectors_Expanse` | ±3 % |

Sonda topografii (`GRAM_FPS_LOG=1 GRAM_VSYNC=off`, kamera oddalona do L4 i do L5, 35 s po
rozgrzewce, baza przeciw drzewu dwa razy): FPS, tick, drawing, presenting; zrzuty: forest-army
L4/L5 piksel w piksel z bazą; topography L4 (latarnie i mosty w płachcie: różnica nazwana), most
przez granicę L3|L4, F10 baner/miniatury/liczby z 20 żołnierzami luzem w forest-army.

### Dokumenty (integrator)

`lod/doc.go` (tabela: figura L4 „plama; co stoi: płachta sektora", L5 „płachta w pół; jednostki
miniaturami", grunt L5 „siatka co czwarty narożnik, albedo w ćwierć"; akapit o banerach i
`Texel`), `CLAUDE.md` (reguła LOD: płachta i jej źródła, L5 gorszą wersją L4, poziom na kawałek
i pierścień, baner ustawieniem; tabela dem bez zmian), `plugins/world/doc.go`, `topography/
doc.go`, `board/doc.go`, `bridges/doc.go`, `selection/doc.go`, `CHANGELOG.md` (pod v0.20.0
unreleased: Added płachty, `Fixed`, banery i liczby, `World_Sheet`; Changed L5 i `Texel`,
jednostki na L5 miniaturami; Removed suma, `Aggregates`, `Read.Aggregate`, `AggregateImage`),
`BENCHMARKS.md`, skille `gram-render` (obrazy płachty, nazwy 8/8, f16 w RG, brak mipmap i jak
to obchodzimy), `gram-plugins` (world :110-135, board :260, topography :305, bridges, selection),
`gram-perf-picture` (:26-46), `doc/roadmap.md` (zdjąć „A far figure's cost" część o sumie;
dodać: wygląd pod efektem nie odświeża płachty; cień płachty na gruncie na L4; skok ticku po
~2 min Madery), `doc/plans-perf.md` (sekcja etapu i wynik; etap `f`: części 2–3 bezprzedmiotowe,
część 1 zostaje). Pamięć `lod-plan.md`.

### Weryfikacja końcowa

Sekwencja z SIMD; wszystkie dema 5 s; topography (zoom L2→L5: latarnie i mosty w płachcie,
most spójny na szwie, F10 banery/miniatury/liczby), forest-army (bez zmian na oko; żołnierze
luzem w obu trybach), flat, atlas, navigation/square, expanse; worktree'y usunięte; gałąź
wypchnięta; zmiany integratora w stage; raport z liczbami i tym, co zostało.

### Co nie wchodzi

Cień płachty na gruncie (L4 nie czyta `GroundShade` z słowa drabiny; płachta w wypieku cienia to
osobny punkt roadmapy), impostor pionowy z wielu kierunków, odświeżanie płachty przy zmianie
samej fazy animacji (`progress`) bez zmiany sprite'a, płachty do plików (etap `d`), chmury po poziomie, selekcja a
śpiący, poziom w rekordzie (etap `f` część 1), relief płachty na płaskiej planszy i heksach
(albedo płasko), więcej niż 8 nazw obrazów klatki.

---

### Wynik etapu `p` (2026-11-06, `feat/sheets`, baza `main` `7496b324` po `e`)

**Wykonanie.** Preludium (agent A, w klonie): `29be1637` `world.Fixed`, `04d64728` suma L5
odchodzi i obrazy płachty, `81766e1d` ustawienia `FarLook`/`Counts` i `Read.Banners`. Części
w worktree'ach od `81766e1d`: E `p/terrain` (`b1078fab`, `71979082`, `49a4780e`, `20988a2a`;
scalenie `7393f835`) i B `p/sheet` (`331b35b2`, `ae862838`, `63ec11c3`; scalenie `1d3b9df2`,
konflikty w `terrain.Draw` i `mesh.wgsl` rozwiązane ręcznie: wiązanie albedo poziomu z E zostaje,
płaska płachta preludium odchodzi) równolegle; potem od `1d3b9df2` D `p/far` (`5ea303c1`,
`b285b981`, `2da56624`; scalenie `d0e51f8e`) i C `p/seams` (`b2929e06`, `195d1de6`, `7c102f6e`;
scalenie `4d4f2cff`, konflikt w polach `figure`: `banner` i `ringed` oba, struktura dalej 88 B)
równolegle. Każde scalenie: sekwencja zielona (90 ok, 41 bez testów), dema.

**Odstępstwa przyjęte.** Preludium: `blobColour` zostaje (część 3 go czyta; teraz
`farColour`), `AsConfig` jedno nietypowane zero, stała `Banners` (nie `Banner`), kafle płaskiej
planszy kładą płachtę od L4, teksel domyślny spada na bok sektora / 64 przy zerowym progu L4.
Część 1: zdjęcia zbierane w `Publish`, nie w `Compose` (stan płachty na wątku gry, do klatki
przez bufor podwójny, wywołanie `img.Defer` planowane w `Swap`; w `Compose` nie działało, bo
renderer świata jest `Overlying` i składa się po gruncie, więc każde zdjęcie było wiecznie
ponawiane); sygnatura liczy wszystkie `Fixed` sektorów rysowanych na L4+ (nie tylko widzianych),
z wysokością i miejscem; migawki tylko dla pól (agregat punktowy się rusza); krata płachty bez
bufora indeksów, instancja na sektor, narożnik na najwyższym szczycie w kwadracie (sam teksel
narożnika topił rzeczy węższe niż kwadrat); instancja tła pisze „pusto, w górę" do reliefu;
heksy kładą albedo płachty też na szczytach L4. Część 4: uniform `AlbedoPx` (albedo czytane w
pikselach obrazu, `U.Px` służy mapie rodzajów), `Painter.Shrunk()` zamiast zmiany `Surface`,
resztka pierścienia poszerza pierścień zamiast kłaść całe komórki (sektor Madery na L5: 94
trójkąty wobec 190 na L4 i 512 pełnych). Część 2: hak `neighbourChanged` usunięty — sygnatura
liczy rzeczy wbite w płachtę, więc przejście sąsiada przez szew podnosi wersję dokładnie raz,
wtedy gdy zdjęcie się zmienia; hak na zbiorze pierścienia fotografowałby sektory bez zmian i
przy przesunięciu kamery (pierścień o rząd sektorów, więcej niż `perFrame`) migotałby je do
punktów; szew L4|L5 nietknięty; pierścień tylko przy otwartych płachtach. Część 3:
`NewSelectionSystem` bez zmiany sygnatury (pole `banners` ustawiane w `Install`); banery to lista
grup w buforze podwójnym, nie rekordy; „pudło rozmiaru sprite'a" = średnie pudło jednostki;
`pick` (hover, komendy wskazane) banerów nie zna, tylko `Select`.

**Sprawdzenia deterministyczne.** forest-army piksel w piksel z bazą na L4 i L5 (las i armia
deklarują własne wersje). topography L5 (próg L4 podniesiony sondą do 0,24, 247 sektorów L5):
2152 piksele w 33 komórkach, wszystkie na mostach, brodach i jednostkach — w bazie rozmyty kolor
w gruncie, w drzewie plama na własnej wysokości (preludium); L4: 2412 pikseli, pokłady mostów
jako cienkie pasy z płachty zamiast łańcucha plam; liczba sektorów namalowanych rośnie 14 → 23 i
stoi. Szew L3|L4: kawałek pokładu przy granicy rysowany żywo (26 pikseli różnicy poza panelem),
wysokość pozostałych w płachcie równa `Z.Top()` z dokładnością 0,02 %. Siatka L5: trzy kroki
1|2|4 obok siebie bez pęknięć (każda krawędź wewnętrzna w dokładnie dwóch trójkątach); albedo w
pół i ćwierć = średnie 2×2 i 4×4 (±1/255, ±2); widmo wiersza na okresie komórki na L5 ≥ 4×
niższe niż na L3. Banery: tryb Points piksel w piksel z bazą; Banners 20 punktów → 2 banery;
liczby „12", „8", „1"; klik w baner zaznacza 12, obok 0. `routes.golden` nietknięte. Na oko do
decyzji: na L4 cienkie rzeki i drogi są bledsze, zmieszane z lasem w tekselu połówki albedo
(67 tys. pikseli różnicy w zrzucie iso L4); jeśli to przeszkadza, L4 może czytać pełne albedo,
pół zostawiając L5.

**Pomiar** (baza na zmianę z drzewem, 2 rundy × `-count=2`; maszyna niosła obciążenie 2–3
z zapisu dysku i indeksera pulpitu, nie z CPU — próg 1,5 `measure.sh` obejściem, mediany):

| Bench | Zmiana | Uwaga |
|---|---|---|
| `World_Draw` wszystkie rungi L2/L4 | −2,4…+4,8 % mediany, min ±2 % | szum; alokacje 0 |
| `World_Draw …level=5` | 0,48 µs → 3,7 µs (49), 0,49 → 309 µs (5000) | plan: L5 rysuje plamy jak L4 (baza nic) |
| `World_Publish …level=5` | −83 % (657 → 112 µs), −16 % (49) | suma odeszła; cel ≤ 60 % spełniony |
| `World_Publish` pozostałe rungi | **+20…28 %** przed poprawką → **−3…−10 %** po `p/publish` | patrz niżej |
| `World_Sheet/sector,64`, `frame,8` | 1,27 ms / 1,96 ms, 1131 alokacji (po `p/photo`; przed: 1,30 / 5,9 ms, 1304 / 7329) | podłoga benchu ~0,45 ms, 436 alokacji (wgpu) |
| `Frame_Topography iso,level=2/3/4/5` | +0,2 / +0,4 / −1,7 / −4,9 % | L4 albedo w pół, L5 siatka co czwarty |
| `Frame_Topography iso,level=4,sheet` | 14,1–14,7 ms wobec 12,8 ms `level=4` (po `p/photo`; przed 14,7) | raster ścian kopuł; cel +0,3 ms nieosiągnięty, roadmapa |
| `Sectors_Expanse banded/unbanded` | −2,1 / −2,6 % mediany, min ±1 % | bez zmian |
| `Painter_Bake whole/cell` | +0,1 / +5,8 % mediany, min +1,1 / +0,7 % | zmniejszanie w cenie, cel ±5 % na granicy |

**Poprawki po pierwszym pomiarze.** `p/publish` (`80054897`): pierwszy pomiar dał `World_Publish`
+20…28 % na każdym rungu bez płachet i banerów; profil (`-list publish`) pokazał czas na liniach,
które nic nie robią — stan płachty i banerów w jednej pętli zerowany i rozlewany na stos przy
każdej encji (8 zapisów), a `seenAny` inlinowane do `Publish` dzieliło rejestry z nowymi
lokalnymi i przeładowywało slot stosu na każdy widok; do tego `figure` 80 → 88 B (3–6 % na
gęstych rungach). Poprawka: zwykły chunk idzie ciasną pętlą jak w bazie, `Fixed` i banery przez
`publishStandIns`; `seenAny` `//go:noinline`; zapisy rekordu w kolejności pól (zera łączone);
`progress` inlinowany przez `phaseOf`; wspólny pomocnik rekordu odrzucony (+1,8 ns na rekord).
Wynik: −3…−10 % względem bazy na każdym rungu. `p/photo` (`f0687cd5`, `3e4ed7db`, `91abe569`):
sektor kosztował 10 przebiegów i ~12 draw, prawie każda alokacja to wgpu (~45 na przebieg, ~25
na draw); teraz runda do `perFrame` sektorów to jedno rysowanie instancjonowane do jednej
ciemni (albedo i relief obok siebie), czyszczenie rysowaniem w tym samym przebiegu, kopie i
połówki jednym rysowaniem na obraz (5 przebiegów, ~10 draw na rundę); krata płachty przez bufor
indeksów nad wspólnymi narożnikami. Obrazy co do bajta te same (ekran 1440p wyspy, cztery
płachty w trzech światach); nowy test GPU: dwa sektory jednej rundy stykające się na nieparzystym
tekselu L5 wychodzą jak po jednym. Reszta kosztu kraty w klatce (+1,3–1,9 ms) to raster ścian
kopuł od szczytu do komórki pod gruntem (912 tys. pikseli na 1440p); odrzucenie trójkątów z
zatopionym narożnikiem zeszłoby do ~0,3 ms, ale zmienia frędzel pod szczytem — do decyzji,
w roadmapie.

**Weryfikacja końcowa.** Sekwencja z SIMD zielona po każdym scaleniu (90 ok, 41 bez testów);
25 dem po 5 s; forest-army piksel w piksel z bazą; worktree'y usunięte; gałąź wypchnięta;
jeden commit squash, PR.

**Co zostało** (roadmapa): cień płachty na gruncie na L4; wygląd pod efektem albo animowany na
rodzaju `Fixed` fotografuje sektor na nowo przy każdej klatce animacji; migawki pól nie są
zapisywane; rzecz przez szew zawijanego świata przycięta w zdjęciu; `Miniature` minimapy mogłaby
czytać ćwiartkę albedo; `lay` buduje trzeci zestaw indeksów przy każdej zmianie reliefu;
`AlbedoSize` nieczytany, `albedo()` w `surface.wgsl` mogłoby czytać `U.AlbedoPx`; hover i
komendy wskazane nie znają banerów; Z banera to średnia jednostek; koszt kraty i rundy jak wyżej;
na L4 drogi i rzeki bledsze przez albedo w pół (do decyzji); skok ticku Madery po ~2 min.

## Madera z agregatami: pola z gramapu, suche rozwinięcie, płachty w kaflach, karawany (plan 2026-11-06, gałąź `feat/madeira-aggregates`, litera `d`, po etapie `p`, v0.20.0)

### Kontekst

Etap `b` dał agregaty (`Aggregate`, `Field`, `Unfold`/`Fold`), etap `p` płachty tego, co stoi,
z migawkami części pól zwiniętych. Madera (`examples/board/topography`) nie ma z tego nic: 37
jednostek, 21 mostów, 7 brodów, latarnia; las jest fakturą gruntu. Etap `d` zaludnia ją tak, by
drabina miała co liczyć: lasy i osady jako pola, karawany i wędrowcy jako agregaty punktowe i
jednostki po drogach, a płachty gotowe od pierwszej klatki i odświeżane sektorowo.

### Decyzje użytkownika (2026-11-06, nie do rewizji)

- gramap zostaje ogólny: nie wie, co to las ani miasto. Jeden mechanizm: pole nad plamą komórek
  danego rodzaju w bloku, od progu komórek; które rodzaje, pod jaką nazwą i od ilu komórek, mówi
  receptura (plik gry). Domyślny próg lasu 4 komórki wpisuje receptura Madery, nie kod.
- Osada idzie tą samą drogą: miejsce w recepturze ma wielkość w **komórkach** (słowa
  hamlet/village/town i ich liczby definiuje receptura, np. 3/8/16; `none` dla przełęczy), gramap
  kładzie tyle komórek rodzaju zabudowanego wokół środka wzdłuż dróg, pole powstaje nad nimi jak
  nad lasem. Co stoi na komórce zabudowanej (ile domów, latarnia), to reguły `Unfold` w demie.
  Żadnego liczenia domów w gramapie.
- Ikony karawan i wędrowców, skład karawan i ich trasy są w demie; świat nic nie przybywa.
- Płachta to dwie tekstury RGBA8 (albedo; relief: szczyt f16 w RG, normalna w BA), bryłę robi z
  nich grunt w shaderze. Plik płachty to bajty tych tekstur, cięte na kafle o stałym boku, PNG
  jako pojemnik (bezstratny, dekoder w bibliotece standardowej). Ważne jest tanie odświeżanie
  sektorowe (gra w rodzaju SimCity); polityka należy do gry: płachty tylko czytane (świat
  statyczny) albo czytane i zapisywane kaflami (dynamiczny).
- Jeden tag v0.20.0 po `p`, `d` i `i`; wpisy pod „v0.20.0 — unreleased".

### Fakty sprawdzone (stan `main` `03ef991e` po `p`)

- **Płachta w biegu** (`plugins/world/sheet.go`, `sheet_gpu.go`): `sheets{albedo, relief,
  albedoL5, reliefL5, perUnit, cols, rows, version/painted/signed/now/inked/wanted/snapped []
  per sektor, folded map[place][]shot, room darkroom, perFrame 8}`; `newSheets(cfg, grid)` liczy
  teksel (`Ladder.Texel`, zero → `1/Scales[L4]`, ≤ 4096 na bok); `open()` tworzy obrazy przy
  `Plugin.Sheets()`; publikacja: `recall` → `begin(views)` (sektory `wanted` na L4+, `ring` przy
  szwie) → `stand(box, h, shot)` dla każdej encji `Fixed` (sygnatura `now[i]`) → `settle`
  (sygnatura ≠ `signed` → `version++`; do `perFrame` sektorów z `painted ≠ version` do
  `f.photos`, `mark(i, full)`) → `gather` (instancje z `cands` i `folded`) → `develop` w `Swap`
  przez `img.Defer`. `fold(place, shots)`/`unfold(place)` z `wholes` podbijają wersję. Płachta
  nie jest zapisywana; migawki pól nie są zapisywane (roadmapa: po wczytaniu sektor z polem
  nigdy nierozwiniętym nie ma migawek, więc odświeżenie go gubi las).
- **Migawki** powstają tylko w `wholes.fold` (`snapping`/`snap`: `shot` z pudła części, `Z`,
  sprite'a L3 przez `levels.at`, `photographable`, `photographed`). Pole nigdy nierozwinięte nie
  ma migawek: dziś w płachcie nie ma go wcale.
- **Rozwinięcie**: `wholes.unfold` → `find` (stretch pola na sektor: `Field.Size` wokół środka,
  przecięcie z `Grid.Box(i)`) → `open` (encja `stretch{Whole, Sector, Box, Folded}`, zapisywana;
  `Unfold{ID, At, Sector, Box, stretch}` do reguł przez `RunWhere`) → reguły dają
  `SpawnPart{Entry, At}` celowane w stretch (`Aim`) → `spawnSystem.part` kładzie pudło
  `placing.laid(k.Position.Resolve(row))` od środka `st.Box` i spawnuje w `take` (grupami,
  `PartOf`). `ikinds.Kind` wyjmuje ze Spec tylko `Position` i `Velocity`; `Z` części daje jej
  Spec (`comp.Const(world.Z{Height})`), a `Altitude` pisze topografia: `altitudeSystem`
  (`relief/altitude.go`) dla ruchomych (`motion.Mover`) i stojących (`cell.On`); część z
  `SpawnPart` bez `cell.On` nie dostaje wysokości (roadmapa „a part standing on the board").
  W forest-army drzewa mają `Altitude` 0 na płaskiej łące, więc nie było widać.
- **Świat nie zna gruntu**: `Write.SetLook(look)` jest jedynym wejściem widoku; nie ma
  `GroundAt`. Topografia ma `Relief.GroundAt(at)`/`Altitude(c)`.
- **gramap** (`cmd/gramap`): receptura `recipe{Area, Zoom, Kinds, Ground, Rivers, Roads, Flat}`;
  `Kinds{Sea, Earth, Sand, Rock, Forest, Brook, Stream, River, Road, Bridge, Ford}`; las to
  komórki wybrane szumem (`soil`: `fbm` w plamach 1700 m, `Forest.Share` 0.45 między 300 a 1400
  m); `Roads.Places map[string][2]float64` (17 miejsc, w tym przełęcze Encumeada i Poiso),
  `Routes`; `roads.nearest(lonlat)` daje komórkę miejsca; `write.go`: `legend` stała (`~ . : ^
  *`), `writeBoard` pisze `entities:` (mosty `from/to`, brody `at/way`), `text()` dokleja
  `units:` ze starej mapy (`unitsOf`, `shiftUnits`); `check.go` kładzie mapę na planszy bez GPU
  i bez rodzajów gry (definiuje rodzaje komórek z receptury, mosty, brody) i tyka raz: **gramap
  nie ma sprite'ów gry, więc płacht nie sfotografuje**; `main.go` drukuje miejsca `[x, y]`.
  Mapa Madery: 640×320 komórek po 100 m, jednostka 3,125 m, sektor 16 komórek (Buckets 2), 800
  sektorów; `map.yaml` 1299 linii, `units:` 37 wierszy z komentarzami tras.
- **Plansza**: `Level` (`level.go`) czyta `board.entities` i `units` jako `yaml.Node`, każdy
  wiersz `kind:` plus pola wiersza rodzaju; `board.Spawn[R](level, sides)` spawnuje wiersze
  rodzajów o wierszu `R`; `cell.ID` w yaml to `[x, y]`; wiersz niewzięty przez żaden `Spawn`
  wywala `Populate`. `NewEntities` daje `Position` z komórki, `cell.On`, `Z{Height}`,
  `StandingTemplate` (`world.Lie{}`, `Fixed{}`, `light.Shadow{Baked}`).
- **forest-army**: las `Field{Size: 1500×990}` jedną encją `{kind: forest, at}`; 144 działki na
  sektor na kracie 42 jednostek (`plotAt`, `plot(k, density)` słowem nad `Unfold` z
  `lod.Seed`), drzewo `treePx 20` z `Collider`; armia agregat punktowy `NewUnits` z
  `MoveOrder`, `navigation.Inherit`, `At(L4, banner)` (dysk z obwódką); `MaxEntCount 1024`.
- **Madera**: `MaxEntCount 512`, `Entities{MinSize 2, MaxSize 32}`, `LOD.Scales{0,0,0.5,0.25,
  0.12,0}`, `Sight 30 km`, `Fog 6 km`; rodzaje `unit, rival, plateau` (`walker`, `NewUnits`,
  `Upright`, `Mark`), `hawk`, `ford`, `lantern` (`NewEntities`, 8 px), `bridge`; `tiles.go`
  definiuje komórki (`ground{name, kind}` z zimową odmianą), `scene.go` style malarza na
  rodzaj komórki; `textures.go` fotografie gruntu. Benche: `World_Tick`, `World_Sheet`,
  `Frame_Topography`; brak bencha ticku Madery (sonda `gram-probe`).
- `render.Image`: `SubImage`, `WritePixels`, `ReadPixels`, `DrawImage`, `CopyFrom`, `Defer`;
  PNG w gramie: `internal/engine/shot.go`, `cmd/gramap/tiles.go`, `textures.go`. **Pułapka**:
  `png.Encode` z `*image.RGBA` dzieli przez alfę (RGBA jest premultiplied) i psuje bajty reliefu
  (A to składowa normalnej); bajty tekstury idą przez `*image.NRGBA` w obie strony, wprost.
- Gałąź `feat/madeira-aggregates` od `main` `03ef991e`; worktree'ów brak; `d/*` wolne.

### Projekt (co buduje etap)

**Suche rozwinięcie** (świat): sektor, który płachta chce sfotografować (`wanted`, `painted ≠
version`), a który pokrywa stretch pola bez migawek (`folded[place]` brak) i nierozwinięty
(brak encji `stretch`), dostaje najpierw **podgląd**: świat mówi regułom zwykłe `Unfold` tego
stretchu (reguły nie odróżniają podglądu od rozwinięcia: ten sam `Seed`, te same słowa, ten sam
wynik), a `SpawnPart` wydane pod nim **nie spawnuje** części, tylko zapisuje jej `shot`
(pudło `placing.laid(k.Position.Resolve(row))`, `Z.Height` ze Spec rodzaju, `Altitude` z gruntu
świata w środku pudła, sprite L3 przez `levels.at` i `photographable`) do `sheets.folded[place]`;
po spłynięciu wszystkich `SpawnPart` kroku `sheets.fold(place, shots)` podbija wersję i sektor
fotografuje się z lasem. Podgląd nie jest stretchem: nie czeka na `Fold`, nie kładzie
`Unfolded`, nie trafia do zapisu. Sektor czekający na podgląd nie fotografuje się bez niego
(flaga `pending` w `settle`). Budżet: `perFrame` podglądów na krok. To zamyka też punkt roadmapy
o migawkach po wczytaniu: brakujące odtwarzają się same. Grunt świat dostaje od topografii:
`Write.SetGround(func(at geom.Vec) float32)` (wzór `SetLook`), zero bez niego (świat płaski).
Części prawdziwe dostają `Altitude` tak samo: topografia ugruntowuje raz encję `Fixed` z `Z`
bez `cell.On` w środku jej pudła (`altitudeSystem`, trzecie zapytanie).

**Płachty w kaflach** (świat, `Config.Sheets world.SheetsCfg{Dir string; Keep bool; Version
string}`): `Dir` to katalog kafli (zero: żadnych plików); kafel to kwadrat `sheetTile` 512
tekseli obrazów L4, dwa pliki PNG na kafel (`albedo-<tx>-<ty>.png`, `relief-<tx>-<ty>.png`,
bajty tekstury przez `image.NRGBA`), i manifest `sheets.yaml` (`version` gry, teksel na
jednostkę, `cols, rows`, bok kafla, rozmiar świata, i dla każdego sektora namalowanego jego
sygnatura `signed[i]` `{n, mix}` oraz czy trzyma coś, `inked`). Odczyt przy `Sheets()`: manifest
niezgodny (inna `Version`, inny teksel lub rozmiar) → katalog ignorowany; zgodny → kafle
wgrywane do obrazów L4 (`SubImage(rect).WritePixels` odroczone `Defer`, jak fotografie), połówki
L5 z nich (`DrawImage` 0.5 na kafel), sektory z manifestu `painted = version`, `signed`
z manifestu, `inked` z manifestu; pierwsza publikacja liczy `now[i]` i podbija wersję tylko tam,
gdzie treść `Fixed` się zmieniła. Migawki pól nie wchodzą do sygnatury (deterministyczne z
`Seed`); zmiana reguł sadzenia to zmiana `Version` w grze (udokumentować). Zapis (`Keep`):
kafel, którego sektor sfotografowano, jest brudny; co najwyżej jeden brudny kafel na sekundę,
nie wcześniej niż 2 s po ostatniej fotografii w nim, czytany `ReadPixels` prostokątem i kodowany
w goroutine, zapisany atomowo (plik tymczasowy, `rename`), manifest po nim; `Keep` fałsz: nic
nie pisze (świat statyczny żyje w pamięci). Pierwszy bieg gry z `Keep` pisze cały katalog;
drugi wczytuje go i fotografuje tylko zmiany. Zapis gry (F5) nie dotyka płacht. Demo Madery:
`Dir` w katalogu podręcznym użytkownika (`os.UserCacheDir()/gram/topography/sheets`), `Keep`
prawda, `Version` stałą w `names.go`.

**gramap ogólny**: receptura dostaje
```yaml
fields:                      # a field over the cells of a kind in each block of the board
  - {cells: forest, kind: forest, grain: 16, least: 4}
  - {cells: built, kind: settlement, grain: 16, least: 1}
settlements:
  cells: built               # the cell kind laid round a place
  sizes: {hamlet: 3, village: 8, town: 16}   # cells
  default: village
kinds: {…, built: built}
roads:
  places:
    Funchal: {at: [-16.9086, 32.6496], size: town}
    Encumeada: {at: [-17.019, 32.755], size: none}
    Seixal: [-17.102, 32.824]        # a bare pair stays: the default size
```
Blok to kwadrat `grain` komórek liczony od komórki (0, 0) mapy (z morzem), więc przy `grain` =
bok sektora w komórkach (Madera 16) bloki pokrywają się z sektorami świata; pole nad komórkami
rodzaju `cells` w bloku, gdy jest ich co najmniej `least`, pisane do `board.entities` jako
`{kind: <kind>, from: [x0, y0], to: [x1, y1], cells: n}` (prostokąt opisany na komórkach,
włącznie; `n` komórek). Komórki zabudowane: po drogach, przed zapisem: od komórki miejsca BFS po
komórkach lądu niebędących drogą, wodą, skałą ani mostem, najpierw sąsiadujące z drogą, potem
po odległości, aż `size` komórek (0 dla `none`); legenda dostaje znak `"@"` dla rodzaju
`settlements.cells`; `check.go` definiuje ten rodzaj jak glebę. Receptura bez `fields` i
`settlements` daje mapę jak dziś (test). `madeira.yaml` dostaje oba i rangi: town Funchal,
Machico, Câmara de Lobos, Ribeira Brava, Santa Cruz, Caniço; hamlet Prazeres, Seixal, Ponta
Delgada, Faial, Ponta do Sol; none Encumeada, Poiso; reszta village (domyślna). Mapa Madery
odnawia się gramapem (`units:` zostaje), a demo dostaje wiersze pól.

**Demo Madery** (`examples/board/topography`): rodzaje
- komórka `built` (`BuiltCell`, ląd, krok 1, kolor kamienny, styl malarza jak skała z
  domieszką ziemi; odmiana zimowa jak reszta);
- `forest` pole: wiersz `fieldRow{From, To cell.ID; Cells int}` → `Position` i `Field{Size}` z
  prostokąta (`comp.Load`), `Aggregate{}`, rola `ForestRole`: reguły `Unfold` sadzą drzewa:
  działka na pół komórki (dwie na komórkę, krata 16 jednostek w komórce 32, potrząśnięta z
  `Seed`), słowo działki: środek działki leży w `Box` stretchu, komórka pod nim jest `forest`
  (słowo czyta planszę) i `lod.Seed(whole, k) % 4 < 3`; bez `Thinned` (Madera nie ścina); `Look`
  bez `At(L4)` ani `At(L5)`: na obu las jest płachtą z migawek;
- `tree` część: `Fixed{}` (szablon `StandingTemplate` bez `cell.On`: `Position` stała jak w
  forest-army, `Z{Height: scale.Figure(12)}`), bez `Collider` i bez zajętości (komórki leśne już
  spowalniają przez mobilność; decyzja do potwierdzenia), rola `TreeRole` z `Folding` → `Despawn`;
  korona 20 px jak w forest-army (kod skopiowany: dema są samowystarczalne);
- `settlement` pole: `fieldRow` jak las, rola `SettlementRole`: `Unfold` stawia dom na każdej
  komórce `built` w `Box` (jedna działka na komórkę, lekko potrząśnięta z `Seed`); w osadzie o
  co najmniej 16 komórkach (słowo liczy komórki `built` w `Box` na planszy) jedna latarnia w
  komórce najbliższej środka (część z `light.Source` jak dzisiejsza latarnia, `Fixed`);
- `house` część: `Fixed{}`, `Z{Height: scale.Figure(6)}`, rysowany w kodzie z góry (prostokąt
  dachu w dwóch odcieniach z kalenicą, obrys ścian), 14 px; `Folding` → `Despawn`;
- `caravan` agregat punktowy jak armia (`NewUnits`, `Mover{Land}`, `Aggregate{}`, `MoveOrder` z
  wiersza, bez kolizji), plan: po dojściu zamienia start z celem (kursuje); `Unfold` ustawia wóz
  i trzech tragarzy w rzędzie wzdłuż kierunku (`Inherit` trasy jak armia), `Fold` zbiera; `Look`:
  `At(L4, caravanIcon)`;
- `wagon`, `porter` części: `Mover{Land}`, tragarz jak `walker` bez `Sight`, wóz 12×8 px (brązowy
  prostokąt, dwa koła), `Folding` → `Despawn`;
- `traveller` jednostka: walker z `MoveOrder` z wiersza i planem kursowania jak karawana,
  pojedynczy, `At(L4, travellerIcon)`.
Ikony 24 px, w kodzie: dysk promienia 11 w kolorze (245, 240, 225, α 0,7) z obwódką 1 px
(40, 35, 30); sylwetka: głowa (koło r 2,2) i tułów (zaokrąglony trapez 5,5 szer., 9 wys.) w
kolorze właściciela z obwódką 1 px; wędrowiec jedna sylwetka pośrodku; karawana trzy w rzędzie
(x 6, 12, 18), środkowa o 2 px wyżej i ×1,2, prawa z garbem juku (ciemnobrązowe koło r 2 przy
ramieniu). Liczby powyżej trzech zostawia się licznikowi na szkle (`Counts`). W `map.yaml`
`units:` dostaje 6 karawan (`caravan`, strona `reserve`: Funchal↔Machico, Funchal↔Ribeira
Brava, Calheta↔Porto Moniz, Porto Moniz↔Santana, Santana↔Machico, Ribeira Brava↔São Vicente)
i 4 wędrowców (Funchal↔Poiso, Câmara de Lobos↔Ponta do Sol, Seixal↔São Vicente,
Faial↔Santa Cruz), współrzędne z wydruku gramapu. `MaxEntCount` 512 → 40 000 (drzewa
rozwinięte przy kamerze: ~230 na sektor leśny, kilkadziesiąt sektorów na L2–L3), `MaxSize`
zostaje 32; `Config.Far: Banners`, `Counts: true` (F10 przełącza); `Config.Sheets` jak wyżej;
`Fold` 3 s.

### Model wykonania

Jak w etapie `p` (`CLAUDE.md` „A stage in parts"): integrator planuje, briefuje, scala, mierzy
(≤ 10 min), pisze dokumenty; agenci Opus 5.5 (`Agent`, `model: opus`): preludium w klonie na
`feat/madeira-aggregates`, części w worktree'ach `/home/karol/worktrees/gram/<część>` na
`d/<część>` od commitu preludium. Wspólne reguły agenta jak w `p`: skille warstw; tylko pliki
części; `go build ./... && go vet ./... && gofmt -l . && go test ./...` z `GOEXPERIMENT=simd`
przed każdym commitem; prefiksy konwencjonalne i trailer `Claude-Session`, bez
`Co-Authored-By`; żadnych zmian w `doc/`, `.claude/`, `BENCHMARKS.md`, `CHANGELOG.md`, `go.mod`,
README; godoc ≤ 2 zdania; linie ≤ 100 kolumn; nic nie mierzyć; dema tylko na kopii
(`gram-probe`); nic nie wypychać; raport końcowy.

### Preludium (agent A, `feat/madeira-aggregates`): kontrakty

Pliki: `plugins/world/{config.go, rw.go, module.go, doc.go, internal/kinds/kinds.go}`,
`plugins/topography/{plugin.go, internal/relief/altitude.go}`, testy.

1. `world.SheetsCfg{Dir, Keep, Version}` i `Config.Sheets` z godoc (bez zachowania; `newSheets`
   tylko przechowuje; gra czyta je z `Resources`, jak resztę `Config`).
2. `Write.SetGround(func(at geom.Vec) float32)`, `Read.GroundAt(at) float32` (zero bez gruntu);
   topografia woła `SetGround(p.relief.GroundAt)` tam, gdzie woła `SetLook`. Test: świat bez
   gruntu daje 0; z ustawionym daje jego wartość.
3. `ikinds.Kind` wyjmuje ze Spec także `Z` (`comp.Template[entity.Z]`, opcjonalne, `HasZ`),
   jak `Position`; test: rodzaj z `Z{Height}` daje je, bez — zero.
4. Topografia ugruntowuje encję `Fixed` z `Z` bez `cell.On` (trzecie zapytanie `altitudeSystem`:
   `Base, Z, world.Fixed`, `Exclude[cell.On]`, `Exclude[motion.Mover]`), raz: `Altitude =
   GroundAt(środek)` gdy różni się o więcej niż `f32.Eps`; test w `relief`: encja `Fixed` położona
   w `Position` dostaje wysokość gruntu, jednostka z `Mover` jak dziś.
5. Sekwencja zielona; commit `feat: the world's ground and sheet files declared, a kind's Z,
   fixed parts grounded` (2–3 commity tematyczne dopuszczalne).

### Część 1 `d/preview` (agent B, po preludium): suche rozwinięcie

Pliki: `plugins/world/{wholes.go, spawn_system.go, preview.go (nowy), sheet.go (flaga
`pending` i wejście `fold` z podglądu, minimalnie), moments.go (godoc Unfold: „także jako
podgląd"), doc.go (akapit w # Aggregates)}`, testy, `bench/world_test.go`. Nie dotyka:
`sheet_gpu.go`, `renderer.go`, `publisher.go`, topografii, gramapu, dem.

1. `sheets` zgłasza sektory do podglądu: w `settle` sektor `wanted`, `painted ≠ version`, bez
   `snapped[i]`, którego pokrywa pole bez migawki i bez stretchu → `pending[i] = true`, bez
   fotografii w tej publikacji; `wholes` czyta `pending` w `unfold` (gdzie i tak idzie po
   polach w `find`) i dla każdego pola pokrywającego taki sektor, bez `folded[place]` i bez
   `holding` tam, otwiera **podgląd**: `Unfold{ID, At, Sector, Box, stretch: id podglądu}` do
   reguł jak w `open` (ta sama ścieżka `RunWhere`), bez encji `stretch`, bez `Unfolded`; podgląd
   żyje w `wholes.previews` (mapa id podglądu → `place`), id z puli świata (`uid` tymczasowy,
   nigdy zapisywany: podgląd trwa jeden krok). Budżet `perFrame` podglądów na krok.
2. `spawnSystem.part`: `cmd.in` jest podglądem → zamiast `take`: `shot` części (pudło
   `placing.laid(k.Position.Resolve(row))` od środka `Box`; `Z` ze Spec (`HasZ`), `Altitude =
   Read.GroundAt(środek)` gdy świat ma wysokości; sprite L3 rodzaju przez atlas, `photographable`;
   `Lie` ze Spec, jeśli jest) do `previews[id].shots`; po `Drain` wszystkich `SpawnPart` kroku:
   `sheets.fold(place, shots)` dla każdego podglądu (pusty podgląd też zapisuje pustą migawkę,
   `snapped[i]` prawda, by sektor nie prosił w kółko), `pending[i] = false`, podgląd usunięty.
   Kolejność w kroku: `wholes.Update` (Unfold) → reguły → `spawnSystem.Update` (Drain): zbadać
   w `module.go`, czy `SpawnPart` z reguł kroku spływa w tym samym kroku; jeśli w następnym,
   podgląd żyje dwa kroki (udokumentować w godoc `previews`).
3. Rozwinięcie prawdziwe stretchu z migawką z podglądu: `open` woła `sheet.unfold(place)` jak
   dziś (migawka znika, części żywe); `fold` kładzie migawkę z części jak dziś.
4. Testy (`plugins/world`, rig z `field_internal_test`/`sectors_internal_test`): pole bez L4 w
   sektorze widzianym na L4, nigdy nierozwiniętym: po dwóch publikacjach `folded[place]` ma tyle
   migawek, ile reguły dałyby części (ten sam `Seed`: rozwinięcie prawdziwe daje tyle samo
   części), żadnej encji części nie ma, `Unfolded` nie ma, zapis nie zawiera podglądu;
   sektor fotografuje się raz po podglądzie (`SheetShots`), nie przed; pole z `At(L4)` nie
   dostaje podglądu; po `Unfold` prawdziwym migawka z podglądu znika; po `Fold` wraca z części;
   wczytanie zapisu z polem zwiniętym: podgląd odtwarza migawkę; `Altitude` migawki równa
   `GroundAt` (świat z `SetGround` stałym).
5. Bench `Benchmark_World_Preview` (`bench/world_test.go`): sektor z polem o 256 częściach,
   podgląd + publikacja, ns i alokacje; cel w „Pomiar".

### Część 2 `d/files` (agent C, po preludium, równolegle z 1 i 3): płachty w kaflach

Pliki: `plugins/world/{sheet_files.go (nowy), sheet.go (wejścia: `loaded(i, signed, inked)`,
`dirty(i)`, minimalnie), sheet_gpu.go (wgranie kafli i połówki L5 odroczone), doc.go (akapit w
# Sheets)}`, testy GPU (`gputest.Need`). Nie dotyka: `wholes.go`, `spawn_system.go`,
`publisher.go`, gramapu, dem.

1. Format jak w „Projekt": `sheetTile 512`; `sheets.yaml` (`go.yaml.in/yaml/v3`, jak gramap i
   plansza): `version`, `per_unit`, `cols`, `rows`, `tile`, `space: [w, h]`, `sectors: [{i, n,
   mix, inked}]` tylko namalowane. PNG przez `image.NRGBA` nad bajtami `ReadPixels`/do
   `WritePixels` (pułapka premultiplied w „Fakty").
2. Odczyt w `open()` (przy `Sheets()`, gdy `Dir ≠ ""`): manifest zgodny → `Defer` wgrania kafli
   do `albedo`/`relief` i połówek L5 (`DrawImage` 0.5 na kafel, jak `develop`), `painted[i] =
   version[i]`, `signed[i]`, `inked[i]`/`shots` z manifestu; niezgodny lub brak → nic, log jedną
   linią (`[world] sheets: …`). Kafel uszkodzony (błąd PNG) → jego sektory jak nienamalowane.
3. Zapis (`Keep`): `develop` oznacza kafle sektorów sfotografowanych brudnymi z czasem; w
   `Swap`/`develop` kolejnej klatki: najstarszy brudny kafel bez fotografii od 2 s, najwyżej
   jeden na sekundę: `ReadPixels` obu obrazów prostokątem kafla (odroczone `Defer`, w klatce),
   kodowanie i zapis w goroutine (plik `.tmp` + `os.Rename`), manifest przepisany po każdym
   kaflu (mały); błąd zapisu logowany raz. `Keep` fałsz: nic.
4. Testy: zapis i odczyt kafla zachowuje bajty co do bajta (albedo z alfą < 255, relief z
   normalną o A < 255); manifest niezgodny ignorowany; po wczytaniu sektor z manifestu jest
   `Painted` i nie fotografuje się, gdy `now == signed`, fotografuje się, gdy treść `Fixed`
   inna; `Keep` fałsz nie tworzy plików; brudny kafel pisany po 2 s, nie wcześniej; świat bez
   `Dir` jak dziś co do bajta (`World_Sheet` bez zmian).
5. Bench `Benchmark_World_SheetFiles` („load,tile": wgranie kafla; „save,tile": odczyt i
   kodowanie kafla) w `bench/world_test.go`.

### Część 3 `d/gramap` (agent D, po preludium, równolegle z 1 i 2): pola i osady z receptury

Pliki: `cmd/gramap/{recipe.go, build.go, fields.go (nowy), settle.go (nowy), write.go,
check.go, main.go, gramap_test.go, madeira.yaml}`, `examples/board/topography/map.yaml`
(odnowiona gramapem, `units:` zachowane). Nie dotyka: pluginów, kodu dem.

1. Receptura jak w „Projekt": `fields []fieldCfg{Cells, Kind, Grain, Least}`, `settlements
   {Cells, Sizes map[string]int, Default string}`, `Kinds.Built`, miejsce jako para albo
   `{at, size}` (własny `UnmarshalYAML`; `size` słowo z `Sizes`, `none`, albo liczba);
   `-flat` bez zmian. Walidacja: `cells` i `kind` niepuste, `grain ≥ 1`, `least ≥ 1`, słowo
   `size` znane.
2. Komórki zabudowane (`settle.go`) po drogach: BFS jak w „Projekt"; miejsce bez komórki
   (`nearest` nie znalazło) pomijane z ostrzeżeniem jak `skipped`. Pola (`fields.go`): bloki
   `grain` od (0, 0) mapy; prostokąt opisany; wiersz `{kind, from, to, cells}`, sortowane po
   `from` jak `standing`. `writeBoard`: legenda `"@"` dla `settlements.cells`, wiersze pól po
   stojących. `check.go`: rodzaj zabudowany jak gleba. `main.go` drukuje liczbę pól na rodzaj i
   komórek zabudowanych na miejsce.
3. Testy: receptura bez `fields`/`settlements` daje tekst mapy jak dotąd (test bajt w bajt na
   małej recepturze z `gramap_test`); blok z `least−1` komórkami nie daje pola, z `least` daje
   prostokąt opisany; komórki zabudowane: `size` komórek, żadna na wodzie, drodze, skale ani
   moście, każda z pierwszych przy drodze; `none` nic; `check` przyjmuje mapę z polami (wiersze
   nieznanych rodzajów są dla planszy tylko czekającymi: sprawdzić, że `check` nie wywala
   `Populate`; jeśli wywala, `check` spawnuje je atrapą rodzaju pola).
4. `madeira.yaml` z `fields`, `settlements`, `built` i rangami jak w „Projekt"; mapa odnowiona
   (`go run ./cmd/gramap -recipe cmd/gramap/madeira.yaml -out examples/board/topography/map.yaml`;
   kafle terenu w `~/.cache/gramap/terrarium` są; bez sieci agent zgłasza i zostawia mapę
   integratorowi); flat (`examples/board/flat/map.yaml`) **nie** odnawiana (płaska plansza
   nie czyta pól w tym etapie; jej mapa bez `fields` zostaje jak jest: sprawdzić, że gramap z
   `-flat` i tą samą recepturą dałby pola — więc flat zostaje przy starym pliku, odnotować).

### Część 4 `d/madeira` (agent E, po scaleniu 1–3): demo Madery

Pliki: `examples/board/topography/{demo.go, names.go, scene.go, tiles.go, forest.go, tree.go,
settlement.go, house.go, caravan.go, traveller.go, icons.go (nowe), walker.go (tragarz
współdzieli profil), map.yaml (tylko `units:`)}`. Nie dotyka: pluginów, gramapu, innych dem.

1. Rodzaje, pola, części, reguły, ikony i konfiguracja jak w „Projekt"; `BuiltCell` w
   `tiles.go` i `scene.go` (styl, kolor, odmiana zimowa); `Config.Sheets`, `Far`, `Counts`,
   `MaxEntCount`, `Fold`. Nazwy w `names.go` z sufiksami (`ForestKind`, `TreeKind`,
   `SettlementKind`, `HouseKind`, `CaravanKind`, `WagonKind`, `PorterKind`, `TravellerKind`,
   `ForestRole`, `TreeRole`, `SettlementRole`, `HouseRole`, `CaravanRole`, `CaravanPlan`,
   `TravellerPlan`, `SheetsVersion`).
2. `spawn`: `board.Spawn[fieldRow]` dla lasów i osad, `board.Spawn[caravanRow]` i
   `board.Spawn[travellerRow]` (oba `{Start, Target}`, kursujące planem).
3. Testy: dema nie są testowane; sprawdzenia deterministyczne sondą na kopii (`gram-probe`):
   po 10 s na L4 (kamera startowa): liczba encji po poziomach (`Read.Sectors()`/telemetria),
   `SheetShots()` stabilizuje się do 0; zoom do L2 nad lasem: drzewa spawnują się tylko na
   komórkach `forest`, dom tylko na `built`, `Altitude` każdego drzewa = `GroundAt` ±`Eps`;
   karawana rozwinięta: 4 części idą tą samą trasą; zwinięta: jeden rekord ikony; F10 Points:
   punkty; Counts: liczby. Zrzuty `GRAM_SHOT`: L4 nad Funchal (osada i las w płachcie), L2 nad
   lasem, L2 nad Funchal, karawana na drodze w obu trybach; drugi bieg z kaflami w katalogu:
   pierwsza klatka ma płachtę pełną (zrzut klatki 1 przeciw klatce 120).
4. Sonda tempa (komendy dla integratora): `GRAM_FPS_LOG=1 GRAM_VSYNC=off` na kopii, 35 s po
   rozgrzewce, kamera startowa (L4) i zoom L2 nad lasem, baza (`main` po `p`, stara mapa)
   przeciw drzewu: tick, drawing, encje.

### Kolejność, szwy, scalanie

Preludium → (1 ∥ 2 ∥ 3) → 4. Szwy: `sheet.go` (1: `pending`/`fold` z podglądu; 2: `loaded`/
`dirty`; integrator scala, nowy kod w `preview.go` i `sheet_files.go`), `Read.GroundAt` (prelude
↔ 1), `SheetsCfg` (prelude ↔ 2 ↔ 4), wiersz pola `{kind, from, to, cells}` (3 ↔ 4), rodzaj
komórki zabudowanej `built` (3 ↔ 4, nazwa w recepturze i `names.go`). Integrator po każdym
scaleniu: sekwencja, dema 5 s, push `feat/madeira-aggregates`, worktree usunięty.

### Pomiar (integrator, ≤ 10 min, baza `main` `03ef991e` na zmianę z drzewem, `measure-nogate.sh`)

| Bench / sonda | Cel |
|---|---|
| `World_Publish`, `World_Draw`, `World_Sheet`, `Sectors_Expanse` | ±2 % (ścieżki nietknięte) |
| `World_Preview` (nowy) | ≤ 0,3 ms na sektor z 256 częściami, alokacje tylko migawek |
| `World_SheetFiles` (nowy) | load,tile ≤ 2 ms; save,tile ≤ 10 ms (PNG w goroutine) |
| `Frame_Topography` | ±2 % (grunt bez zmian) |
| Madera, sonda L4 (start) | tick ≤ baza + 15 % (~1000 pól, 10 agregatów) |
| Madera, `SheetShots` na L4 | 0 po ≤ 150 klatkach bez kafli, ≤ 10 z kaflami |
| Madera, sonda L2 nad lasem | tick ≤ 4 ms z ~5 000 drzew; rozwinięcie sektora ≤ 1 ms |
| Katalog kafli Madery | ≤ 20 MB; pełne pokrycie zapisane w ≤ 2 min gry |

### Dokumenty (integrator)

`CLAUDE.md` (reguła LOD: suche rozwinięcie, płachty w kaflach z polityką gry; tabela dem:
opis topography o lasach, osadach, karawanach; gramap: `fields`, `settlements`),
`plugins/world/doc.go` (# Aggregates: podgląd; # Sheets: kafle), `cmd/gramap` godoc/README
akapit, `CHANGELOG.md` (v0.20.0 unreleased: Added `SheetsCfg`, `SetGround`/`GroundAt`, podgląd,
`fields`/`settlements` gramapu, demo Madery; Changed `Kind` z `Z`, topografia ugruntowuje części;
Fixed roadmapy „a part standing on the board" i „migawki po wczytaniu"), `BENCHMARKS.md`
(`World_Preview`, `World_SheetFiles`, sonda Madery), skille `gram-plugins` (world, topography,
board/gramap), `gram-perf-picture` (kafle), `gram-perf-tick` (podgląd), `gram-game` (demo),
`doc/roadmap.md` (zdjąć dwa punkty, dodać: kolizja drzew, latarnie osad w płachcie bez światła,
flat bez pól, zmiana reguł sadzenia a `Version`), `doc/plans-perf.md` (wynik). Pamięć
`lod-plan.md`.

### Weryfikacja końcowa

Sekwencja z SIMD; wszystkie dema 5 s; topography (start L4: lasy i osady w płachcie od razu w
drugim biegu; zoom L2: drzewa i domy, karawana w częściach; F10 banery/punkty/liczby; zapis i
odczyt F5/F9 z lasem zwiniętym), forest-army piksel w piksel z bazą (bez `Dir`, drzewa z
`Collider` jak były), flat bez zmian; worktree'y usunięte; gałąź wypchnięta; squash, PR.

### Co nie wchodzi

Płachty na płaskiej planszy z plików (flat nie czyta pól), pola na heksach, zapis kafli przy
F5, cień płachty, karawany handlujące (żadnej ekonomii: kursują), kolizja drzew na Maderze (do
decyzji), latarnie osad jako światło w płachcie (płachta nie niesie źródeł światła), minimapa
(etap `i`).

### Wynik etapu `d` (2026-11-06, `feat/madeira-aggregates`, baza `main` `03ef991e` po `p`)

**Wykonanie.** Preludium (agent A, w klonie): `237620c8` (`SheetsCfg`, `SetGround`/`GroundAt`,
`Kind.Z`, części `Fixed`+`PartOf` ugruntowane przez topografię). Części w worktree'ach od
`237620c8`, równolegle: D `d/gramap` (`b697bf5f`, `e1f7dc5a`; scalenie `1d6899c0`), B `d/preview`
(`829c325a`; scalenie `04e1261c`), C `d/files` (`3d28aa71`; scalenie `b7267a80`); potem od
`b7267a80` E `d/madeira` (`2c63dba9`; scalenie `bef40b8f`). Każde scalenie: sekwencja zielona
(90 ok), 25 dem po 6 s (kod 124, bez paniki; topografia startuje na odnowionej mapie dopiero po
części 4, wcześniej nie znała rodzajów pól), push.

**Odstępstwa przyjęte.** Preludium: trzecie zapytanie przebiegu wysokości wymaga `PartOf` obok
`Fixed`, bo bez niego filary mostów (Altitude = dno) byłyby ugruntowane na nowo (test: 2,25
wobec 1). Część 1: podgląd dostaje id od 0xFFFF0000 w dół, żyje dwa kroki, `Unfold` idzie tym
samym RunWhere co prawdziwe; reguły nie odróżniają podglądu, więc pod `Unfold` tylko `SpawnPart`
(zasada dla gier). Część 2: `Swap` planuje wywołanie tylko przy zdjęciach, więc brudne kafle same
ponawiają `Defer` (obejście; czystsza dźwignia to warunek w `Swap`); `Benchmark_World_SheetFiles`
w `plugins/world` (potrzebuje magazynu), poza `measure.sh`; PNG przez `image.NRGBA` (pułapka
premultiplied). Część 3: miejsce drogi jako para albo `{at, size}` (własny `UnmarshalYAML`),
legenda `@`, `check.go` definiuje `built` jak glebę i zastępcze rodzaje pól; gramap zachowuje
tylko `units:`, więc ręczny wiersz latarni z `entities:` przepadł przy regeneracji; pól 260
(239 lasów, 21 osad), nie ~1000 z szacunku planu (ziarno 16 i próg 4 łączą las w bloki); miasta
rosną pasami wzdłuż dróg. Część 4: `Buckets` 8, nie 2 (przy `MaxEntCount` 40 000 kubełek
przestrzeni spada do 64 jednostek, sektor 16 komórek wymaga 8; indeks drobniejszy); kamera
startuje na L4 (`Zoom` 0,15), wóz 7,2×4,8 (większy nie mieścił się w pasie mostu ukośnego i szedł
inną trasą niż tragarze); cztery trasy zamienione na takie, które mają drogę; `LampKind`,
`LampRole`, `CarrierRole`; karawany gracza, ale bez `Allow` (zaznaczenie wymagałoby `Forbid` pod
`Unfold`); latarnia tylko w osadzie o ≥ 16 komórkach `built` w jednym polu (Ribeira Brava i
Caniço; Funchal, Machico, Santa Cruz pocięte ziarnem na mniejsze pola).

**Sprawdzenia deterministyczne (agent E, sonda na kopii, tempo ×4).** Start na L4: 736 sektorów
L4, 450 encji, `Read.Painted` rośnie o 24 na sekundę gry i staje na 173 po 8 s. L2 nad lasem:
24 519 drzew, każde na komórce leśnej, na gruncie (odchyłka 0). L2 nad Funchal i Ribeira Brava:
135–143 domy, każdy na `built`; latarnie 2. Karawana rozwinięta: 4 części, jeden cel, jedna
trasa przez most; po powrocie na L4 części znikają, `Banners()` daje każdą karawanę i wędrowca
banerem (8 w kadrze), po F10 Points 0 banerów, liczby na szkle. Kafle: pierwszy bieg (100 s)
zapisał 8 kafli widocznej części wyspy, 944 KB; drugi bieg ma pełną płachtę od pierwszej
narysowanej klatki (klatki 5 i 10 różnią się od 120 o 40 px, bez kafli o 27–31 tys.). Na oko
(zrzuty): las na L4 ciemnymi blokami z kropkami drzew w płachcie, na L2 gęste korony; domy to
bryły z dachem (ciemne skrzynki w południe); karawana wozem i trzema tragarzami przez most;
banery na L4 to piksel–dwa, liczby czytelne.

**Pomiar** (baza na zmianę z drzewem, 2 rundy × `-count=2`; maszyna niosła obciążenie 1,5–4 z
zapisu dysku, nie z CPU; pierwsza runda baza→drzewo dała drzewo o +1…+5 % wszędzie, także na
gruncie GPU i sektorach, których etap nie tknął, a odwrócona kolejność drzewo→baza dała bazę o
+0…+15 % median: przesunięcie jest z kolejności i obciążenia, nie z kodu; mediany i minima obu
rund razem):

| Bench / sonda | Wynik | Cel |
|---|---|---|
| `World_Publish` wszystkie rungi | min ±4 % w obu kolejnościach, alokacje 0 | ±2 %: szum, bez zmian |
| `World_Draw` wszystkie rungi | min −1…+10 % (baza→drzewo), mediany −2…+7 % | ±2 %: szum (dwie kamery +7–9 %, ścieżka nietknięta) |
| `World_Sheet/sector,64`, `frame,8` | 1,45 / 2,20 ms, +1,6 / +0,7 % min | ±2 % |
| `Sectors_Expanse banded/unbanded` | +1,6 / +1,7 % min, odwrócone +0,4 / −0,9 % | ±2 % |
| `World_Preview/field,256` | **104–117 µs**, 12 KB, 1 alokacja (migawki) | ≤ 0,3 ms ✓ |
| `World_Preview/declared,256` | 18–20 µs, 0 alokacji (pole z własnym L4: bez podglądu) | — |
| `World_SheetFiles/load,tile` | **2,5 ms**, 2,1 MB, 822 alokacji | ≤ 2 ms: minięty o 25 % |
| `World_SheetFiles/save,tile` | **29,4 ms**, 1,7 MB, 528 alokacji (PNG BestSpeed) | ≤ 10 ms: minięty 3×; w goroutine, ≤ 1 kafel/s |
| `Frame_Topography` wszystkie rungi | +1,3…+5,6 % min, +0,6…+7,8 % median (jedna kolejność) | ±2 %: grunt nietknięty, przesunięcie jak wyżej |
| Madera L4 (start, 736 sektorów L4, 1024×768, bez SIMD w obu buildach sondy) | tick **8,7 ms** w stanie ustalonym (11–18 ms przez pierwsze ~6 s: podglądy i hierarchia), publikacja 0,75 ms, rysowanie 2,3 ms, **presenting 22 ms**, 40 FPS; baza (stara mapa, start na L2): tick 2,1 ms, presenting 5,6 ms, 114 FPS | tick ≤ baza + 15 %: **nieosiągnięty**; baza nieporównywalna (L2, bez pól) |
| Madera, `Read.Painted` na L4 | 24 sektory na sekundę gry, 173 po 8 s i stoi (agent E) | 0 zdjęć po ≤ 150 klatkach: ✓ |
| Madera L2 nad lasem | 24 118 drzew na L2 (kamera ortogonalna rozwija wszystko w `Sight`); tick **20–30 ms** (publikacja 5 ms), rysowanie 5 ms, 40–50 FPS; baza 1,6 ms | tick ≤ 4 ms z ~5 000 drzew: **nieosiągnięty**, założenie 5 000 błędne |
| Katalog kafli | 928 KB po 45 s (17 plików, widoczna część wyspy); 944 KB po 100 s (agent E) | ≤ 20 MB ✓; pełne pokrycie w 2 min: nie sprawdzone (kamera stała) |
| Drugi bieg | pełna płachta od pierwszej narysowanej klatki (agent E) | ✓ |

**Wnioski.** Ścieżki nietknięte bez zmian (szum pomiaru). Podgląd tani (0,1 ms na sektor z 256
częściami, koszt leży w regułach gry). Kafle: zapis 29 ms na kafel w goroutine, przy ≤ 1 na
sekundę to ~3 % rdzenia przez czas zapisu; odczyt 2,5 ms na kafel na starcie (Madera: kilkadziesiąt
kafli, poniżej 0,2 s). Dwie rzeczy do decyzji użytkownika, nie do zamknięcia w tym etapie:
(1) **wyspa na L4** kosztuje 22 ms GPU na Intel UHD 620 (presenting; cała wyspa w płachcie z
lasami jako kopułami i kratą płachty w 173 sektorach, do tego 8 fotografii na klatkę w biegu bez
kafli) i 8,7 ms ticku w stanie ustalonym, gdzie baza na starej mapie na L2 miała 2,1 ms: tick
jest po stronie gry (hierarchia tras dla karawan, 260 pól, 800 sektorów) albo świata (sygnatura
736 sektorów w publikacji to 0,75 ms, nie to); profil CPU sondy to następny krok;
(2) **las na L2** to 24 tys. drzew, bo kamera ortogonalna ma jedną skalę w kadrze i rozwija
każde pole w zasięgu `Sight` (30 km): tick 20–30 ms, z czego przebieg wysokości topografii liczy
`GroundAt` dla każdej stojącej części co krok (24 tys. odczytów reliefu; część nie rusza się,
więc gruntowanie raz przy spawnie i przy zmianie reliefu wystarczy), reszta do profilu; drabina
kamery ortogonalnej po odległości albo krótszy `Sight` to decyzja o świecie. Oba w roadmapie.

### Dogrywka `d/shade` (plan 2026-11-07): cień gruntu po sektorach, w kafelkach

**Zgłoszenie użytkownika** (2026-11-07): Madera na `6ca7f88b` staje na ~26 s przy zoomie i przy
przesuwaniu planszy. Sonda (kopia dema, zoom `=`/`-`, W/D trzymane; zrzuty stosów w chwili
stania) pokazała wątek renderu w `render/internal/compose/bake.go` w `joined`, wołanym z
`Baker.gather` → `Plan` → `lighting.plan`. Przyczyna: drzewa i domy stoją na
`board.StandingTemplate`, który domyślnie daje `light.Shadow{Cast: light.Baked}`; zoom L4→L3
kamerą ortogonalną rozwija każdy las w zasięgu (~31 000 drzew), każde drzewo to blok cienia;
`light.Plugin.Publish` buduje co klatkę listę wszystkich bloków świata, `Baker.gather` liczy jej
hash, a gdy się zmieni, `joined` skleja pary blok–blok powtarzanym przebiegiem z `slices.Delete`
(setki milionów porównań), po czym `Plan` planuje wypiek całego cienia. Ten sam przebieg z drzewami
bez cienia (tylko w kopii) nie stanął. Decyzja użytkownika: nie przyspieszać `joined`, lecz usunąć
przyczynę ze struktury (wzór: strony Virtual Shadow Maps z UE5 unieważniane tam, gdzie pada cień
zmienionej rzeczy); cień drzew zostaje `Baked` (wygląd), koszt wypieku do zmierzenia potem.

**Projekt.** Światło publikuje bloki w **zestawach po sektorze**: `render.Lights.Casts
[]BlockSet{Box, Top, Version, Blocks}` zamiast `Blocks []Block`; `light.Plugin` zna sektory przez
`world.Read.SectorOf(x, y)` i `SectorsLen()` (jedyne dotknięcie świata; bez sektorów jeden zestaw);
`standing()` idzie po zapytaniu jak dziś, kładzie blok do zestawu sektora jego środka i miesza
hash zestawu; zestaw o zmienionym hashu dostaje `Version++`; bufory trzymane między klatkami.
Baker piecze **kafelkami** `shadeTile` px (256; Madera 5120×2560 → 200) do tego samego obrazu
dwoma torami: słońce (obrót, wersja reliefu, pierwszy wypiek) pasami jak dziś z `Mix`; treść:
zestaw o nowej wersji brudzi kafelki w zasięgu swojego cienia (`Box` przesunięty o `-sun.xy ·
(Top − Low)/sun.z`, rozszerzony o `far·shadowSoft + Cell`, arytmetyka `shading()`), brudne
kafelki pieczone w budżecie `tilesPerFrame` (8) na klatkę, najstarsze pierwsze, bez przejścia,
`was` kopiowane przed wypiekiem; kafelek = `bakeInto(rect)` + `bake_blocks.wgsl` z instancjami
`near(rect)` po zestawach; `joined` tylko na blokach `near` kafelka (ten sam obraz pokładu, koszt
po kilkuset blokach); `hashed` całej listy odchodzi. Shadery planszy i topografii bez zmian
(`groundShadeOf` czyta ten sam obraz i te same `Mix`).

**Pomiar** (baza `6ca7f88b`): sonda zoom/przesunięcie bez kroku > 100 ms; `Bake_Tile` ≤ 0,5 ms;
`Light_Publish` ≤ 0,4 ms; `Frame_Topography` ±3 %; `World_Publish`, `Sectors_Expanse` ±2 %;
zrzuty topography L2 przy moście, look/light, look/sky piksel w piksel z bazą. Wchodzi do PR #56
jako ostatnia część etapu `d` (bez niej PR scaliłby demo, które staje); dalsze kroki (piętra
siatki, uchwyt na całość, budżet rozwijania) to etap `t`.

**Wynik dogrywki `d/shade`** (2026-11-07, agent Opus w worktree, scalone `--no-ff` do
`feat/madeira-aggregates`; baza `6ca7f88b`, pomiar integratora ~9 min). Przestój zniknął: sonda
zoomu i przesuwania (150 s skryptu, próg 100 ms) na bazie staje cztery razy po ~54 s i nie kończy
się w 250 s; drzewo kończy skrypt bez przestoju, zostają 4 kroki 103–335 ms, które ma też baza:
własne rozwinięcie i zwinięcie świata przy zoomie (178–205 ms z drzewami bez cienia) z
przełączeniem `Coarse` przez dźwignię budżetu (cały cień na nowo, ~42 000 draw po 16 pasach) i
pasy słońca nad lasem (4000–5400 drzew na pas, GPU). Tempo L2 nad lasem (PROBE_AT=104,40,0.8,
45 s, baza/drzewo na zmianę): baza stoi 37 s od pierwszego rozwinięcia i nie loguje ani jednej
klatki, drzewo 58 i 56 FPS (tick 11,1–11,7 ms, rysowanie 3,1 ms, prezentacja 11,7–12,5 ms;
w etapie `d` ten sam widok dał 42 FPS przy 18 s pomiaru, reszta to stanie). Cele chybione:
`Bake_Tile` 4,3–4,8 ms zamiast ≤ 0,5 (GPU: czworokąt każdego bloku do najniższego gruntu; CPU
~15 %, klatka bez wypieku 0,20 ms), `Light_Publish` 0,60–0,61 ms zamiast ≤ 0,4 (0,25 przebieg,
0,18 `Grid.Of`, 0,15 hash; 0 alokacji). Nietknięte: `Frame_Topography` wszystkie rungi ±2 %
min, `World_Publish` i `Sectors_Expanse` w szumie (±3 %). Zrzuty: look/light i look/sky różnią
się tylko panelem, topography L2 przy moście (słońce zamrożone) 25 px o 1/255 poza panelem.
Odstępstwa agenta: `joined` wołany tylko na blokach kafelka, których długości się stykają
(radix po x, test y; test równości z `joined` nad wszystkim), bloki odwrócone jak `joined` by je
odwrócił; testy compose wewnętrzne. Otwarte (roadmapa „The shade in tiles, what is left"): cień
drzew jako decyzja wyglądu, ciaśniejszy czworokąt w `bake_blocks.wgsl`, `Coarse` bez pełnego
wypieku, kafelek bez przejścia, `joined` do usunięcia z uchwytem w `t`.

## Piętra sektorów i uchwyt na całość (plan 2026-11-07, litera `t`, w PR #56 na `feat/madeira-aggregates`, przed `i`, v0.20.0)

### Kontekst i decyzje użytkownika (2026-11-07)

Słowo użytkownika (2026-11-07, po zamknięciu `d/shade`): etap `t` nie czeka na scalenie #56 ani
nie idzie z `main`: preludium i części scalają się do gałęzi PR-a `feat/madeira-aggregates`
(jak dotąd reszta `d`), a PR #56 składa się na nowo na końcu etapu. Części w worktree'ach na
`t/<część>` od commitu preludium.

Z rozmowy o wzorze UE5 (World Partition: aktor w najmniejszej komórce siatki, która mieści go
całego; most jako całość, nigdy kawałki; małe rzeczy według położenia na poziomie 0) wynikły dwa
braki grama: **uchwyt na całość** rzeczy stojących układanych kawałkami (most, mur) i **piętra**
siatki nad sektorem, do których te całości trafiają. Dziś szwy między poziomami liczy publisher na
rzecz i na klatkę (`alive`, `drawn`, `sheetedIn`, `fine`, `publisher.go:382-480`), a most to pomosty
i filary bez niczego, co je spina, sklejane w cień dopiero przez `joined`. Decyzje: reguła „most to
encja na komórkę" zostaje, uchwyt jest dodatkiem nad kawałkami; słowa: **piętro** (`sector.Tier`;
„blok" zajęty przez `render.Block`), `world.Whole{}` na uchwycie, `world.PieceOf{Whole}` na
kawałku (nie `PartOf`: ten znaczy „sumowany przy zwijaniu"), `Write.SpawnWhole`/`SeedWhole`; mała
rzecz (dłuższy bok ≤ `Config.Sectors.Small`, zero = ćwierć boku sektora) na piętrze 0 według
środka, większa na najniższym piętrze, którego komórka mieści ją całą; trwała scena na GPU
(bufory instancji na komórkę, odrzucanie w compute, `DrawIndexedIndirect`; wgpu v0.34.6 ma to w
API, gram nie używa) to osobny etap `v` po `t` i `i`.

### Projekt

- **Piętra** (`plugins/world/sector`): piętro 0 = sektor, piętro k = 2ᵏ×2ᵏ sektorów, najwyższe
  obejmuje świat; `Tiers()`, `TierLen(k)`, `TierBox(k, i)`, `TierOf(k, x, y)`, `Under(k, i, each)`;
  `Fit(box, small) (tier, cell)`. Bez encji.
- **Poziom komórki piętra** (publisher): tabela `levels[tier][cell]` na widok i publikację
  (najdrobniejszy poziom sektorów pod komórką, z `view.Sectors`, ≤ ⁴⁄₃·800 wpisów); rzecz
  `Fixed` > `small` rysowana żywo poziomem komórki, gdy < L4, i w żadnej płachcie; od L4
  fotografowana do każdego pokrytego sektora (jak dziś `stand`). Zastępuje `alive`/`drawn`/
  `sheetedIn`/`fine` dla rzeczy dużych; małe jak dziś z pierścieniem (`ring`, decyzja 2026-11-05).
  `Fit` liczony na publikację z pudła, bez indeksu i zapisu.
- **Uchwyt** (`plugins/world`): `Whole{}` (uchwyt: `Fixed`, `Position` = łączne pudło, `Z` od
  spodu do szczytu, opcjonalnie `Lie` + `Velocity.Dir`, bez sprite'a, `Shadow{Baked}`), `PieceOf
  {Whole}` (kawałek: `Fixed`, `Shadow{NoShadow}`); `SpawnWhole`/`SeedWhole` w jednym kroku
  (`spawnSystem` pisze id uchwytu w `PieceOf` jak `SpawnPart` w `PartOf`); despawn uchwytu usuwa
  kawałki; rejestracja i zapis (`goke.LoadComp`), stare zapisy nie wczytują się (CHANGELOG).
- **Mosty na uchwycie** (`plugins/bridges`): `parts()` zwraca uchwyt (`Position` od `From` do
  `To` × `spec.Width`, `Z{Altitude: Deck − Thickness, Height: Thickness}`, `Lie`, `Dir`,
  `Shadow{Baked}`, `Fixed`, `Whole`) + pomosty (`NoShadow`) i filary (cień własny) przez
  `SpawnWhole`/`SeedWhole`; **`joined` odchodzi** z `bake.go`.
- **Budżet rozwijania** (`wholes.go`): `Ladder.Unfolds` (odcinków na krok, zero = 16); `open`
  rozwija najwyżej tyle, najbliższe obserwatorom pierwsze, reszta czeka zwinięta; podgląd bez zmian.

### Wykonanie

Jak w `d`: preludium (agent A, `feat/tiers`: piętra i `Fit`, `Whole`/`PieceOf`/`SpawnWhole`/
`SeedWhole`, `Config.Sectors.Small`, `Ladder.Unfolds`, testy), części w worktree'ach na
`t/<część>`: `t/tiers` (agent B: tabela poziomów w publisherze, testy szwów zielone, nowy test
mostu przez trzy sektory, `Benchmark_World_Seams`), `t/whole` (agent C, równolegle: mosty na
uchwycie, `joined` usunięte, test równoważności cienia, zrzuty mostu piksel w piksel), `t/budget`
(agent D, równolegle: budżet, test 40 odcinków / `Unfolds: 10` → cztery kroki, sonda zoomu nad
wyspą). Pomiar: `World_Publish` ±2 %, `World_Seams` ≤ baza, `Frame_Topography` ±3 %,
`Sectors_Expanse` ±3 %, `Bake_Tile` ±5 %, sonda Madery: najdłuższy krok ≤ 2× tick ustalony.
Dokumenty: `CLAUDE.md` (piętra i uchwyt w regule LOD), `world/doc.go`, `sector`, `bridges/doc.go`,
`lod/doc.go`, skille, CHANGELOG, roadmapa (zdjąć „ortho camera unfolds everything"; dodać mur
gramapu na uchwycie, etap `v`). Nie wchodzi: trwała scena GPU (`v`), HLOD na piętro, asynchroniczne
kafelki płacht, mur gramapu na uchwycie, `Collider` drzew, cień płachty na L4, `Mix` kafelka cienia.

### Wynik etapu `t` (2026-11-07, w PR #56 na `feat/madeira-aggregates`, baza `19f208ca`)

Preludium (agent, w klonie: `1e239fd8` piętra i `Fit`, `c5ab0d71` `Unfolds`, `aa438f8e` `Small`,
`dcabebe8` uchwyt) i trzy części równolegle w worktree'ach, scalone `--no-ff` w kolejności
tiers → whole → budget (jeden konflikt w `publisher.go`: `t/whole` wykluczało `Whole` z zapytania
renderu, `t/tiers` pomija chunk z `Whole`; zostało pominięcie). Odstępstwa: pudło uchwytu nie
podlega `Entities.MaxSize` (topografia ma `MaxSize` jednej komórki, most kilka); kawałek bez
uchwytu sądzony po własnym pudle; rzecz w komórce piętra nigdy nie rzedzona `Keep`; reguła pięter
tylko w światach z płachtami; `Z` uchwytu `{Deck, Thickness}` (światło czyta leżącą rzecz od
`Altitude` w dół, jak pokład); uchwyt z `vision.Transparency{1}` (ukośny most ciąłby wzrok);
`Ladder.Check` odrzuca ujemne `Unfolds`; stare zapisy wczytują się, ich pokłady rzucają cień po
kawałku (bez uchwytu). Pomiar integratora (~7 min, baza/drzewo na zmianę, bezczynna maszyna):
`World_Publish` wszystkie przypadki w szumie albo drzewo szybsze (min −0,1…+1,7 %, `keep=0.25`
drzewo o 6 % szybsze), `Sectors_Expanse` ±1,4 %, `Frame_Topography` nie zmierzony osobno
(żadna część nie dotyka rysowania klatki; dema zdrowe), `Bake_Tile` 4,36 ms (baza 4,94: bez
`joinedOf` na kafelek). **Chybione: `World_Seams` ≤ baza** (bench `t/tiers`, baza `dcabebe8` z
tym samym plikiem, `-cpu=1`, 3×2): `long` drzewo 168→182 µs (+8 % min, +12 % med), `small`
156→164 µs (+5 % min, +1 % med), 0 alokacji; to `Grid.Fit` (kilka dzieleń całkowitych na
próbowane piętro, ~27 % publikacji długich rzeczy), dźwignia `Fit` w O(1) z pierwszej i ostatniej
kolumny i wiersza sektorów pudła (roadmapa). Sonda zoomu i przesuwania Madery (240 s, próg
100 ms): jeden tick nad progiem, t248 158 ms (baza 251 i dalsze 30–411 ms w t249–t250), po nim
najwyżej 51 ms, tick ustalony 9 ms; drugie zejście najwyżej 33 ms; 31 376 części schodzi w 17
krokach po ~1800. Ten jeden tick to nawigacja (jednostki schodzące z grafu abstrakcyjnego pytają
o trasy naraz: `traffic.walkOn` → `routing.answer` → hpastar), nie rozwinięcie, więc „najdłuższy
krok ≤ 2× tick ustalony" nie jest spełnione przez nawigację (roadmapa). Klatki nad 100 ms przy
krótkim ticku (t478 315 ms, t652 139 ms…) to strona rysowania z `d/shade` (`Coarse`, pasy
słońca). Zrzuty (agent `t/whole`, baza `dcabebe8`): topography L2 przy moście prostym i ukośnym,
flat przy moście: 0 px różnicy poza panelem telemetrii. Dema topography, flat, forest-army,
roads, square, hex, look/light zdrowe. Otwarte w roadmapie „Tiers and the handle, what is left".

### Dogrywka po etapie `t` (2026-11-07 wieczór, w PR #56): `f/photos`, `f/caravan`

Użytkownik: topografia w okienku 20 FPS (przed etapami ~60 w pełnym ekranie), jednostki gracza w
ruchu niezaznaczalne. Diagnoza (agent; UHD 620, okno 1024×768, `GRAM_VSYNC=off`): widok startowy
L4 nieruchomo 38 FPS wobec 110 na `main`, present 24 ms; z pominiętym `drawPhotos` 147. Przyczyna:
krata fotografii 32×32 na każdy sfotografowany sektor (172 w widoku × 3 warstwy ≈ 560 tys.
wierzchołków) z pętlami `topNear`/`coarselyNear` po 75 tekseli na wierzchołek w shaderze
wierzchołków: kod z #55, treść (lasy w niemal każdym sektorze) z `d`. Baker i płachty czyste
(jeden pas na klatkę od słońca, zero kafelków, zero fotografii po 2 s). Zaznaczanie: kod
nietknięty, wozy i tragarze karawan nigdy nie miały `Allow`. Mój pomiar etapu nie objął widoku
startowego L4 (tylko L2 nad lasem i benchmarki): wniosek do protokołu (`gram-perf-picture`).

`f/photos` (`63ebea8f`): szczyty narożników liczone przebiegiem `tops.wgsl` do obrazu (teksel
na narożnik, sektor i 8 sąsiadów) przy zmianie wersji fotografii (`Read.Painted` zwraca wersję,
zmiana łamiąca), ćwiartki po ≤ 8 tekseli (`photoQuad`; Madera 8 na L4, 4 na L5; przy dawnej
gęstości 0 px różnicy, więc przebieg odtwarza dawne szukanie co do bitu).

| widok | `main` | `c09e6f7a` | drzewo |
|---|---|---|---|
| start L4 nieruchomo, dwie rundy | 111 / 106 | 38 / 38 | 135 / 123 |
| zoom 1 (L2) | 172 | 112 | 113 |
| `Frame_Topography/iso,level=4,sheet` | – | 12,9 ms | 13,2 ms (+2,6 %) |

`photoQuad=4`: start 113 FPS, bench 13,0 ms, mniej pikseli zmienionych; zostało 8 (klatkarz
dema przed syntetycznym benchmarkiem samotnych latarni). Zrzuty: różnice tylko z gęstości
(sylwetki koron o 1–2 px na stokach, przesunięcia podpikselowe w lasach); pętla zoomu bez skoku
GPU. Zoom 1 to tick (31 397 encji rozwiniętych), poza tą poprawką.

`f/caravan` (`a5d53457`, `51a2baa2`): część stoi za całością w `selection` (`standsFor` na każdej
ścieżce, obrys wokół części, rozwinięta całość nietrafiana), karawana z `Allow` i `Highlight`.
Sonda: klik w wóz w ruchu zaznacza karawanę i tylko ją, klik w ikonę na L4 też; `Publish`
renderera na równi z bazą, 0 alokacji. Zostaje (roadmapa): `Chosen` rozwiniętej karawany stoi w
miejscu rozwinięcia (C/V za pustym miejscem), całość idąca za środkiem części; piechurzy gracza
nie ruszają na L4 do pierwszej budowy hierarchii (było przed etapem).

### Dogrywka FPS (2026-11-08, w PR #56): `f/still`, `f/shine`, `f/ladder`

Użytkownik po `f/photos`: „nadal fatalny spadek FPS”, „zostawiam cię z tym”. Diagnoza w jego
warunkach (vsync, okno w przodzie): okno 1024×768 trzyma 80–87 FPS (dołek: las na L3 45 FPS
przez ~8 s do adaptera); **pełny ekran 24–26 FPS wobec 55–57 na `main`**: kamera podnosi zoom
startowy do progu wypełnienia okna (0,18 / 0,28 / 0,38 przy 1024 / 1920 / 2560 px), drabina
dema dawała L3 od 0,25, więc cała wyspa (31 397 drzew i domów) stała rozwinięta. Krok 12–15 ms
(`main` 2,4): symulacja 60 kroków/s plus publikacja zajęta w 90–100 %, klatka czeka; krok ponad
~15 ms → 5 kroków na klatkę → 11–13 FPS. Profil kroku: `Grid.index` całej siatki po każdym ruchu
2,2–3,7 ms; kolizja 1,7–2,4 ms (próg siatki 512²: 0,97 ms przy 50 pudłach, kubełki 64 od
`MaxCount` 40 000); wysokość części co krok 1,25–2,2 ms; hierarchia (pierwsza budowa ~33 s)
2,1–2,3 ms; pogoda 2 ms (komórki planszy, jak `main`); minimapa 2,2–4,7 ms publikacji. Rysowanie:
pasy cienia co klatkę nad 31 tys. bloków (295 pasów/300 klatek; bez nich +35 % FPS, w ticku, bo
iGPU i CPU dzielą budżet mocy), stałe klatki na sprite ~6 ms/klatkę, `vkCmdCopyBuffer` ~4 ms
(1,9 MB wierzchołków/klatkę; etap `v`).

- `f/still` (`1fe9b8a1`, `b6f60ce2`): wysokość części przy przyjściu i zmianie wersji reliefu
  (porównanie chunków jak w indekserze świata); minimapa po rodzajach ze znakiem, bez zapytania
  przestrzeni i `Seek`. Pełny ekran L3 (adapter zatrzymany): 17–18 → 21,5 FPS, publikacja 10 →
  5,8 ms; krok wciąż ~13 ms (`Grid.index` 3,7, `Pairs` 2,1, pogoda 2,1). Nawigacja: licznik
  przejezdności stoi, budowa kończy się o 33,5 s gry; kolizja po stronie grama bez dźwigni
  (`CanCollide` już filtruje, koszt to siatka).
- `f/shine` (`cb6e3e19`, `492a1245`, `bb944c2c`): pas cienia na 1/16° obrotu słońca od
  ostatniego, najwyżej jeden na klatkę; skok godziny pasami po jednym na klatkę; światło dalekie
  i niebo raz na klatkę, lampy tylko w pudle zasięgu. Rysowanie 21 → 13,5 ms/klatkę (1080p pełny
  ekran L3), obraz identyczny poza krawędziami cieni w kadencji; benchmarki klatki w szumie,
  `iso,level=3` −2…−7 %.
- `f/ladder` (`53db977f`): `Scales {0, 0, 0.6, 0.4, 0.12, 0}` (L4 pod drzewem 8 px; L3 0,4–0,6;
  L2 od 0,6 zamiast 0,5: między 0,5 a 0,6 znika siatka, brzeg, bieżąca woda i stojące wozy;
  pierwszy wariant `{1.0, 0.5}` odrzucony, bo oddawał to aż do zoomu 1,0 i adapter mógłby tam
  rysować płachtę). Teksel płachty bez zmiany (zależy od progu L4 0,12, nie L3), pamięć podręczna
  użytkownika czytana dalej. Pełny ekran 1920×1080, adapter włączony: **60 FPS, tick 7 ms, 471
  encji** wobec 47–49 FPS, tick 18 ms, 31 397 encji w bazie `f9fed456`; okno i zoom 1 bez zmiany
  obrazu (L4 i L2 jak dotąd). Oko przy `{1.0, 0.5}`: lasy z płachty płaskie, schodkowe krawędzie
  komórek, przy 0,4 piechurzy jako banery z liczbami — tego wariantu nie ma.

Zostaje: warstwa nieruchoma w aabbworld (`feat/still-layer`, w pracy) i część grama po niej;
pomiar okna i zoomu 1 na `f/ladder` nie dokończony (ekran zablokowany; reżimy niezmienione).

### Dogrywka po obrazku z doliny (2026-11-08, w PR #56): `f/counts`, `f/band`, `w/forest`

Użytkownik, zrzut z perspektywy w dolinie: liczby banerów (8, 16, 2, 4) prześwitują przez
zbocze, grupa 30 piechurów płaskowyżu rozbita na cztery banery, drzewa na grzbiecie jako płaskie
owale. Diagnoza: liczby maluje `DrawGlass` na szkle (kawałki glifów z arkusza czcionki na GPU,
ale bez głębi; baner pod nimi ma głębię i ginie za zboczem); klucz grupy banera to (widok,
sektor, rodzaj, poziom), a piechurzy stoją w kolumnach 186–203 i wierszach 127–140 przy
sektorze 16 komórek, więc szwy 192 i 128 tną ich na 2+4 nad i 8+16 pod; drzewo ma jeden wygląd
na L2 i L3 (billboard korony) i płachtę od L4. Warstwa nieruchoma: `f/space` scalone lokalnie
(`45c1957a`), aabbworld `feat/still-layer` wyciągnięty w klonie `/home/karol/CODE/aabbworld`
pod `go.work`; `go.mod` zostaje na v1.19.0 do tagu użytkownika (CI czerwone do tej pory).

Trzy części równolegle, worktree'y od `45c1957a`; żadna nie dotyka dokumentów, skilli,
`CHANGELOG.md`, `BENCHMARKS.md` ani `go.mod`; każda: sekwencja przed commitem, prefiksy, trailer
sesji bez `Co-Authored-By`; pomiary (klatkarz, benchmarki) robi integrator po scaleniu, część
oddaje polecenia i sprawdzenia deterministyczne.

**`f/counts` (world, render): liczba banera w obrazie, z głębią punktu.** Pliki:
`plugins/world/banners.go` (`DrawGlass` odchodzi wraz z asercją `render.Glass`, jeśli nic innego
jej nie używa), `plugins/world/renderer.go`, `render/` (nowe wejście klatki: napis z arkusza
glifów `render.Font` postawiony w punkcie świata jak billboard o stałym rozmiarze w pikselach,
z głębią tego punktu, nad obrazem gruntu i figur: `Frame.Text(font, s, x, y, z, colour)` albo
źródło Direct w `render/internal/compose`; shader własny, naga/Metal zgodnie z `gram-render`),
`plugins/topography/internal/billboards` tylko jeśli głębia reliefu wymaga udziału Looka (wtedy
przez `world.Look`, nie zapytanie do świata). Liczba stoi nad banerem (`countAbove` jak dziś)
w kolorze rodzaju z czarnym podbiciem, widoczna tam i tylko tam, gdzie baner; na L4 w widoku z
góry obraz jak dziś co do piksela poza głębią. Testy: GPU (`gputest.Need`): liczba za blokiem
wyższym od banera nierysowana, przed nim rysowana; `renderer_internal_test` liczb bez szkła.
Zrzuty: topography start L4 z góry (ta sama liczba pikseli napisu), perspektywa z doliny
(napisy znikają za zboczem). Nic na szkle.

**`f/band` (world, demo topography): banery sklejone przez szew, całość za środkiem części,
grupa płaskowyżu jako agregat.** Pliki: `plugins/world/banners.go` (tylko `join`/`farSettle` i
nowe sklejanie), `plugins/world/wholes.go`, testy; `examples/board/topography/{band.go (nowy),
walker.go, demo.go, names.go, map.yaml}`. (1) Przed `farSettle` grupy tego samego rodzaju i
poziomu w sąsiednich sektorach (8 sąsiadów, torus przez szew), których pudła jednostek leżą
najwyżej `Config.Sectors.Small` od siebie, sklejają się w jedną: suma liczb, środek i rozmiar
średnie, członkowie w jednym przebiegu `ids`, klik zaznacza wszystkich; grupy dalej od siebie
zostają osobno. Test: 30 jednostek w czterech sektorach dają jeden baner; dwa rodzaje dwa;
jednostki o sektor dalej osobno; `Benchmark_World_Publish` ±2 %. (2) Agregat punktowy
`Unfolded` (nie `Field`) dostaje co krok `Position` równe łącznemu pudłu swoich części (zapytanie
części po całości już jest), więc jego odcinek, sektor, `Chosen` (C/V) i baner idą za częściami;
roadmapa: „the whole following its parts' middle” schodzi. Test: armia rozwinięta, żołnierze
100 jednostek na wschód, środek całości tam; `routes.golden` nietknięte. (3) Demo: 30 piechurów
płaskowyżu z `map.yaml` to jedna grupa `band` (`BandKind`, figura `band.go`: agregat punktowy jak
armia bez bitwy: `Unfold` stawia części `PlateauKind` co komórkę w bloku 6×5 wokół środka,
`Fold` zwija, `selection.Allow`, baner `playerColor` na L4, `Highlight`, `Mark`); jeden wiersz
mapy `{kind: band, side: reserve, start: [195, 133]}` w miejsce trzydziestu. Sonda: na L4 jeden
baner bez liczby, na L3 30 piechurów zaznaczalnych jako całość.

**`w/forest` (plugin `plugins/forest`, oba dema lasu): las jako plugin, rodzaje drzew.**
Pliki: `plugins/forest/**` (nowy, układ jak `bridges`: `Plugin`, `Types().Define(name, Spec)`,
`doc.go` ze słownikiem), `examples/board/topography/{forest.go, tree.go, demo.go, names.go,
scene.go}`, `examples/board/forest-army/{forest.go, tree.go, demo.go, names.go, scene.go}`;
nie dotyka: `plugins/world`, `plugins/board`, `cmd/gramap`, map. Plugin nad światem i planszą
(konstruktor bierze `*board.Plugin`): rejestr **rodzajów drzew** (`forest.Spec{Height, Crown
(bok pudła w jednostkach świata), Trunk, Colours albo własne SpriteDrawer korony i boku, Shadow,
Roles (role gry dogrywane do drzewa: „felled” forest-army)}`), drzewo `Fixed`, część lasu,
zwijane z sektorem jak dziś (`Folding` → `Despawn` w pluginie); **las** jako agregat polowy
(`forest.Stand{Cells []string (rodzaje komórek, na których rośnie), Density (drzew na komórkę,
dziś 1,5), Jitter, Mix []forest.Share{Tree, Weight, From, To (pas wysokości; zero = wszędzie)}}`):
krata sadzenia, potrząśnięcie, ziarno `lod.Seed` z sektora i działki, wybór rodzaju wagami w
pasie wysokości działki (relief przez plansze: `Heights`), reguły `Unfold` generowane przez plugin
dla rodzaju lasu gry (`forest.Figure` potwierdzane `var _`); wygląd drzewa po szczeblach: L2
`Upright` (bok: pień i korona na czterech ścianach, wierzch: korona), L3 billboard korony jak
dziś, od L4 płachta (bez zmian w świecie). Topografia: trzy rodzaje Madery: wawrzyn
(laurisilva, szeroka ciemna korona, do 800 m), sosna (wysoka, stożkowa, od 600 m), wrzosiec
drzewiasty (niski, płaskowyż od 1300 m); forest-army: jeden rodzaj z rolą „felled”. Testy przez
prawdziwy `Install`: dwa rodzaje w mieszance dają oba po ziarnie, pas wysokości trzyma, gęstość
±10 %, `Folding` usuwa; `routes.golden` i zrzuty forest-army jak baza poza koronami (nowe
sprite'y). Sonda topography: liczba drzew na L2 nad lasem jak dziś ±5 %, tick ±5 %.

**`f/badge` (world, render, selection): baner jako tarcza z liczbą w środku** (słowo
użytkownika 2026-11-08: „baner jednostki to powinna być taka tarcza, np. romb i liczba w środku,
albo okrąg, o kolorze encji gracza”). Dziś baner to punkt w średnim pudle rodzaju (na L4 piksel
lub dwa) z liczbą nad nim. Ma być: znak o stałym rozmiarze w pikselach ekranu w punkcie świata
banera, z głębią tego punktu (jak `render.Labels` z `f/counts`): kształt (koło albo romb)
wypełniony kolorem rodzaju (`farColour`: w demach kolor strony), obwódka ciemna, liczba
jednostek wyśrodkowana w środku (biała lub czarna wedle jasności wypełnienia), bez liczby przy
jednej jednostce. Styl rodzaju jak inne znaki: `world.BannerStyle{Shape (Disc domyślnie, Diamond),
Size px (14), Fill, Rim}` przez `Slot.Styled`; bez stylu koło w kolorze rodzaju. Kształt liczony w
shaderze fragmentów (SDF, antyaliasing na brzegu), jedna instancja na baner, tekst z arkusza
glifów jak `Labels` (wspólny kod w `render/internal/instanced`). `Banner` z `Read.Banners` niesie
`Reach` (promień znaku w pikselach), `selection` trafia baner po nim zamiast stałej `bannerReach`.
Pliki: `plugins/world/{banners.go, look.go lub atlas.go (styl), rw.go (Banner.Reach)}`,
`render/internal/instanced/{labels.go, labels.wgsl lub badges.*}`, `render/text.go` lub nowy plik
aliasów, `plugins/selection/selection_system.go` (tylko zasięg), dema: `forest-army` i
`topography` mogą dać `BannerStyle` (romb dla rywala). Testy GPU: tarcza za blokiem ukryta, liczba
w środku, kolor wypełnienia rodzaju; zaznaczenie: klik w brzeg tarczy trafia. Zrzuty start L4 i
dolina.

Pomiar (integrator, po scaleniu, ekran odblokowany, okno w przodzie, vsync): `f/space` i części
razem przeciw `674cd371`: pełny ekran start L4, zoom 0,5 (L3 rozwinięta wyspa), zoom 1 (L2 nad
lasem), okno 1024×768 start; `World_Publish`, `Frame_Topography` (iso, level=3, level=4,sheet),
`Collision_*`, `Sectors_Expanse`. Dokumenty: CHANGELOG (Added `forest`, liczby z głębią, banery
przez szew, całość za częściami, warstwa nieruchoma; Changed dema), `gram-plugins` (forest,
world: banery, całość za częściami, `Restill`; motion, collision), `gram-perf-tick` (warstwa
nieruchoma), `gram-render` (napis z głębią), `CLAUDE.md` (plugin `forest` w mapie i w
poleceniach), roadmapa, README (tabela dem bez zmian nazw), #56 złożony na nowo.

**Wynik dogrywki** (2026-11-08 wieczór, w PR #56; baza `674cd371` na zmianę z drzewem
`b562d5c3`, oba przez lokalne aabbworld `feat/still-layer`; maszyna bezczynna, vsync, ekran
odblokowany, każdy bieg z ciepłej kopii pamięci podręcznej swojej strony, bo drzewo ma
`madeira-2`; dwa biegi na stronę, mediany sekund 10–40):

| Reżim (topography) | Baza: tick / rysowanie / prezentacja ms | Drzewo | Encje |
|---|---|---|---|
| pełny ekran 1920×1080, start (L4) | 5,1 / 1,7 / 14,4; 60 FPS | 5,3 / 1,7 / 14,4; 60 FPS | 471 → 442 |
| pełny ekran, zoom 0,5 (L3) | 11,5 / 5,5 / 10,3; dołki 52–54 | 8,8 / 5,2 / 10,7; dołki 57–58 | 26 566 → 26 687 |
| pełny ekran, zoom 1 (L2 nad lasem), drzewa `Upright` | 11,7 / 3,8 / 11,7; 60 FPS | 13,5 / **10,6** / 7,0; **49–55 FPS**, dołki 27–34 | 26 566 → 26 687 |
| to samo, drzewa bez `Upright` | — | 7,9 / 3,4 / 12,6; 60 FPS | jw. |
| okno 1024×768, start | 4,5 / 1,6 / 14,6 | 4,4 / 1,6 / 14,6 | 471 → 442 |
| forest-army L2 1280×800 | 0,5 / 0,5 / 15,9 | 0,4 / 0,5 / 15,9 | 623 → 603 |

- Tick przy zoomie 0,5 i (bez bryły) przy zoomie 1 o 3–3,5 ms niższy niż w bazie: warstwa
  nieruchoma (`f/space`).
- **Bryła drzewa na L2** (`Slot.Upright`, pięć kwadratów na drzewo, 26 tysięcy drzew w kadrze
  przy zoomie 1) kosztuje 6–7 ms rysowania na klatkę i ściąga tick za sobą. Decyzja: bryła jest
  wyborem rodzaju drzewa (`forest.Spec.Body`, `w/body`), domyślnie billboard boku; Madera bez
  bryły, forest-army (600 drzew) z bryłą. Sto trzydzieści tysięcy kwadratów ścieżką sprite'ów to
  robota trwałej sceny GPU (etap `v`).
- Benchmarki (3 rundy na zmianę, `-cpu=1`): `Frame_Topography` iso +0,6 %, level=3 +0,4 %,
  level=4,sheet +0,7 %; `World_Publish` −2,0…+1,9 % (rungi 49 w kadrze +0,5…+1,9 %, ~150 ns);
  `Collision_Tick`/`Swept` −0,5…+1,4 %; `Sectors_Expanse` banded −1,3 %; `World_Draw`,
  `World_Frame`, `Text_Panel` w szumie; alokacje bez zmiany.
- Zrzuty (okno 1024×768, klatka 300, adapter zatrzymany): start bez „1” nad banerami i bez
  „8 16 4” nad płaskowyżem (jedna banda, baner jednego, bez liczby); dolina bez żadnej liczby
  przez zbocze (baza: „16 8 4 2” i kilka „1”); lasy inne (trzy rodzaje drzew, nowe sadzenie,
  płachty `madeira-2`: pamięć podręczna użytkownika z `madeira-1` zostanie przefotografowana
  przy pierwszym uruchomieniu).
- `f/badge` wykonane inaczej niż w planie, na lepsze: tarcza stoi nad górą pudła banera (w
  środku tonęła w gruncie przed nim), napis stoi na swoim punkcie (każdy róg z głębią swojej
  wysokości nad punktem, więc to, co za punktem, nie zasłania nic, a to, co przed nim, zasłania;
  granica jednej głębi z `f/counts` znika), styl sam czyni rodzaj banerowanym bez looku L4
  (rywal topografii), tarcza rośnie pod cyfry (`render.BadgeWidth`, mierzone na gorutynie
  publikacji własną kopią czcionki liczb), `Labels.Text` bierze `color.RGBA` (przez
  `color.Color` alokował na etykietę i klatkę). Zrzuty startu i doliny: różnice tylko w
  miejscach banerów.
- Tarcza zmierzona osobno (`b562d5c3` na zmianę z `afc34c4d`, 3 rundy): `World_Publish` −1…−5 %
  (jeden `BadgeWidth` na baner na publikację nie waży), `World_Draw` i `Frame_Topography`
  level=4 ±1 %, alokacje bez zmiany.
- `w/body` (`36efbcb6`): `forest.Spec.Body`, test przez prawdziwe rysowanie (look nagrywający
  `Upright` i `Sprite`); sekwencja zielona w klonie po scaleniu (`567174f0`).

### Przegląd modelu pod optymalizację (2026-11-08 wieczór, Fable sam, słowo użytkownika)

Profil CPU sondy Madery (`scratchpad/prof`: kopia dema z sondą, `pprof` przez 600 ticków po
ustawieniu kamery) w trzech reżimach: start L4 pełny ekran, zoom 0,5 (L3, 26 tys. drzew), zoom 1
(L2 nad lasem), i jeszcze raz start w oknie 30–40 s.

**Co widać.** (1) Przez pierwsze ~25 s hierarchia HPA* buduje się w budżecie 75 000 kroków na tick:
~3 ms na tick w każdym reżimie (tick startu 5,4 ms, po zbudowaniu 3,8 ms); karawany stoją, aż
skończy. (2) `Finder.Reads` czyściło tablicę kroków gruntu wielkości planszy przy każdym odczycie
routera, czyli co tick podczas budowy i przy każdym pytaniu o trasę: 0,9 ms zerowania na tick,
choć każdy krok jest stemplowany przejezdnością i domeną. (3) Pogoda: `weathering.walk` chodzi po
wszystkich komórkach raz na sekundę gry i uruchamia reguły `Overhead` gry na komórkę: 0,9–1,4 ms
na tick, z czego 70 % to próbkowanie kopuły chmur na komórkę (`domeLevel.at`: cztery `cellAt` z
dzieleniami); przy 10 m to 100× więcej, więc to sprawa projektu Madery 10 m, nie łatka.
(4) `renderer.Compose` nad 26 tys. drzew: cztery dostępy do map na warstwę na encję (0,7 ms na
klatkę), `View.Fog` z `hypot` na encję (0,45 ms), `billboards.sprites.add` czyta relief na
billboard (0,7 ms), `light.standing` buduje bloki `Baked` od nowa co klatkę (1,2 ms). (5) Reszta:
publikacja 1,3 ms (26 tys.), zapytanie przestrzeni o kadr 0,7 ms, wołania wgpu ~1,5 ms na klatkę.

**Poprawione** (jeden commit na `feat/madeira-aggregates`): `Finder.Reads` czyści kroki tylko
przy innym terenie, wycenie lub poziomach (`same` przez `reflect`); tablice po sprite'ach
(`flags`, `shadedOf`, `edgeOf`) zamiast map w `Compose` i w `photographed`; `View.Fog` porównuje
kwadraty i pierwiastkuje tylko w pasie mgły. Testy world, navigation, topography, board zielone.

**Pomiar** (baza `ec872bc7` na zmianę z drzewem, dwa przebiegi, mediany sekund 15–45; okno szło w
kadencji 20 FPS, bo ekran był zablokowany pod nieobecność użytkownika, prezentacja 45 ms, więc
tick to trzy kroki na klatkę; porównanie CPU na klatkę stoi, klatkarz w reżimie użytkownika do
sprawdzenia przy biurku):

| Reżim | tick baza → drzewo | rysowanie baza → drzewo |
|---|---|---|
| start L4 | 15,2 → 12,9 ms (−15 %) | 2,40 → 2,10 ms (−12 %) |
| zoom 0,5 L3 | 20,0 → 18,8 ms (−6 %) | 6,7 → 5,2 ms (−22 %) |
| zoom 1 L2 | 19,4 → 18,2 ms (−6 %) | 4,6 → 3,8 ms (−18 %) |

**Zostaje, z liczbami:** relief na billboard (flaga `Fixed` w rekordzie figury: wysokość to
`Z.Altitude`), bramka dla `light.standing` (warstwa nieruchoma się nie zmienia, ale `Z` pisany
po spawnie bez wersji; potrzebna wersja „co stoi” ze świata podnoszona przez indeks warstwy
nieruchomej i przez przebieg wysokości), budżet budowy hierarchii (większy skraca rozruch kosztem
ticku; budowa w puli to praca w hpastar), pogoda po sektorach (projekt 10 m).

## Madera 10 m (plan 2026-11-08 wieczór, litera `m`; słowo użytkownika: „kolejne demo Madery obok istniejącego, z dokładnością do 10 m, dowzorowane dokładnie”; robota Fable'a samego)

### Fakty (sprawdzone 2026-11-08)

- Dzisiejsza Madera: 640×320 komórek po 100 m, jednostka świata 3,125 m (komórka 32 jednostki),
  figury 15× życia, sektor 16 komórek = 1,6 km, 800 sektorów, `MaxEntCount` 40 000, mapa
  `map.yaml` 1 MB (komórki znakami, narożniki reliefu tekstem), demo 884 MB RSS przy starcie.
- 10 m to 6400×3200 = **20,5 mln komórek**. Każda komórka jest dziś encją goke na stałe (`Plot`,
  `Ground`, `Way`, `Built`, znaczniki, role, etykieta) i ma miejsce w tablicach po ordynale:
  terrain (`stamps`, `passage`, `passed`: 24 B), nawigacja (`flats` ~32 B na router), zajętość,
  momenty (`met`), `reached`, `limits`, `drawn` (kopia komórek na każdą zmianę planszy). Samo to
  przy 20,5 mln to gigabajty i przebiegi po 20 mln na krok: pogoda chodzi po wszystkich komórkach
  raz na sekundę gry (0,9–1,4 ms na tick przy 205 tys., czyli ~100 ms przy 10 m), `drawn.publish`
  kopiuje wszystkie komórki przy każdej zmianie, hierarchia HPA* buduje się ~25 s przy 205 tys.
- Kafle terenu AWS (terrarium) kończą się na zoomie 14: na szerokości Madery ~8 m na piksel, a DEM
  pod nimi to SRTM/EU-DEM 25–30 m; „dokładnie 10 m” to dokładność siatki, nie danych: relief
  będzie interpolowany między próbkami co ~25 m. Kafli na wyspę ~300 (dziś zoom 11, kilkanaście).
- YAML z 20 mln komórek i 20,5 mln narożników to ~100 MB tekstu; `yaml.v3` zbuduje z tego drzewo
  węzłów w gigabajtach. Komórki i relief muszą być binarne.
- Kamera ortogonalna ma jedną skalę w kadrze, więc dziś przy zoomie 0,5 rozwija wszystko w zasięgu
  `Sight` (26 tys. drzew na L3 na całej wyspie); przy 10 m byłoby to 2,6 mln drzew.
- gramap przy 10 m (receptura Madery z `cell: 10`, `cells: [6400, 3200]`, zoom 14): **po 40
  minutach bez mapy, szczyt 10,2 GB RSS**, przerwany (`scratchpad/map10`). Jego struktury są na
  komórkę i nieoszczędne: `soils []string`, `corners []float64`, rzeki i drogi w mapach po
  `cell.ID`, A* dróg po całej siatce. Na 10 m potrzebuje części własnej: wysokości z kafli jak
  dziś (szybkie), rzeki, gleby, pola i osady liczone na siatce 50 m i przeniesione na 10 m,
  drogi planowane na 50 m i wygładzone na 10 m, pamięć w tablicach bajtowych, postęp logowany.

### Decyzje (rozmowa 2026-11-08 noc; plan zatwierdzony)

Kierunek użytkownika: sektor rozwija się z „seeda” i diffu. Seed terenu to kafle mapy (dane
zmierzone, nie generowane), seed szczegółów proceduralnych to `lod.Seed` jak dziś; diff to zmiany
gry względem kafli. Trzy korekty moich pierwszych punktów po pytaniach użytkownika:

- **Trasy dalekie idą dokładnie drogami.** Dane statyczne wszystkich komórek (rodzaj, droga,
  relief) leżą w RAM w zwartych tablicach (~140 MB przy 20,5 mln), hierarchia HPA* buduje się
  nad nimi, nie nad encjami, i doprecyzowuje trasę komórka po komórce wszędzie jak dziś; żadnych
  prostych od bramy do bramy. Encje komórek są do zajętości, efektów i reguł: tylko w sektorach
  rozwiniętych.
- **Graf hierarchii to pamięć podręczna gry, nie streszczenie z gramapu.** Koszty wewnątrz
  klastra zależą od wyceny gry (`Mobility`, Tobler, klasy), której gramap nie zna. Graf buduje
  gra (`-prepare`, raz, bez budżetu, przy produkcji mapy), zapisuje obok kafli z odciskiem
  (wersja kafli + odcisk wyceny) i wczytuje na start: **start nie buduje nic** (budowa przy 10 m
  to minuty na rdzeniu). Zmiany graczy (drogi, lasy, relief) idą jedną ścieżką: zapis do
  komórki podnosi wersję sektora; klaster pamięta wersje sektorów, z których powstał;
  niezgodność = `Stale` → przebudowa lokalna w budżecie, rodzice w kolumnie. Wczytanie zapisu:
  diffy podnoszą wersje, klastry zmienione przebudowują się w tle. Niezgodny odcisk wyceny =
  odmowa startu z komunikatem, nie cicha budowa.
- **Rozwijanie z wyprzedzeniem.** Pas sektorów za granicą L3 w kierunku ruchu kamery schodzi na
  L3 wcześniej (budżet `Unfolds` zachowany), tak by kamera nie dojechała do sektora zwiniętego;
  na starcie pierścień widoku startowego w pierwszych krokach. Jak strumieniowanie chunków
  Minecrafta w jednej warstwie (encje żyją, gdzie ktoś patrzy), ale komórka zbiorcza jest
  zawsze, więc reguły idą wszędzie, i dane statyczne mieszczą się w RAM w całości (kafle są
  formatem pliku i teksturą dalekiego gruntu, nie mechanizmem doczytywania).

Zostają: komórka na drabinie (encje w sektorze L0–L3, komórka zbiorcza od L4), mapa jako głowa
YAML i kafle PNG, pierścienie (zrobione), sektor 64 komórek, skala figur 3×, pogoda po encjach.
Zmieniają się dwie reguły domu: „every cell is an ECS entity for good” → „a cell is an entity in
an unfolded sector, the sector one gathering cell from L4; the terrain of every cell is in the
board's store”; „a map is a file” → „a map is a head and its tiles”. Decyzje przy zatwierdzeniu
(2026-11-08): etap na nowej gałęzi `feat/madeira-10m` od `main` po scaleniu #56, nowy PR; kafle
mapy 10 m i plik grafu generowane (`make`, pamięć podręczna w `~/.cache/gram`), nie w repozytorium;
hpastar na gałęzi lokalnej, scalany do grama od razu, tag użytkownika później.

### Projekt (co buduje etap; fakty z kodu 2026-11-08)

Dziś `plugins/board/internal/terrain`: `Cells` trzyma tablice po ordynale (`stamps`, `passage`,
`passed []uint64`, `reached`, `limits`), `store` mapuje ordynał → id encji i czyta rodzaj z
komponentów przez `Seek`; `Set/SetWay` → `touch` + `version++`; `entitySystem.spawn` robi encję
na komórkę, zapis trzyma te encje. `board.Lay` czyta YAML: `cells` znakami, `relief.corners`
tekstem. Nawigacja: `pathfind.Hierarchy` owija `hpastar.Solver`, klaster = sektor świata,
`buildBudget = 75_000` kroków na tick, `Finder.Stale` **chodzi po wszystkich komórkach**
(`CellPassage(c) > since`), hpastar jest **gotowy albo nic** (`Ready()` false przy jednym
klastrze stale → trasy spadają na dokładny A*, czekające `NoRoute`), **bez serializacji**
hierarchii. Pogoda: `weathering.walk` chodzi po encjach `cell.Plot`+`Wring`, `CellCount·d`
komórek na tick.

- **`m/store` (plansza):** magazyn (`kinds []uint8` po tabeli rodzajów, `ways []uint16`,
  narożniki `int16` m lub `float32`, zabudowa i etykiety rzadkie), **wersja na sektor planszy**
  podnoszona w `touch`, **diff** po sektorze (jedna pozycja na komórkę, zagęszczany przy
  zwinięciu). `Kind/Way/Built/Bare` z magazynu; `CellEntity(c)` tylko dla komórki rozwiniętej
  (przegląd wołających). **Strony:** encje komórek sektora spawnowane przy zejściu na L3 (z
  magazynu + realizacja stanu zbiorczego), despawnowane przy wejściu na L4 (suma do zbiorczej,
  diff zmienionych). Komórka zbiorcza: encja sektora (`cell.Plot{Sector}`, `Ground`
  większościowy, `Tags[States]`). Tablice po ordynale → po komórce rozwiniętej + po sektorze.
  Zapis gry: diffy i encje zbiorcze; strony rozwinięte odtwarzane z magazynu+diff (stare zapisy
  niekompatybilne, wpis CHANGELOG). Bench `Board_Pages`, `Board_Store` (20 mln ≤ 300 MB).
- **`m/map`:** głowa YAML + kafle PNG (`cells-<x>-<y>.png` indeks rodzaju, `relief-<x>-<y>.png`
  16-bit metry z przesunięciem jak terrarium, kafel 512); `levelFile.tiles {dir, version, cols,
  rows}`; `Lay` z kafli do magazynu bez `[]cell.Entry`; stare YAML dalej czytane; `gramap -tiles`.
  Bench `Board_Load/10m` ≤ 3 s.
- **`m/gramap`:** tablice bajtowe, siatka robocza `work: 50` (rzeki, gleby, pola, osady,
  drogi), przeniesienie na 10 m (drogi wygładzone po reliefie 10 m), postęp logowany; cel
  ≤ 10 min, ≤ 4 GB.
- **`m/rings`:** zrobione (`b73223a3`).
- **`m/ahead` (świat):** `Ladder.Ahead` (jednostki świata, zero = dzisiejszy margines pół
  sektora) i kierunek ruchu punktu widzenia w `sectors.hold`: sektory przed kamerą schodzą na L3,
  zanim je narysuje; `wholes.open` bez zmiany. Test: kamera jadąca stałą prędkością nie rysuje
  sektora L3 zwiniętego (licznik w sondzie = 0; `Ahead` do zmierzenia).
- **`m/weather` (atmosfera):** `walk` po encjach rozwiniętych i zbiorczych (`CellCount` → liczba
  encji), `Land.Note` zbiorczej na kwadrat sektora. Bench `Weathering_Walk`.
- **hpastar (repozytorium użytkownika, gałąź lokalna):** (a) `Save`/`Load` hierarchii, binarny
  strumień little-endian (nagłówek z opcjami i klasami, po klasie i poziomie klastry: `keys`,
  `states`, `class`, `twins`, `intra`, `offset`, drzewa); (b) gotowość po regionie: `Climb`
  dozwolony, gdy klastry na drodze planu są świeże, odmowa tylko przy stale na drodze; (c)
  `Update(0)` do końca i równoległa przebudowa klastrów, jeśli pomiar `-prepare` tego wymaga.
- **`m/nav` (nawigacja):** `Finder.Stale` z listy komórek zmienionych (plansza: `ChangedSince`
  z wersji sektorów i diffu; O(zmian)); `navigation.Config.Cache{Dir, Keep}`: plik
  `hierarchy-<domain>.bin` z nagłówkiem {wersja kafli, odcisk wyceny (tabela rodzajów: `PaceFor`,
  `Allows`, `Solid`; `Hiking`; `Classes`; `Side`), opcje}; brak pliku = budowa w budżecie jak
  dziś (dema 100 m bez zmiany); zapis atomowy po gotowości, gdy `Keep`, a potem porcjami: klaster
  przebudowany po `Stale` zrzucany sam, w tle, nie cały plik (użytkownik, 2026-11-09: struktura
  budowana raz, zapisywana, zrzucana małymi porcjami, gdy się zmieniła; gdzie i czy decyduje
  twórca gry); `flats` po stronie.
  `navigation.Prepare(board, dir)` publiczne wejście; demo z flagą `-prepare` woła je przed
  `gram.Run`. Bench `Navigation_Stale/10m` (droga 1 km ≤ 20 klastrów w budżecie jednego ticku),
  `Navigation_Load` (≤ 1 s), `Navigation_Prepare` (do zmierzenia; cel ≤ 10 min jednowątkowo).
  **Zrobione 2026-11-10 w PR #58** (słowo użytkownika): hpastar v1.6.0 `Save`/`Load`/`Dump`
  (rekordy klastrów z CRC, stany z `graph.Implicit.Of`), `navigation.Cache{Dir, Keep, Version}`
  i `WithCache`, plik `hierarchy-<domain>.bin` z odciskiem komórek każdego klastra zamiast wersji
  kafli (`BoardRead.CellFingerprint`; klaster nad zmienionymi komórkami `StaleCluster` po
  wczytaniu, reszta gotowa), zrzut co 5 s klastrów przebudowanych i odcisków w miejsce,
  `Counts.Loaded`, Madera trzyma hierarchię w `~/.cache/gram/topography/hierarchy`. Zostają tu:
  `Stale` z listy zmian, `-prepare`/`Prepare`, benchmarki 10 m.
- **`m/madeira10` (demo `examples/board/madeira`):** receptura `cmd/gramap/madeira10.yaml`
  (komórka 10, 6400×3200, zoom 14, `grain` 64, `work` 50), zestaw topografii, skala 3× (jednostka
  0,25 m, komórka 40), `Sectors` 64, `Rings` (L3 ~3 km, L4 ~12 km), `make madeira10-map` (gramap
  + `-prepare`) do `~/.cache/gram`; demo bez plików mówi, co uruchomić. Reżimy: start L4/L5 pełny
  ekran, zoom nad Funchal L3, L2 w lesie, okno; 60 FPS, tick ≤ 8 ms, pamięć ≤ 2 GB, start ≤ 3 s.

### Kolejność

`m/store` → `m/map` → `m/gramap` → `m/ahead` → `m/weather` → hpastar (a, b) → `m/nav` →
`m/madeira10`; po każdej części Madera 100 m bez zmiany obrazu i klatkarza (baza na zmianę,
bezczynna maszyna, ekran odblokowany). Dokumenty: `CLAUDE.md` (dwie reguły domu, graf pamięcią
podręczną gry), `board/doc.go`, `navigation/doc.go`, `lod/doc.go` (`Ahead`), skille board/world/
atmosphere/navigation/gram-perf-navigation, CHANGELOG, roadmapa, README.

### Co nie wchodzi

Trwała scena GPU (etap `v`), minimapa po drabinie (`i`), strumieniowanie kafli mapy z dysku
(wszystko czytane na start, ~140 MB), równoległa budowa w hpastarze bez pomiaru, który jej wymaga.

## Program „Madera 10 m” (plan 2026-11-10 noc, zatwierdzony; zastępuje kolejność z sekcji „Madera 10 m”)

Rozmowa i ankiety 2026-11-10 po v0.21.0. Decyzje: przed etapem `m` wchodzi **etap 0, używany już w
Maderze 100 m**: struktury podziemne (tunele, komory; przejazd w FPP/TPP), mosty (naturalne
rozmiary, wiadukty, z danych rzeczywistych), drogi, tunele i mosty z **OpenStreetMap**, ok. 10
gatunków drzew Madery z danych pokrycia terenu; przy 10 m **rozmiary naturalne** (bez skali
figur); etap `m` w trzech PR-ach; komórka zbiorcza bez reguł, pogoda po polu sektorów; jeden
gramap dla obu receptur; **format najpierw** (seed tylko do odczytu w blokach, diff w RAM z
dziennikiem na czas, przewijanie ekranu nigdy nie dotyka dysku), bo ta sama dokładność na mapie
kraju to inna skala.

| Etap | Gałąź | Co wnosi |
|---|---|---|
| 0a | `feat/underground` | plugin `underground`, komórka o czterech poziomach, kamera i rysowanie wnętrza |
| 0b | `feat/osm-roads` | gramap czyta OSM (drogi, tunele, mosty, zabudowa), mosty w naturalnych rozmiarach, migawka regresji Madery |
| 0c | `feat/madeira-species` | 10 gatunków, rodzaje lasu z danych pokrycia terenu |
| m1 | `feat/madeira-10m-store` | format blokowy, magazyn planszy, diff i dziennik, gramap na tablicach |
| m2 | `feat/madeira-10m-pages` | encje komórek tylko w sektorach rozwiniętych, pogoda po polu, `Ahead` |
| m3 | `feat/madeira-10m` | `Stale` z listy zmian, `-prepare`, hpastar gotowy po regionie, demo |

Każdy etap: gałąź od `main`, jeden PR, własny szczegółowy plan przed startem. Pomiary wstępne
(rig płaski, jedna klasa): budowa hierarchii ~8 µs na komórkę przy boku 64, liniowo (20,5 mln
komórek ≈ 170 s lądu; w budżecie ticku ~47 min gry, więc `-prepare` konieczne, równoległa budowa
nie); encja komórki ~0,55 µs i ~430 B (20,5 mln = 8,8 GB; sektor 64×64 rozwija się ~2,3 ms, więc
`Unfolds` liczone w komórkach); magazyn statyczny ~103 MB. Dane: Geofabrik ma tylko całą
Portugalię, otwartego rastra gatunków nie ma (IFRAM3: laurisilva 16 114 ha, eukaliptus 10 168,
akacje 4 102, sosna nadmorska 2 337, kasztan 594).

### Etap 0a: wynik (2026-11-10, gałąź `feat/underground`, baza `9450d4b3` v0.21.0)

Decyzja z ankiety w trakcie: poziomów komórki ma być N, rozsądnie mało (użytkownik), nie trzeci
poziom na sztywno ani podłoga zagłębiona na miejscu pomostu. Zrobione: `cell.Levels = 4` (grunt
i trzy kondygnacje, `Floor.Storey`, `Floor.Clear`, `Built.More`), `cell.Level` w miejsce każdego
`deck bool` (zajętość, `BoardRead`, `driving.Keeping`, rozkaz ruchu: `Leg.FromOn/ToOn`, poziom
kroku trasy w dwóch bitach `Path.Decks`+`Upper`), węzły nawigacji `4·ordynał+poziom` (finder,
pole, obrys, hierarchia; plik hierarchii z nowym znakiem `GRM2`), reguły zagłębionej podłogi w
finderze (wejście tylko wzdłuż łącza, prześwit `Clear`, wylot zamyka krok po gruncie), `Surface` i
`LevelFor` pod gruntem, podłoga pasa stopniowana między kawałkami; plugin `underground` (`site`,
rodzaje, plik mapy, narzędzie Shift+T, `Courses`), `world.Unseen`, tuba instancjami z własnymi
lampami, wycięcie gruntu nad wylotem (znaki w wolnych kanałach kratownicy, `U.Mouths`), klatka
kamery wolna w tunelu (kamera jadąca nie wymagała zmian). `routes.golden` bez zmiany.
