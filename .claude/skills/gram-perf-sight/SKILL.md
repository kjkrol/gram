---
name: gram-perf-sight
description: Wydajność wzroku grama — skan tylko czytanych obserwatorów (vision.Watched, WithScanAll), relief podany aabbworldowi jako dane stożka, slaby i marsz po polu wysokości w SIMD (AVX2, GOEXPERIMENT=simd); niezmienniki (te same widzenia co do bitu w obu buildach), pułapka zegara pod AVX2; benchmarki Vision_*, Views_Copy; skill simd aabbworlda czytany ścieżką. Czytaj przy zmianie vision, osłony, reliefu pod wzrokiem albo kerneli aabbworlda, i gdy skan zwolnił.
---

# gram-perf-sight

Protokół i pułapki pomiaru: `gram-perf`. Liczby: `BENCHMARKS.md` („Sight", „Over a relief", „The
views' copy"); etap: `doc/plans-perf.md` („SIMD w widzeniu"). Kernele: skill `simd` aabbworlda,
`/home/karol/CODE/aabbworld/.claude/skills/simd/SKILL.md` (stąd czytany ścieżką), i jego
`BENCHMARKS.md` („Sight in SIMD").

## Niezmienniki

- Skanuje się tylko obserwatorów, których coś czyta (`vision.Watched`: reguła `Sighting` roli,
  `SightOutline`); `WithScanAll()` przywraca wszystkich. Benchmarki skanują wszystkich, więc
  udział skanu w grze jest mniejszy, niż mówią.
- Relief idzie do aabbworlda jako dane stożka (`Cone.Heights`), nie callback na punkt; zapytanie
  stożkowe, slaby i marsz po polu wysokości liczy aabbworld w SIMD.
- Te same widzenia co do bitu w buildzie SIMD i zwykłym: zmianę kerneli mierzy się i testuje w
  obu.
- Indeks przestrzeni gotowy przed partią wzroku (`SpaceRead.Ready`); bez tego skan płaci jego
  budowę.

## Pułapki

- Zegar pod AVX2: szczeble gruntu próbkowanego (`step=cell`, `step=50m`) w buildzie SIMD są przy
  skanie równoległym kilka procent wolniejsze, a na jednym CPU szybsze, bo AVX2 na każdym
  hiperwątku obniża zegar skalarnym callbackom. Udokumentowane, nie regresja do ścigania.

## Odrzucone

- Osiem promieni float32 w slabach: remisy odwracały się na pudełkach na linii komórki; zostały
  cztery float64.
