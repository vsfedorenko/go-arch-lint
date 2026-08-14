# Architecture Cookbook

Ready-to-use go-arch-lint configurations for common Go architecture patterns. Each
recipe includes a description, an ASCII dependency diagram, a complete DSL example,
and guidance on when to use it.

> New to the DSL? Read the **[syntax reference](syntax/README.en.md)** first.

All examples assume your project lives under `internal/` and that your `arch.go`
file sits at `.go-arch-lint/arch.go`. Adjust `Workdir` and component paths to match
your layout.

---

## Table of contents

- [1. Clean Architecture (Hexagonal / Ports & Adapters)](#1-clean-architecture-hexagonal--ports--adapters)
- [2. Layered (handler → service → repository)](#2-layered-handler--service--repository)
- [3. Domain-Driven Design (domain → application → infrastructure)](#3-domain-driven-design-domain--application--infrastructure)
- [4. Feature-based (Modular / vertical slices)](#4-feature-based-modular--vertical-slices)
- [5. Simple MVC](#5-simple-mvc)

---

## 1. Clean Architecture (Hexagonal / Ports & Adapters)

### Description

The application core (domain + application logic) owns the business rules and knows
nothing about the outside world. External concerns — HTTP handlers, database
repositories, message-queue publishers — are *adapters* that plug into *ports*
(interfaces) defined by the core. Dependencies always point inward: adapters depend
on the core; the core never depends on an adapter.

### Diagram

```
                    ┌─────────────────────────────────────────┐
                    │                 adapters                 │
                    │   ┌─────────┐   ┌─────────┐   ┌───────┐ │
        HTTP ──────▶│   │  http   │   │   db    │   │  mq   │ │
                    │   └────┬────┘   └────┬────┘   └───┬───┘ │
                    └────────│─────────────│────────────│─────┘
                             │             │            │
                             ▼             ▼            ▼
                    ┌─────────────────────────────────────────┐
                    │            application core              │
                    │   ┌─────────────────────────────────┐   │
                    │   │   core (use cases / services)    │   │
                    │   └────────────────┬────────────────┘   │
                    │                    │                     │
                    │   ┌────────────────▼────────────────┐   │
                    │   │   domain (entities / models)     │   │
                    │   └─────────────────────────────────┘   │
                    └─────────────────────────────────────────┘
```

### DSL example

```go
package main

import . "github.com/vsfedorenko/go-arch-lint/dsl"

var _ = Spec(func() {
    Version(1)
    Workdir("internal")

    Allow(func() {
        DepOnAnyVendor(false)
    })

    ExcludeFiles(`^.*_test\.go$`)

    // --- core (inward) ---
    Component("domain", "domain")        // entities, value objects
    Component("core", "core")            // use cases / application services

    // --- adapters (outward) ---
    Component("http", "adapter/http")    // inbound: REST / gRPC
    Component("db", "adapter/db")        // outbound: persistence
    Component("mq", "adapter/mq")        // outbound: messaging

    // domain is pure business knowledge — any layer may reference it
    CommonComponents("domain")

    Deps("core", func() {
        MayDependOn("domain")
    })

    // every adapter depends inward on the core, never on each other
    Deps("http", func() {
        MayDependOn("core")
    })
    Deps("db", func() {
        MayDependOn("core")
    })
    Deps("mq", func() {
        MayDependOn("core")
    })
})
```

A runnable version of this pattern lives in [`examples/hexagonal`](../examples/hexagonal).

### When to use

- Long-lived applications where business rules must survive framework churn.
- Teams that want to swap transport (HTTP → gRPC) or storage (Postgres → Mongo)
  without touching the core.
- You are willing to define interfaces in the core and implement them in adapters.

---

## 2. Layered (handler → service → repository)

### Description

The classic n-tier split. Each layer may only call the layer directly beneath it:
HTTP/gRPC **handlers** call **services**, services call **repositories**, repositories
talk to the database. Models are shared upward. This is the simplest architecture that
still gives you a dependency direction to enforce.

### Diagram

```
   ┌──────────┐
   │ handler  │   HTTP / gRPC entrypoints, request parsing
   └────┬─────┘
        │
        ▼
   ┌──────────┐
   │ service  │   business logic, orchestrates repositories
   └────┬─────┘
        │
        ▼
   ┌────────────┐
   │ repository │   data access, SQL/NoSQL clients
   └────────────┘

   ┌──────────┐
   │  models  │   shared DTOs / entities (any layer may import)
   └──────────┘
```

### DSL example

```go
package main

import . "github.com/vsfedorenko/go-arch-lint/dsl"

var _ = Spec(func() {
    Version(1)
    Workdir("internal")

    Allow(func() {
        DepOnAnyVendor(false)
    })

    ExcludeFiles(`^.*_test\.go$`)

    Component("handler", "handler")
    Component("service", "service")
    Component("repository", "repository")
    Component("models", "models")

    // models are shared across all layers
    CommonComponents("models")

    Deps("handler", func() {
        MayDependOn("service")
    })

    Deps("service", func() {
        MayDependOn("repository")
    })

    // repository talks only to the DB — no upward dependencies declared
})
```

A runnable version of this pattern lives in [`examples/basic`](../examples/basic).

### When to use

- Small-to-medium services with a single transport and a single datastore.
- Teams familiar with MVC or traditional web stacks.
- You want the lowest cognitive overhead while still forbidding upward imports.

---

## 3. Domain-Driven Design (domain → application → infrastructure)

### Description

A DDD-style onion. The **domain** layer holds entities, aggregates, and domain events
and depends on nothing. The **application** layer orchestrates use cases and coordinates
domain objects. The **infrastructure** layer implements technical concerns (persistence,
messaging, external APIs). An **interfaces** layer exposes the application to the outside
world (HTTP, CLI, gRPC). Dependencies point inward; the domain is the center.

When you have multiple bounded contexts, model each aggregate as its own component and
allow cross-context references only through explicitly declared dependencies.

### Diagram

```
   ┌──────────────────────────────────────────────────┐
   │ interfaces        HTTP / CLI / gRPC controllers   │
   │  ┌────────────────────────────────────────────┐  │
   │  │ application        use cases, app services  │  │
   │  │  ┌──────────────────────────────────────┐  │  │
   │  │  │ domain (the center)                   │  │  │
   │  │  │   user entity ◀── order entity        │  │  │
   │  │  │   (aggregates know each other)        │  │  │
   │  │  └──────────────────────────────────────┘  │  │
   │  └────────────────────────────────────────────┘  │
   │                                                   │
   │ infrastructure   repositories, MQ, external APIs  │
   └──────────────────────────────────────────────────┘
```

### DSL example

```go
package main

import . "github.com/vsfedorenko/go-arch-lint/dsl"

var _ = Spec(func() {
    Version(1)
    Workdir("internal")

    Allow(func() {
        DepOnAnyVendor(false)
    })

    ExcludeFiles(`^.*_test\.go$`)

    // --- bounded contexts (domain) ---
    Component("user-domain", "domain/user")
    Component("order-domain", "domain/order")

    // --- layers ---
    Component("application", "application")
    Component("infrastructure", "infrastructure")
    Component("interfaces", "interfaces")

    // the user aggregate is referenced by the order aggregate
    CommonComponents("user-domain")

    // cross-context: order may reference user, never the reverse
    Deps("order-domain", func() {
        MayDependOn("user-domain")
    })

    Deps("application", func() {
        MayDependOn("order-domain")
    })

    Deps("infrastructure", func() {
        MayDependOn("application", "order-domain")
    })

    Deps("interfaces", func() {
        MayDependOn("application")
    })
})
```

A runnable version of this pattern lives in [`examples/ddd`](../examples/ddd).

### When to use

- Complex business domains with multiple aggregates or bounded contexts.
- You need to express and enforce cross-context dependency rules (e.g. `order` may
  reference `user` but not vice-versa).
- The team practices DDD tactical patterns (entities, value objects, repositories).

---

## 4. Feature-based (Modular / vertical slices)

### Description

Instead of splitting code by technical layer, split it by **feature**. Each feature
module (`user`, `billing`, `notification`, ...) is self-contained and owns its own
handlers, services, and storage. A shared `kernel` or `common` package holds cross-cutting
types. Features may depend on the kernel and on explicitly approved other features —
but never on arbitrary siblings. This keeps modules independently understandable and
movable.

### Diagram

```
   ┌─────────────┐  ┌─────────────┐  ┌──────────────────┐
   │  features/  │  │  features/  │  │   features/      │
   │    user     │  │   billing   │  │  notification    │
   │             │  │             │  │                  │
   │ handler     │  │ handler     │  │ consumer         │
   │ service     │  │ service     │  │ sender           │
   │ repo        │  │ repo        │  │                  │
   └──────┬──────┘  └──────┬──────┘  └────────┬─────────┘
          │                │                  │
          └────────────────┼──────────────────┘
                           │
                           ▼
                  ┌─────────────────┐
                  │  kernel / common │  shared types, errors, utils
                  └─────────────────┘
```

### DSL example

```go
package main

import . "github.com/vsfedorenko/go-arch-lint/dsl"

var _ = Spec(func() {
    Version(1)
    Workdir("internal")

    Allow(func() {
        DepOnAnyVendor(false)
    })

    ExcludeFiles(`^.*_test\.go$`)

    // one component per feature module
    Component("user", "features/user/**")
    Component("billing", "features/billing/**")
    Component("notification", "features/notification/**")

    // shared cross-cutting code
    Component("kernel", "kernel/**")

    // every feature may import the kernel
    CommonComponents("kernel")

    // explicit cross-feature dependencies — add only what you truly need
    Deps("billing", func() {
        MayDependOn("user") // billing references the user aggregate
    })

    // notification has no cross-feature deps; it only uses the kernel (via Common)
})
```

### When to use

- Monorepos or large services where different teams own different modules.
- You want each feature to be independently testable and conceptually movable.
- Microservice-candidate codebases: feature modules are the future service boundaries.

---

## 5. Simple MVC

### Description

A minimal three-way split: **models** hold data structures, **views** render output
(HTML templates, JSON serializers), and **controllers** handle input and coordinate the
other two. Suitable for small web apps, admin panels, or server-rendered sites where a
full layered or hexagonal setup would be overkill.

### Diagram

```
          ┌────────────┐
   ──────▶│ controller │──────┐
   request└────────────┘      │
             │     ▲          │
             ▼     │          ▼
          ┌────────────┐  ┌────────┐
          │   model    │  │  view  │
          └────────────┘  └────────┘
```

### DSL example

```go
package main

import . "github.com/vsfedorenko/go-arch-lint/dsl"

var _ = Spec(func() {
    Version(1)
    Workdir("internal")

    Allow(func() {
        DepOnAnyVendor(false)
    })

    ExcludeFiles(`^.*_test\.go$`)

    Component("controller", "controller/**")
    Component("model", "model/**")
    Component("view", "view/**")

    // models are read by both controllers and views
    CommonComponents("model")

    Deps("controller", func() {
        MayDependOn("view")
    })
})
```

### When to use

- Small server-rendered web apps, admin tools, or internal dashboards.
- You have one transport (HTTP) and minimal business logic.
- A full layered or hexagonal architecture would add ceremony without value.

---

## Tips

- **Start strict, relax deliberately.** Begin with `DepOnAnyVendor(false)` and an empty
  `CommonComponents` list, then add exceptions only when the linter catches a real need.
- **`models` / `domain` as `CommonComponents`.** Pure data packages are the most common
  legitimate cross-layer dependency — declare them common so you don't have to list them
  in every `Deps` block.
- **`DeepScan` for DI containers.** If a component wires dependencies via constructors
  (typical for a DI container or composition root), enable `DeepScan` there so the linter
  follows injections, not just import statements:

  ```go
  Deps("container", func() {
      AnyVendorDeps(true)
      MayDependOn("operations", "services")
  })
  ```

  `DeepScan` defaults to `true` globally; override it per component only when a
  particularly dynamic constructor produces false positives.
- **`IgnoreNotFoundComponents` for optional modules.** If a component's glob may not
  match any package in some configurations (e.g. an optional module), set it globally
  inside `Allow` to avoid noisy errors.
- **Exclude test files and mocks.** The standard pattern `ExcludeFiles(\`^.*_test\\.go$\`)`
  keeps architecture checks focused on production code. Add `\`^.*\\/mock\\/.*$\`` to also
  skip generated mock directories.
- **Split configs with `MergeSpecs`.** Large projects can keep `components`, `deps`, and
  `vendors` in separate files and merge them in `main.go` — see the
  [syntax reference](syntax/README.en.md#func-mergespecsspecs-specdef-specdef).
