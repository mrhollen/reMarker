# reMarker

Automatic SSH syncing for the reMarkable 2 e-ink tablet.

Bidirectional sync between a local `documents/` directory and your reMarkable device over USB Ethernet — no cloud, no internet required.

## Quick Start

```bash
# 1. Set your reMarkable password
export REMARKABLE_PASSWORD="your-password-here"

# 2. Initialize and sync
./remarker init
./remarker sync

# 3. Or run continuously in watch mode
./remarker watch
```

See [Building from Source](#building-from-source) or [Docker](#docker) for installation options.

## Prerequisites

- **Go 1.24+** (or use the Docker setup below)
- **reMarkable 2** connected via USB cable
- USB Ethernet bridge active at `10.11.99.1` (default)

## Building from Source

```bash
git clone https://github.com/hollen/remarker.git
cd remarker
go build -o remarker ./cmd
```

This produces a single binary at `./remarker`. No additional dependencies or vendored libraries are needed.

To install globally:

```bash
go install ./cmd
```

This places `remarker` in your `$GOPATH/bin` (or `$HOME/go/bin`).

## Setup

### 1. Find Your reMarkable Password

Go to **Settings** → **Help** → **Copyrights and licenses** → **GPLv3 Compliance**. The SSH password is displayed on that page.

### 2. Set Environment Variables

The only required variable is `REMARKABLE_PASSWORD`. All others have sensible defaults:

```bash
export REMARKABLE_PASSWORD="your-password-here"
```

### 3. Initialize

Run `init` to create the `.remarker/` directory and sync manifest in your documents folder:

```bash
./remarker init
```

This creates `.remarker/manifest.json` to track file state between syncs.

### 4. Sync

```bash
./remarker sync
```

This performs a bidirectional sync between `./documents/` and the reMarkable's xochitl directory.

## Commands

| Command | Description |
|---------|-------------|
| `remarker init` | Initialize sync manifest in the documents directory |
| `remarker sync` | Perform a one-time bidirectional sync |
| `remarker status` | Preview pending changes without applying them |
| `remarker watch` | Continuously watch for changes and auto-sync |
| `remarker ssh` | Open an interactive SSH shell on the device |

## Configuration

All configuration is via environment variables — no config files.

| Variable | Default | Description |
|----------|---------|-------------|
| `REMARKABLE_PASSWORD` | *(required)* | SSH password for the reMarkable device |
| `REMARKABLE_HOST` | `10.11.99.1` | Device IP address |
| `REMARKABLE_PORT` | `22` | SSH port |
| `REMARKABLE_USER` | `root` | SSH username |
| `REMARKER_SYNC_DIR` | `./documents` | Local directory to sync |
| `SYNC_INTERVAL` | `5m` | Watch mode sync interval |
| `REMARKABLE_CONNECTION_TIMEOUT` | `30s` | SSH connection timeout |

Duration values accept Go duration strings: `30s`, `2m`, `1h`, etc.

## Docker

Build and run with Docker Compose:

```bash
# Set password (required)
export REMARKABLE_PASSWORD="your-password-here"

# Build the image
make build

# Initialize
make init

# Sync once
make sync

# Run watch mode
make watch

# Stop
make stop
```

Or with `docker compose` directly:

```bash
docker compose run --rm remarker sync
docker compose up        # watch mode
docker compose down      # stop
```

Your documents are stored in the `./documents/` directory, mounted as a volume into the container.

## Conflict Resolution

When the same file is modified on both sides since the last sync, reMarker creates a `.conflict` copy to preserve both versions and reports the conflict in the sync summary.

## Notes

- The USB Ethernet bridge (`10.11.99.1`) only works when the device is connected via USB cable, not over Wi-Fi.
- SFTP throughput is approximately 2-4 MB/s.
- Pushing files to the device will briefly blank the xochitl UI as it restarts.
- `.git/` and `.remarker/` directories are always excluded from sync.

## License

Apache 2.0
