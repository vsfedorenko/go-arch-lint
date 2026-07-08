![Logo image](./docs/images/logo.png)

Линтер для контроля хорошей структуры проекта и проверки архитектуры верхнего уровня (слоёв кода)

[![Go Report Card](https://goreportcard.com/badge/github.com/vsfedorenko/go-arch-lint)](https://goreportcard.com/report/github.com/vsfedorenko/go-arch-lint)
[![go-recipes](https://raw.githubusercontent.com/nikolaydubina/go-recipes/main/badge.svg?raw=true)](https://github.com/nikolaydubina/go-recipes)

## Быстрый старт

### Что такое архитектура проекта?

Можно представить простую архитектуру, например классическую часть из «чистой архитектуры» (clean architecture):

![Layouts example](./docs/images/layout_example.png)

И описать её как конфигурацию Go DSL:

```go
// .go-arch-lint/arch.go
package main

import . "github.com/vsfedorenko/go-arch-lint/dsl"

var _ = Spec(func() {
    Version(1)
    Workdir("internal")
    Component("handler", "handlers/*")
    Component("service", "services/**")
    Component("repository", "domain/*/repository")
    Component("model", "models")
    CommonComponents("model")
    Deps("handler", func() { MayDependOn("service") })
    Deps("service", func() { MayDependOn("repository") })
})
```

подробности см. в [синтаксисе конфигурации](docs/syntax/README.md).

Теперь линтер проверит весь код проекта внутри workdir `internal` и покажет предупреждения, когда код нарушает эти правила.

Для наилучшего результата добавьте линтер в CI workflow.

### Пример проблемного кода

Представьте `main.go`, где мы передаём `repository` в `handler` и получаем проблемный поток:

```go
func main() {
  // ..
  repository := booksRepository.NewRepository()
  handler := booksHandler.NewHandler(
    service,
    repository, // !!!
  )
  // ..
}
```

Линтер легко найдёт эту проблему:

![Check stdout example](./docs/images/check-example.png)

### Установка/Запуск

#### Через Docker

```bash
docker run --rm -v ${PWD}:/app vsfedorenko/go-arch-lint:latest-stable-release check --project-path /app
```

[другие docker теги и версии](https://hub.docker.com/r/vsfedorenko/go-arch-lint/tags)

#### Из исходников
Требуется go 1.25+

```bash
go install github.com/vsfedorenko/go-arch-lint@latest
```

Создайте каркас конфигурации в проекте:

```bash
cd ~/code/my-project
go-arch-lint init
```

Это создаст директорию `.go-arch-lint/` с `go.mod`, сгенерированным `main.go` и стартовым `arch.go` для редактирования. Затем запустите линтер:

```bash
go-arch-lint check
```

#### Готовые бинарники

[см. на странице релизов](https://github.com/vsfedorenko/go-arch-lint/releases)

## Использование

### Как добавить линтер в существующий проект?

![Adding linter steps](./docs/images/add-linter-steps.png)

Добавление линтера в проект состоит из нескольких шагов:

1. Текущее состояние проекта
2. Запустите `go-arch-lint init` для создания каркаса `.go-arch-lint/`, затем отредактируйте `arch.go`, чтобы описать идеальную архитектуру проекта
3. Линтер найдёт проблемы в проекте. Не исправляйте их прямо сейчас, а «легализуйте», добавив в конфигурацию и пометив меткой `todo`
4. В свободное время, при работе с техническим долгом и т.д. исправляйте код
5. После исправлений очистите конфигурацию до целевого состояния

### Выполнение

```
Usage:
  go-arch-lint check [flags]

Flags:
      --arch-file string      arch file path (default ".go-arch-lint/arch.go")
  -h, --help                  help for check
      --max-warnings int      max number of warnings to output (default 512)
      --project-path string   absolute path to project directory (where '.go-arch-lint/' is located) (default "./")

Global Flags:
      --json                   (alias for --output-type=json)
      --output-color           use ANSI colors in terminal output (default true)
      --output-json-one-line   format JSON as single line payload (without line breaks), only for json output type
      --output-type string     type of command output, variants: [ascii, json] (default "default")
```

Линтер вернёт:

| Код возврата | Описание                              |
|--------------|---------------------------------------|
| 0            | Архитектура проекта корректна         |
| 1            | Найдены предупреждения                |


### Как это работает?

![How is working](./docs/images/how-is-working.png)

Линтер:
- сопоставляет/помечает **go пакеты** с **компонентами**
- находит все зависимости между компонентами
- строит граф зависимостей
- сравнивает фактический (из кода) и желаемый (из конфигурации) граф зависимостей
- если получен непустой DIFF, значит в проекте есть проблемы

## Граф

Пример конфигурации этого репозитория: [.go-arch-lint/arch.go](.go-arch-lint/arch.go)

![graph](./docs/images/graph-example.png)

Граф зависимостей можно сгенерировать командой `graph`:

```bash
go-arch-lint graph
```

Подробности см. в полной [документации графа](docs/graph/README.md).
