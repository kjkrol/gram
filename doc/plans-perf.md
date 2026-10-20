# Plan: wydajność ticku i klatki, skonsolidowany

Zastępuje kolejność z `plans-gc.md`; zadania stamtąd wchodzą tu jako etapy albo ich części i są
cytowane po numerze (`plans-gc Z4`), bo ich kroki z numerami linii nadal obowiązują. Plik
roboczy, nie część dokumentacji biblioteki. Odwołania do linii sprawdzone na gram `51da9307`,
goke `a6b663b` (v3.5.0), aabbworld `4889c6e` (v1.12.0).

Źródła: `BENCHMARKS.md`, profil CPU `Benchmark_World_Tick/entities=5000` z 2026-10-18 (poniżej),
przegląd systemów i renderera z tej samej sesji, `plans-gc.md`.

## Co wiadomo na starcie

Tick świata przy 5 000 encji, 259 µs na maszynie pomiarowej, 1 alloc, bez śladu GC w profilu:

| Gdzie | Udział |
|:--|--:|
| `motion.moveSystem.rebuild`: kopia Base do `aabbworld.Item`, `Grid.Rebuild`, `remember` | 35% |
| `motion.clampStep`, w tym `math.Hypot` (`math.archHypot` 17% samo) | 26% |
| `aabbworld.Space.Move`: `Surface.Translate`, `move`, `fold`, `wrap` | 25% |
| iteracja goke, `Velocity.Delta`, `Position.MaxStep` | ok. 10% |

`Grid.index()` w tym benchmarku nie biegnie ani razu, bo nikt nie pyta przestrzeni; w grze
odpala je odświeżenie widoku raz na tick, ok. 20 µs (`BENCHMARKS.md`, "Drawing"). Wniosek:
tick nie jest problemem GC, tylko trzech miejsc CPU. Klatka ma inne centra kosztu
(`BENCHMARKS.md`: wyspa 1,8-4 ms, cienie i brzegi liczone na nowo 4-5 ms, encje 0,4-0,65 ms)
i tam liczby per miejsce dopiero trzeba zebrać (etap 0).

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
ruchu (migracja przy każdym zatrzymaniu i ruszeniu; `BENCHMARKS.md` "Markers" mówi, ile to
kosztuje), albo rozbicie `Base` tak, by stojąca encja nie miała `Velocity` (duża zmiana,
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
scalenie fast-forward, pomiar na bezczynnej maszynie. Liczby w `BENCHMARKS.md`, sekcja „The
island's frame”.

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
na przemian. Liczby w `BENCHMARKS.md`, „The island's frame”, akapit „In rings”.

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
`BENCHMARKS.md`, „The island's frame”, akapit „The ground at the camera's resolution…”.

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
`BENCHMARKS.md`, „The island's frame”, akapit „The picture in layers from one composition…”.

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

**Stan** (integrator aktualizuje, 2026-10-20 22:00): fala 1 zrobiona przez pięć agentów Opus 5.5
w worktree (na życzenie użytkownika „możesz zaczynać”, zamiast sesji). 2a i R1 + dziura z R2
scalone do `main` (`aecf9f9c`, `9b90af6d`); 3a i 4a zweryfikowane na swoich gałęziach i czekają
na przegląd, scalenie i wydanie użytkownika (aabbworld `feat/runs` → v1.14.0, goke
`feat/chunk-versions` → v3.6.0, oba z podbiciem `gram/go.mod`); drugi commit R2 (`444c40a7`,
Directy umieszczane) nie scalony, decyzja niżej. Następne: 2b (od `main`, w toku),
3b po podbiciu aabbworld v1.14.0 w `go.mod` (wydane 2026-10-20, podbicie użytkownika). Goke PR #55
niescalony z decyzji użytkownika: 4b i 5 idą razem lokalnie na goke `feat/chunk-versions`
(brief `45-mirror-local`, gałąź `m/mirror`); goke v3.6.0 tylko, jeśli lustro pokaże zysk. Gałęzie części zostają do przeglądu,
worktree gram usunięte.

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
