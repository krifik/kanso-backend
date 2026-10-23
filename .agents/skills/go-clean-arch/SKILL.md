---
name: go-clean-arch
description: >-
  Use this skill when developing, refactoring, or adding features to the Zahra Go backend. Enforces Clean Architecture layers (domain, application, infrastructure, presentation), Fiber routing, GORM conventions, and SSE event handling.
---

# Go Clean Architecture & Fiber Guidelines for Zahra Backend

## Layer Structure & Dependency Rules

Follow the dependency rule: Inner layers must never know about outer layers.

1. **Domain Layer** (`internal/domain/`)
   - `entity/`: Plain Go structs representing core business models (e.g., `Transaction`, `Account`, `Category`).
   - `repository/`: Interfaces defining persistence contracts (e.g., `TransactionRepository`, `AccountRepository`).
   - **Rule**: No dependencies on external frameworks (GORM, Fiber, etc.).

2. **Application Layer** (`internal/application/`)
   - `dto/`: Request/Response data transfer objects.
   - `usecase/`: Business logic orchestration. Interacts exclusively with domain repository interfaces.
   - `sse_hub.go`: Realtime event broadcasting for transaction/account updates.

3. **Infrastructure Layer** (`internal/infrastructure/`)
   - `database/`: Database connection initialization (supports SQLite and PostgreSQL).
   - `repository/`: Concrete GORM implementations of domain repository interfaces.
   - `parser/`: Email parsing logic for automated bank mutation extraction.
   - `config/`: Environment variable loading.

4. **Presentation Layer** (`internal/presentation/http/`)
   - `handler/`: Fiber HTTP handlers for request validation, invoking usecases, and returning JSON/SSE responses.
   - `router/`: Fiber route definitions grouped under `/api/v1/`.

## Key Coding Conventions

- **Error Handling**: Return domain-level errors or explicit sentinel errors from usecases. Handlers map errors to HTTP status codes.
- **Dependency Injection**: Instantiate repositories in `cmd/api/main.go`, inject into usecases, then inject usecases into handlers.
- **Testing**: Place unit tests next to source files (e.g. `email_parser_test.go`) using table-driven test patterns.
