---
name: gram-perf-sight
description: Wydajność wzroku grama — skan tylko czytanych obserwatorów (vision.Watched, WithScanAll), relief podany aabbworldowi jako dane stożka zamiast callbacku, zapytanie stożkowe, slaby i marsz po polu wysokości w SIMD (AVX2, GOEXPERIMENT=simd); niezmienniki (te same widzenia co do bitu w obu buildach), liczby, pułapka zegara pod AVX2; benchmarki Vision_Scan, Vision_ScanWithOutlines, Vision_Relief, Views_Copy; skill simd aabbworlda czytany ścieżką. Czytaj przy zmianie vision, osłony, reliefu pod wzrokiem albo kerneli aabbworlda, i gdy skan zwolnił.
---

# gram-perf-sight

Protokół pomiaru i pułapki są w `gram-perf`; tu mapa tego, co we wzroku zrobiono. Liczby:
`BENCHMARKS.md` (`Vision_*`), historia: `doc/plans-perf.md` („SIMD w widzeniu" i wynik, część E
`s/lockstep`); kernele: `/home/karol/CODE/aabbworld/.claude/skills/simd/SKILL.md` (sesja w tym
repo go ładuje; stąd czyta się go ścieżką) i `BENCHMARKS.md` aabbworlda („Sight in SIMD").

## Gdzie idzie czas

`Vision_Scan`, `Vision_ScanWithOutlines`, `Vision_Relief` (szczeble `profile`, `along`,
`step=cell`, `step=50m`, `15Hz`), `Views_Copy`. Benchmarki skanują każdego obserwatora
(`WithScanAll()`), gra tylko czytanych: udział skanu w grze jest mniejszy, niż mówi benchmark.

## Co zrobiono i na czym stoi

- **Wzrok w SIMD i tylko tam, gdzie czytany** (2026-10-26, litera `s`): skan tylko
  obserwatorów, których coś czyta (`vision.Watched`, reguła `Sighting` roli, `SightOutline`;
  `WithScanAll()` przywraca wszystkich; 37 skanów reliefu na tick board/topography odeszło),
  krata reliefu podana aabbworldowi jako `Cone.Heights` zamiast callbacku na punkt, zapytanie
  stożkowe, slaby i marsz lockstep aabbworlda. `Vision_Scan` 100 obserwatorów 79 → 73 µs
  (szeregowo 129 → 106), 500: 295 → 249; `Vision_Relief/profile` 1,76 → 1,21 ms, przy 15 Hz
  774 → 495 µs; część E (`s/lockstep`): `View_Elevated/heights=field` 1 000 encji 242 → 173 µs,
  gram `Vision_Relief/profile` 1,43 → 1,17 ms, 15 Hz 637 → 487. Niezmiennik: te same widzenia
  co do bitu w obu buildach; benchmarki skanują wszystkich.
- **Zegar pod AVX2**: szczeble gruntu próbkowanego (`step=cell`, `step=50m`) są 2–6% wolniejsze
  w buildzie SIMD przy skanie równoległym i 2–4% szybsze na jednym CPU: AVX2 na każdym
  hiperwątku obniża zegar skalarnym callbackom. Udokumentowane, nie regresja do ścigania.
- **Indeks przed partią wzroku** (fala 3): `SpaceRead.Ready` przed skanem; bez tego
  `Vision_Scan` 500 +78%.

## Próbowano i porzucono

Osiem promieni float32 w slabach (remisy odwracały się na pudełkach na linii komórki; zostały
cztery float64). Nie proponować bez nowego powodu.
