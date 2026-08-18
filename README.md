[**Русский**](README.md) | [English](README.en.md)

---

![Logo image](docs/images/logo.png)

**Линтер архитектуры для Go** (`go-arch-lint`): описываете слои и
зависимости на type-safe Go DSL — линтер находит нарушения в импортах
и инъекциях зависимостей. Подходит для **clean architecture**, **hexagonal**,
**onion** и **DDD**.

[![CI](https://github.com/vsfedorenko/go-arch-lint/actions/workflows/ci.yml/badge.svg)](https://github.com/vsfedorenko/go-arch-lint/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/vsfedorenko/go-arch-lint)](https://goreportcard.com/report/github.com/vsfedorenko/go-arch-lint)
[![Go Reference](https://pkg.go.dev/badge/github.com/vsfedorenko/go-arch-lint.svg)](https://pkg.go.dev/github.com/vsfedorenko/go-arch-lint)
[![Go Version](https://img.shields.io/badge/Go-%3E%3D%201.25-blue)](https://golang.org/dl/)
[![License: MIT](https://img.shields.io/github/license/vsfedorenko/go-arch-lint)](LICENSE)
[![Release](https://img.shields.io/github/v/release/vsfedorenko/go-arch-lint)](https://github.com/vsfedorenko/go-arch-lint/releases)
[![go-recipes](https://raw.githubusercontent.com/nikolaydubina/go-recipes/main/badge.svg?raw=true)](https://github.com/nikolaydubina/go-recipes)

## Установка

```bash
go install github.com/vsfedorenko/go-arch-lint@latest
```

Или через [Docker](https://github.com/vsfedorenko/go-arch-lint/pkgs/container/go-arch-lint):

```bash
docker run --rm -v ${PWD}:/app ghcr.io/vsfedorenko/go-arch-lint:latest check --project-path /app
```

Или [бинарник из релизов](https://github.com/vsfedorenko/go-arch-lint/releases).

## Требования

- **Go 1.25+** (поддерживаются две последние мажорные версии: 1.25 и 1.26; CI тестирует обе).
- `go` должен быть доступен на `PATH` — CLI компилирует `.go-arch-lint/arch.go` через `go run` (кэшируется, повторные запуски ~2s).

## Конфигурация

Конфигурация — это Go-файл. Не YAML, не JSON — обычный Go-код с типобезопасностью и автодополнением в IDE.

```bash
cd ~/code/my-project
go-arch-lint init
```

Создаёт `.go-arch-lint/` с `go.mod`, `arch.go` (ваша спека) и `main.go` (стабильный раннер):

```go
// arch.go — редактируйте этот файл
package main

import (
	. "github.com/vsfedorenko/go-arch-lint/dsl"
)

var spec = Spec(func() {
	Version(1)
	Workdir("internal")

	Component("handler", "handlers/*")
	Component("service", "services/**")
	Component("repository", "domain/*/repository")

	CommonComponents("model")

	Deps("handler", func() {
		MayDependOn("service")
	})
	Deps("service", func() {
		MayDependOn("repository")
	})
})

func main() {
	archlint.MustRun(spec)
}
```

Что здесь происходит:

— `Workdir` задаёт корень, ниже которого линтер ищет Go-пакеты.
— `Component` связывает имя компонента с glob-шаблоном путей.
— `Deps` описывает, на какие компоненты разрешено зависеть.
— `CommonComponents` — компоненты, доступные всем (утилиты, модели).
— `Vendor` и `CanUse` — сторонние библиотеки, разрешённые конкретному компоненту.

Полный список функций DSL — в [документации синтаксиса](docs/syntax/README.md) или через `go doc github.com/vsfedorenko/go-arch-lint/dsl`.

### Рецепты init

Не хотите начинать с чистого листа — возьмите готовый шаблон известной архитектуры:

```bash
go-arch-lint init --recipe hexagonal   # порты и адаптеры: домен+core в центре, http/db смотрят внутрь
go-arch-lint init --recipe ddd         # DDD: bounded contexts + application/infrastructure/interfaces
go-arch-lint init --recipe clean       # чистая архитектура: domain ← usecase ← delivery
```

Рецепт пишет тот же каркас (`.go-arch-lint/`), но `arch.go` сразу описывает слои и правила зависимостей выбранного паттерна. Спека включает `IgnoreNotFoundComponents(true)` — директории слоёв можно создавать постепенно, линтер не упадёт, пока какого-то слоя ещё нет.

## Проверка

```bash
go-arch-lint check
```

Линтер строит граф импортов из реального кода, сравнивает с графом из конфигурации и выводит нарушения:

![Check output](docs/images/check-example.png)

| Код возврата | Значение                     |
|--------------|------------------------------|
| 0            | Нарушений нет                |
| 1            | Найдены нарушения            |

Флаг `--json` переключает вывод в машиночитаемый формат для CI.

### Подавление известных нарушений (baseline)

Для инкрементального внедрения на legacy-коде известное нарушение можно
пометить в исходнике директивой — проверка пройдёт, а новые нарушения
по-прежнему будут фейлить сборку:

```go
//go-arch-lint:ignore            // подавить любое нарушение на строке
//go-arch-lint:ignore beta       // только если цель зависимости — beta
import _ "example.com/app/internal/beta"

//go-arch-lint:ignore-file       // подавить все нарушения в файле
package legacy
```

Директива ставится на самой строке нарушения или строкой выше. Аргумент
(через пробел, можно несколько) фильтрует по цели: имени компонента
(`beta`) или последнему сегменту импорт-пути (`internal/beta` → `beta`).
Подавленные нарушения не исчезают бесследно: в конце отчёта выводится
`suppressed: N (by //go-arch-lint:ignore directives)`, а в JSON — поле
`SuppressedCount`, так что технический долг остаётся видимым.

### Baseline-файл: инкрементальное внедрение

Когда нарушений сотни и размечать исходники директивами нереалистично,
запишите их в baseline-файл — проверка будет падать только на НОВЫХ
нарушениях («не чинить всё сразу, но и не добавлять нового»):

```bash
# 1. Записать текущие нарушения (файл коммитится в репозиторий):
go-arch-lint check --baseline .go-arch-lint/baseline.json --baseline-update

# 2. В CI — только сравнение: известный долг толерируется,
#    новое нарушение фейлит сборку (exit 1):
go-arch-lint check --baseline .go-arch-lint/baseline.json
```

Baseline — JSON с версией схемы и отпечатками (fingerprints) нарушений:
`kind|rule|file` без номеров строк, поэтому правки выше по файлу не
«воскрешают» старый долг как новый. Сводка в текстовом выводе показывает
`baseline: N new, M known (tolerated)`, в JSON — поля `BaselineNewCount`
и `BaselineKnownCount`. Отсутствующий baseline-файл в режиме сравнения —
ошибка конфигурации (exit 2), а не молчаливый пропуск. Когда долг
починен — просто перезапишите baseline (устаревшие отпечатки игнорируются).

Под капотом команды `check`/`mapping`/`graph`/`selfInspect` делегируются в
`.go-arch-lint/` через `go run` — как устроена маршрутизация флагов, выходные
коды и кэширование (холодный старт ~45 с, рабочий режим ~2 с), см.
[docs/delegation.md](docs/delegation.md).

## Граф зависимостей

```bash
go-arch-lint graph --format=mermaid
```

```
graph LR
  handler --> service
  service --> repository
  handler -.-> n0["3rd-cobra"]
```

Четыре формата вывода:

| `--format`  | Куда                         | Зачем                                |
|-------------|------------------------------|--------------------------------------|
| `svg`       | файл (по умолчанию)          | готовое изображение                  |
| `d2`        | stdout                       | исходник d2 для ручной доработки     |
| `plantuml`  | stdout                       | рендер через PlantUML или CI         |
| `mermaid`   | stdout                       | Markdown, GitHub, GitLab             |

Дополнительно: `--type=di` (обратный граф, DI), `--focus=handler` (только один компонент), `--include-vendors` (показать сторонние библиотеки).

## Программный API

go-arch-lint — не только CLI, но и библиотека. Вызов проверки из Go-кода:

```go
import (
	"github.com/vsfedorenko/go-arch-lint"
	. "github.com/vsfedorenko/go-arch-lint/dsl"
)

func runArchCheck() error {
	spec := Spec(func() {
		Version(1)
		Workdir("internal")
		Component("handler", "handlers/*")
		Component("service", "services/**")
		Deps("handler", func() { MayDependOn("service") })
	})

	return archlint.Run(spec,
		archlint.WithProjectPath("."),
		archlint.WithMaxWarnings(100),
	)
}
```

`archlint.MustRun(spec)` — то же самое, но завершает процесс с конвенциональным
кодом возврата: `1` при нарушениях, `2` при ошибке конфигурации (см. `archlint.ExitCode`).

## Команды

| Команда       | Назначение                                        |
|---------------|---------------------------------------------------|
| `init`        | Создать каркас `.go-arch-lint/`; `--recipe clean\|hexagonal\|ddd` — начать с известного паттерна |
| `check`       | Проверить архитектуру                             |
| `graph`       | Сгенерировать граф зависимостей                   |
| `mapping`     | Показать соответствие пакетов и компонентов       |
| `selfInspect` | Проверить архитектуру самого go-arch-lint         |
| `version`     | Вывести версию                                    |

Глобальные флаги: `--project-path` (кратко `-p`), `--max-warnings N` (лимит показа нарушений, по умолчанию 512; код выхода отражает полное число — подробности в [docs/json-schema.md](docs/json-schema.md#output-cap---max-warnings)), `--format text|json|sarif|junit|github-actions|html` (check), `--baseline <file>` + `--baseline-update` (инкрементальное внедрение: известные нарушения толерируются, проверка падает только на новых — см. [Baseline / инкрементальный режим](#baseline--инкрементальный-режим)), `--output-type` (`ascii`/`json`; неизвестное значение — ошибка конфигурации), `--json` (сокращение для `--output-type=json`), `--output-json-one-line` (однострочный JSON; без json-вывода — ошибка конфигурации, а не молчаливое игнорирование), `--output-color` / `--no-colors` (выключить ANSI-цвета).

### Правила видимости (export visibility)

`Visibility(func(){ VisibleTo(...) })` — ограничивает, какие компоненты могут потреблять API указанной компоненты:

```go
Visibility(func() {
    VisibleTo("services")                    // полностью внутренняя: никто не может импортировать
    VisibleTo("models", "services", "container") // только services и container
})
```

- Правило работает по **фактическому импорт-графу**: импорт компоненты вне allow-листа — нарушение
- Сама компонента всегда может использовать свой код (self неявно разрешён)
- Несколько правил на одну компоненту аккумулируются (allow-листы объединяются)
- Сообщение указывает файл-свидетель первого импорта и видимый список

### Размещение интерфейсов (порты с потребителем)

В гексагональной архитектуре интерфейс-порт живёт с **потребителем**, а
не рядом с реализацией. Правило включается одной декларацией:

```go
Interfaces(func() {
	MustLiveWithConsumer()
})
```

Чекер находит интерфейсы, которые использует ровно **один** другой
компонент, и требует перенести декларацию к потребителю:

```
interface 'Iface' must live with its consumer 'alpha' (declared in component 'beta')
```

- Интерфейс, используемый двумя и более компонентами, — законно общий,
  остаётся где объявлен.
- Использование внутри своего компонента не считается.
- Анализ чисто синтаксический (один полный проход парсера, без загрузки
  типов) — быстро и без сети.
- Собственная спека go-arch-lint включает это правило.

### Конвенции именования пакетов

Бесмысленные имена пакетов (`utils`, `helpers`, `common`, …) — свалки, в
которые со временем утекает весь код. Правило включается одной
декларацией в спеке:

```go
Naming(func() {
	ForbiddenPackages("utils", "helpers", "common", "misc", "stuff")
})
```

Чекер сверяет имена всех просканированных пакетов проекта (фактические
`package X` в файлах, не пути) и репортит каждое нарушение один раз на
пакет с числом файлов:

```
Package name utils is forbidden internal/utils (3 file(s)): first at internal/utils/a.go
```

В JSON-выводе (`--format json`) нарушения имеют тип `naming`. Собственная
спека go-arch-lint уже баннит `utils`, `helpers`, `common`, `misc`,
`stuff`.

### Метрики связанности (mapping)

`mapping -s grouped` показывает для каждого компонента метрики по **фактическому** графу импортов:

```
  services:
    coupling: out 3 | in 2 | stability 0.60
```

- `out` (Ce) — на сколько компонентов компонент ссылается (fan-out);
- `in` (Ca) — сколько компонентов ссылаются на него (fan-in);
- `stability` — I = Ce / (Ca + Ce) по Роберту Мартину: `0` — максимально
  стабильный (на него опираются, сам ни на кого не завязан), `1` —
  максимально нестабильный. Ценные для переиспользования компоненты
  должны иметь низкий stability.

В JSON-выводе метрики приходят в `MappingGrouped[].Coupling`
(`omitempty` — для компонентов без зависимостей поле отсутствует).

### JSON-вывод для CI

`check --format json` печатает плоский массив нарушений (стабильный
порядок, схема и примеры интеграции — в [docs/json-schema.md](docs/json-schema.md)):
GitHub Actions-аннотации, GitLab CI-artifacts, семантика exit-кодов.

Для сканеров кода (GitHub Code Scanning, DefectDojo, SARIF-дашборды) есть
`check --format sarif` — лог SARIF 2.1.0 с ruleId, уровнями и координатами
нарушений: [docs/json-schema.md → SARIF](docs/json-schema.md#sarif-output-for-github-code-scanning).

Для CI-дашбордов тестов (GitLab CI test reports, Jenkins JUnit plugin,
Buildkite) есть `check --format junit` — JUnit-отчёт в XML: одно нарушение =
один упавший testcase, у чистого проекта один зелёный `arch-check`.
Рецепт: [docs/json-schema.md → JUnit](docs/json-schema.md#junit-output-for-ci-test-dashboards).

Для людей и архивов есть `check --format html` — самодостаточный
HTML-отчёт (инлайн-CSS, без скриптов и внешних ассетов): карточки счётчиков
по типам правил, таблица нарушений с file:line, экранирование враждебных
путей, тёмная тема по `prefers-color-scheme`. Артефакт CI открывается
напрямую в браузере.
Схема: [docs/json-schema.md → HTML](docs/json-schema.md#html-report-for-humans-and-archives).

Для pull request'ов есть официальный GitHub Action: аннотации `::error`
прямо на строках диффа, установка бинарника из релиза — без JS-скриптов:

```yaml
- uses: actions/checkout@v7
- uses: actions/setup-go@v7
  with: { go-version: '1.25' }
- uses: vsfedorenko/go-arch-lint@main   # после первого релиза — @v2.1
```

Подробности и все inputs: [docs/json-schema.md → GitHub Action](docs/json-schema.md#github-action-with-inline-annotations).

### Коды выхода (check)

| Код | Значение                                                        |
|-----|-----------------------------------------------------------------|
| `0` | Нарушений нет                                                    |
| `1` | Найдены нарушения архитектуры                                    |
| `2` | Ошибка конфигурации/системы (спека не собирается, проект не читается) |

CI может различать «нашли нарушения» (падение сборки) и «сломан конфиг» (конфиг ничего не проверяет — звать мейнтейнера).

## Примеры

В каталоге [`examples/`](examples/) — три демонстрационных проекта:

- **[basic](examples/basic/)** — слоистая архитектура (handler → service → repository).
- **[ddd](examples/ddd/)** — domain-driven design (domain → application → infrastructure → interfaces).
- **[hexagonal](examples/hexagonal/)** — ports and adapters (core → adapters → domain).

Каждый пример содержит `.go-arch-lint/` с конфигурацией arch-lint (`arch.go` + `main.go`).

## Принцип работы

![How is working](docs/images/how-is-working.png)

Линтер сопоставляет Go-пакеты с компонентами по glob-шаблонам, извлекает импорты из AST, строит фактический граф зависимостей и сравнивает его с желаемым графом из конфигурации. Несовпадения — это нарушения архитектуры.

Режим deep scan анализирует вызовы методов и инъекции зависимостей — не только импорты, но и структурное использование типов между компонентами.

## Лицензия

[MIT](LICENSE). Форк проекта [go-arch-lint](https://github.com/fe3dback/go-arch-lint) © [fe3dback](https://github.com/fe3dback).
