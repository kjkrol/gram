---
name: gram-perf-navigation
description: Wydajność nawigacji grama — A* hpastara, pamięć tras po przejezdności planszy, pole grupy i kolumna, hierarchia HPA* w budżecie pracy, wycena kroków w udziałach; niezmienniki, pułapki dynamiki tłumu i pomiaru, odrzucone pomysły; benchmarki Navigation_*, Routing_*, PathFinder_*; skill hpastara czytany ścieżką. Czytaj przy zmianie plannera, routingu, pola, formacji, hierarchii, keepingów, Mobility, Kind.Pace albo hpastara, gdy zatłoczony tick albo trasa zwolniły, i przed planowaniem szukania dwukierunkowego.
---

# gram-perf-navigation

Protokół i pułapki pomiaru: `gram-perf`; jak nawigacja działa: `gram-plugins` (wpis
`navigation`). Liczby: `BENCHMARKS.md` („Navigation", „The routing alone", profile); plan i
wynik etapów: `doc/plans-perf.md` (Etap 2, Fale 3–5, „Udziały tempa", „Kolejka i pole grupy",
„HPA*"). Solver: `/home/karol/CODE/hpastar/.claude/skills/hpastar/SKILL.md` (stąd czytany
ścieżką) i `BENCHMARKS.md` hpastara.

## Model

- Zatłoczony tick to A* (solver, `leveledFrom`, `price`) i odczyty siatki pod nim; otwarty to
  ruch po trasach (`traffic`).
- Prośba o trasę: odpowiedź pamiętana, dla partii (≥ `fieldParty` rozkazów jednego MoveTo)
  marsz po polu grupy, na końcu szukanie; `March` układa partię w formacje nad polem. Trasa
  dalsza niż `Hierarchy.From` wspina się po hierarchii hpastara, a kawałek schodzi po drzewach
  od wejść bez szukania (użytkownik, 2026-10-29: A* tylko na poziomie abstrakcyjnym).

## Niezmienniki

- **Co zmienia to, co szukanie czyta, liczy się w `Board.Passage` przed następnym szukaniem**
  (użytkownik, 2026-10-29: plansza oddziela zmiany przejezdności od reszty). Odpowiedzi
  pamiętane, pola, `flatStep`, `terrainSeen` i hierarchie trzymają się `Passage()` i
  `CellPassage(c)`, nie `Version()`: kołysanie lasu, śnieg i przemalowanie nie gaszą pamięci,
  lód, mur, most i kształtowanie reliefu tak.
- Rzut przejezdny komórki liczy grunt pod drogą osobno, bo `Bare` wycenia krok skośny obok
  drogi. Krok czyta sąsiadów (`cutOff`, `Fenced`/`DecksOver`, `Rise`), więc nieaktualność
  klastra obejmuje pierścień jednej komórki: krok czytający dalej musi go poszerzyć.
- **Plansza bez limitów płaci gałąź na krok**: `p.limited` czytane raz w `begin`, `bears`
  inline; limity komórki czytane raz na szukanie w bitach `read.has`, komórki z limitem pytane
  za każdym razem.
- Trasa raz zaplanowana jest śledzona i planowana na nowo tylko, gdy pęknie.
- `routes.golden` stoi na remisach hpastara (jak `container/heap`): ich zmiana przepisuje go z
  powodem i zmienia dynamikę tłumu.
- **Udziały** (decyzje użytkownika, 2026-10-28): waga kroku to udział tempa maksymalnego w
  [0, 1], 0 odmowa, 1 pełne tempo i nic szybszego; czynniki `cell.Mobility` gry się mnożą
  (neutralne 1), plansza wypełnia `Step`. Koszt = długość / udział, h = odległość: dopuszczalna,
  bo żaden krok nie kosztuje mniej niż jego długość.
- **Jedna miara stoku**: planner, tempo i jazda czytają tę samą strategię ze stokiem do sąsiada
  w kierunku marszu (krok prowadzony to `unit.At.Next`), więc co planner dopuścił, jednostka
  idzie, a odcięcie nie zatrzymuje jej w pół kroku.
- Udział gruntu na płaskim czyta się raz na komórkę i domenę (`flatStep`); strategię na krok
  woła się tylko nad reliefem, bramka ustępowania nad płaską planszą wcale (`Sloped()`).
- Ustępowanie (`YieldShare`) to stosunek udziału kroku do udziału na płaskim, nie próg
  bezwzględny: płaski las wchodzony, klif nie, w każdej strategii.
- Zacięcie liczy się od prawnego tempa: keeping żąda postępu `min(ćwierć ciała,
  MaxSpeed·Share·StallAfter/4)`, więc wolny, legalny marsz po stoku nie gubi rozkazu.
- **Pole == trasa**: koszt komórki w polu to koszt trasy `Find` z niej (1e-4), bo wypełnienie
  wycenia krok n → t tym samym `step`, co `leveledFrom` z n. Pole nie pomija rodzica jak A*,
  więc bywa tańsze, nigdy droższe. Partia poniżej `fieldParty` szuka jak bez pola.
- Liczność partii wpisuje się w rozkaz przy wydaniu (`MoveOrder.Party`); liczona mapą po żywych
  rozkazach co Update kosztowała każdy rung bez pola.
- **Hierarchia nie zmienia tłumu**: krótkie trasy, `FindAround`, partie i kandydaci
  `NearestFree` idą jak bez niej. Próg `From` to czysta funkcja pytania (klucz odpowiedzi
  pamiętanych zostaje); budowa i `Stale` jednym finderem przed prośbami, prośby przez `Search`
  każdego findera, funkcje `Layout` czyste.
- Jedna hierarchia na domenę, klasy hpastara z rodzajów jednostek: krok niesie `Refuses`, część
  niezależna od ciała czytana raz, szybka ścieżka tam, gdzie `bodily` jest fałszywe; domena
  jednej klasy buduje się po krokach swojego ciała. Klasa odmawia każdego kroku z węzła, na
  którym nie stoi, bo etykiety łączą w obie strony, a woda na granicy klastra łączyła brzegi;
  `At` wymienia węzły, na których stoi którakolwiek klasa. `Backward` jest lustrem `Forward` co
  do bitu, maski też.
- **Budowa liczy kroki pracy, nie czas** (`buildBudget`): gotowość zależy od ciągu ticków, nie od
  maszyny. `Sweep` buduje jedną hierarchię: niegotową, o którą proszono najpóźniej.
- `Climb` sam odmawia pary bliższej niż dwa boki klastra (`WithNear` tak samo): hpastar
  szukałby jej w słowniku `Size` × 5 B każdego `Search`.
- Zero alokacji po rozgrzaniu: A* do trzymanych buforów, keepingi, formacje, ponowne
  wypełnienie pola (nowe pole bierze miejsce ostatnio zwolnione w `spacing.Fields`).

## Pułapki

- **Dynamika tłumu przesuwa czas bez śladu w profilu.** `InTheWay` pytające o `Ahead` także
  obcych dało `crowded,100` +28%: uderzeni od tyłu przestali uskakiwać, korek trwał dłużej,
  zablokowani szukali częściej. Znalazła to bisekcja po commitach i profil, nie czytanie kodu.
- W korku to samo pytanie wraca: marsz po polu musi być odpowiedzią pamiętaną, inaczej przegrywa
  z bazą, która odpowiada z pamięci.
- Remisy Dijkstry od celu są inne niż A* (te same koszty, inne komórki): grupa idzie jednym
  drzewem. Daleki cel małej partii wypełnia duży dysk, jednostka odcięta całą osiągalną planszę.
- Kolumna: koszt pola skokowy na granicy komórek odwracał porządek sąsiadów (stąd ciągły
  `costAt`); „zamknięcie" luki mierzone żywotnością marudera nie kończy czekania czoła; tempo za
  kimś obok (półpłaszczyzna zamiast stożka ćwierci obrotu) zamraża przód; stojący członek bez
  `Kept` rwie łańcuch podążania (`Blocks → Detour` wokół własnego czoła).
- `Read.Built` bramkuje `t.built`, ustawiane po dotknięciu, więc rzut czyta `Built` ze store'u.
  Flats routerów czyści się co Update: klucz przejezdności trzyma tylko finder traffic.
- Jednokierunkowe kroki między węzłami stojącymi dalej zawyżają `Reaches`.
- Para bliska przez granicę wysokiego poziomu, szukana na najwyższym poziomie, na którym klastry
  się różnią, robiła objazd do jego wejść: hpastar wybiera poziom po odległości.
- Koniec budowy to `Building()` albo `left == 0`, nie tick bez ukończonego klastra. Budżet 0 po
  stronie grama nie buduje nic, `Update(0)` hpastara buduje wszystko.
- Zmiana poza najtańszymi drogami między wejściami klastra przebudowuje się w ticku zmiany,
  zmiana kosztów wyżej nie, i trasa idzie wtedy A*. `refill` rodzica kosztuje tyle co budowa.
- `into` wypełnienia zostaje własną pętlą, a pola hierarchii stoją na końcu `Finder`: komentarze
  przy nich mówią, ile kosztowało inaczej.

## Odrzucone

- Pasy: jednostka nie widzi innych z wyprzedzeniem (reguła domu w `CLAUDE.md`).
- Kopiec 4-arny (skill hpastara): zmienia remisy, a tłum w gramie przeplanowywał o 55% więcej;
  reżimy wołającego też się liczą (`gram-perf`, „Zysk").
- Arytmetyka następników i porównania słownikowe w SIMD: callback heurystyki na następnik, brak
  gather (`BENCHMARKS.md` hpastara, „Tried and dropped").
- `Way` liczone na każde ciało w `gather`: open wolniej; pisze go `tail`, który i tak liczy hypot.

## Pomiar

- `Routing_*` (`plugins/navigation`) i `PathFinder_*` (`internal/pathfind`) leżą poza `bench/`:
  `measure.sh` ich nie widzi, mierzy się je ręcznie tą samą pętlą.
- `Navigation_Tick` daje każdemu piechurowi własny `MoveTo`; tłum jednym rozkazem to rungi
  `grouped` i `column`. Podążanie, kolumnę i zacięcia mierzy sonda płaskowyżu
  (`doc/plans-perf.md`, „Kolejka i pole grupy"), nie benchmark.
- `Touch(0)` w benchmarkach routingu przebudowuje na op klaster 0 i skanuje `CellPassage`; B/op
  przy krótkim benchtime to słowniki A* z pierwszego szukania każdego `Search`.
- Mała hierarchia buduje się w pierwszym ticku: „bez hierarchii" to świat z `Hierarchy{Off}`.
  Przy tempie ×4 klatka niesie cztery budżety budowy.
- Liczniki (szukania, marsze, wypełnienia, rozwinięcia, wspinaczki): `Read.Counts()`.
- `PathFinder_Hierarchy/build/{classes=4,solvers=4}`: jeden Solver czterech klas wobec czterech
  Solverów jednej; `find/classes=4` to wspinaczka pytana jako ostatnia z czterech klas.

## Na później

- Szukanie dwukierunkowe: cena zależna od kierunku (rodzaj docelowy, znak stoku) wymaga wyceny
  wstecznej w lustrze, stop przy najlepszej trasie ≤ mniejszym z najmniejszych F frontów; zysk
  najwyżej ~2× przy drogim gruncie, żaden przy ciasnej heurystyce i w `NearestFree`/`FindAround`.
- `Best(d)`: plansza, której najlepszy kafel ma udział 0,4, mogłaby liczyć h = 2,5 × odległość;
  na Maderze drogi dają 1.
