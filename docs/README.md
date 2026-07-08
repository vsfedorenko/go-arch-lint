# Документация

- [синтаксис](syntax/README.md)
- [migration-v2](migration-v2.md)

## Расширенное использование

### Справка по DSL API

Схема конфигурации определяется пакетом `github.com/fe3dback/go-arch-lint/dsl`.
Поскольку конфигурация теперь чистый Go, компилятор и автодополнение IDE
заменяют старый слой JSON Schema.

Чтобы посмотреть полное API с сигнатурами и документацией, выполните:

```bash
go doc github.com/fe3dback/go-arch-lint/dsl
```

Или посмотрите конкретную функцию:

```bash
go doc github.com/fe3dback/go-arch-lint/dsl.Spec
go doc github.com/fe3dback/go-arch-lint/dsl.Component
go doc github.com/fe3dback/go-arch-lint/dsl.Deps
```

Описание с примерами см. в [syntax/README.md](syntax/README.md).

### mapping

Сопоставление archfile с исходными файлами можно посмотреть через команду `mapping`.

Доступно два режима:
- список (по умолчанию)
- группировка по компонентам

```bash
go-arch-lint mapping

module: github.com/fe3dback/go-arch-lint
Project Packages:
   app                 /internal/app
   container           /internal/app/internal/container
   commands            /internal/commands/check
   commands            /internal/commands/mapping
   ...
```

```bash
go-arch-lint mapping --scheme grouped

module: github.com/fe3dback/go-arch-lint
Project Packages:
   app:
     /internal/app
   commands:
     /internal/commands/check
     /internal/commands/mapping
   ...
```

Те же данные доступны в формате json с опцией `--json`.
