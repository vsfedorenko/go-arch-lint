[**Русский**](README.md) | [English](README.en.md)

---

![Logo image](docs/images/logo.png)

Линтер архитектуры для Go: описываете слои и зависимости на Go DSL — линтер находит нарушения в импортах и инъекциях зависимостей.

[![Go Report Card](https://goreportcard.com/badge/github.com/vsfedorenko/go-arch-lint)](https://goreportcard.com/report/github.com/vsfedorenko/go-arch-lint)
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

## Конфигурация

Конфигурация — это Go-файл. Не YAML, не JSON — обычный Go-код с типобезопасностью и автодополнением в IDE.

```bash
cd ~/code/my-project
go-arch-lint init
```

Создаёт `.go-arch-lint/` с `go.mod` и `main.go`:

```go
package main

import (
	"github.com/vsfedorenko/go-arch-lint"
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

`archlint.MustRun(spec)` — то же самое, но вызывает `os.Exit(1)` при ошибке.

## Команды

| Команда       | Назначение                                        |
|---------------|---------------------------------------------------|
| `init`        | Создать каркас `.go-arch-lint/`                   |
| `check`       | Проверить архитектуру                             |
| `graph`       | Сгенерировать граф зависимостей                   |
| `mapping`     | Показать соответствие пакетов и компонентов       |
| `selfInspect` | Проверить архитектуру самого go-arch-lint         |
| `version`     | Вывести версию                                    |

Глобальные флаги: `--project-path`, `--output-type` (`ascii`/`json`), `--json`, `--output-color`.

## Примеры

В каталоге [`examples/`](examples/) — три демонстрационных проекта:

- **[basic](examples/basic/)** — слоистая архитектура (handler → service → repository).
- **[ddd](examples/ddd/)** — domain-driven design (domain → application → infrastructure → interfaces).
- **[hexagonal](examples/hexagonal/)** — ports and adapters (core → adapters → domain).

Каждый пример содержит `.go-arch-lint/main.go` с конфигурацией arch-lint.

## Принцип работы

![How is working](docs/images/how-is-working.png)

Линтер сопоставляет Go-пакеты с компонентами по glob-шаблонам, извлекает импорты из AST, строит фактический граф зависимостей и сравнивает его с желаемым графом из конфигурации. Несовпадения — это нарушения архитектуры.

Режим deep scan анализирует вызовы методов и инъекции зависимостей — не только импорты, но и структурное использование типов между компонентами.

## Лицензия

[MIT](LICENSE). Форк проекта [go-arch-lint](https://github.com/fe3dback/go-arch-lint) © [fe3dback](https://github.com/fe3dback).
