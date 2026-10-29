# Etap 4: marsz w formacji — kolumna nad polem (2026-10-28)

Gałąź `feat/column` od `main` (v0.9.0), worktree `/home/karol/worktrees/gram/column`. Plan:
`doc/plans-perf.md` „Kolejka i pole grupy" (etap 4) i „: wynik" etapów 1–3; zasady wspólne:
`doc/briefs/README.md`. Skille przed kodem: `gram-plugin` (komendy, fakty, słownictwo),
`gram-plugins` (navigation: pole, partia, podążanie), `gram-perf-navigation` (pole, kolejka),
`ceremony` (fakty dla planów). **Bez pomiarów czasu** w części.

## Cel

Marsz grupy w formacji, pierwsza formacja to kolumna (gęsiego): porządek z planu, nie z listy
ani z postrzegania — nikt nie wyprzedza, każdy idzie tempem poprzednika, a czoło, jeśli rozkaz
każe kolumnie być zwartą, czeka na ogon. Komenda jest nośna dla kolejnych kształtów (linia,
klin) bez zmiany szwów. Szerokość czoła wynika z terenu: pole, zajętość i podążanie z etapu 2
już to robią; kolumna dokłada porządek i tempo.

## Decyzje użytkownika (2026-10-28)

- Nowa komenda, nie pole w `MoveTo`, i taka, która uniesie wiele formacji z myślą o rozwoju.
- Zwartość i luz kolumny to parametry rozkazu: gra mówi, czy kolumna ma być zwarta i czekać,
  czy wolno jej się rwać.
- Etap teraz, agentem; integrator mierzy i sonduje.

## Kontrakty (nazwy proponowane; w raporcie wskazać, gdzie nazwa nie pasuje do kodu)

- **Komenda** `March` (`internal/order/commands.go`, re-eksport w `plugins/navigation/order.go`
  i `commands.go`): `March{Cell cell.ID; To ceremony.Whom; At geom.Vec; Level Level; In
  Formation}` — „March sends the Selected entities the player owns, or the entity giving it, to
  Cell in a formation: as MoveTo, the group laid out and paced by In." Gracza (jak `MoveTo`,
  przez `Queues()`) albo własna encji. Bez domyślnego klawisza (gra wiąże własny; `MoveTo`
  zostaje prawym przyciskiem). `Append` nie w tej części (dopisać do „poza zakresem").
- **Formacja** (`internal/order/formation.go`): `Formation{Shape Shape; Spacing float32; Closed
  bool; Slack uint8}` — zapisywany wartością (bez interfejsu, bo rozkaz jest zapisywany polami):
  `Shape` (`uint8`): `NoFormation` (0), `Column` (1); kolejne kształty to nowe stałe i kod w
  koordynatorze, nic w komendzie. `Spacing`: odstęp między członkami wzdłuż formacji w ciałach
  (1 = ciało i szczelina `gapFor`; 0 czyta się jako 1). `Closed`: formacja zwarta — czoło czeka,
  gdy ogon odstaje o więcej niż `Slack+1` odstępów; `Closed` fałsz: rwanie dopuszczone, czoło
  nie czeka (członkowie i tak idą tempem poprzednika). Konstruktor `ColumnOf(spacing float32,
  closed bool, slack uint8) Formation` w pakiecie `navigation` (re-eksport). Kształt nieznany
  pluginowi: rozkaz wydany jak `MoveTo`, raz w logu świata.
- **Rozkaz** (`internal/order/order.go`): `Formation Formation` (zero = brak); `AheadOf
  uid.UID64` („the member just ahead of it in the formation's order; 0 for the head or out of a
  formation"), `Lag float32` („how far behind AheadOf it is, in the field's cost"), `TailLag
  float32` („on the head alone: how far the last member lags behind it"). Zapis rośnie: stary
  zapis nie wczyta się (linia CHANGELOG integratora).
- **Wydanie** (`internal/routing/system.go` `carryOut`): `March` idzie tą samą drogą co `MoveTo`
  (członkowie, `nextGroup`, `Party`, miejsca z `Orders`), a każdy rozkaz dostaje `Formation`.
  `Key` pola kwalifikuje prośbę także wtedy, gdy rozkaz ma formację (bez progu `fieldParty`:
  formacja potrzebuje pola do porządku; partia 2 też).
- **Porządek** (`internal/routing/coordinator.go`): w przebiegu zbierającym prośby (albo raz na
  Update, gdy jakiś rozkaz ma formację i pole stoi) koordynator sortuje członków partii z
  formacją po `Field.Cost` komórki każdego (poziom z `Feet`; komórka poza polem: na koniec) i
  pisze `AheadOf`, `Lag` (różnica kosztów do poprzednika) i na czole `TailLag` (koszt ostatniego
  minus koszt czoła). Gdy pola nie ma (plansza za duża, finder widzący): formacja zdegradowana
  do zwykłego marszu, raz w logu. Koszt: sortowanie partii (≤ setki) co Update z formacją;
  zero alokacji po rozgrzaniu (slice'y w koordynatorze).
- **Tempo** (`internal/spacing/body_keeping.go` i `keeping.go`, `Steer`): członek z `AheadOf`
  ≠ 0 i `Lag` poniżej `Spacing` w jednostkach kosztu (odstęp w ciałach × `(2·half + gap)` /
  dłuższy bok komórki, przy udziale 1) ogranicza prędkość chcianą do prędkości `AheadOf` wzdłuż
  swojej drogi (jak `trail` z etapu 2: wyszukanie w indeksie ciał, `along/Share`), bez dotyku i
  bez granicy czasu (porządek to plan, nie styk); poza odstępem idzie pełnym tempem (nadrabia).
  `Watch`/`Blocked`: postęp wymagany jak przy podążaniu (`min(własne, tempo AheadOf)`), a czekanie
  na `AheadOf`, który sam czeka albo idzie, nie jest zacięciem (ten sam `trailing` z `index`:
  `AheadOf` żywy); czoło bez `AheadOf` jak dziś.
- **Czoło** (`traffic/system.go` `head`, przed `tail`): rozkaz z formacją `Closed`, `AheadOf ==
  0` i `TailLag > (Slack+1) × odstęp w koszcie` → trzyma (`Holding` jak `Hold`, bez `WaitedOut`,
  odnawiane co Update, bez zacięć), aż ogon nadrobi; prędkość chciana 0, głowa bez zmian.
- **Fakt** (opcjonalnie, jeśli tanie): `InFormation{Ahead uid.UID64; Lag float32}` dla planów
  (umysłów), ze słowami `Lagging()`, `Leading()`; słownictwo w `doc.go`.
- **Słownictwo** `plugins/navigation/doc.go`: komenda `March`, `Formation`, `Column`, fakt.

## Pliki

Wolno: `plugins/navigation/internal/order/{commands.go,formation.go (nowy),order.go,facts.go}`,
`internal/routing/{system.go,coordinator.go}`, `internal/spacing/{body_keeping.go,keeping.go}`
(tylko `Steer`, `Watch`, `Blocked`), `internal/traffic/{system.go (head: czoło; told.go nie),
facts.go}`, `plugins/navigation/{order.go,commands.go,doc.go}`, testy pakietu `navigation`
(nowy `column_test.go`), `bench/navigation_test.go` (rung `column,entities=1000`: tłum jednym
`March` kolumną zwartą). Szwy nie do ruszenia: pole i `Fields` (poza `Key`), dotyk i reguły
tłumu, planner, `unit.At`, dema, `MoveTo`.

## Kroki

1. `Formation`, `March`, pola rozkazu, re-eksporty, `doc.go`.
2. Wydanie w `carryOut`; `Key` z formacją; test: `March` dla 5 zaznaczonych daje każdemu
   rozkaz z formacją, partią i polem (`Fills == 1`).
3. Porządek w koordynatorze; test: 6 członków w rzędzie w korytarzu → `AheadOf` łańcuchem w
   kolejności kosztu, `Lag` dodatnie, `TailLag` na czole; usunięcie środkowego (despawn) →
   porządek reszty bez dziury po następnym Update.
4. Tempo w `Steer` i zacięcia; test: kolumna 6 (ciała) za wolnym czołem na pasie 0,01 jak w
   etapie 2: prędkości członków ≤ poprzednika, `Stalls` 0, nikt nie wyprzedza (kolejność
   dojścia = kolejność startu).
5. Czoło zwarte; testy: `Closed` z `Slack` 1: ogon zatrzymany (`halted` z rigu etapu 2) →
   czoło staje w ciągu sekundy i rusza, gdy ogon nadrobi; `Closed` fałsz → czoło nie czeka.
6. Kolumna 10 przez korytarz jednej komórki (komórki + kolizja, `newRoadWorld`): dochodzą
   wszyscy, w porządku startu; dwie kolumny w jednej przełęczy (dwa `March`) dochodzą obie.
7. Rung `Navigation_Tick/column,entities=1000` (dodać, nie mierzyć); sekwencja, `-race`, SIMD,
   `golines`; commity na `feat/column` z trailerem.

## Poza zakresem

Kształty inne niż kolumna (nazwy stałych rezerwowane: `Line`, `Wedge`); `Append` do `March`;
klawisz domyślny; rysowanie formacji; zmiany pola; pomiary (integrator: `Navigation_Tick`
wszystkie rungi, `Routing_Group`); sonda (integrator: 30 górali jednym `March` kolumną zwartą
i rwaną: porządek na szczycie, czas ostatniego).

## Raport

Commity; odstępstwa od briefu i dlaczego; czego nie zrobiono; ryzyka (koszt a odległość:
`Lag` w koszcie rośnie na wolnym gruncie; członek poza polem; czoło trzymane w lejku, gdy ogon
nie może nadrobić; dwie formacje tej samej partii po `Advance`); proponowane wpisy do
`CHANGELOG.md` (temat v0.10.0), `doc/behaviour.md` (komendy), skilli `gram-plugins` i
`gram-perf-navigation`; polecenia benchmarków.

## Runda 2 (2026-10-29): kolumna w tłumie ciał pod kolizją

Runda 1 (commity `b07dc611`…`76f88d45`) przechodzi testy, ale sonda płaskowyżu (30 górali
board/topography z (66–71, 48–52) na (115, 61), seed 7, spokojna maszyna, drzewo `feat/column`)
pokazuje, że kolumna nie przechodzi ujścia krawędzi, które `MoveTo` od etapu 2 przechodzi:

- `MoveTo`: 30 z 30, ostatni 291 s (jak v0.9.0).
- Kolumna zwarta, `Slack` 1 (próg czekania 2·`Room` = 11,5 j.; ciało 2,88, `Room` 5,76,
  hamowanie 3 j. przy pełnym tempie): 1 z 30, wszystkie rozkazy stracone do 620 s. Czoło czeka
  od startu (luka 12–17 j.), grupa pełznie pulsami (9 komórek w 60 s), w ujściu (80–81, 55)
  członkowie tracą rozkazy od 99 s, po jednym co 3–5 s.
- Kolumna zwarta, `Slack` 4: 0 z 30 do 900 s, 20 rozkazów żywych: od ~86 s cała grupa stoi
  w ujściu („walking 0" przez 30 s, zacięć brak): czoło 103 na (82,55) czeka (`Gap` 32,
  `Closing` prawdziwe, `Patience` 1,0 s nie biegnie), 107 (miejsce 2, `Lag` 32) stoi z
  `WantSpeed` 0, bo podąża (`Trail`) za 113, który jest za nim w kolumnie; `TrailFor` odnawiane
  (0,5), `Trailed` rośnie do 29 s.
- Kolumna rwana: 14 z 30 do 900 s, 8 rozkazów straconych; kolejne czoła stoją na (82,56) z
  `WantSpeed` 0, czoło zmienia się co kilkanaście sekund.
- Oś czasu zacięć (zwarta, `Slack` 1; sonda `column_probe2.go`): 109 (miejsce 4, `AheadOf`
  103 = czoło stojące w ujściu, `Lag` 6,6) na (80,55)→(81,55), `v 0`, `WantSpeed 0`,
  `TrailFor` 0: `Stalls` 1, 2, 3, 4 co sekundę (81,7–84,9 s); 97 (miejsce 5, `Lag` 7,0–7,1)
  tak samo do 5; 120 (miejsce 6, `Lag` 6,9–7,3) do 5 i rozkaz stracony (99,3 s); potem każdy
  kolejny na miejscu 6 (114, 102, 108, 119, 101, 113) ginie po 4–5 s.

Diagnoza:

1. **Zasięg tempa za krótki, `Lag` zawodny przy styku.** Członek dotykający poprzednika
   (`clearOf` → prędkość 0) ma `Lag` 6,6–7,3, choć ciała w styku dzieli ≤ 2,88: koszt przez
   granicę komórek o udziałach 0,31 → 0,05 zawyża odległość. `pacer` sięga `Room` + hamowanie
   ≈ 6,4, więc członek nie jest tempowany; nie podąża (`WalksMyWay` fałsz: stojący poprzednik
   ani `Along`, ani `Trailing`); liczy zacięcia jak samotnik w korku i po pięciu ginie; reguła
   `Blocks → Detour` każe mu obchodzić własne czoło.
2. **`Closing` nie znaczy „zamyka lukę".** Żywotność (prędkość albo `Trailing`) w węźle
   wzajemnych podążań jest wieczna: maruder stoi z flagą `Trailing`, czoło czeka bez końca.
3. **Stojący członek formacji pęka łańcuch etapu 2.** Czoło czekające i członek tempowany za
   stojącym nie są dla dotyku ani `Along`, ani `OtherTrailing`: tłum za nimi obchodzi
   (`Detour`), wyprzedza (zmiana czoła), zacina się.
4. **Czekanie czoła nie zwalnia formacji z zacięć** poza zasięgiem `pacer`.

Kontrakty rundy 2 (mechanika do decyzji agenta tam, gdzie nie zapisano; nazwy proponowane):

- `Closing` = luka maleje: porównanie z poprzednim ułożeniem (ε), nie żywotność marudera.
  Czoło rusza po `Patience` od chwili, gdy luka przestała maleć, i czeka znów, gdy znów
  maleje. Czekanie czoła jest skończone zawsze.
- Członek trzymany przez formację (tempowany za poprzednikiem albo czoło czekające) jest „w
  formacji" dla reszty: dla dotyku jak podążający (`OtherTrailing` prawdziwe, by
  `WalksMyWay && Groupmate → Trail` grało zamiast `Blocks → Detour`), dla `Body.Alive` żywy.
  Np. pole `Kept` na rozkazie pisane przez keeping (tempowany) i traffic (`awaits`), czytane
  w `gather` do `Body`.
- Formacja czeka jako jedna: koordynator pisze na każdym członku stan czoła z poprzedniego
  ticku (`HeadWaits`); `Watch` i `Blocked` nie liczą zacięcia, póki czoło czeka (czekanie
  czoła skończone, więc zwolnienie też).
- Zasięg tempa: tempowany także w styku z poprzednikiem (`Touching`/`PressedBy`) albo przy
  `Lag` < k·`Room`, k ≥ 2 (`keepPace` i tak hamuje do tempa poprzednika na `Room`); `Lag` dla
  ciał w styku przez granicę komórek o różnym udziale ≤ ciało + szczelina (sprawdzić `costAt`
  i „steeper of the two").
- Kolumna rwana dochodzi tymi samymi mechanizmami bez czekania czoła.

Akceptacja (sonda, seed 7, spokojna maszyna, dwa przebiegi każdego wariantu): `MoveTo` 30 z 30
jak dziś; kolumna zwarta `Slack` 1 i rwana: 30 z 30, żaden rozkaz nie stracony, ostatni ≤
600 s; max `Stalls` i inwersje porządku dojścia (względem miejsc z początku) raportowane;
testy etapów 1–3 i rundy 1 zielone (test rundy 1 wolno zmienić, gdy kontrakt się zmienił:
powiedzieć który i czemu), `-race`, rung bez zmian. Sonda: z katalogu `/home/karol/CODE/gram`
`PROBE_TREE=/home/karol/worktrees/gram/column PROBE_SEED=7 PROBE_CLOSED=1|0 PROBE_SLACK=1
[PROBE_MARCH=0] .claude/skills/gram-probe/probe.sh examples/board/topography <probe.go> 300
<workdir>`; `<probe.go>` to kopia `column_probe2.go` ze scratchpadu integratora (ścieżka w
wiadomości) w katalogu poza repo; linie `n 30 ordered …`, `STALL`, `ORDER GONE`, `END`,
`arrival`. Pułapka: wynik zależy od obciążenia maszyny (skill `gram-probe`, Traps): w czasie
przebiegu nie budować i nie testować.
