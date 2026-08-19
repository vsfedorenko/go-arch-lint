# Миграция на v2.0 (конфигурация Go DSL)

В v2.0 конфигурация переехала из YAML (`.go-arch-lint.yml`) в обычный Go-файл
`.go-arch-lint/arch.go`. Он импортирует пакет `github.com/vsfedorenko/go-arch-lint/dsl`,
поэтому спека — это обычный код: с проверкой типов, подсказками IDE, переменными
и циклами, если они нужны.

Переход обязательный: YAML-ридер удалён, старые конфиги работать перестанут.

## Шаги миграции

1. Установите бинарник v2.0:
   ```bash
   go install github.com/vsfedorenko/go-arch-lint@latest
   ```

2. Создайте каркас новой директории конфигурации в корне проекта:
   ```bash
   cd ~/code/my-project
   go-arch-lint init
   ```
   Появится директория `.go-arch-lint/` с тремя файлами:
   - `go.mod` — фиксирует версию линтера
   - `arch.go` — сюда переносите конфигурацию
   - `main.go` — сгенерированный раннер, не трогайте его

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
// arch.go
package main

import . "github.com/vsfedorenko/go-arch-lint/dsl"

var spec = Spec(func() {
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

Рядом `init` кладёт `main.go` — раннер, который запускает проверку и
пробрасывает флаги CLI. Его не нужно менять.

## Что изменилось помимо конфигурации

- Команда `schema` удалена. Экспорт JSON Schema больше не существует. Используйте
  `go doc github.com/vsfedorenko/go-arch-lint/dsl` для справки по API.
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
- Коды возврата как у других линтеров: `0` — чисто, `1` — есть нарушения,
  `2` — сломана конфигурация (arch.go не компилируется или проект не читается).
  См. `archlint.ExitCode`.

## Устранение неполадок

**«Spec() was not called»:** в `arch.go` должна быть package-level переменная
`var spec = Spec(func() { ... })`. Без `var` код внутри замыкания вообще не
выполнится — Go не запустит его сам.

**Ошибки компиляции в `arch.go`:** сигнатуры функций DSL — это и есть схема.
Компилятор ругается — сверьтесь со [справочником синтаксиса](syntax/README.md)
или выполните `go doc github.com/vsfedorenko/go-arch-lint/dsl`.

**Медленный первый запуск:** Первый `go-arch-lint check` компилирует вашу
конфигурацию. Это занимает от 1 до 3 секунд. Последующие запуски кэшируются
через `$GOCACHE` и занимают несколько сотен миллисекунд. Чтобы принудительно
пересобрать, выполните `go clean -cache` в директории `.go-arch-lint/` или
удалите `.go-arch-lint/go.sum` и запустите снова.
