# reMarker — Agent Instructions

## Project
Automatic syncing tool for the reMarkable 2 e-ink tablet. Written in Go.

## Quick Facts
- **Default branch**: `production` (not `main`)
- **License**: Apache 2.0
- **Env**: `.env` file required at runtime (gitignored)

## Go Conventions
- Standard Go tooling: `go build`, `go test ./...`
- Binaries, `.test` files, and coverage artifacts are gitignored
- No `vendor/` directory (uses module proxy)

## Architecture: Onion / Clean Architecture + DDD

This project follows **Onion (Clean) Architecture** guided by **Domain-Driven Design** principles. All new code must conform.

### Layer structure (inner → outer)
1. **Domain** — Entities, value objects, aggregates, domain services, repository interfaces, domain errors. Depends on nothing else.
2. **Application** — Use cases / application services, DTOs, orchestrating domain logic. Imports only `domain`.
3. **Infrastructure** — Database access, external APIs (reMarkable cloud, SSH, etc.), file I/O, config loading, logging. Implements repository interfaces. Imports `application` and `domain`.
4. **Interface / CLI** — Command-line entry points, argument parsing, wiring (dependency injection). Imports `application` and `infrastructure`.

### Dependency rule
**Inner layers must never import outer layers.** If a domain entity references an infrastructure type, you have an architecture violation. Use interfaces defined in the inner layer and implemented in the outer layer.

### DDD expectations
- Model bounded contexts around reMarkable concepts (documents, notes, devices, sync sessions).
- Entities carry business rules; anemeric structs are an anti-pattern.
- Repository interfaces belong in `domain`; implementations live in `infrastructure`.
- Domain events and aggregates are preferred over procedural scripts.

### Directory layout (guidance, not rigid)
```
internal/
  domain/        # core business model
  application/   # use cases, app services
  infrastructure/ # db, rmapi client, config, adapters
  cli/           # cobra/urfave commands, main wiring
```

## Development Process: TDD

**Test-Driven Development is mandatory.** Every feature and bug fix must follow the red-green-refactor cycle.

### Rules
1. **Write the test first.** No production code without a failing test.
2. **Red** — Confirm the test fails (proves the test is valid).
3. **Green** — Write the minimal code to make the test pass.
4. **Refactor** — Clean up while keeping all tests green.
5. **Commit green.** Never commit with failing tests.

### Testing conventions
- Unit tests live alongside source (`*_test.go`).
- Table-driven tests are preferred (`for _, tc := range tests`).
- Use `testify/assert` or stdlib `testing` — be consistent within a package.
- Infrastructure tests that require external services (e.g., reMarkable API) should use mocks or testcontainers, never hit production.
- Run `go test ./...` before committing. All tests must pass.

## Commit Messages
- Use plain, direct language — grammatical correctness is not required.
- **No prefixes** like `feat:`, `fix:`, `bug:`, `refactor:`, etc.
- The **subject line** should state what the commit accomplishes (e.g. "add document sync use case", not "feat: add document sync").
- Always use the **commit description / body** for detailed information about the changes.

## Project State

### What exists
- All 5 CLI commands implemented: `init`, `sync`, `status`, `watch`, `ssh`
- Full Clean Architecture: domain → application → infrastructure → cli
- SSH to device via Dropbear (password auth, PTY shell, `xterm` terminal type)
- SFTP client with atomic writes (temp + rename) to xochitl directory
- Local filesystem client with SHA256 hashing, skips `.git/` and `.remarker/`
- JSON manifest store at `.remarker/manifest.json` for state tracking
- Watch mode with fsnotify + periodic timer
- Config from environment variables only, no config file
- Docker support with multi-stage build + docker-compose

### Current sync behavior
- **Direction**: local → device only (push). Does NOT pull device files to local.
- **Deletions**: ignored on both sides. Files removed from manifest but not deleted.
- **Scope**: `./documents/` locally ↔ `/home/root/.local/share/remarkable/xochitl` on device
- **Conflicts**: creates `.conflict` copy, reports in sync summary

### Known gaps (not yet implemented)
- **Pull sync**: device → local direction. First sync from empty local does nothing.
- **Delete propagation**: deletions are silently ignored, not configurable.
- **Progress for status/watch**: only sync command has progress callbacks.

### SSH gotchas
- Device runs Dropbear SSH (not OpenSSH) — needs explicit `HostKeyAlgorithms` in config
- `isTerminal()` must use `unix.Termios` (44 bytes) for ioctl, NOT `uint32` (stack corruption)
- `Shell()` must call `session.Wait()` or returns immediately
- Terminal type must be `xterm`, not `xterm-256color` (Dropbear compatibility)
