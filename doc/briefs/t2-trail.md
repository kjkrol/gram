# Etap 2: kolejka — słowo przechodnie i podążanie (2026-10-28)

Gałąź `feat/trail` od `main` (po v0.7.1), worktree `/home/karol/worktrees/gram/trail`. Plan:
`doc/plans-perf.md` „Kolejka i pole grupy" (etap 2) i „: wynik" etapu 1; zasady wspólne:
`doc/briefs/README.md`. Skille do przeczytania przed kodem: `gram-plugin` (momenty, słowa,
komendy, fakty), `gram-plugins` (wpisy navigation i board), `ceremony` (reguły, `cond`, `act`),
`gram-perf-navigation` (gorące ścieżki keepingów; **bez pomiarów** w tej części).

## Cel

Współtowarzysz rozkazu, który idzie przede mną w moją stronę albo czeka w kolejce za kimś, kto
idzie, nie jest przeszkodą do obejścia, tylko kimś, za kim się idzie. Dziś dotyk takiego daje
słowo `Blocks` (`Touch.Moving` znaczy „ma rozkaz", nie „porusza się") i regułę `Detour`:
obejście bokiem, a gdy bramka `YieldShare` odrzuca strome komórki, stanie w miejscu, brak
postępu, zacięcie, po pięciu rozkaz gaśnie. Sonda: 30 górali posłanych jednym `MoveTo` z
płaskowyżu (board/topography) traci 27 rozkazów w kolejce na krawędzi w 51–71 s gry. Po tym
etapie kolejka za pełzającym zgodnie z prawem liderem nie liczy zacięć, a ogon idzie tempem
prowadzonego.

## Kontrakty (nazwy zamknięte przez użytkownika)

- **Słowa dotyku** (`internal/order/touch.go`). Nowe pola `Touch`: `Ahead bool` („Ahead says
  Other lies the way Self faces."), `Along, SameWay bool` („Along says Other moves the way Self
  faces, SameWay that it faces that way too."), `OtherTrailing bool` („OtherTrailing says Other
  trails someone itself."). Słowo `WalksMyWay()` („walks my way"): `Moving && !GivingWay &&
  OtherMoving && !OtherGivingWay && Ahead && (Along || OtherTrailing && SameWay)` — doc: „the
  one in Self's way goes Self's way: under an order, not giving way, ahead of it and moving
  along its facing, or trailing someone and facing its way". **Przechodnie**: stojący w kolejce
  liczy się przez `OtherTrailing`; `SameWay` tnie parę nos w nos, która odnawiałaby się
  nawzajem. Słowo `Groupmate()` („groupmate"): `t.Groupmate` — „one MoveTo sent the two".
- **Liczenie pól** w `traffic/system.go` `fire` z dwóch `bodies.Body`: `Ahead: Way·self.Facing
  < 0` (`Way` to droga, którą Self opuszcza Other, więc Other leży pod −Way; krok odmówiony:
  `wayBetween(why.cell, w.at.Cell)` ma tę samą konwencję), `Along: other.Vel·self.Facing > 0`,
  `SameWay: other.Facing·self.Facing > 0`, `OtherTrailing: other.Trailing`.
- **Komenda** `Trail{Of uid.UID64}` (`internal/order/commands.go`, `host.Aimed` jak `Detour`,
  `Aim`, `Command()`, oba `var _`): „Trail has a unit on the move follow Of, who goes its way:
  no faster than they go, its wait no stall for as long as they lawfully need to clear a cell."
  Re-eksport w `plugins/navigation/order.go` (`Trail`, `WalksMyWay`, `Groupmate`), słownictwo
  w `plugins/navigation/doc.go` („# Crowd" i „# Vocabulary": komenda własna encji, celowana w
  podmiot dotyku). `traffic/told.go`: kolejka `trails command.Queue[order.Trail]` w `All()` i
  `drain` (`t.trail, t.trailOf`). `carryOut` (`system.go`): **ostatni** przypadek `t.trail`
  (najłagodniejsza odpowiedź: stop, aside, detour, pass, hold, settle wygrywają tick, który
  dzielą): `other, ok := s.keep.Other(t.trailOf)`; `if o.TrailFor == 0 { o.Trailed = 0 }` (nowy
  ogon liczy czekanie od nowa, odnowienie je zachowuje); `o.Trailing, o.TrailFor, o.TrailPace =
  other.ID, spacing.TrailMemory, other.MaxSpeed*other.Share`. `head`: `o.TrailFor =
  max(o.TrailFor-d, 0)` obok `Cooldown` i `Retry` (biegnie też dla trzymanych, więc pamięć nie
  zalega na ścieżkach `stood`). **`Trail` to nie `Hold`**: `Hold` zatrzymuje i omija `tail`;
  `Trail` to stan rozkazu czytany przez `Steer` i `Watch` na zwykłej ścieżce `onward`.
- **Rozkaz** (`internal/order/order.go`), po `Cornered`: `Trailing uid.UID64; TrailFor, Trailed
  time.Duration; TrailPace float32` z komentarzem: „Trailing is whom the entity follows for
  TrailFor more, no faster than they go; TrailPace is the ground speed they lawfully make,
  Trailed how long it has followed them without headway."
- **Ciała** (`internal/bodies/bodies.go`): `Body` dostaje `Share`, `MaxSpeed float32` (jak w
  `Member`, etap 1) i `Trailing bool` („follows someone, for a while yet"); `Body.Member()`
  niesie `Share` i `MaxSpeed`; `gather` w `system.go` czyta kolumny `steer` i `pace` (`Share` 1
  bez kolumny; `pace` jest już opcjonalną kolumną zapytania po etapie 1) i ustawia `Trailing =
  o.TrailFor > 0` w bloku `orders != nil`.
- **Keepingi** (`internal/spacing/`). Stałe w `body_keeping.go` obok `StallAfter`:
  `TrailMemory = 500 * time.Millisecond` („TrailMemory is how long a unit goes on trailing one
  it last struck.") i nieeksportowane `maxTrailWait = 3 * time.Second` (najdłużej, ile czekanie
  na jednego prowadzonego nie jest zacięciem). Pomocnik `trailing(o, cellSpan) bool`:
  `o.TrailFor > 0 && o.Trailed < wait`, `wait = maxTrailWait` gdy `TrailPace <= 0`, inaczej
  `min(cellSpan/TrailPace sekund, maxTrailWait)`; `cellSpan` to krótszy bok komórki siatki.
  - `BodyKeeping.Steer`, przed blokiem aside: gdy `o.TrailFor > 0` i indeks ciał zna
    `o.Trailing`: `along := max(0, b.Vel·u)`, `u` wersor `dir` (albo `m.Facing`, gdy `dir`
    zerowy, jak `drive` podaje); `if m.Share > 0 { speed = min(speed, along/m.Share) }` —
    prośba idzie przed udziałem gruntu (motion mnoży `Vel.Value` przez `Pace.Share`), więc
    dorównanie prędkości gruntowej prowadzonego to `along/Share`; `RequestSpeed` tnie do
    `MaxSpeed`. `clearOf` dalej zatrzymuje, póki ciała się stykają.
  - `BodyKeeping.Watch`: `need` jak w etapie 1, ale `min(m.MaxSpeed*m.Share, o.TrailPace)` gdy
    `o.TrailFor > 0` (postęp wymagany podąża za prawnym tempem **prowadzonego**: inaczej ogon na
    szybkim gruncie za pełzającym zgodnie z prawem zacina się, gdy skończy się czekanie); gałąź
    postępu zeruje też `Trailed`; przed liczeniem: `if trailing { o.Trailed += d; return }`.
    `Bump` bez zmian (`Closest = +Inf` czyni następny `Watch` postępem).
  - `CellKeeping.Blocked`: `Trailed = 0` tam, gdzie `Stalled` (`Toward != c`); `if trailing {
    o.Trailed += d; return holder, known }` przed `Stalled += d`; `CellKeeping.Watch` zeruje
    `Trailed` ze `Stalled`. `Steer` komórek bez czapki. Pod komórkami `Trail` to ograniczona
    grzeczność: najwyżej `maxTrailWait`, potem jak dziś (uczy się komórki lidera i obchodzi).
- **Reguła domyślna** (`plugins/navigation/crowd.go`, `goRound`): w `OneOf` przed `Blocks →
  Detour` gałąź `WalksMyWay && Groupmate → Trail` (złożenie warunków tak, jak `cond` ceremony to
  daje; sprawdzić w skillu `ceremony`). Sojusznik innej grupy i obcy dalej obchodzeni;
  `GoalTaken`, `WaitsFirst`, `NoWayRound`, `ClearsTheWay` zachowują pierwszeństwo.
- **Opcjonalnie** (jeśli tanie): fakt `Blocked` z polem `Trailing bool` i słowem `Queued()`
  („blocked by one going my way") dla planów; zmienia rozmiar `Blocked` (zapis).

## Pliki

Wolno: `plugins/navigation/internal/order/{touch.go,touch_test.go,commands.go,order.go,
facts.go}`, `plugins/navigation/internal/bodies/bodies.go`, `plugins/navigation/internal/
traffic/{system.go,told.go,facts.go}`, `plugins/navigation/internal/spacing/{body_keeping.go,
keeping.go}`, `plugins/navigation/{crowd.go,order.go,doc.go}` i testy pakietu `navigation`
(`body_spacing_test.go`, `crowd_test.go`, `bump_test.go`, `cell_spacing_test.go`, nowy plik
testów). Szwy nie do ruszenia: planner (`pathfind`, `routing`, `spots.go`), board, motion,
jazda, `unit.At`, `Member` poza użyciem `Share`/`MaxSpeed`, format `Touch` poza nowymi polami,
dema w `examples/`. `doc/`, skille, `BENCHMARKS.md`, `CHANGELOG.md`, `go.mod`: integratora
(proponować treść w raporcie).

## Kroki

1. Pola `Touch`, słowa `WalksMyWay`, `Groupmate`; `touch_test.go`: `TestTouch_WalksMyWay`
   (tabela w stylu `TestTouch_TheRulesOfTheCrowd`: przed i wzdłuż → tak; przed, stojący, nie
   podąża → nie; przed, podąża, patrzy w moją stronę → tak; przed, podąża, patrzy na mnie →
   nie; za i wzdłuż → nie; `GivingWay` po którejś stronie → nie; `OtherMoving` fałsz → nie).
2. `Trail`, told/drain, re-eksporty, `doc.go`.
3. Pola rozkazu i ciał, `gather`, iloczyny w `fire`.
4. Zanik `TrailFor` w `head`, przypadek w `carryOut`.
5. Stałe, `trailing`, `BodyKeeping.Steer/Watch`, `CellKeeping.Blocked/Watch`.
6. Reguła domyślna; testy pakietu:
   - `TestBodySpacing_TrailingAsksNoMoreThanTheTrailedOneMakes` (goły keeping jak w
     `crowd_test.go` `NewBodyKeeping(finder, nil, nil)` + `Begin` z prowadzonym `Body{Vel
     (0.96, 0), ...}` w indeksie; `Member` o `Share` 0,5 z rozkazem podążającym: `Steer(east,
     96)` → `WantSpeed == 1.92`; `Share` 1 → 0,96; `dir` północ → 0; prowadzony poza indeksem →
     96) i rozszerzenie tabeli `TestBodySpacing_AStallIsJudgedByTheLawfulPace`
     (`stall_pace_test.go`): podążający z `TrailPace` 1 bez postępu: nie `Bumped` w 2,9 s,
     `Bumped` w 4 s (3 s czekania + 1 s zacięcia).
   - `TestCrowd_AColumnOfGroupmatesTrailsItsLeaderOverSlowGround` (`newFieldWorldWith`,
     `BodySpacing`, pas `cell.Kind{Pace: 0.01, Allows: cell.Land}` na dwie komórki jak w
     `TestBodySpacing_ACrawlOverSlowGroundIsNoStall`, jednostki `paced: true`): trzy w rzędzie z
     jedną `Group` (pole `MoveOrder.Group`): w pierwszych sekundach na pasie `Trailing` drugiej
     = id pierwszej, a trzeciej = drugiej (przechodniość); żaden rozkaz nie gaśnie przed 12 s;
     `Stalls` każdej 0; wszystkie dochodzą do celu (limit ~120 s gry).
   - `TestCrowd_AnAllyOfAnotherGroupIsNotTrailed`: to samo z ogonem w innej grupie: `Trailing`
     ogona stale 0 (obchodzi albo zacina się jak dziś).
   - `TestCellSpacing_AQueueBehindACrawlingGroupmateWaitsItsLawfulTimeThenGoesRound`
     (`newRoadWorld`, komórki + kolizja): jedna komórka drogi `Pace: 0.01`, lider i ogon jednej
     grupy do (9,1): `Trailing` ogona = lider w pierwszej sekundzie i `Stalls` 0 w czekaniu; po
     `maxTrailWait` uczy się komórki lidera, schodzi z wiersza 1 i dochodzi obok celu przed
     liderem. (Jeśli rig komórek nie ma `Pace` na jednostkach, dodać opcję jak `paced` w
     `fieldUnit`.)
   - `TestCrowd_WithNoRulesTheOrderStillEnds` musi dalej przechodzić (bez reguły nikt nie
     podąża, zacięcie zostaje ostatnim słowem).
7. Sekwencja: `go build ./... && go vet ./... && gofmt -l . && go test ./...` (testy GPU:
   `GRAM_GPU=software`), `go test -race ./plugins/navigation/...`, `GOEXPERIMENT=simd go test
   ./plugins/navigation/...`; `golines -m 100 -t 4 --no-reformat-tags -l` na dotkniętych plikach
   (poprawiać ręcznie: żadnych przerw po akcesorze; nie przewijać linii sprzed etapu).
8. Commity na `feat/trail` (np. `feat(navigation): a groupmate going my way is trailed, not
   gone round (WalksMyWay, Trail)`; testy mogą iść osobno), prefiksy konwencjonalne, trailer
   `Claude-Session: https://claude.ai/code/session_014GbD5V5FZgxcEYK1zPRzjF`, bez
   `Co-Authored-By`.

## Poza zakresem

Pomiary czasu (integrator: `Navigation_Tick`, `Navigation_Path`, sonda płaskowyżu), dema i
sondy, pole grupy (etap 3), kolumna (etap 4), zmiany plannera i keepingu poza wymienionymi,
obsługa starych zapisów (rozkaz rośnie o ~32 B: stary zapis nie wczyta się; linia CHANGELOG
integratora).

## Raport

Commity; co zmieniono poza briefem i dlaczego; czego nie zrobiono; ryzyka zauważone w kodzie
(cykle podążania, czoło zakleszczone, prowadzony znika, ustępujący, jednostki bez `Mover`/`Pace`,
opóźnienie ticku dotyków z kroków odmówionych pod komórkami); proponowane wpisy do
`CHANGELOG.md` (temat v0.8.0), `doc/behaviour.md` (komendy `Aimed`), skilli `gram-plugins`
(navigation) i `gram-perf-navigation`; dokładne polecenia benchmarków, które kod dotyka
(`go test ./bench -run xxx -bench 'Navigation_Tick|Navigation_Path' -benchtime=200x -count=2`).
