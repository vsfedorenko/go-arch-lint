# Документация

- [синтаксис](syntax/README.md)
- [migration-v2](migration-v2.md)

## Расширенное использование

### Справка по DSL API

Схема конфигурации определяется пакетом `github.com/vsfedorenko/go-arch-lint/dsl`.
Поскольку конфигурация теперь чистый Go, компилятор и автодополнение IDE
заменяют старый слой JSON Schema.

Чтобы посмотреть полное API с сигнатурами и документацией, выполните:

```bash
go doc github.com/vsfedorenko/go-arch-lint/dsl
```

Или посмотрите конкретную функцию:

```bash
go doc github.com/vsfedorenko/go-arch-lint/dsl.Spec
go doc github.com/vsfedorenko/go-arch-lint/dsl.Component
go doc github.com/vsfedorenko/go-arch-lint/dsl.Deps
```

Описание с примерами см. в [syntax/README.md](syntax/README.md).

### mapping

Сопоставление archfile с исходными файлами можно посмотреть через команду `mapping`.

Доступно два режима:
- список (по умолчанию)
- группировка по компонентам

```bash
go-arch-lint mapping

module: github.com/vsfedorenko/go-arch-lint
Project Packages:
   app                 /internal/app
   container           /internal/app/container
   commands            /internal/commands/check
   commands            /internal/commands/mapping
   ...
```

```bash
go-arch-lint mapping --scheme grouped

module: github.com/vsfedorenko/go-arch-lint
Project Packages:
   app:
     coupling: out 1 | in 0 | stability 1.00
     /internal/app
   commands:
     coupling: out 4 | in 1 | stability 0.80
     /internal/commands/check
     /internal/commands/mapping
   ...
```

#### Метрики связанности

Для каждого компонента выводятся метрики по **фактическому** графу импортов
(не по декларациям в спеке):

- `out` (Ce, fan-out) — на сколько компонентов компонент ссылается;
- `in` (Ca, fan-in) — сколько компонентов ссылаются на него;
- `stability` — I = Ce / (Ca + Ce) по Роберту Мартину. `0` — максимально
  стабильный компонент (на него опираются, сам ни на кого не завязан),
  `1` — максимально нестабильный. Компоненты, предназначенные для
  переиспользования, должны иметь низкий stability.

Те же данные доступны в формате json с опцией `--json` (поля
`MappingGrouped[].Coupling`; у компонентов без зависимостей поле
отсутствует — `omitempty`).
