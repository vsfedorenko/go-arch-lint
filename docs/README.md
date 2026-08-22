# Документация

- [Синтаксис DSL](syntax/README.md) — все функции с примерами
- [Миграция с YAML на Go DSL](migration-v2.md) — для проектов на v1.x
- [Кукбук архитектурных паттернов](cookbook.md) — готовые конфиги: clean, DDD, feature-based и другие
- [JSON и другие форматы для CI](json-schema.md) — json/sarif/junit/github-actions, коды выхода, лимиты вывода
- [Модель делегирования](delegation.md) — как CLI запускает вашу спеку через `go run`

## Расширенное использование

### Справка по DSL API

Схема конфигурации определяется пакетом `github.com/vsfedorenko/go-arch-lint/v3/dsl`.
Поскольку конфигурация теперь чистый Go, компилятор и автодополнение IDE
заменяют старый слой JSON Schema.

Чтобы посмотреть полное API с сигнатурами и документацией, выполните:

```bash
go doc github.com/vsfedorenko/go-arch-lint/v3/dsl
```

Или посмотрите конкретную функцию:

```bash
go doc github.com/vsfedorenko/go-arch-lint/v3/dsl.Spec
go doc github.com/vsfedorenko/go-arch-lint/v3/dsl.Component
go doc github.com/vsfedorenko/go-arch-lint/v3/dsl.Deps
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

## JSON-вывод для CI

Схема violation-вывода (`check --format json`) и готовые примеры
интеграции (GitHub Actions, GitLab CI): [json-schema.md](json-schema.md).

## Модель делегирования `go run`

Как лаунчер делегирует команды в `.go-arch-lint/`: маршрутизация флагов,
выходные коды, кэширование (холодный/тёплый запуск ~45 с → ~2 с), все
режимы отказов и известные острые углы: [delegation.md](delegation.md).
