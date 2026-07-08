# Миграция на v2.0 (конфигурация Go DSL)

go-arch-lint v2.0 заменяет файл конфигурации YAML (`.go-arch-lint.yml`)
на чистый Go DSL. Конфигурация теперь это файл `.go-arch-lint/arch.go`, который
импортирует пакет `github.com/fe3dback/go-arch-lint/dsl`. Это даёт проверку
типов, автодополнение в IDE и всю мощь Go (переменные, циклы, вспомогательные
функции) внутри вашей конфигурации.

Это жёсткий переход. YAML reader удалён. Вы должны мигрировать конфигурацию
на формат Go DSL.

## Шаги миграции

1. Установите бинарник v2.0:
   ```bash
   go install github.com/fe3dback/go-arch-lint@latest
   ```

2. Создайте каркас новой директории конфигурации в корне проекта:
   ```bash
   cd ~/code/my-project
   go-arch-lint init
   ```
   Это создаст директорию `.go-arch-lint/`, содержащую:
   - `go.mod` (фиксирует версию линтера)
   - `main.go` (сгенерированный, не редактируйте)
   - `arch.go` (вы редактируете этот файл)

3. Переведите ваш `.go-arch-lint.yml` в `.go-arch-lint/arch.go`, используя
   таблицу соответствия ниже.

4. Удалите старый `.go-arch-lint.yml`.

5. Запустите `go-arch-lint check` для проверки. Первый запуск компилирует ваш
   `arch.go`, что занимает от 1 до 3 секунд. Последующие запуски используют
   кэш сборки Go и работают значительно быстрее.

## Соответствие YAML и DSL

| YAML | Go DSL |
|---|---|
| `version: 3` | `Version(1)` (версия схемы DSL, всегда 1 для v2.0) |
| `workdir: internal` | `Workdir("internal")` |
| `allow: { depOnAnyVendor: false }` | `Allow(func() { DepOnAnyVendor(false) })` |
| `allow: { deepScan: true }` | `DeepScan(true)` внутри callback `Allow` |
| `allow: { ignoreNotFoundComponents: true }` | `IgnoreNotFoundComponents(true)` внутри callback `Allow` |
| `exclude: [a, b]` | `Exclude("a", "b")` |
| `excludeFiles: [regex]` | `ExcludeFiles("regex")` |
| `vendors: { name: { in: x } }` | `Vendor("name", "x")` |
| `vendors: { name: { in: [a,b] } }` | `Vendor("name", "a", "b")` |
| `components: { name: { in: x } }` | `Component("name", "x")` |
| `commonComponents: [a, b]` | `CommonComponents("a", "b")` |
| `commonVendors: [a, b]` | `CommonVendors("a", "b")` |
| `deps: { name: { mayDependOn: [...] } }` | `Deps("name", func() { MayDependOn("...") })` |
| `deps.name.canUse: [...]` | `CanUse("...")` внутри callback `Deps` |
| `deps.name.anyVendorDeps: true` | `AnyVendorDeps(true)` внутри callback `Deps` |
| `deps.name.anyProjectDeps: true` | `AnyProjectDeps(true)` внутри callback `Deps` |
| `deps.name.deepScan: true` | `DeepScan(true)` внутри callback `Deps` (переопределяет глобальное) |

Если вы используете YAML схему V1 или V2, сначала обновитесь до V3 по существующей
документации, затем переведите в DSL.

## Рабочий пример

### До (YAML, `.go-arch-lint.yml`)

```yaml
version: 3
workdir: internal
allow:
  depOnAnyVendor: false
excludeFiles:
  - "^.*_test\\.go$"
components:
  main:       { in: app }
  services:   { in: services/** }
  models:     { in: models/** }
commonComponents:
  - models
vendors:
  cobra: { in: github.com/spf13/cobra }
deps:
  main:
    mayDependOn:
      - services
  services:
    mayDependOn:
      - services
    canUse:
      - cobra
```

### После (Go DSL, `.go-arch-lint/arch.go`)

```go
package main

import . "github.com/fe3dback/go-arch-lint/dsl"

var _ = Spec(func() {
    Version(1)
    Workdir("internal")

    Allow(func() {
        DepOnAnyVendor(false)
    })

    ExcludeFiles(`^.*_test\.go$`)

    Vendor("cobra", "github.com/spf13/cobra")

    Component("main", "app")
    Component("services", "services/**")
    Component("models", "models/**")

    CommonComponents("models")

    Deps("main", func() {
        MayDependOn("services")
    })

    Deps("services", func() {
        MayDependOn("services")
        CanUse("cobra")
    })
})
```

## Что изменилось помимо конфигурации

- Команда `schema` удалена. Экспорт JSON Schema больше не существует. Используйте
  `go doc github.com/fe3dback/go-arch-lint/dsl` для справки по API.
- Флаг `--arch-file` устарел. Конфигурация всегда находится в
  `.go-arch-lint/arch.go` внутри вашего проекта.
- Директория `.go-arch-lint/` это отдельный Go модуль. Если ваш проект использует
  `go.work`, не добавляйте `.go-arch-lint/` в workspace. Это инструментальный
  модуль, а не код проекта.

## Что осталось без изменений

- Команды `check`, `mapping`, `graph` и `selfInspect` работают так же.
- Флаги вроде `--project-path`, `--max-warnings`, `--json` и `--output-type`
  сохранены.
- Логика линтера (assembler, checker, renderer) не изменилась. Изменился только
  формат конфигурации.
- Коды возврата: `0` для чистого результата, `1` для предупреждений.

## Устранение неполадок

**«Spec() was not called»:** Ваш `arch.go` должен содержать
`var _ = Spec(func() { ... })` на уровне пакета. Часть `var _ =` гарантирует,
что код выполнится при инициализации.

**Ошибки компилятора Go в `arch.go`:** Сигнатуры функций DSL и есть
схема. Если компилятор ругается, проверьте [справочник по синтаксису](syntax/README.md)
или выполните `go doc github.com/fe3dback/go-arch-lint/dsl` для точных сигнатур.

**Медленный первый запуск:** Первый `go-arch-lint check` компилирует вашу
конфигурацию. Это занимает от 1 до 3 секунд. Последующие запуски кэшируются
через `$GOCACHE` и занимают несколько сотен миллисекунд. Чтобы принудительно
пересобрать, выполните `go clean -cache` в директории `.go-arch-lint/` или
удалите `.go-arch-lint/go.sum` и запустите снова.
