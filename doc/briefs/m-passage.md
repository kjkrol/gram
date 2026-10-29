# Pomiar etapu A: przejezdność planszy i liczniki (brief agenta mierzącego, 2026-10-29)

Drzewo: worktree `/home/karol/worktrees/gram/passage` (gałąź `feat/passage`, kod etapu A
scalony). Baza: kopia `main` v0.10.0 w katalogu roboczym agenta poza `/home/karol/CODE`
(`git -C /home/karol/CODE/gram worktree add --detach /home/karol/worktrees/gram/base-v0.10.0
v0.10.0`), **z plikami benchmarków drzewa skopiowanymi do bazy tam, gdzie się budują** (nowe
rungi nie zbudują się na bazie: porównać wspólne). Skill `gram-perf` (protokół: na zmianę,
bezczynna maszyna, min z rund, `-cpu=1`, `compare.py`, pułapki: `-bench` poziom po poziomie,
linia loga świata łamie grep, kwota `/tmp`) i `gram-probe` (sonda; pułapki: wynik tłumu
zależy od obciążenia maszyny, parametry sondy przez zmienne środowiska wprost w wierszu
polecenia). Oba drzewa budują się z `GOWORK=off` (tagowane rodzeństwo). **Jeden pomiar
naraz; nic innego nie buduje się ani nie testuje w czasie pomiaru.** Nie zmieniaj kodu; gdy
coś nie biegnie, zgłoś.

## Benchmarki (oczekiwanie: wszystko w szumie, ±2–3%; żaden rung nie traci ≥ 3% mediany)

1. `BASE_ENV="GOWORK=off" TREE_ENV="GOWORK=off" BENCHTIME=200x
   /home/karol/CODE/gram/.claude/skills/gram-perf/measure.sh <out>/tick
   'Navigation_Tick|Navigation_Path|Board_|Gravity_Standing' <baza> <drzewo> 3 2`, potem to samo
   z `EXTRA='-cpu=1'` do `<out>/tick1`.
2. Ręcznie, w tej samej pętli na zmianę (3 rundy × `-count=2`, `-cpu=1`, `GOWORK=off`):
   `go test ./plugins/navigation -run xxx -bench 'Routing_(Path|Routes|Group)' -benchtime=10x`
   i `go test ./plugins/navigation/internal/pathfind -run xxx -bench 'PathFinder_(Terrain|Field)'
   -benchtime=10x`; wyniki do plików i `compare.py`.
3. Profil (`-cpuprofile`) `Board_Altitude` albo rungu planszy, który dotyka komórek, po obu
   stronach, jeśli którykolwiek rung planszy drgnie ≥ 3%: `go tool pprof -top`.

## Sonda Madery (punkt odniesienia dla etapu B)

Sonda `column_probe2.go` ze scratchpadu integratora
(`/tmp/claude-1000/-home-karol-CODE-gram/0fc0edc6-8621-41d0-a635-e70f8e49fc9a/scratchpad/column_probe2.go`)
jako wzór: skopiuj do własnego katalogu poza repo i przerób na `route_probe.go`: (a) na ticku 4
wyślij trzy rozkazy jednostek graczy z `map.yaml` (Funchal→Santana, Machico→Câmara de Lobos,
Poiso→Faial: id po rodzaju i pozycji startowej; `MoveTo{Cell, At, Level}` przez `Carrier().Put`,
po jednej jednostce zaznaczonej `selection.Select{IDs}` na rozkaz) oraz 30 górali jednym
`MoveTo` na (115, 61) jak dotąd; (b) co 5 s gry linia zbiorcza i co tick przyrosty
`navigation.Read.Counts()` (drzewo; na bazie brak `Counts`: tam tylko dojścia i czasy) z
informacją, która jednostka pytała (zmiana `Path.From`/`Length` w rozkazie) — oczekiwane na
długą trasę: 3 szukania na jednostkę (kawałki), rozwinięcia na szukanie; (c) czas ticku, w
którym ktoś pytał: sonda nie sięga do koordynatora, więc uruchom z `GRAM_FPS_LOG=1` i dopasuj
linie „tick" loga (czas symulacji na sekundę) do sekund, w których liczniki szukań rosły;
w raporcie podaj czas ticku w sekundach z pytaniem przeciw sekundom bez; (d) kadencja: `BoardRead.Version()` i `Passage()` co sekundę
gry przez 300 s przy wietrznym starcie (`PROBE_SED` ustawiający `Start: weather.Windy` albo
podobny w `demo.go`; sprawdź nazwy w `plugins/atmosphere/weather`), licząc przejścia wiatru;
(e) END gdy wszyscy doszli albo 900 s. Uruchom na bazie i na drzewie, po dwa razy, ziarno 7
(`PROBE_SEED=7`, `PROBE_TREE=<drzewo>`), `timeout` 300 s realnych:
`.claude/skills/gram-probe/probe.sh examples/board/topography <probe.go> 300 <workdir>` z
katalogu `/home/karol/CODE/gram`. Raportuj: dojścia i czasy (30/30 ≤ 340 s na obu),
szukania/rozwinięcia na rozkaz, `Version` vs `Passage` pod wiatrem (oczekiwane: `Version`
rośnie przy każdym przejściu wiatru, `Passage` nie), czasy ticków z pytaniami.

## Raport

Tabele `compare.py` (min i mediana) dla każdego zestawu; ewentualne rungi ≥ 3% z profilem;
liczby sondy (baza vs drzewo); co nie zbiegło i czemu; polecenia użyte dosłownie.
