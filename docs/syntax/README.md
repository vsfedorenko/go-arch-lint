# Справочник Arch DSL

Arch конфигурация это Go файл (`.go-arch-lint/arch.go`), использующий пакет
`github.com/fe3dback/go-arch-lint/dsl`. Каждая функция DSL фиксирует свою
позицию в исходнике через `runtime.Caller`, поэтому сообщения об ошибках
указывают на точную строку в вашем `arch.go`.

## Точка входа

### `func Spec(fn func()) struct{}`

Регистрирует новый построитель спецификации, устанавливает его как текущий контекст и выполняет `fn`.
Функции DSL, вызванные внутри `fn`, заполняют построитель. Возвращает нулевое
значение `struct{}`, чтобы паттерн `var _ = Spec(...)` срабатывал при инициализации
пакета, до `main()`.

```go
var _ = Spec(func() {
    Version(1)
    Workdir("internal")
    Component("main", "app")
})
```

## Атрибуты верхнего уровня

### `func Version(v int)`

Устанавливает версию схемы DSL. Для v2.0 всегда `1`.

```go
Version(1)
```

### `func Workdir(path string)`

Устанавливает относительную рабочую директорию для анализа. Линтер проверяет
только Go пакеты внутри этой директории.

```go
Workdir("internal")
```

## Глобальные правила

### `func Allow(fn func())`

Открывает блок обратного вызова для глобальных правил разрешения. Вызывайте
`DepOnAnyVendor`, `DeepScan` и `IgnoreNotFoundComponents` внутри `fn`.

```go
Allow(func() {
    DepOnAnyVendor(false)
    DeepScan(true)
    IgnoreNotFoundComponents(false)
})
```

### `func DepOnAnyVendor(b bool)`

Определяет, может ли любой код проекта импортировать любые vendor библиотеки. По умолчанию `false`.
Вызывать внутри `Allow`.

### `func DeepScan(b bool)`

Включает или отключает расширенный AST анализ (отслеживание инъекций через конструкторы).
По умолчанию `true`.

Внутри `Allow`: задаёт глобальное значение по умолчанию. Внутри `Deps`: переопределяет
настройку для отдельного компонента.

### `func IgnoreNotFoundComponents(b bool)`

Когда `true`, компоненты, чей glob не сопоставлен ни с одним пакетом, тихо пропускаются
вместо выдачи ошибки. По умолчанию `false`. Вызывать внутри `Allow`.

## Исключения

### `func Exclude(paths ...string)`

Добавляет директории (относительные пути) для исключения из анализа.

```go
Exclude("vendor", "testdata")
```

### `func ExcludeFiles(patterns ...string)`

Добавляет шаблоны регулярных выражений для имён файлов, которые нужно исключить.
Совпадающие файлы и их пакеты пропускаются во время анализа.

```go
ExcludeFiles(`^.*_test\.go$`, `^.*\/mock\/.*$`)
```

## Компоненты

Компонент это абстракция над одним или несколькими Go пакетами. Один компонент может
сопоставляться с множеством пакетов через glob шаблоны.

### `func Component(name string, paths ...string)`

Определяет именованный компонент, сопоставленный с одним или несколькими относительными
путями пакетов. Поддерживает glob маски (`src/*/engine/**`).

```go
Component("handler", "handlers/*")
Component("services", "services/**", "lib/svc")
```

## Vendors

Vendors это внешние библиотеки из `go.mod`.

### `func Vendor(name string, importPaths ...string)`

Определяет именованный vendor, сопоставленный с одним или несколькими import путями.
Поддерживает glob маски (`github.com/abc/*/engine/**`).

```go
Vendor("cobra", "github.com/spf13/cobra")
Vendor("yaml", "github.com/goccy/go-yaml", "github.com/goccy/go-yaml/**")
```

## Общие списки доступа

### `func CommonComponents(names ...string)`

Помечает компоненты как импортируемые любым пакетом проекта, в обход правил
зависимостей. Полезно для общих моделей или утилитарных пакетов.

```go
CommonComponents("models", "utils")
```

### `func CommonVendors(names ...string)`

Помечает vendors как импортируемые любым пакетом проекта.

```go
CommonVendors("go-common")
```

## Правила зависимостей

### `func Deps(component string, fn func())`

Определяет правила зависимостей для компонента. Вызывайте `MayDependOn`, `CanUse`,
`AnyProjectDeps`, `AnyVendorDeps` и `DeepScan` внутри `fn`. Имя компонента
должно совпадать с одним из определённых через `Component`.

```go
Deps("handler", func() {
    MayDependOn("service", "model")
    CanUse("cobra")
})
```

### `func MayDependOn(components ...string)`

Перечисляет компоненты проекта, которые этот компонент может импортировать.
Должна вызываться внутри `Deps`.

```go
Deps("handler", func() {
    MayDependOn("service")
})
```

### `func CanUse(vendors ...string)`

Перечисляет имена vendors, которые этот компонент может импортировать.
Должна вызываться внутри `Deps`.

```go
Deps("services", func() {
    CanUse("cobra", "yaml")
})
```

### `func AnyProjectDeps(b bool)`

Когда `true`, разрешает компоненту импортировать любой другой пакет проекта.
Полезно для DI контейнеров или точек входа. Должна вызываться внутри `Deps`.

```go
Deps("container", func() {
    AnyProjectDeps(true)
})
```

### `func AnyVendorDeps(b bool)`

Когда `true`, разрешает компоненту импортировать любой пакет vendor.
Должна вызываться внутри `Deps`.

```go
Deps("container", func() {
    AnyVendorDeps(true)
})
```

## Полный пример

```go
// .go-arch-lint/arch.go
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
    Vendor("yaml", "github.com/goccy/go-yaml", "github.com/goccy/go-yaml/**")

    Component("main", "app")
    Component("services", "services/**")
    Component("models", "models/**")

    CommonComponents("models")
    CommonVendors("cobra")

    Deps("main", func() {
        MayDependOn("services")
    })

    Deps("services", func() {
        MayDependOn("services")
        CanUse("cobra", "yaml")
    })
})
```

Таблицу соответствия YAML и DSL см. в [migration-v2.md](../migration-v2.md).

Примеры:
- [.go-arch-lint/arch.go](../../.go-arch-lint/arch.go)
