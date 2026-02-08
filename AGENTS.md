# HSSH - Agent Guide

## Project Overview

HSSH is a high-performance SSH bastion (跳板机) tool written in Go, featuring both CLI and Web UI interfaces. It enables multi-hop SSH connections, file transfers through intermediate hosts, port forwarding, and network latency probing.

**Key Features:**
- Chain SSH connections through multiple hops (bastion hosts)
- File upload via SCP through intermediate servers
- TCP port forwarding through SSH tunnels
- Network path latency probing with caching
- Web-based management UI (React + TypeScript)
- YAML-based configuration

## Technology Stack

### Backend
- **Language:** Go 1.25.6
- **Key Dependencies:**
  - `golang.org/x/crypto` - SSH client implementation
  - `gopkg.in/yaml.v3` - Configuration file parsing
- **Architecture:** Standard Go project layout with `internal/` and `pkg/` packages

### Frontend
- **Framework:** React 18 + TypeScript
- **Build Tool:** Vite 5
- **Styling:** Tailwind CSS 3.3 with custom glassmorphism design
- **State Management:** Zustand 4
- **HTTP Client:** Axios

## Project Structure

```
gmssh/
├── cmd/gmssh/              # Application entry point
│   └── main.go            # CLI argument parsing and command dispatch
├── internal/              # Internal packages (not importable)
│   ├── api/              # HTTP API server (REST + WebSocket)
│   ├── cli/              # CLI command implementations
│   ├── config/           # Configuration management (YAML)
│   ├── profiler/         # Network latency probing
│   ├── proxy/            # TCP port forwarding
│   ├── ssh/              # SSH client and chain management
│   └── transfer/         # SCP file transfer implementation
├── pkg/types/            # Shared type definitions
├── web/                  # React frontend application
│   ├── src/
│   │   ├── api/         # API client functions
│   │   ├── pages/       # Page components (Servers, Transfer)
│   │   ├── stores/      # Zustand state stores
│   │   └── types/       # TypeScript type definitions
│   ├── package.json
│   └── vite.config.ts
└── docs/plans/          # Design documentation (Chinese)
```

## Build and Development

### Prerequisites
- Go 1.25.6 or later
- Node.js 18+ with npm

### Build Commands

```bash
# Build Go binary
go build -o gmssh ./cmd/gmssh

# Development - Frontend only
cd web && npm install && npm run dev
# Frontend dev server runs on http://localhost:18080

# Build frontend for production
cd web && npm run build
# Output: web/dist/

# Build complete application
# 1. Build frontend
cd web && npm run build
# 2. Build Go binary (embeds web/dist)
go build -o gmssh ./cmd/gmssh
```

### Run Commands

```bash
# CLI mode
./gmssh upload --source ./file.txt --target gateway:/data/
./gmssh proxy --local :3306 --remote-host internal-db --remote-port 3306 --via gateway
./gmssh probe --target internal-server
./gmssh status
./gmssh server list
./gmssh server add --name gateway --host gw.example.com --user admin --auth key

# Web UI mode (local only)
./gmssh web --local
# Runs on http://127.0.0.1:8080

# Web UI mode (server mode)
./gmssh web --bind 0.0.0.0:18081
```

## Configuration

Configuration is stored in `~/.gmssh/config.yaml`:

```yaml
hops:
  - name: gateway
    host: gw.example.com
    port: 22
    user: admin
    auth: key
    key_path: ~/.ssh/id_rsa

routes:
  - from: localhost
    to: internal
    via: gateway
    threshold: 50

profiles: []
```

**Note:** The config directory is created automatically with 0700 permissions on first run.

## Code Style Guidelines

### Go Code
- Follow standard Go conventions (gofmt)
- Comments in Chinese for internal documentation
- Error wrapping with context: `fmt.Errorf("context: %w", err)`
- Use `internal/` for non-public packages
- Types defined in `pkg/types/` for shared use

### Frontend Code
- TypeScript with strict mode enabled
- Functional React components with hooks
- Tailwind CSS utility classes
- Chinese UI text (项目主要使用中文界面)

## Key Components

### SSH Chain (`internal/ssh/`)
- `Chain` - Manages multi-hop SSH connections
- `Client` - Individual SSH client with support for connecting through bastions
- Supports both key-based and password authentication
- Uses `golang.org/x/crypto/ssh` for underlying SSH protocol

### File Transfer (`internal/transfer/`)
- `SCPTransfer` - Implements SCP protocol for file upload/download
- Progress reporting via channels
- Support for directory uploads

### Port Forwarding (`internal/proxy/`)
- `PortForwarder` - TCP port forwarding through SSH tunnel
- `ForwarderManager` - Manages multiple active forwards
- Bidirectional data copy with goroutines

### Network Profiler (`internal/profiler/`)
- `NetworkProfiler` - Measures SSH connection latency
- Caching with TTL for latency reports
- Path comparison to select optimal route

## API Endpoints

The Web UI communicates with the Go backend via REST API:

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/servers` | GET/POST | List/add servers |
| `/api/servers/:name` | GET/PUT/DELETE | Server operations |
| `/api/servers/:name/test` | POST | Test connection |
| `/api/routes` | GET/POST | Route preferences |
| `/api/upload` | POST | File upload (multipart) |
| `/api/proxy` | GET/POST | Port forwarding |
| `/api/proxy/:id` | GET/DELETE | Proxy management |
| `/api/ws/progress/:taskId` | GET | Upload progress polling |

## Security Considerations

- **Host Key Verification:** Currently uses `ssh.InsecureIgnoreHostKey()` - not for production
- **Password Storage:** Passwords stored in plaintext in config (marked with `json:"-"` to prevent serialization)
- **File Permissions:** Config directory created with 0700, config file with 0600
- **CORS:** API server allows all origins (`*`) - adjust for production

## Testing

**Note:** The project currently has no automated tests. When adding tests:

- Go tests: Use `*_test.go` files alongside source files
- Frontend tests: Use `.test.ts` or `.test.tsx` naming convention
- Mock SSH connections for unit tests
- Use table-driven tests for Go (idiomatic pattern)

## Known Limitations

1. No test coverage currently implemented
2. Host key verification is disabled
3. Passwords stored in plaintext in config file
4. Web UI port forwarding page is not fully implemented (placeholder)
5. File upload in API is simulated (TODO comment in `executeUpload`)

## Development Workflow

1. Frontend development: `cd web && npm run dev`
2. Backend development: `go run ./cmd/gmssh web --local`
3. Frontend API calls proxy to `localhost:18081` via Vite config
4. Build frontend before committing: `cd web && npm run build`
5. Binary serves static files from `./web/dist/`

## Documentation Language

Project documentation and comments are primarily in **Chinese (中文)**. All UI text, design documents, and inline comments use Chinese as the main language.
