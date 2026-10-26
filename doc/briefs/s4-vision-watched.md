# s4. gram: wzrok skanuje tylko obserwatorów, których ktoś czyta

**Repo, gałąź:** gram, `s/watched` od `perf/sight-simd` (worktree
`/home/karol/worktrees/gram/watched`; buduje się z `go.mod`, aabbworld v1.18.0 z cache).
**Etap:** `doc/plans-perf.md` gram „SIMD w widzeniu", część D.
**Skille:** `gram-plugin` (rejestry, dostarczanie reguł hostom, znaczniki i tagi), `gram-plugins`
(vision, world), `ceremony` (host.PairRules, Marks), `gram-probe`.
**Lektura:** `plugins/vision/doc.go`, `scan_system.go` (`Update`, `job`, `due`, `host`,
`walkCols`, `forget`), `plugin.go` (opcje `With…`, `Install`), `sighting.go`, `sight.go`,
`renderer.go` (co rysuje widoki: GPU przez kamerę z Rays, `SightOutline`), testy pakietu,
`doc/roadmap.md` „Sight's cost", dema: `examples/rules/vision`, `rules/dialog`, `rules/guard`,
`navigation/square`, `navigation/hex`, `board/flat`, `board/topography`.

## Cel

Każdy obserwator z `Sight` i `Eye` jest skanowany co krok (albo `WithFrequency` razy na
sekundę), choć w topography nikt nie czyta wyniku: 37 skanów po relief na tick za nic. Skan ma
iść tylko tam, gdzie ktoś go czyta:
- obserwator gra rolę, której hostowane są reguły `Sighting` (host `PairRules[Sighting]`,
  `host.Marks` obserwatora);
- obserwator nosi `SightOutline` (kształt widoku jest liczony ze skanu);
- obserwator nosi znacznik `vision.Watched{}` (pusty komponent, jak `SightOutline`), który gra
  kładzie na rodzaj, którego `Sighted` czyta sama (system gry, plan, fakt).
Reszta nie jest skanowana, a jej `Sighted` jest wyczyszczony raz (`Count = 0`), gdy przestaje
być czytana. `Plugin.WithScanAll()` przywraca dzisiejsze zachowanie (benchmarki, gry, które
czytają `Sighted` cudzym kodem).

Rysowanie widoków na GPU (kamera z Rays) nie czyta skanu: nie liczy się jako czytanie. Jeśli
okaże się inaczej (sprawdzić w `renderer.go`), zgłosić w raporcie zamiast zgadywać.

## Pliki

Wolno: `plugins/vision/scan_system.go` tylko `Update`, `job`, `due`/`isDue`, `forget` i to, co
decyduje, kto jest skanowany (najlepiej nowy plik `plugins/vision/watched.go`: znacznik, warunek
`watched`, zapytanie o `Marks`); `plugins/vision/plugin.go` (`WithScanAll`, przepuszczenie
hosta); `plugins/vision/doc.go`? Nie: dokumenty są integratora; podać tekst w raporcie. Testy:
`plugins/vision/*_test.go` (nowe i dostosowane), `bench/vision_test.go` (benchmarki wzroku muszą
dalej skanować wszystkich: `WithScanAll()` albo `Watched` w harnessie, do wyboru i do opisania).
Nie wolno: `ground()`, `cone()`, `scanner`, `profiling`, `covering` w `scan_system.go` (część C),
`plugins/vision/renderer.go` poza czytaniem, `examples/*` (dema mają działać bez zmian; jeśli
któreś potrzebuje `Watched`, zgłosić, nie edytować), ceremony, dokumenty, skille, `go.mod`.

## Kroki

1. Mapa czytelników: dla każdego demo z wzroku ustalić, kto czyta `Sighted`/`Sighting` (reguły
   w `hunter.go`, `prey.go`, `host.go`; `SightOutline` w `walker.go`, `demo.go`; `rules/guard`:
   plan straży?). Wynik do raportu.
2. Warunek `watched(id)` w nowym pliku: reguły roli (odczyt `Marks` hosta `Sighting` jak
   `s.host.Read(...)` w `Update`, tanio: raz na chunk, nie per encja, jeśli się da), `SightOutline`
   (kolumna opcjonalna), `Watched` (kolumna opcjonalna). `due(id)` uzupełnione o `watched`.
3. `Sighted` nieskanowanego: `Count = 0` raz (nie co tick); `Sighting` dla takiego obserwatora nie
   jest wysyłany (nie ma reguł) — upewnić się, że `gather`/`DispatchGrouped` tego nie wołają.
4. `WithScanAll()`; benchmarki `bench/vision_test.go` i `bench/vision_relief_test.go` skanują
   wszystkich jak dotąd (inaczej zmierzyłyby nic): przez `WithScanAll()` w harnessie.
5. Testy: obserwator bez czytelnika nie jest skanowany (licznik skanów przez `WithLog`? albo
   `Sighted.Count` zostaje 0 mimo kogoś w stożku); z regułą roli jest; z `SightOutline` jest; z
   `Watched` jest; po zdjęciu `Watched` `Sighted` czyszczony raz; `WithScanAll` skanuje wszystkich;
   testy równoległe (`parallel_test.go`) i częstotliwości dalej przechodzą.
6. Dema (kopie pod `timeout 15s`, nic nie edytować): rules/vision, rules/dialog, rules/guard,
   navigation/square, navigation/hex, board/flat, board/topography: bez paniki; w rules/vision
   łowca dalej goni (sonda gram-probe: `Sighted.Count` łowcy > 0, gdy ofiara w stożku). Test
   dymny kosztu `Vision_Relief` bez `WithScanAll` w kopii benchmarku (do raportu, nie jako
   wynik): ma spaść do zera skanów.

## Poza zakresem

SIMD (A, B, C). Zmiana tego, co widzi skan. Nowe słowa reguł. Plugin AI.

## Zasady

Sekwencja przed każdym commitem: `go build ./... && go vet ./... && gofmt -l . && go test ./...`,
także z `GOEXPERIMENT=simd`. Commity konwencjonalne (`perf:`, `feat:` dla `Watched` i
`WithScanAll`), trailer `Claude-Session: https://claude.ai/code/session_014GbD5V5FZgxcEYK1zPRzjF`,
bez `Co-Authored-By`. Bez pushu. Liczby mierzy integrator. Zasady domu z `CLAUDE.md`: żaden
plik demo nie jest ruszany, komentarze tylko tam, gdzie kod nie powie, linie ≤ 100 kolumn.

## Raport

Mapa czytelników w demach; definicja `watched`; co się dzieje z `Sighted` nieskanowanego; lista
plików; które dema potrzebowałyby `Watched` (jeśli jakieś); polecenia benchmarków i oczekiwany
kierunek; propozycja tekstu do `plugins/vision/doc.go`, CHANGELOG i skilla `gram-plugins`
(vision); szew z C w `scan_system.go`.
