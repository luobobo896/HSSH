# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

HSSH is a high-performance SSH bastion (跳板机) tool written in Go with a React web UI. It enables multi-hop SSH connections, file transfers through intermediate hosts, port forwarding, and network latency probing.

## Build Commands

```bash
# Build Go binary only
go build -o gmssh ./cmd/gmssh

# Build frontend only
cd web && npm run build

# Build complete application (frontend + Go binary)
cd web && npm run build && cd .. && go build -o gmssh ./cmd/gmssh

# Development mode - Frontend with hot reload
cd web && npm run dev
# Dev server: http://localhost:18080, proxies API to localhost:18081

# Development mode - Go backend only
go run ./cmd/gmssh web --local
# Runs on http://127.0.0.1:8080
```

## Test Commands

```bash
# Frontend unit tests
cd web && npm test          # Run once (vitest)
cd web && npm run test:watch    # Watch mode
cd web && npm run test:coverage # With coverage

# E2E tests
npx playwright test e2e/terminal.spec.ts
```

## Project Structure

```
gmssh/
├── cmd/gmssh/         # Application entry point
│   └── main.go        # CLI command dispatch
├── internal/          # Internal Go packages
│   ├── api/          # HTTP API server (REST + WebSocket), embeds web/dist
│   ├── cli/          # CLI command implementations
│   ├── config/       # YAML configuration management (~/.gmssh/config.yaml)
│   ├── profiler/     # Network latency probing with caching
│   ├── proxy/        # TCP port forwarding through SSH tunnels
│   ├── ssh/          # SSH client and multi-hop chain management
│   ├── terminal/     # WebSocket terminal implementation
│   └── transfer/     # SCP file transfer
├── pkg/types/        # Shared type definitions (Hop, Route, etc.)
├── web/              # React + TypeScript frontend
│   ├── src/
│   │   ├── api/     # API client functions (axios)
│   │   ├── pages/   # Page components (Servers.tsx, Transfer.tsx)
│   │   ├── stores/  # Zustand state management
│   │   └── types/   # TypeScript type definitions
│   └── vite.config.ts  # Dev server port 18080, proxies /api to 18081
├── e2e/              # Playwright E2E tests
└── embed.go          # Embeds web/dist into Go binary
```

## Key Architecture Details

### Frontend Embedding
The Go binary embeds the built frontend from `web/dist/` via `embed.go`. After modifying frontend code, you must:
1. Build frontend: `cd web && npm run build`
2. Rebuild Go binary: `go build -o gmssh ./cmd/gmssh`

### SSH Chain Architecture
- `internal/ssh/chain.go` manages multi-hop SSH connections
- Supports connecting through bastion hosts (gateways)
- Both key-based and password authentication
- Host key verification currently disabled (`ssh.InsecureIgnoreHostKey`)

### API Server
- `internal/api/server.go` implements REST API and WebSocket
- Static files served from embedded `web/dist`
- CORS enabled for all origins (`*`)
- Key endpoints: `/api/servers`, `/api/upload`, `/api/proxy`, `/api/terminal` (WebSocket)

### Configuration
- Stored in `~/.gmssh/config.yaml`
- Created automatically with 0700 permissions on first run
- Contains `hops` (servers), `routes` (path preferences), `profiles` (latency cache)

## CLI Commands

```bash
# File upload through bastion
./gmssh upload --source ./file.txt --target gateway:/data/ --via bastion-hk

# Port forwarding
./gmssh proxy --local :3306 --remote-host internal-db --remote-port 3306 --via gateway

# Latency probing
./gmssh probe --target internal-server --via gateway

# Server management
./gmssh server list
./gmssh server add --name gateway --host gw.example.com --user admin --auth key
./gmssh server delete gateway

# Web UI
./gmssh web --local              # localhost:8080
./gmssh web --bind 0.0.0.0:18081 # Network accessible
```

## Code Conventions

### Go
- Comments in Chinese for internal documentation
- Error wrapping: `fmt.Errorf("context: %w", err)`
- Table-driven tests (when tests exist)

### Frontend
- TypeScript with strict mode
- Tailwind CSS utility classes
- Chinese UI text (中文界面)
- Zustand for state management
- Axios for HTTP requests

## Important Notes

- **No test coverage currently** - AGENTS.md mentions this as known limitation
- **Passwords stored in plaintext** in config (marked with `json:"-"`)
- **Host key verification disabled** - uses `ssh.InsecureIgnoreHostKey()`
- Frontend dev server proxies `/api` to `localhost:18081` (see vite.config.ts)
