# reMarker Specification

**Local-only, SSH-based bidirectional sync tool for reMarkable 2 e-ink tablet**

## Summary

reMarker is a local-only, SSH-based bidirectional sync tool for the reMarkable 2 e-ink tablet, written in Go. It syncs the entire xochitl directory between a local filesystem and the device via SSH/SFTP, tracking state in a local manifest file. This document captures all requirements, architecture, domain models, infrastructure packages, current progress, and development guidelines.

## Table of Contents

- [Project Overview](#project-overview)
- [User Story & Requirements](#user-story--requirements)
- [Architecture](#architecture)
- [CLI Commands](#cli-commands)
- [Environment Variables](#environment-variables)
- [Domain Model](#domain-model)
- [Infrastructure Packages](#infrastructure-packages)
- [Current Progress](#current-progress)
- [Critical Context](#critical-context)
- [Development Process](#development-process)

---

## Project Overview

| Property | Value |
|----------|-------|
| **Name** | reMarker |
| **Language** | Go |
| **Module** | `github.com/hollen/remarker` |
| **Purpose** | Automatic bidirectional sync between local filesystem and reMarkable 2 device via SSH/SFTP |
| **Default Git Branch** | `production` (not `main`) |
| **License** | Apache 2.0 |
| **Environment** | `.env` file required at runtime (gitignored) |

### Purpose

reMarker provides automatic, bidirectional synchronization between a local filesystem directory and the reMarkable 2 tablet's document storage. Unlike cloud-based solutions, reMarker operates entirely locally, using SSH to connect to the device and SFTP to transfer files. This ensures privacy, speed, and offline capability.

---

## User Story & Requirements

### Core Requirements

- **Local filesystem only** — No internet/cloud sync; all operations happen locally
- **SSH connection** — Connect to `10.11.99.1:22` as `root`
- **Credential management** — Password provided via `REMARKABLE_PASSWORD` environment variable
- **Sync scope** — Entire xochitl directory: `/home/root/.local/share/remarkable/xochitl/`
- **Bidirectional sync** — Files can be pushed from local to device and pulled from device to local
- **Conflict handling** — Conflicts receive `.conflict` suffix; user is notified
- **Deletion policy** — Deletions default to no action; configurable later
- **State tracking** — Local `.remarker/manifest.json` only; never synced to device
- **Docker-friendly** — Supports volume-mounted `documents/` folder

### CLI Commands

| Command | Description |
|---------|-------------|
| `remarker init` | Create `.remarker/` directory, initialize manifest |
| `remarker sync` | Run bidirectional sync |
| `remarker status` | Preview changes without applying |
| `remarker watch` | Daemon mode with fsnotify + periodic timer |
| `remarker ssh` | Proxy to raw SSH shell on device |

### Configuration

- **No config file** — Configuration via environment variables and CLI flags only
- **Ignore patterns** — `.git/` directories automatically skipped
- **Watch mode** — fsnotify integration + periodic sync (default 5 minutes, configurable via `SYNC_INTERVAL`)

### Default Values

| Variable | Default |
|----------|---------|
| `REMARKABLE_HOST` | `10.11.99.1` |
| `REMARKABLE_PORT` | `22` |
| `REMARKABLE_USER` | `root` |
| `REMARKER_SYNC_DIR` | `./documents` |
| `SYNC_INTERVAL` | `5m` |

---

## Architecture

### Design Principles

reMarker follows **Onion (Clean) Architecture** guided by **Domain-Driven Design (DDD)** principles. This separation ensures:

- Inner layers depend only on abstractions
- Outer layers implement concrete details
- Easy testing with mocks
- Clear separation of concerns

### Layer Structure (inner → outer)

```
┌─────────────────────────────────────────┐
│          Interface / CLI                 │
│  └─ cobra commands, argument parsing     │
│  └─ dependency injection wiring         │
├─────────────────────────────────────────┤
│         Infrastructure                   │
│  └─ SSH, SFTP, localfs, manifest, config │
│  └─ implements repository interfaces     │
├─────────────────────────────────────────┤
│          Application                     │
│  └─ use cases, DTOs, orchestration      │
│  └─ imports only domain                 │
├─────────────────────────────────────────┤
│            Domain                        │
│  └─ entities, value objects, aggregates  │
│  └─ repository interfaces, domain errors │
│  └─ depends on nothing else             │
└─────────────────────────────────────────┘
```

### Dependency Rule

**Inner layers must never import outer layers.**

- `domain` depends on nothing
- `application` imports only `domain`
- `infrastructure` imports `application` and `domain`
- `cli` imports `application` and `infrastructure`

### Directory Layout

```
internal/
  domain/        # core business model
  application/   # use cases, app services, DTOs
  infrastructure/ # db, ssh, sftp, localfs, manifest, config, watcher
  cli/           # cobra/urfave commands, main wiring
```

### DDD Expectations

- Model bounded contexts around reMarkable concepts (documents, notes, devices, sync sessions)
- Entities carry business rules; anonymous structs are an anti-pattern
- Repository interfaces defined in `domain`, implementations in `infrastructure`
- Domain events and aggregates preferred over procedural scripts

---

## CLI Commands

### `remarker init`

Creates the `.remarker/` directory and initializes an empty manifest file.

**Usage:**
```bash
remarker init
```

**Effect:**
- Creates `.remarker/` directory in sync root
- Creates `manifest.json` with initial structure
- Validates SSH connection (optional)

---

### `remarker sync`

Runs a single bidirectional sync operation.

**Usage:**
```bash
remarker sync
```

**Effect:**
- Compares local and device filesystems against manifest
- Applies pending actions (push/pull)
- Handles conflicts with `.conflict` suffix
- Updates manifest with sync timestamp

---

### `remarker status`

Previews changes without applying them.

**Usage:**
```bash
remarker status
```

**Effect:**
- Lists files to be pushed, pulled, or deleted
- Shows conflicts that would occur
- Does not modify manifest or filesystem

---

### `remarker watch`

Daemon mode with continuous monitoring.

**Usage:**
```bash
remarker watch
```

**Effect:**
- Starts fsnotify watcher on sync directory
- Runs periodic sync every `SYNC_INTERVAL` (default 5m)
- Triggers sync on file system events
- Graceful shutdown on interrupt

---

### `remarker ssh`

Proxy to raw SSH shell on device.

**Usage:**
```bash
remarker ssh
```

**Effect:**
- Establishes SSH connection to device
- Opens interactive shell session
- Uses same credentials as sync

---

## Environment Variables

| Variable | Type | Default | Required | Description |
|----------|------|---------|----------|-------------|
| `REMARKABLE_PASSWORD` | string | - | **Yes** | SSH password for root user |
| `REMARKABLE_HOST` | string | `10.11.99.1` | No | reMarkable device IP address |
| `REMARKABLE_PORT` | integer | `22` | No | SSH port |
| `REMARKABLE_USER` | string | `root` | No | SSH username |
| `REMARKER_SYNC_DIR` | string | `./documents` | No | Local sync directory |
| `SYNC_INTERVAL` | duration | `5m` | No | Watch mode sync interval |

---

## Domain Model

### File

Represents a file in the sync system.

| Field | Type | Description |
|-------|------|-------------|
| `Path` | string | Relative path from sync root |
| `Size` | int64 | File size in bytes |
| `ModTime` | time.Time | Last modification time |
| `Hash` | string | SHA256 hash (hex) for integrity check |

---

### Manifest

The central state tracking structure.

| Field | Type | Description |
|-------|------|-------------|
| `Version` | string | Schema version for migrations |
| `LastSync` | time.Time | Timestamp of last successful sync |
| `Entries` | map[string]ManifestEntry | Path → ManifestEntry mapping |

---

### ManifestEntry

Individual file entry in the manifest.

| Field | Type | Description |
|-------|------|-------------|
| `Path` | string | Relative path |
| `Hash` | string | SHA256 hash at time of sync |
| `Size` | int64 | File size in bytes |
| `ModTime` | time.Time | Modification time at time of sync |
| `SyncedAt` | time.Time | Timestamp when this entry was synced |

---

### SyncAction

Represents an action to be taken during sync.

| Field | Type | Description |
|-------|------|-------------|
| `ActionType` | ActionType | Type of action to perform |
| `Path` | string | Relative path of file |
| `SourceFile` | *File | Source file (local or device) |
| `DestFile` | *File | Destination file |

---

### ActionType

Enumeration of possible sync actions.

| Value | Description |
|-------|-------------|
| `none` | No action required |
| `push` | Copy from local to device |
| `pull` | Copy from device to local |
| `conflict` | Conflict detected; requires user intervention |
| `delete_local` | Delete from local filesystem |
| `delete_device` | Delete from device |

---

### SyncResult

Result of a sync operation.

| Field | Type | Description |
|-------|------|-------------|
| `Actions` | []SyncAction | List of actions performed |
| `Conflicts` | int | Number of conflicts encountered |
| `Errors` | []error | List of errors encountered |

---

### Domain Errors

| Error | Description |
|-------|-------------|
| `ConflictError` | Conflict detected between local and device versions |
| `SyncError` | General synchronization failure |
| `ManifestError` | Manifest read/write failure |

---

## Infrastructure Packages

### config

Environment configuration loading and validation.

| Method | Description |
|--------|-------------|
| `Load()` | Loads configuration from environment variables with defaults |
| `Validate()` | Validates configuration values |

**Location:** `infrastructure/config/config.go`

---

### ssh

SSH connection management.

| Method | Description |
|--------|-------------|
| `Dial()` | Establishes SSH connection to device |
| `Close()` | Closes SSH connection |

**Location:** `infrastructure/ssh/ssh.go`

---

### sftp

SFTP client implementing `DeviceRepository`.

| Method | Description |
|--------|-------------|
| `ListFiles()` | Lists files in remote directory |
| `GetFile()` | Downloads file from device |
| `PutFile()` | Uploads file to device (atomic: temp + rename) |
| `DeleteFile()` | Deletes file from device |

**Location:** `infrastructure/sftp/sftp.go`

---

### localfs

Local filesystem client implementing `LocalRepository`.

| Method | Description |
|--------|-------------|
| `ListFiles()` | Lists files in local directory (recursive) |
| `GetFile()` | Reads file from local filesystem |
| `PutFile()` | Writes file to local filesystem |
| `DeleteFile()` | Deletes file from local filesystem |
| `Walk()` | Recursive directory walk, skips `.git/` and `.remarker/` |
| `Hash()` | Computes SHA256 hash of file |

**Location:** `infrastructure/localfs/localfs.go`

---

### manifeststore

Manifest storage implementing `ManifestRepository`.

| Method | Description |
|--------|-------------|
| `Read()` | Loads manifest from disk |
| `Write()` | Saves manifest to disk (atomic) |
| `Update()` | Updates manifest entries |

**Location:** `infrastructure/manifeststore/manifeststore.go`

---

### watcher

File system watcher for watch mode.

| Method | Description |
|--------|-------------|
| `Start()` | Starts fsnotify watcher |
| `Stop()` | Stops watcher |
| `On()` | Registers callback for file system events |

**Features:**
- Recursive watching of sync directory
- Event deduplication
- Skip rules for `.git/` and `.remarker/`

**Location:** `infrastructure/watcher/watcher.go`

---

## Current Progress

### Completed

- ✅ **CLI scaffolding** with cobra
- ✅ **Domain layer** — entities, interfaces, errors
- ✅ **Infrastructure packages** — all with tests
  - `config`
  - `ssh`
  - `sftp`
  - `localfs`
  - `manifeststore`
  - `watcher`

### In Progress

- ⏳ **Application layer** — use cases, DTOs, orchestration
- ⏳ **CLI command implementations** — wired commands

### Pending

- ⏳ Unit and integration tests for application layer
- ⏳ End-to-end testing
- ⏳ Docker configuration

---

## Critical Context

### Connection Requirements

- **SSH only via USB Ethernet bridge** — IP `10.11.99.1`
- **Does not work via Wi-Fi** — Wi-Fi connection does not expose SSH
- **Authentication** — Password found at `Settings → Help → Copyrights and licenses → GPLv3 Compliance`

### Performance

- **Throughput** — ~2-4 MB/s (SSH/SFTP limited)
- **Latency** — Dependent on USB connection quality

### SFTP Considerations

- **Atomic writes required** — Use temp file + rename pattern
- **File locking** — Consider during concurrent operations

### reMarkable Behavior

- **xochitl must be restarted after file pushes** — Causes brief blank UI
- **File naming** — reMarkable uses specific naming conventions
- **Folder structure** — xochitl directory contains user notebook structure

### Local State

- **`.remarker/` directory is local-only** — Never synced to device
- **Manifest format** — JSON with version field for migrations
- **Backups** — Consider manifest backup strategy for safety

---

## Development Process

### Test-Driven Development (TDD)

**Mandatory red-green-refactor cycle:**

1. **Write the test first** — No production code without a failing test
2. **Red** — Confirm the test fails (proves the test is valid)
3. **Green** — Write minimal code to make the test pass
4. **Refactor** — Clean up while keeping all tests green
5. **Commit green** — Never commit with failing tests

### Testing Conventions

- **Unit tests** — Live alongside source code (`*_test.go`)
- **Table-driven tests** — Preferred pattern: `for _, tc := range tests`
- **Test frameworks** — Use `testify/assert` or stdlib `testing` consistently
- **Infrastructure tests** — Use mocks or testcontainers; never hit production services
- **Run before committing** — `go test ./...` must pass

### Commit Messages

- **Plain language** — No prefixes like `feat:`, `fix:`, `bug:`, `refactor:`
- **Subject line** — State what the commit accomplishes
  - Example: `add document sync use case` (not `feat: add document sync`)
- **Body** — Use commit description for detailed information

### Go Conventions

- **Standard tooling** — `go build`, `go test ./...`
- **Binaries and test artifacts** — Gitignored
- **No vendor directory** — Uses module proxy
- **Module path** — `github.com/hollen/remarker`

### Code Quality

- **Linting** — Run before committing
- **Formatting** — `go fmt`
- **Coverage** — Maintain reasonable test coverage
